---
phase: 01-end-to-end-thin-slice
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - go.mod
  - go.sum
  - cmd/squirebot/main.go
  - cmd/squirebot/icon.go
  - internal/logging/logger.go
  - internal/config/config.go
  - assets/icon.ico
  - .gitignore
  - .github/workflows/release.yml
  - README.md
autonomous: true
requirements: [OPS-03]
must_haves:
  truths:
    - "Running `go build ./...` from the repo root produces zero errors"
    - "When the binary executes, it writes a JSON-formatted log line to %LOCALAPPDATA%\\SquireBot\\squirebot.log"
    - "config.json reads/writes succeed without storing any field named refresh_token"
  artifacts:
    - path: "go.mod"
      provides: "Go 1.24 module declaration with locked deps"
      contains: "module github.com/"
    - path: "internal/logging/logger.go"
      provides: "slog + lumberjack JSON logger to %LOCALAPPDATA%\\SquireBot\\squirebot.log"
      contains: "lumberjack.Logger"
    - path: "internal/config/config.go"
      provides: "config.json read/write under %LOCALAPPDATA%\\SquireBot\\config.json (NEVER stores refresh_token)"
      contains: "filepath.Join"
    - path: "cmd/squirebot/main.go"
      provides: "Entry point that initialises logging + config and prints version"
      min_lines: 30
    - path: "assets/icon.ico"
      provides: "Tray icon embedded via go:embed"
  key_links:
    - from: "cmd/squirebot/main.go"
      to: "internal/logging/logger.go"
      via: "logging.Setup() returns *slog.Logger"
      pattern: "logging\\.Setup\\("
    - from: "internal/logging/logger.go"
      to: "gopkg.in/natefinch/lumberjack.v2"
      via: "lumberjack.Logger{MaxSize: 5, MaxBackups: 3}"
      pattern: "lumberjack\\.Logger"
    - from: "internal/config/config.go"
      to: "%LOCALAPPDATA%\\SquireBot\\config.json"
      via: "os.Getenv(\"LOCALAPPDATA\") + filepath.Join"
      pattern: "LOCALAPPDATA"
---

<objective>
Establish the SquireBot Go module skeleton: directory layout, dependency graph, structured logging
with rotation, non-secret config persistence, and a placeholder GoReleaser CI stub. This plan owns
no business logic — it produces the load-bearing scaffolding that every subsequent Phase 1 plan
imports from.

Purpose: Prevent every later plan from re-deciding the same project-shape questions (where logs
land, where config lives, what slog handler to use, what the import path is). Lock OPS-03 (5MB × 3
file rotation) at the foundation so no later code accidentally writes to a different log path.

Output: A buildable `cmd/squirebot/squirebot.exe` that on launch creates the log directory, writes
one structured INFO line ("SquireBot starting", version, pid), reads-or-creates an empty
config.json, then exits cleanly. Wave 2+ plans replace `main.go`'s body with real wiring.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md
@.planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md
@.planning/research/STACK.md
@./CLAUDE.md
</context>

<interfaces>
<!-- Contracts this plan exports for downstream plans (03, 04, 05, 07). Embed verbatim in plan text -->
<!-- so executors don't have to scavenger-hunt the codebase. -->

From internal/logging/logger.go:
```go
package logging

// Setup configures the global slog logger with a JSON handler writing to
// %LOCALAPPDATA%\SquireBot\squirebot.log via lumberjack rotation
// (MaxSize=5 MB, MaxBackups=3, MaxAge=28 days, LocalTime=true, Compress=false).
// Returns the *slog.Logger AND the directory used (so callers can locate logs
// for the tray "Open log folder" action — Plan 07).
func Setup() (*slog.Logger, string)
```

From internal/config/config.go:
```go
package config

// Config is the on-disk shape of %LOCALAPPDATA%\SquireBot\config.json.
// It MUST NOT contain the OAuth refresh_token (Plan 03 stores that in wincred).
type Config struct {
    Version                  int               `json:"version"`             // schema version, =1
    EQFolder                 string            `json:"eq_folder"`           // set by Plan 04 wizard step
    SpreadsheetID            string            `json:"spreadsheet_id"`      // set by Plan 06 picker
    GoogleEmail              string            `json:"google_email"`        // set by Plan 03 OAuth (cached)
    LastKnownInventoryMtime  map[string]string `json:"last_known_inventory_mtime"` // Phase 2 will populate; empty in Phase 1
    LogLevel                 string            `json:"log_level"`           // "info" default
}

// Load reads %LOCALAPPDATA%\SquireBot\config.json. Returns a zero-value Config
// (NOT an error) when the file does not exist. Returns an error only on parse failure.
func Load() (*Config, error)

// Save writes c to %LOCALAPPDATA%\SquireBot\config.json atomically (write-tmp + rename).
func (c *Config) Save() error

// Path returns the absolute path of the config file (for diagnostics).
func Path() string
```

