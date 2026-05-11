# Phase 5: Search + Onboarding + Privacy Polish — Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-11
**Phase:** 5-search-onboarding-privacy-polish
**Areas discussed:** Search query model, Search results layout, Onboarding docs + assets
**Areas skipped (defaulted by Claude):** Eviction workflow UX

---

## Area Selection

| Option | Description | Selected |
|---|---|---|
| Search query model | Free-text fuzzy across all inv:* tabs vs. structured filters vs. hybrid | ✓ |
| Search results layout | Minimal one-liners vs. richer cards | ✓ |
| Eviction workflow UX | Owner-only vs. self-service vs. both | |
| Onboarding docs + assets | README rewrite vs. separate ONBOARDING + screenshots/video format | ✓ |

**Note:** Eviction workflow defaulted to "owner-only sidebar" per ROADMAP §92 sequencing language ("remove their email from workbook share → mark all their characters `is_removed`"). Captured in CONTEXT.md `Claude's Discretion`.

---

## Search query model

### Q1 — Input shape

| Option | Description | Selected |
|---|---|---|
| Free-text only | Case-insensitive substring match against every item Name | |
| Free-text + minimal filters | Free-text + Char dropdown + Slot dropdown above input | ✓ |
| Free-text + inline syntax | `char:Findom slot:HEAD` parser | |
| Free-text + history dropdown | Last ~10 queries on input focus | |

**User's choice:** Free-text + minimal filters
**Notes:** Structure lives in UI controls, not the typed string.

### Q2 — Match semantics

| Option | Description | Selected |
|---|---|---|
| Case-insensitive substring | `russet` matches anywhere within an item Name | ✓ |
| Word-prefix match | Sheets-default behavior; `russ` matches words starting with russ | |
| Fuzzy (Levenshtein ≤2) | Forgives typos; ~5–30ms per item; risks 2-sec budget | |
| Substring + word-boundary boost | Substring wins; word-start matches sort first | |

**User's choice:** Case-insensitive substring
**Notes:** Single-pass scan, predictable, fast. EQ items have long compound names so substring is the natural fit.

### Q3 — Cache shape

| Option | Description | Selected |
|---|---|---|
| Per-query result cache | Key = serialized query, value = result rows | |
| Per-workbook index cache | Whole flat index, needs CacheService chunking (100KB cap) | |
| Per-`inv:Char` cache | One entry per inv tab, ~75KB each, fits cap naturally | (Claude default) |
| No CacheService — always fresh | Skip caching; would not meet SEARCH-04 | |

**User's choice:** Deferred to Claude — *"I have no preference. Please use whichever cache shape you think would be best for the end-user"*
**Claude's pick:** Per-`inv:Char` cache. Rationale captured in CONTEXT.md D-03.

### Q4 — Empty state

| Option | Description | Selected |
|---|---|---|
| Plain "No matches" text | Single line, no suggestions | |
| "No matches" + last-search recall | Plus the last 3 queries as click-to-redo | |
| "No matches" + did-you-mean | Fuzzy second pass when literal substring fails | ✓ |
| "No matches" + scope hint | Plus a list of stale/unsynced chars | |

**User's choice:** "No matches" + did-you-mean fuzzy fallback
**Notes:** Reverses the no-fuzzy direction from Q2 — but only on the no-match branch where the wait is acceptable. Primary path stays fast substring.

---

## Search results layout

### Q1 — Row shape (presented twice; first answered with a freeform note dropping staleness)

**First pass — user notes "No need to indicate sync times" instead of selecting an option.**

| Option | Description | Selected |
|---|---|---|
| ROADMAP-literal one-liner (with staleness) | Single line per match | |
| Card with expand-on-click | Compact card, click to expand | |
| Two-line stacked (with staleness) | Two-line stacked with second-line enrichment | |

**Re-presented without staleness:**

| Option | Description | Selected |
|---|---|---|
| ROADMAP-literal one-liner | Single line, no expansion | |
| Card with expand-on-click | Compact card, click to expand inline | |
| Two-line stacked | Line 1 = item/char/loc/count, Line 2 = wiki + price; tooltip for full summary | ✓ |

**User's choice:** Two-line stacked
**Notes:** **Scope change** — user explicitly dropped SEARCH-03 inline staleness. Captured in CONTEXT.md as `scope_changes`.

### Q2 — Sort order

| Option | Description | Selected |
|---|---|---|
| Group by item name | All matches for one item cluster together; "who has THIS?" intent | ✓ |
| Group by character | All of one char's matches first; "what does Findom have?" intent | |
| Flat list, item-name asc → char asc | No grouping, simple alphabetical | |
| Bank toon first, then alpha | Inv:bank_toon matches first | |

**User's choice:** Group by item name
**Notes:** Within each group, char asc.

### Q3 — High-cardinality handling

| Option | Description | Selected |
|---|---|---|
| No cap, render everything | ~30KB DOM at 200 rows; lag on single-letter typos | |
| Soft cap at ~50 + show more | Batch reveal on click | |
| Hard cap at 100 + warning | Forces refinement | |
| Auto-collapse groups when >5 chars match | Per-group collapse, no global cap | ✓ |

