//go:build !windows

package eqfind

// heuristicScan is a no-op on non-Windows platforms. The Windows implementation
// lives in heuristic_windows.go.
func heuristicScan() string { return "" }
