package docker

import (
	"io"
	"testing"
)

// Closing the logs reader must (1) cancel the command context so the underlying
// `docker logs -f` process is killed (otherwise it blocks on a full stdout pipe
// and leaks), (2) unblock any pending read, and (3) be safe to call more than
// once.
func TestLogsReadCloserCloseCancelsAndClosesOnce(t *testing.T) {
	pr, _ := io.Pipe()
	cancels := 0
	lc := &logsReadCloser{pr: pr, cancel: func() { cancels++ }}

	if err := lc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if cancels != 1 {
		t.Fatalf("cancel called %d times, want 1 (process must be killed on Close)", cancels)
	}
	if _, err := pr.Read(make([]byte, 1)); err == nil {
		t.Fatal("read after Close should fail; the reader wasn't closed")
	}

	// Idempotent: a second Close must not cancel again or panic.
	if err := lc.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if cancels != 1 {
		t.Fatalf("cancel called %d times after double Close, want 1", cancels)
	}
}
