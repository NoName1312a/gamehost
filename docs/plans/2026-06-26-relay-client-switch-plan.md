# Desktop Client Switch (Stage B) — playit.gg → GameNest Tunnel — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the built-in GameNest tunnel the **seamless automatic fallback** for "friends can join" (direct-first; tunnel only when direct fails), and **remove playit.gg entirely** from the engine, installer, and UI.

**Architecture:** The tunnel is already fully built (engine `internal/tunnel`, UI `TunnelShare`) and the `frpc` sidecar is **already bundled** (`tauri.conf.json` `externalBin`, staged by `scripts/build-desktop.ps1`, located by the engine). So this is: (1) flip the engine's tunnel from dormant → **default-on** by baking in the relay control-plane URL (with a `GAMEHOST_TUNNEL_DISABLE` safety valve); (2) delete the playit integration (engine package + routes + manager wiring + installer staging + Rust shell); (3) rework the UI's `ConnectionPanel` to resolve a single best address — direct if reachable, else auto-enable the tunnel — and delete the playit `RelaySetup`. Held on `feat/relay-client-switch` until the relay (Stage A) is green-lit; it ships in the release cut after that, so playit-removal and tunnel-activation go out together (no fallback gap).

**Tech Stack:** Go engine · Tauri v2 (Rust shell) · React 19 + Tailwind v4 UI · frp (`frpc` sidecar) · NSIS installer.

## Global Constraints

- **Spec:** `docs/specs/2026-06-26-relay-client-switch-design.md`. Stage B only (client switch). **Out of scope:** GameNest Plus / vanity *activation* (Stage C — the vanity UI stays carried but dormant), anything relay-side (Stage A).
- **Sharing model:** direct-first; the tunnel engages **automatically** when direct isn't reachable — no user toggle for the fallback. Free tier stays **anonymous** (`gn-…` slugs).
- **Remove playit entirely** (engine package, `/api/system/relay*` routes, manager `Relay` interface + `syncRelay`/`anyRelayServerRunning`, the installer staging, the Rust `resolve_playit`/`GAMEHOST_PLAYIT`, the UI `RelaySetup`). **Keep** `Server.RelayAddress` (Go) + `ServerSummary.relayAddress` (TS) as **deprecated** fields so existing installs don't lose data — just stop writing/displaying them.
- **No engine/API protocol break** beyond removing the playit routes (the tunnel/connectivity endpoints are unchanged).
- **`frpc` is already bundled** — do NOT add bundling; just don't break it.
- **Verification:** Engine = `go test ./...` (must pass) + `go build ./...`. Rust shell = `cargo check` in `desktop/`. UI = `npm --prefix ui run build` + `npm --prefix ui run lint` (0 NEW errors; **7 known pre-existing `react-hooks/set-state-in-effect`**) — no UI test runner; owner does visual on a release. Match the codebase's Go table-test style.
- Commits **DCO signed-off** (`git commit -s`); free **AGPL** core (nothing under `ee/`). Branch `feat/relay-client-switch` (already checked out; **no new branch**).
- **Held until release:** this branch is built now but goes live only in the release cut after the relay is green-lit. The `GAMEHOST_TUNNEL_DISABLE` valve is the emergency off.

---

### Task 1: Engine — bake in the tunnel URL default + `GAMEHOST_TUNNEL_DISABLE`

