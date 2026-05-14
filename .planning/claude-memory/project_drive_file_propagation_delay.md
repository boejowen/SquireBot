---
name: drive.file write-access propagation delay (~50 min)
description: After Drive Picker re-registers a workbook under a new OAuth grant, Spreadsheets.Get works immediately but spreadsheets.values.batchUpdate returns 401 for ~50 minutes. Asymmetric read vs. write propagation is the post-Reauthorize hazard.
type: project
originSessionId: 20b42836-7bb6-4c18-8e07-c2e0277b46d5
---
After a successful Reauthorize+picker cycle, Google's `drive.file` scope exhibits **asymmetric propagation** between read and write access:

- **Read** (`Spreadsheets.Get`): succeeds immediately under the new grant.
- **Write** (`spreadsheets.values.batchUpdate`): returns 401 for an extended window while Google propagates the new grant's drive.file write registration to all Sheets API backends.

**Observed window in Day-4 soak (2026-05-07):** 51 minutes (picker complete 15:45:57Z → first successful write probe 16:37:23Z, 50 probe attempts at 60s intervals). A second cycle earlier the same day showed the window between 25 and 56 minutes (probe attempts 1–25 still failing at 25:23 elapsed; cold-start catch-up succeeded at 56:00 elapsed).

**Why:** Google's `drive.file` scope is per-(file, grant). The picker's API call registers the file under the new grant on whatever Sheets API server it happens to hit. Subsequent batchUpdate calls land on different backends that haven't propagated the registration yet. Read replicas appear to propagate via a fast path; write authorization does not.

**How to apply:**
- Any "Reauthorize → write succeeds within minutes" expectation is wrong. Plan for tens of minutes.
- The shipped fix (commit `304b8bb`) is `runPostReauthProbe`: a background goroutine outside `batchMu` that calls `PingWriteNoLock` (empty batchUpdate, no mutex) every 60s with a 90-min timeout. While the probe runs, `globalAuthSuspended` keeps watcher events from queueing on `batchMu`; tray stays green with status "Reauthorized: waiting for Google propagation…"; Reauthorize menu item stays hidden.
- Do NOT hold `batchMu` during the probe loop (the 0.2.7-soak attempt did this and blocked heartbeat + all other API calls for 25 min). Always probe in a goroutine outside the mutex.
- Do NOT cap the probe at 25 min — observed propagation exceeded that on a clean test account. 90 min has ~40 min headroom against the observed 51-min worst case.
- If real-world propagation ever exceeds 90 min, the timeout surfaces a Reauthorize prompt as a last resort — but do not lower this value without fresh evidence.
- This finding contradicts the implicit assumption in the original AUTH-05 design that Reauthorize → next-write-succeeds is synchronous. Update PITFALLS.md / Phase 2 design notes when next touched.

**Related commits (chain that landed during the Day-4 soak):**
- `13f4dac` — handle 401 from Sheets API as permanent auth failure
- `2d7128d` — onRefresh rebuilds TokenSource from wincred after Reauthorize
- `4f20fc2` — PingNoLock after token swap (later removed)
- `9ba1759` — add picker phase to Reauthorize to re-register workbook under new drive.file grant
- `7072955` — remove PingNoLock from onRefresh (picker Phase 2 handles registration)
- `32d19d4` — pass picker's token source to onRefresh via channel (`globalReauthTSCh`)
- `c9aef96` — probe batchUpdate write access in onRefresh (held batchMu — superseded)
- `304b8bb` — move drive.file propagation probe to background goroutine (final design)

**Evidence:** `docs/soak-reports/2026-05-07-day4-auth05-sc1.md` (Day 4 section); transcript at `docs/soak-transcripts/2026-05-07-day4-auth05-sc1.md`.
