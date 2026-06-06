# SquireBot watcher — Linux

A small background daemon that watches your Project 1999 EQ folder for the
tab-separated `.txt` files produced by `/outputfile inventory` and
`/outputfile spellbook` and uploads them to the SquireBot backend. This is the
**headless Linux build** for guildies running P99 under WINE / Lutris / Proton /
Bottles — no tray icon, no browser sign-in, just a CLI + a systemd user service.

## Install

Extract the tarball and run the installer:

```sh
tar -xzf squirebot-linux-amd64.tar.gz
cd squirebot-linux-amd64
./install.sh
```

`install.sh` (run as your normal user — **not** root):

- installs the `squirebot` binary to `~/.local/bin/`,
- installs the systemd **user** unit to `~/.config/systemd/user/squirebot.service`,
- runs first-time setup (`squirebot --setup`) if you haven't configured it yet,
- enables + starts the service so it autostarts on login.

### Headless / SSH-only boxes

By default the service starts when you log in to your desktop (the moment you'd
be playing P99). If you run on a **headless or SSH-only box** that must keep the
watcher running without a graphical login, install with lingering enabled:

```sh
./install.sh --linger
```

This runs `loginctl enable-linger "$USER"`, so the user service starts at boot
and survives logout. Trade-off: the watcher then runs 24/7 even when nobody's
playing (uploads of unchanged files are cheap no-ops). Default OFF; opt in only
if you need it.

## First-time setup

`squirebot --setup` prompts on the terminal for:

1. your **guild code** (the reusable bearer token your guild gave you), and
2. your **EQ folder** — it auto-scans common WINE prefixes (`$WINEPREFIX`,
   `~/.wine`, Lutris, Bottles, Steam/Proton `compatdata`) first and offers what
   it finds; if it can't locate your install, just type the path (it expands a
   leading `~` and `$VAR`).

## Usage

```sh
squirebot --status                      # health, config path, EQ folder(s) — never prints your code
systemctl --user status squirebot       # service state
journalctl --user -u squirebot -f       # follow the logs
systemctl --user restart squirebot      # restart
systemctl --user disable --now squirebot # stop + disable autostart
```

## Where things live (XDG)

| What            | Path                                  |
| --------------- | ------------------------------------- |
| Config          | `~/.config/squirebot/config.json`     |
| Guild code      | `~/.config/squirebot/guild_code` (mode `0600`) |
| Logs            | `~/.local/state/squirebot/`           |
| Binary          | `~/.local/bin/squirebot`              |
| Autostart unit  | `~/.config/systemd/user/squirebot.service` |

## Auto-update

Auto-update is **automatic — no action needed**. The watcher checks for a new
release daily, downloads + SHA-256-verifies the bare Linux binary, and stages
it; on the next launch it swaps the binary in place and `Restart=always`
relaunches the new version. The Linux watcher selects the Linux asset (never the
Windows `.exe`) from the release manifest.

## WINE note

P99 under WINE keeps the EQ folder (with `eqgame.exe` + `eqclient.ini`) inside a
WINE prefix's `drive_c`. The watcher reads those files directly from your home
directory — there is nothing WINE-specific to configure beyond pointing
`--setup` at the right EQ folder. If you move your prefix, re-run `squirebot
--setup` to update the path.
