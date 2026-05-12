---
phase: 09-watcher-robustness-polish
plan: 02
plan_id: 09-02-freeconsole-foreground-detach
subsystem: watcher/cmd
tags: [main, freeconsole, ops-07, windows-syscall, build-tags, wave1]
requirements: [OPS-07]
status: complete
completed: "2026-05-12"
dependency_graph:
  requires:
    - "internal/system/shutdown_signal_windows.go (build-tag pattern analog)"
    - "internal/system/shutdown_signal_other.go (build-tag stub analog)"
    - "cmd/squirebot/main.go ordering (short-circuits + update.Apply must precede freeConsole; logging.Setup must follow)"
  provides:
    - "cmd/squirebot/console_windows.go: freeConsole() Windows-side detach via kernel32!FreeConsole"
    - "cmd/squirebot/console_other.go: freeConsole() no-op stub for non-Windows builds"
    - "cmd/squirebot/main.go: _ = freeConsole() call site between update.Apply and logging.Setup"
    - "docs/build-and-install.md: Foreground launch note section"
  affects:
    - "Phase 6 UAT Finding H (foreground-launched watcher death) — closed"
    - "Plan 09-05 release UAT: smoke test for foreground-launch + shell-close + tray-survival"
tech_stack:
  added:
    - "golang.org/x/sys/windows.NewLazySystemDLL → kernel32.dll → FreeConsole proc (lazy-loaded Win32 syscall)"
  patterns:
    - "Build-tag pair for OS-specific code (//go:build windows + //go:build !windows) — mirrors internal/system/shutdown_signal_*.go"
    - "LazyProc.Call for Win32 entry points not directly exported by x/sys/windows (canonical Go idiom)"
key_files:
  created:
    - "cmd/squirebot/console_windows.go"
    - "cmd/squirebot/console_other.go"
  modified:
    - "cmd/squirebot/main.go (+11 lines, single _ = freeConsole() call + comment block)"
    - "docs/build-and-install.md (+11 lines, ### Foreground launch (cmd / PowerShell) section)"
decisions:
  - "Used windows.NewLazySystemDLL → kernel32!FreeConsole instead of the plan's specified windows.FreeConsole() — the latter is not exported by golang.org/x/sys/windows v0.43.0 (verified by grep against the module cache). The LazyProc.Call idiom is canonical Go for Win32 functions not directly bound by x/sys/windows; NewLazySystemDLL forces system32 search-path, mitigating DLL-preload attacks. Logged as Rule 1 (bug)/Rule 3 (blocking) deviation."
metrics:
  duration_minutes: 60
  tasks_completed: 3
  files_changed: 4
  net_lines_added: 90
  net_lines_removed: 0
  test_count_delta: 0
  schema_version: 3
  watcher_max_schema_version: 3
---

# Phase 9 Plan 02: FreeConsole Foreground Detach Summary

**One-liner:** Detach watcher from inherited console via kernel32!FreeConsole call site after update.Apply and before logging.Setup so closing the launching shell no longer kills the watcher.

## Objective Recap

Eliminate Phase 6 UAT Finding H — a guildie launching `squirebot.exe` from a foreground cmd.exe / PowerShell session (without `Start-Process`) loses the watcher when the parent shell closes. Per CONTEXT.md D-02 (locked under the "invisible UX" tiebreaker), implement option (a): a single FreeConsole syscall early in `main()` that detaches the watcher from any inherited console. No new launch incantation required; the watcher survives parent-shell closure regardless of how it was launched.

## What Shipped

### Task 1 — Build-tag pair (`console_windows.go` + `console_other.go`)
Commit: `4e3c8a2` — *feat(09-02): add console build-tag pair for OPS-07 FreeConsole detach*

