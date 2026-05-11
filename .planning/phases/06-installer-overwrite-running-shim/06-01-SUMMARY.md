---
phase: 06-installer-overwrite-running-shim
plan: 01
subsystem: watcher-system-ipc
tags: [windows, named-event, shutdown, ipc, inst-06, build-tag-pair]
requires:
  - none (new package; first direct consumer of bare golang.org/x/sys/windows in this repo — eqfind uses windows/registry only)
provides:
  - "SignalShutdown() error — opens-or-creates Local\\SquireBot-Shutdown and SetEvents it (OpenEvent then CreateEvent-on-ERROR_FILE_NOT_FOUND fallback)"
  - "WaitForShutdown(ctx context.Context) <-chan struct{} — listener with ctx-cancel watchdog; defer-closed handle, no leaks"
  - "Non-Windows stubs so cmd/squirebot/main.go compiles on linux/darwin for go vet ./... and CI cross-compile sanity"
  - "Package-internal eventName var test seam — parallel go test does not collide on the production event name"
affects:
  - "Phase 6 Plan 02 (cmd/squirebot/main.go --quit handler + listener goroutine consume these two functions verbatim; signatures locked here)"
  - "Phase 6 Plan 03 (installer/squirebot.nsi pre-install shim ExecWaits squirebot.exe --quit; downstream of Plan 02 which is downstream of this plan)"
tech-stack:
  added: []
  patterns:
    - "Paired build-tag files (//go:build windows + //go:build !windows) mirroring internal/eqfind/registry_*.go — compile-time platform gating, no runtime runtime.GOOS branches."
    - "Two-stage event-handle acquisition for fire-and-forget signaling: OpenEvent first (listener already created the event — common case), CreateEvent fallback on ERROR_FILE_NOT_FOUND (no listener active — benign signal). Manual-reset (CreateEvent arg 2 = 1) so a listener that started before the signal observes the signaled state immediately on WaitForSingleObject."
    - "Ctx-cancel watchdog goroutine that SetEvents the handle on ctx.Done — Windows has no native 'wait on event OR context' primitive, so we synthesize cancellation by signaling our own handle, which lets the WaitForSingleObject goroutine return through its defer-CloseHandle + defer-close(done) path."
    - "Error-wrap style fmt.Errorf(\"API_NAME: %w\", err) for every Windows-API call site (UTF16PtrFromString, OpenEvent, CreateEvent, SetEvent) — matches internal/update/swap.go house style."
    - "No slog imports — the signal side runs from `squirebot.exe --quit` BEFORE logging.Setup per Plan 02; stderr-only logging belongs at the caller (cmd/squirebot/main.go), never inside this package."
    - "Package-level eventName var (not const) as a test seam — uniqueEventName() helper mirrors internal/auth/store_test.go's uniqueEmail pattern (time.Now().UnixNano() suffix); withEventName(t, name) restores via t.Cleanup."
key-files:
  created:
    - internal/system/doc.go (13 lines)
    - internal/system/shutdown_signal_other.go (22 lines)
    - internal/system/shutdown_signal_windows.go (105 lines)
    - internal/system/shutdown_signal_windows_test.go (128 lines)
    - .planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md (this file)
  modified: []
decisions:
  - "Locked the SignalShutdown() and WaitForShutdown(ctx) signatures EXACTLY as specified in the plan's <interfaces> block — Plan 02 will write main.go against these verbatim. No deviations, no signature drift."
  - "Plan 02's NSIS-plugin-vs-System::Call question (CONTEXT D-05) was NOT touched here — this plan is Go-only. The choice belongs in Plan 03's SUMMARY when the NSIS poll loop ships."
  - "Tests are Windows-only (//go:build windows on the _test.go file). Cross-compile sanity for non-Windows is covered by go build of the package + the stub's identical exported signatures. No mock layer is introduced; the simplest path is to gate the tests with the build tag (matching internal/eqfind/ which has no test file at all)."
  - "The 5th test (TestListenerCreatedAfterSignalDoesNotFire) is intentional and documents the actual semantic boundary of 'fire-and-forget': SignalShutdown does not error when no listener is active, but a listener attaching LATER does NOT retroactively observe the lost signal (the kernel event is destroyed when SignalShutdown's eager CloseHandle runs and no other handle exists). The NSIS shim sidesteps this by only signaling when squirebot.exe is detected running — listener guaranteed active."
