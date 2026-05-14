---
layout: default
---

# Secondary-PC bootstrap prompt for Claude Code

Copy/paste the fenced block below into Claude Code on the secondary PC,
**after** you've `git clone`d the repo and launched Claude Code with the
repo root as its working directory.

It rehydrates the per-project auto-memory from the committed snapshot
under `.planning/claude-memory/`, reports which toolchains are
available, and gives a project-status orientation — without installing
anything, committing anything, or touching secrets.

Companion to [`docs/new-machine-setup.md`](./new-machine-setup.md). The
prompt automates §1, §2, the toolchain-check intent of §0, and §4 of
that runbook. Conditional steps (Go build, Apps Script clasp login,
unredacted OAuth values) still need a human decision and live in the
runbook.

---

## The prompt

```
I'm Joe Bowen, working on SquireBot temporarily from this PC for a few days.
My primary PC has Cursor + Claude Code with full project memory; this PC needs
to come up to parity for editing/planning work. I will probably NOT be cutting
releases or building the watcher binary from this PC.

Please do these steps in order. Stop and ask me if any step needs a decision.
Don't push or commit anything in this setup pass.

1. Verify we're in the right repo. Run `git remote -v` and `git status`. The
   remote should be github.com/boejowen/SquireBot. If we're not in the repo
   root or the remote is wrong, stop and tell me.

2. Bootstrap my Claude auto-memory from the repo snapshot.

   - Find this session's per-project memory directory under
     `~/.claude/projects/`. It's the one whose name matches the current
     working directory after Windows-path mangling — easiest is
     `Get-ChildItem $HOME\.claude\projects | Where-Object Name -like
     "*SquireBot*"` and pick the one corresponding to the current cwd.
     If multiple match or none match, list candidates and ask me which.
   - Make sure a `memory/` subdir exists inside it.
   - Copy every `*.md` from `.planning/claude-memory/` (in this repo) into
     that `memory/` directory, overwriting existing files.
   - List the destination directory so I can confirm ~20 files landed,
     including `MEMORY.md` as the index.

3. Report (don't install) which tools are available so I know what work I
   can do from this PC. For each, just print exit status + version one line
   per tool; if missing, say so:
     git --version
     go version
     node --version
     npm --version
     makensis /VERSION   (NSIS — optional, only needed for release installer)

4. Read `docs/new-machine-setup.md` and tell me, based on the toolchain
   report from step 3, which 🟡 conditional steps still apply. For each
   missing tool, note what kind of work it would unlock.

5. Read `CLAUDE.md` and `.planning/STATE.md` to load current project
   context. In one short paragraph, summarize where the project is right
   now and what looks like a sensible next thing to pick up. Do NOT start
   any actual work — just orient yourself.

Hard rules for this pass:
- Don't run `npm install`, `go get`, `go mod download`, or any installer.
- Don't paste real OAuth secrets into
  `.planning/phases/01-end-to-end-thin-slice/oauth-config.json` — the
  committed file is intentionally redacted; if I need them I'll paste them
  in from my password manager and `git update-index --skip-worktree` the
  file myself.
- Don't run `git push`, `git commit`, or modify tracked files.

When done, give me a short "ready to work" summary noting:
- Whether Claude memory was successfully rehydrated (file count + sanity check).
- Which conditional setup steps remain if I want to do watcher or
  Apps Script work.
- The one-paragraph project-status summary from step 5.
```

---

## Usage notes

- **Idempotent.** Safe to re-paste later (e.g., after a fresh `git pull`). It'll re-copy memory files (overwriting) and re-report status with no side effects.
- **First-session edge case.** If Claude Code can't find a matching project dir under `~/.claude/projects/`, that's because it's the very first session in that cwd. Quit Claude Code, restart it from the repo root so the dir gets created, then paste the prompt.
- **Memory drift.** Any new memories Claude writes on the secondary PC stay there. When you return to the primary PC, follow §9 of [`docs/new-machine-setup.md`](./new-machine-setup.md) to merge them back if you want them preserved.
- **Variant for full migration.** If you're permanently switching machines instead of temporarily working remote, swap the opening sentence to "I'm switching primary machines permanently — please bring this PC up to full parity" and drop the "probably NOT be cutting releases" line. The rest of the prompt still works.
