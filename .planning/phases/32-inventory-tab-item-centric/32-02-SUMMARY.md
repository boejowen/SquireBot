---
phase: 32-inventory-tab-item-centric
plan: 02
subsystem: web
tags: [sveltekit, svelte5-runes, master-detail, item-centric, examine-reuse, deep-link, viewer-first, node-tested-helper]

# Dependency graph
requires:
  - phase: 32-inventory-tab-item-centric
    plan: 01
    provides: "GET /api/v1/items (RequireSession) → []ItemRollup; the compute/types.go ItemRollup/ItemHolder snake_case JSON contract"
  - phase: 31-characters-tab-in-game-inventory-window
    provides: "ExaminePanel.svelte (the single escaped composeItemNote {@html} sink; charLastSeen='' omits the footer); the live /characters?c=<name> selection seam; StateBlock; the PaperdollSlot .ico colored-tile mechanic; roster.ts viewer-first/filter pattern"
  - phase: 30-app-shell-5-tab-navigation
    provides: "the SiteShell 5-tab frame + the /inventory route placeholder this plan replaces; AuthGate guard context"
provides:
  - "api.ts ItemRollup / ItemHolder TS interfaces (mirror the Go contract field-for-field) + fetchItems() credentialed wrapper"
  - "web/src/lib/items.ts — pure node-tested viewerFirstItems / filterItems / sortHolders"
  - "the rendered item-centric /inventory master-detail tab (ITEM-01/02/03)"
