---
phase: 09-watcher-robustness-polish
plan: 01
plan_id: 09-01-tray-prereadyqueue
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/tray/tray.go
  - internal/tray/tray_test.go
autonomous: true
requirements: [OPS-06]
tags: [tray, queue, ops-06, robustness, wave1]

must_haves:
  truths:
    - "Calls to SetStatus / SetIconHealth / ShowContinueSetup / HideContinueSetup / ShowReauthorize / HideReauthorize / SetSpreadsheetID made BEFORE OnReady() runs are queued (NOT silently dropped) and replayed in FIFO order once OnReady() fires."
    - "When OnReady() runs, the controller's `ready` flag flips to true under `t.mu` and the pending queue drains in insertion order, then is emptied."
    - "After OnReady() drains, subsequent mutator calls execute live (no longer queued)."
    - "FIFO ordering preserves last-write-wins for paired Show*/Hide* calls (e.g. ShowReauthorize → HideReauthorize replays to net-hidden)."
    - "No second mutex is introduced — the existing `t.mu` (sync.Mutex at tray.go:97) is the only lock; it now also guards `ready` and `pending`."
  artifacts:
    - path: internal/tray/tray.go
      provides: "Controller struct with `ready bool` and `pending []pendingAction` fields; pendingAction typed-action struct; queue-or-execute under t.mu inside every public mutator; drainPending() called from OnReady"
      contains: "pendingAction"
    - path: internal/tray/tray_test.go
      provides: "New tests: TestPreReady_Enqueues, TestPreReady_FIFOOrder, TestPreReady_DrainReplaysInOrder, TestPostReady_ExecutesLive; existing TestMutators_SafeBeforeOnReady remains green"
      contains: "TestPreReady_FIFOOrder"
  key_links:
    - from: "internal/tray/tray.go Controller.SetStatus / SetIconHealth / Show*/Hide* mutators"
      to: "internal/tray/tray.go Controller.drainPending (called from OnReady)"
      via: "t.mu-guarded enqueue when !t.ready"
      pattern: "if !t\\.ready"
    - from: "internal/tray/tray.go OnReady"
      to: "internal/tray/tray.go drainPending"
      via: "drain after menu construction, before go t.loop()"
      pattern: "drainPending|t\\.ready = true"
---

<objective>
Replace the tray controller's silent-no-op-before-OnReady pattern with a queue-and-replay pattern. Today every public mutator (`SetStatus`, `SetIconHealth`, `ShowContinueSetup`, `HideContinueSetup`, `ShowReauthorize`, `HideReauthorize`, `SetSpreadsheetID`) does `if t.mWhatever != nil { ... }`, which silently no-ops before `OnReady()` builds the menu items. This produces the Phase 6 UAT Finding D foot-gun: when `RunApp` fast-fails before Ready (wincred-rebuild failure, or any other pre-Ready abort), the tray opens stuck at "Initialising…" with no working menu.

Per CONTEXT.md D-01 (locked under the "invisible UX" tiebreaker), this plan implements **option (a) queue-and-replay** in `internal/tray/tray.go`. A `pending []pendingAction` slice and a `ready bool` are added under the existing `t.mu`. Every public mutator becomes: under `t.mu`, if `!t.ready` enqueue a typed `pendingAction{kind, payload}` and return; else execute live. `OnReady()` flips `t.ready` to `true` after menu construction and drains the queue in FIFO order under `t.mu`.

This plan is load-bearing for Plan 09-04 (AUTH-07). Without this queue, AUTH-07's boot-time tray triple lands in the silent-no-op window and the AUTH-07 fix does not work. CONTEXT.md D-05 explicitly orders 09-01 in Wave 1 and 09-04 in Wave 2 for this reason.

**Scope discipline (per CONTEXT.md domain section + D-07):**
- No new tray menu items.
- No second mutex.
- No refactoring beyond what the queue requires.
- No schema changes (D-06): `WatcherMaxSchemaVersion = 3` in `internal/sheet/client.go` MUST remain unchanged.
- Typed action struct (not closures) — easier to assert on in tests; pattern map confirms tests need to inspect queue contents.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/09-watcher-robustness-polish/09-CONTEXT.md
@.planning/phases/09-watcher-robustness-polish/09-PATTERNS.md
@CLAUDE.md
@internal/tray/tray.go
@internal/tray/tray_test.go

