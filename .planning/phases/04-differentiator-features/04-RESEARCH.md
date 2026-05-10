# Phase 4: Differentiator Features — Research

**Date:** 2026-05-10
**Method:** Live curl against P1999 wiki API; fixtures committed under `apps-script/src/__fixtures__/`. Page existence verified via `action=query&list=search`. Documentation cross-checked against real responses; live response wins where they disagree.
**Status:** Complete enough to plan. Two findings overturn 04-CONTEXT.md assumptions — flagged below.

---

## 1. Overturned CONTEXT assumptions

### 1a. There is NO separate "Iksar racial tier" wiki page

ROADMAP §SC-1 and 04-CONTEXT.md both assumed an Iksar racial tier exists as a separate wiki page (alongside `Players:Velious_Pre-Raid_Gear` and `Players:Velious_Raiding_Gear`). **It does not.**

Probed candidates:
- `Players:Iksar_Gear` → 404
- `Iksar_Gear` → 404
- `Players:Iksar` → 404
- `Iksar` → exists, but it's the race description page, not gear

Wiki search for "Iksar gear": zero hits.

**Where Iksar items actually live:** inline on the `Players:Velious_Pre-Raid_Gear` page, embedded within the regular per-class sections. Identifiable by name pattern — every Iksar racial item's name starts with `"Iksar "` (e.g., `Iksar Hide Cap`, `Iksar Hide Leggings`, `Iksar Berserker Club`). The Velious Raiding page has zero Iksar items (Iksar racial gear is mid-tier, not raid).

**Resolution:** Phase 4 doesn't need a third trigger or third fixture for Iksar. The `refreshWikiGearTier` parser detects Iksar items by `name.startsWith('Iksar ')` during Pre-Raid parsing and tags them with `tier='Iksar'` instead of `tier='Velious Pre-Raid/Group'`. Single emit per item — no duplicate rows. `gear_check` filters: show `Iksar`-tier rows only when `char.race === 'IKS'`.

**No schema change needed.** Existing `_wiki_gear_tier.tier` column already supports any string value.

### 1b. There are NO standalone per-class spell pages

CONTEXT speculated about page patterns like `<Class>_spells` or `Spells:<Class>`. Probed:
- `Necromancer_spells` → 404
- `Spells:Necromancer` → 404
- `Necromancer_Spell_List` → 404
- `Necromancer_Spells` → exists but is a 1.9 KB disambig stub (lists references to other pages)
- `Necromancer/Spells` → 404

**Where the spell list actually lives:** the **class page itself** (`https://wiki.project1999.com/Necromancer`). It contains `==Level N==` section headers (one per spell-grant level for that class), and within each section a `{{SpellRow|name=<name>|...}}` template per spell.

**Resolution:** `refreshWikiSpells` fetches one page per class — 14 pages total. ~14 fetches × (1s sleep + ~1s API) = ~30s wall-clock. Comfortably under the 5-min trigger budget — no resumable cursor strictly needed. Plan 04-02 should use the cursor pattern anyway for consistency + safety.

---

## 2. Velious gear-tier wiki page format

### Endpoints
- `https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=Players%3AVelious_Pre-Raid_Gear&redirects=true` — 24,286 bytes wikitext
- `https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=Players%3AVelious_Raiding_Gear&redirects=true` — 25,507 bytes wikitext

Both return the same shape — a class-headed HTML-list document. **Both have exactly 14 class headers** (matches the 14 P99 classes).

### Per-class section structure

```
== [[Bard]] ==

<ul><li>  '''Ears'''       - {{:Fingerbone Hoop}}, {{:Pearly Sarnak Bauble}}, {{:Hammered Golden Loop}}
</li><li> '''Fingers'''    - {{:Velium Diamond Wedding Ring}}, {{:Regal Band of Bathezid}}, {{:Coldain Hero's Insignia Ring}}
</li><li> '''Neck'''       - {{:Chipped Velium Amulet}}, {{:Ayillish's Talisman}} or other [[:Category:+6 Talismans|+6 Talisman]], {{:Dragon Tooth Choker}}
</li><li> '''Head'''       - {{:Circlet of Vallon}}, {{:Circlet of the Falinkan}}, {{:Crown of Rile}}
... etc ...
</li></ul>
```

