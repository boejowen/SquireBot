//go:build !windows

package main

// freeConsole is a no-op on non-Windows platforms. The Windows
// implementation lives in console_windows.go.
// Plan 09-02 (OPS-07).
func freeConsole() error { return nil }
