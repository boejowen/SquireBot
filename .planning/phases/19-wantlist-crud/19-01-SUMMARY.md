---
phase: 19-wantlist-crud
plan: 01
subsystem: backend-data-layer
tags: [wantlist, sqlite, goose-migration, store, idor, catalog-search, pigparse]
requires:
  - "web_user(discord_user_id) table (00004)"
  - "pigparse_price table (00001 + 00003 enrich columns)"
  - "store.Store + store.NewStore + store.NewTestDB"
provides:
  - "00006_wantlist.sql migration: wantlist_item + alert_log + partial unique indexes + pigparse_name_idx"
  - "store.WantlistRow + AddWantTx + ListOwnWants + RemoveOwnWantTx + ErrDuplicateWant"
  - "store.CatalogItem + (*Store).SearchCatalog over pigparse_price"
affects:
  - "Plan 02 webadmin/wantlist.go handlers (compose over these store funcs)"
  - "Plan 02 readapi item-search handler (composes over SearchCatalog)"
  - "Phase 20 alert_log writer (table created here, empty)"
tech-stack:
  added: []  # zero new dependencies
  patterns:
    - "Typed driver-code sentinel: *sqlite.Error.Code()==2067 (SQLITE_CONSTRAINT_UNIQUE) -> ErrDuplicateWant, never string-matched"
    - "Two partial unique indexes scoped WHERE active=1 (catalog item_id NOT NULL / custom item_id NULL) for NULL-distinct-safe dedupe"
    - "Reason/priority CHECK constraints as DB-level defense-in-depth"
    - "Bound LIKE + ESCAPE '\\' search; COLLATE NOCASE only on ORDER BY tiebreak"
    - "NULL-safe name scan (sql.NullString) so an id-match on a NULL-name row never errors"
key-files:
  created:
    - internal/backendsrv/migrations/00006_wantlist.sql
    - internal/backendsrv/store/wantlist.go
    - internal/backendsrv/store/wantlist_test.go
    - internal/backendsrv/store/itemsearch.go
    - internal/backendsrv/store/itemsearch_test.go
  modified:
    - internal/backendsrv/migrations/migrate_test.go
decisions:
  - "Soft-delete (active=0) not hard DELETE, so Phase 20 alert_log FK never dangles (Pitfall 4)"
  - "Identity keys on web_user.discord_user_id (the person), not an owner entity (Pitfall 3)"
  - "Duplicate detection via the modernc extended result code 2067, not message string-matching (review MUST-FIX 2)"
  - "COLLATE NOCASE only on ORDER BY; LIKE is already ASCII case-insensitive (review WORTH-FIX 7)"
metrics:
  duration: ~25m
  completed: 2026-06-03
  tasks: 3
  files: 6
---

# Phase 19 Plan 01: Wantlist Backend Data Layer Summary

The persistence + query foundation for the personal wantlist: a forward-only goose migration (`wantlist_item` with reason/priority CHECK constraints + two `active=1`-scoped partial unique indexes + a Phase-20 `alert_log` stub + `pigparse_name_idx`), owner-scoped CRUD keyed on `discord_user_id` with a typed `ErrDuplicateWant` sentinel detected via the modernc driver's extended result code 2067, and a NULL-safe full-Blue-catalog `SearchCatalog` over `pigparse_price`. Zero new dependencies; every file clones a verified P14/P17 analog. No HTTP handlers/routes this plan — those compose in Plan 02.

## What Was Built

### Task 1 — `00006_wantlist.sql` migration + `TestMigrate_00006` (commit `64df3aa`)
- `wantlist_item(id, discord_user_id FK→web_user ON DELETE CASCADE, item_id nullable, item_name, reason CHECK IN ('buy','quest'), priority DEFAULT 'med' CHECK IN ('low','med','high'), note, active DEFAULT 1, created_at)`.
- Two **partial unique indexes** both scoped `WHERE active = 1`: `wantlist_catalog_uidx (discord_user_id, item_id, reason) WHERE item_id IS NOT NULL` and `wantlist_custom_uidx (discord_user_id, item_name, reason) WHERE item_id IS NULL` — SQLite's NULL-distinct semantics make this split necessary (Pitfall 1); `active = 1` scoping lets a removed-then-re-added want avoid colliding with its own tombstone.
- `alert_log` created at full Phase-20 shape, **zero rows written** (test asserts `COUNT(*) == 0`).
- `pigparse_name_idx` added with a load-bearing comment that it does NOT serve the leading-wildcard search query (review WORTH-FIX 7) — kept only as cheap insurance for future prefix/exact lookups.
- `TestMigrate_00006` asserts table/column/index existence, an empty `alert_log`, AND that the reason/priority CHECK constraints bite (a `reason='maybe'` and a `priority='urgent'` insert both fail; a valid-enum insert succeeds).

