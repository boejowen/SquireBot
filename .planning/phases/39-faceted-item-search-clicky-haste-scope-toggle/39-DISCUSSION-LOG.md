# Phase 39: Faceted item search (Clicky / Haste + scope toggle) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-25
**Phase:** 39-faceted-item-search-clicky-haste-scope-toggle
**Areas discussed:** Search surface / home, Facet controls + combination, Default scope + toggle behavior, Catalog-scope row content + actions

---

## Search surface / home

| Option | Description | Selected |
|--------|-------------|----------|
| Inventory tab only | Facets + scope toggle on the Inventory tab; toggle flips its data source. One surface. | |
| Inventory tab + wishlist add search | Also add the Clicky/Haste facets to the wishlist add-item catalog search. | ✓ |

**User's choice:** Inventory tab + wishlist add search.
**Notes:** A small deliberate scope expansion beyond my single-surface recommendation. The scope TOGGLE (holdings↔catalog) lives only on the Inventory tab; the wishlist add-search is catalog-only, so there the facets just narrow the catalog suggestions.

---

## Facet controls + combination

| Option | Description | Selected |
|--------|-------------|----------|
| Two toggles, AND-combined | Clicky + Haste independent toggles; both on = items that are both; none = all. | ✓ |
| Single-select (All / Clicky / Haste) | One control, three states; can't express 'both'. | |

**User's choice:** Two toggles, AND-combined (recommended).
**Notes:** Most expressive; maps directly onto "show me clickies" / "show me haste items," and an item can be both clicky and haste.

---

## Default scope + toggle behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Default Holdings; query + facets persist | Opens on 'who has one'; flipping to catalog keeps the query + facets. | ✓ |
| Default Holdings; reset on toggle | Opens on Holdings; clears query + facets each flip. | |
| Default Full catalog | Opens on 'what exists'. | |

**User's choice:** Default Holdings; query + facets persist (recommended).
**Notes:** The toggle reads as a lens over the same search rather than a reset.

---

## Catalog-scope row content + actions

| Option | Description | Selected |
|--------|-------------|----------|
| Rich row, holders when held, no new pin action | Reuse the examine panel; held items still show holders (catalog = superset); unheld read 'not held'. | ✓ |
| Same + a 'pin to wishlist' action per row | Adds the discovery→wishlist shortcut (extra scope). | |
| Minimal catalog row | name + price + wiki only. | |

**User's choice:** Rich row, holders when held, no new pin action (recommended).
**Notes:** "Pin to wishlist" deferred (that flow already exists on the Wishlist tab). Catalog scope is a superset of holdings — a held item keeps its holder detail in catalog scope.

---

## Claude's Discretion

- The server-side (catalog) vs client-side (holdings) facet split; the catalog↔`item_master` facet join by normalized name (the P38 Option-A namespace bridge — flagged for the researcher); the toggle/persistence state model; the catalog-row render reusing `ExaminePanel`.
- UI phase → a `/gsd-ui-phase 39` UI-spec gate is expected before/within planning.

## Deferred Ideas

- "Pin to wishlist" from catalog search (discovery→wishlist shortcut) — fast-follow, not this phase.
- Facets beyond Clicky/Haste (slot / stat thresholds / weapon type) — REQUIREMENTS "Future Requirements".
