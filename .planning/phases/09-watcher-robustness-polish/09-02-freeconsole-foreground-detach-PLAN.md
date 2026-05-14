---
phase: 09-watcher-robustness-polish
plan: 02
plan_id: 09-02-freeconsole-foreground-detach
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd/squirebot/main.go
  - cmd/squirebot/console_windows.go
  - cmd/squirebot/console_other.go
  - docs/build-and-install.md
autonomous: true
requirements: [OPS-07]
tags: [main, freeconsole, ops-07, windows-syscall, build-tags, wave1]

must_haves:
  truths:
    - "A guildie who launches `squirebot.exe` from a foreground shell (cmd.exe `& exe` or PowerShell `& .\\squirebot.exe`) and then closes the shell does NOT lose the watcher — the process detaches from the parent console before any structured logging begins."
    - "The two pre-existing stderr-capturing short-circuits (`--uninstall-wipe-credentials` and `--quit`) still inherit stdio because freeConsole() runs AFTER both short-circuit checks have decided NOT to exit."
    - "The auto-update self-swap path (`update.Apply()` at main.go:91-98) still writes its error to stderr before any detach, preserving NSIS / parent-process visibility into update failures."
    - "The `slog.Info(\"squirebot exit\")` line at the end of main still emits — slog writes via lumberjack to the log file, unaffected by FreeConsole detachment."
    - "On non-Windows platforms (Linux/Darwin if ever cross-built for dev), `freeConsole()` is a no-op via the build-tagged stub so `go build` succeeds across GOOS values."
  artifacts:
    - path: cmd/squirebot/console_windows.go
      provides: "Windows-only freeConsole() helper that calls golang.org/x/sys/windows.FreeConsole()"
      contains: "windows.FreeConsole"
    - path: cmd/squirebot/console_other.go
      provides: "Non-Windows no-op stub for freeConsole()"
      contains: "//go:build !windows"
    - path: cmd/squirebot/main.go
      provides: "Single `_ = freeConsole()` call placed after both --quit and --uninstall-wipe-credentials short-circuits and after update.Apply, before logging.Setup"
      contains: "freeConsole()"
    - path: docs/build-and-install.md
      provides: "Optional one-line note documenting foreground-launch detach behavior (belt-and-suspenders)"
  key_links:
    - from: "cmd/squirebot/main.go (after update.Apply, before logging.Setup)"
      to: "cmd/squirebot/console_windows.go freeConsole()"
      via: "direct package-internal call"
      pattern: "freeConsole\\(\\)"
    - from: "cmd/squirebot/console_windows.go"
      to: "golang.org/x/sys/windows.FreeConsole"
      via: "Windows API syscall"
      pattern: "windows\\.FreeConsole"
---

<objective>
Eliminate the Phase 6 UAT Finding H foot-gun: a guildie who launches `squirebot.exe` from a cmd.exe / PowerShell session (without `Start-Process`) currently inherits the parent's console handle; closing the parent shell kills the watcher silently.

Per CONTEXT.md D-02 (locked under the "invisible UX" tiebreaker), implement **option (a) `windows.FreeConsole()`** — a single syscall early in `main()` that detaches the watcher from any inherited console. The guildie can launch the exe any way they want (double-click, Start-menu, PowerShell, batch, scheduler) and the watcher survives parent-shell closure.

Per CONTEXT.md specifics §3 and PATTERNS.md §"Existing main.go short-circuit ordering": `freeConsole()` MUST be placed AFTER the `--uninstall-wipe-credentials` block (main.go:39-55) AND AFTER the `--quit` block (main.go:69-76) AND AFTER the `update.Apply()` block (main.go:91-98), but BEFORE `logging.Setup()` (main.go:100). Both short-circuit checks and `update.Apply` write to stderr that the parent NSIS/shutdown-shim process must capture; subsequent slog writes target the log file only.

Build-tag the OS-specific syscall behind `console_windows.go` (+ `console_other.go` stub) following the same pattern the codebase already uses for `internal/system/shutdown_signal_*.go`. The Windows file calls `windows.FreeConsole()`; the stub returns nil.

