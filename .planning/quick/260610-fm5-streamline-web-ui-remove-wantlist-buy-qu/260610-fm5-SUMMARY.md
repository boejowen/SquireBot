---
phase: quick-260610-fm5
plan: 01
status: complete
subsystem: ui
tags: [sveltekit, svelte5, go, sqlite, goose, tanstack-table, wantlist]

# Dependency graph
requires:
  - phase: 19 (Wantlist CRUD)
    provides: wantlist_item schema + /api/v1/wantlist endpoints + WantAddForm/WantlistPanel
  - phase: 20 (Bot + DM + Notification Infrastructure)
    provides: NotificationRow/NotificationPrefsPanel + alert_log the dedup key FKs into
  - phase: 26/27 (character assignment + my-characters views)
    provides: COALESCE(character_id,-1) dedup-key term + the home scope filter this restyles
provides:
  - Reason-free wantlist end-to-end (web UI, API contract, store, wantmatch, EC embed)
  - Migration 00011 (dedupe-then-reindex, reason dropped from the unique key, COALESCE pin held)
  - Member-facing UI with zero raw item IDs and friendly notification source/delivery labels
  - WS3 placement consistency (Inventory nav, segmented home scope toggle, DataGrid Filters disclosure, cross-links, ?view= tab seeding, full-width wantlist grids)
