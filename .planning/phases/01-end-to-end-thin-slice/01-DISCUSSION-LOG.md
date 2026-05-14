# Phase 1: End-to-End Thin Slice — Discussion Log

**Discussion held:** 2026-04-30
**Reference:** `01-CONTEXT.md` (canonical decisions)

This log is for human reference (audits, retrospectives). Downstream agents read CONTEXT.md, not this file.

## Areas Selected

User selected all four offered gray areas:
1. Workbook onboarding
2. First-run UX style
3. EQ folder fallback UX
4. Distribution + update channel

## Discussion

### Area 1 — Workbook onboarding

**Q:** How should the shared guild workbook be created in the first place?

**Options presented:**
- Public template + "Make a copy" link (Recommended)
- SquireBot creates it on first run
- Manual create + paste-in bootstrap script
- You decide

**Initial answer:** User clarified before answering — wanted the workbook to be openly accessible: "I want it to be a shared Google Doc that anyone can look at — even if it means seeing other people's inventories and such."

Claude flagged terminology (Doc → Sheet) and asked the user to confirm a three-part sharing model:
1. Sheet, not Doc
2. Anyone with the link → view-only
3. Edit access granted explicitly to each guildie's email so the watcher can write

**User confirmed:** "1. Sheet, not doc. 2. Anyone with the link. 3. edit access granted explicitly to each guildie's email."

**Re-asked workbook creation question. Selected:** Public template + "Make a copy" link.

**Notes / Claude's defaults captured:**
- SquireBot validates `_meta.canonical_id` on first connect; rejects non-SquireBot sheets with a clear message.
- Switching workbooks is a tray menu item ("Change Workbook…") — Phase 1 minimum just re-runs Picker.

### Area 2 — First-run UX style

**Q:** After SquireBot finishes installing, what should it do?

**Options presented:**
- Pop a setup wizard window (Recommended)
- Silent install, tray icon only, user clicks 'Setup' from tray
- OS notification + tray icon, no auto-window
- You decide

**Selected:** Pop a setup wizard window.

**Notes / Claude's defaults captured:**
- 4-step wizard: Connect Google → Pick workbook → Confirm EQ folder → Done.
- Dismissible mid-flow; tray shows red "Setup needed" when incomplete; "Continue setup…" tray menu item.
- "✓ You're all set" success screen for ~3 seconds, then minimize to tray with toast.
- Wizard tech (native Go UI vs. embedded webview) — Claude's discretion in plan phase.

### Area 3 — EQ folder fallback UX

**Q:** When auto-discovery fails to find the EQ folder, what's the fallback?

**Options presented:**
- Native folder-picker dialog (Recommended)
- Drag-and-drop the EQ folder onto the wizard
- Text-input path with validation
- Both folder-picker AND drag-and-drop

**Selected:** Native folder-picker dialog.

**Notes / Claude's defaults captured:**
- Validate picked folder contains `eqgame.exe`; reject otherwise.
- Phase 1 is single-folder; multi-folder for multiboxers is Phase 2 (WATCH-03).
- Auto-discovery order: prior config → known paths (`C:\P99` etc.) → registry uninstall keys → recursive scan for `eqgame.exe` + `eqclient.ini`.

### Area 4 — Distribution + update channel

**Q:** Where should SquireBot.exe be hosted for download?

**Options presented:**
- Public GitHub Releases (Recommended)
- Private link in guild Discord
- Both — public Releases as canonical, Discord pin links to it
- Self-hosted on a personal site/VPS

**Selected:** Public GitHub Releases.

**Notes / Claude's defaults captured:**
- Public GitHub repo, `goreleaser` produces binary + `latest.json`.
- Phase 1 ships unsigned; the SmartScreen walkthrough is exercised on the dev's clean-VM install. Code-signing decision is a Phase 2 research flag.
- Minimal README in Phase 1; Phase 5 owns the full onboarding doc.

## Deferred Ideas

None new from this discussion — all deferred items already have phase mappings in ROADMAP.md (multi-folder, code signing, auto-updater, spellbook, heartbeat, refresh-token UX, retry/backoff, catch-up, schema bootstrapping, etc.).

## Scope Creep Redirected

None. Discussion stayed within Phase 1 boundary.

---

*Discussion: Phase 1 — End-to-End Thin Slice*
*Logged: 2026-04-30*
