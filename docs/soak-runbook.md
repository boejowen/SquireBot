# Phase 2 Soak-Validation Runbook

> Goal: Prove ROADMAP Phase 2 success criteria 1, 2 (heartbeat part), 4, and 5 (corrupt-update part) by running SquireBot continuously for 7 days with three deliberate injections.

**Audience:** the developer (single contributor; the user). One-time live validation, NOT a CI-automated pipeline. Run AFTER all Phase 2 plans 02-01 through 02-09 land.

**Duration:** 7 calendar days from start. Three scheduled injections (Day 1, Day 4, Day 6). Continuous heartbeat observation.

**Phase status:** As of the landing of Plan 02-10, Phase 2 is **CODE-COMPLETE**. The 7-day soak below is what flips Phase 2 from code-complete to **soak-validated**. Phase 3 planning may begin once code-complete; the soak runs in parallel and gates Phase 2 close-out, NOT Phase 3 start.

---

## Setup

1. **Clean Win11 box.** Recommended: an Azure D2s_v5 VM (matches Phase 1 smoke). Local hardware also fine if you can leave it powered on. You SHOULD use a **separate test SquireBot install** (clean Win11 user account or VM) with a **throwaway Google account** — the invalid_grant injection will revoke this account's OAuth grant, so do NOT use your real production guild watcher.

2. **Install SquireBot.** Use the latest tag's `SquireBot-Setup-X.Y.Z.exe` from the GitHub Releases page. Walk through SmartScreen per [docs/smartscreen-walkthrough.md](smartscreen-walkthrough.md). Complete the wizard with the throwaway Google account + a dedicated test workbook (do NOT use the production guild workbook).

3. **Confirm autostart is wired.** From PowerShell:
   ```powershell
   Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name SquireBot
   ```
   Expected: full path to `%LOCALAPPDATA%\Programs\SquireBot\squirebot.exe`, double-quoted.

4. **Sign out and sign back in. Confirm SquireBot tray appears within 30s WITHOUT manual launch.** This validates SC-2 autostart.

5. **Drop two test files into the watched EQ folder:**
   - `Slampeach-Inventory.txt` (any 5-col TSV; can copy from `internal/parse/testdata/sample-inventory.txt`)
   - `Slampeach-Spellbook.txt` (copy from `internal/parse/testdata/Slampeach-Spellbook.txt`)

   Confirm both upload to the test workbook within 30s. Tail `%LOCALAPPDATA%\SquireBot\squirebot.log` to observe.

6. **Start the soak clock.** Note the current UTC datetime. Schedule injections relative to it:
   - **Day 1 (T+24h):** Quota throttling injection (validates SC-4)
   - **Day 4 (T+96h):** invalid_grant injection (validates SC-1 + AUTH-05)
   - **Day 6 (T+144h):** Corrupt update payload injection (validates SC-1 + SC-5)
   - **Day 7 (T+168h):** Final assertion sweep + write the soak report

7. **Heartbeat observation:** every 24h, expect a `heartbeat written` log line. After Day 1, you should have seen ~2 of these (one immediate fire on each cold start of the watcher + one 24h tick).

8. **Prerequisite check before Day 6:** the corrupt-update test simulates a staged auto-update by directly writing a corrupt `.new` file next to `squirebot.exe`. It does NOT require a real `latest.json` to be served — the injection mocks the post-download state. If you DO want to also test the live download path, you'll need a tagged `v0.2.0-rc1` (or later) release on GitHub Releases — see Plan 02-08 SUMMARY's deferred Task 4 for the rc1 cutting steps.

---

## Day 1 — Quota throttling injection (SC-4)

**Validates:** Sheets API exponential backoff, Retry-After honoring, persistent-failure surfacing.

**Procedure:** see [scripts/soak/inject-quota-throttle.md](../scripts/soak/inject-quota-throttle.md).

**Expected outcome:**
- During the synthetic write storm, the log shows multiple `batchUpdate` failures with status code 429.
- The watcher backs off per the WATCH-07 schedule (look for delays of approx 2s, 4s, 8s between retry attempts in log timestamps).
- The tray status should NOT turn red — quota throttling is transient.
- After the storm subsides (~5 minutes), normal uploads resume.

**Pass criteria:**
- [ ] At least one log line shows a 429 error.
- [ ] At least one log line shows a successful retry after the 429 (i.e., an `uploaded` line for the file you touched during the storm).
- [ ] No `permanent auth failure` log line during the throttle.
- [ ] No tray transition to red.
- [ ] Heartbeat continues to fire on its 24h boundary.

Run the assertion script:
```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario QuotaThrottle
```

---

## Day 4 — invalid_grant injection (SC-1 + AUTH-05)

**Validates:** Refresh-token-death detection, tray red transition, Reauthorize click reopens OAuth, suspended-write semantics.

**Procedure:** see [scripts/soak/inject-invalid-grant.md](../scripts/soak/inject-invalid-grant.md).

**Expected outcome:**
- Within ~5 minutes of revoking the OAuth grant in the Google Account console, the next watcher upload fails with `permanent auth failure — suspending writes`.
- Tray icon transitions to red. Reauthorize… menu item becomes visible.
- Subsequent inventory file changes are observed in the log (`watcher debounced`) but the parse + write phase is SKIPPED with `auth suspended; skipping inventory` (or `... skipping spellbook`).
- Click Reauthorize. Browser opens. Complete OAuth.
- Within seconds: tray returns to green, log shows `Reauthorize complete`, the next file change uploads normally, the wincred entry is replaced (verify with `cmdkey /list | findstr SquireBot`).

