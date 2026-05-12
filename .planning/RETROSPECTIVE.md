# SquireBot Retrospective

Living document. Appended at each milestone close. Cross-milestone trends accumulate at the bottom.

---

## Milestone: v1.0 — Watcher + Workbook + Onboarding

**Shipped:** 2026-05-11
**Phases:** 5 | **Plans:** 31 | **Commits:** 203 | **Timeline:** 11 days

### What Was Built

A working two-half system: a Go watcher running on each guildie's Windows PC, and an Apps Script TypeScript bundle deployed to a shared Google Sheet. Together they answer the Core Value: "what does my character still need, and where in the guild is it?" — without leaving the spreadsheet.

Phase-by-phase output:

- **v0.1.0 thin slice** — installer → OAuth → watcher → workbook in 5 user-facing steps
- **v0.2.0 hardening** — spellbook + multi-folder + retry/backoff + heartbeat + auto-update + schema lock; 7-day soak survived deliberate `invalid_grant` + corrupt-update injections
- **v0.3.0 enrichment** — daily PigParse + weekly wiki triggers; consolidated `view` with hyperlinks, prices, cell-note tooltips, conditional Last-Synced formatting
- **v0.4.0 differentiators** — gear / spell progression checklists; bank toon view with manual coin sidebar; cell-count monitoring; wiki gear-tier + per-class spell scrapers (4 P1999 template variants × 11 caster classes × 1,562 spells)
- **v1.0.0 search + eviction + onboarding** — cross-character search sidebar; lock-guarded eviction workflow with 30-day grace; Jekyll Pages onboarding site live; DOC-02 fake-guildie smoke PASS all 7 checkpoints

### What Worked

- **Thin-slice-first sequencing.** Phase 1 validated three existential pitfalls (Google `client_secret` requirement for Desktop OAuth + PKCE, Production consent screen for token survival, `drive.file` non-sensitive scope) before any downstream work compounded. The five-hotfix flurry at v0.1.0 ship would have been ten times costlier discovered later.
- **Schema freeze at end of Phase 2.** Phase 2's schema lock (with all soft-delete + provenance fields scaffolded even though no UI consumed them in v1) meant zero destructive migrations across Phases 3–5. The one schema bump (v3 added `race` column for Iksar tier) was extend-only and idempotent. Path A confirmed across all of Phase 5 — no `WatcherMaxSchemaVersion` bump needed.
- **Locked architectural decisions caught at planning time.** The "consolidated mega-tabs, never per-character views" decision (Phase 2 D-04) was load-bearing — without it, 12 × 10 × 5 ≈ 600 tabs would have breached Google's 200-tab/workbook limit. Locking it explicitly in CLAUDE.md + ROADMAP + every phase's CONTEXT prevented every subsequent phase from re-litigating it.
- **Live smoke tests over formal verification.** Phase 1 Azure VM, Phase 2 soak runbook, Phase 3 SMOKE-TEST.md, Phase 4 6-step live smoke, Phase 5 DOC-02 fake-guildie smoke — every ship was empirically validated. Fix-pack patches caught during smoke (3c5ea6d setWarningOnly, b9482a6 menu items, 9319c6b template variants) shipped same-day without needing rcN bumps.
- **Sequential-wave executor pattern in Phase 5.** Five plans, five waves, one executor subagent per wave; orchestrator dispatch with checkpoint heartbeats between waves. Worked cleanly with one executor going briefly off-track (parallel reads + fabricated writes cancelled before any damage) — the cancellation discipline + git verification spot-checks kept the tree honest.
- **Mid-execution scope reductions when overhead exceeded value.** The 2026-05-11 drop of instructional PNGs + SmartScreen GIF in 05-05 — after Task 1 had shipped — was the right call. Text-only walkthrough satisfies the spirit of INST-05 + DOC-03 for a 12-person audience; capture overhead (clean Win11 box + ScreenToGif + 7 binary assets) was real money for negligible UX gain.

### What Was Inefficient