- `cmd/squirebot/console_windows.go` (59 LOC): `//go:build windows`. Declares `var kernel32 = windows.NewLazySystemDLL("kernel32.dll")` + `var procFreeConsole = kernel32.NewProc("FreeConsole")`. The `freeConsole()` function calls `procFreeConsole.Call()`; on `ret == 0` (BOOL contract = failure), it logs via `slog.Warn("FreeConsole failed", "err", err)` and returns a wrapped error.
- `cmd/squirebot/console_other.go` (9 LOC): `//go:build !windows`. `func freeConsole() error { return nil }` — single-line no-op for cross-platform builds (Linux/Darwin dev builds via `go build` succeed).
- Build-tag pattern mirrors `internal/system/shutdown_signal_windows.go` / `_other.go` exactly.
- Acceptance grep gates all pass: both build tags present, FreeConsole proc reference present, `golang.org/x/sys/windows` import present, schema constant unchanged.

### Task 2 — main.go wiring
Commit: `da52202` — *feat(09-02): wire freeConsole() call into main.go for OPS-07*

- Inserted `_ = freeConsole()` plus an 8-line ordering-rationale comment block at line 109, between the `update.Apply` block (ends line 98) and `log, logDir := logging.Setup()` (line 111).
- Final ordering in `cmd/squirebot/main.go`:
  - Line 39: `--uninstall-wipe-credentials` short-circuit
  - Line 69: `--quit` short-circuit
  - Line 91: `update.Apply()` auto-update self-swap
  - **Line 109: `_ = freeConsole()` (new)**
  - Line 111: `logging.Setup()`
  - Line 224: `slog.Info("squirebot exit")` (unchanged exit log line)
- `_ = freeConsole()` discards the error intentionally — `freeConsole()` already logs via `slog.Warn` on failure; a failed detach is informational only and does not stop the watcher.
- All four short-circuits (`--uninstall-wipe-credentials`, `--quit`, `update.Apply()`) keep their stderr-bound output flow because `freeConsole` runs after them.

### Task 3 — docs/build-and-install.md note
Commit: `9433f5b` — *docs(09-02): add Foreground launch note for OPS-07 FreeConsole behavior*