metrics:
  duration: ~15min (2 tasks executed sequentially in single agent run)
  completed: 2026-05-11
  tasks_completed: 2 of 2
  commits: 1 (a705f4e feat package + tests)
  files_changed: 4 created, 0 modified (~270 lines added)
  tests_added: 5 (TestSignalUnblocksWait, TestSignalWithNoListenerIsNoOp, TestCtxCancelClosesChannel, TestSignalIsIdempotent, TestListenerCreatedAfterSignalDoesNotFire)
  go_mod_changes: 0 (golang.org/x/sys v0.43.0 already direct dep)
  watcher_rebuild_required: yes-eventually (Plan 02 wires this into main.go; the v1.0.1 ship binary will include both)
---

# Phase 6 Plan 01: Shutdown-Signal Package Summary

**One-liner:** Shipped INST-06 Go-side primitive — new `internal/system` package with paired build-tag files providing `SignalShutdown() error` (named-event signaler) and `WaitForShutdown(ctx) <-chan struct{}` (listener with ctx-cancel watchdog) over a `Local\SquireBot-Shutdown` Windows named event; 5 Windows-only tests pass, no `go.mod` changes, signatures locked for Plan 02 to consume verbatim.

## What shipped

### Task 1 — Package doc + non-Windows stubs (subset of commit `a705f4e`)

Two files lay the cross-platform compile groundwork:

- **`internal/system/doc.go`** (no build tag, 13 lines): the package-level doc comment. Documents both exported symbols at the package boundary so `go doc github.com/boejowen/SquireBot/internal/system` returns useful output on any platform. Calls out the `Local\` namespace rationale (per-session vs. `Global\`).
- **`internal/system/shutdown_signal_other.go`** (`//go:build !windows`, 22 lines): no-op stubs matching the `internal/eqfind/registry_other.go` 7-line pattern. `SignalShutdown()` returns nil; `WaitForShutdown(ctx)` returns a real channel (NOT nil — must be a usable receive operand in `select`) that closes when ctx fires. Function doc comments explicitly cross-reference the Windows counterpart file for grep-discoverability.

`go vet ./internal/system/...` exits 0.

### Task 2 — Windows implementation + 5 tests (subset of commit `a705f4e`)

Two files complete the package:

- **`internal/system/shutdown_signal_windows.go`** (`//go:build windows`, 105 lines):
  - `var eventName = `Local\SquireBot-Shutdown`` — package-level var, not const, so tests can override it without collision. Comment explains the test-seam rationale.
  - **`SignalShutdown()`** — `UTF16PtrFromString(eventName)` → `OpenEvent(EVENT_ALL_ACCESS, ...)`; on `ERROR_FILE_NOT_FOUND` (no listener) → `CreateEvent(nil, manualReset=1, initialState=0, ...)`; `defer CloseHandle`; `SetEvent`. Every API call wraps with `fmt.Errorf("API_NAME: %w", err)`. No slog imports.
  - **`WaitForShutdown(ctx)`** — creates a `done` channel, opens (`CreateEvent`) the handle, spawns a ctx-cancel watchdog goroutine that `SetEvent`s the handle on `<-ctx.Done()`, then spawns the listener goroutine that runs `defer CloseHandle` + `defer close(done)` + `WaitForSingleObject(handle, INFINITE)`. Handle is closed exactly once.

