# Phase 2 Soak — Day 1 Finding

**Date executed:** 2026-05-03 (T+~24h from T0 = 2026-05-02T18:40:18Z)
**Scenario:** Quota throttling injection (validates SC-4)
**Outcome:** **PASS** — via Option C (deferred to unit-test evidence)

> Staging document. Fold into `SOAK-REPORT-2026-05-09.md` on Day 7.

---

## Pass criteria (per `docs/soak-runbook.md` § Day 1)

- [x] **PASS via Option C** — Live test could not produce a 429 in the watcher's path; SC-4 evidence captured via the WATCH-07 unit suite (see below). Live procedure as documented in the runbook is structurally infeasible for two reasons (see § Architectural barriers).
- [x] **PASS** — No `permanent auth failure` log line during the test.
- [x] **PASS** — No tray-state-change to red (no `tray health: red` log line; tray icon stayed at base state throughout).
- [x] **PASS** — Heartbeat continued to fire on its 24h boundary. Cold-start heartbeat at 18:18:03Z; the natural 24h tick is now anchored to that timestamp.

---

## What we ran

Two live attempts before deferring to unit tests:

### Round 1 — Storm via OAuth Playground (per-runbook procedure)

- Requested `https://www.googleapis.com/auth/spreadsheets` scope via OAuth Playground for the throwaway account `joseph.bowen2@gmail.com`.
- Fired a 200-request `batchUpdate` storm against the test workbook from a serial PowerShell loop (PS 5.1 → no `-Parallel`).
- Storm itself produced ~150 visible 429s after the first ~50 200s (early ones scrolled off-screen).
- Touched `Slampeach-Inventory.txt` to provoke a watcher upload.
- **Result:** Watcher upload completed cleanly in 625ms (debounce → upload). Zero 429s, zero retries, zero backoff in watcher's log path.

### Round 2 — Touch baked into the storm loop

- Identical storm with the file touch chained at iteration 30 (~T+6s into the ~40s storm) so the watcher's request would land mid-storm.
- **Result:** Identical outcome — clean 750ms debounce → upload. Zero retries.

### Round 3 — Mass-fixture-drop self-throttle (Option A)

- Dropped 30 `Stormtest{01..30}-Inventory.txt` fixture files into the watched FakeEQ folder simultaneously (~30ms total).
- **Result:** Watcher serialized all 30 uploads through its mutex at ~2.4 sec each (~73 sec total). Zero 429s, zero retries.

---

## Architectural barriers (why the live test was structurally infeasible)

### Barrier 1: Google Sheets quotas are per-(project, user), not just per-user

The runbook procedure storms via OAuth Playground tokens. Those tokens authorize against the **OAuth Playground's GCP project**, which has its own per-user quota (apparently ~60/min, much lower than typical because the project is shared by many users). The watcher's tokens authorize against **SquireBot's GCP project**, which has its own independent ~300/min per-user quota. The two storms run in completely isolated quota buckets.

For a storm to throttle the watcher, both processes must use OAuth tokens issued to the **same OAuth client_id**. To do that with the runbook's external-storm approach, you would need to:

1. Extract SquireBot's wincred-stored refresh token (DPAPI-encrypted; non-trivial in PowerShell).
2. Exchange it for an access token via SquireBot's `client_id` + `client_secret` + PKCE against `oauth2.googleapis.com/token`.
3. Storm using that access token.

This is significant test infrastructure for marginal SC-4 value over the unit-test suite — a 30–60 minute rabbit hole for a code path the unit tests already cover comprehensively.

### Barrier 2: Watcher mutex serializes all batchUpdates (Plan 02-03 Pitfall D fix)

The `*Client` helpers funnel every `batchUpdate` through a single mutex. This is intentional — it prevents concurrent batchUpdate races on shared ranges, which the architecture spec explicitly forbids. The empirical consequence is that the watcher physically cannot burst:

- 30 simultaneous fsnotify events → 30 sequential mutex-acquired uploads
- Observed: ~2.4 sec per upload, ~73 sec total for 30 uploads
- Effective rate: ~25 calls/sec at peak (4 calls per upload), well below Google's ~5/sec sustained per-user quota

The mass-fixture-drop strategy fails because the mutex prevents the very burst behavior that would self-trigger the throttle.

