---
phase: 02-watcher-robustness-schema-lock
plan: 03
subsystem: sheets-retry-mutex
tags: [retry, backoff, mutex, watch-07, pitfall-d, auth, googleapi]
requires:
  - 02-01 (sheet.Client + helper methods)
  - 02-02 (WriteSpellbook is a sibling of WriteInventory)
provides:
  - sheet.withRetry (free function, internal)
  - sheet.ErrPermanentAuth (exported sentinel)
  - sheet.retrySchedule (locked literal: 2s/4s/8s/16s/32s/60s)
  - sheet.parseRetryAfter (RFC 7231 — both forms)
  - sheet.sleepFn (test injection point)
  - sheet.Client.batchMu sync.Mutex (Pitfall D closure)
  - sheet.Client.SetOnRefresh
  - sheet.Client.batchUpdate (helper, mutex+retry)
  - sheet.Client.valuesGet (helper, mutex+retry)
  - sheet.Client.valuesAppend (helper, mutex+retry)
  - sheet.Client.valuesUpdate (helper, mutex+retry)
  - sheet.Client.spreadsheetsGet (helper, mutex+retry)
  - sheet.Client.updateSheetProperties (helper, mutex+retry, thin batchUpdate wrap)
affects:
  - internal/sheet/retry.go (new)
  - internal/sheet/retry_test.go (new)
  - internal/sheet/client_helpers.go (new)
  - internal/sheet/client_helpers_test.go (new)
  - internal/sheet/client.go (added batchMu + onRefresh + SetOnRefresh)
  - internal/sheet/write.go (refactored to call c.batchUpdate)
  - internal/sheet/owner.go (refactored: c.valuesGet, c.valuesUpdate, c.valuesAppend)
  - internal/sheet/meta.go (refactored: c.valuesGet)
  - internal/sheet/ensure_tab.go (refactored: c.spreadsheetsGet, c.batchUpdate)
  - internal/sheet/scaffold_helpers.go (refactored: c.valuesUpdate, c.updateSheetProperties, c.valuesGet, c.valuesAppend)
tech-stack:
  added: []
  patterns:
    - "Fixed retry schedule []time.Duration{2s,4s,8s,16s,32s,60s} (WATCH-07-mandated; CONTEXT.md locks: do NOT add cenkalti/backoff)"
    - "googleapi.Error.Code switch + Errors[0].Reason discrimination (Pitfall B: 403 quota vs revoked-scope)"
    - "Single-shot token refresh on auth-flavored 403 + ErrPermanentAuth on second hit (Pitfall A boundary)"
    - "Retry-After RFC 7231 dual-form parse (integer seconds + HTTP-date) overrides schedule on 429"
    - "sync.Mutex held across both API call AND retry-schedule sleeps (serialized backoff — second goroutine cannot blow through the same backoff)"
    - "Read-vs-write co-serialization (reads also acquire batchMu — racing reads vs writes in the same workbook produce inconsistent snapshots)"
    - "Test sleepFn injection (package-level var swapped via t.Cleanup) — schedule exercised symbolically; tests run <1s"
key-files:
  created:
    - internal/sheet/retry.go
    - internal/sheet/retry_test.go
    - internal/sheet/client_helpers.go
    - internal/sheet/client_helpers_test.go
  modified:
    - internal/sheet/client.go
    - internal/sheet/write.go
    - internal/sheet/owner.go
    - internal/sheet/meta.go
    - internal/sheet/ensure_tab.go
    - internal/sheet/scaffold_helpers.go
