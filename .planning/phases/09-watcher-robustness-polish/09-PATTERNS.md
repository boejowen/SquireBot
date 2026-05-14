# Phase 9: Watcher Robustness Polish — Pattern Map

**Mapped:** 2026-05-12
**Files analyzed:** 8 (across 5 plans)
**Analogs found:** 8 / 8 (all in-repo)

## File Classification

| Plan | New / Modified File | Role | Data Flow | Closest Analog | Match Quality |
|------|---------------------|------|-----------|----------------|---------------|
| 09-01 | `internal/tray/tray.go` (modify) | controller (UI mutator surface) | event-driven (pre-Ready buffer → drain on OnReady) | self — existing Controller methods (lines 245-297) | exact (extending existing struct) |
| 09-01 | `internal/tray/tray_test.go` (modify) | test (offline unit tests) | request-response (mutator → assert) | self — `TestMutators_SafeBeforeOnReady` (lines 49-65) | exact |
| 09-02 | `cmd/squirebot/console_windows.go` (new) | utility (process detach) | one-shot syscall | `internal/system/shutdown_signal_windows.go` build-tag scaffold (lines 1-11) | role-match (Windows syscall behind build tag) |
| 09-02 | `cmd/squirebot/console_other.go` (new, stub) | utility (no-op on non-Windows) | one-shot no-op | `internal/system/shutdown_signal_other.go` (full file) | exact |
| 09-02 | `cmd/squirebot/main.go` (modify) | entry point | request-response (short-circuit + detach + run) | self — `--quit` / `--uninstall-wipe-credentials` short-circuits (lines 39-76) | exact |
| 09-03 | `internal/config/config.go` (modify) | model loader (JSON unmarshal) | file-I/O (read + parse) | self — existing `Load()` (lines 52-85) | exact |
| 09-03 | `internal/config/config_test.go` (modify) | test (table-driven I/O test) | request-response | `TestLoad_Phase1ConfigBackCompat` (lines 122-144) | exact |
| 09-04 | `internal/app/runapp.go` (modify) | service (orchestrator) | event-driven (boot fast-fail → tray queue) | self — `RunApp` cold-start fast-fail block (lines 102-112) + AUTH-05 `suspendForAuth` (lines 628-637) | exact (extends existing pattern) |
| 09-04 | `internal/app/runapp_test.go` (modify) | test (orchestrator unit) | request-response | self — existing table-driven tests + `TestNeedsWizard` (lines 45-68) | role-match |
| 09-05 | `cmd/squirebot/build_constants.go` (modify) | config (build-time constants) | request-response (ldflags injected) | self — `Version = "0.1.0-dev"` (line 29) | exact (one-line bump or ldflag verification only) |
| 09-05 | `.planning/phases/09-watcher-robustness-polish/09-05-release-tag-PLAN.md` (new) | release plan | release-engineering | `.planning/phases/06-installer-overwrite-running-shim/06-05-release-tag-PLAN.md` (full file) | exact |

## Pattern Assignments

### Plan 09-01 — Tray controller pre-Ready queue (OPS-06)

**Files modified:** `internal/tray/tray.go`, `internal/tray/tray_test.go`

**Analog (self-extension):** `internal/tray/tray.go` — existing `Controller` struct (lines 96-117) and mutator methods (lines 245-297). The pattern to mirror is the **existing nil-guard idiom**, which is replaced by **queue-or-execute under mutex**.

#### Existing mutator pattern to REPLACE (tray.go:244-297)

```go
// SetStatus updates the disabled top menu label. Goroutine-safe.
func (t *Controller) SetStatus(s string) {
	if t.mStatus != nil {
		t.mStatus.SetTitle(s)
	}
}

// SetIconHealth swaps the tray icon between green (normal) and red
// (Setup needed / error).
func (t *Controller) SetIconHealth(h Health) {
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

// ShowContinueSetup makes the Continue setup… item visible. D-07.
func (t *Controller) ShowContinueSetup() {
	if t.mContinueSetup != nil {
		t.mContinueSetup.Show()
	}
}
// ... HideContinueSetup, ShowReauthorize, HideReauthorize all identical-shape ...
```