**Pass criteria:**
- [ ] Log shows `permanent auth failure` AND `auth suspended` lines.
- [ ] No silent retry-loop (no log lines of repeated 401/403 errors AFTER the suspend trigger).
- [ ] Tray transition: green → red within 5 minutes of the revoke.
- [ ] Reauthorize click → log shows `Reauthorize start` and `Reauthorize complete`.
- [ ] After re-auth: log shows successful `uploaded` lines for new file changes.
- [ ] Tray returns to green; Reauthorize menu item hidden.

Run the assertion script:
```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario InvalidGrant
```

---

## Day 6 — Corrupt update payload injection (SC-1 + SC-5)

**Validates:** Auto-update SHA-256 verification rejects corrupted .new files; no broken install state.

**Procedure:** see [scripts/soak/inject-corrupt-update.md](../scripts/soak/inject-corrupt-update.md).

**Expected outcome:**
- On next startup (the swap point), `update.Apply()` sees the .new file but fails the SHA-256 verification.
- Both the .new and the .expected-sha256 sidecar are deleted.
- The OLD binary continues running normally.
- The 24h auto-update goroutine eventually re-tries the manifest fetch and (if a real new version exists) re-stages a clean .new on the next cycle.

**Pass criteria:**
- [ ] Log shows `staged hash mismatch: have ..., want ...` line.
- [ ] `%LOCALAPPDATA%\Programs\SquireBot\` directory does NOT contain `.new` or `.expected-sha256` files after the failed apply.
- [ ] The squirebot.exe present after the failed apply matches the version PRIOR to the corruption (verify via `(Get-Item ...\squirebot.exe).VersionInfo.FileVersion` or the log's startup line on the next launch).
- [ ] No tray transition to red (a corrupt update is not a user-actionable failure mode).

Run the assertion script:
```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario CorruptUpdate
```

---

## Day 7 — Final sweep

1. **Heartbeat count check.** Across 7 days, the heartbeat should have fired ~7-8 times (one immediate on each cold start + ~6-7 on 24h ticks, depending on whether the box was off during any night).
   ```powershell
   Select-String -Path "$env:LOCALAPPDATA\SquireBot\squirebot.log*" -Pattern 'heartbeat written' | Measure-Object | Select-Object Count
   ```
   Expected: ≥ 5 (allowing for box-off windows; the strict ROADMAP success criterion is "at least once daily for every active character," which 5+ over 7 days satisfies for a workstation that may sleep overnight).

2. **No silent retry loops.** Confirm no log line repeats more than ~10 times in succession with the same error.
   ```powershell
   Select-String -Path "$env:LOCALAPPDATA\SquireBot\squirebot.log*" -Pattern 'write inventory.*err' |
     Group-Object Line | Where-Object { $_.Count -gt 10 }
   ```
   Expected: empty (no group with count > 10).

3. **Workbook integrity.** Open the test workbook. Verify:
   - `_meta.schema_version` is `1`.
   - `_meta.canonical_id` is the expected workbook canonical id (check Plan 02-01 SUMMARY for the literal).
   - `_char_owner.last_seen` for Slampeach is within the last ~24h.
   - `_status.last_heartbeat` for Slampeach is within the last ~24h.
   - `_status.watcher_version` matches the binary's Version constant.
   - `inv:Slampeach` and `spell:Slampeach` tabs exist with expected columns.

4. **Final assertion script:**
   ```powershell
   .\scripts\soak\grep-log-assertions.ps1 -Scenario AllPhase2
   ```
   This runs the union of all per-scenario assertions plus the pass criteria above.

5. **Write the soak report:** copy this runbook into `.planning/phases/02-watcher-robustness-schema-lock/SOAK-REPORT-{date}.md`, mark each `[ ]` as `[x] PASS` or `[x] FAIL` with notes, and commit. Date = the Day 7 date.

---

## ROADMAP Success Criteria → Soak Test Mapping

| ROADMAP SC | Validated By | This Runbook Section |
|------------|--------------|----------------------|
| SC-1 (7-day run survives invalid_grant + corrupt update) | Day 4 + Day 6 + Day 7 sweep | Day 4, Day 6, Day 7 |
| SC-2 autostart | Setup step 4 | Setup |
| SC-2 heartbeat | Day 7 sweep step 1 | Day 7 |
| SC-3 schema | Plan 02-01 unit tests + Day 7 workbook check | Day 7 step 3 |
| SC-4 backoff + retry-after + 403 + tray | Day 1 quota throttle | Day 1 |
| SC-5 auto-update + walkthrough | Plan 02-06 unit tests + Day 6 corrupt-update + Plan 02-09 walkthrough doc | Day 6 |
| SC-6 spellbook + soft-delete | Plan 02-01 + 02-02 unit tests + Day 7 workbook check | Day 7 step 3 |

---

## Failure recovery during soak

If the watcher crashes or the box reboots mid-soak:
- Sign back in. Verify autostart fires (the heartbeat-on-cold-start log line confirms).
- Resume from the next scheduled injection. Soak duration is "approximate 7 days," not "168.0 hours of continuous uptime" — Phase 2's contract is "watcher is robust to typical user behavior," and signing-out-and-back-in is typical.

If an injection's pass criterion fails:
- Capture the relevant log lines + screenshot of the tray.
- File a Phase 2 hotfix (or a Plan 02-N reopener) before declaring Phase 2 complete.