From cmd/squirebot/main.go:
```go
//go:embed icon.ico
var iconBytes []byte
```

Build command (locked for all later plans):
```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w" \
  -o dist/squirebot.exe ./cmd/squirebot
```
</interfaces>

<tasks>

<task type="auto">
  <name>Task 1: Initialise Go module and pin dependencies</name>
  <files>go.mod, go.sum, .gitignore</files>
  <read_first>
    - .planning/research/STACK.md (lines 28-100, "Recommended Stack" + "Installation" sections)
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§1 "Standard Stack" — locked versions and the install one-liner block at lines 169-184)
    - ./CLAUDE.md (Technology Stack section — locked libs)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-12: GitHub Releases distribution)
  </read_first>
  <action>
    Run `go mod init github.com/boejowen/SquireBot` (use `boejowen` as the GitHub owner — matches
    user email `jbowen@mncivic.com`; if a different owner is preferred at build time, the developer
    can `go mod edit -module github.com/<owner>/squirebot` later — note this in README).

    Then `go get` each of the following at the EXACT versions/lines listed in 01-RESEARCH.md §1:
    - `google.golang.org/api/sheets/v4`         (latest)
    - `google.golang.org/api/oauth2/v2`         (latest — for /userinfo lookup)
    - `golang.org/x/oauth2`                      (latest)
    - `golang.org/x/oauth2/google`               (latest)
    - `github.com/fsnotify/fsnotify@v1.7.0`     (pin minor — Windows reliability fixes since 1.7)
    - `github.com/danieljoos/wincred@v1.2.0`    (pin minor — DPAPI wrapper)
    - `fyne.io/systray@v1.10.0`                 (pin minor — maintained fork of getlantern/systray)
    - `gopkg.in/natefinch/lumberjack.v2@v2.2.1` (pin patch — last known stable)
    - `golang.org/x/text/encoding/charmap`      (latest — Win-1252 decoder for parser in Plan 04)
    - `github.com/sqweek/dialog`                (latest — native folder picker fallback for Plan 04)

    Verify Go 1.24+ is installed: run `go version` and confirm output starts with `go1.24`. If the
    output is older, abort with a clear error message — do NOT downgrade dependency versions to
    fit an older Go.

    Set `go 1.24` in go.mod (require minor 24 explicitly — the toolchain directive may follow).

    Create .gitignore with at minimum: `dist/`, `*.exe`, `squirebot.log`, `squirebot.log.*`,
    `.envrc`, `.idea/`, `.vscode/`, `*.tmp`, `coverage.out`, `node_modules/` (in case Phase 3 ever
    runs from this repo).

    Run `go mod tidy` to validate no missing or dangling deps. Run `go build ./...` (which will
    initially fail until Tasks 2-5 land — that's expected; the verify step below builds AFTER the
    later tasks).
  </action>
  <verify>
    <automated>go mod download &amp;&amp; head -1 go.mod | grep -q 'module github.com/.*squirebot' &amp;&amp; grep -q 'go 1\.24' go.mod &amp;&amp; grep -q 'fsnotify v1.7' go.sum &amp;&amp; grep -q 'wincred v1.2' go.sum &amp;&amp; grep -q 'lumberjack.v2' go.sum &amp;&amp; grep -q 'sheets/v4' go.sum &amp;&amp; grep -q 'oauth2' go.sum</automated>
  </verify>
  <acceptance_criteria>
    - `go.mod` exists at repo root
    - `go.mod` first line matches regex `^module github\.com/.+/squirebot$`
    - `go.mod` contains a line `go 1.24` (no minor lower)
    - `go.sum` exists and contains entries for: `fsnotify v1.7`, `wincred v1.2`, `lumberjack.v2`, `sheets/v4`, `oauth2`, `systray`, `charmap`/`text`, `sqweek/dialog`
    - `.gitignore` contains `dist/` and `*.exe` and `squirebot.log` lines
    - `go mod download` exits 0
  </acceptance_criteria>
  <done>
    Module initialised. All Phase 1 dependencies pinned in go.sum. `go mod download` exits 0.
    `.gitignore` excludes build artefacts and logs.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Create logging + config packages with the locked OPS-03 lumberjack config</name>
  <files>internal/logging/logger.go, internal/config/config.go</files>
  <read_first>
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§10 "Logging" — full §10.1 code block lines 1043-1075 — copy verbatim and adapt; §1 stack list to know slog is stdlib)
    - .planning/research/ARCHITECTURE.md (Configuration section — schema of config.json; the watcher config block listing version/eq_folder/spreadsheet_id/log_level/last_known_inventory_mtime/google_email and the explicit "Refresh tokens DO NOT live here" rule)
    - ./CLAUDE.md ("OAuth scope" + "DPAPI via wincred" rules — refresh_token MUST NOT live in config.json)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (decisions D-09 through D-11 for config field meanings)
  </read_first>
  <action>
    Create `internal/logging/logger.go` implementing the `Setup()` signature in <interfaces> above.
    Use the EXACT lumberjack config from 01-RESEARCH.md §10.1:
    ```go
    rotator := &lumberjack.Logger{
        Filename:   filepath.Join(logDir, "squirebot.log"),
        MaxSize:    5,    // megabytes (OPS-03)
        MaxBackups: 3,    // (OPS-03)
        MaxAge:     28,
        Compress:   false,
        LocalTime:  true,
    }
    handler := slog.NewJSONHandler(rotator, &slog.HandlerOptions{
        Level:     slog.LevelInfo,
        AddSource: true,
    })
    return slog.New(handler), logDir
    ```
    `logDir` MUST resolve to `filepath.Join(os.Getenv("LOCALAPPDATA"), "SquireBot")`. Call
    `os.MkdirAll(logDir, 0o755)` before constructing the rotator. Set the returned logger as the
    process default via `slog.SetDefault(logger)` so package-level `slog.Info` calls in other
    packages route through the same handler.

    Create `internal/config/config.go` implementing the `Config` struct + `Load()` + `Save()` +
    `Path()` from <interfaces>. Save MUST be atomic: write to `<path>.tmp`, then `os.Rename` to
    `<path>` (Windows os.Rename is atomic on the same volume). On Load: if `os.IsNotExist(err)`,
    return `&Config{Version: 1, LogLevel: "info"}` (NOT an error). On any other error, return the
    error wrapped with `fmt.Errorf("config load %s: %w", path, err)`.

    The Config struct MUST NOT contain a `RefreshToken` field, a `Token` field, or any field
    semantically holding OAuth credentials. ONLY: Version, EQFolder, SpreadsheetID, GoogleEmail,
    LastKnownInventoryMtime, LogLevel. (D-13 / AUTH-04 / CLAUDE.md rule.) Add a comment on the
    struct: `// SECURITY: NEVER add a refresh_token field. Refresh tokens live in wincred only (AUTH-04).`

    Add tests `internal/config/config_test.go` covering: (a) Load returns zero-value when file
    missing, (b) Save then Load round-trips, (c) marshalled JSON does NOT contain the substring
    `refresh_token`. Use t.TempDir() and a `Path()`-override hook (export a `var pathFn = defaultPath`
    so tests can swap).

    Optionally add `internal/logging/logger_test.go` with a test that calls Setup() and confirms
    the directory is created (use t.Setenv("LOCALAPPDATA", t.TempDir())).
  </action>
  <verify>
    <automated>go test ./internal/logging/... ./internal/config/... -count=1 -timeout 30s</automated>
  </verify>
  <acceptance_criteria>
    - `grep -n "MaxSize:\s*5" internal/logging/logger.go` returns at least one match
    - `grep -n "MaxBackups:\s*3" internal/logging/logger.go` returns at least one match
    - `grep -n "AddSource: true" internal/logging/logger.go` returns at least one match
    - `grep -nv '^\s*//' internal/config/config.go | grep -i "refresh_token"` returns NO matches (refresh_token must not appear in non-comment lines)
    - `grep -n "atomic\|Rename\|\.tmp" internal/config/config.go` returns at least one match (proof of atomic save)
    - `grep -n "LOCALAPPDATA" internal/config/config.go internal/logging/logger.go` returns at least one match in EACH file
    - `go test ./internal/logging/... ./internal/config/... -count=1` exits 0
    - `go vet ./internal/...` exits 0
  </acceptance_criteria>
  <done>
    `internal/logging/logger.go` exposes `Setup()` returning `(*slog.Logger, string)` with
    OPS-03-compliant rotation. `internal/config/config.go` exposes `Config`, `Load`, `Save`, `Path`
    with no refresh_token field anywhere. Tests pass.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Wire main.go entry point with embedded icon and end-to-end smoke build</name>
  <files>cmd/squirebot/main.go, cmd/squirebot/icon.go, assets/icon.ico, README.md, .github/workflows/release.yml</files>
  <read_first>
    - internal/logging/logger.go (just created — confirm Setup signature)
    - internal/config/config.go (just created — confirm Load signature)
    - .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md (§11.1 "Lifecycle" lines 1108-1159 — main.go skeleton with systray.Run; for Plan 01 we leave systray un-wired and just exit cleanly after smoke logging — Plan 07 wires systray)
    - .planning/phases/01-end-to-end-thin-slice/01-CONTEXT.md (D-12 for goreleaser/GitHub Releases stub; D-14 for README content)
    - .planning/research/STACK.md ("Installation" + "goreleaser" sections for the build invocation)
  </read_first>
  <action>
    Create `assets/icon.ico` — for Phase 1 a stand-in is acceptable (Open Question Q6 in RESEARCH.md
    explicitly accepts a placeholder). Generate a minimal valid 16x16 .ico file: either reuse a
    free CC0 systray-style icon committed as binary, or use the Go stdlib `image/draw` + `image/png`
    + the `github.com/Kodeworks/golang-image-ico` library (do NOT add it as a dep — instead use
    a pre-rendered .ico). Simplest path: download or hand-craft a 16x16 .ico (16x16 BMP wrapped in
    .ico header — 766 bytes). If the executor cannot produce one inline, write a 1x1 single-color
    placeholder .ico (66 bytes minimum valid ICO file: 6-byte header + 16-byte ICONDIRENTRY +
    40-byte BITMAPINFOHEADER + 4 bytes pixel + 4 bytes AND-mask). Document in README.md "Phase 5
    will replace assets/icon.ico with final art."

    Create `cmd/squirebot/icon.go`:
    ```go
    package main

    import _ "embed"

    //go:embed ../../assets/icon.ico
    var iconBytes []byte
    ```

    Create `cmd/squirebot/main.go`:
    ```go
    package main

    import (
        "log/slog"
        "os"
        "runtime"

        "github.com/<owner>/squirebot/internal/config"
        "github.com/<owner>/squirebot/internal/logging"
    )

    var Version = "0.1.0-dev"

    func main() {
        log, logDir := logging.Setup()
        slog.SetDefault(log)

        cfg, err := config.Load()
        if err != nil {
            slog.Error("config load failed", "err", err, "path", config.Path())
            os.Exit(1)
        }

        slog.Info("squirebot starting",
            "version", Version,
            "pid", os.Getpid(),
            "go_version", runtime.Version(),
            "log_dir", logDir,
            "config_path", config.Path(),
            "icon_bytes", len(iconBytes),
            "google_email", cfg.GoogleEmail,
            "spreadsheet_id_set", cfg.SpreadsheetID != "",
            "eq_folder_set", cfg.EQFolder != "",
        )
        // Plan 03/04/05/06/07 will wire OAuth, watcher, sheets, picker, wizard, tray.
        // Plan 01 exits cleanly after the smoke log line.
    }
    ```
    Replace `<owner>` with the actual module owner from go.mod Task 1.

    Create `README.md` with the minimum Phase 1 content per D-14 (download link placeholder, OAuth
    flow blurb, EQ folder picker blurb, "tray turned red" placeholder, SmartScreen walkthrough
    placeholder). Phase 5 expands; Phase 1 just establishes the file. ~30-50 lines.

    Create `.github/workflows/release.yml` as a goreleaser stub (D-12). Phase 1 only needs the
    skeleton; Phase 2 fills it in. Acceptable minimum:
    ```yaml
    name: release
    on:
      push:
        tags: ['v*']
    jobs:
      release:
        runs-on: windows-latest
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-go@v5
            with: { go-version: '1.24' }
          - name: Build (Phase 1 stub - no signing, no NSIS - those are Plan 08 / Phase 2)
            run: |
              mkdir dist
              go build -ldflags="-H=windowsgui -s -w -X main.Version=${{ github.ref_name }}" -o dist/squirebot.exe ./cmd/squirebot
          - uses: actions/upload-artifact@v4
            with: { name: squirebot-exe, path: dist/squirebot.exe }
    ```

    Run `GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w" -o dist/squirebot.exe ./cmd/squirebot`
    from a Linux/macOS shell or from Windows directly. Artifact must exist and be a valid PE32+.
  </action>
  <verify>
    <automated>GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w" -o dist/squirebot.exe ./cmd/squirebot &amp;&amp; test -s dist/squirebot.exe &amp;&amp; go vet ./...</automated>
  </verify>
  <acceptance_criteria>
    - `cmd/squirebot/main.go` exists and compiles
    - `cmd/squirebot/icon.go` contains the literal string `//go:embed ../../assets/icon.ico`
    - `assets/icon.ico` exists with `wc -c < assets/icon.ico` returning a value &gt; 60 (minimum valid .ico)
    - `dist/squirebot.exe` exists after the build command
    - `file dist/squirebot.exe` (if `file` is available) reports `PE32+ executable (GUI) x86-64`; if `file` unavailable, `head -c 2 dist/squirebot.exe | xxd` shows `4d 5a` (the MZ DOS header magic)
    - `go vet ./...` exits 0
    - `.github/workflows/release.yml` exists and contains the literal string `goreleaser` OR `go build -ldflags` (Phase 1 stub is allowed to skip the goreleaser action itself)
    - `README.md` exists with at least 30 lines
    - `grep -nE "Version\s*=" cmd/squirebot/main.go` matches a `Version` variable (so `-X main.Version=...` ldflag works)
  </acceptance_criteria>
  <done>
    `dist/squirebot.exe` builds cross-compiles cleanly. main.go logs a structured INFO line on
    launch with version+pid+log_dir. README.md and release.yml stubs land at standard locations.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| process → filesystem (%LOCALAPPDATA%) | Watcher writes config.json + log files; corrupted/world-writable directory could leak data |
