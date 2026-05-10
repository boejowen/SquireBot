---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: phase-4-discuss-complete
last_updated: "2026-05-10T22:30:00Z"
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 22
  completed_plans: 22
  percent: 80
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-05-09 (Phase 2 shipped as v0.2.0 + v0.2.1; Phase 3 plan 03-01 landed)

## Project Reference

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.
- **Current focus:** Phase 3 (Apps Script Enrichment Foundation) — context captured; research pending
- **Mode:** yolo
- **Granularity:** coarse
- **Total v1 phases:** 5

## Current Position

Phase: 2 (Watcher Robustness + Schema Lock) — ✅ SHIPPED (v0.2.0 → v0.2.1 wizard fix)
Phase: 3 (Apps Script Enrichment Foundation) — 🔵 CONTEXT CAPTURED — research next

- **Phase:** 4 — Differentiator Features (CONTEXT done 2026-05-10; RESEARCH pending)
- **Plan:** none yet
- **Node:** — (none yet)
- **Status:** Phase 4 CONTEXT.md written 2026-05-10 capturing locked decisions (extend `_char_owner` with `race` column → schema_version=3 migration; ship watcher v0.4.0 first; sidebar forms for char info + bank coin; trigger inventory grows from 4 to 7; reuse Phase 3's politeFetch + resumable cursor + builder pattern; same courtesy-emails WAIVED policy). Race tracking added to handle Iksar racial gear tier without showing it as universal noise. All 12 Phase 4 REQs (ENRICH-03/04, BANK-01..04, CHECK-01..05, OPS-07) mapped. Research flag = NEEDED — must capture wikitext fixtures for the 3 Velious gear-tier pages + sample per-class spell pages, then produce parser specs. Phase 3 still complete + shipped (v0.3.0). Next: `/gsd-research-phase 4`.
- **Resume file:** `.planning/phases/04-differentiator-features/04-CONTEXT.md` — read before invoking research.
- **Progress:** ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░ 18/~28 plans complete (Phase 1 + 2 done; Phase 3 in discuss-complete; Phase 4-5 unplanned)

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
