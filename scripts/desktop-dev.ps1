# Launches the GameHost desktop app in dev mode: stages the engine sidecar,
# then runs `tauri dev` (which starts the Vite dev server and the native window).
#
#   powershell -ExecutionPolicy Bypass -File scripts\desktop-dev.ps1
#
# Requires Go, Node, and the Rust MSVC toolchain installed.

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\.cargo\bin"

# Lets the engine trust the Vite dev server's origin. Released builds do not,
# so this has to be opt-in — see allowedOrigins() in engine/internal/config.
$env:GAMEHOST_DEV = "1"

& "$PSScriptRoot\build-desktop.ps1"

$tauri = Join-Path $root "ui\node_modules\.bin\tauri.cmd"
if (-not (Test-Path $tauri)) {
    throw "Tauri CLI not found. Run: npm --prefix `"$root\ui`" install"
}

Push-Location "$root\desktop"
try {
    & $tauri dev
} finally {
    Pop-Location
}
