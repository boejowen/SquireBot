---
layout: default
---

# Install SquireBot

Five steps. Takes about three minutes.

## 1. Download

Grab the latest installer from the [GitHub Releases page](https://github.com/boejowen/SquireBot/releases/latest). The file is named `SquireBot-Setup-v0.4.0.exe` (or whatever the current latest tag is) and weighs about 12 MB. SquireBot installs per-user, so you will not see a User Account Control prompt.

![SquireBot installer dialog on first launch](assets/01-installer.png)

## 2. Run the installer

The binary is unsigned, so Windows SmartScreen will warn you on first run. The warning is expected — code-signing certificates no longer grant instant SmartScreen reputation (Microsoft removed the EV reputation perk in March 2024). Click `More info` to expand the panel, then click `Run anyway`.

![Windows SmartScreen showing the More info link](assets/02-smartscreen-more-info.png)

![Windows SmartScreen expanded showing the Run anyway button](assets/03-smartscreen-run-anyway.png)

![Animated walkthrough of the Windows SmartScreen prompt — click "More info", then "Run anyway".](assets/smartscreen.gif)

The installer itself completes in about five seconds.

## 3. Authorize Google

SquireBot opens your default browser once to ask for Google authorization. The OAuth scope is `drive.file` — Google describes it as `See, edit, create, and delete only the specific Google Drive files you use with this app`. The consent screen is in Production state, so your refresh token will not silently expire.

![Google OAuth consent screen showing the drive.file scope](assets/04-oauth-consent.png)

## 4. Pick the workbook and EQ folder

SquireBot opens two more browser tabs in sequence. The first is the Google Drive Picker — select your guild's shared workbook. The second is a local folder picker for your EverQuest install root (for example, `C:\P99\EverQuest\`). SquireBot tries to auto-detect the EQ folder via known paths and a heuristic scan; if it cannot, you will pick manually.

![EQ folder picker dialog](assets/05-folder-picker.png)

## 5. Trigger your first sync

In EverQuest, type `/outputfile inventory`. EQ writes `<YourChar>-Inventory.txt` to the configured folder. SquireBot detects the new file (fsnotify, 500 ms debounce), parses the five columns (`Location, Name, ID, Count, Slots`), and writes the rows to a new `inv:<YourChar>` tab in the workbook within about 30 seconds. Casters can then run `/outputfile spellbook` to populate `spell:<YourChar>` the same way.

---

If you hit a snag, see [Troubleshooting]({{ "/troubleshooting/" | relative_url }}).
