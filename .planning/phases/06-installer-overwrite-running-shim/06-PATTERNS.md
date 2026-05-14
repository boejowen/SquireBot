# Phase 6: Installer Overwrite-Running Shim — Pattern Map

**Mapped:** 2026-05-11
**Files analyzed:** 7 (2 new Go, 2 modified Go/NSIS, 2 docs, 1 release artifact)
**Analogs found:** 6/7 (release-trigger artifact is config-driven, no analog needed)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| NEW `internal/system/shutdown_signal.go` | system primitive (Windows-only) | event-driven (named-event signal) | `internal/eqfind/registry_windows.go` | role-match (build-tag + `golang.org/x/sys/windows` consumer; different sub-API) |
| NEW `internal/system/shutdown_signal_other.go` (stub) | system primitive (non-Windows stub) | n/a | `internal/eqfind/registry_other.go` | exact (build-tag stub pattern) |
| NEW `internal/system/shutdown_signal_test.go` | test (Windows-only) | round-trip test | `internal/auth/store_test.go` | role-match (build-tagged Windows-only test) |
| MODIFY `cmd/squirebot/main.go` (`--quit` handler) | CLI flag handler | request-response (CLI invoke → exit code) | same file lines 38-54 (`--uninstall-wipe-credentials`) | exact (structural template) |
| MODIFY `cmd/squirebot/main.go` (listener goroutine) | event-driven goroutine | event-driven (block on event → invoke `cancel()`+`systray.Quit()`) | same file lines 138-141 (`OnQuit` callback) | exact (same shutdown funnel) |
| MODIFY `installer/squirebot.nsi` (pre-install shim) | installer script | request-response (read regkey → ExecWait → poll → fallback) | same file lines 108-136 (uninstaller `--uninstall-wipe-credentials` + `taskkill /F`) | exact (same NSIS primitives: `ExecWait`, `IfFileExists`, `StrCmp`, registry reads) |
| MODIFY `docs/troubleshooting.md` (delete lines 50-58) | docs (deletion) | n/a | n/a (pure deletion) | n/a |
| MODIFY `docs/build-and-install.md` (optional `--quit` note) | docs (additive) | n/a | existing docs structure | n/a |

## Pattern Assignments

---

### NEW `internal/system/shutdown_signal.go` (system primitive, Windows-only)

**Analog:** `internal/eqfind/registry_windows.go` (only existing `golang.org/x/sys/windows` consumer in the codebase). Note this analog uses `.../windows/registry` not the bare `windows` package; the new file will be the FIRST direct consumer of `golang.org/x/sys/windows` for `CreateEventW`/`OpenEventW`/`SetEvent`/`WaitForSingleObject`. The build-tag header + package layout, however, is identical.

**Build tag + package doc pattern** (analog lines 1-9):
```go
//go:build windows

package eqfind

import (
	"regexp"

	"golang.org/x/sys/windows/registry"
)
```

Apply verbatim to new file (substitute `package system` + the `windows` import — likely `"golang.org/x/sys/windows"` for `EventW`/`SetEvent`/`WaitForSingleObject`/`CloseHandle`). Confirm `golang.org/x/sys v0.43.0` (already in `go.mod` line 12) exposes those symbols — no new `go get` needed.

**Function-doc + error-wrap style** (analog `registry_windows.go` lines 11-23 and `internal/update/swap.go` lines 56-69):
```go
// Apply checks for a staged update adjacent to the running binary and,
// if present + valid, performs the startup-swap via minio/selfupdate.
//
// Returns:
//   - (true, nil)  -> swap succeeded; caller MUST os.Exit(0).
//   - (false, nil) -> no staged file (common cold-start path).
//   - (false, err) -> any failure mode; caller logs to stderr (logging
//     not yet set up at this point in main) and continues running the
//     OLD binary.
```

The new file's two exported functions (`WaitForShutdown(ctx) <-chan struct{}` and `SignalShutdown() error`) MUST carry the same doc style: explicit return-value table + WHO calls them + WHEN. Wrap raw Windows-API errors with `fmt.Errorf("CreateEventW: %w", err)` (matches swap.go lines 73, 94, 103, 109).

