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

## Milestone: v2.0 — "Off Google"

**Shipped:** 2026-05-31
**Phases:** 6 (Phases 11–16) | **Plans:** 29 | **Timeline:** 4 calendar days (2026-05-28 → 2026-05-31)

### What Was Built

The whole product, replatformed off Google. A self-hosted Go + SQLite backend on a Hetzner VPS (LIVE at `api.squirebot.quest`) replaced the Sheet as both data store and query layer; a static SvelteKit site (LIVE at `squirebot.quest`) replaced the Sheet's UI; the watcher was re-pointed off Google and ~8k LOC of OAuth/Sheets/Picker code deleted; Discord OAuth2 login + officer web forms ported v1's enforcement; and Google was decommissioned (triggers + OAuth client deleted, Sheet abandoned in place) — meeting the "Off Google" goal.

Phase-by-phase output:

- **P11 — Backend foundation + ingest API** — single Go binary behind Caddy auto-HTTPS on a Hetzner VPS, `goose` SQLite schema, per-guildie hashed bearer-token auth, atomic-replace ingest endpoint, nightly R2 backup + drilled restore. PocketBase evaluated and rejected (hand-rolled `net/http` + `modernc.org/sqlite`).
- **P12 — Enrichment migration** — daily PigParse + weekly P1999 wiki as in-process scheduled jobs; the 4 pure parsers + `politeFetch` byte-parity-ported from Apps Script (SHA-1 byte-identical; verified against the LIVE APIs).
- **P13 — Watcher re-target** — sink re-pointed to `backend.Ingest` (raw UTF-8 POST), the entire Google stack DELETED (binary 57% smaller, no Google secret), native "paste your guild code" onboarding via the auto-updater, WATCH-11 first-launch migration.
- **P14 — Web frontend** — versioned Go read API + a greenfield `web/` SvelteKit SPA: one reusable DataGrid ×4, fuzzy search + "did you mean?", escaped rich-HTML tooltips, 5-theme EQ aesthetic. WEB-02 parity proven by Go table-tests translated from the v1 vitest fixtures.
- **P15 — Admin web forms + login** — hand-rolled Discord OAuth2 login (guild-membership-gated, per-user identity captured), + the 3 officer write forms with authorize-under-transaction + `audit_log`.
- **P16 — Cutover + decommission** — a fresh-start char-meta form (no Sheet backfill), the `v2.0.0` release published + the guild flipped via auto-update, Google decommissioned, proven by a committed decommission checklist.

### What Worked

- **Front-loading the ingest path.** The deliberate sequencing — P11 (ingest) + P13 (watcher re-target) before the polished P14 frontend and P15 forms — restored the dark guild's data pipeline first. By the time the website existed there was already something to show; by cutover the first real guildie (Slampeach: 135 items + 54 spells) had validated the whole EQ→fsnotify→upload→auth→SQLite path end-to-end.
- **TDD RED→GREEN as the default execution rhythm.** Nearly every build plan committed a failing test (RED) then the implementation (GREEN) as separate atomic commits. The backend's parser port leaned on this hardest: **byte-parity proof** (SHA-1 byte-identical, exact-count cross-checks by running the TS parsers in Node) was the strongest possible signal that the Apps Script→Go transliteration was faithful.
- **Clone-the-reviewed-original as a quality pattern.** P16's char-meta trio was a line-by-line clone of the already-code-reviewed P15 bank-coin trio (same RequireSession, audited-tx, pure-helpers-in-`.ts`, node-only tests) — including copying the CR-01-safe `type="text" inputmode="numeric"` level input verbatim. Cloning a reviewed-clean original carries its fixes forward for free.
- **Compute-parity via translated fixtures.** WEB-02 (gear/spell progression) was proven by translating the v1 `buildGearCheck`/`buildSpellCheck` vitest seed-arrays + expected tuples into Go table-tests over a temp DB — the cleanest way to prove a reimplementation matches the original semantics without trusting "looks right."
- **Locked decisions surfaced and held.** The "consolidated DataGrid ×4, never per-character views" lock (carried from v1) and the "watcher stays a static bearer token, Discord only at link-time" constraint (999.31) both prevented entire categories of rework. The PocketBase spike verdict was captured in CONTEXT and never re-litigated.
- **The reframe-on-reality discipline.** When 16-CONTEXT recognized the guild had been dark on the Sheet since 2026-05-15, it voided the planned shadow-soak/backfill/parity dance and reframed CUTOVER-01..04 to a fresh-start form + abandon-in-place — turning a 1–2 week soak into a same-day cutover with no loss of safety (inventory + enrichment self-heal).

