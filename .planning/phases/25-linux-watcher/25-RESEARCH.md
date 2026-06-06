# Phase 25: Linux Watcher - Research

**Researched:** 2026-06-06
**Domain:** Cross-platform Go build (CGO-free static linux/amd64), WINE EQ-install discovery, systemd user-service packaging, selfupdate on Linux
**Confidence:** HIGH (build mechanics + codebase facts), MEDIUM (WINE/Lutris/Bottles scan paths — convention-verified, not box-tested), one flagged DECISION (systemd linger)

---

<user_constraints>
## User Constraints (from 25-CONTEXT.md)

### Locked Decisions
- **D-01 (Headless daemon):** Linux watcher is headless — NO `fyne.io/systray`. Runs as a systemd user service, `CGO_ENABLED=0` fully static (no GTK/appindicator/Zenity/CGO). The tray controller becomes a no-op/logging implementation on Linux (build-tag split: real systray on Windows, headless controller on `!windows`). `RunApp`'s `trayCtl.SetStatus/SetIconHealth` calls are UNCHANGED. Status/health surfaces via the log + a CLI `--status`.
- **D-02 (CLI onboarding):** First-run onboarding is a CLI flow over stdin (`squirebot --setup`): prompt for the guild code + confirm/enter the EQ folder. Win32 dialog path stays Windows-only; the `!windows` `onboarding` impl prompts on the terminal instead of returning `ErrUnsupported`. Add `--status`. NO localhost HTTP onboarding surface (HARD CONSTRAINT — watcher stays browser-free).
- **D-03 (0600 file, no keyring):** Store the bearer guild code in a `0600` file under `$XDG_CONFIG_HOME/squirebot/` (default `~/.config/squirebot/`). NOT an OS keyring. The `credstore` interface stays the same; add a `!windows` file-backed implementation beside the Windows `wincred` one.
- **D-04 (Auto-scan WINE + config override):** Discovery probes, in order: `$WINEPREFIX`, `~/.wine/drive_c`, common Lutris/Proton/Bottles prefix locations, walking each (depth-bounded, the existing `eqfind` heuristic) for `eqgame.exe`/`eqclient.ini`. Fall back to the CLI prompt (D-02) and persist the chosen path to config (`EQFolders` already exists). Fill in the `!windows` stubs in `eqfind` (`registry_other.go` stays no-op; `heuristic_other.go` + the known-paths list get real Linux logic).
- **D-05 (XDG base dirs):** Config at `$XDG_CONFIG_HOME/squirebot/config.json` (default `~/.config/squirebot/`); logs at `$XDG_STATE_HOME/squirebot/` (default `~/.local/state/squirebot/`), falling back to config-dir if unset. Branch `defaultPath()` (and the log path) on `runtime.GOOS`; Windows stays `%LOCALAPPDATA%`. Keep the existing atomic `.tmp`→rename write + `0755`/`0600` modes.
- **D-06 (Tarball + install script + systemd user unit):** Ship a `.tar.gz` containing the static `linux/amd64` binary, a README, a systemd **user** unit (`squirebot.service`, `Restart=always`, `WantedBy=default.target`), and an `install.sh` (installs binary to `~/.local/bin`, drops + `systemctl --user enable --now` the unit, runs first-time `--setup` if unconfigured). Autostart = the systemd user unit (the Linux `HKCU\Run` equivalent); no NSIS, no UAC. Existing `minio/selfupdate` auto-update reused as-is (overwrites the binary directly on Linux — no `.exe` rename dance); ensure the update manifest/asset naming covers the linux artifact.
- **D-07 (additive build, Windows unchanged):** Add `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` to the build producing the tarball; Windows NSIS path untouched. Every Linux change behind `//go:build` tags or `runtime.GOOS` so `go test ./...` and the Windows artifact are unaffected. Verify the Windows build still compiles identically. (arm64 deferred unless trivial.)

### Claude's Discretion
- Exact CLI UX/wording for `--setup` / `--status`; systemd unit hardening directives; `install.sh` ergonomics; the precise Lutris/Proton/Bottles scan-path list (research-informed — see §2 below); the XDG-state-vs-cache choice for logs if STATE is awkward; whether to also emit `linux/arm64`.
- Whether the headless tray controller lives as `tray_other.go` (no-op controller) vs a small interface in `app` — planner's call, as long as `RunApp` is unchanged and the Windows tray is intact.

### Locked constraints (carried — DO NOT violate)
- **No browser/loopback/OAuth in the watcher** (carried HARD CONSTRAINT). Onboarding is CLI; the credential is a static bearer token.
- **Windows watcher behavior must not change** — Linux is purely additive.
- The bearer **guild code is a static, reusable token** (v2.1) — persisted, not single-use.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LNX-01 | Cross-compile `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` to a single static binary; runs headless (no systray); tray controller no-op; `RunApp` unchanged; Windows build + `go test ./...` unaffected (additive behind build tags / `runtime.GOOS`). | §1 (CGO-free tray-exclusion pattern — the critical mechanic), §7 (build invocation) |
| LNX-02 | Bearer guild code in a `0600` file under `$XDG_CONFIG_HOME/squirebot/` (no keyring); config + logs follow XDG; Windows `wincred`/`%LOCALAPPDATA%` untouched. | §5 (credstore file impl), §6 (XDG dirs) |
| LNX-03 | EQ-folder discovery finds the WINE-prefix install (`$WINEPREFIX` → `~/.wine/drive_c` → Lutris/Proton/Bottles paths via bounded `eqfind` walk), falling back to a CLI prompt that persists the path. | §2 (WINE scan-path list — highest-risk), §4-eqfind |
| LNX-04 | First-run onboarding + control are CLI (`--setup` prompts guild code + EQ folder over stdin; `--status` prints health/config); no Win32 dialog, no localhost/browser surface. | §3 (CLI onboarding impl), §8 (main.go entry split) |
| LNX-05 | `.tar.gz` ships static binary + README + systemd **user** unit + `install.sh`; `minio/selfupdate` works on Linux with the linux asset in the update manifest. | §3-systemd, §3-selfupdate (asset naming), §7 (tarball CI) |
| LNX-06 | fsnotify watch + 500ms debounce + full-snapshot-replace + `WatcherMaxSchemaVersion` gate + log rotation all function on Linux; covered by existing suite plus new Linux-path unit tests (credstore / eqfind / config). | §9 (Validation Architecture) |
</phase_requirements>

## Summary

This phase is **not new architecture** — the watcher already uses the `//go:build windows` / `//go:build !windows` paired-file idiom across `console`, `eqfind` (registry/heuristic), `system` (shutdown-signal), and `onboarding` (dialog). The Linux port is mostly **filling the `_other`/`!windows` side** plus one genuinely tricky structural change.

The single highest-risk item (§1) is keeping the binary `CGO_ENABLED=0`. `fyne.io/systray` requires CGO on Linux, AND it is imported in **two** places that are NOT currently build-tagged: `internal/tray/tray.go` (the whole package) and — load-bearing — **`cmd/squirebot/main.go` itself** (`import "fyne.io/systray"`, calls `systray.Run`/`systray.Quit` directly, and declares `var trayCtl *tray.Controller`). To get a CGO-free Linux binary, BOTH the `tray` package and the `main.go` systray usage must be split by build tag so the `!windows` binary never imports `fyne.io/systray`. `sqweek/dialog` (the other CGO-on-Linux risk) is already isolated to `dialog_windows.go` and is NOT on the `!windows` path — verified.