- **REQUIREMENTS.md never kept in sync.** The traceability table started saying "Pending" for every row and stayed that way across 5 phases. Only 7 of 69 rows were marked Complete pre-archive even though all phases had shipped. Reconciliation happened at milestone close (i.e., here). For v1.1+: either treat the table as a phase-close housekeeping checkbox or stop pretending to maintain it and rely on STATE.md commentary + git tags as the source of truth.
- **Phases 3 + 4 never wrote SUMMARY.md.** 8 plans across two phases shipped without per-plan summaries. The work is verified empirically (live smokes, tagged releases, commits) but the structured artifact is missing. STATE.md commentary blocks served as ad-hoc replacement.
- **Plan checker iteration 1 in Phase 5 produced 4 blockers + 6 warnings.** Iteration 2 cleared them. Worth a check of *why* the first iteration's plan missed those — they should have been caught during initial planning, not as a re-plan cycle. Possible cause: the plan was drafted too quickly in the discuss → plan pipeline without enough time on the pitfall research.
- **One Phase 5 executor went briefly off-track.** Wave 2's executor started fabricating a "consolidate 8 triggers into 5 orchestrators" plan before reading the actual 05-02 PLAN.md. Cancelled before any file writes landed; no damage. Root cause: parallel-batching reads with potentially-mutating tools in the same message — the executor's own self-imposed rule of "finish reading before any Edit/Write" was articulated *after* the incident in the SUMMARY's "lesson" note.
- **Phase 3 verification was deferred to downstream phases.** SC-1 end-to-end Phase 3 smoke was never run directly; verification came transitively via Phase 4 and Phase 5 live smokes. This is risky — if Phase 3 had a latent bug, it could have only surfaced two phases later.

### Patterns Established

