# Changelog

All notable changes to GameHost are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/).

`scripts/release.ps1` reads the section matching the version you publish and uses
it as the in-app update notes **and** the GitHub release body. Before releasing,
rename `[Unreleased]` to the version you're shipping (e.g. `## [0.4.0] - 2026-06-10`).

## [Unreleased]

### Added
- Opt-in, off-by-default **diagnostics**: anonymous crash and basic usage reports, with a Settings toggle. No personal data is collected, and nothing is sent unless you opt in.
- Settings **"Danger zone -> Remove all servers"** to delete every server, world, and Docker volume in one step.
- The **uninstaller now offers** (opt-in) to remove all game data (Docker containers, volumes, and the data folder) so a clean uninstall leaves nothing behind.

## [0.3.0] - 2026-06-07

### Added
- Cover-art game library with grouped cards, a modernized dashboard, and first-run image-download progress.
- File manager, manual + scheduled backups, daily restart schedules, and 25+ game templates.
- Operator accounts with roles, remote (LAN/HTTPS) access, off-site backups, the Minecraft mod manager, and offline Pro licensing.
