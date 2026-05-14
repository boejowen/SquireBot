---
layout: default
---

# Working on SquireBot from another PC

Step-by-step for picking up SquireBot work on a different Windows PC.
Covers two scenarios:

- **Temporary secondary PC** — you'll be there a few days, your primary PC stays online. Most steps below are "do once and forget"; the watcher build + OAuth-secret restore are only needed if you're cutting a release or doing watcher debugging.
- **Full migration** — you're switching primary machines permanently. Same steps, but you also need to copy unredacted OAuth values from the old PC (they aren't in the repo) and treat the new PC's Claude auto-memory as the new source of truth.

Look for **🟡 Only if…** callouts to skip steps that don't apply to your scenario.

> **Want to automate steps 1, 2, and 4?** Paste
> [`docs/secondary-pc-bootstrap-prompt.md`](./secondary-pc-bootstrap-prompt.md)
> into Claude Code on the secondary PC after cloning. It rehydrates
> auto-memory, audits toolchains, and orients you on project state in
> one pass.

This is **not** the guildie install path — guildies just run
`SquireBot-Setup-X.Y.Z.exe` per [docs/install.md](./install.md).

---

## 0. Prerequisites

Cursor + Claude Code are assumed installed under your account on the
secondary PC (since you said they already are). Everything else below
is the **maximum** set — only install what your work for the trip
actually needs.

| Tool | Version | Needed for |
|---|---|---|
| Git | any recent | always |
| Go | **1.24** | 🟡 only if you'll build the watcher |
| Node.js | 20 LTS or newer | 🟡 only if you'll work on `apps-script/` |
| NSIS | **3.10+** | 🟡 only if you'll cut a release installer |
| PowerShell | 7+ recommended | always (5.1 is fine) |
| `jq` | any | optional — for the bash build one-liner |

Don't install `clasp` globally — it's a project dev-dependency in
`apps-script/package.json` and **must stay on the 2.4.x line** (clasp 3.x
has breaking changes; see
`.planning/phases/03-apps-script-enrichment-foundation/03-RESEARCH.md` §6).

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
- Full GSD planning history under `.planning/`.
- A snapshot of the primary PC's Claude auto-memory under `.planning/claude-memory/`.
- The Apps Script clasp binding (`apps-script/.clasp.json` with the right `scriptId`).
- Docs under `docs/` (build, OAuth setup, deploy, eviction runbook, troubleshooting).

---

## 2. Rehydrate Claude's auto-memory

Claude Code reads per-project memory from
`~/.claude/projects/<sanitized-cwd>/memory/MEMORY.md`. On a PC that
hasn't run Claude Code against this repo before, that directory is
empty. Copy the committed snapshot into place:

```powershell
# 1. Open Claude Code in the repo once so it creates the project dir, then quit.
# 2. Find the sanitized project dir name (it depends on where you cloned the repo):
Get-ChildItem $HOME\.claude\projects | Where-Object Name -like "*SquireBot*"
# 3. Copy the memory files in:
$projectDir = "$HOME\.claude\projects\<paste-name-from-step-2>\memory"
New-Item -ItemType Directory -Force -Path $projectDir | Out-Null
Copy-Item -Force .planning\claude-memory\*.md $projectDir\
```

Then restart Claude Code in the repo. Verify by asking "what do you
remember about this project?" — it should reference P99 file formats,
the consolidated view-tab decision, the OAuth brand-verification
incident, etc.

> **Sync caveat (temporary-PC scenario):** any new memories Claude
> writes on the secondary PC stay there. They won't automatically
> appear on your primary PC when you return. Either:
> - Accept the divergence and let the primary PC's memory remain authoritative, or
> - Copy new memory files from the secondary PC's `memory/` dir back to `.planning/claude-memory/` on the primary PC before committing further.
> The committed `.planning/claude-memory/` snapshot is intentionally a one-time bootstrap, not a live sync.

---

## 3. 🟡 Restore the unredacted OAuth config

Skip this step unless you'll be building the watcher binary or cutting
a release on the secondary PC. Pure code/planning/Apps Script work
doesn't need it.

The committed `.planning/phases/01-end-to-end-thin-slice/oauth-config.json`
has placeholders. Paste the real values from your password manager
(the four real values: `oauth_client_id`, `oauth_client_secret`,
`picker_api_key`, `gcp_project_number`) into the file. Then mark it
skip-worktree so a stray `git add` can't push secrets:

```powershell
git update-index --skip-worktree .planning/phases/01-end-to-end-thin-slice/oauth-config.json
```

Reverse with `--no-skip-worktree` if you ever need to update the
redacted schema in the committed copy.

If you don't have the values handy: get them from Google Cloud Console
→ project `262087828393` → APIs & Services → Credentials. Don't rotate
the secret unless you intend to cut a new release with new ldflags
values — rotation invalidates the credential baked into every
already-installed guildie watcher.

---

## 4. Editor + Claude Code + GSD sanity check

- **Cursor:** open the repo folder. The repo doesn't ship `.vscode/` settings (gitignored).
- **Claude Code:** run `claude` inside the repo. The project's `CLAUDE.md` is auto-loaded.
- **GSD:** `/gsd-progress` should print the current milestone status from `.planning/STATE.md`, `.planning/ROADMAP.md`, etc. If GSD complains the dir is malformed, run `/gsd-health`.

`.claude/settings.local.json` is per-machine and intentionally not in
the repo. The secondary PC has its own.

---

## 5. 🟡 Apps Script — only if you'll touch `apps-script/`

```powershell
cd apps-script
npm install            # installs clasp 2.4.x, esbuild, vitest, @types/google-apps-script
npx clasp login        # opens browser; sign in as the workbook owner (jbowen@mncivic.com)
npm run build          # bundles src/ into dist/Code.js
npm test               # vitest against mocked Apps Script globals
```

`clasp login` writes `~/.clasprc.json` (user-level OAuth, not committed).
The repo already has `apps-script/.clasp.json` with the bound `scriptId`,
so `clasp push` will hit the correct container-bound script.

Deploy on demand:

```powershell
npm run deploy         # or: npx clasp push
```

Full runbook: [docs/apps-script-deploy.md](./apps-script-deploy.md).

---

## 6. 🟡 Watcher — only if you'll build / debug the Go side

Follow [docs/build-and-install.md](./build-and-install.md). Summary:

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

You **do not** need to install the watcher on the secondary PC for
development — `go build` and `go test ./...` give you everything you
need to iterate. Only do a full install + sign-in if you're testing
the end-to-end flow.

NSIS installer (only if cutting a release):

```powershell
& "$env:ProgramFiles (x86)\NSIS\makensis.exe" /DVERSION=0.0.0-dev installer\squirebot.nsi
```

---

## 7. 🟡 Run the watcher end-to-end — only if you need a live install

If you actually install + run `squirebot.exe` on the secondary PC,
you'll go through the wizard and a fresh Google sign-in there
(refresh tokens are stored in Windows DPAPI per-PC; they don't
transfer). Your primary PC's watcher is unaffected.

1. Run `squirebot.exe` — wizard auto-launches if no token is stored.
2. Pick EQ folder, sign in with Google, pick the shared workbook.
3. Wait for tray icon to go "Ready"; next `/outputfile inventory` will write.

Both PCs writing to the same sheet at once is fine — the per-character
write contract (atomic `batchUpdate` clear+write per character) means
they don't collide as long as you don't run the SAME character from
both PCs simultaneously.

---

## 8. Working day-to-day

Once set up, the loop is the same as on the primary PC:

```powershell
git pull
# ... do work, /gsd-* slash commands, edits ...
git push
```

GitHub keeps both PCs in sync for tracked files. Things that do NOT
sync via git, by design:

- Claude auto-memory (see Sync caveat in §2).
- `.claude/settings.local.json` (per-machine permissions).
- The unredacted `oauth-config.json` (skip-worktree on both PCs).
- Windows DPAPI OAuth refresh tokens (per-PC by Windows design).
- Built artifacts: `dist/`, `squirebot.exe`, `node_modules/`.

---

## 9. When you return to the primary PC

Temporary-PC scenario:

1. On the secondary PC, push any work-in-progress branches (or commit + push to `master` if you've been working there).
2. On the primary PC, `git pull`.
3. If you accumulated useful new Claude memory entries on the secondary PC, copy them from `~/.claude/projects/<...>/memory/*.md` over to the primary PC's same directory (manual merge — both PCs may have edited `MEMORY.md`).
4. If you used `clasp push` from the secondary PC, no action needed — the Apps Script container project is the source of truth.

---

## What you do NOT need to recreate

These are baked into the repo or already provisioned in the cloud and
will Just Work as soon as Steps 0–4 are done:

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