<interfaces>
<!-- Key Controller surface the executor will modify. Extracted from internal/tray/tray.go. -->
<!-- Do NOT re-read the file to discover these; they are the contract. -->

From internal/tray/tray.go:

```go
// Existing struct (lines 96-117). Add `ready bool` and `pending []pendingAction` under the existing t.mu.
type Controller struct {
    mu            sync.Mutex
    iconGreen     []byte
    iconRed       []byte
    logDir        string
    spreadsheetID string

    mStatus         *systray.MenuItem
    mWorkbook       *systray.MenuItem
    mLogs           *systray.MenuItem
    mCheckUpdates   *systray.MenuItem
    mChangeWorkbook *systray.MenuItem
    mContinueSetup  *systray.MenuItem
    mReauthorize    *systray.MenuItem
    mQuit           *systray.MenuItem

    onContinueSetup  func()
    onChangeWorkbook func()
    onCheckUpdates   func()
    onReauthorize    func()
    onQuit           func()
}

// Existing OnReady (lines 139-162). After menu construction, BEFORE `go t.loop()`,
// flip t.ready = true under t.mu and drain pending FIFO.
func (t *Controller) OnReady() { /* ... builds menu items ... go t.loop() */ }

// Existing mutators (lines 245-297). Each currently shape:
//   if t.mWhatever != nil { live op }
// Replace with:
//   t.mu.Lock(); defer t.mu.Unlock()
//   if !t.ready { t.pending = append(t.pending, pendingAction{...}); return }
//   live op
```

Existing mutator method names (DO NOT rename or add new ones):
- `SetStatus(s string)`
- `SetIconHealth(h Health)`
- `ShowContinueSetup()`
- `HideContinueSetup()`
- `ShowReauthorize()`
- `HideReauthorize()`
- `SetSpreadsheetID(id string)` — already mutex-guarded (lines 302-313); extend with ready/pending pattern in same shape

Existing test infrastructure (tray_test.go):
- `NewController(tray.Config{})` constructs an offline controller (no systray.Run needed)
- `TestMutators_SafeBeforeOnReady` (lines 49-65) — already verifies no-panic; new tests REPLACE its semantics with "queued, not dropped"
- `TestOnReauthorizeCallback_Wired` (lines 143-155) — pattern for callback wiring tests

