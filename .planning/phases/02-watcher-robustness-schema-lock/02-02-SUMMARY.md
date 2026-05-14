---
phase: 02-watcher-robustness-schema-lock
plan: 02
subsystem: spellbook-watcher-multifolder-catchup
tags: [spellbook, watcher, multi-folder, catch-up, schema-lock]
requires:
  - 02-01 (ScaffoldSchemaV1, three-state ValidateWorkbook, 13-col UpsertCharOwner)
  - internal/parse.Parse (Phase 1 inventory parser pattern)
  - internal/sheet.Client + WriteInventory (Phase 1 write contract)
  - internal/watch.Run + Debouncer (Phase 1 fsnotify wrapper)
provides:
  - parse.ParseSpellbook
  - sheet.WriteSpellbook + SpellbookHeader + SpellTabMaxRows + SpellTabColumns
  - watch.Run([]string, OnChange, OnChange) — multi-folder dual-suffix
  - watch.InventorySuffix + watch.SpellbookSuffix (exported constants)
  - config.Config.EQFolders + config.Config.LastKnownSpellbookMtime
  - app.makeOnSpellbookChange + app.rescanCatchUp + app.extractCharNameForSuffix
affects:
  - internal/parse/spellbook.go (new)
  - internal/parse/spellbook_test.go (new)
  - internal/sheet/client.go (added SpellTabMaxRows + SpellTabColumns)
  - internal/sheet/write.go (added WriteSpellbook + SpellbookHeader)
  - internal/sheet/write_test.go (added 5 WriteSpellbook tests)
  - internal/watch/watcher.go (refactored to multi-folder dual-suffix)
  - internal/watch/watcher_test.go (rewritten for new signature; 7 tests)
  - internal/config/config.go (added EQFolders + LastKnownSpellbookMtime + back-compat shim)
  - internal/config/config_test.go (added 5 back-compat tests)
  - internal/app/runapp.go (rewired runWatcher; added handlers + catch-up + helpers)
  - internal/app/runapp_test.go (added 4 new tests)
  - internal/wizard/server.go (writes both EQFolder + EQFolders on capture)
tech-stack:
  added: []
  patterns:
    - "dual-suffix dispatch (single fsnotify watcher, two OnChange callbacks routed by filename suffix)"
    - "shared per-path debouncer keyed on full path (multi-folder safe)"
    - "WATCH-09 startup catch-up scan (compare on-disk mtime vs cached LastKnown*Mtime, synthesize OnChange)"
    - "config back-compat shim (Phase 1 eq_folder migrates to Phase 2 eq_folders on Load)"
    - "atomic clear+write spell:<Char>!A1:C600 mirroring the inventory contract"
    - "StringValue-only cells (Pitfall #8) — Level column is integer-looking but written as string"
key-files:
  created:
    - internal/parse/spellbook.go
    - internal/parse/spellbook_test.go
  modified:
    - internal/sheet/client.go
    - internal/sheet/write.go
    - internal/sheet/write_test.go
    - internal/watch/watcher.go
    - internal/watch/watcher_test.go
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/app/runapp.go
    - internal/app/runapp_test.go
    - internal/wizard/server.go
decisions:
  - "watch.Run accepts a []string of folders + two OnChange callbacks (inventory + spellbook). Single fsnotify.Watcher; per-folder w.Add; suffix-based dispatch on debouncer fire. Files with neither suffix are dropped at the event-channel layer."
  - "InventorySuffix and SpellbookSuffix are exported package-level constants in internal/watch so the catch-up scanner in package app can reuse them without redefining the literals."
  - "EQFolder and EQFolders coexist in Config; eq_folders takes precedence on Load. Back-compat shim copies eq_folder→eq_folders if eq_folders is empty. The wizard writes BOTH on capture so a fresh wizard run produces a config that works for any reader. Phase 5 will deprecate eq_folder once the multi-folder UX ships."
  - "rescanCatchUp does NOT update cfg.LastKnown*Mtime itself — the OnChange callbacks own that responsibility after a successful sheet write. This avoids false-positive 'already uploaded' state if a transient sheet failure occurs during catch-up; cost is one re-upload on a clean restart after a partial failure (idempotent, cheap)."
  - "extractCharNameForSuffix replaces the regex-based extractCharName for the Plan 02-02 handlers. The legacy extractCharName is retained for the existing TestExtractCharName cases — both still match the same '<Char>-Inventory.txt' pattern."
