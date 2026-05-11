---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: milestone-v1.0-complete
last_updated: "2026-05-11T16:30:00Z"
progress:
  total_phases: 5
  completed_phases: 5
  total_plans: 31
  completed_plans: 31
  percent: 100
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-11 (Phase 5 SHIPPED as v1.0.0 — DOC-02 fake-guildie eviction smoke PASS, Pages site live at https://boejowen.github.io/SquireBot/, distribution report landed, milestone v1.0 COMPLETE)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-11 after v1.0 milestone close)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.
- **Current focus:** Milestone v1.0 COMPLETE (2026-05-11); next milestone undefined. Start via `/gsd-new-milestone` when ready.
- **Mode:** yolo
- **Granularity:** coarse
- **Total v1 phases:** 5 (all shipped)

## Current Position

Phase: 1 (End-to-End Thin Slice) — ✅ SHIPPED (v0.1.0)
Phase: 2 (Watcher Robustness + Schema Lock) — ✅ SHIPPED (v0.2.0 → v0.2.1 wizard fix)
Phase: 3 (Apps Script Enrichment Foundation) — ✅ SHIPPED (v0.3.0)
Phase: 4 (Differentiator Features) — ✅ SHIPPED (v0.4.0 — 2026-05-11)
Phase: 5 (Search + Onboarding + Privacy Polish) — ✅ SHIPPED as v1.0.0 (2026-05-11; milestone v1.0 COMPLETE)

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
- **Resume file:** none — milestone v1.0 SHIPPED. Next work is v1.0.1 / v2 backlog (deferred items listed in 05-05-SUMMARY.md §Deferred backlog).
- **Progress:** ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 31/31 plans complete (all 5 phases SHIPPED; milestone v1.0 COMPLETE)

### Phase 5 SHIPPED (2026-05-11)

**Ship signal:** `ship: phase-5-shipped, distinct-recent: 1, distinct-active: 1` (user, 2026-05-11)

**Sequence of ship-day events:**
1. 05-01 → 05-04 plans executed by gsd-executor sequential waves (commits c85586b → 6e0bce0). 297/297 vitest green at wave-4 boundary; trigger count 7→10; Path A (schema_version=3) held end-to-end.
2. 05-05 Task 1 (Pages site source + README D-12 shrink) shipped via commit df8c2fd.
3. Scope reduction recorded 2026-05-11 mid-execution: instructional PNGs + SmartScreen GIF dropped from D-10; Pages site is text-only. Commit 482f19d stripped image refs from install.md + troubleshooting.md + dev.md, removed docs/assets/.gitkeep, updated 05-CONTEXT.md §Scope Changes + 05-05-PLAN.md Task 2 (Steps A/B/D struck) + 05-05-SUMMARY.md.
4. Pages enabled by user (Settings → Pages → main /docs). First `pages-build-deployment` workflow served `/install/` and `/troubleshooting/` as 404 due to Jekyll default permalinks → fixed via commit 8d1e332 adding `permalink: pretty` to docs/_config.yml. Second deployment served all four pages cleanly.
5. clasp-push deployed apps-script Phase-5 bundle to RiverFAIL VanFRAUD dev workbook (bundle 158KB; 28 globals registered; `Evict Guildie…` menu item visible between `Search…` and `Set Theme…`; Install Triggers reports 10 triggers).
6. DOC-02 fake-guildie eviction smoke executed end-to-end against the dev workbook (seed → sidebar preview → Evict → simulate grace-expiry edit → manual weeklyEvictionArchive run → verify 7-point checklist → recovery test → cleanup). All 7 checkpoints PASS. Archive snapshot non-destructive; un-evict path works.
7. `_phase5DistributionProbe` ran against `_char_owner`: N=1 (last_seen<7d), M=1 (distinct active emails), only writer is boejowen@gmail.com. Single-writer count is expected dev-workbook state (no guildies onboarded yet); rollout is a v1.0.1 / post-ship activity per CONTEXT.md §Scope Changes 2026-05-11.
8. Distribution report committed at `docs/phase5-distribution.md` (commit b668e75).
9. User authorized ship via `ship: phase-5-shipped`. STATE.md flipped to `phase-5-shipped`; REQUIREMENTS.md DOC-01/02/03 marked ✓ Complete (local-only); ROADMAP.md Phase 4 + Phase 5 boxes ticked; 05-05 plan ticked. Tagged `phase5-complete` and `v1.0.0`; tags pushed.

**Milestone v1.0 verdict:** COMPLETE. Functionally validated end-to-end (DOC-02 smoke PASS, Pages site live, watcher v0.4.0 + apps-script Phase-5 bundle deployed on dev workbook). Guild rollout deferred to v1.0.1 / post-ship; the Phase 5 deliverables (search sidebar, eviction workflow, archive backend, onboarding docs) are the prerequisites for that rollout.

### Phase 5 plan execution log

| Phase-Plan | Name | Duration | Tasks | Commits | Completed |
|------------|------|----------|-------|---------|-----------|
| 05-01 | Schema healthcheck + tab hide + bank-toon-name protect | ~20min | 3 | c85586b, ae57d61, a9562b6 | 2026-05-11 |
| 05-02 | Archive lib + weeklyStaleCharArchive + weeklyEvictionArchive | ~7min | 3 | 32f8cfa, 434adf6, ab10732, 4c3b339, 2530de3 | 2026-05-11 |
| 05-03 | Cross-character search sidebar (SEARCH-01..04 + TIP-04) | ~14min | 3 | bd346c4, 1733264, bdd6774 | 2026-05-11 |
| 05-04 | Eviction sidebar + DOC-02 runbook | ~6min | 2 | f213bdb, 2e39b3e, 658b4a6 | 2026-05-11 |
| 05-05 | Jekyll Pages site + README shrink + distribution handoff | ~12min | 1 of 3 (Tasks 2 + 3 USER ACTION REQUIRED) | df8c2fd | 2026-05-11 (Task 1 only) |

**05-05 outcomes:** Task 1 (Pages site source + README D-12 shrink) CODE-COMPLETE. `docs/_config.yml` (Cayman remote_theme v0.2.0), `docs/index.md` (1-paragraph landing), `docs/install.md` (5-step chronological walkthrough), `docs/troubleshooting.md` (8 symptom-keyed H2 sections incl. installer-upgrade workaround per CONTEXT scope_notes), `docs/dev.md` (flat link list + Phase 1..5 SUMMARY index + screenshot-redaction note), `docs/assets/.gitkeep` (placeholder). README shrunk 117 → 11 lines per D-12 (target ≤30). REQUIREMENTS.md SEARCH-03 row updated to ✓ Complete via Path 2 + footnote added (LOCAL ONLY — file is .gitignored). 1 commit (df8c2fd). **Tasks 2 + 3 are USER ACTION REQUIRED:** Task 2 = capture 6 PNG screenshots + record ≤5MB annotated SmartScreen GIF + enable GitHub Pages on the repo settings (source=main, folder=/docs); Task 3 = DOC-02 fake-guildie E2E smoke on dev workbook + produce `docs/phase5-distribution.md` heartbeat-freshness report + user authorizes Phase 5 SHIPPED. DOC-01, DOC-02, DOC-03 status remain PENDING-USER-ACTION; REQUIREMENTS.md rows MUST NOT be marked Complete until the user lands the assets, runs the smoke, and signals `ship: phase-5-shipped`. schema_version=3 unchanged; WatcherMaxSchemaVersion=3 untouched; Path A held across all 5 Phase-5 plans. Phase 5 status: `phase-5-code-complete` (intermediate state pending Task 2 + 3 + ship signal → `phase-5-shipped`).

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

None. Milestone v1.0 SHIPPED 2026-05-11; no carry-over blockers. Tech debt acknowledged in `.planning/milestones/v1.0-MILESTONE-AUDIT.md` (documentation-only; no quality risks).

### v1.0.1 / v1.1+ Backlog (carried in ROADMAP.md)

Twelve items captured in `.planning/ROADMAP.md` § Backlog (999.1 through 999.12). Highlights: installer overwrite-running shim, SignPath OSS approval (in flight), sidebar inline-JS unit tests, PropertiesService recent-query persistence, v2 Wantlist + Discord pinger (WANT-01..08 with open prerequisites). See backlog for full list.

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

**Phase 5 code-complete (2026-05-11). USER ACTION REQUIRED to ship.** Phases 1-4 SHIPPED; Phase 5 plans 05-01..05-05 are all code-complete to the extent Claude can execute them. The remaining work for milestone v1.0 is the two USER ACTION REQUIRED handoffs in `.planning/phases/05-search-onboarding-privacy-polish/05-05-SUMMARY.md`:

1. **Task 2 — Asset capture + Pages enable** (per 05-05-SUMMARY "Task 2" section):
   - Capture 6 PNG screenshots (installer dialog, SmartScreen × 2, OAuth consent, folder picker, tray-red) → `docs/assets/0[1-5]-*.png` + `tray-red.png` (each ≤200KB; PII-redacted).
   - Record annotated SmartScreen GIF (≤5MB, ≤720p, ≤15fps) → `docs/assets/smartscreen.gif`.
   - Enable GitHub Pages: Settings → Pages → Deploy from a branch → main + /docs → Save. Verify `https://boejowen.github.io/SquireBot/` renders Cayman-themed.
   - Commit assets.

2. **Task 3 — DOC-02 fake-guildie smoke + distribution report + ship authorization** (per 05-05-SUMMARY "Task 3" section):
   - Pre-flight asset audit (node -e block) must exit 0.
   - Run the 7-step fake-guildie smoke on the `RiverFAIL VanFRAUD` dev workbook (seed → preview → evict → simulate grace expiry → run `weeklyEvictionArchive` → verify archive + log mutation → cleanup).
   - Run the diagnostic probe (`_phase5DistributionProbe` from SUMMARY) on `_char_owner`; capture `distinctActiveEmails.size` + `distinctRecentEmails.size`.
   - Write `docs/phase5-distribution.md` per the template (TL;DR + per-email heartbeat table + notable-gaps narrative).
   - User authorizes ship via reply: `ship: phase-5-shipped` / `ship: hold, reason: <reason>` / `ship: smoke-regression-investigate` (only valid if count ≤ 1).

**On `ship: phase-5-shipped`:**
- Bump STATE.md status to `phase-5-shipped`, completed_phases=5, percent=100.
- Tick ROADMAP.md Phase 5 box + 05-05 row.
- Mark REQUIREMENTS.md DOC-01, DOC-02, DOC-03 as ✓ Complete (Plan 05-05).
- Tag `phase5-complete` + `v1.0.0` and push.
- Trigger milestone retrospective + reconcile the "56 vs 69 v1 requirements" roadmap-audit TODO from CLAUDE.md.

**Independent of Phase 5:** Day-10 token-survival check fires automatically 2026-05-13T15:00:00Z (routine `trig_01Uog2muQ22CBsjZfqPiSH9r`).

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
