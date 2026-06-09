---
phase: 26-character-assignment
plan: 03
subsystem: web (SvelteKit — api.ts wrappers + member/officer assignment panels + char-meta de-bank-toon)
tags: [assignment, web, svelte, api-wrappers, my-characters, admin-panel, is_bank_toon, pure-helpers]
requires:
  - 26-02 (the 12 assignment HTTP endpoints + the snake_case Go JSON contract these wrappers mirror)
provides:
  - "api.ts: 4 typed interfaces (MyCharacter/ClaimableCharacter/Assignment/PendingRequest) + AllAssignments + DesignateMode + 12 typed fns over the existing getJSON/postJSON cores"
  - "MyCharactersPanel.svelte: member self-service (mine+claimable load, Claim/Release(confirm)/Request/Cancel)"
  - "AssignmentAdminPanel.svelte: officer panel (assignments table assign/reassign/remove, request queue approve/deny, per-char designate bank/bot/none) with a 403 officers-only collapse"
  - "/my-characters route + a member-visible SettingsMenu nav link (outside the officer gate)"
  - "/admin 'Character assignments' form-card section after Monitors"
  - "web/src/lib/assignments.ts: PURE node-testable partitionClaimable + requestStatusLabel (imported by the panels, asserted by assignments.test.ts)"
  - "char-meta de-bank-toon: isBankToon dropped from CharMetaInputs/charMetaPayload/inputsFromChar/charMetaChanged + the form checkbox + saveCharMeta body (officer-only now, OPEN-3)"
affects:
  - 27 (MYVIEW — the 'my characters' inventory filter builds on this assignment truth + the /my-characters surface)
  - 28 (CWANT — wantlist character tag builds on assignment)
tech-stack:
  added: []
  patterns:
    - "pure decision logic in a plain .ts (NOT a .svelte module export) so the node vitest project can import it — the panels import + the test asserts (the WatcherCodesPanel formatLastSeen pattern, but actually wired to a test)"
    - "runes load→phase→AuthGuard lifecycle + route(err) (401→AuthGate/LoginScreen, 403→officers-only collapse) cloned from WatcherCodesPanel (member) + MonitorAdminPanel (officer)"
    - "ConfirmDialog confirm-before-commit for the destructive Release"
    - "every interpolation via plain {} (Svelte auto-escape) — NEVER {@html} (T-26-16)"
key-files:
  created:
    - web/src/lib/assignments.ts
    - web/src/lib/components/MyCharactersPanel.svelte
    - web/src/lib/components/AssignmentAdminPanel.svelte
    - web/src/routes/my-characters/+page.svelte
    - web/src/lib/__tests__/assignments.test.ts
  modified:
    - web/src/lib/api.ts
    - web/src/lib/components/SettingsMenu.svelte
    - web/src/routes/admin/+page.svelte
    - web/src/lib/charmeta.ts
    - web/src/lib/components/CharMetaForm.svelte
    - web/src/lib/__tests__/charmeta.test.ts
decisions:
  - "ClaimableCharacter.assignee is modeled OPTIONAL (?: string | null) even though the 26-02 backend returns only unassigned rows today — so partitionClaimable can split Claim (unassigned) vs Request (contested) in one tested place, forward-compatible if the read ever widens. assignedToOthers is empty in practice now."
  - "The member panel has NO 'list my requests' endpoint (26-02 exposes mine/claimable/claim/release/request/request-cancel only) — the Request/Cancel toggle tracks the caller's outstanding request OPTIMISTICALLY in component state (a Set<character_id>) keyed off a successful Request; cancelRequest is by character_id."
  - "PendingRequest has no status field on the wire (the queue is pending-only) — the officer queue renders requestStatusLabel('pending') literally per row, satisfying the import + node test while matching the API shape."
  - "saveCharMeta body type dropped is_bank_toon (the form spreads a payload that no longer has it); CharMetaItem.is_bank_toon + SaveCharMetaResult.is_bank_toon READ fields stay for display."