**Scope discipline (per CONTEXT.md domain section + D-07):**
- One syscall. Don't refactor `main()`.
- No additional flags, no new env vars.
- No unit test (FreeConsole is OS-level and not unit-testable without elaborate mocking). Acceptance via the manual smoke captured by Plan 09-05 release UAT.
- The `docs/build-and-install.md` belt-and-suspenders note is OPTIONAL per CONTEXT.md Claude's-Discretion; this plan INCLUDES it as a one-line note (it's risk-free and helps developers understand the detach behavior).
- No schema changes (D-06): `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` MUST remain unchanged.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/09-watcher-robustness-polish/09-CONTEXT.md
@.planning/phases/09-watcher-robustness-polish/09-PATTERNS.md
@CLAUDE.md
@cmd/squirebot/main.go
@internal/system/shutdown_signal_windows.go
@internal/system/shutdown_signal_other.go
@docs/build-and-install.md

<interfaces>
<!-- Build-tag pair analog. Mirror exactly. From internal/system/shutdown_signal_*.go. -->

shutdown_signal_windows.go pattern (lines 1-11):
```go
//go:build windows

package system

import (
    "context"
    "errors"
    "fmt"

    "golang.org/x/sys/windows"
)
```

shutdown_signal_other.go pattern (full file):
```go
//go:build !windows

package system

import "context"

// SignalShutdown is a no-op on non-Windows platforms.
func SignalShutdown() error { return nil }
```

cmd/squirebot/main.go relevant ordering (lines 26-100):
```go
func main() {
    // 1. --uninstall-wipe-credentials short-circuit (lines 39-55) — writes stderr, exits.
    if len(os.Args) >= 2 && os.Args[1] == "--uninstall-wipe-credentials" { /* ... os.Exit(0) */ }

    // 2. --quit short-circuit (lines 69-76) — writes stderr, exits.
    if len(os.Args) >= 2 && os.Args[1] == "--quit" { /* ... os.Exit(0) */ }

    // 3. update.Apply auto-update self-swap (lines 91-98) — may write stderr, may exit 0.
    if swapped, err := update.Apply(); err != nil {
        fmt.Fprintf(os.Stderr, "auto-update apply failed: %v\n", err)
    } else if swapped {
        os.Exit(0)
    }

    // <-- INSERT freeConsole() HERE: line ~99/100, between update.Apply and logging.Setup.

    log, logDir := logging.Setup() // line 100
    slog.SetDefault(log)
    // ... rest of watcher startup ...
}
```

The golang.org/x/sys/windows package is already imported by other watcher files (e.g., `internal/system/shutdown_signal_windows.go`); the dependency is already in go.mod. Verify with `go list -m golang.org/x/sys` if uncertain.

