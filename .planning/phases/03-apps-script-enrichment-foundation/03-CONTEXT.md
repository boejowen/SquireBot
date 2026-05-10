# Phase 3: Apps Script Enrichment Foundation — Context

**Gathered:** 2026-05-09
**Status:** Ready for planning (research + patterns complete)
**Source:** Synthesized from ROADMAP.md Phase 3, REQUIREMENTS.md, .planning/research/* (ARCHITECTURE/STACK/SUMMARY/PITFALLS), Phase 1+2 SUMMARY logs, docs/design/eq-aesthetic-theme.md, and STATE.md decisions through v0.2.1.

> **CONTEXT DRIFT NOTE (2026-05-09 post-research):** Several assumptions in this doc were corrected by RESEARCH.md (real-API findings) and PATTERNS.md §1 (actual Phase 2 scaffold). Specifically: (a) view tab columns are `Char|Slot|Item|ID|Count|Wiki|Price|Last Synced` (per `internal/scaffold/scaffold.go` ViewTabs), NOT what §View Tab Column Set proposes; (b) `_pigparse` already has 6 cols not 4; (c) `_item_master` already has `is_quest_item` flag scaffolded; (d) `_meta` already has refresh-timestamp rows + `last_error`; (e) PigParse's `t` field is buy/sell direction not server-type, requiring 2 rows per item; (f) wiki uses `{{Itempage}}` not `{{Item}}` template; (g) `WatcherMaxSchemaVersion=1` constant in `internal/sheet/client.go:40` must be bumped to 2 in coordination with the schema migration; (h) clasp pinned to `^2.4.2`. **Read 03-RESEARCH.md and 03-PATTERNS.md §1 for the corrected truth — they override matching sections of this CONTEXT where they conflict.**

<domain>
## Phase Boundary

**In scope (per ROADMAP.md Phase 3):**
- TypeScript + clasp + esbuild scaffolding for the Apps Script side of the workbook
- Daily PigParse pricing scrape (`GET /api/item/getall/1`) → `_pigparse`
- Weekly P1999 wiki summary scrape (`api.php?action=parse&prop=wikitext`) per item that has appeared in any landing tab → `_item_master`, `_quest_items`
- `politeFetch(url)` wrapper: identifying User-Agent, `ETag`/`If-Modified-Since`, `CacheService`, exponential backoff on 429/503/504, 1s `Utilities.sleep` between wiki requests
- Resumable scrapes: cursor in `PropertiesService` + self-rescheduled trigger when 5-min wall-clock approaches
- Consolidated `view` tab build (leading `Char` column, one row per `inv:*` row across all chars) — onChange-driven
- `bank` tab build: shows the `_meta.bank_toon_name` character's inventory (no coin row in Phase 3 — Phase 4)
- Cell-note tooltips via `Range.setNote()` composing wiki summary + price + quest-item line
- `Last Synced` cell conditional formatting (green ≤7d, orange ≤30d, red >30d)
- `LockService.getDocumentLock().tryLock(30000)` in `try/finally` around every aggregate write that touches a shared range
- Row-count-assertion guards on each scrape (preserve last-known-good on truncated response, write failure to `_meta.last_error`)
- Courtesy contact emails to PigParse operator + P1999 wiki admins, sent and acknowledged BEFORE the daily/weekly triggers fire against live infra
- Theme registry skeleton (just enough to honor `_meta.theme` for the view/bank tabs Phase 3 produces — picker UI deferred per gray-area decision below)

**Out of scope (deferred):**
- `gear_check` and `spell_check` consolidated tabs (Phase 4 — depends on `_wiki_gear_tier` + `_wiki_spells` scrapes also in Phase 4)
- Wiki gear-tier and per-class spell scrapes (Phase 4)
- Manual coin sidebar / `bank_coin_*` cells / `Range.protect()` on coin row (Phase 4)
- HtmlService search sidebar (Phase 5)
- Custom `onOpen` menu / system-tab hiding / weekly schema healthcheck / archival of stale chars (Phase 5)
- Eviction workflow UI (Phase 5)
- README/onboarding polish + SmartScreen video (Phase 5)
- Discord pinger / wantlist (v2)

**Boundary clarification — theme picker:** The polished 6-tile aesthetic picker (`docs/design/eq-aesthetic-theme.md`) is a UX feature, not an enrichment foundation. Phase 3 lays the registry plumbing (`_meta.theme` read, view+bank tabs respect it on render); the picker UI itself ships in Phase 5 alongside the search sidebar (same HtmlService surface, same custom-menu entry point). Until Phase 5 the workbook owner can flip themes by editing `_meta.theme` directly. Default at Phase 3 ship = `minimalist` (matches eq-aesthetic-theme.md system default).

</domain>

<decisions>
## Implementation Decisions

### Stack (carried forward — locked)
- **Apps Script V8 runtime.** Never Rhino. (CLAUDE.md, STACK.md)
- **TypeScript via `clasp` v3.0+ + `esbuild` 0.20+ + `@types/google-apps-script`.** Source-of-truth lives in `apps-script/` (new top-level dir); `clasp push` is part of the deploy step.
- **HtmlService for any UI surface** (theme picker stub, future search sidebar). No web app, no add-on store distribution.
- **Cell notes via `Range.setNote()`** for tooltips. Never custom drawings, never user-cell sentinels.
- **`PropertiesService` (Document scope)** for scrape cursor + ETag cache + last-success timestamps.
- **`CacheService` (Document scope, 6h max TTL)** for hot wiki/PigParse responses inside a single trigger run.
- **`LockService.getDocumentLock().tryLock(30000)` in try/finally** around every aggregate write to a shared range.
- **`UrlFetchApp` only.** No `XmlService` / no `MediaWikiService` shims — wikitext parsing is hand-rolled regex against documented templates (Phase 4 will add gear-tier / spell-list parsers).

### Schema (carried forward — frozen at v1)
- All hidden `_`-prefixed dimension tabs already scaffolded by Phase 2 (`_meta`, `_char_owner`, `_item_master`, `_pigparse`, `_wiki_spells`, `_wiki_gear_tier`, `_quest_items`, `_audit`, `_status`).
- Phase 3 POPULATES (not creates): `_pigparse`, `_item_master`, `_quest_items`, `_status` (last_full_refresh, cell_count), `_meta.last_error`.
- Phase 3 does NOT touch: `_wiki_spells`, `_wiki_gear_tier` (Phase 4 owns these).
- Phase 3 reads-only: `_char_owner.owner_email`, `_meta.bank_toon_name`, `_meta.theme`, `_meta.schema_version`, `_meta.canonical_id`.
- **Extend-only.** If Phase 3 finds it needs a new column on a dimension tab it didn't anticipate, that's a `_meta.schema_version=2` event (idempotent migration + watcher's `WATCHER_MAX_SCHEMA_VERSION` check + script's `SCRIPT_MIN_SCHEMA_VERSION` check).
- **`_meta.theme` row added in Phase 3** (default `minimalist`). Watcher does not write this; Apps Script reads it on render. Pre-Phase-3 workbooks get the row written-if-missing on first Apps Script trigger fire.

### View Consolidation (carried forward — locked)
- `view` tab is ONE consolidated mega-tab with leading `Char` column. No per-character view tabs (would breach Google's 200-tab/workbook limit at guild scale).
- `bank` tab is ONE consolidated tab showing only the bank toon's inventory (named in `_meta.bank_toon_name`).
- Both tabs are visible to every guildie.

### View Tab Column Set (Claude's discretion — overridable)
Locked starting set, ordered left-to-right:
1. `Char` (string, from landing tab name)
2. `Location` (string, passthrough)
3. `Item` (string, passthrough — the user-visible name)
4. `ID` (integer, passthrough — joins `_pigparse` and `_item_master`)
5. `Count` (integer, passthrough)
6. `Slots` (integer, passthrough — bag capacity)
7. `Price (pp)` (number, joined from `_pigparse.price_pp` on `ID`, blank if no row)
8. `Wiki` (hyperlink, from `_item_master.wiki_url`, formula-driven `=HYPERLINK(...)`, blank if no row)
9. `Last Synced` (datetime, from landing tab's `_uploaded_at`, conditional-format colored)

The cell note (hover tooltip) attaches to the `Item` column (col 3), composing: wiki summary (≤200 chars) + current price (`Price: <N>pp at <Last Synced>`) + quest-item line if applicable (`Quest item: used in <quest1>, <quest2>...`).

`bank` tab uses the same column set MINUS `Char`.

### Build Trigger Model (Claude's discretion — overridable)
- **`onChange` simple trigger** wired to fire on `EDIT | INSERT_ROW | INSERT_COLUMN | OTHER` events on any sheet whose name matches `inv:*` or `spell:*`.
- Trigger handler immediately acquires `LockService.getDocumentLock().tryLock(30000)`. If lock can't be acquired (another build in flight), bail silently — the in-flight build will pick up the latest data anyway.
- Handler reads ALL `inv:*` tabs (not just the one that fired) and rebuilds `view` + `bank` from scratch. Full rebuild is correct because (a) consolidated tabs need to reflect cross-char joins, (b) full-snapshot writes are atomic, and (c) workbook-cap math (worst case 12 guildies × ~10 chars × ~150 inv rows ≈ 18,000 cells per rebuild) is well within the 6-min wall-clock budget.
- Per-trigger debounce via `PropertiesService.getDocumentProperties().getProperty('view_last_build')`: if a build completed in the last 10 seconds, skip (silently). Prevents storm rebuilds when watcher pushes 7 chars at once.
- **Time-driven backstop trigger every 1 hour** that runs the same builder. Catches missed `onChange` events (Apps Script simple triggers are best-effort, not guaranteed).
- The 30-second end-to-end success criterion (ROADMAP SC-1) is met by `onChange`'s typical sub-2-second fire latency + ~5–10s build time on realistic data.

### Scrape Triggers (per ROADMAP SC-2)
- **Daily PigParse trigger:** `ScriptApp.newTrigger('refreshPigparse').timeBased().atHour(3).everyDays(1).inTimezone('America/Los_Angeles').create()` → `~03:00 PT`. Single endpoint hit (`GET /api/item/getall/1`), full replace of `_pigparse`, with row-count assertion (refuse to overwrite if response < 90% of last-known row count).
- **Weekly wiki trigger:** `ScriptApp.newTrigger('refreshWikiItems').timeBased().onWeekDay(ScriptApp.WeekDay.SUNDAY).atHour(4).inTimezone('America/Los_Angeles').create()` → `~04:00 PT Sunday`. Iterates the union of item IDs seen across all `inv:*` tabs, fetches each via `politeFetch` with 1s sleep, populates `_item_master` + `_quest_items`.
- **Resumability (per ROADMAP SC-3):** `refreshWikiItems` checks `Utilities.getCpuTime()` (or wall-clock anchor at trigger start) every iteration; at 5 min elapsed it stores `{ cursor: <last_processed_id>, started: <iso> }` in `PropertiesService` and self-reschedules a one-shot trigger 1 minute out. Resumes from cursor, deletes its own one-shot trigger when complete.

### politeFetch Specification (per ROADMAP SC-4 — Claude's discretion on values)
```
politeFetch(url, opts) → { status, body, fromCache, etag }
```
- **User-Agent:** `SquireBot/<version> (+https://github.com/boejowen/SquireBot; <bank_toon_name@gmail.com or owner_email if not set>)`. Identifies the project, links to source, gives operators a contact path. The contact email is read from `_meta.contact_email` (Phase 3 adds this row; default = the workbook's `_char_owner` `owner_email` for the bank toon's row).
- **Caching:** `CacheService.getDocumentCache().get('etag:' + url)` → if present, send `If-None-Match: <etag>`. On `304 Not Modified`, return cached body. On `200`, store new etag + body (cap individual cache entry at 100KB; wikitext is bigger — store hash of last successful fetch instead, retrigger only on hash change).
- **Backoff on 429/503/504:** `2s, 4s, 8s, 16s, 32s` then surface failure (write to `_meta.last_error`, increment `_status.consecutive_scrape_failures`, exit). Honor `Retry-After` header when present (overrides our schedule for that one wait).
- **Inter-request sleep:** `Utilities.sleep(1000)` between every wiki request (NOT PigParse — that's a single batch endpoint, no need). Locked at 1000ms per ROADMAP SC-4 + general courtesy-scraping norms.
- **Timeout:** Apps Script `UrlFetchApp` has no per-request timeout knob; rely on Google's ~5-min process cap. If a single fetch is hanging that long something is very wrong — let the trigger time out.
- **No follow-redirects on POST.** PigParse + wiki are GET-only here; explicit `followRedirects: true` on GET, never POST.

### Row-Count Assertions (per ROADMAP SC-2)
- **PigParse:** if today's response row count < 90% of `_status.last_pigparse_row_count`, refuse to overwrite `_pigparse`, write `truncated_response: today=<N> last=<M>` to `_meta.last_error`, exit. Operator (workbook owner) sees the error in `_meta` and `_status` next time they open the sheet.
- **Wiki items:** per-page assertion. If a `parse` API response has no `wikitext` field or wikitext < 200 chars, treat as failed page (skip, don't blank existing `_item_master` row), increment per-trigger failure counter. If failure counter > 50% of items processed, abort the trigger and surface to `_meta.last_error`.
- **Last-known-good preservation:** The implication of both rules: a failed scrape NEVER blanks dimension data. The `view` tab keeps showing yesterday's prices and last week's wiki summaries until a successful refresh lands.

### Last Synced Conditional Format (per ROADMAP SC-5)
- Applied to col 9 (`Last Synced`) of `view` and the matching col of `bank`.
- Formulas: green if `=NOW()-A1<7`, orange if `=NOW()-A1<30`, red otherwise. Applied via `SpreadsheetApp.Range.setConditionalFormatRules()` in the build step (idempotent — clears existing rules on the column first).

### Lock Discipline (per ROADMAP SC-5)
- **Every** function that writes to a shared range wraps its body:
```typescript
const lock = LockService.getDocumentLock();
if (!lock.tryLock(30000)) { /* log + bail */ return; }
try { /* write */ } finally { lock.releaseLock(); }
```
- "Shared range" = `view`, `bank`, any `_*` tab, and `_meta.theme` (since it can be edited from picker UI in Phase 5).
- Per-character landing-tab writes (`inv:<Char>`, `spell:<Char>`) are watcher-side and use atomic batchUpdate — no Apps Script lock needed because Apps Script never writes there.

### Courtesy Contact (per ROADMAP SC-6) — USER-DEFERRED
- **User explicitly deferred courtesy-contact decision 2026-05-09.** They will decide whether/when/how to email the PigParse operator and P1999 wiki admins at a later time, not as part of Phase 3 execution.
- **Phase 3 implementation impact:**
  - The `politeFetch` wrapper still uses an identifying User-Agent (`SquireBot/<version> (+https://github.com/boejowen/SquireBot)`) — no contact email in UA until user opts in. Operators who want to reach us can find us via the GitHub URL.
  - Live triggers (daily PigParse, weekly wiki) are CREATED and ENABLED on first deploy — no acknowledgment gate.
  - Tests still use local fixtures (`runRefreshPigparseDryRun`, `runRefreshWikiItemsDryRun`) for CI; live triggers run against real infra in production.
  - This violates ROADMAP Phase 3 SC-6 as currently written. SC-6 is **WAIVED for Phase 3 ship**; the verdict-time eval should call this out as a documented waiver, not a regression. If the user later decides to send courtesy emails, that can land as a Phase 3.x patch (UA-string update + a CHANGES.md note).
- **Rationale for deferral (user's call):** request volume is genuinely tiny (1 PigParse GET/day + ~1500 wiki GETs/week with 1s spacing); operators of public APIs/wikis expect untrusted clients; the cost of a delayed launch on a 12-person guild tool exceeds the marginal politeness benefit. The user retains the option to send courtesy emails any time without code changes (the GitHub repo is the standing contact path).

### `_status.last_error` Model (Claude's discretion — overridable)
- Single row in `_status` (key `last_error`), value is a JSON-encoded object: `{ at: <iso8601>, where: 'refreshPigparse'|'refreshWikiItems'|'buildView'|'buildBank', kind: 'truncated_response'|'fetch_failed'|'lock_timeout'|'parse_error'|..., detail: <free string> }`.
- Cleared (set to `{}`) on every successful trigger run for that `where`.
- `_meta.last_error` is the persistent shadow — same shape, NOT cleared on success, only overwritten on next failure. Lets the operator see "the last thing that broke" even after a recovery succeeded.

### Apps Script Deployment Model (Claude's discretion — overridable)
- **One script project per workbook**, container-bound to the workbook (NOT a standalone library, NOT an add-on).
- The script's `clasp` config (`.clasp.json` containing `scriptId`) is per-workbook and lives OUTSIDE the repo (workbook-specific). The repo ships a `apps-script/.clasp.json.example`.
- Bootstrap flow: workbook owner runs `npm run setup` → wizard prompts for the workbook's script ID (or creates a new container-bound script via `clasp create --type sheets --parentId <sheet-id>`) → writes `apps-script/.clasp.json` → first `clasp push` lands the bundled JS.
- Reasoning: a shared library would couple every guild's deployment timing to the upstream version, and Apps Script's library-versioning UX is rough. Per-workbook = each guild updates on their own cadence by `git pull && clasp push`. The cost (each guild has to `clasp push` to get fixes) is mitigated by the rarity of Phase 3 changes after schema-lock.
- A future `Update Apps Script` tray menu in the watcher could automate this, but that's a Phase 5+ idea.

### Build Toolchain
- `apps-script/` directory at repo root.
- `apps-script/src/` for TypeScript sources, with subdirs by concern (`triggers/`, `tabs/`, `scrapes/`, `lib/`).
- `apps-script/dist/` for esbuild output (one bundled `Code.js` + per-html-file outputs for HtmlService).
- `apps-script/package.json` with `clasp`, `esbuild`, `@types/google-apps-script` as devDependencies.
- `apps-script/build.mjs` — esbuild script that bundles to a single `Code.js` (Apps Script V8 supports modern JS but not ES modules at runtime; esbuild's IIFE format works). Externalize globals injected by Apps Script.
- `apps-script/appsscript.json` — manifest with explicit OAuth scopes (`script.external_request`, `spreadsheets`, `script.scriptapp`, `script.container.ui`).
- CI: a `apps-script-build.yml` workflow that runs `npm ci && npm run build` and fails the PR if dist diverges from a clean rebuild. Does NOT auto-push to any workbook.

### Theme Registry (carried forward from docs/design/eq-aesthetic-theme.md)
- **Source of truth:** `_meta.theme` (string key from `{vanilla, kunark, velious, minimalist, heavy, sheets-default}`).
- **Theme registry** at `apps-script/src/lib/themes.ts` — exports `Theme` interface (header bg, header fg, row alt bg, font family, accent, etc.) + `THEMES: Record<ThemeKey, Theme | null>`. The `null` entry for `sheets-default` is the sentinel meaning "do not apply any styling — let Sheets render its native defaults".
- **`applyTheme(sheet, themeKey)`** helper called at the END of every view/bank rebuild. Reads theme from registry; calls `clearTheme(sheet)` first (resets fonts/colors/borders to Sheets defaults so theme switching is clean); applies styling unless theme is `sheets-default`.
- **`clearTheme(range)`** helper that explicitly resets `setFontFamily(null)`, `setBackground(null)`, `setBorder(false, false, false, false, false, false)`, etc. — the `null` arg is the documented Sheets way to revert to default. Without this, switching from Velious (icy blue) → Sheets default would leave residual blue.
- **Phase 3 implementation scope:** registry + `applyTheme` + `clearTheme` + view/bank rebuild calling `applyTheme` at end. NO picker UI yet — workbook owner edits `_meta.theme` cell directly to test (an edit fires `onChange`, which triggers a view+bank rebuild, which re-applies the theme).

### Out-of-Scope Reminders
- The watcher (Go side) does NOT change in Phase 3 except possibly to emit a heartbeat-marker row that triggers `onChange` if the workbook hasn't seen activity in 24h (see Specifics — flagged as TBD).
- No new OAuth scopes. `drive.file` only on the watcher side; the Apps Script side uses container-bound script auth (which gets the workbook's full scope automatically without a consent flow).
- No PII beyond `owner_email` (already in schema since Phase 1).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before researching, planning, or implementing.**

### Phase-local (to be created during research / planning)
- `.planning/phases/03-apps-script-enrichment-foundation/03-RESEARCH.md` — TBD by `/gsd-research-phase 3`. MUST cover: (a) actual JSON shape of `GET /api/item/getall/1` end-to-end with a real curl against PigParse (sample response committed as test fixture), (b) MediaWiki `api.php?action=parse&prop=wikitext` template shapes for per-item summary pages on P1999 (infobox layout, summary paragraph extraction, quest-item template name patterns).

### Project-wide
- `CLAUDE.md` — Project conventions and locked decisions. Phase 3 stack is documented under "Sheet side".
- `.planning/PROJECT.md` — Core value, constraints, key decisions
- `.planning/REQUIREMENTS.md` — REQ-IDs covered: ENRICH-01, ENRICH-02, ENRICH-05, ENRICH-06, ENRICH-07, ENRICH-08, ENRICH-09, VIEW-01, VIEW-02, VIEW-03, VIEW-04, TIP-01, TIP-02, TIP-03, OPS-02
- `.planning/ROADMAP.md` Phase 3 section — Six locked success criteria
- `.planning/STATE.md` — Phase 1 + 2 lessons (especially decision #10 on Google /token client_secret + decision #18 on EV cert reality)

### Phase 3 design + theme work
- `docs/design/eq-aesthetic-theme.md` — Locked aesthetic direction, 5 themes + Sheets default, theme registry pattern, picker delivery deferred to Phase 5
- `docs/design/mockups/eq-aesthetic-preview.html` — 6-tile preview reference
- `docs/design/mockups/eq-aesthetic-picker.html` — picker UI reference (deferred build, used Phase 5)

### Init research (Phase 0) — re-read with Phase 3 lens
- `.planning/research/STACK.md` — `## Sheet side` section is canonical for clasp/esbuild/HtmlService choices
- `.planning/research/ARCHITECTURE.md` — `## Sheet schema` section + dependency graph; `## Three-layer pancake` is the architectural foundation Phase 3 builds the middle+top layers of
- `.planning/research/PITFALLS.md` — Phase 3-relevant pitfalls: #5 (`Session.getActiveUser()` is script owner not writer), #7 (Apps Script 6-min wall-clock), #8 (workbook 200-tab cap), #11 (CacheService size limits), #16 (no library auto-update), #17 (`USER_ENTERED` vs `RAW`), #21 (LockService timeouts under storm), #24 (custom function executions vs trigger executions billed differently)
- `.planning/research/SUMMARY.md` — Phase 0 synthesis, "headline differentiator" framing for the consolidated views

### Phase 2 deliverables Phase 3 sits on top of
- `.planning/phases/02-watcher-robustness-schema-lock/02-CONTEXT.md` — schema-lock decisions, dimension-tab column scaffolds
- `.planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md` — sheet.Client mutex, the schema-scaffold table
- All 10 02-NN-SUMMARY.md plan summaries

### Test fixtures (Phase 3 will add)
- `apps-script/src/__fixtures__/pigparse-getall-1.json` — real curl response, snapshot for parser tests (research phase)
- `apps-script/src/__fixtures__/wiki-parse-<sample-item>.json` — real `api.php?action=parse` response for a known item (research phase)
- `apps-script/src/__fixtures__/wiki-parse-<sample-quest-item>.json` — real response for a quest-component item
- `apps-script/src/__fixtures__/inv-snapshot.tsv` — synthetic 12-char × 10-toon × 150-row inventory snapshot for view-build perf testing

</canonical_refs>

<specifics>
## Specific Ideas

- **Heartbeat-driven onChange:** the watcher already writes a daily heartbeat to `_char_owner.last_seen` (Phase 2). That write fires `onChange`, which means `view`/`bank` rebuild at least once a day even if no inventory changed. Free correctness backstop. Verify in Phase 3 that the heartbeat write doesn't trigger a rebuild storm if all 12 guildies' watchers happen to heartbeat at the same hour (the 10s debounce + lock should handle this).
- **`_meta.theme` write-on-missing:** First Apps Script trigger fire on a pre-Phase-3 workbook should `_meta.setRow('theme', 'minimalist')` if the row is absent. Idempotent.
- **Wiki item universe:** the union of distinct item IDs across all `inv:*` tabs. For ~12 guildies × ~10 chars × ~150 unique items per char, deduplicated, expect ~1500 unique IDs in steady state. At 1s sleep + ~1s API time = ~3000s wall-clock = ~50 min of trigger time. Resumable cursor handles this (5-min trigger × 10 cycles).
- **PigParse JSON snapshot:** during research phase, curl `GET https://pigparse.azurewebsites.net/api/item/getall/1` once, commit the response as a fixture, write the parser against the fixture. Don't trust Swagger doc shape vs actual response — Swagger is often stale.
- **MediaWiki summary extraction:** the `parse?prop=wikitext` response is wikitext, not HTML. Item summary is typically the first paragraph after the infobox template. P1999 wiki uses a `{{Item}}` infobox template (verify in research). Quest-item detection: scan for `{{Quest}}` or `{{QuestItem}}` template references or `[[Category:Quest Items]]` membership — research must confirm exact patterns.
- **Cell note size limit:** ~50KB per cell. Wiki summaries truncated to 200 chars + price line + quest line stays well under. Don't try to embed images.
- **Theme picker deferral cost:** if a Phase 3 user wants to test a non-default theme, they edit `_meta.theme` directly. Document this in the Phase 3 README addendum. Phase 5's picker UI just becomes a more polished way to do the same write.
- **Build perf budget:** worst-case rebuild = 12 chars × 10 toons × 150 rows × 9 cols ≈ 162,000 cell writes. `Range.setValues()` on a 18,000-row × 9-col 2D array is ~5 seconds. Target: <10s end-to-end for full rebuild. If it exceeds 30s, switch to incremental (only changed `inv:*` tab) — but full-rebuild is simpler and correct, so default to that until proven slow.
- **`_pigparse` shape:** assume `[{ id: number, name: string, price_pp: number }, ...]` based on Swagger; confirm in research. Single replace of all rows daily.
- **`_quest_items` join key:** item ID, NOT name. Many EQ items have name collisions (e.g., "Cloak"); item ID is the canonical identifier P99 wiki uses.
- **Deploy verification:** add a `verify-workbook.ts` CLI step to `apps-script/scripts/` that, after `clasp push`, calls a `runHealthcheck()` function via `clasp run` that confirms all expected functions are deployed and returns the deployed git SHA written to `_meta.deployed_apps_script_sha` during the push.

</specifics>

<deferred>
## Deferred Ideas

- **Theme picker UI** (the polished 6-tile dialog from `docs/design/mockups/eq-aesthetic-picker.html`) — Phase 5, alongside search sidebar.
- **`gear_check` and `spell_check` consolidated tabs** + the per-class spell scrape + Velious gear-tier scrape — Phase 4. Phase 3 only populates the data Phase 4 joins against (`_pigparse`, `_item_master`, `_quest_items`).
- **Manual coin sidebar + `bank_coin_*` cells + `Range.protect()`** — Phase 4.
- **HtmlService search sidebar** — Phase 5.
- **Custom `onOpen` menu** (`SquireBot → Search`, `SquireBot → Set Theme...`, etc.) — Phase 5.
- **System-tab hiding** (Phase 5 owns the `setHidden(true)` step on all `_*` tabs).
- **Weekly schema healthcheck Apps Script** (verify all expected tabs exist by ID, write any missing-tab errors to `_meta.last_error`) — Phase 5.
- **Auto-archive of stale chars** (`inventory_mtime > 90d` → hidden `_archive` tab) — Phase 5.
- **Eviction workflow** (`is_removed` UI) — Phase 5.
- **`Update Apps Script` tray menu** (watcher-side helper that runs `clasp push` against the bound workbook) — Phase 5+ if anyone asks.
- **Per-character `view:<Char>` tabs** — REJECTED FOREVER (200-tab limit).

</deferred>

---

*Phase: 03-apps-script-enrichment-foundation*
*Context gathered: 2026-05-09 via auto-mode synthesis (post-Phase-2-soak, post-v0.2.1 wizard fix)*
*Next step: `/gsd-research-phase 3` to fill 03-RESEARCH.md (PigParse JSON shape + MediaWiki template shapes), THEN `/gsd-plan-phase 3`*
