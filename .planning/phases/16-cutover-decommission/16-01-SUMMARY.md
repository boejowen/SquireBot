---
phase: 16-cutover-decommission
plan: 01
subsystem: api
tags: [go, sqlite, svelte, sveltekit, vitest, char-meta, login-only, tdd]

# Dependency graph
requires:
  - phase: 15-admin-web-forms-login
    provides: "webauth.RequireSession + the webadmin audited-tx idiom (withTx/AppendAuditTx) + the BankCoinForm/coin.ts/FormField/postJSON bank-coin trio (the exact clone targets) + AuthGate/AUTH_GUARD_KEY"
  - phase: 11-backend-foundation
    provides: "the character table with class/level/race/is_bank_toon columns already declared (00001_init.sql) — no migration needed; the store *Store + NewTestDB harness"
  - phase: 12-enrichment-job-migration
    provides: "enrich.CLASSES value set (the RACES port follows its precedent) + compute.GearCheck/SpellCheck/bank consumers that go blank->populated once this form writes"
provides:
  - "POST /api/v1/char/meta + GET /api/v1/char/meta-list — login-only (D-03) char-meta write + pick-list (CharMetaSetHandler/CharMetaListHandler)"
  - "store.SetCharMetaTx mutator (UPDATE class/level/race/is_bank_toon WHERE id AND is_removed=0; ErrCharNotFound fail-closed) + store.CharsForMeta *sql.DB wrapper"
  - "enrich.RACES (14 P1999 race abbreviations, ported from eq-constants.ts)"
  - "the /char-meta SvelteKit route + CharMetaForm.svelte (login-only, CR-01-safe level input) + charmeta.ts pure helpers + api.ts fetchCharsForMeta/saveCharMeta wrappers"
  - "the member-accessible /char-meta shell-nav entry (under session?.authenticated, D-03)"
affects: [16-02-mint-codes, 16-03-flip-binary-browser-smoke, 16-04-decommission-checklist, milestone-close-v2.0]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Login-only char-meta write = strict clone of the bank-coin trio (RequireSession, audited-tx, pure-helpers-in-.ts, node-only tests)"
    - "Extend-only struct/SELECT growth: CharMeta + CharsWithMeta gained is_bank_toon at the right edge (compute consumers ignore the extra field)"

key-files:
  created:
    - "internal/backendsrv/store/charmeta.go — SetCharMetaTx (param ? only; level *int64 -> NULL on blank) + ErrCharNotFound + CharsForMeta wrapper"
    - "internal/backendsrv/webadmin/charmeta.go — CharMetaSetHandler (POST) + CharMetaListHandler (GET) + validCharMeta (class in CLASSES, race in RACES, level blank-or-1..60)"
    - "internal/backendsrv/webadmin/charmeta_test.go — 7 cloned coin_test.go-style tests (non-officer-write-persists, bad class/race/level, removed/missing char, list contract)"
    - "web/src/lib/charmeta.ts — pure validation/change-detection helpers cloned from coin.ts (rawToTrimmed CR-01 choke point)"
    - "web/src/lib/components/CharMetaForm.svelte — login-only form (clone of BankCoinForm; CR-01 text+numeric level input)"
    - "web/src/routes/char-meta/+page.svelte — member-accessible route page (clone of bank-coin/+page.svelte)"
    - "web/src/lib/__tests__/charmeta.test.ts — Style A pure-helper (incl. CR-01 number/null regression) + Style B .svelte source-assertion"
  modified:
    - "internal/backendsrv/enrich/eqconst.go — added var RACES (14 abbrevs, IKS load-bearing)"
    - "internal/backendsrv/store/readviews.go — extended CharMeta + CharsWithMeta by is_bank_toon (right edge) + snake_case JSON tags"
    - "cmd/squirebot-server/main.go — wired the two char/meta routes under RequireSession beside the coin block"
    - "web/src/lib/api.ts — CharMetaItem + SaveCharMetaResult interfaces + fetchCharsForMeta/saveCharMeta wrappers"
    - "web/src/lib/components/SiteShell.svelte — /char-meta nav link under session?.authenticated (D-03) + .char-meta-nav CSS"

