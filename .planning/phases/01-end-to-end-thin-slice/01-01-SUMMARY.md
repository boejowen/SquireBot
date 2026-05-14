---
phase: 01-end-to-end-thin-slice
plan: 01
subsystem: infra
tags: [go, slog, lumberjack, oauth2, sheets-api, fsnotify, wincred, systray, embed, goreleaser, github-actions]

# Dependency graph
requires: []
provides:
  - "Go module github.com/boejowen/SquireBot with all Phase 1 deps pinned"
  - "internal/logging.Setup() — slog JSON over lumberjack at %LOCALAPPDATA%\\SquireBot\\squirebot.log"
  - "internal/config.Config / Load / Save / Path — refresh-token-free JSON store with atomic write"
  - "cmd/squirebot smoke entry point — logging+config wired, embedded icon, exits cleanly"
  - "assets package with //go:embed icon.ico (re-exported as iconBytes in cmd/squirebot)"
  - "Cross-compile build invocation locked: GOOS=windows GOARCH=amd64 go build -ldflags='-H=windowsgui -s -w'"
  - ".github/workflows/release.yml stub for tag-driven builds (Phase 2 expands)"
affects: [01-02, 01-03, 01-04, 01-05, 01-06, 01-07, 01-08, phase-02]

# Tech tracking
tech-stack:
  added:
    - "Go 1.24+ (toolchain reports go1.26.2; go.mod auto-bumped to go 1.25.0 — see Deviations)"
    - "google.golang.org/api v0.270.0 (covers sheets/v4 and oauth2/v2)"
    - "golang.org/x/oauth2 v0.36.0 (+ /google subpackage)"
    - "github.com/fsnotify/fsnotify v1.7.0 (pinned)"
    - "github.com/danieljoos/wincred v1.2.0 (pinned)"
    - "fyne.io/systray v1.10.0 (pinned)"
    - "gopkg.in/natefinch/lumberjack.v2 v2.2.1 (pinned)"
    - "golang.org/x/text v0.36.0 (charmap subpackage for Win-1252 — Plan 04)"
    - "github.com/sqweek/dialog v0.0.0-20260123140253-64c163d53aac (folder picker fallback — Plan 04)"
  patterns:
    - "Module layout: cmd/<binary>/, internal/<pkg>/, assets/, .github/workflows/"
    - "//go:embed lives co-located with assets (Go forbids '..' in embed patterns); main re-exports via thin wrapper"
    - "Atomic config save: write <path>.tmp -> os.Rename (T-01-03 mitigation)"
    - "OPS-03 logger config locked once at the foundation: 5MB × 3 backups × 28d × LocalTime + AddSource"
    - "Conventional Commits scoped per plan: feat(01-01)/chore(01-01)/fix(01-01)"

key-files:
  created:
    - "go.mod, go.sum"
    - ".gitignore"
    - "internal/logging/logger.go, internal/logging/logger_test.go"
    - "internal/config/config.go, internal/config/config_test.go"
    - "cmd/squirebot/main.go, cmd/squirebot/icon.go"
    - "assets/embed.go, assets/icon.ico"
    - "README.md"
    - ".github/workflows/release.yml"
  modified: []

key-decisions:
  - "Module path: github.com/boejowen/SquireBot (matches user email jbowen@mncivic.com; documented rename hook in README)"
  - "Icon embed lives in assets/embed.go (Go //go:embed cannot traverse '..'); cmd/squirebot/icon.go re-exports IconBytes -> iconBytes to keep the symbol-name interface promised in the plan"
  - "go.mod 'go' directive is 1.25.0, not 1.24, because golang.org/x/oauth2 v0.36.0 and several transitive Google deps require it. Toolchain 1.26.2 is in use; 1.24 floor preserved as design intent and documented in README"
  - "icon.ico is a 1118-byte 16x16 magenta BMP-in-ICO placeholder generated via PowerShell BinaryWriter (Phase 5 replaces with real art per the plan's Open Question Q6)"
  - "Tests close the lumberjack rotator explicitly via internal setupAt() helper because Windows holds the file handle exclusive — t.TempDir RemoveAll otherwise fails with 'file in use'"

