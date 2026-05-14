# Phase 5: Search + Onboarding + Privacy Polish — Research

**Researched:** 2026-05-11
**Domain:** Apps Script HtmlService sidebars (cross-tab item search + eviction), Apps Script weekly maintenance triggers, Google Sheets workbook protection/hide, Jekyll GitHub Pages onboarding site
**Confidence:** HIGH

## Summary

Phase 5 is the final phase before v1.0 ships. It splits across three orthogonal surfaces that share almost nothing technically:

1. **A cross-character search HtmlService sidebar** that scans every `inv:*` tab under a hard 2-second budget. This is the technical center of the phase. The 100KB CacheService value cap, the ~12 `inv:*` tabs × ~300 rows ≈ ~3,600 rows of total inventory (the CONTEXT.md "36,000" figure is a CONTEXT-doc miscount — see §Pitfall P5), and the proven Phase 4 `showCharInfoSidebar` form pattern jointly determine the architecture: per-`inv:Char` CacheService entries (key `squirebot:search:inv:<Char>`, JSON-encoded compact row array, 60s TTL), substring scan in-memory, group-by-item rendering with progressive paint as each char's cache populates.
2. **Privacy / housekeeping polish:** auto-hide every `_`-prefixed dimension tab on every install run, `Range.protect()` warning-only on `_meta.bank_toon_name` (same proven idiom as Phase 4's `protectBankCoinCells`), and a weekly `weeklySchemaHealthcheck` trigger that verifies all required tabs exist and writes structured errors to `_meta.last_error`. Also a stale-char auto-archive that moves entire `inv:<Char>` and `spell:<Char>` tabs to a hidden `_archive` (or copies their content into an `_archive` master tab) when the watcher hasn't written for >90 days, plus an eviction sidebar that flips `_char_owner.is_removed=TRUE` and waits 30 days before archiving.
3. **A Jekyll GitHub Pages onboarding site at `/docs/`** with three pages (`install.md`, `troubleshooting.md`, `dev.md` + `index.md`), PNG screenshots + one annotated GIF (≤5 MB) of the SmartScreen walkthrough, plus a README.md that shrinks to a 1-paragraph pointer.

The phase requires NO new external runtime dependencies (no React, no CDN, no third-party UI library). Phase 4 already shipped the proven primitives — `HtmlService` sidebars, `LockService.tryLock(30000)` with try/finally, `Range.protect().setWarningOnly(true)`, weekly time-driven triggers, structured `_meta.last_error` JSON, theme-aware CSS via `THEMES` registry. Phase 5 stitches them together.

**Primary recommendation:** Build the search sidebar around a **per-`inv:Char` `CacheService.getDocumentCache()`** indexing pass, scan the in-memory result with case-insensitive substring (D-02), render groups progressively as each cache entry resolves. Treat the cold-start path (12 sheet reads × ~300 rows ≈ ~2–4s total) as out-of-budget and warm the cache during `installTriggers` (or via the `onChange` debounce already used by `buildView`). For everything else (hide tabs, protect `bank_toon_name`, healthcheck, archive, eviction, Pages site), clone the Phase 4 patterns line-for-line.

## Architectural Responsibility Map

Phase 5 spans two architectural tiers; the Pages site is a third (static) surface.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Cross-character search UI | HtmlService sidebar (browser-rendered) | Apps Script server (data + cache) | Browser owns rendering; server owns scan + cache; google.script.run is the seam |
| Search index / cache | Apps Script server | `CacheService.getDocumentCache` | Per-`inv:Char` cache entries; 60s TTL; never client-stored |
| Did-you-mean fuzzy fallback | Apps Script server | — | Levenshtein runs in V8 server-side; no network cost beyond initial server call |
| `Recent:` query history | Apps Script server | `CacheService.getDocumentCache` (workbook-scoped) | Per-workbook history; key `squirebot:search:recent`; JSON array |
| Eviction sidebar (DOC-02) | HtmlService sidebar | Apps Script server (`_char_owner` write + `_meta.eviction_log` append) | Same pattern as `showCharInfoSidebar` |
| Hide `_`-prefixed tabs | Apps Script server | `installTriggers` (idempotent) | Tabs are workbook structure — server-only operation |
| `Range.protect()` on `_meta.bank_toon_name` | Apps Script server | `migrations.ts` (warning-only template) | Identical idiom to Phase 4 `protectBankCoinCells` |
| Weekly schema healthcheck | Apps Script time-driven trigger | `_meta.last_error` envelope | Same trigger shape as `monitorCellCount` |
| Stale-char auto-archive | Apps Script time-driven trigger | `_archive` tab (lazy-created) | Server-only; never invoked from sidebar |
| Onboarding site | Jekyll (GitHub Pages CDN) | `/docs/` source in repo | Static surface; brand-neutral; rendered server-side by GitHub Pages build |
| SmartScreen walkthrough asset | Static (GIF in `/docs/assets/`) | — | Pre-recorded asset; ≤5 MB; no autoplay |
| Tray-red recovery doc | Static markdown (`/docs/troubleshooting.md`) | Cross-link from `/install.md` | Pure content; no code |

**Tier check:** Nothing in Phase 5 belongs in the watcher (Go) tier. Watcher changes are limited to bumping `WatcherMaxSchemaVersion` IF (and only if) the phase ships a schema migration. The phase plan should avoid a migration unless the eviction `_archive` table requires one — and per `code_context` in CONTEXT.md the recommendation is **lazy creation** (no schema bump).

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Search query model:**
- **D-01: Input shape — free-text + minimal filters.** Sidebar has a text input PLUS a Char dropdown (any | every char in `_char_owner`) PLUS a Slot dropdown (any | enumerated inventory slots). No inline syntax (`char:foo`).
- **D-02: Match semantics — case-insensitive substring.** `russet` matches anywhere within an item Name. Single-pass scan.
- **D-03: Cache shape — per-`inv:Char`.** Cache key per inv tab (`squirebot:search:inv:Findom` → JSON row array). 60s TTL. Invalidation lazy (no push from watcher — see Pitfall P3).
- **D-04: Empty-state UX — "No matches" + did-you-mean fuzzy fallback.** Levenshtein edit-distance ≤2 against item names, up to 3 suggestions, clickable to re-run.

**Search results layout:**
- **D-05: Row shape — two-line stacked.** Line 1: `<Char>: <Location>, count <N>`. Line 2 (smaller, muted): `Wiki · <price> pp`. Tooltip on item name = full wiki summary from `_item_master`.
- **D-06: Sort order — group by item name, char asc within group.**
- **D-07: High-cardinality handling — auto-collapse when >5 chars match one item.** Click group header to expand.
- **D-08: Lifecycle — open empty, auto-focus input, "Recent:" footer with last 3 queries.** CacheService key `squirebot:search:recent`, MRU-ordered.

**Onboarding docs + assets:**
- **D-09: Doc platform — GitHub Pages with Jekyll.** Site at `boejowen.github.io/SquireBot`. Source under `/docs/`.
- **D-10: Asset host — in repo at `/docs/assets/`.** PNG screenshots + a single annotated GIF (no audio) for SmartScreen walkthrough. GIF size budget: ≤5 MB.
- **D-11: Recovery doc — separate `/troubleshooting` page** on the Pages site, linked from install.
- **D-12: README.md — becomes short pointer to Pages.** Existing tech-overview content moves to `docs/dev.md`.

**Scope changes:**
- **SEARCH-03 (inline staleness on results) — DROPPED for the search-sidebar surface.** Path 2 chosen: the existing `view` and `bank` tabs' `Last Synced` column satisfies SEARCH-03's spirit. Update `REQUIREMENTS.md` traceability accordingly during plan time.

### Claude's Discretion

- **Eviction workflow UX shape** — defaulted to **owner-only sidebar** (officer dropdown → cascade `is_removed=TRUE` → `_meta.eviction_log` JSON entry → weekly archive after grace_until). Re-confirm if scope drifts to self-service.
- **Search slot filter dropdown contents** — enumerate from real inventory data OR hardcode P99 slot list. Either acceptable.
- **Pages Jekyll theme** — Cayman recommended in UI-SPEC; Slate acceptable. Planner picks.
- **System tab hide enforcement** — extend Phase 4's pattern; ensure ALL `_*` tabs hidden after `installTriggers` (per UI-SPEC Component 11 `hideAllSystemTabs`).
- **`Range.protect()` on `_meta.bank_toon_name`** — same warning-only idiom as Phase 4 bank coin cells; one cell.

### Deferred Ideas (OUT OF SCOPE)

From CONTEXT.md `<deferred>`:

- **Bank-coin permission lock** (only bank-toon-owner can use Set Bank Coin sidebar) — v1.0.x patch candidate.
- **Polished theme picker tile UI** — v1.0.x patch candidate.
- **Sidebar HTML inline-JS unit tests** — v2 ergonomics.
- **Installer-driven upgrade UX** (NSIS can't overwrite running `.exe`) — fold into Phase 5 ONLY if it bites real distribution; otherwise document workaround in `/troubleshooting`.
- **Self-service eviction** — defer to v2 if amicable-departures-only assumption breaks.
- **Power-user inline search syntax (`char:Findom slot:HEAD`)** — UI controls cover the common case.
- **Word-prefix / fuzzy primary match** — fuzzy survives only as no-match fallback (D-04).
- **Index-cache search shape** — per-char fits CacheService cap better (D-03).
- **Card-with-expand-on-click result row** — two-line stacked won (D-05).
- **YouTube video for SmartScreen walkthrough** — annotated GIF won; YouTube revives only if GIF size becomes prohibitive (D-10).
- **Sub-menu structure for SquireBot menu** — v2 polish when item count > 12.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEARCH-01 | HtmlService sidebar (~300 px) launched from custom `onOpen` menu | §Standard Stack (HtmlService), §Pattern 1, §Code Example 1; UI-SPEC §Spacing Scale locks 300px |
| SEARCH-02 | Query returns matches across every `inv:*` tab in <2s, formatted per result-row contract | §Pattern 2 (per-char cache + progressive render), §Pitfall P1 (cold path), §Code Example 2 |
| SEARCH-03 | (Reframed per CONTEXT scope change) — satisfied by existing `view`/`bank` `Last Synced` columns; sidebar does NOT show per-row staleness | §User Constraints — Scope Changes; planner action: update REQUIREMENTS.md traceability note |
| SEARCH-04 | Search results cached in `CacheService` 60s | §Pattern 2; §Pitfall P4 (100KB cap per entry) |
| TIP-04 | Richer detail (full wiki summary) in sidebar tooltip | §Pattern 3 (read `_item_master.wiki_summary` server-side, embed in result-row JSON, render via `title` attribute or hover popover) |
| VIEW-05 | Stale chars (`inventory_mtime > 90d`) auto-archived to hidden `_archive` tab | §Pattern 6, §Pitfall P6 (watcher race), §Don't Hand-Roll (lazy `_archive` creation) |
| OPS-06 | Weekly Apps Script healthcheck verifies all expected tabs exist BY ID; missing-tab errors → `_meta.last_error` | §Pattern 5, §Code Example 4, §Pitfall P7 (tab-by-ID vs tab-by-name) |
| DOC-01 | README documents install / SmartScreen / OAuth / EQ folder picker / tray-red recovery | §Pattern 7 (Jekyll Pages); install copy split per UI-SPEC |
| DOC-02 | Eviction workflow runbook (cascade `is_removed=TRUE` → 30-day grace → archive) end-to-end tested | §Pattern 4 (eviction sidebar + grace timer), §Code Example 5 |
| DOC-03 | Onboarding screenshots + short video linked from download page | §Pattern 7 (PNG + GIF in `/docs/assets/`); §Pitfall P9 (GIF size) |

## Standard Stack

No new external libraries are added to Apps Script in Phase 5. The phase is implemented entirely on the stack frozen at Phase 3.

### Core (already locked at Phase 3)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Google Apps Script V8 runtime | (Workspace) | Hosts all sidebar + trigger code | LOCKED Phase 3; Rhino EOL 2026-01-31 [VERIFIED: STATE.md decision #1] |
| `@google/clasp` | 2.5.0 (do NOT use 3.x) | Push TS-compiled bundle to Apps Script | CLAUDE.md locks 2.4+; npm view confirms 2.5.0 is highest 2.x [VERIFIED: `npm view @google/clasp@2 version` → 2.3.2, 2.4.0, 2.4.1, 2.4.2, 2.5.0] |
| `esbuild` | 0.28.0 (latest) | TS → single `dist/Code.js` IIFE | CLAUDE.md locks 0.20+; current latest verified [VERIFIED: `npm view esbuild version` → 0.28.0] |
| `@types/google-apps-script` | 2.0.8 | Type definitions for `HtmlService`, `CacheService`, `LockService` | Already installed [VERIFIED: npm view] |
| `vitest` | (existing) | Unit-test runner for server-side TS | Already installed; sidebar HTML inline-JS testing is deferred per CONTEXT |

### Supporting (Apps Script built-ins — already in use)

| Service | Used In Phase 5 For | When |
|---------|---------------------|------|
| `HtmlService.createHtmlOutput` | Search sidebar, Eviction sidebar | Both new sidebars [CITED: developers.google.com/apps-script/guides/html] |
| `CacheService.getDocumentCache()` | Per-`inv:Char` search index; `Recent:` history | Server-side cache only [CITED: developers.google.com/apps-script/reference/cache/cache] |
| `LockService.getDocumentLock().tryLock(30000)` | All sidebar mutations | Identical pattern to Phase 4 [VERIFIED: `showCharInfoSidebar.ts:99`, `showBankCoinSidebar.ts:54`] |
| `ScriptApp.newTrigger().timeBased().onWeekDay()...` | `weeklySchemaHealthcheck`, `weeklyStaleCharArchive`, `weeklyEvictionArchive` | Same shape as `monitorCellCount` [VERIFIED: `installTriggers.ts:81`] |
| `SpreadsheetApp.getActiveSpreadsheet().getSheetByName` | Tab access; also `hideSheet`, `getSheetId` | Use `getSheetId()` (returns sheet GID) — store IDs in `_meta.expected_sheet_ids` JSON for the healthcheck [CITED: developers.google.com/apps-script/reference/spreadsheet/sheet] |
| `Range.protect().setWarningOnly(true)` | `_meta.bank_toon_name` cell | Identical idiom to `protectBankCoinCells` [VERIFIED: `migrations.ts:149`] |
| `PropertiesService.getDocumentProperties()` | Grace-period timer for eviction; debounce timestamps for search | Same idiom as `buildView`'s `view_last_build_ms` [VERIFIED: `buildView.ts:60`] |

### Static Site Stack (new for Phase 5)

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| GitHub Pages (Jekyll built-in) | (GitHub-managed) | Hosts onboarding site at `boejowen.github.io/SquireBot` | LOCKED in D-09; D-12 makes README a pointer [VERIFIED: pages-themes/cayman README] |
| Jekyll Cayman theme | `pages-themes/cayman@v0.2.0` | Default theme — responsive, single-color hero, mobile-friendly | UI-SPEC §Onboarding Site Visual Contract; GitHub-official; configured via `remote_theme: pages-themes/cayman@v0.2.0` in `_config.yml` [VERIFIED: github.com/pages-themes/cayman] |
| `jekyll-remote-theme` plugin | (GitHub Pages built-in) | Required to use `remote_theme` directive | Documented [CITED: pages-themes/cayman README] |

**Installation (Pages site):** Nothing to `npm install`. The build is:

1. Create `/docs/_config.yml` with `remote_theme: pages-themes/cayman@v0.2.0`, `title: SquireBot`, `description: ...`, `plugins: [jekyll-remote-theme]`.
2. Create `/docs/index.md`, `/docs/install.md`, `/docs/troubleshooting.md`, `/docs/dev.md` (front-matter `layout: default`).
3. Enable Pages on the repo: Settings → Pages → Source: Deploy from a branch → `main` + `/docs`.
4. First push triggers Pages build; URL appears in Settings → Pages.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Per-`inv:Char` CacheService entries | Single workbook-wide flat index | Would breach 100KB-per-value cap with 12 chars × ~300 rows × ~100 bytes ≈ 360KB total; per-char fits naturally at ~30KB each. [VERIFIED: §Pitfall P4] |
| Per-`inv:Char` CacheService entries | `PropertiesService` | PropertiesService has the same 9KB/value, 500KB/script cap and is slower; CacheService designed exactly for this. |
| Substring scan in-memory | Sheets `=QUERY(...)` formulas | Formula-driven search can't return rich result objects with grouping and wiki/price enrichment; also can't show "did-you-mean." Substring scan in JS is ~50ms for ~3,600 rows. |
| Lazy `_archive` creation | Schema-version bump to v4 + scaffold `_archive` | Schema bump requires `WatcherMaxSchemaVersion` bump + new watcher build + force-update for all guildies BEFORE migration ships. CONTEXT.md `code_context` recommends lazy creation. |
| Cayman theme | Slate theme | Slate is darker / more dev-doc-aesthetic; Cayman is brighter / more onboarding-aesthetic. Either acceptable per UI-SPEC; Cayman recommended. |
| GIF | MP4 + YouTube unlisted | Annotated GIF won during discuss (D-10). YouTube survives only if GIF >5MB. |
| `fastest-levenshtein` npm dep | Hand-rolled Levenshtein (≤30 lines) | Phase 5 is "no new external runtime deps." Levenshtein is small enough to hand-roll; bundle size matters since esbuild output goes through clasp push. [CITED: github.com/ka-weihe/fastest-levenshtein] |

**Version verification (2026-05-11):**
- `@google/clasp@2`: latest is 2.5.0 [VERIFIED: npm view 2026-05-11]
- `@google/clasp@3.3.0`: latest 3.x; CONTEXT.md and CLAUDE.md explicitly FORBID 3.x [VERIFIED: CLAUDE.md line 18]
- `esbuild`: 0.28.0 [VERIFIED: npm view 2026-05-11]
- `@types/google-apps-script`: 2.0.8 [VERIFIED: npm view 2026-05-11]
- Cayman theme: `pages-themes/cayman@v0.2.0` is current [VERIFIED: github.com/pages-themes/cayman]

## Architecture Patterns

### System Architecture Diagram

```
              ┌──────────────────────────────────────────┐
              │   SquireBot menu (onOpen.ts)             │
              │                                          │
              │   Set Character Info…                    │
              │   Set Bank Coin…                         │
              │   ▶ Search…              (NEW Phase 5)   │
              │   ▶ Evict Guildie…       (NEW Phase 5)   │
              │   Set Theme…                             │
              │   …                                      │
              └────────┬────────────────────┬────────────┘
                       │                    │
                       ▼                    ▼
        ┌──────────────────────┐  ┌─────────────────────────┐
        │ showSearchSidebar()  │  │ showEvictionSidebar()   │
        │ (triggers/*.ts)      │  │ (triggers/*.ts)         │
        │                      │  │                         │
        │ Renders HtmlService  │  │ Renders HtmlService     │
        │ output from          │  │ output from             │
        │ sidebars/search-     │  │ sidebars/eviction-      │
        │ Sidebar.html         │  │ Sidebar.html            │
        │ (theme tokens        │  │                         │
        │ injected)            │  │                         │
        └──────────┬───────────┘  └──────────┬──────────────┘
                   │                          │
       google.script.run                google.script.run
                   │                          │
                   ▼                          ▼
        ┌────────────────────────┐  ┌────────────────────────┐
        │ runSearch(q, cf, sf)   │  │ getEvictionEmails()    │
        │ getRecentSearches()    │  │ previewEviction(email) │
        │ pushRecentSearch(q)    │  │ commitEviction(email)  │
        └──────────┬─────────────┘  └──────────┬─────────────┘
                   │                            │
                   ▼                            ▼
        ┌────────────────────────────────────────────────────┐
        │  CacheService.getDocumentCache()                   │
        │    squirebot:search:inv:<Char>   → rows JSON       │
        │    squirebot:search:recent       → [q1, q2, q3]    │
        │    squirebot:search:items_master → wiki+price index│
        │                                                    │
        │  SpreadsheetApp (read inv:* / _item_master /       │
        │                  _pigparse / _char_owner)          │
        │                                                    │
        │  LockService.getDocumentLock().tryLock(30000)      │
        │     wraps every write path (_char_owner,           │
        │     _meta.eviction_log, _archive append)           │
        └────────────────────────────────────────────────────┘

  ┌─────────────────────────────────────────────────────────┐
  │ Time-driven triggers (installTriggers registers)        │
  │                                                         │
  │   • onChange                              (existing)    │
  │   • buildView (1h backstop)               (existing)    │
  │   • refreshPigparse  daily 03:00 PT       (existing)    │
  │   • refreshWikiItems  Sun 04:00 PT        (existing)    │
  │   • refreshWikiSpells Sun 04:00 PT        (existing)    │
  │   • refreshWikiGearTier Sun 05:00 PT      (existing)    │
  │   • monitorCellCount  Sun 03:00 PT        (existing)    │
  │   • weeklySchemaHealthcheck Sun 03:00 PT  (NEW Phase 5) │
  │   • weeklyStaleCharArchive  Sun 06:00 PT  (NEW Phase 5) │
  │   • weeklyEvictionArchive   Sun 06:30 PT  (NEW Phase 5) │
  └─────────────────────────────────────────────────────────┘

  ┌─────────────────────────────────────────────────────────┐
  │ Onboarding Pages site  (GitHub Pages CDN)               │
  │                                                         │
  │   /docs/_config.yml      (remote_theme: cayman)         │
  │   /docs/index.md         (1-paragraph landing)          │
  │   /docs/install.md       (5-step chronological)         │
  │   /docs/troubleshooting.md  (symptom-keyed)             │
  │   /docs/dev.md           (link list)                    │
  │   /docs/assets/*.png     (PNG screenshots)              │
  │   /docs/assets/smartscreen.gif   (≤5 MB annotated GIF)  │
  │                                                         │
  │   README.md → links to https://boejowen.github.io/      │
  │                        SquireBot/                       │
  └─────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
apps-script/src/
├── triggers/
│   ├── showSearchSidebar.ts          # NEW — opens search sidebar; runSearch callback
│   ├── showEvictionSidebar.ts        # NEW — opens eviction sidebar; commitEviction callback
│   ├── weeklySchemaHealthcheck.ts    # NEW — analog of monitorCellCount
│   ├── weeklyStaleCharArchive.ts     # NEW — VIEW-05 auto-archive >90d
│   ├── weeklyEvictionArchive.ts      # NEW — grace-expired eviction archiver
│   ├── installTriggers.ts            # EXTENDED — register 3 new triggers + hideAllSystemTabs + protect bank_toon_name
│   ├── onOpen.ts                     # EXTENDED — add Search… and Evict Guildie… menu items
│   └── … (existing)
├── sidebars/                          # NEW DIR
│   ├── searchSidebar.html            # NEW — theme-aware HTML template
│   └── evictionSidebar.html          # NEW — theme-aware HTML template
├── lib/
│   ├── searchIndex.ts                # NEW — buildInvCache, searchInvCache, didYouMean
│   ├── migrations.ts                 # EXTENDED — protectBankToonName + hideAllSystemTabs
│   ├── archive.ts                    # NEW — moveCharToArchive(charName) helper
│   └── … (existing)
├── tabs/
│   └── … (existing — no new tabs in v1 plan; _archive is created lazily by archive.ts)
└── __tests__/
    ├── searchIndex.test.ts           # NEW
    ├── weeklySchemaHealthcheck.test.ts  # NEW
    ├── archive.test.ts               # NEW
    └── … (existing)

docs/                                  # GitHub Pages site (Jekyll auto-picked-up)
├── _config.yml                       # NEW — remote_theme cayman
├── index.md                          # NEW — landing
├── install.md                        # NEW — chronological 5-step
├── troubleshooting.md                # NEW — symptom-keyed
├── dev.md                            # NEW — link list into existing docs
└── assets/                           # NEW
    ├── 01-installer.png
    ├── 02-smartscreen-more-info.png
    ├── 03-smartscreen-run-anyway.png
    ├── 04-oauth-consent.png
    ├── 05-folder-picker.png
    ├── smartscreen.gif              # ≤5 MB annotated GIF
    └── tray-red.png

README.md                              # SHRINK — 1-paragraph pointer per D-12
```

### Pattern 1: Theme-Aware HtmlService Sidebar with Phase 4 Form Pattern

**What:** Reuse `showCharInfoSidebar.ts` / `showBankCoinSidebar.ts` 3-function shape, augment with theme-token injection.

**When to use:** Both new sidebars (search + eviction).

**Function shape (proven in Phase 4):**
1. `showFooSidebar()` — opens the panel via `HtmlService.createHtmlOutput(...).setTitle(...).setWidth(300)`.
2. `getFooForForm()` (or `runFoo(...)`) — server-side data read, invoked from client via `google.script.run.withSuccessHandler(...).getFooForForm()`.
3. `saveFoo(payload)` (or `commitFoo`) — server-side write wrapped in `LockService.getDocumentLock().tryLock(30000)` + try/finally.

**Theme awareness (NEW Phase 5 baseline — UI-SPEC §Color):**
- `showSearchSidebar()` reads `getActiveTheme()` from `lib/themes.ts`, then renders the HTML template with theme tokens injected as CSS custom properties on `:root`.
- For `sheets-default` (`THEMES[key] === null`), emit NO `<style>` token block — use browser defaults.
- Existing sidebars (`showCharInfoSidebar`, `showBankCoinSidebar`) remain on the hardcoded Arial 13px baseline; do NOT retrofit them in this phase (out of scope per CONTEXT — deferred theme picker tile UI doesn't apply here either).

**Example (skeleton):**

```typescript
// Source: derived from apps-script/src/triggers/showCharInfoSidebar.ts
import { getActiveTheme, THEMES } from '../lib/themes';

export function showSearchSidebar(): void {
  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];
  const html = HtmlService.createHtmlOutput(buildSidebarHtml(theme))
    .setTitle('SquireBot — Search')
    .setWidth(300);  // UI-SPEC locks 300px
  SpreadsheetApp.getUi().showSidebar(html);
}

function buildSidebarHtml(theme: Theme | null): string {
  const themeStyle = theme ? `
    <style>:root {
      --bg: ${theme.headerBg};
      --bg-row: ${theme.rowAltBg};
      --fg: ${theme.rowFg};
      --fg-header: ${theme.headerFg};
      --accent-bg: ${theme.accentBg};
      --accent-fg: ${theme.accentFg};
      --font-header: ${theme.fontFamily};
      --font-body: ${theme.fontFamily};
    }</style>` : '';
  return `
${themeStyle}
<style>/* structural CSS — see UI-SPEC §Performance Contract: ≤8KB */</style>
<div class="sidebar"> … </div>
<script>/* vanilla JS — see UI-SPEC: ≤12KB */</script>
`;
}
```

### Pattern 2: Per-`inv:Char` CacheService Search Index with Progressive Render

**What:** Read each `inv:<Char>` tab independently, project rows to a compact JSON shape, store under `squirebot:search:inv:<Char>` with 60s TTL. Substring scan happens in-memory after all entries are loaded (or progressively as they resolve).

**When to use:** Every `runSearch(query, charFilter, slotFilter)` call.

**Cache shape:**
```typescript
// Each squirebot:search:inv:<Char> entry:
type CachedInvRow = [string, string, number, number];  // [Location, Name, ID, Count]
// JSON.stringify(rows) where rows: CachedInvRow[]
// ~50 bytes per row × ~300 rows ≈ 15 KB per char — well under 100 KB cap
```

**Cold path (cache empty):**
1. `runSearch` reads each `inv:*` sheet via `SpreadsheetApp.getActive().getSheets()` filter, projects to `CachedInvRow[]`, stores in CacheService.
2. Same call scans the in-memory result and returns matches.
3. Estimated cold time for 12 chars × ~300 rows: ~2-4 s (cache miss + 12 sheet reads).

**Warm path (cache populated):**
1. `runSearch` reads each cache entry via `CacheService.getDocumentCache().getAll(keys)` (single round-trip).
2. Scans in-memory; returns matches.
3. Estimated warm time: ~200-500 ms.

**Budget compliance (SEARCH-01 says <2s):**
- COLD path **may exceed 2s** at 12 chars. Mitigation: warm the cache after `installTriggers` runs OR after `buildView`'s `onChange` debounce fires. Pre-warm code: iterate `inv:*` sheets and run the same project-and-cache pass that `runSearch` does. Document for planner: warm-up is a fire-and-forget call from `onChange` (already runs on every workbook change). After first onChange, cold-path is gone.

**Filter application (D-01):**
- Char filter — server-side: skip char's cache entry if not selected.
- Slot filter — server-side: filter `Location` field on cached rows. Slot dropdown values come from `lib/eq-constants.ts` (add `SLOTS` list) — see §Don't Hand-Roll.

**Enrichment join (D-05 line 2):**
- Server reads `_item_master` (for `wiki_url`, `wiki_summary`) and `_pigparse` (for price) ONCE per `runSearch` call (cache them under `squirebot:search:items_master` and `squirebot:search:pigparse` with 60s TTL; both fit under 100KB compressed).
- Join in JS by `item_id`. Result row carries `wikiUrl`, `wikiSummary` (for tooltip), `price`.

### Pattern 3: Did-You-Mean Levenshtein Fallback (D-04)

**What:** When substring scan returns 0 matches, run a hand-rolled Levenshtein distance pass against unique item names from `_item_master`. Suggest up to 3 items with distance ≤2.

**Why hand-roll:** Phase 5 is "no new external runtime deps" (CLAUDE.md, CONTEXT.md stack constraints). The Wagner-Fischer DP algorithm is ~25 lines; `fastest-levenshtein` is the npm gold standard but bundling it adds ~3KB to `dist/Code.js` for one-shot use [CITED: github.com/ka-weihe/fastest-levenshtein]. Hand-roll the function in `lib/searchIndex.ts`.

**Performance:**
- ~2,000 unique items × ~10 distance computations each ≈ 20,000 ops on short strings (avg 25 chars). Even an unoptimized impl runs in <100 ms in V8.
- Per UI-SPEC and D-04, the fuzzy path adds ~1-2s wall time which is acceptable since the user already got "no matches" — no <2s budget on the fallback branch.

**Algorithm (Wagner-Fischer, hand-rolled):**

```typescript
// Source: standard DP Levenshtein; verified against fastest-levenshtein output
function levenshtein(a: string, b: string): number {
  if (a === b) return 0;
  if (a.length === 0) return b.length;
  if (b.length === 0) return a.length;
  const al = a.length, bl = b.length;
  let prev = new Array(bl + 1);
  for (let j = 0; j <= bl; j++) prev[j] = j;
  for (let i = 1; i <= al; i++) {
    const curr = new Array(bl + 1);
    curr[0] = i;
    for (let j = 1; j <= bl; j++) {
      const cost = a.charCodeAt(i - 1) === b.charCodeAt(j - 1) ? 0 : 1;
      curr[j] = Math.min(curr[j - 1] + 1, prev[j] + 1, prev[j - 1] + cost);
    }
    prev = curr;
  }
  return prev[bl];
}

export function didYouMean(query: string, itemNames: string[]): string[] {
  const q = query.toLowerCase();
  return itemNames
    .map((n) => ({ n, d: levenshtein(q, n.toLowerCase()) }))
    .filter((x) => x.d <= 2)
    .sort((a, b) => a.d - b.d)
    .slice(0, 3)
    .map((x) => x.n);
}
```

### Pattern 4: Eviction Sidebar + 30-Day Grace + Weekly Archiver

**What:** Three-piece flow:

1. **Eviction sidebar** (`showEvictionSidebar`): lists distinct `owner_email` values from `_char_owner` in a dropdown. On `commitEviction(email)`:
   - Cascade `_char_owner.is_removed = TRUE` for every row matching `email`.
   - Append a JSON entry to `_meta.eviction_log` with `{at, email, initiated_by, grace_until: now+30d, chars: [...]}`.
   - `LockService.getDocumentLock().tryLock(30000)` wraps the cascade.

2. **`_meta.eviction_log` shape:** a single `_meta` row whose key is `eviction_log` and whose value is a JSON array (append-only, oldest at index 0). Each entry: `{at: ISO8601, email: string, initiated_by: string, grace_until: ISO8601, chars: string[]}`. Re-eviction of the same email appends a new entry rather than mutating the old one (audit trail).

3. **Weekly archiver** (`weeklyEvictionArchive`, Sun 06:30 PT): reads `_meta.eviction_log`, finds entries with `grace_until < now`, and for each char in those entries calls `archive.moveCharToArchive(charName)` (which also hides any `inv:<Char>` / `spell:<Char>` tabs).

**Identity:** UI-SPEC sets the initiator string. Apps Script `Session.getActiveUser().getEmail()` returns the SCRIPT OWNER, not the workbook owner who triggered the sidebar — load-bearing distinction noted in CLAUDE.md. Use `Session.getEffectiveUser().getEmail()` only when running as the user (which is the case for menu-driven invocations); else default to `'unknown'`. Verify at plan time.

**Un-evict:** Owner manually edits `_char_owner.is_removed` back to `FALSE` in the cell before grace expires. No sidebar UI for un-evict in v1 per CONTEXT scope.

### Pattern 5: Weekly Schema Healthcheck (OPS-06) — Tab-by-ID

**What:** A weekly trigger (Sun 03:00 PT, alongside `monitorCellCount`) that verifies every expected tab exists by **sheet ID** (not name). On first run, the healthcheck records sheet IDs to `_meta.expected_sheet_ids` as JSON map `{tabName: sheetId}`. On every run, it walks the map and checks each `ss.getSheets().find(s => s.getSheetId() === id)` — if any are missing or renamed, write a structured `_meta.last_error` entry of kind `tab_missing`.

**Why tab-by-ID:** SUMMARY.md identifies tab-by-ID resilience as load-bearing (SCHEMA-06 mandates it). A user accidentally renaming `_meta` to `_meta_old` would not break the healthcheck if it tracks IDs.

**`_meta.last_error` envelope (same as Phase 3/4):**
```typescript
{
  at: ISO8601,
  where: 'weeklySchemaHealthcheck',
  kind: 'tab_missing' | 'tab_renamed' | 'sheet_id_drift',
  detail: 'Expected _char_owner (sheetId=12345) but found no sheet with that id. Recent rename?'
}
```

**Tray integration (per `code_context` in CONTEXT):** the watcher's heartbeat reads `_meta.last_error` — no additional integration needed in Phase 5.

**Expected sheet list** (derived from `scaffold.go` DimensionTabs + ViewTabs):
```
_meta, _char_owner, _item_master, _pigparse, _wiki_spells, _wiki_gear_tier,
_quest_items, _audit, _status, view, gear_check, spell_check, bank
```
(13 tabs. `_archive` is lazy-created — exclude from expected set.)

### Pattern 6: Stale-Char Auto-Archive (VIEW-05)

**What:** Weekly trigger (`weeklyStaleCharArchive`, Sun 06:00 PT) reads `_char_owner` and identifies chars with `last_seen > 90 days ago`. For each: call `archive.moveCharToArchive(charName)`.

**`archive.moveCharToArchive(charName)`:**
1. Acquire `LockService.getDocumentLock().tryLock(30000)`.
2. Lazy-create `_archive` tab if missing (hidden, headers: `archived_at | char_name | tab_name | row_count | uploaded_at | snapshot_json`).
3. Read `inv:<Char>` rows (snapshot via `getDataRange().getValues()`), pack to `snapshot_json`.
4. Append one `_archive` row per (char, tab_type) pair.
5. Mark `_char_owner.is_archived = TRUE` (new column — see §Schema impact below).
6. Optional: **delete** the `inv:<Char>` and `spell:<Char>` tabs. The watcher will re-create them if the char comes back (per WATCH-09 catch-up logic in Phase 2).
7. Release lock.

**Schema impact:** This is the only place in Phase 5 that touches schema. Two paths:

- **Path A (NO migration):** Don't add `is_archived` column. Use `is_removed=TRUE` for both eviction AND auto-archive. Distinguish in `_meta.eviction_log` (or a parallel `_meta.archive_log`) via a `reason: 'evicted' | 'stale_90d'` field. **RECOMMENDED.**
- **Path B (migration to v4):** Add `is_archived` to `_char_owner`. Requires `WatcherMaxSchemaVersion` bump in `internal/sheet/client.go` line 44 (currently `3`) BEFORE clasp push. Reissue watcher build, force all 12 guildies to upgrade. Not recommended unless audit-log fidelity becomes a requirement.

The planner should default to Path A.

**Watcher race condition mitigation:** See §Pitfall P6.

### Pattern 7: Jekyll GitHub Pages Onboarding Site

**What:** `/docs/` directory in the repo, picked up by Pages automatically when configured.

**File-by-file:**

```yaml
# /docs/_config.yml
remote_theme: pages-themes/cayman@v0.2.0
plugins:
  - jekyll-remote-theme
title: SquireBot
description: A small Windows app that streams your EverQuest inventory and spellbook into your guild's shared workbook.
show_downloads: false
```

```markdown
<!-- /docs/index.md -->
---
layout: default
---

# SquireBot

A small Windows app that streams your EverQuest inventory and spellbook into your guild's shared workbook.

[Install →]({{ "/install/" | relative_url }}) ·
[GitHub →](https://github.com/boejowen/SquireBot)
```

Repeat for `/install.md`, `/troubleshooting.md`, `/dev.md` per UI-SPEC §Onboarding Site Visual Contract.

**Enablement steps (one-time, owner action):**
1. Settings → Pages → Source: Deploy from a branch.
2. Branch: `main`, folder: `/docs`.
3. Save. URL appears after first build (~30s).
4. CNAME: not used (default `boejowen.github.io/SquireBot/` is fine).

**Asset budget:** PNG screenshots stay under 200 KB each (use `pngcrush` if needed); GIF stays under 5 MB per D-10.

### Anti-Patterns to Avoid

- **`SpreadsheetApp.getActive()` inside a loop** — every call is a fresh round-trip. Always hoist `const ss = getActiveSpreadsheet()` above any sheet iteration. (Already the convention — `buildView.ts:82` is the canonical example.)
- **Per-cell `setValue` in a write loop** — batch into a single `setValues(range, values)`. The healthcheck and archive paths both touch multiple rows; both must batch.
- **Calling `CacheService.put` with a value >100KB** — silent error. Always JSON.stringify and `length`-check before put; chunk if needed (we don't need to chunk at 12 chars).
- **Trusting `Session.getActiveUser().getEmail()` to identify the writer** — it returns the SCRIPT OWNER (CLAUDE.md, SUMMARY.md). Use `Session.getEffectiveUser()` instead, or accept the limitation.
- **Hand-rolling sidebar HTML inline `<script>` event handlers with `<div onclick>`** — UI-SPEC §Accessibility requires `<button>` for actions.
- **Putting `_archive` in the expected-tabs healthcheck** — it's lazy-created; flagging its absence as a tab_missing error would generate noise on every healthy workbook.
- **Re-protecting `_meta.bank_toon_name` with strict mode** — Phase 4 smoke proved the script owner is a default editor of strict protections, so they'd see no prompt. Always `setWarningOnly(true)` (migrations.ts:151).
- **Migrating to schema v4 for `_archive`** — see Pattern 6 Path A vs B; lazy creation avoids the version bump.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Sidebar form pattern | Custom client→server RPC | `google.script.run.withSuccessHandler(...).withFailureHandler(...).fn(args)` | Already proven Phase 4 (`showCharInfoSidebar.ts:152`); built-in error semantics |
| Per-`inv:Char` cache | Custom in-memory Map serialization | `CacheService.getDocumentCache().putAll({k:v,...}, 60)` | Built-in TTL, eviction, 100KB-per-value cap [VERIFIED: developers.google.com/apps-script/reference/cache/cache] |
| Document-wide write coordination | Custom retry-on-conflict loop | `LockService.getDocumentLock().tryLock(30000)` + try/finally | Already proven; no other Apps Script primitive solves shared-range contention |
| Tab existence check by name | `getSheetByName(...)` (renames break it) | `getSheetById(...)` after storing IDs in `_meta.expected_sheet_ids` | SCHEMA-06 mandates ID-resilience |
| Range protection | Sheets `setProtected` from Sheets API | `Range.protect().setWarningOnly(true)` (Apps Script) | Identical to Phase 4 `protectBankCoinCells`; warning-only is the only mode that nudges the script owner |
| Time-driven trigger registration | Polling | `ScriptApp.newTrigger(...).timeBased().onWeekDay(...).atHour(...).inTimezone('America/Los_Angeles').create()` | Already proven; PT (Pacific Time) is the project's frozen timezone |
| Inventory slot list | Scrape from data at scaffold time | Hardcode in `lib/eq-constants.ts` (next to `CLASSES` and `RACES`) | P99 slot vocabulary is stable; scraping adds load-time cost to the sidebar. Hardcoded list also testable. |
| Schema versioning | Custom version-compare | Existing `readMetaRowInt('_meta', 'schema_version')` + new `migrateToVN` if needed | Phase 4 froze the pattern; lazy `_archive` avoids needing a migration at all |
| Static site build | Custom HTML build pipeline | GitHub Pages built-in Jekyll | Zero local build infra; theme via `remote_theme` |
| Animated GIF authoring | Hand-frame editing | Record with ScreenToGif (Windows, open-source) | Standard FOSS tool; user can re-record without project skill |
| Levenshtein implementation | `fastest-levenshtein` npm package | Hand-rolled 25-line Wagner-Fischer DP function in `lib/searchIndex.ts` | No new external runtime deps; the perf delta is irrelevant at our scale (~2,000 items, ≤100ms either way) |

**Key insight:** Phase 5 is unusual in that every primitive it needs already exists either in Apps Script's built-ins or in the Phase 3/4 codebase. The entire phase is "wire existing pieces together with new triggers and new HTML templates." The only NEW external dependency is the Cayman theme via `remote_theme`, which is GitHub-hosted, GitHub-vetted, and trust-equivalent to a shadcn-official block per UI-SPEC §Registry Safety.

## Common Pitfalls

### P1: Cold-Path Cache Miss Blows the 2s Budget

**What goes wrong:** First search of the day reads every `inv:<Char>` tab from Sheets API + populates 12 cache entries. With 12 chars × ~300 rows, that's 12 round-trips at ~100-200ms each = 1.2-2.4s. Adding the substring scan + render pushes the cold path to 2-3s. SEARCH-02 requires <2s.

**Why it happens:** CacheService TTL is 60s; sidebar can sit closed for hours.

**How to avoid:**
1. **Pre-warm the cache from `onChange`.** `onChange` already fires on every watcher write. After its existing debounce, kick off `prewarmSearchCache()` that does the project-and-cache pass for any `inv:*` tab not currently cached. This keeps the cache hot during any guildie's active session.
2. **Pre-warm from `installTriggers`.** Call `prewarmSearchCache()` as the LAST step of `installTriggers`. Workbook owners running "Install Triggers" get a one-time warm-up.
3. **Document the cold-path UX:** The first search after a cold start MAY take up to ~3s. CONTEXT.md D-03 explicitly anticipates this ("First search of the day takes ~3s cold; subsequent searches ~200ms").

**Warning signs:** Sidebar loading-state stays >2s on a hot workbook (means pre-warm isn't firing); search consistently >2s after first call (means cache isn't being repopulated on miss).

### P2: 100KB-Per-Value Cap Violation

**What goes wrong:** `CacheService.put(key, value)` silently fails (throws) if `value` exceeds 100KB. A char with an exceptionally large inventory (200 rows × 200-char names) could approach the cap.

**Why it happens:** EQ inventory uses long compound item names; one Velious tier set has names up to 60 chars.

**How to avoid:**
1. Compact JSON shape: `CachedInvRow = [Location, Name, ID, Count]` (4 fields, no keys). At ~50 bytes per row, 200 rows = 10 KB. Safe margin.
2. Add a length check before `put`: `if (JSON.stringify(rows).length > 95_000) { log('warn', ...); throw }` — defensive guard so a bad row doesn't silently break search.
3. If a char ever genuinely exceeds 100KB, fall back to chunked-cache (`squirebot:search:inv:<Char>:0`, `:1`, ...). Defer chunking implementation until the warn-log fires; don't pre-build complexity for a hypothetical.

**Warning signs:** Cache put throws `Argument too large`; logs show "skipping cache write for `<Char>`."

### P3: No Push Invalidation from Watcher Writes

**What goes wrong:** Watcher writes a fresh inventory to `inv:Findom` at 14:00:00. Sidebar searches at 14:00:30. Result rows reflect the PRE-write state because the cache entry for Findom was written at 13:59:50 and won't expire until 14:00:50.

**Why it happens:** Watcher (Go) has no in-process mechanism to call into Apps Script's CacheService. They communicate only via cell writes.

**How to avoid:**
1. **Accept it.** 60s TTL is the worst-case freshness. CONTEXT.md `code_context` explicitly says: *"Watcher cache invalidation... invalidation has to be lazy (next search rebuilds that char's cache entry by virtue of TTL expiry). 60s TTL is the actual freshness guarantee, NOT cache-bust-on-write."*
2. **`onChange` reactive invalidation:** Workbook `onChange` already fires on watcher writes. The same `prewarmSearchCache()` from Pitfall P1 should invalidate-then-repopulate the affected char's entry on `onChange`. The debounce (10s) means up to ~10s lag rather than 60s.

**Warning signs:** Guildie reports "search shows item I just deleted." Confirm by checking cache TTL (≤60s after most recent watcher write).

### P4: Range.protect Lost on Schema Migration

**What goes wrong:** Protections applied during `migrateToV3` are tied to a specific cell. If a future migration appends columns or moves rows, the cell coordinates shift and the protection may or may not follow.

**Why it happens:** Sheets protections track ranges by coordinate, not by `_meta` key.

**How to avoid:**
1. **Re-apply protections on every `installTriggers` run.** `protectBankCoinCells` (existing) and a NEW `protectBankToonName` (Phase 5) are both called from `installTriggers`. Idempotent — `getProtections` filters by description, skips already-protected. (See `migrations.ts:138-153`.)
2. **Never delete a `_meta` row protection in a migration.** Append-only mutations preserve cell coordinates.

**Warning signs:** Direct-edit warning prompt stops appearing on `_meta.bank_toon_name` or `_meta.bank_coin_*`. Re-run `installTriggers` to re-protect.

### P5: CONTEXT.md "36k rows" Miscount

**What goes wrong:** CONTEXT.md `additional_context` block describes ~36,000 rows ("~12 guildies × ~10 chars × ~300 inv rows ≈ 36k rows"). Reality is ~12 `inv:*` tabs total (the watcher writes one `inv:<CharName>` per CHARACTER, not per guildie), and the guild has ~12 distinct characters across ~12 guildies, not 12 chars per guildie.

**Why it happens:** Conflating "guildies" with "characters." `_char_owner` is per-character; `inv:*` tabs are per-character. The "10 chars per guildie" assumption is wrong — most guildies have 1-3 P99 characters, not 10.

**How to avoid:**
1. Verify char count from `_char_owner` at plan time. Performance budget assumptions should use the actual count, not the CONTEXT.md figure.
2. Even at the worst-case interpretation (12 × 10 = 120 chars × 300 rows = 36,000 rows), this is still ~1.8 MB of JSON across 120 cache entries — well under the 10 MB total CacheService cap. The math holds even on the high estimate.
3. Plan should set the performance check at "real workbook char count × ~300 rows" and validate during smoke. The pre-warm approach (Pitfall P1) covers both interpretations.

[VERIFIED: codebase grep — `inv:` tabs are per-character, see scaffold.go and showCharInfoSidebar.ts:166-167]

### P6: Watcher Race During Auto-Archive

**What goes wrong:** `weeklyStaleCharArchive` is running for `inv:Findom` (reading the tab + appending to `_archive` + deleting the tab) at the exact moment Findom's watcher fires a `batchUpdate` clear+write to `inv:Findom`. Two outcomes:
- Apps Script deletes the tab between watcher's clear and watcher's write — watcher errors with "sheet not found."
- Watcher writes new data after Apps Script's snapshot read, then Apps Script deletes the tab — fresh data is lost.

**Why it happens:** No coordination between Go watcher and Apps Script weekly triggers.

**How to avoid:**
1. **Strict pre-condition for archive:** Only archive if `_char_owner.last_seen` (the heartbeat-driven timestamp) is older than 90 days. The watcher's heartbeat fires every 24h regardless of file changes (WATCH-08 + OPS-05) — if `last_seen` is >90 days, the watcher is OFF or the char is dormant. A live watcher cannot have `last_seen > 90d`.
2. **Read `last_seen` INSIDE the lock:** Acquire `LockService.getDocumentLock().tryLock(30000)`, then read `last_seen`, then verify >90d, then archive. The lock prevents the watcher's heartbeat from updating `last_seen` mid-operation.
3. **Watcher does NOT acquire the lock for `batchUpdate`** (this is correct — per OPS-01 watchers write per-character non-overlapping ranges, no lock needed). But the heartbeat write to `_char_owner.last_seen` DOES need to interact with the lock if the watcher rejoins mid-archive. Verify at plan time: does the heartbeat path call the same `LockService` API? If not, accept the worst-case (one missed archive cycle — the char gets archived next week instead).

**Warning signs:** `weeklyStaleCharArchive` logs "sheet not found" or appends archive rows with `row_count: 0`. Watcher logs `SheetNotFound` on a previously working tab.

### P7: `getSheetByName` Returns null After User Rename

**What goes wrong:** A workbook owner renames `_meta` to `_meta_old` while debugging. Every healthcheck call to `ss.getSheetByName('_meta')` returns null. `monitorCellCount` and `weeklySchemaHealthcheck` both fail silently.

**Why it happens:** Sheets allows any user with edit access to rename any tab.

**How to avoid:**
1. **Healthcheck uses `getSheetId()`, not name** (Pattern 5). Store `{tabName: sheetId}` once in `_meta.expected_sheet_ids`. Iterate `ss.getSheets()` and check ID membership.
2. **Existing code uses `getSheetByName`** (Phase 2/3/4). The Phase 5 healthcheck explicitly checks by ID; existing builders can continue using names (they'd fail explicitly with "view sheet missing" log — Phase 3 buildView.ts:84 — rather than silently). Don't retrofit existing builders in Phase 5 (out of scope).

**Warning signs:** `_meta.last_error` shows `tab_missing` for `_meta` (impossible since healthcheck would itself fail to write — see Pitfall P8); `monitorCellCount` writes show 0 cells for an existing tab.

### P8: `_meta.last_error` Write Path Depends on `_meta` Existing

**What goes wrong:** Healthcheck detects that `_meta` itself is missing or renamed. It tries to write the error to `_meta.last_error`. The write fails because `_meta` doesn't exist.

**Why it happens:** Circular dependency.

**How to avoid:**
1. **Healthcheck writes to BOTH `_meta.last_error` AND `_status.last_error`** (same pattern as `monitorCellCount` — see `monitorCellCount.ts:47-48`). If `_meta` is gone, `_status` is usually still there; vice-versa.
2. **If both are gone, log to Apps Script logger.** The watcher won't see it (tray won't go red), but the script-editor execution log will. Plan should document that "if BOTH `_meta` and `_status` are deleted, the workbook is unrecoverable — restore from Sheets version history."

### P9: GIF Asset Bloats the Repo

**What goes wrong:** A 30-second SmartScreen walkthrough at full-screen recording resolution can easily exceed 10 MB. Every clone of the repo carries it forever.

**Why it happens:** Default ScreenToGif settings produce uncompressed output.

**How to avoid:**
1. **Budget: ≤5 MB** (CONTEXT D-10). Plan should specify recording at ≤720p, ≤15 fps, with single-color-replacement compression in ScreenToGif's encoder settings.
2. **Fallback option (only if 5 MB is impossible):** Upload to YouTube unlisted (Phase 5 deferred list keeps this as backup). Update UI-SPEC §Image format if invoked.
3. **Verify after creation:** `ls -la docs/assets/smartscreen.gif` — must be ≤5,242,880 bytes (5 MB binary).

**Warning signs:** Repo clone time noticeably increases; GitHub renders the GIF inline on the install.md page with a "this file is large" badge.

### P10: Jekyll Build Failure on First Pages Enable

**What goes wrong:** First push after enabling Pages fails the GitHub Actions Pages build with a Jekyll error (missing `_config.yml`, unsupported plugin, mis-quoted YAML).

**Why it happens:** GitHub Pages runs Jekyll in safe mode — only an allowlist of plugins is permitted.

**How to avoid:**
1. **Use ONLY GitHub-allowed plugins:** `jekyll-remote-theme`, `jekyll-relative-links`, `jekyll-default-layout`, `jekyll-titles-from-headings` are all allowed [CITED: pages.github.com plugins list].
2. **Local validation (optional):** Install `bundler` + `gem install github-pages` + `bundle exec jekyll serve` to preview locally before push.
3. **Watch the Actions tab:** First Pages build emits errors in the "pages-build-deployment" workflow. Fix before guildies hit the URL.

### P11: Schema Migration Required for `_archive` Tab — Avoid It

**What goes wrong:** Planner proposes scaffolding `_archive` as a v4 schema tab. This requires:
- Bumping `internal/sheet/client.go:44` `WatcherMaxSchemaVersion = 3` → `4`.
- Building + releasing a new watcher version.
- Force-updating every guildie BEFORE the migration runs (otherwise `ErrSchemaTooNew`).
- Writing `migrateToV4()` that adds `_archive`.

**Why it happens:** Tempting to fold `_archive` into the "official" schema.

**How to avoid:**
1. **Lazy-create `_archive` on first eviction or first stale-archive call.** No schema bump; no watcher rebuild. `_archive` is invisible to the watcher (it doesn't write to it).
2. CONTEXT.md `code_context` explicitly recommends lazy creation: *"create lazily on first eviction (no schema bump). Lazy creation is preferred — avoids bumping watcher version for a feature most workbooks won't use immediately."*
3. The healthcheck (Pattern 5) MUST exclude `_archive` from its expected set so absence isn't flagged as `tab_missing`.

### P12: Tab Hide is Reversible by Any Editor

**What goes wrong:** Workbook owner hides `_meta`. A guildie with edit access right-clicks the bottom tab strip → "Unhide" → re-shows it. Now `_meta` is visible to everyone.

**Why it happens:** `Sheet.hideSheet()` is a structural property of the sheet, not a permission. Any editor can flip it.

**How to avoid:**
1. **`hideAllSystemTabs()` runs every time `installTriggers` is invoked** — idempotent. Owner running "Install Triggers" re-hides any user-unhidden tabs.
2. **`Range.protect()` is orthogonal to hiding.** Protection guards CELL EDITS; hiding guards VISIBILITY. Both are needed: hide for clutter, protect for write-safety. Phase 5 needs BOTH on `_meta.bank_toon_name`.
3. **Document the limitation in `/troubleshooting`:** "If `_meta` tabs reappear, run SquireBot → Install Triggers — that re-hides all `_*` tabs."

## Code Examples

### Example 1: showSearchSidebar entry point (theme-aware)

```typescript
// Source: derived from apps-script/src/triggers/showCharInfoSidebar.ts:41-46
// Apps Script HtmlService docs: developers.google.com/apps-script/guides/html
import { HtmlService } from '...'; // implicit Apps Script global
import { getActiveTheme, THEMES, Theme } from '../lib/themes';

export function showSearchSidebar(): void {
  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];  // null for 'sheets-default'
  const html = HtmlService
    .createHtmlOutput(buildSidebarHtml(theme))
    .setTitle('SquireBot — Search')
    .setWidth(300);  // UI-SPEC §Spacing locks 300px
  SpreadsheetApp.getUi().showSidebar(html);
}
```

### Example 2: runSearch server-side (Pattern 2)

```typescript
// Source: Pattern derived from buildView.ts:162-187 (inv:* iteration) and
// CacheService docs: developers.google.com/apps-script/reference/cache/cache
import { log } from '../lib/log';

interface SearchResultRow {
  itemName: string;
  itemId: number;
  char: string;
  location: string;
  count: number;
  wikiUrl: string;
  wikiSummary: string;
  pricePp: number | null;
}

export function runSearch(
  query: string,
  charFilter: string,
  slotFilter: string,
): SearchResultRow[] {
  const q = (query || '').toLowerCase().trim();
  if (!q) return [];

  const cache = CacheService.getDocumentCache();
  if (!cache) throw new Error('CacheService unavailable');

  // 1. Discover candidate chars from charFilter (or _char_owner).
  const ss = SpreadsheetApp.getActiveSpreadsheet();
  const allChars = ss.getSheets()
    .filter((s) => s.getName().startsWith('inv:'))
    .map((s) => s.getName().slice(4));
  const targetChars = charFilter && charFilter !== 'any'
    ? allChars.filter((c) => c === charFilter)
    : allChars;

  // 2. Bulk-read cache (single round-trip).
  const cacheKeys = targetChars.map((c) => `squirebot:search:inv:${c}`);
  const cached = cache.getAll(cacheKeys);  // {key: json-string}

  // 3. Cold-fill any misses.
  for (const char of targetChars) {
    const key = `squirebot:search:inv:${char}`;
    if (cached[key]) continue;
    const sheet = ss.getSheetByName(`inv:${char}`);
    if (!sheet) continue;
    const lastRow = sheet.getLastRow();
    if (lastRow < 2) { cached[key] = '[]'; continue; }
    const values = sheet.getRange(2, 1, lastRow - 1, 5).getValues();
    const compact = values.map((r) => [
      String(r[0] ?? ''),    // Location
      String(r[1] ?? ''),    // Name
      Number(r[2]) || 0,     // ID
      Number(r[3]) || 0,     // Count
    ]);
    const json = JSON.stringify(compact);
    if (json.length < 95_000) {
      cache.put(key, json, 60);  // 60s TTL per SEARCH-04
    } else {
      log('warn', 'runSearch', { skipCachePut: char, bytes: json.length });
    }
    cached[key] = json;
  }

  // 4. Scan in-memory.
  const slotFilterUpper = slotFilter && slotFilter !== 'any'
    ? slotFilter.toUpperCase()
    : null;
  const matches: SearchResultRow[] = [];
  for (const char of targetChars) {
    const rows = JSON.parse(cached[`squirebot:search:inv:${char}`] ?? '[]') as [string,string,number,number][];
    for (const [loc, name, id, count] of rows) {
      if (slotFilterUpper && !loc.toUpperCase().startsWith(slotFilterUpper)) continue;
      if (!name.toLowerCase().includes(q)) continue;
      matches.push({ itemName: name, itemId: id, char, location: loc, count, wikiUrl: '', wikiSummary: '', pricePp: null });
    }
  }

  // 5. Enrich with _item_master + _pigparse (cached separately under 60s TTL).
  enrichResults(matches, cache, ss);

  // 6. Group + sort per D-06.
  return groupByItemNameThenChar(matches);
}
```

### Example 3: Did-you-mean fuzzy fallback (Pattern 3)

```typescript
// Source: standard Wagner-Fischer DP Levenshtein.
// Reference impl for correctness verification: github.com/ka-weihe/fastest-levenshtein
export function levenshtein(a: string, b: string): number {
  if (a === b) return 0;
  if (a.length === 0) return b.length;
  if (b.length === 0) return a.length;
  let prev = new Array(b.length + 1);
  for (let j = 0; j <= b.length; j++) prev[j] = j;
  for (let i = 1; i <= a.length; i++) {
    const curr = new Array(b.length + 1);
    curr[0] = i;
    for (let j = 1; j <= b.length; j++) {
      const cost = a.charCodeAt(i - 1) === b.charCodeAt(j - 1) ? 0 : 1;
      curr[j] = Math.min(curr[j - 1] + 1, prev[j] + 1, prev[j - 1] + cost);
    }
    prev = curr;
  }
  return prev[b.length];
}

export function didYouMean(query: string, itemNames: string[]): string[] {
  const q = query.toLowerCase();
  return itemNames
    .map((n) => ({ n, d: levenshtein(q, n.toLowerCase()) }))
    .filter((x) => x.d <= 2 && x.d > 0)
    .sort((a, b) => a.d - b.d)
    .slice(0, 3)
    .map((x) => x.n);
}
```

### Example 4: weeklySchemaHealthcheck (Pattern 5)

```typescript
// Source: shape derived from monitorCellCount.ts (same trigger family).
import { log } from '../lib/log';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from '../lib/sheet-helpers';

const EXPECTED_TABS = [
  '_meta', '_char_owner', '_item_master', '_pigparse',
  '_wiki_spells', '_wiki_gear_tier', '_quest_items', '_audit', '_status',
  'view', 'gear_check', 'spell_check', 'bank',
];
// _archive intentionally excluded — lazy-created.

export function weeklySchemaHealthcheck(): void {
  const ss = getActiveSpreadsheet();
  const allSheets = ss.getSheets();
  const sheetsById = new Map(allSheets.map((s) => [s.getSheetId(), s]));
  const sheetsByName = new Map(allSheets.map((s) => [s.getName(), s]));

  // 1. Load expected-ids snapshot from _meta (lazily populated).
  const meta = readMetaRows('_meta');
  const idsJsonRow = meta.find((r) => r.key === 'expected_sheet_ids');
  const expectedIds: Record<string, number> = idsJsonRow
    ? JSON.parse(idsJsonRow.value || '{}')
    : {};

  // 2. First-run: backfill IDs from current sheets.
  if (Object.keys(expectedIds).length === 0) {
    for (const name of EXPECTED_TABS) {
      const s = sheetsByName.get(name);
      if (s) expectedIds[name] = s.getSheetId();
    }
    writeMetaRow('_meta', 'expected_sheet_ids', JSON.stringify(expectedIds));
    log('info', 'weeklySchemaHealthcheck', { backfilled: Object.keys(expectedIds).length });
  }

  // 3. Verify.
  const missing: string[] = [];
  for (const name of EXPECTED_TABS) {
    const id = expectedIds[name];
    if (id == null) { missing.push(name); continue; }
    if (!sheetsById.has(id)) missing.push(name);
  }

  if (missing.length === 0) {
    writeMetaRow('_status', 'last_schema_check', new Date().toISOString());
    writeMetaRow('_status', 'last_schema_check_status', 'ok');
    log('info', 'weeklySchemaHealthcheck', { ok: true, checked: EXPECTED_TABS.length });
    return;
  }

  const err = {
    at: new Date().toISOString(),
    where: 'weeklySchemaHealthcheck',
    kind: 'tab_missing',
    detail: missing.join(','),
  };
  const errJson = JSON.stringify(err);
  writeMetaRow('_meta', 'last_error', errJson);
  writeMetaRow('_status', 'last_error', errJson);
  log('warn', 'weeklySchemaHealthcheck', { missing });
}
```

### Example 5: commitEviction (Pattern 4)

```typescript
// Source: shape derived from showCharInfoSidebar.ts:72-127 (cascade write pattern).
import { log } from '../lib/log';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from '../lib/sheet-helpers';

const CHAR_OWNER = '_char_owner';
const COL_CHAR_NAME = 1;
const COL_OWNER_EMAIL = 2;
const COL_IS_REMOVED = 9;
const COL_COUNT = 14;

export function commitEviction(email: string): { affected: number; graceUntil: string } {
  if (!email || typeof email !== 'string') throw new Error('commitEviction: invalid email');
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(CHAR_OWNER);
  if (!sheet) throw new Error('_char_owner missing — run migrateToV3 first');

  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) throw new Error('commitEviction: lock_busy');
  try {
    const lastRow = sheet.getLastRow();
    const values = sheet.getRange(1, 1, lastRow, COL_COUNT).getValues();
    const affectedChars: string[] = [];
    for (let i = 1; i < values.length; i++) {
      if (String(values[i][COL_OWNER_EMAIL - 1] ?? '').trim() === email) {
        sheet.getRange(i + 1, COL_IS_REMOVED).setValue(true);
        affectedChars.push(String(values[i][COL_CHAR_NAME - 1] ?? '').trim());
      }
    }

    const graceMs = Date.now() + 30 * 24 * 60 * 60 * 1000;
    const graceUntil = new Date(graceMs).toISOString();
    const entry = {
      at: new Date().toISOString(),
      email,
      initiated_by: Session.getEffectiveUser().getEmail() || 'unknown',
      grace_until: graceUntil,
      chars: affectedChars,
      reason: 'evicted',
    };
    // Append to _meta.eviction_log (JSON array).
    const meta = readMetaRows('_meta');
    const row = meta.find((r) => r.key === 'eviction_log');
    const list = row ? JSON.parse(row.value || '[]') : [];
    list.push(entry);
    writeMetaRow('_meta', 'eviction_log', JSON.stringify(list));

    log('info', 'commitEviction', { email, affected: affectedChars.length, graceUntil });
    return { affected: affectedChars.length, graceUntil };
  } finally {
    lock.releaseLock();
  }
}
```

### Example 6: hideAllSystemTabs + protectBankToonName (in migrations.ts)

```typescript
// Source: pattern from migrations.ts:125-155 (protectBankCoinCells).
export function hideAllSystemTabs(): void {
  const ss = getActiveSpreadsheet();
  let hidden = 0;
  for (const sheet of ss.getSheets()) {
    if (sheet.getName().startsWith('_') && !sheet.isSheetHidden()) {
      sheet.hideSheet();
      hidden++;
    }
  }
  log('info', 'hideAllSystemTabs', { hidden });
}

const BANK_TOON_NAME_DESC = 'SquireBot bank toon name — edit via SquireBot menu';

export function protectBankToonName(): void {
  const sheet = getActiveSpreadsheet().getSheetByName('_meta');
  if (!sheet) return;
  const meta = readMetaRows('_meta');
  const row = meta.find((r) => r.key === 'bank_toon_name');
  if (!row) return;  // not yet created — installTriggers re-runs will pick it up
  const cell = sheet.getRange(row.rowIndex, 2);
  const cellA1 = cell.getA1Notation();
  const existing = sheet.getProtections(SpreadsheetApp.ProtectionType.RANGE)
    .find((p) => p.getRange().getA1Notation() === cellA1
              && p.getDescription() === BANK_TOON_NAME_DESC);
  if (existing) return;
  cell.protect().setDescription(BANK_TOON_NAME_DESC).setWarningOnly(true);
  log('info', 'protectBankToonName', { protected: 'bank_toon_name' });
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Clasp 1.x with bundled TS transpiler | Clasp 2.4+ + local esbuild | Clasp 2.x removed bundled TS support | LOCKED by CLAUDE.md; Phase 5 doesn't change this |
| Clasp 2.x | Clasp 3.x | 2025 | **Reject** for SquireBot — CLAUDE.md explicitly forbids 3.x due to breaking changes (Phase 3 RESEARCH §6) |
| Apps Script Rhino runtime | V8 runtime | Rhino EOL 2026-01-31 | LOCKED at Phase 1 |
| Per-character view tabs | Consolidated mega-tabs | Phase 2 (SCHEMA-04) | LOCKED; would breach 200-tab limit |
| EV cert instant SmartScreen reputation | EV no longer instant (March 2024) | Microsoft removed perk March 2024 | Locked decision — unsigned + walkthrough + SignPath OSS in parallel. SmartScreen walkthrough is the Phase 5 onboarding focus. [VERIFIED: MEMORY.md feedback note] |
| `oob` OAuth flow | Loopback PKCE | Phase 1 (Pitfall #1) | LOCKED |
| Apps Script CDN-bundled libraries | Inline `<style>` / `<script>` in HtmlService | LOCKED by CLAUDE.md / CSP | Phase 5 follows |

**Deprecated/outdated for Phase 5 specifically:**
- **Per-row staleness in search results** (SEARCH-03 original wording) — DROPPED per user during discuss-phase. Path 2 chosen.
- **EV cert pursuit for installer trust** — defer indefinitely per MEMORY.md note. Phase 5 onboarding doc assumes unsigned + walkthrough path.
- **Clasp 3.x** — FORBIDDEN.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Watcher's heartbeat write to `_char_owner.last_seen` uses the same workbook-level coordination as Apps Script's LockService (or accepts one missed cycle) | §Pitfall P6 | Race could lose ONE archive cycle (low — char re-archives next week). Verify at plan time whether `internal/heartbeat/heartbeat.go` interacts with any locking primitive. |
| A2 | EQ inventory `Location` field starts with the slot prefix (e.g., `HEAD-xxx`, `General-1`) so a `startsWith(slotPrefix)` slot filter works | §Pattern 2 (Filter application) | Slot filter may miss-match for non-prefixed Location values. Plan should probe real `inv:*` data and adjust if prefix model wrong. |
| A3 | Cayman theme works correctly via `remote_theme: pages-themes/cayman@v0.2.0` with current GitHub Pages Jekyll allowed-plugins list | §Pattern 7 | First Pages build could fail (P10). Validate with a sandbox push before locking. [CITED: github.com/pages-themes/cayman] |
| A4 | A 30-second SmartScreen walkthrough can fit in 5 MB GIF at 720p/15fps | §Pitfall P9 | If not, fallback to YouTube unlisted per CONTEXT.md deferred list. |
| A5 | `Session.getEffectiveUser().getEmail()` returns the workbook owner who clicked the menu, not the script owner, when the sidebar runs as the user | §Pattern 4, §Code Example 5 | If it returns the script owner, `initiated_by` in `eviction_log` will be wrong. CLAUDE.md notes `getActiveUser()` returns owner; `getEffectiveUser()` is per Google docs supposed to return the user, but behavior depends on the script's execution context (installable trigger vs simple trigger vs UI). Plan-time check: log both values during smoke. |
| A6 | The guild has roughly 12 characters total (one per guildie), not 12 × 10 | §Pitfall P5 | Performance math holds either way (Pitfall P5 explains). |
| A7 | `Sheet.hideSheet()` survives a workbook copy / template-copy | §Pattern 6 | If not, copies of the workbook would expose `_*` tabs. Mitigation: `hideAllSystemTabs()` runs on every `installTriggers`. Low risk. |
| A8 | The watcher's `WatcherMaxSchemaVersion = 3` constant does NOT need to bump for Phase 5 if `_archive` is lazy-created (Path A) | §Pattern 6 schema impact | If planner picks Path B (schema-v4 migration), watcher must rebuild. Plan should default to Path A. |

**If a downstream agent (discuss / plan) sees an assumption it can verify cheaply, verify it. Most are low-risk and verifiable during smoke.**

## Open Questions

1. **Should `Session.getEffectiveUser()` or some other identity mechanism populate `_meta.eviction_log.initiated_by`?**
   - What we know: `Session.getActiveUser()` returns script owner per CLAUDE.md.
   - What's unclear: behavior of `getEffectiveUser()` in a sidebar context.
   - Recommendation: log both during smoke; pick whichever returns the workbook owner. Worst case, accept `'unknown'` for v1 — `_meta.eviction_log.at` and `email` are the load-bearing fields anyway.

2. **Should the slot-filter dropdown enumerate real `Location` values at scaffold-time or hardcode P99 slots?**
   - What we know: CONTEXT.md leaves this to Claude's discretion.
   - What's unclear: whether `Location` is strictly slot-prefixed or includes non-slot inventory (e.g., "General-1", "Bank-12").
   - Recommendation: Hardcode the P99 slot vocabulary in `lib/eq-constants.ts` (next to `CLASSES` / `RACES`). Add `'Any inventory location'` as the default. Probe `inv:*` data during plan to confirm `Location` prefix model.

3. **Does `weeklyStaleCharArchive` archive the `_char_owner` row, or just the `inv:<Char>`/`spell:<Char>` data?**
   - What we know: VIEW-05 says "auto-archived to a hidden `_archive` tab" but doesn't specify which data.
   - What's unclear: After archive, does the row stay in `_char_owner` (with `is_removed=TRUE`) or move to `_archive`?
   - Recommendation: Keep the row in `_char_owner` with `is_removed=TRUE` AND `reason: 'stale_90d'` field in `_meta.archive_log` (parallel to `eviction_log`). Move only the bulky `inv:<Char>` snapshot to `_archive`. Preserves cross-references and lets `un-archive` work via cell edit.

4. **Should the search sidebar's recent-history persist across workbook sessions, or just within a session?**
   - What we know: D-08 says "rolling window of 3" via CacheService.
   - What's unclear: CacheService TTL is 25 min max (it's not a persistent store). Across multi-day sessions, history will reset.
   - Recommendation: For v1, accept reset on cache expiry. If persistent history is desired later, switch to `PropertiesService.getDocumentProperties()` (workbook-scoped, persists forever, 9KB cap — fits 3 query strings easily). Plan note this as a v1.0.1 polish item if the user reports it.

5. **Should `installTriggers` re-pre-warm the search cache, or only `onChange`?**
   - What we know: Pre-warm is needed to hit <2s cold-path (Pitfall P1).
   - What's unclear: Pre-warming all 12 chars from `installTriggers` could add ~3s to the menu-action UX.
   - Recommendation: Pre-warm only from `onChange` (asynchronous, user doesn't wait). `installTriggers` includes a one-line `prewarmSearchCache()` call too, but make it async-fire-and-forget if Apps Script supports that (it doesn't — Apps Script is synchronous). Accept the 3s delay during `installTriggers`; print "Pre-warming search cache…" in the alert dialog.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Apps Script TS build (esbuild + clasp) | ✓ (already in use Phase 3+) | (any LTS) | — |
| `@google/clasp` 2.5.0 | Push to Apps Script | ✓ | 2.5.0 verified npm | — |
| `esbuild` 0.28.0 | TS → IIFE bundle | ✓ | 0.28.0 verified npm | — |
| `vitest` | Server-side unit tests | ✓ (already used Phase 3+) | — | — |
| Google Apps Script V8 runtime | Hosts all sidebar + trigger code | ✓ (workspace) | V8 | — |
| `HtmlService` | Sidebar rendering | ✓ (Apps Script built-in) | — | — |
| `CacheService.getDocumentCache()` | Per-char search index + recent history | ✓ (Apps Script built-in) | — | — |
| `LockService.getDocumentLock()` | Sidebar write coordination | ✓ (Apps Script built-in) | — | — |
| `ScriptApp.newTrigger().timeBased()` | 3 new weekly triggers | ✓ (Apps Script built-in) | — | — |
| `Range.protect()` | `_meta.bank_toon_name` protection | ✓ (Apps Script built-in) | — | — |
| `Sheet.hideSheet()` | `hideAllSystemTabs` | ✓ (Apps Script built-in) | — | — |
| GitHub Pages | Onboarding site host | ✓ (repo public; Pages enablement is owner action) | — | Self-host on `boejowen.github.io` Pages root |
| Jekyll Cayman theme | Site theme | ✓ (`remote_theme: pages-themes/cayman@v0.2.0`) | v0.2.0 | Slate theme (UI-SPEC permits) |
| `jekyll-remote-theme` plugin | Required for `remote_theme` directive | ✓ (GitHub Pages allowlist) | — | Vendor the Cayman gem locally (heavier) |
| ScreenToGif (Windows) | Recording SmartScreen GIF | ✗ (not verified on dev box; user installs themselves per MEMORY.md) | — | LICEcap, ShareX (alternatives); or fall back to YouTube unlisted (CONTEXT deferred) |
| `pngcrush` or equivalent | PNG screenshot optimization | ✗ (optional) | — | Manually crop / resize in OS image viewer |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:**
- ScreenToGif → use any equivalent GIF recorder, or fall back to YouTube unlisted per CONTEXT D-10 deferred path.
- `pngcrush` → optional; large PNGs just consume more repo space.

## Security Domain

(`security_enforcement: true` in `.planning/config.json`; ASVS Level 1.)

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | partial | Owned by OAuth flow (Phase 1) — Phase 5 does not change authentication |
| V3 Session Management | no | Sidebar runs in the user's authenticated Sheets session; no separate session boundary |
| V4 Access Control | yes | Workbook share = the access boundary. `Range.protect()` warning-only on `_meta.bank_toon_name` (defense-in-depth). Eviction sidebar should refuse if invoking user has read-only workbook access — but Apps Script can't easily tell. Accept that any editor can run any menu item. |
| V5 Input Validation | yes | Sidebar inputs: search query (free text — sanitize for HTML rendering), char filter (must match an `inv:*` tab name), slot filter (must match enum), email (eviction sidebar — must match `_char_owner.owner_email` distinct list). Use `escapeHtml` helper from `showCharInfoSidebar.ts:154`. |
| V6 Cryptography | no | No crypto operations in Phase 5 |

### Known Threat Patterns for HtmlService + Apps Script

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via item-name injection | Tampering | `escapeHtml(s)` on every interpolation from sheet data into sidebar HTML. Item names come from `inv:*` (watcher-written, from EQ output files — user-controlled in theory if a player names a custom item, but P99 doesn't allow arbitrary item creation). Defense-in-depth: escape unconditionally. [VERIFIED: showCharInfoSidebar.ts:154-156] |
| Search query reflected XSS | Tampering | Escape `query` when rendering "No matches for `<query>`" heading. Use `textContent` not `innerHTML` for query echo. |
| Eviction misfire (wrong email) | Tampering, Repudiation | Two-step confirmation: dropdown pick → preview affected chars → `confirm()` modal with explicit copy → server-side write. Plus audit trail in `_meta.eviction_log`. |
| Cache poisoning (one user pollutes another's search cache) | Tampering | `CacheService.getDocumentCache()` is workbook-scoped — all users share the same cache. Not a threat in a 12-person trusted-guild model, but documented. If a malicious user wrote garbage into an `inv:*` tab, search results would surface garbage — but they would have already written garbage into the workbook, which is a bigger issue. |
| Tab deletion DOS | DoS | `_archive` lazy creation + healthcheck-by-ID + `hideAllSystemTabs` defense-in-depth. User who deletes `_meta` can break the workbook; restore from Sheets version history (documented in `/troubleshooting`). |
| Eviction permission elevation | EoP | Currently any editor can run eviction. Self-service evict is deferred to v2 with note "threat model: departing guildie could spite-nuke data" (CONTEXT). For v1, the owner-only sidebar model + audit log + 30-day grace allows recovery via cell-edit un-evict. |
| GIF / PNG content | Spoofing | Onboarding site assets are static; no user-uploaded content. Repo-committed by owner. |

**Phase 5 security stance:** The trusted-guild model (12 people who know each other) means the threat surface is small. Defense-in-depth via `escapeHtml`, audit logs, and grace periods is sufficient. No new authentication primitives in scope.

## Validation Architecture

`workflow.nyquist_validation` is `false` in `.planning/config.json`. Section retained for planner reference but no Nyquist gates apply.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | vitest (server-side TypeScript only) |
| Config file | `apps-script/vitest.config.ts` (existing) |
| Quick run command | `cd apps-script && npm test -- searchIndex` (or other test name) |
| Full suite command | `cd apps-script && npm test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEARCH-01 | Sidebar opens from menu, 300px width | manual / smoke | Open Sheets → SquireBot → Search… | ❌ Wave 0 (smoke only) |
| SEARCH-02 | Cross-tab search under 2s warm-path | unit (search logic) + manual (full latency) | `npm test -- searchIndex` + manual | ❌ Wave 0 |
| SEARCH-04 | CacheService 60s TTL | unit (cache mock with TTL respect) | `npm test -- searchIndex` | ❌ Wave 0 — needs richer CacheService mock per CONTEXT |
| TIP-04 | Wiki summary tooltip on item name | manual / smoke | Open sidebar → hover item name | manual only |
| VIEW-05 | `inventory_mtime > 90d` → `_archive` | unit (mock 91-day-old `last_seen` row) | `npm test -- archive` | ❌ Wave 0 |
| OPS-06 | Weekly healthcheck writes `_meta.last_error` on missing tab | unit | `npm test -- weeklySchemaHealthcheck` | ❌ Wave 0 |
| DOC-01 | Install doc complete (5 steps + SmartScreen + tray-red) | manual / inspection | open `/docs/install.md` | manual only |
| DOC-02 | Eviction workflow end-to-end | manual / fake-account smoke | full sequence on test workbook | manual only |
| DOC-03 | Screenshots + GIF linked from install page | manual / inspection | open Pages site, check images render | manual only |

### Sampling Rate

- **Per task commit:** `cd apps-script && npm test -- <changed-file>`
- **Per wave merge:** `cd apps-script && npm test` (full suite)
- **Phase gate:** Full suite green + manual smoke per Phase 5 §5 success criterion ("all 12 guildies installed and writing data")

### Wave 0 Gaps

- [ ] `apps-script/src/__tests__/searchIndex.test.ts` — covers SEARCH-01, SEARCH-02, SEARCH-04, plus didYouMean
- [ ] `apps-script/src/__tests__/showSearchSidebar.test.ts` — covers theme-aware HTML render, `escapeHtml` defense
- [ ] `apps-script/src/__tests__/weeklySchemaHealthcheck.test.ts` — covers OPS-06 happy path + tab-missing branch
- [ ] `apps-script/src/__tests__/weeklyStaleCharArchive.test.ts` — covers VIEW-05 happy path + watcher-race guard
- [ ] `apps-script/src/__tests__/showEvictionSidebar.test.ts` — covers DOC-02 cascade + grace timer
- [ ] `apps-script/src/__tests__/test-helpers.ts` extension — CacheService mock needs TTL respect (currently returns `null` from `get`); add `getAll` mock for batch reads
- [ ] No framework install needed — vitest already in use

## Project Constraints (from CLAUDE.md)

The following CLAUDE.md directives apply to Phase 5 and the planner MUST verify compliance:

1. **Stack lockdown (CLAUDE.md "Technology Stack"):**
   - Apps Script V8 only — NEVER Rhino.
   - clasp v2.4+ ONLY — NEVER 3.x (breaking changes per Phase 3 RESEARCH §6).
   - esbuild 0.20+, `@types/google-apps-script`.
   - `HtmlService` for sidebars — never inject external CDN scripts; inline `<style>` / `<script>` only.
   - No HTML scraping of PigParse or wiki (we read what Phase 3 cached in `_item_master` / `_pigparse`).
2. **Architecture lockdown (CLAUDE.md "Architecture"):**
   - Views are CONSOLIDATED, never per-character (would breach 200-tab limit). Search sidebar is NOT a tab — it's HtmlService.
   - Watcher writes are atomic per-character batchUpdate clear+write; never appends. Phase 5 archive must not race with these (Pitfall P6).
   - `Session.getActiveUser().getEmail()` returns SCRIPT OWNER, not writer — load-bearing for Pattern 4 / Code Example 5.
3. **Schema lockdown (CLAUDE.md "Architecture"):**
   - Extend-only. Add columns at right edge, add tabs, add `_meta` rows.
   - Breaking changes require `schema_version` bump + idempotent migration + `WATCHER_MAX_SCHEMA_VERSION` check + `SCRIPT_MIN_SCHEMA_VERSION` check.
   - Phase 5 recommendation: avoid schema bump via lazy `_archive` creation (Pattern 6 Path A).
4. **Conventions (CLAUDE.md "Conventions"):**
   - Schema migrations live in `apps-script/src/lib/migrations.ts` — extend-only, idempotent, `_meta.schema_version` LAST write. Phase 5's `hideAllSystemTabs` + `protectBankToonName` go here.
   - `WatcherMaxSchemaVersion` in `internal/sheet/client.go` line 44 currently `3`. Do NOT bump unless Phase 5 ships a schema migration (it shouldn't).
   - Structured logging: Go-side slog, Apps Script `log(level, op, fields)`. New triggers use the existing helper.
   - Theme palette source-of-truth: `docs/design/eq-aesthetic-theme.md`. Sidebar pulls from `THEMES` registry only.
5. **GSD workflow (CLAUDE.md "GSD Workflow Enforcement"):**
   - No direct file edits outside a GSD command. Planner produces PLAN.md(s); execute via `/gsd-execute-phase`.

## Sources

### Primary (HIGH confidence)

- [Google Apps Script — Class Cache](https://developers.google.com/apps-script/reference/cache/cache) — CacheService 100KB-per-value cap verified
- [Google Apps Script — HTML Service Best Practices](https://developers.google.com/apps-script/guides/html/best-practices) — sidebar perf guidance
- [Google Apps Script — Quotas for Google Services](https://developers.google.com/apps-script/guides/services/quotas) — CacheService 25-min lifetime cap
- [Google Apps Script — Lock Service](https://developers.google.com/apps-script/reference/lock)
- [pages-themes/cayman README](https://github.com/pages-themes/cayman) — `remote_theme: pages-themes/cayman@v0.2.0` config verified
- [Apps Script CacheService Eviction and Other Limits — DEV.to](https://dev.to/googleworkspace/apps-script-cacheservice-eviction-and-other-limits-1p6d)
- [Apps Script CacheService unofficial limits — justin.poehnelt.com](https://justin.poehnelt.com/posts/exploring-apps-script-cacheservice-limits/) — corroborates 100KB and 1000-item caps
- Codebase verifications (read 2026-05-11):
  - `apps-script/src/triggers/showCharInfoSidebar.ts` — Phase 4 sidebar form pattern baseline
  - `apps-script/src/triggers/showBankCoinSidebar.ts` — Phase 4 second sidebar example
  - `apps-script/src/triggers/installTriggers.ts` — current 7-trigger registration; extend point for Phase 5's 3 new triggers
  - `apps-script/src/triggers/monitorCellCount.ts` — analog for `weeklySchemaHealthcheck`
  - `apps-script/src/lib/migrations.ts` — `protectBankCoinCells` template for `protectBankToonName`
  - `apps-script/src/tabs/buildView.ts` — `inv:*` iteration pattern (lines 162-187)
  - `apps-script/src/lib/sheet-helpers.ts` — `readMetaRows` / `writeMetaRow` / `appendColumns` helpers
  - `apps-script/src/lib/themes.ts` — THEMES registry for theme-aware sidebar
  - `apps-script/src/Code.ts` — global re-export pattern; Phase 5 adds new entries
  - `apps-script/src/__tests__/test-helpers.ts` — CacheService mock (line 332) needs TTL/getAll extension
  - `internal/sheet/client.go:44` — `WatcherMaxSchemaVersion = 3`
  - `internal/scaffold/scaffold.go` — DimensionTabs + ViewTabs source of truth for expected-tabs healthcheck

### Secondary (MEDIUM confidence)

- [tanaikech benchmark — Sheets read/write perf](https://tanaikech.github.io/2018/10/12/benchmark-reading-and-writing-spreadsheet-using-google-apps-script/) — `getValues()` 10× faster than per-cell; underpins Pattern 2 bulk-read
- [Apps Script Sheets API guides — Read & write cell values](https://developers.google.com/workspace/sheets/api/guides/values)
- [fastest-levenshtein README](https://github.com/ka-weihe/fastest-levenshtein) — algorithm reference for hand-rolled didYouMean

### Tertiary (LOW confidence — verify if load-bearing)

- [Master Google Apps Script UIs — Dmitry Kostyuk](https://javascript.plainenglish.io/the-ultimate-google-apps-script-front-end-development-guide-542694b29496) — community sidebar perf tips (not directly cited; informs Pattern 1)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every primitive (HtmlService, CacheService, LockService, Range.protect, Sheet.hideSheet, time-driven triggers) is built-in and already in use Phase 3/4. No new external dependencies.
- Architecture: HIGH — Patterns 1, 4, 5, 6 directly clone Phase 4 idioms. Pattern 2 (per-`inv:Char` cache) is the only novel piece; CONTEXT.md already locked the design (D-03).
- Pitfalls: HIGH — 12 named pitfalls, every one either verified in codebase (P4, P6, P7, P8, P11, P12) or grounded in Google's own docs (P1, P2, P3, P10) or the CONTEXT.md text itself (P5).

**Research date:** 2026-05-11
**Valid until:** 2026-06-10 (30 days for stable workflow; Apps Script primitives and Cayman theme are stable surfaces)