Every public mutator currently shape `if t.mWhatever != nil { live op }`. That nil-check is the silent-no-op pre-Ready. D-01 replaces it with a `t.mu`-guarded `if t.ready { live op } else { enqueue }`.

#### Existing struct fields to extend (tray.go:96-117)

```go
type Controller struct {
	mu            sync.Mutex
	iconGreen     []byte
	iconRed       []byte
	logDir        string
	spreadsheetID string

	mStatus         *systray.MenuItem
	mWorkbook       *systray.MenuItem
	// ... etc
}
```

Add: `ready bool` and `pending []func()` (or `pending []pendingAction`) under the existing `mu`. The mutex already exists for `spreadsheetID` (lines 302-313); reuse it — do not introduce a second mutex.

#### Existing OnReady wiring to extend (tray.go:139-162)

```go
func (t *Controller) OnReady() {
	if len(t.iconGreen) > 0 {
		systray.SetIcon(t.iconGreen)
	}
	systray.SetTooltip("SquireBot")

	t.mStatus = systray.AddMenuItem(LabelStatus, "")
	t.mStatus.Disable()
	// ... menu builds ...
	go t.loop()
}
```

D-01 drains: after `go t.loop()` (or just before it), lock `t.mu`, set `t.ready = true`, iterate `t.pending` calling each closure, clear `t.pending`, unlock. FIFO order is the slice iteration order — preserves last-write-wins for `ShowReauthorize` → `HideReauthorize` pairs.

#### Existing test pattern to mirror (tray_test.go:49-65)

```go
// SetStatus / SetIconHealth / ShowContinueSetup / HideContinueSetup are
// no-ops when the underlying systray menu items are nil (i.e., before
// OnReady has been called). Verify they don't panic.
func TestMutators_SafeBeforeOnReady(t *testing.T) {
	c := NewController(Config{})
	c.SetStatus("hello")
	c.SetIconHealth(HealthGreen)
	c.SetIconHealth(HealthRed)
	c.ShowContinueSetup()
	c.HideContinueSetup()
}
```

D-01 keeps the "must not panic" semantic but ALSO asserts the call was enqueued. New tests:
- `TestPreReady_Enqueues` — call SetStatus, assert `len(c.pending) == 1`
- `TestPreReady_FIFOOrder` — call SetIconHealth(Red), ShowReauthorize, SetStatus("X") and assert the pending slice's order/length
- `TestPreReady_DrainPattern` — call mutators, simulate Ready (set `t.ready = true` + drain manually via a test-only helper OR call a refactored `drainPending()`), assert pending is empty after

#### Existing callback-wired test pattern to mirror (tray_test.go:143-155)

```go
func TestOnReauthorizeCallback_Wired(t *testing.T) {
	calls := 0
	c := NewController(Config{
		OnReauthorize: func() { calls++ },
	})
	if c.onReauthorize == nil {
		t.Fatal("Controller.onReauthorize not wired from Config.OnReauthorize")
	}
	c.onReauthorize()
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}
```

Pattern: construct `Controller`, exercise via the public API, count side effects. No systray.Run.

