---
phase: 31-characters-tab-in-game-inventory-window
verified: 2026-06-18T12:00:00Z
status: passed
score: 4/4 success criteria verified (7/7 requirement IDs satisfied)
overrides_applied: 0
re_verification:
  previous_status: none
  note: "Initial verification (no prior VERIFICATION.md)."
---

# Phase 31: Characters Tab + In-Game Inventory Window Verification Report

**Phase Goal:** A guildie can find any character and open an inventory window that looks and behaves like the in-game EQ inventory menu — paperdoll equipment slots, general inventory with openable bags, the character's bank below, real wiki item icons, and a right-click-style examine.
**Verified:** 2026-06-18T12:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + plan must_haves)

| #   | Truth (ROADMAP SC) | Status | Evidence |
| --- | ------ | ------ | -------- |
| 1 | Characters tab lists all guild chars (name/level/race/class), viewer-first A-Z → guild → banks/bots; search prioritizes the viewer (CHAR-01/02) | ✓ VERIFIED | `store.RosterFor` (readviews.go:682) — `?`-bound viewer-assignment LEFT JOIN, A-Z `COLLATE NOCASE`; pure Go band-sort. `roster.ts` `bandOf`/`viewerFirst`/`filterRoster` (node-tested, 14 cases). `+page.svelte` 3 bands `YOUR CHARACTERS`/`GUILD`/`BANKS & BOTS`, scoped search. Browser-smoke operator-approved. |
| 2 | Selecting a character opens the in-game window: 23-slot paperdoll + general + bank below, stacked slots show count (CHAR-03/INV-01) | ✓ VERIFIED | `+page.svelte` `?c=` selection → `fetchInventory(selected)` → `<InventoryWindow inventory={inv}>` (line 277). `InventoryWindow.svelte` (484 lines): LEFT/RIGHT/WORN paperdoll, `GENERAL INVENTORY` + `BANK — STORED ITEMS` grids (one renderer), `PaperdollSlot` count badge `count > 1`. See note on the 21-of-23 slot decision below. |
| 3 | A general/bank bag opens to reveal contents that behave like the grid (INV-03) | ✓ VERIFIED | `InventoryWindow.svelte` inline expand keyed on `openBags: Set<location>`, `aria-expanded`, no `scrim`/`modal` (grep 0) — in-flow sub-grid over `slot.children` (D-04). Container detection by `children.length > 0` (smoke fix 2f2c9cf — Slots>0 false-positived on worn gear). |
| 4 | Item icons render from wiki Item_<iconId>.png; hover/tap → click-to-pin examine (name + wiki stats + price + wiki link + last-synced), missing fields omitted (INV-02/INV-04) | ✓ VERIFIED | `PaperdollSlot.svelte` `Item_${iconId}.png` + `onerror` colored-`hsl()`-tile fallback. `examine.ts examineFields` D-08 order + D-09 omission (node-tested). `ExaminePanel.svelte` single escaped `{@html}` via `composeItemNote`/`safeHttpUrl`. statsblock + icon_id flow store→compute→JSON→examine. |

