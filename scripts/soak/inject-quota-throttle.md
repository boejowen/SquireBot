# Inject: Sheets API quota throttling (429)

**Validates:** SC-4 — exponential backoff, Retry-After honoring, transient-recovery semantics.

> ⚠ **DEPRECATED for live testing (2026-05-03).** The OAuth-Playground-storm procedure documented as Option A below was attempted during the first Phase 2 soak and **could not produce a 429 in the watcher's path**, regardless of storm intensity. Two architectural barriers prevent the live test from working as designed. The canonical SC-4 evidence path is the unit suite; see § Canonical evidence (Option C) below.

## Why the live test is structurally infeasible

### Barrier 1: Google quotas are per-(project, user), not per-user

The original Option A (storm via OAuth Playground) authorizes against the **OAuth Playground's GCP project**, which has its own per-user quota (apparently ~60/min — much lower than typical, because the project is shared by many users). The watcher uses **SquireBot's GCP project**, which has its own independent per-user quota (~300/min). The two quota buckets are completely isolated.

Storming OAuth Playground exhausts THAT project's quota and produces 429s in the storm output. The watcher, authorizing against a different project, sees no contention and slips through cleanly. Verified empirically 2026-05-03: 200-request storm produced ~150 visible 429s; concurrent watcher upload completed in 625ms with zero retries.

### Barrier 2: Watcher mutex serializes all batchUpdates (Plan 02-03 Pitfall D fix)

Every `batchUpdate` is funneled through a single `*Client` mutex by design — this prevents concurrent batchUpdate races on shared ranges, which the architecture spec explicitly forbids.

The empirical consequence: the watcher physically cannot burst. 30 simultaneous fsnotify events get serialized into 30 sequential mutex-acquired uploads at ~2.4 sec each. Effective rate is well below Google's ~5/sec sustained per-user quota even under maximum file-drop pressure. Verified 2026-05-03: dropping 30 fixture files at once produced 30 sequential uploads over ~73 seconds with zero 429s.

For a live storm to throttle the watcher, both processes must use OAuth tokens issued by the **same OAuth client_id**. Practically that means storming via the watcher's OWN client (Option B below).

---

## Canonical evidence (Option C — recommended)

```bash
go test ./internal/sheet/... -run TestWithRetry -v
go test ./internal/sheet/... -run "TestClient_(batchUpdate|valuesGet|Mutex)" -v
```

This exercises every code path SC-4 requires, with mocked Google responses for every documented failure mode. The 13 `TestWithRetry_*` tests cover:

| Test | Validates |
|---|---|
| `SuccessOnFirstTry` | No-retry happy path |
| `SuccessAfterTransient5xx` | Transient 5xx recovery |
| `429WithRetryAfterSeconds` | Retry-After honoring (numeric) |
| `429WithRetryAfterHTTPDate` | Retry-After honoring (HTTP-date) |
| `429NoRetryAfterFallsThroughToSchedule` | Exponential backoff schedule |
| `403AuthErrorRefreshThenSuccess` | 403 → refresh → success |
| `403AuthErrorTwiceIsPermanent` | 403 twice = permanent |
| `403UserRateLimitFallsThroughToSchedule` | 403 quota path |
| `403ForbiddenIsPermanentAfterRefresh` | 403 forbidden permanent |
| `5xxExhausted` | Persistent-failure surfacing |
| `NonGoogleAPIErrorIsTransient` | Non-API errors as transient |
| `400IsNotRetried` | 400 not retried |
| `CtxCancellationDuringSleep` | Cancellation during backoff sleep |

Plus the mutex-serialization tests confirming Barrier 2 is intentional behavior, not a bug.

The unit suite is strictly stronger evidence than the live test would have been: it validates every failure mode in isolation with controlled responses, while a live test could at best validate that the same code runs once with whatever response Google happens to return.

---

## Option A — DEPRECATED: OAuth Playground storm

> Kept here for historical context. Do NOT run as part of soak validation.

Goal: emit enough requests against the Sheets API in a short window to exhaust the per-user 60s read or 60s write quota, observed by Google as 429s. Cleanest path: a synthetic write storm from a separate process pointing at the SAME workbook the watcher is using.

Reason this fails: see § Barrier 1 above. The storm exhausts the OAuth Playground project's quota; the watcher's separate project quota is untouched.

1. Open the test workbook in your browser. Note the spreadsheet ID from the URL.

