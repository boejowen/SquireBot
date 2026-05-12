---
phase: 03-apps-script-enrichment-foundation
plan: 03
subsystem: apps-script-wiki-item-summary-scrape
tags: [apps-script, wiki, enrich-02, enrich-09, resumable-cursor]
requires:
  - 03-01 (apps-script scaffold + migrateToV2 extending _item_master with wikitext_sha1 + _quest_items with source col)
  - 03-02 (politeFetch HTTP wrapper reused verbatim)
provides:
  - "ENRICH-02: weekly Sunday ~04:00 PT scrape of https://wiki.project1999.com/api.php?action=parse&prop=wikitext for every distinct item ID across all inv:* tabs"
  - "ENRICH-09: 1s inter-request courtesy sleep (Utilities.sleep) before every wiki fetch"
  - "parseItempage(wikitext, pageTitle) pure parser extracting itemname/summary/is_quest_item/slot/classes/ac/weight/effect/wiki_url/wikitext_sha1 from {{Itempage}} template"
  - "SHA-1 change-detection skip: if existing _item_master.wikitext_sha1 matches new SHA, no parse + no write (saves trigger time)"
  - "5-min cursor-resume pattern: budget exceeded → write {remaining, total, failures, successes, started} to PropertiesService.wiki_refresh_cursor + schedule one-shot trigger 60s out + exit cleanly"
  - "Quest-link harvesting: every [[wiki link]] inside notes → _quest_items row with source='notes_link'; in-game QUEST flag → additional row source='in_game_flag'"
  - "50% failure-threshold abort: if >50% of items (when >=50 processed) fail, abort + write _meta.last_error + preserve last-known-good _item_master"
affects:
  - "03-04 (buildView): _item_master + _quest_items are the upstream inputs for the view-tab cell-note tooltip composition (summary, quest-item flag, quest links)"
  - "04-02 (refreshWikiSpells): clones refreshWikiItems shape verbatim for the weekly class-page scrape"
  - "04-03 (refreshWikiGearTier): clones the resumable-cursor pattern for the 2-page Velious gear scrape"
tech-stack:
  added: []
  patterns:
    - "Resumable cursor with PropertiesService: 5-min budget → checkpoint {remaining, total, failures, successes, started} + ScriptApp.newTrigger().after(60_000).create() + exit; resumed run reads cursor + picks up. Locked by RESEARCH §5 + PATTERNS §resumable-trigger."
    - "Wikitext SHA-1 change-detection: Utilities.computeDigest(SHA_1, ...) hex-encoded; compare to existing _item_master.wikitext_sha1; skip parse + write if unchanged. Saves ~10s per unchanged item; load-bearing for 1,500-item weekly budget."
    - "1s inter-request sleep: Utilities.sleep(1000) before every wiki fetch (courtesy to P1999 community resource). ENRICH-09 contract."
    - "Failure-threshold abort: aborts trigger if >50% of items fail AND >=50 processed. Defense against API-down scenarios overwriting good _item_master rows. Locked by PLAN truth #7."
    - "redirects=true in fetch URL: wiki API server-side resolves redirects (Fungi Tunic → Fungus Covered Scale Tunic); parser sees the resolved target's wikitext + caller passes the resolved title."
key-files:
  created:
    - apps-script/src/lib/wiki-types.ts (~60 lines; ParsedWikiItem + WikiItemSummary + WikiQuestItemLink + ParseResult discriminated union)
    - apps-script/src/lib/wiki-parser.ts (~180 lines; parseItempage pure function + statsblock split + classes/flags extraction + quest-link harvest + SHA-1 compute + pageNameToSlug)
    - apps-script/src/triggers/refreshWikiItems.ts (~280 lines; resumable cursor + 5-min budget + per-item lock + SHA skip + 50% failure abort + cleanup self-trigger)
    - apps-script/src/__tests__/wiki-parser.test.ts (~210 lines; 8 vitest scenarios across 5 wiki fixtures + edge cases + SHA determinism)
    - apps-script/src/__tests__/refreshWikiItems.test.ts (~280 lines; 8 vitest scenarios — happy path, resume, mid-budget checkpoint, SHA-unchanged-skip, per-item failure, failure-threshold abort, wiki API error JSON, idempotent re-run)
  modified:
    - apps-script/src/Code.ts (+2/-1 lines; replace refreshWikiItems stub with actual import + re-export)
