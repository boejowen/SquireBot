---
phase: 25-linux-watcher
fixed_at: 2026-06-06T00:00:00Z
review_path: .planning/phases/25-linux-watcher/25-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Phase 25: Code Review Fix Report

**Fixed at:** 2026-06-06
**Source review:** `.planning/phases/25-linux-watcher/25-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 8 (CR-01, CR-02, WR-01..WR-06)
- Fixed: 8
- Skipped: 0
- IN-01..IN-04 were out of scope (info-only) and intentionally untouched.

All work is additive and behind `//go:build` tags / cross-platform stdlib:
the Windows build, `go test ./...`, the CGO-free Linux closure (0 systray),
and the browser-free constraint are all preserved.

## Fixed Issues

### CR-01: `--status` always exits 0, so `install.sh` never runs first-time `--setup`

**Files modified:** `cmd/squirebot/main.go`, `cmd/squirebot/status_test.go`
**Commit:** `bebe961`
**Applied fix:** `runStatus` now returns `true` IFF a guild code AND at least one
EQ folder are configured; the `--status` dispatcher maps that bool to the exit
code (`0` configured / `1` not), honoring the contract `install.sh` keys its
`--setup` branch on. Extracted a pure `statusConfigured(codeConfigured, eqFolderCount)`
predicate and added a table-driven unit test pinning the exit-code contract
(code+folder → configured; any missing → unconfigured). The guild code is still
never printed (V7). Host `go test ./cmd/squirebot` passes.

### CR-02: onboarding stdin reader drops buffered input between prompts on piped stdin

**Files modified:** `internal/onboarding/dialog_other.go`, `internal/onboarding/dialog_other_test.go`
**Commit:** `b12f013`
**Applied fix:** Replaced the per-prompt `bufio.NewReader(stdin)` with ONE
persistent package-level `*bufio.Reader` (`stdinReader()`), so bytes the first
`ReadString` buffers past its newline (the EQ-folder line on piped stdin) survive
into the second prompt. The `withStdin` test helper now resets `reader = nil` when
it swaps the `stdin` seam. Added `TestPromptsSharePersistentReader`, which drives
both `PromptGuildCode` and `PickEQFolder` from a single
`"MYCODE-123\n~/.wine/...\n"` reader (the scripted/piped case) and asserts both
read correctly. (`!windows`-tagged; test binary cross-compiles clean for Linux.)

### WR-01: `expandPath` env-expansion order / bare `~` with unset `$HOME`

**Files modified:** `internal/onboarding/dialog_other.go`, `internal/onboarding/dialog_other_test.go`
**Commit:** `b12f013` (cohesive with CR-02 — same file)
**Applied fix:** `expandPath` now runs `os.ExpandEnv` once FIRST, then resolves a
leading `~`/`~/` EXACTLY once via `os.UserHomeDir()` (no second ExpandEnv pass on
the joined result). When home cannot be resolved, the path is left verbatim — a
bare `~` no longer silently leaks a literal `~` through a re-expansion; the
caller's `ValidateFolder` rejects it with a clear error. No shell/command
execution. Added `TestExpandPath_BareTildeUnsetHome`. Existing tilde/env tests
still pass (cross-compiled).

### WR-02: heuristic depth-cap off-by-one vs. documented "depth 5"

**Files modified:** `internal/eqfind/heuristic_other.go`, `internal/eqfind/heuristic_other_test.go`
**Commit:** `0ac0bc0`
**Applied fix:** Verified the depth check (`curDepth > maxHeuristicDepthOther`)
runs BEFORE `ValidateFolder`, so depth-6 dirs are pruned and never validated —
depth 5 is the deepest level checked, which already matches both the comment and
`heuristic_windows.go`'s comparator EXACTLY. Kept `>` (switching to `>=` would
stop discovering legitimate depth-5 installs and break Windows parity — the
Windows suite explicitly guards this). Clarified the ambiguous `// depth cap (5)`
comment to state the contract, and added the boundary tests the Linux side was
missing: `TestWalkWineRoot_AtDepthCapFound` (depth 5 found) and
`TestWalkWineRoot_BeyondDepthCapNotFound` (depth 6 pruned), mirroring
`heuristic_windows_test.go`.

### WR-03: systemd self-update restart can trip the start-limiter

