# Phase 20 — Deferred Items (out-of-scope discoveries)

## Pre-existing gofmt drift (NOT introduced by 20-01)

Discovered during 20-01 verification (`gofmt -l internal/backendsrv/store/`):

- `internal/backendsrv/store/itemids_test.go` — gofmt-dirty, unmodified by this plan.
- `internal/backendsrv/store/readviews_test.go` — gofmt-dirty, unmodified by this plan.

Both are unchanged in git status (not touched by 20-01). Out of scope per the
SCOPE BOUNDARY rule (only auto-fix issues directly caused by the current task's
changes). All 20-01 files are gofmt-clean. A future `gofmt -w` housekeeping pass
(or a `/gsd-quick`) should reformat these two pre-existing files.
