# GameHost — Architecture

## Goal
Make self-hosting game servers (Minecraft Java/Bedrock, Valheim, CS2, Rust, …)
approachable for people with **zero** self-hosting knowledge. Beat Crafty
Controller / Pterodactyl / AMP on **ease of install** and **UX**, while matching
their game breadth.

## The two-component split
The product is split into a headless **Engine** and a **UI**. This is the single
most important decision: it lets the *same* engine power both the desktop app
(now) and a home-server / VPS deployment (later) with no rewrite.

```
┌─ Desktop app (Tauri shell — Windows) ────────────────────┐
│                                                            │
│   UI (React)  ──HTTP + WebSocket──►  Engine (Go daemon)    │
│      ▲                                    │                │
│      └── shell launches + supervises ─────┘                │
│                                           ▼                │
│                                  Container runtime (Docker) │
│                                    ├ minecraft-1            │
│                                    ├ valheim-1              │
│                                    └ cs2-1                  │
└────────────────────────────────────────────────────────────┘
```

### Engine (`engine/`, Go)
- Talks to the container runtime (Docker SDK).
- Manages game-server lifecycle: create / start / stop / delete.
- Streams server consoles over WebSocket; relays user commands to stdin.
- Loads the **game template registry**.
- Will own: files, backups, schedules, networking (UPnP + relay), auth.
- Distributes as a single static binary → trivial to drop on a VPS.

### UI (`ui/`, React + Vite + Tailwind + shadcn/ui)
- Pure client of the Engine's API. No game logic lives here.
- Same bundle served by the desktop shell and by the headless server.

### Desktop shell (`desktop/`, Tauri v2 — **Windows, implemented**)
- Thin Rust wrapper that bundles the Engine binary as a **sidecar** plus the UI
  assets, spawns the Engine on loopback (`127.0.0.1:8723`), and points a native
  WebView2 window at it. Kills the Engine on exit; single-instance launch.
- **Windows-only by design.** Linux/Mac users are served by the headless build
  (the *same* Engine + UI over the web), so nothing here is platform-locked.
- Tauri over Electron: far smaller installers (NSIS `.exe`), native webview.
- Build: `scripts/desktop-build.ps1` → `desktop/target/release/bundle/nsis/`.
  The bundled templates are staged into `desktop/resources/`; the engine
  sidecar into `desktop/binaries/` (both git-ignored, regenerated each build).

## Why Docker-per-game
One container per server gives isolation, per-server CPU/RAM limits, clean
version switching, and — crucially — makes **adding a new game a data change,
not a code change**.

## Game template registry
Each game is one YAML file in `templates/`. A template maps a game onto a
container image plus the user-facing variables shown in the create-server form.
Three runtime patterns cover most games:

| Runtime    | Covers                                  | Example image                 |
|------------|-----------------------------------------|-------------------------------|
| `java`     | Minecraft Java (Vanilla/Paper/Fabric…)  | `itzg/minecraft-server`       |
| `steamcmd` | Valheim, CS2, Rust, ARK, Palworld, …    | `lloesche/valheim-server`, …  |
| `custom`   | Anything bespoke (e.g. Bedrock)         | `itzg/minecraft-bedrock-server` |

## The two UX battlegrounds (our differentiators)
1. **Install** — non-technical users fail at install. The desktop app + a
   guided **setup wizard** (detect → enable WSL2 → install/manage Docker) is the
   answer. **Implemented:** the engine exposes `GET /api/system/setup` (per-step
   status) and `POST /api/system/setup/{step}` (one-click fixes that trigger a
   UAC-elevated `wsl --install` / `winget install Docker.DockerDesktop`, or start
   Docker); the UI renders these as a stepwise wizard. Windows-only fix actions
   are guarded by `runtime.GOOS`, so the UI stays a pure engine client. The engine
   abstracts the runtime so we can later bundle a minimal engine instead of
   requiring Docker Desktop.
2. **Networking** — port-forwarding is the #1 blocker for "my friends can't
   connect." Plan: automatic UPnP port-mapping + a built-in relay/tunnel
   fallback so it works with zero router config.

## Roadmap
- **M0** Monorepo scaffold; UI ↔ Engine; Docker probe + setup-wizard surface. ← *here*
- **M1** Create/start/stop/delete a Paper server; live console + command input.
- **M2** Template registry → Bedrock + Valheim + one SteamCMD game; resource limits.
- **M3** File manager, backups, scheduled tasks, settings editor.
- **M4** Networking: UPnP auto-forward + relay fallback.
- **M5** Tauri desktop packaging/installer (Windows) ✅; polish + headless server-mode build still open.

## API (M0)
| Method | Path                   | Purpose                          |
|--------|------------------------|----------------------------------|
| GET    | `/api/health`          | Engine liveness + version        |
| GET    | `/api/system/runtime`  | Docker connectivity (drives banner) |
| GET    | `/api/system/setup`    | Setup-wizard prerequisite steps  |
| POST   | `/api/system/setup/{step}` | Run a one-click setup fix (Windows) |
| GET    | `/api/templates`       | List game templates              |
| GET    | `/api/templates/{id}`  | Single template                  |
