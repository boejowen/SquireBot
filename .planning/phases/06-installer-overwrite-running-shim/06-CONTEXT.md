# Phase 6: Installer Overwrite-Running Shim — Context

**Gathered:** 2026-05-11
**Status:** Ready for planning (research flag optional — NSIS pre-install patterns + Windows named-event signaling from Go are both well-trodden; planner may skip /gsd-research if comfortable)
**Mode:** discuss (user delegated all decisions to Claude with the directive "make whichever choices you think will make the installer as simple and automatic as possible for end users")

---

## Why this phase exists (one paragraph)

Phase 6 closes the last installer rough edge from the v1.0 ship: NSIS cannot replace a `.exe` that is currently running, so the current `docs/troubleshooting.md` § "Installer won't overwrite a running SquireBot" tells guildies to manually right-click the tray and Quit before re-running the installer. That manual step is precisely the kind of friction the v1.0 setup-ceiling decision said the project would not tolerate ("click the installer, click Allow, click Allow"). It is also the only known reason a guildie would ever fail to upgrade, which matters because every future SquireBot release ships via this same installer (auto-update covers the in-process self-swap path, but manual re-installs and SignPath-OSS-signed re-distributions still hit the installer). Phase 6 makes the installer detect a running watcher, signal it to quit gracefully, fall back to a hard kill if needed, then proceed with the file overwrite — invisibly. The user sees the tray icon flicker; nothing else changes.

<domain>
## Phase Boundary

**In scope (per ROADMAP §40-50 + REQUIREMENTS.md INST-06):**