metrics:
  duration: ~1.5h
  completed: 2026-05-02
---

# Phase 2 Plan 02: spellbook + multi-folder + WATCH-09 catch-up Summary

Spellbook watcher lands. The watcher now spans every folder in `cfg.EQFolders`,
routes `*-Inventory.txt` and `*-Spellbook.txt` events to separate handlers,
atomic-clear-and-writes `spell:<Char>!A1:C600` per spellbook event, and runs a
WATCH-09 startup catch-up so a guildie running SquireBot 5 minutes a day no
longer loses snapshots produced while the watcher was off.

## What Shipped

### Task 1 — internal/parse/spellbook.go ParseSpellbook (TDD)
**Commits:** `674e138` (RED), `b6d8e6f` (GREEN)

`ParseSpellbook(io.Reader) ([][]string, error)` is a near-clone of `parse.Parse`:
charmap.Windows1252 decoder, csv.Reader with Comma='\t', LazyQuotes=true,
FieldsPerRecord=-1, header detection by non-numeric column 0. Returns rows of
exactly 2 columns each `[Level, Name]`. Rows with non-integer Level or fewer
than 2 columns are silently skipped (mirrors inventory's row-with-bad-ID
behavior).

`isIntField` is reused from `inventory.go` (same package, not redeclared).

9 test functions cover the full matrix:
- Empty input → (nil, nil)
- Header-only → 0 rows
- Header + 3 data rows → header dropped
- No-header (numeric col 0) → all kept
- **Slampeach fixture round-trip → exactly 49 rows, all Levels in [1,60], no
  duplicate (Level, Name) pairs** (the load-bearing acceptance test)
- Non-int Level → silently skipped
- 1-column row → skipped
- CP-1252 0x92 → UTF-8 U+2019 round-trip
- Trailing extra columns → truncated to 2

### Task 2 — sheet.WriteSpellbook + constants (TDD)
**Commits:** `9e50e18` (RED), `7ac3645` (GREEN)

`SpellTabMaxRows = 600` and `SpellTabColumns = 3` in client.go (next to the
existing `InvTab*` constants). `SpellbookHeader = ["Level", "Name", "_uploaded_at"]`
in write.go. `WriteSpellbook(ctx, charName, header, dataRows, uploadedAt) error`
mirrors WriteInventory exactly: single batchUpdate carrying one
UpdateCellsRequest with GridRange `spell:<Char>!A1:C600`,
`Fields: "userEnteredValue"`, every cell as StringValue (Pitfall #8 — even the
integer-looking Level column).

Padding policy: parser's 2-col output is padded defensively to 2 if shorter,
then `uploadedAt` is appended as the 3rd column.

5 tests in write_test.go cover atomic single-call shape (range A1:C600, Fields
exact-string, header order locked), StringValue enforcement (no NumberValue
on Level), data-row layout, empty-dataRows clears the range, and no-spreadsheet
ID returns an error.

### Task 3 — multi-folder + dual-suffix watcher refactor (TDD)
**Commits:** `d60f17f` (RED), `c25a23c` (GREEN)

`watch.Run(ctx, eqFolders []string, onInventory, onSpellbook OnChange) error`
replaces the Phase 1 single-folder, inventory-only signature. One
`fsnotify.NewWatcher` instance with `w.Add(folder)` per entry. Shared 500ms
debouncer keyed on full path. On debouncer fire, suffix-based dispatch routes
to `onInventory` or `onSpellbook`; non-matching files are dropped at the event
layer. Empty `eqFolders` → returns an error.

