# Phase 25: Linux Watcher - Context

**Gathered:** 2026-06-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Produce a working **Linux build of the per-guildie watcher** so the handful of guildies who run Project 1999 under **WINE** on Linux can upload their `/outputfile inventory|spellbook` `.txt` files to the backend — same core behavior as the Windows watcher (watch the EQ folder → debounce → parse → upload over HTTPS with the static bearer guild code → auto-update), minus the Windows-only GUI/installer chrome.

**Cross-cutting platform phase** (like Phase 24) — NOT part of the v2.2 wantlist/Discord theme; appended to the active milestone by sequential numbering. Touches ONLY the watcher (`cmd/squirebot/`, `internal/{credstore,eqfind,config,logging,onboarding,tray,system,watch,update,app}`, build/release) — orthogonal to the backend/web/bot.

**Explicitly NOT in this phase:** macOS build (defer), a Linux system-tray GUI (we go headless), native `.deb`/`.rpm` packages (defer — tarball only), any backend/web/bot change, any change to the Windows watcher's behavior (the Windows build must remain byte-for-byte equivalent — Linux work is additive via build tags / `runtime.GOOS` branches).
</domain>

<decisions>
## Implementation Decisions

### UI mode (the load-bearing choice)
- **D-01 (Headless daemon):** The Linux watcher is **headless** — NO `fyne.io/systray` tray on Linux. It runs as a background process (a **systemd user service**), keeping the binary **`CGO_ENABLED=0` fully static** (no GTK/appindicator/Zenity, no CGO). The tray controller becomes a no-op/logging implementation on Linux (build-tag split: real systray on Windows, headless controller on `!windows`), so `RunApp`'s `trayCtl.SetStatus/SetIconHealth` calls are unchanged. Status/health surfaces via the log + a CLI `--status` instead of a tray icon.

### Onboarding & control (CLI, no GUI)
- **D-02 (CLI onboarding):** First-run onboarding is a **CLI flow over stdin** (e.g. `squirebot --setup`): prompt for the guild code (the v2.1 reusable bearer token) and confirm/enter the EQ folder. The Win32 dialog path stays Windows-only; the `!windows` `onboarding` impl prompts on the terminal instead of returning `ErrUnsupported`. Add `--status` (print health/last-upload/config path) for a headless health check. NO localhost HTTP onboarding surface (avoid reintroducing a browser/loopback flow — the watcher stays browser-free, carried HARD CONSTRAINT).

### Credential storage
- **D-03 (0600 file, no keyring):** Store the bearer guild code in a **`0600` file under XDG config** (`$XDG_CONFIG_HOME/squirebot/`, default `~/.config/squirebot/`). NOT an OS keyring — a headless box may have no `secret-service`/D-Bus daemon, and a keyring lib adds a CGO/daemon dependency. A `0600` file in `~/.config` is the standard CLI-tool token convention (gh/aws/ssh). The `credstore` interface stays the same; add a `!windows` file-backed implementation beside the Windows `wincred` one.

### EQ-folder discovery
- **D-04 (Auto-scan WINE + config override):** Discovery probes, in order: the **`$WINEPREFIX`** env var, **`~/.wine/drive_c`**, and common **Lutris/Proton/Bottles** prefix locations, walking each (depth-bounded, the existing `eqfind` heuristic) for the `eqgame.exe`/`eqclient.ini` sentinels. If nothing is found, fall back to the CLI prompt (D-02) and persist the chosen path to config (the `EQFolders` multi-folder field already exists). Fill in the `!windows` stubs in `eqfind` (`registry_other.go` stays a no-op; `heuristic_other.go` + the known-paths list get real Linux logic).

