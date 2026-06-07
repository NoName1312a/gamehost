package api

import (
	"sync"
	"time"
)

// loginLimiter throttles failed login attempts per client IP to slow password
// brute-forcing once remote access is exposed. It's a simple fixed-window
// counter: after max failures within window, that IP is blocked until the
// window passes. A successful login resets the IP.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginWindow
	max      int
	window   time.Duration
}

type loginWindow struct {
	count   int
	resetAt time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{attempts: map[string]*loginWindow{}, max: max, window: window}
}

// blocked reports whether ip has reached the failure cap within the live window.
func (l *loginLimiter) blocked(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.attempts[ip]
	if !ok || now.After(w.resetAt) {
		return false
	}
	return w.count >= l.max
}

// fail records a failed attempt for ip, starting a new window if needed.
func (l *loginLimiter) fail(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.attempts[ip]
	if !ok || now.After(w.resetAt) {
		l.attempts[ip] = &loginWindow{count: 1, resetAt: now.Add(l.window)}
		return
	}
	w.count++
}

// reset clears ip's counter (called after a successful login).
func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	delete(l.attempts, ip)
	l.mu.Unlock()
}
