---
phase: 06-installer-overwrite-running-shim
plan: 04
type: execute
wave: 1
depends_on: []
files_modified:
  - docs/troubleshooting.md
  - docs/build-and-install.md
autonomous: true
requirements: [INST-06]
tags: [docs, troubleshooting, manual-workaround-removal]

must_haves:
  truths:
    - "docs/troubleshooting.md no longer contains a section titled 'Installer won't overwrite a running SquireBot' (deleted per ROADMAP §47)."
    - "docs/troubleshooting.md no longer instructs users to right-click the tray and Quit before re-running the installer (the manual workaround is retired)."
    - "docs/build-and-install.md documents `squirebot.exe --quit` as a manual debug aid (useful when iterating on a local rebuild without killing via Task Manager)."
    - "No other docs file references the now-deleted troubleshooting section (no broken internal links)."
  artifacts:
    - path: docs/troubleshooting.md
      provides: "Troubleshooting page with the installer-overwrite section removed (lines 50-58 in the pre-edit file)"
    - path: docs/build-and-install.md
      provides: "Build runbook with a short Manual debug aids subsection documenting --quit"
  key_links:
    - from: docs/build-and-install.md
      to: cmd/squirebot/main.go --quit handler (shipped in Plan 06-02)
      via: "documentation reference to the CLI flag and its semantics"
      pattern: "--quit"
---

<objective>
Retire the manual installer-overwrite workaround from `docs/troubleshooting.md` (ROADMAP §47 success criterion 4) and add a short documentation note about the new `--quit` CLI flag to `docs/build-and-install.md` as a manual debug aid.

This plan runs in Wave 1 in parallel with Plan 01 (`internal/system` package) — no Go code dependency. The troubleshooting deletion is safe to ship before the Go/NSIS changes because if someone reads the page between this commit and the v1.0.1 release, they still have access to the manual workaround via git history; the documented behavior change (no manual stop required) only takes effect when the v1.0.1 binary lands per Plan 05.

The `build-and-install.md` addition references `--quit` as documented in CONTEXT.md / PATTERNS.md. Per CONTEXT.md the build-and-install.md edit is "POSSIBLY MODIFY" — planner's call; we INCLUDE it because the flag is a genuinely useful local-rebuild debug aid and documenting it here costs ~10 lines.

Output: two docs files modified, no behavior change in code, both edits non-blocking for the rest of Phase 6.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md
@.planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md
@docs/troubleshooting.md
@docs/build-and-install.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Delete the manual-stop section from docs/troubleshooting.md</name>
  <files>docs/troubleshooting.md</files>
  <read_first>
    - docs/troubleshooting.md (entire file — the target section is lines 50-58 as of 2026-05-11; verify before deleting)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (section "MODIFY docs/troubleshooting.md — delete lines 50-58")
    - .planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md (verification_hooks criterion 4)
  </read_first>
  <behavior>
    - The section starting with `## Installer won't overwrite a running SquireBot` (currently line 50) and continuing through the end of the manual workaround paragraph (currently line 58) is removed in its entirety.
    - The trailing blank line at line 58 (if any) is removed; the file ends cleanly after the preceding section's content.
    - No other section in the file is modified — the auth, workbook-not-updating, SmartScreen, ErrSchemaTooNew, drive.file propagation, hidden tabs, and eviction sections all remain intact.
    - The file's frontmatter (lines 1-3 with `layout: default`) is unchanged.
  </behavior>
  <action>
**Step 1: Re-verify the section is still at lines 50-58.**

Open `docs/troubleshooting.md`, confirm:
- Line 50: `## Installer won't overwrite a running SquireBot`
- Line 51: blank
- Line 52: starts with `NSIS installers cannot replace a` (the intro paragraph)
- Line 53: blank
- Line 54: `1. Right-click the tray icon and select` followed by backtick-Quit-backtick.
- Line 55: starts with `2. Confirm SquireBot is no longer running` and mentions Task Manager + squirebot.exe
- Line 56: `3. Re-run the installer.`
- Line 57: blank
- Line 58: `A future v1.0.x release may add a quit-and-relaunch shim, but the manual stop-then-install path is the current workaround.`

