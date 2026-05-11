---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: phase-5-in-progress
last_updated: "2026-05-11T05:24:00Z"
progress:
  total_phases: 5
  completed_phases: 4
  total_plans: 31
  completed_plans: 30
  percent: 97
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-11 (Phase 5 plan 05-04 SHIPPED — eviction sidebar + DOC-02 runbook live; 30/31 plans complete)

## Project Reference

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.
- **Current focus:** Phase 5 (final phase) — CONTEXT captured 2026-05-11; ready for /gsd-plan-phase 5
- **Mode:** yolo
- **Granularity:** coarse
- **Total v1 phases:** 5

## Current Position

Phase: 1 (End-to-End Thin Slice) — ✅ SHIPPED (v0.1.0)
Phase: 2 (Watcher Robustness + Schema Lock) — ✅ SHIPPED (v0.2.0 → v0.2.1 wizard fix)
Phase: 3 (Apps Script Enrichment Foundation) — ✅ SHIPPED (v0.3.0)
Phase: 4 (Differentiator Features) — ✅ SHIPPED (v0.4.0 — 2026-05-11)
Phase: 5 (Search + Onboarding + Privacy Polish) — 🟢 IN PROGRESS (4/5 plans complete: 05-01 + 05-02 + 05-03 + 05-04 SHIPPED 2026-05-11)

