---
phase: 25-linux-watcher
plan: 01
subsystem: watcher (build seam — tray, entrypoint, config/logging paths)
tags: [linux, cgo-free, build-tags, systray-exclusion, xdg, sigterm, headless]
requires: []
provides:
  - "linux/amd64 CGO-free compile closure for ./cmd/squirebot (zero fyne.io/systray)"
  - "headless *tray.Controller (!windows) with the identical exported API as the Windows tray"
  - "build-tag-split blocking main loop (runMainLoop) with a mandatory SIGINT/SIGTERM->cancel() handler on Linux"
  - "XDG config (~/.config/squirebot) + XDG state-logs (~/.local/state/squirebot) on Linux"
affects:
  - "internal/tray (now build-tag-split)"
  - "cmd/squirebot/main.go (systray-free; tail extracted to run_*.go)"
  - "internal/config/config.go + internal/logging/logger.go (runtime.GOOS path branch)"
tech-stack:
  added: []   # no new dependency — D-01/D-07: keep the Linux path CGO-free + dep-clean
  patterns:
    - "//go:build windows / //go:build !windows paired files to EXCLUDE a CGO import (systray) from a platform closure — NOT a runtime.GOOS branch (which would still CGO-link)"
    - "signal.NotifyContext(ctx, SIGINT, SIGTERM) for graceful systemd-user-service stop"
    - "runtime.GOOS path branch (no import → safe as a runtime check) for XDG vs %LOCALAPPDATA%"
key-files:
  created:
    - internal/tray/tray_other.go
    - internal/tray/tray_other_test.go
    - cmd/squirebot/run_windows.go
    - cmd/squirebot/run_other.go
  modified:
    - internal/tray/tray_windows.go        # git mv of tray.go + //go:build windows (bytes otherwise unchanged)
    - internal/tray/tray_windows_test.go   # git mv of tray_test.go + //go:build windows
    - cmd/squirebot/main.go                # drop systray import; replace tail with runMainLoop(...)
    - internal/config/config.go            # defaultPath() runtime.GOOS branch
    - internal/config/config_test.go       # TestDefaultPath_XDG
    - internal/logging/logger.go           # defaultLogDir() runtime.GOOS branch
    - internal/logging/logger_test.go      # TestDefaultLogDir_XDGState + GOOS-guarded TestSetupCreatesLogDir
decisions:
  - "Reproduced the tray Controller as a CONCRETE type on both platforms (no interface) so app.RunApp(... t *tray.Controller) stays byte-identical (D-07 / anti-pattern in RESEARCH Pattern 1)."
  - "Excluded systray via build tags at BOTH import sites (tray pkg AND main.go) — a runtime.GOOS guard would not remove the import and CGO would still link (RESEARCH Pitfall 1)."
  - "run_other.go installs its own SIGINT/SIGTERM handler because system.WaitForShutdown(ctx) on !windows watches ONLY ctx.Done() and registers no OS-signal handler — without it systemd `--user stop` would SIGKILL mid-ingest (LNX-05)."
  - "config uses runtime.GOOS + os.UserConfigDir() (NOT on Windows, where it returns %AppData% Roaming, not %LOCALAPPDATA%); logs hand-roll $XDG_STATE_HOME (no stdlib UserStateDir, no new dep)."
metrics:
  duration: "~1 session"
  completed: 2026-06-06
  task-commits: 3
  files-touched: 11
---

# Phase 25 Plan 01: CGO-Free Headless Build Seam Summary

Made the watcher's `linux/amd64` compile closure CGO-free by build-tag-excluding `fyne.io/systray` from the `!windows` build at BOTH its import sites (the `internal/tray` package AND `cmd/squirebot/main.go`), gave the headless Linux tail a mandatory SIGINT/SIGTERM→`cancel()` graceful-shutdown handler, and branched config + log paths onto XDG base dirs on Linux — all additive behind `//go:build` tags / `runtime.GOOS`, with the Windows build and `go test ./...` byte-for-byte unaffected.

## What Was Built

**Task 1 — tray split (`bb8e214`)**
- `git mv internal/tray/tray.go → tray_windows.go` + `//go:build windows` (no other byte changed — the live Windows tray contract is intact, D-07/T-25-04).
- `git mv internal/tray/tray_test.go → tray_windows_test.go` + `//go:build windows` (it uses Windows-only internals `pendingSnapshot`/`isReady`/`simulateReady`/`actStatus`).
- New `internal/tray/tray_other.go` (`//go:build !windows`): reproduces the EXACT exported surface (`MenuItem`, `MenuPlan`, `Health`/`HealthGreen`/`HealthRed`, `Config`, `Controller`, `NewController`, `OnReady`, `OnExit`, `SetStatus`, `SetIconHealth`, `LogDir`, the five `Label*` consts) as slog no-ops; imports only `log/slog`; NO systray.
- New `internal/tray/tray_other_test.go` (`//go:build !windows`): asserts MenuPlan order, `NewController(Config{LogDir:"/x"}).LogDir()=="/x"`, and that the no-op mutators never panic.

