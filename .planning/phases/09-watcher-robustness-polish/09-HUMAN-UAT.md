---
status: blocked
phase: 09-watcher-robustness-polish
source: [09-VERIFICATION.md]
started: 2026-05-13T04:30:00Z
updated: 2026-05-13T04:30:00Z
blocked_on: "999.19 — Google OAuth brand verification (submitted 2026-05-13, in Google review queue, ETA 3–5 business days)"
---

## Current Test

[awaiting Google brand verification approval before any on-VM scenario can run]

## Tests

All five tests below are programmatically verified in `09-VERIFICATION.md`. They cannot be run end-to-end on a fresh v1.0.2 install until Google approves brand verification — every new OAuth flow currently fails with `Access blocked: Authorization Error / Error 400: invalid_request` independent of which watcher version is running. See `.planning/debug/v1-0-2-oauth-invalid-client-incident.md` for the full incident trail.

### 1. AUTH-07 boot-time invalid_grant → Reauthorize UX
expected: After revoking the refresh token in Google Account settings between sessions, restart the watcher: red tray icon AND visible Reauthorize menu item appear from boot. Clicking Reauthorize reopens the OAuth flow and recovers without restart.
result: pending — blocked on 999.19 (cannot OAuth at all)

### 2. OPS-06 wincred fast-fail recovery
expected: When `buildTokenSourceFromWincred` fails on boot (simulate by corrupting wincred or denying read access), the tray menu opens with a working state (red icon + Reauthorize or ContinueSetup visible) — NOT stuck at "Initialising…" with no recovery path.
result: pending — partial run possible (the queue + tray UX can be exercised without completing OAuth; reauth-click path blocked)

### 3. OPS-07 foreground-shell-close detach
expected: Launch `squirebot.exe` from `cmd.exe` (NOT via `Start-Process`), then close the cmd window. The watcher process keeps running (visible in Task Manager / tray icon stays alive). No silent death.
result: pending — runnable independently of OAuth; can be executed at any time

### 4. CONFIG-01 BOM-prefixed config.json
expected: Hand-edit `%LOCALAPPDATA%\SquireBot\config.json` in Notepad (which writes a UTF-8 BOM by default), save, restart watcher. The watcher boots normally — no `invalid character 'ï' looking for beginning of value` error visible in the log file.
result: pending — runnable independently of OAuth; can be executed at any time (the watcher still starts and parses config even while OAuth is blocked; only Sheets writes fail)

### 5. v1.0.1 → v1.0.2 auto-update smoke
expected: An existing v1.0.1 watcher install picks up `latest.json` on its next periodic check, downloads `SquireBot-Setup-1.0.2.exe` (or `squirebot.exe` per OPS-04 self-update path), and successfully upgrades to v1.0.2 on next restart.
result: pending — runnable independently of OAuth; the update path itself doesn't use OAuth

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 1 (test 1 fully blocked on 999.19; tests 2/3/4/5 runnable now but deferred to be batched once OAuth unblocks)

## Gaps

No code gaps. The phase shipped 5/5 must-haves with programmatic verification; the on-VM smoke is the canonical follow-up per Plan 09-05 / PATTERNS.md. Once 999.19 (Google brand verification) clears, retry tests 1 and 2 to confirm the Reauthorize UX is visible from boot and the queue drains as designed.
