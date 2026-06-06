---
phase: 24-watcher-test-hardening-c1-c2-coverage
reviewed: 2026-06-03T15:50:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/app/runapp.go
  - internal/app/runapp_test.go
  - internal/eqfind/heuristic_windows_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 24: Code Review Report

**Reviewed:** 2026-06-03T15:50:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

This was a refactor + test-hardening phase. The twin upload handlers
(`makeOnInventoryChange` / `makeOnSpellbookChange`) were collapsed into a single
`makeOnFileChange` body parameterized by a `fileKind` descriptor, plus an extracted
`handleIngestErr` helper. Five spellbook behavior tests and five `walkRoot` tests
were added.

**Refactor fidelity: verified.** I diffed the collapsed body against the deleted
twin handlers line-by-line (`git diff 6f2207f..HEAD`). The inventory/spellbook
asymmetry is faithfully preserved:
- `slogNoun` keeps op strings noun-specific ("uploaded inventory" vs "uploaded
  spellbook") — confirmed in the verbose test logs.
- `traySuffix` preserves the v1 wording asymmetry ("Last upload: Foo" vs
  "Last upload: Foo spellbook" / "Last upload failed: Foo spellbook").
- Per-kind `LastKnown*Mtime` maps are wired via the `mtimeMap` closure; the
  204-persist tests prove the correct kind-specific map is written and the other
  is untouched.
- The pointer-to-map indirection (`m := fk.mtimeMap(cfg); if *m == nil { *m = make(...) }`)
  correctly reproduces the original lazy-init-then-assign semantics.

The package builds clean (`go build`), vets clean (`go vet`), and all 15
new/affected tests pass with branch-confirming log output (401/426/409/empty-skip/
catch-up all fire). No BLOCKERs found.

The findings below concern **test quality** — specifically test helpers with latent
bugs and assertions that do not pin the boundaries they claim to exercise. None
affect production correctness today, but each weakens the safety net this phase
exists to build.

## Warnings

### WR-01: `readAll` test helper does a single unbuffered `Read` — silently truncates request bodies

**File:** `internal/app/runapp_test.go:233-240`
**Issue:** The `ingestRecorder` body capture uses a single `r.Body.Read(buf)` call:
```go
func readAll(r *http.Request) (string, error) {
	buf := make([]byte, r.ContentLength)
	if r.ContentLength <= 0 {
		return "", nil
	}
	_, err := r.Body.Read(buf)
	return string(buf), err
}
```
`io.Reader.Read` is permitted to return fewer than `len(buf)` bytes in one call —
there is no loop and the returned byte-count is discarded. For small bodies over
httptest loopback this *usually* fills in one read, so the bug is currently latent
(no existing test asserts on `ir.lastBody`). But the moment a future test checks the
POSTed content (the obvious next hardening step for "did we send the right
CP1252-decoded body?"), it will intermittently see a truncated string and either
flake or pass falsely. The function is also named `readAll` — exactly the contract
it does NOT honor.
**Fix:** Use `io.ReadAll`, which loops to EOF:
```go
import "io"

func readAll(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	return string(b), err
}
```
This also removes the dependency on `r.ContentLength` (which is -1 for chunked
requests).

### WR-02: `walkRoot` depth-cap tests never pin the boundary (depth 5 vs 6) — an off-by-one in the cap would pass undetected

**File:** `internal/eqfind/heuristic_windows_test.go:31-57`
**Issue:** `maxHeuristicDepth = 5` and the production check is `if curDepth > maxHeuristicDepth { SkipDir }` (so depth 5 is the deepest *found* level, depth 6 is pruned). `TestWalkRoot_FindsSentinelPairAtDepth` only tests depths 1, 2, 3; `TestWalkRoot_BeyondDepthCapNotFound` tests depth 6. The exact boundary — depth 5 must be FOUND, depth 6 must be PRUNED — is never asserted. If someone changed the cap to `>=` (off-by-one), depth-5 installs would silently stop being discovered, yet every test here would still pass (3 ≤ 4, 6 ≥ 5 both prune-or-find as expected). The "depth cap" tests therefore do not actually guard the cap value they were written to protect.
**Fix:** Add the two boundary cases:
```go
// Depth 5 (exactly at the cap) MUST be found.
func TestWalkRoot_AtDepthCapFound(t *testing.T) {
	root := t.TempDir()
	want := plantEQAt(t, root, "a", "b", "c", "d", "e") // depth 5
	if got := walkRoot(context.Background(), root); got != want {
		t.Errorf("walkRoot at exactly maxHeuristicDepth = %q, want %q", got, want)
	}
}
// (depth 6 already covered by TestWalkRoot_BeyondDepthCapNotFound)
```

### WR-03: `TestRescanCatchUp_FiresOnNewerFiles` mixes atomic counters with unsynchronized slice appends, signaling a concurrency model that isn't real

**File:** `internal/app/runapp_test.go:118-145`
**Issue:** The callbacks use `atomic.AddInt64(&invCount, 1)` (implying concurrent
invocation) but in the same closure do a plain `invPaths = append(invPaths, p)`
(NOT safe under concurrency). This is internally contradictory. `rescanCatchUp`
actually invokes callbacks *synchronously* (runapp.go:301 is a direct `cb(...)` on
the calling goroutine), so neither the atomics nor a mutex are needed — but a reader
can't tell that from the test, and if `rescanCatchUp` ever spawned goroutines, the
atomic counter would pass while the slice append would race and corrupt/lose paths.
The `-race` detector could not be run here (no cgo/gcc on this box), so the mixed
model isn't even caught by tooling. Pick one model and make it consistent.
**Fix:** Drop the atomics (callbacks are synchronous) — plain `int`/`[]string` is
correct and self-documenting:
```go
var invPaths, spbPaths []string
onInv := func(p string) { invPaths = append(invPaths, p) }
onSpb := func(p string) { spbPaths = append(spbPaths, p) }
// ... assert len(invPaths) == 1, etc.
```
(Apply the same simplification to `TestRescanCatchUp_MissingFolderIsSkipped:198-206`.)

## Info

### IN-01: Stale comment references line numbers that no longer exist

**File:** `internal/app/runapp.go:320-322`
**Issue:** The `handleIngestErr` doc says it was "Extracted verbatim from the twin
handlers (was runapp.go:355-372 ≡ :419-437)". Line-number references in comments rot
immediately on the next edit and are already meaningless post-refactor (the file is
now 465 lines and those handlers are gone). The "≡" claim is also unverifiable by a
future reader.
**Fix:** Drop the line numbers; keep the intent ("extracted from the former twin
inventory/spellbook handlers; behavior-preserving").

### IN-02: `fileKind` doc says "five tokens" but the struct lists five fields where one (`kind`) and another (`suffix`) overlap conceptually with derivable values

**File:** `internal/app/runapp.go:306-314`
**Issue:** Minor: `slogNoun` is always equal to `kind` in both call sites
("inventory"/"inventory", "spellbook"/"spellbook"). They are documented as
independently meaningful, but no call site diverges them. This is defensible (they
*could* diverge, and keeping them separate documents intent) — flagging only so a
future maintainer doesn't assume they must stay in lockstep or, conversely, collapse
them and lose the documented seam. No change required.

### IN-03: `TestWatcherRunningGuard_SecondEntryNoOps` depends on real DPAPI via `credstore.Store` and skips silently off-Windows

**File:** `internal/app/runapp_test.go:269-332`
**Issue:** The guard test reaches the watcher phase by storing a real guild code
through `credstore.Store` (real DPAPI on Windows) and `t.Skipf`-ing when the
credential store is unavailable. On CI runners or any non-Windows/headless box this
HIGH-01-regression test becomes a silent no-op, so the regression guard it protects
could rot without anyone noticing a red build. This matches the existing platform
assumption in the codebase, so it's acceptable, but the skip means coverage of this
critical guard is Windows-host-only.
**Fix:** None required given the project is Windows-only and go1.26/Windows is the
stated target. Optionally note in the phase VERIFICATION that this test is skipped
on non-Windows CI so the guard's coverage isn't assumed everywhere.

### IN-04: `pruneNames` map has gofmt-misaligned struct-literal values (cosmetic)

**File:** `internal/eqfind/heuristic_windows.go:59-68`
**Issue:** Not in the changed file set for this phase, but observed while tracing
`walkRoot`. The `pruneNames` entries are not column-aligned the way gofmt would emit
them (e.g. `"Program Files (x86)": {},` vs neighbors). `go vet` passed and `go build`
is clean, so this is purely cosmetic and likely just how the source reads here.
**Fix:** Run `gofmt -w` on the file if it is ever re-touched. No action needed for
this phase.

---

_Reviewed: 2026-06-03T15:50:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
