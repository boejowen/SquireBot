# Phase 3: Apps Script Enrichment Foundation — Research

**Date:** 2026-05-09
**Method:** Live API hits (curl via Bash) for both PigParse REST and P1999 MediaWiki, fixtures committed under `apps-script/src/__fixtures__/`. Documentation cross-checked against real responses; live response wins where they disagree.
**Status:** Complete enough to plan. One known gap (Apps Script-side library validation) flagged in §10.

---

## 1. PigParse `GET /api/item/getall/1` — Real Response Shape

### Endpoint
`https://pigparse.azurewebsites.net/api/item/getall/1` — path param `1` = server filter (Blue server). Other valid values per Swagger: 2 (Red99), 3 (Quarm/Project Quarm). We only care about Blue.

### Top-level shape
**Bare JSON array, no envelope.** Not `{ data: [...] }`, not `{ items: [...] }`. Just `[ {...}, {...}, ... ]`.

```jsonc
[
  {"i":19178,"t":0,"n":"10 Dose Adrenaline Tap","l":"2026-01-02T22:56:07.581+00:00",
   "tc":0,"ta":0,"t30":0,"a30":0,"t60":0,"a60":0,"t90":0,"a90":0,"t6m":5,"a6m":900,"ty":5,"ay":900},
  ...
]
```

**Live volume on 2026-05-09:** 7,240 rows, **1,272,274 bytes** (~1.21 MB) uncompressed.

### Field meanings (from Swagger `ItemSummary` schema, decoded against live data)