### Paths (XDG)
- **D-05 (XDG base dirs):** Config at `$XDG_CONFIG_HOME/squirebot/config.json` (default `~/.config/squirebot/`); logs at `$XDG_STATE_HOME/squirebot/` (default `~/.local/state/squirebot/`) — state is the correct XDG class for logs, falling back to config-dir if unset. Branch `defaultPath()` (and the log path) on `runtime.GOOS`; Windows stays `%LOCALAPPDATA%`. Keep the existing atomic `.tmp`→rename write + `0755`/`0600` modes.

### Distribution & autostart
- **D-06 (Tarball + install script + systemd user unit):** Ship a **`.tar.gz`** containing the static `linux/amd64` binary, a README, a **systemd user unit** (`squirebot.service`, `Restart=always`, `WantedBy=default.target`), and an **`install.sh`** that installs the binary to `~/.local/bin`, drops + `systemctl --user enable --now` the unit, and runs first-time `--setup` if unconfigured. Autostart = the systemd user unit (the Linux equivalent of the `HKCU\Run` key); no NSIS, no UAC. The existing `minio/selfupdate` auto-update is reused as-is (it overwrites the binary directly on Linux — no `.exe` rename dance), so Linux watchers self-update like Windows ones; ensure the update manifest/asset naming covers the linux artifact.

### Build & CI
- **D-07 (additive build, Windows unchanged):** Add `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` to the build (goreleaser/release workflow) producing the tarball; the Windows NSIS path is untouched. Every Linux-specific change is behind `//go:build` tags or `runtime.GOOS` so `go test ./...` and the Windows artifact are unaffected. Verify the Windows build still compiles identically. (arm64 Linux deferred unless trivial.)

### Claude's Discretion
- Exact CLI UX/wording for `--setup` / `--status`; the systemd unit hardening directives; the `install.sh` ergonomics; the precise Lutris/Proton/Bottles scan-path list (research-informed); the XDG-state-vs-cache choice for logs if STATE is awkward; whether to also emit `linux/arm64`.
- Whether the headless tray controller lives as `tray_other.go` (no-op controller) vs a small interface in `app` — planner's call, as long as `RunApp` is unchanged and the Windows tray is intact.

### Locked constraints (carried — DO NOT violate)
- **No browser/loopback/OAuth in the watcher** (carried HARD CONSTRAINT — the v2.0/P13 browser-free decision). Onboarding is CLI, the credential is a static bearer token.
- **Windows watcher behavior must not change** — Linux is purely additive.
- The bearer **guild code is a static, reusable token** (v2.1) — persisted, not single-use.
</decisions>

<specifics>
## Specific Ideas

