# Phase 6 Discussion Log

**Date:** 2026-05-11
**Mode:** discuss (default; gray-area selection delegated by user)
**Phase:** 6 — Installer Overwrite-Running Shim
**Outcome:** All 4 surfaced gray areas locked by Claude per user directive; 1 unsurfaced area (process detection) also defaulted; deferred ideas captured.

---

## Areas surfaced

Four gray areas were generated from the ROADMAP success criteria + the existing `installer/squirebot.nsi` + `cmd/squirebot/main.go` scout:

1. **Graceful-exit signal mechanism** — How does NSIS tell a running `squirebot.exe` to quit?
2. **v1.0.0 → v1.0.1 upgrade path** — The currently-installed binary doesn't speak any new shutdown protocol; how is this transition handled?
3. **Shutdown timeout + in-flight writes** — How long does NSIS wait between signal and hard kill, and what does the watcher do with pending writes?
4. **Post-install relaunch policy** — Always relaunch, only-if-was-running, or rely on autostart?

A fifth gray area (process detection method) was bundled into D-01/D-05 because the chosen `--quit` mechanism makes a Find-Process plugin unnecessary.

## User selection

User declined to select specific areas and answered:

> "I have no preference for any of those. Please make whichever choices you think will make the installer as simple and automatic as possible for end users."

Interpreted as a blanket directive to optimize for end-user UX simplicity. All five decisions in `<decisions>` follow that brief; the trade-off rationales are recorded in CONTEXT.md so the user can audit and override in `/gsd-plan-phase` if needed.

## Question-by-question record

### Area 1 — Graceful-exit signal mechanism

**Options Claude weighed:**

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| WM_CLOSE on systray window | Native; no new code | Brittle — `fyne.io/systray` doesn't expose HWND; class name is implementation-defined | ❌ |
| Named pipe IPC server | Clean IPC | Persistent goroutine; attack surface; more code/tests | ❌ |
| Named Windows event + `--quit` CLI flag | Zero new deps; idempotent; fast; `--quit` doubles as manual debug aid | Requires per-OS `//go:build windows` file | ✅ |
| `taskkill /F` only | Simplest | Violates ROADMAP criterion 2 (graceful + fallback); abrupt UX | ❌ |

**Locked:** D-01 — `--quit` CLI flag opens `Local\SquireBot-Shutdown` event; listener goroutine in normal-launch path waits on it and invokes existing `OnQuit`.

### Area 2 — v1.0.0 → v1.0.1 upgrade path

**Options Claude weighed:**

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| Always `ExecWait --quit` | Single code path | v1.0.0's unknown-flag fall-through (`cmd/squirebot/main.go:38`) spawns a duplicate watcher. ❌ catastrophic | ❌ |
| Always `taskkill /F` | Simple | Violates spec for v1.0.1+ upgrades | ❌ |
| Version-gated: read HKCU `DisplayVersion`, branch on < 1.0.1 | Handles legacy cleanly; full graceful path for future | One extra NSIS conditional | ✅ |
| Document one-time manual stop for v1.0.0 users only | Zero installer code | Friction for the one upgrade we promised to fix | ❌ |

**Locked:** D-02 — version-gated. < 1.0.1 → skip `--quit`, taskkill /F directly. >= 1.0.1 → ExecWait --quit + taskkill /F fallback.

### Area 3 — Shutdown timeout + in-flight writes

**Options Claude weighed:**

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| Drain in-flight writes (block until pending batchUpdates settle) | "No write loss" feels safer | batchUpdate retry envelope can take minutes; risks installer hang; WATCH-09 already handles re-upload | ❌ |
| Abandon in-flight writes; 3s timeout | Aggressive | A slow but valid shutdown might trip the hard-kill prematurely | ❌ |
| Abandon in-flight writes; 10s timeout | Healthy watcher exits in <1s; budget absorbs cold goroutines and OS jitter; WATCH-09 makes it safe | None material | ✅ |
| Abandon; 30s timeout | Very forgiving | User-perceptible installer hang in the failure path | ❌ |

**Locked:** D-03 — abandon in-flight writes; 10s NSIS hard-cap with 250ms poll interval; fallback `taskkill /F` if not exited.

### Area 4 — Post-install relaunch policy

**Options Claude weighed:**

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| Always relaunch (current) | Symmetric with fresh-install; defense-in-depth via HKCU autostart | None material | ✅ |
| Only relaunch if was running pre-install | Preserves "user had it off" intent | Requires pre-install state global var; extra code path | ❌ |
| Skip relaunch; let autostart pick up at next logon | Minimal installer code | User waits for reboot; bad UX | ❌ |

**Locked:** D-04 — keep existing `Exec '"$INSTDIR\${EXE_NAME}"'` line at the end of Section "Install". No change.

### Bundled — Process detection method

Bundled into D-01 + D-05. `ExecWait --quit` is the detection (no-op if not running). Between graceful signal and fallback, planner picks `nsProcess` plugin or `System::Call` against `OpenProcess`. Either is acceptable; preference noted but not locked.

## Canonical refs accumulated

(See CONTEXT.md `<canonical_refs>` section for the full table with rationale.)

- `.planning/ROADMAP.md` § Phase 6
- `.planning/REQUIREMENTS.md` § INST-06
- `installer/squirebot.nsi` (especially lines 48, 105, 136, 43)
- `cmd/squirebot/main.go` (especially lines 38-54, 95-145)
- `internal/tray/tray.go:234`
- `.github/workflows/release.yml`
- `docs/troubleshooting.md` lines 50-58

## Deferred ideas captured

| Idea | Disposition |
|------|-------------|
| Single-instance enforcement | Backlog candidate for v1.0.2 / v1.1 |
| Drain in-flight writes on shutdown | Only revisit if soak shows write loss |
| Toast/balloon "updating" notification | Backlog; aesthetics-only |
| Uninstaller graceful path | Defer indefinitely |
| NSIS plugin standardization (`nsProcess` blessed for future use?) | Capture planner's choice in 06-SUMMARY.md |

## Claude's discretion items

All five locked decisions are Claude's discretion items, per the user's blanket directive. Each has an explicit trade-off rationale in CONTEXT.md `<decisions>` so the user can audit and override at plan time.

## Time

- Discussion start: 2026-05-11
- Discussion end: 2026-05-11 (single-turn delegation; no iteration)
- Areas surfaced: 4 (+ 1 bundled)
- Areas explored: 5 (all defaulted)
- Plans estimated: 3-5
