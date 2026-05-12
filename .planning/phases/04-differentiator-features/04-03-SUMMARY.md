---
phase: 04-differentiator-features
plan: 03
subsystem: apps-script-wiki-gear-tier-scrape-and-gear-check-builder
tags: [apps-script, gear-check, enrich-04, check-02, check-03, iksar-tier, velious]
requires:
  - 04-01 (eq-constants WIKI_SLOT_TO_INV_SLOTS many-to-many pair-slot mapping; _char_owner.race column from schema v=3 for Iksar filter)
  - 04-02 (refreshWikiSpells shape cloned for refreshWikiGearTier; buildSpellCheck cloned for buildGearCheck; onChange wiring extended)
  - 03-02 (politeFetch HTTP wrapper)
provides:
  - "ENRICH-04: weekly Velious gear-tier wiki scrape — refreshWikiGearTier fetches Players:Velious_Pre-Raid_Gear and Players:Velious_Raiding_Gear via politeFetch redirects=true; per-class <li> block parser with Iksar racial tagging"
  - "CHECK-02/03: gear_check tab — 7 cols (Char, Class, Tier, Slot, Have, Recommended, Status); Status = OK | MISSING | OTHER; sorted Char asc, Tier asc (Iksar last), Slot asc"
  - "CHECK-05 remaining: onChange wiring for inv:* ↔ _wiki_gear_tier dependency triggers buildGearCheck (10s debounced)"
  - "wiki-gear-tier-parser pure function: splits wikitext on ^==\\[\\[ClassName\\]\\]==$ headers; finds <li> blocks; extracts slot via /'''slot'''/; extracts items via /\\{\\{:Item Name\\}\\}/g; strips parenthetical notes from item names"
  - "Iksar tagging: when parsing the Pre-Raid page, items whose stripped name starts with 'Iksar ' emit with tier='Iksar' (overriding 'Velious Pre-Raid/Group')"
  - "Full _wiki_gear_tier replace at end-of-2-fetches (NOT per-page): partial writes would leave inconsistent state given 2-page collective definition"
  - "Unknown wiki slot labels collected and logged to _meta.last_error as warning {kind: 'unknown_wiki_slots', detail: comma-list}"
affects:
  - "04-04 (installTriggers expansion): adds refreshWikiGearTier Sunday 05:00 PT trigger (offset from refreshWikiSpells Sunday 04:00 PT to avoid lock contention)"
  - "Phase 4 SHIPPED: 04-03 + 04-04 jointly enable the live-smoke that ships v0.4.0"
tech-stack:
  added: []
  patterns:
    - "Iksar racial tagging via item-name prefix match: items starting with 'Iksar ' in the Velious Pre-Raid page emit with tier='Iksar' instead of 'Velious Pre-Raid/Group'. Single-emit per item (no double-row). Locked by RESEARCH §2."
    - "Many-to-many pair-slot matching: WIKI_SLOT_TO_INV_SLOTS['Ears']=['EAR1','EAR2'] etc. Char OK if EITHER slot has the recommended item. Char's invSlots query is set-membership across all in WIKI_SLOT_TO_INV_SLOTS[slot]."
    - "Parenthetical strip on item names: 'Whetstone (Worn)' → 'Whetstone'. Wiki has both forms inconsistently. Locked by PLAN truth #3."
    - "Full-table replace at end-of-2-fetches (not per-page): partial fetches would leave the table in inconsistent state because the 2 pages collectively define the entire _wiki_gear_tier content. Cursor pattern reused for code-shape consistency but single end-of-run write."
    - "Per-(class, slot) recommendation list: multiple recommended items per slot (rank=1, 2, 3...). buildGearCheck emits one gear_check row per (char × tier × slot × recommended-item)."
    - "OK/MISSING/OTHER status semantics: OK = char has the recommended item in any pair-slot; MISSING = char has nothing in any pair-slot; OTHER = char has SOMETHING in those slots but not the recommended item."
