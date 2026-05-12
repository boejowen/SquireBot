---
phase: 04-differentiator-features
plan: 02
subsystem: apps-script-wiki-spell-scrape-and-spell-check-builder
tags: [apps-script, spell-check, enrich-03, check-04, wiki-class-pages]
requires:
  - 04-01 (eq-constants CLASSES + CLASS_DISPLAY_TO_ABBREV; _char_owner.race column from schema v=3; charInfoSidebar populated class+level)
  - 03-02 (politeFetch HTTP wrapper reused verbatim)
  - 03-03 (refreshWikiItems shape cloned as template for refreshWikiSpells)
provides:
  - "ENRICH-03: weekly per-class wiki page scrape — refreshWikiSpells fetches https://wiki.project1999.com/api.php?action=parse&page=<ClassDisplayName>&redirects=true for all 14 classes; 1s inter-request sleep"
  - "CHECK-04: spell_check tab — 5 cols (Char, Class, Level, Spell, Status); Status = KNOWN | MISSING; sorted Char asc, Level asc, Spell asc; per-char filter level<=char.level"
  - "CHECK-05 partial: onChange wiring for spell:* AND inv:* tabs triggers buildSpellCheck (10s debounced); end-of-refreshWikiSpells calls buildSpellCheck so users see fresh data immediately"
  - "wiki-spell-parser pure function: splits wikitext on ^==Level N==$ headers; extracts {{SpellRow|name=...}} templates with name= parameter anywhere within (not first-position only); normalizeSpellName(s) helper for join keys"
  - "Warrior class (degenerate, zero spells, zero ==Level N== headers) parses cleanly to 0 rows — not an error"
  - "Resumable cursor pattern (cloned from refreshWikiItems): overkill at 14 fetches but consistent with project pattern; cursor key 'wiki_spells_refresh_cursor'"
  - "Per-class _wiki_spells block replace: delete existing class rows bottom-up + append new ones — composite key (class, level, spell_name)"
affects:
  - "04-03 (refreshWikiGearTier + buildGearCheck): clones refreshWikiSpells shape for the 2-page Velious gear scrape; clones buildSpellCheck for buildGearCheck"
  - "04-04 (installTriggers expansion): adds refreshWikiSpells Sunday 04:00 PT trigger"
tech-stack:
  added: []
  patterns:
    - "Multi-template SpellRow handling: P1999 wiki pages use SpellRow / RadSpellRow / RadSpellRow2 / SongRow variants. Initial implementation handled only SpellRow; live-smoke fix-pack 9319c6b extended to all 4 variants (5 → 11 caster classes covered)."
    - "Name parameter robustness: |name= may not be FIRST parameter inside {{SpellRow}}. Regex \\|\\s*name\\s*=\\s*([^\\n|]+) matches anywhere within the template body. Locked by RESEARCH §5 #4."
    - "normalizeSpellName: trim + toLowerCase + strip leading 'Spell: ' prefix + drop non-alphanumeric. Stable join key between wiki spells and spellbook tab Name column (which P99 dumps verbatim with quirks)."
    - "Per-class block replace (not full-replace): bottom-up deleteRow scan keyed on class column, then append; preserves other classes' rows during partial-resume scenarios."
    - "end-of-trigger buildSpellCheck call: users see fresh data immediately after refreshWikiSpells completes without waiting for next onChange. Same pattern adopted by 04-03 (refreshWikiGearTier → buildGearCheck)."
