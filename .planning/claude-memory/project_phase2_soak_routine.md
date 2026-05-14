---
name: Phase 2 soak verdict routine scheduled
description: One-time remote agent fires 2026-05-10T14:00:00Z to assess Phase 2 soak completion + render `docs/phase2-complete-verdict.md`. Day 0 sealed 2026-05-02; T0 = 2026-05-02T18:40:18Z.
type: project
originSessionId: dfdf0595-b2de-450e-a3e8-15ecb9220949
---
A one-time remote agent (routine `trig_01BNsPc4HkYQxzaxu3Fe9M5c`) is scheduled to fire **2026-05-10T14:00:00Z** (Sun May 10, 9 AM Chicago).

**Soak Day 0 sealed 2026-05-02.** T0 = **2026-05-02T18:40:18Z** (scaffold-completion timestamp from squirebot.log on the Azure VM). Logon-cycle smoke passed; cross-user watch verified; spellbook (49 rows) + inventory (250 rows) uploaded cleanly.

**Soak environment:**
- Test workbook ID: `1bx3I_x_4ppF8jNNuvvHTCtKk2VRMDO3tModc-fLgIhs`
- Throwaway Google account: `joseph.bowen2@gmail.com`
- Test EQ folder on Azure VM: `C:\Users\guildie\Desktop\FakeEQ\` (cross-user watch — admin SquireBot user reads standard guildie user's folder)
- Watched character: `Slampeach` (49 spellbook rows, 250 inventory rows from sample fixtures)
- Phase 1 leftover in workbook: `inv:VMTester` tab (harmless)

**Soak schedule (locked off T0):**
- Day 1 — 2026-05-03T18:40Z (Sun May 3, 1:40 PM Chicago): quota throttling injection
- Day 4 — 2026-05-06T18:40Z (Wed May 6, 1:40 PM Chicago): invalid_grant injection
- Day 6 — 2026-05-08T18:40Z (Fri May 8, 1:40 PM Chicago): corrupt update payload injection
- Day 7 — 2026-05-09T18:40Z (Sat May 9, 1:40 PM Chicago): final assertion sweep + soak report

**Why:** Phase 2 ships in two milestones (code-complete 2026-05-02, soak-validated when the 7-day soak passes). The agent observes the soak outcome at the natural calendar boundary, classifies it (`PASS` / `PARTIAL` / `NOT_STARTED` / `FAIL`), cross-checks the other deferred items (rc1 release, AUTH-03 negative test, SignPath app, logon-cycle smoke, LICENSE), and produces `docs/phase2-complete-verdict.md` with a ready-to-paste PR description and concrete next-action commands.

**Findings already captured for the Day-7 sweep:**
- ✅ FIXED in rc2 (commit b36909e): `_meta` and `_status` dimension tabs landed VISIBLE because `EnsureSheet` side-effects (from ValidateWorkbook + heartbeat) ran before scaffold's loop. Fix added a "present-but-visible → HideSheet" branch + tests. Verified in production 2026-05-04T03:45Z.
- ✅ FIXED in rc2 (commit 593a9af): tray red/green icon distinction was a stand-in (same bytes); Phase 5 polish promoted with distinct green/red icons (1118-byte BMP-in-ICO each, BGRA 00 CC 22 FF / 00 00 CC FF). Day-4 "tray turns red" pass criterion is now visually verifiable.
- ✅ DOCUMENTED in rc2 (commit bdf37e2): live SC-4 quota throttle test marked DEPRECATED in runbook; SC-4 evidence path now points to WATCH-07 unit suite as canonical.
- ⚠️ STILL OPEN: Manifest 404 root cause is GitHub's `/releases/latest/` URL skipping prereleases — NOT "latest.json not published" (rc2 publishes it). Will self-resolve when first non-prerelease v0.2.0 ships post-soak. See `project_phase2_soak_day1.md` Finding A correction.

**Mid-soak binary swap (2026-05-04T03:45Z):** Soak switched from rc1 to rc2 binary on the Azure VM after the three Day-1 follow-up fixes landed. Soak validity unaffected (Day-1 evidence was Option C unit-test-based; Days 4/6/7 run against rc2).

**How to apply:**
- The routine writes a verdict file but does **NOT** push tags or open PRs autonomously — the user owns the `phase2-complete` tag decision.
- GitHub is now connected (gh CLI authenticated + Claude GitHub App installed on `boejowen/SquireBot`), so the agent can attempt `gh pr create` for the verdict.
- Manage the routine at https://claude.ai/code/routines/trig_01BNsPc4HkYQxzaxu3Fe9M5c. Re-arm via update with a new `run_once_at` if soak gets delayed.

**Companion reminder routines (each fires 10 min before its 1:40 PM Chicago injection slot):**
- `trig_01SVa4Lgn6ioro4qgXCK2b2v` — Day-1 quota-throttle. Fires 2026-05-03T18:30:00Z (Sun May 3, 1:30 PM Chicago). Outputs OAuth Playground steps + storm + file-touch + pass criteria. Manage: https://claude.ai/code/routines/trig_01SVa4Lgn6ioro4qgXCK2b2v
- `trig_019BxZzis183AfXAddATtaQu` — Day-4 invalid_grant. Fires 2026-05-06T18:30:00Z (Wed May 6, 1:30 PM Chicago). Outputs Google Account revoke steps + tray-red verification + Reauthorize flow + pass criteria. Manage: https://claude.ai/code/routines/trig_019BxZzis183AfXAddATtaQu
- `trig_018bBE4fcd9VSTiwEgEPm3Yh` — Day-6 corrupt-update (Option A direct injection). Fires 2026-05-08T18:30:00Z (Fri May 8, 1:30 PM Chicago). Outputs garbage `.new` + bogus sidecar staging + restart + cleanup verification + pass criteria. Manage: https://claude.ai/code/routines/trig_018bBE4fcd9VSTiwEgEPm3Yh
