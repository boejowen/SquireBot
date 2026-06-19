# Phase 31 — Discussion Log

**Date:** 2026-06-18
**Mode:** discuss (interactive). Human-reference record only — downstream agents read `31-CONTEXT.md`.

## Area selection
Presented 4 phase-specific gray areas (multiSelect). All four selected:
Item icons (INV-04) · Bag drill-down (INV-03) · Bank fidelity (INV-01) · Examine + missing data (INV-02).

What was already locked upstream (not re-asked): Variant D paperdoll + click-to-pin + bank-below-general (sketch 002); examine content/order (MANIFEST); list ordering viewer-first + search priority (CHAR-01/02); the Phase 29 `StructuredInventory` data model (which keeps empty slots).

## Decisions (one batched pass — all locked to the recommended option)

### Item icons (INV-04) → "Extend weekly wiki job"
- Options: Extend weekly wiki job (rec) · Curated/static map · Fetch-on-demand+cache · You decide.
- Selected: **Extend weekly wiki job** — capture each item's icon id during the existing weekly P1999 wiki enrichment, cache server-side; colored-tile fallback (locked) covers gaps so coverage can ship incrementally. → D-01/D-02/D-03.

### Bag drill-down (INV-03) → "Inline expand"
- Options: Inline expand (rec) · Modal pop-out · Into pinned panel · You decide.
- Selected: **Inline expand** — bag contents expand inline, behaving like the grid; the Phase 29 `Children` nesting feeds it. → D-04.

### Bank fidelity (INV-01) → "Faithful bank grid"
- Options: Faithful bank grid (rec) · Simple bank list · You decide.
- Selected: **Faithful bank grid** — character's own bank as an EQ bank-window grid below the paperdoll, reusing the same grid + drill-down component (faithful AND DRY). → D-05.

### Examine + missing data (INV-02) → "Hover preview + pin"
- Options: Hover preview + pin (rec) · Click-to-pin only · You decide.
- Selected: **Hover preview + pin** — desktop hover preview + click/tap pins to a side panel; matches "hover or tap." → D-06.

## Confirmed discretion defaults (surfaced, accepted via "Write CONTEXT.md")
- Single pinned panel, replace-on-click (not multi-pin) → D-07.
- Graceful missing data: omit empty fields, never blank/broken → D-09.
- Empty/sparse states: empty paperdoll positions; "no inventory synced yet"; show available meta → D-11.
- Icon storage extend-only; exact wiki-field/mechanism a research item → D-03.
- Examine may reuse `ItemTooltip.svelte` for hover; pinned panel is the fuller examine.
- Pixel layout / mobile reflow / slot-grid styling → `/gsd-ui-phase 31`.

## Deferred ideas
Modal bag pop-out (rejected for inline); multi-pin compare board (rejected for single panel).
Out-of-phase by design: Inventory tab (P32), Banks tab + valuation (P33), Wishlist rework (P34).

## Scope creep
None — discussion stayed within the Phase 31 boundary.
