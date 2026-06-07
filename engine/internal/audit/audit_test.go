package audit

import (
	"bytes"
	"strings"
	"testing"
)

func TestRecordWritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Record("local", "POST", "/api/servers", 201)

	line := buf.String()
	for _, want := range []string{`"actor":"local"`, `"method":"POST"`, `"path":"/api/servers"`, `"status":201`} {
		if !strings.Contains(line, want) {
			t.Errorf("audit line missing %s:\n%s", want, line)
		}
	}
	if !strings.HasSuffix(line, "\n") {
		t.Error("each entry should be newline-terminated (JSON lines)")
	}
}

func TestNilLoggerIsNoOp(t *testing.T) {
	var l *Logger
	// Recording on a nil logger must not panic (audit is optional).
	l.Record("x", "y", "z", 1)
}
