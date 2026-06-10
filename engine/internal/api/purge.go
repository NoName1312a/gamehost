package api

import (
	"log/slog"
	"net/http"
)

// purgeData removes every game server — containers, data volumes, and records.
// It backs the uninstaller's opt-in "remove all game data" step. Best-effort:
// it always reports how many were removed (so uninstall can proceed) and logs
// any partial failure rather than erroring out.
func (a *API) purgeData(w http.ResponseWriter, r *http.Request) {
	removed, err := a.mgr.PurgeAll(r.Context())
	if err != nil {
		slog.Warn("purge: some servers failed to remove", "removed", removed, "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}