**Score:** 4/4 ROADMAP success criteria verified; 7/7 requirement IDs satisfied.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00012_item_icon.sql` | extend-only icon_id column | ✓ VERIFIED | `ALTER TABLE item_master ADD COLUMN icon_id INTEGER` (nullable, no DEFAULT/UNIQUE). Migrate test present. |
| `internal/backendsrv/migrations/00013_item_statsblock.sql` | extend-only statsblock column (smoke addition) | ✓ VERIFIED | `ALTER TABLE item_master ADD COLUMN statsblock TEXT`. Both deployed live. |
| `internal/backendsrv/enrich/wikiitem.go` | lucy_img_ID + statsblock parse | ✓ VERIFIED | `parseIconID` (atoi, 0 on bad/negative — T-31-01/15 type safety); `cleanStatsblock`; `ParsedWikiItem.IconID`/`.Statsblock`. |
| `internal/backendsrv/store/readviews.go` | RosterFor + icon_id/statsblock SELECT | ✓ VERIFIED | `RosterFor` (?-bound), `im.icon_id`/`im.statsblock` in `InventoryForChar` SELECT/scan. |
| `internal/backendsrv/compute/types.go` + `inventory.go` | InventorySlot.icon_id/statsblock + CharacterInventory.last_seen | ✓ VERIFIED | append-only JSON tags; `slotFromRow` copies IconID/Statsblock; `inv.LastSeen = rows[0].LastSeen`. |
| `internal/backendsrv/readapi/inventory.go` | GET /api/v1/inventory/{char} | ✓ VERIFIED | `PathValue("char")` → `compute.StructuredInventory`; nil→[] (empty-not-404); V7 logging. |
| `internal/backendsrv/readapi/characters.go` | GET /api/v1/characters | ✓ VERIFIED | `UserFromContext` → `RosterFor`; pre-sized []→`[]`; snake_case contract. |
| `web/src/lib/api.ts` | fetchCharacters/fetchInventory + interfaces | ✓ VERIFIED | both wrappers over `getJSON`; `encodeURIComponent(char)`; RosterCharacter/InventorySlot/CharacterInventory interfaces. |
| `web/src/lib/roster.ts` | pure viewer-first sort/filter | ✓ VERIFIED | `bandOf`/`viewerFirst`/`filterRoster`, type-only import, node-tested. |
| `web/src/lib/examine.ts` | pure D-08 order + omission | ✓ VERIFIED | `examineFields`; uses `slot.statsblock` for the stats line; node-tested. |
| `web/src/lib/components/PaperdollSlot.svelte` (227 ln) | icon + onerror fallback + count + bag marker | ✓ VERIFIED | substantive; `Item_${`, `onerror`, `aria-label`, `hsl()` tile, count badge, `aria-expanded`. |
| `web/src/lib/components/ExaminePanel.svelte` (173 ln) | D-08 rows + single escaped {@html} | ✓ VERIFIED | substantive; `composeItemNote`+`safeHttpUrl`; exactly one `{@html}` (composed escaped body). |
| `web/src/lib/components/InventoryWindow.svelte` (484 ln) | generic prop-driven window | ✓ VERIFIED | substantive; PaperdollSlot+ExaminePanel; 3 eyebrows; inline expand; no scrim/modal/@html. |
| `web/src/routes/characters/+page.svelte` (448 ln) | tab wired to window | ✓ VERIFIED | substantive; `InventoryWindow` rendered, `fetchInventory()` called, `No inventory synced yet` + `Pick a character` states; Phase-30 placeholder removed. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| main.go | readapi.NewInventory/NewCharacters | `RequireSession(db, …)` route registration | ✓ WIRED | main.go:362-363, both `RequireSession`-wrapped; no `RequireOfficer`. |
| inventory.go | compute.StructuredInventory | `PathValue("char")` dispatch | ✓ WIRED | inventory.go:57-59. |
| characters.go | store.RosterFor | viewer id from UserFromContext | ✓ WIRED | characters.go:71-73. |
| enrich/jobs/wiki.go | store.ItemMaster.IconID/Statsblock | upsert literal | ✓ WIRED | wiki.go:248-249 (statsblock omission fixed in b138e59). |
| readviews.go InventoryForChar | item_master.icon_id/statsblock | `im.icon_id`/`im.statsblock` SELECT | ✓ WIRED | readviews.go:290, scan 339-340. |
| +page.svelte | InventoryWindow | `fetchInventory(selected)` → `<InventoryWindow inventory={inv}>` | ✓ WIRED | +page.svelte:150,277. |
| PaperdollSlot.svelte | wiki Item_<iconId>.png | `<img onerror>` colored-tile fallback | ✓ WIRED | PaperdollSlot.svelte:112-115. |
| ExaminePanel.svelte | composeItemNote/safeHttpUrl | the single sanctioned {@html} sink | ✓ WIRED | ExaminePanel.svelte:42-44,62. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| /characters list | `roster` | `fetchCharacters()` → `RosterFor` SQL over `character` + `character_assignment` | Yes (real DB query, viewer-bound) | ✓ FLOWING |
| InventoryWindow | `inventory` | `fetchInventory(char)` → `compute.StructuredInventory` → `InventoryForChar` SQL | Yes (real DB join over inventory_item + item_master) | ✓ FLOWING |
| PaperdollSlot icon | `icon_id` | item_master.icon_id (lucy_img_ID enrichment) | Yes (parsed + backfilled by weekly job) | ✓ FLOWING |
| ExaminePanel stats | `statsblock` | item_master.statsblock (cleanStatsblock) | Yes (parsed + persisted; backfill logic re-writes pre-column rows) | ✓ FLOWING |
| ExaminePanel last-synced | `inventory.last_seen` | character.last_seen (NOT slot.last_listed — Pitfall 2 honored) | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Go backend suite | `go test ./internal/backendsrv/...` | all packages ok (compute/enrich/readapi/store/migrations) | ✓ PASS |
| Web unit suite | `cd web && npm test` | 27 files, 347 tests passing | ✓ PASS |
| Web build | `cd web && npm run build` | adapter-static built in 8.62s, site → build/ | ✓ PASS |
| Migration files sequential | Glob 0001*.sql | 00012 + 00013 are the latest, after 00011 | ✓ PASS |
| Both read routes session-gated | grep main.go | 2 RequireSession matches, 0 RequireOfficer | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| CHAR-01 | 31-02, 31-03 | Guild char list w/ meta, viewer-first → guild → banks/bots | ✓ SATISFIED | RosterFor + roster.ts + 3-band list |
| CHAR-02 | 31-02, 31-03 | Per-char search prioritizing the viewer | ✓ SATISFIED | filterRoster preserves viewer-first ranking |
| CHAR-03 | 31-02, 31-03, 31-04 | Selecting a char opens its inventory window | ✓ SATISFIED | `?c=` selection → fetchInventory → InventoryWindow render |
| INV-01 | 31-02, 31-04 | In-game window: paperdoll + general + bank, stacked counts | ✓ SATISFIED | InventoryWindow paperdoll + grids + count badge |
| INV-02 | 31-01, 31-02, 31-04 | Hover/tap examine: name+stats+price+wiki+last-synced | ✓ SATISFIED | examine.ts D-08, statsblock stats line, ExaminePanel |
| INV-03 | 31-02, 31-04 | Bags open to grid-like contents | ✓ SATISFIED | inline expand over children, no modal |
| INV-04 | 31-01, 31-02, 31-04 | Item icons from P1999 wiki | ✓ SATISFIED | parseIconID → icon_id → Item_<id>.png + colored fallback |

All 7 requirement IDs from the PLAN frontmatter cross-reference cleanly to REQUIREMENTS.md (line 105 maps Phase 31 → exactly these 7). No orphaned requirements. (INV-05/DATA-01/02 belong to Phase 29 and are out of this phase's scope.)

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| +page.svelte | 3, 214, 322 | "placeholder"/"coming soon" | ℹ️ Info | False positives: a comment describing the *replaced* placeholder, the `placeholder=` HTML attr, and the `::placeholder` CSS selector. No stub. |

No blocker or warning anti-patterns. No TODO/FIXME, no `return null` stubs, no hardcoded empty data flowing to render, no orphaned components.

### Notable Deviation (intentional, operator-approved)

**21-of-23 paperdoll slots rendered (Charm + Power Source omitted).** The plan/UI-SPEC acceptance said "render all 23 canonical slots." During browser-smoke, the executor (commit 2f2c9cf and the layout commits) deliberately omitted the Charm and Power Source positions because they are post-Velious slots Project 1999 will never implement, so they can never hold an item — rendering them as permanently-empty tiles would be misleading. The backend `slotconst.go` still defines all 23 (the data contract is intact); only the UI hides two unfillable positions. This is faithful to INV-01's "EQ paperdoll arrangement" for a P1999 client and was approved in the operator browser-smoke. Recorded as a deviation, not a gap — it does not impair the goal.

### Human Verification Required

The phase's one human-verify checkpoint (Task 31-04 Task 3: deploy to prod + browser-smoke the DOM-dependent behaviors that node vitest cannot see — paperdoll render, hover/pin examine, inline bag expand, remote icon load + colored-tile onerror fallback, master-detail selection, all 5 themes) is **SATISFIED**: per the verification context, the human operator deployed to squirebot.quest (backend binary + migrations 00012/00013 + web bundle) and confirmed the browser-smoke PASSED across the 5 themes. The 8 follow-up `fix(31)`/`feat(31)` commits are the fix-forward iterations from that smoke. No outstanding human items remain.

### Gaps Summary

None. All four ROADMAP success criteria are achieved in the codebase, all 7 requirement IDs are satisfied, every key link is wired, data flows end-to-end (real DB queries, not stubs), automated gates are green (Go all-ok; web 347/347; build ok), code review found 0 critical / 0 HIGH, and the human-verify browser-smoke checkpoint is operator-approved. The two code-review WARNINGs (WR-01 empty-wiki-row edge case, WR-04 race/class separator cosmetics) and the smoke-added 00013 migration are non-blocking polish, not goal-blocking gaps.

---

_Verified: 2026-06-18T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
