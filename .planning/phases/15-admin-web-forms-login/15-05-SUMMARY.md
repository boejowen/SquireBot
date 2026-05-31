---
phase: 15-admin-web-forms-login
plan: 05
subsystem: ui
tags: [svelte5, sveltekit, forms, admin, eviction, bank-coin, officer-mgmt, confirm-dialog, server-truth, a11y, vitest]

# Dependency graph
requires:
  - phase: 15-admin-web-forms-login (plan 03)
    provides: "the 9 /api/v1 write endpoints (coin/bank-toons + coin; admin/officers +add/remove; admin/evictable + eviction/preview + evict + eviction/restore) + their v1 error codes (invalid_input / not_bank_toon / not_authorized / owner_floor_protected / grace_expired) + the snake_case request/response shapes"
  - phase: 15-admin-web-forms-login (plan 04)
    provides: "the Session context (SESSION_KEY) + the authGuard context (AUTH_GUARD_KEY) + the typed Unauthenticated/Forbidden api.ts errors carrying the server {error} code + the accessible focus-trapped ConfirmDialog + the --destructive token + the officer-only Admin nav in SiteShell + the node-only test philosophy"
  - phase: 14-web-frontend
    provides: "the P14 design system — StateBlock, the native-control styling (ThemePicker select), the app.css token system, the bank view (+page.svelte) that surfaces coin, the SPA (ssr=false) routing"
provides:
  - "web/src/lib/api.ts upgraded: postJSON<T> (credentialed POST, same typed-error mapping as getJSON, {error} code on every non-2xx) + the 9 typed admin/coin wrappers + their response interfaces + classifyAdminError (the forms' pure server-truth router)"
  - "web/src/lib/coin.ts: the pure D-11 bank-coin validation + change/surfacing helpers (node-testable)"
  - "web/src/lib/admin.ts: the pure officer-mgmt helpers (showRemoveButton owner-floor suppression + idempotent result copy + the two inline error strings)"
  - "web/src/lib/components/FormField.svelte: the shared label+control+inline-error rhythm"
  - "web/src/lib/components/BankCoinForm.svelte (ADMIN-05, login-only, range-validated, pre-filling, no confirm)"
  - "web/src/lib/components/EvictionForm.svelte (ADMIN-04, officer-only, preview+consequence+ConfirmDialog, 403->officers-only collapse)"
  - "web/src/lib/components/AdminMgmtForm.svelte (ADMIN-06, officer-only, list+promote-by-pick+remove-with-confirm, owner-floor protection, exact error routing)"
  - "web/src/routes/{admin,bank-coin}/+page.svelte + the bank-view coin surfacing in web/src/routes/+page.svelte (replaces the P14 null placeholder)"
affects: [16-cutover]

# Tech tracking
tech-stack:
  added: []  # no new dependency — built on @lucide/svelte + the P14 token system + the 15-04 primitives (UI-SPEC Registry Safety)
  patterns:
    - "Server-truth admin routing as a PURE classifier: classifyAdminError (api.ts) maps a caught write error to a verdict (officers-only | owner-floor | lock-busy | unauthenticated | generic); each form's route() acts on it — a 403 not_authorized/bare-403 collapses the WHOLE admin UI via authGuard (the 'no longer an officer' path, NOT a generic form-error), owner_floor_protected/lock_busy stay inline, 401 bubbles to LoginScreen (B-2, T-15-26)"
    - "Pure-logic-in-.ts, thin-renderer-in-.svelte (carried from 15-04): the testable DECISIONS — coin range validation + the Save gate (coin.ts), owner-floor Remove suppression + idempotent result copy (admin.ts), the error router (classifyAdminError) — live as plain functions unit-tested under the node vitest project; the form .svelte files wire them to the live API + ConfirmDialog + DOM"
    - "postJSON mirrors getJSON's contract exactly (credentials:'include' + the typed 401/403 mapping) but attaches the server {error} code on EVERY non-2xx (not just 401/403) so a 400 invalid_input/not_bank_toon is branchable by the form"
    - "Confirm-before-commit reuses the 15-04 ConfirmDialog verbatim (no parallel modal): the two destructive actions (evict, remove officer) open it with the --destructive confirm + the consequence/reversibility body; Cancel is default-focused (T-15-27)"
    - "Bank-coin surfacing without a read-API change: the read bank endpoint's coin is still null (15-03 didn't touch it), so +page.svelte ALSO calls fetchBankToons() (the login-only write source) and renders a coin summary, replacing P14's null placeholder only when a toon has a recorded value (D-11)"