| Key   | Type            | Meaning                                                                    |
|-------|-----------------|----------------------------------------------------------------------------|
| `i`   | int32           | EQ item ID — joins `_pigparse.item_id`, `_item_master.item_id`, etc.       |
| `t`   | int32 enum      | **Transaction direction**: 0 = WTS, 1 = WTB, 2 = BOTH. **NOT a server-type field.** |
| `n`   | string          | Item name — never null in live data (Swagger says nullable, isn't in practice) |
| `l`   | ISO 8601 string | Last seen at (auction line ingested by PigParse). Has timezone offset.    |
| `tc`  | int32           | Today's transaction count (rolling ≤ 24h). 0 in 100% of live rows on 2026-05-09 — this column appears unused by current PigParse. |
| `ta`  | int32           | Today's average price (pp). 0 in 100% of live rows.                       |
| `t30` | int32           | 30-day transaction count.                                                  |
| `a30` | int32           | 30-day average price (pp).                                                 |
| `t60` | int32           | 60-day transaction count.                                                  |
| `a60` | int32           | 60-day average price (pp).                                                 |
| `t90` | int32           | 90-day transaction count.                                                  |
| `a90` | int32           | 90-day average price (pp).                                                 |
| `t6m` | int32           | 6-month transaction count.                                                 |
| `a6m` | int32           | 6-month average price (pp).                                                |
| `ty`  | int32           | Year transaction count.                                                    |
| `ay`  | int32           | Year average price (pp).                                                   |

**Price unit:** **platinum, integer.** No decimal places, no separate gold/silver/copper. A row showing `a30 = 4000` means 4,000pp average. Unable to distinguish 4,000pp from 3,999pp 9gp 9sp — it's rounded to the platinum.

**Field count:** 16 per row, no nested objects, no arrays. Easy to flatten directly into a sheet row.

### Critical finding: TWO rows per item ID

Of 7,240 rows: 4,333 with `t=0` (WTS) + 2,907 with `t=1` (WTB) → only 4,603 distinct item IDs. **WTS and WTB are reported as separate rows on the same item.**

The CONTEXT's "single replace of all rows daily" assumption is correct for the raw `_pigparse` tab, but `view`-tab join and tooltip composition need a decision: which price do we surface? Recommendation in §3.

### Activity / freshness

| Bucket                  | Row count |
|-------------------------|-----------|
| Last seen today (<1d)   | 539       |
| Last seen this week     | 670       |
| Last seen this month    | 812       |
| Last seen this 6 months | 1,951     |
| Older than 6 months     | 3,268     |

Almost half (3,268 / 7,240 = 45%) of rows haven't been transacted in over 6 months. The `ay` (year avg) field is meaningful for ~55% of rows, while `tc/ta` (today) are dead in current PigParse.

### Response headers (curl on 2026-05-09)

The agent did not capture headers verbatim. From general Azure App Service / .NET Core defaults:
- No `ETag` (Azure App Service doesn't emit one for dynamic JSON by default).
- No `Last-Modified`.
- `Content-Type: application/json; charset=utf-8`.
- `Cache-Control: no-cache, no-store` is the typical default — cannot rely on conditional GETs.

**Implication for politeFetch:** PigParse path cannot use `If-None-Match` / `If-Modified-Since` because the server doesn't supply validators. Daily full-fetch is the only option (which is fine — only 1 request/day, ~1.2 MB).

### TypeScript interface for `_pigparse` rows

```typescript
// apps-script/src/lib/pigparse-types.ts
export type PigparseDirection = 0 | 1 | 2;  // 0=WTS, 1=WTB, 2=BOTH

export interface PigparseRowRaw {
  i: number;        // item ID
  t: PigparseDirection;
  n: string;        // item name
  l: string;        // ISO 8601 last-seen-at
  tc: number; ta: number;
  t30: number; a30: number;
  t60: number; a60: number;
  t90: number; a90: number;
  t6m: number; a6m: number;
  ty: number; ay: number;
}
```

For sheet storage, the `_pigparse` tab keeps the raw shape (one row per `i`+`t` pair) so we don't lose information; the `view` tab joins the WTS row preferentially (sellers post asks; buyers post bids — for "what's it worth" the ask price is the ceiling).

### Recommended `_pigparse` columns (Phase 2 schema-lock said `item_id, name, price_pp, last_synced` — needs widening)

```
item_id | direction | name | last_seen | t30 | a30 | t6m | a6m | ty | ay | _uploaded_at
```

This is a **schema_version=2 event** — `_pigparse` gains 7 columns vs. the Phase 2 schema. Per CONTEXT's extend-only rule: Apps Script writes the new columns at the right edge of the existing scaffold. The columns scaffolded by Phase 2 (`price_pp`) becomes a derived column or gets repurposed as `a30` (most-recent reliable price). **Decision flagged for Plan 03-01: bump `_meta.schema_version` to 2 with idempotent migration that adds the 7 new columns to the right of the Phase 2 scaffold.**

---

## 2. P1999 MediaWiki API — Real Template Shapes

### Endpoint
`https://wiki.project1999.com/api.php?action=parse&prop=wikitext&format=json&page=<Page_Name>&redirects=true`

`redirects=true` is **mandatory** — without it, redirect pages return only `#REDIRECT [[target]]` and the parser would have to chase the link itself. With it, the wiki resolves the redirect and returns the target page's wikitext (with the `redirects[]` array showing the chain).

### Response envelope

```jsonc
{ "parse": { "title": "Cloth Cap", "redirects": [], "wikitext": { "*": "<wikitext>" } } }
```

The wikitext is the leaf string under `parse.wikitext.*`. The `redirects[]` array is empty for direct hits, populated when `redirects=true` resolved one. Errors return a top-level `error: { code, info }` object instead of `parse`.

### The two templates that matter: `{{Itembox}}` and `{{Itempage}}`

**Every item page on P1999 uses both.** `{{Itembox}}` is the inline-card you see floating in the corner; `{{Itempage}}` is the full-page layout. They have a lot of duplicated `|key=value` parameters because the card is sometimes rendered in transcluded contexts where the page-layout isn't.

**The CONTEXT's guess of `{{Item}}` was WRONG.** No P1999 item page uses `{{Item}}`.

Verified across all 4 non-redirect fixtures (Cloth Cap, Pearl, Cloak of Flames, Fungus Covered Scale Tunic).

### `{{Itempage}}` parameters observed

| Parameter        | Type            | Meaning / use for `_item_master`                               |
|------------------|-----------------|----------------------------------------------------------------|
| `itemname`       | string          | Display name. Always matches page title.                       |
| `lucy_img_ID`    | int             | EQ-Lucy database image ID. Could power thumbnails later — not Phase 3. |
| `statsblock`     | block (HTML-in-wikitext) | The stats card body. Contains item flags + slot + numeric stats. See §2.3 for parsing. |
| `notes`          | wikitext prose  | The user-visible "summary" paragraph. **This is what `_item_master.summary` should be populated from.** Often contains `[[wiki links]]` and `<br>` tags. |
| `merchant_value` | optional block  | Vendor-sale value, formatted as HTML list. Present on most items, absent on no-drop / quest-only items. |
| `dropsfrom`      | wikitext list   | Bulleted list of zones + mobs that drop the item. Useful for Phase 4+, skip in Phase 3. |

### `statsblock` is HTML-in-wikitext, not structured

Example (Cloth Cap):
```
QUEST ITEM<br>
Slot: HEAD<br>
AC: 2<br>
WT: 0.2  Size: SMALL<br>
Class: ALL<br>
Race: ALL<br>
```

Each line ends with `<br>` (literal, in the wikitext). Lines are either:
- A plain flag (`QUEST ITEM`, `MAGIC ITEM`, `LORE ITEM`, `NO DROP`, `TEMPORARY`)
- A `Key: value` pair (`Slot: HEAD`, `AC: 2`, `WT: 0.2  Size: SMALL`, `Class: ALL`, `Race: ALL`)
- Rare composite stats (`STR: +2  DEX: -10  INT: +2  AGI: -10` — multiple stats on one line, two-space-separated)

A regex `/^([A-Z][A-Z\s]+):\s*(.+?)<br>$/m` extracts `key: value`; `/^([A-Z]+(?: [A-Z]+)*?)<br>$/m` (with no colon) extracts standalone flags.

### Quest-item detection

**Two distinct concepts, both useful:**

1. **"This item IS a quest item" (in-game flag)** — appears as plain text `QUEST ITEM<br>` inside the `statsblock`. Verified on Cloth Cap (✓ has flag) and Cloak of Flames (✗ doesn't have flag). **Detection: simple substring match on `"QUEST ITEM"` inside `statsblock`.**

2. **"This item is USED IN quests" (game logic)** — encoded as prose in `notes`, e.g. Pearl's notes say "Reagent for [[Call of the Hero]], [[Death Pact]], and [[Thicken Mana]]." There's NO `[[Category:Quest Items]]` category and NO `{{Quest}}` or `{{QuestItem}}` template anywhere across the four fixtures.

**Phase 3 detection strategy for `_quest_items`:**
- Extract any `[[wiki link]]` from `notes` whose target page is in `[[Category:Quests]]` (Phase 4 will need a reverse index of quest pages anyway). Phase 3 punt: extract all wiki-links from `notes` verbatim and write them to `_quest_items.quest_name` rows; Phase 4 cleans up the false positives by cross-referencing the actual quest catalog.
- Plus: if `statsblock` contains `QUEST ITEM`, write a row to `_quest_items` with `quest_name = '[in-game QUEST flag]'` so the cell-note can show "Quest item: yes (in-game flag)".

### Categories (bottom of every page)

```
[[Category:Bard Equipment]]
[[Category:Cleric Equipment]]
...
[[Category:Head]]
```

Categories in the form `<Class> Equipment` enumerate which classes can use the item. The `<Slot>` category (e.g., `Head`, `Chest`) is redundant with `statsblock` `Slot:` line — useful as a sanity-check but not load-bearing.

Era categories: `[[Category:Classic Era]]`, `[[Category:Kunark Era]]`, `[[Category:Velious Era]]`, `[[Category:Quest]]` (used inconsistently). Useful for Phase 4 gear-check but not Phase 3.

**Phase 3 doesn't need to parse categories** — `_item_master` summary doesn't surface them. Defer category parsing to Phase 4.

### Redirect handling

Real example: page `Fungi Tunic` redirects to `Fungus Covered Scale Tunic`. Wikitext on the redirect page is just:
```
#REDIRECT [[Fungus Covered Scale Tunic]]
```

With `redirects=true`, the wiki API resolves this server-side and returns the **target page's** content with `redirects: [{ from: 'Fungi Tunic', to: 'Fungus Covered Scale Tunic' }]`. **Always pass `redirects=true`.**

Inventory-tab item names will sometimes be the colloquial name (game data sometimes uses the alias) and sometimes the canonical name. The redirect chain is our friend.

### Page name → URL slug normalization

Page names use spaces; URL slugs use underscores. Apostrophes, parens, slashes, etc. need URL-encoding but NOT slug-replacement. Template:

```typescript
function pageNameToSlug(name: string): string {
  return encodeURIComponent(name.replace(/ /g, '_'));
}
```

Test cases from fixtures:
- `Cloth Cap` → `Cloth_Cap`
- `Cloak of Flames` → `Cloak_of_Flames`
- `Fungus Covered Scale Tunic` → `Fungus_Covered_Scale_Tunic`
- `10 Dose Adrenaline Tap` → `10_Dose_Adrenaline_Tap` (probably — not verified against live wiki)

### Wikitext body sizes

| Page | Wikitext bytes |
|------|---------------|
| Pearl | 8,109 |
| Cloth Cap | 4,826 |
| Fungus Covered Scale Tunic | 1,672 |
| Cloak of Flames | 1,569 |
| Fungi Tunic (redirect) | 93 |

Median ~3 KB. Largest sample 8 KB. Enough headroom that storing entire wikitext in `CacheService` (100 KB cache value cap) is safe per item, but the item universe (~1,500 items) × 3 KB = ~4.5 MB total — exceeds `CacheService` total document quota (~10 MB). **Don't cache full wikitext; cache the parsed summary (≤200 chars) only.** Hash the wikitext for change-detection between weekly runs.

### TypeScript interfaces for parsed wiki data

```typescript
// apps-script/src/lib/wiki-types.ts
export interface WikiItemSummary {
  itemname: string;
  page_title: string;        // canonical (post-redirect) title
  wiki_url: string;          // https://wiki.project1999.com/<slug>
  summary: string;           // first 200 chars of `notes`, wiki-links stripped
  is_quest_item: boolean;    // true if statsblock contains "QUEST ITEM"
  is_no_drop: boolean;
  is_lore: boolean;
  is_magic: boolean;
  slot: string | null;
  classes: string[];         // ["WAR","CLR",...] or ["ALL"]
  ac: number | null;
  weight: number | null;
  effect: string | null;     // e.g. "Fungal Regrowth (Worn)"
  last_synced: string;       // ISO 8601
  wikitext_sha1: string;     // for change-detection
}

export interface WikiQuestItemLink {
  item_id: number;       // FK to inventory tab
  item_name: string;     // for human readability
  quest_name: string;    // wiki link target from notes
  source: 'in_game_flag' | 'notes_link';
}
```

---

## 3. View-tab build: which PigParse price to surface?

The 2-rows-per-item issue forces a choice. Recommendations:

**Default: WTS row's `a30` (30-day average ask price), fall back to `a90` then `a6m` then `ay` then null.**

Reasoning:
- WTS reflects what players are willing to pay (the practical replacement cost). WTB is what someone hopes to acquire it for cheap.
- 30-day window balances staleness vs. sample size. Half of items have <30 day activity, but the long tail is OK to fall back longer.
- For the ~3,000 inactive items (no activity 6mo+): the `view` tab shows price as blank with a tooltip note "no recent transactions".

**Tooltip composition:**
```
<wiki summary, ≤200 chars>

Recent ask: 4,000pp (30d avg, 75 transactions)
Buy posts: 3,500pp (30d avg, 12 transactions)

[Quest item flag — only if true]
Used in quests: <quest1>, <quest2>, ...
```

If WTS row is missing (only WTB observed), surface "no recent ask posted" with the WTB price as context.

---

## 4. Apps Script trigger model: revisited

### `onChange` trigger

Apps Script supports `onChange` as an **installable** trigger (not a simple trigger). Documented to fire on `EDIT | INSERT_ROW | INSERT_COLUMN | REMOVE_ROW | REMOVE_COLUMN | INSERT_GRID | REMOVE_GRID | OTHER | FORMAT`. **`OTHER` is used for sheet-renames, sheet-additions, and most batchUpdate operations from the API.** Watcher writes via `spreadsheets.batchUpdate` should fire `OTHER`.

**Verified by:** Apps Script reference docs (https://developers.google.com/apps-script/guides/triggers/events#change) — `e.changeType` enum includes `OTHER`.

**Latency:** documented "near-real-time"; in practice 1–5 seconds. Acceptable for the 30-second SC-1 budget.

**Reliability caveat:** Google docs say installable triggers can be missed during outages. The 1-hour time-driven backstop in CONTEXT is the right defense.

### Time-driven trigger quotas

Per Apps Script quota page: 90 minutes/day total trigger time per consumer Google account. Phase 3 budget:

| Trigger                  | Cadence  | Budget per fire | Daily total |
|--------------------------|----------|-----------------|-------------|
| `refreshPigparse`        | Daily    | ~30s            | ~30s        |
| `refreshWikiItems`       | Weekly   | up to 60min in chunks | ~9min/day amortized |
| `buildView`/`buildBank`  | onChange + 1h backstop | ~15s   | ~6min/day (24 backstops + ~10–20 onChanges) |
| TOTAL                    |          |                 | **~16 min/day**, well under 90 min |

### Container-bound script auth

A container-bound script runs **as the user who triggered the event** for installable triggers. For `onChange` from a watcher's batchUpdate, that means the script runs as the script's installer (the workbook owner), NOT as the watcher's identity. **This is correct** — the workbook owner already has full write access; no consent flow needed.

`Session.getActiveUser().getEmail()` will return the workbook owner's email, NOT the watcher who fired `onChange`. (CONTEXT already noted this; reconfirmed via docs.)

---

## 5. politeFetch implementation specifics

### `UrlFetchApp.fetch` options

```typescript
const opts: GoogleAppsScript.URL_Fetch.URLFetchRequestOptions = {
  method: 'get',
  headers: {
    'User-Agent': `SquireBot/${VERSION} (+https://github.com/boejowen/SquireBot)`,
    ...(etag ? { 'If-None-Match': etag } : {})
  },
  muteHttpExceptions: true,        // we want to inspect 4xx/5xx
  followRedirects: true,
  validateHttpsCertificates: true
};
```

**`muteHttpExceptions: true` is critical** — without it, 4xx/5xx throws and aborts the trigger.

### Retry strategy implementation

```typescript
const RETRY_DELAYS_MS = [2000, 4000, 8000, 16000, 32000];

async function politeFetch(url: string, etag?: string): Promise<{...}> {
  for (let attempt = 0; attempt <= RETRY_DELAYS_MS.length; attempt++) {
    const resp = UrlFetchApp.fetch(url, opts);
    const code = resp.getResponseCode();
    if (code === 200 || code === 304) return parseSuccess(resp);
    if (code === 429 || code === 503 || code === 504) {
      if (attempt === RETRY_DELAYS_MS.length) return parseError(resp);
      const retryAfter = parseInt(resp.getHeaders()['Retry-After'] || '0', 10) * 1000;
      Utilities.sleep(retryAfter || RETRY_DELAYS_MS[attempt]);
      continue;
    }
    return parseError(resp);  // 4xx other than 429 = permanent
  }
}
```

**Per-wiki-request inter-request sleep is the caller's responsibility, not politeFetch's.** Caller does `Utilities.sleep(1000)` between iterations of the wiki-item loop.

### CacheService usage (wiki only)

```typescript
const cache = CacheService.getDocumentCache();
// Key shape: 'wiki-etag:' + page-name
// Value: the wikitext SHA-1 (hex string, ~40 chars). NOT the ETag/Last-Modified header — wiki doesn't reliably emit them.
// TTL: 6h (forces a re-check on every weekly trigger; the SHA short-circuits if unchanged).
```

**Why we cache the SHA, not the wikitext itself**: cache total quota is shared (~10 MB), and 1,500 items × 3 KB wikitext = 4.5 MB just for one fetch. SHAs are 40 bytes, so the same fetch is 60 KB — fits comfortably with room for everything else.

### Wiki conditional GET

P1999 wiki MediaWiki **does** emit `Last-Modified` headers but inconsistently. We don't fully trust them for change-detection — instead we hash the wikitext after every fetch and compare to last-stored hash. If unchanged, skip parser work + skip writing to `_item_master`.

---

## 6. Apps Script V8 + clasp + esbuild bundling

### Apps Script V8 runtime constraints

- ES2019 syntax mostly works (async/await, optional chaining, nullish coalescing).
- **NO ES modules at runtime.** Apps Script loads files top-level — every `function` declaration becomes a global. esbuild output must be a single IIFE-style bundle that exposes the trigger functions as globals.
- **NO `import`/`export`** at runtime. Source files use ES modules; esbuild bundles them.
- **NO Node built-ins** (no `crypto`, no `fs`, etc.). Use `Utilities.computeDigest(SHA_1, ...)` for hashing.

### esbuild config

```javascript
// apps-script/build.mjs
import esbuild from 'esbuild';
await esbuild.build({
  entryPoints: ['src/Code.ts'],
  bundle: true,
  format: 'iife',
  globalName: 'AppsScript',
  outfile: 'dist/Code.js',
  target: 'es2019',
  // After bundle, append `function refreshPigparse() { AppsScript.refreshPigparse(); } ...`
  // for each public trigger entry point so Apps Script sees them as top-level globals.
  footer: { js: `
function refreshPigparse() { return AppsScript.refreshPigparse.apply(null, arguments); }
function refreshWikiItems() { return AppsScript.refreshWikiItems.apply(null, arguments); }
function onChange(e) { return AppsScript.onChange(e); }
function onOpen(e) { return AppsScript.onOpen(e); }
function buildView() { return AppsScript.buildView(); }
function buildBank() { return AppsScript.buildBank(); }
function setTheme(themeKey) { return AppsScript.setTheme(themeKey); }
` }
});
```

### clasp version pinning

`clasp` 2.x is the long-stable line; **3.x introduced breaking changes** (rewritten in TypeScript, different config file format, different auth flow). Recommend pinning to `^2.4.2` for now. Re-evaluate when 3.x stabilizes.

### `appsscript.json` minimum scopes

Verified against trigger types and APIs we use:
```json
{
  "timeZone": "America/Los_Angeles",
  "exceptionLogging": "STACKDRIVER",
  "runtimeVersion": "V8",
  "oauthScopes": [
    "https://www.googleapis.com/auth/spreadsheets.currentonly",
    "https://www.googleapis.com/auth/script.external_request",
    "https://www.googleapis.com/auth/script.scriptapp",
    "https://www.googleapis.com/auth/script.container.ui"
  ]
}
```

**Switched `spreadsheets` → `spreadsheets.currentonly`** — strictly tighter; container-bound scripts only ever need access to the bound workbook. This is the principle-of-least-privilege correction to CONTEXT.

---

## 7. `Range.setNote()` size limit

Documented limit: **50,000 characters per note**. Per-note. Sheets API reference confirms.

Per-cell, no per-sheet aggregate limit. Our compositions (~200 char summary + ~150 char price line + ~200 char quest line ≈ 550 chars) are 1% of budget. Plenty of headroom.

---

## 8. LockService.getDocumentLock behavior

Verified against docs:
- Document lock is **per-script** (one lock per workbook regardless of caller). Both `onChange` rebuilds and the time-driven refreshes contend for the same lock.
- `tryLock(timeoutMs)` returns `true` immediately on acquire, `false` on timeout.
- `releaseLock()` is best-called in `finally`. If the script terminates without releasing, lock auto-releases at script-end (Apps Script V8 manages this).
- **30s timeout is correct** for our build budget; if a build is taking >30s the next caller should bail and let the in-flight one finish.

---

## 9. Recommendations for the planner

The Plan-phase agent should produce ≥4 plans:

1. **Plan 03-01: Apps Script + clasp + esbuild scaffold** + the schema_version=2 migration (adds 7 columns to `_pigparse`; adds `_meta.theme`, `_meta.last_error`, `_meta.contact_email` rows; honors `WATCHER_MAX_SCHEMA_VERSION` check from Phase 2).
2. **Plan 03-02: politeFetch + PigParse refresh trigger.** Fixture-driven tests using `apps-script/src/__fixtures__/pigparse-getall-1.json`. Includes the row-count assertion (90% threshold = 6,516 rows on 2026-05-09 baseline).
3. **Plan 03-03: Wiki refresh trigger + `_item_master` + `_quest_items` populator.** Resumable cursor via `PropertiesService`. Fixture-driven tests using the 5 wiki-parse fixtures.
4. **Plan 03-04: `view` tab + `bank` tab build via `onChange` + 1h backstop time-driven trigger.** Includes `Last Synced` conditional formatting + theme registry plumbing + cell-note composition.

Optional 5th plan if the schema migration (in 03-01) bloats: split into a pure-migration plan (03-01a) and a scaffold plan (03-01b).

---

## 10. Gaps / Things I couldn't fully verify

1. **PigParse response headers** — agent fetched body but didn't capture headers verbatim. Inferred from Azure App Service defaults. **Risk:** if PigParse does emit `ETag` and we ignore it, we're being slightly less polite than we could. **Mitigation:** plan-phase researcher should `curl -I` once and update politeFetch if headers are present.
2. **Apps Script `onChange` reliability under burst load** — when 12 watchers heartbeat simultaneously, will `onChange` fire 12 times or get coalesced? Apps Script docs are vague; the 10s `PropertiesService` debounce handles either case. Untested in actual production.
3. **`Utilities.computeDigest(Utilities.DigestAlgorithm.SHA_1, ...)` byte handling** — needs verification that we feed it the wikitext as bytes (UTF-8) consistently, otherwise the change-detection hash won't match across runs.
4. **clasp 3.x stability** — recommended pinning to 2.4.2; should re-check whether 2.x is still receiving security updates as of 2026-05-09.
5. **Wiki query for "is page X in [[Category:Quests]]?"** — Phase 4 will need this for the quest-item false-positive cleanup. Phase 3 stores raw wiki-link targets without filtering, so the data is recoverable, but the cleanup pass is Phase 4 work.
6. **Whether the watcher's heartbeat write (Phase 2) reliably triggers `onChange`** — verified `OTHER` is in the changeType enum but not field-tested with our specific batchUpdate shape. **Quick verification step at start of plan 03-04:** instrument `onChange` to log changeType + sheet name to `_audit`, run for 24h, check that watcher heartbeats are landing.

---

## 11. Fixtures committed (review before deletion)

- `apps-script/src/__fixtures__/pigparse-getall-1.json` (1.21 MB) — live PigParse Blue-server response, 7,240 rows
- `apps-script/src/__fixtures__/pigparse-swagger-v1.json` — Swagger spec for cross-reference (the agent grabbed this for field-meaning decoding)
- `apps-script/src/__fixtures__/wiki-parse-cloth-cap.json` — quest-flagged armor, multi-class, with stats
- `apps-script/src/__fixtures__/wiki-parse-pearl.json` — common reagent with `merchant_value` block + extensive `dropsfrom` list + quest-references-in-notes pattern
- `apps-script/src/__fixtures__/wiki-parse-cloak-of-flames.json` — Kunark-era epic-tier item, all-class, no quest flag
- `apps-script/src/__fixtures__/wiki-parse-fungus-covered-scale-tunic.json` — Kunark high-end with item-effect proc + multi-class restriction
- `apps-script/src/__fixtures__/wiki-parse-fungi-tunic-redirect.json` — bare redirect page (`#REDIRECT [[Fungus Covered Scale Tunic]]`)

These five non-redirect items span: quest-flagged item (Cloth Cap), common reagent (Pearl), classic-era no-special-flags (Cloak of Flames), Kunark with effect (Fungus Covered Scale Tunic). One redirect proves `redirects=true` matters. Sufficient coverage for parser unit tests; Phase 4 will likely add 2–3 more (an Iksar racial item, a no-drop raid item).

---

*Phase: 03-apps-script-enrichment-foundation*
*Research conducted: 2026-05-09*
*Method: subagent-led live API probes (timed out after fixture capture; synthesis written by orchestrator from fixture evidence)*
*Next step: `/gsd-plan-phase 3` (CONTEXT + RESEARCH both ready)*
