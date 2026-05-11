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

## Cross-Milestone Trends

*This section accumulates as more milestones close. After v1.0, it has one data point — patterns will emerge from v1.1+ onward.*

### Decision Quality

| Milestone | Locked decisions | Revised mid-phase | Reverted post-ship |
|-----------|------------------|---------------------|---------------------|
| v1.0 | 11 (PROJECT.md Key Decisions) + 12 (Phase 5 CONTEXT D-NN) | 3 scope reductions (ENRICH-09 waived, INST-05/DOC-03 image-drop, SEARCH-03 Path 2) | 0 |

### Phase Velocity

| Milestone | Phases | Plans | Days | Plans/day | Notes |
|-----------|--------|-------|------|-----------|-------|
| v1.0 | 5 | 31 | 11 | 2.8 | One-person project; sequential phases |

### Test Coverage Trajectory (apps-script vitest)

| Phase end | Test count | New tests this phase |
|-----------|-----------|----------------------|
| Phase 4 (v0.4.0) | ~110 (Go) + baseline TS | (varied — multi-language project) |
| Phase 5 plan 05-01 | 229 | +29 vs. baseline |
| Phase 5 plan 05-02 | 246 | +17 |
| Phase 5 plan 05-03 | 283 | +37 |
| Phase 5 plan 05-04 | 297 | +14 |
| v1.0 ship | 297 | 0 (05-05 was docs-only) |

### Watcher LOC Trajectory (Go)

| Milestone | LOC | Notes |
|-----------|-----|-------|
| v1.0 | 14,189 | Includes tests; production-only would be substantially less |

### Tags Shipped

| Milestone | Watcher tags | Apps-script deploy |
|-----------|--------------|---------------------|
| v1.0 | v0.1.0 → v0.2.0/v0.2.1 → v0.3.0 → v0.4.0/v0.4.0-rc1 → v1.0.0 | clasp-push at end of Phase 5 to RiverFAIL VanFRAUD dev workbook |

---

*Last appended: 2026-05-11 during `/gsd-complete-milestone v1.0`.*
