---
layout: default
title: Privacy Policy
---

# SquireBot Privacy Policy

**Effective:** 2026-05-31
**Last updated:** 2026-05-31

SquireBot is an open-source desktop Windows application that streams a single Project 1999 guild's EverQuest character data to a small web service **your own guild runs** (self-hosted — the v2.0 release removed Google entirely). SquireBot collects no telemetry and sends your data to no outside company: only to your guild's own backend, its off-site backup, and GitHub (for app distribution and auto-update). This page explains exactly what data SquireBot touches, where it goes, and how to remove it.

If you are a guildie running SquireBot and have questions, file an issue at [github.com/boejowen/SquireBot/issues](https://github.com/boejowen/SquireBot/issues) or contact your guild's maintainer (the person who runs the server and gave you your guild code).

---

## What SquireBot reads from your computer

SquireBot watches one folder you choose during setup (typically your EverQuest game folder) for files produced by the EQ client's `/outputfile` command:

| File pattern | Contents read |
|---|---|
| `<CharName>-Inventory.txt` | Tab-separated inventory rows: `Location, Item Name, Item ID, Count, Bag Slots` |
| `<CharName>-Spellbook.txt` | Tab-separated spellbook rows: `Level, Spell Name` |

SquireBot does **not** read any file outside this folder. It does not scan your disk, read your email, access your browser data, or touch any other application's files.

## What SquireBot uploads and where

The contents of those files are uploaded to **your guild's own SquireBot backend** at `https://api.squirebot.quest`, over HTTPS. That backend is a small open-source server your guild's maintainer runs on a virtual private server they control — not a Google, Microsoft, or other big-tech service. Each upload is tagged with the originating character's name so the guild views can attribute rows correctly.

There is **no Google involvement of any kind** — no Google sign-in, no Google Drive, no Google Sheets, no Google scopes. (Earlier versions wrote to a Google Sheet; v2.0 removed it completely.)

## Identity

- **The watcher** authenticates to the backend with a **guild code** — a random token your maintainer issues to you and you paste in once. The backend stores only a one-way hash of it, never the code itself. The watcher does **not** send your name, email, or any personal account; your uploads are attributed only to the owner label your maintainer assigned to your code.
- **The website** (`squirebot.quest`), where you view the data, asks you to **sign in with Discord**. That tells the site your Discord account so it can show the right pages and let officers manage the guild. Discord login is used by the website only — the watcher never touches it.

## What SquireBot stores locally on your computer

| Location | Contents | Lifetime |
|---|---|---|
| Windows Credential Manager (DPAPI-encrypted, accessible only to your Windows user account) | Your guild code | Until you uninstall SquireBot |
| `%LOCALAPPDATA%\SquireBot\config.json` | The EQ folder path (and the backend URL) | Until you uninstall SquireBot |
| `%LOCALAPPDATA%\SquireBot\squirebot.log` (and rotated copies) | Operational events: file-change notices, upload counts, errors. Does **not** include the contents of your inventory or spellbook files. | Capped by rotation (the most recent few MB of activity) |

None of this local data is transmitted off your computer except the uploads described above.

## What SquireBot does **not** do

- No analytics, no telemetry, no crash reporting to any third party
- No advertising, no tracking pixels, no remote profiling
- No background network activity besides uploads to your guild's backend (when your tracked files change) and a once-per-day check at `https://github.com/boejowen/SquireBot/releases/latest/download/latest.json` for app updates
- No data sale, no data sharing, no aggregation for third-party use
- No Google, Microsoft, or other big-tech account access

## Where your data ultimately lives

Your character data lives in your guild's own backend database, run by your guild's maintainer on a server they control. For resilience, that database is backed up nightly to a private, access-controlled **Cloudflare R2** bucket (off-site storage); your data is included in those backups. No other copy exists outside your guild's backend and its backups.

To correct or delete your data, contact your guild's maintainer — they control the backend and can remove or archive your characters.

## How to stop, revoke, or delete

- **Stop uploading immediately:** right-click the SquireBot tray icon → Quit.
- **Revoke your access server-side:** ask your maintainer to revoke your guild code. Once revoked, the backend rejects your watcher's uploads even if it keeps running.
- **Wipe local state and uninstall:** use *Add or remove programs* in Windows → SquireBot → Uninstall. The uninstaller clears `%LOCALAPPDATA%\SquireBot\` and removes the Credential Manager entry holding your guild code.
- **Remove data already uploaded:** contact your maintainer — only they can edit or delete rows on the guild's backend.

## Open source and auditability

SquireBot's full source — both the desktop watcher and the guild backend — is at [github.com/boejowen/SquireBot](https://github.com/boejowen/SquireBot) under an open-source license. Every claim on this page is verifiable in the source. Notable references:

- Guild-code onboarding (native dialog, no browser): `internal/onboarding/`
- Watcher upload over HTTPS to `POST /api/v1/ingest`: `cmd/squirebot/` and the backend client under `internal/`
- The guild backend (storage, auth, website): `cmd/squirebot-server/`, `internal/backendsrv/`

If you find a discrepancy between this policy and the application's actual behavior, please open an issue — that is a bug we will fix immediately.

## Children's privacy

SquireBot is a desktop tool for adult-hobbyist EverQuest guilds. It is not directed at children under 13 and does not knowingly collect data from them. If you are a parent or guardian and believe a child has installed SquireBot, ask the guild's maintainer to remove their data from the backend, then uninstall the application following the steps above.

## Changes to this policy

If this policy changes, the new version replaces this page and the "Last updated" date at the top will change. Material changes will be announced in the [GitHub Releases notes](https://github.com/boejowen/SquireBot/releases). If a future version of SquireBot collects new categories of data or transmits to new destinations, that will be disclosed here before the relevant release ships.

## Contact

Open an issue at [github.com/boejowen/SquireBot/issues](https://github.com/boejowen/SquireBot/issues) for privacy questions, or contact your guild's maintainer for data already on the guild backend — only they can edit or delete it.
