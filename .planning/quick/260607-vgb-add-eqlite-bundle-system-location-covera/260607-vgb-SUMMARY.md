---
phase: 260607-vgb
plan: 01
subsystem: watcher / eqfind autodetect
tags: [eqfind, autodetect, eqlite, linux, windows, wine, detection-only]
requires: []
provides:
  - "EQLite bundle autodetect on Linux (WINE drive_c, HOME-relative, system paths)"
  - "EQLite bundle autodetect on Windows (C:\\ literals + %USERPROFILE% incl. Desktop/Downloads)"
  - "Injectable systemKnownHits + systemCandidateRoots package vars (test seams)"
affects: [internal/eqfind]
tech-stack:
  added: []
  patterns: ["build-tag-split detection (_windows / _other)", "injectable package-var test seam", "first-match-wins ValidateFolder gate"]
key-files:
  created:
    - internal/eqfind/knownpaths_other_test.go
    - internal/eqfind/knownpaths_windows_test.go
  modified:
    - internal/eqfind/knownpaths_other.go
    - internal/eqfind/heuristic_other.go
    - internal/eqfind/heuristic_other_test.go
    - internal/eqfind/knownpaths_windows.go
decisions:
  - "EQLite covered name-aware as a real relocatable bundle on BOTH platforms; validation sentinel (eqgame.exe + eqclient.ini) UNCHANGED"
  - "system-root walk shares the single 30s heuristic ctx deadline — no new timeout budget (T-vgb-02)"
metrics:
  duration: ~12m
  completed: 2026-06-07
  tasks: 2
  commits: 2
  files_changed: 6
---

# Phase 260607-vgb Plan 01: EQLite Bundle + System-Location Autodetect Coverage Summary

Widened `internal/eqfind` so a P99 "EQLite" portable bundle is found without the manual folder prompt on BOTH Linux and Windows: Linux now probes EQLite under WINE drive_c, under $HOME (incl. ~/Desktop), and at absolute system paths (/opt/everquest/EQLite etc.); Windows now probes C:\ EQLite literals and %USERPROFILE% EQLite/Desktop/Downloads. The eqgame.exe+eqclient.ini validation sentinel and every scan bound (depth cap 5, 30s timeout, no-symlink, prune lists, first-match-wins, `>` depth boundary) are unchanged. Detection-only — no schema/version/write-contract change.

## What Was Built

### Task 1 — Linux (`_other`) EQLite + system-location coverage (commit f948fb9)
- `knownpaths_other.go`:
  - `subdirs` extended with `"EQLite"` and `everquest/EQLite` (WINE drive_c hits).
  - New HOME-relative block (after the WINE-root loop): `~/EQLite`, `~/Desktop/EQLite`, `~/everquest/EQLite`, ValidateFolder-gated.
  - New injectable package var `systemKnownHits = {/opt/everquest/EQLite, /opt/EQLite, /usr/local/games/EQLite}`, probed last.
  - Ordering preserved (first-match-wins): WINE-prefix subdir hits → HOME hits → systemKnownHits → "".
- `heuristic_other.go`:
  - New injectable package var `systemCandidateRoots = {/opt, /usr/local/games, /games}`.
  - `heuristicScan()` gained a second loop (AFTER the wineCandidateRoots loop, BEFORE `return ""`) that stat-filters each system root to an existing dir, honors the SAME `ctx.Done()` check between roots, and reuses `walkWineRoot` (depth cap 5, prune list, no-symlink, first-match) as-is. `maxHeuristicDepthOther` and the `>` boundary untouched. Catches `/opt/everquest/EQLite` at relative depth 2.
- Tests (build-tagged `!windows`, reuse `plantSentinels` + `isolateWineEnv`):
  - `heuristic_other_test.go` → `TestHeuristicScan_FindsEQUnderSystemRoot` (override `systemCandidateRoots` to a planted tmp tree, assert the depth-2 EQLite hit).
  - NEW `knownpaths_other_test.go` → `TestDefaultKnownPaths_WinePrefixEQLiteDirectHit`, `TestDefaultKnownPaths_HomeDesktopEQLiteHit`, `TestDefaultKnownPaths_SystemKnownHitsOverride`.

### Task 2 — Windows (`_windows`) EQLite coverage (commit add17e9)
- `knownpaths_windows.go`:
  - `candidates` extended with `C:\EQLite`, `C:\everquest\EQLite`, `C:\Games\EQLite`.
  - `%USERPROFILE%` block extended with `EQLite`, `Desktop\EQLite`, `Downloads\EQLite`.
  - First-match-wins + ValidateFolder gate (existing loop) unchanged.
