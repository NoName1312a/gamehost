package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// User management is owner-only. Loopback (the local desktop user) is the owner;
// remote callers need a session whose account has the owner role.

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	if !a.requireOwner(r) {
		writeJSON(w, http.StatusForbidden, errMsg("only the owner can manage users"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": a.auth.ListUsers()})
}

func (a *API) addUser(w http.ResponseWriter, r *http.Request) {
	if !a.requireOwner(r) {
		writeJSON(w, http.StatusForbidden, errMsg("only the owner can manage users"))
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if err := a.auth.AddUser(body.Username, body.Password, body.Role); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !a.requireOwner(r) {
		writeJSON(w, http.StatusForbidden, errMsg("only the owner can manage users"))
		return
	}
	if err := a.auth.DeleteUser(chi.URLParam(r, "username")); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
