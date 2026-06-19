---
phase: 33-banks-tab-valuation
plan: 02
subsystem: web
tags: [svelte, sveltekit, banks-tab, master-detail, valuation, bank-item-search, reuse]

# Dependency graph
requires:
  - phase: 33-banks-tab-valuation
    provides: "Plan 33-01 GET /api/v1/banks (BanksView/BankRowSummary Go contract) + the is_bank holder flag the item-search filters"
  - phase: 31-characters-tab-in-game-inventory-window
    provides: "InventoryWindow.svelte (reused per bank) + the /characters winStatus/invFor window-column state machine copied verbatim"
  - phase: 32-inventory-tab-item-centric
    provides: "/inventory list+search+holders shape + ItemRollup/ItemHolder + fetchItems() reused for the BANK-03 search + items.ts precedent"
provides:
  - "web/src/lib/api.ts BanksView/BankRowSummary interfaces + fetchBanks() (snake_case mirror of the Go contract)"
  - "web/src/lib/banks.ts pure helpers — sortBanksAZ + bankItemSearch (is_bank filter + bank-slice qty recompute) + sortBankHolders + bankByName"
  - "web/src/routes/banks/+page.svelte — the rendered Banks master-detail tab (D-02 summary + D-01 list/D-03 search toggle + D-04 per-bank header + reused window)"