affects: [wantlist, notifications, web-ui, deploy (orchestrator must run goose 00011 via the deploy runbook)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Disclosure toggle on DataGrid: aria-expanded button, NO aria-controls id (4 grid instances per page would duplicate a static id)"
    - "Page-intro-as-snippet: route passes its header into a panel component so card vs full-width regions can split (route-scoped styles still apply to snippet markup)"
    - "?view= query param seeds home tab state, validated against the TABS allowlist on mount (T-fm5-04)"
    - "migrations.UpTo(db, version) test-only helper in embed.go for partial-migration tests (external test package cannot reach the embedded FS)"

key-files:
  created:
    - internal/backendsrv/migrations/00011_wantlist_drop_reason_dedup.sql
  modified:
    - internal/backendsrv/store/wantlist.go (AddWantTx drops reason param; INSERT keeps literal 'buy')
    - internal/backendsrv/webadmin/wantlist.go (addWantReq drops Reason; validReasons deleted)
    - internal/backendsrv/wantmatch/match.go + internal/backendsrv/ec/embed.go (Note-only whyWanted, never-empty contract)
    - internal/backendsrv/migrations/embed.go (test-only UpTo helper)
    - web/src/lib/components/WantAddForm.svelte (reason-free add; no item-ID spans)
    - web/src/lib/columns.ts (no reason/id/item_id columns)
    - web/src/lib/components/NotificationRow.svelte (friendly source map; ERROR → NOT SENT)
    - web/src/lib/components/DataGrid.svelte (display-header facet labels; Filters disclosure; 'Filter this table…')
    - web/src/lib/components/SiteShell.svelte (Inventory nav link)
    - web/src/routes/+page.svelte (segmented My characters/Guild toggle; ?view= seeding)
    - web/src/lib/components/WantlistPanel.svelte (add-card + full-width grids; announces grid-adjacent)
    - web/src/routes/wantlist/+page.svelte (intro passed as snippet; card removed from route)
    - web/src/routes/char-meta/+page.svelte + bank-coin/+page.svelte + admin/+page.svelte + my-characters/+page.svelte (purpose lines, cross-links, Admin h1, Back to bank)
    - web/src/lib/components/SettingsMenu.svelte ('Character details' → 'Set class & level')

key-decisions:
  - "ReasonCell.svelte DELETED (sole importer was columns.ts); reason COLUMN retained in SQLite (CHECK-referenced), store writes literal 'buy' forever"
  - "Migrate-test partial-migration mechanism = embed.go test-only UpTo helper (NOT direct goose calls — external test package can't reach the unexported embedded FS, and os.DirFS would test the on-disk files instead of the embedded artifact production runs)"
  - "DataGrid Filters disclosure carries aria-expanded but NO aria-controls (DataGrid mounts 4x per page; a static id would duplicate)"
  - "Wantlist full-width breakout via a children snippet: the route passes its intro into WantlistPanel's 720px add-card; loading/error phases render a bare StateBlock (the home-page idiom)"
  - "NotificationPrefsPanel mute hint sits OUTSIDE the dimming .monitors wrapper — it is navigation, not a monitor setting, so it never dims with master-off"

patterns-established:
  - "Seg-toggle scope filters: .seg/.seg-btn block duplicated per component (Svelte scoped styles; CONTEXT allowed duplication over premature extraction)"

requirements-completed: [QUICK-260610-fm5]

# Metrics
duration: ~4.5h wall clock across 3 executor sessions (Task 1 committed 11:45, Task 2 11:54, Task 3 16:04 CDT)
completed: 2026-06-10
---

# Quick Task 260610-fm5: Streamline Web UI Summary

**Wantlist buy/quest reason removed end-to-end (web + Go API + migration 00011 dedupe-then-reindex), all raw item IDs and enum leaks stripped from member-facing UI, and 9 placement-consistency fixes (Inventory nav, segmented scope toggle, DataGrid Filters disclosure, cross-links, ?view= tab seeding, full-width wantlist grids).**

## Performance

- **Duration:** ~4.5h wall clock, 3 executor sessions (session-limit handoffs after Task 1 and mid-Task 3)
- **Started:** 2026-06-10 (Task 1 commit 16:45:31Z)
- **Completed:** 2026-06-10 (Task 3 commit 21:04:54Z)
- **Tasks:** 3/3
- **Files modified:** 19 (Task 1) + 8 (Task 2) + 11 (Task 3); unique file count lower (columns.ts, WantAddForm, DataGrid touched by multiple tasks)

## Accomplishments

- **WS1 (reason removal):** Adding a wantlist item no longer asks buy/quest; same-item re-add in the same character scope now returns the friendly 409 duplicate error. Migration 00011 dedupes (soft-delete, MIN(id) keeper) then recreates both unique indexes WITHOUT reason while keeping the `COALESCE(character_id,-1)` term. The `reason` column persists (SQLite CHECK constraint); the store writes the literal `'buy'` forever. EC embed's "Why you wanted it" is Note-only with the never-empty `"on your wantlist"` fallback.
- **WS2 (extraneous-info strip):** Zero raw item IDs on member-facing surfaces (view/bank grids, wantlist grids, search results, add form). Notification rows use friendly source labels (EC auction alert / WTS alert / Raid target alert / SquireBot alert) and NOT SENT instead of ERROR. DataGrid facet labels use display headers. Tooltip says 'Quest item'.
- **WS3 (placement consistency):** All 9 items — labeled Inventory nav link (Inventory → Wantlist → Notifications); home scope filter restyled to the wantlist segmented My characters/Guild toggle (default Guild, presentation-only); per-column filters behind a 44px aria-expanded Filters disclosure (default hidden) with global placeholder 'Filter this table…'; mute-bell hint on /notifications linking /wantlist; purpose lines on /char-meta + /bank-coin; h1 'Admin' on /admin; bidirectional /my-characters ↔ /char-meta cross-links + SettingsMenu rename to 'Set class & level'; /bank-coin 'Back to bank' → /?view=bank with allowlist-validated tab seeding; /wantlist intro + add form in the 720px card while filter bar + grids break out full width; remove/mute announces moved directly above the my-wants grid.

## Task Commits

Each task was committed atomically (code only; commit_docs=false — this SUMMARY is uncommitted for the orchestrator):

1. **Task 1: WS1 — remove wantlist buy/quest reason end-to-end** - `e082e7c` (feat)
2. **Task 2: WS2 — strip item IDs and raw enums from member-facing UI** - `9698c57` (refactor)
3. **Task 3: WS3 — placement consistency** - `bf8b31d` (refactor)

## Verification Gates (all green, run after Task 3)

- `npm run check`: **0 errors / 0 warnings** (485 files)
- `npm test`: **306 passed (306)**, 24 files — baseline was 303; Task 2 added 3 (NotificationRow sourceLabel coverage). NB: `npm test -- --run` FAILS on vitest 4.1.7 (script already expands to `vitest --run`; doubled flag errors) — use plain `npm test`.
- `npm run build`: success (adapter-static)
- `go build ./...`: OK; `go vet ./internal/backendsrv/... ./cmd/squirebot-server/...`: OK
- `go test ./internal/backendsrv/...`: all 18 packages ok (402 top-level Test functions; TestMigrate_00006/00010 pass UNCHANGED; TestMigrate_00011_WantlistDropReasonDedup added)

## Migrate-test UpTo Mechanism (Task 1)

**The embed.go test-only helper** (option 2), not direct goose calls: `migrations.UpTo(db *sql.DB, version int64) error` added to `internal/backendsrv/migrations/embed.go`, mirroring RunMigrations' exact `SetBaseFS(embedMigrations)` + `SetDialect("sqlite3")` setup then `goose.UpTo(db, ".", version)`. Rationale: migrate_test.go is an external test package (`migrations_test`) that cannot reach the unexported embedded FS; `os.DirFS(".")` would have tested the on-disk .sql files — a different artifact than the embedded FS production migrates from. TestMigrate_00011(c) drives it: open at v10 → seed cross-reason dups + one char-tagged row → UpTo v11 → assert MIN(id) keepers active=1, dups active=0, char-tagged row survives (COALESCE pin), COUNT(*) unchanged (soft-delete only).

**Dedupe blast radius on a reason-duplicate-free DB:** zero rows deactivated (the dedupe UPDATEs only touch rows that collide once reason leaves the key). The live DB may carry real buy+quest pairs for the same (user, item, char-scope) — those will have their newer row soft-deleted (active=0, alert_log FKs survive). Expected and intended.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 17th AddWantTx call site the audit map missed**
- **Found during:** Task 1 (WS1 reason removal)
- **Issue:** CONTEXT.md listed 16 AddWantTx call sites, all in store/wantlist_test.go; `go vet` caught `seedWant`'s AddWantTx call in `store/alertlog_test.go:24` with the old arity
- **Fix:** Dropped the `"buy"` arg the same way; the helper's raw-SQL path was unaffected
- **Files modified:** internal/backendsrv/store/alertlog_test.go
- **Verification:** go vet + go test ./internal/backendsrv/store/... green
- **Committed in:** e082e7c (Task 1 commit)

**2. [Rule 1 - Bug] Inherited Task 3 working-tree edits referenced .seg/.seg-btn classes with no CSS**
- **Found during:** Task 3 (session-limit handoff review — the prior executor was cut off after writing the home +page.svelte markup but before the styles)
- **Issue:** The segmented My characters/Guild toggle markup used `.seg`/`.seg-btn` classes, but Svelte styles are component-scoped and +page.svelte's style block had no such rules — the toggle would render as unstyled browser-default buttons (svelte-check does NOT flag missing classes)
- **Fix:** Duplicated the WantlistPanel .seg/.seg-btn block into +page.svelte (per CONTEXT: "duplicating the small style block is acceptable") + added a `.seg-btn:disabled` rule for the !hasMine case the wantlist original never needed
- **Files modified:** web/src/routes/+page.svelte
- **Verification:** npm run check 0/0; markup/CSS class names match the WantlistPanel original
- **Committed in:** bf8b31d (Task 3 commit)

### Content-vs-line-number drift (matched on content per plan instruction, no functional impact)

- migrate_test.go "L780" (listed as a comment reword) was actually a functional SQL filter (`AND reason = 'buy' AND active = 1`) — left as-is; still correct since the column persists and seeds write 'buy' (Task 1)
- webadmin/wantlist.go header-comment line numbers off by ~1 — matched on content (Task 1)

### Minor scope note (Task 3, within plan discretion)

- WantlistPanel gained an optional `children: Snippet` prop so the route's intro renders inside the panel's 720px add-card (the only way to keep "intro + WantAddForm in the same card" while the grids escape it — WantlistPanel previously took no props). Markup/CSS-only in effect; one behavioral nuance: during the loading/error phases the page now shows a bare StateBlock without the intro card, matching the home page's existing idiom.

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug), 2 drift notes, 1 discretionary structure note
**Impact on plan:** All fixes necessary for correctness; no scope creep. DEFERRED list untouched (no /char-meta fold-in, no MyCharactersPanel message reshuffle, no AssignmentAdminPanel username join).

