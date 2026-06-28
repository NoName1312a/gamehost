# GameHost uninstall cleanup — invoked (opt-in) by the NSIS uninstaller's
# pre-uninstall hook. Removes the app's Docker containers + volumes and its data
# directory. Best-effort: every step is guarded so a failure never blocks the
# uninstall.
$ErrorActionPreference = "SilentlyContinue"

# Remove all gamehost-* containers (running or stopped).
$containers = docker ps -aq --filter "name=gamehost-"
if ($containers) { docker rm -f $containers | Out-Null }

# Remove all gamehost-* volumes (saved worlds + per-server data).
$volumes = docker volume ls -q --filter "name=gamehost-"
if ($volumes) { docker volume rm $volumes | Out-Null }

# Remove the engine data directory (servers.json, auth, license, settings).
$dataDir = Join-Path $env:APPDATA "gamehost"
if (Test-Path $dataDir) { Remove-Item -Recurse -Force $dataDir }