key-files:
  created:
    - apps-script/src/lib/wiki-spell-types.ts (~60 lines; WikiSpellRow + SpellParseResult discriminated union + normalizeSpellName helper)
    - apps-script/src/lib/wiki-spell-parser.ts (~140 lines; parseClassPage pure function; handles SpellRow variants + degenerate zero-level case)
    - apps-script/src/triggers/refreshWikiSpells.ts (~280 lines; cloned refreshWikiItems shape with cursor + per-class replace + end-of-run buildSpellCheck call)
    - apps-script/src/tabs/buildSpellCheck.ts (~180 lines; lock + debounce + join + sort + applyTheme)
    - apps-script/src/__tests__/wiki-spell-parser.test.ts (~190 lines; 6 vitest scenarios against 3 fixtures + edge cases)
    - apps-script/src/__tests__/refreshWikiSpells.test.ts (~210 lines; 6 vitest scenarios)
    - apps-script/src/__tests__/buildSpellCheck.test.ts (~180 lines; 7 vitest scenarios)
  modified:
    - apps-script/src/Code.ts (+2 lines; export refreshWikiSpells + buildSpellCheck)
    - apps-script/src/triggers/onChange.ts (+1 line; call buildSpellCheck after buildBank)
    - apps-script/build.mjs (+2 lines; add 2 new TRIGGER_GLOBALS entries; CI assertion catches mis-sync)
decisions:
  - "Resumable cursor pattern reused at 14-fetch scale even though overkill: project-wide consistency wins over micro-optimization."
  - "Warrior (0 spells, 0 level headers) parses cleanly to 0 rows as success: degenerate case must NOT be a failure (would trip 50% failure-threshold abort)."
  - "Per-class block replace (not full _wiki_spells clear+rewrite): preserves prior-class rows during mid-run cursor pauses + partial failures."
  - "normalizeSpellName join key is lossy by design: 'Spell: Burst of Flame' and 'Burst of Flame' both normalize to 'burstofflame'. Captures P99 spellbook dump quirks where row data sometimes has 'Spell:' prefix and sometimes doesn't."
  - "End-of-trigger buildSpellCheck call: users see fresh data immediately. Tradeoff = 1 extra build per weekly run; acceptable."
  - "Auto-derive level from spellbook (highest spell ≈ char level) REJECTED per CONTEXT — unreliable for low-level chars + alts. User-entry via 04-01 sidebar is the only reliable signal."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-10T18:04:38-05:00
  tasks_completed: 6 of 6
  commits: 5 (9bfc63e feat wiki-spell-types + parser + tests against 3 fixtures; 0cebef5 feat refreshWikiSpells trigger with resumable cursor + tests; 7069f23 feat buildSpellCheck builder + tests; b6dacaf chore wire spell_check into onChange + Code.ts + TRIGGER_GLOBALS; 80d59d8 chore STATE.md → plan 04-02 complete)
  files_changed: 10 (7 created + 3 modified, ~1245 lines added)
  tests_added: 19 (6 wiki-spell-parser + 6 refreshWikiSpells + 7 buildSpellCheck)
  trigger_count_after: 4 (refreshWikiSpells time-driven trigger registration deferred to 04-04 installTriggers expansion 4→7)
  schema_version_after: 3 (unchanged from 04-01)
  watcher_rebuild_required: false (apps-script-only)
---

# Phase 4 Plan 02: Wiki Spell Scrape + spell_check Builder Summary

**One-liner:** Shipped the per-class wiki spell scrape (ENRICH-03) + `spell_check` consolidated tab (CHECK-04, CHECK-05 partial) — `refreshWikiSpells` fetches one wiki page per class via politeFetch with 1s courtesy sleep, parses `==Level N==` section headers + `{{SpellRow|name=...}}` templates (and the 3 variant templates discovered live-smoke), full-replaces `_wiki_spells` per class block, calls `buildSpellCheck` at end-of-run so users see fresh data immediately; `buildSpellCheck` joins `_char_owner.class+level ↔ spell:<char> ↔ _wiki_spells` filtered by `level<=char.level`, normalizes spell names via `normalizeSpellName` (toLowerCase + alphanumeric-only) as the join key, emits (Char, Class, Level, Spell, Status) rows with KNOWN/MISSING status sorted Char-then-Level-then-Spell.

## What shipped

### Task 1-2 — wiki-spell-types + wiki-spell-parser (commit `9bfc63e`)

`WikiSpellRow`: `{class, level, spell_name, normalized_name, last_refreshed}`. `SpellParseResult` discriminated union: `{ ok: true, rows, levelHeaders, spellCount }` or `{ ok: false, reason: 'wikitext_too_short' | 'page_error' }`.

