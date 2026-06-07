package safe

import "testing"

func TestGuardRecoversPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic escaped Guard: %v", r)
		}
	}()
	Guard("test-task", func() { panic("boom") })
	// Reaching here means Guard contained the panic instead of propagating it.
}

func TestGuardRunsFn(t *testing.T) {
	ran := false
	Guard("test-task", func() { ran = true })
	if !ran {
		t.Fatal("Guard did not run the function")
	}
}

func TestGoRecoversPanic(t *testing.T) {
	done := make(chan struct{})
	// A panic inside Go must not crash the process; it runs and signals done.
	Go("test-task", func() {
		defer close(done)
		panic("boom")
	})
	<-done
}
