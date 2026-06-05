# Phase 19 — Deferred Items

Out-of-scope discoveries logged during execution (not fixed — SCOPE BOUNDARY rule).

## 19-01

- **gofmt-dirty pre-existing files (out of scope):** `gofmt -l internal/backendsrv/store`
  flags `internal/backendsrv/store/itemids_test.go` and
  `internal/backendsrv/store/readviews_test.go`. Both were last modified in Phase 14
  (commit `0dc3b35`), long before this plan, and are unrelated to 19-01's changes. All
  four files this plan created/modified are gofmt-clean. Left untouched per the executor
  SCOPE BOUNDARY rule (only auto-fix issues directly caused by the current task).