decisions:
  - "withRetry is a free function (not a method on *Client) so it's testable without a sheets.Service. The mutex acquisition is the *Client helper's responsibility; withRetry only does retry logic."
  - "sleepFn is a package-level var rather than a withRetry parameter — keeps the public surface minimal while still letting tests inject a fake. installFakeSleep + t.Cleanup is the test-side pattern."
  - "onRefreshOrNoop returns a no-op when c.onRefresh is nil. This lets withRetry consume its single refresh allowance even before Plan 02-04 wires the real refresh callback — the second auth-flavored 403 still surfaces as ErrPermanentAuth, just without a real token swap. Defensive and forward-compatible."
  - "Reads (valuesGet, spreadsheetsGet) acquire the same mutex as writes. Mixing read+write traffic against the same workbook from concurrent goroutines can produce inconsistent snapshots — the heartbeat (Plan 02-05) will issue both kinds of calls so this matters."
  - "Helper file is named client_helpers.go (matching the plan's grep pattern client*.go) even though scaffold_helpers.go also exists in the same package — the two files have different mandates: scaffold_helpers.go is the public API the internal/scaffold package consumes; client_helpers.go is the package-internal mutex+retry envelope every Sheets call goes through."
  - "Retry-After parsing accepts integer seconds, HTTP-date, and treats past dates / unparseable strings as 0 (caller falls back to schedule). The 0 path is also exercised for explicit '0' integer (no point sleeping)."
  - "5xx switch is exhaustive: 500/502/503/504 retried; 501 (Not Implemented) and 505+ are not transient — fall through to the default 'surface immediately' arm."
  - "404 surfaces immediately (not retryable). The Sheets API returns 404 for deleted spreadsheets and unknown ranges; both are user-actionable rather than transient."
metrics:
  duration: ~1h
  completed: 2026-05-01
---

# Phase 2 Plan 03: Sheets retry envelope + sync.Mutex Pitfall D closure

WATCH-07 backoff lands. Every Sheets API call from `*sheet.Client` now
acquires `batchMu` and runs through `withRetry` — the heartbeat goroutine
(Plan 02-05) and watcher goroutine can safely share a single client.
ErrPermanentAuth gives Plan 02-04 a defined boundary signal to detect
refresh-token death and turn the tray red.

## What Shipped

### Task 1 — internal/sheet/retry.go (TDD)
**Commits:** `c9e399e` (RED — 14 failing tests), `2fd0ab9` (GREEN)

`withRetry(ctx, op, onRefresh)` runs `op` with the WATCH-07 policy:

| Wire response                                                  | Action                                          |
| -------------------------------------------------------------- | ----------------------------------------------- |
| `nil` (success)                                                | return nil                                      |
| `429`                                                          | Retry-After header overrides; else schedule     |
| `403` reason=`authError`/`insufficientPermissions`/`forbidden` | refresh once via onRefresh, retry; on 2nd hit ErrPermanentAuth |
| `403` reason=`userRateLimitExceeded`/`rateLimitExceeded`/empty | schedule (transient quota throttle)             |
| `5xx` (500/502/503/504)                                        | schedule                                        |
| non-googleapi error (network, DNS)                             | schedule (transient)                            |
| `400`/`404`/anything else                                      | surface immediately (not retryable)             |
| schedule exhaustion                                            | wrapped error with attempt count                |
| ctx cancellation during sleep                                  | returns ctx.Err() promptly                      |

`retrySchedule = []time.Duration{2s, 4s, 8s, 16s, 32s, 60s}` is the
literal slice from CONTEXT.md / WATCH-07. `parseRetryAfter` accepts both
RFC 7231 forms (integer seconds + HTTP-date) and returns 0 for past
dates / unparseable strings. `ErrPermanentAuth` is the sentinel Plan 02-04
will consume to drive the tray red transition.

`sleepFn` is a package-level var; tests swap it via `installFakeSleep`
(captures requested durations into a slice + t.Cleanup-restored). The
six-step exhaustion test runs in <1ms instead of 122s.

