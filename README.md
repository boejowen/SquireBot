# SquireBot

A small app that streams your EverQuest (Project 1999) inventory and spellbook to your guild's shared website at [squirebot.quest](https://squirebot.quest). It runs on **Windows** (a quiet system-tray app) and **Linux** (a headless daemon, for guildies who play P99 under WINE / Lutris / Proton / Bottles).

No Google sign-in and no browser on the watcher — just a one-time guild code your maintainer gives you.

## Install

- **Windows** — https://boejowen.github.io/SquireBot/install/ (download `SquireBot-Setup.exe`, per-user, no admin prompt).
- **Linux** — download `squirebot-linux-amd64.tar.gz` from the [latest release](https://github.com/boejowen/SquireBot/releases/latest), extract, and run `./install.sh`. Full instructions: [`packaging/linux/README.md`](packaging/linux/README.md). It's a static, CGO-free binary that runs as a **systemd user service** and finds your EQ folder inside the WINE prefix automatically.

Both builds auto-update from GitHub Releases, so this is normally a one-time install.

**GitHub:** https://github.com/boejowen/SquireBot
**Developer notes:** https://boejowen.github.io/SquireBot/dev/

---

For full project context, see `CLAUDE.md`.
