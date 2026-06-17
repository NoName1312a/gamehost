// GameHost desktop shell.
//
// A thin Windows wrapper that bundles the Go engine, launches it as a sidecar
// on loopback, and renders the React UI in a native WebView2 window. The engine
// and UI are unchanged from the headless/dev setup, so the same two components
// still power the future Linux/Mac self-host deployment.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::path::PathBuf;
use std::sync::Mutex;

use tauri::{Manager, RunEvent};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

/// Holds the running engine sidecar so it can be killed when the app exits.
struct EngineProcess(Mutex<Option<CommandChild>>);

/// Resolve the game-template directory for both the installed app (bundled
/// resources) and `tauri dev` (the repo's `templates/`, via the compile-time
/// manifest dir). Returns `None` if nothing is found, in which case the engine
/// falls back to its own default template resolution.
fn resolve_templates_dir(app: &tauri::App) -> Option<PathBuf> {
    let mut candidates: Vec<PathBuf> = Vec::new();
    if let Ok(res) = app.path().resource_dir() {
        candidates.push(res.join("templates"));
    }
    // Dev fallbacks — these paths only exist on the build machine.
    candidates.push(PathBuf::from(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/resources/templates"
    )));
    candidates.push(PathBuf::from(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../templates"
    )));
    candidates.into_iter().find(|p| p.is_dir())
}

/// Resolve the bundled playit relay agent: next to the app exe (where Tauri
/// installs externalBins, target-triple stripped) for the installed app, or the
/// staged binary under the crate dir for `tauri dev`. Returns `None` if not
/// found, in which case the engine falls back to a system/winget playit.
fn resolve_playit() -> Option<PathBuf> {
    let mut candidates: Vec<PathBuf> = Vec::new();
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            candidates.push(dir.join("playit.exe"));
        }
    }
    candidates.push(PathBuf::from(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/binaries/playit-x86_64-pc-windows-msvc.exe"
    )));
    candidates.into_iter().find(|p| p.is_file())
}

/// Resolve the bundled frpc client for the built-in tunnel, mirroring
/// resolve_playit: next to the app exe (installed app) or the staged binary
/// under the crate dir (`tauri dev`). None falls back to a system/PATH frpc.
fn resolve_frpc() -> Option<PathBuf> {
    let mut candidates: Vec<PathBuf> = Vec::new();
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            candidates.push(dir.join("frpc.exe"));
        }
    }
    candidates.push(PathBuf::from(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/binaries/frpc-x86_64-pc-windows-msvc.exe"
    )));
    candidates.into_iter().find(|p| p.is_file())
}

fn main() {
    tauri::Builder::default()
        // Single-instance must be the first plugin registered. A second launch
        // focuses the existing window instead of starting a second engine.
        .plugin(tauri_plugin_single_instance::init(|app, _argv, _cwd| {
            if let Some(win) = app.get_webview_window("main") {
                let _ = win.unminimize();
                let _ = win.set_focus();
            }
        }))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .manage(EngineProcess(Mutex::new(None)))
        .setup(|app| {
            let templates_dir = resolve_templates_dir(app);

            let mut sidecar = app
                .shell()
                .sidecar("engine")
                .expect("engine sidecar binary is not bundled");
            if let Some(dir) = &templates_dir {
                sidecar = sidecar.env("GAMEHOST_TEMPLATES", dir.to_string_lossy().to_string());
            }
            // Tell the engine to exit if we (its parent) die, so it never orphans
            // and leaves engine.exe locked / port 8723 held. It watches its stdin
            // pipe, which the OS closes when this process exits.
            sidecar = sidecar.env("GAMEHOST_PARENT_WATCH", "1");
            // Point the engine at the bundled playit agent so the relay needs no
            // separate winget install. The engine runs it only while hosting.
            if let Some(playit) = resolve_playit() {
                sidecar = sidecar.env("GAMEHOST_PLAYIT", playit.to_string_lossy().to_string());
            }
            // Point the engine at the bundled frpc for the built-in tunnel. This
            // only locates the binary; the tunnel itself stays dormant until the
            // engine is given a control-plane URL (GAMEHOST_TUNNEL_URL).
            if let Some(frpc) = resolve_frpc() {
                sidecar = sidecar.env("GAMEHOST_FRPC", frpc.to_string_lossy().to_string());
            }
            // GAMEHOST_DATA is left unset: the engine defaults to
            // %APPDATA%\gamehost\data, which is correct for a desktop install.

            let (mut rx, child) = sidecar.spawn().expect("failed to start the engine");
            app.state::<EngineProcess>()
                .0
                .lock()
                .unwrap()
                .replace(child);

            // Drain the engine's output so its pipe never blocks; mirror it to
            // our own stdout/stderr for debugging during dev.
            tauri::async_runtime::spawn(async move {
                while let Some(event) = rx.recv().await {
                    match event {
                        CommandEvent::Stdout(bytes) => {
                            print!("[engine] {}", String::from_utf8_lossy(&bytes));
                        }
                        CommandEvent::Stderr(bytes) => {
                            eprint!("[engine] {}", String::from_utf8_lossy(&bytes));
                        }
                        CommandEvent::Terminated(payload) => {
                            eprintln!("[engine] exited: {:?}", payload.code);
                            break;
                        }
                        _ => {}
                    }
                }
            });

            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building GameHost")
        .run(|app_handle, event| {
            // Kill the engine sidecar when the app shuts down so it never
            // lingers as an orphan process.
            if let RunEvent::Exit = event {
                if let Some(state) = app_handle.try_state::<EngineProcess>() {
                    if let Some(child) = state.0.lock().unwrap().take() {
                        let _ = child.kill();
                    }
                }
            }
        });
}
