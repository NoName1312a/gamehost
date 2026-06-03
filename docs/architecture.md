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
┌─ Desktop app (Tauri shell, later) ───────────────────────┐
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

### Desktop shell (`desktop/`, Tauri — added at M5)
- Thin Rust wrapper that bundles the Engine binary + UI assets, launches the
  Engine on a loopback port, and points a native webview at it.
- Tauri over Electron: far smaller installers, native webview.

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
   answer. The engine abstracts the runtime so we can later bundle a minimal
   engine instead of requiring Docker Desktop.
2. **Networking** — port-forwarding is the #1 blocker for "my friends can't
   connect." Plan: automatic UPnP port-mapping + a built-in relay/tunnel
   fallback so it works with zero router config.

## Roadmap
- **M0** Monorepo scaffold; UI ↔ Engine; Docker probe + setup-wizard surface. ← *here*
- **M1** Create/start/stop/delete a Paper server; live console + command input.
- **M2** Template registry → Bedrock + Valheim + one SteamCMD game; resource limits.
- **M3** File manager, backups, scheduled tasks, settings editor.
- **M4** Networking: UPnP auto-forward + relay fallback.
- **M5** Polish, Tauri packaging/installer, headless server-mode build.

## API (M0)
| Method | Path                   | Purpose                          |
|--------|------------------------|----------------------------------|
| GET    | `/api/health`          | Engine liveness + version        |
| GET    | `/api/system/runtime`  | Docker connectivity (drives wizard) |
| GET    | `/api/templates`       | List game templates              |
| GET    | `/api/templates/{id}`  | Single template                  |
