---
name: v0.2.0 wizard handoff bug (BUG-001) — RESOLVED in v0.2.1
description: First-install hang in v0.2.0 caused by wizard.Run() blocking on browser-fired /wizard/shutdown POST. Fixed 2026-05-09 in commit 71f7b76 (signalDone now fires in handleEQFolderConfirm). Shipped v0.2.1.
type: project
originSessionId: 20b42836-7bb6-4c18-8e07-c2e0277b46d5
---

> **STATUS UPDATE 2026-05-09:** ✅ RESOLVED. Fix committed as `71f7b76`, shipped in v0.2.1. Root cause: wizard `Run()` blocked on `<-s.done` waiting for `done.html`'s 3-second setTimeout to POST `/wizard/shutdown`; if user closed browser before timer fired, channel never sent on, watcher never started. Fix: signal completion immediately in `handleEQFolderConfirm` after `cfg.Save()`, before redirect to `/done`. Browser becomes irrelevant to completion. Two regression tests in `internal/wizard/server_test.go`. README workaround callout removed.

v0.2.0 has a bug in the first-install flow: after the setup wizard completes (OAuth → workbook picker → EQ folder), the running watcher doesn't get notified to reload the new config. Tray stays in "Initializing" state indefinitely. The wizard DOES persist `config.json` correctly to disk; only the in-process notification is broken.

**Why:** Discovered 2026-05-09 during boejowen's first-guildie-install validation on dev box, immediately after v0.2.0 shipped (commit `333f878`). README's `releases/latest/download/SquireBot-Setup.exe` link is live; if guildies install before v0.2.1, every one of them hits this.

**How to apply:**
- **Do NOT mass-distribute v0.2.0 to the guild yet.** Either wait for v0.2.1 (the fix), or distribute with the README workaround prominently surfaced and a clear "if it hangs, quit + relaunch" note.
- README has a heads-up callout at the end of the install steps + a full Known issues entry pointing at the workaround. Both committed.
- Bug tracked locally at `.planning/bugs/v0.2.1-wizard-handoff.md` (gitignored — local dev artifact).
- Workaround is `Stop-Process squirebot -Force; Start-Process "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe"`. Catch-up upload then fires for any pre-existing inventory/spellbook files.
- Fix is likely a missing channel send between `internal/wizard/server.go` and `runWatcher` in `internal/app/runapp.go` — needs investigation. Trace the wizard→watcher handoff code path. Possible files: `cmd/squirebot/main.go`, `internal/wizard/server.go`, `internal/app/runapp.go`.

**Discovery context:**
- Test env: clean wipe of `config.json` + `cmdkey /delete:LegacyGeneric:target=SquireBot:boejowen@gmail.com` to clear stale Phase 1 dev state, then fresh launch
- Wizard logs showed all 3 steps complete cleanly (oauth callback → workbook picked "RiverFAIL VanFRAUD" → eq-folder confirmed)
- 2-minute gap with zero log activity after eq-folder confirmation
- Tray's "Open Workbook" reported "no spreadsheet configured yet"
- After quit + relaunch: scaffold ran, watcher started, catch-up uploaded 7 characters' inventory + 1 spellbook in 7 seconds. End-to-end works.

**Related but separate finding (also confirmed 2026-05-09):** the boejowen Google account's pre-existing OAuth refresh token (issued during Phase 1 testing on 2026-05-01) had silently expired with `invalid_grant: Token has been expired or revoked` when the v0.2.0 install tried to use it for the OLD workbook scaffold. That's Pitfall #1 (the 7-day Testing-mode expiry) firing exactly as predicted — useful confirmation that AUTH-05 surfaces it correctly and that the Production-flip on 2026-05-01 doesn't retroactively un-expire pre-flip tokens.
