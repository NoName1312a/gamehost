package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/leop1/gamehost/engine/internal/setup"
)

// setupReport returns the guided-wizard prerequisite state (WSL2 + Docker).
func (a *API) setupReport(w http.ResponseWriter, r *http.Request) {
	// Detection shells out to docker/wsl, so allow a little headroom.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, setup.Detect(ctx, a.rt))
}

// runSetupStep launches the fix for one wizard step (enable-wsl, install-docker,
// start-docker). It's fire-and-forget: the elevated installer runs in its own
// (UAC-approved) process and the UI re-checks status afterward.
func (a *API) runSetupStep(w http.ResponseWriter, r *http.Request) {
	res, err := setup.RunAction(chi.URLParam(r, "step"))
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, setup.ErrUnsupported) {
			code = http.StatusNotImplemented
		}
		writeJSON(w, code, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, res)
}