**Files modified:** `packaging/linux/squirebot.service`
**Commit:** `f7f8736`
**Applied fix:** Added `StartLimitIntervalSec=0` (start-rate limiter OFF) so a
clean self-update `exit(0)` + `Restart=always` relaunch — or a brief transient
crash loop — never trips systemd's default 5-starts/10s limit and strands the
unit in `failed` (requiring manual `reset-failed`). Added `SuccessExitStatus=0`
to make the post-swap exit-0-is-success intent explicit; `RestartSec=5` provides
backoff. Documented the rationale in a comment.

### WR-04: `install.sh` never verifies `~/.local/bin` is on `$PATH`

**Files modified:** `packaging/linux/install.sh`
**Commit:** `bd197d7`
**Applied fix:** After install, the script now checks whether `$HOME/.local/bin`
is on `$PATH`; if not, it prints a non-fatal warning with the full-path
invocation (`$BIN_DST --status`) and a `~/.profile` remediation, instead of
telling the user to run `squirebot` bare (which would dead-end in "command not
found" on minimal/SSH boxes). The service itself is unaffected (absolute
`ExecStart`). `set -eu`, idempotency, and quoting preserved; `sh -n` clean.

### WR-05: `--setup` ignores SIGINT/SIGTERM

**Files modified:** `cmd/squirebot/main.go`
**Commit:** `c47efdc`
**Applied fix:** The `--setup` branch now derives `setupCtx` via
`signal.NotifyContext(context.Background(), SIGINT, SIGTERM)` (mirroring
`run_other.go`'s `runMainLoop`). Ctrl-C now cancels `setupCtx`, which is already
threaded into `app.RunSetup` → `bc.Validate`, so a hung network validation aborts
cleanly instead of waiting for SIGKILL. The blocking stdin reads remain inherently
uninterruptible (acceptable for an interactive tool). `syscall.SIGINT/SIGTERM`
exist on Windows too, so the Windows build is unaffected.

### WR-06: corrupt staged update can brick every headless launch

**Files modified:** `cmd/squirebot/main.go`, `internal/update/swap.go`, `internal/update/swap_test.go`
**Commit:** `81b4b28`
**Applied fix:** Added `update.RemoveStaged()` (+ testable `removeStagedAt`) which
deletes the `<exe>.new` + `<exe>.expected-sha256` pair idempotently. `main.go`'s
headless launcher now calls it when `update.Apply()` returns an error, so a
SHA-256-verified-but-unswappable stage (swap.go State 5 preserves it for retry)
is discarded rather than retried on every `Restart=always` launch. The next
process runs the current binary; the next daily check re-downloads + re-verifies
+ re-stages — the SHA-256 gate is preserved end-to-end (a bad stage is never
applied, only discarded and re-fetched). The failure is re-logged via `slog`
once `logging.Setup` has run (Apply runs before logging, so only stderr is
available at that point). `swap.go`'s State-5 preserve-for-retry behavior is
unchanged (the discard policy lives in the launcher, leaving Test 5 valid). Added
`removeStagedAt` delete-both / no-op-when-empty tests.

## Skipped Issues

None — all 8 in-scope findings were fixed.

## Final Gate Results

All run from the repo root on the Windows host (cross-compiling for the targets):

| Gate | Command | Result |
|------|---------|--------|
| Host unit tests | `go test ./...` | PASS (all packages ok) |
| Linux static build | `GOOS=linux CGO_ENABLED=0 go build ./cmd/squirebot` | exit 0 |
| 0 systray in Linux closure | `GOOS=linux CGO_ENABLED=0 go list -deps ./cmd/squirebot \| grep -c systray` | `0` |
| Linux vet | `GOOS=linux go vet ./...` | exit 0 (no findings) |
| Linux build-all | `GOOS=linux go build ./...` | exit 0 |
| Windows build | `GOOS=windows go build ./cmd/squirebot` | exit 0 |
| install.sh syntax | `sh -n packaging/linux/install.sh` | exit 0 |

New/updated tests: `cmd/squirebot/status_test.go` (CR-01), the two persistent-reader
+ bare-tilde tests in `internal/onboarding/dialog_other_test.go` (CR-02/WR-01),
the two depth-boundary tests in `internal/eqfind/heuristic_other_test.go` (WR-02),
and the two `removeStagedAt` tests in `internal/update/swap_test.go` (WR-06). The
`!windows`-tagged tests cross-compile clean for Linux (`GOOS=linux go test -c`)
and run on Linux CI; the `cmd/squirebot` and `internal/update` tests run on the
host and pass.

---

_Fixed: 2026-06-06_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
