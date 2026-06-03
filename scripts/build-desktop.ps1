# Stages the GameHost engine (as a Tauri sidecar) and game templates for the
# desktop shell. Run automatically by desktop-dev.ps1 / desktop-build.ps1 before
# invoking the Tauri CLI.
#
#   powershell -ExecutionPolicy Bypass -File scripts\build-desktop.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# Tauri requires the sidecar to be named with the host target triple suffix.
$triple = "x86_64-pc-windows-msvc"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $env:Path += ";C:\Program Files\Go\bin"
}

Write-Host "Building engine..." -ForegroundColor Cyan
Push-Location "$root\engine"
try {
    go build -o bin\engine.exe .\cmd\engine
} finally {
    Pop-Location
}

$binDir = Join-Path $root "desktop\binaries"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
Copy-Item "$root\engine\bin\engine.exe" (Join-Path $binDir "engine-$triple.exe") -Force
Write-Host "Staged sidecar: desktop\binaries\engine-$triple.exe" -ForegroundColor Green

$tplDir = Join-Path $root "desktop\resources\templates"
New-Item -ItemType Directory -Force -Path $tplDir | Out-Null
Copy-Item "$root\templates\*.yaml" $tplDir -Force
$count = (Get-ChildItem $tplDir -Filter *.yaml).Count
Write-Host "Staged $count game templates." -ForegroundColor Green
