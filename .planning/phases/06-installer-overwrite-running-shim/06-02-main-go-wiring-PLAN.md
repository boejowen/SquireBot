---
phase: 06-installer-overwrite-running-shim
plan: 02
type: execute
wave: 2
depends_on: [06-01-shutdown-signal-package-PLAN]
files_modified:
  - cmd/squirebot/main.go
autonomous: true
requirements: [INST-06]
tags: [cli-flag, shutdown, listener-goroutine, windows]

must_haves:
  truths:
    - "Running `squirebot.exe --quit` on a host where a watcher is active causes the active watcher to invoke its existing `OnQuit` shutdown path (cancel() + systray.Quit()) and exit cleanly within 1 second."
    - "Running `squirebot.exe --quit` on a host where NO watcher is active is a benign no-op: the invocation exits 0 within 1 second and never starts a wizard, tray, or background work."
    - "An unknown CLI flag (`squirebot.exe --bogus`) STILL falls through to normal launch (preserves existing v1.0.0 behavior; we do NOT add unknown-flag rejection in this phase)."
    - "The normal-launch path now spawns a single listener goroutine that funnels through the same `cancel() + systray.Quit()` shutdown sequence as the tray's Quit menu — no `os.Exit()` shortcut, no duplicate cleanup."
  artifacts:
    - path: cmd/squirebot/main.go
      provides: "--quit flag handler (BEFORE update.Apply, AFTER --uninstall-wipe-credentials block) + listener goroutine (AFTER RunApp launch, BEFORE slog.Info('squirebot starting'))"
      contains: "system.SignalShutdown|system.WaitForShutdown"
  key_links:
    - from: cmd/squirebot/main.go
      to: internal/system
      via: "import + two call sites (SignalShutdown in --quit handler, WaitForShutdown in listener goroutine)"
      pattern: "github.com/boejowen/SquireBot/internal/system"
    - from: cmd/squirebot/main.go (listener goroutine)
      to: cmd/squirebot/main.go (line ~140 OnQuit callback)
      via: "shared shutdown funnel — both call cancel() + (listener also calls systray.Quit())"
      pattern: "cancel\\(\\)"
---

<objective>
Wire the `internal/system` package shipped in Plan 01 into `cmd/squirebot/main.go` at two specific insertion points:

1. **`--quit` CLI flag handler** — immediately AFTER the existing `--uninstall-wipe-credentials` block (current main.go line 54), BEFORE `update.Apply()` (current line 69). Mirrors the structural template of `--uninstall-wipe-credentials` exactly: `os.Args[1] == "--quit"`, stderr-only logging (before `logging.Setup`), `os.Exit(0)` on every branch including errors.

2. **Named-event listener goroutine** — immediately AFTER `go app.RunApp(ctx, cfg, bc, trayCtl)` (current line 145), BEFORE the `slog.Info("squirebot starting", ...)` block (current line 147). The goroutine blocks on `<-system.WaitForShutdown(ctx)` and, on signal, calls BOTH `cancel()` AND `systray.Quit()` (the listener fires from OUTSIDE the tray click handler, so it must invoke `systray.Quit()` explicitly to unblock `systray.Run` on line 161 — the existing `OnQuit` callback only needs to call `cancel()` because `internal/tray/tray.go:234` does `systray.Quit()` itself).

Purpose: completes the watcher-side half of the graceful-shutdown contract. After this lands, the NSIS shim (Plan 03) can `ExecWait '"$INSTDIR\squirebot.exe" --quit'` and observe the watcher exit within the 10s timeout.

Output: a modified `cmd/squirebot/main.go` (one file, two diff regions, ~30 inserted lines) plus one `system` import added to the existing import block.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md
@.planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md
@.planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md
@cmd/squirebot/main.go
@internal/tray/tray.go

<interfaces>
<!-- From Plan 01 (now shipped). The executor consumes these exact signatures. -->

```go
package system

func SignalShutdown() error
func WaitForShutdown(ctx context.Context) <-chan struct{}
```

<!-- Existing shutdown funnel that the listener MUST mirror (do NOT invent new shutdown semantics): -->

