# Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades - Research

**Researched:** 2026-06-18
**Domain:** Go backend (SQLite/goose migration + compute-on-read + owner-scoped write API + EC-matcher repoint) + SvelteKit web tab
**Confidence:** HIGH (every claim verified against live code in this session)

## Summary

Phase 34 reworks the live v2.2/v2.3 item-centric `wantlist_item` table into a per-character,
per-equipment-slot upgrade wishlist. The work is overwhelmingly **reuse + repoint**, not new
invention: the EC-monitor/notify/`alert_log` spine stays untouched (only `wantmatch` repoints its
source table), the Velious `_wiki_gear_tier` suggestion data + `GearTierPrices` name-keyed price read
already ship (Phase 29 built `GearTierPrices` *for this phase*), the equipped-item-per-slot detection
ships in `compute.StructuredInventory` (Phase 31), the examine ships in `ExaminePanel`, and the
viewer-first list ships in `RosterFor`/`roster.ts`. The genuinely new surface is: (1) ONE new
`wishlist_item` table + migration `00014` (schema → v14), (2) a `wantmatch` repoint, (3) a small
compute-on-read suggestion+equipped-slot read, (4) an owner-scoped add/remove/ping-toggle write API
cloned line-for-line from `webadmin/wantlist.go`, and (5) the `/wishlist` Svelte tab.

The single load-bearing structural fact the planner MUST internalize: **`alert_log.wantlist_item_id`
is a FK `REFERENCES wantlist_item(id)`** (00007). D-01 retires `wantlist_item`. The new
`wishlist_item` table must therefore become the FK target — meaning `alert_log` must be rebuilt to
point at `wishlist_item(id)`, OR the FK relaxed. This is the only place the "clean break" has teeth.
Everything else (the watcher schema-gate worry, the WATCHER_MAX_SCHEMA_VERSION bump) is a **non-issue
in the off-Google backend** — see Pitfall 1.