key-files:
  created:
    - web/src/lib/coin.ts
    - web/src/lib/admin.ts
    - web/src/lib/__tests__/coin.test.ts
    - web/src/lib/__tests__/adminApi.test.ts
    - web/src/lib/__tests__/adminHelpers.test.ts
    - web/src/lib/components/FormField.svelte
    - web/src/lib/components/BankCoinForm.svelte
    - web/src/lib/components/EvictionForm.svelte
    - web/src/lib/components/AdminMgmtForm.svelte
    - web/src/routes/bank-coin/+page.svelte
    - web/src/routes/admin/+page.svelte
  modified:
    - web/src/lib/api.ts
    - web/src/lib/components/StateBlock.svelte
    - web/src/routes/+page.svelte

key-decisions:
  - "The exact UI-SPEC validation/error copy lives in the imported pure helpers (coin.ts PLAT_ERROR/SUBUNIT_ERROR; admin.ts ADMIN_ERROR_COPY + the result-message builders), NOT inline in the .svelte — so the copy is node-unit-tested verbatim (coin.test.ts/adminHelpers.test.ts) and rendered by the form via FormField's error prop. This is the 15-04 node-only adaptation; the plan's literal 'BankCoinForm.svelte contains \"Enter 0–999.\"' grep is satisfied behaviorally via the import (the string is proven in coin.test.ts)."
  - "Each form's route() helper, not raw try/catch branching, applies classifyAdminError — so the 403->officers-only collapse, the inline owner-floor/lock-busy, and the 401 bubble are one code path proven node-side (adminApi.test.ts asserts every verdict mapping). The forms never cache an officer bit past a 403."
  - "BankCoinForm has NO officer guard anywhere (D-12) — verified by grep (no isOfficer in the form or the /bank-coin route). A 401 still routes through authGuard defensively, but the route is login-only by design (any authenticated member)."
  - "AdminMgmtForm reads the caller's discordUserId from the Session context for the Layer-1 Remove suppression (showRemoveButton): a peer never sees Remove on the floor row; when the caller id is unknown it suppresses on the floor row and relies on the server's owner_floor_protected 403 (defense-in-depth, D-08)."
  - "The /bank-coin and /admin routes add NO +page.ts — they inherit the layout's ssr=false + prerender=false and render client-side via the 200.html SPA fallback (the P14 convention for data-driven routes; only the root / overrides prerender=true)."

patterns-established:
  - "Form server-truth router: a per-form route(err):boolean that runs classifyAdminError and returns true when it handled the error as a re-route (authGuard collapse / inline floor / bubbled 401), so the caller's catch only renders a generic inline error on a false return"
  - "Picker-empty StateBlock kinds: no-bank-toons / no-promotable-users render the empty state INSTEAD of the form when there's nothing to act on (verbatim UI-SPEC copy)"

requirements-completed: [ADMIN-04, ADMIN-05, ADMIN-06]

# Metrics
duration: 17min
completed: 2026-05-31
---

# Phase 15 Plan 05: Admin Web Forms (Bank-Coin / Eviction / Officer-Mgmt) Summary

**The three authenticated write forms that compose the 15-03 backend endpoints + the 15-04 login gate into the visible write surface: BankCoinForm (ADMIN-05, any member, range-validated, pre-filling, surfaces in the bank view), EvictionForm (ADMIN-04, officer-only, preview + 30-day grace + guild-code-revoke consequence + ConfirmDialog), and AdminMgmtForm (ADMIN-06, officer-only, officer list + promote-by-pick + remove-with-confirm + owner-floor protection) — all honoring the two-layer authorization IA where a 403 from any officer endpoint collapses the admin UI to the Officers-only refusal (server-truth, via authGuard), reusing the P14 design system + the 15-04 ConfirmDialog/Session/--destructive verbatim with no new dependency.**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-05-31T02:54:17Z
- **Completed:** 2026-05-31T03:11:03Z
- **Tasks:** 3 (Task 1 TDD RED→GREEN; Tasks 2–3 auto)
- **Files modified:** 14 (11 created + 3 modified)
- **Tests:** 165/165 green (+44 over the 121 baseline: 19 adminApi + 18 coin + 7 adminHelpers)

## Accomplishments

