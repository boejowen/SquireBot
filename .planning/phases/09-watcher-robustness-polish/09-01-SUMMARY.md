---
phase: 09-watcher-robustness-polish
plan: 01
plan_id: 09-01-tray-prereadyqueue
subsystem: tray
tags: [tray, queue, ops-06, robustness, wave1]
requirements: [OPS-06]
dependency_graph:
  requires: []
  provides:
    - "Pre-Ready mutator-call queue + FIFO drain on OnReady — load-bearing primitive for Plan 09-04 (AUTH-07) boot-time tray triple"
  affects:
    - "internal/tray/tray.go Controller"
    - "internal/tray/tray_test.go"
tech_stack:
  added: []
  patterns:
    - "Queue-and-replay under existing struct mutex (no second lock introduced)"
    - "Typed-action struct (vs. closures) for testable queue contents"
    - "Caller-holds-mu contract on drainPending / applyIconHealthLocked private helpers"
key_files:
  created: []
  modified:
    - internal/tray/tray.go
    - internal/tray/tray_test.go
decisions:
  - "Typed pendingAction struct + actionKind enum (not closures) — tests can introspect kind/status/health fields without reflection"
  - "Single mutex (existing t.mu) extended to cover ready + pending — D-01 explicit constraint"
  - "drainPending requires caller to hold t.mu (documented contract); dispatches to live execution path directly, NOT through public mutators (avoids re-acquiring the lock and deadlock)"
  - "SetSpreadsheetID always writes t.spreadsheetID regardless of readiness (preserves existing reader behavior); ALSO enqueues for drain symmetry"
  - "applyIconHealthLocked extracted so live + replay paths share icon-swap logic — single source of truth for HealthGreen/HealthRed dispatch"
  - "Test-only accessors (pendingSnapshot, isReady, simulateReady) live in tray.go (same package as tests) and are package-private (lowercase) — no production caller"
metrics:
  duration: "~30 min"
  completed_date: "2026-05-12"
  tasks_completed: 3
  commits: 3
  tests_added: 5
  tests_replaced: 1
---

# Phase 9 Plan 01: Tray Pre-OnReady Queue Summary

Queue-and-replay primitive in `internal/tray/tray.go` replaces the silent-no-op pre-Ready
mutator pattern. Every public mutator (`SetStatus`, `SetIconHealth`, `ShowContinueSetup`,
`HideContinueSetup`, `ShowReauthorize`, `HideReauthorize`, `SetSpreadsheetID`) now takes
`t.mu`, branches on `t.ready`, and either enqueues a typed `pendingAction` or executes
live. `OnReady()` flips `t.ready` to true and drains the queue in FIFO insertion order
under the same critical section, before launching `go t.loop()`. No second mutex; no new
menu items; no schema change.

## What Shipped

- **Type scaffolding (Task 1, commit `3c054e9`):**
  - `type actionKind int` with `iota`-numbered constants for all 7 mutator kinds
  - `type pendingAction struct { kind actionKind; status string; health Health; spreadsheetID string }`
  - `Controller.ready bool` and `Controller.pending []pendingAction` fields added under existing `t.mu`
  - Private helpers: `drainPending()` (caller-holds-mu; switch-dispatches each queued action to the live execution path) and `applyIconHealthLocked(h Health)` (factored out so both live and replay share the icon-swap)
  - Test surface: `pendingSnapshot() []pendingAction` (returns a copy) and `isReady() bool`
  - Smoke test `TestPendingAction_Zero` confirms fresh Controller has ready=false and empty pending

- **Mutator rewrite (Task 2, commit `4f29b9d`):**
  - All 7 public mutators rewritten with the queue-or-execute shape: `t.mu.Lock()` + `defer Unlock()` + `if !t.ready { append; return }` + live op
  - `SetSpreadsheetID` keeps its existing `t.spreadsheetID = id` write (always); additionally enqueues `actSetSpreadsheetID` if !ready so the drain path stays symmetric with the other mutators
  - Replacement test `TestPreReady_EnqueuesNotDrops` asserts 8 mutator calls produce 8 queued entries (vs. the old "no panic" assertion)
  - Old `TestMutators_SafeBeforeOnReady` removed (`grep -cE 'TestMutators_SafeBeforeOnReady' internal/tray/tray_test.go` = 0)

