---
phase: 09-watcher-robustness-polish
reviewed: 2026-05-13T00:39:05Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - cmd/squirebot/main.go
  - cmd/squirebot/console_windows.go
  - cmd/squirebot/console_other.go
  - internal/app/runapp.go
  - internal/config/config.go
  - internal/tray/tray.go
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-05-13T00:39:05Z
**Depth:** standard
**Files Reviewed:** 6 (docs/build-and-install.md excluded — documentation, not source)
**Status:** issues_found
**Diff range:** 4450f9a..839a41e

## Summary

Phase 9 ships four small fixes (OPS-06 pre-Ready tray queue, OPS-07 console
detach, CONFIG-01 UTF-8 BOM strip, AUTH-07 boot revoked-token classifier)
plus a NSIS-only doc update. Implementation is conservative — no new package
dependencies are introduced (`golang.org/x/sys v0.43.0` was already a
transitive dep), no new mutexes are added (the existing `t.mu` correctly
guards `pending` and `ready`), and the canonical AUTH-05 status string
`"Reauthorize: refresh token died. Click Reauthorize…"` is preserved
verbatim across `suspendForAuth` and the new `applyBootAuthError` revoked
branch.

`go vet` is clean across all four touched packages. No race conditions or
goroutine leaks were identified. Concurrency review of the tray
queue-and-drain pattern (the highest-risk change) confirms the lock
discipline: the OnReady drain runs `t.mu.Lock(); t.ready = true;
drainPending(); t.mu.Unlock()` as a single critical section, and every
mutator either appends to `pending` or applies live under the same lock,
preventing both lost updates and torn state.

Two warnings found: (1) `cmd/squirebot/console_windows.go` is not gofmt-clean
(extra space in the `var (...)` block), which will fail any
`gofmt -l` CI gate; and (2) `freeConsole()`'s implementation contradicts its
own godoc on the no-console-attached case — the doc claims "returns nil",
but the code logs a Warn and returns a non-nil error. Both are
low-blast-radius but should be fixed before tagging v1.0.2.

Four `info`-level findings cover redundant queue work in
`SetSpreadsheetID`, a misleading inline comment in `freeConsole`'s error
path, dead-effectively `actSetSpreadsheetID` enum value, and a
discarded-return-value pattern in `runapp.go`.

## Warnings

### WR-01: `console_windows.go` is not gofmt-clean (CI gate)

**File:** `cmd/squirebot/console_windows.go:17-19`
**Issue:** The `var (...)` block aligns `kernel32` against `procFreeConsole`
with the wrong column width. `gofmt -d` reports a diff: the `=` after
`kernel32` has 9 spaces of padding, but gofmt wants 8 (aligning to the
single-space gap after the longest identifier `procFreeConsole`). Any CI
job running `gofmt -l ./...` or `go fmt ./... && git diff --exit-code` will
fail on this file. The other three Phase 9 Go files in the diff scope are
gofmt-clean; `internal/config/config.go` also has a gofmt diff but it is
on the pre-existing `Config` struct alignment (not introduced by Phase 9
— verified by inspecting the diff).
**Fix:**
```go
var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole = kernel32.NewProc("FreeConsole")
)
```
(Single space before `=` on each line; gofmt reflows automatically — run
`gofmt -w cmd/squirebot/console_windows.go`.)

### WR-02: `freeConsole()` doc-comment contradicts implementation on no-console path