**Task 2 — main.go systray tail split (`e84c6c4`)**
- `main.go` drops `import "fyne.io/systray"`; the inline shutdown-listener goroutine + `systray.Run` tail are replaced by a single `runMainLoop(ctx, cancel, trayCtl)` call. Doc/comment references to `systray.` were reworded so the grep gates are clean.
- `run_windows.go` (`//go:build windows`): the named-event listener + `systray.Run` tail, behavior byte-equivalent to the old main.go.
- `run_other.go` (`//go:build !windows`): `sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)`; a goroutine drives `cancel()` on signal/parent-cancel; then `<-ctx.Done()` blocks. No systray import (LNX-05).

**Task 3 — XDG paths (`52b96c6`)**
- `config.defaultPath()`: `runtime.GOOS == "windows"` → `%LOCALAPPDATA%\SquireBot\config.json` (unchanged); else `os.UserConfigDir()/squirebot/config.json` (XDG, lowercase), with `~/.config` fallback on error (T-25-03).
- `logging.defaultLogDir()` (new helper, called from `Setup()`): Windows `%LOCALAPPDATA%\SquireBot` (unchanged); else `$XDG_STATE_HOME/squirebot` with `~/.local/state/squirebot` fallback (hand-rolled, no new dep).
- New `TestDefaultPath_XDG` + `TestDefaultLogDir_XDGState` (both env branches); the existing `TestSetupCreatesLogDir` is now `runtime.GOOS`-guarded so it asserts the right default per platform.

## Verification Results

| Gate | Result |
|------|--------|
| `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` | PASS (whole module) |
| `go list -deps ./cmd/squirebot` (linux) `grep -c fyne.io/systray` | **0** |
| `go list -deps ./cmd/squirebot` (linux) `grep -c sqweek/dialog` | **0** |
| `grep -nE 'SIGTERM|signal.Notify' cmd/squirebot/run_other.go` | matches (LNX-05) |
| `grep -n 'signal.Notify' cmd/squirebot/run_windows.go` | none (Windows unchanged) |
| `go.mod` still requires `fyne.io/systray` | yes (Windows build uses it — correct) |
| `CGO_ENABLED=0 GOOS=windows go build ./cmd/squirebot` | PASS |
| `go test ./...` (Windows host) | PASS — 0 failures across the module |
| `GOOS=linux go vet ./...` | PASS (exit 0) |

## Deviations from Plan

None — the plan executed exactly as written. The only adjustment was rewording three comment lines (the package doc + two inline comments) that literally contained `systray.`/`fyne.io/systray` so the grep acceptance gates return cleanly; no behavioral change.

## Threat Mitigations Applied

- **T-25-01 (Tampering — build closure):** systray excluded via `//go:build`, proven by the `grep -c == 0` closure gate (not a runtime guard).
- **T-25-04 (Elevation — Windows tray regression):** `tray_windows.go` is a byte-for-byte rename; `go test ./internal/tray` passes on Windows.
- **T-25-10 (DoS — SIGTERM→SIGKILL):** `run_other.go` installs the mandatory `signal.NotifyContext(SIGINT,SIGTERM)` handler driving `cancel()`.
- **T-25-03 (DoS — UserConfigDir error):** `defaultPath()` falls back to `~/.config` instead of panicking.

## No Stubs / No New Threat Surface

This plan creates no UI-facing stubs and introduces no new network/auth/file-trust surface beyond the threat model (the XDG file paths are per-user `$HOME` writes already covered by T-25-02). No `## Known Stubs` or `## Threat Flags` needed.

## For the Next Plan (25-02)

The build seam is now CGO-free, so 25-02 can add the Linux runtime impls behind the same `//go:build !windows` idiom: the `0600`-file credstore (`store_windows.go`/`store_other.go` split), the WINE-prefix `eqfind` walk (`heuristic_other.go` + the `defaultHeuristicScan`/`defaultKnownPaths` GOOS-guard relax — RESEARCH Pitfall 2), and the CLI `--setup`/`--status` onboarding (`dialog_other.go` stdin prompts). `tray.Controller` and `RunApp` are untouched by that work.

## Self-Check: PASSED

All 4 created files + 4 modified files verified present on disk; all 3 task commits (`bb8e214`, `e84c6c4`, `52b96c6`) verified in git history.
