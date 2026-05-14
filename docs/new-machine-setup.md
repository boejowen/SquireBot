---
layout: default
---

# New-machine setup (developer workstation)

Step-by-step for bringing up a fresh Windows PC as a SquireBot **maintainer**
workstation — Cursor + Claude Code + GSD, full repo, ability to build the
watcher and deploy Apps Script.

This is **not** the guildie install path — guildies just run
`SquireBot-Setup-X.Y.Z.exe` per [docs/install.md](./install.md).

> **Before you wipe the old PC, grab these from it:**
>
> 1. The four real values from `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` on the old machine (committed copy is redacted): `oauth_client_id`, `oauth_client_secret`, `picker_api_key`, `gcp_project_number`. Save them in your password manager.
> 2. Optionally, the contents of `~/.claude/projects/C--Users-Virus-Canary-Desktop-Claude-SquireBot/memory/` — a verbatim copy is already committed under `.planning/claude-memory/`, so this is a fallback only.

---

## 0. Prerequisites — install yourself

Install these manually before touching the repo. SquireBot's build scripts
do **not** auto-install toolchains.

| Tool | Version | Notes |
|---|---|---|
| Git | any recent | `winget install Git.Git` works |
| Go | **1.24** | `go version`; needs to match `go.mod` |
| Node.js | 20 LTS or newer | for `apps-script/` clasp + vitest |
| NSIS | **3.10+** | needed for the installer; `scoop install nsis` or download from <https://nsis.sourceforge.io> |
| PowerShell | 7+ recommended | 5.1 works too |
| `jq` | any | only for the bash build one-liner; skippable |
| Cursor or VS Code | latest | optional — repo is editor-agnostic |
| Claude Code CLI | latest | `npm i -g @anthropic-ai/claude-code` (or per official docs) |
| GSD | latest | per <https://github.com/anthropics/claude-code> / your usual GSD install path |

Don't install `clasp` globally — it's a project dev-dependency in
`apps-script/package.json` and **must stay on the 2.4.x line** (clasp 3.x has
breaking changes; see `.planning/phases/03-apps-script-enrichment-foundation/03-RESEARCH.md` §6).

---

## 1. Clone the repo

```powershell
cd $HOME\Desktop      # or wherever you keep code
git clone https://github.com/boejowen/SquireBot.git
cd SquireBot
git status            # should be clean, on master
```

The clone includes:

- All source (`cmd/`, `internal/`, `apps-script/src/`, `installer/`).
- Full GSD planning history under `.planning/` (phases, research, seeds, bugs, retrospectives).
- A snapshot of the Claude auto-memory under `.planning/claude-memory/`.
- The Apps Script clasp binding (`apps-script/.clasp.json` with the right `scriptId`).
- Docs under `docs/` (build, OAuth setup, deploy, eviction runbook, troubleshooting).

---

## 2. Restore the real OAuth config

The committed `.planning/phases/01-end-to-end-thin-slice/oauth-config.json`
has placeholders. Paste in the real values you saved from the old PC:

```json
{
  "schema_version": 2,
  ...
  "oauth_client_id":     "262087828393-...apps.googleusercontent.com",
  "oauth_client_secret": "GOCSPX-...",
  "picker_api_key":      "AIzaSy...",
  "gcp_project_number":  "262087828393",
  "consent_screen_status": "PRODUCTION",
  "consent_screen_published_at": "2026-05-01T14:17:22Z"
}
```

**Do not commit this back.** Add a local-only ignore so a stray `git add`
can't push secrets:

```powershell
git update-index --skip-worktree .planning/phases/01-end-to-end-thin-slice/oauth-config.json
```

(Reverse with `--no-skip-worktree` if you ever need to update the redacted
schema in the committed copy.)

If you lost the values, regenerate from Google Cloud Console per
[docs/oauth-setup.md](./oauth-setup.md) — but note that rotating the
OAuth client secret invalidates the secret baked into already-released
binaries, so existing guildie installs would break until you cut a new
release. Prefer restoring from password-manager backup.

---

## 3. Rehydrate Claude's auto-memory

Claude Code reads per-project memory from
`~/.claude/projects/<sanitized-cwd>/memory/MEMORY.md`. On a fresh install
that directory is empty. Copy the committed snapshot into place:

```powershell
# Adjust the destination path if Cursor/Claude Code chose a different sanitized name.
# Easiest: launch Claude Code in the repo once; it creates the project dir, then quit.
$projectDir = "$HOME\.claude\projects\C--Users-$($env:USERNAME)-Desktop-SquireBot\memory"
New-Item -ItemType Directory -Force -Path $projectDir | Out-Null
Copy-Item -Force .planning\claude-memory\*.md $projectDir\
```

Then restart Claude Code. Verify by asking it "what do you remember about
this project?" — it should reference P99 file formats, the consolidated
view-tab decision, the OAuth brand-verification incident, etc.

If the sanitized project-dir name differs on your new PC, find the right
one with `Get-ChildItem $HOME\.claude\projects` after Claude Code has run
once in the repo.

---

## 4. Editor + Claude Code + GSD config

- **Cursor / VS Code:** open the repo folder. The repo doesn't ship `.vscode/` (gitignored).
- **Claude Code:** run `claude` inside the repo. The project's `CLAUDE.md` is auto-loaded and tells the model the stack/architecture/conventions.
- **GSD:** verify the slash commands work — `/gsd-progress` should print the current milestone status (read from `.planning/STATE.md`, `.planning/ROADMAP.md`, etc.). If GSD complains the dir is malformed, run `/gsd-health`.

