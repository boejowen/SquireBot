//go:build windows

package main

import (
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
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole = kernel32.NewProc("FreeConsole")
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
// Returns nil unconditionally — safe to call regardless of console state.
// A no-console process (GUI-subsystem / Explorer double-click launch) is the
// common benign case: FreeConsole returns BOOL 0 with a benign LastError, which
// this function logs at Debug, not Warn (logging.Setup has not yet run, so the
// record falls through slog's default handler to stderr — Debug keeps it out of
// the way on every GUI launch). A genuine detach failure (a console WAS attached
// but could not be released) is likewise non-fatal: it is informational only and
// the watcher continues regardless, so it does not surface as an error return.
//
// FreeConsole returns a BOOL (non-zero = success). On a ret==0 result the syscall
// also surfaces a Win32 LastError via the LazyProc.Call third return. Per MSDN,
// GetLastError after a no-console call is benign (ERROR_INVALID_HANDLE on
// processes that never had a console), so ret==0 is treated as the benign
// no-console case rather than a hard failure — matching this function's
// "returns nil unconditionally / safe to call unconditionally" contract.
func freeConsole() error {
	ret, _, err := procFreeConsole.Call()
	if ret == 0 {
		// ret==0 is the benign no-console case on a GUI/Explorer launch (and a
		// genuine detach failure is non-fatal anyway — the watcher continues).
		// Log at Debug so it does not spam Warn on every GUI launch; logging.Setup
		// has not run yet, so this falls through slog's default handler to stderr.
		slog.Debug("FreeConsole returned 0 (likely no console attached); continuing", "err", err)
		return nil
	}
	return nil
}