metrics:
  duration: ~30 min
  completed: 2026-06-08
  tasks: 4
  files: 11
---

# Phase 26 Plan 03: Character Assignment Web UI Summary

**One-liner:** The Phase 26 user-facing surface for ASSIGN-01..05 — twelve typed `api.ts` wrappers over the 26-02 endpoints (4 new snake_case interfaces + `AllAssignments`/`DesignateMode` + 6 member + 6 officer fns delegating to the existing credentialed `getJSON`/`postJSON`), a member `MyCharactersPanel` (`/my-characters` + a member-visible `SettingsMenu` link) that loads mine+claimable and wires Claim/Release(ConfirmDialog)/Request/Cancel, an officer `AssignmentAdminPanel` (a new `/admin` "Character assignments" form-card) with the assignments table (assign/reassign/remove), the pending-request queue (approve/deny), and the per-character designate bank/bot/none control plus a 403 officers-only collapse, a shared **pure** `web/src/lib/assignments.ts` (`partitionClaimable` + `requestStatusLabel`) the panels import and `assignments.test.ts` asserts under the node-only vitest project, and the char-meta de-bank-toon cleanup that removes the now-officer-only bank checkbox from the member form and `is_bank_toon` from the form inputs/payload/change-detection.

## What Was Built

### Task 1 — api.ts typed wrappers + interfaces (commit `1059397`)
- Added `MyCharacter` / `ClaimableCharacter` / `Assignment` / `PendingRequest` (snake_case, mirroring `store.Assignment` / `store.PendingRequest` / `store.ClaimableChar` JSON), `AllAssignments` (`{assignments, requests}`), and the `DesignateMode` union (`'bank'|'bot'|'none'`).
- Added the 12 fns: member `fetchMyCharacters` / `fetchClaimable` / `claimChar` / `releaseChar` / `requestChar` / `cancelRequest`; officer `fetchAllAssignments` / `officerAssign(character_id, assignee)` / `officerRemoveAssign` / `approveRequest(request_id)` / `denyRequest(request_id)` / `designateChar(character_id, mode)`. Each delegates to the existing `getJSON`/`postJSON` (credentials:'include' + typed Unauthenticated/Forbidden), member bodies carry `character_id` only (D-02 / Pitfall 1). `getJSON`/`postJSON` themselves untouched.

### Task 2 — MyCharactersPanel + /my-characters route + nav link + pure helper (commit `e4e1a33`)
- `web/src/lib/assignments.ts` (new): PURE `partitionClaimable(claimable)` → `{unassigned, assignedToOthers}` (split by a non-empty `assignee` — null/undefined/'' → unassigned) and `requestStatusLabel(status)` (`pending`→'Pending', etc.; unknown round-trips). Plain .ts (NOT a .svelte module export) so the node project can import it.
- `MyCharactersPanel.svelte` (new): `getContext<AuthGuard>` + `type Phase` + `$state` + `onMount→load()` (Promise.all mine+claimable) + the `route(err)` 401→guard helper (WatcherCodesPanel pattern). Renders "Your characters" (Release → ConfirmDialog) and "Characters you can claim" (instant Claim for `split.unassigned`; Request/Cancel toggle for `split.assignedToOthers`, tracked optimistically). Imports `partitionClaimable` (does not inline the split). Plain `{}` escape only.
- `web/src/routes/my-characters/+page.svelte` (new): thin route (account/+page shape) with `<svelte:head><title>` + `<MyCharactersPanel />`.
- `SettingsMenu.svelte` (modified): added `<a href="/my-characters" …>My characters</a>` beside /account + /char-meta, OUTSIDE the `{#if session?.isOfficer}` Admin gate (member-visible, D-06/D-08).
- `assignments.test.ts` (new): asserts `partitionClaimable` (mixed/all-unassigned/empty) AND `requestStatusLabel` (all four known + two unknown) — both inside `expect(...)`.

