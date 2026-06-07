package api

import (
	"testing"
	"time"
)

func TestLoginLimiterBlocksAfterMax(t *testing.T) {
	l := newLoginLimiter(3, time.Minute)
	now := time.Unix(1000, 0)
	ip := "1.2.3.4"

	if l.blocked(ip, now) {
		t.Fatal("should not be blocked initially")
	}
	l.fail(ip, now)
	l.fail(ip, now)
	if l.blocked(ip, now) {
		t.Error("should not be blocked before reaching the max")
	}
	l.fail(ip, now)
	if !l.blocked(ip, now) {
		t.Error("should be blocked once the max failures is reached")
	}
	// The window expires, clearing the block.
	if l.blocked(ip, now.Add(2*time.Minute)) {
		t.Error("should be unblocked after the window passes")
	}
}

func TestLoginLimiterResetOnSuccess(t *testing.T) {
	l := newLoginLimiter(3, time.Minute)
	now := time.Unix(1000, 0)
	ip := "1.2.3.4"

	l.fail(ip, now)
	l.fail(ip, now)
	l.reset(ip) // a successful login clears the counter
	l.fail(ip, now)
	if l.blocked(ip, now) {
		t.Error("reset should have cleared the earlier failures")
	}
}

func TestLoginLimiterIsPerIP(t *testing.T) {
	l := newLoginLimiter(2, time.Minute)
	now := time.Unix(1000, 0)
	l.fail("1.1.1.1", now)
	l.fail("1.1.1.1", now)
	if l.blocked("2.2.2.2", now) {
		t.Error("a different IP must not be blocked by another's failures")
	}
}
