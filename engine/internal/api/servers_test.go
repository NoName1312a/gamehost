package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/server"
)

// do issues a request against the real router from a loopback address (the
// trusted desktop case, so auth doesn't reject it).
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = "127.0.0.1:8723"     // the UI addresses the engine at a loopback host
	r.Header.Set(csrfHeader, "1") // legit client (the UI) always sends this
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestCreateServerReturnsCreated(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := do(t, h, http.MethodPost, "/api/servers", `{"templateId":"test-mc","name":"My MC","port":25565}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["id"] == nil || got["name"] != "My MC" {
		t.Errorf("unexpected create response: %v", got)
	}
}

func TestCreateServerRejectsUnknownTemplate(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := do(t, h, http.MethodPost, "/api/servers", `{"templateId":"nope","name":"X"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for unknown template, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestCreateServerRejectsDuplicatePort(t *testing.T) {
	h, _, _ := newTestAPI(t)
	if rec := do(t, h, http.MethodPost, "/api/servers", `{"templateId":"test-mc","name":"A","port":25565}`); rec.Code != http.StatusCreated {
		t.Fatalf("first create: want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	rec := do(t, h, http.MethodPost, "/api/servers", `{"templateId":"test-mc","name":"B","port":25565}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate port: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestListAndDeleteServer(t *testing.T) {
	h, mgr, _ := newTestAPI(t)
	s, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "Listed", Port: 25565})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	// List reflects the created server (docker-less Inspect reports "not created").
	rec := do(t, h, http.MethodGet, "/api/servers", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0]["id"] != s.ID {
		t.Fatalf("list did not contain created server: %v", list)
	}

	// Delete removes it (docker errors are ignored by the manager).
	if rec := do(t, h, http.MethodDelete, "/api/servers/"+s.ID, ""); rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, "/api/servers", "")
	var after []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if len(after) != 0 {
		t.Fatalf("server still listed after delete: %v", after)
	}
}

func TestAuditLogsMutationsNotReads(t *testing.T) {
	h, _, _, auditBuf := newTestAPIFull(t)

	// A mutating request is recorded with method, path, status, and actor.
	do(t, h, http.MethodPost, "/api/servers", `{"templateId":"test-mc","name":"A","port":25565}`)
	line := auditBuf.String()
	for _, want := range []string{`"method":"POST"`, `"path":"/api/servers"`, `"status":201`, `"actor":"local"`} {
		if !strings.Contains(line, want) {
			t.Errorf("audit log missing %s:\n%s", want, line)
		}
	}

	// A read (GET) is not recorded.
	auditBuf.Reset()
	do(t, h, http.MethodGet, "/api/servers", "")
	if auditBuf.Len() != 0 {
		t.Errorf("GET should not be audited, got:\n%s", auditBuf.String())
	}
}

func TestLicenseStatusAndRejectsBadKey(t *testing.T) {
	h, _, _ := newTestAPI(t)
	// Default tier is free.
	rec := do(t, h, http.MethodGet, "/api/license", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pro":false`) {
		t.Fatalf("license status: %d %s", rec.Code, rec.Body.String())
	}
	// A bogus key is rejected and stays free.
	if rec := do(t, h, http.MethodPost, "/api/license", `{"key":"garbage"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad license: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestSetScheduleValidation(t *testing.T) {
	h, mgr, _ := newTestAPI(t)
	s, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "Sched", Port: 25565})
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}
	// Invalid time is rejected.
	if rec := do(t, h, http.MethodPut, "/api/servers/"+s.ID+"/schedule", `{"restartAt":"99:99"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid schedule: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	// Valid time is accepted.
	if rec := do(t, h, http.MethodPut, "/api/servers/"+s.ID+"/schedule", `{"restartAt":"03:30","backupAt":""}`); rec.Code != http.StatusOK {
		t.Fatalf("valid schedule: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	// Unknown server id is a 404.
	if rec := do(t, h, http.MethodPut, "/api/servers/deadbeef/schedule", `{"restartAt":"03:30"}`); rec.Code != http.StatusBadRequest {
		// SetSchedule returns "server not found" as a 400 from this handler.
		t.Fatalf("unknown server schedule: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}