key-files:
  created:
    - apps-script/src/lib/wiki-gear-tier-types.ts (~50 lines; Tier union type + WikiGearTierRow + GearTierParseResult)
    - apps-script/src/lib/wiki-gear-tier-parser.ts (~180 lines; parseGearTierPage pure function with Iksar tagging + parenthetical strip + unknown-slot collection)
    - apps-script/src/triggers/refreshWikiGearTier.ts (~180 lines; 2-page fetch + per-page parse + accumulated full-replace + end-of-run buildGearCheck)
    - apps-script/src/tabs/buildGearCheck.ts (~220 lines; multi-tier filtering + Iksar race gate + OK/MISSING/OTHER status + sort)
    - apps-script/src/__tests__/wiki-gear-tier-parser.test.ts (~200 lines; 8 vitest scenarios)
    - apps-script/src/__tests__/refreshWikiGearTier.test.ts (~180 lines; 5 vitest scenarios)
    - apps-script/src/__tests__/buildGearCheck.test.ts (~250 lines; 9 vitest scenarios)
  modified:
    - apps-script/src/Code.ts (+2 lines; export refreshWikiGearTier + buildGearCheck)
    - apps-script/src/triggers/onChange.ts (+1 line; call buildGearCheck after buildSpellCheck)
    - apps-script/build.mjs (+2 lines; add 2 new TRIGGER_GLOBALS entries)
decisions:
  - "Iksar items emit ONCE with tier='Iksar' override (not duplicated in 'Velious Pre-Raid/Group'): Iksar-tier rows shown ONLY for chars whose race='IKS'. Avoids double-listing for non-Iksar chars."
  - "Item IDs are null in _wiki_gear_tier: wiki transclusions {{:Item Name}} don't expose IDs. Match-by-name (case-insensitive) is the only join key. Item ID resolution (following transclusions) would be wiki-traffic heavy; deferred to v2."
  - "Full-table replace at end-of-2-fetches: 2 pages collectively define the entire table; per-page replace would leave inconsistent partial state if second fetch fails. Single accumulated write."
  - "Unknown wiki slot labels logged but not blocking: P1999 wiki occasionally introduces new slot vocab (`GreaterEars` etc.). _meta.last_error captures for review; build continues."
  - "Tier sort order: 'Velious Pre-Raid/Group' (1), 'Velious Raiding' (2), 'Iksar' (3). Iksar listed last per UI convention (racial supplement to baseline tiers)."
  - "gear_check rank-aware comparison (knowing item X is better than recommended Y) DEFERRED to v2: requires per-item stat comparison logic not in scope for v1."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-10T18:24:59-05:00
  tasks_completed: 6 of 6
  commits: 5 (d5e5786 feat wiki-gear-tier types + parser + tests; 4877f8d feat refreshWikiGearTier trigger + tests; f2e5f19 feat buildGearCheck builder + tests; 7e28a6c chore wire gear_check into onChange + Code.ts + TRIGGER_GLOBALS; 9df0a25 chore STATE.md -> plan 04-03 complete)
  files_changed: 10 (7 created + 3 modified, ~1260 lines added)
  tests_added: 22 (8 wiki-gear-tier-parser + 5 refreshWikiGearTier + 9 buildGearCheck)
  trigger_count_after: 4 (refreshWikiGearTier time-driven trigger registration deferred to 04-04 installTriggers expansion 4→7)
  schema_version_after: 3 (unchanged from 04-01)
  watcher_rebuild_required: false (apps-script-only)
---

# Phase 4 Plan 03: Velious Gear-Tier Scrape + gear_check Builder Summary

**One-liner:** Shipped **the headline differentiator** (ENRICH-04 + CHECK-02/03/05) — `refreshWikiGearTier` fetches Players:Velious_Pre-Raid_Gear and Players:Velious_Raiding_Gear via politeFetch with redirects=true, parses per-class `<li>` blocks with slot extraction (`/'''([^']+)'''/`) and item extraction (`/\{\{:([^}]+)\}\}/g`), strips parenthetical notes from item names, tags items starting with `'Iksar '` in the Pre-Raid page with tier='Iksar' (single-emit override), full-replaces `_wiki_gear_tier` at end-of-2-fetches and calls `buildGearCheck` — which joins `inv:* ↔ _wiki_gear_tier ↔ _char_owner.race+class` using the many-to-many `WIKI_SLOT_TO_INV_SLOTS` pair-slot mapping to emit OK/MISSING/OTHER status per (char × tier × slot × recommended-item), filtering Iksar-tier rows to race='IKS' chars only. This is the answer to "what does my character still need?"

## What shipped