windows.FreeConsole signature:
```go
func FreeConsole() (err error)
```
Returns nil if the calling process did not have a console (which is the case when launched via the GUI subsystem or after a prior FreeConsole) — safe to call unconditionally.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create cmd/squirebot/console_windows.go and cmd/squirebot/console_other.go build-tag pair</name>
  <files>cmd/squirebot/console_windows.go, cmd/squirebot/console_other.go</files>
  <read_first>
    - internal/system/shutdown_signal_windows.go (full file — canonical build-tag pattern)
    - internal/system/shutdown_signal_other.go (full file — canonical no-op stub pattern)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Plan 09-02" (build-tag scaffold pattern, lines covering shutdown_signal pair)
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-02 (rationale + ordering rule)
  </read_first>
  <action>
    Create `cmd/squirebot/console_windows.go` with EXACTLY this content (whitespace-significant for build-tag parsing):

    ```go
    //go:build windows

    package main

    import (
    	"log/slog"

    	"golang.org/x/sys/windows"
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
    // unconditionally. On any FreeConsole error, log at Warn level via slog
    // (which falls back to stderr because logging.Setup has not yet run —
    // intentional: an error here means the detach failed and the user may
    // care to see it during dev).
    func freeConsole() error {
    	if err := windows.FreeConsole(); err != nil {
    		slog.Warn("FreeConsole failed", "err", err)
    		return err
    	}
    	return nil
    }
    ```

    Create `cmd/squirebot/console_other.go` with EXACTLY this content:

    ```go
    //go:build !windows

    package main

    // freeConsole is a no-op on non-Windows platforms. The Windows
    // implementation lives in console_windows.go.
    // Plan 09-02 (OPS-07).
    func freeConsole() error { return nil }
    ```

    DO NOT modify `cmd/squirebot/main.go` in this task — that's Task 2.
    DO NOT add a test file — FreeConsole is OS-level and not unit-testable per CONTEXT.md D-07.

    Schema-impact assertion (per CONTEXT.md D-06): neither new file is anywhere near the schema boundary. `internal/sheet/client.go` is untouched.
  </action>
  <verify>
    <automated>go build ./cmd/squirebot/... && go vet ./cmd/squirebot/...</automated>
  </verify>
  <acceptance_criteria>
    - `cmd/squirebot/console_windows.go` exists.
    - `cmd/squirebot/console_other.go` exists.
    - `grep -nE '^//go:build windows$' cmd/squirebot/console_windows.go` matches exactly 1 line (line 1).
    - `grep -nE '^//go:build !windows$' cmd/squirebot/console_other.go` matches exactly 1 line (line 1).
    - `grep -nE 'windows\.FreeConsole\(\)' cmd/squirebot/console_windows.go` matches exactly 1 line.
    - `grep -nE 'func freeConsole\(\) error' cmd/squirebot/console_windows.go` matches exactly 1 line.
    - `grep -nE 'func freeConsole\(\) error \{ return nil \}' cmd/squirebot/console_other.go` matches exactly 1 line.
    - `grep -nE '"golang.org/x/sys/windows"' cmd/squirebot/console_windows.go` matches exactly 1 line.
    - `go build ./cmd/squirebot/...` exits 0 (the build-tag selects console_windows.go on GOOS=windows).
    - `go vet ./cmd/squirebot/...` exits 0.
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (unchanged).
  </acceptance_criteria>
  <done>Build-tag pair lands with FreeConsole call on Windows + nil-return stub elsewhere; `go build` clean; `go vet` clean; schema constant unchanged.</done>
</task>