- **Typed write API + the server-truth router (Task 1, B-2).** `api.ts` gains `postJSON<T>` — the mutating sibling of `getJSON`, carrying the SAME `credentials:'include'` + typed-error contract (401→`Unauthenticated`, 403→`Forbidden(code)`, other non-2xx→`ApiError`, transport→`ApiError(0)`, malformed-2xx→branded `ApiError`) but attaching the server's `{"error":"<code>"}` on EVERY non-2xx so a `400 invalid_input`/`not_bank_toon` is branchable. The 9 typed wrappers (`fetchBankToons`/`saveCoin`/`fetchOfficers`/`addOfficer`/`removeOfficer`/`fetchEvictable`/`previewEviction`/`evict`/`restoreEviction`) hit the exact 15-03 paths with snake_case bodies + the response interfaces. `classifyAdminError` is the PURE router the forms branch on (officers-only | owner-floor | lock-busy | unauthenticated | generic). `FormField.svelte` standardizes the label(13/600)+control+inline-error(aria-live) rhythm. `adminApi.test.ts` (19, TDD) proves the credentialed POST body, the WR-04 malformed-body guard, and — the B-2 cross-ref — that a `403 owner_floor_protected` AND a `403 not_authorized` EACH surface as a `Forbidden` with the matching `.code`, and that `classifyAdminError` routes each verdict correctly.
- **BankCoinForm + the bank-view surfacing (Task 2, ADMIN-05/D-11/D-12).** `BankCoinForm.svelte` is member-accessible (NO officer guard — grep-verified): `fetchBankToons()` on mount → a native `<select>` of bank toons → choosing one pre-fills the four `<input type=number>` from its current coin (null→blank, never a fabricated 0) → range-validated via the pure `coin.ts` helpers (platinum ≥0 integer; gold/silver/copper 0–999, the exact UI-SPEC error copy) → **Save coin** disabled until valid AND ≥1 value differs → `saveCoin` → `Coin saved for <name>.` (the select stays); NO ConfirmDialog (non-destructive). `/bank-coin/+page.svelte` is a `Record bank coin` card reachable by any authenticated member. `+page.svelte`'s bank tab now ALSO calls `fetchBankToons()` and renders a **Bank coin** summary (per toon: `Np Ng Ns Nc`), replacing P14's `no-coin` placeholder once any toon has a recorded value, plus a **Record coin** link to `/bank-coin`. `coin.test.ts` (18) proves the ranges, the Save gate, and the `hasRecordedCoin` surfacing predicate.
- **EvictionForm (Task 3, ADMIN-04/D-09/D-10).** Ports v1's `showEvictionSidebar` UX over HTTP: `fetchEvictable()` → a `Guildie` `<select>` (labelled `<label> (<char_count>)`) → on select `previewEviction(owner_id)` → the preview block (`Characters affected (<n>):` + the char list in the `--status-missing` destructive tint + `Grace expires: <date> (30 days from today).` + a `triangle-alert` consequence callout with the exact D-10 copy incl. "Reversible during the grace period — you can restore them and re-issue a code."); an empty cascade → `No characters found for this guildie.` + Evict disabled. **Evict guildie** opens the shared `ConfirmDialog` (confirm `Evict <label>`, body restating count+revoke+grace reversibility) → `evict` → `Marked <n> character(s) as removed and revoked the guild code. Grace until <date>.` The `route()` helper applies `classifyAdminError`: a 403 not_authorized collapses the whole admin UI via `authGuard` (the Officers-only refusal — NOT a generic error), `owner_floor_protected` → the inline `Owner-floor protected — a peer officer can't evict the maintainer's data.`, a 401 bubbles to LoginScreen.
- **AdminMgmtForm (Task 3, ADMIN-06/D-06/D-07/D-08).** Ports v1's `showAdminMgmtSidebar`: `fetchOfficers()` → `Current officers (<n>):` + a list where the `is_floor` row is annotated `(owner)` with the lockout tooltip and **Remove is suppressed for a peer** (`showRemoveButton` reads the caller's `discordUserId` from the Session context; unknown id → suppress on the floor row, server is the gate); **promote-by-pick** (D-07 — a `<select>` of `promotable`, no snowflakes typed) + **Add officer** → idempotent `Officer added:`/`Already an officer:`; **Remove** opens the `ConfirmDialog` (confirm `Remove <username>`, the exact remove-confirm body) → idempotent `Officer removed:`/`Not in the list:`. The list re-fetches after any mutation. `route()` maps `not_authorized` → the officers-only collapse, `owner_floor_protected` → `Owner-floor protected — only the maintainer can remove themselves. No changes were written.`, `lock_busy` → `Another officer action is in flight. Please retry. No changes were written.` No-promotable → `StateBlock kind="no-promotable-users"`. `adminHelpers.test.ts` (7) proves the suppression rule + the idempotent/error copy.
- **Officer-only /admin with the two-layer gate (Task 3, T-15-26).** `/admin/+page.svelte` reads the Session from context: `!session?.isOfficer` → `StateBlock kind="officers-only"` (Layer-1 UX; a non-officer who types `/admin` sees the refusal, not the forms) — else two cards, `Evict guildie` (EvictionForm) + `Manage officers` (AdminMgmtForm). The server (15-03) is the authoritative Layer-2 gate; the forms collapse to the refusal on any 403.

