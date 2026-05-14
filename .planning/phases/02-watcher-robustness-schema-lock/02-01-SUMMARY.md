---
phase: 02-watcher-robustness-schema-lock
plan: 01
subsystem: workbook-schema-lock
tags: [schema, scaffold, validate, char_owner, spellbook]
requires:
  - phase-01-complete
  - sheet.Client
  - sheet.EnsureSheet
provides:
  - internal/scaffold.ScaffoldSchemaV1
  - sheet.WorkbookState (3-state enum)
  - sheet.ListSheets
  - sheet.WriteHeaderRow
  - sheet.HideSheet
  - sheet.ReadColumn
  - sheet.AppendRow
  - 13-column UpsertCharOwner
affects:
  - internal/sheet/meta.go
  - internal/sheet/owner.go
  - internal/sheet/ensure_tab.go
  - internal/sheet/scaffold_helpers.go (new)
  - internal/scaffold/scaffold.go (new)
  - internal/scaffold/scaffold_test.go (new)
  - internal/app/runapp.go
  - internal/picker/server.go
  - internal/auth/oauthconfig.go
  - cmd/squirebot/main.go
  - CLAUDE.md
  - .planning/research/ARCHITECTURE.md (gitignored)
  - .planning/research/SUMMARY.md (gitignored)
tech-stack:
  added: []
  patterns:
    - "idempotent scaffold (ListSheets bulk-read → per-tab presence check → create-or-skip)"
    - "three-state validation enum (Empty / Matches / Wrong)"
    - "values.Update single-cell refresh for last_seen on UpsertCharOwner match path"
key-files:
  created:
    - internal/scaffold/scaffold.go
    - internal/scaffold/scaffold_test.go
    - internal/sheet/scaffold_helpers.go
  modified:
    - internal/sheet/meta.go
    - internal/sheet/owner.go
    - internal/sheet/owner_test.go
    - internal/sheet/ensure_tab.go
    - internal/app/runapp.go
    - internal/picker/server.go
    - internal/auth/oauthconfig.go
    - cmd/squirebot/main.go
    - CLAUDE.md
decisions:
  - "ScaffoldSchemaV1 lives in internal/scaffold, not internal/sheet — keeps the sheet client free of policy and lets future code reach it without circular imports."
  - "Helper API exposed on sheet.Client (WriteHeaderRow, HideSheet, ReadColumn, AppendRow) instead of leaking *sheets.Service to scaffold — preserves encapsulation."
  - "_char_owner UPDATE-MATCH path now refreshes last_seen via values.Update on _char_owner!K{row} (single-cell write); first_seen + class + level are preserved across heartbeats."
  - "WorkbookStateMatches is paired with err = ErrSchemaTooNew on the schema-too-new branch — state still says Matches because the workbook IS ours, but the err signals refuse-to-write."
  - "MetaRows/DimensionTabs/ViewTabs are package-level slices, not maps — preserves the locked column order Phase 3+ scripts read by index."
metrics:
  duration: ~2h
  completed: 2026-05-02
---

# Phase 2 Plan 01: watcher-robustness-schema-lock Plan 01 Summary

Schema lock at `_meta.schema_version=1`. Idempotent `ScaffoldSchemaV1` creates every Phase 2 dimension tab + every consolidated mega-tab placeholder + every required `_meta` KV row, even those Phase 2 leaves empty. ValidateWorkbook refactored to a three-state enum so a guildie picking the wrong workbook gets a hard refusal (Pitfall C closed).

## What Shipped

### Task 1 — three-state ValidateWorkbook + bootstrapMeta deleted
**Commit:** `72059b6`

`ValidateWorkbook` returns `(WorkbookState, error)` where state is one of `WorkbookStateEmpty`, `WorkbookStateMatches`, `WorkbookStateWrong`. The Phase 1 inline-bootstrap path is gone — `bootstrapMeta` is deleted entirely; `internal/scaffold.ScaffoldSchemaV1` owns every `_meta` row write. Pitfall C defensive: a workbook with `_meta` rows but no `canonical_id` row returns `WorkbookStateWrong + ErrWrongWorkbook` (refuse rather than overwrite user data). Six unit tests in `meta_test.go` cover the matrix.