```go
// cmd/squirebot/main.go OnQuit callback (lines 138-141 today):
OnQuit: func() {
    slog.Info("Quit clicked — cancelling root context")
    cancel()
},

// internal/tray/tray.go:230-235 (Quit menu handler):
case _, ok := <-t.mQuit.ClickedCh:
    ...
    slog.Info("Quit clicked")
    if t.onQuit != nil {
        t.onQuit()       // -> calls cancel()
    }
    systray.Quit()       // <-- tray.go calls this AFTER onQuit
    return
```

The listener goroutine fires OUTSIDE the tray click handler, so it MUST do both halves (cancel + systray.Quit) explicitly to unblock systray.Run on main.go:161.
</interfaces>

<insertion_anchors>
<!-- EXACT line anchors in cmd/squirebot/main.go AS OF this plan's authoring (2026-05-11). The executor MUST re-grep before inserting; line numbers may shift after Wave 1 plans land (Wave 1 does not touch main.go, so they should be stable). -->

Anchor A — `--quit` handler insertion point:
  - Insert AFTER the closing `}` of the `--uninstall-wipe-credentials` block (currently line 54: `}`).
  - Insert BEFORE the `// Plan 02-06 (OPS-04) startup-swap:` comment block (currently line 56).
  - Leave one blank line between the new block and each surrounding block.

Anchor B — listener goroutine insertion point:
  - Insert AFTER `go app.RunApp(ctx, cfg, bc, trayCtl)` (currently line 145).
  - Insert BEFORE the `slog.Info("squirebot starting", ...)` call (currently line 147).
  - Leave one blank line between the new block and each surrounding block.

Anchor C — `system` package import insertion point:
  - Insert in the existing import block (lines 8-23) in the project's internal-import alphabetical group (current order: app, auth, config, logging, tray, update — line 17-22).
  - The new import goes BETWEEN `internal/logging` (line 20) and `internal/tray` (line 21) to keep alphabetical order (logging < system < tray).
  - Exact line: `"github.com/boejowen/SquireBot/internal/system"`.
</insertion_anchors>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add `--quit` CLI flag handler + `system` import</name>
  <files>cmd/squirebot/main.go</files>
  <read_first>
    - cmd/squirebot/main.go (entire file, especially lines 25-77 — the existing `--uninstall-wipe-credentials` handler is the structural template)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (section "MODIFY cmd/squirebot/main.go — --quit flag handler block")
    - .planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md (D-01 implementation sketch lines 96-99)
    - .planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md (the exact SignalShutdown signature shipped in Plan 01)
  </read_first>
  <behavior>
    - `squirebot.exe --quit` invocation calls `system.SignalShutdown()` and exits 0 within 1 second.
    - The handler runs BEFORE `update.Apply()` (auto-update has no business running during a `--quit` invocation).
    - The handler runs BEFORE `logging.Setup` — all output MUST go to `os.Stderr` via `fmt.Fprintf` / `fmt.Fprintln`, never `slog`.
    - On error from `SignalShutdown`: log to stderr, exit 0 (NSIS falls back to `taskkill /F` regardless — never block the installer on signal failure).
    - The import block stays alphabetized: `system` goes between `logging` and `tray`.
  </behavior>
  <action>
**Step 1: Add the `system` import.**

In the existing import block (currently lines 8-23), insert a new line:

```go
	"github.com/boejowen/SquireBot/internal/system"
```

between line 20 (`"github.com/boejowen/SquireBot/internal/logging"`) and line 21 (`"github.com/boejowen/SquireBot/internal/tray"`). After insertion the internal-import alphabetical order is: app, auth, config, logging, system, tray, update.

**Step 2: Add the `--quit` handler block.**

Insert immediately AFTER line 54 (the closing `}` of the `--uninstall-wipe-credentials` block) and BEFORE the `// Plan 02-06 (OPS-04) startup-swap:` comment on line 56. Use exactly this code (paste verbatim, do NOT paraphrase):

```go

	// Plan 06 (INST-06): --quit. Invoked by the NSIS pre-install shim to
	// gracefully stop a running watcher before file overwrite. Opens the
	// Local\SquireBot-Shutdown named event and signals it; the running
	// instance's listener goroutine observes the signal and unwinds
	// through cancel() + systray.Quit(). This invocation exits 0 always
	// — a signal with no listener is a benign no-op per D-01, and NSIS
	// falls back to taskkill /F on timeout regardless of any error here.
	//
	// Runs FIRST (before update.Apply) — auto-update has no business
	// firing during a --quit signal invocation. Logging is not yet set
	// up; use stderr for all output (matches --uninstall-wipe-credentials
	// and update.Apply's stderr-only contract).
	if len(os.Args) >= 2 && os.Args[1] == "--quit" {
		if err := system.SignalShutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown signal failed: %v\n", err)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "shutdown signal sent")
		os.Exit(0)
	}
```

