package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/leop1/gamehost/engine/internal/account"
	"github.com/leop1/gamehost/engine/internal/server"
)

// accountStatus reports whether a GameNest platform account is configured and,
// if so, whether this device is linked. When GAMENEST_PLATFORM_URL is unset no
// store is wired and configured=false is returned so the UI can hide the feature.
func (a *API) accountStatus(w http.ResponseWriter, r *http.Request) {
	if a.account == nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false, "linked": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"linked":     a.account.Linked(),
	})
}

// linkAccount exchanges a user-supplied one-time code for a persistent device
// credential from the platform. 503 when not configured; 400 on empty or
// rejected code; 200 on success.
func (a *API) linkAccount(w http.ResponseWriter, r *http.Request) {
	if a.account == nil {
		writeJSON(w, http.StatusServiceUnavailable, errMsg("account not configured"))
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if body.Code == "" {
		writeJSON(w, http.StatusBadRequest, errMsg("code is required"))
		return
	}
	if err := a.account.Link(r.Context(), body.Code); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("link failed: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// unlinkAccount removes the stored device credential. 503 when not configured.
func (a *API) unlinkAccount(w http.ResponseWriter, r *http.Request) {
	if a.account == nil {
		writeJSON(w, http.StatusServiceUnavailable, errMsg("account not configured"))
		return
	}
	_ = a.account.Unlink()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// setVanitySlug assigns a custom DNS-label slug to a tunnel-enabled server
// (PUT body {"name":"..."}). An empty name reverts to the auto-generated slug.
// 400 on validation failure (reserved prefix, bad characters, etc.).
func (a *API) setVanitySlug(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if err := a.mgr.SetVanitySlug(id, body.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// accountLinker is the subset of *account.Store the manager-level Account
// interface drives. An interface keeps the adapter unit-testable with a fake.
type accountLinker interface {
	Linked() bool
	Entitlement(ctx context.Context, slug string) (string, error)
}

// accountAdapter bridges the manager's Account interface to the account
// package's own types, so an account.Store satisfies server.Account without
// the manager importing the account package.
type accountAdapter struct{ st accountLinker }

// AdaptAccount wraps an account.Store so the server manager can drive it
// through Manager.SetAccount.
func AdaptAccount(st *account.Store) server.Account { return accountAdapter{st: st} }

func (a accountAdapter) Linked() bool { return a.st.Linked() }

func (a accountAdapter) Entitlement(ctx context.Context, slug string) (string, error) {
	return a.st.Entitlement(ctx, slug)
}