### What Was Inefficient

- **The node-only web test suite is DOM-blind — a recurring, expensive lesson.** Because the repo deliberately has no `@testing-library/svelte` (the toolchain-install rule), web vitest exercises pure helpers + `.svelte` source assertions, not a real DOM. This let **green suites hide real crashing bugs twice**: P15 shipped 165 green tests yet two BLOCKERs (a BankCoinForm number-input coercion crash, an eviction epoch-seconds date) that only a browser/code-review caught; P16's CR-01 level input had the same hazard (guarded by copying the reviewed-safe input + a number/null regression, then a live browser smoke). The mitigation — always code-review + browser-smoke a frontend before calling it verified — works, but it's a standing tax the test suite should be carrying.
- **Human-gated ops plans had to be reconciled after the fact.** 16-02/03/04 are `autonomous:false`; they were executed conversationally by the orchestrator (over SSH) + the maintainer (plaintext minting + Google-console actions in their own session), NOT a gsd-executor. Their SUMMARYs were **back-filled** afterward to keep GSD state consistent (and to stop a re-run from re-publishing/re-deploying). It worked, but the workflow has no first-class slot for "the human did this step; record it" — so the bookkeeping was manual.
- **A deploy foot-gun cost a live debugging detour.** The 16-03 deploy hit a blank white screen because `sudo cp -r` left the `_app/` dirs `drwx------` (root umask) so Caddy's user couldn't traverse them → 200.html served for every JS asset. Diagnosed + fixed live (`chmod -R a+rX`) and documented (`backend-deploy.md §7.5`), but it's the kind of thing a deploy script should enforce, not a runbook step to remember.
- **Live UAT checklists never formally ticked.** P12/P14/P15 each have a HUMAN-UAT file; most of P14/P15's smokes were actually exercised during the 16-03 live deploy (login gate 401, Slampeach end-to-end, char-meta CR-01 browser smoke PASSED), but the checklists weren't ticked — so the artifacts read "pending" even though the behavior was observed. Carried as acknowledged deferred items at close.

### Patterns Established

- **Byte-parity port verification.** When transliterating logic across languages (Apps Script TS → Go), prove equivalence by (a) hashing identical inputs to identical digests and (b) running BOTH implementations against the same fixtures and asserting exact output counts/tuples. Stronger than any "looks correct" review.
- **Clone-the-reviewed-original.** When a new surface is structurally a sibling of an already-code-reviewed one, clone it line-by-line (including its safety quirks and comments) rather than re-deriving — the review's fixes ride along. (Bank-coin trio → char-meta trio.)
- **Authorize-under-transaction (WR-04 close).** Officer/owner-floor write handlers re-check the caller's authority as their FIRST in-tx SELECT (not just at the route), closing the TOCTOU window between gate and write.
- **One version-compare doctrine, per side.** The watcher's `IsNewer` and the server's `ingest.IsOlder` are deliberate *separate copies* with the same pre-release-ranking direction but opposite fail-safety (watcher fails-closed on the update decision; server fails-closed on the gate) — no cross-import. Load-bearing for the coordinated auto-update flip.
- **Server-side ground-truth verification of every write.** UI/visual success is never trusted alone (a direct consequence of the DOM-blind-suite lesson) — the char-meta smoke and the Slampeach ingest were both confirmed by reading the rows back out of SQLite over SSH.
- **Reframe success criteria when the world changed under the plan.** A CONTEXT doc can supersede a phase's Goal/Success-Criteria when the premise (here: a live Sheet to soak against) no longer holds — recorded explicitly so the requirements archive can map the reframed outcome.