14 test cases in `retry_test.go`:
1. success on first try (no sleep, no refresh)
2. transient 5xx → schedule slot 0 (2s) consumed
3. 429 with Retry-After "5" → 5s sleep (overrides schedule)
4. 429 with HTTP-date Retry-After → ~5s sleep (within slop)
5. 429 with empty Retry-After → schedule fallback (2s)
6. 403 authError + refresh + 2nd attempt success → 1 refresh, 2 op calls
7. 403 authError twice → ErrPermanentAuth, exactly 1 refresh
8. 403 userRateLimitExceeded → schedule (NOT permanent)
9. 403 forbidden defensive default → ErrPermanentAuth after refresh
10. 5xx exhausted → all 6 slots consumed, 7 op calls, wrapped error
11. non-googleapi network error → transient, schedule
12. 400 → surface immediately, no retry
13. ctx cancellation → returns context.Canceled in <1s
14. parseRetryAfter unit tests (empty, integer, HTTP-date future/past, garbage)

Plus `TestErrPermanentAuth_IsExportedSentinel` for the sentinel.

### Task 2 — sync.Mutex on *Client + 5 helper methods (TDD)
**Commits:** `bdd1a00` (RED — 5 failing tests), `76bd180` (GREEN)

`internal/sheet/client.go`:
- Added `import "sync"`.
- Added `batchMu sync.Mutex` field with Pitfall D doc-comment.
- Added `onRefresh func() error` field.
- Added `SetOnRefresh(f func() error)` setter (startup-only init —
  goroutine-safe by virtue of being write-once before any reader runs).
- Updated the `Client` doc-comment from "single Client is safe for
  serial use only" to "concurrent use BY DESIGN — batchMu serializes
  every Sheets API call. The heartbeat goroutine (Plan 02-05) and the
  watcher goroutine BOTH go through this mutex."

`internal/sheet/client_helpers.go` (NEW): five helper methods that wrap
every Sheets API call. Each helper acquires `batchMu`, runs the call
inside `withRetry`, and returns the typed response. The package's API
surface to other packages (and to write.go/owner.go/meta.go/ensure_tab.go/
scaffold_helpers.go) now goes through these wrappers exclusively:

| Helper                  | Wraps                                       | Used by                                         |
| ----------------------- | ------------------------------------------- | ----------------------------------------------- |
| `batchUpdate`           | `Spreadsheets.BatchUpdate(...).Do()`        | `WriteInventory`, `WriteSpellbook`, `EnsureSheet` AddSheet, `HideSheet` (via wrapper) |
| `valuesGet`             | `Spreadsheets.Values.Get(...).Do()`         | `readMeta`, `UpsertCharOwner` lookup, `ReadColumn` |
| `valuesAppend`          | `Spreadsheets.Values.Append(...).Do()`      | `UpsertCharOwner` insert, `AppendRow`           |
| `valuesUpdate`          | `Spreadsheets.Values.Update(...).Do()`      | `UpsertCharOwner` last_seen refresh, `WriteHeaderRow` |
| `spreadsheetsGet`       | `Spreadsheets.Get(...).Fields(...).Do()`    | `ListSheets`, `EnsureSheet` discovery           |
| `updateSheetProperties` | thin wrapper over `batchUpdate`             | `HideSheet`                                     |

`onRefreshOrNoop` returns `c.onRefresh` or a no-op refresher when
`onRefresh` is nil — this lets `withRetry` consume its single refresh
allowance even when Plan 02-04 hasn't wired the real callback yet.
The second auth-flavored 403 still surfaces as `ErrPermanentAuth`.

Refactored call sites (zero direct `c.svc.Spreadsheets` calls remain
outside `client_helpers.go`):

