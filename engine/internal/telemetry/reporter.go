package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"
)

// Event is a single anonymous telemetry record. It carries no personal data:
// just an event type, the engine version, a timestamp, and a small string map
// (e.g. OS, panic message, code stack).
type Event struct {
	Type      string            `json:"type"`
	Engine    string            `json:"engine"`
	Timestamp int64             `json:"ts"`
	Data      map[string]string `json:"data,omitempty"`
}

// Reporter sends events to a configured endpoint, but only when the user has
// opted in. With no endpoint configured (the default), it is a complete no-op,
// so nothing ever leaves the machine.
type Reporter struct {
	store    *Store
	endpoint string
	version  string
	client   *http.Client
	now      func() time.Time
}

// NewReporter builds a reporter. endpoint may be empty (telemetry disabled at
// the transport level regardless of consent).
func NewReporter(store *Store, endpoint, version string) *Reporter {
	return &Reporter{
		store:    store,
		endpoint: endpoint,
		version:  version,
		client:   &http.Client{Timeout: 10 * time.Second},
		now:      time.Now,
	}
}

// Send posts an event, best-effort. It is silently skipped unless the user has
// opted in AND an endpoint is configured. Errors are swallowed — telemetry must
// never affect engine behavior.
func (r *Reporter) Send(ev Event) {
	if r == nil || r.endpoint == "" || !r.store.IsEnabled() {
		return
	}
	ev.Engine = r.version
	if ev.Timestamp == 0 {
		ev.Timestamp = r.now().UnixMilli()
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "gamehost-engine/"+r.version)
	resp, err := r.client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// Recover is deferred at the top of a goroutine to catch an unhandled panic,
// log it, and (if opted in) report it. It always swallows the panic so the
// process/goroutine doesn't die from a crash we've already recorded.
func (r *Reporter) Recover(task string) {
	if v := recover(); v != nil {
		slog.Error("panic recovered", "task", task, "panic", v)
		r.ReportPanic(task, v)
	}
}

// ReportPanic sends a crash event for an already-recovered panic. It does not
// log (callers that recover via package safe already log), making it a clean
// hook for safe.OnPanic.
func (r *Reporter) ReportPanic(task string, v any) {
	r.Send(Event{
		Type: "crash",
		Data: map[string]string{
			"task":  task,
			"panic": fmt.Sprintf("%v", v),
			"stack": string(debug.Stack()),
			"os":    runtime.GOOS,
		},
	})
}
