# GameHost (working title)

Self-host game servers — **Minecraft (Java & Bedrock), Valheim, CS2, Rust and
more** — with a clean UI and *zero* self-hosting knowledge required.

GameHost runs each game server in its own Docker container, driven by a small
Go **engine** daemon and a React **control panel**. It ships first as a desktop
app; the same engine later runs headless on a home server or VPS — no rewrite.

> Status: **M0** — scaffold. The panel boots and talks to the engine; creating
> real servers lands in M1. See [`docs/architecture.md`](docs/architecture.md).

## Repo layout
```
engine/      Go daemon — Docker control, REST + WebSocket API
ui/          React + Vite + Tailwind control panel
templates/   Game definitions (one YAML per game)
docs/        Design docs
```

## Quick start (dev)

**Prereqs:** Go 1.22+, Node 20+. Docker Desktop is only needed once you actually
run a game server (M1+) — the panel runs fine without it and shows a setup step.

### 1. Engine
```bash
cd engine
go mod tidy        # fetch deps (first run only)
go run ./cmd/engine
# → engine listening on http://127.0.0.1:8723
```
> Just installed Go? Open a fresh terminal so `go` is on your PATH.

### 2. UI
```bash
cd ui
npm install        # first run only
npm run dev
# → http://localhost:5173
```

Open the UI. With no Docker yet you'll see the **setup banner** — that's the
expected non-technical-user path; server creation arrives in M1.

## Enabling Docker (to run game servers)
Windows (run an **Administrator** PowerShell):
```powershell
wsl --install                                  # reboot if prompted
winget install -e --id Docker.DockerDesktop
```
Launch Docker Desktop once after install, then restart the engine.

## Roadmap
- [x] **M0** Scaffold; UI ↔ engine; Docker probe + setup surface
- [ ] **M1** Create/start/stop/delete a Paper server; live console
- [ ] **M2** Bedrock + Valheim + a SteamCMD game; resource limits
- [ ] **M3** Files, backups, schedules, settings
- [ ] **M4** Networking: UPnP auto-forward + relay fallback
- [ ] **M5** Tauri desktop packaging; headless server build
