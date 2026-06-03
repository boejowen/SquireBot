---
phase: 18-watcher-cleanups-verify-or-close
plan: 01
status: complete
completed: 2026-06-02
requirements: [WATCH-12, WATCH-13, WATCH-14]
autonomous: false
key-files:
  created: []
  modified: []
tasks_completed: 3
code_changed: false
---

# Plan 18-01 SUMMARY — Watcher Cleanups (Verify-or-Close)

**Outcome:** All three carried-forward watcher fixes (WATCH-12/13/14) confirmed live-correct with **zero new code**. WATCH-14's ops residual closed: no production watcher was ever stuck — the `0.4.0-rc1` install is the disposable Azure test VM, not a guildie watcher. SC1–SC4 all satisfied.

## Task 1 — Confirm WATCH-12/13/14 code (auto) ✓

Verification evidence (all run against the live tree, no edits made):

| Success Criterion | Check | Result |
|-------------------|-------|--------|
| SC1 — WATCH-12 gofmt | `gofmt -l cmd/squirebot/console_windows.go` | **empty** (clean) |
| SC2 — WATCH-13 Debug-not-Warn | `grep -c slog.Debug` / `grep -c slog.Warn` on `console_windows.go` | **1 / 0**; freeConsole doc (lines 35–49) matches impl (50–61) — `ret==0` benign no-console case logged at Debug |
| SC3 — WATCH-14 IsNewer | `grep` for D-05 cases in `manifest_test.go` | both present at **lines 191–192** (`rc-updates-to-final` true, `final-does-not-downgrade-to-rc` false) |
| SC3 — test green | `go test ./internal/update/...` | **ok** |

**No code change was needed** — D-05's pre-release→final coverage (`TestIsNewer_SemVerPreRelease`, `2.0.0-rc1↔2.0.0`) was already present and green. `manifest_test.go` was NOT modified (the conditional fix in `files_modified` did not fire). The fixes shipped in v2.0 Plan 13-04 (`c930fc2`, `e758fb0`, `3e8e53b`) are all live.

## Task 2 — D-02 stale-version survey (auto, read-only) ✓

Ran one read-only `sqlite3` SELECT against the live backend (`/var/lib/squirebot/squirebot.db` on the Hetzner VPS, via non-interactive `ssh bash -s`; no DB writes, no schema/code/telemetry added):

```sql
SELECT c.name, c.watcher_version, c.last_seen, o.label, o.discord_user_id
FROM character c JOIN owner o ON o.id = c.owner_id
WHERE c.is_removed = 0 AND c.watcher_version IS NOT NULL
  AND (c.watcher_version LIKE '%-%' OR c.watcher_version < '2.0.0')
ORDER BY c.last_seen DESC;
```

**Result: 0 rows.** No backend-reporting watcher is on a pre-release or stale version. The full roster is **7 active characters across 3 owners (broccolifart, lern41, tontinethearistocrat), all on `2.0.0`**, all with fresh `last_seen` (2026-06-01). Distinct `watcher_version` values seen: `2.0.0` only (n=7). **No other guildie is silently stuck.**

## Task 3 — HUMAN-UAT reinstall (checkpoint) — RESOLVED: no production watcher stuck

The plan's premise (a real maintainer watcher stuck on `0.4.0-rc1` needing reinstall) was a **stale assumption carried from the v2.0 close**. The evidence refutes it:

- **This machine** (the maintainer's PC): `HKCU\...\Uninstall\SquireBot` → `DisplayVersion = 2.0.0`, running `...\AppData\Local\Programs\SquireBot\squirebot.exe`. On the current release — not stuck.
- **Backend:** every reporting watcher (7/7 toons) is on `2.0.0` (Task 2). No `0.4.0-rc1` present.
- **By elimination** (user-confirmed): the only `0.4.0-rc1` install is the **disposable Azure test VM**, which targets the old Google flow, does not report to the production backend, and has zero production impact.

**User decision (2026-06-02):** Close as resolved, no action. SC4 is satisfied — the maintainer's actual machine and all reporting guildies are on 2.0.0; the `0.4.0-rc1` test VM is not a production watcher and needs no reinstall. (A separate, non-blocking note: per the Azure-access memory the Azure sub is Pay-As-You-Go — if that test VM is still running it may be worth decommissioning to stop billing, tracked as a follow-up, NOT part of this milestone.)

## Self-Check: PASSED

- SC1 gofmt clean ✓ · SC2 Debug-not-Warn + doc/impl reconciled ✓ · SC3 IsNewer pre-release test green ✓ · SC4 no stuck production watcher; maintainer PC + all guildies on 2.0.0 ✓
- Zero new watcher code, zero schema changes, zero telemetry built, no Discord OAuth added to the watcher (HARD CONSTRAINT upheld) ✓
- Deciding rule honored: simplest guildie experience, no rebuilding shipped work ✓

## Deviations

The Task 3 checkpoint resolved without a reinstall because the stuck-watcher premise was stale — surfaced to the user with evidence; user chose "close as resolved." No code touched in the entire phase.