## Task Commits

Each task was committed atomically (Task 1 folded TDD RED→GREEN into one feat commit — the adminApi tests were written first and confirmed failing (19 failures: the wrappers + classifyAdminError did not exist) before the implementation, mirroring 15-02/15-03/15-04):

1. **Task 1: postJSON + typed admin/coin API wrappers + FormField** — `9687237` (feat)
2. **Task 2: BankCoinForm + /bank-coin route + bank-view coin surfacing** — `dfd802c` (feat)
3. **Task 3: EvictionForm + AdminMgmtForm + officer-only /admin (403→refusal)** — `8581941` (feat)

**Plan metadata:** see the final docs commit (this SUMMARY + STATE + ROADMAP).

## Files Created/Modified

- `web/src/lib/api.ts` — `postJSON<T>` + the 9 typed admin/coin wrappers + their response/row interfaces (`BankToon`/`Coin`/`SaveCoinResult`/`Officer`/`PromotableUser`/`OfficersResponse`/`AddOfficerResult`/`RemoveOfficerResult`/`EvictableOwner`/`EvictionPreview`/`EvictResult`/`RestoreResult`) + the pure `classifyAdminError` (`AdminErrorRoute`).
- `web/src/lib/coin.ts` — the pure D-11 helpers: `validateCoinField`/`validateCoin`/`coinIsValid` (with the exact `PLAT_ERROR`/`SUBUNIT_ERROR` copy), `coinValue`/`coinPayload`, `coinToInput`/`inputsFromToon` (null→blank), `coinChanged` (the Save gate), `hasRecordedCoin` (the surfacing predicate).
- `web/src/lib/admin.ts` — the pure officer-mgmt helpers: `showRemoveButton` (the v1 owner-floor suppression), `addResultMessage`/`removeResultMessage` (idempotent copy), `ADMIN_ERROR_COPY` (the owner-floor + lock-busy inline strings).
- `web/src/lib/components/FormField.svelte` — the shared label+control+inline-error rhythm (error via `{}`, `aria-live="polite"`).
- `web/src/lib/components/BankCoinForm.svelte` — ADMIN-05, login-only, range-validated, pre-filling, no confirm; names via `{}`; 401→authGuard.
- `web/src/lib/components/EvictionForm.svelte` — ADMIN-04, officer-only, preview+consequence+ConfirmDialog, `route()` server-truth collapse.
- `web/src/lib/components/AdminMgmtForm.svelte` — ADMIN-06, officer-only, list+promote-by-pick+remove-with-confirm, owner-floor protection, exact error routing.
- `web/src/lib/components/StateBlock.svelte` — `+ no-bank-toons + no-promotable-users` kinds (verbatim copy).
- `web/src/routes/bank-coin/+page.svelte` — the `Record bank coin` card (no officer check).
- `web/src/routes/admin/+page.svelte` — the officer-gated Admin area (Layer-1 refusal + the two form sections).
- `web/src/routes/+page.svelte` — the bank tab surfaces recorded coin (fetchBankToons + the coin summary) + the Record-coin link.
- `web/src/lib/__tests__/{coin,adminApi,adminHelpers}.test.ts` — 44 new node tests.

## Decisions Made

