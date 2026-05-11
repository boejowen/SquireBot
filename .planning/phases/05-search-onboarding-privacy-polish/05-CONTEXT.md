# Phase 5: Search + Onboarding + Privacy Polish — Context

**Gathered:** 2026-05-11
**Status:** Ready for research (research flag = NOT needed per ROADMAP §96; planner may still request a research pass for the search-budget perf model and the GitHub Pages Jekyll setup)

---

## Why this phase exists (one paragraph)

Phase 5 is the **last phase before milestone v1.0 ships**. Phase 4 made the workbook the single source of truth for "what does my character still need." Phase 5 makes it the single source of truth for "**where in the guild is it?**" — the core-value question's other half — via a cross-character item search sidebar. It also turns the install experience from "scary unsigned binary on Windows" into a routine guildie does once and forgets, so all 12 guildies end up actually running SquireBot. And it covers the housekeeping (system-tab hide, weekly schema healthcheck, eviction workflow) that keeps a multi-user workbook safe for the long haul. Without Phase 5, SquireBot is a polished tool that 2–3 nerds use; with Phase 5, it's the guild's daily workbench.

<domain>
## Phase Boundary

**In scope (per ROADMAP §84-96 + REQUIREMENTS.md):**

- **Cross-character search sidebar** (SEARCH-01..04, partially TIP-04):
  - HtmlService sidebar ~300 px wide, opened from `onOpen` SquireBot menu
  - Free-text query box + Char dropdown filter + Slot dropdown filter
  - Substring match across every `inv:*` tab; <2 s budget
  - Per-`inv:Char` cache in CacheService (60 s TTL, invalidate on watcher write)
  - Two-line stacked result rows, grouped by item name, char asc within group
  - Auto-collapse groups when >5 chars match a single item
  - "No matches" → did-you-mean fuzzy fallback
  - "Recent:" footer with last 3 queries (clickable re-run)
  - Wiki-link + price (when present from `_pigparse`) inline; full wiki summary on hover
- **Privacy / housekeeping polish** (VIEW-05, OPS-06):
  - Auto-hide all `_*`-prefixed dimension/system tabs (idempotent on every install)
  - Weekly Apps Script healthcheck verifies expected tabs exist by ID; missing-tab errors → `_meta.last_error`
  - Stale-char auto-archive: `inventory_mtime > 90d` → hidden `_archive` tab
  - `Range.protect()` (warning-only) on `_meta.bank_toon_name` (Phase 4 already covered `_meta.bank_coin_*`)