**Logging note:** `SignalShutdown()` is called from `--quit` handler which runs BEFORE `logging.Setup` — same constraint as `update.Apply()`. Use `fmt.Fprintf(os.Stderr, ...)` for any signaler-side failure, NOT `slog`. Mirror swap.go's comment block lines 66-69 verbatim in spirit.

---

### NEW `internal/system/shutdown_signal_other.go` (non-Windows stub)

**Analog:** `internal/eqfind/registry_other.go` (the entire file, 7 lines):

```go
//go:build !windows

package eqfind

// scanUninstallKeys is a no-op on non-Windows platforms. The Windows
// implementation lives in registry_windows.go.
func scanUninstallKeys() string { return "" }
```

**Apply identically.** New file stubs both exported functions:
- `SignalShutdown() error { return nil }` (no-op; CI cross-compile gate)
- `WaitForShutdown(ctx context.Context) <-chan struct{}` — return a channel that closes only when `ctx.Done()` fires (so callers' select arms still work on non-Windows builds without spurious wake-ups).

Rationale: this is a `cmd/squirebot/main.go` import; main.go has NO build tag and must compile on linux/darwin for `go vet ./...` and CI cross-compile sanity (even though the only shipped target is Windows). Same constraint that birthed `registry_other.go`.

---

### NEW `internal/system/shutdown_signal_test.go` (Windows-only test)

**Analog:** `internal/auth/store_test.go` lines 1-30:

```go
//go:build windows

package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/danieljoos/wincred"
)

// uniqueEmail returns a per-test wincred target name that cannot
// collide with a real user's credentials. example.invalid is an
// IETF-reserved TLD so it can never be a Google account.
func uniqueEmail(name string) string {
	return fmt.Sprintf("squirebot-test-%s-%d@example.invalid", name, time.Now().UnixNano())
}
```

**Apply:** prefix test file with `//go:build windows`. Use a per-test unique event name (e.g. `Local\SquireBot-Shutdown-test-<unix-nano>`) so parallel `go test ./...` runs don't collide on the same global event handle. The `uniqueEmail` helper is the prior art for "build a per-test unique name from `time.Now().UnixNano()`".

**Round-trip test shape** (analog `TestStoreAndReadRoundTrip` lines 20-30): write → read → assert; cleanup via `t.Cleanup(func(){ _ = DeleteToken(email) })`. Mirror it: `SignalShutdown()` → assert `<-WaitForShutdown(ctx)` unblocks within e.g. 500ms; cleanup closes the event handle.

**Secondary analog for goroutine-driven test:** `internal/update/swap_test.go` shows the "package-level seam" pattern (swap.go line 54: `var selfApplyFn = selfupdate.Apply`) used to mock Windows-API calls in tests. If full mocking is needed (e.g. on a non-Windows dev box for unit-test coverage), this is the template — but the simpler path is to keep tests Windows-only and gate them with the build tag (matching `internal/eqfind/registry_windows.go` which has no companion `registry_test.go` at all).

---

### MODIFY `cmd/squirebot/main.go` — `--quit` flag handler block

**Analog:** SAME FILE lines 26-54 (the `--uninstall-wipe-credentials` block). This is the structural template.

**Verbatim shape to copy** (lines 38-54):
```go
if len(os.Args) >= 2 && os.Args[1] == "--uninstall-wipe-credentials" {
    cfg, err := config.Load()
    if err != nil {
        fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
        os.Exit(0)
    }
    if cfg.GoogleEmail == "" {
        fmt.Fprintln(os.Stderr, "no email in config; nothing to wipe")
        os.Exit(0)
    }
    if err := auth.DeleteToken(cfg.GoogleEmail); err != nil {
        fmt.Fprintf(os.Stderr, "wincred delete failed for %s: %v\n", cfg.GoogleEmail, err)
        os.Exit(0)
    }
    fmt.Fprintf(os.Stderr, "wincred entry removed for %s\n", cfg.GoogleEmail)
    os.Exit(0)
}
```

**Applied pattern for `--quit`:**
```go
// Plan 06 (INST-06): --quit. Invoked by the NSIS pre-install shim to
// gracefully stop a running watcher before file overwrite. Opens the
// Local\SquireBot-Shutdown named event and signals it; the running
// instance's listener goroutine observes the signal and unwinds through
// systray.Quit() + cancel(). This invocation exits 0 always — a signal
// with no listener is a no-op, and NSIS falls back to taskkill /F on
// timeout regardless.
//
// Runs FIRST (before update.Apply) — auto-update has no business firing
// during a --quit signal invocation. Logging is not yet set up; use
// stderr for any error output (matches --uninstall-wipe-credentials and
// update.Apply's stderr-only contract).
if len(os.Args) >= 2 && os.Args[1] == "--quit" {
    if err := system.SignalShutdown(); err != nil {
        fmt.Fprintf(os.Stderr, "shutdown signal failed: %v\n", err)
        os.Exit(0)
    }
    fmt.Fprintln(os.Stderr, "shutdown signal sent")
    os.Exit(0)
}
```

**Conventions to preserve:**
- `len(os.Args) >= 2 && os.Args[1] == "--quit"` (NOT `flag.Parse()` — main.go avoids the `flag` package; consistency).
- `os.Exit(0)` on EVERY exit branch (including errors) — the existing precedent on line 50: "the uninstaller must not block on a guildie who never completed the wizard but ran the installer." Same logic here: the installer must not block on a `--quit` invocation that fails for any reason; `taskkill /F` is the fallback.
- Place this block IMMEDIATELY AFTER the `--uninstall-wipe-credentials` block (line 54) and BEFORE `update.Apply()` on line 69. Add a `system` import (new line in the import block at line 22, between `internal/logging` and `internal/tray`).

---

### MODIFY `cmd/squirebot/main.go` — named-event listener goroutine

**Analog:** SAME FILE lines 138-141 (the `OnQuit` callback) — the canonical shutdown funnel.

```go
OnQuit: func() {
    slog.Info("Quit clicked — cancelling root context")
    cancel()
},
```

Combined with `internal/tray/tray.go` line 234 (`systray.Quit()`), the full shutdown sequence is: tray-menu click → `OnQuit` → `cancel()` → tray.go calls `systray.Quit()` → `systray.Run` unblocks in main → `cancel()` (line 164, defensive double-call) → process exits.

**Applied pattern for the listener goroutine** (insert into main.go after `RunApp` launch on line 145, BEFORE the `slog.Info("squirebot starting", ...)` block):

```go
// Plan 06 (INST-06): named-event shutdown listener. Blocks on
// Local\SquireBot-Shutdown; on signal, funnels through the SAME path
// as the tray's Quit menu (cancel() + systray.Quit()). Idempotent —
// double-fire (tray Quit + installer --quit racing) is harmless because
// systray.Quit is internally idempotent and cancel() is a no-op on a
// cancelled ctx. Goroutine exits on either signal-received OR ctx.Done.
go func() {
    select {
    case <-system.WaitForShutdown(ctx):
        slog.Info("shutdown signal received — cancelling root context", "op", "system.shutdown_signal")
        cancel()
        systray.Quit()
    case <-ctx.Done():
        // Normal shutdown (tray Quit or other cancel). Goroutine exits;
        // event handle cleanup happens in WaitForShutdown's defer.
        return
    }
}()
```

**Conventions to preserve:**
- The slog op-string convention: existing calls use short verb-style messages (`"Quit clicked — cancelling root context"`, `"Reauthorize clicked — running OAuth flow"`, `"auto-update applied"`). Match that voice. The optional `"op"` structured key (`"op", "system.shutdown_signal"`) is consistent with the project-wide "structured logging both Go side (slog) and Apps Script side" rule in CLAUDE.md.
- MUST call BOTH `cancel()` AND `systray.Quit()` (the tray Quit handler at tray.go:230-234 does both implicitly — the explicit callback only does `cancel()` because tray.go does `systray.Quit()` itself after). Since our listener fires from OUTSIDE the tray's click handler, it must call `systray.Quit()` explicitly to unblock `systray.Run` on line 161.
- DO NOT call `os.Exit()` — the existing shutdown contract is "cancel() + systray.Quit() → systray.Run returns → main exits." Plumbing through that path preserves the `defer cancel()` on line 96 and the `slog.Info("squirebot exit")` on line 165.

---

### MODIFY `installer/squirebot.nsi` — pre-install shim in `Section "Install"`

**Analog A (hard-kill syntax):** SAME FILE line 136 (uninstaller `taskkill`):
```nsis
; Always: stop running instance before deleting the .exe (graceful kill).
ExecWait 'taskkill /IM "${EXE_NAME}" /F'
```

**Apply verbatim** as the fallback step inside the new pre-install block. The comment "graceful kill" is a misnomer in the uninstaller (it's actually a hard kill); the new shim should use accurate phrasing: `; Fallback: hard-kill if the watcher did not exit within the timeout.`

**Analog B (registry read pattern):** SAME FILE line 43 + line 89 (the `DisplayVersion` write that this read mirrors):
```nsis
!define REGPATH_UNINSTSUBKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"
...
WriteRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion" "${APPVERSION}"
```

**Applied read pattern** (pre-install block):
```nsis
ReadRegStr $0 HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"
; $0 is empty string if regkey missing → version-compare fails closed → skip --quit
```

**Analog C (conditional ExecWait + IfFileExists):** SAME FILE lines 129-133 (uninstaller's "binary still exists?" guard):
```nsis
StrCmp $UninstallWipe "1" 0 SkipWipeBinary
IfFileExists "$INSTDIR\${EXE_NAME}" RunWipeBinary SkipWipeBinary
RunWipeBinary:
    ExecWait '"$INSTDIR\${EXE_NAME}" --uninstall-wipe-credentials'
SkipWipeBinary:
```

**Apply directly** as the template for the version-gated `--quit` invocation. Use the same `Label:` + `Goto`/`StrCmp` jump style — do NOT introduce new control-flow primitives (NSIS `${If}` macros from `LogicLib.nsh` are not currently `!include`'d; sticking with `StrCmp`/`IfFileExists`/`Goto` keeps the file consistent).

**Sketch of the full pre-install block** (planner refines exact NSIS syntax):
```nsis
; Plan 06 (INST-06): pre-install shim. Detect running watcher, signal
; graceful exit, poll for process exit, fall back to hard kill.
;
; Version gate: v1.0.0 binaries don't recognize --quit and would spawn
; a duplicate tray. For pre-1.0.1 installs, skip --quit entirely and
; go straight to taskkill /F.
ReadRegStr $0 HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"
${VersionCompare} "$0" "1.0.1" $1
; $1 == 2 means $0 < 1.0.1 (or empty); $1 == 0 or 1 means $0 >= 1.0.1
StrCmp $1 "2" SkipQuitSignal
IfFileExists "$INSTDIR\${EXE_NAME}" RunQuitSignal SkipQuitSignal
RunQuitSignal:
    ExecWait '"$INSTDIR\${EXE_NAME}" --quit'
    ; Poll for process exit (10s cap, 250ms interval).
    ; planner picks: nsProcess::FindProcess OR System::Call OpenProcess+WaitForSingleObject
    ; [poll loop here]
SkipQuitSignal:
; Always: hard-kill fallback (no-op if process already exited).
ExecWait 'taskkill /IM "${EXE_NAME}" /F'
```

**Conventions to preserve:**
- Comment density: existing file uses 4-5 line comment blocks before each non-trivial section (lines 4-8, 16-19, 99-101). Match it.
- Reference the requirement ID in the lead comment: existing file says `; -- INST-01 (no UAC, no command-line steps) --` and `; -- INST-04 (autostart) --`. New block should lead with `; -- INST-06 (overwrite-running shim) --`.
- Insert the new block at the TOP of `Section "Install"` (line 78), BEFORE `SetOutPath "$INSTDIR"`. The pre-install detection must run before any file writes.
- `${VersionCompare}` requires `!include "WordFunc.nsh"` at the top of the file (NSIS built-in, no plugin dep). Add the include alongside the existing `!define` block (around line 42).
- DO NOT modify the `Exec '"$INSTDIR\${EXE_NAME}"'` post-install relaunch line (105) per D-04.

---

### MODIFY `docs/troubleshooting.md` — delete lines 50-58

**Analog:** None (pure deletion).

**Verification hook (ROADMAP criterion 4):** after edit, `grep -n "Installer won't overwrite a running SquireBot" docs/troubleshooting.md` must return zero matches. Also remove the section reference if anything else in the file links to it (none observed at scout time, but planner should re-grep).

---

### MODIFY `docs/build-and-install.md` — optional `--quit` note

**Analog:** existing structure of `docs/build-and-install.md` (a runbook with table-of-prereqs + procedural sections). The user-decision in CONTEXT.md called this "POSSIBLY MODIFY" — planner's call.

**Suggested addition** (short, ~3 lines, placed in a new "Manual debug aids" subsection or appended to an existing one):
```markdown
### Manual debug aids

- `squirebot.exe --quit` — signals a running watcher to shut down gracefully (Plan 06 / INST-06). Useful when iterating on a local rebuild without killing via Task Manager. No-op if no instance is running.
```

---

### NEW release-trigger artifacts (tag `v1.0.1` + GitHub Release)

**Analog:** `.github/workflows/release.yml` (already exists; configured for tag-driven release; no Phase 6 changes needed).

**Per CONTEXT.md D-06 + D-07:** the release pipeline runs automatically on `git push origin v1.0.1`. No workflow edits unless the planner discovers a missing NSIS plugin during shim implementation (which would require a `choco install nsis-plugin-X` step — STOP and surface to user per the toolchain-install guardrail in CLAUDE.md / user-memory).

**Verification:** ROADMAP criterion 5 — `dist/SquireBot-Setup-1.0.1.exe`, `dist/squirebot.exe` (v1.0.1), `dist/latest.json` (with `"version": "1.0.1"`) all attached to the GitHub Release. The release workflow already does all three steps (release.yml steps 4-6 per the workflow's lead comment).

---

## Shared Patterns

### Build-tag file naming (Windows-only code)
**Source:** `internal/eqfind/registry_windows.go` + `internal/eqfind/registry_other.go` (paired file pattern).
**Apply to:** `internal/system/shutdown_signal.go` + `internal/system/shutdown_signal_other.go` + `internal/system/shutdown_signal_test.go`.

```go
//go:build windows   // or //go:build !windows for the stub
```

The pair-file pattern (vs. a single file with conditional code) is the established project convention. Compile-time gating is cleaner than runtime `runtime.GOOS == "windows"` branches.

### Error-wrap style
**Source:** `internal/update/swap.go` (lines 73, 94, 103, 109).
**Apply to:** all new code in `internal/system/shutdown_signal.go`.

```go
return false, fmt.Errorf("os.Executable: %w", err)
return false, fmt.Errorf("read sidecar hash: %w", err)
```

Pattern: short verb-or-API-name + `%w`. Never bare `return err` from a function with meaningful context. Match this for `CreateEventW`, `OpenEventW`, `SetEvent`, `WaitForSingleObject`, `CloseHandle` wrappers.

### Stderr-only logging before `logging.Setup`
**Source:** `cmd/squirebot/main.go` lines 41, 45, 49, 52 + `internal/update/swap.go` lines 66-69 (the doc comment articulating the rule).
**Apply to:** `--quit` handler in main.go AND `SignalShutdown()` in shutdown_signal.go (both called before logging is initialized).

```go
fmt.Fprintf(os.Stderr, "shutdown signal failed: %v\n", err)
```

NEVER use `slog` from code paths that may run before `logging.Setup()` on main.go line 78. The `--quit` handler runs at line 38-ish (after this insertion); `logging.Setup` runs at line 78. The listener goroutine, by contrast, is spawned AFTER line 78 and SHOULD use `slog`.

### slog op naming
**Source:** project-wide convention (see CLAUDE.md "Conventions" §6: "Structured logging both Go side (slog) and Apps Script side (`log(level, op, fields)` JSON-encoding helper) — keeps logs greppable").
**Apply to:** the listener goroutine's slog call.

Existing examples in main.go are verb-style messages without an explicit `op` key (`slog.Info("Quit clicked — cancelling root context")`). The structured-op convention is more strictly enforced in Apps Script land. For Go, follow the in-file style: short imperative messages, structured `key, value` pairs for variable data. Suggested:
```go
slog.Info("shutdown signal received — cancelling root context")
```
(No mandatory `op` key — would be inconsistent with the surrounding code in main.go. The file path + log line is sufficient for grep.)

### Idempotent + fire-and-forget signaling
**Source:** project convention articulated in CONTEXT.md D-01 ("Named Windows event (chosen): zero new dependencies ... idempotent (signal-with-no-listener is a no-op)") AND inherited from Phase 2 BUG-001 carry-forward note ("the named-event pattern must be defensive about 'what if the signal arrives but no listener is waiting' (answer: idempotent no-op; named-event signal is fire-and-forget)").
**Apply to:** `SignalShutdown()` MUST succeed (return nil) when the event exists but no goroutine is waiting on it. Win32 `SetEvent` on an auto-reset event with no waiter sets the event to signaled; the next `WaitForSingleObject` returns immediately. This is the natural Windows-API behavior — just verify and document it.

---

## No Analog Found

| File | Role | Data Flow | Reason | Planner guidance |
|------|------|-----------|--------|------------------|
| Direct `golang.org/x/sys/windows` event-API usage (`CreateEventW`, `OpenEventW`, `SetEvent`, `WaitForSingleObject`, `CloseHandle`) | system primitive | event-driven | No existing file uses the bare `windows` package — only `windows/registry` (eqfind) and the indirect `TheTitanrain/w32` via `sqweek/dialog`. | Reference `golang.org/x/sys` v0.43.0 godoc directly (already in `go.mod` line 12). Common pattern: `windows.CreateEvent(nil, 1, 0, windows.StringToUTF16Ptr(`Local\SquireBot-Shutdown`))`; check err; `defer windows.CloseHandle(handle)`. Use `windows.WaitForSingleObject(handle, windows.INFINITE)` in a goroutine and pipe the result into a channel for the `select` in the listener. |
| NSIS `VersionCompare` from `WordFunc.nsh` | installer primitive | string comparison | No existing `!include` of `WordFunc.nsh` in `installer/squirebot.nsi`. | Standard NSIS built-in (NOT a plugin, no CI fetch needed). Add `!include "WordFunc.nsh"` near the top of the .nsi file. Reference: NSIS docs at https://nsis.sourceforge.io/Docs/WordFunc/WordFunc.html (per CONTEXT.md canonical_refs note). |
| NSIS poll loop between `--quit` and `taskkill /F` | installer primitive | polling | No existing poll loop in `installer/squirebot.nsi`. | Planner picks between (a) `nsProcess` plugin (cleaner syntax, requires CI fetch + plugin install) or (b) `System::Call 'kernel32::OpenProcess(...)` + `WaitForSingleObject` (no plugin, more code). CONTEXT.md D-05 leaves this open. Per CLAUDE.md "User installs missing toolchains themselves" — if (a) is chosen, STOP and surface to user before plan executes. Default to (b) for fewest moving parts (matches D-05 rationale: "every extra NSIS plugin is one more thing CI has to fetch and one more thing that can break on a future NSIS version bump"). |

---

## Metadata

**Analog search scope:** `internal/**/*.go`, `cmd/squirebot/main.go`, `installer/squirebot.nsi`, `docs/troubleshooting.md`, `docs/build-and-install.md`, `.github/workflows/release.yml`, `go.mod`.
**Files scanned:** 64 Go files, 1 NSIS file, 2 docs files, 1 workflow file.
**Build-tagged Windows file inventory:** 3 (`internal/eqfind/registry_windows.go`, `internal/eqfind/heuristic_windows.go`, `internal/auth/store_test.go`).
**Pattern extraction date:** 2026-05-11.

---

## Planner Mapping Hints (cross-reference to verification hooks)

| ROADMAP criterion | Plans touched | Primary patterns to copy |
|-------------------|---------------|--------------------------|
| 1: clean upgrade over running tray | All Go + NSIS plans | --uninstall-wipe-credentials block (main.go:38-54), uninstaller taskkill (squirebot.nsi:136) |
| 2: signal graceful, wait, hard-kill fallback | Plans for shutdown_signal.go + nsi | windows/registry build-tag pattern, error-wrap style from swap.go |
| 3: post-install autostart + resumes writes | NSIS plan + acceptance | UNCHANGED — preserve squirebot.nsi line 102 (HKCU Run) + line 105 (Exec) |
| 4: docs/troubleshooting.md edit | Docs plan | Pure deletion of lines 50-58 |
| 5: binary v1.0.1 built, tagged, published | Release plan | release.yml is config-driven; tag = ship gate |