- **Phase:** 4 — Differentiator Features ✓ SHIPPED as v0.4.0
- **Plan:** All 4 plans (04-01..04) landed; v0.4.0 tag pushed; live smoke PASS
- **Node:** — (none yet)
- **Status:** v0.4.0 promoted from v0.4.0-rc1 on 2026-05-11T02:36:11Z (CI run 25647367282 success, 2m). Watcher binary unchanged from rc1 (Version string differs: "0.4.0" vs "0.4.0-rc1" → different SHA but same source). Apps-script HEAD at commit 9319c6b is what's deployed via clasp.

  **Live smoke test on dev box (2026-05-10) — PASS for all 6 steps:** installer + tray green, version=0.4.0-rc1 confirmed in heartbeat log, ErrSchemaTooNew startup gate fires correctly with clear user-facing message, migrateToV3 runs cleanly + Install Triggers reports 7 triggers + bank-coin protect, Range.protect warning prompt fires on direct edit, gear_check populates with proper OK/MISSING/OTHER status, spell_check populates with 359 rows across all 11 caster classes (1,562 spells in _wiki_spells).

  **3 fix-pack patches landed during smoke (apps-script only — no rc2 needed):**
    - 3c5ea6d: bank coin protection — flip setWarningOnly(true) so script owner sees the prompt (was strict mode → owner is default editor → no UX); auto-call protectBankCoinCells from saveBankCoin so first-save-creates-rows immediately gets protection (closed lazy-creation gap).
    - b9482a6: missing on-demand menu items for refreshWikiSpells/refreshWikiGearTier/monitorCellCount (plan 04-04 added them to installTriggers but missed onOpen menu).
    - 9319c6b: wiki-spell parser handles all 4 P1999 template variants — {{SpellRow}} (CLR/PAL/NEC/MAG/ENC, 5 classes), {{RadSpellRow}} (WIZ/SHM/RNG/SHD, 4 classes), {{RadSpellRow2}} (DRU only, 1 class), {{Template:SongRow}} with inline |level=N (BRD only, 1 class). Live sanity-check confirmed all 11 caster classes parse: 1,562 expected spells across the workbook (vs 784 broken). Pre-fix _wiki_spells only had 5 classes; post-fix has 11.

  **Phase 4 deferred to Phase 5:** bank-coin permission lock (only bank-toon-owner can use sidebar), polished theme picker UI, cross-character search sidebar, system-tab hide, weekly schema healthcheck, eviction workflow, stale-char auto-archive, sidebar HTML inline-JS unit tests, installer-driven upgrade UX (current installer can't overwrite running .exe — workaround: stop process first; possible fix: bundle a quit-then-install shim).

  **Next:** /gsd-discuss-phase 5 to capture context for the final phase. After Phase 5 ships, milestone v1.0 complete.
- **Resume file:** `.planning/phases/05-search-onboarding-privacy-polish/05-05-PLAN.md` — final plan in the sequential 05-01 → 05-05 execution chain (05-01 + 05-02 + 05-03 + 05-04 shipped 2026-05-11).
- **Progress:** ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 30/31 plans complete (Phases 1+2+3+4 SHIPPED; Phase 5: 05-01 + 05-02 + 05-03 + 05-04 shipped, 05-05 pending)

### Phase 5 plan execution log

| Phase-Plan | Name | Duration | Tasks | Commits | Completed |
|------------|------|----------|-------|---------|-----------|
| 05-01 | Schema healthcheck + tab hide + bank-toon-name protect | ~20min | 3 | c85586b, ae57d61, a9562b6 | 2026-05-11 |
| 05-02 | Archive lib + weeklyStaleCharArchive + weeklyEvictionArchive | ~7min | 3 | 32f8cfa, 434adf6, ab10732, 4c3b339, 2530de3 | 2026-05-11 |
| 05-03 | Cross-character search sidebar (SEARCH-01..04 + TIP-04) | ~14min | 3 | bd346c4, 1733264, bdd6774 | 2026-05-11 |
| 05-04 | Eviction sidebar + DOC-02 runbook | ~6min | 2 | f213bdb, 2e39b3e, 658b4a6 | 2026-05-11 |

**05-04 outcomes:** DOC-02 sidebar code + runbook live. `triggers/showEvictionSidebar.ts` (1 opener + 3 google.script.run callbacks: `getEvictionEmails`, `previewEviction`, `commitEviction`) — 300px theme-aware HtmlService sidebar at `SquireBot → Evict Guildie…` (between `Search…` and `Set Theme…`). Lock-guarded `commitEviction` cascades `is_removed=TRUE` across an `owner_email`'s `_char_owner` rows and appends an `{at, email, initiated_by, grace_until, chars, reason:'evicted'}` envelope to `_meta.eviction_log` — consumed by 05-02's `weeklyEvictionArchive` after the 30-day grace. Idempotent (only flips FALSE→TRUE; already-removed chars are no-op). Defensive: `Session.getEffectiveUser()` wrapped in try/catch with `'unknown'` fallback (Assumption A5); JSON.parse on prior log wrapped in try/catch (T-05-04-07 mitigation). Option A: inline `SIDEBAR_BODY` String.raw template, no companion .html file (consistent with 05-03). XSS-defended via inline `escapeHtml` (T-05-04-01..02). `docs/eviction-runbook.md` (DOC-02, 4.6KB, 5 sections — lifecycle, un-evict, post-archive recovery, edge cases, permissions note). SQUIREBOT_HANDLERS count UNCHANGED at 10 (sidebar callbacks live in TRIGGER_GLOBALS but not in handlers). schema_version=3 unchanged; WatcherMaxSchemaVersion=3 untouched; Path A held end-to-end. 297/297 vitest green (+14 tests: 12 showEvictionSidebar + 2 installTriggers cumulative-survival), npm run build TRIGGER_GLOBALS CI assertion passes with all 15 cumulative 05-01..05-04 entries. The fake-guildie E2E smoke + REQUIREMENTS.md DOC-02 mark-complete is owned by 05-05.

**05-03 outcomes:** SEARCH-01..04 + TIP-04 live. `lib/searchIndex.ts` (runSearch + levenshtein/didYouMean + prewarmSearchCache + getRecentSearches/pushRecentSearch + enrichResults + listInventorySlots) is the pure-logic search engine; `triggers/showSearchSidebar.ts` is the 300px theme-aware HtmlService sidebar (Option A — inline SIDEBAR_BODY String.raw constant, no companion .html file). CacheService key namespace `squirebot:search:*` — per-`inv:Char` 60s-TTL cache + recent-3 MRU + items_master/pigparse join caches. Auto-collapse groups with >5 chars (D-07) caps DOM at the high-cardinality query (single-letter-typo) edge case. Hand-rolled Wagner-Fischer Levenshtein (≤2 distance, ≤3 cap) on the no-match fallback. `onChange` + `installTriggers` best-effort pre-warm (try/catch envelope — throws do NOT propagate). SQUIREBOT_HANDLERS count UNCHANGED at 10 (no new time-driven trigger). SEARCH-03 PARTIAL via Path 2 (cache-freshness tooltip on Search button + cross-reference to existing view/bank Last Synced columns; per-row staleness intentionally NOT shown per user direction). schema_version=3 unchanged; WatcherMaxSchemaVersion=3 untouched; Path A held end-to-end. 283/283 vitest green (+37 tests: 27 searchIndex + 8 showSearchSidebar + 2 onChange), npm run build TRIGGER_GLOBALS CI assertion passes. test-helpers.ts upgrades (Map-backed TTL CacheService + fluent HtmlService builder + showSidebar capture) are forward-compatible scaffolding for any future Apps Script work needing those mocks.

**05-01 outcomes:** OPS-06 (weekly Sun-03:00 PT tab-by-id verification) live; `_meta.expected_sheet_ids` is the new sheet-id source-of-truth (lazy-backfilled, resilient to user renames per Pitfall P7); `_meta.last_error` `kind='tab_missing'` envelope wired to the existing watcher heartbeat tray-red signal. `protectBankToonName` (warning-only Range.protect) + `hideAllSystemTabs` (idempotent `_`-prefixed sweep) wired into `installTriggers` (trigger count 7 → 8). schema_version=3 unchanged; WatcherMaxSchemaVersion=3 untouched; no watcher rebuild required. 229/229 vitest green, npm run build TRIGGER_GLOBALS CI assertion passes.

**05-02 outcomes:** VIEW-05 + DOC-02 back-end live. `lib/archive.moveCharToArchive(charName, reason)` is the shared lock-guarded helper (LockService.getDocumentLock().tryLock(30000), Pitfall P6 mitigation: read `last_seen` INSIDE the lock). `weeklyStaleCharArchive` (Sun 06:00 PT) scans `_char_owner.last_seen` and archives every char older than 90 days; `weeklyEvictionArchive` (Sun 06:00 PT) consumes `_meta.eviction_log` entries with grace-expired entries atomically per-entry (failed entries retry next week; idempotency in `moveCharToArchive` prevents double-archive). New hidden `_archive` tab lazy-created with 7-col schema (archived_at, char_name, tab_type, row_count, uploaded_at, reason, snapshot_json). `_meta.archive_log` audit-trail KV row added (append-only JSON array). Trigger count 8 → 10. schema_version=3 unchanged; WatcherMaxSchemaVersion=3 untouched; Path A held end-to-end. 246/246 vitest green (+17 tests), npm run build TRIGGER_GLOBALS CI assertion passes. 05-04 (eviction sidebar) is the upcoming PRODUCER of `_meta.eviction_log` entries that this plan's archiver consumes after the 30-day grace.

### Phase 5 Planning Artifacts (2026-05-11)

| File | Purpose |
|------|---------|
| 05-CONTEXT.md | 12 LOCKED decisions D-01..D-12 + scope reductions (SEARCH-03 row-staleness deferred, "12 guildies" relaxed to "even a handful is fine" per user 2026-05-11) |
| 05-DISCUSSION-LOG.md | Discussion record from /gsd-discuss-phase |
| 05-UI-SPEC.md | UI design contract (6/6 dimensions PASS) — 300px sidebar, 4-pt spacing, theme-aware via CSS custom properties from `apps-script/src/lib/themes.ts` |
| 05-RESEARCH.md | Technical research (12 pitfalls catalogued, schema-impact Path A — NO WatcherMaxSchemaVersion bump) |
| 05-PATTERNS.md | 24 files classified with analog `path:line` references — sidebar 3-function shape from showCharInfoSidebar.ts, weekly-trigger `_meta.last_error` envelope from monitorCellCount.ts |
| 05-01-PLAN.md → 05-05-PLAN.md | 5 sequential plans (wave 1 protections+healthcheck → wave 5 onboarding+rollout) |

**Plan checker verdict (iteration 2/3):** VERIFICATION PASSED — 4 blockers + 6 warnings from iteration 1 all resolved.

## Performance Metrics

| Metric | Value |
|--------|-------|
| Phases planned | 1 / 5 |
| Phases complete | 1 / 5 |
| Plans complete | 8 / 8 (Phase 1) |
| Nodes complete | 0 |
| Requirements mapped | 69 / 69 |
| Requirements complete | 12 / 69 (INST-01..03, AUTH-01..04+06, WATCH-01, WATCH-04, OPS-01, OPS-03) |
| Active blockers | 0 |
| Hotfixes landed in Phase 1 | 5 (client_secret, ctx-canceled, picker views, log-folder menu, module rename) + 1 docs (oauth-setup) |

### Plan Execution Log

| Phase-Plan | Name | Duration | Tasks | Commits | Completed |
|------------|------|----------|-------|---------|-----------|
| 01-01 | Repo skeleton | ~55min | 3 | 1abb22a, 4900420, ddb594e | 2026-05-01 |
| 01-02 | OAuth Cloud setup | ~40min | 3 | fd4ff3c, 1a98df4 (hotfix) | 2026-05-01 |
| 01-03 | OAuth loopback PKCE | — | — | 7de3704, 429ec6c, e570943 | 2026-05-01 |
| 01-04 | EQ watcher + parser | — | — | c469d7f, 2824bda, ca1071a | 2026-05-01 |
| 01-05 | Sheets writer | — | — | 7106941, ef2329b, 58493f9 | 2026-05-01 |
| 01-06 | Drive Picker | — | — | 0c132f0, d3ccecc, ed410a0 | 2026-05-01 |
| 01-07 | Tray + wizard + RunApp | — | — | 7172153, c60b2ec, e04dd06 | 2026-05-01 |
| 01-08 | NSIS installer + release CI | — | — | 63c759f, 90508a4 | 2026-05-01 |

**Phase 1 hotfixes (in order):**

| # | Subject | Commit |
|---|---------|--------|
| 1 | Add client_secret to OAuth token exchange (Google contract) | 30361bf, 7aadec9 |
| 2 | Use Background ctx for TokenSource (request ctx cancels) | 10f0e92 |
| 3 | Picker shows owned + shared spreadsheets | a11dd9a |
| 4 | Wire Open log folder menu item to existing handler | 66497d1 |
| 5 | Rename module path to github.com/boejowen/SquireBot | 57c283b |
| 6 | Correct oauth-setup runbook to require client_secret | 1a98df4 |

## Accumulated Context

### Decisions Log

Decisions surfaced and locked during initialization (recorded in `PROJECT.md` Key Decisions; mirrored here for state continuity):

1. **Stack: Go watcher + Apps Script V8 (TypeScript via clasp + esbuild).** Single-binary Windows install (~12MB), no runtime to ship. PigParse REST API + MediaWiki API mean no HTML scraping in v1.
2. **OAuth scope = `drive.file` only.** Non-sensitive, no Google verification audit. Workbook selected via Drive Picker on first run.
3. **OAuth consent screen flips to Production before first guildie installs.** Testing-mode refresh tokens silently expire every 7 days for non-Workspace users.
4. **Consolidated filterable view tabs (`view`, `gear_check`, `spell_check`, `bank`) — never per-character view tabs.** Per-character views would breach Google's 200-tab/workbook hard limit at guild scale (12 × 10 × 5 ≈ 600 tabs). Locked.
5. **Schema evolution is extend-only.** Add columns at right edge, add tabs, add `_meta` rows. Breaking changes require `schema_version` bump + idempotent migration.
6. **Refresh tokens stored only in Windows Credential Manager via DPAPI (`wincred`).** Never in config file or any other plaintext location.
7. **Watcher writes are full-snapshot replaces per character per file.** Atomic clear+write via `spreadsheets.batchUpdate`. Never appends, never row-diffs.
8. **All scraping runs in Apps Script, not in 12 watchers in parallel.** Polite citizen of community resources; single source of truth.
9. **Code-signing strategy is open.** Phase 2 research must select EV vs OV cert (vs unsigned + walkthrough fallback) and integrate with `goreleaser`.
10. **(Phase 1 lesson, locked 2026-05-01)** Google's `/token` endpoint requires `client_secret` for Desktop OAuth clients even with PKCE — contrary to OAuth 2.0 spec. The desktop secret is effectively public per Google's own docs and is baked into the binary alongside the client ID. RESEARCH.md §4.1's "PKCE replaces client_secret" was a spec-correct / contract-wrong claim and is corrected in `docs/oauth-setup.md`.

### Open TODOs

- **(Day-10 token-survival check, scheduled)** Routine `trig_01Uog2muQ22CBsjZfqPiSH9r` fires 2026-05-13T15:00:00Z to validate AUTH-03 / Pitfall #1 (refresh token survives the 7-day Testing-mode expiry boundary). If it succeeds, AUTH-03 is structurally validated permanently. If it fails, Phase 1 blocker — re-open consent-screen publishing investigation.
- **(SmartScreen UX, deferred)** Real-world SmartScreen behavior (Mark-of-the-Web zone identifier) was untestable on the Azure VM smoke because RDP clipboard transfer doesn't tag MOTW. Will validate on first real GitHub-Releases download by a guildie.
- (Phase 2 research) Select code-signing certificate path (EV vs OV vs unsigned-with-walkthrough). The Phase 1 deferral above will be retired once a signed binary exists.
- (Phase 2 inline) Verify spellbook file format against a real EQ-produced sample before locking the `spell:<Char>` schema.
- (Phase 3 research) Probe PigParse `GET /api/item/getall/1` JSON shape end-to-end with real curl; produce field-level parser spec.
- (Phase 3 research) Probe MediaWiki `api.php?action=parse&prop=wikitext` template shapes for per-item summary pages.
- (Phase 3 inline) Send courtesy contact emails to PigParse operator and P1999 wiki admins **before** any live trigger runs.
- (Phase 4 research) Produce parser spec for `Players:Velious_Pre-Raid_Gear`, `Players:Velious_Raiding_Gear`, and Iksar racial tier wiki pages from real wikitext samples.
- (Roadmap audit) Reconcile the "56 v1 requirements" figure in `REQUIREMENTS.md` summary with the actual literal count of 69 REQ-IDs at the next milestone audit.

### Active Blockers

None.

## Session Continuity

### Last Session Summary

**2026-05-01 → 2026-05-02:** Executed all of Phase 1 (plans 01-01 through 01-08) end-to-end. Smoke validated on dev box AND on a clean Azure D2s_v5 Win11 VM as a non-admin standard user. 16/17 acceptance criteria green; only SmartScreen MOTW behavior deferred to first real download. Five code hotfixes landed during smoke: missing `client_secret` in token exchange (Google contract, not OAuth 2.0 spec), request-context cancellation killing the background TokenSource, Picker not showing owned spreadsheets (duplicate `setOwnedByMe(false)` DocsViews), tray missing Open-log-folder menu entry (was already coded; lifted into testable `MenuPlan()`), and module path renamed to `github.com/boejowen/SquireBot` after user clarified the actual GitHub identity. Sixth fix was a docs-only correction to `docs/oauth-setup.md` so future re-provisioning doesn't trip the same client_secret bug. Twelve requirements complete: INST-01..03, AUTH-01..04+06, WATCH-01, WATCH-04, OPS-01, OPS-03. Day-10 token-survival routine scheduled. Phase tagged `phase1-complete`.

**2026-05-01:** Executed Phase 1 Plan 01 (repo skeleton). Initialised Go module
`github.com/boejowen/SquireBot`, pinned all Phase 1 dependencies, built the
OPS-03 lumberjack logger and the refresh-token-free config store, wired
`cmd/squirebot/main.go` with embedded icon and structured smoke logging, added
the GitHub Actions release stub.

**2026-04-30:** Initialization session. Established `PROJECT.md`, `REQUIREMENTS.md`
(69 v1 / 8 v2-deferred), four research documents, and the converged `SUMMARY.md`.

### Next Action

Phase 1 is complete. The Day-10 token-survival check fires automatically on 2026-05-13 — leave it alone until then unless you want to validate sooner.

When ready for Phase 2:

1. `/gsd-research-phase 2` — code-signing certificate decision (EV vs OV vs unsigned-with-walkthrough). This research output also retires the deferred SmartScreen UX validation from Phase 1.
2. `/gsd-plan-phase 2` after research lands.
3. Continue per roadmap.

### Files of Record

- `.planning/PROJECT.md` — core value, requirements summary, constraints, key decisions.
- `.planning/REQUIREMENTS.md` — full REQ-ID list with traceability table.
- `.planning/ROADMAP.md` — 5-phase plan with success criteria and coverage map.
- `.planning/STATE.md` — this file.
- `.planning/phases/01-end-to-end-thin-slice/01-0{1..8}-SUMMARY.md` — per-plan execution summaries.
- `.planning/research/SUMMARY.md` — research synthesis.
- `.planning/research/STACK.md` — locked stack decisions.
- `.planning/research/ARCHITECTURE.md` — sheet schema and data-flow design.
- `.planning/research/PITFALLS.md` — 27 pitfalls catalogue.
- `.planning/research/FEATURES.md` — feature inventory.
- `.planning/config.json` — granularity, mode, workflow toggles, `commit_docs: false`.
- `docs/oauth-setup.md` — GCP Console runbook (committed; corrected 2026-05-02).
- `docs/build-and-install.md` — local build + sideload runbook (committed).

---

*State initialized: 2026-04-30 after roadmap creation. Phase 1 complete: 2026-05-02.*
