---
quick_id: 260621-px6
slug: clean-up-the-dead-wantlist-code
subsystem: api
tags: [dead-code-removal, wantlist, wishlist, svelte, go, store-helpers]

# Dependency graph
requires:
  - phase: 34-wishlist-rework
    provides: the P34 clean break that dropped wantlist_item + unregistered the /api/v1/wantlist routes, orphaning this code
provides:
  - the entire dead item-centric wantlist subgraph removed (frontend + backend)
  - sqliteConstraintUnique + boolToInt relocated to a dedicated store/sqliteconstraint.go (no longer hostage to wantlist.go)
affects: [store, webadmin, web/columns, web/api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared package-private store helpers live in a dedicated file (sqliteconstraint.go), not piggy-backed on a feature file that can be deleted"

key-files:
  created:
    - internal/backendsrv/store/sqliteconstraint.go
  modified:
    - web/src/lib/columns.ts
    - web/src/lib/api.ts
    - web/src/lib/__tests__/api.test.ts
    - internal/backendsrv/store/wishlist.go
    - internal/backendsrv/store/guildchannel.go
  deleted:
    - web/src/lib/components/WantAddForm.svelte
    - web/src/lib/components/cells/WantItemCell.svelte
    - web/src/lib/components/cells/WantMuteCell.svelte
    - web/src/lib/components/cells/WantRemoveCell.svelte
    - web/src/lib/components/cells/InGuildCell.svelte
    - web/src/lib/wantlist/priority.ts
    - web/src/lib/wantlist/priority.test.ts
    - web/src/lib/wantlist/holders.ts
    - web/src/lib/wantlist/holders.test.ts
    - internal/backendsrv/store/wantlist.go
    - internal/backendsrv/webadmin/wantlist.go

key-decisions:
  - "Relocated the 2 LIVE shared helpers (sqliteConstraintUnique, boolToInt) to a new sqliteconstraint.go BEFORE deleting wantlist.go, so assignment.go/guildchannel.go/wishlist.go keep compiling"
  - "Kept CatalogItem + searchCatalog in api.ts (they live in the wantlist block but are REUSED by the wishlist typed-entry add) — surgical, not block-wholesale, removal"
  - "Left the /wantlist→/wishlist redirect route and the alert_log.wantlist_item_id column name untouched — both are LIVE P34 artifacts, out of scope for dead-code removal"

patterns-established:
  - "Pattern: trace the full dead subgraph + identify danger-zone files (live code sharing a file with dead code) BEFORE deleting, then make surgical edits to the shared files"

requirements-completed: []

# Metrics
duration: ~20min
completed: 2026-06-21
---

# Quick Task 260621-px6: Clean up the dead wantlist code Summary

**Removed the entire orphaned item-centric wantlist subgraph (5 dead Svelte components + the wantlist/ lib dir + the dead store/webadmin Go files) left behind by the Phase-34 clean break, surgically preserving every live consumer that shared a file with it — and relocated the two package-shared SQLite helpers to a dedicated sqliteconstraint.go so they outlived their old home.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2 (frontend, backend)
- **Files modified:** 5 modified, 1 created, 11 deleted
- **Commits:** 2 atomic task commits

## Accomplishments
- Deleted 5 dead frontend components (WantAddForm + WantItemCell/WantMuteCell/WantRemoveCell/InGuildCell) and the now-empty `web/src/lib/wantlist/` dir (priority.ts + holders.ts + their tests).
- Surgically trimmed `columns.ts` (dropped wantlistColumns/guildWantlistColumns/prioritySort/guildPrioritySort + now-unused imports incl. PriorityCell) while keeping the 3 live view/gear/spell factories used by `/guild-views`.
- Surgically trimmed `api.ts` (dropped fetchOwnWants/fetchGuildWants/addWant/removeWant/muteWant + WantlistRow/GuildWantRow) while keeping the LIVE CatalogItem/searchCatalog (reused by the wishlist) and the entire wishlist/characters/items/banks/notification/assignment API.
- Relocated `const sqliteConstraintUnique = 2067` + `func boolToInt` verbatim into a new `store/sqliteconstraint.go`, then deleted the dead `store/wantlist.go` + `webadmin/wantlist.go`. The 3 live consumers (assignment.go, guildchannel.go, wishlist.go) keep resolving both helpers.

## Task Commits

1. **Task 1: Frontend dead-code removal + surgical columns.ts/api.ts/api.test.ts edits** - `5026da6` (chore)
2. **Task 2: Backend — relocate shared store helpers, delete dead wantlist Go files** - `1b5bd29` (chore)

## Files Created/Modified
- `internal/backendsrv/store/sqliteconstraint.go` (CREATED) - new home for the 2 package-shared SQLite helpers (sqliteConstraintUnique, boolToInt).
- `web/src/lib/columns.ts` - dropped the 4 wantlist column functions + their now-unused imports; kept viewColumns/gearCheckColumns/spellCheckColumns.
- `web/src/lib/api.ts` - dropped the 5 wantlist wrappers + 2 interfaces; kept CatalogItem/searchCatalog.
- `web/src/lib/__tests__/api.test.ts` - dropped the muteWant import + its /api/v1/wantlist/mute test.
- `internal/backendsrv/store/wishlist.go` - repointed 2 "declared in wantlist.go" doc comments → sqliteconstraint.go.
- `internal/backendsrv/store/guildchannel.go` - repointed 1 "defined in wantlist.go" doc comment → sqliteconstraint.go.

## Decisions Made
- **Surgical, not wholesale.** The `// --- Wantlist + catalog search` block in api.ts mixed dead wantlist wrappers with LIVE `CatalogItem`/`searchCatalog` (reused by the wishlist typed-entry add, WISH-07). Kept the live pair, re-headed the block to reflect its new purpose.
- **Helpers first, deletion second.** Created sqliteconstraint.go and verified `go build ./...` would resolve the helpers before removing wantlist.go — assignment.go/guildchannel.go/wishlist.go would not compile otherwise.
- **PriorityCell import dropped from columns.ts** — it was only referenced by the two removed wantlist factories; `npm run check` confirmed 0 unused-import warnings after removal.

## Deviations from Plan

### Plan assumption corrected (no code impact)

**1. [Rule 3 - Blocking-adjacent] The plan's assumed `*_test.go` files for the dead backend code do not exist**
- **Found during:** Task 2 (backend deletion)
- **Issue:** The plan instructed deleting `store/wantlist_test.go` and `webadmin/wantlist_test.go`. Neither file exists in the repo (verified via `ls internal/backendsrv/{store,webadmin}/*want*` — only `wantlist.go` in each dir).
- **Fix:** Nothing to delete — proceeded with deleting only the two `wantlist.go` files. No test coverage was lost (there was none to lose for these dead files).
- **Files modified:** none (a no-op correction)
- **Verification:** `go test ./...` GREEN — no broken/missing-test references.
- **Committed in:** `1b5bd29` (Task 2 commit; documented in the commit body)

---

**Total deviations:** 1 (a plan-assumption correction — the assumed test files were absent; zero code impact)
**Impact on plan:** None on scope or behavior. Pure dead-code removal as planned.

## Issues Encountered
- A stale doc comment in `web/src/routes/wishlist/+page.svelte` references "the WantAddForm debounce idiom, CLONED" (an idiom reference, not an import). It is NOT a live consumer and is out of scope for this plan's file list, so it was left untouched — `npm run check` is clean (0/0) and the comment harms nothing. Flagged here for completeness.

## Gate Evidence (all green)

**Task 1 (frontend):**
- `npm --prefix web run check` → 497 FILES, 0 ERRORS, 0 WARNINGS
- `npm --prefix web test` → 27 test files, 362 tests passed
- `npm --prefix web run build` → adapter-static "✔ done" (site written to build/)

**Task 2 (backend):**
- `go build ./...` → rc=0
- `go vet ./internal/backendsrv/...` → rc=0 (clean)
- `go test ./...` → ALL `ok` (store 93s, webadmin 63s, readapi 38s, compute 36s, all pass)

**Final cross-checks (plan Acceptance):**
- `grep "/api/v1/wantlist"` (excl. .planning) → only `cmd/squirebot-server/main.go:346` (a "routes were REMOVED" comment) + zero frontend refs. No live calls/handlers.
- `wantlist_item` in production .go → only the LIVE `alert_log.wantlist_item_id` column (kept by 00014, Pitfall 6 option B) + migration tests pinned to pre-00014 schema versions. No live `wantlist_item` table use.
- `sqliteConstraintUnique`/`boolToInt` resolve from `store/sqliteconstraint.go`, consumed by assignment.go/guildchannel.go/wishlist.go.

## Out of Scope (untouched, by design)
- No deploy (these paths were already dead/unreachable as of the v2.4 close — zero runtime change).
- No migration, no schema bump (schema stays v13).
- No `v*` tag. Watcher untouched.
- The `/wantlist`→`/wishlist` redirect route + `alert_log.wantlist_item_id` column name are LIVE P34 artifacts — intentionally preserved.

## Self-Check: PASSED

- FOUND: `internal/backendsrv/store/sqliteconstraint.go` (created)
- FOUND: `260621-px6-SUMMARY.md` (created)
- GONE: `store/wantlist.go`, `webadmin/wantlist.go`, `web/src/lib/wantlist/` dir (deleted)
- FOUND commit `5026da6` (Task 1, frontend)
- FOUND commit `1b5bd29` (Task 2, backend)

---
*Quick task: 260621-px6*
*Completed: 2026-06-21*