- NEW `knownpaths_windows_test.go` (build-tagged `windows`): `TestDefaultKnownPaths_UserProfileDesktopEQLiteHit` — `t.Setenv("USERPROFILE", tmp)`, plant `tmp/Desktop/EQLite` sentinel pair via a local `plantSentinelPair` helper (the `_other` `plantSentinels` is tagged out on Windows), assert the Desktop hit. RUNS green natively here.

## Verify Gate Results (repo root, this Windows host)

| # | Command | Result |
|---|---------|--------|
| 1 | `go build ./...` | PASS (STEP1_OK) |
| 2 | `GOOS=linux go build ./...` | PASS (STEP2_OK — compiles the `!windows` source) |
| 3 | `go vet ./internal/eqfind/...` | PASS (STEP3_OK) |
| 4 | `GOOS=linux go vet ./internal/eqfind/...` | PASS (STEP4_OK — compiles the `_other` TESTS too) |
| 5 | `go test ./internal/eqfind/... ./internal/app/... ./internal/onboarding/...` | PASS (all `ok`) |
| 6 | `go test ./...` (whole module) | PASS (all packages `ok`/no-test-files; WHOLE_MODULE_EXIT=0) |

The new Windows test (`TestDefaultKnownPaths_UserProfileDesktopEQLiteHit`) executed green in steps 5/6 (eqfind `ok`). No unrelated/pre-existing package failed to build or test.

## KNOWN LIMITATION

The new Linux (`_other`) tests — `TestHeuristicScan_FindsEQUnderSystemRoot`,
`TestDefaultKnownPaths_WinePrefixEQLiteDirectHit`,
`TestDefaultKnownPaths_HomeDesktopEQLiteHit`,
`TestDefaultKnownPaths_SystemKnownHitsOverride` — CANNOT EXECUTE on this Windows
host because they are build-tagged `//go:build !windows`. They were
**compile-verified only**, via `GOOS=linux go vet ./internal/eqfind/...` (verify-gate
step 4). They EXECUTE for real on a Linux box / CI — the same Phase 25 pattern used
for all prior `_other` test files. Do NOT read step 4/the green build as "the
`_other` tests ran green on Windows" — they did not run here. The Windows
(`_windows`) test DID run green natively (steps 5/6).

## OUT OF SCOPE (deliberately not done)

- **No version bump / tagged release.** This task only lands code. Auto-update ships
  the binary later via a tagged `release.yml` run; no deploy, tag, or push was done.
- **Already-configured watchers don't re-detect.** An existing watcher with
  `cfg.eq_folder` already set won't re-run autodetect — this only helps FRESH setups
  (the `--setup` / first-run path that calls `eqfind.Discover()`).
- No `onboarding` / `config` / `cmd` changes; no schema/migration/`WatcherMaxSchemaVersion`
  change (detection logic only).

## Deviations from Plan

None — plan executed exactly as written. All scope guards honored: depth `>` boundary
unchanged, all bounds (depth 5, 30s timeout, no-symlink, prune lists, first-match-wins)
preserved, ordering (WINE → HOME → systemKnownHits) as specified, only `internal/eqfind`
files touched, code-only commits (no `.planning/` staged).

## Known Stubs

None.

## Threat Flags

None — no new network endpoint, auth path, or trust-boundary surface. The system-root
walk inherits the Phase 25 T-25-07 accepted local-FS risk; both new vars are
stat-filtered/short and share the existing 30s ctx deadline (T-vgb-01 accept /
T-vgb-02 mitigate, both already addressed in the implementation).

## Commits

- `f948fb9` feat(eqfind): add EQLite + system-location coverage to Linux autodetect
- `add17e9` feat(eqfind): add EQLite coverage to Windows autodetect known-paths

## Self-Check: PASSED

- internal/eqfind/knownpaths_other.go — FOUND (modified)
- internal/eqfind/heuristic_other.go — FOUND (modified)
- internal/eqfind/heuristic_other_test.go — FOUND (modified)
- internal/eqfind/knownpaths_other_test.go — FOUND (created)
- internal/eqfind/knownpaths_windows.go — FOUND (modified)
- internal/eqfind/knownpaths_windows_test.go — FOUND (created)
- commit f948fb9 — FOUND in git log
- commit add17e9 — FOUND in git log
