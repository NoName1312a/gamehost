package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSendDoesNothingWhenDisabled(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	store := NewStore(t.TempDir()) // off by default
	rep := NewReporter(store, srv.URL, "test")
	rep.Send(Event{Type: "feature_used"})

	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("disabled telemetry must make no network calls, got %d", n)
	}
}

func TestSendPostsEventWhenEnabled(t *testing.T) {
	got := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev Event
		_ = json.NewDecoder(r.Body).Decode(&ev)
		got <- ev
	}))
	defer srv.Close()

	store := NewStore(t.TempDir())
	if err := store.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	rep := NewReporter(store, srv.URL, "1.2.3")
	rep.Send(Event{Type: "engine_start"})

	select {
	case ev := <-got:
		if ev.Type != "engine_start" {
			t.Errorf("type: got %q, want engine_start", ev.Type)
		}
		if ev.Engine != "1.2.3" {
			t.Errorf("engine version: got %q, want 1.2.3", ev.Engine)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("enabled telemetry should have posted an event")
	}
}

func TestSendDoesNothingWithEmptyEndpoint(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	// Opted in, but no endpoint configured: must be a safe no-op (no panic/block).
	rep := NewReporter(store, "", "v")
	rep.Send(Event{Type: "x"})
}

func TestRecoverCatchesPanicAndReportsWhenEnabled(t *testing.T) {
	got := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev Event
		_ = json.NewDecoder(r.Body).Decode(&ev)
		got <- ev
	}))
	defer srv.Close()

	store := NewStore(t.TempDir())
	if err := store.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	rep := NewReporter(store, srv.URL, "9.9.9")

	func() {
		defer rep.Recover("worker")
		panic("boom")
	}() // if Recover didn't catch it, this test goroutine would crash

	select {
	case ev := <-got:
		if ev.Type != "crash" {
			t.Errorf("type: got %q, want crash", ev.Type)
		}
		if !strings.Contains(ev.Data["panic"], "boom") {
			t.Errorf("crash should carry the panic value, got %q", ev.Data["panic"])
		}
		if ev.Data["task"] != "worker" {
			t.Errorf("task: got %q, want worker", ev.Data["task"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a panic under Recover (enabled) should report a crash event")
	}
}

func TestRecoverSwallowsPanicWhenDisabled(t *testing.T) {
	store := NewStore(t.TempDir()) // disabled
	rep := NewReporter(store, "http://127.0.0.1:0", "v")
	func() {
		defer rep.Recover("worker")
		panic("boom")
	}()
	// Reaching here proves the panic was recovered even with telemetry off.
}