### Task 2 — `store/wantlist.go` owner-scoped CRUD + typed sentinel (commit `a7c1629`)
- `WantlistRow` struct with JSON tags; `ItemID *int64` / `Note *string` so NULLs marshal as JSON `null`.
- `var ErrDuplicateWant` + `const sqliteConstraintUnique = 2067`; a NAMED `sqlite "modernc.org/sqlite"` import (the package previously only blank-imported the driver).
- `AddWantTx` returns the new id; on insert error, `errors.As(err, &sqliteErr) && sqliteErr.Code() == 2067` → `ErrDuplicateWant` (driver-code detection, NOT string-matching the message — review MUST-FIX 2).
- `ListOwnWants` owner-scoped (`WHERE discord_user_id = ? AND active = 1 ORDER BY created_at DESC`), non-nil `make([]WantlistRow, 0)` slice, nullable scan via `sql.Null*` → pointers.
- `RemoveOwnWantTx` IDOR-safe soft-delete: `UPDATE ... SET active = 0 WHERE id = ? AND discord_user_id = ? AND active = 1`; cross-owner remove is a silent `(false, nil)` no-op.
- Tests cover insert, list (owner-scoped + active-only + non-nil empty), soft-remove, the cross-owner IDOR no-op, and dedupe → `ErrDuplicateWant` vs same-item-different-reason (both catalog and custom paths).

### Task 3 — `store/itemsearch.go` full-catalog `SearchCatalog` (commit `1b0f3a1`)
- `CatalogItem{ItemID, Name, CurrentAvg *float64}` (omitempty price); `escapeLike` escapes `\`, `%`, `_`.
- `(*Store).SearchCatalog(ctx, q, limit)` over **`pigparse_price`** (the full Blue catalog — NOT the guild-seen-only table; Pitfall A1) with bound `LIKE ? ESCAPE '\'` substring + `CAST(item_id AS TEXT) = ?` id-equality, prefix-first ranking, `COLLATE NOCASE` only on the ORDER BY tiebreak (the LIKE is already ASCII case-insensitive — review WORTH-FIX 7).
- `name` scanned via `sql.NullString` so an id-match on a NULL-name row returns it without erroring (review WORTH-FIX 4); `current_avg` via `sql.NullFloat64` → `*float64`.
- `q` is bound as a `?` value, never concatenated; error wraps carry `len(q)` only, never `q` (V7).
- Tests cover substring/case-insensitive, prefix-ranked-ahead, id-equality, id-match-on-NULL-name (no error), literal `%` (ESCAPE), the LIMIT cap, empty-corpus → non-nil empty slice, and nullable `current_avg`.

## Deviations from Plan

None — the plan executed exactly as written. Two acceptance-grep wordings (`owner_id`/`UNIQUE constraint failed` in `wantlist.go`; `item_master` in `itemsearch.go`) initially matched explanatory comments that *named the forbidden pattern in order to negate it*. The code never contained the forbidden constructs; the comments were reworded to avoid the literal tokens so the grep gates pass cleanly, with no change to behavior.

## Verification

- `go build ./...` → exit 0.
- `go test -count=1 ./internal/backendsrv/migrations/... ./internal/backendsrv/store/...` → both `ok`.
- `gofmt -l` on all five created/modified-by-this-plan files → clean.
- 00001-00005 migrations untouched (`git diff --name-only` over them → empty).

## Threat Model Coverage

| Threat | Disposition | How met |
|--------|-------------|---------|
| T-19-01 IDOR (remove/list) | mitigate | Owner-scoped `WHERE discord_user_id = ?`; cross-owner remove proven a `(false,nil)` no-op by `TestRemoveOwnWantTx_CrossOwnerSilentNoOp`. |
| T-19-02 SQLi (SearchCatalog) | mitigate | `q` + LIKE terms bound as `?` with `ESCAPE '\'`; grep-gated to 0 concatenation; literal-`%` test proves no wildcard blowup. |
| T-19-03 constraint bypass (NULL item_id / bad enum) | mitigate | Two `active=1` partial unique indexes + reason/priority CHECK constraints (migrate test proves CHECK rejection). |
| T-19-04 unbounded LIKE DoS | accept | `LIMIT` + (Plan 02) `len(q)>=2` guard, behind RequireSession for a 12-person guild. |
| T-19-05 PII/query leak in logs | mitigate | Errors carry `len(q)` only; note text never logged; grep-gated. |

## Deferred Issues

Logged to `.planning/phases/19-wantlist-crud/deferred-items.md`: two PRE-EXISTING gofmt-dirty test files (`store/itemids_test.go`, `store/readviews_test.go`, last touched in Phase 14 commit `0dc3b35`) are out of scope per the SCOPE BOUNDARY rule and were left untouched. All files this plan created/modified are gofmt-clean.

## Self-Check: PASSED

- Created files exist: `00006_wantlist.sql`, `store/wantlist.go`, `store/wantlist_test.go`, `store/itemsearch.go`, `store/itemsearch_test.go` — all FOUND.
- Commits exist: `64df3aa`, `a7c1629`, `1b0f3a1` — all FOUND.
