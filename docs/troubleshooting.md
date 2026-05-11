---
layout: default
---

# Troubleshooting

Tray icon turned red? Workbook stopped updating? Start here.

## Tray icon is red

The most common cause is an OAuth token failure (`invalid_grant`). Under the Testing-mode consent screen the refresh token silently expires after 7 days, but SquireBot ships with a Production consent screen so this should be rare.

1. Right-click the tray icon and select `Reauthorize…`. Your browser opens; complete the Google sign-in flow.
2. If the tray stays red after reauthorizing, open `%LOCALAPPDATA%\SquireBot\squirebot.log` and read the most recent `ERROR` line.
3. If the error mentions `invalid_grant` after a Reauthorize, your Google account may have revoked the app at [https://myaccount.google.com/permissions](https://myaccount.google.com/permissions) — restore the grant and try again.

## Workbook isn't updating

1. Check the tray color first. Red is auth; orange is a Sheets API problem; green means the watcher thinks it is fine.
2. If green, check `%LOCALAPPDATA%\SquireBot\squirebot.log` for `WATCH-09 catch-up` lines — the watcher rescans on restart.
3. If the watcher is fine, the workbook's Apps Script side may need a manual trigger run. Open the workbook and select `SquireBot → Install Triggers` from the menu.

## SmartScreen warning won't go away

Windows builds reputation per-binary-hash. Every SquireBot update re-triggers SmartScreen until the new binary's reputation accrues. There is no shortcut around this: follow the same `More info → Run anyway` flow each time. (EV code-signing certificates lost the instant-reputation perk in March 2024, so signing would not help.) SignPath OSS approval is the only path that could change this; status lives at [docs/signpath-application.md](https://github.com/boejowen/SquireBot/blob/main/docs/signpath-application.md).

## "ErrSchemaTooNew" in the log

The workbook is at a higher `_meta.schema_version` than this watcher binary supports. The watcher refuses to write rather than corrupt data.

1. Right-click the tray icon and select `Check for updates`.
2. If no update is offered, download the latest installer manually from [GitHub Releases](https://github.com/boejowen/SquireBot/releases/latest) and run it.
3. After the new binary launches, the schema check passes and uploads resume.

## drive.file permission propagation (~50 minute delay)

After a fresh `Reauthorize…` plus a Google Drive Picker pick, write access to the workbook can take up to about 50 minutes to propagate on Google's side. The read side returns immediately; only `batchUpdate` writes return `401` during the window. SquireBot's startup probe goroutine waits up to 90 minutes for this and surfaces a yellow tray during the wait.

1. Wait. The tray flips back to green when the probe succeeds.
2. If the wait exceeds 90 minutes (the cap), file an issue at [github.com/boejowen/SquireBot/issues](https://github.com/boejowen/SquireBot/issues) — that is past the worst observed propagation delay.

## Hidden tabs (`_meta`, `_char_owner`, etc.) reappeared

Any editor of the workbook can right-click a tab and choose `Unhide sheet`. The underscore-prefixed system tabs are hidden by default. To re-hide them, open the workbook and select `SquireBot → Install Triggers`. The `hideAllSystemTabs` step is idempotent — it always re-hides every underscore-prefixed tab without other side effects.

## I need to remove a guildie

See the [Eviction runbook](https://github.com/boejowen/SquireBot/blob/main/docs/eviction-runbook.md). Short version: `SquireBot → Evict Guildie…`, select the email, confirm. A 30-day grace period applies before the chars are auto-archived; un-evict is a manual `_char_owner.is_removed = FALSE` cell edit before grace expires.