- **OnReady wiring (Task 3, commit `79295b4`):**
  - OnReady's tail now takes `t.mu`, sets `t.ready = true`, calls `drainPending()`, releases `t.mu`, then launches `go t.loop()`
  - `simulateReady()` test-only helper mirrors OnReady's drain block exactly so offline tests exercise the same code path without `systray.Run`
  - New tests: `TestPreReady_FIFOOrder` (5-call insertion-order check; asserts each `snap[i].kind` matches expectation and that the `health`/`status` payload fields round-trip correctly), `TestSimulateReady_DrainsQueue` (drain empties queue + flips isReady), `TestPostReady_ExecutesLive` (post-ready mutators do NOT enqueue; `SetSpreadsheetID("ssid-123")` reflects through `SpreadsheetID()`)

## must_haves Truths — Verified

1. **"Calls to SetStatus / SetIconHealth / ShowContinueSetup / HideContinueSetup / ShowReauthorize / HideReauthorize / SetSpreadsheetID made BEFORE OnReady() runs are queued (NOT silently dropped) and replayed in FIFO order once OnReady() fires."**
   Verified by `TestPreReady_EnqueuesNotDrops` (8 calls → 8 queued), `TestPreReady_FIFOOrder` (5 calls preserve insertion order; payload fields preserved), and `TestSimulateReady_DrainsQueue` (drain empties queue under the same code path as OnReady).

2. **"When OnReady() runs, the controller's `ready` flag flips to true under `t.mu` and the pending queue drains in insertion order, then is emptied."**
   Verified structurally by `OnReady()` source (single critical section: Lock → `t.ready = true` → `t.drainPending()` → Unlock) and functionally by `TestSimulateReady_DrainsQueue` (post-drain `pendingSnapshot()` returns 0 entries; `isReady()` returns true). `drainPending()` sets `t.pending = nil` at the tail of the for-range loop.

3. **"After OnReady() drains, subsequent mutator calls execute live (no longer queued)."**
   Verified by `TestPostReady_ExecutesLive`: after `simulateReady()`, 4 mutator calls (`SetStatus`, `SetIconHealth`, `ShowReauthorize`, `SetSpreadsheetID`) leave the pending queue at length 0, and `SetSpreadsheetID("ssid-123")` reflects through `SpreadsheetID()`.

4. **"FIFO ordering preserves last-write-wins for paired Show*/Hide* calls (e.g. ShowReauthorize → HideReauthorize replays to net-hidden)."**
   Verified by `TestPreReady_FIFOOrder` asserting the `actShowReauthorize` at index 1 precedes `actHideReauthorize` at index 3, and the second `actStatus` ("recovered") at index 4 overwrites the first ("auth error") at index 2 during drain. `drainPending` iterates `t.pending` in slice order (insertion order), so paired Show/Hide replays exactly as the live path would.

5. **"No second mutex is introduced — the existing `t.mu` (sync.Mutex at tray.go:97) is the only lock; it now also guards `ready` and `pending`."**
   Verified by source: `grep -nE 'sync\.Mutex' internal/tray/tray.go` matches exactly 1 line (the existing `mu sync.Mutex` in the Controller struct). All 7 mutators + `OnReady` drain + `simulateReady` + `pendingSnapshot` + `isReady` take the same `t.mu`.

## Verification — All Hooks Green

