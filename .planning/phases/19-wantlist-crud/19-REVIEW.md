---
phase: 19-wantlist-crud
reviewed: 2026-06-05T00:00:00Z
depth: deep
files_reviewed: 28
files_reviewed_list:
  - internal/backendsrv/migrations/00006_wantlist.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/wantlist.go
  - internal/backendsrv/store/wantlist_test.go
  - internal/backendsrv/store/itemsearch.go
  - internal/backendsrv/store/itemsearch_test.go
  - internal/backendsrv/webadmin/wantlist.go
  - internal/backendsrv/webadmin/wantlist_test.go
  - internal/backendsrv/readapi/itemsearch.go
  - internal/backendsrv/readapi/itemsearch_test.go
  - cmd/squirebot-server/main.go
  - cmd/squirebot-server/main_test.go
  - web/src/lib/api.ts
  - web/src/lib/columns.ts
  - web/src/lib/components/StateBlock.svelte
  - web/src/lib/components/SiteShell.svelte
  - web/src/lib/components/WantAddForm.svelte
  - web/src/lib/components/WantlistPanel.svelte
  - web/src/lib/components/cells/InGuildCell.svelte
  - web/src/lib/components/cells/PriorityCell.svelte
  - web/src/lib/components/cells/ReasonCell.svelte
  - web/src/lib/components/cells/WantItemCell.svelte
  - web/src/lib/components/cells/WantRemoveCell.svelte
  - web/src/lib/wantlist/holders.ts
  - web/src/lib/wantlist/holders.test.ts
  - web/src/lib/wantlist/priority.ts
  - web/src/lib/wantlist/priority.test.ts
  - web/src/routes/wantlist/+page.svelte
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-06-05
**Depth:** deep
**Files Reviewed:** 28
**Status:** issues_found

## Summary

Reviewed the full Phase 19 (Wantlist CRUD) change set: the migration + store + handlers + read API on the Go side, route wiring in `main.go`, and the SvelteKit wantlist surface (panel, add-form, five cell components, the DOM-free `holders`/`priority` logic, `api.ts`, `columns.ts`). The code is already deployed and passed a human browser-smoke; this review classifies each finding by hotfix-vs-backlog.

**The implementation is strong and the security spine is sound.** I specifically re-verified every cross-AI plan-review fix against the shipped code, and they are all implemented correctly:

