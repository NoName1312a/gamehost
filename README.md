# GameHost (working title)

Self-host game servers — **Minecraft (Java & Bedrock), Valheim, CS2, Rust and
more** — with a clean UI and *zero* self-hosting knowledge required.

GameHost runs each game server in its own Docker container, driven by a small
Go **engine** daemon and a React **control panel**. It ships first as a desktop
app; the same engine later runs headless on a home server or VPS — no rewrite.

> Status: **M1 + Windows desktop app** — create/start/stop/delete servers with a
> live console, now packaged as a one-click Windows app (Tauri) that auto-launches
> the engine. Lifecycle is driven via the Docker CLI for now (the Docker Go SDK is
> mid module-split); the runtime is abstracted so the SDK can drop in later.
> See [`docs/architecture.md`](docs/architecture.md).

## Repo layout
```
engine/      Go daemon — Docker control, REST + WebSocket API
ui/          React + Vite + Tailwind control panel
desktop/     Tauri v2 shell (Windows) — bundles + launches the engine
templates/   Game definitions (one YAML per game)
docs/        Design docs
```

## Quick start (dev)

**Prereqs:** Go 1.22+, Node 20+. Docker Desktop is only needed once you actually
run a game server — the panel runs fine without it and shows a setup step.

**One command (Windows):** `powershell -ExecutionPolicy Bypass -File scripts\dev.ps1`
opens the engine and UI in two windows. Or run them manually:

### 1. Engine
```bash
cd engine
go mod tidy        # fetch deps (first run only)
go run ./cmd/engine
# → engine listening on http://127.0.0.1:8723
```
> Just installed Go? Open a fresh terminal so `go` is on your PATH.

### 2. UI
```bash
cd ui
npm install        # first run only
npm run dev
# → http://localhost:5173
```

Open the UI. With no Docker yet you'll see the **setup banner** — that's the
expected non-technical-user path; server creation arrives in M1.

## Desktop app (Windows)
The desktop shell wraps the UI in a native window and **auto-launches the engine**
as a bundled sidecar — no terminal, no separate engine/UI startup.

**One-time prereqs:** [Rust](https://rustup.rs) (MSVC toolchain), the
**VS 2022 C++ Build Tools** (`winget install -e --id Microsoft.VisualStudio.2022.BuildTools
--override "--add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"`),
plus Go and Node. WebView2 ships with Windows 10/11.

```powershell
# Dev — hot-reload window + engine sidecar:
powershell -ExecutionPolicy Bypass -File scripts\desktop-dev.ps1

# Build the installer:
powershell -ExecutionPolicy Bypass -File scripts\desktop-build.ps1
# → desktop\target\release\bundle\nsis\GameHost_<version>_x64-setup.exe
```

> Docker is still required to actually run game servers (below). A guided
> in-app setup wizard for that is the next milestone.

## Enabling Docker (to run game servers)
**In the desktop app this is one-click:** when Docker isn't ready, GameHost shows
a guided **setup wizard** that detects what's missing (WSL2 → Docker Desktop →
start Docker) and fixes each step for you via a Windows prompt — no terminal.

Prefer to do it by hand? Windows, in an **Administrator** PowerShell:
```powershell
wsl --install                                  # reboot if prompted
winget install -e --id Docker.DockerDesktop
```
Launch Docker Desktop once after install, then restart the engine.

## Roadmap
- [x] **M0** Scaffold; UI ↔ engine; Docker probe + setup surface
- [x] **M1** Create/start/stop/delete servers; live WebSocket console; per-server CPU/RAM limits
- [ ] **M2** Expand the game library (more templates); first-run image-pull progress
- [ ] **M3** Files, backups, schedules, settings
- [ ] **M4** Networking: UPnP auto-forward + relay fallback
- [~] **M4** Networking: automatic **UPnP** port-forward + sharable connect address ✅; relay fallback still open
- [~] **M5** Tauri desktop packaging (Windows ✅); headless server build still open