### Task 3 — AssignmentAdminPanel + /admin section (commit `80e5ebc`)
- `AssignmentAdminPanel.svelte` (new): MonitorAdminPanel rhythm (`getContext<AuthGuard>` + `route(err)` 403→officers-only collapse + `onMount→load()` via `fetchAllAssignments`). Three blocks: (1) all-assignments list (char + `discord_user_id` holder) with a per-row assignee-id field → `officerAssign` Reassign + a `officerRemoveAssign` Remove + a 3-button designate control (`designateChar(character_id, 'bank'|'bot'|'none')`); (2) the pending-request queue (char + requester + the `requestStatusLabel('pending')` chip) with Approve (`approveRequest(req.id)`) / Deny (`denyRequest(req.id)`). A shared `act()` runs each mutation under a `busyKey` guard with uniform reason()-mapped inline errors + reload. Imports `requestStatusLabel`. Plain `{}` escape only.
- `admin/+page.svelte` (modified): imported `AssignmentAdminPanel`, added one `<section class="form-card"><h2 class="form-title">Character assignments</h2><AssignmentAdminPanel /></section>` after Monitors. The Layer-1 `{#if !isOfficer}` refusal already gates the page.

### Task 4 — Strip bank-toon from CharMetaForm + charmeta.ts (commit `c08b4e6`)
- `charmeta.ts` (modified): dropped `isBankToon` from `CharMetaInputs`; dropped `is_bank_toon` from `charMetaPayload`'s return type AND object (now class/level/race only); dropped the `isBankToon` line from `inputsFromChar`; dropped the `isBankToon` clause from `charMetaChanged`. Validation helpers untouched.
- `CharMetaForm.svelte` (modified): removed the bank-toon `<FormField>` checkbox + its `bind:checked`, the two `inputs` initializers' `isBankToon`, the `is_bank_toon` key in the optimistic `chars.map` echo, the `.checkbox-row` CSS, and the stale doc comments.
- `api.ts` (modified): `saveCharMeta` body type dropped `is_bank_toon` (the read-only `CharMetaItem.is_bank_toon` / `SaveCharMetaResult.is_bank_toon` fields stay).
- `charmeta.test.ts` (modified): dropped `isBankToon` from the `inputs()` factory + the payload/changed/inputsFromChar/CR-01 assertions; ADDED a guard that `charMetaPayload(...)` does NOT have an `is_bank_toon` key.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `saveCharMeta` body type narrowed to drop `is_bank_toon`**
- **Found during:** Task 4 — CharMetaForm spreads `charMetaPayload(inputs)` into `saveCharMeta({ character_id, ...payload })`; once the payload no longer carries `is_bank_toon`, the existing `saveCharMeta` body type (which required `is_bank_toon: boolean`) would no longer typecheck (`npm run check`).
- **Fix:** Narrowed the `saveCharMeta` body param to `{ character_id, class, level, race }` (the read-only `is_bank_toon` reply fields stay). The plan's interfaces note explicitly says "the FORM no longer binds/sends it," and Task 4 owns the charmeta de-bank-toon — this is within scope.
- **Files modified:** `web/src/lib/api.ts`
- **Commit:** `c08b4e6`

### Design reconciliations (within Claude's discretion / the plan's interfaces note)

- **`ClaimableCharacter.assignee` modeled optional, `partitionClaimable.assignedToOthers` empty in practice.** The 26-02 `/assignments/claimable` returns ONLY unassigned, non-shared, live chars (`store.ClaimableChar = {character_id, name}`), so there is no contested row in the claimable list today. The optional `assignee?` field + the partition keep the Claim-vs-Request decision in one tested place, forward-compatible, without fabricating data. Members request a contested char by character_id via `requestChar` once such a row surfaces.
- **No "my pending requests" read exists** (26-02 member endpoints are mine/claimable/claim/release/request/request-cancel). The Request→Cancel toggle tracks the caller's outstanding request OPTIMISTICALLY in component state (`Set<character_id>`); `cancelRequest` is by character_id. This is the faithful surface for the shipped API.
- **`PendingRequest` has no wire `status`** (the officer queue is pending-only). The queue renders `requestStatusLabel('pending')` literally per row — satisfying the imported-and-tested-helper requirement while matching the API.