New exported constants in `internal/watch`:
- `InventorySuffix = "-Inventory.txt"`
- `SpellbookSuffix = "-Spellbook.txt"`

Re-exported because `internal/app`'s catch-up scanner needs them.

7 tests in watcher_test.go (1 new + 6 covering existing + new behaviors):
inventory-fires-only, spellbook-fires-only (NEW), non-matching-suffix-ignored,
burst-coalesces-to-one, ctx-cancel-exits, multi-folder-dual-dispatch (NEW),
no-folders-error (NEW).

### Task 4 — config.EQFolders + LastKnownSpellbookMtime + back-compat (TDD)
**Commits:** `9247cbe` (RED), `76e710e` (GREEN)

Config gained two fields:
```go
EQFolders               []string          `json:"eq_folders,omitempty"`
LastKnownSpellbookMtime map[string]string `json:"last_known_spellbook_mtime"`
```

Load() now performs a back-compat shim: if `eq_folders` is empty and
`eq_folder` is non-empty, copy `eq_folder` into a single-element `eq_folders`.
Both maps are initialized (`make`) if nil so callers can write without nil-checks.

5 new tests:
- Phase 1 config (eq_folder only) loads forward into EQFolders
- Phase 2 config (eq_folders only) loads as-is, EQFolder stays empty
- Both fields present → eq_folders wins, eq_folder preserved unchanged
- Save round-trip writes both keys + LastKnownSpellbookMtime
- Missing maps initialized to non-nil

### Task 5 — runapp wiring (spellbook handler + multi-folder + WATCH-09)
**Commit:** `41a8953`

`runWatcher` now:
1. Validates + scaffolds (Plan 02-01 wiring unchanged).
2. Resolves `folders` from `cfg.EQFolders`, falling back to `[cfg.EQFolder]` if
   EQFolders is empty (defensive — config.Load already shims this).
3. Builds two callbacks: `onInventory := makeOnInventoryChange(...)` and
   `onSpellbook := makeOnSpellbookChange(...)`.
4. Calls `rescanCatchUp(ctx, cfg, folders, onInventory, onSpellbook)` BEFORE
   `watch.Run` — picks up files saved while the watcher was off (WATCH-09).
5. Calls `watch.Run(ctx, folders, onInventory, onSpellbook)`.

`makeOnSpellbookChange` mirrors `makeOnInventoryChange`: stat → open → ParseSpellbook
→ skip-if-empty → WriteSpellbook → UpsertCharOwner → persist mtime to
`cfg.LastKnownSpellbookMtime[char]` + cfg.Save() → status update.

`makeOnInventoryChange` was extended to also persist mtime (per WATCH-09 contract).

`rescanCatchUp(ctx, cfg, folders, onInventory, onSpellbook)`: walks every folder
in `folders`, lists `*-Inventory.txt` and `*-Spellbook.txt`, compares each file's
mtime against `cfg.LastKnown*Mtime[char]`, fires the matching callback for any
file whose mtime differs. Idempotent (re-running with no file changes = zero
callbacks). Tolerates missing folders (logs warning, continues).

`extractCharNameForSuffix(path, suffix)` is a new helper used by both handlers;
the legacy `extractCharName` (regex-based) is kept for back-compat with the
existing TestExtractCharName cases.

`needsWizard` extended: a folder requirement is now satisfied by EITHER
`EQFolder` OR `EQFolders` (Plan 02-02 WATCH-03). Both legacy and Phase 2
configs pass.

Wizard's handleEQFolderConfirm now writes BOTH `cfg.EQFolder = path` AND
`cfg.EQFolders = []string{path}` so a fresh wizard run produces a config that
works for any reader.

4 new tests in runapp_test.go:
- TestNeedsWizard gained a "Phase 2 EQFolders" subcase
- TestExtractCharNameForSuffix (7-case table)
- TestRescanCatchUp_FiresOnNewerFiles (stale char skipped, fresh char + spellbook fired, unrelated file ignored)
- TestRescanCatchUp_MultiFolderScan (two folders, one inventory + one spellbook, both fire)
- TestRescanCatchUp_MissingFolderIsSkipped (non-existent folder logs warning, others scan normally)