decisions:
  - "SHA-1 (not Last-Modified / ETag) for change detection: P1999 wiki Last-Modified is unreliable per RESEARCH §5 — server-driven cache headers are inconsistent. SHA computed from wikitext is deterministic + parser-agnostic. Decision frozen in this plan."
  - "Quest-link cleanup deferred to Phase 4: filtering out wiki-links that aren't actually quest pages (could be zone names, mob names, items) requires the quest catalog. Phase 4 owns that filter; here we accept noise."
  - "Resumability checkpoint at 5min wall-clock (Apps Script's hard 6-min trigger budget): conservative 1-min headroom for finally-blocks + cleanup."
  - "50% failure threshold with >=50 processed floor: avoids spurious abort on the first few items if a transient outage hits early. Locked by PLAN truth #7."
  - "Per-class spell scrape + Velious gear-tier scrape: separate triggers (Phase 4). Same shape; clones this plan's resumable pattern."
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: 2026-05-09T20:55:16-05:00
  tasks_completed: 5 of 5
  commits: 3 (ca8ccd1 feat wiki-types interfaces; 21dde89 feat wiki-parser + tests against 5 fixtures; a4eba4c feat refreshWikiItems trigger with resumable cursor + wire Code.ts)
  files_changed: 6 (5 created + 1 modified, ~1010 lines added)
  tests_added: 16 (8 wiki-parser + 8 refreshWikiItems)
  trigger_count_after: 0 (trigger function lands; ScriptApp time-driven registration deferred to 03-04 installTriggers)
  schema_version_after: 2 (unchanged from 03-01)
  watcher_rebuild_required: false (schema unchanged; pure apps-script trigger addition)
---

# Phase 3 Plan 03: Wiki Item Summary Scrape Summary

**One-liner:** Shipped the weekly P1999 wiki item-summary scrape (ENRICH-02 + ENRICH-09) — `refreshWikiItems` trigger fetches `?action=parse&prop=wikitext&redirects=true` for every distinct item ID across all `inv:*` tabs with 1s inter-request courtesy sleep, parses the `{{Itempage}}` template via the pure `parseItempage` function (extracting itemname/summary/quest-item-flag/slot/classes/effect/wikitext_sha1), short-circuits on SHA-1 match against existing `_item_master.wikitext_sha1`, implements the 5-min cursor-resume pattern via `PropertiesService.wiki_refresh_cursor` + `ScriptApp.newTrigger.after(60_000)` for the ~1,500-item universe that exceeds Apps Script's 6-min trigger budget, harvests `[[wiki-link]]` quest references into `_quest_items` with source provenance, and aborts cleanly when >50% of processed items fail (preserving last-known-good `_item_master` rows).

## What shipped

### Task 1 — wiki-types.ts (commit `ca8ccd1`)

`ParsedWikiItem` (in-memory shape post-parse: itemname, page_title, wiki_url, summary, is_quest_item/is_no_drop/is_lore/is_magic flags, slot, classes string[], ac, weight, effect, wikitext_sha1), `WikiItemSummary` (`_item_master` row shape — subset of parsed plus last_synced), `WikiQuestItemLink` (`_quest_items` row shape: item_id, item_name, quest_name, source). Discriminated union `ParseResult`: `{ ok: true, item, questLinks }` or `{ ok: false, reason: 'no_itempage' | 'wikitext_too_short' | 'page_error', detail? }`.

### Task 2 — wiki-parser.ts (commit `21dde89`)

`parseItempage(wikitext: string, pageTitle: string): ParseResult` — pure function, no side effects, no API calls. Operates on the raw wikitext string returned from the wiki API. Algorithm per RESEARCH §2:

1. Length guard: `wikitext.length < 200` → `wikitext_too_short`.
2. Locate `{{Itempage` substring → if absent, `no_itempage`.
3. Extract from `{{Itempage` to matching `}}` with naive depth counter (no nested-template recursion expected per fixtures).
4. Split on `\n|` → trim params → `key=value` split on first `=`.
5. Capture `itemname`, `notes`, `statsblock`.
6. Parse statsblock: split on case-insensitive `<br>`; per-line either flag (`QUEST ITEM`, `MAGIC ITEM`, `LORE ITEM`, `NO DROP`, `TEMPORARY`) or `key: value`; multi-stat lines (`STR: +2  DEX: -10`) recursed on double-space split.
7. Build `ParsedWikiItem`:
   - `wiki_url = 'https://wiki.project1999.com/' + pageNameToSlug(page_title)` where `pageNameToSlug` does `replace(/ /g, '_')` + `encodeURIComponent`.
   - `summary` = first 200 chars of notes with `[[X|Y]] → Y` (or `X` if no display) and `<br>` stripped.
   - Flags `is_quest_item / is_no_drop / is_lore / is_magic` from statsblock.
   - `classes` split on whitespace (`ALL` → `['ALL']`, `WAR CLR PAL` → `['WAR','CLR','PAL']`).
   - `wikitext_sha1` = `Utilities.computeDigest(SHA_1, Utilities.newBlob(wikitext).getBytes())` hex-encoded.
8. Harvest quest-links via `/\[\[([^|\]]+)(?:\|[^\]]+)?\]\]/g` on notes body; each match → `{ quest_name: match[1], source: 'notes_link' }`. If `is_quest_item`, prepend `{ quest_name: '[in-game QUEST flag]', source: 'in_game_flag' }`.