| process → embedded resources (icon, future client_id) | Build-time constants are baked in; risk if binary is tampered with |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-01-01 | Information Disclosure | Logger writing OAuth tokens to disk | mitigate | OPS-03 logger uses slog with explicit allow-list of fields; logger_test.go MUST add a future regression test for "log JSON does not contain refresh_token / access_token" — Plan 03 owns the test once tokens exist; Plan 01 establishes the slog handler that gives us this control point |
| T-01-02 | Information Disclosure | config.json on world-readable %LOCALAPPDATA% containing OAuth secrets | mitigate | Hard schema enforcement: Config struct in internal/config/config.go has zero credential fields; acceptance criteria greps for `refresh_token` substring and rejects on match; CLAUDE.md "DPAPI via wincred" rule encoded as a code comment |
| T-01-03 | Tampering | Atomic config save partial-write race | mitigate | Save() writes to `.tmp` then os.Rename — single-volume rename is atomic on Windows NTFS |
| T-01-04 | Denial of Service | Unbounded log growth filling user disk | mitigate | lumberjack.MaxSize=5MB × MaxBackups=3 = 20MB cap; MaxAge=28d secondary cap |
| T-01-05 | Tampering | Compromised binary distribution | accept | Phase 1 ships unsigned per D-13; SmartScreen + GitHub Releases SHA-256 mitigate; Phase 2 adds code signing |
| T-01-06 | Information Disclosure | logDir created with overly permissive mode | accept | os.MkdirAll(logDir, 0o755) is the Go idiom; on Windows MkdirAll mode is advisory and inherits parent ACL — %LOCALAPPDATA% defaults to user-only ACL |
</threat_model>

