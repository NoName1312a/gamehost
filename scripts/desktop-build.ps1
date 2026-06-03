# Builds the GameHost Windows installer (NSIS .exe) via Tauri.
#
#   powershell -ExecutionPolicy Bypass -File scripts\desktop-build.ps1
#
# Output: desktop\target\release\bundle\nsis\GameHost_<version>_x64-setup.exe

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

$env:Path += ";C:\Program Files\Go\bin;$env:USERPROFILE\.cargo\bin"

& "$PSScriptRoot\build-desktop.ps1"

$tauri = Join-Path $root "ui\node_modules\.bin\tauri.cmd"
if (-not (Test-Path $tauri)) {
    throw "Tauri CLI not found. Run: npm --prefix `"$root\ui`" install"
}

Push-Location "$root\desktop"
try {
    & $tauri build
} finally {
    Pop-Location
}

Write-Host "Installer written to desktop\target\release\bundle\nsis\" -ForegroundColor Green