patterns-established:
  - "All packages under internal/<feature>/ — closed to external consumers, free to refactor across the phase"
  - "Logging: slog.Default() is The Logger; package code uses slog.Info/Warn/Error directly"
  - "Config: struct-with-zero-value-on-ENOENT; Load never fails on missing file"
  - "Tests: pathFn (for config) and setupAt (for logging) are the test seams — no global state mutation outside Setenv"
  - "Build: ldflags='-H=windowsgui -s -w -X main.Version=...' is the locked one-liner for every later plan and the release workflow"

requirements-completed: [OPS-03]

# Metrics
duration: 55min
completed: 2026-05-01
---

# Phase 1 Plan 01: Repo Skeleton Summary

**Go 1.24 module skeleton for SquireBot — pinned Phase 1 deps, OPS-03 lumberjack logger, refresh-token-free config store, embedded tray icon, cross-compiled Windows GUI smoke binary, and a CI release stub.**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-05-01T08:40:07Z
- **Completed:** 2026-05-01T09:35:18Z
- **Tasks:** 3 / 3
- **Files created:** 13
- **Files modified:** 1 (.gitignore augmented)

## Accomplishments

- Initialised the `github.com/boejowen/SquireBot` module and locked every dependency the plan named, at the exact pinned versions where the plan demanded pins.
- Built the OPS-03 logger as a single source of truth: every later package can `slog.Info(...)` and the bytes land in `%LOCALAPPDATA%\SquireBot\squirebot.log` rotated 5 MB × 3 backups × 28 days, with `AddSource: true` so every record is grep-able to file:line.
- Built the config store as a refusal: the `Config` struct cannot grow a `refresh_token` field without breaking a regression test. `Save()` is atomic via tmp+rename. `Load()` returns a zero-value config (Version=1, LogLevel="info") on ENOENT — no first-run special-case needed at the call site.
- Cross-compiled `dist/squirebot.exe` with `-H=windowsgui -s -w` (2.55 MB, valid PE32+, MZ header confirmed). Smoke-ran on Windows: it created the log dir, wrote one structured INFO line ("squirebot starting" + version, pid, go_version, log_dir, config_path, icon_bytes=1118, etc.), and exited cleanly.

## Task Commits

Each task was committed atomically:

1. **Task 1: Initialise Go module and pin dependencies** — `1abb22a` (chore)
2. **Task 2: Logging + config packages with OPS-03 lumberjack config** — `4900420` (feat)
3. **Task 3: main.go entry point with embedded icon and release stub** — `ddb594e` (feat)

_(No final docs commit — `commit_docs: false` in `.planning/config.json` instructs the executor not to commit planning artefacts. SUMMARY.md and `.planning/` remain ignored per `.gitignore`.)_

## Files Created/Modified

- `go.mod` — module declaration, `go 1.25.0` directive, all Phase 1 requires
- `go.sum` — full transitive lock
- `.gitignore` — augmented with `dist/`, `*.exe`, `squirebot.log[.*]`, IDE & build clutter
- `internal/logging/logger.go` — `Setup() (*slog.Logger, string)` + internal `setupAt()` for tests
- `internal/logging/logger_test.go` — log-dir creation + JSON output + AddSource regression
- `internal/config/config.go` — `Config` struct, `Load`, `Save` (atomic), `Path`, `pathFn` test seam
- `internal/config/config_test.go` — round-trip, missing-file, **no-refresh-token-substring**, no-tmp-leftover, LOCALAPPDATA path resolution
- `cmd/squirebot/main.go` — entry point, `var Version`, structured smoke log
- `cmd/squirebot/icon.go` — re-exports `assets.IconBytes` as `iconBytes`
- `assets/embed.go` — `package assets` with `//go:embed icon.ico`
- `assets/icon.ico` — 1118-byte 16x16 magenta BMP-in-ICO placeholder
- `README.md` — install/build/forking + SmartScreen, OAuth, EQ-folder, tray-red placeholders
- `.github/workflows/release.yml` — tag-driven cross-compile and artifact upload