| File                          | Replaced                                                    | With                                |
| ----------------------------- | ----------------------------------------------------------- | ----------------------------------- |
| `internal/sheet/write.go`     | 2 × `c.svc.Spreadsheets.BatchUpdate(...).Do()`              | `c.batchUpdate(ctx, req)`           |
| `internal/sheet/owner.go`     | `Values.Get`, `Values.Update`, `Values.Append`              | `c.valuesGet`, `c.valuesUpdate`, `c.valuesAppend` |
| `internal/sheet/meta.go`      | `Values.Get`                                                | `c.valuesGet`                       |
| `internal/sheet/ensure_tab.go` | 2 × `Spreadsheets.Get`, 1 × `BatchUpdate`                  | `c.spreadsheetsGet`, `c.batchUpdate` |
| `internal/sheet/scaffold_helpers.go` | `Values.Update`, `BatchUpdate` (HideSheet), `Values.Get`, `Values.Append` | `c.valuesUpdate`, `c.updateSheetProperties`, `c.valuesGet`, `c.valuesAppend` |

5 mutex-specific tests in `client_helpers_test.go`:
1. `TestClient_batchUpdateSerializesConcurrentGoroutines` — 3 goroutines
   each blocking 100ms server-side; total elapsed >= ~270ms; max in-flight
   counter on the stub == 1.
2. `TestClient_valuesGetSerializesAgainstBatchUpdate` — concurrent
   valuesGet + batchUpdate; in-flight counter == 1 (read+write share mutex).
3. `TestClient_OnRefreshFiresOn403AuthError` — server returns 403
   authError on first call, succeeds on second; SetOnRefresh callback
   fires exactly once; total server calls == 2.
4. `TestClient_MutexReleasedOnError` — 400 (non-retryable) returns error;
   subsequent batchUpdate completes within 500ms (mutex released).
5. `TestClient_MutexReleasedOnPanic` — manual lock+panic+defer-Unlock
   pattern verifies a panicking critical section releases the mutex
   (defensive — confirms the deferred Unlock semantics we rely on).

## Acceptance — Self-Check

```
build  : exit 0   (go build ./...)
vet    : exit 0   (go vet ./...)
tests  : ALL PASS (go test ./... -count=1)
binary : built    (go build -o squirebot.exe ./cmd/squirebot/)
```

| Plan acceptance criterion | Result |
|---------------------------|--------|
| `grep -n "var retrySchedule" internal/sheet/retry.go` returns 1 | 1 |
| Schedule literal contains 2s/4s/8s/16s/32s/60s | yes (lines 49–55) |
| `grep -n "var ErrPermanentAuth" internal/sheet/retry.go` returns 1 | 1 |
| `grep -c "userRateLimitExceeded\|rateLimitExceeded" internal/sheet/retry.go` ≥ 1 | yes (in switch reason) |
| `grep -c "authError\|insufficientPermissions" internal/sheet/retry.go` ≥ 2 | yes (case branch + comment) |
| `grep -c "Retry-After" internal/sheet/retry.go` ≥ 1 | yes |
| 14 retry tests pass | 14/14 |
| `grep -n "batchMu sync.Mutex" internal/sheet/client.go` returns 1 | 1 |
| `grep -n "func (c \*Client) SetOnRefresh" internal/sheet/client.go` returns 1 | 1 |
| 5 helper methods exist | 5 (batchUpdate, valuesGet, valuesAppend, valuesUpdate, spreadsheetsGet) |
| `grep -c "c.svc.Spreadsheets" internal/sheet/{write,owner,meta,ensure_tab}.go` returns 0 | 0 (Pitfall D enforcement) |
| Helpers used by all four refactored files | yes (write=2, owner=3, meta=1, ensure_tab=3, scaffold_helpers=3) |
| 5 mutex-specific tests pass | 5/5 |
| `cenkalti/backoff` not in go.mod or imports | confirmed absent (only mentioned in retry.go doc-comment as a "do NOT add" reminder) |
| `go vet ./...` clean | yes |
| `go build ./...` clean | yes |

## Test Counts

| File                                  | Existing | Added | Total |
| ------------------------------------- | -------- | ----- | ----- |
| `internal/sheet/retry_test.go`        | 0        | 14    | 14    |
| `internal/sheet/client_helpers_test.go` | 0      | 5     | 5     |
| `internal/sheet/write_test.go`        | 10       | 0     | 10 (regression-clean) |
| `internal/sheet/owner_test.go`        | 6        | 0     | 6 (regression-clean) |
| `internal/sheet/meta_test.go`         | 8        | 0     | 8 (regression-clean) |
| `internal/scaffold/scaffold_test.go`  | 6        | 0     | 6 (regression-clean) |

