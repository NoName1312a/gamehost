package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/leop1/gamehost/engine/internal/network"
)

// ConnectivityInfo tells the UI how friends can reach a server: the public
// address, whether the router auto-forwarded the port, and the LAN IP to use
// for a manual port-forward when it didn't.
type ConnectivityInfo struct {
	Running         bool   `json:"running"`
	Port            int    `json:"port"`
	Protocol        string `json:"protocol"`
	ExternalIP      string `json:"externalIP,omitempty"`
	ExternalAddress string `json:"externalAddress,omitempty"`
	LocalIP         string `json:"localIP,omitempty"`
	UPnPAvailable   bool   `json:"upnpAvailable"`
	Forwarded       bool   `json:"forwarded"`
}

func (a *API) connectivity(w http.ResponseWriter, r *http.Request) {
	s, ok := a.mgr.Get(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	info := ConnectivityInfo{Protocol: "tcp", LocalIP: network.LocalIP()}
	if len(s.Ports) > 0 {
		info.Port = s.Ports[0].Host
		info.Protocol = s.Ports[0].Protocol
	}
	info.Running = a.rt.Inspect(ctx, s.ContainerName()).Running
	if a.net != nil {
		info.UPnPAvailable = a.net.Probe(ctx).UPnPAvailable
		info.ExternalIP = a.net.ExternalIP()
		info.Forwarded = a.net.IsMapped(info.Port, info.Protocol)
		if info.ExternalIP != "" && info.Port > 0 {
			info.ExternalAddress = info.ExternalIP + ":" + strconv.Itoa(info.Port)
		}
	}
	writeJSON(w, http.StatusOK, info)
}

// connectivityTest runs an on-demand external reachability probe (slow: it asks
// public nodes to connect back to the server's port).
func (a *API) connectivityTest(w http.ResponseWriter, r *http.Request) {
	s, ok := a.mgr.Get(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return
	}
	if len(s.Ports) == 0 {
		writeJSON(w, http.StatusBadRequest, errMsg("server has no ports"))
		return
	}
	port := s.Ports[0].Host
	proto := s.Ports[0].Protocol
	if a.net == nil || a.net.ExternalIP() == "" {
		writeJSON(w, http.StatusOK, network.Reachable{Detail: "Your public IP isn't known yet — try again in a moment."})
		return
	}
	if !strings.EqualFold(proto, "tcp") {
		// UDP is connectionless; an external "is it open" probe isn't reliable.
		writeJSON(w, http.StatusOK, network.Reachable{
			Detail: "This game uses UDP, which can't be reliably tested from outside. Have a friend try connecting to confirm.",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, network.CheckTCPReachable(ctx, a.net.ExternalIP(), port))
}