`.claude/settings.local.json` is per-machine and intentionally not in the
repo. Recreate any permission allowlists you want via `/fewer-permission-prompts`
or by manually editing the file as Claude Code prompts you.

---

## 5. Apps Script (clasp + esbuild + vitest)

```powershell
cd apps-script
npm install            # installs clasp 2.4.x, esbuild, vitest, @types/google-apps-script
npx clasp login        # opens browser; sign in as the workbook owner (jbowen@mncivic.com)
npm run build          # bundles src/ into dist/Code.js
npm test               # runs vitest against mocked Apps Script globals
```

`clasp login` writes `~/.clasprc.json` (user-level OAuth, not committed).
The repo already has `apps-script/.clasp.json` with the bound `scriptId`,
so `clasp push` will hit the correct container-bound script as long as
the signed-in account owns or has edit access to it.

Deploy on demand:

```powershell
npm run deploy         # or: npx clasp push
```

Full runbook with safety checks: [docs/apps-script-deploy.md](./apps-script-deploy.md).

---

## 6. Build the watcher

Follow [docs/build-and-install.md](./build-and-install.md). The summary is:

```powershell
cd <repo root>
$cfg = Get-Content .planning\phases\01-end-to-end-thin-slice\oauth-config.json | ConvertFrom-Json
$ldflags = @(
  "-X main.OAuthClientID=$($cfg.oauth_client_id)",
  "-X main.OAuthClientSecret=$($cfg.oauth_client_secret)",
  "-X main.PickerAPIKey=$($cfg.picker_api_key)",
  "-X main.GCPProjectNumber=$($cfg.gcp_project_number)"
) -join ' '
go build -ldflags="$ldflags" -o squirebot.exe .\cmd\squirebot
.\squirebot.exe --version
```

The bash one-liner equivalent (with `jq`) is in `docs/build-and-install.md`.

To produce the NSIS installer:

```powershell
& "$env:ProgramFiles (x86)\NSIS\makensis.exe" /DVERSION=0.0.0-dev installer\squirebot.nsi
```

The release pipeline (`.github/workflows/release.yml`) does the same thing
in CI when you push a `vX.Y.Z` tag; the local path exists for sideload
testing on a clean VM.

---

## 7. Re-authorize the watcher

The OAuth refresh token lives in Windows DPAPI (Credential Manager) and
**does not move between PCs**. First-run wizard handles this:

1. Run `squirebot.exe` from anywhere — it auto-launches the wizard if no token is stored.
2. Pick your EQ folder (where `inventory_*.txt` / `spellbook_*.txt` land).
3. Click "Sign in with Google" — opens browser, sign in as a guildie account.
4. Pick (or create) the shared workbook with the Drive Picker.
5. Wizard hands off to the tray. Wait for "Ready" — first write will follow your next `/outputfile inventory` in-game.

If the browser tab seems stuck after "Sign in succeeded", re-check
`.planning/claude-memory/project_v021_wizard_handoff_bug.md` — that bug
was fixed in v0.2.1; the symptom should not appear on the current
release. If it does, file a bug.

If you hit `invalid_client` during sign-in: the OAuth secret restored in
Step 2 doesn't match what Google Cloud Console has. Re-copy from
password manager.

---

## 8. Smoke-verify end-to-end

1. In EQ, run `/outputfile inventory` and `/outputfile spellbook`.
2. Watch the tray icon — should briefly animate, then `_status` cell in the shared workbook updates with your latest write timestamp.
3. Open the shared workbook; your `inv:<CharName>` tab should have the new rows.
4. Trigger an Apps Script refresh (`Extensions → Apps Script → run refreshAll`, or wait for the time-driven trigger). The `view` and `gear_check` consolidated tabs should reflect your character.

If anything fails, [docs/troubleshooting.md](./troubleshooting.md) covers
the top symptoms. The `_audit` and `_status` tabs in the workbook are the
first places to look.

---

## 9. Optional housekeeping

- **Move scheduled routines off the old PC.** If you had GSD/Claude Code routines (e.g. `trig_01...` from the v1.0 milestone), they were running on the old machine's daemon and will not auto-migrate. Use `/schedule list` to see what was scheduled; re-create what's still relevant.
- **Old PC decommission:** after you confirm the new machine writes successfully to the shared sheet, uninstall the watcher from the old PC via Settings → Apps. The token in DPAPI will be orphaned but not security-critical — Google will eventually expire the refresh token.
- **Re-enable `.gitignore` hygiene:** since `.planning/` is now tracked, the watcher's `WATCHER_MAX_SCHEMA_VERSION` workflow and the GSD phase artifacts will go into git from now on. Be conscious about what gets committed — phase work that produces secrets (new OAuth flows, API keys) should still be redacted before commit.

---

## What you do NOT need to recreate

These are baked into the repo or already provisioned in the cloud and
will Just Work as soon as Steps 0–5 are done:

- Google Cloud Console project + OAuth consent screen (in PRODUCTION since 2026-05-01).
- The shared workbook + its container-bound Apps Script (clasp `scriptId` is in the repo).
- The GitHub Pages site at `boejowen.github.io/SquireBot` (privacy policy + Search Console verification).
- Release artifacts on `github.com/boejowen/SquireBot/releases` (latest is v1.0.2).

---

## Reference

- Stack + architecture + conventions: [CLAUDE.md](../CLAUDE.md)
- Current state: `.planning/STATE.md`, `.planning/ROADMAP.md`
- Lessons learned: `.planning/RETROSPECTIVE.md`
- Open backlog: search `.planning/ROADMAP.md` for `999.` items
- Phase summaries: `.planning/phases/<NN>-*/<NN>-SUMMARY.md`
