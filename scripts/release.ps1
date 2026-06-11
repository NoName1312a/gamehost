# Builds a SIGNED GameHost desktop release and publishes it to the public
# auto-update feed (NoName1312a/gamehost-releases): uploads the NSIS installer
# and a latest.json manifest that the in-app updater reads.
#
#   powershell -ExecutionPolicy Bypass -File scripts\release.ps1 -Version 0.1.1
#
# The version MUST match desktop\tauri.conf.json. Requires the updater signing
# key at %USERPROFILE%\.tauri\gamehost-updater.key and gh authenticated.

param([Parameter(Mandatory = $true)][string]$Version)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$repo = "NoName1312a/gamehost-releases"
$tag = "v$Version"
$gh = "C:\Program Files\GitHub CLI\gh.exe"

# --- locate the updater signing key (key has no password) ---
$keyPath = Join-Path $env:USERPROFILE ".tauri\gamehost-updater.key"
if (-not (Test-Path $keyPath)) { throw "updater signing key not found at $keyPath" }

# --- build the installer ---
& "$PSScriptRoot\desktop-build.ps1"

$nsisDir = Join-Path $root "desktop\target\release\bundle\nsis"
$setup = Get-ChildItem $nsisDir -Filter "*-setup.exe" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $setup) { throw "no NSIS installer found in $nsisDir" }

# --- sign the installer explicitly (do NOT rely on build-time auto-signing:
# the TAURI_SIGNING_PRIVATE_KEY env var does not reliably reach the `tauri
# build` child, so createUpdaterArtifacts silently produces no .sig). Signing
# the final installer bytes here is what the in-app updater verifies.
#
# The signing key is encrypted with an EMPTY password, so tauri only decrypts
# when handed an explicit empty `-p ""` argument. PowerShell 5.1 silently drops
# a bare `""` arg (shifting FILE into --password) and treats an empty *env* var
# as unset (-> "wrong password"). The stop-parsing token `--%` passes the rest
# of the line to the native command verbatim — preserving the literal `""` —
# while still expanding cmd-style %VAR% references for the paths. ---
$tauri = Join-Path $root "ui\node_modules\.bin\tauri.cmd"
if (-not (Test-Path $tauri)) { throw "Tauri CLI not found at $tauri" }
[Environment]::SetEnvironmentVariable("TAURI_SIGNING_PRIVATE_KEY_PASSWORD", $null, "Process")
[Environment]::SetEnvironmentVariable("TAURI_SIGNING_PRIVATE_KEY", $null, "Process")
$env:GH_SIGN_KEY = $keyPath
$env:GH_SIGN_FILE = $setup.FullName
& $tauri --% signer sign -f "%GH_SIGN_KEY%" -p "" "%GH_SIGN_FILE%"
if ($LASTEXITCODE -ne 0) { throw "tauri signer sign failed (exit $LASTEXITCODE)" }
$sigPath = "$($setup.FullName).sig"
if (-not (Test-Path $sigPath)) { throw "signature not found after signing: $sigPath" }

# --- release notes: pull this version's section out of CHANGELOG.md, so the
# in-app updater dialog and the GitHub release body show real notes. Falls back
# to a generic line if the version has no section yet. ---
$notes = "GameNest $Version"
$changelogPath = Join-Path $root "CHANGELOG.md"
if (Test-Path $changelogPath) {
  $lines = Get-Content $changelogPath -Encoding UTF8
  $start = -1
  for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match "^##\s*\[$([regex]::Escape($Version))\]") { $start = $i + 1; break }
  }
  if ($start -ge 0) {
    $collected = @()
    for ($j = $start; $j -lt $lines.Count; $j++) {
      if ($lines[$j] -match "^##\s*\[") { break }
      $collected += $lines[$j]
    }
    $body = ($collected -join "`n").Trim()
    if ($body) { $notes = $body }
  } else {
    Write-Warning "CHANGELOG.md has no section for [$Version]; using a generic note."
  }
}

# --- assemble latest.json (the updater manifest) ---
$assetUrl = "https://github.com/$repo/releases/download/$tag/$($setup.Name)"
$manifest = [ordered]@{
  version   = $Version
  notes     = $notes
  pub_date  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  platforms = [ordered]@{
    "windows-x86_64" = [ordered]@{
      signature = (Get-Content $sigPath -Raw).Trim()
      url       = $assetUrl
    }
  }
}
$latestPath = Join-Path $nsisDir "latest.json"
# Write UTF-8 WITHOUT a BOM: the Tauri updater parses this with serde_json,
# which rejects a leading BOM (Set-Content -Encoding utf8 on PS 5.1 adds one).
$json = $manifest | ConvertTo-Json -Depth 6
[System.IO.File]::WriteAllText($latestPath, $json, (New-Object System.Text.UTF8Encoding $false))

# --- publish (installer + manifest) as the latest release ---
& $gh release create $tag $setup.FullName $latestPath --repo $repo --title "GameNest $Version" --notes $notes
Write-Host "Published $tag to $repo" -ForegroundColor Green