Existing Health enum (lines 71-77):
```go
type Health int
const ( HealthGreen Health = iota; HealthRed )
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add pendingAction type, ready/pending fields, and drainPending() helper to Controller</name>
  <files>internal/tray/tray.go, internal/tray/tray_test.go</files>
  <read_first>
    - internal/tray/tray.go (full file — 317 LOC; you MUST read it entirely before editing)
    - internal/tray/tray_test.go (full file — read for existing test patterns; `withTempConfig` analogs)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Plan 09-01" (analog: tray.go:244-297 mutator pattern; tray.go:302-313 SpreadsheetID mutex pattern; tray.go:96-117 Controller struct)
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-01 (queue-and-replay decision rationale)
  </read_first>
  <behavior>
    - pendingAction is a typed struct: `type pendingAction struct { kind actionKind; status string; health Health; spreadsheetID string }`.
    - actionKind is an iota'd enum: `actStatus`, `actIconHealth`, `actShowContinueSetup`, `actHideContinueSetup`, `actShowReauthorize`, `actHideReauthorize`, `actSetSpreadsheetID`.
    - Controller gains two new fields under the existing `t.mu`: `ready bool` and `pending []pendingAction`.
    - `drainPending()` is a private method that REQUIRES `t.mu` to be held by caller. It iterates `t.pending` in slice order, dispatches each `pendingAction.kind` to the live-execution path (NOT back through the public mutator — that would re-take the mutex), then sets `t.pending = nil`.
    - Tests assert `pendingAction` field values directly (kind, status string, health enum). Closures would not allow this introspection.
  </behavior>
  <action>
    Open `internal/tray/tray.go` and make the following exact additions:

    1. After the `Health` const block (line 77) and BEFORE the `Config` struct (line 79), add:

       ```go
       // actionKind tags a deferred mutator call queued before OnReady. Plan 09-01 (OPS-06).
       type actionKind int

       const (
           actStatus actionKind = iota
           actIconHealth
           actShowContinueSetup
           actHideContinueSetup
           actShowReauthorize
           actHideReauthorize
           actSetSpreadsheetID
       )

       // pendingAction is a single deferred mutator call. Only one payload field
       // is meaningful per kind (the others stay zero-valued). Plan 09-01.
       type pendingAction struct {
           kind          actionKind
           status        string // actStatus
           health        Health // actIconHealth
           spreadsheetID string // actSetSpreadsheetID
       }
       ```

    2. Extend the `Controller` struct (line 96-117) by inserting two new fields directly after `spreadsheetID string` (line 101), still inside the same `mu`-guarded group:

       ```go
       // OPS-06 / Plan 09-01: queue-and-replay so pre-OnReady mutator calls
       // are not silently dropped. Both fields are guarded by t.mu (above).
       ready   bool
       pending []pendingAction
       ```

    3. Add a private `drainPending()` method directly after `NewController` (after line 133). It assumes `t.mu` is held by the caller:

       ```go
       // drainPending replays every queued mutator call against the now-live
       // menu items, in FIFO insertion order. The caller MUST hold t.mu.
       // Plan 09-01 / OPS-06.
       func (t *Controller) drainPending() {
           for _, a := range t.pending {
               switch a.kind {
               case actStatus:
                   if t.mStatus != nil {
                       t.mStatus.SetTitle(a.status)
                   }
               case actIconHealth:
                   t.applyIconHealthLocked(a.health)
               case actShowContinueSetup:
                   if t.mContinueSetup != nil {
                       t.mContinueSetup.Show()
                   }
               case actHideContinueSetup:
                   if t.mContinueSetup != nil {
                       t.mContinueSetup.Hide()
                   }
               case actShowReauthorize:
                   if t.mReauthorize != nil {
                       t.mReauthorize.Show()
                   }
               case actHideReauthorize:
                   if t.mReauthorize != nil {
                       t.mReauthorize.Hide()
                   }
               case actSetSpreadsheetID:
                   t.spreadsheetID = a.spreadsheetID
               }
           }
           t.pending = nil
       }
       ```

    4. Add a private helper `applyIconHealthLocked(h Health)` (icon swap logic factored out of the public method so both the queued-replay and the live path share the same implementation). Place it right after `drainPending`:

       ```go
       // applyIconHealthLocked performs the systray icon swap. Caller MUST hold
       // t.mu. Plan 09-01 / OPS-06.
       func (t *Controller) applyIconHealthLocked(h Health) {
           switch h {
           case HealthGreen:
               if len(t.iconGreen) > 0 {
                   systray.SetIcon(t.iconGreen)
               }
           case HealthRed:
               if len(t.iconRed) > 0 {
                   systray.SetIcon(t.iconRed)
               }
           }
       }
       ```

    5. Add a TEST-ONLY accessor that lets `tray_test.go` (same package) inspect the queue length and contents. Place it at the end of the file:

       ```go
       // pendingSnapshot returns a copy of the pending-action queue for tests.
       // Plan 09-01 / OPS-06 — test surface only; not called from production.
       func (t *Controller) pendingSnapshot() []pendingAction {
           t.mu.Lock()
           defer t.mu.Unlock()
           out := make([]pendingAction, len(t.pending))
           copy(out, t.pending)
           return out
       }

       // isReady reports whether OnReady has run and drained the queue.
       // Plan 09-01 / OPS-06 — test surface only.
       func (t *Controller) isReady() bool {
           t.mu.Lock()
           defer t.mu.Unlock()
           return t.ready
       }
       ```

    6. In `internal/tray/tray_test.go`, add a single new test for the type scaffolding (the full queue/drain tests land in Task 3):

       ```go
       func TestPendingAction_Zero(t *testing.T) {
           c := NewController(Config{})
           if c.isReady() {
               t.Error("freshly constructed Controller should not be ready")
           }
           if snap := c.pendingSnapshot(); len(snap) != 0 {
               t.Errorf("pendingSnapshot() = %d entries, want 0", len(snap))
           }
       }
       ```

    Do NOT modify the existing public mutators in this task (that's Task 2). Do NOT modify `OnReady` in this task (that's Task 3).

    Schema-impact assertion (per CONTEXT.md D-06): this task touches `internal/tray/tray.go` only. The constant `WatcherMaxSchemaVersion = 3` lives in `internal/sheet/client.go`, which is NOT in this task's scope. No schema change.
  </action>
  <verify>
    <automated>go build ./internal/tray/... && go vet ./internal/tray/... && go test ./internal/tray/ -run TestPendingAction_Zero -v</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE 'type pendingAction struct' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'type actionKind int' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'actStatus actionKind = iota' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'ready\s+bool' internal/tray/tray.go` matches at least 1 line (the new field).
    - `grep -nE 'pending \[\]pendingAction' internal/tray/tray.go` matches at least 1 line.
    - `grep -nE 'func \(t \*Controller\) drainPending\(\)' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'func \(t \*Controller\) applyIconHealthLocked\(h Health\)' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'func \(t \*Controller\) pendingSnapshot\(\)' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'func \(t \*Controller\) isReady\(\)' internal/tray/tray.go` matches exactly 1 line.
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (schema unchanged).
    - `go test ./internal/tray/ -run TestPendingAction_Zero` passes.
    - `go vet ./internal/tray/...` exits 0.
  </acceptance_criteria>
  <done>Type scaffolding (pendingAction, actionKind enum, ready/pending fields, drainPending, applyIconHealthLocked, test accessors) lands in tray.go; one smoke test passes; `go build ./...` clean; schema constant unchanged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Convert every public mutator to queue-or-execute under t.mu</name>
  <files>internal/tray/tray.go, internal/tray/tray_test.go</files>
  <read_first>
    - internal/tray/tray.go (full file, freshly after Task 1)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Existing mutator pattern to REPLACE" (lines ~33-65) — exact rewrite shape
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-01 paragraphs on FIFO ordering + capacity (unbounded)
  </read_first>
  <behavior>
    - Each public mutator takes `t.mu.Lock()` at entry and `defer t.mu.Unlock()`.
    - When `!t.ready`: append a typed `pendingAction` to `t.pending` and `return`.
    - When `t.ready`: execute the live operation (via `applyIconHealthLocked` for icon, or direct `t.mWhatever.SetTitle/.Show()/.Hide()`).
    - `SetSpreadsheetID(id)` follows the same pattern but ALSO writes `t.spreadsheetID = id` on the live path (since the field is read by `SpreadsheetID()` regardless of menu readiness — keep the existing behavior of always storing, but only queue if state setters need future replay; for this method, ALWAYS store + only queue the action if `!t.ready` is also a no-op for downstream — see action body).
    - The `SpreadsheetID() string` reader is unchanged (already mutex-guarded).
  </behavior>
  <action>
    In `internal/tray/tray.go`, locate the existing mutator block (lines ~244-297) and replace each method body. Keep method names, receivers, and signatures identical. Concrete replacements:

    ```go
    // SetStatus updates the disabled top menu label. Goroutine-safe. Pre-Ready
    // calls are queued and replayed by OnReady. Plan 09-01 / OPS-06.
    func (t *Controller) SetStatus(s string) {
        t.mu.Lock()
        defer t.mu.Unlock()
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actStatus, status: s})
            return
        }
        if t.mStatus != nil {
            t.mStatus.SetTitle(s)
        }
    }

    // SetIconHealth swaps the tray icon between green (normal) and red. Pre-Ready
    // calls are queued. Plan 09-01 / OPS-06.
    func (t *Controller) SetIconHealth(h Health) {
        t.mu.Lock()
        defer t.mu.Unlock()
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actIconHealth, health: h})
            return
        }
        t.applyIconHealthLocked(h)
    }

    // ShowContinueSetup makes the Continue setup… item visible. D-07. Pre-Ready
    // calls are queued. Plan 09-01 / OPS-06.
    func (t *Controller) ShowContinueSetup() {
        t.mu.Lock()
        defer t.mu.Unlock()
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actShowContinueSetup})
            return
        }
        if t.mContinueSetup != nil {
            t.mContinueSetup.Show()
        }
    }

    // HideContinueSetup hides the Continue setup… item. Pre-Ready calls are queued. Plan 09-01.
    func (t *Controller) HideContinueSetup() {
        t.mu.Lock()
        defer t.mu.Unlock()
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actHideContinueSetup})
            return
        }
        if t.mContinueSetup != nil {
            t.mContinueSetup.Hide()
        }
    }

    // ShowReauthorize makes the Reauthorize… item visible. AUTH-05 + AUTH-07 path.
    // Pre-Ready calls are queued. Plan 09-01 / OPS-06.
    func (t *Controller) ShowReauthorize() {
        t.mu.Lock()
        defer t.mu.Unlock()
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actShowReauthorize})
            return
        }
        if t.mReauthorize != nil {
            t.mReauthorize.Show()
        }
    }

    // HideReauthorize hides the Reauthorize… item. Pre-Ready calls are queued. Plan 09-01.
    func (t *Controller) HideReauthorize() {
        t.mu.Lock()
        defer t.mu.Unlock()
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actHideReauthorize})
            return
        }
        if t.mReauthorize != nil {
            t.mReauthorize.Hide()
        }
    }
    ```

    For `SetSpreadsheetID(id string)` (the existing implementation at tray.go:302-313 already takes t.mu):
    - Keep `t.spreadsheetID = id` ALWAYS (so the in-memory field tracks state even before Ready — preserving existing reader behavior).
    - Additionally, if `!t.ready`, append a `pendingAction{kind: actSetSpreadsheetID, spreadsheetID: id}` to `t.pending` so the drain path is consistent (the drain re-assigns, which is a no-op semantically since the field is already set — keep it for symmetry and so the queue length reflects all deferred calls).

    Replacement:

    ```go
    // SetSpreadsheetID updates the cached spreadsheet ID. Plan 09-01 / OPS-06:
    // the in-memory field is always kept current; if pre-Ready, the assignment is
    // also queued so the drain path is symmetric with the other mutators.
    func (t *Controller) SetSpreadsheetID(id string) {
        t.mu.Lock()
        defer t.mu.Unlock()
        t.spreadsheetID = id
        if !t.ready {
            t.pending = append(t.pending, pendingAction{kind: actSetSpreadsheetID, spreadsheetID: id})
        }
    }
    ```

    Update the existing `TestMutators_SafeBeforeOnReady` test (tray_test.go:49-65) — change its name and semantics to reflect the new contract:

    ```go
    // TestPreReady_EnqueuesNotDrops verifies that mutator calls made before
    // OnReady are queued (not silently dropped). Plan 09-01 / OPS-06. Replaces
    // the original TestMutators_SafeBeforeOnReady "no panic" assertion.
    func TestPreReady_EnqueuesNotDrops(t *testing.T) {
        c := NewController(Config{})
        c.SetStatus("hello")
        c.SetIconHealth(HealthGreen)
        c.SetIconHealth(HealthRed)
        c.ShowContinueSetup()
        c.HideContinueSetup()
        c.ShowReauthorize()
        c.HideReauthorize()
        c.SetSpreadsheetID("abc")
        snap := c.pendingSnapshot()
        if len(snap) != 8 {
            t.Fatalf("pending = %d entries, want 8", len(snap))
        }
        // No panic.
    }
    ```

    Delete the original `TestMutators_SafeBeforeOnReady` body (replace it wholesale with the new test above, or remove it entirely — but DO NOT leave a duplicate "no-panic" assertion).
  </action>
  <verify>
    <automated>go build ./internal/tray/... && go vet ./internal/tray/... && go test ./internal/tray/ -run TestPreReady_EnqueuesNotDrops -v</automated>
  </verify>
  <acceptance_criteria>
    - `grep -cE 't\.mu\.Lock\(\)' internal/tray/tray.go` matches at least 8 (one per mutator + existing SpreadsheetID reader + drainPending callers).
    - Every one of these strings appears in `internal/tray/tray.go`: `kind: actStatus`, `kind: actIconHealth`, `kind: actShowContinueSetup`, `kind: actHideContinueSetup`, `kind: actShowReauthorize`, `kind: actHideReauthorize`, `kind: actSetSpreadsheetID`. Verify with `grep -nE 'kind: act(Status|IconHealth|ShowContinueSetup|HideContinueSetup|ShowReauthorize|HideReauthorize|SetSpreadsheetID)' internal/tray/tray.go` matches 7 distinct lines.
    - `grep -nE 'if !t\.ready' internal/tray/tray.go` matches at least 7 lines (one per mutator).
    - `grep -nE 'TestPreReady_EnqueuesNotDrops' internal/tray/tray_test.go` matches exactly 1 line.
    - `grep -cE 'TestMutators_SafeBeforeOnReady' internal/tray/tray_test.go` returns 0 (old test fully replaced).
    - `go test ./internal/tray/ -run TestPreReady_EnqueuesNotDrops` passes.
    - `go vet ./internal/tray/...` exits 0.
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line.
  </acceptance_criteria>
  <done>All 7 public mutators (SetStatus, SetIconHealth, ShowContinueSetup, HideContinueSetup, ShowReauthorize, HideReauthorize, SetSpreadsheetID) queue-or-execute under t.mu; the replacement test passes; schema constant unchanged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Wire drainPending into OnReady; add FIFO-order and post-Ready-live tests</name>
  <files>internal/tray/tray.go, internal/tray/tray_test.go</files>
  <read_first>
    - internal/tray/tray.go (full file, freshly after Tasks 1+2)
    - internal/tray/tray_test.go (full file, freshly after Task 2's edits)
    - .planning/phases/09-watcher-robustness-polish/09-PATTERNS.md §"Existing OnReady wiring to extend" (drain placement before `go t.loop()`)
    - .planning/phases/09-watcher-robustness-polish/09-CONTEXT.md §D-01 paragraphs on drain ordering + the load-bearing AUTH-07 dependency
  </read_first>
  <behavior>
    - After all menu items are constructed in OnReady (currently at line ~159 just before `go t.loop()`), the controller takes `t.mu`, sets `t.ready = true`, calls `drainPending()`, releases `t.mu`, then proceeds to `go t.loop()`.
    - Tests cannot call `systray.Run`/the real `OnReady` (no desktop session in CI), so they exercise drain via a test-only helper `simulateReady()` that flips `t.ready = true` + calls `drainPending()` under the mutex — the same code path OnReady uses, minus the systray menu construction. Tests verify FIFO order by inspecting `pendingSnapshot()` before simulateReady and observing the drain effect (queue empties; `isReady()` returns true; post-simulateReady mutator calls do NOT append to pending).
  </behavior>
  <action>
    In `internal/tray/tray.go`:

    1. Modify `OnReady` (currently ends with `go t.loop()` at line 161). Replace its tail with:

       ```go
           systray.AddSeparator()
           t.mQuit = systray.AddMenuItem(plan[6].Label, plan[6].Tooltip) // Quit

           // Plan 09-01 / OPS-06: drain any mutator calls queued before OnReady.
           // Must run AFTER all systray.AddMenuItem calls (so the *MenuItem fields
           // are non-nil) and BEFORE go t.loop() (so the loop starts in a stable
           // state). The flag flip + drain run under t.mu in a single critical
           // section so concurrent SetStatus/SetIconHealth/etc. either land in the
           // queue (and are drained here) or land live (after we release t.mu).
           t.mu.Lock()
           t.ready = true
           t.drainPending()
           t.mu.Unlock()

           go t.loop()
       }
       ```

       (Do not remove the `t.mContinueSetup.Hide()` / `t.mReauthorize.Hide()` calls already in OnReady at lines 155 and 157 — they continue to set default-hidden state on the menu items themselves. The drain then replays any pre-Ready `ShowReauthorize` and the last call wins.)

    2. Add a test-only helper to `internal/tray/tray.go` (next to the other test surface helpers added in Task 1):

       ```go
       // simulateReady is a TEST-ONLY helper that flips the ready flag and drains
       // the pending queue. Mirrors OnReady's drain block exactly. Allows offline
       // tests to exercise the drain code path without a live systray. Plan 09-01.
       func (t *Controller) simulateReady() {
           t.mu.Lock()
           t.ready = true
           t.drainPending()
           t.mu.Unlock()
       }
       ```

    3. Add three new tests to `internal/tray/tray_test.go` (after `TestPreReady_EnqueuesNotDrops`):

       ```go
       // TestPreReady_FIFOOrder verifies that queued actions retain insertion
       // order so paired Show*/Hide* calls replay last-write-wins. Plan 09-01.
       func TestPreReady_FIFOOrder(t *testing.T) {
           c := NewController(Config{})
           c.SetIconHealth(HealthRed)
           c.ShowReauthorize()
           c.SetStatus("auth error")
           c.HideReauthorize()
           c.SetStatus("recovered")

           snap := c.pendingSnapshot()
           if len(snap) != 5 {
               t.Fatalf("pendingSnapshot len = %d, want 5", len(snap))
           }
           wantKinds := []actionKind{
               actIconHealth, actShowReauthorize, actStatus, actHideReauthorize, actStatus,
           }
           for i, w := range wantKinds {
               if snap[i].kind != w {
                   t.Errorf("snap[%d].kind = %v, want %v", i, snap[i].kind, w)
               }
           }
           if snap[0].health != HealthRed {
               t.Errorf("snap[0].health = %v, want HealthRed", snap[0].health)
           }
           if snap[2].status != "auth error" {
               t.Errorf("snap[2].status = %q, want %q", snap[2].status, "auth error")
           }
           if snap[4].status != "recovered" {
               t.Errorf("snap[4].status = %q, want %q", snap[4].status, "recovered")
           }
       }

       // TestSimulateReady_DrainsQueue verifies simulateReady empties the queue
       // and flips isReady. Plan 09-01.
       func TestSimulateReady_DrainsQueue(t *testing.T) {
           c := NewController(Config{})
           c.SetStatus("queued before ready")
           c.SetIconHealth(HealthRed)
           if len(c.pendingSnapshot()) != 2 {
               t.Fatalf("pre-ready len = %d, want 2", len(c.pendingSnapshot()))
           }
           if c.isReady() {
               t.Fatal("isReady() = true before simulateReady; want false")
           }

           c.simulateReady()

           if !c.isReady() {
               t.Error("isReady() = false after simulateReady; want true")
           }
           if got := c.pendingSnapshot(); len(got) != 0 {
               t.Errorf("post-drain pending len = %d, want 0", len(got))
           }
       }

       // TestPostReady_ExecutesLive verifies that after OnReady (here simulated),
       // subsequent mutator calls do NOT append to the pending queue. Plan 09-01.
       func TestPostReady_ExecutesLive(t *testing.T) {
           c := NewController(Config{})
           c.simulateReady() // skip the queued phase entirely

           c.SetStatus("live")
           c.SetIconHealth(HealthGreen)
           c.ShowReauthorize()
           c.SetSpreadsheetID("ssid-123")

           if got := c.pendingSnapshot(); len(got) != 0 {
               t.Errorf("post-ready pending len = %d, want 0 (live execution should not enqueue)", len(got))
           }
           // SetSpreadsheetID always writes the field regardless of readiness.
           if c.SpreadsheetID() != "ssid-123" {
               t.Errorf("SpreadsheetID() = %q, want %q", c.SpreadsheetID(), "ssid-123")
           }
       }
       ```
  </action>
  <verify>
    <automated>go build ./internal/tray/... && go vet ./internal/tray/... && go test ./internal/tray/ -v</automated>
  </verify>
  <acceptance_criteria>
    - `grep -nE 't\.ready = true' internal/tray/tray.go` matches exactly 2 lines (OnReady + simulateReady).
    - `grep -nE 't\.drainPending\(\)' internal/tray/tray.go` matches exactly 2 lines (OnReady + simulateReady).
    - `grep -nE 'func \(t \*Controller\) simulateReady\(\)' internal/tray/tray.go` matches exactly 1 line.
    - All of these tests appear in `internal/tray/tray_test.go`: `TestPendingAction_Zero`, `TestPreReady_EnqueuesNotDrops`, `TestPreReady_FIFOOrder`, `TestSimulateReady_DrainsQueue`, `TestPostReady_ExecutesLive`.
    - `go test ./internal/tray/ -v -run 'TestPendingAction_Zero|TestPreReady_EnqueuesNotDrops|TestPreReady_FIFOOrder|TestSimulateReady_DrainsQueue|TestPostReady_ExecutesLive'` reports 5/5 PASS.
    - Full package test run `go test ./internal/tray/ -count=1` exits 0 (no regressions in existing tests like `TestOnReauthorizeCallback_Wired`, `TestMenuPlan*`, etc.).
    - `go vet ./internal/tray/...` exits 0.
    - `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line.
  </acceptance_criteria>
  <done>OnReady drains the queue; simulateReady test helper added; 4 new tests + 1 smoke test all pass; existing tray tests remain green; `go test ./...` from repo root exits 0; schema constant unchanged.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| caller → Controller mutator | Goroutine boundary: any goroutine in the watcher (RunApp, runWatcher, reauth flow) calls mutators; queue + mutex are the safety net |
| pre-Ready → post-Ready | Time boundary: between `NewController` and `OnReady` completion, no menu items exist; queue bridges this window |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-09-01-01 | Denial of Service | Controller.pending queue | accept | Queue capacity is unbounded per CONTEXT.md D-01 ("the dark window is short — milliseconds to seconds; even a pathological 1000 queued calls is trivial memory"). In-process only; no external attacker can append. Risk: low. |
| T-09-01-02 | Tampering | Controller.ready / Controller.pending race | mitigate | Both fields are guarded by the existing `t.mu sync.Mutex` (tray.go:97). Drain happens inside the same critical section as the flag flip (OnReady + simulateReady both take t.mu, set ready=true, drainPending(), release). No second mutex introduced (CONTEXT.md D-01). |
| T-09-01-03 | Tampering | drainPending re-entrancy | mitigate | `drainPending` documents and requires the caller to hold `t.mu`. It dispatches directly to the live execution path inside the switch (NOT back through public mutators that re-take the mutex), avoiding deadlock. |
| T-09-01-04 | Information Disclosure | pendingSnapshot / isReady test helpers | accept | Helpers are package-private (lowercase) and reachable only from `package tray` test files. No production caller; no external surface. |

**Schema impact:** NONE. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` in `internal/sheet/client.go` stays at 3 (verifier grep gate: `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line).
</threat_model>

<verification>
1. `go build ./...` exits 0.
2. `go vet ./...` exits 0.
3. `go test ./internal/tray/ -count=1 -v` exits 0; reports the 5 new tests passing (`TestPendingAction_Zero`, `TestPreReady_EnqueuesNotDrops`, `TestPreReady_FIFOOrder`, `TestSimulateReady_DrainsQueue`, `TestPostReady_ExecutesLive`) plus existing tray tests green.
4. `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` matches exactly 1 line (schema unchanged).
5. `grep -cE 'TestMutators_SafeBeforeOnReady' internal/tray/tray_test.go` returns 0 (old "no-panic" test fully replaced).
6. `grep -nE 'type pendingAction struct' internal/tray/tray.go` matches exactly 1 line.
7. `grep -nE 'func \(t \*Controller\) drainPending\(\)' internal/tray/tray.go` matches exactly 1 line.
8. `grep -nE 't\.ready = true' internal/tray/tray.go` matches exactly 2 lines (OnReady + simulateReady).
</verification>

<success_criteria>
- Pre-Ready mutator calls are queued in FIFO order under `t.mu`, replayed in OnReady, and the queue empties.
- Post-Ready mutator calls execute live (no queue accumulation).
- No second mutex; no new menu items; no schema change.
- All 5 new tests + the existing tray test suite pass.
- The Controller is now structurally ready for AUTH-07 (Plan 09-04) to fire `SetIconHealth(Red) + SetStatus + ShowReauthorize` on the boot fast-fail path and have those calls replay correctly in OnReady.
</success_criteria>

<output>
After completion, create `.planning/phases/09-watcher-robustness-polish/09-01-SUMMARY.md` summarizing:
- The queue+drain implementation shape (pendingAction typed-action struct, ready flag, single-mutex design).
- Verification of all must_haves truths.
- Confirmation that `WatcherMaxSchemaVersion = 3` is unchanged.
- Test count delta (5 new tests; 1 old test replaced).
- Hand-off note for Plan 09-04: "The boot-fast-fail tray triple (SetIconHealth(Red) + SetStatus + ShowReauthorize) is now safe to call before OnReady."
</output>
