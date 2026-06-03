---
phase: 24-watcher-test-hardening-c1-c2-coverage
verified: 2026-06-03T16:10:00Z
status: passed
score: 10/10 must-haves verified
overrides_applied: 0
---

# Phase 24: Watcher Test Hardening (C1/C2 Coverage) Verification Report

**Phase Goal:** Close Church-of-Clean-Code audit findings C1 (`makeOnSpellbookChange` had ZERO tests), C2 (`internal/eqfind` real filesystem-walk discovery — `walkRoot` 0% covered), and REFACTOR (byte-for-byte twin upload-handler duplication) by (a) collapsing the twin handlers into one shared `makeOnFileChange` + `handleIngestErr` with NO inventory behavior change, (b) adding 5 spellbook behavior tests, and (c) adding direct `walkRoot` tests against a `t.TempDir()` tree.
**Verified:** 2026-06-03T16:10:00Z
**Status:** passed
**Re-verification:** No — initial verification (a 24-REVIEW.md exists but no prior VERIFICATION.md)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Inventory upload path behaves identically after refactor (existing `TestMakeOnInventoryChange_*` stay green) | ✓ VERIFIED | `go test -run TestMakeOnInventoryChange` → 4/4 PASS (204PersistsMtime, 401NoLoopSetsRed, EmptyFileSkipsNoRequest, 426UpdateNeeded) |
| 2 | Spellbook path has behavior tests for 204/401/426/409-cross-owner/empty-file | ✓ VERIFIED | `go test -run TestMakeOnSpellbookChange -v` → 5/5 PASS; all five named funcs present in runapp_test.go |
| 3 | Exactly one shared `makeOnFileChange` body + one extracted `handleIngestErr`; verbatim copy-paste gone | ✓ VERIFIED | `grep -c 'func makeOnFileChange('`=1, `grep -c 'func handleIngestErr('`=1, `grep -c 'errors.Is(err, backend.ErrUnauthorized)'`=1 (was 2 across twins) |
| 4 | User-visible asymmetry preserved (spellbook tray appends " spellbook"; inventory does not; slog ops noun-specific; each `LastKnown*Mtime` map drives its own path) | ✓ VERIFIED | `fk.slogNoun` threaded in 8 ops (stat/open/read/uploaded/empty); `traySuffix` in success + failure tray text; per-kind `mtimeMap` closures at runapp.go:424/436; success format `"Last upload: %s%s at %s"` reproduces both wordings |
| 5 | `walkRoot` finds the sentinel pair at depth 1, 2, 3 under a TempDir root | ✓ VERIFIED | `TestWalkRoot_FindsSentinelPairAtDepth` PASS (table-driven depths 1/2/3) |
| 6 | `walkRoot` does NOT find a pair beyond maxHeuristicDepth (5) — depth cap prunes | ✓ VERIFIED | `TestWalkRoot_BeyondDepthCapNotFound` PASS (6 sub-levels → curDepth 6 > 5) |
| 7 | `walkRoot` does NOT match inside a pruned dir name (node_modules) | ✓ VERIFIED | `TestWalkRoot_PrunedDirNotFound` PASS |
| 8 | `walkRoot` ignores a decoy (only eqgame.exe, missing eqclient.ini) and returns the real pair | ✓ VERIFIED | `TestWalkRoot_DecoyMissingFileIgnoredRealPairFound` PASS |
| 9 | `walkRoot` returns "" on an empty tree with no sentinels | ✓ VERIFIED | `TestWalkRoot_EmptyTreeReturnsEmpty` PASS |
| 10 | The eqfind test runs on the Windows leg and does NOT walk real drives | ✓ VERIFIED | First line `//go:build windows` (count 1); `walkRoot(context`=5, `heuristicScan(`=0, `candidateDrives(`=0, `t.TempDir()`=5; ran locally on Windows box → 5/5 PASS |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/app/runapp.go` | `makeOnFileChange` + `handleIngestErr` (dedup) | ✓ VERIFIED | Both present once; twin bodies replaced by thin wrappers (makeOnInventoryChange/makeOnSpellbookChange each ×1); gofmt-clean; builds |
| `internal/app/runapp_test.go` | 5 spellbook behavior tests | ✓ VERIFIED | All five `TestMakeOnSpellbookChange_*` present; assert on `LastKnownSpellbookMtime` (14 refs); call `makeOnSpellbookChange(context.Background`×5; gofmt-clean |
| `internal/eqfind/heuristic_windows_test.go` | Direct walkRoot tests against TempDir | ✓ VERIFIED | `//go:build windows`; 5 `TestWalkRoot_*`; `plantEQAt` helper; never calls heuristicScan/candidateDrives; gofmt-clean |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| makeOnInventoryChange / makeOnSpellbookChange | makeOnFileChange | thin wrappers passing a fileKind descriptor | ✓ WIRED | runapp.go:419 & :431 both `return makeOnFileChange(...)` with per-kind fileKind |
| makeOnFileChange | handleIngestErr | error-mapping call after bc.Ingest | ✓ WIRED | runapp.go:398 `if handleIngestErr(err, charName, fk.slogNoun, fk.traySuffix, t) { return }` |
| heuristic_windows_test.go | walkRoot | direct call with t.TempDir() root + context.Background() | ✓ WIRED | 5 direct `walkRoot(context.Background(), root)` calls |
| //go:build windows | heuristic_windows.go (windows TU) | build-tag match so walkRoot in scope | ✓ WIRED | tag present; package compiled & tests ran on Windows |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Full app+eqfind packages compile | `go build ./...` | exit 0 | ✓ PASS |
| Inventory regression guard | `go test -run TestMakeOnInventoryChange -v` | 4/4 PASS | ✓ PASS |
| Spellbook behavior tests | `go test -run TestMakeOnSpellbookChange -v` | 5/5 PASS | ✓ PASS |
| walkRoot discovery tests | `go test -run TestWalkRoot -v` | 5/5 PASS | ✓ PASS |
| Full packages green | `go test ./internal/app/... ./internal/eqfind/...` | ok / ok, exit 0 | ✓ PASS |
| gofmt clean | `gofmt -l runapp.go runapp_test.go heuristic_windows_test.go` | no output | ✓ PASS |

### Requirements Coverage

Requirement IDs come from `.planning/CLEAN-CODE-REPORT.md` (Church audit), NOT the v2.2 REQUIREMENTS.md (which tracks WANT-01..08). Cross-referenced against the report as instructed.

| Requirement | Source | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| C1 | CLEAN-CODE-REPORT §C1 | `makeOnSpellbookChange` had ZERO tests | ✓ SATISFIED | 5 spellbook tests cover 204/401/426/409/empty-file branches, all PASS |
| C2 | CLEAN-CODE-REPORT §C2 | `internal/eqfind` real discovery ~15% (walkRoot 0%) | ✓ SATISFIED (partial, per phase scope) | 5 walkRoot tests cover depth/depth-cap/prune/decoy/empty against TempDir; `heuristicScan`/`candidateDrives` end-to-end remain un-unittestable by design (walk real drives) — explicitly scoped out by the plan |
| REFACTOR | CLEAN-CODE-REPORT "Two-birds note" | twin-handler duplication (~50 lines + verbatim switch :355-372 ≡ :419-437) | ✓ SATISFIED | Collapsed to one `makeOnFileChange` + one `handleIngestErr`; error-switch count 2→1; inventory tests green (no behavior change) |

No orphaned requirements: C1/C2/REFACTOR each map to a plan (`requirements:` frontmatter: 24-01 = [C1, REFACTOR], 24-02 = [C2]) and are all satisfied.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| internal/app/runapp_test.go | ~233 | `readAll` helper single unbuffered `Read` (WR-01 from REVIEW) | ℹ️ Info | Latent test-helper bug; no current test asserts on captured body, so spellbook/inventory tests are unaffected. Does not impact phase goal. |
| internal/eqfind/heuristic_windows_test.go | 31-57 | Depth-cap boundary (depth 5 found / 6 pruned) not pinned (WR-02 from REVIEW) | ℹ️ Info | Tests verify depths 1-3 found and depth 6 pruned; exact cap boundary unasserted. Truths 5-6 still verified; an off-by-one in the cap is the only undetected mutation. Test-quality gap, not a goal failure. |
| internal/eqfind/heuristic_windows.go | 59-68 | `pruneNames` gofmt-misaligned (IN-04) | ℹ️ Info | Pre-existing source NOT in this phase's change set; `gofmt -l` on the modified files is clean; `go build`/`go vet` pass. Out of scope. |

No 🛑 Blocker or ⚠️ Warning anti-patterns affecting goal achievement. The 24-REVIEW.md (standard depth, 0 critical) independently diffed the collapsed body line-by-line against the deleted twins and confirmed refactor fidelity.

### Human Verification Required

None. This is a test-hardening + refactor phase with no UI, runtime, or external-service surface. All acceptance criteria are programmatically verifiable via the Go test suite (Windows box, Go 1.26 — the `//go:build windows` walkRoot tests run here) and were confirmed passing.

### Gaps Summary

No gaps. All 10 must-have truths verified, all 3 artifacts pass all four levels (exist, substantive, wired, data-flowing via test execution), all 4 key links wired, full `go build` + `go test` green, and all plan grep acceptance criteria satisfied. The REFACTOR collapsed the verbatim error-switch from 2 occurrences to 1, the inventory regression guard (4 tests) stays green proving no behavior change, and the user-visible inventory/spellbook asymmetry (tray wording, noun-specific slog ops, per-kind mtime maps) is preserved. C2 is closed to the extent scoped (walkRoot is testable; full `heuristicScan` walking real drives is intentionally out of scope per discover.go:108 and the plan).

The three REVIEW warnings are test-quality observations (latent `readAll` truncation, unpinned depth-cap boundary, mixed atomic/slice model in a pre-existing rescanCatchUp test) — none invalidate the verified truths and all are optional hardening for a future pass.

---

_Verified: 2026-06-03T16:10:00Z_
_Verifier: Claude (gsd-verifier)_