<verification>
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/...` exits 0
- `dist/squirebot.exe` exists and is a valid PE32+ executable
- Running the .exe on Windows produces `%LOCALAPPDATA%\SquireBot\squirebot.log` containing a JSON line with key `"msg":"squirebot starting"`
- `grep -rE "refresh[_-]?token" --include="*.go" cmd/ internal/ | grep -v "_test\.go" | grep -v "wincred" | grep -v '^[^:]*:\s*//' ` exits with 0 matches (no refresh-token storage outside of tests/wincred/comments)
</verification>

<success_criteria>
The repo skeleton is complete when:
- A new clone + `go mod download && go build ./...` succeeds with no errors
- The smoke binary writes one JSON-formatted INFO line to `%LOCALAPPDATA%\SquireBot\squirebot.log`
- config.Load/config.Save round-trip without storing any OAuth secret
- Lumberjack rotation config is OPS-03-compliant (5MB × 3 files)
- `.github/workflows/release.yml` and `README.md` stubs exist (D-12, D-14)
- assets/icon.ico is embedded and readable from main.go
- No file in `internal/` or `cmd/` writes refresh tokens to disk
</success_criteria>

<output>
After completion, create `.planning/phases/01-end-to-end-thin-slice/01-01-SUMMARY.md` documenting:
- Module path chosen (`github.com/<owner>/squirebot`)
- Locked dependency versions (copy from `go.sum` for the load-bearing libs)
- Logger config (MaxSize, MaxBackups, MaxAge, Compress, LocalTime)
- config.Config field list (and explicit assertion: no refresh_token field)
- Build invocation (the one-liner with ldflags)
- Any deviations from RESEARCH.md and why
</output>