- **holdersFor reduce-by-char + SUM (MUST-FIX 1):** Correct. `holders.ts` builds a `Map<char, number>` accumulating `count`, then sorts by `localeCompare`. The node test (`holders.test.ts`) explicitly asserts the multi-row-same-char summing case (`Borticus: 2`), so the P15 node-blind trap is closed.
- **Duplicate → 409 via typed sentinel (MUST-FIX 2):** Correct. `AddWantTx` detects `*sqlite.Error.Code() == 2067` (`SQLITE_CONSTRAINT_UNIQUE`) via `errors.As` and returns `store.ErrDuplicateWant`; `mapWantErr` matches it with `errors.Is`. No string-matching of the driver message. Real-DB tests cover both the catalog and custom-want duplicate paths plus the same-item-other-reason 200 path.
- **"In guild" not "In bank" (MUST-FIX 3):** Correct. `InGuildCell.svelte` renders "In guild" / "Not in guild" / em-dash everywhere; the all-inventory `fetchView()` join is the honest superset.
- **IDOR / owner-from-session (D-02):** Correct and well-tested. Owner is always `caller(ctx)` (= `webauth.UserFromContext`); the request body carries no owner field; `RemoveOwnWantTx` is owner-scoped and a cross-owner remove is a silent `(false, nil)` no-op proven against a real DB. The route-gate test (`main_test.go`) proves anon→401 and member→admitted for all three wantlist routes + item-search.
- **Catalog search SQL injection / V7 (D-10):** Correct. `q` and the built LIKE wildcards are bound as `?` values with `ESCAPE '\'`; `escapeLike` escapes `\`/`%`/`_` (backslash first); `q` is never concatenated and never logged (errors carry `len(q)` only, the handler logs `qlen` only). Literal-wildcard and id-on-NULL-name cases are tested.
- **Defense-in-depth additions:** DB-level `CHECK` constraints on the reason/priority enums (tested to bite), `sql.NullString` scan on the nullable catalog `name`, note trimmed before the 280-rune cap (280 spaces → stored NULL, tested), and zero `{@html}` on any user data (item names, custom labels, notes all render via Svelte `{}` auto-escape).

No blockers. Two warnings and three info items remain, all follow-up/backlog (none warrant a hotfix on the deployed build).

## Warnings

### WR-01: Debounce timer not cleared on unmount (stale fetch after panel teardown)

**File:** `web/src/lib/components/WantAddForm.svelte:35,67`
**Issue:** `debounceTimer` is set with `setTimeout(... DEBOUNCE_MS)` in `onQueryInput` but is never cleared on component destroy. There is no `onDestroy`/cleanup. If the form unmounts during the ~250ms debounce window — e.g. the parent panel collapses because `route(err)` caught a bubbled 401 and re-routed to the LoginScreen, or the user navigates away mid-type — the pending timer still fires `runSearch(q)`, issuing a now-pointless `searchCatalog` fetch and assigning to `$state` on a torn-down component. In Svelte 5 with runes this is benign (no thrown error, the state write is discarded), so it is not a crash, but it is a wasted authenticated request and a latent leak if the debounce window is ever lengthened.
**Fix:** Clear the timer on teardown (and the same guard makes `pick()`/`resetStaging()` fully race-free):
```svelte
import { onDestroy } from 'svelte';
onDestroy(() => {
    if (debounceTimer) clearTimeout(debounceTimer);
});
```
Classification: follow-up backlog item (not a hotfix — no user-visible defect on the deployed build).

### WR-02: Catalog `item_name` snapshot is client-trusted, never re-derived server-side

**File:** `internal/backendsrv/webadmin/wantlist.go:111-114,139-145`
**Issue:** For a catalog want (`item_id` set), the handler stores the client-supplied `item_name` verbatim rather than looking up the canonical name from `pigparse_price` by `item_id`. A crafted request `{"item_id":1001,"item_name":"anything","reason":"buy"}` is accepted and persisted with the bogus label. This is an integrity/trust smell, not a vulnerability: the in-guild join keys on `item_id` (not the name), `WantItemCell` renders the name via `{}` auto-escape (no XSS), and the `item_id` still drives Phase 20+ alert matching. The code comment explicitly documents this as an accepted trade-off (review JUDGMENT-CALL 8). Flagging so it is tracked, not lost — a future surface that treats `item_name` as authoritative (e.g. an alert DM body) would inherit the spoofable label.
**Fix:** When `item_id != nil`, re-derive the snapshot name server-side and ignore the body's `item_name`:
```go
// inside AddWantTx or the handler, for item_id-bearing wants:
//   name := store.CatalogNameByID(ctx, tx, *itemID)  // SELECT name FROM pigparse_price WHERE item_id = ?
//   if name != "" { itemName = name }
```
Classification: follow-up backlog item.

## Info

### IN-01: `pigparse_name_idx` cannot serve the search query (dead index)

**File:** `internal/backendsrv/migrations/00006_wantlist.sql:51-57`
**Issue:** The migration creates `CREATE INDEX pigparse_name_idx ON pigparse_price(name COLLATE NOCASE)`, but `SearchCatalog` uses a leading-wildcard `name LIKE '%…%'` plus `CAST(item_id AS TEXT) = ?` — neither can use a B-tree on `name`, so SQLite full-scans `pigparse_price` regardless. The migration's own comment correctly and honestly documents this (it is NOT presented as a DoS mitigation), so this is informational only. At guild scale (a few thousand catalog rows behind `RequireSession` + a 2-rune-min guard + `LIMIT 25`) the full scan is harmless.
**Fix:** Drop the index (cosmetic), or move to FTS5 only if substring perf ever matters. No action needed now.

### IN-02: `searchLimit` and `NOTE_CAP`/`DEBOUNCE_MS` are local magic numbers (acceptable, noted for parity)

**File:** `internal/backendsrv/readapi/itemsearch.go:27`, `web/src/lib/components/WantAddForm.svelte:27-28`, `internal/backendsrv/webadmin/wantlist.go:77`
**Issue:** The 280-rune note cap exists independently as `NOTE_CAP = 280` (client) and the literal `280` in `validWant` (server); the search limit is `searchLimit = 25` (server) with no client mirror. These agree today and each is a named const at its own layer, but the two `280`s are a duplicated constant that could drift if one side is edited. The client/server note caps and rune-counting (`noteRuneCount` vs `utf8.RuneCountInString`) are correctly aligned today and tested.
**Fix:** None required. If touched later, consider a single shared source of the 280 cap. Informational.

### IN-03: `in_guild` faceted-filter values are coarse strings the user never sees labeled

**File:** `web/src/lib/columns.ts:238-243`
**Issue:** The `in_guild` column's `accessorFn` returns the raw coarse status `'in' | 'na' | 'not'` to back the secondary sort + facet filter, while the cell renders "In guild" / "Not in guild" / em-dash. The faceted `<select>` for this column will therefore show the raw tokens `in`/`na`/`not` as filter options rather than human labels. `enableGlobalFilter:false` correctly prevents phantom global-filter matches, but the facet dropdown UX shows the internal tokens. This is a minor polish gap, not a correctness bug (filtering still works), and the human browser-smoke evidently accepted it.
**Fix:** If the facet dropdown is meant to read nicely, map the coarse value to a label in the facet control, or expose a separate display string. Informational/backlog.

---

_Reviewed: 2026-06-05_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
