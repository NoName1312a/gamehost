//go:build !windows

// Package secret encrypts small secrets at rest. On non-Windows platforms there
// is no DPAPI; this is a passthrough placeholder (the file is still 0600). A
// future headless build can swap in an OS keychain or an age-based scheme here.
package secret

// Protect returns data unchanged (no OS secret store wired on this platform).
func Protect(data []byte) ([]byte, error) { return data, nil }

// Unprotect returns data unchanged.
func Unprotect(data []byte) ([]byte, error) { return data, nil }
