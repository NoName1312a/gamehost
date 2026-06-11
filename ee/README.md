# ee/ — GameNest commercial features

This directory is reserved for GameNest's **commercial** features and is **not**
covered by the repository's AGPL-3.0 license. Its contents are licensed under the
**GameNest Commercial License** (see [`LICENSE`](./LICENSE)).

## Why this exists

GameNest is **open core**:

- **The desktop application is, and will remain, fully free and open source**
  (AGPL-3.0). Everything you need to self-host game servers — unlimited servers,
  scheduled backups & restarts, off-site backups, the mod manager — lives in the
  open-source tree and is free for everyone, forever.
- **Future paid offerings** — a managed **hosted/cloud** version and **team/scale**
  features (multi-node fleets, SSO/RBAC, priority support) — will live here, under a
  separate commercial license, so the open-source core stays clean of copyleft
  contamination from proprietary code.

## Current status

**Empty.** There are no commercial features today; the desktop app is 100% free.
This directory establishes the boundary up front so the open-source core and any
future commercial code never get tangled. See the licensing rationale in
[`../CONTRIBUTING.md`](../CONTRIBUTING.md).