`normalizeSpellName(s)`:
```typescript
s.trim().toLowerCase().replace(/^spell:\s*/i, '').replace(/[^a-z0-9]/g, '')
```
Captures P99 spellbook dump quirks where the in-game `/outputfile spellbook` sometimes prefixes spell names with `Spell: ` and sometimes doesn't.

`parseClassPage(wikitext, classDisplayName, classAbbrev)`:
1. Length guard: `wikitext.length < 200` → `wikitext_too_short`.
2. Split wikitext on `/^==\s*Level\s+(\d+)\s*==\s*$/m` capturing level numbers + section bodies.
3. For each `(level, sectionBody)`: find `{{SpellRow}}` template instances. Within each, extract `name=` parameter via `\|\s*name\s*=\s*([^\n|]+)` — matches **anywhere within** the template body (NOT first-position only — locked by RESEARCH §5 #4).
4. For each spell: trim, compute `normalizeSpellName`, emit row.
5. Return `{ ok: true, rows, levelHeaders: N, spellCount: M }`.

6 vitest scenarios against 3 fixtures + edge cases:
- **Necromancer**: levelHeaders=25; emits at least 100 rows; "Cavorting Bones" → "cavortingbones".
- **Paladin (hybrid)**: levelHeaders=17; spellCount=66.
- **Warrior (degenerate)**: levelHeaders=0; spellCount=0; ok=true (NOT a failure).
- Empty wikitext → `wikitext_too_short`.
- normalizeSpellName edge cases (Spell: prefix, apostrophes, capitalization).
- name-position robustness: synthetic SpellRow with `|type=...|name=...` ordering parses correctly.

### Task 3 — refreshWikiSpells trigger (commit `0cebef5`)

Cloned refreshWikiItems.ts shape. Adaptations:
- Item universe: `CLASSES.map(c => ({display: invertedMap[c], abbrev: c}))` (14 entries) instead of `collectInventoryItemRefs()`.
- Per-item handler: fetch class page, `parseClassPage(wikitext, classDisplayName, classAbbrev)`, then `replaceWikiSpellsForClass(classAbbrev, rows)` (bottom-up deleteRow scan on class column, then append).
- Headers: `['class', 'level', 'spell_name', 'normalized_name', 'last_refreshed']`.
- End-of-run: `writeMetaRow('_meta', 'last_wiki_spell_refresh', now)`; `writeMetaRow('_status', 'last_wiki_spell_count', totalRows)`; call `buildSpellCheck()`.
- Cursor key: `'wiki_spells_refresh_cursor'`. HANDLER_NAME: `'refreshWikiSpells'`.

6 vitest scenarios: happy path (3 classes populated); Warrior (0 spells, counted as success not failure); per-class fetch failure (404 counted, processing continues); failure-threshold abort (>50% of >=7 processed); resume cursor (mid-run timeout → cursor saved + second run resumes); buildSpellCheck called at end.

### Task 4 — buildSpellCheck builder (commit `7069f23`)

Cloned buildView shape with single-tab consolidation. Algorithm:

1. Acquire 30s lock + 10s debounce check via `PropertiesService.spell_check_last_build_ms`.
2. `readCharOwnerWithMetadata(ss)`: returns chars with class+level set (skip chars without metadata).
3. `readWikiSpellsByClass(ss)`: Map<class, WikiSpellRow[]>.
4. `readSpellbooksByChar(ss)`: for each `spell:*` tab, read col 2 (Name), normalize, collect into Set keyed by char name from `tabName.slice(6)`.
5. For each char with class+level: get wikiRows for char.class, filter `w.level <= c.level`, for each compute `status = known.has(w.normalized_name) ? 'KNOWN' : 'MISSING'`, emit row.
6. Sort Char asc, Level asc, Spell asc.
7. Clear data range + setValues + applyTheme + `writeMetaRow('_status', 'last_spell_check_build', now)`.
8. Update `spell_check_last_build_ms` in PropertiesService. Release lock.

7 vitest scenarios: 1 char NEC lvl 10 + 5 _wiki_spells NEC entries (3 ≤ lvl 10) + spell:Slampeach with 2 normalized matches → 2 KNOWN + 1 MISSING; char without metadata skipped; char without spell tab → all MISSING; multi-char sort; lock contention silent; debounce skips; applyTheme called.

### Task 5-6 — Wire onChange + Code.ts + build.mjs + STATE.md (commits `b6dacaf`, `80d59d8`)

`onChange.ts`: appended `buildSpellCheck()` call after `buildBank()`. Pattern continues full-rebuild-on-any-change with 10s debounce shared across builds (single `view_last_build_ms` property; future plans split if needed).

`Code.ts`: 2 new re-exports — `refreshWikiSpells`, `buildSpellCheck`. CI assertion catches.

`build.mjs` TRIGGER_GLOBALS: 2 new entries. assertExportsMatchGlobals from 04-01 passes.

## Deviations from Plan

Plan executed essentially as written. One important post-plan fix-pack landed during Phase 4 live-smoke (out-of-scope for this SUMMARY but documented in v1.0-ROADMAP.md):

**Fix-pack `9319c6b` (2026-05-10 21:09)** — wiki-spell-parser handles 3 template variants. The shipped parser handled only `{{SpellRow}}`; P1999 wiki uses `SpellRow / RadSpellRow / RadSpellRow2 / SongRow` (Bards use SongRow). Pre-fix: live smoke showed 5 caster classes covered. Post-fix: all 11 caster classes (1,562 spells in _wiki_spells). The fix retrofitted the parser regex without changing the public API.

## Schema impact

None — schema_version remains at 3. This plan POPULATES `_wiki_spells` (existing scaffold tab) + adds the new `spell_check` view tab. Both tabs already exist via Phase 2 scaffold. No new columns, no new rows, no migration.

## Verification log

```
$ npm test -- wiki-spell-parser
Tests       6 passed (6)

$ npm test -- refreshWikiSpells
Tests       6 passed (6)

$ npm test -- buildSpellCheck
Tests       7 passed (7)

$ npm run build
(exit 0 — 16 trigger globals; assertExportsMatchGlobals passes)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/lib/wiki-spell-types.ts`
- FOUND: `apps-script/src/lib/wiki-spell-parser.ts`
- FOUND: `apps-script/src/triggers/refreshWikiSpells.ts`
- FOUND: `apps-script/src/tabs/buildSpellCheck.ts`
- FOUND: `apps-script/src/__tests__/wiki-spell-parser.test.ts`
- FOUND: `apps-script/src/__tests__/refreshWikiSpells.test.ts`
- FOUND: `apps-script/src/__tests__/buildSpellCheck.test.ts`

**Commits exist:**
- FOUND: `9bfc63e` — feat(apps-script): wiki-spell-types + parser + tests against 3 fixtures
- FOUND: `0cebef5` — feat(apps-script): refreshWikiSpells trigger with resumable cursor + tests
- FOUND: `7069f23` — feat(apps-script): buildSpellCheck builder + tests
- FOUND: `b6dacaf` — chore(apps-script): wire spell_check into onChange + Code.ts + TRIGGER_GLOBALS
- FOUND: `80d59d8` — chore: STATE.md → plan 04-02 complete

## Next plan

`/gsd-execute-phase 4` spawned plan `04-03` — the **headline differentiator**: Velious gear-tier wiki scrape + `gear_check` consolidated tab. `refreshWikiGearTier` fetches `Players:Velious_Pre-Raid_Gear` and `Players:Velious_Raiding_Gear`, parses per-class `<li>` blocks with Iksar racial tagging (items starting with `'Iksar '` in the Pre-Raid page get tier='Iksar'); `buildGearCheck` joins `inv:* ↔ _wiki_gear_tier ↔ _char_owner.race+class` to emit OK/MISSING/OTHER status per (char, tier, slot, recommended-item) — the answer to "what does my character still need?"

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 04-02-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
