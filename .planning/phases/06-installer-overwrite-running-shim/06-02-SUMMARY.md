---
phase: 06-installer-overwrite-running-shim
plan: 02
subsystem: cmd/squirebot
tags: [cli-flag, shutdown, listener-goroutine, windows, inst-06]
requirements: [INST-06]
status: complete
completed: 2026-05-11
dependency-graph:
  requires:
    - internal/system (Plan 06-01 — SignalShutdown, WaitForShutdown)
    - cmd/squirebot/main.go existing flag-handler + tray-wiring scaffolding
  provides:
    - "squirebot.exe --quit CLI contract for the NSIS pre-install shim (Plan 06-03)"
    - "named-event listener goroutine that funnels through cancel() + systray.Quit()"
  affects:
    - cmd/squirebot/main.go
tech-stack:
  added: []
  patterns:
    - "--uninstall-wipe-credentials structural template (placement BEFORE update.Apply, stderr-only logging, os.Exit(0) on every branch)"
    - "Listener goroutine select over <-system.WaitForShutdown(ctx) and <-ctx.Done() (cannot leak on tray-driven shutdown)"
key-files:
  created: []
  modified:
    - cmd/squirebot/main.go
decisions:
  - "Followed CONTEXT.md D-01: --quit handler placed BEFORE update.Apply()"
  - "Followed CONTEXT.md D-03: listener calls cancel() + systray.Quit() directly — no os.Exit, no drain coordination"
metrics:
  duration: ~10 min
  tasks_completed: 2
  files_modified: 1
  commits: 2
---

# Phase 6 Plan 02: Main.go Wiring Summary

One-liner: Wires `internal/system.SignalShutdown` and `WaitForShutdown` into `cmd/squirebot/main.go` via a new `--quit` CLI flag handler and a named-event listener goroutine, completing the Go-side half of the INST-06 graceful-shutdown contract.

## Insertion Points (Final Line Numbers, Post-Insert)

| Insertion | Lines (final) | Notes |
| --------- | ------------- | ----- |
| `system` import | line 21 (between `logging` line 20 and `tray` line 22) | Alphabetical order preserved: app, auth, config, logging, system, tray, update |
| `--quit` handler block | lines 57-76 | Immediately after `--uninstall-wipe-credentials` block (lines 38-55), BEFORE `update.Apply()` block (lines 79+) |
| Listener goroutine | lines 169-193 | Immediately after `go app.RunApp(ctx, cfg, bc, trayCtl)` (line 167), BEFORE `slog.Info("squirebot starting", ...)` (line 195) |

Total inserted lines: ~48 (22 in Task 1 commit, 26 in Task 2 commit). Total final file length: 214 lines (was 166).

## Unchanged Code (Verified)

- `--uninstall-wipe-credentials` block (lines 38-55) — untouched.
- Auto-update block `update.Apply()` (lines ~79-99) — untouched.
- `logging.Setup()` call (line ~101) — untouched.
- `OnQuit: func() { ... cancel() ... }` callback (line ~161-164) — untouched. Still only calls `cancel()` because `internal/tray/tray.go:234` does `systray.Quit()` itself after the callback. The new listener fires from OUTSIDE that click handler, so it MUST do both.
- `systray.Run(trayCtl.OnReady, trayCtl.OnExit)` (line ~209) — untouched. This is what `systray.Quit()` from the listener unblocks.
- `defer cancel()` (line ~118) — untouched. Listener's `cancel()` is a second invocation; `context.CancelFunc` is documented safe to call multiple times.

## Verification Results

- `go build ./cmd/squirebot` — exit 0
- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `go test ./internal/system/...` — PASS (existing 13 tests from Plan 06-01)
- Acceptance-criteria greps (Task 1 + Task 2) — all matched at expected line counts
- Insertion order check — verified: `os.Args[1] == "--uninstall-wipe-credentials"` (line 38) < `os.Args[1] == "--quit"` (line 69) < `update.Apply()` (line 91); `go app.RunApp(` (line 167) < `system.WaitForShutdown(ctx)` (line 183) < `slog.Info("squirebot starting"` (line 195)
- No `slog` in the `--quit` handler block (stderr only, runs before `logging.Setup`)
- No `os.Exit` near the listener goroutine (funnels through `cancel() + systray.Quit()` per D-03)