### Key Lessons

1. **A green test suite is not "verified" when the suite can't see the DOM.** This caused real, shipping-blocking bugs in P15 and a near-miss in P16. Until the repo adopts a DOM-capable frontend test layer (deferred by the toolchain-install rule), frontend work MUST get a code review + a live browser smoke before it's called done. This is now the single most-repeated lesson of the milestone.
2. **Front-load the path that delivers value / de-risks the most.** With the guild dark, the ingest path was both the value (data flows again) and the risk (does the whole new pipeline work?). Building it first meant every later phase was decorating a working core, not betting on one.
3. **Reframe, don't force.** The original P16 plan (shadow-soak + backfill + parity) was written when the Sheet was live; by execution it wasn't. Recognizing that and reframing to fresh-start + abandon-in-place saved ~1–2 weeks and removed an entire class of backfill bugs — at no safety cost because the data self-heals.
4. **Self-hosting trades a platform's gate for ops responsibility — mostly worth it here.** Escaping Google's brand-verification gate cost a ~$67/yr VPS + a handful of ops foot-guns (the `chmod` blank-screen, systemd secret handling, the prerelease-stuck-watcher caveat). For a system Google had simply walled off, that trade was clearly correct.
5. **Human-in-the-loop ops need a bookkeeping slot.** The `autonomous:false` cutover plans worked, but their SUMMARYs were back-filled by hand. Treating "the maintainer performed this step; here's the verification" as a first-class artifact (rather than a reconciliation afterthought) would keep GSD state honest without manual catch-up.
6. **Keep the watcher dumb on purpose.** The hard constraint that the watcher stays a static bearer token (no Discord OAuth *in* the watcher) is what makes 999.31 self-service linking safe to build next — and is the whole reason v2.0's "Off Google" doesn't quietly reintroduce a 7-day-token/refresh fragility on an unattended uploader.

### Cost Observations