| Hook | Result |
|------|--------|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./internal/tray/ -count=1 -v` | 16/16 PASS (5 new + 11 existing) |
| `go test ./...` (full repo) | all packages green |
| `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` | exactly 1 line (schema unchanged) |
| `grep -cE 'TestMutators_SafeBeforeOnReady' internal/tray/tray_test.go` | 0 (old test fully replaced) |
| `grep -nE 'type pendingAction struct' internal/tray/tray.go` | exactly 1 line |
| `grep -nE 'func \(t \*Controller\) drainPending\(\)' internal/tray/tray.go` | exactly 1 line |
| `grep -cE 't\.ready = true' internal/tray/tray.go` | 2 (OnReady + simulateReady) |
| `grep -cE 't\.drainPending\(\)' internal/tray/tray.go` | 2 (OnReady + simulateReady) |
| `grep -nE 'kind: act(Status\|IconHealth\|ShowContinueSetup\|HideContinueSetup\|ShowReauthorize\|HideReauthorize\|SetSpreadsheetID)' internal/tray/tray.go` | 7 distinct lines (one per mutator kind) |
| `grep -cE 'if !t\.ready' internal/tray/tray.go` | 7 (one per public mutator) |

## Schema Impact

**NONE.** `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` is unchanged (still
exactly 1 matching grep line). No `_meta` rows added, no tab columns added, no
`apps-script/` source touched. CONTEXT.md D-06 schema-impact assertion holds.

## Test Count Delta

| Category | Count | Notes |
|----------|-------|-------|
| New tests | 5 | `TestPendingAction_Zero`, `TestPreReady_EnqueuesNotDrops`, `TestPreReady_FIFOOrder`, `TestSimulateReady_DrainsQueue`, `TestPostReady_ExecutesLive` |
| Replaced tests | 1 | `TestMutators_SafeBeforeOnReady` (no-panic-only) → `TestPreReady_EnqueuesNotDrops` (queue-length assertion) |
| Existing tests retained | 10 | All other tray tests untouched and green (including `TestSetSpreadsheetID_Mutates`, `TestShowHideReauthorize_SafeBeforeOnReady`, all `TestMenuPlan_*`, both callback-wiring tests) |

## Deviations from Plan

None. Plan executed exactly as written. The plan was unusually detailed (full source
snippets for every insertion) and lined up cleanly with the existing code shape.

## Hand-off Note for Plan 09-04 (AUTH-07)

**The boot-fast-fail tray triple (`SetIconHealth(Red)` + `SetStatus(...)` + `ShowReauthorize`)
is now safe to call before `OnReady`.** Pre-Ready calls land in `t.pending` in insertion
order and replay against the live menu items the moment `OnReady` finishes building them.
The fix in Plan 09-04's `runapp.go` `buildTokenSourceFromWincred` error branch can fire
all three calls without any awareness of menu lifecycle — the queue makes the dark window
invisible to the caller (and, by D-01's design, invisible to the guildie too).

Specifically:

- Plan 09-04 can call `t.SetIconHealth(tray.HealthRed)` immediately on classifying
  `auth.IsRevokedRefreshToken(err)` — it enqueues an `actIconHealth` entry.
- `t.SetStatus("Auth error — sign in again")` (or whatever exact string Plan 09-04 picks
  per CONTEXT.md Claude's-Discretion §3) enqueues an `actStatus` entry.
- `t.ShowReauthorize()` enqueues an `actShowReauthorize` entry.
- When `OnReady` arrives, all three actions drain in order: red icon swap, status label
  update, Reauthorize menu item shown. From the guildie's perspective the tray opens
  already in the auth-error state with a clickable Reauthorize — no "Initialising…"
  stranded state. This is the load-bearing dependency CONTEXT.md D-05 enforces via the
  wave ordering (09-01 Wave 1, 09-04 Wave 2).

## Commits

| Hash | Task | Files | Summary |
|------|------|-------|---------|
| `3c054e9` | Task 1 | tray.go (+90), tray_test.go (+10) | pendingAction scaffolding: actionKind enum, struct, ready/pending fields, drainPending, applyIconHealthLocked, test accessors, smoke test |
| `4f29b9d` | Task 2 | tray.go (+44/-22), tray_test.go (+19/-5) | All 7 public mutators converted to queue-or-execute; replaced TestMutators_SafeBeforeOnReady with TestPreReady_EnqueuesNotDrops |
| `79295b4` | Task 3 | tray.go (+22), tray_test.go (+75) | OnReady drains queue under t.mu; simulateReady helper; 3 new tests (FIFO order, drain semantics, post-ready live execution) |

## Self-Check: PASSED

- `internal/tray/tray.go`: FOUND (modified, ~420 lines)
- `internal/tray/tray_test.go`: FOUND (modified, ~360 lines)
- `.planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md`: FOUND (this file)
- Commit `3c054e9`: FOUND in git log
- Commit `4f29b9d`: FOUND in git log
- Commit `79295b4`: FOUND in git log
- Schema constant `WatcherMaxSchemaVersion = 3`: UNCHANGED