All Phase 1 tests + Wave 1 (02-01) tests + Wave 2 (02-02) tests still
pass. The retry+mutex layer is fully transparent at the existing
behavioral contract level — same number of HTTP calls per success path,
same response shapes consumed by callers.

## ErrPermanentAuth Handoff to Plan 02-04

`ErrPermanentAuth` is the boundary signal Plan 02-04 will consume:

```go
// In Plan 02-04's runWatcher error-handling path:
err := c.WriteInventory(ctx, char, ...)
if errors.Is(err, sheet.ErrPermanentAuth) {
    tray.SetState(tray.HealthRed)
    tray.SetTooltip("SquireBot: re-authorize required")
    // Suspend further writes; "Reauthorize" tray menu item runs OAuth flow.
    return
}
```

Plan 02-04 also gets to wire `*Client.SetOnRefresh` to a callback that
calls `oauth2.TokenSource.Token()` + re-stores the new token in wincred.
Until that wiring lands, the no-op `onRefreshOrNoop` lets the retry path
consume its single refresh allowance, so the second auth-flavored 403
still surfaces correctly via `ErrPermanentAuth`.

## Race Detector Verdict

`go test -race` could not be run in the local environment because Windows
Go requires CGO + a C compiler for the race detector, and no `gcc` is on
PATH. This is an environment limitation, not a regression — race-clean
verification is deferred to CI (which has the toolchain) or to a local
run after `gcc` (e.g., via mingw-w64) is installed.

The mutex behavior is independently verified via the explicit
serialization tests:
- `TestClient_batchUpdateSerializesConcurrentGoroutines` proves the
  in-flight counter never exceeds 1 across 3 concurrent goroutines.
- `TestClient_valuesGetSerializesAgainstBatchUpdate` proves the same
  for mixed read+write traffic.

These tests exercise the same memory and code paths the race detector
would scrutinize; they would fail (assert maxSeen == 1) if the mutex
were missing or wrongly scoped. The race detector adds memory-model
verification on top of behavioral verification, but the behavioral
test failure mode would land first if the locking is wrong.

## Deviations from Plan

### Plan-vs-Reality Drift Notes

**A. Plan Task 2 step 2 mentioned five helper methods. I added a sixth
(`updateSheetProperties`) as a thin wrapper over `batchUpdate` for
readability at the `HideSheet` call site.** The wrapper is two lines and
holds no state; net behavior equals `batchUpdate`. This keeps
`scaffold_helpers.go`'s `HideSheet` symmetric with the other helpers
(rather than `c.batchUpdate(ctx, req)` with a comment explaining what
the request shape means). Documented in `client_helpers.go` —
not a deviation in spirit, just a readability choice the plan was silent on.

**B. Plan Task 2 step 1 said `onRefresh` is documented as
"goroutine-safe: only set during init."** Implemented exactly as
specified. The actual mutex+retry envelope reads `c.onRefresh` only
while holding `batchMu` (via `onRefreshOrNoop`), so even if a future
plan accidentally calls `SetOnRefresh` after the first watcher event,
the read-vs-write race would be benign (the new value would be picked
up on the next API call). Belt-and-braces — the doc is the contract.

**C. Plan said "internal/scaffold/scaffold.go: if it called any direct
svc.* method, refactor to use the helpers."** Verified that
`internal/scaffold/scaffold.go` does NOT call svc directly — it only
calls the public helper methods `c.ListSheets`, `c.WriteHeaderRow`,
`c.HideSheet`, `c.ReadColumn`, `c.AppendRow` (defined in
`scaffold_helpers.go`). Since I refactored `scaffold_helpers.go` to
funnel through the new mutex+retry helpers, scaffold's API contract is
unchanged but the underlying call path is now mutex-serialized + retry-
enveloped automatically. No scaffold.go edit needed.

