---
phase: 24-watcher-test-hardening-c1-c2-coverage
plan: 01
subsystem: watcher (internal/app upload handlers)
tags: [refactor, test-hardening, dedup, slog, tray]
requires:
  - internal/watch (InventorySuffix/SpellbookSuffix, OnChange)
  - internal/backend (ErrUnauthorized/ErrCrossOwner/ErrVersionTooOld, Client.Ingest)
  - internal/config (LastKnown{Inventory,Spellbook}Mtime, Save)
  - internal/tray (Controller)
provides:
  - makeOnFileChange (single shared upload-handler body)
  - handleIngestErr (extracted error-mapping helper)
  - fileKind descriptor (per-kind token bundle)
  - five TestMakeOnSpellbookChange_* behavior tests
affects:
  - internal/app/runapp.go
  - internal/app/runapp_test.go
tech-stack:
  added: []
  patterns:
    - "descriptor-parameterized shared body to kill intra-file twin duplication"
    - "thread slogNoun to keep slog ops greppable across a shared helper (CLAUDE.md)"
key-files:
  created: []
  modified:
    - internal/app/runapp.go
    - internal/app/runapp_test.go
decisions:
  - "Kept makeOnInventoryChange/makeOnSpellbookChange as thin wrappers so rescanCatchUp + runWatcher stay untouched and the two test suites stay symmetric."
  - "Built the success tray line with a single format string \"Last upload: %s%s at %s\" + traySuffix so traySuffix=\"\" reproduces inventory wording and \" spellbook\" reproduces spellbook wording byte-for-byte (no second format string)."
  - "Threaded slogNoun through handleIngestErr so the 5xx/default arm keeps noun-specific ops (upload inventory/upload spellbook) instead of collapsing to a bare \"upload\"."
metrics:
  duration: ~6 min
  completed: 2026-06-03T15:39:23Z
  tasks: 2
  files: 2
requirements: [C1, REFACTOR]
---

# Phase 24 Plan 01: Watcher upload-handler dedup + spellbook coverage Summary

Collapsed the byte-for-byte twin upload handlers (`makeOnInventoryChange` / `makeOnSpellbookChange`) into one shared `makeOnFileChange` body plus an extracted `handleIngestErr` error-mapping helper, then added five spellbook behavior tests so the spellbook path can no longer rot independently — NO behavior change to the inventory hot path.

## What Was Built

**Task 1 — Refactor (`internal/app/runapp.go`):**
- Added `fileKind` descriptor bundling the five per-kind tokens: `kind`, `suffix`, `slogNoun`, `traySuffix`, `mtimeMap`.
- Extracted the verbatim error switch (was `runapp.go:355-372` ≡ `:419-437`) into `handleIngestErr(err, charName, slogNoun, traySuffix, t) (stop bool)`.
- Folded both handler bodies into a single `makeOnFileChange(...)` that uses `fk.slogNoun` in every slog op (stat/open/read/empty/uploaded), `fk.traySuffix` in the success line, and dereferences `fk.mtimeMap(cfg)` to persist into the correct `LastKnown*Mtime` map.
- Reduced `makeOnInventoryChange` / `makeOnSpellbookChange` to thin wrappers passing a `fileKind`. `rescanCatchUp`, `runWatcher`, `extractCharNameForSuffix`, and `extractCharName` are untouched.

**Task 2 — Tests (`internal/app/runapp_test.go`):**
- Added five `TestMakeOnSpellbookChange_*` tests mirroring the four inventory tests 1:1 plus a new cross-owner case: `_204PersistsMtime`, `_401NoLoopSetsRed`, `_EmptyFileSkipsNoRequest`, `_426UpdateNeeded`, `_409CrossOwnerNoPersist`.
- Reused the existing `ingestRecorder` / `fastBackend` / `withTempLOCALAPPDATA` seams; no new imports.

## Requirements Closed

- **C1** — `makeOnSpellbookChange`'s 204 / 401 / 426 / 409-cross-owner / empty-file branches are now behavior-tested (was ZERO tests).
- **REFACTOR** — the ~50-line twin-handler copy-paste + the verbatim error-switch collapse into `makeOnFileChange` + `handleIngestErr`, with the inventory path's existing tests still green (regression guard).

## Verification

- `go build ./internal/app/...` — succeeds.
- `go test ./internal/app/...` — exits 0 (four pre-existing inventory tests + five new spellbook tests all green).
- `go test ./internal/app/... -run TestMakeOnSpellbookChange -v` — 5 PASS lines.
- `gofmt -l internal/app/runapp.go internal/app/runapp_test.go` — clean (no output).
- Dedup confirmed: `grep -c 'func makeOnFileChange('` == 1, `grep -c 'func handleIngestErr('` == 1, `grep -c 'errors.Is(err, backend.ErrUnauthorized)'` == 1 (was 2 across the twins).
- Asymmetry preserved: `" spellbook"` literal present; success format `"Last upload: %s%s at %s"`; noun-specific slog ops retained via `fk.slogNoun`; thin wrappers `makeOnInventoryChange`/`makeOnSpellbookChange` each present once.

## Deviations from Plan

None — plan executed exactly as written.

## Commits

- `5834a43` refactor(24-01): collapse twin upload handlers into makeOnFileChange + handleIngestErr
- `096b4e4` test(24-01): add five spellbook behavior tests (C1 coverage)

## Self-Check: PASSED
- FOUND: internal/app/runapp.go (makeOnFileChange + handleIngestErr present)
- FOUND: internal/app/runapp_test.go (five TestMakeOnSpellbookChange_* present)
- FOUND: 5834a43
- FOUND: 096b4e4