**File:** `cmd/squirebot/console_windows.go:36-39, 51-58`
**Issue:** The doc-comment promises: "Returns nil if the process had no
console attached (e.g., launched via the GUI subsystem or by Explorer
double-click) — safe to call unconditionally." But the implementation
returns `fmt.Errorf("FreeConsole: %w", err)` whenever `ret == 0`, AND
logs `slog.Warn("FreeConsole failed", "err", err)`. Per MSDN, FreeConsole
on a process without an attached console returns 0 (BOOL false) and sets
LastError to ERROR_INVALID_HANDLE — exactly the path the doc says
returns nil. The caller in main.go discards the return value with `_ =
freeConsole()`, so the user-visible impact is only a spurious slog.Warn
at startup before `logging.Setup()` runs (it lands on stderr, which for a
console-subsystem build is harmless but noisy under tools that capture
stderr — including the very NSIS / parent-shell capture path the call
ordering comment in main.go cites). Worse, if Phase 10+ ever flips to
the GUI subsystem (`-ldflags="-H=windowsgui"`), every cold start will
log this warning into the default handler.
**Fix:** Either fix the implementation to match the doc (preferred — the
contract is sensible):
```go
func freeConsole() error {
	ret, _, err := procFreeConsole.Call()
	if ret != 0 {
		return nil // success
	}
	// Distinguish "no console to free" (benign) from a real failure.
	if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_INVALID_HANDLE {
		return nil
	}
	slog.Warn("FreeConsole failed", "err", err)
	return fmt.Errorf("FreeConsole: %w", err)
}
```
Or, if the noisy-Warn behavior is intentional, update the doc-comment to
say so explicitly and remove the "safe to call unconditionally" promise.

## Info

### IN-01: `SetSpreadsheetID` queues a redundant `actSetSpreadsheetID`