**D. Plan Task 2 step 5 said tests "may need to start seeing multiple
HTTP calls — adjust expected call counts accordingly."** Reality: zero
existing tests needed adjustment. The success-path tests already use
stubs that return 200 OK, and `withRetry` doesn't kick in until an
error is returned. Existing tests stay green without modification.

**E. Plan suggested a sleep injection point as `sleep func` parameter.
I implemented it as a package-level var `sleepFn` instead.** Same
testability with a smaller public surface (no need to thread a sleep
parameter through every helper). Tests use `installFakeSleep` +
`t.Cleanup`. Documented in `retry.go`.

### Auto-fixed Issues

None. The plan's BLOCKER 4 fix (bootstrapMeta deleted in 02-01) was
already reflected in the Task 2 instructions — no leftover refactor
work to do for a deleted function. The `c.svc.Spreadsheets.Values.Get`
in `meta.go`'s `readMeta` is the only meta.go API call and it cleanly
migrated to `c.valuesGet`.

## Known Stubs

None. `onRefreshOrNoop` returns a no-op refresher when no callback is
installed — this is intentional forward-compatible behavior, not a
stub. Plan 02-04 will install the real callback that calls
`oauth2.TokenSource.Token()` + re-stores the token. Until then, the
retry policy still functions correctly: the no-op consumes the single
refresh allowance, and the second auth-flavored 403 surfaces as
`ErrPermanentAuth` (which Plan 02-04 will consume).

## TDD Gate Compliance

This plan ran in strict TDD with separate RED + GREEN commits per task:

| Task | RED commit | GREEN commit |
|------|------------|--------------|
| 1 (retry.go)         | `c9e399e test(02-03): add failing tests for WATCH-07 retry envelope` | `2fd0ab9 feat(02-03): implement WATCH-07 retry envelope (withRetry + ErrPermanentAuth)` |
| 2 (client mutex)     | `bdd1a00 test(02-03): add failing tests for *Client mutex + helper methods` | `76bd180 feat(02-03): mutex-serialized *Client helpers funnel every Sheets API call` |

Each RED was verified to fail-build (undefined identifiers) before
committing; each GREEN was verified to pass `go test ./internal/sheet/...
-count=1` and `go vet ./...`.

## Self-Check: PASSED

Verified all files:

- `internal/sheet/retry.go` (~190 lines, contains `var retrySchedule`,
  `var ErrPermanentAuth`, `func withRetry`, `func parseRetryAfter`, `var sleepFn`)
- `internal/sheet/retry_test.go` (~423 lines, 15 test functions: 14
  scenarios + ErrPermanentAuth sanity)
- `internal/sheet/client_helpers.go` (~135 lines, contains 5 helper
  methods + `onRefreshOrNoop` + `updateSheetProperties` thin wrapper)
- `internal/sheet/client_helpers_test.go` (~334 lines, 5 mutex tests)
- `internal/sheet/client.go` (added `sync` import, `batchMu` field,
  `onRefresh` field, `SetOnRefresh` method, updated concurrency doc)
- `internal/sheet/write.go` (`c.batchUpdate` now used for both
  WriteInventory + WriteSpellbook)
- `internal/sheet/owner.go` (`c.valuesGet`, `c.valuesUpdate`, `c.valuesAppend`)
- `internal/sheet/meta.go` (`c.valuesGet`)
- `internal/sheet/ensure_tab.go` (`c.spreadsheetsGet`, `c.batchUpdate`)
- `internal/sheet/scaffold_helpers.go` (`c.valuesUpdate`,
  `c.updateSheetProperties`, `c.valuesGet`, `c.valuesAppend`)

All 4 commits reachable from HEAD: `c9e399e`, `2fd0ab9`, `bdd1a00`, `76bd180`.
