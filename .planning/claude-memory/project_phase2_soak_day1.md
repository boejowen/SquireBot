---
name: Phase 2 soak Day-1 result + finding
description: Day-1 quota throttle test executed 2026-05-03. Live test infeasible due to per-(project,user) quota isolation + watcher mutex serialization. SC-4 evidence captured via 13 WATCH-07 unit tests passing.
type: project
originSessionId: dfdf0595-b2de-450e-a3e8-15ecb9220949
---
**Day-1 executed 2026-05-03** (~13:30 CDT, ahead of the original 13:40 schedule).

**Outcome: PASS via Option C (deferred to unit-test evidence).**

**Live test results (Round 1 + Round 2):**
- OAuth Playground storm of 200 batchUpdate requests → ~150 returned 429 against the OAuth Playground GCP project's per-user quota.
- BUT the watcher's batchUpdates (touch-triggered uploads to inv:Slampeach) slipped through cleanly with zero retries — 625-820ms debounce-to-upload, no errors.
- Mass-fixture-drop (Option A): 30 Stormtest character files dropped simultaneously. Watcher serialized all 30 uploads through its mutex at ~2.4 sec each (~73 sec total). Zero 429s, zero retries.

**Why live test was structurally infeasible (TWO architectural barriers):**
1. **Google quotas are per-(project, user)**, not just per-user. A storm from OAuth Playground's GCP project doesn't share the quota bucket with SquireBot's OAuth client. To actually throttle the watcher externally, you'd need to extract SquireBot's wincred refresh token, exchange via SquireBot's client_id+client_secret, and storm using that — significant test infrastructure work for marginal value.
2. **Watcher mutex (Plan 02-03 Pitfall D fix)** serializes all batchUpdates through `*Client` helpers. The watcher physically cannot burst — it processes uploads sequentially at ~1/sec, well below Google's 5/sec sustained per-user quota. Even 30 simultaneous file events get serialized.

**SC-4 evidence captured:** `go test ./internal/sheet/... -run TestWithRetry -v` → all 13 tests PASS in 0.83s:
- TestWithRetry_SuccessOnFirstTry
- TestWithRetry_SuccessAfterTransient5xx
- TestWithRetry_429WithRetryAfterSeconds (Retry-After numeric)
- TestWithRetry_429WithRetryAfterHTTPDate (Retry-After HTTP-date)
- TestWithRetry_429NoRetryAfterFallsThroughToSchedule (exponential backoff)
- TestWithRetry_403AuthErrorRefreshThenSuccess
- TestWithRetry_403AuthErrorTwiceIsPermanent
- TestWithRetry_403UserRateLimitFallsThroughToSchedule
- TestWithRetry_403ForbiddenIsPermanentAfterRefresh
- TestWithRetry_5xxExhausted
- TestWithRetry_NonGoogleAPIErrorIsTransient
- TestWithRetry_400IsNotRetried
- TestWithRetry_CtxCancellationDuringSleep

Plus mutex serialization tests in `internal/sheet/client_helpers_test.go`.

**Action items for soak report:**
1. Document the runbook gap: `docs/soak-runbook.md` Day-1 procedure should be revised to reflect the per-(project,user) quota isolation issue and the mutex serialization preventing self-throttling. Recommend either: (a) defer to unit-test evidence as the canonical SC-4 path, OR (b) document a wincred-token-extraction procedure for the live test.
2. Pink-tray-icon finding: `internal/tray/tray.go:16-17` notes "green/red icon distinction is currently a stand-in (same bytes for both); a distinct red overlay is deferred." So Day-4 invalid_grant test cannot validate "tray turns red" visually — must rely on log signals + Reauthorize menu item visibility instead.

**Cleanup status (in progress):**
- 30 fixture files in `C:\Users\guildie\Desktop\FakeEQ\Stormtest*-Inventory.txt` — user to delete via `Remove-Item`
- 30 inv:Stormtest01..30 tabs in test workbook — user manually deleting (batchUpdate cleanup script errored; manual deletion via Sheets UI)
- 30 phantom rows in _char_owner + 30 rows in _audit — intentionally NOT cleaning (workbook is throwaway, soft-delete via is_removed deferred to Phase 3)

**Watcher state at end of Day-1:** still running healthy on Azure VM. No tray red transition, no permanent auth failure, heartbeat fired on cold-start (18:18:03Z). Ready for Day-4 invalid_grant on Wed May 6 at 13:30 CDT.

---

## Late-evening 2026-05-03 / early 2026-05-04 follow-up: rc2 cut and deployed mid-soak

After Day-1 closed, three Day-1 follow-up commits landed on master (b36909e scaffold-hide fix, bdf37e2 runbook revisions, 593a9af distinct green/red tray icons), then `v0.2.0-rc2` was tagged + released via CI. User reinstalled rc2 on the Azure VM at ~03:45:41Z 2026-05-04. Verified in production:
- New green tray icon visible (BGRA 00 CC 22 FF — Phase 5 polish promotion confirmed)
- Log line `scaffold: hid pre-existing dimension tab tab=_status` fired during the rc2 cold-start scaffold sweep — direct production evidence of the fix
- Both `_meta` and `_status` now show as hidden in the workbook's "All sheets" popup
- Touch-trigger upload: 1.2-second debounce-to-upload, 250 rows, no errors
- Heartbeat chars=32 (was 2 before Day-1) — 30 Stormtest entries still in `_char_owner` from Day-1 mass-fixture-drop; intentionally not cleaned per Day-1 finding

**Soak validity unaffected by binary swap.** Day-1 evidence was unit-test-based (Option C), so already locked in. Days 4, 6, 7 will execute against the rc2 binary.

## Finding A correction: manifest 404 root cause is /latest/ URL, not missing file

Day-0 finding originally claimed `latest.json` wasn't published in rc1. That was wrong. rc2 explicitly publishes `latest.json` (workflow step "Write latest.json (Phase 2 manifest with both URLs)" landed). Yet the auto-update check still 404s post-rc2 install:

```
auto-update check failed: CheckOnce manifest: manifest fetch
https://github.com/boejowen/SquireBot/releases/latest/download/latest.json: HTTP 404
```

**Real root cause:** GitHub's `/releases/latest/` URL component **always skips prereleases**. rc2 is auto-tagged `prerelease: true` (workflow detects the `-rc2` suffix). With no non-prerelease release in existence, `/latest/` resolves to nothing → 404.

**Implication for soak:**
- Day-6 corrupt-update test uses Option A (direct .new injection), so it's UNAFFECTED — the manifest fetch path isn't on its critical path.
- Auto-update goroutine will continue logging 404 warnings every 24h until a non-prerelease tag exists. That's noise, not a bug.
- After Phase 2 soak passes and the user promotes the first non-prerelease `v0.2.0`, `/latest/` will resolve correctly and the 404s will stop.

**Three remediation options for the verdict doc:**
1. Promote a non-prerelease v0.2.0 immediately after soak passes — fixes `/latest/` resolution naturally.
2. Make the auto-updater configurable to follow prereleases during the rc cycle (code change; not blocking).
3. Document as known-limitation — auto-update is operational only after the first non-prerelease release. Acceptable for the rc cycle.

Recommended: option 1 (natural fix via promotion).
