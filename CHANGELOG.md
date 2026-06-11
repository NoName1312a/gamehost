# Changelog

All notable changes to GameNest are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/).

`scripts/release.ps1` reads the section matching the version you publish and uses
it as the in-app update notes **and** the GitHub release body. Before releasing,
rename `[Unreleased]` to the version you're shipping (e.g. `## [0.4.0] - 2026-06-10`).

## [Unreleased]

### Changed
- **GameNest is now free and open source (AGPLv3).** Every feature is unlocked for everyone — there is no longer a paid "Pro" tier. Unlimited concurrent servers, scheduled backups & restarts, off-site backups, and the Minecraft mod manager are now free for all.
- Renamed the app from "GameHost" to **GameNest** (the engine, repo, and updater feed keep the internal `gamehost` name for now).

### Removed
- The Pro paywall and all license-key feature gating. (A managed hosted version and optional supporter keys are planned; the desktop app stays free forever. The Settings "Plan" section is now "Supporter / Hosted".)

### Added
- Opt-in, off-by-default **diagnostics**: anonymous crash and basic usage reports, with a Settings toggle. No personal data is collected, and nothing is sent unless you opt in.
- Settings **"Danger zone -> Remove all servers"** to delete every server, world, and Docker volume in one step.
- The **uninstaller now offers** (opt-in) to remove all game data (Docker containers, volumes, and the data folder) so a clean uninstall leaves nothing behind.
- Open-source project files: `LICENSE` (AGPL-3.0), `CONTRIBUTING.md`, `SECURITY.md`, `NOTICE`, and an `ee/` directory reserved for future commercial features.

## [0.3.0] - 2026-06-07

### Added
- Cover-art game library with grouped cards, a modernized dashboard, and first-run image-download progress.
- File manager, manual + scheduled backups, daily restart schedules, and 25+ game templates.
- Operator accounts with roles, remote (LAN/HTTPS) access, off-site backups, the Minecraft mod manager, and offline Pro licensing.