## Manual Smoke

Performed locally on Windows (this dev machine):

- `go build -o dist/squirebot-dev.exe ./cmd/squirebot` — clean build
- `./dist/squirebot-dev.exe --quit` (no listener present) — exited 0 with `shutdown signal sent` on stderr in <1s ✓ (matches the no-listener branch: `OpenEvent` returns `ERROR_FILE_NOT_FOUND`, fallback `CreateEvent` + `SetEvent` succeeds, exits 0)

**Deferred to integration UAT** (requires a real Windows user session with the OAuth-completed config + green-tray watcher):
- Full round-trip: launch `squirebot-dev.exe` to green-tray steady state in one terminal, run `squirebot-dev.exe --quit` in another, confirm tray icon disappears and process exits cleanly within ~1s.
- This will be exercised end-to-end by Plan 06-03's NSIS shim during the v1.0.0 → v1.0.1 upgrade UAT on a clean Win11 VM.

## CLI Contract Plan 06-03 Will Invoke

Plan 06-03 (NSIS pre-install shim) can now safely invoke:

```nsis
ExecWait '"$INSTDIR\squirebot.exe" --quit'
```

Contract:
- Exits 0 within ~1 second on every branch (success, OpenEvent failure, CreateEvent fallback failure, SetEvent failure — see lines 70-75 of main.go).
- Idempotent: a second `--quit` invocation while the first is propagating is harmless (named event is signaled; `cancel()` and `systray.Quit()` are idempotent in the running watcher).
- Safe with no listener: spawns no tray, no wizard, no goroutines; just calls `system.SignalShutdown()` and exits.
- Safe against v1.0.0 binary: NOT (v1.0.0 doesn't recognize `--quit` and would spawn a duplicate tray). Plan 06-03 honors CONTEXT.md D-02 and version-gates this invocation on `DisplayVersion >= "1.0.1"`.

## Decisions Honored

- **D-01 (named-event mechanism):** `system.SignalShutdown` opens (or creates) `Local\SquireBot-Shutdown`, sets it, exits 0. Listener funnels through the canonical `cancel() + systray.Quit()` path.
- **D-03 (abandon in-flight writes, no drain):** Listener immediately calls `cancel()` + `systray.Quit()` on signal — no wait-for-pending-writes loop, no shutdown-coordination channel. In-flight `batchUpdate` calls observe ctx cancellation and abandon; WATCH-09 catch-up re-uploads on next launch.
- **D-04 (post-install relaunch unchanged):** Confirmed — Plan 06-02 does not touch the NSIS `installer/squirebot.nsi` file at all. Post-install `Exec` line stays as-is (will be verified in Plan 06-03's edits to the .nsi).

## Threat Model — Mitigations Applied

- **T-06-11 (Listener leak on double-fire):** Mitigated. Listener's `select` exits on whichever arm fires first; `<-ctx.Done()` arm guarantees the goroutine exits when shutdown comes from another path.
- **T-06-20 (Listener fires before systray.Run binds):** Accepted per plan. `cancel()` is the primary shutdown trigger and unaffected by systray state; if `systray.Quit()` is a pre-Run no-op in fyne.io/systray v1.10.0, the cancelled context propagates through `app.RunApp` and main unwinds on the natural path. No code change required; documented for future investigation if v1.0.1 soak surfaces a hang.

## Deviations from Plan

None — plan executed exactly as written. Both insertions match the verbatim code blocks in the plan's `<action>` sections.

## Deferred Issues

None.

## Self-Check: PASSED

- File modified — FOUND: `cmd/squirebot/main.go` (now 214 lines)
- Commit Task 1 — FOUND: `5256382` (feat(06-02): add --quit CLI flag handler and system import)
- Commit Task 2 — FOUND: `a36e72f` (feat(06-02): add named-event shutdown listener goroutine)
- Build clean — VERIFIED: `go build ./...` exit 0, `go vet ./...` exit 0
- Tests pass — VERIFIED: `go test ./internal/system/...` PASS
- Smoke (no-listener path) — VERIFIED: `./dist/squirebot-dev.exe --quit` → "shutdown signal sent" + exit 0 in <1s