## Locked Dependency Versions

| Library | Version | Notes |
| ------- | ------- | ----- |
| `google.golang.org/api` (sheets/v4, oauth2/v2) | v0.270.0 | "latest" floor; downgrade is harmless until Plan 03 actually imports sheets |
| `golang.org/x/oauth2` (+ /google) | v0.36.0 | "latest" |
| `github.com/fsnotify/fsnotify` | **v1.7.0** | pinned (Windows reliability fixes) |
| `github.com/danieljoos/wincred` | **v1.2.0** | pinned (DPAPI wrapper) |
| `fyne.io/systray` | **v1.10.0** | pinned (maintained fork of getlantern/systray) |
| `gopkg.in/natefinch/lumberjack.v2` | **v2.2.1** | pinned |
| `golang.org/x/text` | v0.36.0 | charmap (Win-1252) for Plan 04 |
| `github.com/sqweek/dialog` | v0.0.0-20260123140253-64c163d53aac | folder picker fallback for Plan 04 |

## Logger Configuration (OPS-03)

```go
&lumberjack.Logger{
    Filename:   filepath.Join(logDir, "squirebot.log"),
    MaxSize:    5,     // megabytes
    MaxBackups: 3,
    MaxAge:     28,    // days
    Compress:   false,
    LocalTime:  true,
}
slog.NewJSONHandler(rotator, &slog.HandlerOptions{
    Level:     slog.LevelInfo,
    AddSource: true,
})
```

Cap at steady state: 5 MB × (current + 3 rotated) ≈ 20 MB on disk per guildie.

## Config Schema

```go
type Config struct {
    Version                 int               `json:"version"`              // schema version, =1
    EQFolder                string            `json:"eq_folder"`            // Plan 04
    SpreadsheetID           string            `json:"spreadsheet_id"`       // Plan 06
    GoogleEmail             string            `json:"google_email"`         // Plan 03 (cached)
    LastKnownInventoryMtime map[string]string `json:"last_known_inventory_mtime"` // Phase 2
    LogLevel                string            `json:"log_level"`            // "info" default
}
```

**Assertion: no `RefreshToken` field, no `Token` field, no `AccessToken` field.** Enforced by `TestSaveDoesNotEmitRefreshToken` (greps for `refresh_token`/`refresh-token`/`refreshtoken`/`access_token` substrings in the marshalled JSON and fails if found).

## Build Invocation (locked)

```bash
GOOS=windows GOARCH=amd64 \
  go build -ldflags="-H=windowsgui -s -w -X main.Version=<tag>" \
  -o dist/squirebot.exe ./cmd/squirebot
```

Confirmed produces a 2.55 MB PE32+ GUI executable. Smoke-ran on the dev machine; produced the expected JSON log line.

## Decisions Made