## Verify Gate Results (run from web/)

| Gate | Command | Result |
|------|---------|--------|
| Check | `npm run check` | PASS — svelte-check + tsc, 482 files, 0 errors, 0 warnings |
| Build | `npm run build` | PASS — adapter-static wrote the site, exit 0 |
| Test | `npm test` | PASS — 22 files / 287 tests passed (incl. the 5 assignments.test.ts + the updated charmeta.test.ts) |

Verification greps (from the plan):
- `/my-characters` link present in SettingsMenu and NOT wrapped in an officer gate — CONFIRMED.
- `AssignmentAdminPanel` imported + rendered in admin/+page.svelte — CONFIRMED.
- `charMetaPayload` no longer contains `is_bank_toon` (the only match in charmeta.ts is a doc comment; the payload-omits guard test passes) — CONFIRMED.
- `assignments.ts` exports `partitionClaimable` + `requestStatusLabel`; MyCharactersPanel imports `partitionClaimable`, AssignmentAdminPanel imports `requestStatusLabel` — CONFIRMED.
- `assignments.test.ts` asserts both helpers inside `expect(...)` lines and runs green — CONFIRMED.
- No `{@html}` in either new panel — CONFIRMED.

## ⚠ Browser-Smoke Gap (flag for /gsd-ui-review)

vitest is NODE-ONLY here (no jsdom / @testing-library/svelte per the toolchain-install rule), so the green test suite covers the PURE helpers (`partitionClaimable`, `requestStatusLabel`) and the charmeta logic — it does NOT cover the panels' RENDERING or interaction. The following are NOT verified by these tests and need a browser-smoke on prod (or a full local stack with a seeded `sb_session`) before this phase is called verified:
- The member Claim / Release(confirm) / Request / Cancel flow at `/my-characters` (load states, ConfirmDialog, the optimistic Request→Cancel toggle, per-row busy disabling, the inline 409 char_shared/already_assigned/duplicate_request error copy).
- The officer assign/reassign/remove + approve/deny + designate(bank/bot/none) flow in the `/admin` "Character assignments" section, including the 403 officers-only collapse.
- The `/my-characters` nav link appearing in the gear SettingsMenu for a member.
- The CharMetaForm rendering with the bank checkbox gone.

Deploy is OUT of scope for this plan (the backend redeploy + web tarball/atomic-swap happen at phase close).

## Known Stubs

None — every panel is fully wired to the 26-02 endpoints via the api.ts fns; no placeholder values, no mock data sources. `partitionClaimable.assignedToOthers` being empty in practice is an API-shape reality (claimable = unassigned-only), not a stub — the panel renders the unassigned rows it does receive.

## Threat Flags

None — no security surface beyond the plan's `<threat_model>` (T-26-15..18) was introduced. The panels send only character_id / assignee / request_id / mode and rely on the server (26-02) for all authorization; every interpolation uses plain `{}` auto-escape (no `{@html}`); the officer panel's 403 collapse + the /admin Layer-1 refusal are UX, with the server's RequireOfficer + in-tx IsOfficerTx the real gate.

## Self-Check: PASSED

- Created files exist:
  - `web/src/lib/assignments.ts` — FOUND
  - `web/src/lib/components/MyCharactersPanel.svelte` — FOUND
  - `web/src/lib/components/AssignmentAdminPanel.svelte` — FOUND
  - `web/src/routes/my-characters/+page.svelte` — FOUND
  - `web/src/lib/__tests__/assignments.test.ts` — FOUND
- Commits exist on `master`:
  - `1059397` (Task 1) — FOUND
  - `e4e1a33` (Task 2) — FOUND
  - `80e5ebc` (Task 3) — FOUND
  - `c08b4e6` (Task 4) — FOUND
