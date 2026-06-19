# Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades - Discussion Log

> **Audit trail only.** Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-18
**Phase:** 34-wishlist-rework-per-character-per-slot-upgrades
**Areas discussed:** Existing wantlist data fate, Auto-removal mechanism, Old /wantlist route, Slot scope
**Mode:** default (interactive); all 4 areas decided in one consolidated pass; all locked to the recommended option (the user's delegate-and-lock pattern).

---

## Existing wantlist data fate (WISH-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Clean break (retire it) | Retire `wantlist_item`; the new per-slot wishlist starts empty; guildies re-add | ✓ |
| Best-effort migrate | Map character-tagged wants into an "unslotted/general" bucket per char | |
| You decide | Lock to recommended | |

**User's choice:** Clean break (D-01)
**Notes:** Item-centric wants don't map onto per-slot upgrade targets; migration would need an awkward no-slot bucket. The `wantmatch` matcher must repoint from `wantlist_item` to the new wishlist table.

---

## Auto-removal mechanism (WISH-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Derived (compute-on-read) | The slot's wishlist hides items the char currently has; nothing destructively deleted; self-heals; explicit remove = real delete | ✓ |
| Stored prune on upload | Permanently DELETE the row when the item is seen on the char (ingest-time hook; destructive) | |
| You decide | Lock to recommended | |

**User's choice:** Derived compute-on-read (D-02)

---

## Old /wantlist route (WISH-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Remove + redirect | `/wantlist` 308 → `/wishlist`; delete the old page | ✓ |
| Keep both | Two overlapping surfaces | |
| You decide | Lock to recommended | |

**User's choice:** Remove + redirect (D-03)
**Notes:** Consistent with the P30 5-tab restructure (notifications already moved to the Wishlist tab).

---

## Slot scope (WISH-02 / WISH-04)

| Option | Description | Selected |
|--------|-------------|----------|
| All worn slots (incl. empty) | All ~21 P31 paperdoll equipment slots get a wishlist + suggestions, even empty ones | ✓ |
| Only filled slots | Only slots with a currently-equipped item | |
| You decide | Lock to recommended | |

**User's choice:** All worn slots incl. empty (D-04)
**Notes:** Same worn-slot taxonomy as the P31 paperdoll (Charm/Power Source stay omitted — post-Velious).

---

## Claude's Discretion
- The new wishlist schema (migration → v14; per char+slot+item+ping flag) + whether it drops the retired `wantlist_item`.
- The compute-on-read auto-removal join (normalized name vs item_id).
- The class+slot → `_wiki_gear_tier` suggestion mapping (reuse `gearcheck.go` vs a new per-slot read) + the "Raid"/not-for-sale tagging.
- One vs several read-API routes; owner-scoped IDOR-safe writes (the v2.2 `account.go`/`wantlist` pattern).
- Exact per-slot list/accordion layout → UI-SPEC.

## Deferred Ideas
- Migrating old wantlist entries (rejected — clean break).
- Group/Raid binary tiering (WISH-04 = flat list + "Raid" tag, not a split).
- Cross-character/officer shared wishlist views (owner-scoped only).
- After P34: v2.4 milestone audit/close.