key-decisions:
  - "Blank/omitted level -> SQL NULL (valid); a non-blank level must be 1..60 (A2, Claude's discretion) — a member may know class+race before level, and spellcheck treats NULL level as the correct unleveled skip"
  - "Extended CharMeta + CharsWithMeta by one right-edge is_bank_toon column (planner's recommendation) rather than a sibling read — the GET pick-list needs it for pre-fill; compute consumers ignore it"
  - "CharMeta gained snake_case JSON tags (it now crosses the API as the form pick-list); compute consumers use field access, unaffected"

patterns-established:
  - "Char-meta form is a line-by-line clone of the code-reviewed bank-coin trio — the CR-01 text+numeric level input and the plain-{} name render are copied verbatim"
  - "Style-B source-assertion tests must avoid putting the forbidden literal (type=number / isOfficer) in the asserted file's own comments"

requirements-completed: [CUTOVER-02]

# Metrics
duration: 38min
completed: 2026-05-31
---

# Phase 16 Plan 01: Char-metadata Web Form + Backend Write Endpoint Summary

**Login-only `POST /api/v1/char/meta` + `GET /api/v1/char/meta-list` and a `/char-meta` SvelteKit form that lets any signed-in member set class/level/race/is_bank_toon on an existing character — the Google-free fresh-start replacement for the Sheet backfill (no migration; the columns already existed).**

## Performance

- **Duration:** ~38 min
- **Started:** 2026-05-31T17:25:00Z (approx, plan read)
- **Completed:** 2026-05-31T18:03:00Z (approx)
- **Tasks:** 2 (both TDD: RED test commit -> GREEN feat commit)
- **Files modified:** 12 (7 created, 5 modified)

## Accomplishments
- Backend write surface: `CharMetaSetHandler` (POST) + `CharMetaListHandler` (GET), login-only under `RequireSession` (D-03 — never `RequireOfficer`/`IsOfficer`), server-side value-set validation (`class ∈ enrich.CLASSES`, `race ∈ enrich.RACES`, level blank->NULL or 1–60), one BEGIN IMMEDIATE tx composing `SetCharMetaTx` + a `char_meta_set` audit row. Parameterized `?` SQL only.
- `enrich.RACES` ported verbatim from `eq-constants.ts` (14 abbreviations; `IKS` is load-bearing — gearcheck keys the Iksar tier on it). `CharMeta`/`CharsWithMeta` extended by `is_bank_toon` at the right edge (extend-only) for the form's pre-fill.
- Frontend: a `/char-meta` route + `CharMetaForm.svelte` (a strict clone of `BankCoinForm`) with the CR-01-safe `type="text" inputmode="numeric"` level input, class/race `<select>`s, and an `is_bank_toon` checkbox; pure helpers in `charmeta.ts`; `fetchCharsForMeta`/`saveCharMeta` wrappers over the existing credentialed `getJSON`/`postJSON` cores.
- Member-accessible nav: a `/char-meta` link in `SiteShell` under `{#if session?.authenticated}` (NOT the officer block) — D-03 honored at the nav layer.
- Tests: 7 cloned Go handler tests (incl. the non-officer-write-persists proof reading the columns back + the `char_meta_set` audit assertion) and 28 web tests (Style A pure-helper incl. the CR-01 number/null regression, Style B `.svelte` source-assertions for the input type and the nav placement).

## Task Commits

Each task was committed atomically (TDD: test -> feat):

1. **Task 1: Backend char-meta endpoint + store mutator + RACES port**
   - `64b9f9d` (test) — failing `charmeta_test.go` (RED; undefined handlers)
   - `9f6407b` (feat) — RACES + CharMeta/CharsWithMeta extension + store/charmeta.go + webadmin/charmeta.go + main.go routes (GREEN)
2. **Task 2: Frontend char-meta form (helpers + api + form + route + nav)**
   - `80fbb23` (test) — failing `charmeta.test.ts` (RED; missing module)
   - `8fbbce3` (feat) — charmeta.ts + api.ts wrappers + CharMetaForm.svelte + route + SiteShell nav (GREEN)

**Plan metadata:** _(this SUMMARY + STATE.md, committed separately)_

