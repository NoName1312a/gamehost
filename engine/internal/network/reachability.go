package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// reachabilityBase is the external TCP-reachability checker. check-host.net is a
// free, keyless, multi-node service: it connects to host:port from several
// public nodes and reports per-node success/failure — exactly the "can someone
// on the internet reach this port" question. Kept as a var so it can be pointed
// at a self-hosted checker later.
var reachabilityBase = "https://check-host.net"

// Reachable is the outcome of an external port-reachability probe.
//
// Checked distinguishes "we got an answer" from "we couldn't run the check"
// (service down, offline) so the UI can say "not reachable" vs "couldn't
// verify" rather than crying wolf.
type Reachable struct {
	Open    bool   `json:"open"`
	Checked bool   `json:"checked"`
	Detail  string `json:"detail"`
}

// CheckTCPReachable asks an external service whether host:port accepts a TCP
// connection from the public internet. Best-effort and bounded: any transport
// error degrades to {Checked:false} rather than a false "closed".
//
// Only meaningful for TCP. UDP is connectionless, so external "is it open"
// probes are unreliable — callers should not present a UDP result as definitive.
func CheckTCPReachable(ctx context.Context, host string, port int) Reachable {
	target := host + ":" + strconv.Itoa(port)
	reqURL := reachabilityBase + "/check-tcp?" + url.Values{"host": {target}}.Encode()

	var start struct {
		OK        int    `json:"ok"`
		RequestID string `json:"request_id"`
	}
	if err := getJSON(ctx, reqURL, &start); err != nil {
		return Reachable{Detail: "couldn't reach the connection checker: " + err.Error()}
	}
	if start.OK != 1 || start.RequestID == "" {
		return Reachable{Detail: "connection checker declined the request"}
	}

	// Results stream in per node over a few seconds; poll until at least one
	// node has a verdict or we run out of time.
	resURL := reachabilityBase + "/check-result/" + start.RequestID
	deadline := time.Now().Add(18 * time.Second)
	for {
		var raw map[string]json.RawMessage
		if err := getJSON(ctx, resURL, &raw); err == nil {
			open, decided := interpret(raw)
			if decided {
				if open {
					return Reachable{Open: true, Checked: true, Detail: "A test connection from the internet reached the port."}
				}
				return Reachable{Open: false, Checked: true, Detail: "No test connection from the internet could reach the port."}
			}
		}
		if time.Now().After(deadline) {
			return Reachable{Detail: "the connection check timed out"}
		}
		select {
		case <-ctx.Done():
			return Reachable{Detail: "the connection check was cancelled"}
		case <-time.After(2 * time.Second):
		}
	}
}

// interpret reads check-host.net's per-node results. Each node value is null
// (still pending), or a one-element array holding either {"time":..,"address":..}
// on a successful connect or {"error":".."} on failure. Returns (open, decided):
// decided is true once we have a definitive answer (any node connected -> open;
// all reporting nodes failed -> closed).
func interpret(raw map[string]json.RawMessage) (open, decided bool) {
	reported, failures := 0, 0
	for _, v := range raw {
		if len(v) == 0 || string(v) == "null" {
			continue // node hasn't reported yet
		}
		var arr []map[string]any
		if json.Unmarshal(v, &arr) != nil || len(arr) == 0 || arr[0] == nil {
			continue
		}
		reported++
		entry := arr[0]
		if _, bad := entry["error"]; bad {
			failures++
			continue
		}
		if _, ok := entry["time"]; ok {
			return true, true // a node connected -> reachable
		}
		if _, ok := entry["address"]; ok {
			return true, true
		}
	}
	// All nodes that have reported failed, and we have enough of them.
	if reported >= 2 && failures == reported {
		return false, true
	}
	return false, false
}

func getJSON(ctx context.Context, u string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// LocalIP returns this machine's primary LAN IP (for manual port-forward
// instructions), or "" if it can't be determined.
func LocalIP() string { return strings.TrimSpace(localIP()) }
