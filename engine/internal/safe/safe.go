// Package safe runs background goroutines that recover and log panics, so a
// single background task (a scheduled job, a per-connection console loop) can't
// take down the whole engine process. chi's Recoverer only wraps the HTTP
// handler call, not goroutines spawned inside it — those need this.
package safe

import "log/slog"

// OnPanic, if set, is invoked with the task name and recovered value whenever
// Guard catches a panic. It lets an optional observer (e.g. crash telemetry)
// see background panics without this package importing it. Set once at startup.
var OnPanic func(task string, v any)

// Guard runs fn synchronously, recovering and logging any panic.
func Guard(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("background task panicked", "task", name, "panic", r)
			if OnPanic != nil {
				OnPanic(name, r)
			}
		}
	}()
	fn()
}

// Go runs fn in a goroutine under Guard.
func Go(name string, fn func()) { go Guard(name, fn) }
