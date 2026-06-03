---
phase: 24-watcher-test-hardening-c1-c2-coverage
plan: 02
subsystem: eqfind
tags: [testing, go, windows, filesystem-walk, coverage]
requires: []
provides:
  - "Direct walkRoot/sentinel-walk test coverage for internal/eqfind (C2)"
affects:
  - internal/eqfind
tech-stack:
  added: []
  patterns:
    - "//go:build windows test that drives a platform-private walker against a t.TempDir() tree"
    - "depth-aware sentinel-pair planter (plantEQAt) extending discover_test.go:makeFakeEQDir"
key-files:
  created:
    - internal/eqfind/heuristic_windows_test.go
  modified: []
decisions:
  - "Test walkRoot DIRECTLY against t.TempDir() roots — never heuristicScan/candidateDrives (would walk real C:/D:/E:)"
  - "Depth-cap case uses 6 sub-levels (curDepth 6 > maxHeuristicDepth 5) so the holding dir is SkipDir'd pre-validation"
metrics:
  duration: ~7m
  completed: 2026-06-03
  tasks: 1
  files: 1
---

# Phase 24 Plan 02: eqfind walkRoot Discovery Coverage Summary

Drives `internal/eqfind`'s real filesystem-walk discovery (`walkRoot` + `ValidateFolder` sentinel matching) directly against synthetic `t.TempDir()` trees, lifting the package off its orchestration-only (~15%) coverage floor onto the actual EQ-folder discovery logic — without ever touching real drives.

## What Was Built

A new `internal/eqfind/heuristic_windows_test.go` (`//go:build windows`) with a depth-aware `plantEQAt` helper and five tests exercising `walkRoot`:

| Test | Asserts |
| ---- | ------- |
| `TestWalkRoot_FindsSentinelPairAtDepth` | pair at depth 1, 2, 3 (within `maxHeuristicDepth`=5) is found |
| `TestWalkRoot_BeyondDepthCapNotFound` | pair 6 levels deep (curDepth 6 > 5) is pruned by the depth cap → `""` |
| `TestWalkRoot_PrunedDirNotFound` | full pair inside a pruned name (`node_modules`) is never matched → `""` |
| `TestWalkRoot_DecoyMissingFileIgnoredRealPairFound` | decoy dir with only `eqgame.exe` (no `eqclient.ini`) is ignored; the real complete pair elsewhere is returned |
| `TestWalkRoot_EmptyTreeReturnsEmpty` | empty tree with no sentinels → `""` |

`plantEQAt(t, root, sub...)` creates `root/sub.../{eqgame.exe,eqclient.ini}` and returns the leaf dir, letting each case bury the install at an arbitrary depth to drive the depth-cap and prune branches. It mirrors `discover_test.go:makeFakeEQDir` but is depth-aware.

## Why It Matters

Closes audit finding C2 (partial, per phase scope): the eqfind orchestration layer (`Discover`) was well-tested with injected probe fakes, but the real `walkRoot` filesystem walk that finds a guildie's EQ folder on a fresh install was 0% covered. `discover.go:108` documents end-to-end `heuristicScan()` as un-unittestable (it enumerates real `C:/D:/E:`); this test partially closes that gap by exercising `walkRoot` — the part that IS testable against a synthetic tree — across its depth-cap, prune, sentinel-match, and miss branches.

## Build-Tag Constraint (load-bearing)

`walkRoot`, `pruneNames`, and `maxHeuristicDepth` exist ONLY in the `//go:build windows` translation unit (`heuristic_windows.go`); `heuristic_other.go` (`//go:build !windows`) defines only a stub `heuristicScan`. The test file therefore carries `//go:build windows` as its first line — without it, the file fails to compile on Linux/macOS (`undefined: walkRoot`). It runs on the Windows CI leg and on the dev's Windows box; `go test ./internal/eqfind/...` exercised it locally and all five tests passed.

## Verification

- `go test ./internal/eqfind/... -run TestWalkRoot -v` → 5/5 PASS
- `go test ./internal/eqfind/...` (full package) → ok (pre-existing discover/validate tests still green)
- `gofmt -l internal/eqfind/heuristic_windows_test.go` → no output (clean)
- `go vet ./internal/eqfind/...` → clean
- Grep acceptance: build-tag count 1, `walkRoot(context` 5, `heuristicScan(` 0, `candidateDrives(` 0, `t.TempDir()` 5

## Deviations from Plan

None - plan executed exactly as written. The depth-cap test's borderline note (adjust sub-level count if Windows path-separator counting straddled the cap differently) did not trigger — 6 sub-levels produced the expected miss on the first run.

## Commits

- `0e97e77`: test(24-02): drive walkRoot directly across depth/prune/decoy cases (C2)

## Self-Check: PASSED

- `internal/eqfind/heuristic_windows_test.go` — FOUND
- commit `0e97e77` — FOUND