**Primary recommendation:** Add migration `00014_wishlist.sql` that (a) creates `wishlist_item`
(per-character + canonical worn-slot + item target + `pinged`/ping-toggle + `muted` + `active`
soft-delete + owner `discord_user_id`), (b) rebuilds `alert_log` to FK `wishlist_item(id)` (zero rows
to copy after the clean break), (c) drops `wantlist_item` + its indexes. Repoint `wantmatch.ForItem`/
`ForName` to `wishlist_item`. Clone `webadmin/wantlist.go` → `webadmin/wishlist.go` (owner-scoped
add/remove/ping-toggle). Build the `/wishlist` tab as a viewer-first character list (banks/bots
excluded) → per-slot accordion (all 21 worn slots) reusing `StructuredInventory` for the equipped item
+ `GearTierPrices` for suggestions + `ExaminePanel` for the examine. There is NO `_meta.schema_version`
row and NO `WATCHER_MAX_SCHEMA_VERSION` constant in the Go backend — the CLAUDE.md note about
`internal/sheet/client.go` is stale Google-Sheets-era guidance (that package no longer exists).

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WISH-01 | Character list (viewer's first A-Z, then others), excludes guild banks/bots, per-character | `store.RosterFor` already returns viewer-first 3 bands; this phase drops the banks/bots band. `roster.ts`/`filterRoster` already supplies the search ranking. New read or client-filter omits `is_bank_toon || is_guild_bot`. |
| WISH-02 | Selecting a character shows its equipped slots (NOT the window) with the currently-equipped item per slot | `compute.StructuredInventory(char).Equipment` already yields the equipped item per canonical slot (`InventorySlot.CanonicalSlot`/`.Item`). Iterate the fixed 21-worn-slot list (InventoryWindow.svelte LEFT/RIGHT/WORN) so EMPTY slots appear too (D-04). |
| WISH-03 | Each equipped slot holds an open-ended set of upgrade targets; an item leaves on sighting OR explicit remove | New `wishlist_item` table (per char + slot + item). Compute-on-read auto-removal (D-02): hide a target whose normalized name is in `StructuredInventory` for that char. Explicit remove = soft-delete (`active=0`), the `RemoveOwnWantTx` twin. |
| WISH-04 | Per slot, complete Velious Pre-raid/Group + Raiding suggestions for class+slot, price+wiki+last-listed, "Raid" tag for no-drop/raid-only, not-for-sale | `store.GearTierPrices()` returns every `_wiki_gear_tier` row (tier/class/slot/name + name-keyed price/last-listed). Filter by char's `CharMeta.class` + map the slot via `enrich.WIKI_SLOT_TO_INV_SLOTS`. "Raid" tag = `tier == "Velious Raiding"` (NO separate no-drop column exists — see Pitfall 3). |
| WISH-05 | Discord ping toggle per item; badge when SquireBot pings (EC tunnel); reuses EC+notify spine | Repoint `wantmatch.ForItem`/`ForName` to `wishlist_item` (the ONLY change to the spine). Ping toggle = the existing `muted` mechanism (inverted: pinged-ON default, like wantlist). Badge derives from an `alert_log` row whose `wantlist_item_id` (→ rename to `wishlist_item_id`) matches the wishlist row id. |
| WISH-06 | Hover/tap examine (stats, price, wiki, last-synced) | Reuse `ExaminePanel.svelte` + `examineFields` verbatim; build an `InventorySlot`-shaped object for each wishlist target (the examine takes an `InventorySlot`). |
| WISH-07 | Wishlist search covers all items on any wishlist + the non-bank/bot characters | A new pure `.ts` filter over the loaded wishlist rows + the roster (the `filterRoster` analog). DOM-free, node-testable. |
</phase_requirements>

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01 — Clean break:** retire the old `wantlist_item`; the new per-character/per-slot wishlist is the
  source of truth and starts empty. The EC matcher (`wantmatch`) MUST repoint from `wantlist_item` to the
  new table. Whether the migration DROPS `wantlist_item` (with a `schema_version` bump) or leaves it
  dormant is planner's discretion under the extend-only rule.
- **D-02 — Derived (compute-on-read) auto-removal:** a slot's wishlist HIDES any target the character
  currently has equipped/in inventory — nothing destructively deleted on sighting; the view self-heals.
  The user's explicit "remove" IS a real delete (soft-delete).
- **D-03 — Remove + redirect:** `/wantlist` 308-redirects to `/wishlist`; the old wantlist page is
  deleted. (NOTE: P30 already did this — `web/src/routes/wantlist/+page.ts` is already a 308 stub and the
  old page is already gone. See "Web" §.)
- **D-04 — All worn slots, including empty:** all ~21 P31 paperdoll equipment slots get a wishlist +
  class/slot suggestions, even empty slots. Use the SAME worn-slot taxonomy as the P31 paperdoll
  (Charm/Power Source omitted — post-Velious).

### Claude's Discretion
- The new schema (a NEW wishlist table, schema → v14; whether the migration also drops `wantlist_item`).
- The auto-removal join key (normalized name vs item_id — almost certainly normalized name).
- The suggestion mapping (class+slot → `_wiki_gear_tier`; reuse `gearcheck.go` tier logic vs a new read).
- Whether the per-slot wishlist + suggestions are one read-API route or several; the web helper(s) + fetch
  wrappers; session-gated; owner-scoped per the v2.2 IDOR-safe `wantlist`/`account.go` pattern.
- Exact per-slot list/accordion layout, the equipped-item display, mobile reflow → UI-SPEC.

### Deferred Ideas (OUT OF SCOPE)
- Migrating the old wantlist entries (rejected by D-01 — clean break).
- Group/Raid binary tiering (WISH-04: a flat complete list with a "Raid" tag, NOT a Group/Raid split).
- Cross-character "shared wishlist" / officer views (owner-scoped only).
- After this phase: v2.4 feature-complete → `/gsd-audit-milestone` → `/gsd-complete-milestone`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Wishlist storage (per char+slot+item+ping) | Database / Store | — | New `wishlist_item` table; owner-keyed on `discord_user_id` (the PERSON, not a watcher entity). |
| Auto-removal (hide sighted targets) | API / compute-on-read | — | D-02 is a READ-time derivation over `StructuredInventory` — NOT an ingest hook, NOT stored prune. |
| Suggestions (class+slot → gear-tier) | API / compute-on-read | Store (`GearTierPrices`) | Pure transform over the already-shipped name-keyed gear-tier read; no new SQL needed beyond what exists. |
| Equipped-item-per-slot | API / compute (`StructuredInventory`) | Store (`InventoryForChar`) | Already computed in Phase 31; the wishlist consumes the `Equipment[]` slice. |
| EC-hit matching (ping) | API / `wantmatch` seam | Store (`wishlist_item`) | The shared matcher; repoint its source table. The EC job, notify, alert_log, DM are UNTOUCHED. |
| Add / remove / ping-toggle | API / webadmin write | Store (owner-scoped *Tx) | IDOR-safe owner-scoped writes; a guildie mutates only their OWN wishlist; audited. |
| Character list (viewer-first, no banks/bots) | Frontend + API (`RosterFor`) | — | Reuse the roster read; exclude the banks/bots band (WISH-01). |
| Per-slot list + examine render | Browser / Client | — | Svelte accordion; reuses `ExaminePanel`; EQ theme tokens; no new tier. |
| Wishlist search | Browser / Client (pure `.ts`) | — | DOM-free filter (WISH-07), node-testable. |

## Standard Stack

This is an in-repo phase — the "stack" is the existing toolchain. No new libraries.

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Go | 1.24 | backend | `[VERIFIED: go.mod / CLAUDE.md]` |
| modernc.org/sqlite | (in go.mod) | pure-Go SQLite driver; extended result codes for `ErrDuplicate` detection | `[VERIFIED: store/wantlist.go imports modernc.org/sqlite]` |
| goose (embedded migrations) | (in repo) | versioned, idempotent, forward-only SQL migrations in `internal/backendsrv/migrations/` | `[VERIFIED: 00001–00013 + migrate_test.go]` |
| SvelteKit (Svelte 5 runes) | (in web/) | the web tab; `$state`/`$derived`/`$props` runes | `[VERIFIED: WantlistPanel.svelte / characters/+page.svelte use runes]` |
| vitest (node project) | (in web/) | node-only unit tests over pure `.ts` helpers (DOM-blind) | `[VERIFIED: roster.ts / examine.ts node-tested; CLAUDE.md]` |

**No installation step.** No `npm install`, no new Go module. (Per the toolchain-install rule, if any tool
were missing the executor stops and waits for the user — but nothing new is needed here.)

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────────────────────────────────────┐
  WRITE PATH (owner-scoped, session-derived discord_user_id, NEVER from body)    │
  Browser /wishlist                                                              │
    │  add target  ──────► POST /api/v1/wishlist          ─┐                     │
    │  remove      ──────► POST /api/v1/wishlist/remove    ├─► webadmin/wishlist.go
    │  ping toggle ──────► POST /api/v1/wishlist/ping      ─┘   (clone of wantlist.go)
    │                                                          │  withTx → store *Tx
    │                                                          ▼
    │                                              ┌────────────────────────┐
    │                                              │ wishlist_item (NEW)    │
    │                                              │  id, discord_user_id,  │
    │                                              │  character_id, slot,   │
    │                                              │  item_id?, item_name,  │
    │                                              │  pinged, active        │
    │                                              └───────────┬────────────┘
  READ PATH (session-gated; compute-on-read)                  │
    │  load char's slots ─► GET /api/v1/wishlist/{char} ──► readapi/wishlist.go
    │                                                        │  compute.WishlistFor(char):
    │                                                        │   • StructuredInventory(char).Equipment  ──► equipped item per slot
    │                                                        │   • ListOwnWishlist(char) targets        ──► per-slot targets
    │                                                        │   • AUTO-REMOVAL: drop a target whose     ◄── normalized-name join
    │                                                        │     norm(name) is held by the char
    │                                                        │   • GearTierPrices() filtered class+slot ──► suggestions (Raid tag = tier)
    │                                                        │   • alert_log rows for this user         ──► badge (pinged + alert hit)
    │                                                        ▼
    │  viewer-first list ─► GET /api/v1/characters ───────► RosterFor (exclude banks/bots client-side or new read)
    │
  EC MONITOR (UNTOUCHED except the matcher source table)
    EC job (scheduler) ─► wantmatch.ForItem(item_id) ─► [SELECT … FROM wishlist_item …] ─► notify.Send ─► alert_log INSERT + DM
                                  ▲ REPOINT here (the ONLY spine change)
```

### Component Responsibilities

| File | Status | Responsibility |
|------|--------|----------------|
| `internal/backendsrv/migrations/00014_wishlist.sql` | CREATE | `wishlist_item` table + indexes; rebuild `alert_log` FK → `wishlist_item(id)`; DROP `wantlist_item` + indexes (D-01). |
| `internal/backendsrv/store/wishlist.go` | CREATE | Owner-scoped `*Tx` mutators (`AddWishlistTx`/`RemoveOwnWishlistTx`/`SetPingedTx`) + readers (`ListOwnWishlist`, optionally a guild read). Clone of `wantlist.go`. |
| `internal/backendsrv/wantmatch/match.go` | MODIFY | Change both SELECTs `FROM wantlist_item` → `FROM wishlist_item` + column names; keep the `active=1 AND <ping>` gate + non-nil-slice + scanHits shape. |
| `internal/backendsrv/store/alertlog.go` | MODIFY (column rename) | `wantlist_item_id` → `wishlist_item_id` across `InsertAlertTx`/`RecentAlertExists`/the dedup index. (Or keep the column name; see Pitfall 6.) |
| `internal/backendsrv/compute/wishlist.go` | CREATE | `WishlistFor(char)` pure-ish transform: equipped item per slot + targets (auto-removal) + suggestions + badge flags. The `view.go`/`inventory.go` public-fn→pure-helper split. |
| `internal/backendsrv/compute/types.go` | MODIFY (append-only) | New `WishlistView`/`WishlistSlot`/`WishlistTarget`/`WishlistSuggestion` structs (snake_case tags, append-only — NEVER rename an existing tag). |
| `internal/backendsrv/readapi/wishlist.go` | CREATE | `GET /api/v1/wishlist/{char}` (RequireSession), the `inventory.go` handler shape. |
| `internal/backendsrv/webadmin/wishlist.go` | CREATE | Add/remove/ping-toggle handlers; the `wantlist.go` clone (owner session-derived, audited, validated). |
| `cmd/squirebot-server/main.go` | MODIFY | Register the new wishlist routes (RequireSession); remove the 4 old `/api/v1/wantlist*` route registrations (D-01/D-03) OR keep them dormant (planner's call — see Pitfall 7). |
| `web/src/routes/wishlist/+page.svelte` | REWRITE | Replace the P30 WantlistPanel+Notifications placeholder with the per-character/per-slot wishlist (master-detail). KEEP the Notifications region (NAV-04). |
| `web/src/lib/wishlist/*.ts` | CREATE | Pure DOM-free helpers: viewer-first list (no banks/bots), per-slot grouping, auto-removal filter (if done client-side), wishlist search (WISH-07). Node-tested. |
| `web/src/lib/api.ts` | MODIFY (append) | `fetchWishlist`/`addWishlist`/`removeWishlist`/`setWishlistPing` wrappers + the `WishlistView`/`WishlistTarget` interfaces. |
| `web/src/lib/components/WantlistPanel.svelte` + `web/src/lib/wantlist/*` | DELETE (after rewire) | The old item-centric surface, superseded (D-01/D-03). NOTE: only `groupByChar.ts` (+ test) + `WantlistPanel.svelte` are deletable — `priority.ts` (`priorityRank`, `noteRuneCount`) and `holders.ts` (`type Holder`) have LIVE consumers (`columns.ts`/`guild-views`/`InGuildCell`/`WantAddForm`) and MUST be KEPT (see Pitfall 7). The `/wishlist` placeholder currently still mounts `WantlistPanel` — that mount goes away. |

### Pattern 1: NEW `wishlist_item` schema (migration 00014, schema → v14)

**What:** One table keyed on the PERSON (`discord_user_id`) + an OPTIONAL `character_id` + the canonical
worn-slot + the item target. Mirror `wantlist_item`'s security-bearing columns.

```sql
-- Source: derived from 00006_wantlist.sql + 00010 + 00011 + 00007 (the wantlist lineage)
-- +goose Up
-- Phase 34 (WISH-01..07). The per-character / per-slot upgrade wishlist replaces the
-- retired item-centric wantlist_item (D-01 clean break). Forward-only; 00001-00013 unedited.
--
-- Identity = web_user.discord_user_id (the PERSON, the DM target — the wantlist precedent).
-- character_id is NOT NULL here (every wishlist target is scoped to a character + slot,
-- unlike the wantlist's optional tag). slot is the canonical Title-case worn-slot token
-- ("Head"/"Finger1"/"Primary") — the compute.classifySlot vocabulary (slotconst.go).
CREATE TABLE wishlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  character_id    INTEGER NOT NULL REFERENCES character(id),  -- scoped to a char (no account-level)
  slot            TEXT NOT NULL,                              -- canonical worn-slot ("Head","Finger1",…)
  item_id         INTEGER,                                    -- NULL ⇒ typed/custom target OR a gear-tier item (no id)
  item_name       TEXT NOT NULL,                              -- snapshot: catalog name OR suggestion name OR typed label
  pinged          INTEGER NOT NULL DEFAULT 1,                 -- WISH-05 ping toggle (default-ON, the notify mechanism)
  active          INTEGER NOT NULL DEFAULT 1,                 -- soft-delete (explicit remove; alert_log FK never dangles)
  created_at      INTEGER NOT NULL
);
CREATE INDEX wishlist_user_idx    ON wishlist_item(discord_user_id);
CREATE INDEX wishlist_char_idx    ON wishlist_item(character_id);
CREATE INDEX wishlist_item_id_idx ON wishlist_item(item_id);
-- Dedup an exact active re-add of the same target in the same char+slot. NULL item_id is
-- DISTINCT in a UNIQUE index, so the custom branch keys on item_name (the 00006 split-index idiom).
CREATE UNIQUE INDEX wishlist_catalog_uidx ON wishlist_item(discord_user_id, character_id, slot, item_id)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wishlist_custom_uidx  ON wishlist_item(discord_user_id, character_id, slot, item_name) WHERE item_id IS NULL     AND active = 1;

-- D-01: rebuild alert_log to FK wishlist_item(id) (00007 made it FK wantlist_item(id)).
-- alert_log has ZERO rows in the WISH path before this phase ships (EC alerts were keyed on
-- wantlist_item; the clean break discards them too — confirm with the planner). SQLite can't
-- ALTER a FK, so DROP+CREATE. Copy nothing (clean break) OR copy the historical rows with
-- wishlist_item_id=NULL if the inbox history is worth keeping (planner's call — Pitfall 6).
DROP INDEX IF EXISTS alert_log_dedup_idx;
DROP TABLE alert_log;
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wishlist_item_id INTEGER REFERENCES wishlist_item(id) ON DELETE CASCADE,  -- renamed FK (NULLABLE: test-alert)
  discord_user_id  TEXT NOT NULL,
  source           TEXT NOT NULL,
  item_id          INTEGER,
  detail           TEXT,
  sent_at          INTEGER NOT NULL,
  send_status      TEXT NOT NULL,
  read_at          INTEGER
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wishlist_item_id, source, item_id, sent_at);

-- D-01: drop the retired wantlist_item + its indexes.
DROP INDEX IF EXISTS wantlist_user_idx;
DROP INDEX IF EXISTS wantlist_item_id_idx;
DROP INDEX IF EXISTS wantlist_catalog_uidx;
DROP INDEX IF EXISTS wantlist_custom_uidx;
DROP TABLE wantlist_item;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00013): explicit no-op.
SELECT 1;
```

**Drop-vs-dormant call (recommended: DROP).** D-01 is an explicit "retire from app use." Dropping
`wantlist_item` keeps the schema honest (no zombie table the matcher could accidentally read) and is
clean given the matcher repoint. The ONLY entanglement is `alert_log`'s FK — handled by the rebuild
above. A planner who prefers to leave `wantlist_item` dormant CAN, but then must STILL rebuild
`alert_log`'s FK (you can't have a wishlist `pinged` row produce an `alert_log` row that FKs
`wantlist_item`). Dormant-but-FK-intact is the worst of both worlds; pick DROP. **Whichever path:
`alert_log` MUST point at `wishlist_item(id)` or relax the FK.**

### Pattern 2: The `wantmatch` repoint (WISH-05 — load-bearing)

**What:** `wantmatch.ForItem`/`ForName` (one file, `internal/backendsrv/wantmatch/match.go`) are the ONLY
consumers of `wantlist_item` in the alerting path. They are called from exactly ONE live site:
`internal/backendsrv/ec/ec.go:211` (`wantmatch.ForItem(ctx, db, item.ItemID)`). The WTS/raid monitors
(`ForName`) are parked (`monitor_flag` ships `wts=0`, `raid=0`).

```sql
-- BEFORE (match.go ForItem):
SELECT w.id, w.discord_user_id, w.item_id, w.item_name, w.note, c.name AS character_name
  FROM wantlist_item w
  LEFT JOIN character c ON c.id = w.character_id
 WHERE w.item_id = ? AND w.active = 1 AND w.muted = 0

-- AFTER (wishlist_item):
SELECT w.id, w.discord_user_id, w.item_id, w.item_name, c.name AS character_name
  FROM wishlist_item w
  JOIN character c ON c.id = w.character_id            -- INNER: character_id is NOT NULL now
 WHERE w.item_id = ? AND w.active = 1 AND w.pinged = 1  -- pinged=1 replaces muted=0 (inverted sense)
```

**Repoint deltas the planner MUST handle:**
1. `muted = 0` → `pinged = 1` (the ping toggle is the inverse of the wantlist's mute: a wishlist item
   defaults to pinged-ON and the user toggles it OFF; same net gate, opposite column polarity).
2. `wantlist_item.note` (the wantlist's free-text "why") has NO analog in the new schema — drop it from
   the SELECT and from `Hit.Note`. CHECK the EC embed: `ec/embed.go:97 whyWanted(hit)` reads `hit.Note`.
   Either keep a nullable `note`/`reason` column on `wishlist_item` for embed parity, or have `whyWanted`
   degrade gracefully when `Note == nil` (it already handles nil — `Note` is `*string`). Simpler:
   drop note; let `whyWanted` return "" for a nil note.
3. `LEFT JOIN character` → `JOIN character` (the tag is now mandatory). `CharacterName` stays DISPLAY-ONLY
   — the DM target is ALWAYS `wishlist_item.discord_user_id` (the want owner), NEVER derived from the
   character. This is the load-bearing T-28-06 invariant; the regression test
   `TestForItem_DMTargetIsWantOwner_NotCharacterOwner` must be ported.
4. `ForName` matches `item_name` EXACTLY + `COLLATE NOCASE` (Pitfall 6 in match.go — NOT a LIKE
   substring). Keep that. Custom/typed targets (item_id NULL) are reachable ONLY via `ForName` (parked).
5. Port `wantmatch/match_test.go` (it currently seeds `wantlist_item` rows) to seed `wishlist_item`.

### Pattern 3: Suggestion mapping (WISH-04) — `GearTierPrices` + class + slot

**What:** Phase 29 built `store.GearTierPrices()` *explicitly for this phase* (its doc comment says
"Consumed by Phase 34 (WISH-04)"). It returns EVERY `wiki_gear_tier` row with the name-keyed PigParse
price + last-listed already resolved. The suggestion read is a pure FILTER over that slice.

```go
// compute/wishlist.go — pure suggestion mapping (no new SQL)
// For a character of class C and a canonical worn-slot S:
//   1. Find the wiki prose slot(s) whose WIKI_SLOT_TO_INV_SLOTS maps to S.
//   2. Take every GearTierPriceRow where row.Class == C AND row.Slot == wikiSlot.
//   3. Both "Velious Pre-Raid/Group" and "Velious Raiding" tiers (the complete flat list, D-04).
//   4. Tag "Raid" iff row.Tier == "Velious Raiding"; that tag ⇒ render "not for sale".
//   5. Attach price/last-listed from the row (HasPrice false ⇒ "no price").
```

**The canonical-slot ↔ wiki-slot bridge (CRITICAL — two different vocabularies).** The new
`wishlist_item.slot` uses the **inventory Title-case canonical** (`"Head"`, `"Finger1"`, `"Ear1"`,
`"Primary"`) — the `compute/slotconst.go` vocabulary. The gear-tier rows use the **wiki prose slot**
(`"Head"`, `"Fingers"`, `"Ears"`, `"Primary"`) mapped by `enrich.WIKI_SLOT_TO_INV_SLOTS` to UPPERCASE inv
tokens (`"FINGER1"`/`"FINGER2"`). The planner must build the inverse map: canonical worn-slot →
wiki-slot. Examples that bite:
- `"Finger1"` and `"Finger2"` → wiki `"Fingers"` (one wiki slot covers both numbered slots).
- `"Ear1"`/`"Ear2"` → wiki `"Ears"`; `"Wrist1"`/`"Wrist2"` → wiki `"Wrists"`.
- `"Head"`→`"Head"`, `"Primary"`→`"Primary"` (singletons map 1:1).
- `"Ammo"` and `"Charm"`/`"Power"` have NO wiki gear-tier slot — those slots get an empty suggestion
  list (Charm/Power are omitted anyway; Ammo is a worn slot with no Velious tier suggestions → fine).

Build this inverse by inverting `enrich.WIKI_SLOT_TO_INV_SLOTS` and case-folding: each UPPERCASE inv
token (`"FINGER1"`) maps back to its wiki slot (`"Fingers"`); the canonical `"Finger1"` upper-cases to
`"FINGER1"` to look it up. Do NOT re-key `gearcheck.go`'s logic — `gearcheck.go` walks tiers→class→slot
the OTHER direction (wiki-slot → which inv slots to check). For WISH-04 you go canonical-slot → wiki-slot
→ filter gear-tier rows. A small new pure helper in `compute/wishlist.go` is cleaner than bending
`gearcheck.go`.

### Pattern 4: Auto-removal join (D-02 / WISH-03) — normalized name

**What:** A target HIDES from a slot's wishlist when the character currently holds that item. The join key
is **normalized name** (`lower(trim(name))`) — NOT item_id. This is the established cross-namespace join
key throughout the codebase (the `pp_rep` CTE, `ItemRollup`, `holdersFor`): the PigParse/gear-tier
catalog ids ≠ EQ inventory ids, and gear-tier suggestion rows have NO item_id at all. A typed target also
has no reliable id. So normalize-and-compare names.

```go
// compute/wishlist.go — auto-removal (D-02)
inv := StructuredInventory(ctx, s, char)        // Phase 31
held := make(map[string]bool)                    // normalized names the char holds ANYWHERE
for _, slot := range append(append(inv.Equipment, inv.General...), inv.Bank...) {
    if slot.Item != "" { held[norm(slot.Item)] = true }
    for _, child := range slot.Children { if child.Item != "" { held[norm(child.Item)] = true } }
}
// A wishlist target is HIDDEN (not deleted) when held[norm(target.ItemName)] is true.
// norm = strings.ToLower(strings.TrimSpace(name)) — the pp_rep convention.
```

Scope note (planner decision): D-03 says "leaves the slot's wishlist when SquireBot sees it ON THAT
CHARACTER." Use that character's OWN `StructuredInventory` (held ANYWHERE on the char — equipment +
bags + bank), NOT the guild-wide inventory. The compute-on-read nature means it self-heals: if the item
later leaves the character, the target reappears (nothing was deleted). Equip-in-slot vs held-in-bag:
the requirement says "sees it on that character" — hold-anywhere is the literal reading and the safe
default; if the planner wants stricter "equipped in THAT slot only," that's a UI-SPEC refinement.

### Pattern 5: Owner-scoped write API (the `wantlist.go` clone)

**What:** Clone `webadmin/wantlist.go` → `webadmin/wishlist.go`. Every security property carries over
verbatim:
- Owner is `caller(ctx)` = `webauth.UserFromContext` (session-derived) — the body carries NO owner.
- `RemoveOwnWishlistTx` / `SetPingedTx` match `WHERE id = ? AND discord_user_id = ? AND active = 1` → a
  cross-owner mutation is `RowsAffected=0 → (false,nil)`: a silent IDOR no-op that never leaks the row's
  existence (the `RemoveOwnWantTx`/`SetMutedTx` twins).
- `character_id` in the add body is UNTRUSTED — authorize it in the SAME tx via
  `store.IsCharAssignedToTx(ctx, tx, characterID, callerID)` BEFORE the insert (the
  `AddWantHandler` T-28-05 guard). A guildie can only wishlist for a character ASSIGNED to them.
  `store.ErrCharNotAssigned` → 403.
- Add + audit in ONE `withTx` (BEGIN IMMEDIATE). Audit detail carries IDs ONLY — never the item name
  free-text beyond what's already there (V7). Use audit ops `wishlist_add` / `wishlist_remove` /
  `wishlist_ping`.
- `ErrDuplicateWishlist` (the unique-index 2067 extended-result-code detection — NOT string-matching the
  driver message) → 409 `{"error":"duplicate"}`. Port the `sqliteConstraintUnique = 2067` idiom.
- Routes register under `webauth.RequireSession` (NEVER `RequireOfficer` — owner-scoped, every member
  manages their OWN wishlist), mirroring lines 342-344 of `main.go`.

### Pattern 6: Badge derivation (WISH-05)

**What:** The badge appears beside a wishlist item when SquireBot has pinged the user about it. A ping
produces an `alert_log` row whose `wishlist_item_id` (renamed FK) = the wishlist row's id. The badge for
a wishlist target is therefore: "does an `alert_log` row exist with `wishlist_item_id = <this row's id>`
and (planner's choice) `read_at IS NULL` for unread-only, or any row for ever-pinged?"

The existing `alert_log` already supports this — `store.alertlog.go` has `ListInbox` (owner-scoped) and
the nav badge uses `UnreadCount`. For the per-item badge, add a small read: e.g.
`AlertedWishlistIDs(ctx, db, discordID) → map[int64]bool` (the set of wishlist ids this user has any
alert_log row for, owner-scoped). The compute layer attaches `pinged_hit: true` to each target whose id
is in that set. The NAV-04 unread badge on the Wishlist TAB stays exactly as P30 wired it (no change).

### Anti-Patterns to Avoid
- **Re-keying the gear-tier suggestion read off item_id.** Gear-tier rows have NO item_id (always NULL).
  Join price + match held items by NORMALIZED NAME only (Pitfall 3 in `readviews.go`).
- **Deriving the DM recipient from `character_id`/the character owner.** The DM target is ALWAYS the
  wishlist row's `discord_user_id` (T-28-06). The character tag is DISPLAY-ONLY.
- **A stored prune on upload (ingest hook) for auto-removal.** D-02 is explicitly compute-on-read; do NOT
  add an ingest-time delete. The watcher is UNTOUCHED.
- **Optimistic client mutation of the wishlist grid.** The `WantlistPanel` precedent: add/remove/toggle
  ALWAYS re-fetch from the server (authoritative). Never optimistic-mutate (T-19-16).
- **A second `{@html}` sink.** The ONLY sanctioned escaped-HTML sink is `composeItemNote` inside
  `ExaminePanel`. Every other name/label renders via plain `{}` (Svelte auto-escapes). Reuse
  `ExaminePanel` as-is.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Equipped item per slot | A new inventory parse | `compute.StructuredInventory(char).Equipment` | Phase 31 already classifies + names every worn slot (incl. paired Ear/Finger/Wrist numbering + Held-cursor exclusion). |
| Suggestion price + last-listed | A new price join | `store.GearTierPrices()` | Built in Phase 29 for exactly WISH-04; name-keyed pp_rep CTE already collapses fan-out. |
| Viewer-first character list | A new sort | `store.RosterFor` + `web/src/lib/roster.ts` | Already viewer-first 3-band; just exclude the banks/bots band. |
| The examine | A new tooltip | `ExaminePanel.svelte` + `examineFields` | The single escaped sink + D-08 order + D-09 omission, all node-tested. |
| Owner-scoped IDOR-safe CRUD | New auth logic | Clone `webadmin/wantlist.go` + `store/wantlist.go` | Every security property (session-owner, silent-no-op IDOR, 2067 dup detection, audited tx) is already proven. |
| EC matching | A new monitor | Repoint `wantmatch` | The EC job + notify + alert_log + DM spine is untouched; one SELECT changes. |
| Wishlist search / filter | Inline DOM filtering | A pure `.ts` (the `filterRoster` analog) | DOM-free → node-testable (the DOM-blind-tests reality). |

**Key insight:** This phase's correctness comes almost entirely from REUSING proven seams. The genuinely
novel logic is small: the schema, the slot↔wiki-slot inverse map, the auto-removal name join, and the
badge read. Treat any larger rewrite as a smell.

## Runtime State Inventory

This is a rename/retire/migration phase (D-01 retires `wantlist_item`). The 5 categories:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** | `wantlist_item` rows on the LIVE prod DB (Hetzner VPS SQLite). v2.2 P19 shipped 2026-06-05; ~2 weeks of real guildie entries. D-01 is an explicit clean break — these are DISCARDED. `alert_log` rows FK these (EC alerts since the EC monitor went live). | Migration 00014 DROPs `wantlist_item` + rebuilds `alert_log`. Confirm with the user that discarding ~2 weeks of wantlist entries + the EC alert inbox history is acceptable (D-01 says yes). The migration runs on the live DB via the deploy. |
| **Live service config** | None. The EC monitor reads `monitor_flag`/`guild_channel` (unchanged); the watcher posts to `/api/v1/ingest` (untouched). No n8n/Datadog/Tailscale config embeds "wantlist". | None — verified: the only `wantlist`-string runtime coupling is the matcher SQL (repointed) + the routes (re-registered). |
| **OS-registered state** | None. No Task Scheduler/pm2/systemd unit references "wantlist". The backend is one `squirebot-server` binary; the deploy is a binary swap + goose migrate. | None — verified: the migration applies on server start (goose runs `RunMigrations` at boot). |
| **Secrets / env vars** | None. No SOPS key, .env var, or CI secret named for the wantlist. | None — verified by absence in the grep. |
| **Build artifacts / installed packages** | None for the watcher (UNTOUCHED — no rebuild, no new tag; a `vX.Y.Z` tag would needlessly fire the watcher CI). The backend binary IS rebuilt+deployed (normal). The `web/` bundle rebuilds (normal). The OLD `web/src/lib/wantlist/groupByChar.ts` + `WantlistPanel.svelte` become dead code → delete to avoid a stale-import build break. (`priority.ts` + `holders.ts` stay — live consumers in `columns.ts`/`guild-views`/`InGuildCell`/`WantAddForm`.) | Delete `groupByChar.ts` (+ test) + `WantlistPanel.svelte` AFTER rewiring `/wishlist`; KEEP `priority.ts` + `holders.ts`. Do NOT tag a `v*` release for this (watcher unchanged — the watcher-release memory note). |

**The canonical question — after every repo file is updated, what runtime systems still hold "wantlist"?**
The LIVE prod SQLite `wantlist_item` table + the FK'd `alert_log` rows. The migration is the ONLY thing
that touches them; once 00014 applies on deploy, no runtime state references the old table. This is a
DATA migration (DROP existing rows) AND a code edit (repoint the matcher) — both must appear in the plan.

## Common Pitfalls

### Pitfall 1: The phantom watcher schema-gate (the CONTEXT's WATCHER_MAX worry is moot)
**What goes wrong:** The CONTEXT + CLAUDE.md mention bumping `WATCHER_MAX_SCHEMA_VERSION` /
`internal/sheet/client.go` and a `_meta.schema_version` bump for breaking migrations. A planner could
waste a task trying to find/bump these.
**Why it happens:** Those are **Google-Sheets-era** mechanisms. `[VERIFIED: internal/sheet/ does not
exist (Glob found 0 files); grep for WatcherMaxSchemaVersion/schema_version finds matches ONLY in
apps-script/, migration doc-comments, and planning docs — ZERO in the off-Google Go backend].` The
off-Google watcher posts to `/api/v1/ingest`; the ingest path has NO schema-version check
`[VERIFIED: grep for schema_version/SchemaVersion in internal/backendsrv/ingest = no matches]`. The
backend's "schema version" IS the goose `goose_db_version` table (migrations 00010 + 00011's own doc
comments explicitly say "Backend-only: NO _meta.schema_version bump and NO WatcherMaxSchemaVersion
change — watcher never touches wantlist_item").
**How to avoid:** State in the plan: there is NO watcher schema gate to bump; "schema v14" simply means
goose migration 00014 applied. The watcher is UNTOUCHED and the ingest API does not gate on schema.
**Warning signs:** A task that says "bump WatcherMaxSchemaVersion in internal/sheet/client.go" — that
file does not exist.

### Pitfall 2: The slot-vocabulary mismatch (canonical Title-case vs wiki UPPERCASE vs wiki-prose)
**What goes wrong:** Suggestions silently return ZERO rows for every slot because the wishlist's
canonical slot (`"Finger1"`) is compared against gear-tier rows keyed on the wiki prose slot
(`"Fingers"`) or the UPPERCASE inv token (`"FINGER1"`).
**Why it happens:** THREE vocabularies coexist: (a) `compute/slotconst.go` canonical Title-case
(`"Finger1"`, the wishlist slot + the paperdoll); (b) `enrich/eqconst.go` `WIKI_SLOT_TO_INV_SLOTS` keys
= wiki prose (`"Fingers"`) → values = UPPERCASE inv (`"FINGER1"`); (c) the `wiki_gear_tier.slot` column =
the wiki prose slot (`"Fingers"`). `slotconst.go`'s own comment warns the two slot maps are deliberately
distinct and reusing one as the other "would silently leave every equipment row unclassified."
**How to avoid:** Build the canonical→wiki-prose inverse: invert `WIKI_SLOT_TO_INV_SLOTS` (UPPERCASE inv
token → wiki prose slot), then map a canonical worn-slot to its UPPERCASE form to look up the wiki slot.
Unit-test it: `"Finger1"` and `"Finger2"` BOTH → `"Fingers"`; `"Ear1"`/`"Ear2"` → `"Ears"`;
`"Head"`→`"Head"`. Verify against real `wiki_gear_tier` data on the live DB.
**Warning signs:** A slot's suggestion list is always empty even for a class that has Velious gear there.

### Pitfall 3: There is NO no-drop/raid column — "Raid" tag = the tier, not a flag
**What goes wrong:** A plan looks for a `no_drop`/`is_raid` column on `wiki_gear_tier` to drive the WISH-04
"Raid" tag and finds none, then invents one or scrapes it.
**Why it happens:** `enrich/wikigear.go`'s `WikiGearTierRow` has ONLY `Tier/Class/Slot/ItemID(always
nil)/ItemName/Rank`. `[VERIFIED: store.GearTierPriceRow + readviews.go SELECT confirm tier/class/slot/
item_name/rank + price columns only — no no-drop flag].` The Velious gear-tier pages encode "raid-only/
no-drop" by which TIER the item sits in: `"Velious Raiding"` items are the raid-tier, no-drop, not-for-
sale set; `"Velious Pre-Raid/Group"` are the grouping/buyable set. `enrich.TierVeliousRaiding` is the
exact literal `"Velious Raiding"`.
**How to avoid:** Tag "Raid" + "not for sale" iff `row.Tier == "Velious Raiding"`. Show BOTH tiers as one
flat complete list (D-04 / WISH-04 explicitly "flat, no Group/Raid binary"). Do NOT scrape a no-drop
flag from the wiki — the watcher/enrich is untouched and no such column exists.
**Warning signs:** A migration adding a `no_drop` column to `wiki_gear_tier`, or an `enrich/wikigear.go`
parser change.

### Pitfall 4: The `alert_log` FK on the retired table (the clean-break sharp edge)
**What goes wrong:** The migration drops `wantlist_item` while `alert_log.wantlist_item_id REFERENCES
wantlist_item(id)` still exists → either the DROP fails (FK in use) or future `alert_log` inserts from a
wishlist ping violate the FK.
**Why it happens:** `[VERIFIED: 00007_notify.sql line 50 — wantlist_item_id INTEGER REFERENCES
wantlist_item(id) ON DELETE CASCADE].` SQLite can't ALTER a FK; you must DROP+CREATE `alert_log`.
**How to avoid:** The migration (Pattern 1) rebuilds `alert_log` to FK `wishlist_item(id)` BEFORE/at the
same step as dropping `wantlist_item`. Decide whether to copy the old `alert_log` inbox history (with
`wishlist_item_id=NULL`) or discard it (clean break — recommended). NOTE: SQLite FK enforcement is
often OFF by default (`PRAGMA foreign_keys`) — verify whether the repo enables it; if FKs are NOT
enforced at runtime, the DROP succeeds regardless, but the rebuild is STILL correct hygiene and required
for `RecentAlertExists`/`InsertAlertTx`'s `wishlist_item_id` column to exist.
**Warning signs:** A migration that drops `wantlist_item` but leaves `alert_log` untouched.

### Pitfall 5: DOM-blind node tests — green ≠ works in the browser
**What goes wrong:** `npm test` passes (node vitest) but the wishlist tab crashes or mis-renders in the
browser — the precedent: P15 had 165 green tests + 2 crashing BLOCKERs; P31 needed 8 fix-forward smoke
commits.
**Why it happens:** `[VERIFIED: web vitest is node-only; no @testing-library/svelte (toolchain-install
rule); roster.ts/examine.ts comments confirm .svelte files are EXCLUDED from the node project].`
**How to avoid:** Extract ALL wishlist logic (viewer-first list, per-slot grouping, auto-removal filter,
slot↔wiki-slot map, search) into pure `.ts` helpers + node-test them; then DEPLOY-then-browser-smoke the
rendered tab. localhost `npm run dev` can't auth against prod (cookie Domain + CORS apex-only) — smoke
on the DEPLOYED build (or a full local stack with SQUIREBOT_COOKIE_INSECURE).
**Warning signs:** "165 tests green" cited as the verification of a Svelte render.

### Pitfall 6: The `alert_log.wantlist_item_id` column rename ripples
**What goes wrong:** Renaming the FK column breaks `store/alertlog.go` (`InsertAlertTx`,
`RecentAlertExists`, the dedup index) and `notify/dm.go` (passes `WantID` → `wantlist_item_id`).
**Why it happens:** `[VERIFIED: alertlog.go InsertAlertTx INSERTs wantlist_item_id; RecentAlertExists
WHERE wantlist_item_id = ?; alert_log_dedup_idx on wantlist_item_id].`
**How to avoid:** TWO clean options — (A) rename the column to `wishlist_item_id` everywhere
(alertlog.go + the dedup index + the migration) for clarity, OR (B) KEEP the column name
`wantlist_item_id` (it's just an integer FK; only the FK TARGET table changes to `wishlist_item(id)`) to
minimize churn — the `notify.Alert.WantID` field and all of alertlog.go stay byte-identical, only the
migration's `REFERENCES` clause changes. **(B) is lower-risk and recommended** unless the planner wants
the cleaner naming. Either way `wantmatch.Hit.WantID` keeps its name (it's the FK id the notify records).
**Warning signs:** Half-renamed — the migration says `wishlist_item_id` but alertlog.go still says
`wantlist_item_id`.

### Pitfall 7: Stale `/wantlist` API routes + dead web components
**What goes wrong:** After repointing the matcher and dropping `wantlist_item`, the 4 old
`/api/v1/wantlist*` routes (lines 342-348 + the mute at 390) still call `store.AddWantTx`/`ListOwnWants`/
etc., which query the now-dropped table → 500 on any hit. And the `/wishlist` placeholder still mounts
`WantlistPanel` (which calls `fetchOwnWants`).
**Why it happens:** D-01/D-03 retire the surface but the route registrations + the placeholder mount are
separate edits.
**How to avoid:** In the SAME phase: remove the `/api/v1/wantlist`, `/api/v1/wantlist/remove`,
`/api/v1/wantlist/guild`, `/api/v1/wantlist/mute` route registrations from `main.go` (and the
`webadmin/wantlist.go` + `store/wantlist.go` + `wantlist_test.go` files, or leave the files but unregister
the routes — planner's call). Rewrite `/wishlist/+page.svelte` to stop mounting `WantlistPanel`; delete
`WantlistPanel.svelte` + `web/src/lib/wantlist/groupByChar.ts` (+ test) ONLY. **KEEP
`web/src/lib/wantlist/priority.ts` (`priorityRank`/`noteRuneCount`) and `holders.ts` (`type Holder`)** —
they have LIVE consumers: `columns.ts:29-30` imports both (and `columns.ts` feeds the live
`guild-views/+page.svelte` + `InGuildCell.svelte`), and `WantAddForm.svelte:27` imports `noteRuneCount`.
Deleting the whole `wantlist/` dir would break `npm run check && npm run build`. The `/wantlist` ROUTE
(`+page.ts`) is ALREADY a 308 redirect (P30) — D-03 is mostly satisfied; just confirm it still redirects.
**Warning signs:** A 500 on `/api/v1/wantlist` after deploy, or a build error from a dangling
`fetchOwnWants`/`priorityRank`/`Holder` import (deleting too much OR too little).

### Pitfall 8: Ping-toggle polarity (default-ON, inverse of the wantlist mute)
**What goes wrong:** A wishlist item is added but never pings because `pinged` defaults to 0, or the
toggle's sense is inverted in the UI.
**Why it happens:** The wantlist used `muted` (default 0 = pinging). WISH-05 frames it as a "ping toggle"
— the natural mental model is `pinged` (default 1 = pinging). The matcher gate flips from `muted = 0` to
`pinged = 1`.
**How to avoid:** Pick ONE polarity and keep it consistent end-to-end: `pinged INTEGER NOT NULL DEFAULT
1`; matcher `WHERE … pinged = 1`; the toggle sets `pinged = 0` to silence. The UI "ping bell" lit = a
notify-default behavior matching the wantlist's default-ON (`notifyprefs.go` D-01 default-ON).
**Warning signs:** A new wishlist item that produces no EC DM despite the EC monitor being live.

## Code Examples

### Reading the equipped item per slot + auto-removal (compute/wishlist.go)
```go
// Source: compute/inventory.go StructuredInventory (Phase 31) + readviews.go GearTierPrices (Phase 29)
func WishlistFor(ctx context.Context, s *store.Store, discordID, char string) (WishlistView, error) {
    inv, err := compute.StructuredInventory(ctx, s, char)   // equipped item per slot + held set
    if err != nil { return WishlistView{}, err }
    targets, err := store.ListOwnWishlist(ctx, s.DB(), discordID, char) // active per-slot targets
    if err != nil { return WishlistView{}, err }
    tiers, err := s.GearTierPrices(ctx)                      // every gear-tier row + name-keyed price
    if err != nil { return WishlistView{}, err }
    meta := charMetaByName[char]                            // CharMeta.Class for class+slot filtering
    // ... per worn-slot: equipped = inv.Equipment[canonical]; suggestions = filter(tiers, class, wikiSlot(canonical));
    //     targets filtered by auto-removal (held[norm(name)]); attach badge from alertedIDs.
}
```

### The owner-scoped ping toggle (store/wishlist.go — the SetMutedTx twin)
```go
// Source: store/wantlist.go SetMutedTx (line 265) — same IDOR guard, inverse column
func SetPingedTx(ctx context.Context, tx *sql.Tx, wishID int64, discordID string, pinged bool) (bool, error) {
    res, err := tx.ExecContext(ctx,
        `UPDATE wishlist_item SET pinged = ? WHERE id = ? AND discord_user_id = ? AND active = 1`,
        boolToInt(pinged), wishID, discordID)
    if err != nil { return false, fmt.Errorf("set pinged (id=%d): %w", wishID, err) }
    n, err := res.RowsAffected()
    if err != nil { return false, fmt.Errorf("set pinged rows-affected (id=%d): %w", wishID, err) }
    return n > 0, nil   // cross-owner → RowsAffected=0 → (false,nil): silent no-op (never leaks existence)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `wantlist_item` (item-centric, optional char tag) | `wishlist_item` (per char + worn slot + target) | This phase | The model shifts from "I want item X" to "this slot on this char could be upgraded to Y." |
| `muted` (default-pinging, opt-OUT per want) | `pinged` (default-ON ping toggle, same default behavior) | This phase | Same net default; the column polarity flips to match the "ping toggle" framing. |
| Suggestions: none (wantlist had no suggestions) | `GearTierPrices()` complete Velious flat list, name-keyed price | This phase consumes the P29 read | The wishlist surfaces actionable upgrades the wantlist couldn't. |
| Google-Sheets `_meta.schema_version` + watcher `WatcherMaxSchemaVersion` gate | goose `goose_db_version`; ingest API has no schema gate | v2.0 "Off Google" (2026-05-31) | The CLAUDE.md schema-gate guidance is stale for this backend. |

**Deprecated/outdated:**
- The CLAUDE.md / CONTEXT references to `internal/sheet/client.go` `WatcherMaxSchemaVersion` and
  `_meta.schema_version` — Google-Sheets-era; that package no longer exists in the off-Google backend.
- The `web/src/lib/wantlist/groupByChar.ts` + `WantlistPanel.svelte` surface — superseded by this phase.
  (`priority.ts` + `holders.ts` are NOT deprecated — still consumed by the consolidated guild-views.)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Discarding the live `wantlist_item` rows + the FK'd `alert_log` history is acceptable (D-01 clean break) | Runtime State / Pattern 1 | If the user wanted the EC inbox history preserved, the `alert_log` rebuild should COPY rows with `wishlist_item_id=NULL` instead of dropping. D-01 says clean break — but the alert_log inbox history is a distinct artifact worth a one-line confirm. |
| A2 | Auto-removal scope = held ANYWHERE on the character (equipment + bags + bank), not equipped-in-THAT-slot-only | Pattern 4 | If "sees it on that character" means equipped-in-slot only, a target wouldn't auto-hide when the item is sitting in a bag. The literal requirement reading is hold-anywhere; this is a UI-SPEC refinement, not a blocker. |
| A3 | Ping default = ON (`pinged DEFAULT 1`), matching the wantlist's default-ON notify behavior | Pattern 1 / Pitfall 8 | If the user wants new wishlist items to default to NOT pinging (opt-in per item), flip the default to 0. The default-ON notify precedent (notifyprefs.go D-01) suggests ON. |
| A4 | `character_id` is NOT NULL on `wishlist_item` (every target is char+slot-scoped; no account-level wishlist) | Pattern 1 | The wishlist is inherently per-character/per-slot, so an account-level target makes no sense — but if the planner wants a "general/unslotted" bucket it'd need nullable. D-01 explicitly rejected the unslotted bucket. |
| A5 | Ammo/Charm/Power worn slots have no Velious gear-tier suggestions (empty list is correct) | Pattern 3 | If the wiki gear-tier pages DO list Ammo items, the inverse slot map must include it. WIKI_SLOT_TO_INV_SLOTS has no Ammo/Charm/Power key — verified — so empty is correct for the shipped data. |

## Open Questions (RESOLVED)

1. **Keep `alert_log` inbox history across the clean break?**
   - What we know: D-01 retires the wantlist; `alert_log` FKs `wantlist_item`. The rebuild discards the
     FK'd rows unless explicitly copied.
   - What's unclear: whether the ~2 weeks of EC alert inbox history should survive (copied with
     `wishlist_item_id=NULL`) or be discarded with the wantlist.
   - Recommendation: discard (clean break, simplest, A1). One-line user confirm during plan/discuss.
   - **RESOLVED:** discard the inbox history — the 00014 migration COPIES NO ROWS into the rebuilt
     `alert_log` (clean break, D-01 / planned in 34-01 Task 1).

2. **One read route or several?**
   - What we know: `GET /api/v1/wishlist/{char}` can return everything for a char (slots + equipped +
     targets + suggestions + badge) in one compute pass; the discretion is explicitly the planner's.
   - Recommendation: ONE route per char (`/api/v1/wishlist/{char}`) returning the full `WishlistView`
     (the `inventory.go` precedent) + the write routes. The character LIST reuses `/api/v1/characters`
     (filter banks/bots client-side, or a thin new read). Fewer routes = fewer round-trips.
   - **RESOLVED:** ONE read route `GET /api/v1/wishlist/{char}` → full `WishlistView` (planned in 34-02
     Task 3); the character LIST reuses `/api/v1/characters` (banks/bots filtered client-side, 34-03).
     The WISH-07 item search lazily `fetchWishlist(char)`s every non-bank/bot character over that same
     per-char route (34-03 Task 2) — no separate guild-wide wishlist-items read route is added.

3. **Is SQLite FK enforcement on at runtime?**
   - What we know: SQLite defaults `PRAGMA foreign_keys = OFF`; whether the repo enables it affects
     whether the DROP-table needs the rebuild-first ordering (it's correct hygiene regardless).
   - Recommendation: order the migration to rebuild `alert_log` before dropping `wantlist_item` so it's
     correct under EITHER setting; check `store/db.go` for a `foreign_keys` pragma during planning.
   - **RESOLVED:** the 00014 migration rebuilds `alert_log` BEFORE dropping `wantlist_item`
     (rebuild-before-drop) — correct under EITHER `PRAGMA foreign_keys` setting (34-01 Task 1, T-34-01).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | backend build/test | ✓ (project builds today) | 1.24 | — |
| goose (embedded) | the new migration | ✓ | in-repo | — |
| modernc.org/sqlite | store CRUD + dup detection | ✓ (in go.mod) | in-repo | — |
| Node + npm (web/) | the Svelte tab build/test | ✓ (web builds today) | in-repo | — |
| vitest (node project) | the pure-`.ts` helper tests | ✓ | in-repo | — |
| Hetzner VPS + SSH (id_ed25519) | the deploy (34-04) | ✓ (ops access documented) | — | — |
| R2 (rclone, squirebot-backups) | the pre-migration backup floor | ✓ | — | — |

**No new dependency is introduced by this phase.** Every tool is already in the repo/toolchain (the
toolchain-install rule does not fire). The ONLY environment action is the 34-04 deploy (build + scp +
restart + goose run + R2 backup), which uses the proven P26/P28/P32/P33 deploy path.

## Sources

- Live codebase (this session, VERIFIED inline): `internal/backendsrv/migrations/00006_wantlist.sql`,
  `00007_notify.sql`, `migrate_test.go`; `store/wantlist.go`, `store/alertlog.go`, `store/readviews.go`,
  `store/assignment.go`; `wantmatch/match.go`, `ec/ec.go`, `ec/embed.go`; `compute/inventory.go`,
  `compute/types.go`, `compute/slotconst.go`; `enrich/eqconst.go`, `enrich/wikigear.go`;
  `readapi/inventory.go`, `readapi/characters.go`; `webadmin/wantlist.go`; `cmd/squirebot-server/main.go`.
- Web: `web/src/lib/api.ts`, `web/src/lib/roster.ts`, `web/src/lib/columns.ts`,
  `web/src/lib/wantlist/{groupByChar,holders,priority}.ts`, `web/src/lib/components/WantlistPanel.svelte`,
  `WantAddForm.svelte`, `cells/InGuildCell.svelte`, `ExaminePanel.svelte`; `web/src/routes/wishlist/+page.svelte`,
  `web/src/routes/wantlist/+page.ts`, `web/src/routes/inventory/+page.svelte`, `web/src/routes/characters/+page.svelte`.
- Phase 29 RESEARCH/SUMMARY (`GearTierPrices` built for WISH-04); Phase 31 SUMMARY (`StructuredInventory`
  paperdoll slots); the 34-CONTEXT.md locked decisions (D-01..D-04); CLAUDE.md (off-Google backend, the
  stale `internal/sheet` note); project memory (off-Google migration, watcher-release-versioning, the
  PigParse/in-game item-id namespace split, the DOM-blind node-test reality).

> NOTE (reconstruction): this RESEARCH.md was rebuilt after an accidental overwrite during the Phase-34
> revision pass; the body above is restored from the in-context copy. The substantive content (Patterns
> 1-6, Pitfalls 1-8, the Assumptions Log, the RESOLVED Open Questions) is intact. The "Sources" and
> "Environment Availability" tails were reconstructed from the verified file list and may differ in
> incidental wording from the original.