**User's choice:** Auto-collapse groups when >5 chars match
**Notes:** Smart UX — collapse-by-density rather than hard truncation.

### Q4 — Lifecycle

| Option | Description | Selected |
|---|---|---|
| Open empty, no persistence | Blank state every open | |
| Open empty + remember last query (CacheService) | "Recent: <last query>" link | (modified) |
| Open with last query auto-rerun | Reopens to previous results | |
| Open empty + auto-focus + Enter submits | Pure ergonomics, no recall | |

**User's choice:** *"Option 2, but show the last three searches under the 'recent: <last query>' section"*
**Notes:** Modified Option 2 — keep last 3 (not 1). CacheService key `squirebot:search:recent`, MRU rolling window.

---

## Onboarding docs + assets

### Q1 — Doc file shape

| Option | Description | Selected |
|---|---|---|
| Rewrite README.md | Onboarding becomes the README content | |
| Separate ONBOARDING.md, README points | Short README, long ONBOARDING.md | |
| Single ONBOARDING.md, README untouched | Don't touch existing README | |
| GitHub Wiki / Pages | Offsite from main repo | ✓ |

**User's choice:** GitHub Wiki / Pages
**Notes:** Onboarding goes off-repo for visual chrome.

### Q2 — Wiki vs Pages

| Option | Description | Selected |
|---|---|---|
| GitHub Wiki | Free per-repo wiki, simple Markdown, basic | |
| GitHub Pages (Jekyll default) | Static site, custom theme + CSS, build step | ✓ |
| GitHub Pages (no Jekyll, raw HTML/MD) | `.nojekyll` flag, lightest static site | |

**User's choice:** GitHub Pages with Jekyll
**Notes:** `boejowen.github.io/SquireBot`; source under `/docs`.

### Q3 — Asset host + video format

| Option | Description | Selected |
|---|---|---|
| All in repo /docs/assets/, video as MP4 | Single source of truth; binary diff churn | |
| Screenshots in repo, video as YouTube embed | Video hosted offsite | |
| Screenshots in repo, video as annotated GIF | Self-hosted, no audio, autoplay-friendly | ✓ |
| Screenshots as GitHub Release attachments | Zero repo bloat, URL fragility | |

**User's choice:** Screenshots in repo + annotated GIF
**Notes:** Self-hosting bias. GIF size budget ≤5 MB.

### Q4 — Recovery doc location

| Option | Description | Selected |
|---|---|---|
| Inline troubleshooting at end of install page | One page guildies Ctrl-F | |
| Separate Troubleshooting page | Dedicated `/troubleshooting` linked from install | ✓ |
| FAQ format (Q&A) | StackOverflow-style | |
| Decision tree | Interactive yes/no flow | |

**User's choice:** Separate `/troubleshooting` page
**Notes:** Recovery is search-driven (panic-mode), not read-linearly.

---

## Claude's Discretion

- **Eviction workflow UX shape** — defaulted to owner-only sidebar (cascade `is_removed=TRUE` across departed `owner_email`'s chars; 30-day grace via PropertiesService timer; weekly trigger archives grace-expired). Rationale: ROADMAP §92 sequencing implies owner action; self-service evict adds threat-model risk that v1 doesn't need.
- **Search cache shape** (D-03) — user deferred; per-`inv:Char` chosen for the CacheService 100 KB-cap fit + warm-search performance.
- **Search slot filter dropdown contents** — planner enumerates from real inventory data OR hardcodes P99 slot list.
- **Pages Jekyll theme** — planner picks default; user can refine post-ship.
- **System tab hide enforcement** — extend Phase 4's pattern; ensure ALL `_*` tabs hidden after `installTriggers`.
- **`Range.protect()` on `_meta.bank_toon_name`** — same warning-only idiom as Phase 4 bank coin cells.

## Deferred Ideas (also mirrored in CONTEXT.md `<deferred>`)

- Bank-coin permission lock (Phase 4 carry-over) → v1.0.x patch candidate
- Polished theme picker tile UI (Phase 4 carry-over) → v1.0.x patch candidate
- Sidebar HTML inline-JS unit tests (Phase 4 carry-over) → v2 ergonomics
- Installer-driven upgrade UX (Phase 4 carry-over) → fold into Phase 5 only if 12-guildie distribution blocks; otherwise document workaround in `/troubleshooting`
- Self-service eviction → v2 if amicable-departures-only assumption breaks
- Power-user inline search syntax (`char:Findom slot:HEAD`) → v2
- Word-prefix / fuzzy primary match → fuzzy survives only as no-match fallback
- Index-cache search shape → per-char fits CacheService cap better
- Card-with-expand-on-click result row → two-line stacked won
- YouTube video for SmartScreen walkthrough → annotated GIF won; revive only if GIF size becomes prohibitive
- Sub-menu structure for SquireBot menu → v2 polish when item count > 12