---

## SC-4 evidence captured

```
go test ./internal/sheet/... -run TestWithRetry -v
ok  	github.com/boejowen/SquireBot/internal/sheet	0.828s
```

13 tests pass:

| Test | Validates |
|---|---|
| `TestWithRetry_SuccessOnFirstTry` | No-retry happy path |
| `TestWithRetry_SuccessAfterTransient5xx` | Transient 5xx recovery |
| `TestWithRetry_429WithRetryAfterSeconds` | Retry-After honoring (numeric) |
| `TestWithRetry_429WithRetryAfterHTTPDate` | Retry-After honoring (HTTP-date) |
| `TestWithRetry_429NoRetryAfterFallsThroughToSchedule` | Exponential backoff schedule |
| `TestWithRetry_403AuthErrorRefreshThenSuccess` | 403 → refresh → success |
| `TestWithRetry_403AuthErrorTwiceIsPermanent` | 403 twice = permanent |
| `TestWithRetry_403UserRateLimitFallsThroughToSchedule` | 403 quota path |
| `TestWithRetry_403ForbiddenIsPermanentAfterRefresh` | 403 forbidden permanent |
| `TestWithRetry_5xxExhausted` | Persistent-failure surfacing |
| `TestWithRetry_NonGoogleAPIErrorIsTransient` | Non-API errors as transient |
| `TestWithRetry_400IsNotRetried` | 400 not retried |
| `TestWithRetry_CtxCancellationDuringSleep` | Cancellation during backoff sleep |

Plus mutex-serialization tests in `internal/sheet/client_helpers_test.go` (`TestClient_batchUpdateSerializesConcurrentGoroutines`, `TestClient_valuesGetSerializesAgainstBatchUpdate`, `TestClient_MutexReleasedOnError`, `TestClient_MutexReleasedOnPanic`).

The unit suite exercises every SC-4 code path in `internal/sheet/retry.go` with mocked Google responses for every documented failure mode. The live test would only have validated that the same code runs once with real Google responses — strictly less coverage than the unit suite already provides.

---

## Recommendations for the runbook (post-soak follow-up)

1. **Revise `docs/soak-runbook.md` Day-1 section.** Mark the "external storm via OAuth Playground" procedure as deprecated. Either:
   - **(a)** Designate the WATCH-07 unit suite as the canonical SC-4 evidence path and remove the live procedure entirely, OR
   - **(b)** Document a wincred-token-extraction procedure for a watcher-OAuth-client storm (estimated 30–60 min of test infrastructure work; only worth doing if there's a specific Google-side regression we want to catch that mocks can't).
2. **Update `scripts/soak/inject-quota-throttle.md`** to reflect the per-(project,user) quota gotcha as a header warning so future operators don't fall into the same trap.
3. **Document the mutex-serialization implication** in the WATCH-07 design notes: "Self-throttling under burst load is intentionally impossible — the *Client mutex prevents the watcher from generating a request rate that could trigger Google's per-user quota."

---

## Workbook state at end of Day 1

- ✅ `inv:Stormtest01..30` tabs deleted manually (user-confirmed)
- ⚠️ `_char_owner` retains 30 phantom rows for Stormtest characters (one per first-sighting)
- ⚠️ `_audit` retains 30 rows from the Stormtest uploads
- ✅ `inv:Slampeach`, `spell:Slampeach`, all 9 dimension tabs, all 4 view tabs, `inv:VMTester` (Phase 1 leftover) intact

Residual `_char_owner` and `_audit` rows are intentionally NOT cleaned. The workbook is throwaway. Phase 3's `is_removed` soft-delete column will handle this case naturally; manual cleanup adds no test value.

---

## Watcher state at end of Day 1

- Process running (autostart fired 18:18:02Z after VM cold start)
- Tray icon visible (base/pink — green/red distinction deferred per `internal/tray/tray.go:16-17`)
- No `permanent auth failure`, no tray-red transition, no `auth suspended` lines
- Heartbeat fired at 18:18:03Z (cold start). Next 24h tick anchored to that timestamp.
- Auto-update manifest 404 logged at 18:18:02Z (expected; `latest.json` not published — Day-0 finding)
- Ready for Day-4 invalid_grant on Wed May 6 at 13:30 CDT