2. From any machine with curl + a separately-OAuth'd token (e.g., a quick gcloud CLI session against the same project), run the storm. Below is the **bash/WSL/Git-Bash** form:

   ```bash
   # In a Bash/WSL/Git-Bash shell. Adjust SPREADSHEET_ID and ACCESS_TOKEN.
   SPREADSHEET_ID="..."
   ACCESS_TOKEN="$(gcloud auth print-access-token)"
   for i in $(seq 1 200); do
     curl -s -X POST \
       -H "Authorization: Bearer $ACCESS_TOKEN" \
       -H "Content-Type: application/json" \
       -d '{"requests":[{"updateCells":{"range":{"sheetId":0,"startRowIndex":0,"endRowIndex":1,"startColumnIndex":0,"endColumnIndex":1},"rows":[{"values":[{"userEnteredValue":{"stringValue":"storm"}}]}],"fields":"userEnteredValue"}}]}' \
       "https://sheets.googleapis.com/v4/spreadsheets/${SPREADSHEET_ID}:batchUpdate" \
       -o /dev/null -w "%{http_code} " &
   done
   wait
   echo
   ```

   This fires 200 batchUpdate calls in parallel. Within ~10 seconds, you should see 429 status codes returned for at least some — Google's per-user quota is 60 write requests per 60 seconds.

   **PowerShell equivalent (no gcloud required if you've got a token via another path):**
   ```powershell
   $SpreadsheetId = "..."
   $AccessToken   = "..." # paste from `gcloud auth print-access-token` or any other source
   $Body = '{"requests":[{"updateCells":{"range":{"sheetId":0,"startRowIndex":0,"endRowIndex":1,"startColumnIndex":0,"endColumnIndex":1},"rows":[{"values":[{"userEnteredValue":{"stringValue":"storm"}}]}],"fields":"userEnteredValue"}}]}'
   1..200 | ForEach-Object -Parallel {
     $r = Invoke-WebRequest -Method POST -Uri "https://sheets.googleapis.com/v4/spreadsheets/$using:SpreadsheetId`:batchUpdate" `
       -Headers @{ Authorization = "Bearer $using:AccessToken"; "Content-Type" = "application/json" } `
       -Body $using:Body -SkipHttpErrorCheck
     Write-Host $r.StatusCode -NoNewline
   } -ThrottleLimit 50
   ```

3. **Simultaneously** trigger a watcher upload by touching an inventory file. The watcher's batchUpdate will land in the same throttled window:
   ```powershell
   (Get-Item "$env:LOCALAPPDATA\Soak\Slampeach-Inventory.txt").LastWriteTime = Get-Date
   ```

4. Observe the watcher's log:
   ```powershell
   Get-Content -Tail 30 "$env:LOCALAPPDATA\SquireBot\squirebot.log" -Wait
   ```
   Expect: at least one error line mentioning 429 or `userRateLimitExceeded`. Subsequent log lines (separated by ~2s, ~4s, ~8s timestamps) should show the retry attempts. Eventually a successful `uploaded` line.

5. Wait ~5 minutes for the quota window to fully close. Touch the file again — should upload immediately and normally.

### Option A pass criteria (kept for historical reference)

These were the original criteria. **They cannot actually be met in practice** because of the per-(project, user) quota isolation described in Barrier 1 — no number of OAuth-Playground-storm requests will produce a 429 in the watcher's log path.

- At least one 429 (or `userRateLimitExceeded` / `rateLimitExceeded`) in the log.
- At least one successful `uploaded` line after the 429 (proves the retry envelope drained the schedule and recovered).
- No `permanent auth failure` log line during the throttle window.
- No tray transition to red.
- Heartbeat continues to fire on its 24h boundary.

### Option A assertion script

```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario QuotaThrottle
```

This script grep's the live log for the criteria above; running it after a Playground storm will return FAIL on the "429 in log" criterion (because the watcher's log won't contain one). Use Option C's `go test` invocation as the canonical evidence path instead.

---

## Option B — Watcher-OAuth-client storm (advanced; only if specifically required)

> Estimated 30–60 min of test infrastructure work. Skip unless investigating a specific Google-side regression that the Option C unit suite cannot catch.

To actually throttle the watcher with a live storm, the storm must use a token issued by **SquireBot's own OAuth client_id** (defeats Barrier 1) and the watcher's mutex serialization must be acknowledged as the throughput ceiling (Barrier 2 cannot be defeated — it's intentional).

### Sketch

1. **Locate the wincred entry holding the watcher's refresh token:**
   ```powershell
   cmdkey /list | findstr SquireBot
   ```
   The target name is the `Generic` credential SquireBot writes via the `wincred` package. The blob is DPAPI-encrypted to the user account currently signed in (typically the `SquireBot` admin account in soak setups).

2. **Decrypt and parse the blob.** Wincred via `cmdkey` does NOT print the secret. Use the Win32 `CredRead` API via PowerShell P/Invoke to read the credential blob, then JSON-decode the contained refresh token. Alternatively, write a small Go helper that reuses `internal/auth`'s wincred reader and prints the refresh token to stdout (do NOT commit this helper).

3. **Exchange the refresh token for an access token** via Google's `/token` endpoint:
   ```powershell
   $resp = Invoke-RestMethod -Method POST -Uri "https://oauth2.googleapis.com/token" -Body @{
       grant_type    = "refresh_token"
       refresh_token = $REFRESH_TOKEN
       client_id     = $SQUIREBOT_CLIENT_ID
       client_secret = $SQUIREBOT_CLIENT_SECRET  # see locked decision #10: required even with PKCE
   } -ContentType "application/x-www-form-urlencoded"
   $AccessToken = $resp.access_token
   ```
   `client_id` and `client_secret` come from the OAuth config bundled into the watcher build (see `OAUTH_CONFIG_JSON` CI secret or the local source-of-truth file).

4. **Storm using the watcher's token.** Same loop body as Option A, but now hitting SquireBot's per-user quota bucket. The watcher's concurrent upload will land in a contended window and trigger 429 → WATCH-07 backoff → eventual success.

5. **Pass criteria** — same as Option A's original list, now actually achievable.

### Why this isn't worth it for typical soak validation

- The `internal/sheet/retry.go` code path is identical between unit tests and live execution. The unit tests use `httptest`-mocked Google responses; a live Option B test uses real Google responses. Same code, different envelope.
- Extracting + handling the wincred refresh token introduces a non-trivial credential-handling step that's easy to get wrong.
- The Day-7 soak report has alternative SC-4 evidence (Option C unit suite) that is strictly more comprehensive.

Reach for Option B only if you have a hypothesis that requires real Google responses to validate (e.g., "Google changed their 429 envelope format and the unit fixtures are stale").