### Task 1-2 — wiki-gear-tier-types + parser (commit `d5e5786`)

`Tier = 'Velious Pre-Raid/Group' | 'Velious Raiding' | 'Iksar'`. `WikiGearTierRow`: `{tier, class, slot, item_id: null, item_name, rank, last_refreshed}`. `GearTierParseResult` discriminated union with `unknownSlots: string[]` collection.

`parseGearTierPage(wikitext, tier)`:
1. Length guard.
2. Split on `/^==\s*\[\[([^\]]+)\]\]\s*==$/m` — class display names + section bodies. If 0 class headers → `no_class_sections`.
3. For each `(className, sectionBody)`: map className → 3-letter abbrev via `CLASS_DISPLAY_TO_ABBREV[className]`; skip if not in map.
4. Find `<li>...</li>` blocks via `/<li[^>]*>([\s\S]*?)<\/li>/g`.
5. Per-block: extract slot via `/'''([^']+)'''/`; track unknown slots; extract items via `/\{\{:([^}]+)\}\}/g`; strip parenthetical notes via `s.replace(/\s*\([^)]*\)\s*/g, '')`.
6. For each item: rank = position + 1. **Iksar tagging:** if `tier === 'Velious Pre-Raid/Group' && itemName.startsWith('Iksar ')`, override `effectiveTier = 'Iksar'`. Emit row.
7. Return `{ ok: true, rows, classCount, itemCount, unknownSlots }`.

8 vitest scenarios against 2 fixtures + edge cases: Pre-Raid (classCount=14, ~480+ rows, Iksar count >20); Raiding (classCount=14, ZERO Iksar rows); "Iksar Hide Cap" → `tier: 'Iksar' + class: 'MNK' + slot: 'Head'`; "Cloak of Flames" → regular Velious Pre-Raid/Group; parenthetical strip (`Whetstone (Worn)` → `Whetstone`); empty wikitext; no headers → `no_class_sections`; unknown-slot tracking (synthetic `'''GreaterEars'''` → `unknownSlots: ['GreaterEars']`).

### Task 3 — refreshWikiGearTier trigger (commit `4877f8d`)

Cloned refreshWikiSpells shape. 2 hardcoded page names: `Players:Velious Pre-Raid Gear`, `Players:Velious Raiding Gear`. Per-page handler: fetch, `parseGearTierPage(wikitext, tier)`, accumulate rows.

**Replacement strategy** (different from per-class block replace):
```typescript
function replaceAllWikiGearTier(allRows): void {
  const sheet = ss.getSheetByName('_wiki_gear_tier');
  const lastRow = sheet.getLastRow();
  if (lastRow > 1) sheet.getRange(2, 1, lastRow - 1, 7).clearContent();
  if (allRows.length > 0) {
    sheet.getRange(2, 1, allRows.length, 7).setValues(allRows.map(...));
  }
}
```

Single end-of-run write (after both fetches succeed); cursor pattern reused for code-shape consistency but 2 fetches make it overkill. On both-fetches-fail: failure-threshold abort + error written; existing _wiki_gear_tier preserved.

End-of-run: `writeMetaRow('_meta', 'last_wiki_gear_refresh', now)`; `writeMetaRow('_status', 'last_wiki_gear_count', totalRows)`; if `unknownSlots.length > 0` write warning to `_meta.last_error`; call `buildGearCheck()`.

5 vitest scenarios: happy path (~960 rows); Pre-Raid 404 → Raiding still processes; both 404 → failure threshold abort; unknown slot detected → warning; buildGearCheck called at end.

### Task 4 — buildGearCheck builder (commit `f2e5f19`)

Cloned buildSpellCheck shape with multi-tier filtering + race gate.