Call sites updated:
- `internal/app/runapp.go runWatcher` branches on state.
- `internal/picker/server.go pickerResult` accepts Empty + Matches; rejects Wrong + any error path.

### Task 2 — internal/scaffold package
**Commit:** `f780d05`

`ScaffoldSchemaV1(ctx, *sheet.Client) error` is the single entry point. It ensures:
- 9 hidden `_`-prefixed dimension tabs: `_meta` (2 cols), `_char_owner` (13 cols), `_item_master` (7), `_pigparse` (6), `_wiki_spells` (5), `_wiki_gear_tier` (7), `_quest_items` (4), `_audit` (6), `_status` (6).
- 4 visible consolidated mega-tab placeholders: `view`, `gear_check`, `spell_check`, `bank` (header rows only).
- 13 `_meta` KV rows: `schema_version=1`, `canonical_id=squirebot-v1-workbook-2026`, plus 11 placeholders for refresh timestamps + bank coin slots.

Idempotent — second run: zero AddSheet calls, zero header writes, zero hide-property updates, zero meta-row appends. Cost: 2 read calls (`ListSheets` + `_meta!A:A`).

New helpers exposed on `sheet.Client`: `ListSheets` (bulk title→sheetId map), `WriteHeaderRow`, `HideSheet`, `ReadColumn`, `AppendRow` — all use `valueInputOption=RAW` per OPS-01.

Six tests in `scaffold_test.go`: empty-workbook, headers-match-locked-schema, meta-rows-with-locked-values, idempotent-second-run, partial-scaffold-no-overwrite, hide-on-create.

### Task 3 — UpsertCharOwner extended to 13 columns
**Commit:** `206b06b`

`UpsertCharOwner` signature gained a `watcherVersion` parameter. Insert path appends a 13-column row to `_char_owner!A:M` matching the locked `DimensionTabs[_char_owner]` header exactly (defaults: empty strings for class/level/display_name/discord_handle; `FALSE` for is_bank_toon/is_hidden/is_removed; `blue` for server). Match path (`charName + email both equal`) now refreshes column K (`last_seen`) only via `values.Update` — preserves `first_seen` + everything else. Mismatch path unchanged: `slog.Warn`, no overwrite, `last_seen` NOT touched.

`watcherVersion` plumbing: `auth.BuildConstants` gained `WatcherVersion` field; `cmd/squirebot/main.go` wires `Version` from `-ldflags`; `internal/app/runapp.go` threads `bc` through `runWatcher → makeOnInventoryChange → UpsertCharOwner`.

Six tests in `owner_test.go`: 13-cell append with locked defaults, last_seen-refresh-on-match, log-on-mismatch, server-hard-coded-and-watcher-version-plumbed, creates-tab-if-missing, no-spreadsheet-id-error.

### Task 4 — wire ScaffoldSchemaV1 into runWatcher
**Commit:** `0805482`

`runWatcher` now calls `scaffold.ScaffoldSchemaV1(ctx, sc)` immediately after `ValidateWorkbook` clears Empty or Matches and before tray status / watch.Run. Idempotent — fully-scaffolded workbook costs two reads on every cold start.

### Task 5 — Slot → Level rename
**Commit:** `25c794e`

`CLAUDE.md` spellbook landing-tab column reference now says `Level, Name, _uploaded_at`. Parallel edits to `.planning/research/ARCHITECTURE.md` and `.planning/research/SUMMARY.md` landed on disk but are gitignored (`.planning/` is local-only). Inventory `Slots` (plural) is unchanged.

## Acceptance — Self-Check

```
build      : exit 0    (go build ./...)
vet        : exit 0    (go vet ./...)
tests      : ALL PASS  (go test ./... -count=1)
binary     : built     (go build ./cmd/squirebot/...)
```

Per-task acceptance criteria all green:

| Criterion | Result |
|-----------|--------|
| `grep -c "WorkbookStateEmpty\|WorkbookStateMatches\|WorkbookStateWrong" internal/sheet/meta.go` ≥ 3 | 16 |
| `grep -n "func (c \*Client) ValidateWorkbook(ctx context.Context) (WorkbookState, error)" internal/sheet/meta.go` = 1 | ✓ |
| `grep -c "ValidateWorkbook(ctx)" internal/app/runapp.go` = 1 | ✓ |
| `grep -n "state, vErr := sc.ValidateWorkbook" internal/app/runapp.go` = 1 | ✓ |
| `grep -c 'func bootstrapMeta' internal/sheet/*.go` = 0 | ✓ (zero matches across all sheet files) |
| `grep -c "func ScaffoldSchemaV1" internal/scaffold/scaffold.go` = 1 | ✓ |
| `grep -c "discord_handle" internal/scaffold/scaffold.go` ≥ 1 | 2 |
| `grep -c "schema_version" internal/scaffold/scaffold.go` ≥ 1 | 4 |
| `grep -c "func (c \*Client) ListSheets" internal/sheet/ensure_tab.go` = 1 | ✓ |
| `grep -c "_char_owner!A:M" internal/sheet/owner.go` ≥ 1 | 1 |
| `grep -c "WatcherVersion" internal/auth/oauthconfig.go` ≥ 1 | 2 |
| `grep -c 'WatcherVersion:    Version' cmd/squirebot/main.go` = 1 | ✓ |
| `grep -c "bc.WatcherVersion" internal/app/runapp.go` ≥ 1 | 1 |
| `grep -c '"blue"' internal/sheet/owner.go` ≥ 1 | 3 |
| `grep -c '"FALSE"' internal/sheet/owner.go` ≥ 3 | 3 |
| `grep -c "scaffold.ScaffoldSchemaV1" internal/app/runapp.go` ≥ 1 | 1 |
| `grep -c "WorkbookStateWrong" internal/app/runapp.go` ≥ 1 | 1 |
| `grep -E "spell:.*\\bSlot\\b" CLAUDE.md` returns 0 | ✓ |
| `grep -c "Level, Name, _uploaded_at" CLAUDE.md` ≥ 1 | 1 |
| `grep -E "spell:.*\\bSlot\\b" .planning/research/ARCHITECTURE.md` returns 0 | ✓ |
| `grep -E "spell:.*\\bSlot\\b" .planning/research/SUMMARY.md` returns 0 | ✓ |
| `grep -c "Slots\b" CLAUDE.md` ≥ 1 (inventory column unchanged) | 1 |

## Test Counts

| File | Tests |
|------|-------|
| `internal/sheet/meta_test.go` | 6 (ValidateWorkbook 3-state matrix) + 2 EnsureSheet |
| `internal/sheet/owner_test.go` | 6 (Plan 02-01 rewrite — was 4 in Phase 1) |
| `internal/scaffold/scaffold_test.go` | 6 (new) |

## Schema-Lock State

After ScaffoldSchemaV1 first run, the workbook contains:

- `_meta`: 13 rows including `schema_version=1`, `canonical_id=squirebot-v1-workbook-2026`.
- 12 additional tabs (8 hidden dimension + 4 visible mega-tab placeholders), each with header row matching the locked schema in 02-RESEARCH.md §Pattern 5.
- 9 underscore-prefixed tabs are hidden via `UpdateSheetProperties{Hidden:true}`.

Subsequent ScaffoldSchemaV1 runs are no-ops. Schema is now extend-only — any future column appends at the right edge of an existing tab without bumping `schema_version`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] internal/picker/server.go ValidateWorkbook call site**

The plan's Task 1 listed `internal/app/runapp.go` as the only call site needing update, but `internal/picker/server.go:187` also called the old `ValidateWorkbook(ctx) error` signature. Without updating it, `go build ./...` would fail on master. Fixed inline as part of the Task 1 commit.

- **Found during:** Task 1 build verification.
- **Issue:** Picker would have a compile error after the meta.go refactor.
- **Fix:** Updated picker server's pickerResult handler to read `(state, err)` and accept Empty + Matches while rejecting Wrong + any error.
- **Files modified:** `internal/picker/server.go`.
- **Commit:** `72059b6`.

### Plan-vs-Reality Drift Notes

**A. Plan Task 1 said "Update internal/app/runapp.go's runWatcher (the only caller)". Reality: picker/server.go is also a caller.** Documented above as Rule 3 fix.

**B. Plan Task 2's `<action>` step 1 included a `BankToonName` MetaRow but used the literal `bank_toon_name` key. Implemented as `bank_toon_name` (matching the plan's literal text), confirmed against the 02-CONTEXT.md and 02-RESEARCH.md row enumerations.**

