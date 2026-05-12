//go:build windows

package main

import (
	"fmt"
	"log/slog"

	"golang.org/x/sys/windows"
)

// kernel32 + procFreeConsole resolve the Win32 FreeConsole entry point at
// first use. Lazy-loading via x/sys/windows.NewLazySystemDLL is the
// canonical Go idiom for Win32 functions not directly bound by
// `golang.org/x/sys/windows` (FreeConsole is one of them in v0.43.0).
// NewLazySystemDLL forces the system32 search path, mitigating DLL-preload
// attacks.
var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole  = kernel32.NewProc("FreeConsole")
)

// freeConsole detaches the watcher process from any inherited console
// (the parent cmd.exe / PowerShell that launched squirebot.exe). After
// this call, closing the launching shell no longer kills the watcher.
//
// Plan 09-02 (OPS-07). Fixes Phase 6 UAT Finding H: foreground-launched
// watcher dies silently when the parent shell closes.
//
// Ordering rule: this MUST be called AFTER the --quit and
// --uninstall-wipe-credentials short-circuits return non-exiting (those
// paths write to stderr that NSIS / parent-process captures) and AFTER
// update.Apply() runs, but BEFORE logging.Setup() so subsequent slog
// output writes only to the lumberjack-backed log file.
//
// Returns nil if the process had no console attached (e.g., launched via
// the GUI subsystem or by Explorer double-click) — safe to call
// unconditionally. On any FreeConsole failure, log at Warn level via slog
// (which falls back to stderr because logging.Setup has not yet run —
// intentional: a detach failure is informational only and the watcher
// continues regardless).
//
// FreeConsole returns a BOOL (non-zero = success). On failure the syscall
// also surfaces a Win32 LastError via the LazyProc.Call third return.
// Per MSDN, GetLastError after a successful no-console call is benign
// (ERROR_INVALID_HANDLE on processes that never had a console); we map
// the BOOL contract directly and only return non-nil when ret == 0.
func freeConsole() error {
	ret, _, err := procFreeConsole.Call()
	if ret == 0 {
		// Some Windows builds set err to "The operation completed
		// successfully." even on no-console processes; we only treat
		// ret==0 as a real failure (BOOL contract). Surface via slog
		// for dev visibility; logging.Setup has not run yet, so this
		// falls through slog's default handler to stderr.
		slog.Warn("FreeConsole failed", "err", err)
		return fmt.Errorf("FreeConsole: %w", err)
	}
	return nil
}
