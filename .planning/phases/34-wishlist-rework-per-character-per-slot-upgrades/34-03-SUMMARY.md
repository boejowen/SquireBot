---
phase: 34-wishlist-rework-per-character-per-slot-upgrades
plan: 03
subsystem: ui
tags: [sveltekit, svelte5-runes, wishlist, master-detail, examine, server-truth-writes, owner-scoped, xss, node-vitest]

# Dependency graph
requires:
  - phase: 34-01
    provides: "compute.WishlistView/WishlistSlot/WishlistTarget/WishlistSuggestion snake_case JSON contract (compute/types.go) — the TS interfaces mirror it field-for-field"
  - phase: 34-02
    provides: "GET /api/v1/wishlist/{char} (RequireSession) + POST /api/v1/wishlist//remove//ping (owner-scoped, session-derived owner, IsCharAssignedToTx 403)"
  - phase: 31-characters-tab-in-game-inventory-window
    provides: "ExaminePanel (the single escaped composeItemNote {@html} sink, reused via the asSlot seam) + the /characters viewer-first master-detail shape + roster.ts precedent"
  - phase: 32-inventory-tab-item-centric
    provides: "the /inventory master-detail shape verbatim (the asSlot ExaminePanel reuse seam, the .examine-wrap static override, the ?i= history.replaceState selection idiom) + items.ts node-test precedent"
  - phase: 30-app-shell-5-tab-navigation
    provides: "the /wishlist placeholder + the Notifications region (NotificationPrefsPanel + NotificationInbox, NAV-04) + the /wantlist→/wishlist 308 redirect (kept, verify-only)"
  - phase: 19-wantlist (v2.2)
    provides: "Toggle (role=switch ping toggle) + ConfirmDialog (destructive remove) + WantAddForm debounce/seq-guard idiom (cloned, not imported) + the server-truth onAdded/doRemove/onMute re-fetch discipline + searchCatalog (reused unchanged)"
provides:
  - "web/src/lib/api.ts: WishlistView/WishlistSlot/WishlistTarget/WishlistSuggestion interfaces (mirror the Go contract) + fetchWishlist/addWishlist/removeWishlist/setWishlistPing wrappers"
  - "web/src/lib/wishlist/wishlist.ts: pure node-tested wishlistBandOf/wishlistRoster (banks/bots EXCLUDED viewer-first) + filterWishlistRoster + searchWishlistItems (WISH-07 cross-wishlist grouping over the WHOLE corpus, no scope-down)"
  - "web/src/routes/wishlist/+page.svelte: the per-character per-slot master-detail Wishlist tab (viewer-first list, WISH-07 two-group lazy-fetch search, the 21-slot accordion, target rows + suggestions + ping Toggle + EC-hit badge + reused ExaminePanel, server-truth add/remove/ping, KEEPS the Notifications region)"
  - "deleted WantlistPanel.svelte + web/src/lib/wantlist/groupByChar.ts (+ test); KEPT priority.ts + holders.ts (live consumers)"
