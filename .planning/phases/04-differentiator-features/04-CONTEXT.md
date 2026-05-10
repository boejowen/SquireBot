# Phase 4: Differentiator Features — Context

**Gathered:** 2026-05-10
**Status:** Ready for research (research flag = needed)
**Source:** Synthesized from ROADMAP.md Phase 4, REQUIREMENTS.md (12 REQs in scope), .planning/research/* (FEATURES/PITFALLS/SUMMARY), Phase 3 code-complete state (apps-script/ stack), 03-CONTEXT.md + 03-PATTERNS.md (carry-forward decisions), Phase 3 smoke-verdict findings.

---

## Why this phase exists (one paragraph)

PROJECT.md's core value statement: *"Every guildie can answer 'what does my character still need, and where in the guild is it?' without leaving the spreadsheet."* Phase 1 delivered the inventory upload. Phases 2 + 3 hardened the watcher and built the foundation (PigParse pricing, wiki summaries, consolidated `view` + `bank` tabs). **Phase 4 is the answer to the question.** The `gear_check` tab is the per-character Velious shopping list. The `spell_check` tab is the per-character "spells your class can train at your level that you don't yet know" checklist. The `bank` coin sidebar makes the shared bank's plat balance trackable. Without these, SquireBot is a fancy data plumbing project; with them, it earns its keep.

<domain>
## Phase Boundary

**In scope (per ROADMAP Phase 4):**
- Per-class spell scrape (`refreshWikiSpells` weekly trigger) → populates `_wiki_spells`
- Velious gear-tier scrape (`refreshWikiGearTier` weekly trigger) → populates `_wiki_gear_tier` (3 tiers: `Velious Pre-Raid/Group`, `Velious Raiding`, `Iksar`)
- `gear_check` consolidated tab (rebuild via `buildGearCheck`): Status `OK | MISSING | OTHER` per (char, tier, slot)
- `spell_check` consolidated tab (rebuild via `buildSpellCheck`): Status `KNOWN | MISSING` per (char, level, spell)
- Sidebar form for char metadata: class + level + race per character (extends `_char_owner`)
- Sidebar form for bank coin entry: PP / GP / SP / CP (writes `_meta.bank_coin_*`)
- `Range.protect()` on `_meta.bank_coin_*` cells (sidebar is the only update path)
- `bank` tab gets a coin row (rendered from `_meta.bank_coin_*` at the top of the bank tab)
- Cell-count monitor (weekly) → `_status.cell_count`; alarm threshold 5M of 10M cap
- Schema migration to `schema_version=3`: adds `race` column to `_char_owner`
- Watcher constant bump: `WatcherMaxSchemaVersion` 2 → 3 (ship as v0.4.0)

**Out of scope (deferred):**
- Cross-character search sidebar (Phase 5)
- Custom-menu polish (theme picker tile UI, system-tab hide, weekly schema healthcheck) — Phase 5
- Auto-archive of stale chars (`inventory_mtime > 90d` → `_archive`) — Phase 5
- Eviction workflow (`is_removed` UI, 30-day grace) — Phase 5
- Discord pinger / wantlist — v2
- Race auto-detection from inventory data — punted in favor of explicit form entry
- Spell preferences ("don't show me spells I don't want") — v2 ergonomics
- gear_check showing items the char currently has but isn't ON the recommended tier (could be better/worse — we don't compare in v1)

**Boundary clarification — race tracking:** `_char_owner` schema bumps to v=3 with a `race` column, captured via the Phase 4 sidebar form. Without race, the `Iksar` racial tier in `gear_check` would either: (a) show for every char as noise, or (b) hide universally and miss the Iksar shopping list entirely. Both are bad. Schema bump is the cleanest fix; same migration pattern as Phase 3's v=2 bump. Cost is one column + one form field + one Go-side constant bump.

</domain>

<decisions>
## Implementation Decisions

### Stack (carried forward — locked from Phase 3)
- TypeScript + clasp (^2.4) + esbuild (^0.20) + vitest (^1.6).
- Apps Script V8 runtime; `appsscript.json` scopes already correct (Phase 3).
- HtmlService for sidebars (already used by Phase 3 theme picker stub).
- `politeFetch` (UA + retry + Retry-After + ETag/304) reused as-is.
- Resumable cursor pattern (5-min budget + 60s self-reschedule) reused from `refreshWikiItems`.
- LockService 30s tryLock around all aggregate writes.
- Per-workbook container-bound clasp deployment.
- Test mocks (`apps-script/src/__tests__/test-helpers.ts`) reused; will need extensions for sidebar form interactions (any `google.script.run` callback patterns).

### Schema (extend-only, bumping to v=3)

**`_char_owner` column add (right edge):** `race` (string, e.g. "IKS", "HUM", "BAR", "ERU", "ELF", "HIE", "DEF", "HEF", "DWF", "TRL", "OGR", "HFL", "GNM", "VAH"). 14 races per Velious-era P1999.

**Migration `migrateToV3()`:**
1. Read `_meta.schema_version`. If `3`, no-op. If not `2`, throw.
2. `appendColumns('_char_owner', ['race'])` — adds the column at right edge.
3. `writeMetaRow('_meta', 'schema_version', '3')` LAST.

Same idempotent pattern as `migrateToV2`. Same ship-watcher-first sequencing.

**Watcher-side constant:** `internal/sheet/client.go:44` `WatcherMaxSchemaVersion = 2 → 3`.

**`MetaRows` (Phase 2 scaffold):** no new rows needed in Phase 4 — `last_wiki_spell_refresh` and `last_wiki_gear_refresh` already scaffolded back in Phase 2.

### View tab schema (already locked by Phase 2 scaffold)

`gear_check` (per `internal/scaffold/scaffold.go` ViewTabs):
```
Char | Class | Tier | Slot | Have | Recommended | Status
```

`spell_check`:
```
Char | Class | Level | Spell | Status
```

These are the column names Phase 4 must populate. No deviation; if research surfaces a need (e.g., "Status needs to be split into Status + Detail"), bump schema_version=4 with a careful column add. Unlikely.

### gear_check Status enum — `OK | MISSING | OTHER`
- `OK`: char's inventory contains the recommended item for this (tier, slot, class) row.
- `MISSING`: char doesn't have the recommended item AND has nothing else equipped in that slot.
- `OTHER`: char has SOMETHING in that slot but it's not the recommended item.
Phase 4 does NOT compare item quality (a Wurmslayer in MAINHAND is technically `OTHER` even if it's better than the recommended Pre-Raid item). v2 idea: rank-aware comparison.

### spell_check Status enum — `KNOWN | MISSING`
- `KNOWN`: char's spellbook contains a spell whose normalized_name matches the (class, level) row.
- `MISSING`: char's spellbook does not contain a matching spell.

### Class/level/race capture — sidebar form, no auto-derive
- Auto-derive from spellbook is unreliable (low-level chars, hybrid-ish levels, classes that share spells like NEC/SHD).
- Sidebar form, opened from the SquireBot menu's new **"Set Character Info…"** item, lists all rows in `_char_owner` with editable class/level/race fields.
- Form validates: class is one of 14 P99 classes (`WAR, CLR, PAL, RNG, SHD, DRU, MNK, BRD, ROG, SHM, NEC, WIZ, MAG, ENC`); level is 1–60; race is one of 14.
- Save writes back to `_char_owner` (upsert by char_name).
- After save, fires `buildGearCheck` + `buildSpellCheck` (post-save callback via `google.script.run.withSuccessHandler`).
- Phase 5 can polish: auto-prompt when buildView detects a new char with empty class/level/race; explicit "needs setup" indicator on view rows.

### Bank coin sidebar — write-only via form, Range.protect() blocks raw edits
- New SquireBot menu item: **"Set Bank Coin…"**.
- Form has 4 number inputs: PP, GP, SP, CP.
- Pre-populates from current `_meta.bank_coin_*` row values.
- Save writes back via `writeMetaRow('_meta', 'bank_coin_pp', ...)` etc.
- After save, fires `buildBank` so the bank tab's coin row updates.
- `Range.protect()` applied to `_meta.bank_coin_pp/gp/sp/cp` cells in the migration plan: any direct edit attempt prompts "This range is protected, edits via SquireBot menu only".
- Permission model: protect with `editors.add(SpreadsheetApp.getActiveSpreadsheet().getOwner())` — script (running as workbook owner via container-bound auth) bypasses the protection.
- Any guildie with edit access to the workbook can use the form; the form runs the script which has full access.

### bank tab coin row rendering
- Phase 3 left bank as inventory-only.
- Phase 4 prepends a single header-styled row at row 2 (between header and inventory data) showing the 4 coin values:
  ```
  CHAR_NAME (bank toon) | COIN | Platinum: <pp> | Gold: <gp> | Silver: <sp> | Copper: <cp> |  | <last updated timestamp>
  ```
- This row's Slot column shows literal `COIN`. Other columns are repurposed for the coin display since users will scan the bank visually.
- Below row 2 is the regular inventory data per Phase 3's buildBank (sorted, with notes).
- Cleaner alternative considered but rejected: separate `bank_coin` tab (would need a new ViewTab in Phase 4 — schema risk; KISS wins).

### Per-class spell scrape (`refreshWikiSpells`)
- Page pattern: research-flag (likely `<Class>_spells` or `Spells:<Class>` per the wiki's MediaWiki namespace conventions; need to verify).
- 14 classes total. If single page per class: 14 fetches × ~2s each (1s sleep + 1s API) = 28s — comfortably under 5min budget.
- If per-level pages (`<Class>_spells_level_<N>`): 14 × ~50 levels = 700 fetches = 12min — needs resumable cursor.
- Decision: write the trigger with the resumable cursor pattern from `refreshWikiItems` either way. Safe overkill if it's actually fast.
- Output: `_wiki_spells` rows with `(class, level, spell_name, normalized_name, last_refreshed)`. The `normalized_name` is the join key for `spell_check`.

### Velious gear-tier scrape (`refreshWikiGearTier`)
- 3 specific pages per REQUIREMENTS:
  - `Players:Velious_Pre-Raid_Gear`
  - `Players:Velious_Raiding_Gear`
  - Iksar racial tier — page name TBD by research (likely `Players:Iksar_Gear` or similar)
- Each page has tables of `(slot, class) → recommended item`.
- Parser must extract those table rows.
- Wiki `parse?prop=wikitext` returns wikitext; tables in wikitext use `{| ... |}` syntax with `|-` row separators and `|` cell separators.
- This is HARDER than item parsing because table structure is variable. Research will produce the parser spec.
- Output: `_wiki_gear_tier` rows with `(tier, class, slot, item_id, item_name, rank, last_refreshed)`. `rank` is the position in the slot-list (1, 2, 3 for primary/secondary/tertiary recommendation per slot).
- `item_id` may be NULL when the wiki doesn't link to an item page with an ID. Acceptable; the `gear_check` join can fall back to name matching for items without IDs.

### gear_check join logic
For each character `c` with class `cls`, level `lv`, race `r`:
1. Determine which tiers to render: always `Velious Pre-Raid/Group` + `Velious Raiding`. Add `Iksar` IFF `r === 'IKS'`.
2. For each (tier, slot) where `_wiki_gear_tier` has a row matching `(tier, slot, class=cls)` (or `class='ALL'`):
   - Look up char's inventory rows in `inv:<c>` for the matching slot (Slot column).
   - If any inv row's item_id matches the recommended item_id (or item_name fallback): Status = `OK`, Have = inv item name.
   - If char has SOMETHING in that slot but not a match: Status = `OTHER`, Have = the actual item name.
   - If char has nothing in that slot: Status = `MISSING`, Have = blank.
3. Emit one row per (char, tier, slot, recommended-item) combination.

Edge case: a slot might have multiple recommendations (rank 1, 2, 3). Surface ALL of them — guildies will pick. So rows = (char × tier × slot × ranked_recommendation).

### spell_check join logic
For each character `c` with class `cls` and level `lv`:
1. From `_wiki_spells`, get all rows where `class = cls AND level <= lv`.
2. For each such row, check if `inv` no — wait, spells live in `spell:<c>` not `inv:<c>`.
3. From `spell:<c>`, get all spells (Level + Name).
4. Normalize each spellbook name and each `_wiki_spells.spell_name`.
5. Status = `KNOWN` if any normalized spellbook name matches; else `MISSING`.
6. Emit one row per (char, level, spell-from-wiki).

Normalization: `name.toLowerCase().replace(/[^a-z0-9]/g, '')`. Strips spaces, apostrophes, "Spell:" prefix if present, etc.

### Cell count monitor (`OPS-07`)
- New weekly trigger: `monitorCellCount` (separate from existing wiki-refresh triggers; can run in parallel since it doesn't write much).
- Algorithm: iterate all sheets; for each sheet, `lastRow * lastColumn`; sum. (Note: this counts the addressable range, not actual non-empty cells, but addressable range is what counts against Google's 10M cap.)
- Write total to `_status.cell_count`.
- If total > 5,000,000: write warning to `_meta.last_error` JSON `{at, where:'monitorCellCount', kind:'cell_count_threshold', detail:'<count>/10000000'}`.
- No automatic action beyond surfacing. Phase 5 owns archival.
- Cadence: weekly Sunday 03:30 PT (offset from refreshPigparse 03:00 + refreshWikiItems 04:00 to avoid lock contention).

### Trigger inventory (post-Phase-4)

`installTriggers()` updated to install **7 triggers** (was 4 in Phase 3):

1. `onChange` — sheet-bound.
2. `buildView` hourly backstop.
3. `refreshPigparse` daily 03:00 PT.
4. `refreshWikiItems` weekly Sunday 04:00 PT (Phase 3).
5. **`refreshWikiSpells` weekly Sunday 04:30 PT** (Phase 4 new).
6. **`refreshWikiGearTier` weekly Sunday 05:00 PT** (Phase 4 new).
7. **`monitorCellCount` weekly Sunday 03:30 PT** (Phase 4 new).

After `refreshWikiSpells` completes, it triggers `buildSpellCheck`. After `refreshWikiGearTier` completes, it triggers `buildGearCheck`. The `onChange` handler also triggers both builders when an `inv:*` or `spell:*` tab changes (for the catch-up case where wiki data exists but inventory just updated).

### onChange handler extension
Phase 3's `onChange` calls `buildView` + `buildBank`. Phase 4 extends to ALSO call `buildGearCheck` + `buildSpellCheck`. Same 10s debounce + LockService discipline. Each of the 4 builders independently locks/debounces; the onChange fan-out doesn't need its own coordination.

### Cross-side coordination (carry forward from Phase 3)
- Watcher v0.4.0 ships BEFORE the Apps Script migration runs. Same sequencing as Phase 3 → v0.3.0.
- `WatcherMaxSchemaVersion` bump in `internal/sheet/client.go` is task 1 of plan 04-01.
- `_char_owner` `race` column appended to `internal/scaffold/scaffold.go` `DimensionTabs[_char_owner].Headers` so fresh workbooks scaffold it at v=2 (and the v=3 migration writes-if-absent for existing v=2 workbooks).

### Courtesy contact — STILL WAIVED
Per user decision 2026-05-09. Phase 4's wiki traffic is small (~14 spell pages + 3 gear-tier pages weekly = 17 fetches/week = 68/month). User-Agent stays `SquireBot/<version> (+https://github.com/boejowen/SquireBot)`. No contact email until user opts in.

### Claude's discretion (overridable)
- Coin row rendering exact format (Slot column = `COIN`, etc.) — visual choice
- Sidebar form HTML styling — minimal Phase 5 polishes
- `gear_check` row sort order (currently char asc → tier asc → slot asc)
- `spell_check` row sort order (currently char asc → level asc → spell asc)
- Whether to include rank=1 only or all ranks of recommended gear (defaulting to all)
- monitorCellCount alarm formatting
- Race column abbreviation set (using P99-standard 3-letter codes from the in-game stats screen)

</decisions>

<canonical_refs>
## Canonical References

### Phase-local (TBD via research/planning)
- `.planning/phases/04-differentiator-features/04-RESEARCH.md` — TBD by `/gsd-research-phase 4`. MUST cover:
  - (a) Per-class spell wiki page format (single page per class? per-level pages?). Sample wikitext for at least 3 classes spanning archetypes (e.g., Necromancer pure caster, Paladin hybrid, Warrior melee).
  - (b) Velious Pre-Raid/Group page wikitext (table structure, slot/class encoding).
  - (c) Velious Raiding page wikitext (likely similar but separate confirmation needed).
  - (d) Iksar racial tier: locate the actual page; confirm naming.
  - (e) Slot vocabulary on the gear-tier pages: do they use `MAIN`/`PRIMARY`/`MAINHAND`? Match to the `Slot` values in inv tabs.
- `.planning/phases/04-differentiator-features/04-PATTERNS.md` — file-by-file analogs. Most code reuses Phase 3 patterns (politeFetch, resumable cursor, builder shape, lock+debounce); the new bits are: sidebar forms, table parser, Range.protect.

### Project-wide
- `CLAUDE.md` — project conventions (locked).
- `.planning/PROJECT.md` — core value, key decisions log.
- `.planning/REQUIREMENTS.md` — REQ-IDs Phase 4 covers: ENRICH-03, ENRICH-04, BANK-01..04, CHECK-01..05, OPS-07.
- `.planning/ROADMAP.md` Phase 4 section — 5 locked success criteria.
- `.planning/STATE.md` — Phase 1+2+3 ship history; Phase 4 entry point.

### Phase 3 deliverables Phase 4 sits on top of
- `apps-script/src/lib/politeFetch.ts` — reused as-is.
- `apps-script/src/lib/wiki-parser.ts` — different parsing concerns (table vs item template); Phase 4 adds `wiki-table-parser.ts` for the gear-tier pages, separate file.
- `apps-script/src/lib/sheet-helpers.ts` — `appendColumns`, `writeMetaRow`, `readMetaRows` reused for the v=3 migration.
- `apps-script/src/lib/themes.ts` — `applyTheme` called at end of buildGearCheck + buildSpellCheck.
- `apps-script/src/triggers/refreshWikiItems.ts` — template for refreshWikiSpells + refreshWikiGearTier.
- `apps-script/src/tabs/buildView.ts` — template for buildGearCheck + buildSpellCheck (same lock/debounce/applyTheme structure).
- `apps-script/src/tabs/composeNotes.ts` — extend composeItemNote pattern for cell-note tooltips on gear_check ("Why is this MISSING? Drops from <zone>; quest from <NPC>" — Phase 4 nice-to-have, not REQ).

### Init research (Phase 0) — Phase 4-relevant sections
- `.planning/research/FEATURES.md` — Spell + gear progression are P1 priority.
- `.planning/research/PITFALLS.md` — wiki scrape budget pitfalls (already mitigated by Phase 3 cursor pattern).
- `.planning/research/SUMMARY.md` — Phase 4 is the headline differentiator; parser regression is high-blast-radius (per init research).
- `.planning/research/ARCHITECTURE.md` — _wiki_spells + _wiki_gear_tier scaffolds.

### Phase 3 smoke verdict
- `docs/phase3-smoke-verdict.md` — 4 last-mile bug fixes. Of those, lessons relevant to Phase 4:
  - **TRIGGER_GLOBALS in build.mjs** must be updated for ALL new globals (refreshWikiSpells, refreshWikiGearTier, buildGearCheck, buildSpellCheck, monitorCellCount, showCharInfoSidebar, showBankCoinSidebar, etc.). Add a CI assertion in plan 04-01 to fail builds if exports diverge from globals.
  - **`readMetaRows` header agnostic** — already fixed in Phase 3; Phase 4 reuses.

### Test fixtures (Phase 4 will add to `apps-script/src/__fixtures__/`)
- `wiki-velious-preraid-gear.json` — full page wikitext (research phase, real curl).
- `wiki-velious-raiding-gear.json` — same.
- `wiki-iksar-racial-tier.json` — same.
- `wiki-class-spells-necromancer.json` — pure caster archetype.
- `wiki-class-spells-paladin.json` — hybrid archetype.
- `wiki-class-spells-warrior.json` — empty case (no spells beyond Bind Wound; the parser must handle the no-spells degenerate).
- (Optional) 1–2 more class spell fixtures spanning edge cases.

</canonical_refs>

<specifics>
## Specific Ideas

- **CI assertion for TRIGGER_GLOBALS divergence:** add a check that grep's `export {` from `Code.ts` and compares against `TRIGGER_GLOBALS` in `build.mjs`. Fails the build if they disagree. Lesson from Phase 3 bug `d0a2645`.
- **Class enum constant:** define `CLASSES = ['WAR', 'CLR', 'PAL', 'RNG', 'SHD', 'DRU', 'MNK', 'BRD', 'ROG', 'SHM', 'NEC', 'WIZ', 'MAG', 'ENC']` in `apps-script/src/lib/eq-constants.ts`. Same for `RACES`. Single source of truth.
- **Slot vocabulary normalization:** the inv tab's Slot column might say `MAIN1`/`MAINHAND`/`PRIMARY` depending on EQ version; the wiki gear-tier pages might use different vocab. Plan 04-04 should include a `normalizeSlot()` helper that maps both into a common vocabulary. Unit-tested with both wiki + inv samples.
- **Spell normalization function:** `normalizeSpellName(s) = s.toLowerCase().replace(/^spell:\s*/i, '').replace(/[^a-z0-9]/g, '')`. Test with edge cases: "Endure Cold", "spell: Burst of Flame", "Numb the Dead" (Necromancer Lv4).
- **gear_check + spell_check rebuild on weekly wiki refresh:** at the end of `refreshWikiSpells`, call `buildSpellCheck()`; same for `refreshWikiGearTier` → `buildGearCheck()`. So users see the new data without waiting for the next inv:* change.
- **Race detection from inventory ban:** I considered scanning inv for race-locked items (e.g., "Iksar-only" gear) but rejected it — too unreliable. Sidebar form is the source of truth.
- **Bank coin row formula:** the row could use spreadsheet formulas like `=INDIRECT("_meta!B" & MATCH("bank_coin_pp", _meta!A:A, 0))` to live-update from `_meta` without buildBank rebuild. Considered but rejected — the buildBank rebuild is fast and explicit; formulas in cells make the bank tab harder to debug.
- **Sidebar form pre-population:** for char-info form, fetch all `_char_owner` rows server-side via `google.script.run.withSuccessHandler(populateForm).readAllChars()`. Render one row per char with empty-or-current class/level/race fields.
- **Spell name normalization deduplication:** if two spells normalize to the same string (rare, but spell name collisions exist), `_wiki_spells` stores both rows; `spell_check` join just counts as KNOWN if either matches.

</specifics>

<deferred>
## Deferred Ideas

- **Cross-character search sidebar** — Phase 5 owns this. The HtmlService search bar that returns "who has Lustrous Russet Coat?" answers in <2s.
- **Custom-menu polish** — system-tab hide, weekly schema healthcheck, polished theme picker — Phase 5.
- **Auto-archive of stale chars** (`inventory_mtime > 90d` → hidden `_archive` tab) — Phase 5.
- **Eviction workflow** (`is_removed` UI, 30-day grace, automatic archive) — Phase 5.
- **gear_check rank-aware comparison** — knowing a Wurmslayer is better than the Pre-Raid recommended item. v2.
- **Race auto-detection from inventory data** — rejected for v1 (unreliable); revisit if user feedback demands it.
- **Discord pinger / wantlist** — v2.
- **Spell preferences** ("hide spells I don't want to learn") — v2.
- **Gear preferences** ("hide DRU items if I don't multi-class") — v2.
- **Coin entry permissions** — currently any guildie with edit access can update via the form; if abuse becomes an issue, Phase 5 can lock the form to a designated bank-toon-owner email.
- **`gear_check` cell notes** with drop info, quest source, current PigParse price for the recommended item — nice-to-have, not REQ.

</deferred>

---

*Phase: 04-differentiator-features*
*Context gathered: 2026-05-10 (post-Phase-3 ship + smoke PASS)*
*Next step: `/gsd-research-phase 4` — must capture wikitext fixtures for the 3 Velious gear-tier pages + sample per-class spell pages, then produce parser specs for both shapes.*