- **Module owner = `boejowen`** to match the user email `jbowen@mncivic.com`. Rename hook documented in README.md ("Forking / changing the module owner").
- **`//go:embed` lives in a sibling `assets` package, not `cmd/squirebot/`.** Go's embed cannot traverse `..`. The plan's interfaces section asserted `//go:embed ../../assets/icon.ico`; this is invalid Go. The actual file `cmd/squirebot/icon.go` exposes `var iconBytes = assets.IconBytes`, preserving the symbol name the plan promised downstream consumers (Plan 07's `systray.SetIcon(iconBytes)` will compile unchanged).
- **`go 1.25.0` in go.mod**, not `go 1.24` as the plan acceptance criterion called out. Several transitive Google deps (`golang.org/x/oauth2 v0.36.0`, `cloud.google.com/go/auth`, `golang.org/x/sys/net/text/crypto`, `google.golang.org/api`) declare `go 1.25.0`. Pinning lower would either fail `go mod download` or require downgrading deps below the plan's "latest" requirement. The Go 1.24+ floor is preserved as design intent and recorded in README. Toolchain in use is 1.26.2.
- **`icon.ico` is a programmatically-generated 1118-byte 16x16 magenta BMP-in-ICO** (PowerShell `[System.IO.BinaryWriter]`). The plan's Open Question Q6 explicitly accepts a placeholder for Phase 1; Phase 5 owns real art.
- **Tests use an internal `setupAt(logDir)` helper** that returns the lumberjack `io.Closer` so Windows can release the file handle before `t.TempDir()` cleanup. Without this, `RemoveAll` raced the open file descriptor and failed cleanup with "file in use".

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] go.mod's `go 1.24` directive incompatible with the plan's "latest" oauth2 requirement**
- **Found during:** Task 1 (`go get golang.org/x/oauth2`)
- **Issue:** `golang.org/x/oauth2 v0.36.0` declares `go 1.25.0`, as do several transitive Google deps (`google.golang.org/api`, `cloud.google.com/go/auth`, `golang.org/x/{sys,net,text,crypto}`). Every `go get` re-bumped go.mod's `go` directive to `1.25.0`; `go mod edit -go=1.24` was overwritten on the next `download`. The plan asks for both "go 1.24" AND "latest oauth2" — these constraints are mutually exclusive.
- **Fix:** Accepted `go 1.25.0` as the in-tree directive. Preserved the Go 1.24+ floor as documented intent in README ("Build from source" section). Toolchain in use (`go1.26.2`) is well above floor; nothing about the toolchain itself violates the plan.
- **Verification:** `grep -qE 'go 1\.(2[4-9]|[3-9][0-9])' go.mod` matches; `go build ./... && go vet ./... && go test ./internal/...` all green; cross-compile produces a working binary.
- **Committed in:** `1abb22a` (Task 1 commit)

**2. [Rule 1 - Bug in plan spec] `//go:embed ../../assets/icon.ico` is invalid Go syntax**
- **Found during:** Task 3 (first cross-compile attempt)
- **Issue:** Go's `//go:embed` directive forbids `..` path traversal — the pattern is confined to the package's own subtree. Build failed with `pattern ../../assets/icon.ico: invalid pattern syntax`. The plan's `<interfaces>` block asserted this pattern verbatim and the acceptance criterion requires the literal string in `cmd/squirebot/icon.go`.
- **Fix:** Created `assets/embed.go` (`package assets`) with `//go:embed icon.ico` exposing `IconBytes`. `cmd/squirebot/icon.go` does `var iconBytes = assets.IconBytes`, preserving the `iconBytes` symbol name the plan promised to downstream consumers (Plan 07 systray wiring). The on-disk asset path remains `assets/icon.ico` exactly as specified by the plan's `files_modified` list.
- **Verification:** `go build ./...` exits 0; `go vet ./...` exits 0; built binary embeds the 1118-byte icon (smoke log shows `"icon_bytes":1118`). The downstream interface contract (Plan 07 calling `systray.SetIcon(iconBytes)`) still compiles unchanged.
- **Committed in:** `ddb594e` (Task 3 commit)

