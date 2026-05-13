---
layout: default
title: Privacy Policy
---

# SquireBot Privacy Policy

**Effective:** 2026-05-13
**Last updated:** 2026-05-13

SquireBot is an open-source desktop Windows application that streams a single Project 1999 guild's EverQuest character data into a shared Google Sheet that guild has authorized. SquireBot has no server, collects no telemetry, and sends data to no third party other than Google Sheets (the destination spreadsheet) and GitHub (for app distribution and auto-update). This page explains exactly what data SquireBot touches, where it goes, and how to remove it.

If you are an end user (a guildie running SquireBot) and have questions, file an issue at [github.com/boejowen/SquireBot/issues](https://github.com/boejowen/SquireBot/issues) or contact your guild's spreadsheet owner.

---

## What SquireBot reads from your computer

SquireBot watches one folder you choose during the install wizard (typically your EverQuest game folder) for files produced by the EQ client's `/outputfile` command:

| File pattern | Contents read |
|---|---|
| `<CharName>-Inventory.txt` | Tab-separated inventory rows: `Location, Item Name, Item ID, Count, Bag Slots` |
| `<CharName>-Spellbook.txt` | Tab-separated spellbook rows: `Level, Spell Name` |

SquireBot does **not** read any file outside this folder. It does not scan your disk, read your email, access your browser data, or touch any other application's files.

## What SquireBot writes to Google

The contents of the files above are written to the **single Google Sheet** you authorized during install, using the Google Sheets API. Each row is tagged with the originating character's name and your Google account email (so the guild can tell whose data each row represents — see "Identity" below).

The OAuth scope SquireBot requests is **`https://www.googleapis.com/auth/drive.file`**. This is Google's most restrictive Drive scope: it grants SquireBot access only to files it created itself or that you explicitly opened with SquireBot. SquireBot **cannot** read any other file in your Google Drive, cannot list your other files, and cannot see your other Sheets, Docs, or folders. Your authorized guild sheet is the only file SquireBot can touch.

## Identity

When you sign in, SquireBot reads your Google account's primary email address (via the standard OAuth `userinfo.email` scope) and records it on the data rows you upload, so the guild's shared sheet can attribute rows correctly across multiple guildies. SquireBot does not read your name, profile photo, calendar, contacts, gender, age, or any other Google profile field.

## What SquireBot stores locally on your computer

| Location | Contents | Lifetime |
|---|---|---|
| Windows Credential Manager (DPAPI-encrypted, accessible only to your Windows user account) | OAuth refresh token | Until you uninstall SquireBot or revoke its access in your [Google account permissions](https://myaccount.google.com/permissions) |
| `%LOCALAPPDATA%\SquireBot\config.json` | The EQ folder path, your guild sheet ID, your Google email | Until you uninstall SquireBot |
| `%LOCALAPPDATA%\SquireBot\squirebot.log` (and rotated copies) | Operational events: file change notifications, upload counts, errors. Does **not** include the contents of your inventory or spellbook files. | Capped by rotation (most recent few MB of activity) |

None of this local data is transmitted off your computer.

## What SquireBot does **not** do

- No analytics, no telemetry, no crash reporting to any third party
- No advertising, no tracking pixels, no remote profiling
- No background network activity besides Google Sheets uploads (when your tracked files change) and a once-per-day check at `https://github.com/boejowen/SquireBot/releases/latest/download/latest.json` for app updates
- No data sale, no data sharing, no aggregation for third-party use
- No interaction with any Google product other than the single Sheet you authorized and the Google OAuth/userinfo endpoints required to authenticate to it

## Where your data ultimately lives

Your character data lives inside the Google Sheet that your guild's spreadsheet owner created and that you chose during setup. That sheet is owned and controlled by the spreadsheet owner under their own Google account, not by SquireBot. To request deletion of your rows from that sheet, contact the spreadsheet owner directly. SquireBot has no copy outside that sheet and cannot delete on your behalf.

## How to revoke SquireBot's access

- **Stop uploading immediately:** Right-click the SquireBot tray icon → Quit.
- **Revoke the OAuth grant from Google's side:** Visit [myaccount.google.com/permissions](https://myaccount.google.com/permissions), find "SquireBot", and click Remove access. After this, SquireBot's refresh token will be rejected by Google and the watcher will surface a Reauthorize prompt rather than continue uploading.
- **Wipe local state and uninstall:** Use *Add or remove programs* in Windows → SquireBot → Uninstall. The uninstaller clears `%LOCALAPPDATA%\SquireBot\` and removes the Windows Credential Manager entry holding the refresh token.

## Open source and auditability

SquireBot's full source code is at [github.com/boejowen/SquireBot](https://github.com/boejowen/SquireBot) under an open-source license. Every claim on this page is verifiable in the source. Notable file references:

- OAuth flow and token storage: `internal/auth/`
- File watching and read paths: `internal/watch/`, `internal/parse/`
- Google Sheets write path: `internal/sheet/`
- Log output configuration: `internal/logging/`

If you find a discrepancy between this policy and the actual behavior of the application, please open an issue — that is a bug we will fix immediately.

## Children's privacy

SquireBot is a desktop tool for adult-hobbyist EverQuest guilds. It is not directed at children under 13 and does not knowingly collect data from them. If you are a parent or guardian and believe a child has installed SquireBot, ask the guild's spreadsheet owner to remove their rows from the shared sheet, then uninstall the application following the steps above.

## Changes to this policy

If this policy changes, the new version will replace this page and the "Last updated" date at the top will change. Material changes will be announced in the [GitHub Releases notes](https://github.com/boejowen/SquireBot/releases). If a future version of SquireBot collects new categories of data or transmits to new destinations, that will be disclosed here before the relevant release ships.

## Contact

Open an issue at [github.com/boejowen/SquireBot/issues](https://github.com/boejowen/SquireBot/issues) for privacy questions or data requests SquireBot itself can answer. For data already written into your guild's shared sheet, contact your guild's spreadsheet owner — only they can edit or delete rows in that sheet.