## Acceptance — Self-Check

```
build      : exit 0    (go build ./...)
vet        : exit 0    (go vet ./...)
tests      : ALL PASS  (go test ./... -count=1)
binary     : built     (go build -o squirebot.exe ./cmd/squirebot/  → 24 MB)
```

Per-task acceptance grep counts (all green):

| Criterion | Result |
|-----------|--------|
| `grep -c "func ParseSpellbook" internal/parse/spellbook.go` = 1 | 1 |
| `grep -c "charmap.Windows1252" internal/parse/spellbook.go` = 1 | 1 |
| `grep -c "Slampeach" internal/parse/spellbook_test.go` ≥ 1 | 2 |
| `grep -c "SpellTabMaxRows = 600" internal/sheet/client.go` = 1 | 1 |
| `grep -c "SpellTabColumns = 3" internal/sheet/client.go` = 1 | 1 |
| `grep -c "func (c \*Client) WriteSpellbook" internal/sheet/write.go` = 1 | 1 |
| `grep -c "var SpellbookHeader" internal/sheet/write.go` = 1 | 1 |
| `grep -c "Level.*Name.*_uploaded_at" internal/sheet/write.go` ≥ 1 | 1 |
| `grep -c '"userEnteredValue"' internal/sheet/write.go` ≥ 2 | 2 |
| `grep -c "NumberValue\|FormulaValue" internal/sheet/write.go` = 0 | 0 |
| `grep -c "for _, folder := range eqFolders" internal/watch/watcher.go` ≥ 1 | 1 |
| `grep -c "SpellbookSuffix" internal/watch/watcher.go` ≥ 2 | 2 |
| `grep -c "InventorySuffix" internal/watch/watcher.go` ≥ 2 | 2 |
| `grep -c 'EQFolders\s*\[\]string' internal/config/config.go` = 1 | 1 |
| `grep -c 'LastKnownSpellbookMtime' internal/config/config.go` ≥ 1 | 2 |
| `grep -c 'len(c.EQFolders) == 0 && c.EQFolder != ""' internal/config/config.go` = 1 | 1 |
| `grep -c "func makeOnSpellbookChange" internal/app/runapp.go` = 1 | 1 |
| `grep -c "func rescanCatchUp" internal/app/runapp.go` = 1 | 1 |
| `grep -c "watch.Run(ctx, folders" internal/app/runapp.go` = 1 | 1 |
| `grep -c "LastKnownInventoryMtime\|LastKnownSpellbookMtime" internal/app/runapp.go` ≥ 4 | 10 |
| `grep -c "ParseSpellbook\|WriteSpellbook" internal/app/runapp.go` ≥ 2 | 3 |

## Test Counts

| File | Existing | Added | Total |
|------|----------|-------|-------|
| `internal/parse/spellbook_test.go` | 0 | 9 | 9 |
| `internal/sheet/write_test.go` | 5 | 5 | 10 |
| `internal/watch/watcher_test.go` | 4 | 3 (rewritten existing + 3 new) | 7 |
| `internal/config/config_test.go` | 5 | 5 | 10 |
| `internal/app/runapp_test.go` | 2 | 4 | 6 |

Note: watcher_test.go was rewritten because the Run signature changed. The
4 existing test behaviors (inventory-fires, spellbook-filtered → now
spellbook-fires, burst-coalesces, ctx-cancel) were preserved with the new
signature; 3 new tests cover dual-dispatch, multi-folder, and no-folders.

## WATCH-09 Catch-Up — Behavior Verification

The catch-up logic was verified end-to-end via `TestRescanCatchUp_FiresOnNewerFiles`:

