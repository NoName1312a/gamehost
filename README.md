# GameNest

**Host game servers for you and your friends — free, open source, and dead simple.**

GameNest is a Windows desktop app that runs **Minecraft (Java & Bedrock), Valheim,
CS2, Rust and more** in one click. It solves the part everyone gets stuck on —
*"my friends can't connect"* — with automatic **UPnP port-forwarding** and a
**relay fallback** for tricky routers/CGNAT, so you get a shareable address without
touching your router settings.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
![Platform: Windows](https://img.shields.io/badge/platform-Windows-0078D6)
![Free & open source](https://img.shields.io/badge/free%20%26%20open%20source-yes-brightgreen)

> **No paywalls.** Unlimited servers, scheduled backups & restarts, off-site
> backups, and the mod manager are **all free**. A managed **hosted** version (so you
> don't have to keep your PC on) is coming later — self-hosting stays free forever.

<!-- TODO: add a 10-second demo GIF here — the "create a server → friends connect" magic moment -->

## What it is

GameNest runs each game server in its own **Docker** container, driven by a small
Go **engine** daemon and a React **control panel**, wrapped in a native Tauri
desktop window that auto-launches the engine. No terminal, no config files, no
self-hosting knowledge required.

- **One-click servers** — pick a game, name it, go. ~28 game templates.
- **Friends can actually connect** — automatic UPnP + playit.gg relay fallback, with
  a copy-paste connect address.
- **Backups & schedules** — daily restart/backup times and off-site copies to a NAS,
  external drive, or synced cloud folder.
- **Minecraft mod manager** — paste [Modrinth](https://modrinth.com) slugs; mods
  install automatically on next start.
- **Runs locally & private** — the engine binds to `127.0.0.1`; your data stays on
  your machine.

## Install

Download the latest one-click installer from the
[**Releases**](https://github.com/NoName1312a/gamehost/releases) page and run it.
The app guides you through a one-time Docker setup on first launch.

> The installer auto-updates from the public update feed on launch and from
> **Settings → Check for updates**.

## Build from source

**Prereqs:** Go 1.22+, Node 20+. Docker Desktop is only needed once you actually run
a game server.

```bash
# Engine — REST + WebSocket API on http://127.0.0.1:8723
go -C engine run ./cmd/engine

# UI — http://localhost:5173
npm --prefix ui install   # first run only
npm --prefix ui run dev
```

Desktop app (needs [Rust](https://rustup.rs) + VS 2022 C++ Build Tools):

```powershell
# Dev — hot-reload window + engine sidecar:
powershell -ExecutionPolicy Bypass -File scripts\desktop-dev.ps1
# Build the installer:
powershell -ExecutionPolicy Bypass -File scripts\desktop-build.ps1
```

See [`docs/architecture.md`](docs/architecture.md) for the design.

## Repo layout

```
engine/      Go daemon — Docker control, REST + WebSocket API
ui/          React + Vite + Tailwind control panel
desktop/     Tauri v2 shell (Windows) — bundles + launches the engine
templates/   Game definitions (one YAML per game)
ee/          Reserved for future commercial features (separate license; empty today)
docs/        Design docs
```

## Community & contributing

Contributions, bug reports, and new game templates are welcome — see
[`CONTRIBUTING.md`](CONTRIBUTING.md). Questions and help: **join the Discord**
(link coming with the public launch). Found a security issue? See
[`SECURITY.md`](SECURITY.md).

## License

GameNest is **open core**:

- The application is licensed under the **GNU AGPL v3.0** — see [`LICENSE`](LICENSE).
  You can self-host, modify, and share it freely.
- The [`ee/`](ee/) directory is reserved for future **commercial** features (the
  hosted/cloud and team tiers) under a separate license — it's empty today, and the
  desktop app is, and will remain, fully free and open source.

*GameNest is a trademark of its authors; the AGPL/commercial licenses do not grant
trademark rights.*
