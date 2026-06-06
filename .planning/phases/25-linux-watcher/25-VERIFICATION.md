---
phase: 25-linux-watcher
verified: 2026-06-06T00:00:00Z
status: human_needed
score: 6/6 must-haves verified (code); 1 on-machine UAT outstanding
overrides_applied: 0
human_verification:
  - test: "On a real Linux+WINE box: install the tarball via ./install.sh, point it at a WINE-prefix EQ folder, run P99 + /outputfile inventory, confirm the .txt is uploaded to https://api.squirebot.quest with the static bearer guild code"
    expected: "The watcher discovers the WINE EQ folder (or accepts the CLI-prompted path), uploads the snapshot over HTTPS, and the web view reflects the data"
    why_human: "Requires a real Linux host with WINE + a live P99 client + the live backend; cannot be exercised on the Windows dev box (no ELF execution, no systemd, no WINE)"
  - test: "systemd user-service lifecycle: ./install.sh enables squirebot.service; systemctl --user stop delivers SIGTERM → graceful exit (not SIGKILL); Restart=always relaunches; --linger opt-in survives logout"
    expected: "Clean start/stop/restart via systemctl --user; SIGTERM unwinds the run context (exit 0); no mid-ingest hard-kill"
    why_human: "No systemd / loginctl on the Windows dev box; the SIGINT/SIGTERM handler is code-verified but its live systemd interaction is on-machine only"
  - test: "Linux auto-update end-to-end: a Linux watcher reads latest.json, selects binary_url_linux (the bare ELF, never the .exe), SHA-256-verifies, swaps on next launch"
    expected: "selfupdate downloads + verifies + swaps the bare linux squirebot; a Windows-only (legacy) manifest is a no-op skip, never a wrong-arch download"
    why_human: "Requires a published GitHub Release with the linux assets + a running Linux watcher; the selection logic is unit-verified but the live download/swap is on-machine only"
  - test: "Run the new !windows unit tests on a Linux runner (go test ./internal/credstore/... ./internal/eqfind/... ./internal/onboarding/...)"
    expected: "TestFileStore_RoundTripAndPerms (0600), TestHeuristicScan_FindsEQUnderWinePrefix / RespectsDepthCap, TestPromptGuildCode/PickEQFolder cancel cases all PASS on linux/amd64"
    why_human: "These tests are compile-verified on the Windows host (cross-compile OK) but cannot EXECUTE here — the host cannot run a linux/amd64 ELF test binary; run-proof needs a linux box/CI"
---

# Phase 25: Linux Watcher Verification Report

**Phase Goal:** Guildies who run Project 1999 under WINE on Linux can install and run the watcher — a headless, fully-static (`CGO_ENABLED=0`) `linux/amd64` build that watches the WINE-prefix EQ folder, uploads `/outputfile` `.txt` over HTTPS with the static bearer guild code, and auto-updates — with the same robustness as the Windows watcher, minus the Windows GUI/installer chrome. ADDITIVE: the Windows build + `go test ./...` must remain unaffected.
**Verified:** 2026-06-06
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (mapped to the 6 ROADMAP Success Criteria)