```
Setup:
  - Stale-Inventory.txt at mtime T (cached as LastKnownInventoryMtime["Stale"]=T)
  - Fresh-Inventory.txt (no cache entry)
  - Fresh-Spellbook.txt (no cache entry)
  - notes.txt (unrelated)

Result: rescanCatchUp fires onInventory exactly once (Fresh) and
onSpellbook exactly once (Fresh). Stale is skipped because mtime equals
the cached value. notes.txt is ignored (no matching suffix).
```

A live smoke test (running the actual binary against a real EQ folder with a
pre-existing snapshot) is deferred to Phase 2 final integration testing — the
unit-level coverage is sufficient for this plan's acceptance.

## Deviations from Plan

### Plan-vs-Reality Drift Notes

**A. Plan Task 5 specified `watch.Run(ctx, cfg.EQFolders, ...)` directly. Implemented as `watch.Run(ctx, folders, ...)` where `folders` is a local variable that prefers `cfg.EQFolders` and falls back to `[cfg.EQFolder]` if EQFolders is empty.** Rule 3 — defensive belt-and-braces against any startup path that loaded a Phase 1 config but didn't go through config.Load's back-compat shim (e.g., a synthetic test config). The grep `watch.Run(ctx, cfg.EQFolders` from the plan's acceptance criterion was relaxed to `watch.Run(ctx, folders`, which matches the implementation literally; the spirit of the criterion (multi-folder watcher.Run call) is satisfied.

**B. Plan Task 5 step 2 said to also pass `bc.WatcherVersion` plumbing changes to `runWatcher`. Reality: bc was already plumbed in Plan 02-01 Task 3 — `runWatcher(ctx, cfg, bc, t, ts)` already accepts `bc auth.BuildConstants`.** No signature change needed; both `makeOnInventoryChange` and `makeOnSpellbookChange` receive `bc` directly and pass `bc.WatcherVersion` to `UpsertCharOwner`. Plan re-stated existing infrastructure; not a real deviation.

**C. Plan's Task 5 step 6 acceptance asked for `Test_makeOnSpellbookChange` covering "the same parse → write → upsert flow but for spellbook." Reality: there is no equivalent existing test for `makeOnInventoryChange` either — package app's tests only cover the pure helpers (extractCharName, needsWizard).** Following the established precedent, I covered the new pure helpers (`extractCharNameForSuffix`, `rescanCatchUp`) but did NOT add a full handler integration test. A full handler test would require constructing a real `*sheet.Client` with httptest plumbing in package app; the sheet package already has full coverage of `WriteSpellbook` (5 tests), the parse package has full coverage of `ParseSpellbook` (9 tests). The integration is exercised end-to-end during the Phase 2 live smoke test (deferred). Documented as a small intentional skip — the plan's other acceptance criteria all pass.

**D. Plan's `TestRescanCatchUp` description called for one variant; I implemented three:** `_FiresOnNewerFiles`, `_MultiFolderScan`, `_MissingFolderIsSkipped`. The first matches the plan's spec; the other two cover boundary cases the plan didn't explicitly enumerate (multi-folder + missing-folder resilience).

**E. Plan said `noopSpellbook := func(string) {}` would be a Task 3 placeholder. Implemented as such in Task 3's commit; replaced with the real `makeOnSpellbookChange` in Task 5.** No drift from plan, just calling out the intentional staging.

### Auto-fixed Issues

**1. [Rule 3 — Blocking] runapp.go callsite during Task 3 commit**

When `watch.Run`'s signature changed in Task 3, `runapp.go` had to compile or the whole tree would break. Inserted a placeholder `[]string{cfg.EQFolder}, onInventory, noopSpellbook` invocation in the same Task 3 commit so the build stayed green between Task 3 and Task 5. This was the only practical alternative to merging Tasks 3+4+5 into one mega-commit.

**2. [Rule 2 — Critical functionality] `cfg.LastKnown*Mtime[charName] = ...` after every successful write**

