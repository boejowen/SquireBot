---
layout: default
---

# Install SquireBot

Four steps, about three minutes. **No Google sign-in and no browser** — just a one-time guild code your maintainer gives you.

## 1. Download

Grab the latest installer from the [GitHub Releases page](https://github.com/boejowen/SquireBot/releases/latest), or download it directly:

[**SquireBot-Setup.exe**](https://github.com/boejowen/SquireBot/releases/latest/download/SquireBot-Setup.exe)

SquireBot installs per-user, so you will **not** see a User Account Control (admin) prompt.

## 2. Run the installer

The binary is unsigned, so Windows SmartScreen will warn you on first run. This is expected — code-signing certificates no longer grant instant SmartScreen reputation (Microsoft removed the EV reputation perk in March 2024). The warning dialog shows a single blue `Don't run` button and a small `More info` link near the top. Click `More info`, then click `Run anyway`. The installer finishes in about five seconds and SquireBot starts in your system tray.

## 3. Paste your guild code

On first launch, SquireBot opens a small **"Paste your guild code"** dialog — a plain text box, no browser and no Google. Paste the one-time code your guild maintainer DM'd you and click OK. SquireBot checks it with the server, stores it securely on your PC (Windows DPAPI), and never asks again.

> Don't have a code? Ask your maintainer — each guildie gets their own unique code.

## 4. Pick your EQ folder, then sync

Next, SquireBot asks for your EverQuest install folder (for example, `C:\P99\EverQuest\`). It tries to auto-detect this; if it can't, pick it manually.

Then, in EverQuest, type `/outputfile inventory`. EQ writes `<YourChar>-Inventory.txt` to that folder; SquireBot detects the new file (fsnotify, 500 ms debounce), parses the columns (`Location, Name, ID, Count, Slots`), and uploads the rows to the guild backend within about 30 seconds. Casters can then run `/outputfile spellbook` to upload their spellbook the same way.

View everything — your inventory, gear and spell checklists, and the shared guild bank — at **[squirebot.quest](https://squirebot.quest)** (sign in with Discord).

---

**Upgrading from an older SquireBot?** Just install over the top. On its first launch the new version automatically clears the old Google login and asks for your guild code instead; your EverQuest folder setting is kept. (If your old watcher never auto-updated, downloading and running this installer is the fix.)

If you hit a snag, see [Troubleshooting]({{ "/troubleshooting/" | relative_url }}).
