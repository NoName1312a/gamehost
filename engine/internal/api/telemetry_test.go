package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestTelemetryStatusDefaultsToDisabled(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := do(t, h, http.MethodGet, "/api/system/telemetry", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Errorf("telemetry should default to disabled: %s", rec.Body.String())
	}
}

func TestSetTelemetryEnablesAndPersists(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := do(t, h, http.MethodPost, "/api/system/telemetry", `{"enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Errorf("POST should report enabled:true, got %s", rec.Body.String())
	}
	rec2 := do(t, h, http.MethodGet, "/api/system/telemetry", "")
	if !strings.Contains(rec2.Body.String(), `"enabled":true`) {
		t.Errorf("status after enabling should read enabled:true, got %s", rec2.Body.String())
	}
}

func TestSetTelemetryCanDisable(t *testing.T) {
	h, _, _ := newTestAPI(t)
	_ = do(t, h, http.MethodPost, "/api/system/telemetry", `{"enabled":true}`)
	rec := do(t, h, http.MethodPost, "/api/system/telemetry", `{"enabled":false}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("disable: want 200 enabled:false, got %d (%s)", rec.Code, rec.Body.String())
	}
}