affects: [33-03 (live deploy + browser-smoke of this rendered tab across the 5 themes)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Left-column mode toggle keyed off query emptiness: empty -> A-Z list, non-empty -> scoped item-search (the P32 scoped-search reduced to a single owning column)"
    - "In-tab holder deep-link: the item-search holder row calls the SAME select(bankName) the list rows call (no route change), unlike P32's cross-tab /characters?c= jump"
    - "Bank-slice qty recompute in the pure helper (the P32 rollup totals are GUILD-WIDE — recompute summed_qty/holder_count from the is_bank slice, Pitfall 3)"

key-files:
  created:
    - web/src/lib/banks.ts
    - web/src/lib/__tests__/banks.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/routes/banks/+page.svelte

key-decisions:
  - "bank/bot tag (UI-SPEC §C planner-discretion): Option (a) — a single neutral 'bank' tag on every row. BankRowSummary carries no per-row is_bank_toon/is_guild_bot flags, and every row in this banks-only list IS a guild bank/bot, so the tag marks 'why the row is here' without a cosmetic backend field"
  - "?b=<bankName> as the selection URL param, used CONSISTENTLY in the onMount read AND the select write (no ?i=/?c= mix — the plan-review resolution #1)"
  - "Nil plat renders 'not recorded' (NEVER '0 plat'); a recorded 0 reads '0 plat'; value is a real sum -> '0 pp' clean when unpriced (Pitfall 2 / nullable discipline)"
  - "load() fetches /api/v1/banks AND /api/v1/items in parallel (Promise.all) — both session-gated; a 401/403 on either routes to the AuthGate guard"

patterns-established:
  - "Surfacing + composition page: two verbatim analogs (list/search/holders from /inventory, the window column from /characters) + ONE net-new sub-block (the D-02 summary band) + ONE new pure algorithm (banks.ts is_bank filter)"

requirements-completed: [BANK-01, BANK-02, BANK-03]

# Metrics
duration: 6min
completed: 2026-06-19
---

# Phase 33 Plan 02: Banks Tab (web) Summary

**The `/banks` placeholder is replaced with the real master-detail Banks tab — a D-02 guild valuation summary header (total item value pp + total platinum), an A-Z bank/bot list whose left column toggles to a BANK-03 item-search scoped to bank holders (with the bank-slice qty RECOMPUTED off the guild-wide P32 rollup), and a D-04 per-bank value/plat header above the reused P31 InventoryWindow — pinned in-tab by clicking either a list row or a search holder; web-only, typecheck + build + 370 node tests green.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-06-19T01:46:33Z
- **Completed:** 2026-06-19T01:52:29Z
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- **api.ts contract (Task 1):** appended `BankRowSummary` + `BanksView` interfaces mirroring the Plan 33-01 Go contract field-for-field (snake_case; `plat: number | null`) and a `fetchBanks()` wrapper over `getJSON<BanksView>('/api/v1/banks')` — an OBJECT generic, not the array generic `fetchItems` uses. The P32 `ItemRollup`/`ItemHolder` are REUSED (not redeclared) for the BANK-03 search.
- **banks.ts pure helpers (Task 2, TDD):** `sortBanksAZ` (plain A-Z, NOT viewer-first — banks aren't assigned chars, D-01); `bankItemSearch` (the ONE new web algorithm — keep only `is_bank` holders, drop zero-bank items, **recompute** `summed_qty`/`holder_count` from the bank slice, name-filter, A-Z); `sortBankHolders` (A-Z, band collapsed); `bankByName` (the D-04 no-second-fetch lookup). DOM-free, immutable. RED (the `../banks` import failed) was verified before GREEN; 11/11 node cases pass, including the Pitfall-3 "Blue Diamond 40× guild / 3× across 2 banks → 3 / 2" recompute assertion.
- **banks/+page.svelte (Task 3):** fully replaced the P30 placeholder. The D-02 summary band ("GUILD BANKS" eyebrow + `{value} pp · {plat} plat`, numbers accent/tabular-nums, units dimmed) spans above a two-pane grid. The left column toggles off query emptiness: empty → the A-Z bank list (name + `{n} items` + a `bank` tag); non-empty → `bankItemSearch` results, each item name + holder rows. The right column copies the `/characters` `winStatus`/`invFor` stale-drop machine verbatim, rendering the D-04 per-bank header (nil plat → "not recorded") above `<InventoryWindow inventory={inv} />`. Clicking a list row OR an item-search holder calls the same `select(bankName)` → pins that bank's window in-tab (no route change). Selection is URL-reflected via `?b=`.

## Task Commits

Each task was committed atomically:

1. **Task 1: api.ts BanksView/BankRowSummary + fetchBanks()** — `577d503` (feat)
2. **Task 2: pure banks.ts helpers + node tests (TDD)** — `900c4d6` (feat; RED→GREEN verified, test+impl one commit per the same-module convention)
3. **Task 3: rebuild banks/+page.svelte master-detail tab** — `cc0a7b8` (feat)

## Files Created/Modified

- `web/src/lib/api.ts` — **+`BankRowSummary`/`BanksView`** interfaces (snake_case, append-only; `plat: number | null`) + **`fetchBanks()`** (`getJSON<BanksView>` — object, not array). The BANK-03 search reuses the existing P32 `ItemRollup`/`ItemHolder`/`fetchItems` (not redeclared).
- `web/src/lib/banks.ts` — **+`sortBanksAZ`/`bankItemSearch`/`sortBankHolders`/`bankByName`** (pure, DOM-free, immutable). The IRON-LAW-equivalent file header documents the `is_bank`-is-server-stamped / presentation-only discipline.
- `web/src/lib/__tests__/banks.test.ts` — 11 node cases (A-Z sort + immutability; drop non-bank-only items; keep only `is_bank` holders; the Pitfall-3 bank-slice recompute; distinct-char `holder_count`; name-filter trim/case-insensitive; empty-query A-Z; `bankByName` hit/miss).
- `web/src/routes/banks/+page.svelte` — the rebuilt Banks tab (P30 placeholder fully replaced). Two verbatim analogs (`/inventory` list/search/holders + `/characters` window column) + the net-new D-02 summary band + the D-04 per-bank header.

## Decisions Made

- **bank/bot tag = Option (a), a single neutral `bank` tag** (UI-SPEC §C planner-discretion). The 33-01 `BankRowSummary` carries `name`/`item_count`/`value`/`unpriced`/`plat` but NOT the per-row `is_bank_toon`/`is_guild_bot` booleans, and the UI-SPEC explicitly offered option (a) — render one `bank` tag for every row (all rows ARE guild banks in this banks-only list; the tag marks "why the row is here") — over adding a cosmetic backend field. No backend change made.
- **`?b=<bankName>` as the selection param, used consistently** in both the `onMount` read (`get('b')`) and the `select` write (`searchParams.set('b', …)`) — the plan-review resolution #1. No `?i=`/`?c=` mix (verified: grep counts 0 for both `get('[ic]')` and `set('[ic]'`).
- **Nil-plat copy:** `selectedBank.plat === null` → "not recorded" (never "0 plat"); a recorded 0 → "0 plat"; the value is a real sum → "0 pp" clean. This reconciles the UI-SPEC's "real sums → 0 pp/0 plat" note with the Go `*int64` nullable plat (the 33-PATTERNS §banks/+page.svelte reconciliation).
- **Parallel page load (`Promise.all([fetchBanks(), fetchItems()])`)** — both reads are session-gated and independent; a 401/403 on either hands off to the AuthGate guard. The empty state (`bv.banks.length === 0`) routes to `StateBlock kind="no-bank-toons"` ("No bank characters yet" — the P15 kind reused per UI-SPEC §H).

## Deviations from Plan

None — plan executed exactly as written. The two plan-review resolutions (`?b=` consistency, no new `{@html}` sink) and the two planner-discretion choices (bank/bot tag option, nil-plat copy) were applied as specified; no auto-fix (Rule 1/2/3) or architectural pause (Rule 4) was needed.

## Issues Encountered

None — every analog (the `/inventory` + `/characters` pages, `items.ts`/`roster.ts`, `StateBlock`'s `no-bank-toons` kind, `LastSyncedCell` at `cells/`) was exactly as the plan + pattern-map line-numbers described. The 33-01 Go contract field names matched the planned TS interfaces 1:1.

## Threat Surface

No new threat surface. This is a web-only, read-only browse/select tab over existing session-gated reads (`/api/v1/banks`, `/api/v1/items`, `/api/v1/inventory/{char}`). All guildie-controlled names (bank, item, char, slot) render via plain `{}` (Svelte auto-escapes); NO new `{@html}` sink was introduced (the only raw-HTML is `composeItemNote` inside the reused `ExaminePanel`, transitively via `InventoryWindow` — unchanged). The `?b=` deep-link value is `encodeURIComponent`'d in `select()`. No `## Threat Flags` needed.

## Known Stubs

None. The tab is fully wired: the summary header reads real `BanksView` numbers, the list reads `banksView.banks`, the search recomputes off the live `fetchItems()` rollup, and the window loads via the existing `fetchInventory(bankName)`. No hardcoded empty values flow to the UI — every data path is a real fetch. (The rendered DOM is a browser-smoke gap by design — node vitest is DOM-blind — and is verified in Plan 33-03's deploy-then-smoke, NOT a stub.)

## Next Phase Readiness

- The web half of the Banks tab is built and typechecks/builds clean. The rendered DOM (the summary header render, the list⇄search toggle, the selection→pin, the D-04 nil-plat copy, and especially the in-tab holder→bank-window deep-link) is NOT browser-verified here — node vitest is DOM-blind, and `npm run dev` can't auth against prod (cookie Domain=squirebot.quest + apex-only CORS).
- **Plan 33-03** deploys the 33-01 backend + this web tab to prod and browser-smokes the tab across all 5 EQ themes (the UI-SPEC §"Browser-smoke checklist" 10 points).

## Self-Check: PASSED

- All 4 files verified on disk (banks.ts, banks.test.ts, api.ts, banks/+page.svelte).
- All 3 commits verified in git log (577d503, 900c4d6, cc0a7b8).
- `npm --prefix web run check` 0 errors / 0 warnings; `npm --prefix web run build` ok (adapter-static); `npm --prefix web test` 370/370 green (incl. the 11 new banks.test.ts cases with the Pitfall-3 recompute assertion).
- Grep gates: `coming soon`=0, `{@html}`=0, `/characters?c=`=0; `?b=` read+write both literal `'b'`.

---
*Phase: 33-banks-tab-valuation*
*Completed: 2026-06-19*