<task type="auto">
  <name>Task 2: Wire freeConsole() call into main() at the correct ordering position</name>
  <files>cmd/squirebot/main.go</files>
  <read_first>
    - cmd/squirebot/main.go (full file — read all 214 LOC; you MUST understand the ordering of all short-circuits and update.Apply before editing)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Existing main.go short-circuit ordering" (exact line numbers + ordering rule)
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-02 paragraphs on ordering ("after --quit block + after update.Apply, before logging.Setup")
  </read_first>
  <action>
    Open `cmd/squirebot/main.go` and locate the area between the `update.Apply()` block (currently ends at line 98 with closing `}`) and `log, logDir := logging.Setup()` (currently line 100). Insert exactly:

    ```go
    	// Plan 09-02 (OPS-07): detach from any inherited console. Must run AFTER
    	// the --uninstall-wipe-credentials, --quit, and update.Apply short-circuit
    	// blocks above (those paths write to stderr that NSIS / parent process
    	// captures), but BEFORE logging.Setup so subsequent slog writes target
    	// only the lumberjack-backed log file. Closing the launching shell no
    	// longer kills the watcher. Safe (no-op) when the process has no
    	// console (e.g., launched via Explorer double-click). See
    	// console_windows.go / console_other.go for the build-tagged
    	// implementations.
    	_ = freeConsole()

    ```

    The `_ = freeConsole()` discards the error intentionally — `freeConsole()` already logs via slog.Warn on failure, and a failed detach is informational only (the watcher continues regardless; the worst case is the legacy "dies when shell closes" bug for this one user).

    Do NOT remove or reorder any existing code. The `--uninstall-wipe-credentials` block, `--quit` block, and `update.Apply()` block remain untouched at their current positions. Do NOT add any other freeConsole call elsewhere in the file. Do NOT modify imports — `freeConsole` lives in `package main` (same package as main.go), so no new import is needed.

    Verify the existing `slog.Info("squirebot exit")` line is still present near the end of main() at approximately line 213. CONTEXT.md D-02 verification requires this line still emit; freeConsole only detaches console stdio — slog writes via lumberjack to the log file, unaffected.

    Schema-impact assertion (per CONTEXT.md D-06): `internal/sheet/client.go` not modified by this task.
  </action>
  <verify>
    <automated>go build ./cmd/squirebot/... && go vet ./cmd/squirebot/... && go test ./... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE '_ = freeConsole\(\)' cmd/squirebot/main.go` matches exactly 1 line.
    - `grep -nE 'slog\.Info\("squirebot exit"\)' cmd/squirebot/main.go` matches exactly 1 line (existing exit log line preserved).
    - `grep -nE '"--uninstall-wipe-credentials"' cmd/squirebot/main.go` matches exactly 1 line (existing short-circuit preserved).
    - `grep -nE '"--quit"' cmd/squirebot/main.go` matches exactly 1 line (existing short-circuit preserved).
    - `grep -nE 'update\.Apply\(\)' cmd/squirebot/main.go` matches exactly 1 line (existing auto-update block preserved).
    - Run a PowerShell ordering check: `Select-String -Path cmd/squirebot/main.go -Pattern 'update\.Apply|freeConsole|logging\.Setup' | Select-Object LineNumber, Line` — confirm `update.Apply` line number < `freeConsole` line number < `logging.Setup` line number.
    - `go build ./cmd/squirebot/...` exits 0.
    - `go vet ./cmd/squirebot/...` exits 0.
    - `go test ./... -count=1` exits 0 (no regressions in the broader test suite).
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (unchanged).
  </acceptance_criteria>
  <done>`_ = freeConsole()` is wired into main() at exactly one location, sequenced after update.Apply and before logging.Setup; existing short-circuits and exit-log line are untouched; full test suite still passes; schema constant unchanged.</done>
</task>

<task type="auto">
  <name>Task 3: Add a one-line note to docs/build-and-install.md documenting foreground-launch detach</name>
  <files>docs/build-and-install.md</files>
  <read_first>
    - docs/build-and-install.md (full file — read it before editing; pick a placement that's "above the fold" for the relevant section)
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-02 Claude's-Discretion paragraph (the note is optional; this plan includes it as belt-and-suspenders)
  </read_first>
  <action>
    Open `docs/build-and-install.md` and find a section discussing manual launch of `squirebot.exe` (likely under a heading like "Manual launch", "Debugging", "Foreground launch", or "Running the watcher manually"). If no such section exists, add a small section titled `### Foreground launch (cmd / PowerShell)` near the existing manual-launch / debugging guidance.

    Insert exactly this note (verbatim):

    ```
    > **Foreground launch (cmd.exe / PowerShell):** `squirebot.exe` calls
    > `FreeConsole()` immediately on startup (after the `--uninstall-wipe-credentials`,
    > `--quit`, and auto-update short-circuits), so it detaches from any
    > inherited console. You can launch it from a foreground shell with `&
    > .\squirebot.exe` (PowerShell) or `squirebot.exe` (cmd) and the watcher
    > will keep running even after you close the shell. No `Start-Process`
    > wrapper required. To see live structured-log output instead, tail the
    > log file under `%LOCALAPPDATA%\SquireBot\logs\`. — Plan 09-02 / OPS-07
    ```

    This is a single blockquote; do NOT promote it to a new top-level heading. Place it adjacent to existing foreground / launch guidance if present; otherwise add a short `### Foreground launch (cmd / PowerShell)` heading immediately above it.

    Schema-impact assertion (per CONTEXT.md D-06): doc-only change; no Go source touched in this task; `internal/sheet/client.go` untouched.
  </action>
  <verify>
    <automated>grep -cE 'FreeConsole\(\)' docs/build-and-install.md</automated>
  </verify>
  <acceptance_criteria>
    - `grep -cE 'FreeConsole\(\)' docs/build-and-install.md` returns ≥ 1.
    - `grep -cE 'Plan 09-02|OPS-07' docs/build-and-install.md` returns ≥ 1 (provenance citation present).
    - `grep -cE 'Start-Process' docs/build-and-install.md` returns ≥ 1 (the note explains why Start-Process is no longer required).
    - Doc file ends in a single trailing newline (markdownlint-friendly): `tail -c 1 docs/build-and-install.md` returns a newline character (LF or CRLF, either fine on Windows).
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (unchanged).
  </acceptance_criteria>
  <done>One-blockquote note about FreeConsole / foreground launch lives in docs/build-and-install.md alongside existing launch guidance; references Plan 09-02 / OPS-07; schema constant unchanged.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| parent shell → watcher process | Inherited console handle; this plan severs it via FreeConsole |