**Format reliably consistent across all 28 sections (14 classes × 2 tiers):**
- Each `<li>` line: `'''SlotLabel''' - {{:Item1}}, {{:Item2}}, ..., {{:ItemN}}`
- SlotLabel is bolded with triple quotes (wikitext bold)
- Items are `{{:Item Name}}` template transclusions (the `:` prefix means "transclude this article inline")
- Multiple items separated by `,` (rank order: 1st = preferred, 2nd = fallback, etc.)
- Some items have parenthetical notes: `(Chardok 2.0)`, `(Worn)`, `or other ...` — should be stripped from item names but ranked normally

### Slot vocabulary (Pre-Raid + Raiding)

Wiki uses these labels: `Ears, Fingers, Neck, Head, Face, Chest, Arms, Back, Waist, Shoulders, Wrists, Legs, Hands, Feet, Primary, Secondary, Range`

EQ inventory `Location` column uses different vocabulary (per `internal/parse/inventory.go`): `EAR1, EAR2, FACE, NECK, HEAD, SHOULDERS, ARMS, WRIST1, WRIST2, HANDS, FINGER1, FINGER2, CHEST, BACK, WAIST, LEGS, FEET, PRIMARY, SECONDARY, RANGE, AMMO` (also `GENERAL1-8` and `BANK1-N` which aren't equipment slots).

**Slot normalization map** (apps-script/src/lib/eq-constants.ts):
```typescript
export const WIKI_SLOT_TO_INV_SLOTS: Record<string, string[]> = {
  'Ears': ['EAR1', 'EAR2'],
  'Fingers': ['FINGER1', 'FINGER2'],
  'Wrists': ['WRIST1', 'WRIST2'],
  'Neck': ['NECK'], 'Head': ['HEAD'], 'Face': ['FACE'],
  'Chest': ['CHEST'], 'Arms': ['ARMS'], 'Back': ['BACK'],
  'Waist': ['WAIST'], 'Shoulders': ['SHOULDERS'], 'Legs': ['LEGS'],
  'Hands': ['HANDS'], 'Feet': ['FEET'],
  'Primary': ['PRIMARY'], 'Secondary': ['SECONDARY'], 'Range': ['RANGE'],
};
```

For pair slots (Ears, Fingers, Wrists), `gear_check` checks BOTH inventory slots when looking for a match — having the item in either slot counts as OK.

### Iksar items (per finding 1a above)

Pre-Raid page: 24+ items whose names start with `Iksar `. Search regex: `\{\{:\s*Iksar [^}]+\}\}`. These items appear within the regular class sections (e.g., `Iksar Hide Cap` is in the Monk section). The parser tags them with `tier='Iksar'` instead of `tier='Velious Pre-Raid/Group'`.

Raiding page: 0 Iksar items confirmed (no end-of-Velious raid gear is Iksar-racial-locked).

### TypeScript types for `_wiki_gear_tier` rows

```typescript
// apps-script/src/lib/wiki-gear-types.ts
export type Tier = 'Velious Pre-Raid/Group' | 'Velious Raiding' | 'Iksar';

export interface WikiGearTierRow {
  tier: Tier;
  class: string;        // 'WAR', 'CLR', 'PAL', etc. — 3-letter abbrev
  slot: string;         // 'Head', 'Chest', 'Primary', etc. — wiki vocabulary; gear_check normalizes via WIKI_SLOT_TO_INV_SLOTS
  item_id: number | null; // NULL — wiki transclusions don't expose IDs without follow-fetch
  item_name: string;    // verbatim from {{:Name}}, with parenthetical notes stripped
  rank: number;         // 1-based position in the slot's recommendation list
  last_refreshed: string; // ISO 8601
}
```

`item_id: NULL` is acceptable per CONTEXT — the gear_check join can fall back to name-matching when the inv row's item ID doesn't appear in `_wiki_gear_tier`.

### Parser strategy (refreshWikiGearTier)

1. Fetch each of the 2 pages via politeFetch (with redirects=true).
2. Split wikitext on `^==\s*\[\[([^\]]+)\]\]\s*==$` (multiline) — yields per-class sections + the class name.
3. Within each section, find all `<li>` blocks.
4. For each `<li>`: extract slot label via `'''([^']+)'''` regex; extract items via `\{\{:([^}]+)\}\}` regex.
5. For each item: strip parenthetical notes (` (anything)`); rank = position in the comma list.
6. **Iksar tagging:** if `item_name.startsWith('Iksar ')` AND we're parsing the Pre-Raid page, override `tier='Iksar'` for that emit.
7. Map class display name to 3-letter abbrev (via `CLASSES` map).
8. Emit `WikiGearTierRow` per (item, slot) pair.

**Estimated row count:** ~14 classes × ~17 slots × ~2 items = ~480 rows per tier × 2 tiers = ~960 rows + ~24 Iksar rows ≈ 1,000 rows total.

### Sample parser output (Monk Pre-Raid section)

Input (excerpt):
```
== [[Monk]] ==
<ul><li>  '''Ears'''       - {{:Fingerbone Hoop}}, {{:Pearly Sarnak Bauble}}
</li><li> '''Head'''       - {{:Circlet of Vallon}}, {{:Circlet of the Falinkan}}, {{:Crown of Rile}}
```

Expected output:
```
{ tier: 'Velious Pre-Raid/Group', class: 'MNK', slot: 'Ears', item_name: 'Fingerbone Hoop', rank: 1, item_id: null }
{ tier: 'Velious Pre-Raid/Group', class: 'MNK', slot: 'Ears', item_name: 'Pearly Sarnak Bauble', rank: 2, item_id: null }
{ tier: 'Velious Pre-Raid/Group', class: 'MNK', slot: 'Head', item_name: 'Circlet of Vallon', rank: 1, item_id: null }
... etc
```

---

## 3. Per-class spell wiki page format

### Endpoint
`https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=<ClassName>&redirects=true`

Page sizes vary widely:
| Class | Page size | Level headers | SpellRow count |
|-------|-----------|---------------|----------------|
| Necromancer | 71 KB | 25 | (large; not counted) |
| Paladin     | 21 KB | 17 | 66 |
| Warrior     | 11 KB | 0  | 0 |

The Warrior page has zero `==Level N==` headers and zero `{{SpellRow}}` instances (no spells beyond Bind Wound, which the wiki doesn't list as a spell). Parser must handle this degenerate case as an empty result, not an error.

### Per-class spell template structure

Each class page has level-grouped sections:

```
==Level 1==

<table cellpadding='5' cellborder='0' cellspacing='0' width='92%'>
{{SpellHeaderRow |hasType=true}}

{{SpellRow
|name=Cavorting Bones
|type=Summon
|description=Skeleton Warrior, Level 1.
|era={{Classic Short}}
|school=Con.
|location=Vendor.
|mana=15
}}

{{SpellRow
|name=Coldlight
|type=Utility
...
}}
```

**Key fields the parser cares about:**
- `name=<spell name>` — the join key (after normalization)

Optional fields the parser ignores in v1 (could be useful in v2 for tooltips):
- `type`, `description`, `era`, `school`, `location`, `mana`

### Parser strategy (refreshWikiSpells)

1. For each class in `CLASSES` (14 entries):
   a. politeFetch the class's wiki page.
   b. Find all `^==\s*Level\s+(\d+)\s*==$` headers via regex; capture level number.
   c. For each section (text between consecutive level headers), find `{{SpellRow|name=<name>|...}}` templates via regex `\{\{\s*SpellRow\s*\n\s*\|\s*name\s*=\s*([^\n|]+)`.
   d. Trim spell name; compute `normalized_name = name.toLowerCase().replace(/[^a-z0-9]/g, '')`.
   e. Emit row `(class, level, spell_name, normalized_name, last_refreshed)`.
2. Total estimated rows: ~14 classes × ~20 levels × ~5 spells per level = **~1,400 rows**.
3. Sleep 1s between class fetches (per ROADMAP SC-4).

**Pages with zero spells** (Warrior + Rogue + Monk likely): emit zero rows for that class. Not an error.

### TypeScript types for `_wiki_spells` rows

```typescript
// apps-script/src/lib/wiki-spell-types.ts
export interface WikiSpellRow {
  class: string;          // 3-letter abbrev
  level: number;          // 1..60
  spell_name: string;     // verbatim from {{SpellRow|name=...}}
  normalized_name: string; // lowercase + alphanumeric-only — join key
  last_refreshed: string;  // ISO 8601
}
```

### spell_check join logic (clarifies CONTEXT)

For each character `c` with class `cls` and level `lv`:
1. From `_wiki_spells`, get all rows where `class === cls AND level <= lv`.
2. Read `spell:<c>` tab. Each row: `Level | Name`. (Per Phase 2 schema lock; confirmed by user testing 2026-05-09.)
3. Build a `Set<string>` of normalized spellbook spell names.
4. For each `_wiki_spells` row: Status = `KNOWN` if normalized_name is in the set, else `MISSING`.
5. Emit one row per (char, level, spell) combination. Sort: char asc → level asc → spell asc.

---

## 4. Recommendations for the planner

The Plan-phase agent should produce 4 plans:

1. **Plan 04-01: schema_version=3 migration + watcher constant bump + char-info sidebar form**
   - Bump `WatcherMaxSchemaVersion` 2→3 in `internal/sheet/client.go`.
   - Append `race` column to `internal/scaffold/scaffold.go` `DimensionTabs[_char_owner].Headers`.
   - Add `eq-constants.ts` with `CLASSES`, `RACES`, `WIKI_SLOT_TO_INV_SLOTS`.
   - `migrateToV3()` in `apps-script/src/lib/migrations.ts` — idempotent, schema_version-LAST.
   - HtmlService sidebar form `apps-script/src/triggers/showCharInfoSidebar.ts` — reads all `_char_owner` rows, surfaces editable class/level/race per char, save writes back + fires `buildGearCheck` + `buildSpellCheck`.
   - SquireBot menu adds **"Set Character Info…"** item.
   - CI assertion: `Code.ts` exports must match `build.mjs` `TRIGGER_GLOBALS` (Phase 3 lesson from bug `d0a2645`).

2. **Plan 04-02: refreshWikiSpells + spell_check builder + wire onChange**
   - `apps-script/src/lib/wiki-spell-parser.ts` — pure parser per §3 strategy.
   - `apps-script/src/triggers/refreshWikiSpells.ts` — 14 fetches, 1s sleep, resumable cursor (overkill but consistent with `refreshWikiItems`).
   - `apps-script/src/tabs/buildSpellCheck.ts` — buildView-shaped builder.
   - Update `onChange.ts` to also call `buildSpellCheck` (alongside `buildView` + `buildBank`).
   - At end of `refreshWikiSpells`, call `buildSpellCheck` (so users see fresh data immediately).
   - Fixture-tested using `wiki-class-necromancer.json` (66 spells), `wiki-class-paladin.json` (66 spells), `wiki-class-warrior.json` (0 spells degenerate case).

3. **Plan 04-03: refreshWikiGearTier + gear_check builder**
   - `apps-script/src/lib/wiki-gear-tier-parser.ts` — pure parser per §2 strategy + Iksar tagging.
   - `apps-script/src/triggers/refreshWikiGearTier.ts` — 2 fetches, much-less-than-budget, but cursor pattern for safety.
   - `apps-script/src/tabs/buildGearCheck.ts` — joins inventory ↔ wiki gear tier ↔ char class+race; emits OK/MISSING/OTHER rows.
   - Update `onChange.ts` to also call `buildGearCheck`.
   - At end of `refreshWikiGearTier`, call `buildGearCheck`.
   - Fixture-tested using `wiki-velious-preraid-gear.json` + `wiki-velious-raiding-gear.json` + a synthetic char-with-Iksar-race fixture.

4. **Plan 04-04: bank coin sidebar + bank coin row + Range.protect + monitorCellCount + installTriggers update**
   - HtmlService sidebar form `showBankCoinSidebar.ts` — 4 number inputs, save writes `_meta.bank_coin_*`, fires `buildBank`.
   - SquireBot menu adds **"Set Bank Coin…"** item.
   - Update `buildBank` to render the coin row at row 2 of the bank tab.
   - Apply `Range.protect()` to `_meta.bank_coin_*` cells (called once during `migrateToV3` or installTriggers).
   - `monitorCellCount` weekly trigger Sun 03:30 PT.
   - Update `installTriggers.ts` to install 7 triggers (was 4 in Phase 3).

### Test fixture inventory (already committed by this research phase)

- `apps-script/src/__fixtures__/wiki-velious-preraid-gear.json` (24 KB)
- `apps-script/src/__fixtures__/wiki-velious-raiding-gear.json` (25 KB)
- `apps-script/src/__fixtures__/wiki-class-necromancer.json` (71 KB) — pure caster archetype
- `apps-script/src/__fixtures__/wiki-class-paladin.json` (21 KB) — hybrid archetype
- `apps-script/src/__fixtures__/wiki-class-warrior.json` (11 KB) — degenerate-no-spells case

---

## 5. Gaps / Things I couldn't fully verify

1. **Iksar racial item universe is complete** — I confirmed Pre-Raid page contains Iksar items via name-pattern detection (24+ items). Other Iksar items might exist on the Raiding page but didn't trigger my regex. **Mitigation:** parser scans BOTH pages for Iksar-prefixed items (cheap + safe).

2. **Slot vocabulary edge cases on the wiki side** — I sampled the Monk Pre-Raid section. Other classes might use slightly different slot labels (e.g., "Two-handed" vs "Primary" for 2H weapons; `Held` vs `Primary` for shields). Plan 04-03 should add a fallback `unknownSlots: string[]` collection logged to `_meta.last_error` for any wiki slot label NOT in `WIKI_SLOT_TO_INV_SLOTS`.

3. **Spell name collisions across classes** — some spells share names across classes (e.g., DRU/RNG share many nature spells). The `_wiki_spells` rows store `(class, level, spell_name)` so collisions are fine — each class gets its own row. The `spell:<char>` tab doesn't carry class info, but the join is constrained by the char's class anyway.

4. **The `{{SpellRow}}` template's `name` field** — I observed it as the first parameter in every sample. I haven't verified it's always first; if some classes' SpellRow templates put `name` later (e.g., `{{SpellRow|type=...|name=...}}`) my regex misses it. **Fix:** parser should regex all `\|name\s*=\s*([^\n|]+)` within each `{{SpellRow}}` block, not just the first parameter position. Implementation detail; solvable in plan 04-02.

5. **Wiki page rate-limit behavior** — Phase 3 saw zero 429s during ~150 fetches. Phase 4 adds ~14 + ~2 weekly fetches; well below any rate limit. politeFetch's exhaustion logic + retry is in place if it ever happens.

6. **`Range.protect()` permission semantics** — I haven't verified that a script running as the workbook owner can write to a protected range without triggering the protection's "you don't have permission" prompt. Apps Script docs claim protected ranges allow the owner; needs validation in plan 04-04 with a smoke test.

---

## 6. Fixtures committed (review before deletion)

| Fixture | Size | Purpose |
|---------|------|---------|
| `wiki-velious-preraid-gear.json` | 24 KB | Pre-Raid tier parser tests + Iksar tagging |
| `wiki-velious-raiding-gear.json` | 25 KB | Raiding tier parser tests |
| `wiki-class-necromancer.json` | 71 KB | Pure caster spell parser test |
| `wiki-class-paladin.json` | 21 KB | Hybrid class spell parser test |
| `wiki-class-warrior.json` | 11 KB | Degenerate-zero-spells parser test |

5 new fixtures total. Plan 04-02/04-03 should add 1-2 more if they discover unhandled edge cases (e.g., a class with unusual slot labels).

---

*Phase: 04-differentiator-features*
*Research conducted: 2026-05-10*
*Method: orchestrator-direct live curl (no subagent — same approach as Phase 3 due to subagent-timeout pattern)*
*Next step: `/gsd-plan-phase 4` (CONTEXT + RESEARCH both ready). Apply the two corrections from §1 (no separate Iksar page; spell list lives on the class page) — they're already documented here so the planner just needs to follow §4's plan breakdown.*
