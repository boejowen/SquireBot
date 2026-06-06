---
status: partial
phase: 25-linux-watcher
source: [25-VERIFICATION.md]
started: 2026-06-06
updated: 2026-06-06
---

## Current Test

[awaiting on-machine testing on a real Linux + WINE + systemd host — the watcher is code-complete + cross-compile-verified (static ELF, CGO-free, all gates green); these 4 items need a Linux box + a published release, exactly like the Windows watcher's on-machine UATs.]

## Tests

### 1. Live watch → upload on Linux + WINE
expected: On a Linux box running P99 under WINE, `install.sh` + `--setup` (guild code + EQ folder, auto-detected from the WINE prefix or entered) → the watcher detects a `/outputfile inventory|spellbook` `.txt` write in the WINE-prefix EQ folder, debounces 500 ms, parses, and uploads over HTTPS; the rows appear on squirebot.quest for that character.
result: [pending] code path is the same cross-platform watch+parse+upload as Windows (fsnotify/inotify); needs a real WINE EQ folder to confirm discovery + a live upload.
why_human: requires a Linux+WINE host with a P99 install and a real guild code — cannot be exercised from the Windows dev box.

### 2. systemd user-service lifecycle + autostart
expected: After `install.sh`, `systemctl --user status squirebot` is `active (running)`; it survives logout/login (or with `install.sh --linger` survives without a session); `systemctl --user stop` terminates it cleanly (the SIGTERM→cancel handler), `Restart=always` relaunches after a crash, and `StartLimitIntervalSec=0` prevents a transient loop stranding the unit `failed`.
result: [pending] unit + handler are code-verified; live `systemctl --user` behavior needs a real systemd host.
why_human: systemd user-unit lifecycle can only be observed on the running Linux box.

### 3. Auto-update swap on Linux
expected: With a published release carrying the linux asset (`binary_url_linux` + sha256 in the manifest), a running Linux watcher downloads the bare linux `squirebot` binary, SHA-256-verifies it, swaps in place (no handle lock), exits, and systemd `Restart=always` relaunches the new version; a corrupt staged update is discarded (RemoveStaged) and the next check re-downloads rather than bricking the launch.
result: [pending] manifest OS-asset selection + RemoveStaged self-heal are code-verified + unit-tested; the end-to-end swap needs a published release + a Linux host.
why_human: requires a tagged Release with the linux assets + a Linux box to observe the live swap+restart.

### 4. Execute the `!windows` unit tests on a Linux runner
expected: `go test ./internal/credstore/... ./internal/eqfind/... ./internal/onboarding/... ./internal/config/... ./internal/logging/...` RUN (not just compile) green on a linux/amd64 runner (the Windows dev box can only cross-compile the ELF test binaries, not execute them).
result: [pending] all `!windows` test files are compile-verified here (`GOOS=linux go test -c` exits 0); they run-verify on a Linux CI runner / box.
why_human: the Windows host cannot execute a linux ELF test binary; needs a linux runner.

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps

(none — all 6 LNX requirements are code-verified against the tree; these 4 are deferred on-machine confirmations requiring a Linux+WINE+systemd host and a published release, not code gaps. A local snapshot tarball was assembled + verified to confirm packaging assembles end-to-end.)