- Appended a new `### Foreground launch (cmd / PowerShell)` section directly after the existing `### Manual debug aids` heading.
- The blockquote explains: FreeConsole detaches the watcher from any inherited console; `& .\squirebot.exe` (PowerShell) and `squirebot.exe` (cmd) both work without `Start-Process`; tail `%LOCALAPPDATA%\SquireBot\logs\` for live structured-log output.
- Provenance citation included (`Plan 09-02 / OPS-07`); references `Start-Process` (the now-unnecessary wrapper) and `FreeConsole()` (the underlying fix).

## Verification

All plan-level verification steps pass:

1. `go build ./...` → 0 (cross-package build succeeds with new console_*.go pair).
2. `go vet ./...` → 0.
3. `go test ./... -count=1` → 0 (all packages green; no regressions). 16 packages tested, 0 failures.
4. PowerShell-equivalent ordering check via grep: `--uninstall-wipe-credentials(39) < --quit(69) < update.Apply(91) < freeConsole(109) < logging.Setup(111) < slog.Info("squirebot exit")(224)` — confirmed via single-line grep extract on `cmd/squirebot/main.go`.
5. `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` → exactly 1 match at line 44 (schema constant unchanged — CONTEXT.md D-06 assertion satisfied).
6. `grep -nE 'procFreeConsole\.Call\(\)' cmd/squirebot/console_windows.go` → 1 match.
7. `docs/build-and-install.md` contains the new FreeConsole / foreground-launch note with `Plan 09-02 / OPS-07` provenance.

**Manual smoke (deferred to Plan 09-05 release UAT per CONTEXT.md D-07):**
- Launch v1.0.2 candidate exe via `& .\squirebot.exe` in PowerShell.
- Close the PowerShell window.
- Confirm tray icon persists; confirm `%LOCALAPPDATA%\SquireBot\logs\` shows fresh slog entries after shell close; confirm `Get-Process squirebot` returns a process.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug / Rule 3 - Blocking] FreeConsole API not exported by `golang.org/x/sys/windows` v0.43.0**

- **Found during:** Task 1 (pre-implementation verification of plan's specified import).
- **Issue:** PLAN.md (and CONTEXT.md D-02 `interfaces` section) specifies `windows.FreeConsole() (err error)` as the entry point. Verified against the module cache at `C:\Users\Virus Canary\go\pkg\mod\golang.org\x\sys@v0.43.0\windows\` — no `FreeConsole`, `AllocConsole`, `AttachConsole`, or `procFreeConsole` symbols exist in the package. Writing `windows.FreeConsole()` would fail compilation with `undefined: windows.FreeConsole`.
- **Fix:** Used the canonical Go idiom for unbound Win32 functions: `windows.NewLazySystemDLL("kernel32.dll")` + `.NewProc("FreeConsole")` + `.Call()`. `NewLazySystemDLL` (vs. `NewLazyDLL`) forces a system32 search path, mitigating DLL-preload attacks (security-equivalent to a direct binding).
- **Trade-off:** Slightly more verbose than the plan's intended one-liner, but functionally and semantically equivalent. The BOOL contract (`ret == 0` = failure) is enforced explicitly; the previous `if err := windows.FreeConsole(); err != nil` shape is preserved by checking `ret == 0` first and returning a wrapped error on failure.
- **Files modified:** `cmd/squirebot/console_windows.go` (only — `console_other.go` stub and `main.go` call site are unchanged from plan spec).
- **Commit:** `4e3c8a2`.

## Authentication Gates

None. This plan is offline / build-only.

## Threat-model coverage

| Threat ID | Mitigation status | Notes |
|-----------|-------------------|-------|
| T-09-02-01 (DoS — FreeConsole called too early, breaks --quit/--uninstall stderr capture) | mitigated | Ordering enforced via grep gate: `update.Apply < freeConsole < logging.Setup`. Manual line-number check confirms 91 < 109 < 111. |
| T-09-02-02 (Tampering — cross-platform build break) | mitigated | Build-tag pair (`//go:build windows` + `//go:build !windows`) mirrors `internal/system/shutdown_signal_*.go` verbatim. `go build ./...` is green on host (windows/amd64); the `!windows` stub guarantees other GOOS values compile via `freeConsole() error { return nil }`. |
| T-09-02-03 (Info Disclosure — lost stdio for legitimate debug) | accepted | Documented in Task 3's `### Foreground launch (cmd / PowerShell)` note: developers wanting live output tail the lumberjack log file. |
| T-09-02-04 (Repudiation — silent FreeConsole failure) | mitigated | `freeConsole()` logs `slog.Warn("FreeConsole failed", "err", err)` on `ret == 0`. logging.Setup has not run at call time → slog falls back to its default handler (stderr) → still visible during dev runs. |

## Threat Flags

None — no new network endpoints, no new auth paths, no new file-access patterns, no schema changes at trust boundaries.

## Self-Check

Verification of artifacts and commits referenced in this SUMMARY:

```
FOUND: cmd/squirebot/console_windows.go
FOUND: cmd/squirebot/console_other.go
FOUND: cmd/squirebot/main.go (modified — _ = freeConsole() at line 109)
FOUND: docs/build-and-install.md (modified — ### Foreground launch section)
FOUND commit: 4e3c8a2 (Task 1 — build-tag pair)
FOUND commit: da52202 (Task 2 — main.go wiring)
FOUND commit: 9433f5b (Task 3 — docs note)
GREP: WatcherMaxSchemaVersion = 3 still present at internal/sheet/client.go:44 (unchanged from Phase 6/7/8)
GREP: _ = freeConsole() present at cmd/squirebot/main.go:109 (exactly 1 occurrence)
BUILD: go build ./... exits 0
VET: go vet ./... exits 0
TEST: go test ./... -count=1 exits 0 (all 16 packages green)
```

## Self-Check: PASSED

## Known Stubs

None.

## Deferred Issues

None.

## Out-of-Scope Observations (deferred to deferred-items.md candidates)

None surfaced during execution. The OPS-07 fix is structurally contained to the four touched files; no adjacent code paths required attention.