## Files Created/Modified
See the `key-files` frontmatter above for the full annotated list.

## Decisions Made
- **Blank level -> NULL, non-blank level 1–60 (A2):** documented in `validCharMeta` and `validateLevel`. A member may know a char's class+race before its level; `spellcheck.go` treats a NULL level (->0) as the correct unleveled skip.
- **Extend `CharMeta` + `CharsWithMeta` by `is_bank_toon` (right edge):** the planner's recommended option (vs a sibling read). The GET pick-list needs it for pre-fill; the existing `compute` consumers ignore the new field.
- **`CharMeta` gained snake_case JSON tags:** it now crosses the API boundary as the form's pick-list payload (matching the frontend `CharMetaItem` interface). The compute consumers use field access, so the tags are inert for them.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Style-B source-assertion tripped by the form's own comments**
- **Found during:** Task 2 (vitest run after first GREEN attempt)
- **Issue:** The plan's CR-01 acceptance requires asserting `CharMetaForm.svelte` source contains `inputmode="numeric"` AND does NOT contain `type="number"`, plus no `isOfficer`. My initial form copied BankCoinForm's explanatory comments verbatim, which contained the literal strings `type="number"` (in the CR-01 rationale) and `isOfficer` (in the package doc). The `.not.toContain(...)` assertions therefore failed on the comment text, not on real markup.
- **Fix:** Reworded the two comments to convey the same intent without the forbidden literals ("a number-typed input" / "never consults officer status"). The actual level input markup was already correct (`type="text" inputmode="numeric"`), and the form genuinely never calls `isOfficer` — so the assertions are now meaningful (they prove no real `type="number"` attribute / no `isOfficer` call exists).
- **Files modified:** web/src/lib/components/CharMetaForm.svelte
- **Verification:** `npx vitest run` -> 200/200 green (was 2 failing).
- **Committed in:** `8fbbce3` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 bug — a self-inflicted test failure in my own first pass, fixed before the GREEN commit).
**Impact on plan:** No scope change. The fix strengthens the source-assertion (it now reflects the file's true contents). All plan contracts honored verbatim.

## Issues Encountered
- `CharsWithMeta` is a `*Store` method (`s.db`), but the handler holds a `*sql.DB` (like `BankToonsHandler`, which uses the free-function `store.ListBankToons`). Resolved per the plan's stated fallback: added a thin `store.CharsForMeta(ctx, db)` free-function wrapper that delegates to `NewStore(db).CharsWithMeta(ctx)`.

## User Setup Required
None — no external service configuration. (The live deploy, code minting, binary release, and Google-console decommission are the separate human-gated Plans 16-02/03/04.)

## Next Phase Readiness
- **Plan 16-01 is code-complete and locally verified.** Backend: `go build`/`go vet` clean from repo root; full `go test ./internal/backendsrv/...` green (no regression from the CharMeta change). Frontend: `npm run check` 0/0, `npx vitest run` 200/200, `npm run build` emits `index.html` + `200.html`.
- **Deferred (by design, per the plan):** the REQUIRED browser smoke of the form — the node-only web suite is structurally DOM-blind to the CR-01 crash (memory `web-tests-node-only-blind-to-dom`) — is performed in **Plan 16-03 Task 1** against the live deploy ("the Plan-01 browser smoke, repeated against production"). One smoke, where it matters most.
- **Unblocks:** 16-02 (mint ~12 codes), 16-03 (publish the v2.0.0 binary + flip + the live browser smoke), 16-04 (decommission checklist). The char-meta form is now the missing piece that makes `gear_check`/`spell_check`/`bank` non-blank once a member sets each char's class/level/race/is_bank_toon.

## Self-Check: PASSED

All 7 created code/test files + the SUMMARY exist on disk; all 4 task commits (`64b9f9d`, `9f6407b`, `80fbb23`, `8fbbce3`) are present in the git log. Backend `go test ./internal/backendsrv/...` green; web `npm run check` 0/0, `npx vitest run` 200/200, `npm run build` emits index.html + 200.html.

---
*Phase: 16-cutover-decommission*
*Completed: 2026-05-31*