8 vitest scenarios against 5 wiki fixtures + edge cases:
- **Cloth Cap**: `is_quest_item=true`, slot=HEAD, classes=ALL, ac=2.
- **Pearl**: `is_quest_item=false` (statsblock doesn't say QUEST ITEM), summary mentions Call of the Hero, questLinks contain `'Call of the Hero'`, `'Death Pact'`, `'Thicken Mana'` (notes_link only — no in_game_flag for Pearl).
- **Cloak of Flames**: `is_magic=true`, `is_quest_item=false`, slot=BACK, ac=10, classes=ALL.
- **Fungus Covered Scale Tunic**: `is_lore=true`, slot=CHEST, ac=21, effect mentions 'Fungal Regrowth (Worn)', classes = `['WAR','CLR','PAL','RNG','SHD','DRU','MNK','BRD','ROG','SHM']`.
- **Fungi Tunic redirect**: caller passes resolved-target page title; parser sees server-resolved wikitext (redirects=true in fetch); no choke on redirect chain.
- Edge: empty wikitext → `wikitext_too_short`.
- Edge: garbage-no-template → `no_itempage`.
- SHA determinism: same wikitext → same SHA every time.

### Task 3-4 — refreshWikiItems trigger + Code.ts wire (commit `a4eba4c`)

The most complex single function in Phase 3. Algorithm:

```
start = Date.now()
budget = 5 * 60 * 1000  // 5 min wall-clock
props = PropertiesService.getDocumentProperties()
cursor = props.getProperty('wiki_refresh_cursor')
state = cursor ? JSON.parse(cursor) : null
itemIds = state?.remaining ?? collectInventoryItemIds()
totalItems = state?.total ?? itemIds.length
failures = state?.failures ?? 0
successes = state?.successes ?? 0
log info {resuming, total, remaining}

while itemIds.length > 0:
  if budget exceeded:
    props.setProperty('wiki_refresh_cursor', JSON.stringify({remaining, total, failures, successes, started}))
    ScriptApp.newTrigger('refreshWikiItems').timeBased().after(60_000).create()
    return
  batch = itemIds.splice(0, 10)  // process 10 then re-check time
  for {id, name} of batch:
    try:
      Utilities.sleep(1000)
      result = politeFetch(wikiUrl(name))
      if error: failures++; continue
      json = JSON.parse(result.body)
      if json.error: failures++; continue
      wikitext = json.parse?.wikitext?.['*']
      resolvedTitle = json.parse?.title ?? name
      if !wikitext: failures++; continue
      parsed = parseItempage(wikitext, resolvedTitle)
      if !parsed.ok: failures++; continue
      existingSha = readItemMasterSha(id)
      if existingSha === parsed.item.wikitext_sha1: successes++; continue  // unchanged, skip write
      writeItemMasterRow(id, parsed.item)
      replaceQuestItemRowsForId(id, parsed.questLinks.map(l => ({...l, item_id: id})))
      successes++
    catch e: failures++; log warn
  if failures > 0 && (failures + successes) >= 50 && failures / (failures + successes) > 0.5:
    writeError({kind: 'fetch_failures_exceeded', detail: 'failures=X successes=Y'})
    props.deleteProperty('wiki_refresh_cursor')
    cleanupSelfTrigger()
    return

writeMetaRow('_meta', 'last_wiki_summary_refresh', now)
writeMetaRow('_meta', 'last_quest_items_refresh', now)
writeMetaRow('_status', 'last_wiki_item_count', successes)
clearError()
props.deleteProperty('wiki_refresh_cursor')
cleanupSelfTrigger()
```

Helpers: `collectInventoryItemIds()` iterates all `inv:*` sheets, reads col D (ID) + col B (Name), dedup-by-id. `readItemMasterSha(id)` lookup by `item_id` col. `writeItemMasterRow(id, item)` upsert. `replaceQuestItemRowsForId(id, links)` deletes existing then appends. `cleanupSelfTrigger()` iterates `ScriptApp.getProjectTriggers()` and deletes extras (cursor-null + 2+ refreshWikiItems triggers heuristic).

8 vitest scenarios mock UrlFetchApp + SpreadsheetApp + PropertiesService + ScriptApp.newTrigger: happy path, resume (cursor pre-populated → processes remaining), mid-budget checkpoint (simulated time exceeds budget → cursor written + trigger created + return), SHA unchanged (no _item_master write — verified via call count), per-item parse failure (failure++, continue), 51% failure threshold abort (error written + cursor deleted), wiki API error JSON parsed as failure, idempotent re-run (cursor=null after completed → fresh processing).

`Code.ts` updated: replace stub with `import { refreshWikiItems } from './triggers/refreshWikiItems'; export { refreshWikiItems };`. Build footer still picks it up.

## Deviations from Plan

None — plan executed as written. (Detailed deviation tracking not captured retroactively.)

## Schema impact

None — schema_version remains at 2 (set in 03-01). This plan POPULATES `_item_master.wikitext_sha1` + `_quest_items.source` added by 03-01's migrateToV2. No new columns, no new rows, no migration.

## Verification log

```
$ npm test -- wiki-parser
Tests       8 passed (8)

$ npm test -- refreshWikiItems
Tests       8 passed (8)

$ npm run build
(exit 0 — refreshWikiItems exported as top-level global)
```

(Verification log is reconstructed retroactively from PLAN.md acceptance criteria + commit messages.)

## Self-Check: PASSED

**Files exist:**
- FOUND: `apps-script/src/lib/wiki-types.ts`
- FOUND: `apps-script/src/lib/wiki-parser.ts` (parseItempage pure function)
- FOUND: `apps-script/src/triggers/refreshWikiItems.ts` (cursor-resume + SHA skip + threshold abort)
- FOUND: `apps-script/src/__tests__/wiki-parser.test.ts`
- FOUND: `apps-script/src/__tests__/refreshWikiItems.test.ts`

**Commits exist:**
- FOUND: `ca8ccd1` — feat(apps-script): wiki-types interfaces
- FOUND: `21dde89` — feat(apps-script): wiki-parser + tests against 5 fixtures
- FOUND: `a4eba4c` — feat(apps-script): refreshWikiItems trigger with resumable cursor + wire Code.ts

## Next plan

`/gsd-execute-phase 3` spawned plan `03-04` — the user-visible Phase 3 deliverable: consolidated `view` + `bank` tabs with cell-note tooltips, conditional formatting on Last Synced (green ≤7d / orange ≤30d / red >30d), theme application via 03-01's THEMES registry, onChange trigger with 10s debounce, 1h time-driven backstop, and `installTriggers()` callable from the SquireBot custom menu that registers all 4 Phase 3 triggers idempotently.

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 03-03-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