- Target audience: a few guildies running P99 via WINE/Lutris/Proton on Linux — CLI-comfortable; a background daemon + `--setup`/`--status` is the right ergonomic, not a tray GUI.
- The deliverable from THIS phase is a **built, unit-tested, packaged** Linux tarball ready for a Linux guildie to smoke-test live (the real "watches the WINE EQ folder + uploads" confirmation needs a Linux+WINE box → a human UAT, like the Windows watcher's on-machine UATs).
- Keep parity with the Windows watcher's robustness work (500 ms debounce, always-re-read on event, full-snapshot replace, schema-version gate, log rotation) — these are platform-agnostic and already pass on Linux.
</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project rules
- `CLAUDE.md` — watcher stack + the "Never use" list (no oob OAuth, no polling-instead-of-fsnotify, no trusting fsnotify payloads on Windows), schema-evolution + `WatcherMaxSchemaVersion` rule, structured slog.
- `.planning/PROJECT.md` — watcher core value + the browser-free / bearer-token constraints.

### The platform surface to port (already build-tag-split — fill the `!windows`/Linux side)
- `cmd/squirebot/main.go` + `console_windows.go` / `console_other.go` — entry point; the headless/systray-skip decision lands here.
- `internal/credstore/store.go` — the `wincred` DPAPI store; add a `!windows` 0600-file impl (D-03).
- `internal/eqfind/{discover.go,registry_windows.go,registry_other.go,heuristic_windows.go,heuristic_other.go}` — EQ discovery; implement the Linux WINE scan (D-04).
- `internal/config/config.go` — `defaultPath()` `%LOCALAPPDATA%` → XDG branch (D-05); `internal/logging/` log path.
- `internal/onboarding/{dialog.go,dialog_windows.go,dialog_other.go}` — replace the `!windows` `ErrUnsupported` stubs with CLI stdin prompts (D-02).
- `internal/tray/tray.go` — the `explorer.exe` call + the systray run loop; split so Linux gets a headless no-op controller (D-01).
- `internal/system/{shutdown_signal_windows.go,shutdown_signal_other.go}` — context-only shutdown on Linux is already sufficient.
- `internal/app/runapp.go` — `RunApp` lifecycle; `runOnboarding()` + `pickAndSaveEQFolder()` call sites; must stay unchanged in shape (the controller/onboarding impls change underneath).
- `internal/update/` (`swap.go`, `manifest.go`) — already cross-platform; confirm the Linux asset naming in the manifest (D-06).
- `internal/watch/watcher.go` — already cross-platform (fsnotify/inotify); no change expected.
- `.goreleaser.yaml` + `.github/workflows/release.yml` + `installer/squirebot.nsi` — the Windows pipeline; ADD a parallel Linux build/tarball (D-07), Windows path untouched.
- `go.mod` — `danieljoos/wincred` is Windows-only (build-tagged); no new CGO dep is allowed on the Linux path (headless keeps it CGO-free).

### Prior-art / patterns
- `.planning/phases/24-watcher-test-hardening-c1-c2-coverage/` — the most recent watcher-only phase (twin-handler refactor + eqfind walk tests); the `eqfind` tests there are the analog for new Linux-scan tests.
- `.planning/research/STACK.md` / `SUMMARY.md` — original watcher stack rationale (fsnotify, selfupdate, systray, NSIS).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (already cross-platform — no change)
- `internal/watch` (fsnotify/inotify), `internal/update` (minio/selfupdate), `internal/parse` (tab-separated parsing), the upload/sheet-client path, config schema, log rotation (lumberjack) — all platform-agnostic; they already work on Linux.
- The build-tag scaffolding already exists for console, eqfind registry/heuristic, shutdown-signal, and onboarding dialog — the Linux port is mostly **filling the `_other`/`!windows` side**, not new architecture.

### Established Patterns
- `//go:build windows` / `//go:build !windows` paired files (console_windows.go/console_other.go, etc.) — the idiom every Linux impl follows.
- `RunApp` talks to a tray *controller* via `SetStatus`/`SetIconHealth` (goroutine-safe, pre-Ready queue) — so a headless no-op controller drops in without touching `RunApp`.
- Atomic config write (`.tmp`→rename), 0600/0755 modes — reuse on Linux.

### Integration Points
- `main.go` decides tray-vs-headless (build tag / GOOS) and wires the CLI subcommands (`--setup`, `--status`).
- `credstore`, `eqfind`, `onboarding`, `config`/`logging` each gain a Linux implementation behind the existing build-tag seam.
- Release pipeline gains a Linux build + tarball artifact + (auto-update) manifest entry.
</code_context>

<deferred>
## Deferred Ideas

- **macOS build** — same `!windows` seams would extend to darwin, but out of scope now.
- **Linux system-tray GUI** — rejected for this phase (CGO/GTK/Zenity cost); could be a later opt-in.
- **Native `.deb`/`.rpm`/Flatpak/Snap packages** — tarball-only now; native packages are a later polish if demand appears.
- **linux/arm64** — only if trivial; otherwise deferred.
- **A Linux onboarding GUI** (Zenity) — CLI is the chosen path; GUI could be a later nicety.

### Reviewed Todos (not folded)
None — no pending todos matched this phase.
</deferred>

---

*Phase: 25-linux-watcher*
*Context gathered: 2026-06-06*
