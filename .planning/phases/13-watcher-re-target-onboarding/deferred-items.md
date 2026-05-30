# Phase 13 — Deferred Items

Out-of-scope discoveries logged during execution (NOT fixed — they were not
caused by the current task's changes; SCOPE BOUNDARY rule).

| Discovered | Plan | Item | Why deferred |
|------------|------|------|--------------|
| 2026-05-30 | 13-01 | `cmd/squirebot-server/main.go` `const defaultAddr` line is not `gofmt -l` clean (a one-extra-space comment-alignment nit at lines ~50-51). Verified present in commit `aabf9b8` (pre-Phase-13). | Pre-existing formatting nit in the `const` block, unrelated to 13-01's route-registration edit lower in the file. Out of scope per SCOPE BOUNDARY. A future watcher-polish plan (13-04 already carries gofmt nits 999.20/21) or a `gofmt -w` sweep can absorb it. |
