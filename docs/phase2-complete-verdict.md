# Phase 2 complete — verdict

**Generated:** 2026-05-09 (7 days post-code-complete; phase2-code-complete tag set 2026-05-02)
**Soak status:** PASS
**Verdict:** Ready to tag `phase2-complete`

> Reconstructed from source materials after the original verdict commit (`1a77d74`, written by the cloud agent for routine `trig_01BNsPc4HkYQxzaxu3Fe9M5c`) failed to push from the Anthropic cloud sandbox (proxy 403 on outbound git remotes). Classification (PASS) and recommended next actions preserved from the agent's final summary; prose synthesized locally from the consolidated soak report and commit history.

## Soak summary

All four checkpoint days verified PASS in the consolidated report `docs/soak-reports/2026-05-07-day4-auth05-sc1.md`. Soak status banner reads `COMPLETE`.

| Day | Date verified | Scenario | Validates | Status |
|---|---|---|---|---|
| 1 | 2026-05-03 | Quota throttling injection (Option C — unit-test evidence) | SC-4 backoff/retry | ✅ PASS |
| 4 | 2026-05-07 | `invalid_grant` injection + Reauthorize flow | SC-1 invalid_grant, AUTH-05 | ✅ PASS |
| 6 | 2026-05-08 | Corrupt update payload injection | SC-1 corrupt update, SC-5 auto-update | ✅ PASS |
| 7 | 2026-05-08 | Final sweep (heartbeat count, log integrity, workbook integrity, AllPhase2 assertion script) | SC-2 autostart + heartbeat, SC-3 schema, SC-6 spellbook + soft-delete | ✅ PASS |

**ROADMAP success criteria (all 8):**

| SC | Status | Evidence |
|---|---|---|
| SC-1 invalid_grant | PASS | Day 4 |
| SC-1 corrupt update | PASS | Day 6 |
| SC-2 autostart | PASS | NSIS install's HKCU\\…\\Run entry verified during Day-7 admin login |
| SC-2 heartbeat | PASS | Day 7 (≥7 heartbeats observed) |
| SC-3 schema | PASS | Unit tests + sheet-side `_meta.schema_version=1` verified Day 7 |
| SC-4 backoff/retry | PASS (via Option C) | Day 1 — WATCH-07 unit suite (13 `TestWithRetry_*` tests) |
| SC-5 auto-update | PASS | Day 6 |
| SC-6 spellbook + soft-delete | PASS | Unit tests + `_char_owner.is_removed` column present in workbook schema, verified Day 7 |

## Notable findings from soak

**1. `drive.file` write-access propagation delay (~50 minutes) — discovered Day 4.** Google's `drive.file` scope exhibits asymmetric read/write propagation after Drive Picker re-registers a workbook under a new OAuth grant: read calls succeed immediately, but `spreadsheets.values.batchUpdate` returns 401 for ~50 minutes. Empirical worst case observed: 51 minutes (50 probe attempts at 60s intervals). Not documented anywhere in Google's `drive.file` reference. Fix shipped as `runPostReauthProbe` in commit `304b8bb`: a background goroutine outside `batchMu` polling `PingWriteNoLock` every 60s with a 90-min timeout. While the probe runs, the tray stays green with status "Reauthorized: waiting for Google propagation…", and the Reauthorize menu item is hidden. On probe success, suspension clears and uploads resume on the next file event. Documented in `docs/soak-reports/2026-05-07-day4-auth05-sc1.md` § Day 4, in `README.md` § Known issues (commit `4c7771e`), and in `.planning/research/PITFALLS.md` as Pitfall 7a (local).

**2. The "staged hash mismatch" log line cannot exist by design — Day 6 finding.** The original soak runbook expected `update.Apply()` to log a "staged hash mismatch" message when rejecting a corrupt `.new` payload. In practice that line is unverifiable in `squirebot.log`: `update.Apply()` runs in `cmd/squirebot/main.go` BEFORE `logging.Setup()`, so swap errors go to stderr only via `fmt.Fprintf(os.Stderr, ...)`, and `Start-Process` discards stderr by default. The corrupt-rejection produces zero user-visible evidence in production. Behavioral evidence (`.new` and `.expected-sha256` deleted; binary size unchanged) is the canonical proof. Suggests a Phase 4 observability item: log the early-startup swap result to a separate `swap.log` via stdlib before slog is set up.

