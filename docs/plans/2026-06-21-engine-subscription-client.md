# GameNest Engine — Hosted-Subscription Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the desktop engine speak the new subscription protocol: keep free tunnels in the relay's `gn-` namespace, let a user link their GameNest account by pasting a one-time code, and — for a linked subscriber with a reserved vanity name — fetch a short-lived entitlement token from the platform and attach it to the relay allocation so the tunnel comes up on the user's stable vanity address.

**Architecture:** A new `engine/internal/account` package owns a small HTTP client to the platform (`GAMENEST_PLATFORM_URL`, dormant when empty) plus a DPAPI-protected device credential at `data/account-token` (mirrors the existing `tunnel-token`). The tunnel `Client.Allocate` gains an optional `entitlement` field. The server `Manager` gains an `Account` collaborator: when a server's tunnel slug is a vanity name (not `gn-…`) and the account is linked, the manager fetches an entitlement for that slug and passes it through to the relay. The UI's existing "Supporter" settings section is repurposed into "GameNest Plus / account linking."

**Tech Stack:** Go 1.x (engine) · chi router · `net/http/httptest` for tests · React 19 + Vite + TypeScript (UI). Relay + platform are the other two repos.

## Global Constraints

- **Free-tunnel slug namespace = `gn-…`.** The merged relay (`gamenest-relay`) only accepts tokenless allocations whose slug matches `^gn-[a-z0-9]{4,50}$`. The engine's current `genSlug()` (`engine/internal/server/manager.go:358`) emits `"gn"+hex` (no hyphen) and MUST change to `"gn-"+hex`. Vanity (paid) slugs are user-chosen names that must NOT start with `gn-`.
- **Entitlement token is opaque to the engine.** The engine fetches it from the platform's `POST /api/entitlement` (Bearer device token, body `{ "slug": "<vanity>" }` → `{ "token": "...", "exp": <unix> }`) and forwards it verbatim to the relay's `/v1/allocate` as `entitlement`. The engine never signs or parses it.
- **Dormant by default.** With `GAMENEST_PLATFORM_URL` empty, no account client is constructed, the account endpoints report not-configured, and behavior is unchanged. Mirrors the existing `GAMEHOST_TUNNEL_URL` gating (`engine/cmd/engine/main.go:54-57`).
- **Account credential storage** mirrors the tunnel token: DPAPI via the existing `secret` package, `gh1:` prefix, file `<dataDir>/account-token`.
- **Test-first** (per `CONTRIBUTING.md`): add/update Go tests with every behavior change; keep `go -C engine test ./...` and `go -C engine vet ./...` green. The UI has **no test runner** — verify with `npm --prefix ui run build` (tsc -b + vite) and `npm --prefix ui run lint`.
- **Commits must be signed off:** `git commit -s -m "…"` (DCO/CLA). Conventional Commits. This is **free AGPL core — do NOT put any of this under `ee/`** (the subscription is a hosted service; the client is free core). Work on branch `feat/subscription-client` (not `main`).
- Path/build: engine module under `engine/`; run Go commands with `go -C engine …`.

## Account dependency

All Go tasks (1–5) are verifiable now with `httptest` mocks of the platform — no live accounts needed. The UI (Task 6) is build/lint-verified. True end-to-end (engine → live platform → relay granting a real vanity tunnel) needs the platform deployed with the user's Supabase/Lemon Squeezy + the relay switched on with the platform's `GAMENEST_ENTITLEMENT_PUBKEY`; that is the go-live step, out of this plan's automated scope.

---

### Task 1: Free-slug namespace fix (`gn-` prefix)

**Files:**
- Modify: `engine/internal/server/manager.go:358-362` (`genSlug`)
- Test: `engine/internal/server/manager_test.go`

**Interfaces:**
- Produces: `genSlug()` returns a slug matching `^gn-[a-z0-9]{4,50}$` (relay free namespace).

- [ ] **Step 1: Write the failing test**

Add to `engine/internal/server/manager_test.go`:

```go
func TestGenSlugMatchesRelayFreeNamespace(t *testing.T) {
	re := regexp.MustCompile(`^gn-[a-z0-9]{4,50}$`)
	for i := 0; i < 100; i++ {
		s := genSlug()
		if !re.MatchString(s) {
			t.Fatalf("genSlug()=%q does not match the relay free namespace ^gn-[a-z0-9]{4,50}$", s)
		}
	}
}
```

Ensure `regexp` is imported in the test file.

- [ ] **Step 2: Run to verify it fails**

Run: `go -C engine test ./internal/server/ -run TestGenSlugMatchesRelayFreeNamespace`
Expected: FAIL — current `genSlug` returns `"gn"+hex` (no hyphen), which does not match.

- [ ] **Step 3: Implement**

Change `genSlug` in `engine/internal/server/manager.go`:

```go
func genSlug() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "gn-" + hex.EncodeToString(b[:]) // e.g. "gn-3f9a2b1c4d5e"; relay free namespace ^gn-[a-z0-9]{4,50}$
}
```

- [ ] **Step 4: Run to verify it passes + full suite**

Run: `go -C engine test ./internal/server/ && go -C engine vet ./...`
Expected: PASS; vet clean. (Existing tests that assert on slugs may need the `gn-` form — update any that hardcode `gn<hex>`.)

- [ ] **Step 5: Commit**

```bash
git add engine/internal/server/manager.go engine/internal/server/manager_test.go
git commit -s -m "fix(tunnel): emit gn- free slugs to match the relay namespace"
```

---

### Task 2: `Client.Allocate` carries an optional entitlement