Conventions enforced (from PATTERNS.md "MODIFY cmd/squirebot/main.go — --quit flag handler block"):
- `len(os.Args) >= 2 && os.Args[1] == "--quit"` — NOT `flag.Parse()` (main.go avoids the `flag` package; consistency).
- `os.Exit(0)` on EVERY exit branch including errors (matches `--uninstall-wipe-credentials` line 50 rationale: "the uninstaller must not block on a guildie who never completed the wizard but ran the installer" — same logic here).
- `fmt.Fprintf(os.Stderr, ...)` and `fmt.Fprintln(os.Stderr, ...)` — NOT `slog` (runs before `logging.Setup` on line 78).
- Comment leads with `// Plan 06 (INST-06):` matching the `// Plan 02-07 (INST-04 / CONTEXT.md Q3):` style of the surrounding code.
- Place IMMEDIATELY after the `--uninstall-wipe-credentials` block, BEFORE the auto-update block. This is non-negotiable per CONTEXT.md D-01 (signal handler must run before any goroutine spawns or any I/O happens).

**Do NOT modify:**
- The `--uninstall-wipe-credentials` block (lines 38-54).
- The auto-update block (lines 56-76).
- The `logging.Setup()` call (line 78).
- Any code after line 78 in this task — the listener goroutine (Task 2) handles that.
  </action>
  <verify>
    <automated>
      # shell: bash
      cd /c "C:/Users/Virus Canary/Desktop/Claude/SquireBot" && go build ./cmd/squirebot 2>&1 | tee /tmp/build-quit.log && go vet ./cmd/squirebot 2>&1 | tee -a /tmp/build-quit.log && grep -c "error" /tmp/build-quit.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path cmd/squirebot/main.go -Pattern '"github\.com/boejowen/SquireBot/internal/system"'` matches exactly 1 line in the import block.
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'os\.Args\[1\] == "--quit"'` matches exactly 1 line.
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'system\.SignalShutdown\(\)'` matches exactly 1 line in this task's scope (Task 2 adds the WaitForShutdown call later).
    - `Select-String -Path cmd/squirebot/main.go -Pattern '// Plan 06 \(INST-06\): --quit'` matches exactly 1 line (lead comment with REQ-ID).
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'shutdown signal sent'` matches exactly 1 line (the success-path stderr message).
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'shutdown signal failed: %v'` matches exactly 1 line (the error-path stderr message).
    - Insertion order check: grep `os.Args[1] == "--uninstall-wipe-credentials"` line N1; grep `os.Args[1] == "--quit"` line N2; grep `update.Apply()` line N3. Verify N1 < N2 < N3 (handler ordering enforced).
    - `Select-String -Path cmd/squirebot/main.go -Pattern '"--quit"' -Context 0,5 | Select-String -Pattern 'slog\.'` matches zero lines (no slog in the --quit handler block; stderr only because logging.Setup hasn't run yet).
    - `go build ./cmd/squirebot` exits 0.
    - `go vet ./cmd/squirebot` exits 0.
  </acceptance_criteria>
  <done>
    The `--quit` flag handler is in place, the `system` import is added in alphabetical order, the build is clean, and no `slog` calls leak into the pre-`logging.Setup` code path. The listener goroutine in Task 2 will close the loop.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add named-event listener goroutine</name>
  <files>cmd/squirebot/main.go</files>
  <read_first>
    - cmd/squirebot/main.go (after Task 1's edits — re-read; especially the OnQuit callback around line 138-141, the `go app.RunApp` line ~145, and the `systray.Run` call ~line 161)
    - internal/tray/tray.go lines 226-238 (the Quit menu handler — the listener mirrors its `cancel() + systray.Quit()` sequence)
    - .planning/phases/06-installer-overwrite-running-shim/06-PATTERNS.md (section "MODIFY cmd/squirebot/main.go — named-event listener goroutine")
    - .planning/phases/06-installer-overwrite-running-shim/06-CONTEXT.md (D-03 shutdown semantics — abandon in-flight writes, no drain coordination)
    - .planning/phases/06-installer-overwrite-running-shim/06-01-SUMMARY.md (WaitForShutdown signature)
  </read_first>
  <behavior>
    - On normal launch, a single new goroutine is spawned that blocks on `<-system.WaitForShutdown(ctx)` AND `<-ctx.Done()` in a select.
    - When the named event fires (typically from `squirebot.exe --quit`): goroutine calls `slog.Info("shutdown signal received — cancelling root context")`, then `cancel()`, then `systray.Quit()`. Process exits cleanly via the existing main-goroutine flow (`systray.Run` returns â†’ main returns).
    - When ctx is cancelled by ANOTHER path (tray Quit, OnQuit callback): the goroutine's `<-ctx.Done()` arm fires and returns; no double-cancel, no double-systray.Quit.
    - The listener funnels through the EXACT SAME shutdown path as the tray Quit menu. No `os.Exit()` calls, no new shutdown semantics.
    - Double-fire safety: if both the tray Quit AND a `--quit` signal race, `cancel()` on an already-cancelled ctx is a no-op and `systray.Quit()` is internally idempotent in `fyne.io/systray` (verified per PATTERNS.md note).
  </behavior>
  <action>
**Step 1: Locate the insertion point.**

After Task 1's import addition, find the line `go app.RunApp(ctx, cfg, bc, trayCtl)` (currently main.go line 145 BEFORE Task 1; line numbers shift by 0 because Task 1's insertions are above line 78). The listener goroutine MUST be inserted on the line(s) immediately BELOW this `go app.RunApp(...)` call and ABOVE the `slog.Info("squirebot starting", ...)` block.

**Step 2: Insert the listener goroutine.**

Paste exactly this block, verbatim:

```go

	// Plan 06 (INST-06): named-event shutdown listener. Blocks on
	// Local\SquireBot-Shutdown; on signal, funnels through the SAME path
	// as the tray's Quit menu (cancel() + systray.Quit()). Idempotent —
	// double-fire (tray Quit + installer --quit racing) is harmless
	// because systray.Quit is internally idempotent and cancel() on an
	// already-cancelled ctx is a no-op. Goroutine exits on either signal
	// OR ctx.Done so it cannot leak when shutdown comes from another path.
	//
	// Per D-03: no drain coordination. In-flight batchUpdate calls
	// observe ctx cancellation through the existing mutex-funneled
	// sheet.Client retry envelope and abandon. WATCH-09 catch-up
	// re-uploads any missed file changes on next launch.
	go func() {
		select {
		case <-system.WaitForShutdown(ctx):
			slog.Info("shutdown signal received — cancelling root context")
			cancel()
			systray.Quit()
		case <-ctx.Done():
			// Normal shutdown from another path (tray Quit, OS signal).
			// WaitForShutdown's internal goroutine also observes ctx.Done
			// and cleans up its event handle via defer.
			return
		}
	}()