**3. Two soak-process bugs uncovered and fixed.**
- **`[byte[]](1..1024)` PowerShell cast bug** (commit `d47466c`): the original Day-6 injection script and routine prompt used this expression to generate corrupt bytes. Fails in PowerShell because `[byte]` is unsigned 8-bit and the cast errors at value 256. Replaced with `New-Object byte[] 1024`.
- **Assertion script (`scripts/soak/grep-log-assertions.ps1`) needed three corrections** (commit `8106427`): removed the impossible 429-log requirement (replaced with WATCH-07 unit-suite reference per Day-1 finding), removed the false-positive no-permanent-auth-failure absence check (Day-4's invalid_grant produces these legitimately), and replaced the unverifiable staged-hash-mismatch requirement with behavioral evidence. Added `-ExePath` parameter with auto-detect to handle both NSIS-managed and hand-rolled install layouts.

## Deferred-items status

| Item | Status | Notes |
|---|---|---|
| `v0.2.0-rc1` release | ✅ Released 2026-05-02 | Tag `v0.2.0-rc1` present; CI workflow ran successfully |
| `v0.2.0-rc2` release | ✅ Released 2026-05-04 | Tag `v0.2.0-rc2` present; included Day-1 follow-up fixes (`b36909e` dimension-tab visibility, `593a9af` distinct icon bytes, `bdf37e2` SC-4 deprecation note) |
| AUTH-03 negative test | OPEN with effective waiver | The OAuth-consent-screen-Production gate enforces itself in CI (`release.yml` AUTH-03 check). Whether to capture an explicit "deliberately-failing rc was tagged + observed" test record is the user's call. The CI gate's existence + Production status verified pre-rc1 is the practical equivalent. Suggest documenting the waiver in a brief follow-up commit. |
| SignPath OSS application | ✅ SUBMITTED 2026-05-02 | Pending SignPath Foundation review (~1–4 weeks per community reports). Tracker at `docs/signpath-application.md`. Not blocking Phase 2 closure — Phase 2 ships unsigned + walkthrough by design. |
| Logon-cycle smoke test | ✅ PASSED at Day-0 setup (2026-05-02) | Verified on Azure VM cold-start during soak environment sealing. Not re-tested mid-soak because soak install runs continuously; logon-cycle behavior is independent. |
| LICENSE file | ✅ Present at repo root | Committed `fa7e9b4` 2026-05-02. SignPath OSS eligibility gate satisfied (commit `e163a64` recorded the closure). |

## Recommended next actions

In order of urgency:

### 1. Push the verdict + tag `phase2-complete`

```bash
git push origin master
git tag phase2-complete
git push origin phase2-complete
```

This finalizes the milestone marker. The `phase2-complete` tag is what downstream tooling (and future-you) will reference as the canonical "Phase 2 done" point.

### 2. Cut the final `v0.2.0` release

```bash
git tag v0.2.0
git push origin v0.2.0
```

This fires `.github/workflows/release.yml` which:
- Builds the NSIS installer
- Computes SHA-256 sums
- Publishes `latest.json` (resolves the auto-update manifest 404 issue noted on Day 0)
- Creates the GitHub Release page that guildies will download from

**Decision point before tagging:** the AUTH-03 negative-test waiver. The CI gate enforces the OAuth-consent-Production requirement automatically; the question is whether you want an explicit test record committed first. Recommend either (a) skip — gate's existence is sufficient documentation, or (b) add a 3-line note to a follow-up commit before tagging. Either is fine; don't let this block the release.

### 3. Distribute to guildies

Once `v0.2.0` is live on the GitHub Releases page:
- Send the GitHub Releases URL + `docs/smartscreen-walkthrough.md` link to your guild Discord
- **Confirm the OAuth consent screen is set to Production in the GCP console** before any guildie installs (Testing-mode tokens silently expire after 7 days — Pitfall #1, the existential one). The CI AUTH-03 gate enforces this for the binary, but the GCP-console toggle is a manual setting; verify it directly in the console.

## PR description (copy-paste-ready)

If you decide a marker PR is more appropriate than a release-only flow, the body below is ready to paste into `gh pr create` (though since work landed directly on `master`, a release tag is probably more idiomatic — see § 2 above).

**Title:** `chore(release): Phase 2 complete — Watcher Robustness + Schema Lock`

**Body:**

```markdown
## Phase 2: Watcher Robustness + Schema Lock — COMPLETE

Code-complete: 2026-05-02 (`phase2-code-complete` tag)
Soak-validated: 2026-05-08 (all 4 checkpoint days PASS)
Verdict: 2026-05-09 (`docs/phase2-complete-verdict.md`)

### Scope

~30 commits since `phase2-code-complete`, including:
- 8 `fix(auth):` commits (`13f4dac` → `304b8bb`) implementing the `drive.file` post-Reauthorize propagation-wait probe (`runPostReauthProbe`) — discovered during Day-4 soak
- 2 release-pipeline fixes (`8a78327` prerelease tag detection, `a6bf682` Go version bump, `b6e2c1b` actions Node 24 bump)
- 4 soak-doc commits filing Day-1, Day-4, Day-6, Day-7 PASS evidence into `docs/soak-reports/`
- 1 design doc lock-in for Phase 3 EQ-aesthetic theme system (`170966e` → `2a686cd`)
- README Known-issues section (`4c7771e`) and assertion-script alignment (`8106427`, `d47466c`)

### Success criteria covered

All 8 ROADMAP SCs PASS — see `docs/phase2-complete-verdict.md` for the full table.

### Notable architectural finding

Google's `drive.file` scope has a ~50-minute write-access propagation delay after Drive Picker re-registration (read access propagates immediately). Not documented in Google's reference. Mitigation: `runPostReauthProbe` background goroutine with 90-min timeout; tray stays green with "Reauthorized: waiting for Google propagation…" status during the wait. See `README.md` § Known issues, `docs/soak-reports/2026-05-07-day4-auth05-sc1.md` § Day 4 Findings, and `.planning/research/PITFALLS.md` Pitfall 7a (local).

### Deferred to future phases

- SignPath OSS Code Signing — application submitted 2026-05-02, pending Foundation review
- Cross-user-watch install layout — soak validated single-user-as-guildie pattern; cross-user (admin SquireBot reads guildie folder) deferred to Phase 4
- Stderr observability for early-startup swap rejection — Phase 4 polish item

Phase 3 (Apps Script + view tabs + sidebar) unblocked.
```