- **NSIS pre-install shim** — detect running `squirebot.exe`, signal graceful exit, wait, fall back to hard kill on timeout, then File overwrite.
- **Watcher-side `--quit` flag** — new CLI handler in `cmd/squirebot/main.go` that opens a Windows named event and signals it (so a second `squirebot.exe --quit` invocation can shut down the first instance without IPC plumbing or NSIS plugins).
- **Watcher-side named-event listener** — a small goroutine started during normal launch that waits on `Local\SquireBot-Shutdown` and invokes the existing `OnQuit` (`cancel()` + `systray.Quit()`) when signaled.
- **NSIS version gate for v1.0.0 → v1.0.1** — reads `DisplayVersion` from `HKCU\…\Uninstall\SquireBot`; if `< 1.0.1`, skip the `--quit` step and go straight to `taskkill /F` (the v1.0.0 binary would spawn a duplicate tray on an unknown CLI flag — see Constraint #1 below).
- **Post-install relaunch unchanged** — keeps the existing `Exec '"$INSTDIR\${EXE_NAME}"'` line. User sees a brief tray flicker; new binary takes over within seconds.
- **Troubleshooting doc update** — remove the "Installer won't overwrite a running SquireBot" section from `docs/troubleshooting.md` (lines 50-58 in the file as of 2026-05-11). The manual workaround is retired.
- **Watcher binary v1.0.1 release** — build + tag + GitHub Release via existing `.github/workflows/release.yml`; updated `latest.json` so already-installed watchers can self-update through the existing `minio/selfupdate` startup-swap path.

**Out of scope (deferred to later phases / backlog):**

- **Single-instance enforcement** (prevent accidental double-launch via two-tray-icons scenario). The named-event infrastructure landing here makes this cheap, but actively *refusing* a second startup is a behavior change with its own UX questions (silent exit? toast? "another instance is running" dialog?). Defer to a future hardening phase; capture as deferred idea below.
- **Auto-update flow improvements.** `minio/selfupdate` startup-swap is the OTHER upgrade path and already works (Plan 02-06). This phase does not touch it.
- **NSIS uninstaller graceful path.** The uninstaller's existing `taskkill /F` on line 136 of `installer/squirebot.nsi` is fine — uninstall is a one-way operation and the user explicitly clicked Uninstall. Don't gold-plate it.
- **SignPath OSS signing** (backlog item 999.9) — orthogonal; lands as a hotfix when Foundation review completes.

**Explicitly NOT a Phase 6 ambiguity (defaulted by Claude per user directive):**

All four gray areas surfaced during scout were defaulted toward end-user simplicity. See `<decisions>` below for the full rationale; the short version:

- **Signal mechanism:** named Windows event opened/signaled by `squirebot.exe --quit`. No new IPC server, no NSIS plugin, all Go.
- **v1.0.0 legacy path:** version-gated; skip `--quit` for pre-1.0.1 installs (taskkill /F directly).
- **Shutdown semantics:** abandon in-flight writes (WATCH-09 catch-up handles re-upload on next launch); 10s timeout before hard-kill fallback.
- **Post-install launch:** unchanged from current installer behavior (always relaunch).

</domain>

<canonical_refs>
## Canonical refs (downstream MUST read before planning)

| Path | Why |
|------|-----|
| `.planning/ROADMAP.md` § "Phase 6: Installer Overwrite-Running Shim" | The 5 success criteria are the ship gate. Especially criteria 4 (troubleshooting.md edit) and 5 (binary + tag + latest.json) which are easy to forget. |
| `.planning/REQUIREMENTS.md` § INST-06 | The single requirement this phase covers. |
| `installer/squirebot.nsi` | The file being modified. Pre-install section is currently empty; this is where the shim lands. Note `RequestExecutionLevel user` (line 48) — pre-install must not assume admin. Note also that the uninstaller (lines 108-163) already does `taskkill /F` so the prior art for hard-kill is right there. |
| `cmd/squirebot/main.go` lines 38-54 | Where the existing `--uninstall-wipe-credentials` flag-handler block lives. The new `--quit` handler goes adjacent. Critical: as written today (line 38), any unknown flag falls through into a normal launch — that is why v1.0.0 cannot be invoked with `--quit` safely. |
| `cmd/squirebot/main.go` lines 95-145 | The ctx cancellation + tray controller wiring. The named-event listener goroutine attaches here; it must call the same `cancel()` + `systray.Quit()` path that `OnQuit` already calls. |
| `internal/tray/tray.go` line 234 (`systray.Quit()`) | The canonical clean-shutdown entry point. The new listener must funnel through this, not through a `os.Exit()`. |
| `.github/workflows/release.yml` | The release pipeline that produces the v1.0.1 binary + installer + `latest.json`. Tag `v1.0.1` on master triggers it. AUTH-03 PRODUCTION gate is still active — do not regress it. |
| `docs/troubleshooting.md` lines 50-58 | The section to delete. |
| `docs/build-and-install.md` | Local-rebuild docs that may reference `--quit` once it ships; planner should check whether to add a short note about it (manual graceful-quit is a useful debug aid). |

No external docs (no ADRs for this scope). NSIS 3.10+ docs live at https://nsis.sourceforge.io/Docs/ if the planner needs detail on `ExecWait`, `IfFileExists`, `ReadRegStr`, or version-string comparison primitives — but the patterns needed here are all in the existing `installer/squirebot.nsi`.
</canonical_refs>

<prior_decisions>
## Carried forward from earlier phases

| Source | Decision | How it applies here |
|--------|----------|---------------------|
| Phase 1 (INST-01) | Installer must not require UAC; `RequestExecutionLevel user` is locked. | Any pre-install probe MUST NOT call privileged APIs. `taskkill /IM squirebot.exe /F` (user-session-scoped) is fine; `taskkill /F /T` against a SYSTEM-owned process is irrelevant — watcher never runs elevated. |
| Phase 1 (INST-04) | Autostart via `HKCU\…\Run`. | Post-install relaunch is also covered by autostart at next logon, but the immediate `Exec` in the installer means users don't have to wait until reboot. Keep both paths. |
| Phase 2 (OPS-04 / Plan 02-06) | `minio/selfupdate` startup-swap is the AUTO-update path; the installer is the MANUAL re-install path. | Phase 6's shim is for the manual path only. The auto-update path already handles "binary in use" because the swap happens at startup BEFORE the new binary takes over — there's no running new instance to overwrite. Do not conflate the two. |
| Phase 2 (WATCH-09) | On restart, the watcher rescans the EQ folder via `fsnotify` catch-up so any file changes during downtime are picked up. | This is what makes "abandon in-flight writes" safe — there is no data loss, just a delay of however-long-the-installer-took before the next launch's catch-up re-uploads. |
| Phase 2 BUG-001 fix (v0.2.1 commit `71f7b76`) | Wizard handoff `signalDone` fires immediately after `cfg.Save()`. | Tangentially relevant: a duplicate watcher instance is a real risk if shutdown signaling goes wrong. The named-event pattern must be defensive about "what if the signal arrives but no listener is waiting" (answer: idempotent no-op; named-event signal is fire-and-forget). |
| v1.0.1 milestone open (2026-05-11) | No schema bump. `_meta.schema_version` stays at 3; `WatcherMaxSchemaVersion` stays at 3. | The watcher rebuild for Phase 6 does not affect schema. The version-gate logic in NSIS reads the installer registry's `DisplayVersion`, not `_meta.schema_version` — those are independent values. |
| v1.0.1 milestone open (2026-05-11) | Phase 6 is the only Go-side change in v1.0.1; produces the binary release that tags the milestone. | This is the deliverable that makes the tag `v1.0.1` meaningful. CI must produce a clean installer and a working `latest.json` or the tag is moot. |
| User memory: EV cert no longer grants instant SmartScreen reputation (2024-03) | Default = unsigned + walkthrough; SignPath OSS in parallel. | v1.0.1 binary ships UNSIGNED, same as v1.0.0. Users on v1.0.0 already accepted the walkthrough; re-install hits SmartScreen again per-binary-hash but that's documented and unchanged. |
| User memory: User installs missing toolchains themselves | Don't run installers or fabricate config. | If planner needs a missing tool (NSIS plugin? new Go module?), STOP and surface the gap to the user. Don't `go get` something the user hasn't approved. |
| User memory: Phase 2 deferred items + LICENSE-file gap blocking SignPath OSS | Tracked; not in Phase 6 scope. | Do not bundle a LICENSE-file fix or SignPath retry into this phase. INST-06 is the single requirement; resist scope drift. |

</prior_decisions>

<decisions>
## Implementation decisions (locked)

### D-01: Graceful-exit mechanism — `squirebot.exe --quit` + Windows named event

**Decision:** The watcher gains a `--quit` CLI flag. When invoked with `--quit`, the binary opens a process-local named Windows event (`Local\SquireBot-Shutdown`), signals it via `SetEvent`, and exits 0. The normally-launched watcher creates the same named event at startup and starts a goroutine that blocks on `WaitForSingleObject`; when the event fires, the goroutine invokes the existing `OnQuit` callback (`cancel()` + `systray.Quit()`).

**Rationale (vs. alternatives):**

- **WM_CLOSE on the systray window:** brittle. `fyne.io/systray` does not expose the HWND, and the internal window class name is an implementation detail subject to change across library versions. NSIS would need `FindWindow` by class — a recipe for a future silent regression.
- **Named pipe IPC server:** clean but heavyweight. Persistent goroutine, new attack surface (any process in the user session can connect), more code, more tests. Overkill for a one-way shutdown signal.
- **`taskkill /F` only (skip graceful entirely):** violates the ROADMAP success criterion 2 which explicitly mandates "signal it to exit gracefully, waits for it, and falls back to a hard kill if the process does not exit within the timeout." Also: visible UX downside — the tray icon disappears abruptly instead of gracefully cleaning up the heartbeat row first.
- **Named Windows event (chosen):** zero new dependencies (Windows API only; reuses `golang.org/x/sys/windows` already in the dep tree via fsnotify), idempotent (signal-with-no-listener is a no-op), fast (sub-millisecond signal delivery), and the `--quit` flag becomes a useful manual-debug aid for free.

**Implementation sketch (planner will refine):**

- New file `internal/system/shutdown_signal.go` exposes `WaitForShutdown(ctx) <-chan struct{}` (listener side) and `SignalShutdown() error` (signaler side). Windows-only file (`//go:build windows`).
- `cmd/squirebot/main.go` adds a `--quit` flag-handler block immediately after the existing `--uninstall-wipe-credentials` block (lines 38-54), BEFORE `update.Apply()`. Calls `SignalShutdown()`, prints a short stderr line, exits 0.
- Normal-launch path (after logging.Setup) spawns a goroutine: `<-system.WaitForShutdown(ctx)` → `cancel()` + `systray.Quit()`.
- Event name: `Local\SquireBot-Shutdown` (per-session, not `Global\` — no cross-session bleed; matches the per-user-installation model).

### D-02: v1.0.0 → v1.0.1 upgrade path — version-gated hard kill

**Decision:** NSIS pre-install reads `DisplayVersion` from `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\SquireBot`. If the value is missing OR lexicographically `< "1.0.1"`, skip the `ExecWait --quit` step entirely and proceed directly to `taskkill /IM squirebot.exe /F`. If `>= "1.0.1"`, `ExecWait '"$INSTDIR\${EXE_NAME}" --quit'`, wait up to 10s, then `taskkill /IM squirebot.exe /F` as fallback (returns non-zero if the process is already gone; that's expected and ignored).

**Rationale:** the v1.0.0 binary has only one flag-handler (`--uninstall-wipe-credentials`, `cmd/squirebot/main.go:38`); any other CLI arg falls through into a normal launch and spawns a duplicate watcher. Invoking `--quit` against a v1.0.0 binary would therefore create the exact mess the shim is trying to prevent. Hard-kill is benign for v1.0.0 because:
1. `batchUpdate` is in-process; killing the watcher abandons the request but doesn't corrupt the workbook (Sheets `batchUpdate` is server-side atomic).
2. WATCH-09 catch-up re-uploads any missed file changes on next launch.
3. The v1.0.0 user is doing a one-time upgrade; the brittle path is not used again.

**Edge cases handled:**
- Fresh install (no prior version): `ReadRegStr` returns empty → version comparison fails closed → `taskkill /F` runs and returns "process not found" → ignored. No-op net effect. ✓
- Side-by-side prior version somehow at `1.0.2-rc1`: lexicographic string compare on semver-like strings is wrong in general; planner should use NSIS's `VersionCompare` from `WordFunc.nsh` (built-in plugin, no external dep) for correctness. Document this in the plan.
- Prior install was uninstalled but reg key was orphaned: `ReadRegStr` succeeds; `taskkill /F` finds no process; harmless.

### D-03: Shutdown semantics — abandon in-flight writes; 10s NSIS timeout

**Decision:**

- **Watcher side:** the named-event listener calls `cancel()` + `systray.Quit()` and exits the main loop. In-flight `batchUpdate` calls observe context cancellation through the existing mutex-funneled `sheet.Client` retry envelope and abandon. No drain coordination, no "wait for pending writes" path.
- **NSIS side:** between the `ExecWait --quit` and the `taskkill /F` fallback, NSIS sleeps and polls for process exit using `nsProcess::FindProcess` (or `System::Call` on `OpenProcess` if no plugin allowed — planner picks) with a **10-second hard cap**. Loop interval: 250ms (so worst-case observed shutdown latency in the green path is ~250ms after the watcher actually exits).

**Rationale:**

- 10 seconds is plenty for a healthy watcher: ctx cancellation propagates through goroutines in single-digit milliseconds; the longest in-flight operation is a `batchUpdate` which, when cancelled, returns within a network RTT (sub-second).
- 10 seconds is short enough that a user clicking through an installer doesn't feel a hang. NSIS shows "Installing..." during this window; the standard installer UX absorbs it.
- Abandoning writes is safe because WATCH-09 catch-up exists and is tested (Plan 02-09 / WATCH-09 ships in v1.0.0). Re-implementing a drain path for "the last 2 seconds of pending writes" is complexity for negligible gain.
- The pathological "watcher hung in 90-min OAuth propagation probe" scenario is fine: it's blocked on a network call in a goroutine that respects ctx cancellation, so it will exit when `cancel()` fires. If for some reason it doesn't, `taskkill /F` at T+10s handles it.

### D-04: Post-install relaunch — unchanged

**Decision:** Keep the existing `Exec '"$INSTDIR\${EXE_NAME}"'` line at the end of the `Section "Install"` block (`installer/squirebot.nsi:105`). Do NOT add "only relaunch if the watcher was running before" logic.

**Rationale:**

- "Only relaunch if it was running" requires NSIS to remember pre-install state across the install section (a global var set during the pre-install detection step). Doable but adds a code path nobody exercises in the common case.
- Always-relaunch matches the fresh-install UX exactly. One code path, one mental model.
- If the user wanted the watcher off, they had it off pre-install (no detection, no kill, no shutdown). The installer just installed and launched — same as a first-time install. Symmetric.
- Post-install launch is *also* covered by HKCU autostart at next logon, so even if the immediate `Exec` failed silently, the user would get the watcher back. Defense in depth.

### D-05: Process detection — implicit via `--quit` + `taskkill`

**Decision:** Do NOT add a Find-Process NSIS plugin (FindProcDLL, NsProcess) for detection. The `ExecWait '"$INSTDIR\${EXE_NAME}" --quit'` call IS the detection — if the binary is there and another instance is running, `--quit` signals it; if not, it's a fast no-op. Subsequent `taskkill /IM squirebot.exe /F` is the residual cleanup AND fallback.

For polling between `--quit` and `taskkill`, planner picks between `nsProcess` plugin (small, mature, widely available) or pure `System::Call` against `OpenProcess` + `WaitForSingleObject` (no plugin, slightly more code). Either is acceptable; my preference is `nsProcess` for readability but I do not lock it.

**Rationale:** every extra NSIS plugin is one more thing CI has to fetch and one more thing that can break on a future NSIS version bump. The native-tooling path is fewer moving parts.

### D-06: latest.json — schema-stable, contents updated

**Decision:** No schema change to `dist/latest.json`. Same shape as v1.0.0 (per `.github/workflows/release.yml` step 5: `installer_url`, `installer_sha256`, `binary_url`, `binary_sha256`, `version`). The release workflow regenerates it on tag push automatically — no manual edit needed.

**Rationale:** the auto-update path (Plan 02-06 / OPS-04) consumes `latest.json` and must remain compatible with v1.0.0 watchers in the wild. Schema change would orphan them. The contents update naturally (new URLs + sha256s + `version: "1.0.1"`) without manual intervention.

### D-07: Release tag = `v1.0.1`

**Decision:** Phase 6 ships as `git tag v1.0.1` on master, pushed to GitHub. This triggers `.github/workflows/release.yml` which produces the installer, computes sha256s, regenerates `latest.json`, and creates the GitHub Release. The tag IS the ship gate per ROADMAP §49.

**Tag conventions:** matches v0.1.0, v0.2.0, v0.2.1, v0.3.0, v0.4.0, v1.0.0 pattern. CI's `${{ github.ref }}` extraction already handles the `v` prefix.

</decisions>

<code_context>
## Reusable assets + integration points

| Path | What it gives Phase 6 |
|------|----------------------|
| `cmd/squirebot/main.go:38-54` | The existing `--uninstall-wipe-credentials` flag-handler is the structural template for the new `--quit` handler. Same block placement (BEFORE `update.Apply()`), same `os.Exit(0)` style, same stderr logging. |
| `cmd/squirebot/main.go:95-145` | The tray controller's `OnQuit` callback already does `cancel()`. The new shutdown listener funnels through this same path — no new shutdown semantics to invent. |
| `internal/tray/tray.go:234` (`systray.Quit()`) | The canonical "unblock main goroutine and exit cleanly" call. Listener calls this; main returns; process exits. |
| `installer/squirebot.nsi:48` (`RequestExecutionLevel user`) | Locks the no-UAC constraint. The shim must respect it (use `HKCU` registry reads, user-session `taskkill`). |
| `installer/squirebot.nsi:136` (`ExecWait 'taskkill /IM "${EXE_NAME}" /F'`) | Prior art for the hard-kill fallback. Copy the syntax verbatim into the install section. |
| `installer/squirebot.nsi:43` (`REGPATH_UNINSTSUBKEY`) | The registry path constant the version-gate read uses (`ReadRegStr HKCU "${REGPATH_UNINSTSUBKEY}" "DisplayVersion"`). |
| `.github/workflows/release.yml` | Tag `v1.0.1` → build + makensis + sha256 + latest.json + GitHub Release. No CI changes needed for Phase 6 unless planner discovers a missing NSIS plugin (in which case STOP and surface to user per the toolchain-install guardrail). |
| `golang.org/x/sys/windows` (already a transitive dep via `fsnotify`) | Provides `CreateEvent`, `OpenEvent`, `SetEvent`, `WaitForSingleObject`. No new go.mod entry needed. Planner should verify this is reachable from `internal/system/shutdown_signal.go` without an `_ "..."` blank import. |
</code_context>

<deferred_ideas>
## Noted for Later (NOT in Phase 6 scope)

| Idea | Where it goes |
|------|---------------|
| **Single-instance enforcement** — actively prevent a second tray icon from appearing when a user double-clicks the autostart shortcut. The named-event infrastructure landing here makes this cheap (named mutex at startup), but the UX questions (silent exit? toast? "another instance is running" dialog?) and the regression risk to the existing wizard flow (Phase 2 BUG-001 territory) make it its own decision. | Add to backlog as a candidate behavior-hardening item for v1.0.2 or v1.1. |
| **Drain in-flight writes on graceful shutdown** — if WATCH-09 catch-up ever proves unreliable, revisit. Currently no evidence of write loss, so don't pay the complexity tax. | Open issue ONLY if soak monitoring shows missed writes attributable to installer-driven shutdowns. |
| **Notify the user before shutdown** — toast / balloon "SquireBot is updating, will restart in a moment." Nice-to-have polish; the tray flicker is already short enough that users won't notice. | Backlog candidate. Aesthetics-only. |
| **Uninstaller graceful path** — replace the uninstaller's existing `taskkill /F` (line 136) with the new `--quit` + fallback pattern. Symmetric with the installer. Low-value: uninstall is one-way and the user explicitly clicked Uninstall, so abruptness is acceptable. | Defer indefinitely unless a soak issue surfaces. |
| **NSIS plugin standardization** — if the planner picks `nsProcess` for the poll-between-quit-and-kill phase, future installer work will accumulate plugin deps. Worth a conscious decision before Phase 6 ships: "is `nsProcess` blessed for future use, or do we stay pure-`System::Call`?" | Capture the planner's choice in the Phase 6 SUMMARY.md so future installers don't re-litigate. |

</deferred_ideas>

<scope_changes>
## Scope changes during this discussion

None. The user delegated all gray-area decisions to Claude with the directive "make whichever choices you think will make the installer as simple and automatic as possible for end users." All five locked decisions follow that brief directly:

- D-01 (named event + `--quit` CLI flag): zero new dependencies, no plugin, all testable Go.
- D-02 (version-gated hard kill for v1.0.0): one-time legacy handling, doesn't compromise the graceful path for future upgrades.
- D-03 (10s timeout, abandon in-flight writes): shortest viable timeout for a healthy watcher; WATCH-09 catch-up makes write abandonment safe.
- D-04 (post-install relaunch unchanged): one code path, identical to fresh-install UX.
- D-05 (no Find-Process plugin): fewer CI moving parts.

Phase 6 ships INST-06 only. No additions, no subtractions.

</scope_changes>

<verification_hooks>
## Verification hooks (planner: these are the criteria the executor must satisfy)

From ROADMAP §44-48 (Phase 6 success criteria):

1. **v1.0.1 installer over running tray upgrades cleanly with no manual stop step** — UAT: install v1.0.0 on a clean Win11 VM, let it run to green-tray steady state, then double-click the v1.0.1 installer downloaded from the staging GitHub Release. Expected: tray icon disappears briefly, new tray icon appears, no dialog asks user to quit anything. Verify via `_meta.last_heartbeat` continuing to update.
2. **NSIS pre-install signals graceful exit, waits, falls back to hard kill on timeout** — Unit-testable: the `--quit` flag-handler in `cmd/squirebot/main.go` is testable in isolation (call `SignalShutdown()`, assert event fires, assert process exits 0). The 10s NSIS poll + taskkill fallback is best verified by an integration smoke (set up a stub binary that ignores the signal, time how long NSIS takes before falling back).
3. **Post-install new watcher autostarts and resumes writes; no token re-auth** — UAT: same v1.0.0→v1.0.1 upgrade as #1; verify next file change in EQ folder produces an `inv:<Char>` update within debounce + write window. No OAuth flow on screen.
4. **`docs/troubleshooting.md` no longer instructs users to manually stop the tray app** — Plain grep: `grep -n "stop\|quit\|right-click" docs/troubleshooting.md` should NOT match the install-related section after the edit. Section title "Installer won't overwrite a running SquireBot" should be gone (or rewritten to describe the new graceful path).
5. **Binary v1.0.1 built, tagged, published; `latest.json` updated** — CI artifact verification: `git tag v1.0.1` → release workflow green → `dist/SquireBot-Setup-1.0.1.exe`, `dist/squirebot.exe` (v1.0.1), and `dist/latest.json` (with `version: "1.0.1"`, fresh sha256s, correct download URLs) all attached to the GitHub Release.

Planner should structure plans so each success criterion maps to at least one plan with a clear ship gate.

</verification_hooks>

---

## Plan-phase entry signal

This phase is **ready for planning**. Suggested invocation:

```
/clear
/gsd-plan-phase 6 --skip-research
```

Research is optional — the named-event + NSIS pre-install pattern is well-trodden territory and the existing codebase has all the prior art needed (uninstaller's `taskkill`, the `--uninstall-wipe-credentials` flag-handler pattern, the auto-update startup-swap as a reference for "binary swaps without user friction"). If the planner wants a research pass for `golang.org/x/sys/windows` event-API specifics or NSIS `VersionCompare` semantics, `/gsd-plan-phase 6` (no `--skip-research`) is fine.

Estimated plan count: **3-5 plans**.

- Plan 1: Watcher `--quit` flag + named-event signaler (Go).
- Plan 2: Watcher named-event listener goroutine + main.go wiring (Go).
- Plan 3: NSIS pre-install shim (`installer/squirebot.nsi`) — version gate, `ExecWait --quit`, poll loop, `taskkill /F` fallback.
- Plan 4: Troubleshooting doc edit + (optional) build-and-install.md note about `--quit` as a manual debug aid.
- Plan 5: Release plumbing — `latest.json` smoke, v1.0.1 tag push, GitHub Release verification.

The planner may consolidate Plans 1+2 if the Go-side changes are small enough. Planner's call.