- **`internal/system/shutdown_signal_windows_test.go`** (`//go:build windows`, 128 lines):
  - `uniqueEventName(name)` returns `Local\SquireBot-Shutdown-test-<name>-<unix-nano>` — mirrors `internal/auth/store_test.go`'s `uniqueEmail` helper.
  - `withEventName(t, name)` overrides the package-level var, restores via `t.Cleanup`.
  - **`TestSignalUnblocksWait`** — listener attaches, 50ms sleep (so the listener-already-active OpenEvent path of SignalShutdown is exercised), signal, channel closes within 500ms. The round-trip happy path.
  - **`TestSignalWithNoListenerIsNoOp`** — SignalShutdown with no listener returns nil via the CreateEvent fallback path.
  - **`TestCtxCancelClosesChannel`** — listener attached but no signal; cancel ctx; channel closes within 500ms via the watchdog → SetEvent → WaitForSingleObject-returns → defer-close chain. No goroutine leak.
  - **`TestSignalIsIdempotent`** — two consecutive SignalShutdowns both return nil.
  - **`TestListenerCreatedAfterSignalDoesNotFire`** — signal first, sleep 50ms (kernel event tears down because no other handle exists after SignalShutdown's defer CloseHandle), then attach a listener with a 200ms ctx timeout. Channel must NOT close before ctx expires — proves the lost-signal-is-actually-lost semantics. The test only treats the close as a pass if `ctx.Err() != nil` at the moment of close (channel closed via the watchdog path).

`go test ./internal/system/... -count=1 -v` reports `--- PASS` on all 5 tests in 1.6s. `go vet ./internal/system/...` exits 0. `GOOS=linux go build ./internal/system/...` exits 0 (stub cross-compiles).

## The signatures Plan 02 will consume

```go
package system

// Windows-only real impl; non-Windows stub returns nil.
func SignalShutdown() error

// Windows-only real impl spawns 2 goroutines; non-Windows stub spawns 1 (ctx.Done watcher).
func WaitForShutdown(ctx context.Context) <-chan struct{}
```

The `// Plan 06 (INST-06): --quit` block in `cmd/squirebot/main.go` will:

```go
if len(os.Args) >= 2 && os.Args[1] == "--quit" {
    if err := system.SignalShutdown(); err != nil {
        fmt.Fprintf(os.Stderr, "shutdown signal failed: %v\n", err)
        os.Exit(0)
    }
    fmt.Fprintln(os.Stderr, "shutdown signal sent")
    os.Exit(0)
}
```

The normal-launch listener goroutine in `cmd/squirebot/main.go` will:

```go
go func() {
    select {
    case <-system.WaitForShutdown(ctx):
        slog.Info("shutdown signal received — cancelling root context")
        cancel()
        systray.Quit()
    case <-ctx.Done():
        return
    }
}()
```

Both snippets are from `06-PATTERNS.md` and are reproduced here so Plan 02's executor does not re-derive them.

## Threat-register coverage

All 5 STRIDE items from the plan's `<threat_model>` are addressed:

- **T-06-01 (Spoofing — any user-session process can SetEvent)**: ACCEPT. `Local\` namespace bounds the surface to the user's logon session; an attacker who can run code in the user's session already trumps SquireBot's security model. WATCH-09 catch-up re-uploads after restart so the worst-case impact is a brief tray disappearance.
- **T-06-02 (Tampering — eventName test seam)**: MITIGATE. Lowercase `eventName` is unexported; only the in-package `withEventName(t, name)` helper overrides it; no external input feeds in.
- **T-06-03 (DoS — signal flood)**: ACCEPT. `SetEvent` on a manual-reset event is O(1); the listener processes one signal and exits. Subsequent SetEvents on the now-closed handle no-op.
- **T-06-04 (Info Disclosure — leaked handle)**: MITIGATE. Both goroutines `defer windows.CloseHandle(handle)`; the ctx-cancel watchdog ensures the WaitForSingleObject goroutine returns. `TestCtxCancelClosesChannel` is the regression check — a leaked handle would manifest as a hung listener goroutine and the 500ms timeout would fail the test.
- **T-06-05 (EoP — privileged invocation)**: ACCEPT. `EVENT_ALL_ACCESS` carries no privileges; the watcher runs at user integrity per INST-01.

## Deviations from Plan

None. Plan executed exactly as written.

The implementation file matches the `<action>` code block verbatim; the test file matches the `<action>` test block verbatim; the `<interfaces>` signatures are honored without drift. All acceptance criteria greps pass:

- `var eventName = `Local\SquireBot-Shutdown`` — 1 match.
- `golang.org/x/sys/windows` import — 1 match.
- `windows.(CreateEvent|OpenEvent|SetEvent|WaitForSingleObject|CloseHandle)` — 8 call sites across the 5 APIs.
- `fmt.Errorf("(UTF16PtrFromString|OpenEvent|CreateEvent|SetEvent): %w"` — 4 matches.
- `slog\.` — 0 matches.

## Notes for v1.0.1 SUMMARY (deferred until Phase 6 plans 02+03 ship)

- **NSIS plugin standardization (CONTEXT D-05 + deferred_ideas)**: still open. This plan made no NSIS decision; the choice between `nsProcess` plugin and `System::Call OpenProcess+WaitForSingleObject` is for Plan 03's executor when the poll loop ships. Plan 03's SUMMARY should record the choice here so future installers don't re-litigate.
- **Single-instance enforcement (deferred_ideas)**: the named-event infrastructure landed here makes it cheap (a named mutex at startup) but it remains an explicit non-goal for v1.0.1 per the deferred-ideas table.

## Verification log

```
$ go vet ./internal/system/...
(exit 0)

$ go test ./internal/system/... -count=1 -v
=== RUN   TestSignalUnblocksWait
--- PASS: TestSignalUnblocksWait (0.05s)
=== RUN   TestSignalWithNoListenerIsNoOp
--- PASS: TestSignalWithNoListenerIsNoOp (0.00s)
=== RUN   TestCtxCancelClosesChannel
--- PASS: TestCtxCancelClosesChannel (0.00s)
=== RUN   TestSignalIsIdempotent
--- PASS: TestSignalIsIdempotent (0.00s)
=== RUN   TestListenerCreatedAfterSignalDoesNotFire
--- PASS: TestListenerCreatedAfterSignalDoesNotFire (0.28s)
PASS
ok  	github.com/boejowen/SquireBot/internal/system	1.618s

$ GOOS=linux go build ./internal/system/...
(exit 0 — stub cross-compiles)

$ git diff --diff-filter=D --name-only HEAD~1 HEAD
(empty — no unexpected deletions)
```

## Self-Check: PASSED

**Files exist (all 4 new + this SUMMARY):**

- FOUND: `internal/system/doc.go` (package doc, no build tag)
- FOUND: `internal/system/shutdown_signal_other.go` (`//go:build !windows`, 22 lines)
- FOUND: `internal/system/shutdown_signal_windows.go` (`//go:build windows`, 105 lines, exports SignalShutdown + WaitForShutdown)
- FOUND: `internal/system/shutdown_signal_windows_test.go` (`//go:build windows`, 128 lines, 5 tests)
- FOUND: `.planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md` (this file)

**Commits exist:**

- FOUND: `a705f4e` — feat(06-01): add internal/system shutdown-signal package (INST-06)

All claims in this SUMMARY are verifiable via the commands in the verification log above.

## Next plan

Phase 6 Plan 02 wires `system.SignalShutdown` + `system.WaitForShutdown` into `cmd/squirebot/main.go`:

- New `--quit` flag-handler block immediately after the existing `--uninstall-wipe-credentials` block (main.go:38-54), BEFORE `update.Apply()`.
- Listener goroutine spawned after `RunApp` launch (around main.go:145), funnels through the existing `cancel()` + `systray.Quit()` shutdown path.
- The signatures locked in this plan are honored verbatim — Plan 02's executor must not modify the package.

Plan 03 then writes the NSIS pre-install shim that drives the chain: read `DisplayVersion`, version-gate `1.0.1+`, `ExecWait '"$INSTDIR\squirebot.exe" --quit'`, poll for exit (10s cap), `taskkill /F` fallback.