**Differs from analog:**
- Existing methods are pure nil-guard no-ops; new methods become **queue-or-execute** with `t.mu` already held.
- The existing `t.mu` is currently only used for `spreadsheetID` (tray.go:303-313). The plan **extends** its scope to cover `ready` and `pending`. Do not introduce a second mutex.
- Tests today assert "no panic" only (`TestMutators_SafeBeforeOnReady`). New tests must assert **queue contents** — that requires either a test-only accessor (e.g., `pendingLen()` package-private function in `tray_test.go`'s package) or making `pending` package-visible. Closures vs typed-action struct is Claude's-Discretion per CONTEXT.md D-07; typed actions are easier to test (can inspect kind).

---

### Plan 09-02 — `windows.FreeConsole()` foreground detach (OPS-07)

**Files modified/new:** `cmd/squirebot/main.go` (modify), `cmd/squirebot/console_windows.go` (new), `cmd/squirebot/console_other.go` (new, stub). Optional 1-line note in `docs/build-and-install.md`.

**Analog (build-tag scaffold):** `internal/system/shutdown_signal_windows.go` + `internal/system/shutdown_signal_other.go` pair.

#### Build-tag header pattern (shutdown_signal_windows.go:1-11)

```go
//go:build windows

package system

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)
```

Mirror exactly for `console_windows.go`:
- `//go:build windows` line
- `package main`
- import `"golang.org/x/sys/windows"`

#### Non-Windows stub pattern (shutdown_signal_other.go full file)

```go
//go:build !windows

package system

import "context"

// SignalShutdown is a no-op on non-Windows platforms. The Windows
// implementation lives in shutdown_signal_windows.go.
func SignalShutdown() error { return nil }
```

Mirror exactly: `console_other.go` has `//go:build !windows`, `package main`, exposes `freeConsole()` as a no-op returning nil.

#### Existing main.go short-circuit ordering to extend (cmd/squirebot/main.go:26-76)

```go
func main() {
	// Plan 02-07 (INST-04 / CONTEXT.md Q3): --uninstall-wipe-credentials.
	// ...
	if len(os.Args) >= 2 && os.Args[1] == "--uninstall-wipe-credentials" {
		// ... reads stderr-bound output, os.Exit(0)
	}

	// Plan 06 (INST-06): --quit.
	// ...
	if len(os.Args) >= 2 && os.Args[1] == "--quit" {
		// ... reads stderr-bound output, os.Exit(0)
	}

	// Plan 02-06 (OPS-04) startup-swap: BEFORE any other goroutine,
	// before logging.Setup, before config.Load, check for a staged
	// update adjacent to the running binary.
	if swapped, err := update.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "auto-update apply failed: %v\n", err)
	} else if swapped {
		os.Exit(0)
	}

	log, logDir := logging.Setup()
	slog.SetDefault(log)
	// ...
```

D-02 placement: insert `_ = freeConsole()` **after the `--quit` block (line 76) and after the `update.Apply` block (lines 91-98) but BEFORE `logging.Setup` (line 100)**. The `--uninstall-wipe-credentials` and `--quit` short-circuits MUST run first (they write to stderr that NSIS captures). Auto-update apply writes to stderr too pre-detach.

Per CONTEXT.md specifics §3: the two short-circuit checks must keep their inherited stdio. Place `FreeConsole` strictly after them. Confirmed by reading main.go:26-76.

#### Existing exit-log line (cmd/squirebot/main.go:213)

```go
systray.Run(trayCtl.OnReady, trayCtl.OnExit)

// Tray quit → tear down background work.
cancel()
slog.Info("squirebot exit")
```

CONTEXT.md D-02 verification: this line MUST still emit. It does — it writes via slog to the log file, not stdio. `FreeConsole` only detaches console stdio; the lumberjack-backed log file is unaffected.

**Differs from analog:**
- The system package's split is by **package boundary**; here the split is inside `package main`. The build-tag mechanism is identical.
- The Windows file calls one syscall (`windows.FreeConsole()`); the analog calls multiple (CreateEvent/OpenEvent/SetEvent). Strictly simpler.
- No test analog — `FreeConsole` is OS-level and not unit-testable without elaborate mocking. CONTEXT.md D-07 accepts manual smoke evidence captured in the 09-05 release plan or `09-02-SMOKE.md`.

---

### Plan 09-03 — UTF-8 BOM strip in `config.Load()` (CONFIG-01)

**Files modified:** `internal/config/config.go`, `internal/config/config_test.go`

**Analog (self-extension):** `internal/config/config.go` — existing `Load()` (lines 52-85). The fix is a 3-LOC insertion.

#### Existing Load() to extend (config.go:52-85)

```go
func Load() (*Config, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{Version: 1, LogLevel: "info"}, nil
		}
		return nil, fmt.Errorf("config load %s: %w", p, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config load %s: %w", p, err)
	}
	if c.Version == 0 {
		c.Version = 1
	}
	// ...
}
```

D-04 insertion site: between line 54 (`data, err := os.ReadFile(p)`) and line 62 (`json.Unmarshal(data, &c)`). After the not-exist short-circuit on line 56-58.

Required insertion (per D-04, ~3 LOC including import):

```go
// CONFIG-01: strip a leading UTF-8 BOM. Notepad / PS5.1 Set-Content
// -Encoding utf8 both write a BOM by default; encoding/json doesn't
// auto-strip and fails with "invalid character 'ï' looking for beginning of value".
data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
```

Plus `"bytes"` added to the existing import block (config.go:9-16).

#### Existing test pattern to mirror (config_test.go:122-144 — TestLoad_Phase1ConfigBackCompat)

```go
func TestLoad_Phase1ConfigBackCompat(t *testing.T) {
	p := withTempConfig(t)
	body := `{
  "version": 1,
  "eq_folder": "C:\\P99",
  "spreadsheet_id": "abc",
  "google_email": "g@example.com",
  "log_level": "info"
}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.EQFolders) != 1 || c.EQFolders[0] != `C:\P99` {
		t.Errorf("EQFolders = %#v, want [C:\\P99] migrated from eq_folder", c.EQFolders)
	}
	// ...
}
```

Pattern (exact reuse):
1. Call `withTempConfig(t)` (lines 10-19) to redirect `pathFn` to a temp file.
2. Build a JSON body string.
3. `os.WriteFile(p, []byte(body), 0o600)`.
4. Call `Load()` and assert on the returned Config.

New test `TestLoad_StripsUTF8BOM`:
- Step 1-3 identical, but prepend `[]byte{0xEF, 0xBB, 0xBF}` to the JSON body via `append([]byte{0xEF, 0xBB, 0xBF}, body...)` then `os.WriteFile`.
- Step 4: assert no error and that fields parsed correctly (e.g., `c.SpreadsheetID == "abc"`).

#### withTempConfig helper to reuse (config_test.go:10-19)

```go
func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	orig := pathFn
	pathFn = func() string { return p }
	t.Cleanup(func() { pathFn = orig })
	return p
}
```

Existing infrastructure; new test does not need to create new helpers.

**Differs from analog:**
- Nothing — this is a direct extension of the same Load() with the same test idiom.
- CONTEXT.md specifics §4: stick to ≤5 LOC src + 1 unit test. Resist the urge to also strip BOMs from `auth.StoredToken` or `latest.json` reads (D-04 closing paragraph).

---

### Plan 09-04 — Boot-time `invalid_grant` → red + Reauthorize (AUTH-07)

**Files modified:** `internal/app/runapp.go`, `internal/app/runapp_test.go`. Possibly thin additions to `internal/tray/tray.go` (only if a new helper is needed — D-05 says unlikely).

**Analog (self-extension):** Two existing patterns in this codebase that compose:
- `internal/app/runapp.go:102-112` — the cold-start `buildTokenSourceFromWincred` fast-fail block (current behavior to MODIFY).
- `internal/app/runapp.go:628-637` — `suspendForAuth` running-state AUTH-05 helper (the canonical tray-call triple to MIRROR).

#### Current cold-start fast-fail path to MODIFY (runapp.go:102-112)

```go
// Watcher path. If we came through the wizard, ts is live; otherwise
// (skip-wizard cold start) we rebuild it from wincred.
if ts == nil {
	built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
	if err != nil {
		slog.Error("token rebuild from wincred failed", "err", err)
		t.SetStatus(fmt.Sprintf("Auth error: %v", err))
		t.SetIconHealth(tray.HealthRed)
		t.ShowContinueSetup()
		return
	}
	ts = built
}
```

D-03 fix: branch on the error using the existing `auth.IsRevokedRefreshToken` classifier (already imported on line 38 / used at line 621). On match, mirror AUTH-05's tray triple instead of `ShowContinueSetup`. On non-match, keep existing behavior (the user's wincred entry is broken in a non-revoked way; ContinueSetup is the right path).

Suggested rewrite shape:

```go
if ts == nil {
	built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
	if err != nil {
		slog.Error("token rebuild from wincred failed", "err", err)
		if auth.IsRevokedRefreshToken(err) {
			// AUTH-07: boot-time invalid_grant. Mirror AUTH-05 running-state UX.
			// Calls land in pre-Ready window; OPS-06 queue drains them on OnReady.
			t.SetIconHealth(tray.HealthRed)
			t.SetStatus("Auth error — sign in again")
			t.ShowReauthorize()
		} else {
			t.SetStatus(fmt.Sprintf("Auth error: %v", err))
			t.SetIconHealth(tray.HealthRed)
			t.ShowContinueSetup()
		}
		return
	}
	ts = built
}
```

#### AUTH-05 canonical tray-call triple to MIRROR (runapp.go:628-637 — suspendForAuth)

```go
func suspendForAuth(authSuspended *atomic.Bool, t *tray.Controller, charName, kind string, err error) {
	if authSuspended != nil {
		authSuspended.Store(true)
	}
	slog.Error("permanent auth failure — suspending writes",
		"char", charName, "kind", kind, "err", err)
	t.SetIconHealth(tray.HealthRed)
	t.SetStatus("Reauthorize: refresh token died. Click Reauthorize…")
	t.ShowReauthorize()
}
```

**Canonical AUTH-05 status string:** `"Reauthorize: refresh token died. Click Reauthorize…"` (runapp.go:635).

Per CONTEXT.md Claude's-Discretion §3: "Mirror AUTH-05 if it has a canonical phrasing; otherwise pick the shorter option." It does have one. The planner should consider using **the same exact string** for boot-time AUTH-07 (perfect parity with running-state) OR a slightly shorter `"Auth error — sign in again"` per D-03. The exact string is delegated; the **call sequence** (Red icon → SetStatus → ShowReauthorize) is fixed.

Note: there's no `suspendForAuth`-like authSuspended flag to flip on boot — boot-time, the watcher hasn't started yet, so there's no in-flight upload loop to suspend. Just the three tray calls. The watcher will not be running, so no skip-check is needed.

#### Existing classifier usage site for reference (runapp.go:614-622)

```go
// isPermanentAuthErr returns true if err is the boundary signal Plan
// 02-03's withRetry produces on a second auth-flavored 403
// (sheet.ErrPermanentAuth) OR the canonical Google refresh-token-dead
// shape Plan 02-04 Task 1's IsRevokedRefreshToken matches against.
func isPermanentAuthErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sheet.ErrPermanentAuth) {
		return true
	}
	return auth.IsRevokedRefreshToken(err)
}
```

This is the running-state classifier wrapper. Boot-time, `sheet.ErrPermanentAuth` cannot apply (no Sheets call has happened); only `auth.IsRevokedRefreshToken` is in play. Per D-03, call `auth.IsRevokedRefreshToken(err)` directly — do NOT reuse `isPermanentAuthErr` (which would expand the trigger surface unnecessarily).

#### Existing test analog for runapp orchestration (runapp_test.go:45-68 — TestNeedsWizard table-driven)

```go
func TestNeedsWizard(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{"empty", &config.Config{}, true},
		// ...
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := needsWizard(tc.cfg)
			if got != tc.want {
				t.Errorf("needsWizard(%+v) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}
```

For AUTH-07 the unit test surface is the classifier branching, not the full `RunApp` (which requires a full bc + tray + ctx wiring). Recommended approach:

1. Extract the branch logic into a small testable helper, e.g., `applyBootAuthError(t *tray.Controller, err error)` (~10 LOC inside runapp.go), then unit-test that against a mock `*tray.Controller`.
2. Use `auth.IsRevokedRefreshToken`-matching errors as test inputs. Look at `internal/auth/refresh_test.go` (existing file — already covers this classifier) for sample errors that match:

```go
// Sample matchable error (from auth/refresh.go context):
&oauth2.RetrieveError{ErrorCode: "invalid_grant"}
// Or a string-wrapped form:
errors.New("oauth2: \"invalid_grant\" refresh token revoked")
```

The mock `*tray.Controller` is straightforward — `tray.NewController(tray.Config{})` already works in tests (per tray_test.go:14 — `NewController` accepts an all-empty Config). Test asserts that after the helper runs, the controller's pending queue (from OPS-06) contains the 3 expected actions.

**Cross-plan dependency (load-bearing, called out in CONTEXT.md specifics §2):**
- Without OPS-06's queue, these three `t.SetIconHealth` / `t.SetStatus` / `t.ShowReauthorize` calls land in the pre-Ready silent-no-op window (today's bug). With the queue, they replay in `OnReady`. **Plan 09-04 cannot land before 09-01.** D-05 wave ordering: 09-01 is wave 1, 09-04 is wave 2.

**Differs from analog:**
- AUTH-05 fires from the running-state inventory/spellbook write path; AUTH-07 fires from the cold-start `buildTokenSourceFromWincred` failure. Same tray UX, different upstream trigger.
- AUTH-05 sets `globalAuthSuspended.Store(true)` because there's an active watcher to gate. AUTH-07 has no watcher yet — skip the atomic flag write.
- AUTH-07's status string is a Claude's-Discretion choice between the canonical AUTH-05 string and a shorter D-03-suggested variant.

---

### Plan 09-05 — v1.0.2 binary release tag + `latest.json` refresh

**Files modified/new:**
- `cmd/squirebot/build_constants.go` — `Version = "0.1.0-dev"` (line 29) is the dev default; the **release path** injects `Version` via `-ldflags="-X main.Version=$VERSION"` (see `.github/workflows/release.yml:166`). No source bump needed for release; the tag drives the ldflag.
- `.planning/phases/09-watcher-robustness-polish/09-05-release-tag-PLAN.md` — new plan file mirroring 06-05.
- No `latest.json` source file in the repo — it is generated by `release.yml:243-266` per-tag.
- No `release.yml` modifications expected (CONTEXT.md D-08: "Reuse `release.yml` GitHub Action unchanged where possible").

**Analog (full-file template):** `.planning/phases/06-installer-overwrite-running-shim/06-05-release-tag-PLAN.md` (full file).

#### Frontmatter pattern (06-05-release-tag-PLAN.md:1-39)

```yaml
---
phase: 06-installer-overwrite-running-shim
plan: 05
type: execute
wave: 4
depends_on:
  - 06-01-shutdown-signal-package-PLAN
  - 06-02-main-go-wiring-PLAN
  - 06-03-nsis-preinstall-shim-PLAN
  - 06-04-docs-update-PLAN
files_modified: []
autonomous: false
requirements: [INST-06]
tags: [release, ci, tag, latest-json, github-release]

must_haves:
  truths:
    - "A new git tag `v1.0.1` exists on origin/master at a commit that contains all four prior plan's changes (Plans 01-04)."
    # ... etc
```

Mirror exactly for 09-05:
- `phase: 09-watcher-robustness-polish`
- `plan: 05`
- `wave: 3` (per CONTEXT.md D-05 — three waves, not four)
- `depends_on:` lists 09-01..09-04
- `requirements: [AUTH-07, OPS-06, OPS-07, CONFIG-01]` (or `[]` with the ship-gate convention — 06-05 uses `[INST-06]` even though it's a release plan, mirroring is fine)
- `tags: [release, ci, tag, latest-json, github-release]` unchanged

#### Tag + push + monitor pattern (06-05-release-tag-PLAN.md:182-264 — Task 3)

```
1. Tag the current HEAD:
   git tag -a v1.0.1 -m "Phase 6 (INST-06): installer overwrite-running shim. Watcher binary v1.0.1 release."

2. Push the tag:
   git push origin v1.0.1

3. Monitor the workflow run:
   gh run watch --exit-status

4. After the run completes successfully, verify:
   gh release view v1.0.1

5. Verify the latest.json contents:
   gh release download v1.0.1 -p latest.json -O /tmp/latest.json
   cat /tmp/latest.json
```

Mirror exactly for v1.0.2: substitute `v1.0.1` → `v1.0.2`, tag message → `"Phase 9 (AUTH-07/OPS-06/OPS-07/CONFIG-01): watcher robustness polish. Watcher binary v1.0.2 release."`

#### Pre-tag readiness sweep pattern (06-05-release-tag-PLAN.md:80-131 — Task 1)

```
1. git status — working tree clean.
2. git log --oneline -20 — Plans 01-04 commits are present.
3. git branch --show-current — confirm we are on master.
4. go build ./... exits 0.
5. go vet ./... exits 0.
6. go test ./... exits 0.
7. Select-String -Path cmd/squirebot/main.go -Pattern 'os\.Args\[1\] == "--quit"' matches 1 line (Plan 02 shipped).
...
```

Mirror for 09-05 with NEW grep gates (per CONTEXT.md D-06 — schema-impact assertion):
- `grep -nE 'WatcherMaxSchemaVersion\s*=\s*3' internal/sheet/client.go` must match (UNCHANGED — schema stays at 3)
- `grep -nE 'SCRIPT_MIN_SCHEMA_VERSION.*3' apps-script/src/lib/migrations.ts` must match (UNCHANGED)

Plus new Phase 9 file-state gates, one per fix:
- `Select-String -Path internal/tray/tray.go -Pattern 'pending'` matches ≥1 (OPS-06 queue field present)
- `Select-String -Path cmd/squirebot/console_windows.go -Pattern 'FreeConsole'` matches 1 (OPS-07 file exists)
- `Select-String -Path internal/config/config.go -Pattern 'TrimPrefix.*0xEF.*0xBB.*0xBF'` matches 1 (CONFIG-01 fix present)
- `Select-String -Path internal/app/runapp.go -Pattern 'IsRevokedRefreshToken.*err' -Context 0,2` matches ≥2 (AUTH-07 boot wiring added; AUTH-05's existing isPermanentAuthErr already has 1)

#### Existing release.yml shape (release.yml:39-46)

```yaml
on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:
    inputs:
      version:
        description: "Version (e.g., 0.1.0) -- omit the leading 'v'."
        required: true
```

No changes needed — pushing `v1.0.2` triggers the same path as `v1.0.1` did. The `Compute version` step (release.yml:102-126) parses `v1.0.2` → `version=1.0.2`, `numeric_version=1.0.2.0`. All assets are named with the version interpolation; no hardcoded `1.0.1` anywhere.

#### Version constant — no source change needed at tag time

`cmd/squirebot/build_constants.go:29`:

```go
Version           = "0.1.0-dev"
```

This is the **fallback for local-dev builds without ldflags**. Release CI overrides it (release.yml:162-166):

```yaml
$ldflags = "-H=windowsgui -s -w " +
           "-X main.OAuthClientID=$env:OAUTH_CLIENT_ID " +
           # ...
           "-X main.Version=${{ steps.ver.outputs.version }}"
```

So the v1.0.2 tag becomes the version through the ldflag chain. Bumping `"0.1.0-dev"` is **optional** — Phase 6 didn't do it for `v1.0.1`. Planner picks; CONTEXT.md D-08 doesn't require it. The release tag itself is the source-of-truth.

**Differs from analog:**
- Plan count under `depends_on:` is 4 fix plans (09-01..09-04), not Phase 6's 4 fix plans (06-01..06-04). Structurally identical.
- Phase 9 ships pure binary changes — no installer changes (NSIS shim already shipped in v1.0.1, untouched). Phase 6's plan referenced `installer/squirebot.nsi` grep gates; 09-05's gates target the four new fix files instead.
- 09-05's UAT (Task 4 in the analog) needs to verify **all four fixes** in a single VM run, vs. Phase 6's single INST-06 verification. Suggested UAT outline:
  1. Pre-stage: revoke the test guildie's OAuth grant in Google account settings (creates AUTH-07 reproduction state).
  2. Auto-update from v1.0.1 → v1.0.2.
  3. After update, tray icon should be **red with visible Reauthorize from boot** (AUTH-07 + OPS-06 acceptance).
  4. Click Reauthorize, complete flow → tray goes green.
  5. Hand-edit `config.json` to insert a leading UTF-8 BOM with Notepad → restart → watcher starts cleanly (CONFIG-01 acceptance).
  6. Launch `squirebot.exe` from a `& exe` PowerShell session → close the shell → tray persists, log file shows watcher still alive (OPS-07 acceptance).

---

## Shared Patterns

### Build-tag pairs for Windows-only syscalls
**Source:** `internal/system/shutdown_signal_windows.go:1-11` + `internal/system/shutdown_signal_other.go` (full file)
**Apply to:** Plan 09-02 (`cmd/squirebot/console_windows.go` + `cmd/squirebot/console_other.go`)

Both files in package `system` use exact `//go:build windows` and `//go:build !windows` headers, splitting the Windows-only `golang.org/x/sys/windows` import out of the cross-platform file. This is the canonical pattern in the codebase.

### Structured logging via slog (CLAUDE.md convention)
**Source:** every Go file under `internal/`
**Apply to:** all Plan 09 source changes
Examples in scope:
- `runapp.go:88` — `slog.Error("wizard failed", "err", res.Err)`
- `runapp.go:495` — `slog.Info("uploaded", "char", charName, "rows", len(rows))`
- `reauth.go:127` — `slog.Info("Reauthorize start", "email", cfg.GoogleEmail)`

Pattern: `slog.<Level>("<event description>", "<key>", value, ...)`. Never `slog.Errorf`. Keys are snake_case. Never log refresh tokens, access tokens, or full OAuth URLs (CONTEXT.md PITFALL § + reauth.go:156-157 comment).

### Goroutine-safe mutator surface guarded by struct mutex
**Source:** `internal/tray/tray.go:96-117` + `internal/tray/tray.go:302-313` (SetSpreadsheetID/SpreadsheetID accessor pair)
**Apply to:** Plan 09-01 (new pending queue + `ready` flag share the same `t.mu`)

```go
func (t *Controller) SetSpreadsheetID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spreadsheetID = id
}
```

Don't introduce a second mutex. The existing `sync.Mutex` at `Controller.mu` (line 97) is the lock all goroutine-safe state goes under. OPS-06's `ready` + `pending []func()` (or `[]pendingAction`) join it.

### Table-driven Go tests
**Source:** `internal/config/config_test.go:122-260` and `internal/app/runapp_test.go:16-93`
**Apply to:** Plans 09-01, 09-03, 09-04 test additions

Pattern: cases slice with named entries, `t.Run(tc.name, ...)` per case, single-line `t.Errorf` on mismatch. Use `withTempConfig(t)` for I/O tests. Don't introduce vitest-style frameworks or testify.

### atomic.Bool for cross-goroutine flags
**Source:** `internal/app/reauth.go:83` (`var globalAuthSuspended atomic.Bool`) + `internal/app/reauth.go:99` (`globalPostReauthPending`)
**Apply to:** N/A for Phase 9 — AUTH-07 boot path has no concurrent reader; the pattern is here for awareness only (in case planner thinks AUTH-07 needs a flag — it doesn't, because boot is single-threaded).

---

## No Analog Found

None. All 8 files have direct in-repo analogs (mostly self-extension).

## Metadata

**Analog search scope:** `cmd/squirebot/`, `internal/tray/`, `internal/app/`, `internal/auth/`, `internal/config/`, `internal/system/`, `internal/update/`, `.github/workflows/`, `.planning/phases/06-installer-overwrite-running-shim/`
**Files scanned:** 14 Go source files, 2 Go test files, 1 YAML workflow, 1 PLAN.md template
**Pattern extraction date:** 2026-05-12