Flip the tunnel from dormant (no default URL) to **default-on**, gated by a disable valve. Use a small testable helper so the resolution logic is unit-tested (`main.go` itself isn't easily testable).

**Files:**
- Modify: `engine/cmd/engine/main.go` (the `GAMEHOST_TUNNEL_URL` gate ~`:50-58`)
- Create: `engine/cmd/engine/tunnelurl.go` + `engine/cmd/engine/tunnelurl_test.go` (the helper + test)

**Interfaces:**
- Produces: `func resolveTunnelURL(getenv func(string) string) string` — returns `""` (dormant) iff disabled; the env override if set; else the baked default.

- [ ] **Step 1: Write the failing test** — `engine/cmd/engine/tunnelurl_test.go`:
```go
package main

import "testing"

func TestResolveTunnelURL(t *testing.T) {
	const def = defaultTunnelURL
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default when unset", map[string]string{}, def},
		{"env override", map[string]string{"GAMEHOST_TUNNEL_URL": "https://example.test"}, "https://example.test"},
		{"disabled valve", map[string]string{"GAMEHOST_TUNNEL_DISABLE": "1"}, ""},
		{"disabled beats override", map[string]string{"GAMEHOST_TUNNEL_DISABLE": "1", "GAMEHOST_TUNNEL_URL": "https://x"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			get := func(k string) string { return c.env[k] }
			if got := resolveTunnelURL(get); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it — expect FAIL**
```bash
cd engine && go test ./cmd/engine/ -run TestResolveTunnelURL
```
Expected: FAIL (undefined `resolveTunnelURL`/`defaultTunnelURL`).

- [ ] **Step 3: Implement** — `engine/cmd/engine/tunnelurl.go`:
```go
package main

// defaultTunnelURL is the GameNest relay control-plane, baked in so the tunnel
// is on by default. (Owner: confirm the exact production URL at release.)
const defaultTunnelURL = "https://cp.coderaum.com"

// resolveTunnelURL decides the control-plane URL the tunnel agent uses.
// GAMEHOST_TUNNEL_DISABLE=<non-empty> forces the tunnel off (returns ""), the
// emergency valve. Otherwise GAMEHOST_TUNNEL_URL overrides the baked default.
func resolveTunnelURL(getenv func(string) string) string {
	if getenv("GAMEHOST_TUNNEL_DISABLE") != "" {
		return ""
	}
	if u := getenv("GAMEHOST_TUNNEL_URL"); u != "" {
		return u
	}
	return defaultTunnelURL
}
```

- [ ] **Step 4: Wire it in `main.go`** — replace the gate (~`:50-58`):
```go
var tunAgent *tunnel.Agent
if url := os.Getenv("GAMEHOST_TUNNEL_URL"); url != "" {
	tunAgent = tunnel.New(cfg.DataDir, url)
}
```
with:
```go
var tunAgent *tunnel.Agent
if url := resolveTunnelURL(os.Getenv); url != "" {
	tunAgent = tunnel.New(cfg.DataDir, url)
}
```

- [ ] **Step 5: Run tests + build — expect PASS**
```bash
cd engine && go test ./... && go build ./...
```
Expected: PASS; build clean. (`api.tunnel()` will now report `configured: true` in a default build.)

- [ ] **Step 6: Commit**
```bash
git add engine/cmd/engine/tunnelurl.go engine/cmd/engine/tunnelurl_test.go engine/cmd/engine/main.go
git commit -s -m "feat(engine): tunnel on by default (baked relay URL) + GAMEHOST_TUNNEL_DISABLE valve"
```

---

### Task 2: Engine — remove the playit integration

Delete the playit package + API + routes + manager wiring + its tests. Keep `Server.RelayAddress` as a deprecated persisted field (don't lose existing users' data). After this the engine compiles with no playit references.

**Files:**
- Delete: `engine/internal/relay/` (whole package: `relay.go`, `relay_test.go`, `relay_secret_test.go`, any others)
- Delete: `engine/internal/api/relay.go`
- Modify: `engine/internal/api/router.go` (remove 4 routes, the `relay` import, the `Relay` field on `Deps` + `API`, the assignment)
- Modify: `engine/cmd/engine/main.go` (remove `relay.New`, the import, the `Relay:` dep, `relayAgent.Stop()`, pass `nil`/drop the manager arg)
- Modify: `engine/internal/server/manager.go` (remove the `Relay` interface, the `relay` field, the `rel Relay` param, `syncRelay()`, `anyRelayServerRunning()`, and the `m.syncRelay()` call sites; keep `Server.RelayAddress` field + drop `SetRelayAddress` or make it a no-op — see steps)
- Modify: `engine/internal/server/manager_test.go` (delete `TestRelayRunsOnlyWhileHosting`; update `NewManager` calls to the new signature)

**Interfaces:**
- Produces: `server.NewManager` loses its `rel Relay` parameter — new signature `NewManager(dataDir string, rt Runtime, net Networking, reg *templates.Registry) *Manager`. The `api.Deps`/`api.API` lose their `Relay`/`relay` field.

- [ ] **Step 1: Delete the playit package + API file**
```bash
git rm -r engine/internal/relay
git rm engine/internal/api/relay.go
```

- [ ] **Step 2: Remove relay from the router** (`engine/internal/api/router.go`)
Remove the `relay` import (~`:22`); the four route registrations:
```go
r.Get("/system/relay", a.relayStatus)
r.Post("/system/relay/link", a.relayLink)
r.Post("/system/relay/{action}", a.relayAction)
r.Put("/servers/{id}/relay-address", a.setRelayAddress)
```
the `Relay *relay.Agent` field from `Deps` (~`:42`) and the `relay *relay.Agent` field from `API` (~`:61`); and the `relay: d.Relay` assignment in `NewRouter`/`&API{...}` (~`:75`).

- [ ] **Step 3: Remove relay from `manager.go`**
Remove: the `Relay` interface (~`:131-135`); the `relay Relay` field (~`:194`); the `rel Relay` parameter from `NewManager` (~`:216`) + the `relay: rel` init (~`:224`); the `syncRelay()` + `anyRelayServerRunning()` methods (~`:927-959`); and every `m.syncRelay()` call site (~`:401`, `:800`, and in `Start`/`Stop`/`Delete`). **Keep** the `Server.RelayAddress` struct field (`json:"relayAddress,omitempty"`) — mark it `// deprecated: playit removed; retained so existing servers.json isn't lost`. Remove the `SetRelayAddress` manager method (its only caller was the deleted API handler).

- [ ] **Step 4: Remove relay from `main.go`**
Remove the `relay` import (~`:27`), `relayAgent := relay.New(cfg.DataDir)` (~`:50`), `Relay: relayAgent,` in `api.Deps` (~`:109`), and `relayAgent.Stop()` (~`:169`). Update the `server.NewManager(...)` call (~`:76`) to drop the relay arg: `server.NewManager(cfg.DataDir, rt, netMapper, reg)`.

- [ ] **Step 5: Fix the tests** (`engine/internal/server/manager_test.go`)
Delete `TestRelayRunsOnlyWhileHosting` and the `fakeRelay` type/uses. Update every `NewManager(...)` call in the test to the new 4-arg signature (drop the `rel`/`fakeRelay{}` argument). Leave all tunnel tests (`TestTunnelSharesRunningServersOnly`, `TestTunnelSlugIsStable`, `TestVanitySlugEntitlementFetched`, etc.) intact.

- [ ] **Step 6: Verify**
```bash
cd engine && go build ./... && go test ./...
```
Expected: build clean (no undefined `relay`/`syncRelay`/`Relay` references); all remaining tests pass. `grep -rn "relay\." engine/ --include=*.go | grep -iv relayaddress` should return nothing (only the deprecated `RelayAddress` field remains).

- [ ] **Step 7: Commit**
```bash
git add -A engine/
git commit -s -m "refactor(engine): remove the playit.gg integration (tunnel is the fallback now)"
```

---

### Task 3: Installer + Rust shell — remove playit from the bundle

Stop shipping the playit binary and stop the Rust shell from resolving/injecting it. (`frpc` staging stays untouched.)

**Files:**
- Modify: `desktop/tauri.conf.json` (drop `"binaries/playit"` from `externalBin`)
- Modify: `scripts/build-desktop.ps1` (delete the playit download/stage block ~`:56-70`)
- Modify: `desktop/src/main.rs` (delete `resolve_playit()` ~`:44-56` + the `GAMEHOST_PLAYIT` injection ~`:104-106`)

**Interfaces:** none (build config + shell).

- [ ] **Step 1: Drop playit from `externalBin`** (`desktop/tauri.conf.json`)
Change `"externalBin": ["binaries/engine", "binaries/playit", "binaries/frpc"]` → `"externalBin": ["binaries/engine", "binaries/frpc"]`.

- [ ] **Step 2: Remove playit staging** (`scripts/build-desktop.ps1`)
Delete the playit block (the `$playitVer`/`$playitUrl` download + the `Copy-Item … playit-$triple.exe` ~`:56-70`). Leave the `frpc` staging block intact.

- [ ] **Step 3: Remove playit from the Rust shell** (`desktop/src/main.rs`)
Delete the `resolve_playit()` fn (~`:44-56`) and the block that sets `GAMEHOST_PLAYIT` (~`:104-106`). Leave `resolve_frpc()` + the `GAMEHOST_FRPC` injection.

- [ ] **Step 4: Verify the shell compiles**
```bash
cd desktop && cargo check
```
Expected: clean (no reference to the removed `resolve_playit`). Confirm `tauri.conf.json` is valid JSON (`node -e "JSON.parse(require('fs').readFileSync('desktop/tauri.conf.json'))"` or a parse check). (A full `tauri build` is the owner's release step.)

- [ ] **Step 5: Commit**
```bash
git add desktop/tauri.conf.json scripts/build-desktop.ps1 desktop/src/main.rs
git commit -s -m "build(desktop): stop bundling playit (tunnel-only fallback)"
```

---

### Task 4: UI — `ConnectionPanel` auto-fallback + delete `RelaySetup`

Rework the share UI to resolve **one** best address: direct if reachable, else **auto-enable the tunnel**. Delete the playit `RelaySetup`. Keep the vanity (Plus) control, dormant.

**Files:**
- Modify: `ui/src/components/ServerDetail.tsx` (`ConnectionPanel` rework; delete `RelaySetup` + `relayPill`; slim `TunnelShare` → a `VanityControl`; drop the `relay` prop on `ConnectionPanel` + `ServerDetail`)
- Modify: `ui/src/App.tsx` (drop the `relay={…}` prop passed to `<ServerDetail>`)

**Interfaces:**
- Produces: `ConnectionPanel` signature loses `relay` → `{ s, tunnel, account, onChanged }`. `ServerDetail` props lose `relay`. (`App.tsx`'s `relay` `useAsync` is removed in Task 5.)

- [ ] **Step 1: Delete `RelaySetup` + `relayPill`; slim `TunnelShare` to `VanityControl`**
In `ServerDetail.tsx`: delete the `RelaySetup` component (~`:89-193`), the `relayPill` constant (~`:71-74`), and the `Relay` type import (~`:9`). Replace `TunnelShare` (~`:199-286`) with a focused vanity-only control (the enable/stop/address now live in `ConnectionPanel`):
```tsx
// Plus-only: a reserved vanity address. Dormant until GameNest Plus (Stage C);
// shown only when an account is configured + linked.
function VanityControl({ s, account, onChanged }: { s: ServerSummary; account?: AccountStatus; onChanged: () => void }) {
  const [vanityName, setVanityName] = useState("");
  const [vanityBusy, setVanityBusy] = useState(false);
  const [vanityError, setVanityError] = useState<string | null>(null);
  const plusLinked = account?.configured && account?.linked;
  if (!plusLinked) return null;
  async function applyVanity() {
    setVanityBusy(true);
    setVanityError(null);
    try {
      await api.setVanity(s.id, vanityName.trim());
      onChanged();
    } catch (e) {
      setVanityError(friendlyError(e));
    } finally {
      setVanityBusy(false);
    }
  }
  return (
    <div className="mt-2">
      <label className="text-[11px] uppercase tracking-wide text-zinc-500">Reserved address (Plus)</label>
      <div className="mt-1 flex items-center gap-2">
        <input
          value={vanityName}
          onChange={(e) => setVanityName(e.target.value)}
          placeholder={s.tunnelSlug ?? "your-name"}
          className="min-w-0 flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 font-mono text-xs text-zinc-100 outline-none focus:border-emerald-500"
        />
        <button onClick={applyVanity} disabled={vanityBusy || !vanityName.trim()} className="shrink-0 rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50">
          {vanityBusy ? "…" : "Set"}
        </button>
      </div>
      {vanityError && <p className="mt-1 text-xs text-rose-400">{vanityError}</p>}
    </div>
  );
}
```

- [ ] **Step 2: Rework `ConnectionPanel`** — replace the component (~`:293-470`) with the auto-fallback version. Keep the connectivity probe + auto-test; add an `autoTunneled` ref + an effect that auto-enables the tunnel when direct has failed; resolve a single primary address:
```tsx
function ConnectionPanel({
  s, tunnel, account, onChanged,
}: {
  s: ServerSummary;
  tunnel?: TunnelStatus;
  account?: AccountStatus;
  onChanged: () => void;
}) {
  const [conn, setConn] = useState<Connectivity | null>(null);
  const [testing, setTesting] = useState(false);
  const [test, setTest] = useState<Reachable | null>(null);
  const autoTested = useRef(false);
  const autoTunneled = useRef(false);

  // Load connectivity while running; reset auto-guards when it stops.
  useEffect(() => {
    if (!s.running) {
      setConn(null); setTest(null);
      autoTested.current = false; autoTunneled.current = false;
      return;
    }
    let alive = true;
    api.connectivity(s.id).then((c) => alive && setConn(c)).catch(() => {});
    return () => { alive = false; };
  }, [s.id, s.running]);

  async function runTest() {
    setTesting(true);
    try {
      const r = await api.testConnectivity(s.id);
      setTest(r);
    } catch (e) {
      setTest({ open: false, checked: false, detail: friendlyError(e) });
    } finally {
      setTesting(false);
    }
  }

  // Auto-run the reachability test once we have a forwarded TCP external address.
  useEffect(() => {
    if (!conn || !conn.running || autoTested.current) return;
    if (conn.forwarded && /tcp/i.test(conn.protocol) && conn.externalAddress) {
      autoTested.current = true;
      runTest();
    }
  }, [conn]);

  const directAddr = conn?.externalAddress ?? s.externalAddress;
  const forwarded = conn?.forwarded ?? s.shared;
  const reachableConfirmed = !!(test && test.checked && test.open);
  const testedNotOpen = !!(test && test.checked && !test.open);
  // Direct works: we have an address AND (it's UPnP-forwarded and not proven-closed, or a test confirmed it).
  const directOK = !!directAddr && (reachableConfirmed || (forwarded && !testedNotOpen));
  // Direct has been ruled out: connectivity loaded, not direct-OK, and either no UPnP forward or a test came back closed.
  const directFailed = !!conn && !directOK && (testedNotOpen || !forwarded);

  // Auto-fallback: when direct fails and the tunnel is available, turn it on (once).
  useEffect(() => {
    if (autoTunneled.current) return;
    if (!s.running || !tunnel?.configured || s.useTunnel) return;
    if (!directFailed) return;
    autoTunneled.current = true;
    api.setUseTunnel(s.id, true).then(onChanged).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [s.running, s.id, s.useTunnel, tunnel?.configured, directFailed]);

  if (!s.running) {
    return <p className="text-sm text-zinc-500">Start the server to get a connection address.</p>;
  }

  return (
    <div className="space-y-3">
      {directOK ? (
        <CopyRow label="Friends connect to" addr={directAddr!} pill={reachableConfirmed ? reachablePill : forwardedPill} />
      ) : s.tunnelAddress ? (
        <CopyRow label="Friends connect to" addr={s.tunnelAddress} pill={tunnelPill} />
      ) : tunnel?.configured ? (
        <p className="text-sm text-zinc-400">Setting up a public address through GameNest — your friends will be able to join in a moment…</p>
      ) : directAddr ? (
        <CopyRow label="Friends connect to (unverified)" addr={directAddr} pill={forwardedPill} />
      ) : (
        <p className="text-sm text-zinc-400">Working out how friends can reach you…</p>
      )}

      {/* Verify-direct affordance (kept for the forwarded-but-untested case) */}
      {directAddr && !reachableConfirmed && (
        <button onClick={runTest} disabled={testing} className="rounded-lg border border-zinc-700 px-3 py-1 text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-50">
          {testing ? "Checking…" : "Test connection"}
        </button>
      )}
      {test && test.checked && !test.open && (
        <p className="text-xs text-amber-400/80">Direct connection isn't reachable — sharing through GameNest instead.</p>
      )}

      {/* Plus vanity, dormant until Stage C */}
      {s.useTunnel && <VanityControl s={s} account={account} onChanged={onChanged} />}
    </div>
  );
}
```

- [ ] **Step 3: Drop the `relay` prop on `ServerDetail` + the mount + App**
In `ServerDetail.tsx`: remove `relay,` from the destructure (~`:773`) and `relay?: Relay;` from the props type (~`:784`); change the mount (~`:906`) to `<ConnectionPanel s={server} tunnel={tunnel} account={account} onChanged={onChanged} />`.
In `ui/src/App.tsx`: remove the `relay={relay.status === "ok" ? relay.data : undefined}` prop from the `<ServerDetail …>` call (~`:290`). (The `relay` `useAsync` itself is removed in Task 5.)

- [ ] **Step 4: Verify**
```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new errors). tsc will flag any leftover `RelaySetup`/`relay`-prop reference — there should be none in `ServerDetail.tsx`/`App.tsx` (App's `relay` `useAsync` stays until Task 5 but is now unused — note: an unused `const relay = useAsync(...)` will trip `@typescript-eslint/no-unused-vars`, so **App's `relay` useAsync must be removed in this task too** — fold Task 5's App line in here: delete `const relay = useAsync<Relay>(api.relay, nonce + tick);` ~`:113` and the `Relay` import if now unused).

- [ ] **Step 5: Manual check (owner, at release)**
**Acceptance:** a running server with working UPnP shows a **direct** address (reachable/auto-forwarded pill); a server whose UPnP fails **automatically** shows a **via GameNest** address (no button, no playit). There is no "playit" anywhere in the UI. Plus vanity input shows only when an account is linked.

- [ ] **Step 6: Commit**
```bash
git add ui/src/components/ServerDetail.tsx ui/src/App.tsx
git commit -s -m "feat(ui): direct-first auto tunnel fallback; remove playit RelaySetup"
```

---

### Task 5: UI — remove the dead playit API + the `relayAddress` display legs

Final cleanup: delete the now-unused playit `api` methods and the `relayAddress` fallback legs in onboarding + the checklist. Keep the `ServerSummary.relayAddress` **type** field (deprecated) so the engine's retained data still type-checks.

**Files:**
- Modify: `ui/src/lib/api.ts` (delete `relay`/`relayAction`/`relayLink`/`setRelayAddress` methods + the `Relay` type; keep `relayAddress?` on `ServerSummary`, marked deprecated)
- Modify: `ui/src/components/Onboarding.tsx` (drop the `relayAddress` leg from the live-step address chain)
- Modify: `ui/src/components/GetStartedChecklist.tsx` (drop `relayAddress` from `hasShareAddress`)

**Interfaces:** Consumes nothing new. After this, `api.relay*` and the `Relay` type no longer exist client-side.

- [ ] **Step 1: Remove the playit `api` methods + `Relay` type** (`ui/src/lib/api.ts`)
Delete the four methods (~`:308-312`: `relay`, `relayAction`, `relayLink`, `setRelayAddress`) and the `Relay` interface (~`:53-60`). On `ServerSummary`, keep `relayAddress?: string;` but update its comment to `// deprecated (playit removed); retained so existing servers keep the value`.

- [ ] **Step 2: Drop `relayAddress` from onboarding** (`ui/src/components/Onboarding.tsx`)
Change the three occurrences (~`:124`, `:129`, `:133`) from `liveServer.tunnelAddress || liveServer.externalAddress || liveServer.relayAddress` to `liveServer.tunnelAddress || liveServer.externalAddress` (drop the `|| …relayAddress`), including the `|| ""` copy fallback.

- [ ] **Step 3: Drop `relayAddress` from the checklist** (`ui/src/components/GetStartedChecklist.tsx`)
In `hasShareAddress` (~`:5`), change `Boolean(s.tunnelAddress || s.externalAddress || s.relayAddress)` → `Boolean(s.tunnelAddress || s.externalAddress)`.

- [ ] **Step 4: Verify**
```bash
npm --prefix ui run build && npm --prefix ui run lint
```
Expected: build + lint pass (0 new). `grep -rn "api.relay\|RelaySetup\|\.relayAddress" ui/src` returns only the deprecated `ServerSummary.relayAddress` type field (no usages).

- [ ] **Step 5: Commit**
```bash
git add ui/src/lib/api.ts ui/src/components/Onboarding.tsx ui/src/components/GetStartedChecklist.tsx
git commit -s -m "chore(ui): remove dead playit api + relayAddress display legs"
```

---

## Definition of Done

- `cd engine && go test ./... && go build ./...` green; no playit references remain (`grep` clean except the deprecated `RelayAddress` field).
- `cd desktop && cargo check` clean; `tauri.conf.json` valid; `externalBin` = engine + frpc only.
- `npm --prefix ui run build && npm --prefix ui run lint` clean (0 new); no `RelaySetup`/`api.relay*`/playit anywhere.
- **Conceptual walk:** default build → `api.tunnel()` reports `configured: true`; a running server resolves a single best address (direct if reachable, else auto-tunnel); no playit UI, no playit binary, no playit routes; `GAMEHOST_TUNNEL_DISABLE=1` returns the engine to dormant.
- **Held for release:** branch stays until the relay (Stage A) is green-lit; then the owner bumps the version + cuts the release (playit-removal + tunnel-on ship together).

Then invoke **superpowers:finishing-a-development-branch**.

## Self-Review

**Spec coverage:** auto-fallback sharing flow → Task 4 (the `directOK`/`directFailed` resolution + auto-enable effect). Remove playit (engine) → Task 2. Remove playit (bundle/shell) → Task 3. Bake URL + disable valve → Task 1. UI playit removal + `relayAddress` migration (keep field, drop display) → Tasks 4–5. Vanity stays dormant → Task 4 (`VanityControl`, plus-gated). `frpc` already bundled → untouched (noted). Out-of-scope (Plus activation, relay-side) → respected. ✓

**Placeholder scan:** the baked `defaultTunnelURL` value is an explicit owner-confirm constant (enumerated in the spec's open items), not a TODO. The `~:NN` line numbers are "locate by content" anchors against the current files (verified by the research that wrote this plan). Every code step shows complete code. No TBD.

**Type consistency:** `resolveTunnelURL(getenv)`/`defaultTunnelURL` (Task 1) used in `main.go`. `NewManager` 4-arg signature (Task 2) matches the `main.go` call + the test updates. `ConnectionPanel{ s, tunnel, account, onChanged }` (Task 4, no `relay`) matches its mount; `ServerDetail` drops `relay`; `App` drops the `relay` prop + (Task 4 Step 4) the `relay` `useAsync`. `VanityControl{ s, account, onChanged }` (Task 4) rendered in `ConnectionPanel`. `api.relay*` + `Relay` removed (Task 5) after their last UI use is gone (Task 4) — order is correct. `ServerSummary.relayAddress` kept as a deprecated type field so the engine's retained data still type-checks. ✓

**Ordering note:** Task 4 must remove App's `relay` `useAsync` (not just the prop) or lint's no-unused-vars trips — folded into Task 4 Step 4. Task 5 then only removes the `api`-layer methods + the `relayAddress` display legs, which have no remaining consumers.
