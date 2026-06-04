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

# Bundle the playit relay agent (headless, signed CLI) as a sidecar so users
# don't need a separate winget install / tray app. Cached by version. The engine
# runs it on demand (only while a relay-shared server is hosting) via the
# GAMEHOST_PLAYIT path the desktop shell passes in.
$playitVer = "1.0.6"
$playitUrl = "https://github.com/playit-cloud/playit-agent/releases/download/v$playitVer/playit-windows-x86_64-signed.exe"
$cacheDir = Join-Path $binDir ".cache"
New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
$playitCache = Join-Path $cacheDir "playit-$playitVer.exe"
if (-not (Test-Path $playitCache)) {
    Write-Host "Downloading playit agent v$playitVer..." -ForegroundColor Cyan
    Invoke-WebRequest -Uri $playitUrl -OutFile $playitCache -UseBasicParsing
}
Copy-Item $playitCache (Join-Path $binDir "playit-$triple.exe") -Force
Write-Host "Staged sidecar: desktop\binaries\playit-$triple.exe" -ForegroundColor Green

$tplDir = Join-Path $root "desktop\resources\templates"
New-Item -ItemType Directory -Force -Path $tplDir | Out-Null
Copy-Item "$root\templates\*.yaml" $tplDir -Force
$count = (Get-ChildItem $tplDir -Filter *.yaml).Count
Write-Host "Staged $count game templates." -ForegroundColor Green