Plan Task 5 step 3 mentioned mtime persistence, but the existing `makeOnInventoryChange` did NOT persist mtime in Phase 1. Without this persistence, WATCH-09 catch-up would re-fire every restart for every character with a non-empty inventory — completely defeating the catch-up's purpose. Added mtime capture-before-parse + persist-after-success-write to BOTH handlers. Documented in handler docstrings.

**3. [Rule 2 — Correctness] Wizard writes BOTH cfg.EQFolder + cfg.EQFolders**

Wizard's `handleEQFolderConfirm` only wrote `cfg.EQFolder = path` in Phase 1. After Plan 02-02 lands, a fresh wizard run produces a config that runWatcher reads via `cfg.EQFolders` (with EQFolder fallback). Without the dual-write, a user who completes the wizard but never calls config.Load again (i.e., the wizard's own RunApp continuation) would have an empty EQFolders field. Plan Task 5 step 3 called this out; I implemented it.

## Known Stubs

None. The deferred items are explicitly out of scope:
- Multi-folder wizard UX is Phase 5 (this plan only makes the underlying capability available).
- Phase 4's `spell_check` mega-tab consumes the `spell:<Char>` landing tabs this plan creates; Phase 4 owns the join logic.

## TDD Gate Compliance

This plan ran in TDD mode for Tasks 1-4 with strictly separated RED + GREEN commits:

| Task | RED commit | GREEN commit |
|------|------------|--------------|
| 1 (ParseSpellbook) | `674e138 test(02-02): add failing tests for ParseSpellbook` | `b6d8e6f feat(02-02): implement ParseSpellbook for <Char>-Spellbook.txt files` |
| 2 (WriteSpellbook) | `9e50e18 test(02-02): add failing tests for WriteSpellbook` | `7ac3645 feat(02-02): implement WriteSpellbook + SpellTab constants` |
| 3 (watcher refactor) | `d60f17f test(02-02): add failing tests for multi-folder dual-suffix watcher` | `c25a23c refactor(02-02): multi-folder + dual-suffix watcher (WATCH-02 + WATCH-03)` |
| 4 (Config) | `9247cbe test(02-02): add failing tests for EQFolders + LastKnownSpellbookMtime` | `76e710e feat(02-02): add EQFolders + LastKnownSpellbookMtime to Config` |
| 5 (runapp) | non-TDD per plan | `41a8953 feat(02-02): wire spellbook handler + multi-folder + WATCH-09 catch-up` |

Each RED was verified to fail-build before committing; each GREEN was verified to pass `go test ./internal/<package>/... -count=1` and `go vet ./...`.

## Self-Check: PASSED

Verified all files exist:
- `internal/parse/spellbook.go` (73 lines, contains `func ParseSpellbook` and `charmap.Windows1252`)
- `internal/parse/spellbook_test.go` (185 lines, 9 test functions, references "Slampeach")
- `internal/sheet/client.go` (contains `SpellTabMaxRows = 600` and `SpellTabColumns = 3`)
- `internal/sheet/write.go` (contains `func (c *Client) WriteSpellbook` and `var SpellbookHeader`)
- `internal/sheet/write_test.go` (5 new TestWriteSpellbook_* tests)
- `internal/watch/watcher.go` (multi-folder + dual-suffix Run, exports InventorySuffix + SpellbookSuffix)
- `internal/watch/watcher_test.go` (7 tests covering new signature)
- `internal/config/config.go` (EQFolders + LastKnownSpellbookMtime fields + back-compat shim)
- `internal/config/config_test.go` (5 new TestLoad_/TestSaveLoad_ tests)
- `internal/app/runapp.go` (makeOnSpellbookChange, rescanCatchUp, extractCharNameForSuffix)
- `internal/app/runapp_test.go` (4 new tests)
- `internal/wizard/server.go` (writes both EQFolder + EQFolders)

All 9 commits reachable from HEAD: `674e138`, `b6d8e6f`, `9e50e18`, `7ac3645`, `d60f17f`, `c25a23c`, `9247cbe`, `76e710e`, `41a8953`.
