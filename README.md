# SquireBot

> Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

A small Windows app that every member of a ~12-person Project 1999 (Classic EverQuest emulator) guild installs on their PC. SquireBot watches the EverQuest folder for tab-separated text files produced by the in-game `/outputfile inventory` and `/outputfile spellbook` commands and pushes their contents into a single shared Google Sheet. The sheet (Apps Script + TypeScript, landing in Phase 3) joins each guildie's data with the [P1999 wiki](https://wiki.project1999.com/) and the [PigParse REST API](https://pigparse.azurewebsites.net) to produce per-character inventory views, gear/spell progression checklists vs. Velious tiers, a shared bank with cross-character search, and item tooltips.

See [.planning/PROJECT.md](.planning/PROJECT.md) for the full project context.

## Install

**👉 [Download the latest SquireBot installer](https://github.com/boejowen/SquireBot/releases/latest)**
(this link always points at the newest version)

On the release page that opens, click **`SquireBot-Setup-<version>.exe`** under "Assets" to download.

### Setup steps

1. **Run the installer.** Windows will show a **"Windows protected your PC"** warning — that's normal for unsigned apps. Click **More info → Run anyway**. SquireBot installs without admin rights, so you won't see a "do you want to allow this app to make changes" prompt. Stuck on the warning? See our [30-second walkthrough](docs/smartscreen-walkthrough.md) for screenshots and browser-specific tips.

2. **Finish the setup wizard** that opens after install:
   - **Sign into Google** in the browser tab that appears.
   - **Allow Google Drive access when prompted** — there will be a Drive permission to approve (sometimes shown as a checkbox to tick). SquireBot can't write to the guild sheet without it.
   - **Pick the guild workbook** from the picker. If you don't see it listed, ask your guild lead to share the workbook with your Gmail address first.
   - **Confirm the EverQuest folder** — SquireBot tries to find it automatically; just verify the path it shows is correct for your install.

3. **You're done.** SquireBot now lives in your system tray. To test it, run `/outputfile inventory` in EQ — the guild sheet should update within a few seconds.

For local building from source, see [docs/build-and-install.md](docs/build-and-install.md). For the OAuth setup runbook (Cloud Console steps for forks), see [docs/oauth-setup.md](docs/oauth-setup.md).

## Tray menu

Right-click the tray icon for these options:

| Item                     | Purpose                                                                          |
| ------------------------ | -------------------------------------------------------------------------------- |
| Status (top, disabled)   | Current state, e.g. "Last upload: Foo at 14:32"                                  |
| Open Workbook            | Opens the configured Google Sheet in your browser                                |
| Open log folder          | Opens `%LOCALAPPDATA%\SquireBot\` in Explorer (where `squirebot.log*` lives)     |
| Check for updates        | Manually triggers an update check (auto-checks every 24h via `latest.json`)     |
| Change Workbook…         | Re-runs the Drive Picker to switch to a different workbook                       |
| Continue setup…          | (hidden unless setup is incomplete)                                              |
| Reauthorize…             | (hidden unless your Google refresh token died — see "tray turned red" below)    |
| Quit                     | Exit SquireBot                                                                   |

## Tray turned red — what now?

A red tray icon means one of three things:

- **Setup needed** — wizard didn't finish (you closed the browser before granting consent, or skipped the workbook picker). Click **Continue setup…** in the tray menu to resume from where you left off.
- **Refresh token died** — Google's refresh token has expired or been revoked (e.g., you changed your Google password, hit the Testing-mode 7-day expiry on a non-Production OAuth client, or manually revoked SquireBot at <https://myaccount.google.com/permissions>). Click **Reauthorize…** to redo OAuth. The watcher resumes immediately on success — see Plan 02-04 for the implementation details.
- **Watcher error** — sheets API rejected a write, the workbook was deleted, or the schema doesn't match. See `%LOCALAPPDATA%\SquireBot\squirebot.log` for the structured error. Common causes: workbook deleted (re-pick via Change Workbook…), sheet schema mismatch (run a fresh install), persistent Sheets API failures (the watcher backs off with `2/4/8/16/32/60s` per WATCH-07 before surfacing).

Most red-tray states are recoverable in under 30 seconds via the tray menu. If you're stuck, share the relevant lines from the log file with your guild leader.

## Known issues

- **After Reauthorize, uploads pause for up to ~50 minutes.** When you click **Reauthorize…** to recover from a dead refresh token, the OAuth handshake itself completes in seconds — but Google's Drive backend takes time to propagate write access for the workbook under the new permission grant. During this window the tray stays **green** with status **"Reauthorized: waiting for Google propagation…"** and the watcher waits in the background. You don't need to do anything; the next `/outputfile` after the wait completes uploads normally. If the wait exceeds 90 minutes, the tray will eventually go red and ask you to Reauthorize again — that's a fallback that should never trigger in practice (worst observed wait so far is 51 minutes). See [docs/soak-reports/2026-05-07-day4-auth05-sc1.md](docs/soak-reports/2026-05-07-day4-auth05-sc1.md) for the full investigation.

## Auto-update

SquireBot auto-updates daily via a `latest.json` manifest fetched from GitHub Releases. SHA-256 verification of the new binary happens before any swap, and the swap itself is **startup-only** (Windows file-locking forbids in-process replacement of a running `.exe`). The new binary lands at `<exepath>.new`; on next launch the swap completes before the main goroutine starts. Manual trigger: tray's **Check for updates**.

## Uninstalling

See [docs/build-and-install.md "Uninstalling"](docs/build-and-install.md#uninstalling) for the full uninstall flow including the "Also delete saved configuration and Google account credentials?" prompt (default = preserve, so re-installing later resumes without re-OAuth).

## Where things live on a guildie's PC

| Path                                                                          | Purpose                                                                              |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `%LOCALAPPDATA%\Programs\SquireBot\squirebot.exe`                             | The watcher binary                                                                   |
| `%LOCALAPPDATA%\SquireBot\squirebot.log`                                      | Rotated log (5 MB × 3 backups, 28-day cap — OPS-03)                                  |
| `%LOCALAPPDATA%\SquireBot\config.json`                                        | Non-secret settings (EQ folder, spreadsheet ID, cached email)                        |
| Windows Credential Manager, target `SquireBot:<google-email>`                 | OAuth refresh token (DPAPI-protected via wincred — AUTH-04)                          |
| `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\SquireBot`                | Autostart-on-logon entry (INST-04, no Task Scheduler / Service)                      |

> **Refresh tokens NEVER live in `config.json`.** They are stored in Windows Credential Manager only — see AUTH-04 in [.planning/REQUIREMENTS.md](.planning/REQUIREMENTS.md) and the security comment on the `Config` struct in `internal/config/config.go`.

## Project status

Phase: **2 of 5** (Watcher Robustness + Schema Lock — in progress). Phase 1 shipped 2026-05-02 (tagged `phase1-complete`). See [.planning/ROADMAP.md](.planning/ROADMAP.md) for the full phased plan.

## Code signing

SquireBot ships **unsigned**. Code-signing certificates no longer grant instant SmartScreen reputation as of March 2024 (Microsoft removed EV's reputation perk), and a 12-user audience would never accumulate enough downloads to clear the reputation curve regardless of cert type. We've applied for free SignPath Foundation OSS code signing in parallel — see [docs/signpath-application.md](docs/signpath-application.md) for status. The unsigned + walkthrough path will remain the default until/unless SignPath approves.

## Forking / changing the module owner

The default module path is `github.com/boejowen/SquireBot`. If you fork to your own account:

```bash
go mod edit -module github.com/<your-owner>/squirebot
# update the matching imports in cmd/squirebot/main.go and any new packages
```

You'll also need to provision your own OAuth client + Picker API key — see [docs/oauth-setup.md](docs/oauth-setup.md).

## Repository layout

```
cmd/squirebot/         entry point (main.go, icon.go)
internal/auth/         OAuth + wincred token storage
internal/config/       JSON config persistence (no OAuth secrets)
internal/logging/      slog + lumberjack rotator (OPS-03)
internal/parse/        inventory / spellbook tab-separated parsers
internal/sheet/        Sheets API client + atomic batchUpdate writer
internal/tray/         system tray + wizard
internal/watch/        fsnotify-driven file watcher
installer/             NSIS installer source (squirebot.nsi)
assets/                embedded resources (icon)
docs/                  build / install / oauth / smartscreen / signpath docs
.github/workflows/     GitHub Actions release pipeline
.planning/             phase plans, research, state (gitignored)
```

## License

See [LICENSE](LICENSE).
