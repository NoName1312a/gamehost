package api

import (
	"context"
	"net/http"
	"time"
)

// networkStatus reports whether the router supports automatic port forwarding
// (UPnP) and the public IP friends would connect to. Discovery runs in the
// background; a short timeout here means an early poll may report "checking",
// and the UI's recurring poll picks up the result once it lands.
func (a *API) networkStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, a.net.Probe(ctx))
}
