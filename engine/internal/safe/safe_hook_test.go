package safe

import "testing"

func TestGuardInvokesOnPanicHook(t *testing.T) {
	var gotTask string
	var gotVal any
	OnPanic = func(task string, v any) { gotTask, gotVal = task, v }
	defer func() { OnPanic = nil }()

	Guard("worker", func() { panic("boom") })

	if gotTask != "worker" {
		t.Errorf("task: got %q, want worker", gotTask)
	}
	if gotVal != "boom" {
		t.Errorf("panic value: got %v, want boom", gotVal)
	}
}

func TestGuardWithoutHookStillRecovers(t *testing.T) {
	OnPanic = nil
	// Must recover the panic (not propagate) even with no hook installed.
	Guard("worker", func() { panic("boom") })
}
