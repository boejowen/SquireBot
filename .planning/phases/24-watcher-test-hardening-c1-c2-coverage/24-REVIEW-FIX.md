---
phase: 24-watcher-test-hardening-c1-c2-coverage
fixed_at: 2026-06-03T16:10:00Z
review_path: .planning/phases/24-watcher-test-hardening-c1-c2-coverage/24-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 24: Code Review Fix Report

**Fixed at:** 2026-06-03T16:10:00Z
**Source review:** .planning/phases/24-watcher-test-hardening-c1-c2-coverage/24-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (the 3 WARNINGs; INFO items IN-01..IN-04 out of scope under `critical_warning`)
- Fixed: 3
- Skipped: 0

All fixes are test-quality only — no production code was changed. Verified after
all three fixes on Windows / Go 1.26:
- `go test ./internal/app/... ./internal/eqfind/...` → both packages `ok` (includes the new boundary test).
- `gofmt -l internal/app/runapp_test.go internal/eqfind/heuristic_windows_test.go` → printed nothing.
- `go vet ./internal/app/... ./internal/eqfind/...` → clean.

## Fixed Issues

### WR-01: `readAll` test helper does a single unbuffered `Read` — silently truncates request bodies

**Files modified:** `internal/app/runapp_test.go`
**Commit:** 03012fd
**Applied fix:** Replaced the single-`Read` `readAll` helper (which discarded the
returned byte count and relied on `r.ContentLength`) with `io.ReadAll(r.Body)`,
which loops to EOF and works for chunked requests. Added the `io` import.

### WR-02: `walkRoot` depth-cap tests never pin the boundary (depth 5 vs 6)

**Files modified:** `internal/eqfind/heuristic_windows_test.go`
**Commit:** f1f4596
**Applied fix:** Added `TestWalkRoot_AtDepthCapFound`, which plants a sentinel pair
at exactly `maxHeuristicDepth=5` (`plantEQAt(t, root, "a","b","c","d","e")`) and
asserts it IS found. Combined with the existing depth-6 prune test, this pins the
`curDepth > maxHeuristicDepth` boundary so an off-by-one (`>=`) would now fail. The
test lives in the existing `//go:build windows`-tagged file.

### WR-03: `TestRescanCatchUp_*` mixes atomic counters with unsynchronized slice appends

**Files modified:** `internal/app/runapp_test.go`
**Commit:** 4adbb91
**Applied fix:** Dropped the `atomic.AddInt64`/`atomic.LoadInt64` counters in
`TestRescanCatchUp_FiresOnNewerFiles` and `TestRescanCatchUp_MissingFolderIsSkipped`
and replaced them with plain `[]string`/`int`, since `rescanCatchUp` invokes its
callbacks synchronously on the calling goroutine. Added a clarifying comment.
`sync/atomic` import retained (still used by `ingestRecorder`).

---

_Fixed: 2026-06-03T16:10:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