**C. Plan Task 2 step 2 said "If the read returns zero rows → (WorkbookStateEmpty, nil)" — but ListSheets is an UNCONDITIONAL bulk read at the top of ScaffoldSchemaV1, NOT inside a conditional. Implemented as the unconditional pattern (matches the spirit: cheap, single Get; lets the rest of the function compute presence locally).**

**D. The plan's Task 2 acceptance criterion `grep -c "is_hidden\|is_removed" internal/scaffold/scaffold.go` ≥ 2 returned 1 instead of 2 because both column names are on the same line of the DimensionTabs[_char_owner] entry. Both columns ARE scaffolded (verified by the test `TestScaffoldSchemaV1_HeaderRowsMatchLockedSchema`); the literal grep count just happens to land at 1 line (with 2 column names on it). This is a literal-vs-intent mismatch in the acceptance grep, not a missed implementation.**

**E. Helper API design choice: instead of letting scaffold reach into sheet.Client.svc directly (private field), I added five exported helper methods on sheet.Client (`WriteHeaderRow`, `HideSheet`, `ReadColumn`, `AppendRow`, plus `ListSheets`). This is documented in `internal/sheet/scaffold_helpers.go` and preserves encapsulation. The plan was silent on this; recorded as an implementation decision.**

**F. The plan's Task 3 step 6 said `auth.BuildConstants.Validate() does NOT need to require WatcherVersion`. Implemented as: WatcherVersion is added to the struct but Validate() is unchanged (still requires only the four OAuth values). Empty WatcherVersion writes "" into _char_owner.M, which the plan explicitly allows.**

## Known Stubs

None. Every column scaffolded by ScaffoldSchemaV1 is intentionally empty per Phase 2's "scaffold even unused columns" mandate (CONTEXT.md SCHEMA-05 — soft-delete + discord_handle scaffolded though no v1 UI populates them). These are documented intentional placeholders, not stubs blocking the plan's goal.

## TDD Gate Compliance

This plan ran in TDD mode for Tasks 1–3:

- **Task 1 RED:** `45c9fb6 test(02-01): add failing three-state tests for ValidateWorkbook` (pre-existing on master before this execution session — implementation was already underway).
- **Task 1 GREEN:** `72059b6 refactor(02-01): three-state ValidateWorkbook + delete bootstrapMeta`.
- **Task 2 RED+GREEN:** `f780d05` (single combined commit — six new tests + scaffold.go added together; tests exercise the new code).
- **Task 3 RED+GREEN:** `206b06b` (single combined commit — owner_test.go rewritten + owner.go extended together).
- **Task 4:** non-TDD; `0805482`.
- **Task 5:** docs-only; `25c794e`.

The plan was structured `tdd="true"` for the first three tasks but did not require strictly separated RED + GREEN commits per task (the plan's TDD scope was per-plan, and the ValidateWorkbook RED was already committed by an earlier session). Net result: all behaviour is test-covered and verified passing.

## Self-Check: PASSED

Verified:
- `internal/scaffold/scaffold.go` — exists, contains `ScaffoldSchemaV1`, references `_char_owner`, `discord_handle`, `is_hidden`, `is_removed`, `schema_version`.
- `internal/scaffold/scaffold_test.go` — exists, 6 test functions, all pass.
- `internal/sheet/scaffold_helpers.go` — exists, contains `WriteHeaderRow`, `HideSheet`, `ReadColumn`, `AppendRow`.
- `internal/sheet/ensure_tab.go` — contains `ListSheets`.
- `internal/sheet/meta.go` — three-state enum + new ValidateWorkbook signature; bootstrapMeta deleted.
- `internal/sheet/owner.go` — 13-column UpsertCharOwner with watcherVersion param + last_seen refresh on match.
- `internal/auth/oauthconfig.go` — WatcherVersion field added.
- `cmd/squirebot/main.go` — wires `WatcherVersion: Version`.
- `internal/app/runapp.go` — three-state branching + scaffold.ScaffoldSchemaV1 call + bc plumbing.
- `internal/picker/server.go` — three-state branching.
- `CLAUDE.md` — spellbook column says `Level, Name, _uploaded_at`.
- All 5 commits reachable from HEAD: `72059b6`, `f780d05`, `206b06b`, `0805482`, `25c794e`.