```

**Conventions enforced (from PATTERNS.md "MODIFY cmd/squirebot/main.go — named-event listener goroutine"):**
- The slog message style matches the surrounding code (`"Quit clicked — cancelling root context"`, `"Reauthorize clicked — running OAuth flow"` — short imperative messages with em-dash).
- MUST call BOTH `cancel()` AND `systray.Quit()` in the signal arm. The existing `OnQuit` callback only calls `cancel()` because `internal/tray/tray.go:234` does `systray.Quit()` itself after invoking the callback. The listener fires from OUTSIDE that click handler, so it MUST do both.
- DO NOT call `os.Exit()` from the listener — the existing contract is `cancel() + systray.Quit() â†’ systray.Run returns on line 161 â†’ main exits`. Plumbing through that path preserves the `defer cancel()` on line 96 and the `slog.Info("squirebot exit")` on line 165.
- Lead comment references `Plan 06 (INST-06)` and `D-03` for traceability — matches the surrounding `Plan 02-06 (OPS-04)` comment style.
- The listener uses `slog` (NOT stderr) because by this point in main.go `logging.Setup` has already run (line 78, well before line ~145).

**Do NOT modify:**
- The existing `OnQuit` callback (lines 138-141). It stays as-is — only does `cancel()` because the tray.go code path will call `systray.Quit()` after invoking it.
- The `systray.Run(trayCtl.OnReady, trayCtl.OnExit)` call (line ~161). This still blocks main; `systray.Quit()` from the listener is what unblocks it on signal.
- The `defer cancel()` on line 96. The listener's `cancel()` is a second invocation on the same cancel func; Go cancel funcs are safe to call multiple times.
  </action>
  <verify>
    <automated>
      # shell: bash
      cd /c "C:/Users/Virus Canary/Desktop/Claude/SquireBot" && go build ./cmd/squirebot 2>&1 | tee /tmp/build-listener.log && go vet ./cmd/squirebot 2>&1 | tee -a /tmp/build-listener.log && go build ./... 2>&1 | tee -a /tmp/build-listener.log && grep -c "error" /tmp/build-listener.log
    </automated>
  </verify>
  <acceptance_criteria>
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'system\.WaitForShutdown\(ctx\)'` matches exactly 1 line.
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'shutdown signal received — cancelling root context'` matches exactly 1 line.
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'systray\.Quit\(\)'` matches at least 1 line in the listener goroutine block (search within 15 lines after the `system.WaitForShutdown` line).
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'case <-ctx\.Done\(\):'` matches at least 1 line in the listener goroutine block.
    - `Select-String -Path cmd/squirebot/main.go -Pattern '// Plan 06 \(INST-06\): named-event shutdown listener'` matches exactly 1 line.
    - Insertion order check: grep `go app.RunApp(` line N1; grep `system.WaitForShutdown(ctx)` line N2; grep `slog.Info\("squirebot starting"` line N3. Verify N1 < N2 < N3.
    - `Select-String -Path cmd/squirebot/main.go -Pattern 'os\.Exit' -Context 2,2 | Select-String -Pattern 'WaitForShutdown'` returns zero matches (NO os.Exit anywhere near the listener — the funnel through cancel+systray.Quit is mandatory per D-03).
    - `go build ./cmd/squirebot` exits 0.
    - `go build ./...` exits 0 (whole-tree cross-compile check).
    - `go vet ./...` exits 0.
    - Total `system\.SignalShutdown\(\)` call sites in main.go = 1 (Task 1's --quit handler); total `system\.WaitForShutdown` call sites = 1 (this task).
  </acceptance_criteria>
  <done>
    The listener goroutine is wired in, builds clean, and routes shutdown signals through the canonical `cancel() + systray.Quit()` funnel. End-to-end Go-side INST-06 path is complete: external `squirebot.exe --quit` â†’ SignalShutdown â†’ named event â†’ listener goroutine â†’ graceful exit. Plan 03 (NSIS shim) can now `ExecWait '"$INSTDIR\squirebot.exe" --quit'` and observe the exit.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| CLI argv â†’ process behavior | `os.Args[1]` is parsed by string compare; matched arg triggers a behavior change before any other initialization. |
| External signal â†’ shutdown | The named-event listener accepts a shutdown trigger from any process in the same user session that can OpenEvent the `Local\` event. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-06-06 | Spoofing | `--quit` CLI arg invoked by a hostile user-session process | accept | Same as T-06-01 in Plan 01: user-session integrity bounds the attack surface. Worst outcome is graceful watcher exit + WATCH-09 catch-up on next launch. No data loss. The handler only invokes `SignalShutdown` — does not load config, does not touch credentials, does not write to any file. |
| T-06-07 | Tampering | Unknown CLI flags fall through to normal launch (existing behavior, preserved) | accept | This is the documented v1.0.0 behavior and CONTEXT.md D-02 explicitly relies on it for the version-gate logic. We do NOT add `flag.Parse()` strict mode in this phase — that would be a behavior change requiring its own decision. Documented as deferred idea in CONTEXT.md (single-instance enforcement). |
| T-06-08 | Repudiation | Shutdown signal source is not logged with identifying info | accept | The slog line `"shutdown signal received — cancelling root context"` is intentionally generic. The watcher cannot reliably attribute the signal to a specific process (Windows named events are not signed). Audit needs are met by the INSTALLER LOG (NSIS) showing the ExecWait '--quit' invocation. |
| T-06-09 | Denial of Service | Repeated `--quit` invocations terminate the watcher repeatedly | mitigate | The autostart Run-key launches the watcher at next logon. A persistent DoS would require continuous attacker presence in the user session; if achieved, the attacker has full user-integrity code execution already (see T-06-01). |
| T-06-10 | Elevation of Privilege | Signal handler runs with elevated privileges if main.go is somehow invoked with admin | accept | The watcher is launched by HKCU autostart (user integrity) and the NSIS installer runs at user integrity (`RequestExecutionLevel user`). There is no documented path that elevates the watcher; this threat is theoretical. The handler does no privileged operations regardless (only signals an event + exits). |
| T-06-11 | Tampering / Lifecycle | Listener goroutine leaks if both shutdown paths fire and cancel/systray.Quit are not idempotent | mitigate | The listener's `select` exits on whichever arm fires first. `cancel()` on an already-cancelled ctx is documented safe (`context.CancelFunc` contract). `systray.Quit()` is internally idempotent in `fyne.io/systray` (verified per PATTERNS.md note from Phase 1 prior art). The `<-ctx.Done()` arm specifically exists to avoid the goroutine blocking forever on `WaitForShutdown` after a tray-driven shutdown — covered by Plan 01's TestCtxCancelClosesChannel. |
| T-06-20 | Race / Lifecycle | Listener goroutine fires before `systray.Run` binds — `systray.Quit()` pre-Run is library-internal in fyne.io/systray v1.10.0 | accept | (a) Installer takes seconds before signaling; the watcher's normal boot reaches `systray.Run` sub-second after the listener goroutine is spawned, so the race window is vanishingly small in the real shutdown path. (b) `cancel()` is the primary shutdown trigger and is unaffected by systray state — once the root context is cancelled, `app.RunApp`'s downstream goroutines unwind regardless. (c) If `systray.Quit()` is a pre-Run no-op, the cancelled context propagates through `app.RunApp` and main returns on the goroutine's natural unwind path. No code change required; documented for future investigation if v1.0.1 soak surfaces a hang. |

ASVS L1: no `high` severity threats. Mitigations leverage existing Phase 1 invariants (`RequestExecutionLevel user`, HKCU-only autostart) and Plan 01 lifecycle tests.
</threat_model>

<verification>
- `go build ./cmd/squirebot` exits 0.
- `go build ./...` exits 0 (cross-package cross-compile sanity).
- `go vet ./...` exits 0.
- `go test ./...` exits 0 (Plan 01 tests still pass; main.go has no new tests but no regressions either).
- Manual smoke (developer's Windows box, OPTIONAL but recommended):
  1. `go build -o dist/squirebot-dev.exe ./cmd/squirebot`
  2. `dist/squirebot-dev.exe` in one terminal — wizard or tray launches.
  3. `dist/squirebot-dev.exe --quit` in another terminal — first invocation exits 0 with "shutdown signal sent" on stderr; the running watcher exits cleanly (tray icon disappears, no crash dialog).
- Manual smoke (no listener case): `dist/squirebot-dev.exe --quit` with no prior watcher running — exits 0 with "shutdown signal sent" on stderr, does NOT spawn a wizard/tray.
</verification>

<success_criteria>
- `cmd/squirebot/main.go` has both insertions: `--quit` handler (line ~56 area, before `update.Apply()`) and listener goroutine (line ~147 area, after `go app.RunApp`).
- The `internal/system` import is added in alphabetical position (between `logging` and `tray`).
- Whole-tree build + vet + test all green.
- ROADMAP Â§45 success criterion 2 fully covered on the Go side: "signals `squirebot.exe` to exit gracefully" + "waits for it" (the wait is delivered by the listener invoking `cancel() + systray.Quit()`, which makes the main goroutine return and the process exit). Plan 03 (NSIS) ships the "falls back to a hard kill if the process does not exit within the timeout" half.
- D-01, D-03 honored: named event mechanism, no drain coordination, listener funnels through the canonical shutdown path (no `os.Exit`).
- D-04 honored: post-install relaunch unchanged (this plan does NOT touch the NSIS Exec line).
</success_criteria>

<output>
After completion, create `.planning/phases/06-installer-overwrite-running-shim/06-02-SUMMARY.md` capturing:
- The two insertion points with their final line numbers (post-insert).
- Confirmation that the `--uninstall-wipe-credentials` block + auto-update block + OnQuit callback are unchanged.
- Manual smoke result (running watcher exits cleanly on `--quit`) if performed.
- The exact CLI contract Plan 03 will invoke: `ExecWait '"$INSTDIR\squirebot.exe" --quit'` — exits 0 within ~1s, idempotent, safe if no listener.
</output>
</content>
</invoke>