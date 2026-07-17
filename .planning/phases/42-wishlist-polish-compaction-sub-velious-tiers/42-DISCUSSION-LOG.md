# Phase 42: Wishlist polish — compaction + sub-Velious tiers - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-16
**Phase:** 42-wishlist-polish-compaction-sub-velious-tiers
**Areas discussed:** Tier labels & ordering, Sub-Velious scope + sources, Compaction target (WISHUI-01)

---

## Tier labels & ordering (D-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Tier badge, ladder low→high | Add a tier badge ([Classic]/[Kunark]/[Pre-Raid]/[Raid]) + order the per-slot list Classic → Kunark → Velious Pre-Raid → Velious Raiding. Legible progression. | ✓ |
| Tier badge, best/endgame first | Same badge, reversed (Velious Raiding at top). | |
| No badge, just append | Simplest, no UI change — but new tiers sort first, unlabeled (confusing). | |

**User's choice:** Tier badge, ladder low→high (recommended default).
**Notes:** Today `WishlistSuggestion` has no tier label + sorts alphabetically → adding Classic/Kunark unlabeled would be noise. Add a `tier` field + badge + an explicit tier RANK (not alphabetic); no migration (Go sort or SQL CASE).

---

## Sub-Velious scope + sources (D-02 / D-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Kunark + classic, raid-tagged | Both tiers; researcher confirms the same-format wiki pages; sub-Velious raid/no-drop items get the Raid/not-for-sale tag. | ✓ |
| Kunark only | Just Kunark group/raid gear; defer classic. | |
| Whatever parses cleanly | Delegate the exact page set to the researcher. | |

**User's choice:** Kunark + classic, raid-tagged (recommended default).
**Notes:** No migration, no new parser IF the pages match the section/bold/transclusion format. The exact page titles are a RESEARCH item (the load-bearing WISHUI-02 uncertainty) — a page that doesn't parse is skipped gracefully.

---

## Compaction target — WISHUI-01 (D-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Density polish | Tighten accordion gap 24→16, slot padding 16→12, row heights 44→40; keep the two-pane structure. | ✓ |
| Fuller restructure | Rework the layout beyond spacing. | |
| Minimal | Barely touch it. | |

**User's choice:** Density polish (recommended default).
**Notes:** Mirrors the CHARUI-01 "tighten toward the sibling-tab density" approach; web-only, low risk.

---

## Claude's Discretion

The user delegated the remaining sub-decisions (all three answered with the recommended default — yolo mode). Locked in CONTEXT.md: the Iksar ladder position, the tier-rank mechanism (Go sort vs SQL CASE), the exact compaction CSS values, the tier-badge visual, and whether `is_raid` stays tier-name-based or moves to P37's `is_no_drop`.

## Research flagged (not skipped)

The exact P1999 Kunark/classic gear-tier wiki page titles + whether they follow the parser's MediaWiki format — the load-bearing WISHUI-02 assumption. The phase-researcher confirms via the wiki API; a non-conforming page is flagged (new parser branch or drop the tier).

## Deferred Ideas

- Per-tier collapse/filter (hide Classic once in Velious).
- Non-gear suggestion sources (spells, tradeskill).
- Epic/quest-specific tiers.
- A wishlist architecture rework (WISHUI-01 is density polish only).
