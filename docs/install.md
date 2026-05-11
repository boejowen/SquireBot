---
layout: default
---

# Install SquireBot

Five steps. Takes about three minutes.

## 1. Download

Grab the latest installer from the [GitHub Releases page](https://github.com/boejowen/SquireBot/releases/latest). The file is named `SquireBot-Setup-v0.4.0.exe` (or whatever the current latest tag is) and weighs about 12 MB. SquireBot installs per-user, so you will not see a User Account Control prompt.

## 2. Run the installer

The binary is unsigned, so Windows SmartScreen will warn you on first run. This is expected — code-signing certificates no longer grant instant SmartScreen reputation (Microsoft removed the EV reputation perk in March 2024). The warning dialog has a single blue button (`Don't run`) and a small `More info` link near the top. Click `More info`, and the dialog expands to reveal a `Run anyway` button — click that. The installer itself completes in about five seconds.

## 3. Authorize Google

SquireBot opens your default browser once to ask for Google authorization. The OAuth scope is `drive.file` — Google describes it as `See, edit, create, and delete only the specific Google Drive files you use with this app`. The consent screen is in Production state, so your refresh token will not silently expire.

## 4. Pick the workbook and EQ folder

SquireBot opens two more browser tabs in sequence. The first is the Google Drive Picker — select your guild's shared workbook. The second is a local folder picker for your EverQuest install root (for example, `C:\P99\EverQuest\`). SquireBot tries to auto-detect the EQ folder via known paths and a heuristic scan; if it cannot, you will pick manually.

## 5. Trigger your first sync

In EverQuest, type `/outputfile inventory`. EQ writes `<YourChar>-Inventory.txt` to the configured folder. SquireBot detects the new file (fsnotify, 500 ms debounce), parses the five columns (`Location, Name, ID, Count, Slots`), and writes the rows to a new `inv:<YourChar>` tab in the workbook within about 30 seconds. Casters can then run `/outputfile spellbook` to populate `spell:<YourChar>` the same way.

---

If you hit a snag, see [Troubleshooting]({{ "/troubleshooting/" | relative_url }}).