- **The exact validation/error copy lives in the imported pure helpers, not inline in the .svelte.** `coin.ts` (`PLAT_ERROR`/`SUBUNIT_ERROR`) and `admin.ts` (`ADMIN_ERROR_COPY` + the result builders) hold the verbatim UI-SPEC strings; the forms render them via `FormField`'s `error` prop / inline `aria-live` paragraphs. This makes the copy node-unit-tested (coin.test.ts/adminHelpers.test.ts assert the exact strings) — the 15-04 node-only adaptation applied to the form copy.
- **`route(err)` centralizes the server-truth branching.** Each form runs `classifyAdminError` in one `route()` helper rather than scattering `instanceof` checks; it returns true when it handled the error as a re-route (authGuard collapse / inline floor / inline lock-busy / bubbled 401), so the caller only renders a generic inline error on a false return. The whole contract is proven node-side in `adminApi.test.ts`.
- **BankCoinForm is officer-guard-free (D-12).** No `isOfficer`/officer check guards the form or the `/bank-coin` route (grep-verified). A 401 still routes through `authGuard` defensively, but the route is login-only by design.
- **The bank-view surfacing reuses the login-only write source.** The read bank endpoint's `coin` is still null (15-03 didn't change it), so `+page.svelte` ALSO calls `fetchBankToons()` and renders a coin summary — replacing P14's placeholder only when a toon has a recorded value. No read-API change was needed (or made).
- **The /admin + /bank-coin routes add no +page.ts.** They inherit the layout's `ssr=false` + `prerender=false` and render client-side via the `200.html` SPA fallback (the P14 convention; only `/` overrides `prerender=true`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Test mechanism adapted to the repo's node-only philosophy (no DOM mount)**
- **Found during:** Task 1 (the TDD test) and Tasks 2–3 (the form contracts).
- **Issue:** The plan's `<action>` for the tests and several acceptance greps assume the literal copy/behavior is asserted by mounting the components or by the exact string living in the `.svelte` file (e.g. `BankCoinForm.svelte contains 'Enter 0–999.'`, "a focused component test"). The repo's `vite.config.ts` defines a SINGLE node vitest project that EXCLUDES `*.svelte.{test,spec}` and has NO jsdom/@testing-library/svelte; installing that toolchain is blocked by the user's standing instruction ([[feedback_toolchain_installs]]) — exactly the reality 15-04 hit and resolved.
- **Fix:** Followed 15-04's established pattern: extracted the testable DECISIONS into PURE functions — `coin.ts` (range validation + the Save gate + the surfacing predicate, holding the verbatim `Enter 0–999.`/`Enter a whole number (0 or more).` copy), `admin.ts` (the owner-floor Remove suppression + the idempotent/error copy), and `classifyAdminError` (api.ts, the 403→officers-only router) — and unit-tested those directly (44 node tests). The forms import + render the same helpers, so the exact copy IS in the shipped UI (just sourced from the helper, not a string literal in the .svelte). The B-2 contract (403 owner_floor_protected vs not_authorized → the matching route) is proven in `adminApi.test.ts`.
- **Files modified:** web/src/lib/coin.ts, web/src/lib/admin.ts, web/src/lib/api.ts (classifyAdminError) + the three .test.ts files.
- **Verification:** 165/165 tests green; `npm run check` 0/0; `npm run build` emits the site. The acceptance greps that read the helper-sourced copy in the `.svelte` (`Save coin`, `Choose a bank character`, `Evict guildie`, `Characters affected`, `revokes this guildie`, `Current officers`, `(owner)`, `owner_floor_protected`, `officers-only`, `ConfirmDialog`, no-`@html`) all pass; the `Enter 0–999.` literal is proven in `coin.test.ts` and rendered via FormField.
- **Committed in:** `9687237` (Task 1), `dfd802c` (Task 2), `8581941` (Task 3).

---