| # | Truth (SC) | Status | Evidence |
|---|-----------|--------|----------|
| 1 | SC-1 / LNX-01: CGO-free headless linux/amd64 build; no systray; RunApp unchanged; Windows + `go test ./...` unaffected | ✓ VERIFIED | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/squirebot` → exit 0, output is a static ELF (`\x7fELF`). `go list -deps ./cmd/squirebot` (linux) `grep -cE 'fyne.io/systray\|sqweek/dialog'` = **0**. systray excluded via `//go:build` at BOTH sites: `internal/tray/{tray_windows.go,tray_other.go}` AND `cmd/squirebot/{run_windows.go,run_other.go}` (build-tag split, not a runtime.GOOS branch). `RunApp(ctx context.Context, cfg *config.Config, baseURL, version string, t *tray.Controller)` signature byte-unchanged (D-07). Windows `go build ./cmd/squirebot` → exit 0; `go test ./...` → exit 0 (0 failures). |
| 2 | SC-2 / LNX-02: 0600 guild-code file under `$XDG_CONFIG_HOME/squirebot/`; config+logs follow XDG; Windows wincred/%LOCALAPPDATA% untouched | ✓ VERIFIED | `internal/credstore/store_other.go` (`//go:build !windows`): atomic tmp+rename write at `os.UserConfigDir()/squirebot/guild_code`, dir 0o700, file 0o600; imports only os/filepath/strings; `grep slog\|wincred` = 0 (token never logged). `store_windows.go` retains wincred (8 refs), `//go:build windows`. `config.go::defaultPath` + `logger.go::defaultLogDir` branch on `runtime.GOOS` (Windows → `%LOCALAPPDATA%`, else XDG config / `$XDG_STATE_HOME`). Test fns `TestFileStore_RoundTripAndPerms` + `TestFileStore_StoreTightensPreexistingLoosePerms` present (assert 0o600). |
| 3 | SC-3 / LNX-03: WINE-prefix EQ discovery (WINEPREFIX → ~/.wine → Lutris/Proton/Bottles) for eqgame.exe/eqclient.ini + CLI fallback | ✓ VERIFIED | `discover.go::defaultHeuristicScan` guard relaxed to `GOOS != windows && != linux` (darwin still no-op); `heuristic_other.go` is a real bounded walk — `wineCandidateRoots()` enumerates WINEPREFIX/~/.wine/Lutris(native+winegames+Flatpak)/Bottles(native+Flatpak)/Steam-Proton compatdata, os.Stat-filtered + Glob-expanded + deduped; `walkWineRoot` mirrors the Windows walk (depth 5, 30s ctx timeout, no symlink, prune users/ProgramData/$Recycle.Bin/windows but NOT Program Files, first ValidateFolder wins). `knownpaths_other.go` has NO `C:\` literals. CLI fallback = `onboarding.PickEQFolder`. Test fns FindsEQUnderWinePrefix / RespectsDepthCap / DefaultKnownPaths present. |
| 4 | SC-4 / LNX-04: CLI `--setup` (stdin guild code + EQ folder) + `--status`; no Win32 dialog; no localhost/browser surface | ✓ VERIFIED | `dialog_other.go` real stdin `PromptGuildCode`/`PickEQFolder` via `var stdin io.Reader = os.Stdin` seam; empty/EOF → ErrCancelled; ErrUnsupported stubs gone. `app.RunSetup` is NEW + additive (reuses runOnboarding/pickAndSaveEQFolder, never calls watch.Run). `main.go` dispatches `--setup`/`--status` after logging.Setup+config.Load; `runStatus` prints "configured"/"not configured" (NEVER the code). `grep -rni 'localhost\|loopback\|http.ListenAndServe\|net.Listen' internal/onboarding` → only 3 negative-assertion doc comments, ZERO actual listeners. |
| 5 | SC-5 / LNX-05: `.tar.gz` (binary+README+systemd user unit+install.sh) + auto-update via linux manifest asset | ✓ VERIFIED | `manifest.go` adds `BinaryURLLinux`/`BinarySHA256Linux` (omitempty, additive) + `binaryAsset()` runtime.GOOS selector (windows→.exe, else→bare linux); `check.go` routes all 4 read sites through `binURL`/`binSHA` (no raw `m.BinaryURL` in download/verify path) → empty linux fields on a legacy manifest hit the existing skip no-op (never downloads the .exe). `packaging/linux/squirebot.service` (USER unit, Restart=always, RestartSec=5, NoNewPrivileges, WantedBy=default.target, NO ProtectHome). `install.sh` (`bash -n` clean) → ~/.local/bin, daemon-reload, first-run --setup gated on --status, `enable --now`, `--linger` opt-in (default OFF, loginctl enable-linger inside the flag branch). `release.yml` adds linux build (GOOS:linux)+tarball+bare-binary-hash+linux manifest fields+both assets uploaded; Windows path intact (squirebot.exe ×12, makensis ×12, -H=windowsgui retained). |
| 6 | SC-6 / LNX-06: fsnotify/debounce/snapshot-replace/schema-gate/log-rotation work on Linux (platform-agnostic) + new linux-path unit tests | ✓ VERIFIED (code) / ⚠ run-proof on linux | watch/update/config are platform-agnostic; `GOOS=linux go vet ./...` exit 0; linux test binaries compile for all 8 affected packages. New linux-path tests (credstore/eqfind/onboarding) are compile-verified on host; `internal/update` linux-selection tests RUN green on host. Live fsnotify→upload behavior is the human UAT (truth 1 of the human section). |

**Score:** 6/6 truths code-verified. The live on-machine behaviors (watch→upload, systemd lifecycle, auto-update swap, linux test execution) are human UATs — exactly the same class as the Windows watcher's on-machine UATs, NOT code gaps.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/tray/tray_other.go` | headless no-op Controller, identical API, no systray | ✓ VERIFIED | `//go:build !windows`; full exported surface; imports only log/slog |
| `cmd/squirebot/run_other.go` | headless tail + SIGINT/SIGTERM→cancel | ✓ VERIFIED | `signal.NotifyContext(ctx, SIGINT, SIGTERM)` → goroutine drives cancel(); no systray |
| `cmd/squirebot/run_windows.go` | systray.Run tail (windows) | ✓ VERIFIED | `//go:build windows`; systray.Run + named-event shutdown; no signal.Notify |
| `internal/credstore/store_other.go` | 0600-file store under XDG | ✓ VERIFIED | atomic tmp+rename 0o600 / 0o700; no slog/wincred |
| `internal/eqfind/heuristic_other.go` | bounded WINE walk | ✓ VERIFIED | depth 5 + 30s timeout + no-symlink + prune set |
| `internal/eqfind/knownpaths_other.go` | linux direct-hits, no C:\ | ✓ VERIFIED | WINEPREFIX/~/.wine roots; zero `C:\` literals |
| `internal/onboarding/dialog_other.go` | CLI stdin prompts, no browser | ✓ VERIFIED | ErrUnsupported stubs replaced; stdin seam |
| `internal/app/runapp.go` | additive RunSetup, RunApp unchanged | ✓ VERIFIED | `func RunSetup(...)` present; RunApp signature byte-unchanged |
| `internal/update/manifest.go` | linux fields + binaryAsset | ✓ VERIFIED | BinaryURLLinux/BinarySHA256Linux + GOOS selector |
| `packaging/linux/squirebot.service` | systemd user unit | ✓ VERIFIED | WantedBy=default.target, Restart=always, no ProtectHome |
| `packaging/linux/install.sh` | idempotent installer | ✓ VERIFIED | `bash -n` clean; enable --now; --linger opt-in; first-run --setup |
| `.github/workflows/release.yml` | additive linux build/tarball/manifest | ✓ VERIFIED | linux build+tarball+assets; Windows NSIS path untouched |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| main.go | runMainLoop | build-tag-split blocking tail (no systray.Run in main.go) | ✓ WIRED |
| run_other.go | cancel | signal.NotifyContext SIGINT+SIGTERM | ✓ WIRED |
| config.defaultPath / logger.defaultLogDir | XDG vs LOCALAPPDATA | runtime.GOOS branch | ✓ WIRED |
| credstore.store_other | $XDG_CONFIG_HOME/squirebot/guild_code | os.UserConfigDir + atomic 0o600 | ✓ WIRED |
| discover.defaultHeuristicScan | heuristicScan (WINE walk) | relaxed GOOS guard (windows\|\|linux) | ✓ WIRED |
| main.go | app.RunSetup + eqfind.Discover | --setup arg dispatch | ✓ WIRED |
| check.go | binaryAsset (m.BinaryURLLinux/SHA) | runtime.GOOS-selected asset | ✓ WIRED |
| release.yml latest.json | binary_url_linux | bare linux squirebot asset | ✓ WIRED |
| install.sh | ~/.config/systemd/user/squirebot.service | install + enable --now | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CGO-free linux build | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/squirebot` | exit 0, static ELF | ✓ PASS |
| systray/sqweek absent (linux) | `go list -deps ./cmd/squirebot \| grep -cE 'systray\|sqweek/dialog'` | 0 | ✓ PASS |
| Windows build intact | `go build ./cmd/squirebot` | exit 0 | ✓ PASS |
| Full host suite (additive) | `go test ./...` | exit 0, 0 failures | ✓ PASS |
| Linux cross-vet | `GOOS=linux go vet ./...` | exit 0 | ✓ PASS |
| Linux test compile (8 pkgs) | `GOOS=linux go test -c -o /dev/null ./internal/{credstore,eqfind,onboarding,app,update,config,logging,tray}` | all OK | ✓ PASS |
| install.sh POSIX syntax | `bash -n packaging/linux/install.sh` | exit 0 | ✓ PASS |
| no ProtectHome in unit | `grep -c ProtectHome squirebot.service` | 0 | ✓ PASS |
| linux update-selection tests | `go test ./internal/update/...` (host) | PASS (binaryAsset, wrong-OS no-op) | ✓ PASS |
| !windows unit tests EXECUTE | (linux/amd64 ELF test binary) | cannot run on Windows host | ? SKIP → human |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| LNX-01 | 25-01 | CGO-free headless linux build; no systray; RunApp unchanged; additive | ✓ SATISFIED | Truth 1 |
| LNX-02 | 25-01, 25-02 | 0600 XDG guild-code file + XDG config/logs; Windows untouched | ✓ SATISFIED | Truths 1,2 |
| LNX-03 | 25-02 | WINE-prefix EQ discovery + CLI fallback | ✓ SATISFIED | Truth 3 |
| LNX-04 | 25-02 | CLI --setup/--status; no Win32/browser | ✓ SATISFIED | Truth 4 |
| LNX-05 | 25-01, 25-03 | tarball + systemd user unit + install.sh + auto-update; graceful SIGTERM | ✓ SATISFIED (code); live UAT pending | Truths 5, run_other.go SIGTERM |
| LNX-06 | 25-02 | platform-agnostic fsnotify/debounce/snapshot/schema/rotation + new linux unit tests | ✓ SATISFIED (code); linux run-proof pending | Truth 6 |

