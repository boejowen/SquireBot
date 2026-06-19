# Phase 30: App Shell + 5-Tab Navigation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-17
**Phase:** 30-app-shell-5-tab-navigation
**Areas discussed:** Routing & URL model, Phase 30 scope (stubs vs. rehome), Settings tab + gear-menu fate, Per-tab search shell

**Resolution mode:** The user selected all four gray areas and gave a single governing
criterion — *"For each of these, I want you to choose whichever option you think will produce
the simplest end-user experience."* Per the delegate-on-criterion pattern, all four areas were
locked in one pass against that rule (no further per-question prompting).

---

## Routing & URL model

| Option | Description | Selected |
|--------|-------------|----------|
| Real per-tab SvelteKit routes (`/characters` … `/settings`) + redirects from old URLs | Deep-linkable, refresh-stable, back-button correct; old bookmarks redirect (no 404) | ✓ |
| Client-side tab switching on one route | Simpler routing layer, but refresh/back/bookmark all lose tab state | |

**User's choice:** Simplest end-user experience → real per-tab routes (D-01) + redirect every
existing path to its new home (D-02).
**Notes:** "Simplest" read as: the URL always reflects where you are; no bookmark should break.

---

## Phase 30 scope: stubs vs. rehome now

| Option | Description | Selected |
|--------|-------------|----------|
| Pure chrome with placeholder bodies for all 5 tabs | Cleanest separation, but temporarily removes working features (wantlist, notifications, settings, inventory) | |
| Rehome what already exists; stub only the new tabs; never strand the user | Wishlist + Settings functional immediately; Characters/Inventory/Banks placeholdered; classic inventory/bank views kept reachable | ✓ |

**User's choice:** Simplest end-user experience → rehome the ready surfaces, stub only the new
ones, and preserve inventory/bank access through the transition (D-03, D-03b, D-04).
**Notes:** Inventory/bank lookup is the product's core value — it must not regress while 31–33 build.

---

## Settings tab + gear-menu fate

| Option | Description | Selected |
|--------|-------------|----------|
| Settings = one page with inline sections + settings search; gear dissolves; identity+sign-out stay top-right | Everything configurable in one place; fewer hops; conventional sign-out location | ✓ |
| Settings = a menu of links to the existing standalone sub-pages | Less composition work, but adds navigation round-trips | |

**User's choice:** Simplest end-user experience → single Settings page composing existing panels
as sections (D-05); header gear dissolves, minimal identity + Sign out stay top-right, Theme moves
into Settings (D-06).
**Notes:** Also resolved the NAV-03 vs. sketch-lock conflict: ALL notification surfaces (badge +
inbox + prefs) live on the Wishlist tab, not Settings (D-07) — one mental model, one place.

---

## Per-tab search shell

| Option | Description | Selected |
|--------|-------------|----------|
| Each tab's search lands WITH its content; functional on live tabs, none on stubs | No dead/inert search bars; search always works when visible | ✓ |
| Build inert search-bar slots on all 5 tabs now | Establishes the slot everywhere, but stub tabs show a search box that does nothing | |

**User's choice:** Simplest end-user experience → no dead search bars; Wishlist + Settings get a
working scoped search in Phase 30; Characters/Inventory/Banks get theirs with their content in
31/32/33 (D-08).
**Notes:** NAV-02 is pattern-established in Phase 30 and completed per-tab as content lands.

---

## Claude's Discretion

- Exact redirect mechanism and where the preserved classic inventory view is retained.
- Shared-layout vs. shell-component structure for the 5 tabs.
- Mobile/responsive collapse of the tab strip; exact settings-search behavior.

## Deferred Ideas

- Characters tab + inventory window (P31); Inventory item-list (P32); Banks valuation (P33);
  per-slot Wishlist rework + WISH-07 search (P34).
- Retiring the preserved classic inventory/gear/spell view (when P31–33 replace it).
- Gear Check / Spell Check long-term home (decided in P34 / out of v2.4 spec; kept reachable for now).
