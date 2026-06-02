---
phase: 17-self-service-watcher-linking
plan: 03
subsystem: ui
tags: [sveltekit, svelte5, clipboard, show-once-secret, account-page, vitest]

# Dependency graph
requires:
  - phase: 17-self-service-watcher-linking (17-02)
    provides: the 3 login-only /account endpoints (POST/GET/POST-revoke account/codes) the page consumes
  - phase: 15-admin-web-forms-login
    provides: Discord OAuth2 session + AuthGate (AUTH_GUARD_KEY/SESSION_KEY) + SiteShell member nav + ConfirmDialog/StateBlock/EvictionForm patterns
provides:
  - "/account member page (form-card shell) gated by the existing AuthGate"
  - "WatcherCodesPanel: generate → show-once plaintext reveal w/ copy-to-clipboard + paste instructions + irreversibility warning → active-codes list (#N/created/last-seen) → confirm-before-commit per-code revoke"
  - "fetchOwnCodes/mintOwnCode/revokeOwnCode api wrappers + OwnCode type"
  - "member-visible (NOT officer-gated) Account nav entry"
  - "StateBlock no-codes empty-state kind"
affects: [phase-18-watcher-cleanups, future-wantlist-discord-pinger]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Show-once secret surface: plaintext held ONLY in component $state (mintedPlaintext) for panel lifetime — never localStorage, never re-fetched, never logged; rendered via {} auto-escape, never {@html}"
    - "navigator.clipboard.writeText with user-select:all manual fallback (never block on Clipboard API denial)"
    - "Member-page (non-officer) gating: AuthGate at layout, nav entry under session?.authenticated (NOT session?.isOfficer); the hidden nav is UX, server 401→AuthGate→LoginScreen is the real boundary"
    - "Non-autonomous browser-smoke checkpoint for DOM/clipboard surfaces vitest cannot see (P15 precedent)"

key-files:
  created:
    - web/src/routes/account/+page.svelte
    - web/src/lib/components/WatcherCodesPanel.svelte
  modified:
    - web/src/lib/api.ts
    - web/src/lib/components/SiteShell.svelte
    - web/src/lib/components/StateBlock.svelte
    - web/src/lib/__tests__/api.test.ts

key-decisions:
  - "Plaintext minted code lives only in mintedPlaintext Svelte state — never persisted, re-fetched, or logged (T-17.03-01 / LINK-04)"
  - "All interpolation via Svelte {} auto-escape; no {@html} anywhere in the panel (T-17.03-02)"
  - "Account nav placed under session?.authenticated, never session?.isOfficer — member-facing surface (T-17.03-03)"
  - "Clipboard API failure is non-blocking — user-select:all on the token is the manual fallback"
  - "Verified in a real browser against the LIVE deployed site (vitest is DOM/clipboard-blind) — not unit tests alone"

patterns-established:
  - "Show-once secret reveal panel pattern (copy + manual-select fallback + irreversibility warning + dismiss-is-cosmetic)"
  - "EvictionForm load→confirm→commit + optimistic-collapse reused for per-code revoke"

requirements-completed: [LINK-01, LINK-03, LINK-04, LINK-05]

# Metrics
duration: ~1 day (incl. blocking human browser-smoke checkpoint)
completed: 2026-06-02
---

# Phase 17 Plan 03: /account Member Page Summary

