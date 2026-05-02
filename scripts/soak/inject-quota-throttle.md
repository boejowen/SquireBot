# Inject: Sheets API quota throttling (429)

**Validates:** SC-4 — exponential backoff, Retry-After honoring, transient-recovery semantics.

## Procedure

Goal: emit enough requests against the Sheets API in a short window to exhaust the per-user 60s read or 60s write quota, observed by Google as 429s. Cleanest path: a synthetic write storm from a separate process pointing at the SAME workbook the watcher is using.

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

## Pass criteria

- At least one 429 (or `userRateLimitExceeded` / `rateLimitExceeded`) in the log.
- At least one successful `uploaded` line after the 429 (proves the retry envelope drained the schedule and recovered).
- No `permanent auth failure` log line during the throttle window.
- No tray transition to red.
- Heartbeat continues to fire on its 24h boundary (i.e., a `heartbeat written` line appears within the test window or the next tick).

## Run the assertion script

```powershell
.\scripts\soak\grep-log-assertions.ps1 -Scenario QuotaThrottle
```
