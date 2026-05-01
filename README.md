# SquireBot

A small Windows app that every member of a ~12-person Project 1999 (Classic
EverQuest emulator) guild installs on their PC. It watches the EQ folder for
the tab-separated text files produced by `/outputfile inventory` and
`/outputfile spellbook` and pushes their contents into a single shared Google
Sheet. The sheet is the real product — see `.planning/PROJECT.md` for the
full vision.

> **Phase 1 status:** repo skeleton only. The binary launches, writes one
> structured log line, and exits. OAuth, Drive Picker, the file watcher, the
> Sheets writer, and the tray UI all land in later Plans (03–07).

## Install (will land in Plan 08)

A signed NSIS installer will eventually be published to **GitHub Releases**.
Until then, build from source (see below).

## Build from source

Prereqs: **Go 1.24+** on Windows, macOS, or Linux (the watcher cross-compiles).

```bash
GOOS=windows GOARCH=amd64 \
  go build -ldflags="-H=windowsgui -s -w" \
  -o dist/squirebot.exe ./cmd/squirebot
```

`-H=windowsgui` suppresses the console window so a tray-only app does not
flash a black box on startup; `-s -w` strips debug symbols (~30% smaller).

For a stamped release build:

```bash
go build -ldflags="-H=windowsgui -s -w -X main.Version=v0.1.0" \
  -o dist/squirebot.exe ./cmd/squirebot
```

## Forking / changing the module owner

The default module path is `github.com/jbowen-mn/squirebot`. If you fork to
your own account, rename in one shot:

```bash
go mod edit -module github.com/<your-owner>/squirebot
# update the matching imports in cmd/squirebot/main.go and any new packages
```

## Where things live on a guildie's PC

| Path | Purpose |
| ---- | ------- |
| `%LOCALAPPDATA%\SquireBot\squirebot.log` | Rotated log (5 MB × 3 backups, 28-day cap — OPS-03) |
| `%LOCALAPPDATA%\SquireBot\config.json` | Non-secret settings (EQ folder, spreadsheet ID, cached email) |
| Windows Credential Manager, target `SquireBot:<google-email>` | OAuth refresh token (DPAPI-protected) |

> **Refresh tokens NEVER live in `config.json`.** They are stored in
> Windows Credential Manager only — see AUTH-04 in
> `.planning/REQUIREMENTS.md` and the security comment on the `Config`
> struct in `internal/config/config.go`.

## SmartScreen walkthrough (placeholder — expanded in Phase 5)

Phase 1 ships unsigned (D-13). When you double-click the `.exe` for the
first time, Windows will show a blue "Windows protected your PC" screen.
Click **More info → Run anyway**. Phase 2 adds a code-signing certificate
that suppresses this prompt.

## OAuth flow (placeholder — implemented in Plan 03)

On first launch the app will open your default browser, prompt you to sign
in to Google, and ask for permission to access **only the spreadsheets you
explicitly select with this app** (`drive.file` scope — non-sensitive).
You then pick the shared guild workbook from a Drive Picker dialog. Both
steps happen exactly once per Google account per machine.

## EQ folder picker (placeholder — implemented in Plan 04)

The watcher tries to auto-detect your EverQuest install in this order:

1. Previous SquireBot config (if any)
2. Common install paths: `C:\P99`, `C:\Project1999`, `C:\Games\Project1999`
3. Registry uninstall keys for "Project 1999" / "EverQuest"
4. Recursive scan for a folder containing both `eqgame.exe` and
   `eqclient.ini`

If all four fail, you will be asked to point at the folder yourself.

## "Tray turned red, what now?" (placeholder — Plan 07)

The tray icon glows red whenever something needs your attention (setup
incomplete, OAuth refresh failed, sheets API rejected a write, etc.).
Right-click the tray icon and pick the highlighted action item.

## Assets

`assets/icon.ico` ships as a 1118-byte 16x16 magenta placeholder. Phase 5
will replace it with real art.

## Repository layout

```
cmd/squirebot/         entry point (main.go, icon.go)
internal/logging/      slog + lumberjack rotator (OPS-03)
internal/config/       JSON config persistence (no OAuth secrets)
assets/                embedded resources (icon)
.github/workflows/     GitHub Actions (release stub for now)
.planning/             phase plans, research, state (gitignored)
```