affects: [34-04 (deploy-then-browser-smoke the rendered tab across 5 themes)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "WISH-07 cross-wishlist search: lazily fetchWishlist(char) for EVERY non-bank/bot character on the first query, cache in a Record keyed by name, feed Object.values(cache) to the pure searchWishlistItems — NO scope-to-loaded-view escape hatch (a selected char also seeds the cache from its already-fetched view so re-search doesn't re-fetch)"
    - "Per-slot typed-entry add WITHOUT N child components: a single page-level addSlot/addQuery/addResults debounce (the WantAddForm idiom CLONED, not imported — so deleting WantlistPanel doesn't break the add) keyed to the currently-open accordion section"
    - "The asSlot ExaminePanel reuse seam (a representative InventorySlot per target/equipped/suggestion, category:'general', charLastSeen='') — the /inventory + /characters precedent reused verbatim; the .examine-wrap drops the sticky positioning IN THIS TAB"
    - "Server-truth writes (T-34-15): every add/remove/ping awaits the POST then await loadWishlist(selected) — never optimistic; the WISH-07 corpus cache refreshes from the re-fetched view"
    - "Owner gate is presentation-only (T-34-14): ownsSelected = selectedChar.is_mine renders the add/ping/remove controls; the server re-authorizes every write (the read API serves every non-bank/bot char's wishlist by design — the WISH-07 browse leg)"

key-files:
  created:
    - web/src/lib/wishlist/wishlist.ts
    - web/src/lib/wishlist/__tests__/wishlist.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/routes/wishlist/+page.svelte
  deleted:
    - web/src/lib/components/WantlistPanel.svelte
    - web/src/lib/wantlist/groupByChar.ts
    - web/src/lib/wantlist/groupByChar.test.ts

key-decisions:
  - "The add body's REQUIRED character_id (RosterCharacter has no id) is mapped via fetchMyCharacters() name→character_id — loaded alongside fetchCharacters() in the onMount Promise.all; the add control only renders for the viewer's own (is_mine) chars so a non-owned char never reaches the add path"
  - "Accordion sections default to EXPANDED iff the slot has ≥1 target (UI-SPEC §D), collapsed otherwise; a per-slot user toggle overrides for the current selection; the expansion map resets to the default whenever the loaded view changes (new char)"
  - "The api.ts wantlist wrappers (fetchOwnWants/addWant/removeWant/fetchGuildWants/searchCatalog) were LEFT in place — searchCatalog is reused by the wishlist add; fetchGuildWants/GuildWantRow/WantlistRow are consumed by columns.ts/guild-views/InGuildCell; removing them was optional cleanup, skipped to avoid breaking live consumers"
  - "Narrow deletion (Pitfall 7): ONLY WantlistPanel.svelte + groupByChar.ts(+test) deleted; priority.ts (priorityRank → columns.ts; noteRuneCount → WantAddForm) + holders.ts (type Holder → columns.ts + InGuildCell) KEPT — deleting them breaks check+build"

patterns-established:
  - "searchWishlistItems(corpus, query): a pure, immutable, insertion-ordered cross-source group-by-item-name that lists every (char, slot) holding — the caller owns the corpus (no fetch/scope inside the helper)"
  - "A visually-hidden polite aria-live <p class=live> at the page top announces every add/remove/ping result (the v2.2 mute-announce idiom)"

requirements-completed: [WISH-01, WISH-02, WISH-03, WISH-04, WISH-05, WISH-06, WISH-07]

# Metrics
duration: 10min
completed: 2026-06-19
---

# Phase 34 Plan 03: Wishlist Web Tab — Per-Character Per-Slot Master-Detail Summary

**The `/wishlist` SvelteKit tab built to the 34-UI-SPEC: a viewer-first character list (banks/bots excluded), the WISH-07 two-group scoped search whose WISHLIST-ITEMS corpus is EVERY non-bank/bot character's wishlist (lazy-fetch + cache, no scope-down), the 21-slot accordion (equipped + auto-removal-filtered targets + class/slot suggestions + ping Toggle + EC-hit badge + the reused ExaminePanel), and server-truth owner-scoped add/remove/ping — plus the `api.ts` interfaces/wrappers (mirroring the Go WishlistView contract) and a pure node-tested `wishlist.ts`. The superseded WantlistPanel + groupByChar are deleted; priority.ts/holders.ts are kept. NO backend/watcher change; NO deploy (34-04).**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-06-19T04:09:24Z
- **Completed:** 2026-06-19T04:19:04Z
- **Tasks:** 3
- **Files:** 7 changed (2 created, 2 modified, 3 deleted)

## Accomplishments
- **api.ts (Task 1):** the 4 `WishlistView`/`WishlistSlot`/`WishlistTarget`/`WishlistSuggestion` interfaces mirror `compute/types.go` field-for-field (snake_case; `price: number | null`, `item_id: number | null`) + `fetchWishlist` (an OBJECT generic, char `encodeURIComponent`'d) / `addWishlist` (body carries NO owner — session-derived) / `removeWishlist` / `setWishlistPing`. `searchCatalog` reused unchanged for the typed-entry add.
- **wishlist.ts (Task 1, pure + node-tested):** `wishlistBandOf` (two non-bank bands) + `wishlistRoster` (drops every `is_bank_toon || is_guild_bot` row even when `is_mine`, then mine→guild A-Z, a NEW array) + `filterWishlistRoster` (banks/bots-excluded name filter preserving order) + `searchWishlistItems` (the WISH-07 cross-wishlist group-by-item-name over the WHOLE passed-in corpus, listing each (char, slot) — never scopes or fetches). 15 node cases, RED-verified before GREEN.
- **/wishlist/+page.svelte (Task 2):** the per-character per-slot master-detail. LEFT = the scoped search + the two-band viewer-first char list; on a query the left column toggles to two groups — CHARACTERS (`filterWishlistRoster`) + WISHLIST ITEMS (`searchWishlistItems` over the FULL lazily-fetched+cached corpus of every non-bank/bot char's wishlist, with a "Searching all wishlists…" affordance; clicking a result pins THAT char). RIGHT = the detail header + a single pinned reused `<ExaminePanel>` + the D-01 "No targets yet" block + the 21-slot server-ordered accordion: each slot = an `aria-expanded` collapsible header (default-open iff ≥1 target) + the EQUIPPED line (examine-able) + the target rows (name + price + Wiki↗ + `LastSyncedCell` last-listed + Raid tag + ping `Toggle` + "Seen in EC" badge + `--destructive` Remove→`ConfirmDialog`) + (owned only) the cloned-debounce typed-entry add + the suggestion picker (name + price/"Not for sale" + Wiki + last-listed + Raid tag + Add). Every add/remove/ping awaits the POST then re-fetches; a non-owned char renders read-only. The P30 Notifications region (NotificationPrefsPanel + NotificationInbox, NAV-04) stays below the two-pane; the badge store is never imported/mutated. Names via plain `{}`; NO new `{@html}` sink; `?c=` `encodeURIComponent`'d.
- **Deletions (Task 3):** `git rm` `WantlistPanel.svelte` (unmounted after the Task-2 rewrite) + `groupByChar.ts`/`groupByChar.test.ts` (only importer was WantlistPanel). `priority.ts` + `holders.ts` KEPT (live consumers). The `/wantlist`→`/wishlist` 308 redirect confirmed intact (no edit).

## Task Commits

Each task was committed atomically:

1. **Task 1: api.ts interfaces/wrappers + pure wishlist.ts helper + node test** — `ab4a2af` (feat; TDD RED `wishlist` module-missing → GREEN 15/15)
2. **Task 2: rewrite /wishlist/+page.svelte (per-character per-slot master-detail)** — `18fdd61` (feat)
3. **Task 3: delete superseded WantlistPanel + groupByChar (KEEP priority.ts/holders.ts)** — `0e46d65` (feat)

## Files Created/Modified
- `web/src/lib/api.ts` — appended the WishlistView contract interfaces + fetch/mutation wrappers (the `// --- Wishlist (34-03)` block after `removeWant`)
- `web/src/lib/wishlist/wishlist.ts` — the pure banks/bots-excluded viewer-first filter + the WISH-07 cross-wishlist search
- `web/src/lib/wishlist/__tests__/wishlist.test.ts` — 15 node cases (banks/bots-excluded ordering + the cross-char match on a NON-selected char)
- `web/src/routes/wishlist/+page.svelte` — REWRITTEN: the per-character per-slot master-detail tab (replaces the P30 WantlistPanel placeholder)
- `web/src/lib/components/WantlistPanel.svelte` — DELETED (superseded; unmounted after the rewrite)
- `web/src/lib/wantlist/groupByChar.ts` + `groupByChar.test.ts` — DELETED (only importer was WantlistPanel)

## Decisions Made
- All four locked decisions (D-01 clean-break empty state, D-02 server-computed auto-hide [no client filter], D-03 redirect intact, D-04 21 worn slots) applied as specified.
- The WISH-07 corpus is built lazily (first query) + cached, and a selected char seeds the cache from its already-fetched `view` — so re-searching never re-fetches an already-loaded char, yet the corpus is ALWAYS the whole non-bank/bot set (no scope-to-loaded escape hatch).
- The add `character_id` is sourced from `fetchMyCharacters()` (RosterCharacter carries no id), loaded in the onMount `Promise.all` alongside `fetchCharacters()`.
- The WantAddForm debounce idiom was CLONED inline (page-level addSlot/addQuery state) rather than mounting the component — so deleting WantlistPanel (Task 3) leaves the add path intact and the page owns its own per-slot add.

## Deviations from Plan

None — plan executed exactly as written.

The narrow deletion scope, the kept api.ts wantlist wrappers (searchCatalog reuse), and the kept priority.ts/holders.ts are all explicit plan instructions (Task 3 NOTE / Pitfall 7), not deviations.

## Issues Encountered
None. The TDD RED (module-missing) was verified before GREEN; check/test/build stayed green at each task boundary.

## Threat Model — Mitigation Verification
- **T-34-13 (XSS):** MITIGATED — every char/item/slot name + the search query renders via plain `{}` (auto-escaped); grep confirms the ONLY `@html` token in the file is the doc-comment naming the rule — no new `{@html}` directive. The reused ExaminePanel's escaped `composeItemNote` is the single sink. `?c=` is `encodeURIComponent`'d.
- **T-34-14 (UI is not the gate):** ACCEPTED-by-design — the read-only render for a non-owned char is UX; the server re-authorizes every write (the 34-02 `IsCharAssignedToTx` 403); the read API serves every non-bank/bot char's wishlist (the WISH-07 browse leg).
- **T-34-15 (optimistic drift):** MITIGATED — every add/remove/ping awaits the POST then `await loadWishlist(selected)`; the corpus cache refreshes from the re-fetched view.
- **T-34-16 (dead-import build break):** MITIGATED — WantlistPanel + groupByChar deleted AFTER the rewrite unmounted them; grep-clean of residual imports; priority.ts/holders.ts kept so their live consumers still resolve.

## Verification
- `cd web && npm run check` — **0 errors / 0 warnings** (508 files, down 3 after the deletions; the kept priority.ts/holders.ts + every other import resolve).
- `cd web && npm test` — **380 passed (29 files)**, incl. the 15 new `wishlist.ts` cases (banks/bots-excluded viewer-first + WISH-07 cross-char grouping on a non-selected char).
- `cd web && npm run build` — adapter-static build ok (no dangling import from the deletions).
- Grep: no remaining `import` of `WantlistPanel` / `groupByChar` in `web/src`; `$lib/wantlist/priority` + `$lib/wantlist/holders` still imported by columns.ts/InGuildCell/WantAddForm; the `/wantlist`→`/wishlist` 308 redirect intact.
- **DEFERRED to 34-04 (node vitest is DOM-blind):** the list/search/accordion/target-row/ping/suggestion/remove/examine/auto-hide DOM render — browser-smoked on the deployed build across the 5 EQ themes (incl. smoke #10: search finds an item on a DIFFERENT character's wishlist than the selected one).

## User Setup Required
None — no external service configuration. (The tab goes live at the 34-04 deploy; watcher untouched, no migration this plan.)

## Next Phase Readiness
- **34-04 unblocked:** deploy the web build to squirebot.quest + run the 14-point browser-smoke checklist (34-UI-SPEC § Browser-smoke) across the 5 EQ themes — the DOM render, the WISH-07 cross-char item search, the server-truth writes, the owner-scoping read-only render, the auto-hide-when-held (D-02), and the examine are all browser-only verifications. After 34-04, v2.4 is feature-complete → milestone audit/close.
- No git tag (watcher UNTOUCHED — a `v*` tag would needlessly fire the watcher CI).

## Self-Check: PASSED

All created files exist on disk (`wishlist.ts`, `wishlist.test.ts`); the 3 deleted files are gone; all 3 task commits (`ab4a2af`, `18fdd61`, `0e46d65`) are present in git history; `npm run check` 0/0 + `npm test` 380 green + `npm run build` ok.

---
*Phase: 34-wishlist-rework-per-character-per-slot-upgrades*
*Completed: 2026-06-19*