**Files:**
- Modify: `engine/internal/tunnel/client.go:147-172` (`Allocate`)
- Test: `engine/internal/tunnel/client_test.go` (add or extend; use `httptest`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `func (c *Client) Allocate(ctx context.Context, slug string, ports []PortReq, entitlement string) (Allocation, error)` — when `entitlement != ""`, the request body includes `"entitlement": <token>`; when empty, the body is unchanged from today.

- [ ] **Step 1: Write the failing test**

Add to `engine/internal/tunnel/client_test.go` a test that stands up an `httptest` server capturing the `/v1/allocate` body and asserts the `entitlement` field is present only when supplied. (Follow the existing client-test setup in this file for `register`+`allocate`; if none exists, model the server on the relay's response shape: `{"slug":...,"secret":"s","frps":{"addr":"a","token":"t"},"proxies":[{"name":"n","role":"game","proto":"tcp","remotePort":40000,"address":"slug.gn.coderaum.com:40000"}]}`.)

```go
func TestAllocateIncludesEntitlementWhenSet(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/register":
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceId": "d", "token": "tok"})
		case "/v1/allocate":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"slug": gotBody["slug"], "secret": "s",
				"frps":    map[string]string{"addr": "a", "token": "t"},
				"proxies": []any{},
			})
		}
	}))
	defer srv.Close()

	c := NewClient(t.TempDir(), srv.URL) // adjust to the actual constructor name/signature
	if _, err := c.Allocate(context.Background(), "alice", []PortReq{{Role: "game", Proto: "tcp"}}, "ent.tok"); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if gotBody["entitlement"] != "ent.tok" {
		t.Fatalf("entitlement not forwarded: %v", gotBody["entitlement"])
	}

	gotBody = nil
	if _, err := c.Allocate(context.Background(), "gn-abcd12", []PortReq{{Role: "game", Proto: "tcp"}}, ""); err != nil {
		t.Fatalf("free allocate: %v", err)
	}
	if _, present := gotBody["entitlement"]; present {
		t.Fatalf("entitlement must be absent for the free path, got %v", gotBody["entitlement"])
	}
}
```

(Use the real `Client` constructor + `PortReq` from `client.go`. If the constructor is unexported or differs, adapt — the assertion is what matters.)

- [ ] **Step 2: Run to verify it fails**

Run: `go -C engine test ./internal/tunnel/ -run TestAllocateIncludesEntitlement`
Expected: FAIL — compile error (`Allocate` has 3 params) or `entitlement` absent.

- [ ] **Step 3: Implement**

In `engine/internal/tunnel/client.go`, change `Allocate` to add the `entitlement string` parameter and include it in the request body only when non-empty:

```go
func (c *Client) Allocate(ctx context.Context, slug string, ports []PortReq, entitlement string) (Allocation, error) {
	// ... existing device-token ensure ...
	reqBody := map[string]any{"slug": slug, "ports": ports}
	if entitlement != "" {
		reqBody["entitlement"] = entitlement
	}
	// ... existing POST /v1/allocate + decode ...
}
```

Update the existing caller in `engine/internal/tunnel/tunnel.go` (the `Reconcile` Allocate call, ~`tunnel.go:93`) to pass an entitlement. For now, thread it from `Desired`: add `Entitlement string` to the `Desired` struct (`tunnel.go:21-25`) and pass `d.Entitlement`. Free desireds leave it `""`.

- [ ] **Step 4: Run to verify it passes + suite**

Run: `go -C engine test ./internal/tunnel/ && go -C engine vet ./...`
Expected: PASS; vet clean.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/tunnel/
git commit -s -m "feat(tunnel): forward optional entitlement on allocate"
```

---

### Task 3: `account` package — platform client + linked credential

**Files:**
- Create: `engine/internal/account/account.go`
- Test: `engine/internal/account/account_test.go`

**Interfaces:**
- Consumes: the existing `secret` package (DPAPI Protect/Unprotect) — find its import path from `engine/internal/tunnel/client.go`'s token storage and reuse it.
- Produces:
  - `func New(dataDir, platformURL string) *Store`
  - `func (s *Store) Link(ctx context.Context, code string) error` — `POST {platformURL}/api/link` body `{ "code": code }` → `{ "deviceToken": "..." }`; store it DPAPI-protected at `<dataDir>/account-token`.
  - `func (s *Store) Unlink() error` — delete the token file.
  - `func (s *Store) Linked() bool` — a credential is stored.
  - `func (s *Store) Entitlement(ctx context.Context, slug string) (string, error)` — `POST {platformURL}/api/entitlement` with `Authorization: Bearer <deviceToken>` and body `{ "slug": slug }` → returns `token` from `{ "token": "...", "exp": <int> }`.

- [ ] **Step 1: Write the failing test**

Create `engine/internal/account/account_test.go` using `httptest` to mock the platform. Cover: `Link` stores a credential (`Linked()` true after); `Entitlement` sends the bearer + slug and returns the token; `Unlink` clears it. Run on Windows so DPAPI works.

```go
func TestLinkThenEntitlement(t *testing.T) {
	var gotAuth, gotSlug string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/link":
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["code"] != "CODE123" {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceToken": "dev-tok"})
		case "/api/entitlement":
			gotAuth = r.Header.Get("Authorization")
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			gotSlug = b["slug"]
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "ent.tok", "exp": 123})
		}
	}))
	defer srv.Close()

	s := New(t.TempDir(), srv.URL)
	if s.Linked() {
		t.Fatal("fresh store must not be linked")
	}
	if err := s.Link(context.Background(), "CODE123"); err != nil {
		t.Fatalf("link: %v", err)
	}
	if !s.Linked() {
		t.Fatal("should be linked after Link")
	}
	tok, err := s.Entitlement(context.Background(), "alice")
	if err != nil {
		t.Fatalf("entitlement: %v", err)
	}
	if tok != "ent.tok" || gotSlug != "alice" || gotAuth != "Bearer dev-tok" {
		t.Fatalf("bad entitlement flow: tok=%q slug=%q auth=%q", tok, gotSlug, gotAuth)
	}
	if err := s.Unlink(); err != nil || s.Linked() {
		t.Fatalf("unlink failed: err=%v linked=%v", err, s.Linked())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go -C engine test ./internal/account/`
Expected: FAIL — package/functions undefined.

- [ ] **Step 3: Implement**

Create `engine/internal/account/account.go`. Mirror the tunnel client's DPAPI token persistence (read its exact `secret` import + `gh1:` scheme from `engine/internal/tunnel/client.go:79-115`). Implement `New`, `Link`, `Unlink`, `Linked`, `Entitlement` per the Interfaces. Use a `*http.Client` with a sane timeout. Return clear errors on non-200 responses. Keep the device token in memory after load to avoid re-reading per call (load lazily from disk on first use).

- [ ] **Step 4: Run to verify it passes + suite**

Run: `go -C engine test ./internal/account/ && go -C engine vet ./...`
Expected: PASS; vet clean.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/account/
git commit -s -m "feat(account): platform client + DPAPI-linked device credential"
```

---

### Task 4: Manager wiring — vanity slug + entitlement fetch

**Files:**
- Modify: `engine/internal/server/manager.go` (`Account` interface, `SetVanitySlug`, `tunnelWants` entitlement fetch, `TunnelWant.Entitlement`)
- Test: `engine/internal/server/manager_test.go` (fake `Account` + fake `Tunnel`)

**Interfaces:**
- Consumes: `account.Store` (adapted to a manager-local interface, like `Tunnel`).
- Produces:
  - manager-local `type Account interface { Linked() bool; Entitlement(ctx context.Context, slug string) (string, error) }`
  - `func (m *Manager) SetAccount(a Account)` (setter, NOT `NewManager` — keep existing tests green, mirroring `SetTunnel`)
  - `func (m *Manager) SetVanitySlug(id, name string) error` — validates `name` matches `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$` AND not `^gn-`, sets `Server.TunnelSlug = name`; empty name reverts to an auto `gn-` slug.
  - `TunnelWant.Entitlement string` — populated in `tunnelWants` when the server's slug is a vanity (not `gn-`) and `Account.Linked()`.

- [ ] **Step 1: Write the failing test**

Add to `engine/internal/server/manager_test.go` a test using a fake `Account` (returns a canned token, records the slug) and a fake `Tunnel` (records the `TunnelWant`s). Assert: a server with a vanity slug + linked account produces a `TunnelWant` whose `Entitlement` is the fetched token and whose slug is the vanity name; a server with a `gn-` slug produces a want with empty `Entitlement`; `SetVanitySlug` rejects `gn-foo` and names with bad chars.

```go
type fakeAccount struct{ linked bool; gotSlug string }
func (f *fakeAccount) Linked() bool { return f.linked }
func (f *fakeAccount) Entitlement(_ context.Context, slug string) (string, error) { f.gotSlug = slug; return "ent-" + slug, nil }

// ... test sets up a Manager with SetTunnel(fakeTunnel) + SetAccount(&fakeAccount{linked:true}),
// creates a running server, SetVanitySlug(id,"alice"), triggers a sync, and asserts the
// fake tunnel saw a want {Slug:"alice", Entitlement:"ent-alice"}. Also assert SetVanitySlug
// returns an error for "gn-bad" and "Bad Name".
```

(Model the harness on the existing tunnel manager tests in this file — reuse their fake-tunnel + running-server setup.)

- [ ] **Step 2: Run to verify it fails**

Run: `go -C engine test ./internal/server/ -run Vanity`
Expected: FAIL — `SetAccount`/`SetVanitySlug`/`TunnelWant.Entitlement` undefined.

- [ ] **Step 3: Implement**

Add the `Account` interface + `m.acct` field + `SetAccount`. Add `TunnelWant.Entitlement`. In `tunnelWants` (`manager.go:925-955`), for each wanted server: if `TunnelSlug` does not start with `gn-` and `m.acct != nil && m.acct.Linked()`, call `m.acct.Entitlement(ctx, slug)` and set `want.Entitlement` (on error, log + skip the tunnel for that server — best-effort, do not crash the sync). Thread `Entitlement` through the `tunnel.Desired`/adapter so it reaches `Client.Allocate` (Task 2). Implement `SetVanitySlug` with the validation regex.

- [ ] **Step 4: Run to verify it passes + suite**

Run: `go -C engine test ./internal/server/ && go -C engine vet ./...`
Expected: PASS; vet clean.

- [ ] **Step 5: Commit**

```bash
git add engine/internal/server/
git commit -s -m "feat(tunnel): fetch entitlement for vanity slugs via the account collaborator"
```

---

### Task 5: API + main wiring (account endpoints, dormant platform URL)

**Files:**
- Create: `engine/internal/api/account.go`
- Modify: `engine/internal/api/router.go` (Deps + routes), `engine/internal/api/tunnel.go` (adapter pattern reference)
- Modify: `engine/cmd/engine/main.go` (`GAMENEST_PLATFORM_URL`, construct account store, wire to manager + Deps)
- Test: `engine/internal/api/account_test.go`

**Interfaces:**
- Produces:
  - `GET /api/account` → `{ "configured": bool, "linked": bool }`
  - `POST /api/account/link` body `{ "code": "..." }` → links; 400 on bad code; 503 when not configured.
  - `DELETE /api/account/link` → unlinks.
  - `PUT /api/servers/{id}/vanity` body `{ "name": "..." }` → `SetVanitySlug`; `{ "name": "" }` reverts to auto.
  - An `accountAdapter` bridging `*account.Store` → the manager's `Account` interface (mirror `AdaptTunnel`, `tunnel.go:56`).

- [ ] **Step 1: Write the failing test**

Create `engine/internal/api/account_test.go`. Use the existing API test harness in this package (see `tunnel`/`license` handler tests) with a fake or real `account.Store` pointed at an `httptest` platform. Assert: `GET /api/account` reports `configured:false` when no store; with a store, `POST /api/account/link {code}` links and `GET` then reports `linked:true`; `DELETE` unlinks; `PUT /servers/{id}/vanity {name:"alice"}` succeeds and `{name:"gn-x"}` → 400.

- [ ] **Step 2: Run to verify it fails**

Run: `go -C engine test ./internal/api/ -run Account`
Expected: FAIL — handlers/routes undefined.

- [ ] **Step 3: Implement handlers + routes**

Create `engine/internal/api/account.go` with the handlers (read `account` via `Deps.Account`, nil ⇒ `configured:false`/503). Register routes in `router.go` inside the protected group (mirror the `license` routes at `router.go:102-104`). Add `Account *account.Store` to `Deps` + `API` (mirror `Tunnel`/`License`). Add `accountAdapter` + `AdaptAccount`.

- [ ] **Step 4: Wire main.go**

In `engine/cmd/engine/main.go`, after the `GAMEHOST_TUNNEL_URL` block (`main.go:54-57`), add:

```go
var acct *account.Store
if url := os.Getenv("GAMENEST_PLATFORM_URL"); url != "" {
	acct = account.New(cfg.DataDir, url)
	mgr.SetAccount(api.AdaptAccount(acct))
}
```

Pass `acct` into `api.Deps{ ..., Account: acct }` (it is fine for `Deps.Account` to be nil when dormant — handlers report `configured:false`).

- [ ] **Step 5: Run full suite + vet**

Run: `go -C engine test ./... && go -C engine vet ./...`
Expected: all packages PASS; vet clean.

- [ ] **Step 6: Commit**

```bash
git add engine/internal/api/ engine/cmd/engine/main.go
git commit -s -m "feat(api): account link/status + vanity endpoints; GAMENEST_PLATFORM_URL gating"
```

---

### Task 6: UI — account linking + Plus status + per-server vanity name

**Build/lint-verified only (no UI test runner).**

**Files:**
- Modify: `ui/src/lib/api.ts` (add `account()`, `linkAccount(code)`, `unlinkAccount()`, `setVanity(id,name)` + types)
- Modify: `ui/src/components/Settings.tsx` (repurpose the "Supporter" section, `Settings.tsx:346-390`)
- Modify: the server detail/connection UI where the tunnel "Share with friends" control lives — add a "Use my GameNest name" field when the account is linked.

**Interfaces:**
- Consumes: the Task 5 endpoints.
- Produces: `api.account()` → `{ configured: boolean; linked: boolean }`; `api.linkAccount(code)`, `api.unlinkAccount()`, `api.setVanity(id, name)`.

- [ ] **Step 1: API client**

In `ui/src/lib/api.ts`, add (mirroring the `license`/`tunnel` calls at `api.ts:288-308`):

```ts
account: () => get<AccountStatus>("/api/account"),
linkAccount: (code: string) => send<AccountStatus>("POST", "/api/account/link", { code }),
unlinkAccount: () => send<AccountStatus>("DELETE", "/api/account/link"),
setVanity: (id: string, name: string) =>
  send<{ status: string }>("PUT", `/api/servers/${id}/vanity`, { name }),
```

with `export interface AccountStatus { configured: boolean; linked: boolean }`.

- [ ] **Step 2: Settings section**

Repurpose the "Supporter" block in `Settings.tsx` into "GameNest Plus": when `account().configured`, show either a "Paste your link code" input → `linkAccount` (with a link to the dashboard to get the code), or, when `linked`, a "Linked ✓ / Unlink" state. Keep the existing dark styling. Leave the legacy supporter `license` key handling intact (it is independent) or hide it behind an "Advanced" toggle.

- [ ] **Step 3: Per-server vanity control**

Where the tunnel address is shown for a server, when the account is linked add a small "Use my GameNest name" text field that calls `setVanity(id, name)` and then refreshes; show the resulting `your-name.gn.coderaum.com` address. Gate the whole control on `account().configured && linked` so it is invisible in the dormant/free case (byte-for-byte unchanged when not configured).

- [ ] **Step 4: Verify**

Run: `npm --prefix ui run build && npm --prefix ui run lint`
Expected: tsc + vite build pass; no NEW lint errors (pre-existing warnings may remain).

- [ ] **Step 5: Commit**

```bash
git add ui/src/
git commit -s -m "feat(ui): account linking, Plus status, per-server vanity name"
```

---

## Go-live (after the platform is deployed)

Set `GAMENEST_PLATFORM_URL=https://<platform>` (and keep `GAMEHOST_TUNNEL_URL=https://cp.coderaum.com`) for the engine; deploy the relay with `GAMENEST_ENTITLEMENT_PUBKEY` set to the platform's signing pubkey (flips entitlements on); then live e2e: link a Plus account in the app → reserve a name on the dashboard → set it on a server → confirm the tunnel comes up on `your-name.gn.coderaum.com` and a friend connects; cancel the subscription → within one token TTL the engine can no longer get an entitlement and the server falls back to (or loses) the vanity tunnel.

## Self-Review

**Spec coverage** (engine section of `gamenest-relay/docs/specs/2026-06-20-tunnel-subscription-design.md`):
- "paste-a-link-code account linking" → Task 3 (`Link`) + Task 5 (endpoint) + Task 6 (UI). ✓
- "fetch entitlements + attach to allocate" → Task 3 (`Entitlement`) + Task 2 (allocate param) + Task 4 (manager fetch). ✓
- "gn- free-slug-per-session" carry-forward → Task 1 (gn- namespace fix; the slug is generated once per server and reused, so it is stable within a session and re-sync is idempotent). ✓
- "Plan UI" → Task 6. ✓
- Dormant-by-default → Task 5 (`GAMENEST_PLATFORM_URL` gating). ✓
- Squat-proofing contract (engine never asks for a vanity name without an entitlement; free stays `gn-`) → Task 4 (only non-`gn-` slugs trigger an entitlement fetch; `SetVanitySlug` forbids `gn-`). ✓

**Placeholders:** none — Tasks 1–4 carry full test + impl code or precise edits anchored to real `file:line`. Tasks 5–6 give exact endpoints, the dormant-wiring code, and the exact `api.ts` additions; their bodies follow the existing `tunnel`/`license` handler + client patterns named by anchor.

**Type consistency:** `Account` interface (`Linked()`, `Entitlement(ctx,slug)`) consistent across Task 3 (`account.Store`), Task 4 (manager interface), Task 5 (adapter). `Allocate(ctx,slug,ports,entitlement)` consistent across Task 2 and the Task 4 thread-through. `AccountStatus{configured,linked}` consistent across Task 5 (JSON) and Task 6 (TS).

**Interop guardrails:** Task 1 aligns free slugs to the relay's `^gn-[a-z0-9]{4,50}$`; the entitlement is forwarded verbatim (Task 2) so it stays byte-exact with the platform signer + relay verifier already proven to interoperate.