If the line numbers have shifted (e.g., someone edited the file since 2026-05-11), use the section heading `## Installer won't overwrite a running SquireBot` as the authoritative anchor — delete from that heading through the end of the paragraph beginning `A future v1.0.x release...` (inclusive).

**Step 2: Delete the entire section.**

Use the Edit tool to remove the 9 lines (50-58 inclusive) — heading, blank line, intro paragraph, blank line, ordered list (3 items), blank line, closing paragraph.

After deletion, the file ends with the `## I need to remove a guildie` section (currently lines 46-48 — the `See the [Eviction runbook]...` paragraph) followed by the file's natural EOF. Confirm there is exactly one trailing newline at EOF (matches markdown convention; matches the file's existing style).

**Do NOT modify:**
- The frontmatter (lines 1-3).
- The intro paragraph (lines 5-7).
- Any of the other 7 sections (Tray icon is red, Workbook isn't updating, SmartScreen warning, ErrSchemaTooNew, drive.file propagation, Hidden tabs, I need to remove a guildie).
- The links to other docs (smartscreen-walkthrough, signpath-application, eviction-runbook).

**Do NOT add a replacement section.** ROADMAP §47 says "the manual workaround block is removed" — no "see the new behavior" stub, no "this is now automatic" note. Silent deletion is the correct interpretation: when v1.0.1 ships (Plan 05), the section is gone because the user no longer needs to do anything; the installer handles it.
  </action>
  <verify>
    <automated>powershell -NoProfile -Command "if (Select-String -Path docs/troubleshooting.md -Pattern 'Installer won''t overwrite a running SquireBot' -SimpleMatch) { Write-Host FAIL_SECTION_PRESENT; exit 1 }; if (Select-String -Path docs/troubleshooting.md -Pattern 'manual stop-then-install' -SimpleMatch) { Write-Host FAIL_MANUAL_PHRASE; exit 1 }; if (Select-String -Path docs/troubleshooting.md -Pattern 'quit-and-relaunch shim' -SimpleMatch) { Write-Host FAIL_SHIM_PHRASE; exit 1 }; Write-Host PASS; exit 0"</automated>
  </verify>
  <acceptance_criteria>
    - Grep `Select-String -Path docs/troubleshooting.md -Pattern "Installer won't overwrite a running SquireBot" -SimpleMatch` matches zero lines.
    - Grep for the substring `manual stop-then-install` matches zero lines.
    - Grep for the substring `quit-and-relaunch shim` matches zero lines.
    - Grep `Select-String -Path docs/troubleshooting.md -Pattern '^## I need to remove a guildie$'` matches exactly 1 line (the section preceding the deleted block is intact).
    - Grep `Select-String -Path docs/troubleshooting.md -Pattern '^## Tray icon is red$'` matches exactly 1 line.
    - Grep `Select-String -Path docs/troubleshooting.md -Pattern '^## Hidden tabs'` matches exactly 1 line (other sections unchanged).
    - Grep `Select-String -Path docs/troubleshooting.md -Pattern '^---$'` matches exactly 2 lines (frontmatter delimiters preserved).
    - File still parses as valid markdown (no truncated code fences, no orphan list items). Manual inspection if any uncertainty.
    - Whole-repo grep for links to the deleted section: search docs/*.md, README.md, and .planning markdown for the substrings `installer-wont-overwrite` and `installer-won-t-overwrite` — both should return zero matches.
  </acceptance_criteria>
  <done>
    The "Installer won't overwrite a running SquireBot" section is gone from `docs/troubleshooting.md`. No other section is touched. ROADMAP §47 success criterion 4 is satisfied. Plan 05's release tag is the moment this documentation change becomes user-visible alongside the new installer behavior.
  </done>
</task>

<task type="auto">
  <name>Task 2: Add Manual debug aids subsection to docs/build-and-install.md</name>
  <files>docs/build-and-install.md</files>
  <read_first>
    - docs/build-and-install.md (full file — confirm structure; the only existing "Manual" mentions are at lines 36 (NSIS install) and 254 (Manual recovery section); the new subsection slots near the end for discoverability)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (section "MODIFY docs/build-and-install.md — optional --quit note")
    - .planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md (the --quit CLI contract shipped in Plan 02)
  </read_first>
  <behavior>
    - A new subsection titled `### Manual debug aids` exists in `docs/build-and-install.md`.
    - The subsection documents `squirebot.exe --quit` with: what it does, when to use it, what happens if no watcher is running.
    - The subsection references "Plan 06 / INST-06" inline for traceability (matches the file's existing convention of citing plan IDs — see line 9 "01-CONTEXT.md D-13", line 23 "Plan 01-02").
    - The subsection is placed near the end of the file (after existing procedural sections), so a reader scanning for build steps doesn't trip over it.
    - No other section in the file is modified.
  </behavior>
  <action>
**Step 1: Find the insertion point.**

Locate the bottom of `docs/build-and-install.md`. Find the last `##` section before EOF. Insert the new `### Manual debug aids` subsection at the end of the file (or just before any final footer/license boilerplate if one exists).

If the file ends with a recovery section like `### Manual recovery (if you chose "No" but later want a full wipe)` (line 254 per the prior grep result), insert the new subsection AFTER it. The new subsection becomes the last content in the file.

**Step 2: Insert the subsection.**

Append this block (paste verbatim; preserve the blank line above the heading):

```markdown

### Manual debug aids

These flags are not used by end users; they are intended for iterating on a local
rebuild without going through Task Manager.

- **`squirebot.exe --quit`** — signals a running watcher to shut down gracefully
  via a Windows named event (`Local\SquireBot-Shutdown`). The watcher's listener
  goroutine observes the signal and invokes the same `cancel() + systray.Quit()`
  shutdown path that the tray Quit menu uses. Exits 0 within ~1s regardless of
  whether a watcher is currently running (signal-with-no-listener is a benign
  no-op).
  - Use when: iterating on a local rebuild and you want the previous tray
    instance gone before launching the new one without killing via Task Manager.
  - The NSIS installer's pre-install shim uses the same flag to upgrade over a
    running watcher (Plan 06 / INST-06).
  - Does NOT delete config or credentials. For that, use the NSIS uninstaller
    and answer "Yes" to the wipe prompt (`--uninstall-wipe-credentials`).
```

Conventions enforced:
- Use `### ` (3 hashes) to match the existing subsection depth (the file uses `##` for top-level sections and `###` for subsections — see the prereq subsections near the top).
- Inline plan-ID reference `Plan 06 / INST-06` matches existing style (line 9, line 23).
- Bullet list with sub-bullets matches the file's existing list style.
- No code-block invocation that would suggest end users should run this (the lead paragraph explicitly scopes the audience to local-rebuild developers).

**Do NOT modify:**
- The frontmatter / top matter (lines 1-15 ish).
- The `## Prerequisites` table.
- Any of the existing procedural sections (build, package, install, smoke, troubleshoot-build).
- The footer if one exists.

**Do NOT cross-link to `docs/troubleshooting.md`'s deleted section.** Plan 04 Task 1 removed it; the cross-link would be a broken anchor.
  </action>
  <verify>
    <automated>powershell -NoProfile -Command "$h = Select-String -Path docs/build-and-install.md -Pattern '^### Manual debug aids$'; if (-not $h -or $h.Count -ne 1) { Write-Host FAIL_HEADING; exit 1 }; if (-not (Select-String -Path docs/build-and-install.md -Pattern 'squirebot\.exe --quit' -SimpleMatch)) { Write-Host FAIL_QUIT_FLAG; exit 1 }; if (-not (Select-String -Path docs/build-and-install.md -Pattern 'Plan 06 / INST-06' -SimpleMatch)) { Write-Host FAIL_PLAN_REF; exit 1 }; Write-Host PASS; exit 0"</automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path docs/build-and-install.md -Pattern '^### Manual debug aids$'` matches exactly 1 line.
    - `Select-String -Path docs/build-and-install.md -Pattern 'squirebot\.exe --quit'` matches at least 1 line (in the new subsection).
    - `Select-String -Path docs/build-and-install.md -Pattern 'Local\\SquireBot-Shutdown' -SimpleMatch` matches at least 1 line (event name documented for grep-ability).
    - `Select-String -Path docs/build-and-install.md -Pattern 'Plan 06 / INST-06' -SimpleMatch` matches at least 1 line.
    - `Select-String -Path docs/build-and-install.md -Pattern 'signal-with-no-listener'` matches at least 1 line (the no-op contract is documented).
    - Existing `## Prerequisites` heading is still present (not accidentally clobbered).
    - The file size GREW (not shrank) — sanity check via `(Get-Item docs/build-and-install.md).Length` exceeds the pre-edit size.
  </acceptance_criteria>
  <done>
    `docs/build-and-install.md` has a new `### Manual debug aids` subsection documenting the `--quit` flag with reference to Plan 06 / INST-06. The flag is now discoverable to developers iterating on local rebuilds, and the cross-reference makes the connection to the installer's pre-install shim explicit.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Documentation → user behavior | Removing the manual workaround in docs while the new installer is not yet shipped (Plan 05) creates a documentation gap window. |
| Build-and-install.md → developer instructions | Documenting `--quit` invites developers to use it in scripts; misuse is bounded by the no-op contract. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-17 | Information Disclosure | The build-and-install.md subsection reveals the named-event identifier `Local\SquireBot-Shutdown` to public-facing docs | accept | The event name is already in the source code (cmd/squirebot/main.go after Plan 02, internal/system/shutdown_signal_windows.go after Plan 01), which is open-source on github.com/boejowen/SquireBot. The event is per-session local; T-06-01 in Plan 01 already accepted the spoofing risk. Documenting the name does not add attack surface. |
| T-06-18 | Denial of Service | If users discover --quit before v1.0.1 ships (via the docs change), they may run it on a v1.0.0 binary expecting graceful shutdown — v1.0.0 falls through to normal launch and spawns a duplicate tray | mitigate | The Plan 04 docs change ships AFTER (or alongside) the v1.0.1 release tag per the natural git history. The Wave 1 dependency-free placement is for parallel planning convenience, NOT for ship ordering. Plan 05 (release tag) is the user-visible cutover point. If docs land on master before v1.0.1 ships, the worst case is a v1.0.0 user reading docs/build-and-install.md (a developer-targeted file) and running `--quit`, getting a duplicate tray — they kill both via Task Manager and the system self-corrects on next launch. Low-probability, low-impact, self-recovering. |
| T-06-19 | Tampering | The deleted troubleshooting section was the documented workaround for a real failure mode (installer cannot overwrite running .exe) — deleting it before the fix ships breaks user-facing recovery instructions | accept | The deletion is committed atomically with the rest of the phase. Plan 05 (release tag) is the ship gate; the git tag and the master HEAD will agree. Between commit and tag (minutes to hours), a v1.0.0 user hitting the failure can still find the workaround in git history or via web search (the page is indexed). Acceptable risk window. |

ASVS L1: no `high` severity threats. All risks are documentation-process risks, not code vulnerabilities. Mitigations are timing/coordination, not technical controls.
</threat_model>

<verification>
- `docs/troubleshooting.md` no longer contains any of the deleted-section anchor phrases.
- `docs/build-and-install.md` has the new subsection with `--quit` documented.
- No file outside `docs/` is touched.
- No Go code, no NSIS code, no CI workflow changes (pure docs plan).
- Whole-repo grep for `installer-wont-overwrite` returns zero matches (no broken anchor links).
</verification>

<success_criteria>
- `docs/troubleshooting.md` no longer contains the manual-stop section.
- `docs/build-and-install.md` has a `### Manual debug aids` subsection documenting `--quit`.
- ROADMAP §47 success criterion 4 satisfied: docs no longer instruct users to manually stop the tray app before reinstalling.
- No other doc, code, or CI file is modified.
- Plan can ship in parallel with Plans 01, 02, 03 (Wave 1 placement) without conflict (no overlap in files_modified).
</success_criteria>

<output>
After completion, create `.planning/phases/06-installer-overwrite-running-shim/06-04-SUMMARY.md` capturing:
- Exact line numbers deleted from `docs/troubleshooting.md` (post-edit confirmation).
- Final position of the new subsection in `docs/build-and-install.md`.
- Confirmation that no other files were modified.
- Note: if landing on master ahead of v1.0.1 tag, there is a small "docs ahead of binary" window per T-06-19 — flag for the executor to coordinate with Plan 05's tag push.
</output>
</content>
</invoke>