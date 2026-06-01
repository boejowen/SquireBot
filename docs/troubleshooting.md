---
layout: default
---

# Troubleshooting

Tray icon red? Data not showing up on the site? Start here.

## Tray icon is red

A red tray icon means SquireBot can't upload to the guild backend. The usual causes:

1. **Guild-code problem.** If your code was never entered, was mistyped, or was revoked/re-issued, the server rejects uploads. Ask your maintainer to confirm (or re-issue) your code, then re-run the [installer](https://github.com/boejowen/SquireBot/releases/latest/download/SquireBot-Setup.exe) and paste the code again.
2. **Network or server unreachable.** Confirm you have internet and that `https://api.squirebot.quest` loads in a browser. Corporate VPNs or DNS filtering can block it.
3. Open `%LOCALAPPDATA%\SquireBot\squirebot.log` and read the most recent `ERROR` line — it names the exact cause (for example a `401` rejected code or a network error).

## My data isn't showing at squirebot.quest

1. Make sure you're **signed in with Discord** at [squirebot.quest](https://squirebot.quest).
2. Check the tray color — green means SquireBot thinks its uploads are succeeding.
3. In EverQuest, run `/outputfile inventory` (and `/outputfile spellbook` for casters). Uploads land within about 30 seconds.
4. If the tray is green but nothing appears, check `%LOCALAPPDATA%\SquireBot\squirebot.log` for the most recent upload line.

## My watcher is on an old version / won't update

SquireBot auto-updates itself from GitHub, but a watcher installed from an old **pre-release** build can get stuck and never update (it misjudges newer versions). The fix is always the same — download and run the latest installer manually; it replaces the running copy in place:

[**SquireBot-Setup.exe** (latest)](https://github.com/boejowen/SquireBot/releases/latest/download/SquireBot-Setup.exe)

On its next launch it asks for your guild code (your EverQuest folder setting is kept).

## SmartScreen warning won't go away

Windows builds reputation per-binary-hash, so every SquireBot update re-triggers SmartScreen until the new binary's reputation accrues. There is no shortcut: follow the same `More info → Run anyway` flow each time. (EV code-signing certificates lost the instant-reputation perk in March 2024, so signing would not help.) SignPath OSS approval is the only path that could change this; status lives at [docs/signpath-application.md](https://github.com/boejowen/SquireBot/blob/main/docs/signpath-application.md).

## I need to remove a guildie (officers)

Eviction now lives on the website, not in a spreadsheet. Sign in at [squirebot.quest](https://squirebot.quest) as an officer and use the **Admin** page to evict a guildie. A 30-day grace period applies before their characters are archived, and you can restore them on the same page within that window.

---

Still stuck? File an issue at [github.com/boejowen/SquireBot/issues](https://github.com/boejowen/SquireBot/issues).