- **Sequential-wave execution with checkpoint heartbeats** — `[checkpoint] phase N wave M/X plan PP starting/complete (P/Q done)` lines bracketing each subagent dispatch. Keeps SSE streams warm and gives a stable grep handle for partial transcripts.
- **Locked-decision banner pattern in CONTEXT.md** — D-NN entries with one-line rationale, locked-by date, and (sometimes) supersession by later scope reductions (e.g., D-10 superseded 2026-05-11). The audit trail of why a decision was made is preserved next to the decision itself.
- **Producer/consumer envelope contracts in `_meta` cells** — `_meta.eviction_log` JSON arrays as the wire format between sidebars (producer) and weekly archive triggers (consumer). The schema is documented in the producing plan's SUMMARY and verified by an assertion in the consumer's tests.
- **Inline `SIDEBAR_BODY` template-literal pattern** — all HtmlService sidebars (showCharInfoSidebar, showBankCoinSidebar, showSearchSidebar, showEvictionSidebar) use an inline String.raw template constant rather than a companion .html file. Single source of truth per sidebar; trivially XSS-defended via inline `escapeHtml`; future ergonomic improvement (extract to `sidebars/*.html`) deferred uniformly.
- **TRIGGER_GLOBALS ↔ Code.ts assertExportsMatchGlobals() CI check** — every Apps Script trigger function name must appear in both `Code.ts` (re-exports for Apps Script's global-name discovery) and `build.mjs` `TRIGGER_GLOBALS` (the build footer that re-exports them). The build asserts equality between the two lists; CI fails if they drift. Caught at least 2 desync incidents during Phase 5.
- **Cumulative-survival tests in `installTriggers.test.ts`** — each new phase's wiring added 2–3 assertions that prior phases' triggers were still registered. Caught zero regressions but the safety net is psychologically valuable for sequential-wave execution where you're building on prior waves' state.

### Key Lessons

- **Spec ≠ contract.** The single most expensive lesson of v1.0: Google's `/token` endpoint requires `client_secret` for Desktop OAuth clients even with PKCE, contrary to the OAuth 2.0 spec. RESEARCH.md §4.1's "PKCE replaces client_secret" was spec-correct and contract-wrong. Locked into PROJECT.md Key Decisions #10. Generalized lesson: always verify against the actual server before locking architectural assumptions about it.
- **EV certs no longer grant SmartScreen reputation.** Microsoft removed the EV perk in March 2024. STACK.md + PITFALLS.md #2 had recommended EV; corrected during Phase 2. Default became unsigned + walkthrough; SignPath OSS application running in parallel. Generalized lesson: vendor benefits expire silently; reverify trust assumptions when they're load-bearing.
- **`drive.file` is non-sensitive — no verification audit needed.** Locking the scope at `drive.file` (instead of broader `drive`/`spreadsheets`) saved a Google verification audit + the Production-state consent screen makes refresh tokens long-lived. The constraint is annoying (workbook must be picked via Drive Picker, not by ID) but the trade is correct for a 12-person guild.
- **`drive.file` write-access propagation can take ~50 minutes after Reauthorize.** The read side returns immediately; only `batchUpdate` writes return 401 during the window. Probe goroutine outside the batch mutex with 90-min timeout is the shipped fix. Generalized lesson: distributed-system propagation is real even on Google's edge.
- **Live smokes catch what tests don't.** Three fix-packs landed during v0.4.0 smoke (setWarningOnly default, missing menu items, wiki template variants) — none were caught by the test suite. Test mocks can't reproduce real DOM events, real Apps Script execution context, or real wiki HTML. Keep doing live smokes per phase.
- **The cancellation safety net works.** When the Wave 2 executor went off-track, the user's intervention cancelled the parallel tool batch before any file writes landed. The discipline of "no parallel batching of reads with mutating tools" is articulated; need to bake it into the gsd-executor agent's prompt for v1.1+.
- **Mid-execution scope reductions are fine when scoped + recorded.** Two locked reductions during Phase 5 (instructional images, SEARCH-03 inline staleness) plus one earlier (ENRICH-09 courtesy emails). Each one took ~5 minutes to record in CONTEXT.md + PLAN.md banner + SUMMARY.md note. The cost is trivial vs. the discipline buy.

### Cost Observations

- **Model mix this milestone:** primarily Opus 4.7 (1M context) for orchestrator + most subagents. No model-routing optimization done. Future opportunity: route executor subagents to Sonnet 4.6 for the per-plan execution loops; reserve Opus for orchestration + planning + review.
- **Sessions:** roughly one /clear per phase ship; the Phase 5 execution + ship + milestone-close arc spans this single long session (~6 hours wall-clock with the user's smoke-test interludes).
- **Notable efficiency observation:** the discuss → plan → execute pipeline shines on small phases (1–5 plans). The Phase 2 ten-plan execution required more careful wave grouping; Phase 5's five sequential one-plan waves were trivial to manage by comparison.
- **Notable inefficiency observation:** the milestone audit step did meaningful work (caught the REQUIREMENTS.md drift) but the workflow's `gsd-sdk` delegation calls were dead weight on this machine (no SDK installed). For projects using `gsd-tools.cjs` legacy harness, the audit workflow needs an explicit "no SDK; do it manually" branch.

---

## Milestone: v1.0.1 — Installer + Permissions Hardening

**Shipped:** 2026-05-12
**Phases:** 3 | **Plans:** 12 | **Commits:** 63 (since v1.0.0) | **Timeline:** 2 calendar days

### What Was Built

A small surgical patch milestone closing v1.0 carry-over debt: NSIS pre-install shim that gracefully stops a running watcher before file overwrite (retires the manual `taskkill` workaround); `_meta.guild_admins` allowlist with code-enforced officer-only eviction + admin-management UX with owner-floor lockout protection; sidebar inline-JS test infrastructure (JSDOM in vitest + 4 net-new sidebar test files); recent-search MRU migrated from CacheService 25-min TTL to durable `PropertiesService.getUserProperties()`; 8 retroactive Phase 3+4 SUMMARY.md files retiring v1.0's documentation debt. Schema gates untouched throughout (`_meta.schema_version=3`, `WatcherMaxSchemaVersion=3`).

Phase-by-phase output:

- **Phase 6 (v1.0.1 binary tag)** — `internal/system` Windows named-event IPC package (`SignalShutdown` + `WaitForShutdown`) + `--quit` CLI flag + NSIS pre-install shim with 10s `tasklist`+`nsExec` poll loop + `taskkill /F` fallback + `docs/troubleshooting.md` manual-workaround retirement + GitHub Release `v1.0.1` with installer + bare binary + `latest.json`. UAT-verified on Azure VM against real running watchers (both `--quit` and legacy paths exercised).
- **Phase 7 (clasp push)** — `apps-script/src/lib/admin.ts` policy primitives (9 public exports + 20-scenario vitest suite) + `triggers/showAdminMgmtSidebar.ts` (300px sidebar + 3 `google.script.run` callbacks + owner-floor enforcement) + eviction sidebar admin guards + `onOpen` lazy bootstrap + 2 new menu items + dev-workbook 5-hook smoke (5/5 PASS with peer-admin in second browser session).
- **Phase 8 (clasp push)** — `vitest.config.ts` `environment: 'jsdom'` + `mountSidebar()` helper with indirect-eval + nested-`<script>` extraction + 4 net-new sidebar inline-JS test files (5/5 shipping sidebars covered) + `searchIndex.ts` MRU swap to `PropertiesService.getUserProperties()` + 8 retroactive Phase 3+4 SUMMARY.md files. Test count 297 → 336 (+39).

### What Worked

- **Three-phase, single-ship-gate-per-phase structure.** No cross-phase tangle: Phase 6 was Go-only, Phases 7+8 were apps-script-only. Schema untouched everywhere — no migration logic, no `WatcherMaxSchemaVersion` bump. The "patch milestone" framing held; resisted scope creep effectively despite Phase 6 UAT surfacing 4 v1.0.2 candidates (deferred cleanly to backlog 999.13–999.16).
- **Wave-based execution with the discuss → plan → execute pipeline.** Phase 6 ran 5 plans in 4 waves; Phase 7 ran 3 plans in 2 waves; Phase 8 ran 4 plans in 2 waves. Wave-1 + Wave-2 worktree merges proved their value on Phase 8 (Plan 08-01 + 08-03 + 08-04 in parallel). One manual retry was required after a git merge silently no-op'd; recoverable, didn't break the pattern.
- **Locked-decisions banner in CONTEXT.md, carried forward.** D-NN decisions in each Phase CONTEXT.md (D-01..D-06 in Phase 7, etc.) gave subagents a stable reference for "why this and not that." Pattern transferred cleanly from v1.0 with no friction.
- **Live smoke for permission-sensitive code.** Phase 7's dev-workbook smoke — actually using two browser sessions (workbook owner `boejowen@gmail.com` + peer `joseph.bowen2@gmail.com`) — caught the latent v1.0 OAuth manifest bug (`userinfo.email` missing). Hook 3 fail revealed the issue immediately; without live smoke this would have shipped to all guildies and broken every admin guard for everyone except the workbook owner.
- **Cumulative test count discipline (297 → 336).** Each plan added tests cumulatively; no test deletions; every wave merge required `npm test` GREEN. The TRIGGER_GLOBALS ↔ Code.ts sync assertion (Phase 3 d0a2645) caught a desync attempt during Plan 07-02 and blocked the build until corrected — pattern is paying dividends.
- **TEST-02 historical correction in-flight.** Plan 08-02 caught that the original TEST-02 wording listed `showThemePickerModal` as a sidebar; it's actually a `showModalDialog`. Correcting the requirement wording inline (not pretending the original was right) + identifying the actual 5th sidebar (Admin-Mgmt) + deferring its inline-JS test to v1.1 was the right call. Resisted the temptation to retrofit history.

### What Was Inefficient

- **No audit step run.** v1.0.1 closed in yolo mode without a `/gsd-audit-milestone` pass. The phase verifier (Phase 8: 5/5 must-haves) + dev-workbook smoke (Phase 7: 5/5 hooks) provided sufficient signal for this 8-requirement patch, but for v1.1+ feature milestones the audit should run.
- **Phase 6 UAT surfaced 4 v1.0.2 candidates that weren't anticipated.** UAT Findings C, D, F, H (999.13–999.16) are real edge cases — boot-time `invalid_grant` traps, pre-`OnReady` controller calls, UTF-8 BOM in config, console-detach foreground death. Risk: these were latent in v1.0 and would have been latent in v1.0.1 too without the live VM UAT. Discipline: keep doing live install UAT per installer-touching phase.
- **Manifest scope gap (`userinfo.email`) was a v1.0 bug latent until Phase 7.** The latent `initiated_by='unknown'` audit-log fallback ran across every v1.0-era eviction silently. Future rule: when introducing a new `Session.getEffectiveUser()` consumer, audit the manifest scopes the same time you audit the call site.
- **Wave-1 merge required manual retry on Phase 8.** Initial `git merge` silently no-op'd; the safety check + retry caught it but cost ~5 minutes. Worth checking if this is a Windows worktree quirk or a deeper pattern.
- **One mid-execution test regression in `showSearchSidebar.test.ts`.** Tests 4/6 broke during Phase 8 because jsdom event-firing semantics drifted from the CacheService-mock baseline. Fixed inline as `ec68e3f` but should have been caught by the dependency analysis at Plan 08-01 (any test changing `mountSidebar` infra could ripple into existing sidebar tests). Generalized lesson: when a test helper changes shape, re-run the FULL suite under the new helper before merging, not just the helper's own tests.

### Patterns Established

- **Owner-floor lockout pattern** — `_meta.workbook_owner_floor` row + dual enforcement (server-side throw `'owner_floor_protected'` + client-side Remove-button suppression) + visual `(owner)` annotation. Generalizes to any future "this user is special, cannot be removed by peers" pattern.
- **Mid-smoke OAuth-scope discovery is a real failure mode** — Phase 7 Hook 3 fail led to commit `544bef8` (manifest scope addition) within the smoke session. Pattern: when a code path is correctly fail-closed but you get unexpected fail-closure during live smoke, suspect the platform contract (manifest, OAuth scopes, environment vars) before suspecting the new code.
- **TEST-02-style historical correction** — when a requirement's wording turns out to be empirically wrong (theme picker is not a sidebar), correct it inline in REQUIREMENTS.md with a "Historical correction" callout instead of either ignoring it or pretending the wording was always different. Audit trail preserved + new wording accurate.
- **Sidebar inline-JS test scaffold** — `mountSidebar()` helper with indirect-eval + nested-`<script>` querySelectorAll extraction. Reusable for every future sidebar; pattern documented in Phase 8 PATTERNS.md.

### Key Lessons

- **Schema untouchedness is a feature, not an accident.** v1.0.1's "no schema bump" framing held end-to-end because Phase 6 was Go-only (binary release reason for tag, not schema reason) and Phases 7+8 used only extend-only `_meta` rows + persistence-layer changes. The CONTEXT.md "verification hook 5" grep gates (`migrations.ts` schema_version write count, `WatcherMaxSchemaVersion` value) caught every drift attempt. Generalized lesson: bake the schema-immutability invariant into the verification hooks for any milestone that promises "no schema change."
- **Two-account UAT catches what single-account UAT can't.** Phase 7's dev-workbook smoke with `joseph.bowen2@gmail.com` in a second browser session was load-bearing — single-account testing would have missed the `userinfo.email` manifest gap because the workbook owner's identity always resolves correctly (different code path). Future rule: any feature involving Apps Script identity comparisons (`Session.getEffectiveUser`, `getOwner`, ACL checks) MUST be UAT'd with at least 2 distinct Google accounts.
- **`gsd-sdk` is still not on this machine.** v1.0 retrospective flagged the gsd-sdk-delegation calls as dead weight; v1.0.1 ran into the same wall during this `/gsd-complete-milestone` invocation. The workflow's explicit fallback ("if SDK missing, do it manually") branch is what shipped this archive. Future rule: skill workflows should treat the SDK as optional, not assumed.
- **Patch milestones are sized right at 2 days.** 8 requirements across 3 phases × 4 plans avg = 12 plans = ~6 plans/day. Comparable to v1.0's 2.8 plans/day average across 11 days, but the v1.0.1 plans were uniformly smaller (Plan 08-04 was pure doc-backfill; Plan 06-04 was -479 / +995 bytes of docs; Plan 08-03 was ~3 commits including the test). Heuristic: a "patch milestone" should be 2 calendar days, ≤15 plans, ≤10 requirements, no schema change. If any of those bound breaks, it's a "minor milestone" (v1.1) instead.

### Cost Observations

- **Model mix this milestone:** still primarily Opus 4.7 (1M context) for orchestrator + executor + planner + reviewer subagents. Sonnet routing for executors still not implemented; v1.0 retrospective noted this as future opportunity. Worth experimenting in v1.1+.
- **Sessions:** v1.0.1 spanned ~3 distinct sessions (one per phase). The 2-day calendar window included real-world delays (waiting for CI runs, dev-workbook smoke setup, etc.) — not 2 full days of model time.
- **Notable efficiency observation:** the discuss → plan → execute pipeline shines even harder on these tiny phases. Phase 7's discuss-phase made 4 decisions in one user delegation ("make whichever choice(s) that will make the end-user experience as simple as possible"); 6 phase decisions emerged (D-01..D-06) cleanly without further question.
- **Notable inefficiency observation:** roadmap audit step in `/gsd-progress` would have caught the 56 → 69 REQ-ID drift mentioned in v1.0 backlog. Carried over unfixed into v1.0.1; not blocking but a paper-cut for anyone reading historical numbers.

---

## Cross-Milestone Trends

*This section accumulates as more milestones close. After v1.0.1, it has two data points — patterns are starting to emerge but anchor heavily on v1.0 (5 phases, 11 days, 31 plans) vs. v1.0.1 (3 phases, 2 days, 12 plans, patch-sized).*

### Decision Quality

| Milestone | Locked decisions | Revised mid-phase | Reverted post-ship |
|-----------|------------------|---------------------|---------------------|
| v1.0 | 11 (PROJECT.md Key Decisions) + 12 (Phase 5 CONTEXT D-NN) | 3 scope reductions (ENRICH-09 waived, INST-05/DOC-03 image-drop, SEARCH-03 Path 2) | 0 |
| v1.0.1 | 11 (CONTEXT D-NN across Phases 6/7/8) | 2 mid-phase fixes (Plan 07-03 Test 12 reframe; Plan 07-03 mid-smoke OAuth manifest scope add) | 0 |

### Phase Velocity

| Milestone | Phases | Plans | Days | Plans/day | Notes |
|-----------|--------|-------|------|-----------|-------|
| v1.0 | 5 | 31 | 11 | 2.8 | One-person project; sequential phases |
| v1.0.1 | 3 | 12 | 2 | 6.0 | Patch-sized plans (doc backfill, persistence swap, single-IPC primitive); two-wave parallelism on Phases 7+8 |

### Test Coverage Trajectory (apps-script vitest)

| Phase end | Test count | New tests this phase |
|-----------|-----------|----------------------|
| Phase 4 (v0.4.0) | ~110 (Go) + baseline TS | (varied — multi-language project) |
| Phase 5 plan 05-01 | 229 | +29 vs. baseline |
| Phase 5 plan 05-02 | 246 | +17 |
| Phase 5 plan 05-03 | 283 | +37 |
| Phase 5 plan 05-04 | 297 | +14 |
| v1.0 ship | 297 | 0 (05-05 was docs-only) |
| Phase 7 plan 07-01 | 317 | +20 (admin.test.ts 20 scenarios) |
| Phase 7 plan 07-02 | 324 | +7 (adminMgmtSidebar.test.ts 7 scenarios) |
| Phase 7 plan 07-03 | 324 | 0 (reseeded eviction tests; net 0) |
| Phase 8 plan 08-01 | 327 | +3 (mountSidebar + getUserProperties mocks) |
| Phase 8 plan 08-03 | 328 | +1 (vi.useFakeTimers 25-min persistence) |
| Phase 8 plan 08-02 | 336 | +8 (4 sidebar inline-JS × 2 cases) |
| v1.0.1 ship | 336 | +39 across v1.0.1 |

### Watcher LOC Trajectory (Go)

| Milestone | LOC | Notes |
|-----------|-----|-------|
| v1.0 | 14,189 | Includes tests; production-only would be substantially less |
| v1.0.1 | 14,507 | +318 LOC: new `internal/system` Windows named-event IPC package + `--quit` CLI handler + listener goroutine + 5 Windows-only tests |

### TypeScript LOC Trajectory (apps-script)

| Milestone | LOC | Notes |
|-----------|-----|-------|
| v1.0 | ~11,000 | Baseline at v1.0.0 |
| v1.0.1 | 13,266 | +~2k LOC: `lib/admin.ts` (357) + `admin.test.ts` (462) + `triggers/showAdminMgmtSidebar.ts` (263) + `adminMgmtSidebar.test.ts` (219) + 4 sidebar inline-JS test files + 8 retroactive Phase 3+4 SUMMARY.md backfills |

### Tags Shipped

| Milestone | Watcher tags | Apps-script deploy |
|-----------|--------------|---------------------|
| v1.0 | v0.1.0 → v0.2.0/v0.2.1 → v0.3.0 → v0.4.0/v0.4.0-rc1 → v1.0.0 | clasp-push at end of Phase 5 to dev workbook |
| v1.0.1 | v1.0.1 (binary release; pushed by Phase 6 ship gate 2026-05-11) | clasp-push for Phase 7 (Plan 07-03) + Phase 8 to dev workbook |

### Milestone Sizing Heuristic (emerging from 2 data points)

A "patch milestone" should hit roughly:

- ≤15 plans
- ≤10 requirements
- ≤2 calendar days
- 0 schema changes
- Plans/day ≥ 5 (smaller plans drive higher throughput)

If 2+ of these bound break, it's a "minor milestone" (v1.1-style) instead. v1.0.1 fit cleanly in this envelope; v1.0 didn't (it was the foundation milestone).

---

*Last appended: 2026-05-12 during `/gsd-complete-milestone v1.0.1`.*
