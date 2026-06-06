// Package onboarding provides the v2.0 watcher's first-run onboarding controls
// (CONTEXT D-3, FORK 1 Option A, USER-LOCKED): a native Windows text-input dialog
// that collects the one-line guild code, plus the relocated sqweek folder dialog
// that collects the EQ folder. There is NO browser and NO loopback HTTP listener
// anywhere in this package — that localhost surface is exactly what the Phase 13
// deletion removes (WATCH-09; RESEARCH Pitfall 3). The native controls are the
// only UI.
//
// The package is split by build tag so the dev box (Windows) gets the real Win32
// dialog while `go build ./...` / `go test ./...` still compile on a non-Windows
// CI runner:
//
//	dialog.go          - this file: the shared error vars + package doc (all platforms).
//	dialog_windows.go  - //go:build windows  : the Win32 input dialog + sqweek folder dialog.
//	dialog_other.go    - //go:build !windows : CLI stdin prompts (Phase 25, D-02 headless Linux).
//
// The function signatures are declared per-platform (in the _windows / _other
// files), not here:
//
//	func PromptGuildCode(title, prompt string) (string, error)
//	func PickEQFolder(title string) (string, error)
//
// CONTRACT NOTE: this package is purely "show the native control + return the
// value". It does NOT validate the guild code against the backend and does NOT
// run eqfind.ValidateFolder — the CALLER (Plan 03's onboarding flow) does both
// (it POSTs to /api/v1/whoami to validate the code, and re-prompts the EQ folder
// with the verbatim "doesn't look like an EverQuest install" message on a failed
// ValidateFolder). Keeping validation out of here makes the package a thin,
// testable UI layer.
package onboarding

import "errors"

// ErrCancelled is returned when the user dismisses a dialog without confirming
// (clicks Cancel, presses Esc, or closes the window). The caller treats it as
// "the guildie backed out" — not an error to log loudly, just a re-prompt or
// abort signal.
var ErrCancelled = errors.New("onboarding: user cancelled")

// ErrUnsupported was historically returned by the non-Windows stubs. As of
// Phase 25 (LNX-04) the !windows path is a real CLI stdin flow and no longer
// returns it; the sentinel is retained (distinct from ErrCancelled) for
// back-compat and in case a future platform needs an "unsupported" branch.
var ErrUnsupported = errors.New("onboarding: native dialog unsupported on this platform")
