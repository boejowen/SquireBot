//go:build !windows

package eqfind

// scanUninstallKeys is a no-op on non-Windows platforms. The Windows
// implementation lives in registry_windows.go.
func scanUninstallKeys() string { return "" }