- **Eviction workflow** (DOC-02 + supporting code):
  - Owner-only sidebar UX (defaulted — see Claude's Discretion)
  - Officer marks departed guildie → cascades `is_removed=TRUE` across that `owner_email`'s chars → 30-day grace → auto-archive
  - Documented end-to-end runbook
- **Onboarding documentation as a GitHub Pages site** (DOC-01, DOC-03):
  - Jekyll-default Pages site at `boejowen.github.io/SquireBot`
  - Source under `/docs` in the repo (Jekyll picks it up automatically)
  - Install page: prereqs → installer download → SmartScreen walkthrough → OAuth → EQ folder picker → "what to expect"
  - Separate `/troubleshooting` page (linked from install): tray-red causes, ErrSchemaTooNew, drive.file propagation, log-folder location
  - PNG screenshots + a single annotated GIF (no audio) of the SmartScreen walkthrough, all in `/docs/assets/`
  - README.md becomes a short pointer to the Pages site (not the long doc itself)
- **Distribution success criterion** (ROADMAP §93): "all 12 guildies are installed and writing data" — this is an outcome, not a code task. Track via `_char_owner` `owner_email` distinct count + heartbeat freshness; phase doesn't ship until 12 distinct emails have written within the last 7 days.

**Out of scope (deferred to v2):**

- Discord pinger / wantlist (per PROJECT.md / REQUIREMENTS.md v2 list)
- Race auto-detection from inventory data (Phase 4 punt; sidebar form is the v1 path)
- Polished theme picker tile UI (Phase 4 deferred backlog — keep deferred unless trivial after Phase 5 lands)
- Bank-coin permission lock (only bank-toon-owner can use sidebar) — Phase 4 deferred backlog; skipped here per area-selection cull
- Sidebar HTML inline-JS unit tests — Phase 4 deferred backlog; integration-test via smoke
- Installer-driven upgrade UX (NSIS can't overwrite running .exe) — fold into Phase 5 ONLY if it bites the 12-guildie distribution. Otherwise: document the workaround in `/troubleshooting` and defer the structural fix.
- Inline staleness on search results (SEARCH-03 as originally written) — see Scope Changes section below

**Explicitly NOT a Phase 5 ambiguity (defaulted by Claude per area-selection):**

- Eviction workflow UX shape: defaulted to **owner-only sidebar** (officer marks the departed email; cascade flips `is_removed=TRUE` on that owner_email's chars; PropertiesService timer fires the archive after 30 days; un-evict is a manual `is_removed=FALSE` cell edit by the owner). Rationale: ROADMAP §92 explicitly sequences "remove their email from workbook share → mark all their characters `is_removed`" — that's an owner action; self-service evict adds threat-model complexity (departing guildie could nuke data for spite) that v1 doesn't need.

</domain>

<decisions>
## Implementation Decisions

### Search query model

- **D-01: Input shape — free-text + minimal filters.** Sidebar has a text input PLUS a Char dropdown (any | every char in `_char_owner`) PLUS a Slot dropdown (any | HEAD | CHEST | EAR1 | EAR2 | ARMS | WRIST1 | WRIST2 | LEGS | FEET | HANDS | NECK | FINGER1 | FINGER2 | SHOULDERS | BACK | WAIST | RANGE | AMMO | PRIMARY | SECONDARY | inventory slots, etc. — exact list TBD by planner from inventory data). No inline syntax (`char:foo`); structure lives in UI controls only.
- **D-02: Match semantics — case-insensitive substring.** `russet` matches anywhere within an item Name. Single-pass scan, predictable, fast. EQ items have long compound names so substring is the natural fit; word-boundary boost was rejected as not worth the test surface.
- **D-03: Cache shape — per-`inv:Char`.** Cache key per inv tab (`squirebot:search:inv:Findom` → JSON row array). 12 cache entries × ~75 KB each fits CacheService's 100 KB-per-value cap. Invalidation: each watcher write to `inv:<Name>` MUST evict that cache key. First search of the day takes ~3 s cold (12 reads); subsequent searches (any query) ~200 ms. Match against the 2-second budget: planner should add a cold-warm-up path (e.g., `installTriggers` warms the cache once after cold start) if the cold path can't hit budget.
- **D-04: Empty-state UX — "No matches" + did-you-mean fuzzy fallback.** Primary search uses fast substring (D-02). When substring returns zero rows, run a Levenshtein second pass (edit distance ≤2) against item names and surface up to 3 suggestions: *"No matches for 'clok'. Did you mean: Cloak of Confusion, Cloak of Flames?"* Fuzzy adds ~1–2 s; acceptable on the no-match branch since the user already got nothing.

### Search results layout

- **D-05: Row shape — two-line stacked.** Line 1: `<ItemName> · <Char>: <Location>, count <N>`. Line 2 (smaller, muted): wiki link + PigParse price (when present). Tooltip on the item name shows the full wiki summary from `_item_master`. Always-shown enrichment, no expand/collapse on the row itself (collapse is at the GROUP level — see D-07).
- **D-06: Sort order — group by item name, char asc within group.** All matches for the same item cluster under a group header (the item name itself — printed once). Within each group, rows sorted by char name alphabetically.
- **D-07: High-cardinality handling — auto-collapse when >5 chars match.** When a single item is held by more than 5 chars, that group renders collapsed: `Bone Chips · 9 chars [expand]`. Other groups render expanded normally. Click the group header to expand/collapse. Prevents single-letter typos like `bone` from blowing up the DOM.
- **D-08: Lifecycle — open empty, auto-focus input, "Recent:" footer.** Sidebar opens empty; search input auto-focused; Enter submits. Footer shows "Recent:" with the last 3 queries as clickable re-run links. History persisted in CacheService keyed per workbook (key: `squirebot:search:recent`); rolling window of 3.

### Onboarding docs + assets

- **D-09: Doc platform — GitHub Pages with Jekyll.** Site at `boejowen.github.io/SquireBot`. Source under `/docs` in the repo (Jekyll picks it up automatically when Pages is enabled in repo settings → Source: Deploy from a branch → main + `/docs`). Custom theme + CSS available; planner picks a minimal/readable theme (Cayman, Slate, or similar — defer to docs-writer).
- **D-10: Asset host — in repo at `/docs/assets/`.** PNG screenshots + a single annotated GIF (no audio) for the SmartScreen walkthrough. Self-hosted, no external dependencies (no YouTube). GIF size budget: ≤5 MB to keep repo bloat tolerable; if longer than that, planner should propose a workaround.
- **D-11: Recovery doc — separate `/troubleshooting` page on the Pages site.** Linked from the install page. Structured by symptom (tray turned red → check log → match against known causes). Owns the "what now?" recovery copy. NOT inline at the bottom of the install page.
- **D-12: README.md — becomes short pointer to Pages.** Strip the existing tech-overview content (move to `docs/dev.md` or similar). New README structure: 1-paragraph "what is SquireBot," install link → `https://boejowen.github.io/SquireBot/install`, contributor link → `docs/dev.md`. Keeps GitHub's default landing page guildie-friendly without forcing a long scroll.

### Claude's Discretion

- **Eviction workflow UX** (DOC-02 implementation, not the doc itself): default to **owner-only sidebar**. Reads `_char_owner.owner_email` distinct list, presents as dropdown; "evict <email>" cascades `is_removed=TRUE` on that email's chars; writes a `_meta.eviction_log` JSON entry with timestamp + initiated_by + grace_until (now + 30d); a weekly trigger checks for grace-expired evictions and moves those chars' rows to `_archive`. Planner may refine but should not flip to self-service without re-discussing.
- **Search slot filter dropdown contents.** Planner enumerates from real inventory data (could be a one-time scrape of all distinct `Location` values across `inv:*` tabs at scaffold time, OR a hardcoded list of P99-known slots). Either is fine; pick whichever is simpler for the test mocks.
- **Pages Jekyll theme.** Planner picks a default; user can refine after Phase 5 ships.
- **System tab hide enforcement.** Already mostly handled by Phase 4's scaffold (which hides pre-existing dimension tabs). Planner verifies that ALL `_*`-prefixed tabs are hidden after `installTriggers` runs and adds a defensive `hideAllSystemTabs()` to the install path if not.
- **`Range.protect()` on `_meta.bank_toon_name`.** Same idiom as Phase 4's bank_coin protection (`setWarningOnly(true)` — script owner is default editor of strict protections so warning-only is the only UX that nudges the owner). Apply during the next `installTriggers` run.

### Scope Changes (vs. ROADMAP/REQUIREMENTS originals)

- **SEARCH-03 — staleness inline on search results — DROPPED by user during discuss.** User explicitly said *"No need to indicate sync times"* when shown the row-shape preview. Two paths the planner can take:
  1. Drop SEARCH-03 from the v1 acceptance criteria entirely. Update REQUIREMENTS.md to mark SEARCH-03 as deferred-to-v2 with a note.
  2. Keep SEARCH-03 but satisfy it on a different surface: the existing `view` and `bank` tabs already have a `Last Synced` column populated by Phase 3's builders, so staleness IS visible — just not in the search sidebar specifically. Argue this satisfies SEARCH-03's spirit ("results show staleness") even if not its letter ("inline").
  Recommend path (2) — costs nothing and preserves the requirement on paper. Planner to confirm during research/plan.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project foundations
- `.planning/PROJECT.md` — core value, key decisions log, constraints
- `.planning/REQUIREMENTS.md` — full REQ-ID list with traceability table; Phase 5 owns SEARCH-01..04, TIP-04, VIEW-05, OPS-06, DOC-01..03
- `.planning/ROADMAP.md` §84–96 — Phase 5 scope, 5 success criteria, dependency on Phase 4
- `CLAUDE.md` — project conventions (locked stack + architecture rules)

### Carry-forward decisions from prior phases
- `.planning/phases/04-differentiator-features/04-CONTEXT.md` — Phase 4 decisions still in force: stack, sidebar form pattern, Range.protect warning-only idiom, locked schema_version=3
- `.planning/phases/03-apps-script-enrichment-foundation/03-CONTEXT.md` — Phase 3 decisions: politeFetch, resumable cursor, LockService 30s tryLock, builder shape, debounce pattern
- `.planning/phases/03-apps-script-enrichment-foundation/03-PATTERNS.md` — file-by-file analogs (sidebar pattern, builder pattern, trigger pattern)
- `.planning/STATE.md` — current state (Phase 4 SHIPPED) + Phase 5 carry-over backlog from Phase 4

### Research foundations (locked early)
- `.planning/research/ARCHITECTURE.md` — sheet schema (tab inventory, dim tab purposes); Phase 5 must NOT add new dimension tabs
- `.planning/research/PITFALLS.md` — known landmines (200-tab limit, 10M cell cap, drive.file propagation delay)
- `.planning/research/STACK.md` — locked stack: Apps Script V8 + clasp v2.4 + esbuild + vitest; HtmlService for sidebars
- `.planning/research/FEATURES.md` — feature inventory (search sidebar listed as Phase 5)

### Existing code (Phase 4 deployed at HEAD = `9319c6b` apps-script changes; watcher v0.4.0)
- `apps-script/src/triggers/showCharInfoSidebar.ts` — **THE** sidebar form pattern to clone for the search sidebar (`google.script.run` callbacks, validation, lock contention)
- `apps-script/src/triggers/showBankCoinSidebar.ts` — second sidebar example (simpler form)
- `apps-script/src/triggers/installTriggers.ts` — extend with cell-count + healthcheck + system-tab-hide + bank-toon-name protect
- `apps-script/src/lib/migrations.ts` — `protectBankCoinCells` is the warning-only Range.protect template
- `apps-script/src/triggers/onOpen.ts` — add 'Search…' menu item between 'Set Bank Coin…' and 'Set Theme…'
- `apps-script/src/__tests__/test-helpers.ts` — Apps Script mocks (extend for search-sidebar tests; CacheService mock already there)
- `apps-script/src/triggers/monitorCellCount.ts` — analog for the new healthcheck trigger (similar weekly-cron-driven shape, structured _meta.last_error JSON)
- `internal/heartbeat/heartbeat.go` — watcher writes heartbeat into `_char_owner`; Phase 5's "12 guildies installed" criterion reads from this

### External / runtime dependencies
- Google Apps Script CacheService docs — 100 KB value cap, 25 min lifetime cap (relevant for D-03)
- GitHub Pages with Jekyll — repo settings → Pages → Source: Deploy from a branch → `main` + `/docs`
- `docs/apps-script-deploy.md` — current Phase 4 deploy runbook (Phase 5 should add: how to enable Pages, how to rebuild Pages on doc updates)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **Sidebar form pattern** (`showCharInfoSidebar.ts`, `showBankCoinSidebar.ts`): 3-function shape — `show*Sidebar()` opens HtmlService panel, `get*ForForm()` reads server data via `google.script.run`, `save*()` validates + writes + invokes downstream builders. **Search sidebar is a fourth instance of this pattern** (with `runSearch(query, charFilter, slotFilter)` instead of save). google.script.run round-trip + withSuccessHandler/withFailureHandler wiring is identical.
- **CacheService mock** (`test-helpers.ts`): already mocks `getDocumentCache().get/put`. Search sidebar tests need richer mock (TTL respect, key listing for invalidation tests).
- **Range.protect warning-only idiom** (`migrations.ts → protectBankCoinCells`): proven during Phase 4 smoke. Apply identically to `_meta.bank_toon_name`.
- **Resumable cursor pattern** (`refreshWikiSpells.ts`, `refreshWikiGearTier.ts`, `refreshWikiItems.ts`): not directly applicable to search (which must complete in <2 s, no resume), but IS applicable to the weekly schema healthcheck trigger if it grows beyond budget.
- **Weekly trigger shape** (`monitorCellCount.ts`): exact analog for the new schema healthcheck — same Sun-3am offset family, same `_meta.last_error` JSON shape with `kind: 'tab_missing'`.
- **Auto-hide of pre-existing tabs** (`scaffold.go`): Phase 4 fix-pack added "scaffold: hid pre-existing dimension tab" log line. Phase 5 should generalize this to a `hideAllSystemTabs()` callable from `installTriggers`.

### Established Patterns

- **Per-cell `Range.protect(true)` warning-only** is the only protection variant we use (script owner is default editor of strict protections, so strict mode is invisible to them — Phase 4 smoke discovery).
- **`_meta.last_error` JSON envelope**: `{at, where, kind, detail}` — schema healthcheck conforms.
- **`google.script.run` callbacks** are global functions exported in `Code.ts` AND listed in `build.mjs` `TRIGGER_GLOBALS`. The build-time CI assertion (Phase 3 lesson) catches Code.ts↔globals divergence.
- **`onOpen` SquireBot menu** (now ~10 items after Phase 4): adding 'Search…' makes it ~11. Planner should consider whether to introduce sub-menus (`SquireBot → Tools →`) if the menu gets crowded; not critical at 11 items.
- **README.md is currently the long-form doc** (~150 lines including stack, install hint, screenshot of view tab). Phase 5 D-12 says replace this with a short Pages pointer; the existing content moves to `docs/dev.md` (or splits across the new Pages install/troubleshooting pages).

### Integration Points

- **Search sidebar opens via menu**: `onOpen.ts` adds `addItem('Search…', 'showSearchSidebar')` between Set Bank Coin and Set Theme.
- **Search reads `inv:*`**: iterates `ss.getSheets()` filtering names matching `inv:*`; for each, reads col B (Name) + col A (Location) + col D (Count) + col C (ID).
- **Search reads `_item_master` for wiki link + summary**: existing tab populated by Phase 3 `refreshWikiItems`.
- **Search reads `_pigparse` for price**: existing tab populated by Phase 3 `refreshPigparse`.
- **Watcher cache invalidation**: when watcher writes `inv:<Name>`, the apps-script side has no notification mechanism — invalidation has to be lazy (next search rebuilds that char's cache entry by virtue of TTL expiry). 60s TTL is the actual freshness guarantee, NOT cache-bust-on-write. Planner: do NOT design for push invalidation.
- **Healthcheck integrates with `_meta.last_error`**: existing tray-side reader already surfaces `_meta.last_error` to the watcher's heartbeat which sets the tray red.
- **Eviction `_archive` tab**: hidden tab not in current scaffold; planner needs to decide whether to add it to `internal/scaffold/scaffold.go` (schema bump v3→v4) OR create lazily on first eviction (no schema bump). Lazy creation is preferred — avoids bumping watcher version for a feature most workbooks won't use immediately.
- **GitHub Pages enablement**: requires repo-owner action in GitHub settings. Planner should include "enable Pages on `main`/`docs`" as an explicit step in the deploy runbook update.

</code_context>

<specifics>
## Specific Ideas

- **"No need to indicate sync times"** (search results) — user direction during area-2 Q1, captured as scope change for SEARCH-03.
- **Show last 3 recent searches** (not 1) — user note on area-2 Q4. CacheService key `squirebot:search:recent`, value = JSON array, MRU-ordered, capped at 3.
- **Group results by item name** (not by char) — user picked this in area-2 Q2 against the alternative "all of Findom's matches first." Reflects the dominant query intent: "who has THIS item?" not "what does Findom have?"
- **Auto-collapse groups when >5 chars** — user picked this in area-2 Q3. The 5-threshold is an implementation choice; planner may tune to 4 or 6 based on real data, but the COLLAPSE behavior is locked.
- **Annotated GIF, no audio** — user picked this in area-3 Q3 over MP4 + YouTube. Self-hosting bias.
- **Separate troubleshooting page** — user picked this in area-3 Q4 over inline-at-end-of-install. Rationale: recovery content is search-driven (guildie comes to it via tray-red panic), not read-linearly.
- **Workbook is `RiverFAIL VanFRAUD`** (per dev box context) — solo workbook, no concurrent guildies during Phase 5 development. Live-deploy risk is low; rc-then-promote is still the right cadence.

</specifics>

<deferred>
## Deferred Ideas

### Carried over from Phase 4 deferred backlog (STATE.md) — STAY DEFERRED

- **Bank-coin permission lock** (only bank-toon-owner can use Set Bank Coin sidebar) — v1.0.x patch candidate. Not folded into Phase 5; keep deferred unless it bites real usage.
- **Polished theme picker tile UI** (6-tile grid per `docs/design/mockups/eq-aesthetic-picker.html`) — v1.0.x patch candidate. Phase 5's docs work doesn't need it.
- **Sidebar HTML inline-JS unit tests** — v2 ergonomics. Smoke tests prove the sidebars work; unit-testing inline `<script>` blocks needs a JSDOM setup not currently in vitest.
- **Installer-driven upgrade UX** (NSIS can't overwrite running .exe) — fold into Phase 5 ONLY if it actually blocks the 12-guildie distribution criterion. Otherwise add the workaround to `/troubleshooting` and defer the structural fix to v1.0.1 or v2.

### New deferred ideas surfaced during discuss

- **Self-service eviction** (departing guildie quits cleanly without owner action) — threat model: departing guildie could spite-nuke data. Defer to v2 if amicable-departures-only assumption stops holding.
- **Power-user inline search syntax** (`char:Findom slot:HEAD`) — area-1 Q1 alternative; deferred. UI controls (D-01) handle the common case.
- **Word-prefix / fuzzy primary match** (area-1 Q2 alternatives) — substring won; fuzzy survives only as the no-match fallback (D-04).
- **Index-cache search shape** (whole-workbook flat index vs. per-char) — area-1 Q3 alternative. Per-char fits the CacheService cap naturally (D-03); index would need chunking.
- **Card-with-expand-on-click result row** (area-2 Q1 alternative) — two-line stacked won; expand-on-click adds DOM complexity without enough win.
- **YouTube video for SmartScreen walkthrough** (area-3 Q3 alternative) — annotated GIF won; YouTube survives only if GIF size becomes prohibitive.
- **Sub-menu structure for SquireBot menu** (when item count grows past ~12) — v2 polish.

</deferred>

---

*Phase: 5-search-onboarding-privacy-polish*
*Context gathered: 2026-05-11*