**The /account member page: a SvelteKit route + WatcherCodesPanel that mints a watcher code, reveals the plaintext exactly once in a copy-to-clipboard show-once panel (paste instructions + "won't see again" warning), lists active codes (#N/created/last-seen), and revokes any single code via confirm-before-commit — backed by 3 typed credentialed api wrappers and a member-visible Account nav entry.**

## Performance

- **Duration:** ~1 day (two autonomous tasks + a blocking human browser-smoke checkpoint against the live VPS)
- **Tasks:** 3 (2 autonomous + 1 human-verify checkpoint)
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments
- `fetchOwnCodes` / `mintOwnCode` / `revokeOwnCode` api wrappers + `OwnCode` type, reusing `getJSON`/`postJSON` (credentialed + typed errors), with mirroring wrapper unit tests in `api.test.ts`.
- `StateBlock` gained a `no-codes` empty-state kind (17-UI-SPEC copy).
- `WatcherCodesPanel.svelte` — the one genuinely-new, security-sensitive frontend surface: generate → show-once plaintext panel (copy-to-clipboard with check/"Copied!" swap + `user-select:all` manual fallback + paste-into-watcher instructions + `triangle-alert` irreversibility warning) → active-codes list → ConfirmDialog-gated per-code optimistic-collapse revoke. Plaintext lives only in `mintedPlaintext` state; no `{@html}`, no `localStorage`.
- `/account/+page.svelte` form-card shell mirroring char-meta, AuthGate-gated at the layout (no page-level officer check).
- Member-visible "Account" nav entry in `SiteShell.svelte` under `session?.authenticated` (not the officer Admin block).
- **Browser smoke (Task 3) PASSED** — verified live, all 7 steps confirmed working.

## Task Commits

1. **Task 1: api wrappers + OwnCode type + StateBlock no-codes kind** — `fb68afc` (feat)
2. **Task 2: WatcherCodesPanel + /account page + member Account nav** — `fc59fbf` (feat)
3. **Task 3: Browser-smoke /account (blocking human-verify checkpoint)** — APPROVED (no code commit; verification only)

**Plan metadata:** this SUMMARY + STATE + ROADMAP (docs: complete 17-03)

## Files Created/Modified
- `web/src/routes/account/+page.svelte` (created) — /account form-card shell rendering `<WatcherCodesPanel />`; no `+page.ts` (inherits SPA fallback); no officer gate.
- `web/src/lib/components/WatcherCodesPanel.svelte` (created) — the generate → show-once → list → confirm-revoke panel (Svelte 5 runes).
- `web/src/lib/api.ts` (modified) — `OwnCode` type + `fetchOwnCodes`/`mintOwnCode`/`revokeOwnCode` (reuse `getJSON`/`postJSON`).
- `web/src/lib/components/SiteShell.svelte` (modified) — member-visible `href="/account"` Account nav under `session?.authenticated`.
- `web/src/lib/components/StateBlock.svelte` (modified) — `no-codes` empty-state kind.
- `web/src/lib/__tests__/api.test.ts` (modified) — wrapper unit tests (path + method + `{}` mint body).

## Task 3: Browser-Smoke Checkpoint — PASSED

The plan is NON-AUTONOMOUS by design: `web/` vitest is node-only and cannot see the DOM, the show-once reveal, or the clipboard (the P15 precedent shipped 2 crashing BLOCKERs behind 165 green tests — see MEMORY: web-tests-node-only). Task 3 was a `checkpoint:human-verify` with a `blocking` gate.

**Deploy note:** The backend (Waves 1–2) and this frontend were deployed to the LIVE VPS — `api.squirebot.quest` at schema **v5**, `squirebot.quest` serving `/account`. The login-only account routes are **401-gated** (confirmed against the live server, not just unit-mocked). The human ran all 7 verification steps against the deployed site and replied **"approved"** (all passed):

1. **Member-only nav** — "Account" entry appears for a logged-in member, ABSENT for a logged-out visitor. PASS.
2. **Generate → show-once plaintext** — freshly minted code appears in the highlighted panel with the irreversibility warning + paste instructions. PASS.
3. **Copy-to-clipboard** — "Copy" swaps to "Copied!" and the clipboard holds the code (manual `user-select:all` fallback present). PASS.
4. **Reload never re-reveals** — after Done + reload, plaintext is GONE; the code shows only as a #N row (created + "never used yet"). PASS (LINK-04 confirmed live).
5. **Additive 2nd code** — generating a second code keeps both #N rows; neither removed. PASS (LINK-03).
6. **Scoped confirm-revoke** — ConfirmDialog opens (default focus Cancel; Esc/backdrop dismiss with no change); confirming collapses only the targeted row, the other code remains. PASS (LINK-05).
7. **Heavy-theme contrast** — show-once panel + destructive Revoke styling spot-checked in the parchment/light "Heavy" theme. PASS.

## Decisions Made
None beyond the plan — followed the plan as specified. The security-sensitive constraints (plaintext in state only, no `{@html}`, no `localStorage`, member-not-officer nav) were implemented exactly as the threat register prescribed.

## Deviations from Plan

None — plan executed exactly as written. (No functional deviations; no auto-fixes required.)

## Issues Encountered
None. `npm run check` is clean on continuation (443 files, 0 errors, 0 warnings); the prior `npm run build` (Task 2 verify) and `npm test -- api` (Task 1 verify) passed during autonomous execution.

## User Setup Required
None — no external service configuration introduced by this plan. (The page consumes the already-deployed Wave-2 endpoints; Discord login was provisioned in Phase 15.)

## Next Phase Readiness
- Phase 17 self-service watcher-linking feature is now end-to-end live: backend (17-01/17-02) + frontend (17-03) deployed and browser-verified. LINK-01/03/04/05 satisfied by this plan; LINK-02 (Discord-identity↔owner linkage) and LINK-06 (`mint-code` CLI removal) landed in 17-02.
- **Note:** This SUMMARY finalizes plan 17-03 only. Phase-level verification/completion (and any advance to Phase 18) is owned by the orchestrator — this agent does NOT mark the phase complete.

## Self-Check: PASSED

- `web/src/routes/account/+page.svelte` — FOUND
- `web/src/lib/components/WatcherCodesPanel.svelte` — FOUND
- commit `fb68afc` (Task 1) — FOUND in git log
- commit `fc59fbf` (Task 2) — FOUND in git log
- `npm run check` — 0 errors, 0 warnings on continuation

---
*Phase: 17-self-service-watcher-linking*
*Completed: 2026-06-02*