The second highest-risk item (§2) is the WINE EQ-folder scan-path list. The good news: the existing `eqfind` cascade (`knownPaths → registry → heuristic`) maps cleanly — `registry_other.go` stays a no-op, `heuristic_other.go` gets a real depth-bounded WINE-prefix walk, and the Linux known-paths list enumerates `$WINEPREFIX`, `~/.wine`, Lutris, Bottles (incl. Flatpak), and Steam/Proton compatdata prefixes.

**Primary recommendation:** Build-tag-split the `tray` package into a real systray controller (`//go:build windows`) and a headless no-op controller (`//go:build !windows`) exposing the IDENTICAL `Controller` API (`NewController`, `SetStatus`, `SetIconHealth`, `OnReady`, `OnExit`, `LogDir`), then split `cmd/squirebot/main.go`'s systray-run tail into `run_windows.go` (`systray.Run(...)`) / `run_other.go` (block on `ctx.Done()`). This keeps `app.RunApp`'s signature byte-for-byte identical and the Windows binary unchanged.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| EQ-folder discovery (WINE) | Watcher / `eqfind` | — | filesystem probing is a client-only concern; backend never sees paths |
| Credential at-rest storage | Watcher / `credstore` | OS (DPAPI on Win, file perms on Linux) | per-machine secret; D-03 chooses a `0600` file over a keyring daemon |
| Autostart / lifecycle | OS (systemd user mgr) | Watcher (`Restart=always` relaunch on self-update exit) | the systemd unit is the Linux `HKCU\Run` equivalent |
| Self-update swap | Watcher / `internal/update` | OS (atomic rename) | `minio/selfupdate.Apply` is cross-platform; Linux uses rename-in-place |
| Status/health surface | Watcher (log + `--status`) | — | no tray on Linux; CLI replaces the icon |
| Build + packaging | CI (release.yml) | — | additive linux build job alongside the untouched Windows NSIS job |

## Standard Stack

No NEW dependencies. The whole point of D-01/D-03 is to keep the Linux path CGO-free and dependency-clean. The existing deps split cleanly:

| Dep | linux/amd64 CGO-free? | On the `!windows` watcher path? | Action |
|-----|----------------------|--------------------------------|--------|
| `fyne.io/systray` v1.10.0 | NO — needs CGO (GTK/appindicator) on Linux | **YES today** (`main.go` + `tray.go`) — MUST be excluded | Build-tag both import sites behind `//go:build windows` [VERIFIED: cmd/squirebot/main.go:15, internal/tray/tray.go:30] |
| `github.com/sqweek/dialog` | NO — needs CGO (GTK) on Linux | **NO** — only imported in `dialog_windows.go` (`//go:build windows`) | None — already isolated [VERIFIED: internal/onboarding/dialog_windows.go:1,33] |
| `github.com/danieljoos/wincred` v1.2.0 | builds everywhere but is Windows-semantic | imported un-tagged in `credstore/store.go` | Build-tag `store.go` → `store_windows.go`; add `store_other.go` (file impl) [VERIFIED: internal/credstore/store.go:23-27] |
| `golang.org/x/sys/windows` | Windows-only package | only in `console_windows.go`, `dialog_windows.go`, `shutdown_signal_windows.go` (all tagged) | None — already tagged [VERIFIED] |
| `github.com/fsnotify/fsnotify` v1.7.0 | YES (pure Go; inotify on Linux) | YES — the watch path | None — already cross-platform |
| `github.com/minio/selfupdate` v0.6.0 | YES (pure Go) | YES — the update path | None; confirm asset naming (§3) |
| `gopkg.in/natefinch/lumberjack.v2` | YES (pure Go) | YES — logging | None |
| `golang.org/x/oauth2`, `discordgo`, `goose`, `modernc/sqlite` | n/a | NOT on the watcher binary (server-only) | None |

**No `npm install` / no new `go get`.** The build invocation is the deliverable (§7).

### Why no XDG library (e.g. `adrg/xdg`)
Go's stdlib `os.UserConfigDir()` already returns `$XDG_CONFIG_HOME` or `~/.config` on Unix [CITED: pkg.go.dev/os, golang/go#29960]. Only `$XDG_STATE_HOME` (logs, D-05) is NOT covered by stdlib — but its fallback (`~/.local/state`) is a three-line hand-roll. Adding a dependency for three lines violates the dependency-clean intent. **Recommendation: hand-roll, no new dep.**

## Architecture Patterns

### System Architecture Diagram (Linux watcher startup)

```
            squirebot (linux/amd64 static, CGO_ENABLED=0)
                              │
          ┌───────────────────┴────────────────────┐
          │  main()  (build-tag-neutral prologue)   │
          │   --uninstall-wipe-credentials / --quit  │  (Win-only paths no-op on Linux)
          │   update.Apply()  ── staged .new? ──► swap + os.Exit(0)
          │   --setup  ──► CLI onboarding (stdin)  ─┐
          │   --status ──► print health + exit     ││
          └───────────────────┬────────────────────┘│
                              │                     │
                  logging.Setup() (XDG_STATE)        │
                  config.Load()   (XDG_CONFIG)        │
                              │                     │
                  app.RunApp(ctx,cfg,…, trayCtl) ◄───┘
                              │   (trayCtl = headless no-op controller on Linux)
              ┌────────────────┴───────────────────┐
              │ credstore.Read() (0600 file)         │ no code → onboarding (CLI)
              │ eqfind / cfg.EQFolders               │ no folder → WINE scan → CLI prompt
              │ watch.Run (inotify, 500ms debounce)  │
              │   on change → read → CP1252→UTF-8 → POST /api/v1/ingest (Bearer code)
              │ update.RunDailyCheck (GitHub Releases manifest → stage .new)
              └────────────────┬───────────────────┘
                              │
            run_other.go: <-ctx.Done()  (replaces systray.Run; blocks main)
                              │
          self-update staged → exit → systemd Restart=always relaunches new binary
```

File-to-implementation mapping is in the Component Responsibilities table below, not the diagram.

### Component Responsibilities (what each build-tag file owns)