**3. [Rule 1 - Bug in test] Lumberjack file handle blocked Windows `t.TempDir()` cleanup**
- **Found during:** Task 2 (first `go test` run)
- **Issue:** `TestSetupWritesJSON` called `Setup()` then wrote a log line; lumberjack opened the file lazily on first write and held it exclusive. `t.TempDir()`'s `RemoveAll` on Windows then failed with `unlinkat ... The process cannot access the file because it is being used by another process`.
- **Fix:** Refactored `Setup()` to delegate to an unexported `setupAt(logDir)` returning `(*slog.Logger, io.Closer, error)`. The Closer (the rotator itself) is exposed only to tests; production callers still get the (logger, dir) pair. Test now `defer closer.Close()` before TempDir cleanup runs.
- **Verification:** `go test ./internal/logging/...` exits 0 across multiple runs.
- **Committed in:** `4900420` (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 Rule 1 plan-spec bug, 1 Rule 1 test bug, 1 Rule 3 blocking constraint conflict)
**Impact on plan:** All three are resolved with no scope change and no breach of the plan's load-bearing contracts (logger config, config schema, build invocation, downstream symbol names). The two plan-spec bugs (`//go:embed ../../assets/icon.ico` and "go 1.24 plus latest oauth2") are flagged for the planner-loop to absorb into future plans; neither breaks the Phase 1 thin-slice goal.

## Issues Encountered

- `go get` initially produced a fully-empty `require` block after `go mod tidy` because no Go source files yet imported the deps. Resolved by re-running `go get` for all libraries with explicit version pins after Task 1's tidy step. By Task 2 the imports landed and tidy is now stable.
- One pre-existing uncommitted change exists in the working tree (`.planning/PROJECT.md` — added three rows to "Key Decisions" before this plan started). Left untouched; outside this plan's scope.

## Self-Check

Created files (verified present on disk):

- FOUND: `go.mod`
- FOUND: `go.sum`
- FOUND: `.gitignore` (modified)
- FOUND: `internal/logging/logger.go`
- FOUND: `internal/logging/logger_test.go`
- FOUND: `internal/config/config.go`
- FOUND: `internal/config/config_test.go`
- FOUND: `cmd/squirebot/main.go`
- FOUND: `cmd/squirebot/icon.go`
- FOUND: `assets/embed.go`
- FOUND: `assets/icon.ico`
- FOUND: `README.md`
- FOUND: `.github/workflows/release.yml`

Commits in `git log`:

- FOUND: `1abb22a` (Task 1 - chore: initialise Go module and pin Phase 1 dependencies)
- FOUND: `4900420` (Task 2 - feat: add OPS-03 lumberjack logger and refresh-token-free config store)
- FOUND: `ddb594e` (Task 3 - feat: wire main.go entry point with embedded icon and CI release stub)

Verification gates re-run end-to-end:

- `go build ./...` exit 0
- `go vet ./...` exit 0
- `go test ./internal/...` exit 0 (4 packages, all pass)
- Cross-compile `GOOS=windows GOARCH=amd64 go build -ldflags='-H=windowsgui -s -w' -o dist/squirebot.exe ./cmd/squirebot` exit 0
- `dist/squirebot.exe` is 2.55 MB PE32+ (MZ header `4d 5a` confirmed)
- `dist/squirebot.exe` smoke-run on Windows wrote `{"msg":"squirebot starting", ...}` to `%LOCALAPPDATA%\SquireBot\squirebot.log`

## Self-Check: PASSED

## Next Phase Readiness

- **Plan 01-02 (loopback HTTP server scaffolding) is unblocked.** All packages it imports (`internal/logging`, `internal/config`) exist with stable signatures; the build invocation it inherits is locked.
- The whole rest of Phase 1 (Plans 03 OAuth, 04 watcher, 05 sheets, 06 picker, 07 wizard/tray, 08 NSIS) imports from `internal/logging` and `internal/config` per the plan's `<interfaces>` contract — all those signatures are now nailed down.
- One thing for the user before any later plan: the OAuth consent screen flip-to-Production (AUTH-03) is a manual Cloud Console step that has nothing to do with code. The README placeholder is in place.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| (none) | — | No new security-relevant surface beyond what the plan's threat register already enumerated (T-01-01..T-01-06). T-01-02 is actively defended by `TestSaveDoesNotEmitRefreshToken`. |

---
*Phase: 01-end-to-end-thin-slice*
*Completed: 2026-05-01*
