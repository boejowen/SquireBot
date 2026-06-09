---
phase: 26-character-assignment
doc: human-uat
date: 2026-06-08
environment: production (squirebot.quest / api.squirebot.quest, schema v9)
tester: maintainer (officer, Discord 216769394393481216)
verdict: PASS (with 1 MEDIUM UI bug found → logged 999.33; 2 items deferred)
---

# Phase 26 — Browser-Smoke UAT (production)

Closes the `human_needed` items from `26-VERIFICATION.md` (web interactive flows +
live-DB seed correctness). Run live against prod on 2026-06-08 by the maintainer in a
browser with DevTools open. The node-only vitest suite (287 green) cannot cover Svelte
rendering/interaction — this doc is that coverage.

## Result summary

| Block | Item | Result |
|-------|------|--------|
| A | Member: claim / release (ConfirmDialog) / re-claim; inline `409` error copy; per-row busy disabling | ✅ PASS |
| A5 | Request→Cancel toggle on a contested char | ⚪ N/A by design — see Deferred #2 |
| B | Officer: assign / reassign / remove; designate bank/bot/none | ✅ PASS |
| B (bug) | Designate bank/bot removes the char from the panel | 🐞 FOUND → root-caused → recovered → logged **999.33** |
| B4 | Approve a pending request | ⚪ Not exercised — no request can be filed via UI today (same root as A5) |
| C1 | Gear/Settings menu shows a member-visible "My characters" link | ✅ PASS |
| C2 | CharMetaForm renders with NO bank-toon checkbox (officer-only now) | ✅ PASS |
| — | Live-DB 00009 apply + auto-seed correctness against real guild data | ✅ PASS (verified directly on-box: 4/4 seeded, real bank toon excluded) |
| B (403) | Non-officer `/admin` officers-only collapse | ⏸ DEFERRED — see Deferred #1 |

**Verdict: PASS.** Every officer/member happy path works on prod; the live migration +
seed are correct. One genuine MEDIUM UI bug was surfaced and is documented with a fix
path (999.33). The remaining open items are a UI-collapse confirmation (server gate
already code-verified) and two flows whose UI trigger condition cannot occur today.

## Pre-flight (unauthenticated probes)

- apex `https://squirebot.quest/` → `200`
- `https://squirebot.quest/my-characters` → `200`
- `https://api.squirebot.quest/api/v1/assignments/mine` (no session) → `401` (auth gate holds)

## Bug found — 999.33: guild-bank/bot designation is a one-way door in the officer UI

**Severity:** MEDIUM (operational dead-end, **no data loss**). **Status:** logged to backlog
as **999.33**; recovered on prod.

**Root cause:** `DesignateCharTx` clears the `character_assignment` row when a char is set
to bank/bot (correct — a shared char has no single assignee). But `ListAllAssignments`
`JOIN`s `character_assignment`, and `AssignmentAdminPanel` renders ONLY that list — which
is also the host of the per-row Designate (bank/bot/none) + Reassign/Remove controls. The
char is simultaneously excluded from the member claimable read
(`is_bank_toon=0 AND is_guild_bot=0`). Net: once designated bank/bot, a character is
unreachable from every UI surface and can only return to `mode:none` via a direct
API/DB call. The data layer is correct and reversible; this is purely a UI reachability gap.

**How it surfaced:** the tester designated all 4 assigned characters during the B5 step;
each vanished from the panel as designated, emptying the "Character assignments" section.

**Recovery (prod, 2026-06-09 ~01:30 UTC):**
1. Fresh R2 backup taken first (`squirebot-2026-06-09.db.gz`).
2. Read-only inspection via `audit_log` (`event='char_designate'`) identified the exact
   chars touched today: ids **1, 14, 15, 16**. **Findom (id 4, `is_bank_toon=1`)** had NO
   designate event → confirmed pre-existing genuine guild bank toon → left untouched.
3. Scoped transaction: cleared `is_bank_toon`/`is_guild_bot` on ids 1/14/15/16 and re-ran
   the migration's exact auto-seed restricted to those ids (`assigned_by='migration'`,
   linked-owner join). Added an `assignment_restore` audit row per char.
4. Verified: `character_assignment` back to 4/4 (Slampeach→216769…, Silven/Regnor/Umbrigg
   →787030…); only Findom remains flagged (bank). Tester confirmed both `/admin` and
   `/my-characters` render correctly.

**Suggested fix (carried on 999.33):** surface designated chars in the officer panel
(an "all characters / designations" view, or a dedicated designated-chars section) so the
designate control — including `mode:none` — is reachable for already-designated chars.
Good candidate to fold into Phase 27/28 web work or fix before v2.3 close.

## Deferred items

1. **Non-officer `/admin` officers-only collapse (403).** Needs a second, non-officer
   Discord account; deferred at the tester's discretion. **Low risk:** the server-side gate
   is already code-verified — `RequireOfficer` + in-tx `IsOfficerTx`, with a passing
   table-driven test hitting every officer endpoint as a non-officer
   (`TestOfficerEndpoints_NonOfficer_Rejected`). This UAT would only confirm the UI collapse
   renders, not the security boundary.

2. **Request→Cancel (A5) and officer Approve/Deny (B4) interactive flows.** The member
   claimable read returns unassigned chars only, so a *contested* char never surfaces in the
   member UI today and the **Request** button has no way to appear — therefore no pending
   request can be filed for an officer to Approve/Deny. This is the documented
   `assignedToOthers is empty in practice` reconciliation (26-03-SUMMARY). The underlying API
   (request / duplicate-`409` / approve-denies-siblings) is covered by passing node tests.
   If members should be able to request a teammate's character, that's a small follow-up
   (widen the claimable read to include contested chars + a confirm) — relates to 999.33's
   panel-reachability theme.

## Evidence

- Ops access: SSH to the Hetzner box (`5.78.232.85`) as `root`; DB at
  `/var/lib/squirebot/squirebot.db` (owner `squirebot`). `audit_log` columns are
  `event, char_name, actor, detail, at` (NOT `action`).
- Schema confirmed at goose v9; `character_assignment` + `assignment_request` + `is_guild_bot`
  present and populated.
- Backlog: 999.33 committed (`9c3022f`).