| Concern | Windows file (unchanged) | New `!windows` file | Contract to preserve |
|---------|--------------------------|---------------------|----------------------|
| systray run loop | `cmd/squirebot/main.go` tail (`systray.Run`) | `cmd/squirebot/run_other.go` (`<-ctx.Done()`) | `RunApp` signature; blocking main |
| tray controller | `internal/tray/tray_windows.go` (rename of today's `tray.go`) | `internal/tray/tray_other.go` (no-op `Controller`) | `NewController`, `SetStatus`, `SetIconHealth`, `OnReady`, `OnExit`, `LogDir`, `Config`, `Health`, `HealthGreen/Red` |
| credstore | `internal/credstore/store_windows.go` (rename of today's `store.go`) | `internal/credstore/store_other.go` (0600 file) | `Store(code) error`, `Read() (string,error)`, `Delete() error` |
| onboarding prompts | `dialog_windows.go` (unchanged) | `dialog_other.go` (stdin prompts, replaces `ErrUnsupported`) | `PromptGuildCode(title,prompt) (string,error)`, `PickEQFolder(title) (string,error)` |
| EQ heuristic scan | `heuristic_windows.go` (unchanged) | `heuristic_other.go` (WINE walk, replaces `return ""`) | `heuristicScan() string` |
| EQ known paths | `discover.go` `defaultKnownPaths()` (Windows literals) | branch on `runtime.GOOS` OR split a `knownpaths_*.go` | `defaultKnownPaths() string` |
| config path | `config.go` `defaultPath()` (LOCALAPPDATA) | branch on `runtime.GOOS` | `defaultPath() string` |
| log path | `logging/logger.go` `Setup()` (LOCALAPPDATA) | branch on `runtime.GOOS` | `Setup() (*slog.Logger, string)` |
| console detach | `console_windows.go` (FreeConsole) | `console_other.go` (already no-op) | `freeConsole() error` — DONE |
| shutdown signal | `shutdown_signal_windows.go` | `shutdown_signal_other.go` (already ctx-only) | DONE |

### Pattern 1: Build-tag-split a package keeping ONE exported API (the tray)
**What:** Today `internal/tray/tray.go` is un-tagged and imports `fyne.io/systray`. Split it so the EXPORTED surface (`Controller`, `NewController`, `Config`, `SetStatus`, `SetIconHealth`, `OnReady`, `OnExit`, `LogDir`, `Health`, `HealthGreen`, `HealthRed`, `MenuPlan` if still referenced) is identical on both platforms; only the Windows file imports systray.
**When to use:** Any package whose dependency is platform-CGO but whose API the rest of the program calls unconditionally.
**Critical detail:** `app.RunApp` takes `t *tray.Controller` (a concrete pointer, not an interface) [VERIFIED: internal/app/runapp.go:83]. So `Controller` MUST remain a concrete type with the same method set on both platforms — the `!windows` version is a struct whose methods log/no-op. Do NOT convert `RunApp` to take an interface (that would touch the Windows path and violate D-07's "RunApp unchanged in shape").

```go
// internal/tray/tray_other.go
//go:build !windows
package tray

import "log/slog"

type Health int
const ( HealthGreen Health = iota; HealthRed )

type Config struct {
    IconGreen, IconRed []byte
    LogDir             string
    OnCheckUpdates     func()
    OnEnterGuildCode   func()
    OnQuit             func()
}

type Controller struct{ logDir string }

func NewController(c Config) *Controller { return &Controller{logDir: c.LogDir} }
func (t *Controller) SetStatus(s string)        { slog.Info("status", "msg", s) }
func (t *Controller) SetIconHealth(h Health)    { slog.Info("health", "state", h) }
func (t *Controller) OnReady()                  {}
func (t *Controller) OnExit()                   {}
func (t *Controller) LogDir() string            { return t.logDir }
```
[ASSUMED] exact field/method set — planner must diff against `tray.go` and reproduce EVERY symbol `main.go`/`app` references (notably `Config.OnCheckUpdates/OnEnterGuildCode/OnQuit`, used in `main.go:133-161`).

### Pattern 2: Split the `main.go` systray tail (the systray import lives in main, too)
**What:** `cmd/squirebot/main.go:15` imports `fyne.io/systray` and lines 178-205 call `systray.Quit()` / `systray.Run(...)`. Extract those two call sites behind build tags.
**Recommended shape:**
```go
// run_windows.go //go:build windows
func runMainLoop(ctx context.Context, cancel context.CancelFunc, trayCtl *tray.Controller) {
    go func() {
        select {
        case <-system.WaitForShutdown(ctx): cancel(); systray.Quit()
        case <-ctx.Done():
        }
    }()
    systray.Run(trayCtl.OnReady, trayCtl.OnExit) // blocks
    cancel()
}
// run_other.go //go:build !windows
func runMainLoop(ctx context.Context, cancel context.CancelFunc, _ *tray.Controller) {
    go func() { <-system.WaitForShutdown(ctx); cancel() }()
    <-ctx.Done() // blocks until cancel
}
```
Then `main.go` (now systray-free) just calls `runMainLoop(ctx, cancel, trayCtl)`. `main.go` must DROP its `import "fyne.io/systray"`. The OS-signal handler (SIGTERM/SIGINT) that systemd sends on stop should also feed `cancel()` on Linux — add `signal.NotifyContext` in `run_other.go` (or share it). [VERIFIED: systray usage in main.go at lines 15, 183, 205]

### Pattern 3: CLI onboarding over stdin (replaces the `!windows` ErrUnsupported stubs)
`dialog_other.go` currently returns `ErrUnsupported` for both `PromptGuildCode` and `PickEQFolder` [VERIFIED: internal/onboarding/dialog_other.go:11-18]. Replace with stdin prompts. The CALLER (`app.runOnboarding`/`pickAndSaveEQFolder`) is UNCHANGED — it already loops on the returned error and re-prompts [VERIFIED: internal/app/runapp.go:136-202]. Keep the same `ErrCancelled` sentinel (e.g., empty line or EOF = cancel) so the caller's `errors.Is(err, ...)` branches still work.

```go
// internal/onboarding/dialog_other.go  //go:build !windows
func PromptGuildCode(title, prompt string) (string, error) {
    fmt.Fprintln(os.Stderr, title)
    fmt.Fprint(os.Stderr, prompt+" ")
    r := bufio.NewReader(os.Stdin)
    line, err := r.ReadString('\n')
    if err != nil && line == "" { return "", ErrCancelled } // EOF/non-tty
    code := strings.TrimSpace(line)
    if code == "" { return "", ErrCancelled }
    return code, nil
}
func PickEQFolder(title string) (string, error) {
    // First try eqfind.Discover() (the WINE auto-scan), then prompt if empty.
    // Caller already runs eqfind.ValidateFolder + re-prompts on failure.
    fmt.Fprint(os.Stderr, title+": ")
    line, err := bufio.NewReader(os.Stdin).ReadString('\n')
    if err != nil && line == "" { return "", ErrCancelled }
    p := strings.TrimSpace(line)
    if p == "" { return "", ErrCancelled }
    return os.ExpandEnv(p), nil // expand ~/$WINEPREFIX-style input — or handle ~ explicitly
}
```
**`--setup` / `--status` subcommand wiring** lands in `main.go` (build-tag-neutral — they're just `os.Args[1]` checks like the existing `--quit` / `--uninstall-wipe-credentials` blocks at main.go:38-66). `--setup` runs the onboarding flow synchronously and exits; `--status` reads config + credstore + last-upload mtimes and prints, then exits. Both must run AFTER `logging.Setup`/`config.Load` for `--status` (it needs config). [ASSUMED] exact ordering — planner decides; mirror the existing arg-dispatch idiom.

### Anti-Patterns to Avoid
- **Converting `RunApp` to take a `TrayController` interface.** Tempting, but it touches the Windows compile path → violates "Windows byte-for-byte equivalent." Keep `Controller` a concrete type that exists on both platforms with the same methods.
- **`runtime.GOOS` checks that still IMPORT systray.** A `if runtime.GOOS == "windows"` guard does NOT remove the import from the Linux binary — the CGO link still happens. systray MUST be excluded via `//go:build`, not a runtime branch. (Contrast: `defaultPath()`/`defaultKnownPaths()` CAN use `runtime.GOOS` because they import nothing platform-specific.)
- **Putting logs in `$XDG_CACHE_HOME`.** Logs are STATE, not cache (cache is deletable-anytime); D-05 correctly picks `$XDG_STATE_HOME`.
- **Following symlinks in the WINE walk.** `filepath.WalkDir` doesn't by default [VERIFIED: heuristic_windows.go:73 comment] — keep that property; WINE prefixes are full of symlinks into the host fs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| In-place binary swap on Linux | custom rename dance | `minio/selfupdate.Apply` (already used) | handles rename + rollback + checksum; Linux needs no `.exe` lock dance |
| `$XDG_CONFIG_HOME` resolution | env parsing | `os.UserConfigDir()` (stdlib) | returns `$XDG_CONFIG_HOME` or `~/.config` [CITED: pkg.go.dev/os] |
| inotify watch + debounce | raw inotify syscalls | `fsnotify` (already used) | cross-platform; the watcher's 500ms debounce already wraps it |
| Autostart at login/boot | cron `@reboot`, `.desktop` autostart, rc.local | systemd **user** unit | the standard Linux per-user daemon mechanism; survives logout only with linger (§3 DECISION) |
| Log rotation | custom size tracking | `lumberjack` (already used) | platform-agnostic |

**Key insight:** Every "hard" Linux primitive this phase needs is already a dependency that works CGO-free on Linux. The phase adds ZERO new libraries — it only adds build-tagged glue and a packaging script.

## Detailed Findings

### §1 — CGO-free headless build (THE critical mechanic) [VERIFIED + CITED]

`fyne.io/systray` links CGO on Linux (it binds GTK/libappindicator). To keep `CGO_ENABLED=0`, the Linux binary must NOT import it at all. Two import sites exist today, both un-tagged:

1. `internal/tray/tray.go:30` — `import "fyne.io/systray"` (whole package). [VERIFIED]
2. `cmd/squirebot/main.go:15` — `import "fyne.io/systray"`; calls `systray.Quit()` (line 183) and `systray.Run(trayCtl.OnReady, trayCtl.OnExit)` (line 205). [VERIFIED]

**Fix (the cleanest Go pattern):**
- Rename `internal/tray/tray.go` → `internal/tray/tray_windows.go`, add `//go:build windows`. Create `internal/tray/tray_other.go` (`//go:build !windows`) with the identical exported API but no-op bodies and NO systray import (Pattern 1 above).
- Extract `main.go`'s systray tail into `run_windows.go` / `run_other.go` (Pattern 2). Drop `import "fyne.io/systray"` from `main.go`.

**Verifying the dep is windows-only after tagging:** once both import sites are `//go:build windows`, run on the dev box:
```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...        # must succeed
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go list -deps ./cmd/squirebot | Select-String systray   # must return NOTHING
```
`go.mod` will still LIST `fyne.io/systray` as a direct require (it's still used by the Windows build) — that's correct and expected; the test is that it's absent from the linux/amd64 dependency CLOSURE, not from go.mod. [VERIFIED: go.mod:6]

**Other transitive CGO on the `!windows` path:** `sqweek/dialog` is the only other GTK/CGO dep, and it is ALREADY isolated — imported solely in `dialog_windows.go` (`//go:build windows`, import at line 33) [VERIFIED]. `godbus/dbus` (go.mod:24, indirect) is pulled by systray, NOT independently on the watcher path — it leaves the closure once systray does. **Net: after tagging the two systray sites, the linux/amd64 closure is CGO-free.** Confirm with the `go list -deps` grep above (treat as the acceptance gate for LNX-01).

### §2 — WINE EQ-install path conventions (highest-risk external) [MEDIUM — convention-verified]

A P99-under-WINE install puts the EQ folder (with `eqgame.exe` + `eqclient.ini`) inside a WINE prefix's `drive_c`. The `eqfind` Linux scan should probe these roots, in priority order, then run the existing depth-bounded walk (`maxHeuristicDepth=5`, `ValidateFolder` = both sentinels present) under each:

| # | Root (expand `$HOME`, `$WINEPREFIX`) | Source |
|---|--------------------------------------|--------|
| 1 | `$WINEPREFIX/drive_c` (if `$WINEPREFIX` set) | WINE convention [CITED: ArchWiki Wine] |
| 2 | `~/.wine/drive_c` (the default prefix) | WINE default [CITED] |
| 3 | `~/Games/*/drive_c` (Lutris default install root) | [CITED: forums.lutris.net] |
| 4 | `~/.local/share/lutris/runners/winegames/*/drive_c` + Lutris per-game prefixes (configurable; the game's prefix is in its YAML) | [CITED: Lutris forums] |
| 5 | `~/.var/app/net.lutris.Lutris/data/lutris/...` (Lutris **Flatpak**) | Flatpak app-data convention [CITED] |
| 6 | `~/.local/share/bottles/bottles/<name>/drive_c` (Bottles, native) | [CITED: linux-gaming.kwindu.eu] |
| 7 | `~/.var/app/com.usebottles.bottles/data/bottles/bottles/<name>/drive_c` (Bottles **Flatpak**) | [CITED: Lutris/Bottles wiki] |
| 8 | `~/.local/share/Steam/steamapps/compatdata/*/pfx/drive_c` (Steam **Proton**) | [CITED: ValveSoftware/Proton wiki] |
| 9 | `~/.steam/steam/steamapps/compatdata/*/pfx/drive_c` (Steam alt symlink path) | Steam convention |

Concrete probe strategy for `heuristic_other.go`:
- Build the candidate-root list above (skipping any that don't `os.Stat`), expanding globs (`*`) with `filepath.Glob`.
- For each existing root, run a `walkRoot`-style depth-bounded walk (reuse the Windows heuristic's structure: depth cap, prune list, 30s context timeout, no symlink follow) calling `ValidateFolder`.
- Prune names to skip inside a `drive_c`: `windows`, `Program Files`, `Program Files (x86)` only if EQ isn't conventionally there — actually EQ under WINE often LIVES in `Program Files`, so DON'T prune those here (unlike the native-Windows scan). Prune only `users`, `ProgramData`, `$Recycle.Bin`. [ASSUMED A1 — prune list for the WINE walk]
- First match wins; `discover.go`'s cascade returns it.

`defaultKnownPaths()` (the fast no-walk first layer) should ALSO try a few direct hits before the heuristic: `$WINEPREFIX/drive_c/P99`, `$WINEPREFIX/drive_c/Program Files/Project1999`, `~/.wine/drive_c/Project1999`, `~/.wine/drive_c/P99`, etc. — mirroring the Windows `C:\P99`/`C:\Project1999` list [VERIFIED: discover.go:71-94].

**The walk is correct but the path list is a best-effort enumeration** — a Linux+WINE guildie's actual layout is the real test (the UAT). The fallback CLI prompt (D-02/D-04) is the safety net, so an incomplete list degrades to "type your path," not failure. Flag for the planner: the scan should be **generous (many roots) but bounded (depth+timeout)**; correctness of any single root is MEDIUM confidence.

### §3 — Packaging: systemd user unit, install.sh, selfupdate

#### systemd user unit (recommended contents)
```ini
# squirebot.service  -> installed to ~/.config/systemd/user/squirebot.service
[Unit]
Description=SquireBot watcher (P99 inventory/spellbook uploader)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%h/.local/bin/squirebot
Restart=always
RestartSec=5
# hardening (discretionary — D-01 says headless; these are safe for a headless daemon):
# NoNewPrivileges=true
# ProtectSystem=strict / ReadWritePaths for XDG dirs — but the watcher writes config+logs
#   under $HOME, so strict sandboxing needs ReadWritePaths; keep it MINIMAL to avoid breaking
#   the WINE-folder reads (the EQ folder can be anywhere under $HOME). Recommend NoNewPrivileges
#   only, skip ProtectHome (it needs to READ the EQ folder under $HOME).

[Install]
WantedBy=default.target
```
[CITED: ArchWiki Systemd/User — `WantedBy=default.target` is the user-instance "multi-user" analog]

`install.sh` steps:
```sh
install -Dm755 squirebot ~/.local/bin/squirebot
install -Dm644 squirebot.service ~/.config/systemd/user/squirebot.service
systemctl --user daemon-reload
~/.local/bin/squirebot --status >/dev/null 2>&1 || ~/.local/bin/squirebot --setup   # first-run onboarding if unconfigured
systemctl --user enable --now squirebot.service
# optional, see DECISION: loginctl enable-linger "$USER"
```

#### DECISION — systemd linger (FLAGGED, can't be settled from research alone)
A systemd **user** service runs only while the user has an active login session UNLESS `loginctl enable-linger <user>` is set; with linger, it starts at boot and survives logout [CITED: ArchWiki Systemd/User; deardevices.com].

**The judgment call:** The watcher only needs to run *while the guildie is playing P99* — i.e., while they're logged into their desktop. In that window a plain user service (no linger) is already running. So **linger is NOT strictly required** for the core use case, and enabling it means the watcher runs 24/7 even when nobody's playing (more uploads of unchanged files — though the mtime-cache makes those no-ops, and idle inotify is cheap).

- **Recommend (default):** Do NOT auto-enable linger in `install.sh`; the unit starts on desktop login (the moment a guildie would play). Mention linger in the README as an opt-in for "always-on" boxes.
- **Counter-argument:** if a guildie runs a headless box and connects via SSH (no graphical login), the user manager may not auto-start at boot without linger. For that audience, `enable-linger` is needed.

**Resolution path:** this is a one-line `install.sh` choice. Suggest the planner present it as a `--linger` flag to `install.sh` (off by default), OR settle it in discuss-phase. It does NOT block the build. **Risk if wrong:** low — a guildie whose watcher didn't autostart runs `systemctl --user start squirebot` or re-runs install with `--linger`.

#### selfupdate as a systemd service [VERIFIED via source]
`minio/selfupdate.Apply` on Linux: writes `.target.new`, renames running `target → .target.old`, renames `.target.new → target`, then **deletes `.target.old`** (no Windows hide-dance), and does NOT require the running process to release its file handle (Linux permits renaming an open binary; the running process keeps the old inode) [CITED: github.com/minio/selfupdate/apply.go]. This composes cleanly with the existing flow: the daily check (`update.RunDailyCheck`) stages `<exe>.new` + `.expected-sha256`; on next launch `update.Apply()` (main.go:81) swaps and `os.Exit(0)`s [VERIFIED: swap.go:131-153, check.go]. Under `Restart=always`, that exit triggers systemd to relaunch the now-new binary within `RestartSec`. **No code change to `internal/update` is required for the swap itself** — it already works on Linux.

**One nuance:** `swap.go`'s success path does `os.Remove(exe + ".old")` (line 150) as best-effort cleanup; on Linux `selfupdate` already deleted it, so the `os.Remove` is a harmless no-op (ENOENT ignored). No change needed.

#### Update manifest / asset naming (THE one update-path change needed) [VERIFIED]
The updater picks its asset from `latest.json`'s `binary_url` field [VERIFIED: manifest.go:56-63, check.go:101-123 downloads `m.BinaryURL`]. Today `release.yml` writes a SINGLE `binary_url` pointing at `squirebot.exe` (release.yml:222). A Linux watcher reading the SAME manifest would try to download the **Windows** `.exe`. Options:

- **Minimal (recommended):** Add OS-specific fields to the manifest, e.g. `binary_url_linux` + `binary_sha256_linux`, and have `internal/update` pick the field by `runtime.GOOS`. This is a small, backwards-compatible `Manifest` struct addition (the doc at manifest.go:42-44 says additions must be new-fields-only — honored) + a `runtime.GOOS` branch in `check.go` where it reads `m.BinaryURL`/`m.BinarySHA256`. The Linux release job writes a `.tar.gz` asset AND a bare `squirebot` linux binary (the updater swaps the BINARY, not the tarball — `selfupdate.Apply` replaces the executable in place), so `binary_url_linux` should point at the bare linux `squirebot` binary asset, not the tarball.
- The Windows `binary_url`/`binary_sha256` stay exactly as-is (Windows unchanged, D-07).

[ASSUMED A2 — exact manifest field names] `binary_url_linux`/`binary_sha256_linux` vs a nested `assets: {windows:…, linux:…}` map. Either works; the flat new-field form is the smaller diff and keeps Phase-1/2 manifests parsing. The planner should pick one and update BOTH `manifest.go` (struct + nothing else; `Fetch` is generic) and `check.go` (the `m.BinaryURL` read → GOOS-selected) and `release.yml` (emit the linux fields + asset).

**Note:** the Linux updater also needs the bare `squirebot` (linux) binary published as a release asset (alongside the `.tar.gz`), because `selfupdate.Apply` swaps the raw executable — it can't unpack a tarball. So the release job uploads: `squirebot-linux-amd64.tar.gz` (for humans/install.sh) AND `squirebot` (bare, for the auto-updater's `binary_url_linux`).

### §4 — credstore + eqfind specifics

#### §5 — Credential 0600 file (D-03) [VERIFIED interface]
`credstore` exposes exactly three functions: `Store(code string) error`, `Read() (string, error)`, `Delete() error` [VERIFIED: store.go:43-71]. The Windows impl is un-tagged today; split it: rename `store.go` → `store_windows.go` (add `//go:build windows`), add `store_other.go` (`//go:build !windows`).

Conventional shape (matches gh/aws/ssh token convention):
- File: `$XDG_CONFIG_HOME/squirebot/guild_code` (a dedicated file, NOT inside `config.json` — config.go's SECURITY comment forbids secrets in the config struct [VERIFIED: config.go:4-6,21], and a separate file lets `Delete` just `os.Remove`).
- Mode `0600` (file), `0700` (dir). Use the same atomic `.tmp`→rename write the config uses [VERIFIED: config.go:101-124] so a crash can't leave a truncated token.
- `Read` returns a not-found error when the file is absent — the caller (`app.RunApp` at runapp.go:84-85) treats `err != nil || code == ""` as "needs onboarding," so a plain `os.ReadFile` ENOENT satisfies the contract.

Security notes to capture (carry the existing rules): the guild code is a **static reusable bearer token** — treat like an API token. NEVER world-readable (0600 enforces), NEVER logged (V7 rule, carried from store.go:11-12), NEVER written into `config.json`. `os.WriteFile(path, code, 0600)` does NOT guarantee 0600 if the file pre-exists with looser perms — `os.WriteFile` keeps existing perms; so `Store` should `os.Remove` then create, OR `os.Chmod(path, 0600)` after write, to be safe. [ASSUMED A3 — the chmod-after-write detail; planner should include it]

```go
// internal/credstore/store_other.go  //go:build !windows
func storePath() (string, error) {
    dir, err := os.UserConfigDir(); if err != nil { return "", err }
    return filepath.Join(dir, "squirebot", "guild_code"), nil
}
func Store(code string) error {
    p, err := storePath(); if err != nil { return err }
    if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil { return err }
    tmp := p + ".tmp"
    if err := os.WriteFile(tmp, []byte(code), 0o600); err != nil { return err }
    return os.Rename(tmp, p)
}
func Read() (string, error) {
    p, err := storePath(); if err != nil { return "", err }
    b, err := os.ReadFile(p); if err != nil { return "", err }
    return strings.TrimSpace(string(b)), nil
}
func Delete() error {
    p, err := storePath(); if err != nil { return err }
    return os.Remove(p)
}
```

#### eqfind (D-04) [VERIFIED interface]
`discover.go`'s cascade is `knownPathsProbe → registryProbe → heuristicProbe` [VERIFIED: discover.go:40-51]. `registryProbe`/`heuristicProbe` already no-op off Windows via `runtime.GOOS` checks (discover.go:98-116) calling into `scanUninstallKeys()`/`heuristicScan()` (the `_other` stubs). For Linux:
- `registry_other.go` stays a no-op (`return ""`) — no Linux registry. [VERIFIED: registry_other.go]
- `heuristic_other.go` gets the real WINE walk (§2), replacing `return ""`. [VERIFIED: heuristic_other.go]
- BUT note: `defaultHeuristicScan()` (discover.go:111-116) currently early-returns `""` when `runtime.GOOS != "windows"`. **That guard must be relaxed on Linux** so `heuristicScan()` actually runs. Either drop the GOOS guard (let the `_other` impl decide) or extend it to `windows||linux`. [VERIFIED: discover.go:111-116 — this is a small but easy-to-miss edit]
- Same for `defaultKnownPaths()` (discover.go:71): it hardcodes `C:\…` Windows literals. Branch on `runtime.GOOS` (or split `knownpaths_windows.go`/`knownpaths_other.go`) so Linux gets the `$WINEPREFIX`/`~/.wine` candidate list. [VERIFIED: discover.go:71-94]

`ValidateFolder` (both sentinels) is platform-agnostic and unchanged [VERIFIED: discover.go:56-67].

### §6 — XDG base dirs (D-05) [VERIFIED + CITED]
- **Config:** `os.UserConfigDir()` returns `$XDG_CONFIG_HOME` or `~/.config` on Unix [CITED: pkg.go.dev/os]. `defaultPath()` (config.go:45-47) branches: Windows → `LOCALAPPDATA`, else → `filepath.Join(userConfigDir, "squirebot", "config.json")`. `runtime.GOOS` branch is the right idiom (the function imports nothing platform-specific) — no build-tag file needed. [VERIFIED: config.go:44-47]
- **Logs:** `logging.Setup()` (logger.go:25-30) hardcodes `LOCALAPPDATA`. Branch on `runtime.GOOS`: Windows → `LOCALAPPDATA`, else → `$XDG_STATE_HOME` or `~/.local/state` (stdlib has NO `UserStateDir`, so hand-roll: `os.Getenv("XDG_STATE_HOME")`, fallback `filepath.Join(home, ".local/state")`), then `…/squirebot/`. If STATE is awkward, D-05 permits config-dir fallback. [VERIFIED: logger.go:25-30]
- Modes (`0755` dir, `0600` config) and the atomic write are reused unchanged [VERIFIED: config.go:104,112; logger.go:36].

`runtime.GOOS` branching (not a tagged file) is correct for BOTH because neither function imports a platform package — the existing code already uses `runtime.GOOS` for exactly this kind of branch in `eqfind/discover.go`.

### §7 — Cross-compile + CI (D-07) [VERIFIED pipeline]

**Build invocation** (cross-compiles fine from the Windows dev box — the server already does this):
```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.Version=<ver> -X main.BackendBaseURL=https://api.squirebot.quest" \
  -o dist/squirebot ./cmd/squirebot
```
Note: NO `-H=windowsgui` (that's a Windows-only linker flag — the Linux build omits it). The two `-X` vars match `build_constants.go` (`Version`, `BackendBaseURL`) [VERIFIED: build_constants.go:16-19].

**CI integration:** `release.yml` is a LINEAR explicit-step Windows-runner workflow (does NOT use goreleaser) [VERIFIED: release.yml:11-14,51]. Add the Linux build as ADDITIVE steps in the SAME job (the Windows runner cross-compiles Linux fine) OR a parallel job. Recommended additive steps after the existing Windows binary build:
1. Build `dist/squirebot` (linux/amd64, command above).
2. Assemble the tarball: `dist/squirebot`, `README` (linux), `squirebot.service`, `install.sh` → `tar -czf squirebot-linux-amd64.tar.gz`.
3. Compute SHA-256 of BOTH the tarball and the bare `dist/squirebot` (the bare binary is what the auto-updater swaps; §3).
4. Extend the "Write latest.json" step to ALSO emit `binary_url_linux` + `binary_sha256_linux` (pointing at the bare `squirebot` linux asset). Windows fields unchanged.
5. Upload `squirebot-linux-amd64.tar.gz` + `squirebot` (bare) as release assets alongside the Windows ones.

The Windows NSIS path (NSIS install/verify, makensis, installer alias) is **untouched** (D-07). `.goreleaser.yaml` is only for local snapshot builds and CI ignores it [VERIFIED: .goreleaser.yaml:1-9]; OPTIONALLY add a `goos: [linux]` build there too for local `goreleaser build --snapshot`, but it's not load-bearing for the release.

**Acceptance gate for the build:** after tagging systray (§1), `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` succeeds AND `go list -deps ./cmd/squirebot | grep systray` is empty AND the Windows build (`GOOS=windows … go build ./cmd/squirebot`) still succeeds byte-compatibly. Run `go vet ./...` + `go test ./...` on the dev box (Windows) to confirm the `!windows` files at least compile under cross-build (`GOOS=linux go vet ./...`).

## Common Pitfalls

### Pitfall 1: `runtime.GOOS` guard that doesn't remove the systray import
**What goes wrong:** Planner gates `systray.Run` behind `if runtime.GOOS=="windows"` but leaves `import "fyne.io/systray"` in `main.go` → the Linux build still tries to CGO-link GTK → `CGO_ENABLED=0` build fails with linker errors.
**How to avoid:** Build-tag the IMPORT site (separate `_windows.go`/`_other.go` files), not a runtime branch. The `go list -deps | grep systray` gate catches it.
**Warning signs:** `# fyne.io/systray` or `undefined reference to gtk_…` in the linux build output.

### Pitfall 2: Forgetting the `defaultHeuristicScan` / `defaultKnownPaths` GOOS guards
**What goes wrong:** You implement a perfect `heuristic_other.go` WINE walk, but `discover.go:111` still early-returns `""` for non-Windows, so the walk never runs. EQ auto-discovery silently always falls back to the CLI prompt.
**How to avoid:** Relax the `runtime.GOOS != "windows"` guard in `defaultHeuristicScan()` (and branch `defaultKnownPaths()`). [VERIFIED: discover.go:98-116]
**Warning signs:** `eqfind.Discover()` always returns `ErrNotFound` on Linux even when EQ is in `~/.wine`.

### Pitfall 3: Auto-updater downloads the Windows `.exe` on a Linux box
**What goes wrong:** `latest.json` has one `binary_url` = `squirebot.exe`; the Linux watcher's daily check downloads it, SHA matches (it's a valid file), `selfupdate.Apply` swaps a Windows PE over the Linux ELF → next launch fails `exec format error` → `Restart=always` loops a broken binary.
**How to avoid:** OS-specific manifest fields (§3) + `runtime.GOOS` selection in `check.go`. Ship the bare linux `squirebot` as its own asset.
**Warning signs:** systemd journal shows `exec format error` restart loop after an update.

### Pitfall 4: `os.WriteFile` doesn't tighten perms on an existing file
**What goes wrong:** Re-onboarding writes the guild code to an existing `guild_code` file that somehow has `0644`; `os.WriteFile`'s mode arg is ignored when the file exists → token world-readable.
**How to avoid:** Write via `.tmp` + rename (fresh file gets 0600), or `os.Chmod` after write. The atomic-rename approach (mirroring config.go) gets this for free.

### Pitfall 5: systemd user service doesn't start at boot without linger (audience-dependent)
**What goes wrong:** A headless/SSH-only guildie enables the unit, reboots, and the watcher isn't running because there's no graphical login session and linger is off.
**How to avoid:** Document `loginctl enable-linger` (or an `install.sh --linger` flag) for headless boxes. For the common "desktop gamer" case it's a non-issue (login starts it). See the §3 DECISION.

## Runtime State Inventory

> This is a build-target-addition phase, not a rename/refactor. No existing runtime state is renamed or migrated. The Linux watcher is a NEW artifact reading NEW (XDG) paths; the Windows watcher's state (`%LOCALAPPDATA%`, wincred) is untouched.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None renamed — Linux uses NEW paths (`~/.config/squirebot/`, `~/.local/state/squirebot/`); no migration of any Windows store. | None |
| Live service config | None — the systemd user unit is NEW; no existing unit to re-register. | None |
| OS-registered state | NEW systemd user unit `~/.config/systemd/user/squirebot.service` (the Linux `HKCU\Run` analog). Created by `install.sh`, not migrated. | install.sh registers it |
| Secrets/env vars | The bearer guild code moves from wincred (Windows) to a NEW `0600` file (Linux) — these are DIFFERENT machines; no cross-platform migration. The token VALUE is the same reusable v2.1 code the guildie pastes. | None (fresh onboarding per box) |
| Build artifacts | NEW release assets: `squirebot-linux-amd64.tar.gz` + bare `squirebot`. The Windows `squirebot.exe`/installer assets are unchanged. | release.yml emits the new assets |

**Nothing is renamed or migrated** — verified by reading all platform-surface files; the phase is purely additive (D-07).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Linux daemon via cron `@reboot` / rc.local / `.desktop` autostart | systemd **user** services (`WantedBy=default.target`, `Restart=always`) | systemd-ubiquitous since ~2015 | the standard per-user-daemon mechanism; `loginctl enable-linger` for logout survival [CITED: ArchWiki] |
| XDG via manual `$XDG_CONFIG_HOME` parsing | `os.UserConfigDir()` stdlib (Go 1.13+) | Go 1.13 | one stdlib call; only `$XDG_STATE_HOME` still needs hand-roll [CITED: golang/go#29960] |

**Deprecated/outdated:** none relevant — all the watcher's existing deps already support linux/amd64 CGO-free.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Prune list for the WINE `drive_c` walk should NOT prune `Program Files` (EQ-under-WINE often installs there), unlike the native-Windows scan. | §2 | Low — if pruned, EQ-in-Program-Files is missed → CLI-prompt fallback still works |
| A2 | Manifest gains flat `binary_url_linux`/`binary_sha256_linux` fields (vs a nested per-OS map). | §3 | Low — both work; flat form is the smaller diff and keeps old manifests parsing |
| A3 | `Store` should chmod-after-write or write-via-tmp to guarantee 0600 on a pre-existing file. | §5 | Low-Med — a world-readable token is a real (if local-only) leak; easy to get right |
| A4 | Exact tray `Controller` field/method set to reproduce in `tray_other.go`. | Pattern 1 | Med — a missed symbol = compile error on Linux (caught immediately by the build) |
| A5 | `--setup`/`--status` arg-dispatch ordering in `main.go`. | Pattern 3 | Low — mirrors the existing `--quit` idiom; planner decides exact placement |

**These assumptions are low-risk because the CLI/build feedback loop catches structural mistakes immediately, and every gray-area degrades to the CLI-prompt fallback rather than failure.** The one genuine open DECISION (systemd linger, §3) is policy, not mechanics.

## Open Questions

1. **systemd linger default (the one real decision).**
   - What we know: a user service runs on desktop login without linger; survives logout/boot-without-login only WITH linger [CITED].
   - What's unclear: whether any target guildie runs P99 on a headless/SSH-only box (would need linger) vs. a normal desktop (doesn't).
   - Recommendation: default OFF, expose `install.sh --linger`, document in README. Settle in discuss-phase if the audience is known. Does NOT block the build.

2. **arm64 (deferred per D-07 "unless trivial").**
   - It IS trivial to ADD (`GOOS=linux GOARCH=arm64 CGO_ENABLED=0`) since the binary is CGO-free — same build line, different `GOARCH`. But it adds a second asset + a second manifest entry + an untestable target (no arm64 P99/WINE box). Recommendation: skip for this phase (keep scope tight); note it's a one-line add later.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go cross-compile (linux/amd64) | the build | ✓ (dev box already cross-compiles; server uses it) | go 1.25.7 [VERIFIED: go.mod:3] | — |
| systemd (user) | autostart on the GUILDIE's box | n/a (target machine, not dev) | — | manual `squirebot &` / desktop autostart |
| WINE/Lutris/Proton/Bottles | the guildie's P99 install | n/a (target machine) | — | CLI prompt for EQ path (D-04) |
| A Linux+WINE box for live UAT | the smoke test | ✗ (no such box in this env) | — | ship built+unit-tested tarball; human UAT (per CONTEXT specifics) |

**Missing dependencies with no fallback:** none block the BUILD. The live "watches the WINE EQ folder + uploads" confirmation needs a Linux+WINE box → human UAT, exactly like the Windows watcher's on-machine UATs (CONTEXT specifics). This phase's deliverable is a **built, unit-tested, packaged** tarball.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test ./...`) |
| Config file | none (standard Go) |
| Quick run command | `go test ./internal/credstore/... ./internal/eqfind/... ./internal/config/...` |
| Full suite command | `go test ./...` (Windows dev box) + `GOOS=linux go vet ./...` (cross-compile sanity) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LNX-01 | linux/amd64 CGO-free build, no systray in closure | build/smoke | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./... && go list -deps ./cmd/squirebot \| Select-String systray` (expect none) | ❌ Wave 0 (CI step) |
| LNX-01 | Windows build still compiles | build | `GOOS=windows go build ./cmd/squirebot` | ✅ (existing CI) |
| LNX-02 | credstore file Store/Read/Delete round-trip + 0600 perms | unit | `go test ./internal/credstore/...` (new `store_other_test.go`, tag `!windows`) | ❌ Wave 0 |
| LNX-02 | config defaultPath → XDG on linux; log path → XDG_STATE | unit | `go test ./internal/config/... ./internal/logging/...` | ❌ Wave 0 (new linux-path cases) |
| LNX-03 | eqfind WINE walk finds sentinel pair under a fake prefix; falls back to ErrNotFound | unit | `go test ./internal/eqfind/...` (new `heuristic_other_test.go`, mirrors Phase-24 walk tests) | ❌ Wave 0 |
| LNX-04 | CLI PromptGuildCode/PickEQFolder parse stdin; empty/EOF → ErrCancelled | unit | `go test ./internal/onboarding/...` (new `dialog_other_test.go` with an `io.Reader` seam) | ❌ Wave 0 (may need a stdin seam) |
| LNX-06 | fsnotify/debounce/snapshot/schema-gate/rotation already pass | unit | `go test ./internal/watch/... ./internal/update/...` (existing, platform-agnostic) | ✅ |

### Sampling Rate
- **Per task commit:** `go test ./internal/<touched>/...` + `GOOS=linux go vet ./...`
- **Per wave merge:** `go test ./...` (Windows) + `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...`
- **Phase gate:** full suite green + linux build green + systray absent from linux closure + Windows binary unchanged, before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/credstore/store_other_test.go` — covers LNX-02 (round-trip + 0600). Needs `store.go`→`store_windows.go` rename first.
- [ ] `internal/eqfind/heuristic_other_test.go` — covers LNX-03 (WINE walk under a `t.TempDir()` fake prefix; mirror Phase-24's eqfind walk tests).
- [ ] `internal/onboarding/dialog_other_test.go` — covers LNX-04 (stdin parse). May require a small `var stdin io.Reader = os.Stdin` seam to inject test input.
- [ ] `internal/config` + `internal/logging` — add linux-path cases (XDG resolution) to the existing test files; may need a `pathFn`/env seam (config already has `pathFn` [VERIFIED: config.go:42]).
- [ ] CI: linux build step + `go list -deps | grep systray` gate (the LNX-01 acceptance gate) added to `release.yml` (or a new `linux-build.yml` PR check mirroring `apps-script-build.yml`).

## Security Domain

> `security_enforcement` not explicitly false; included. The watcher is a local-only client handling one secret (the bearer guild code).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | partial | the bearer guild code (presented to backend); unchanged from Windows — no new auth surface |
| V6 Cryptography | no (no new crypto) | the token is stored at-rest as plaintext behind FS perms (D-03 accepts this — a headless box has no DPAPI/keyring equivalent without a CGO daemon dep); SHA-256 verify on self-update is existing + unchanged |
| V7 Secret Logging | yes | NEVER log the guild code (carried rule, store.go:11-12); the new file impl + `--status` must not print it |
| V12 File/Resource | yes | `0600` token file, `0700` dir; bounded + no-symlink-follow WINE walk (carries the existing heuristic threat model: depth cap + timeout + prune) |

### Known Threat Patterns for the Linux watcher
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Token readable by other local users | Information disclosure | `0600` file in `~/.config` (per-user); chmod-after-write to be safe (A3) |
| `--status` leaking the token to a shared terminal/journal | Information disclosure | `--status` prints config path + health, NOT the code (V7) |
| Self-update swapping a wrong-arch/forged binary | Tampering | existing SHA-256 verify in `check.go`/`swap.go` (unchanged); plus OS-specific asset selection (§3 Pitfall 3) so arch can't be crossed |
| WINE walk following a symlink out of the prefix into a denied/huge tree | DoS / traversal | `filepath.WalkDir` no-symlink-follow + depth cap + 30s timeout (carried from heuristic_windows.go) |
| systemd unit running with excess privilege | Elevation | it's a USER service (no root); optional `NoNewPrivileges=true` hardening |

## Sources

### Primary (HIGH confidence — codebase, cited by path:line)
- `cmd/squirebot/main.go` — systray import + Run/Quit call sites (15, 183, 205); arg-dispatch idiom (38-66); update.Apply (81); RunApp wiring (128-165)
- `internal/tray/tray.go` — systray import (30) + full Controller API to reproduce
- `internal/credstore/store.go` — Store/Read/Delete interface (43-71); un-tagged; security rules (11-12)
- `internal/eqfind/discover.go` — cascade (40-51); `defaultKnownPaths` Windows literals (71-94); GOOS guards on heuristic/registry (98-116); `ValidateFolder` (56-67)
- `internal/eqfind/heuristic_windows.go` — the walk pattern to mirror (depth cap, prune, timeout, no-symlink)
- `internal/eqfind/{heuristic_other.go,registry_other.go}` — the `!windows` stubs to fill / keep
- `internal/config/config.go` — `defaultPath` LOCALAPPDATA (44-47); atomic write + modes (101-124); `pathFn` seam (42); secrets-forbidden comment (4-6,21)
- `internal/logging/logger.go` — `Setup` LOCALAPPDATA log dir (25-30)
- `internal/onboarding/{dialog.go,dialog_other.go,dialog_windows.go}` — signatures (dialog.go), ErrUnsupported stubs to replace (dialog_other.go), sqweek isolated to dialog_windows.go (1,33)
- `internal/app/runapp.go` — RunApp takes `*tray.Controller` concrete (83); onboarding loop unchanged (136-202)
- `internal/update/{manifest.go,check.go,swap.go}` — manifest binary_url (manifest.go:56-63); GOOS-agnostic download (check.go:101-123); swap rename dance + `.old` cleanup (swap.go:131-153)
- `internal/system/shutdown_signal_other.go` — ctx-only shutdown already sufficient
- `.goreleaser.yaml` / `.github/workflows/release.yml` — linear Windows pipeline, goreleaser not used in CI (release.yml:11-14); build ldflags (release.yml:113-128); latest.json emission (204-229)
- `go.mod` — dep list + go 1.25.7

### Secondary (MEDIUM — external, verified against official docs)
- minio/selfupdate `apply.go` — Linux rename-in-place, deletes `.old`, no handle release needed [github.com/minio/selfupdate/blob/master/apply.go]
- Go stdlib `os.UserConfigDir` → `$XDG_CONFIG_HOME`/`~/.config` [pkg.go.dev/os; golang/go#29960]
- systemd user services + `loginctl enable-linger` [wiki.archlinux.org/title/Systemd/User]
- WINE/Lutris/Bottles/Proton prefix `drive_c` locations [forums.lutris.net; linux-gaming.kwindu.eu; github.com/ValveSoftware/Proton/wiki; ArchWiki Wine]

### Tertiary (LOW — single-source/convention, flag for UAT)
- Exact Lutris/Bottles/Flatpak nested paths vary by version + user config — the scan is generous+bounded with a CLI fallback; a real Linux+WINE guildie box is the true validation.

## Metadata

**Confidence breakdown:**
- Build mechanics (CGO-free tray exclusion, cross-compile, manifest): HIGH — verified against code + selfupdate source + Go stdlib
- credstore/config/logging/onboarding ports: HIGH — interfaces verified by path:line; the work is filling known seams
- WINE scan paths (§2): MEDIUM — conventions cited from multiple sources but layout varies; CLI fallback de-risks it
- systemd linger policy: a DECISION, not a confidence level — flagged for discuss-phase, doesn't block

**Research date:** 2026-06-06
**Valid until:** ~2026-09-06 (stable domain — systemd/WINE/Go conventions move slowly; selfupdate + deps are pinned)