**File:** `internal/tray/tray.go:415-422`
**Issue:** Unlike the other six mutators, `SetSpreadsheetID` applies its
write live (sets `t.spreadsheetID = id` immediately) AND queues an
`actSetSpreadsheetID` action that will replay the same assignment when
`drainPending` runs. The replay is a no-op in steady state (assignment is
idempotent) and is correct, but the queued entry is dead-effective work.
This is called out in the godoc ("symmetric with the other mutators"),
but the symmetry argument is weak — the other six mutators *must* queue
because they touch `*MenuItem` fields that are nil pre-Ready. The
`spreadsheetID` field exists at construction time. Either drop the queue
branch (cheaper) or drop the immediate assignment (more symmetric but
breaks the `loop()` ClickedCh handler if a click somehow fires pre-Ready
— though by construction it can't).
**Fix:** Drop the queue branch — the immediate assignment is sufficient:
```go
func (t *Controller) SetSpreadsheetID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spreadsheetID = id
}
```
And remove the `actSetSpreadsheetID` enum value + its `drainPending`
case + the `spreadsheetID` field on `pendingAction`.

### IN-02: Misleading "successfully" comment on the FreeConsole error path

**File:** `cmd/squirebot/console_windows.go:52-55`
**Issue:** The inline comment reads "Some Windows builds set err to 'The
operation completed successfully.' even on no-console processes; we only
treat ret==0 as a real failure (BOOL contract)." This describes the
`ret != 0, err != nil` case (success with a stale-but-non-error
LastError), which is exactly the path the code does NOT enter — that
branch returns nil. The comment belongs at the function head, not on the
`ret == 0` branch. Inside the `ret == 0` block, the comment is
self-contradicting: if `ret == 0`, the call failed and err *should* be a
real Win32 error, not "completed successfully".
**Fix:** Move the comment up to where the syscall return is destructured,
and rephrase the ret==0 branch's comment to describe the actual failure
case (e.g., "ret==0 means FreeConsole rejected the call — usually
ERROR_INVALID_HANDLE on a process that never had a console attached, but
also fires on other rare faults. Log + return so the caller can
inspect.").

### IN-03: `_ = applyBootAuthError(t, err)` discards a classification used only by tests

**File:** `internal/app/runapp.go:113`
**Issue:** `applyBootAuthError` returns a `bootAuthClassification` that
the production caller deliberately discards (`_ =`). The return value
exists solely so the test in `runapp_test.go` (verified to be its only
non-package-internal consumer) can assert which branch ran. Two cleaner
options: (a) export the function from the package and have it return
nothing, with tests inspecting tray side effects instead; (b) make the
function return `(bootAuthClassification, error)` and have the caller log
on non-revoked. Current shape is a minor anti-pattern (production-API
shaped by test convenience) but not a bug. Calling it out for future
cleanup, not blocking.
**Fix:** Either return `()` and assert tray side effects in tests, or
exfiltrate the classification via a test-only seam (e.g., a package-level
hook variable). No action required for v1.0.2.

### IN-04: `slog.Warn` in `freeConsole` may write to a stderr the call is about to detach

**File:** `cmd/squirebot/console_windows.go:55`
**Issue:** On the `ret == 0` failure path, the code calls `slog.Warn(...)`
before returning. The default slog handler writes JSON-formatted records
to `os.Stderr`. If the underlying FreeConsole syscall partially succeeded
(detached the console but reported failure — rare but documented in
older Windows builds), the write would target a now-invalid handle.
Windows discards such writes silently rather than crashing, so this is
not a defect; flagged only because the doc-comment elsewhere in the file
cites stderr-capture ordering as a load-bearing concern. Combined with
WR-02, the safest path is to keep `freeConsole` silent on the
no-console-attached case and only log on a truly unexpected error.
**Fix:** Covered by WR-02's proposed fix — return nil silently on
ERROR_INVALID_HANDLE, log only on other errors.

---

## Items explicitly checked and found clean

- **Concurrency in tray.go**: `pending` + `ready` are both guarded by the
  pre-existing `t.mu`. No second mutex introduced. `drainPending` is
  called only from contexts that hold `t.mu` (OnReady, simulateReady,
  per the "Caller MUST hold t.mu" comment). The `OnReady` drain block
  acquires the lock, flips ready, drains, releases — single critical
  section, no torn state visible to concurrent mutators. `loop()` is
  spawned AFTER drain releases the lock, so click handlers see a stable
  menu.
- **AUTH-07 boot classifier**: `applyBootAuthError`'s revoked branch
  calls the tray triple in the same order as `suspendForAuth`
  (`SetIconHealth(Red)`, `SetStatus(...)`, `ShowReauthorize`) with the
  *exact* canonical status string `"Reauthorize: refresh token died.
  Click Reauthorize…"` (verbatim byte-for-byte match verified). The
  non-revoked branch preserves the pre-AUTH-07 ordering
  (`SetStatus`, `SetIconHealth`, `ShowContinueSetup`). Pre-Ready queue
  (Plan 09-01) correctly buffers all three calls since `applyBootAuthError`
  fires from `RunApp` which runs concurrently with systray.Run.
- **UTF-8 BOM strip in config.go**: `bytes.TrimPrefix(data, []byte{0xEF,
  0xBB, 0xBF})` removes exactly one leading 3-byte BOM. Per Go docs,
  TrimPrefix is a single-occurrence operation — it will not strip
  mid-document occurrences or chained BOMs. UTF-16 BOMs (0xFF 0xFE / 0xFE
  0xFF) are NOT stripped, but `encoding/json` would reject those anyway
  with a clear error, so this is acceptable scope-narrowing per Plan 09-03.
- **`console_windows.go` LazySystemDLL**: Uses
  `windows.NewLazySystemDLL("kernel32.dll")` which forces the system32
  search path (mitigates DLL preload attacks — comment correctly notes
  this). `kernel32.NewProc("FreeConsole")` is the canonical idiom.
  `procFreeConsole.Call()`'s `(ret, _, err)` destructure correctly
  implements the FreeConsole BOOL contract (non-zero = success).
- **No new package dependencies**: `golang.org/x/sys v0.43.0` is already
  in `go.mod` (transitive via fyne/wincred). No `go get` was needed.
- **No goroutine leaks**: `_ = freeConsole()` is synchronous. The new
  drain block in OnReady is bounded by `len(t.pending)`. No new
  goroutines spawned.
- **No panics introduced**: All `*MenuItem` derefs in `drainPending` are
  guarded by `if t.mContinueSetup != nil` etc., even though the call site
  in OnReady runs AFTER `systray.AddMenuItem`. The guard is defensive
  but harmless.

---

_Reviewed: 2026-05-13T00:39:05Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