- **Model mix this milestone:** still primarily Opus (1M context) for orchestrator + executor + planner + reviewer subagents; Sonnet routing for executors remained unimplemented (flagged as an opportunity since v1.0). The TDD RED→GREEN per-plan loops are the obvious candidate to route cheaper.
- **Sessions:** spread across the 4-day window via `/gsd-resume-work`; the cutover (16-02/03/04) interleaved real-world latency (release.yml build, SSH deploys, the maintainer's Google-console actions, guildie herding) rather than continuous model time.
- **Notable efficiency observation:** 29 plans in 4 calendar days (~7.25 plans/day) — the highest throughput of any milestone, driven by small TDD plans + wave parallelism (P11/P12/P14 each ran parallel Wave-1 foundation plans). Front-loading + clone-the-reviewed-original both cut planning overhead.
- **Notable inefficiency observation:** the DOM-blind-suite tax shows up as cost too — two code-review-and-fix cycles (P15's 2 BLOCKERs + 7 WARNINGs; P16's MD-01/LR-01) that a DOM-capable suite might have caught at execution time instead of review time.

---

## Milestone: v2.1 — Self-Service Watcher Linking

**Shipped:** 2026-06-02
**Phases:** 2 | **Plans:** 4 | **Commits:** 20 | **Timeline:** 2 days

### What Was Built

Self-service watcher linking on top of the v2.0 "Off Google" backend. A signed-in guildie visits `squirebot.quest/account`, clicks Generate, and gets a watcher code minted server-side against their Discord session — shown once, copy-to-clipboard, paste into the watcher. They can list their active codes (#N / created / last-seen) and revoke any one. Minting is additive (multiple PCs). The `mint-code` CLI is gone. Plus Phase 18 verified-closed three carried-forward watcher cleanups (gofmt, freeConsole, SemVer `IsNewer`).

### What Worked

- **Composing existing infrastructure.** v2.1 added almost no new primitives — it wired together P15 Discord login + sessions, `auth.MintCode`, and the `web_user`/`owner`/`guild_code` tables. The whole milestone was 2 days because the hard parts (auth, hashing, ingest) already existed.
- **Deploy-then-smoke for the DOM-blind frontend.** Honoring the standing "web vitest is DOM-blind" lesson, Phase 17 was deployed to the live VPS and browser-smoked by the user before being called done — catching exactly the class of bug (clipboard / show-once / reload-reveal) that unit tests can't see. No crashing-BLOCKER-behind-green-tests repeat of P15.
- **Same-milestone code-review-fix + redeploy.** The 4 advisory warnings from Phase 17's review were fixed and redeployed within the milestone rather than deferred, keeping `master` and prod in sync at close.
- **Verifying a verify-or-close phase against live data.** Phase 18 didn't trust the carried-forward "maintainer's watcher is stuck on 0.4.0-rc1" note — it queried the live backend (`watcher_version` telemetry) and the local registry, and found the premise was stale (the box was a disposable Azure test VM). Cheap checks debunked a phantom task.
- **Delegating gray areas with a single rule.** Phase 18's discussion locked all four gray areas in one pass under the user's "simplest guildie experience" criterion instead of a multi-turn interview — fast and faithful.

### What Was Inefficient

- **A stale residual survived from the v2.0 close into the v2.1 roadmap.** The "stuck maintainer watcher" was written into REQUIREMENTS/ROADMAP/CONTEXT and even a full PLAN before anyone checked whether it was real. It wasn't. The verify-or-close framing caught it, but the phantom still cost a discuss + plan + verification cycle. Lesson: spot-check carried-forward residuals against live state *before* they become requirements.
- **`commit_docs: false` vs. canonical artifacts friction.** Throughout the milestone I committed planning artifacts (CONTEXT/SUMMARY/VERIFICATION/plans) despite `commit_docs: false`, because leaving them uncommitted risks the known planner-clobber. Worth reconciling the config with actual practice.

### Patterns Established

- **Deploy → human-smoke → finalize** for any frontend phase (the non-autonomous browser checkpoint is now the norm, not the exception).
- **Verify-or-close phases query live state** (backend DB, registry, telemetry) rather than re-implementing or trusting notes.

### Key Lessons

- A carried-forward "residual" is a *hypothesis*, not a fact — verify it cheaply before it becomes scoped work.
- When the user hands you a deciding rule ("simplest X"), lock all open decisions in one pass and record the rule; don't keep asking.

### Cost Observations

- Model mix: ~100% opus (single-session, Opus 4.8 1M).
- Sessions: 1 continuous session (discuss → plan → execute → deploy → review-fix → milestone close across both phases).
- Notable: composing existing infra made this the cheapest milestone yet per requirement (9 reqs, 4 plans, 2 days, near-zero new watcher code).

---

## Milestone: v2.5 — Ownership Cleanup

**Shipped:** 2026-06-22
**Phases:** 2 (35–36) | **Plans:** 3

> _(Retrospective sections for v2.2 / v2.3 / v2.4 were skipped during their closes — this resumes the habit at v2.5.)_

### What Was Built
Owner-less guild banks/bots via a reserved "guild" sentinel owner (id 1000000; migration 00015 seed + backfill) so designation is decoupled from ownership and banks survive eviction (OWN-01/02/04); shared-character-safe eviction by narrowing the per-owner cascade through the existing `audit_log` `cross_owner_write` trail, with a one-shared-predicate cascade/preview parity + an all-shared-owner web fix (OWN-03). Backend + a small web change; deployed live (prod schema v15); watcher untouched, no `v*` tag.

### What Worked
- **Adversarial gates caught two real BLOCKERs before ship.** Code-review caught that the Phase-35 sentinel was guarded in the eviction *picker* but not the destructive *write path* (an officer POST could've wiped the guild bank). The plan-checker caught that narrowing the Phase-36 preview disabled the Evict button for an all-shared owner (so their code couldn't be revoked). Both were premise errors the implementer/planner missed but a fresh adversarial reader found.
- **Reusing the existing audit trail** (`cross_owner_write`) as the sharing signal kept Phase 36 migration-free.
- **One shared SQL predicate for cascade + preview** made the read/write divergence (the Phase-35 CR-01 class of bug) structurally impossible in Phase 36 — applying a lesson within the same milestone.

### What Was Inefficient
- The `--auto` discuss over-specified an edge case ("all-shared owner stays evictable") as load-bearing, which then forced a mid-plan scope expansion (backend-only → backend + web) once the plan-checker proved it couldn't be met backend-only. Surfacing that coupling in discuss would have scoped it as backend+web from the start.

### Patterns Established
- **Sentinel-row over nullable-column** for "owner-less / system-owned" semantics when the FK column is `NOT NULL` — keeps every join working, smallest blast radius.
- **Derive a fact from the existing audit trail** before adding a table — the `cross_owner_write` log was already the sharing record.

### Key Lessons
- A `--auto` discuss can manufacture a load-bearing requirement the codebase can't satisfy in the intended scope — the plan-checker is the backstop that surfaces it; budget for a possible scope ratification.
- When a phase narrows what a destructive action *does*, audit every UI gate that reads the *old* shape (the `EvictionForm` `cascadeEmpty` gate was invisible until the plan-checker traced it).

### Cost Observations
- Model mix: ~100% opus (single continuous Opus 4.8 1M session: execute P35 → review-fix → auto-advance P36 discuss→plan→execute → Claude-driven prod deploy → browser-smoke → milestone close).
- Sessions: 1 continuous session spanning two phases + a live deploy + the milestone archive.
- Notable: the adversarial-gate cost (2 code-reviews + 2 plan-checks + 2 verifiers) paid for itself by catching 2 ship-blocking BLOCKERs.

---

## Cross-Milestone Trends

*This section accumulates as more milestones close. After v2.1 it has four data points — v1.0 (5 phases, 11 days, 31 plans, foundation), v1.0.1 (3 phases, 2 days, 12 plans, patch), v2.0 (6 phases, 4 days, 29 plans, replatform), and v2.1 (2 phases, 2 days, 4 plans, compose-existing-infra feature). Trend: once the platform exists, feature milestones get dramatically cheaper — v2.1 shipped a full self-service feature in 2 days by wiring together v2.0 primitives.*

### Decision Quality

| Milestone | Locked decisions | Revised mid-phase | Reverted post-ship |
|-----------|------------------|---------------------|---------------------|
| v1.0 | 11 (PROJECT.md Key Decisions) + 12 (Phase 5 CONTEXT D-NN) | 3 scope reductions (ENRICH-09 waived, INST-05/DOC-03 image-drop, SEARCH-03 Path 2) | 0 |
| v1.0.1 | 11 (CONTEXT D-NN across Phases 6/7/8) | 2 mid-phase fixes (Plan 07-03 Test 12 reframe; Plan 07-03 mid-smoke OAuth manifest scope add) | 0 |
| v2.0 | 6 (PROJECT.md Key Decisions) + per-phase CONTEXT D-NN across P11–16 | 1 host re-pick (Oracle → Hetzner, pre-build) + 1 deploy re-pick (Cloudflare Pages → apex-on-Caddy, P14) + the P16 CUTOVER-01..04 reframe | 0 |

### Phase Velocity

| Milestone | Phases | Plans | Days | Plans/day | Notes |
|-----------|--------|-------|------|-----------|-------|
| v1.0 | 5 | 31 | 11 | 2.8 | One-person project; sequential phases |
| v1.0.1 | 3 | 12 | 2 | 6.0 | Patch-sized plans (doc backfill, persistence swap, single-IPC primitive); two-wave parallelism on Phases 7+8 |
| v2.0 | 6 | 29 | 4 | 7.25 | Highest throughput yet — small TDD RED→GREEN plans + Wave-1 parallelism on P11/P12/P14; a full replatform (backend + website + watcher re-target) in 4 days, with the cutover interleaving real-world latency |

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
| v2.0 (watcher) | shrank materially | The ~8k-LOC Google stack (`internal/auth/sheet/scaffold/picker/wizard/heartbeat` + reauth.go, 41 files) DELETED; ~1k re-target LOC added (`internal/backend/credstore/onboarding`); go.mod shed the oauth2/google-api tree. **Binary 7.07 MB vs the pre-deletion 16.44 MB = 57% smaller; 0 Google strings.** (A new Go *backend* — `internal/backendsrv/*` + `cmd/squirebot-server` — was also built this milestone, separate from the watcher.) |

### TypeScript LOC Trajectory (apps-script)

| Milestone | LOC | Notes |
|-----------|-----|-------|
| v1.0 | ~11,000 | Baseline at v1.0.0 |
| v1.0.1 | 13,266 | +~2k LOC: `lib/admin.ts` (357) + `admin.test.ts` (462) + `triggers/showAdminMgmtSidebar.ts` (263) + `adminMgmtSidebar.test.ts` (219) + 4 sidebar inline-JS test files + 8 retroactive Phase 3+4 SUMMARY.md backfills |
| v2.0 | 13,266 (apps-script, frozen) → **decommissioned** | The apps-script project (+ its 336 vitest suite) was retired with the Sheet at cutover. NEW frontend LOC moved to `web/` (SvelteKit/TS, node-only vitest — 200 tests at close); the enrichment parsers + politeFetch were ported from this apps-script TS to Go (`internal/backendsrv/enrich`). |

### Tags Shipped

| Milestone | Watcher tags | Apps-script deploy |
|-----------|--------------|---------------------|
| v1.0 | v0.1.0 → v0.2.0/v0.2.1 → v0.3.0 → v0.4.0/v0.4.0-rc1 → v1.0.0 | clasp-push at end of Phase 5 to dev workbook |
| v1.0.1 | v1.0.1 (binary release; pushed by Phase 6 ship gate 2026-05-11) | clasp-push for Phase 7 (Plan 07-03) + Phase 8 to dev workbook |
| v2.0 | v2.0.0 (the clean Google-free binary; published 2026-05-31 at the Phase 16 cutover) | n/a — apps-script DECOMMISSIONED; replaced by VPS deploys of `squirebot-server` + the `web/` static bundle (served by Caddy) |

### Milestone Sizing Heuristic (3 data points)

A "patch milestone" should hit roughly:

- ≤15 plans
- ≤10 requirements
- ≤2 calendar days
- 0 schema changes
- Plans/day ≥ 5 (smaller plans drive higher throughput)

If 2+ of these bound break, it's a "minor milestone" (v1.1-style) or larger. v1.0.1 fit cleanly in this envelope; v1.0 didn't (foundation). **v2.0 is a third archetype — a "replatform milestone":** 29 plans / 26 requirements / 4 days / multiple schema migrations (00001→00004) / 7.25 plans/day. It breaks every patch bound but shipped fast because the plans were small + TDD'd + wave-parallel — high plan count and high throughput aren't mutually exclusive when each plan is a tight RED→GREEN slice.

### Recurring Lesson Watch (verified across milestones)

- **Live smokes / code review catch what tests don't.** v1.0 (3 fix-packs in the v0.4.0 smoke), v1.0.1 (the `userinfo.email` manifest gap surfaced by two-account UAT), v2.0 (the P15 node-suite-invisible BLOCKERs + the deploy blank-screen). Three milestones, same lesson — for v2.0 it sharpened into "the node-only web suite is DOM-blind; a frontend isn't verified without a code review + browser smoke."
- **Spec ≠ contract / reverify load-bearing external assumptions.** v1.0's Google `client_secret`-with-PKCE; v1.0.1's EV-cert reverification; v2.0's brand-verification gate that ultimately forced the whole "Off Google" replatform. External platform behavior shifts under you — when it's load-bearing, verify against the real thing.

---

*Last appended: 2026-06-22 during `/gsd-complete-milestone v2.5`. (Note: per-milestone retrospective sections for v2.2 / v2.3 / v2.4 were skipped at their closes; the v2.5 entry resumes the habit. The Cross-Milestone Trends tables below were last refreshed at v2.1 and do not yet include v2.2–v2.5.)*