```typescript
const TIER_SORT_ORDER = { 'Velious Pre-Raid/Group': 1, 'Velious Raiding': 2, 'Iksar': 3 };

function runBuild(): void {
  const chars = readCharOwnerWithMetadata(ss);
  const wikiGear = readWikiGearTier(ss); // Map<tier, Map<class, WikiGearTierRow[]>>
  const charInventories = readInventoriesByChar(ss); // Map<char, InvRow[]>

  for (const c of chars) {
    if (!c.class) continue;
    const tiersToShow = ['Velious Pre-Raid/Group', 'Velious Raiding'];
    if (c.race === 'IKS') tiersToShow.push('Iksar');  // race gate
    for (const tier of tiersToShow) {
      const classGear = wikiGear.get(tier)?.get(c.class) ?? [];
      const bySlot = groupBy(classGear, g => g.slot);
      for (const [slot, recommendations] of bySlot) {
        const invSlots = WIKI_SLOT_TO_INV_SLOTS[slot] ?? [];
        const charItemsInSlots = (charInventories.get(c.char_name) ?? [])
          .filter(it => invSlots.includes(it.location));
        for (const rec of recommendations) {
          const matched = charItemsInSlots.find(it =>
            it.itemName.toLowerCase() === rec.item_name.toLowerCase()
          );
          let status, have;
          if (matched) { status = 'OK'; have = matched.itemName; }
          else if (charItemsInSlots.length > 0) { status = 'OTHER'; have = charItemsInSlots[0].itemName; }
          else { status = 'MISSING'; have = ''; }
          rows.push([c.char_name, c.class, tier, slot, have, rec.item_name, status]);
        }
      }
    }
  }
  rows.sort(/* char asc, tier-rank asc, slot asc */);
}
```

9 vitest scenarios: happy NEC-lvl60-HUM with 5 entries (2 matches → 2 OK + 3 MISSING); race=IKS adds Iksar tier rows; race=HUM excludes Iksar; OTHER status (HEAD has wrong item); MISSING status (HEAD empty); pair-slot match (Iksar Hide Cap in EAR2 → OK); char without metadata skipped; lock contention silent; sort order verified.

### Task 5-6 — Wire onChange + Code.ts + build.mjs + STATE.md (commits `7e28a6c`, `9df0a25`)

`onChange.ts`: appended `buildGearCheck()` call after `buildSpellCheck()`. Now full chain: `buildView() → buildBank() → buildSpellCheck() → buildGearCheck()`.

`Code.ts`: 2 new re-exports — `refreshWikiGearTier`, `buildGearCheck`.

`build.mjs` TRIGGER_GLOBALS: 2 new entries. assertExportsMatchGlobals passes.

## Deviations from Plan

None — plan executed as written. (Detailed deviation tracking not captured retroactively.)

## Schema impact

None — schema_version remains at 3. This plan POPULATES `_wiki_gear_tier` (existing scaffold tab) + adds the new `gear_check` view tab (existing scaffold tab). No new columns, no new rows, no migration.

## Verification log

```
$ npm test -- wiki-gear-tier-parser
Tests       8 passed (8)

$ npm test -- refreshWikiGearTier
Tests       5 passed (5)

$ npm test -- buildGearCheck
Tests       9 passed (9)

$ npm run build
(exit 0 — 18 trigger globals; assertExportsMatchGlobals passes)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/lib/wiki-gear-tier-types.ts`
- FOUND: `apps-script/src/lib/wiki-gear-tier-parser.ts`
- FOUND: `apps-script/src/triggers/refreshWikiGearTier.ts`
- FOUND: `apps-script/src/tabs/buildGearCheck.ts`
- FOUND: `apps-script/src/__tests__/wiki-gear-tier-parser.test.ts`
- FOUND: `apps-script/src/__tests__/refreshWikiGearTier.test.ts`
- FOUND: `apps-script/src/__tests__/buildGearCheck.test.ts`

**Commits exist:**
- FOUND: `d5e5786` — feat(apps-script): wiki-gear-tier types + parser + tests
- FOUND: `4877f8d` — feat(apps-script): refreshWikiGearTier trigger + tests
- FOUND: `f2e5f19` — feat(apps-script): buildGearCheck builder + tests
- FOUND: `7e28a6c` — chore(apps-script): wire gear_check into onChange + Code.ts + TRIGGER_GLOBALS
- FOUND: `9df0a25` — chore: STATE.md -> plan 04-03 complete

## Next plan

`/gsd-execute-phase 4` spawned plan `04-04` — the Phase 4 wrap: bank coin sidebar (BANK-01..04), cell-count monitoring (OPS-07), Range.protect on bank_coin cells, and the installTriggers expansion 4 → 7 (adds Sunday refreshWikiSpells + refreshWikiGearTier + monitorCellCount). After 04-04 lands: Phase 4 CODE-COMPLETE → live smoke against real workbook → ship v0.4.0.

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 04-03-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
