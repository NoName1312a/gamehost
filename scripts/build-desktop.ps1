# Stages the GameHost engine (as a Tauri sidecar) and game templates for the
# desktop shell. Run automatically by desktop-dev.ps1 / desktop-build.ps1 before
# invoking the Tauri CLI.
#
#   powershell -ExecutionPolicy Bypass -File scripts\build-desktop.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# Tauri requires the sidecar to be named with the host target triple suffix.
$triple = "x86_64-pc-windows-msvc"

# Optional code-signing for the bundled native binaries. Unsigned exes — frpc
# especially — get flagged by Windows Defender / SmartScreen on end-user machines,
# so a real release MUST sign them (same cert as the installer). Enable by setting
# EITHER GAMEHOST_SIGN_CMD (a command template; "{f}" is replaced with the quoted
# file path — e.g. an Azure Trusted Signing signtool invocation) OR
# GAMEHOST_SIGN_THUMBPRINT (a code-signing cert in the local store, used via
# signtool; timestamp URL overridable with GAMEHOST_SIGN_TS). No-op when neither
# is set, so dev builds stay unsigned.
function Invoke-Sign($path) {
    if ($env:GAMEHOST_SIGN_CMD) {
        $cmd = $env:GAMEHOST_SIGN_CMD.Replace("{f}", '"' + $path + '"')
        Write-Host "Signing (custom): $path" -ForegroundColor Cyan
        Invoke-Expression $cmd
        if ($LASTEXITCODE -ne 0) { throw "GAMEHOST_SIGN_CMD failed for $path" }
    } elseif ($env:GAMEHOST_SIGN_THUMBPRINT) {
        $signtool = Get-Command signtool.exe -ErrorAction SilentlyContinue
        if (-not $signtool) { throw "GAMEHOST_SIGN_THUMBPRINT set but signtool.exe not on PATH (install the Windows SDK)" }
        $ts = if ($env:GAMEHOST_SIGN_TS) { $env:GAMEHOST_SIGN_TS } else { "http://timestamp.digicert.com" }
        Write-Host "Signing (thumbprint): $path" -ForegroundColor Cyan
        & $signtool.Source sign /sha1 $env:GAMEHOST_SIGN_THUMBPRINT /fd SHA256 /tr $ts /td SHA256 $path
        if ($LASTEXITCODE -ne 0) { throw "signtool failed for $path" }
    }
    # else: unset -> leave unsigned (dev build).
}

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
Invoke-Sign (Join-Path $binDir "engine-$triple.exe")
Write-Host "Staged sidecar: desktop\binaries\engine-$triple.exe" -ForegroundColor Green

# Bundle the frp client (frpc) as a sidecar for the built-in GameNest tunnel
# (the "Share with friends" feature). The engine runs it on demand via the
# GAMEHOST_FRPC path the desktop shell passes in. We COMPILE frpc from source
# (pinned to $frpVer) rather than shipping frp's prebuilt release binary, because
# the prebuilt frpc.exe is widely flagged as PUA/malware by Windows Defender — a
# from-source build has a hash that isn't in AV signature databases. (frp's go.mod
# uses `replace` directives, so `go install pkg@version` is refused; we build from
# a shallow git checkout instead.) Sign it too (Invoke-Sign) for the durable fix.
$cacheDir = Join-Path $binDir ".cache"
New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
$frpVer = "0.69.1"
$stagedFrpc = Join-Path $binDir "frpc-$triple.exe"
$frpSrc = Join-Path $cacheDir "frp-src-$frpVer"
if (-not (Test-Path (Join-Path $frpSrc "go.mod"))) {
    Write-Host "Fetching frp v$frpVer source..." -ForegroundColor Cyan
    if (Test-Path $frpSrc) { Remove-Item -Recurse -Force $frpSrc }
    & git clone --depth 1 --branch "v$frpVer" https://github.com/fatedier/frp "$frpSrc"
    if ($LASTEXITCODE -ne 0) { throw "git clone frp v$frpVer failed" }
}
# frpc/frps embed web/*/dist (their admin dashboards) via go:embed; those dirs are
# produced by frp's frontend build and absent from a source checkout. We run frpc
# headless (no dashboard), so drop a placeholder file so the embed pattern resolves.
foreach ($d in @("web\frpc\dist", "web\frps\dist")) {
    $dist = Join-Path $frpSrc $d
    New-Item -ItemType Directory -Force -Path $dist | Out-Null
    $idx = Join-Path $dist "index.html"
    if (-not (Test-Path $idx)) { Set-Content -Path $idx -Value "<!-- dashboard disabled in GameNest build -->" -Encoding utf8 }
}
Write-Host "Compiling frpc v$frpVer from source..." -ForegroundColor Cyan
$frpcBuildDir = Join-Path $cacheDir "frpc-build"
New-Item -ItemType Directory -Force -Path $frpcBuildDir | Out-Null
$builtFrpc = Join-Path $frpcBuildDir "frpc.exe"
Push-Location $frpSrc
try {
    & go build -trimpath -o "$builtFrpc" ./cmd/frpc
    if ($LASTEXITCODE -ne 0) { throw "go build frpc failed" }
} finally {
    Pop-Location
}
Copy-Item $builtFrpc $stagedFrpc -Force
Invoke-Sign $stagedFrpc
Write-Host "Staged sidecar (compiled from source): desktop\binaries\frpc-$triple.exe" -ForegroundColor Green

$tplDir = Join-Path $root "desktop\resources\templates"
New-Item -ItemType Directory -Force -Path $tplDir | Out-Null
Copy-Item "$root\templates\*.yaml" $tplDir -Force
$count = (Get-ChildItem $tplDir -Filter *.yaml).Count
Write-Host "Staged $count game templates." -ForegroundColor Green
