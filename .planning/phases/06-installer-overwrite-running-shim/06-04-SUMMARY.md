---
phase: 06-installer-overwrite-running-shim
plan: 04
subsystem: docs
tags: [docs, troubleshooting, build-runbook, manual-workaround-removal]
requires: []
provides:
  - "Retired manual installer-overwrite workaround from user-facing troubleshooting"
  - "Documented --quit CLI flag as a developer debug aid in build runbook"
affects:
  - docs/troubleshooting.md
  - docs/build-and-install.md
tech_stack:
  added: []
  patterns:
    - "Inline plan-ID references (Plan 06 / INST-06) for traceability"
    - "Silent deletion over 'now-automatic' stub (per ROADMAP §47 success criterion 4)"
key_files:
  created:
    - .planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md
  modified:
    - docs/troubleshooting.md
    - docs/build-and-install.md
decisions:
  - "Delete the 'Installer won't overwrite a running SquireBot' section in full rather than rewrite it (no 'this is now automatic' note; the user simply no longer needs to do anything)"
  - "Place the new 'Manual debug aids' subsection at the END of build-and-install.md, AFTER the 'CI parity' section, with a horizontal-rule separator — keeps developer-targeted material out of the way of the build/install procedural flow"
  - "Use ### depth (subsection) rather than ## (top-level) to signal 'this is auxiliary developer information', matching the existing depth used for SmartScreen walkthrough and Manual recovery subsections"
metrics:
  duration: "~5 minutes"
  completed: 2026-05-11
  tasks_completed: 2
  files_changed: 2
---

# Phase 6 Plan 04: Docs Update Summary

Removed the manual installer-overwrite workaround from `docs/troubleshooting.md` and documented the new `--quit` CLI flag in `docs/build-and-install.md` as a developer debug aid — two-file pure-docs change satisfying ROADMAP §47 success criterion 4 with no code impact.

## What changed

### Task 1: docs/troubleshooting.md

Deleted the entire `## Installer won't overwrite a running SquireBot` section block (heading + intro paragraph + 3-item ordered list + closing paragraph). Pre-edit this lived at lines 50-58 inclusive; post-edit the file ends cleanly with the `## I need to remove a guildie` section.

- Pre-edit file size: 4231 bytes
- Post-edit file size: 3752 bytes (-479 bytes, ~9 lines removed)
- Section boundaries preserved: 1× `## I need to remove a guildie`, 1× `## Tray icon is red`, 1× `## Hidden tabs ...`, 2× `---` frontmatter delimiters — all intact.
- No replacement stub added; ROADMAP §47 says the manual workaround block is removed full stop. When v1.0.1 ships (Plan 05), the section is gone because the user no longer needs to do anything.

### Task 2: docs/build-and-install.md

Appended a new `### Manual debug aids` subsection at the END of the file, after the `## CI parity` section, with a horizontal-rule separator above it. The subsection:

- Documents `squirebot.exe --quit` with: what it does, when to use it, the no-op-with-no-listener contract.
- Names the Windows named event (`Local\SquireBot-Shutdown`) for grep-ability — already public per cmd/squirebot/main.go after Plan 06-02 (threat T-06-17 accepted in the plan's threat model).
- Cross-references `Plan 06 / INST-06` inline (matching the existing convention used at line 9 for `01-CONTEXT.md D-13` and line 23 for `Plan 01-02`).
- Notes that `--quit` does NOT delete config or credentials (orthogonal to the uninstaller's `--uninstall-wipe-credentials` flag).

- Pre-edit file size: 16172 bytes
- Post-edit file size: 17167 bytes (+995 bytes, ~20 lines added)
- Heading count: 1× `### Manual debug aids` ✓
- Flag mention: 1× `squirebot.exe --quit` ✓
- Plan reference: 1× `Plan 06 / INST-06` ✓
- Event name: 1× `Local\SquireBot-Shutdown` ✓
- No-op contract phrase: 1× `signal-with-no-listener` ✓
- Existing `## Prerequisites` heading preserved: 1× ✓

## Acceptance criteria

All Task 1 and Task 2 `<acceptance_criteria>` blocks verified via PowerShell `Select-String`:

| Criterion | Result |
|-----------|--------|
| `troubleshooting.md`: `Installer won't overwrite a running SquireBot` matches | 0 ✓ |
| `troubleshooting.md`: `manual stop-then-install` matches | 0 ✓ |
| `troubleshooting.md`: `quit-and-relaunch shim` matches | 0 ✓ |
| `troubleshooting.md`: `## I need to remove a guildie` matches | 1 ✓ |
| `troubleshooting.md`: `## Tray icon is red` matches | 1 ✓ |
| `troubleshooting.md`: `## Hidden tabs` matches | 1 ✓ |
| `troubleshooting.md`: `^---$` matches | 2 ✓ |
| Repo-wide: `installer-wont-overwrite` / `installer-won-t-overwrite` matches | 0 / 0 ✓ |
| `build-and-install.md`: `### Manual debug aids` matches | 1 ✓ |
| `build-and-install.md`: `squirebot.exe --quit` matches | 1 ✓ |
| `build-and-install.md`: `Plan 06 / INST-06` matches | 1 ✓ |
| `build-and-install.md`: `Local\SquireBot-Shutdown` matches | 1 ✓ |
| `build-and-install.md`: `signal-with-no-listener` matches | 1 ✓ |
| `build-and-install.md`: `## Prerequisites` preserved | 1 ✓ |
| `build-and-install.md` size grew | 16172 → 17167 ✓ |

## Deviations from Plan

None. Plan executed exactly as written. The action steps in both tasks specified the exact insertion points and content; both were applied verbatim.

## Outcomes

- ROADMAP §47 success criterion 4 satisfied: `docs/troubleshooting.md` no longer instructs users to manually stop the tray app before reinstalling.
- The `--quit` flag (shipped in Plan 06-02) is now discoverable to developers iterating on local rebuilds, with the cross-reference making the connection to the installer's pre-install shim (Plan 06-03) explicit.
- No Go code, NSIS code, CI workflow, or schema change in this plan — pure docs, dependency-free.
- Files outside `docs/` were not touched. Pre-existing modifications to `.planning/ROADMAP.md`, `.planning/STATE.md`, and the untracked `internal/system/` directory belong to other Wave 1 work (Plan 01) and are not part of this commit.

## Deferred

Nothing deferred from this plan. The plan was a pure docs delta with no exploratory scope.

## Coordination note (T-06-19 from plan threat model)

The plan's threat register flagged a small "docs ahead of binary" window: if this commit lands on master before v1.0.1 ships (Plan 05's release tag), a v1.0.0 user reading `docs/build-and-install.md` could try `--quit` and get a duplicate tray icon (v1.0.0 falls through unknown flags into a normal launch).

Mitigation: `docs/build-and-install.md` is a developer-targeted file (build/install runbook, not end-user docs), so the exposure is low. Plan 05's release tag IS the user-visible cutover point. Worst case is one developer experiencing a brief duplicate-tray on a v1.0.0 binary, killed via Task Manager, self-corrects on next launch. Accepted per the plan's threat disposition.

## Self-Check: PASSED

- docs/troubleshooting.md: edits verified via grep above (all 8 criteria PASS).
- docs/build-and-install.md: edits verified via grep above (all 7 criteria PASS).
- .planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md: this file exists (you are reading it).
- Commits: pending creation in the next step; will be appended to this section by the executor's git-log lookup post-commit.