| watcher process → log file | slog → lumberjack: detach does NOT affect file I/O (only stdio) |
| NSIS / shutdown-shim → short-circuits | --uninstall-wipe-credentials and --quit must keep inherited stderr so NSIS captures their output |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-09-02-01 | Denial of Service | FreeConsole called too early (breaks --quit / --uninstall stderr capture) | mitigate | Ordering rule (CONTEXT.md D-02 + specifics §3): freeConsole() is placed AFTER both short-circuit blocks AND AFTER update.Apply, BEFORE logging.Setup. Task 2 acceptance grep enforces line-number order: update.Apply < freeConsole < logging.Setup. |
| T-09-02-02 | Tampering | Cross-platform build break | mitigate | Build-tag pair `//go:build windows` + `//go:build !windows` mirrors the existing `internal/system/shutdown_signal_*.go` pattern verbatim. `go build ./...` on any GOOS picks exactly one file. |
| T-09-02-03 | Information Disclosure | Lost stdio for legitimate debug | accept | Developers wanting foreground log output should tail the log file (note added in Task 3). FreeConsole only detaches stdio, not the log file. |
| T-09-02-04 | Repudiation | Silent FreeConsole failure | mitigate | freeConsole() logs `slog.Warn("FreeConsole failed", "err", err)` on non-nil return. logging.Setup has not yet initialized at call time, so slog falls back to stderr (slog's default handler) — intentionally visible during dev. |

**Schema impact:** NONE. `internal/sheet/client.go` is not in `files_modified`. Verifier grep gate: `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (unchanged from Phase 6/7/8).
</threat_model>

<verification>
1. `go build ./...` exits 0 (cross-package build succeeds with the new console_*.go pair).
2. `go vet ./...` exits 0.
3. `go test ./... -count=1` exits 0 (no regressions; FreeConsole has no unit test).
4. PowerShell ordering check confirms `update.Apply` < `freeConsole` < `logging.Setup` line numbers in `cmd/squirebot/main.go`.
5. `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (schema unchanged).
6. `grep -nE 'windows\.FreeConsole' cmd/squirebot/console_windows.go` matches exactly 1 line.
7. `docs/build-and-install.md` contains the new FreeConsole / foreground-launch note with `Plan 09-02` provenance.

**Manual smoke (deferred to Plan 09-05 release UAT, per CONTEXT.md D-07):**
- Launch v1.0.2 candidate exe from `& .\squirebot.exe` in PowerShell.
- Close the PowerShell window.
- Confirm SquireBot tray icon persists; confirm `%LOCALAPPDATA%\SquireBot\logs\` shows fresh slog entries; confirm the process is still alive via `Get-Process squirebot`.
</verification>

<success_criteria>
- A guildie launching from foreground PowerShell / cmd no longer experiences silent death on shell close.
- Both stderr-capturing short-circuits (--quit, --uninstall-wipe-credentials) and update.Apply's stderr output are preserved by virtue of strict ordering.
- The build-tag pair compiles on every supported GOOS.
- No schema change; no new test infrastructure; no behavior regression in the broader test suite.
</success_criteria>

<output>
After completion, create `.planning/phases/09-watcher-robustness-polish/09-02-SUMMARY.md` summarizing:
- File scaffold added (console_windows.go + console_other.go); call site wired into main.go.
- Ordering verification (update.Apply < freeConsole < logging.Setup line numbers).
- docs/build-and-install.md note added.
- Schema constant `WatcherMaxSchemaVersion = 3` confirmed unchanged.
- Manual smoke deferred to 09-05 release UAT (PowerShell foreground-launch + shell-close + tray-survival check).
</output>
