# Phase 33: Banks Tab + Valuation - Context

**Gathered:** 2026-06-18
**Status:** Ready for planning

<domain>
## Phase Boundary

The **Banks tab** — the guild-wide answer to *"what's in the guild banks, and what is it worth?"* — a web
surface (`web/`, SvelteKit) in the Phase 30 app shell over Phase 29 data, reusing the Phase 31 inventory
window and the Phase 32 master-detail / item-rollup patterns. Three requirements:

1. **Banks-only list + window (BANK-01)** — list only guild-bank characters; selecting one opens its
   in-game inventory window.
2. **Valuation (BANK-02)** — the total PigParse item value held by bank characters + the total platinum
   held across the guild banks (from the manual bank-coin entries).
3. **Per-item bank search (BANK-03)** — a name search across the items held by the guild banks.

**Out of phase scope (owned elsewhere — do not build here):**
- The in-game inventory window itself (Phase 31 SHIPPED — REUSE `InventoryWindow`, do not rebuild).
- The bank valuation + total-platinum math (Phase 29 SHIPPED — `compute.BankValuationFor` + `TotalPlatinum`;
  this phase SURFACES them, it does not recompute).
- The 5-tab shell, routing, the Banks route placeholder + its scoped-search placement (Phase 30 SHIPPED —
  this phase fills the Banks-tab placeholder body).
- The item-centric Inventory tab (Phase 32 SHIPPED — REUSE its item-rollup + master-detail + holders
  deep-link pattern, scoped to bank holders).
- The per-character/per-slot Wishlist (Phase 34, WISH-01..07).
- Exact pixel layout, mobile reflow of the list/summary/detail → `/gsd-ui-phase 33` (UI hint = yes).

</domain>

<decisions>
## Implementation Decisions

All four discussed gray areas were locked to the recommended option in one pass (the user's established
delegate-and-lock pattern — see [[feedback_delegate_gray_areas]]).

### Bank roster scope & ordering (BANK-01)
- **D-01:** **List bank toons AND guild bots (`IsBankToon || IsGuildBot`), A-Z.** Both designations (from
  v2.3) hold shared guild goods, so both belong in "the guild banks" — and their holdings BOTH count toward
  the BANK-02 valuation totals. Ordering is plain alphabetical: banks aren't anyone's assigned / "viewer"
  characters, so the Characters-tab viewer-first ordering doesn't apply — this is the roadmap's "same
  ordering style as Characters" reduced to its banks-only case. Rejected: bank-toons-only (would drop bot
  holdings from both the list and the totals).

### Valuation display (BANK-02)
- **D-02:** **One guild-wide summary header; bank rows stay clean.** A single summary at the top of the tab
  shows the total PigParse item value across all bank + bot holdings AND the total platinum across the guild
  banks; the bank list rows stay clean (name + item count). Rejected: per-row value/platinum subtotals
  (busier rows — the per-bank number lives in the detail instead, D-04).

### Item search behavior (BANK-03)
- **D-03:** **Item-centric search, Phase-32 style.** The per-item search is the P32 item-rollup pattern scoped
  to bank holders: type an item name → see which bank(s) hold it (qty / slot) → clicking a holder opens that
  bank's inventory window. Consistent with the Inventory tab's search + cross-tab jump. Rejected: a bank-list
  filter (can't see item qty/details without opening each window).

### Selected-bank detail (BANK-01 / BANK-02)
- **D-04:** **Per-bank value/platinum header above the reused window.** When a bank is selected, the detail
  shows a small header with THAT bank's own item value + platinum, then the reused P31 `InventoryWindow`.
  (Guild-wide totals stay in the top summary, D-02; the per-bank slice lives here.)

### Carried forward (locked upstream — NOT re-discussed, apply as-is)
- **Master-detail + click-to-pin**, reusing P31's generic prop-driven `InventoryWindow` per bank (built for
  exactly this reuse); single pinned detail, replace-on-click (P31 D-06/07, P32 D-03a).
- **Valuation/platinum math already exists** — `compute.BankValuationFor` + `TotalPlatinum` (P29): surface,
  don't recompute. "Value" = sum of name-keyed `pickPrice` over bank items, unpriced → 0 (the P29
  `BankValuation` posture); platinum = the manual bank-coin entries summed across bank toons.
- **Real wiki icons + colored-tile fallback** and the **examine** content/order come free with the reused
  P31 window.
- **EQ theme tokens only**, 44px touch targets, focus-visible 2px accent outline.
- **Node web tests are DOM-blind** ([[web-tests-node-only-blind-to-dom]]) → the tab MUST be browser-smoked on
  a DEPLOYED build ([[web-local-dev-cant-auth-against-prod]]).

### Claude's Discretion (researcher/planner owns these)
- **Backend shape** — whether to add a bank-scoped rollup endpoint or compose existing reads:
  `compute.BankValuationFor` / `TotalPlatinum` (P29) for the totals, `RosterFor` band-2 filtered to
  `IsBankToon || IsGuildBot` for the list, the P31 inventory route per bank, and the P32 `/api/v1/items`
  rollup filtered to bank holders for the item search. Compute-on-read; extend-only; `?`-bound; session-gated
  (`RequireSession`, NOT officer); never string-concat names into SQL.
