# Launches the GameHost engine and UI in two terminal windows for local dev.
#
#   powershell -ExecutionPolicy Bypass -File scripts\dev.ps1
#
# Requires Go and Node on PATH (open a fresh terminal after installing Go).
# The UI will be at http://localhost:5173; the engine at http://127.0.0.1:8723.

$root = Split-Path -Parent $PSScriptRoot

Write-Host "Starting GameHost engine..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList @(
  "-NoExit", "-Command", "Set-Location '$root\engine'; go run ./cmd/engine"
)

Write-Host "Starting GameHost UI (http://localhost:5173)..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList @(
  "-NoExit", "-Command", "Set-Location '$root\ui'; npm run dev"
)

Write-Host "Launched. Close the two new windows (Ctrl+C) to stop." -ForegroundColor Green