affects: [33 (Banks tab — can reuse the same rollup-list + holders shape), 34 (Wishlist — the same examine-reuse seam + deep-link pattern)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Examine reuse seam: build a representative InventorySlot-shaped object from an ItemRollup (charLastSeen='' omits the per-item footer) so ExaminePanel is reused with ZERO change"
    - "Master-detail on ONE route: ?i=<name> via history.replaceState, single pinned detail (replace-on-click), no per-item route file"
    - "Holder deep-link: an <a href> into the live /characters?c=<encodeURIComponent(name)> P31 seam"
    - "Pure DOM-free items.ts (viewer-first sort / name filter / holder band sort) node-tested; the rendered DOM stays a browser-smoke gap"

key-files:
  created:
    - web/src/lib/items.ts
    - web/src/lib/__tests__/items.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/routes/inventory/+page.svelte

key-decisions:
  - "Inline PigParse price renders as plain text (no ↗ link): the ItemRollup contract carries no per-item PigParse URL, so no PigParse link 'applies' (UI-SPEC §Typography); the value formats via the examine's formatPp posture (Math.round + en-US grouping)"
  - "Detail-header meta = qty/holder summary ONLY; price/wiki ship in the list row + the reused ExaminePanel (UI-SPEC §F sanctions folding the header meta to avoid a triple-rendered name/price/wiki line)"
  - "Icon tile inlined (32px list / 40px detail) reusing the PaperdollSlot .ico colored-gradient + <img onerror> mechanic — extraction was optional (UI-SPEC §Color / planner's discretion); inline keeps scope tight and the contract identical"
  - "TDD task 1 landed as a single feat commit (failing test + implementation together) — the 32-01 TDD convention; the structural/grep acceptance gates are the verification"

requirements-completed: [ITEM-01, ITEM-02, ITEM-03]

# Metrics
duration: 11min
completed: 2026-06-18
---

# Phase 32 Plan 02: Inventory Tab (Item-Centric) — Web Master-Detail Tab Summary

**The SvelteKit web half of the item-centric Inventory tab over `GET /api/v1/items`: the `api.ts` ItemRollup/ItemHolder interfaces + `fetchItems()` wrapper, a pure node-tested `items.ts` (viewer-first sort / name filter / holder band sort), and the rebuilt `/inventory/+page.svelte` master-detail tab — a bespoke viewer-first selectable item list whose detail is the REUSED P31 `ExaminePanel` plus a holders table whose rows deep-link into the live `/characters?c=<name>` window.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-06-18T18:03:19Z
- **Completed:** 2026-06-18T18:14:22Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- `api.ts`: `ItemRollup` + `ItemHolder` TS interfaces mirroring `compute/types.go` field-for-field (snake_case: `name`/`summed_qty`/`holder_count`/`is_mine`/`price`/`prices`/`wiki_url`/`wiki_summary`/`is_quest_item`/`icon_id`/`statsblock`/`holders`; holder `char`/`slot_label`/`qty`/`last_synced`/`is_mine`/`is_bank`), REUSING the existing `PriceDetail` (no redeclare); `fetchItems()` credentialed `getJSON` wrapper over `GET /api/v1/items` ([] on empty, typed 401/403 the AuthGate re-routes).
- `web/src/lib/items.ts`: pure, immutable, DOM-free `viewerFirstItems` (is_mine first, then A-Z) / `filterItems` (viewer-first-preserving name substring; empty → full set; no-match → []) / `sortHolders` (band mine→guild→banks, A-Z within; is_mine wins the bank tie-break). Doc-commented as presentation-only (the T-27-01 negative property — never access control; `is_mine` is server-stamped).
- `web/src/lib/__tests__/items.test.ts`: 12 node cases (viewer-first floats mine then A-Z; case-insensitive within band; new-array immutability; viewer-priority search; empty/whitespace query → full set; no-match → []; holder band order + A-Z + is_mine tie-break + immutability).
- `web/src/routes/inventory/+page.svelte`: replaced the P30 placeholder with the master-detail tab — `onMount` one-shot `fetchItems()` (401/403 → `authGuard`, else error `StateBlock` + Retry; loading/empty states), scoped viewer-priority search (`Search items…` + "Items on your characters match first." hint) → `$derived filterItems`, `?i=<name>` selection via `history.replaceState` (no new route; single pinned detail; replace-on-click D-03a; pre-selected from `?i=` on mount). LEFT = bespoke viewer-first selectable `<button aria-pressed>` rows (single run, no band labels): 32px colored-tile icon (trusted-int `icon_id` `<img onerror>` over the deterministic gradient), item name, `{summed_qty} · {holder_count} holder(s)` headline (accent tabular-nums numbers, dimmed unit word, singular "1 holder"), inline PigParse price (omitted when null, D-09), "Wiki ↗" link (only when `wiki_url`), accent "you" tag when `is_mine`, accent left-border + accent name on `.selected`. RIGHT = the "Pick an item" prompt until selection, then the 40px-icon + name header + the REUSED `<ExaminePanel slot={asSlot} charLastSeen="">` + the HOLDERS table (Character · Where · Qty · Last synced) whose rows `sortHolders` mine→guild→banks and deep-link to `/characters?c=${encodeURIComponent(h.char)}` (44px, full focus/hover), with a defensive "No holders" line.

## Task Commits

1. **Task 1: api.ts ItemRollup/ItemHolder + fetchItems() + pure items.ts + node tests** — `e7314bb` (feat)
2. **Task 2: rebuild /inventory/+page.svelte as the master-detail Inventory tab** — `2d36dbc` (feat)

_Note: Task 1 (tdd) landed as a single feat commit (the failing test + the helper/interfaces together), per the 32-01 TDD convention — the structural + grep acceptance gates are the verification._

## Files Created/Modified
- `web/src/lib/api.ts` — appended the `ItemRollup` + `ItemHolder` interfaces (mirror the Go contract; reuse `PriceDetail`) + the `fetchItems()` wrapper beside `fetchInventory`.
- `web/src/lib/items.ts` — NEW pure `viewerFirstItems` / `filterItems` / `sortHolders` (mirror `roster.ts`; immutable; DOM-free).
- `web/src/lib/__tests__/items.test.ts` — NEW 12-case node vitest (the `roster.test.ts` factory + describe/it idiom).
- `web/src/routes/inventory/+page.svelte` — REBUILT from the P30 placeholder into the master-detail item-centric tab.

## Decisions Made
- **Inline price renders as plain text (no PigParse `↗` link).** The `ItemRollup` contract carries no per-item PigParse URL, so per UI-SPEC §Typography ("the price text itself is NOT a link unless a PigParse URL applies") no link applies — the value formats via `Math.round(price).toLocaleString('en-US')` (the examine `formatPp` posture). The wiki `↗` link, which has a real `wiki_url`, IS rendered.
- **Detail-header meta carries the qty/holder summary ONLY.** Price + wiki appear on the list row and inside the reused `ExaminePanel`; UI-SPEC §F explicitly sanctions folding the header meta to avoid a triple-rendered name/price/wiki line. The header keeps the icon + name `<h2>` + the `{qty} guild-wide across {N} holder(s)` summary (which `ExaminePanel` does NOT render).
- **Icon tile inlined** (32px list / 40px detail) reusing the `PaperdollSlot` `.ico` colored-gradient + `<img onerror>` mechanic + `hueFor` name-derived hue. Extraction into a shared component was optional (UI-SPEC §Color / planner's discretion); inline keeps the diff tight and the behavior contract identical (`icon_id === 0` skips the `<img>`; a load error hides it; the colored tile shows through).
- **The wiki link `stopPropagation`s its click** so following the link doesn't also pin the row's detail (the row is a `<button>` and the link is nested inside it).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Reworded comment prose off the literal `{@html}` / "coming soon" grep-gate tokens**
- **Found during:** Task 2 (grep-gate verification)
- **Issue:** The plan's acceptance gates require `inventory/+page.svelte` to contain NO `@html` directive and NO "coming soon" stub string (grep returns nothing). My security/documentation comments mentioned `{@html}` literally (4 sites) and described the file as replacing the "coming soon" placeholder — the code has neither the directive nor the stub copy in the rendered output, but the literal tokens in the comments tripped the grep gates. This is the same fix-forward the 32-01 backend work and the P31 work applied for the `@html`/`pickPrice` grep gates.
- **Fix:** Reworded the comments to describe the behavior without the literal tokens ("escaped raw-HTML composeItemNote sink"; "the Phase-30 placeholder stub"). No code/behavior change — the template remains `{@html}`-free and the rendered output carries no stub copy.
- **Files modified:** web/src/routes/inventory/+page.svelte
- **Verification:** the negative grep gates (`@html`, `fonts.googleapis`, "coming soon") now return 0; `npm --prefix web run check` 0/0 + `npm --prefix web run build` ok re-confirmed.
- **Committed in:** `2d36dbc` (Task 2 commit — the rewording landed before the commit)

---

**Total deviations:** 1 auto-fixed (1 blocking grep-gate). No behavior change; no scope creep — both tasks landed exactly as designed.

## Threat Surface
No new security surface introduced. The plan's `<threat_model>` mitigations are all in place:
- **T-32-07 (XSS via item/char names):** item + character names render via plain `{}` interpolation (Svelte auto-escapes) in the list rows, the detail header, and the holders table. The ONLY raw-HTML sink is the reused `ExaminePanel`'s escaped `composeItemNote` — this file adds NO new sink (grep-`@html`-returns-nothing acceptance criterion holds).
- **T-32-08 (holder deep-link):** `encodeURIComponent(h.char)` on the `/characters?c=` href.
- **T-32-09 (icon `<img>` src):** `Item_${it.icon_id}.png` uses a TRUSTED INTEGER `icon_id`; `icon_id === 0` skips the `<img>`; a bad id falls back to the colored tile via `onerror`.
- **T-32-10 (search no-results echo):** the query echoes only through `StateBlock kind="no-results"` `{query}` (plain `{}`, auto-escaped).
- **T-32-11 (client filter ≠ access control):** the viewer-first sort/filter is presentation only; documented in `items.ts`.

## Known Stubs
None — the placeholder is fully replaced. The tab renders live data from `GET /api/v1/items`; no hardcoded empty values flow to the UI. (`is_mine` / `price` / `icon_id` / `statsblock` are server-truth from Plan 32-01.)

## Verification
- `npm --prefix web test` — 359 tests / 28 files pass (347 prior + the 12 new `items.test.ts` cases).
- `npm --prefix web run check` — 507 files, 0 errors / 0 warnings.
- `npm --prefix web run build` — adapter-static build ok (wrote site to `build`).
- Grep gates (positive): `fetchItems` (2), `ExaminePanel` (8), `filterItems` (2), `sortHolders` (2), `charLastSeen=""` (4), `aria-pressed` (1), `/characters?c=` (3) all present in `inventory/+page.svelte`; `viewerFirstItems`/`filterItems`/`sortHolders` exported from `items.ts`; `export interface ItemRollup`/`ItemHolder` + `export function fetchItems(` in `api.ts`, with NO second `PriceDetail`.
- Grep gates (negative): `@html`, `fonts.googleapis`, "coming soon" all return 0 in `inventory/+page.svelte`.
- **DOM-blind gap (carried):** node vitest is `environment:node` — the rendered list ordering, selection→pin, holders table, icon fallback, and the holder deep-link are NOT browser-verified here. Their real verification is the Plan **32-03 deploy-then-browser-smoke** on a DEPLOYED build (`npm run dev` can't auth against prod). Backend binary already restarted in 32-01's deploy step registers the route the tab consumes.

## User Setup Required
None — no external service configuration. (Deploy of the rendered tab + the browser-smoke is Plan 32-03 / the established web atomic-swap; the `GET /api/v1/items` route the tab consumes ships with the 32-01 backend binary.)

## Next Phase Readiness
- The item-centric Inventory tab is web-complete and gate-green; it consumes the live `GET /api/v1/items` contract from Plan 32-01. **Plan 32-03 = deploy-then-browser-smoke** (the DOM-blind verification: viewer-first ordering, viewer-priority search, the `{qty} · {N} holders` headline, the inline price omit, selection→pin + replace-on-click, the examine D-08 order + omission, the holders table with viewer's chars first, the holder deep-link opening `/characters?c=`, the icon load + colored-tile fallback, all 5 EQ themes incl. Heavy + Minimalist contrast).
- The examine-reuse seam + the holders-list + deep-link patterns established here are reusable by Phase 33 (Banks tab) and Phase 34 (Wishlist).

## Self-Check: PASSED

All 2 created files (`web/src/lib/items.ts`, `web/src/lib/__tests__/items.test.ts`) + the 2 modified files exist on disk; both task commits (`e7314bb`, `2d36dbc`) are in the git log; this SUMMARY exists.

---
*Phase: 32-inventory-tab-item-centric*
*Completed: 2026-06-18*