All 6 LNX IDs from PLAN frontmatter (LNX-01..06) are accounted for in REQUIREMENTS.md (lines 43–48) and mapped to Phase 25. No orphaned requirements. (Note: REQUIREMENTS.md status table still shows LNX-01..04/06 as "Pending" and only LNX-05 as "Complete" — a stale tracking-table state, not a code gap; the code for all six is present and verified.)

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No stubs, TODO/FIXME, empty handlers, or hollow returns in the phase-25 diff | ℹ️ Info | The `dialog_other.go` ErrUnsupported placeholders were REPLACED with real stdin impls; `heuristic_other.go` `return ""` stub replaced with the real walk. The only `localhost`/`loopback` strings in internal/onboarding are negative-assertion doc comments. |

### Notes on `WatcherMaxSchemaVersion`

The phase brief asks to confirm `WatcherMaxSchemaVersion` is untouched. It is: the v2.0 "Off Google" cutover removed the watcher's sheet client, so no watcher code references `WatcherMaxSchemaVersion` anymore — the only remaining hit is a comment in a backend SQL migration. Nothing in the phase-25 diff touches it. Truth holds (untouched).

### Additive Guarantee

`git diff --name-only` across the full phase-25 commit range (excluding `.planning/`) touches ONLY: `cmd/squirebot/*`, `internal/{tray,credstore,eqfind,onboarding,app,config,logging,update}/*`, `packaging/linux/*`, `.github/workflows/release.yml`, `.gitattributes`, `.gitignore`. ZERO `internal/backendsrv`, `web/`, `apps-script/`, or bot changes. Windows build compiles, `go test ./...` is green, the Windows tray/credstore/dialog files retain `//go:build windows` and their bodies. Additive guarantee VERIFIED.

### Human Verification Required

See the `human_verification` frontmatter (4 items): the live WINE watch→upload, the systemd user-service lifecycle (SIGTERM/Restart/--linger), the live linux auto-update swap, and running the `!windows` unit tests on a linux runner. All four require a real Linux+WINE+systemd host / a published Release / a linux CI runner — none executable on this Windows dev box, and all are the same class as the Windows watcher's on-machine UATs.

### Gaps Summary

No code gaps. Every must-have truth (all 6 ROADMAP Success Criteria) is code-verified against the actual codebase: the linux/amd64 closure builds CGO-free as a static ELF with zero systray/sqweek, the build-tag split is at both import sites (not a runtime branch), the 0600 XDG credstore / WINE discovery / CLI onboarding / OS-aware auto-update / packaging all exist and are wired, and the additive guarantee holds (Windows build + `go test ./...` green, no backend/web/bot change). Status is `human_needed` solely because the live on-machine confirmations (watch→upload→autostart→self-update + linux test execution) require a real Linux+WINE box, exactly as the brief anticipated.

---

_Verified: 2026-06-06_
_Verifier: Claude (gsd-verifier)_