## Browser-Smoke Gap (orchestrator: post-deploy)

The web vitest suite is node-only and DOM-blind (P15/P26/P27 precedent — green tests ≠ works in browser). These need a post-deploy browser smoke:

1. **Home segmented My characters/Guild toggle** — scope flip, char drill-down select, disabled state with zero claimed characters
2. **DataGrid Filters disclosure** — default hidden, toggle expands/collapses, filters still apply while hidden, on all 5 grid instances (4 home + wantlist)
3. **?view=bank seeding** — /bank-coin 'Back to bank' lands on the home Bank tab; unknown ?view= values ignored
4. **Full-width wantlist layout** — intro + add form in the 720px card, filter bar + grids full width; remove/mute announces above the grid
5. **Reason-free add flow** — add an item end-to-end (NO 500 — proves the 'buy' literal INSERT against the live DB); re-add the same item → friendly duplicate error
6. **Friendly notification labels** — detail-less inbox rows show 'EC auction alert' etc., never `ec_auction`; failed delivery shows NOT SENT

Also orchestrator-owned: deploy runs goose 00011 against the live DB (docs/backend-deploy.md runbook) — NOT done here.

## Deploy Record (orchestrator, 2026-06-10 ~21:18 UTC)

**DEPLOYED LIVE** per docs/backend-deploy.md:

1. Pushed `e082e7c..bf8b31d` to origin/master.
2. R2 backup BEFORE migration: `squirebot-2026-06-10.db.gz` (522,908 B) confirmed in `r2:squirebot-backups`.
3. Backend: cross-compiled linux/amd64 (12.9 MB) → scp → `.bak` kept → `install` → `systemctl restart`. Journal: **`OK 00011_wantlist_drop_reason_dedup.sql (3.07ms)` → goose version 11**, bot reconnected (guild logged), scheduler jobs:4, listening :8090, service active.
4. Live DB verify: both unique indexes reason-free WITH `COALESCE(character_id,-1)` retained; wants 12 active / 4 inactive; **dedupe pass deactivated 0 rows** (no guildie had a buy+quest pair — nobody's visible wantlist changed).
5. Web: fresh build → tarball → scp → atomic swap with the load-bearing `chmod -R u=rwX,go=rX`.
6. External smoke GREEN: apex 200; fresh hashed entry `start.B2_zCE6i.js` served `text/javascript` (blank-screen canary clear); `/api/v1/assignments/mine`, `/api/v1/wantlist`, `/api/v1/wantlist/guild`, `/api/v1/items/search` all 401 (login-gated, registered).

Remaining: the 6-item browser-smoke checklist above needs a logged-in (Discord) human pass — headless smoke can't authenticate.

## Issues Encountered

- Session-limit handoffs split execution across 3 executors (after Task 1; mid-Task 3). Task 3 inherited uncommitted working-tree edits for SiteShell (complete) and home +page.svelte (markup complete, CSS missing — see deviation 2). Both reviewed, kept, and built upon per handoff instructions.

## Known Stubs

None — all changes wire to live data sources; no placeholder content introduced.

## Self-Check: PASSED

- `internal/backendsrv/migrations/00011_wantlist_drop_reason_dedup.sql` — FOUND
- `web/src/lib/components/cells/ReasonCell.svelte` — DELETED (as planned)
- Commits `e082e7c`, `9698c57`, `bf8b31d` — FOUND on master
- `git status` — no .planning/ files staged or committed; working tree clean except this uncommitted planning dir
- Gates re-verified green in this session (web check 0/0, 306 tests, build OK; go build/vet/test OK)

---
*Quick task: 260610-fm5-streamline-web-ui-remove-wantlist-buy-qu*
*Completed: 2026-06-10*