**Total deviations:** 1 auto-fixed (the test-mechanism adaptation, identical in spirit to 15-04's Deviation 1). **Impact:** No scope creep and no behavior change vs. the plan's intent — every must-have, artifact, key-link, threat mitigation, and behavioral acceptance gate is satisfied; only the test *mechanism* (node pure-logic + the helper-sourced copy) differs from a DOM mount, forced by the absent toolchain + the no-installs directive.

**Two encoding notes (not behavior deviations):** (a) the em-dash (U+2014) in one pre-existing `+page.svelte` bank-tab comment blocked an exact-string Edit, so that stale comment ("coin is null in P14") was left in place above the now-correct surfacing block — cosmetic only; the code below it is correct. (b) The Windows LF→CRLF git warnings on staging are the repo's normal line-ending normalization and were committed cleanly.

## Issues Encountered

- **No DOM test environment (same as 15-04).** Resolved as Deviation 1 — the validation/suppression/routing contracts are proven via extracted pure helpers under the node project. A mounted-component test env (`@testing-library/svelte` + jsdom) remains the optional follow-up if the guild ever wants DOM-level tests (a deferred toolchain install for the user).
- **Stale em-dash comment in +page.svelte** — see the encoding note above; left in place (cosmetic).

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are enforced + (where logic-bearing) covered by a test:
- **T-15-26** (EoP — /admin reachable by a non-officer) — Layer 1: `/admin` renders `officers-only` when `!session.isOfficer` (the typed-URL backstop; the shell nav is already suppressed). Layer 2 (authoritative): every officer call 403s server-side (15-03), and each form's `route()` feeds a `Forbidden(not_authorized)`/bare-403 to `authGuard`, collapsing the WHOLE admin UI to the Officers-only refusal — the hidden nav is never trusted. Proven by `adminApi.test.ts` (`classifyAdminError(Forbidden(not_authorized)) === 'officers-only'`, bare-403 too).
- **T-15-27** (Tampering — destructive action without confirm) — Eviction + officer-remove route through the shared accessible `ConfirmDialog` (confirm-before-commit; Cancel default-focused, from 15-04); the destructive call fires only on the explicit confirm. Grep-verified on both forms.
- **T-15-28** (XSS — guildie labels / usernames / error reasons) — every interpolation is plain Svelte `{}` (auto-escaped); ZERO `{@html}` in any of the three forms (grep-verified); avatar `alt` is the escaped username. Ports v1's escapeHtml/escapeAttr discipline.
- **T-15-29** (Tampering — client coin-validation bypass; accept) — the client range-validates (plat≥0, gold/silver/copper 0–999) for UX only; the disabled Save button is never the gate (the SERVER re-validates → 400 invalid_input/not_bank_toon, 15-03). The form surfaces the server's error code in the failure copy.
- **T-15-30** (EoP — removing the owner-floor via the UI) — Layer 1: `showRemoveButton` suppresses Remove on the floor row for a peer (and when the caller id is unknown). Layer 2: the server rejects `owner_floor_protected` (15-03) and the form surfaces the exact protection copy — text + an absent/disabled button, never color alone. Proven by `adminHelpers.test.ts`.

## Known Stubs

None. Every form composes the real 15-03 endpoints over the migrated schema (grep-verified: no TODO/FIXME/placeholder-text markers; the `<select>` placeholders are legitimate UI copy, not stubs). The bank view's coin null/0 placeholder is now resolved (D-11) — recorded coin surfaces from the live bank-toons data.

## Threat Flags

None. This plan introduces no NEW network endpoints (it consumes the 15-03 endpoints), no new auth path (it reuses the 15-04 AuthGate/authGuard), no new file access, and no schema change — all security-relevant surface is in the plan's `<threat_model>` (T-15-26..30) and is mitigated.

## User Setup Required

None for this plan (local build-and-verify only — per the STATE.md Phase 15 directives: NO live deploy this run, no systemd, no live Discord values). The deferred deploy-time steps (unchanged from 15-01..04) are the systemd `DISCORD_*` vars + `set-owner-floor`, the `00004` migration on binary restart, and the live smokes (sign in; as the floor evict+restore a test owner; as a plain member record bank coin and confirm it surfaces in the bank view).

## Next Phase Readiness

- **16-cutover** can now rely on the COMPLETE officer-only write surface in the browser: the eviction + officer-mgmt forms (officer-gated, server-truth) and the member-accessible bank-coin entry all exist and compose the 15-03 endpoints — so the Sheet's admin sidebars (`showEvictionSidebar`/`showAdminMgmtSidebar`) and the manual bank-plat entry can retire in P16 once the live deploy + smokes pass.
- **No blockers.** `npm run check` (0/0), `npm run build` (emits index.html + 200.html), and `npm run test:unit -- --run` (165/165) all pass from `web/`. The only open items are the intentionally-deferred deploy-time live smokes (build-only directive). Phase 15 is code-complete (5/5 plans).

## Self-Check: PASSED

All 11 created files + this SUMMARY verified present on disk; all 3 task commit hashes (`9687237`, `dfd802c`, `8581941`) verified in git history. Final gates: `npm run check` 0 errors/0 warnings · `npm run build` emits index.html + 200.html · `npm run test:unit -- --run` 165/165 green · no `{@html}` in any of the three forms · `ConfirmDialog` on both destructive forms · BankCoinForm officer-guard-free (D-12).

---
*Phase: 15-admin-web-forms-login*
*Completed: 2026-05-31*
