# Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades - Context

**Gathered:** 2026-06-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Rework the live v2.2/v2.3 **wantlist** (item-centric, character-tagged) into a **per-character,
per-equipment-slot upgrade wishlist** — the guild-wide answer to *"what can I get to improve my
characters?"* This is the LAST v2.4 phase; it fills the Phase-30 `/wishlist` placeholder and supersedes the
old `/wantlist`. Seven requirements:

1. **WISH-01** — character list (viewer's first A-Z, then others), excludes guild banks/bots, per-character.
2. **WISH-02** — selecting a character shows its **equipped slots** (NOT the in-game window) with the
   currently-equipped item per slot.
3. **WISH-03** — each equipped slot holds an **open-ended** set of user-entered upgrade targets (typed or
   chosen from suggestions); an item leaves the slot's wishlist when SquireBot sees it on that character, or
   when the user removes it.
4. **WISH-04** — per slot, suggest upgrades from the **complete** Velious Pre-raid/Grouping + Raiding lists
   for that character's class+slot (from the existing `_wiki_gear_tier` data); each suggestion shows PigParse
   price, wiki link, last-listed date; no-drop/raid-only tagged "Raid" + shown not-for-sale (no Group/Raid binary).
5. **WISH-05** — each wishlisted item has a Discord **ping toggle**; when SquireBot pings the user (the item
   appeared in the EC tunnel), a **badge** appears beside that item. REUSES the shipped EC-monitor +
   notification spine (not rebuilt).
6. **WISH-06** — hovering/tapping any item shows the right-click-style **examine** (stats, price, wiki, last-synced).
7. **WISH-07** — wishlist **search** covers all items on any wishlist plus the non-bank/bot characters.

**Out of phase scope (owned elsewhere — do not build here):**
- The EC-monitor + notification/DM spine (v2.2 SHIPPED — REUSE `wantmatch`/`notify`/`alert_log`/the EC job;
  only repoint the matcher's source table from `wantlist_item` to the new wishlist table).
- The equipped-slot detection (Phase 31 SHIPPED — REUSE `classifySlot`→`SlotEquipment`/`StructuredInventory`
  for the currently-equipped item per slot; this phase shows a per-slot LIST, not the visual paperdoll window).
- The Velious gear-tier data + class/level (Phase 29 / v2.3 SHIPPED — `_wiki_gear_tier` via `enrich/wikigear.go`,
  `compute/gearcheck.go`, `store/charmeta.go` class/level; name-keyed price/last-listed).
- The 5-tab shell + Wishlist route placeholder + scoped search + notifications badge/inbox (Phase 30 SHIPPED).
- The watcher (UNTOUCHED).
- Exact pixel layout / mobile reflow → `/gsd-ui-phase 34` (UI hint = yes).

</domain>

<decisions>
## Implementation Decisions

All four discussed gray areas were locked to the recommended option (the user's delegate-and-lock pattern —
see [[feedback_delegate_gray_areas]]).

### Existing wantlist data fate (WISH-03)
- **D-01:** **Clean break — retire the old wantlist.** The new per-character/per-slot wishlist is the source of
  truth and starts empty; guildies re-add targets. The v2.2/v2.3 `wantlist_item` (item-centric, character-tagged)
  is retired from app use — its model (item-centric wants) does not map cleanly onto per-slot upgrade targets,
  and best-effort migration would need an awkward "unslotted/general" bucket that doesn't fit the per-slot UI.
  Rejected: best-effort migrate (preserves ~2 weeks of entries but pollutes the per-slot model). NOTE for
  planning: the EC matcher (`wantmatch`) currently reads `wantlist_item` — it MUST repoint to the new wishlist
  table; the migration shape (drop vs leave-dormant the old table) is planner's discretion under the extend-only
  rule (a versioned breaking migration with a `schema_version` bump is acceptable if it drops `wantlist_item`).

### Auto-removal mechanism (WISH-03)
- **D-02:** **Derived (compute-on-read).** A slot's wishlist HIDES any target the character currently has
  equipped/in inventory — nothing is destructively deleted on sighting, so the view is always consistent and
  self-heals if the item later leaves the character. The user's explicit "remove" IS a real delete. Rejected:
  stored prune on upload (destructive; needs an ingest-time hook; re-add required if the item leaves).

### Old /wantlist route (WISH-01)
- **D-03:** **Remove + redirect.** `/wantlist` 308-redirects to `/wishlist`; the old wantlist page is deleted.
  One wishlist surface, consistent with the P30 5-tab restructure (which already moved notifications onto the
  Wishlist tab). Rejected: keep both (two overlapping surfaces).

### Slot scope (WISH-02 / WISH-04)
- **D-04:** **All worn slots, including empty.** All ~21 P31 paperdoll equipment slots get a wishlist + class/slot
  suggestions, even slots the character currently has nothing equipped in (you can wishlist for an empty slot).
  Rejected: only-filled-slots (can't wishlist for a slot you haven't filled yet). Use the SAME worn-slot taxonomy
  as the P31 paperdoll (Charm/Power Source stay omitted — post-Velious, per the P31 close).

### Carried forward (locked upstream — apply as-is)
- **Character list ordering** = viewer's-first-then-A-Z, banks/bots excluded (the P31/P32 `RosterFor` viewer-first
  pattern, MINUS the bank band — WISH-01 explicitly excludes banks/bots).
- **The examine** (WISH-06) = the reused P31 `ExaminePanel` (the single escaped `composeItemNote` `{@html}` sink;
  D-08 order, D-09 graceful omission).
- **Suggestions** (WISH-04) come from `_wiki_gear_tier` (the data `compute/gearcheck.go` already reads) keyed by
  the character's class (`store/charmeta.go`) + slot; show the COMPLETE Pre-raid/Grouping + Raiding lists flat;
  no-drop/raid-only → "Raid" tag + not-for-sale. Name-keyed PigParse price + last-listed (P29).
- **Ping toggle + badge** (WISH-05) reuse the shipped EC-monitor + `notify`/`alert_log` spine; per-want ping
  toggle = the existing notify mechanism; the badge derives from an `alert_log` hit for that wishlist item.
- **EQ theme tokens only**, 44px touch targets, focus-visible 2px accent outline.
- **Node web tests are DOM-blind** ([[web-tests-node-only-blind-to-dom]]) → browser-smoke on a DEPLOYED build
  ([[web-local-dev-cant-auth-against-prod]]).

### Claude's Discretion (researcher/planner owns these)
- **The new schema** — a NEW wishlist table (per character + slot + item target + ping-toggle flag), schema → v14.
  Extend-only; whether the migration also drops the retired `wantlist_item`/its indexes (D-01) is planner's call
  (a `schema_version` bump + idempotent migration if it drops). The `wantmatch` matcher repoint is the load-bearing
  reuse seam.
- **Auto-removal join** — how the compute-on-read filter (D-02) matches a wishlist target against the character's
  current inventory (normalized name vs item_id — almost certainly normalized name, the established join key).
- **Suggestion mapping** — class+slot → `_wiki_gear_tier` rows; how `gearcheck.go`'s existing tier logic is reused
  vs a new per-slot read; the "Raid"/not-for-sale tagging from the gear-tier no-drop flag.
- **Whether the per-slot wishlist + suggestions are one new read-API route or several**; the new pure web helper(s)
  + `fetch*()` wrappers; compute-on-read; `?`-bound; session-gated (`RequireSession`, login-gated — the wishlist
  is OWNER-scoped per the v2.2 IDOR-safe `wantlist`/`account.go` pattern, NOT officer/guild-wide for writes).
- Exact per-slot list/accordion layout, the equipped-item display, mobile reflow → UI-SPEC.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — "Phase 34: Wishlist Rework — Per-Character Per-Slot Upgrades" (goal, 5 success criteria,
  Depends on 29/30/31 + the v2.2 EC/notification spine).
- `.planning/REQUIREMENTS.md` — WISH-01..07.

### Reused subsystems (the load-bearing reuse — MUST read)
- `internal/backendsrv/wantmatch/match.go` — `ForItem`/`ForName` (the EC matcher to REPOINT from `wantlist_item`
  to the new wishlist table; WISH-05).
- `internal/backendsrv/store/{wantlist.go,alertlog.go,notifyprefs.go}` + migrations `00006_wantlist`,
  `00007_notify`, `00010_character_tagged_wantlist`, `00011_wantlist_drop_reason_dedup` — the retired wantlist +
  the notify/alert_log spine to reuse.
- `internal/backendsrv/compute/gearcheck.go` + `internal/backendsrv/enrich/wikigear.go` — the `_wiki_gear_tier`
  Velious data + tier logic for WISH-04 suggestions.
- `internal/backendsrv/store/charmeta.go` — `CharMeta{class, level, race}` (the class for class+slot suggestions).
- `internal/backendsrv/compute/inventory.go` — `classifySlot`/`SlotEquipment`/`StructuredInventory` (the
  currently-equipped item per slot, WISH-02; the auto-removal inventory join, D-02).

### Prior phase context (continuity)
- `.planning/phases/31-characters-tab-in-game-inventory-window/31-CONTEXT.md` — the equipped-slot taxonomy,
  `ExaminePanel`, the `RosterFor` viewer-first list.
- `.planning/phases/30-app-shell-5-tab-navigation/30-CONTEXT.md` — the Wishlist tab + scoped search + the
  notifications badge/inbox living on this tab; the route placeholder + the redirect pattern (old paths 308).
- `.planning/phases/29-data-foundation-inventory-parse-price-value-aggregation/29-CONTEXT.md` — name-keyed
  price/last-listed; the `_wiki_gear_tier` data.

### Project guidelines
- `CLAUDE.md` — compute-on-read, **extend-only** schema (migrations version-stamped + idempotent + `schema_version`
  bump for breaking changes — relevant to the D-01 retire), watcher untouched, EQ-theme single `[data-theme]`
  writer, owner-scoped IDOR-safe writes (the `account.go`/`wantlist` pattern), session-gated reads.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (backend)
- `wantmatch/match.go` (`ForItem`/`ForName`) — the EC matcher; repoint its source to the new wishlist table (WISH-05).
- `store/{wantlist.go,alertlog.go,notifyprefs.go}` + the `notify` package + the EC job (v2.2) — the ping/DM/badge spine.
- `compute/gearcheck.go` + `enrich/wikigear.go` — `_wiki_gear_tier` per class+slot (WISH-04 suggestions).
- `store/charmeta.go` (`CharMeta.class`) — class for the class+slot suggestion lookup.
- `compute/inventory.go` (`classifySlot`/`SlotEquipment`/`StructuredInventory`) — equipped item per slot (WISH-02) +
  the compute-on-read auto-removal join (D-02).
- `internal/backendsrv/readapi/*` + `store/readviews.go` (`RosterFor`) — the session-gated read-API + viewer-first
  list (WISH-01, banks/bots excluded).

### Reusable Assets (web)
- `web/src/routes/wishlist/+page.svelte` — the Phase-30 **placeholder** to build into the real wishlist tab.
- `web/src/routes/wantlist/+page.svelte` + `web/src/lib/wantlist/{groupByChar,holders,priority}.ts` — the OLD
  wantlist page/helpers (D-03: redirect /wantlist→/wishlist + delete the old page; mine the helpers for reuse).
- `web/src/lib/components/ExaminePanel.svelte` (P31) — the reused examine (WISH-06).
- `web/src/routes/characters/+page.svelte` + `web/src/lib/roster.ts` (P31) — the viewer-first list pattern (WISH-01).
- `web/src/lib/api.ts` — the credentialed fetch wrappers (add wishlist `fetch*()` twins).

### Established Patterns
- Compute-on-read (`compute/` zero SQL); extend-only `goose` (version-stamped, idempotent); `?`-bound, never name-concat.
- OWNER-scoped IDOR-safe writes (the v2.2 `wantlist`/`account.go` shape — a guildie mutates only their OWN wishlist,
  audited); session-gated reads. V7 slog counts/status only.
- Pure DOM-free helpers (`.ts`) node-tested; the rendered tab browser-smoked on deploy.

### Integration Points
- NEW wishlist schema (migration → v14) + read/write API (owner-scoped) + the `wantmatch` repoint; the web `/wishlist`
  tab consumes the P30 shell + scoped search + notifications badge; the examine reuses `ExaminePanel`; suggestions
  read `_wiki_gear_tier`; the equipped item + auto-removal read `StructuredInventory`.

</code_context>

<specifics>
## Specific Ideas

- Per-character view = a per-slot list/accordion (NOT the visual paperdoll window): each worn slot shows the
  currently-equipped item + that slot's open-ended wishlist targets + the class/slot Velious suggestions.
- A wishlist target row = name + PigParse price + Wiki link + last-listed date + a "Raid" tag (no-drop/raid-only,
  not-for-sale) + the ping-toggle + (when pinged) the EC-hit badge + the hover/tap examine.
- The wishlist is OWNER-scoped (you edit your own characters' wishlists); the search (WISH-07) spans all wishlists +
  the non-bank/bot characters.
- Reuse the live EQ themes unchanged.

</specifics>

<deferred>
## Deferred Ideas

- Migrating the old wantlist entries (rejected by D-01 — clean break).
- Group/Raid binary tiering (WISH-04 explicitly: a flat complete list with a "Raid" tag, NOT a Group/Raid split).
- Cross-character "shared wishlist" / officer views — out of scope (owner-scoped only).
- After this phase: v2.4 is feature-complete → milestone audit/close (`/gsd-audit-milestone` → `/gsd-complete-milestone`).

None dropped — each is out of scope or a future consideration.

</deferred>

---

*Phase: 34-wishlist-rework-per-character-per-slot-upgrades*
*Context gathered: 2026-06-18*