- **Item-search wiring** — reuse the P32 `/api/v1/items` rollup (client-filtered to `is_bank` holders, which
  the holder rows already carry) vs a new bank-scoped endpoint.
- **Per-bank value/platinum sourcing (D-04)** — the per-bank slice of `BankValuation` / `TotalPlatinum`.
- **Likely no new migration** — bank/bot designation + bank-coin shipped in v2.3; icon/statsblock in P31's
  00012/00013 (schema v13). Confirm in research.
- Exact list / summary / detail layout, mobile reflow, row density → deferred to UI-SPEC.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — "Phase 33: Banks Tab + Valuation" Phase Details (goal, 3 success criteria,
  Depends on Phases 29 + 30 + 31).
- `.planning/REQUIREMENTS.md` — BANK-01, BANK-02, BANK-03.

### Prior phase context (continuity)
- `.planning/phases/31-characters-tab-in-game-inventory-window/31-CONTEXT.md` — the reusable generic
  `InventoryWindow`, the examine, master-detail / click-to-pin, the selection seam.
- `.planning/phases/32-inventory-tab-item-centric/32-CONTEXT.md` — the item-rollup + master-detail list +
  holders deep-link pattern the bank item-search reuses (`is_bank` is already on each holder).
- `.planning/phases/29-data-foundation-inventory-parse-price-value-aggregation/29-CONTEXT.md` —
  `BankValuationFor` / `TotalPlatinum`, the bank/bot designation, name-keyed price, the item-id-namespace
  caveat (group/join by normalized name, never raw item_id).
- `.planning/phases/30-app-shell-5-tab-navigation/30-CONTEXT.md` — the shell/routing + per-tab scoped-search
  the Banks route plugs into (the Banks route is a Phase-30 placeholder this phase fills).

### Locked design direction
- `.planning/sketches/MANIFEST.md` + `.planning/sketches/003-inventory-and-banks-lists/` — sketch 003 covers
  BOTH the inventory and banks lists (master-detail; the banks list = banks-only + a valuation summary).

### Project guidelines
- `CLAUDE.md` — compute-on-read, extend-only schema, watcher untouched, EQ-theme single `[data-theme]`
  writer, consolidated-views RELAXED (guild-wide grid + per-bank master-detail both allowed).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (backend)
- `internal/backendsrv/compute/inventory.go` — `BankValuationFor(ctx, store) → BankValuation` +
  `buildBankValuation(rows, toons)` + `TotalPlatinum(banks []BankToon) → int64` (Phase 29): the BANK-02
  totals, ready to surface.
- `internal/backendsrv/store/readviews.go` — `RosterFor` band-2 = banks/bots (`IsBankToon || IsGuildBot`);
  the bank-list filter source.
- `internal/backendsrv/compute/itemrollup.go` + `internal/backendsrv/readapi/items.go` (Phase 32) — the item
  rollup (`is_bank` already on each holder) the bank item-search reuses / filters.
- `internal/backendsrv/readapi/{inventory.go,characters.go}` (Phase 31) — the per-bank inventory-window route
  + the session-gated read-API pattern.

### Reusable Assets (web)
- `web/src/routes/banks/+page.svelte` — the Phase-30 **placeholder** to replace with the real Banks tab.
- `web/src/lib/components/InventoryWindow.svelte` (Phase 31) — reused per bank (generic / prop-driven).
- `web/src/routes/inventory/+page.svelte` + `web/src/lib/items.ts` (Phase 32) — the master-detail list +
  item-rollup helpers the bank list + item-search mirror.
- `web/src/lib/api.ts` — `fetchItems` / `fetchInventory` wrappers; add a `fetchBanks()` twin if a bank-scoped
  endpoint lands.

### Established Patterns
- Compute-on-read (`compute/` authors zero SQL); extend-only `goose`; `?`-bound, never name-concat.
- Pure DOM-free helpers (`.ts`) node-tested; the rendered DOM stays a browser-smoke gap.
- Session-gated read routes under `webauth.RequireSession` (login-gated, NOT officer-only); V7 slog carries
  counts/status, never raw item/char values.

### Integration Points
- The Banks route consumes the Phase 30 shell + scoped-search; the detail reuses the Phase 31 window; the
  item-search reuses the Phase 32 rollup; the totals come from Phase 29 compute.

</code_context>

<specifics>
## Specific Ideas

- Top summary form: "**Guild banks: ~{total item value} pp · {total platinum} plat**" spanning the tab.
- Per-bank detail header (D-04): "**{bank name} — {bank's item value} pp · {bank's platinum} plat**" above
  the reused inventory window.
- Item-search results deep-link into the holding bank's window (the P32 cross-tab jump, here within the
  Banks tab).
- Reuse the live EQ themes unchanged (structure/data, not a re-skin).

</specifics>

<deferred>
## Deferred Ideas

- **Sort/filter controls on the bank list** (by value, by item count) beyond the locked A-Z + item search —
  future polish only, revisit if the default proves insufficient.
- **Per-item value column inside the bank window** — the window is reused as-is from P31; per-item price/value
  already lives in its examine.
- **Per-character/per-slot Wishlist** — Phase 34 (WISH-01..07).

None of the above is dropped — each is owned by its mapped downstream phase or is future polish.

</deferred>

---

*Phase: 33-banks-tab-valuation*
*Context gathered: 2026-06-18*
