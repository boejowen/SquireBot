---
phase: 25-linux-watcher
reviewed: 2026-06-06T00:00:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - cmd/squirebot/run_other.go
  - cmd/squirebot/main.go
  - internal/app/runapp.go
  - internal/credstore/store_other.go
  - internal/eqfind/discover.go
  - internal/eqfind/heuristic_other.go
  - internal/eqfind/knownpaths_other.go
  - internal/onboarding/dialog_other.go
  - internal/onboarding/dialog.go
  - internal/update/manifest.go
  - internal/update/check.go
  - internal/config/config.go
  - internal/logging/logger.go
  - internal/tray/tray_other.go
  - packaging/linux/install.sh
  - packaging/linux/squirebot.service
  - .github/workflows/release.yml
findings:
  critical: 2
  warning: 6
  info: 4
  total: 12
status: issues_found
---

# Phase 25: Code Review Report

**Reviewed:** 2026-06-06
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

The Linux watcher port is largely well-constructed: the credstore atomic write enforces 0600 correctly, the SHA-256 download gate is intact, the GOOS-keyed `binaryAsset` selection genuinely prevents a Linux box from swapping a Windows PE, and the no-op tray controller faithfully reproduces the Windows surface. The Windows path is untouched.

However, there are **two BLOCKER-level defects in the install/onboarding control flow** that defeat the headless install story:

1. `squirebot --status` **always exits 0**, but `install.sh` keys its "is this configured?" branch on `--status` exiting non-zero. The result: `install.sh` will report "already configured" and **never run `--setup`** on a fresh, unconfigured box — the watcher installs, starts, finds no guild code, and sits red forever with no prompt.
2. The stdin onboarding reader re-wraps `os.Stdin` in a fresh `bufio.Reader` per prompt, so on **non-interactive/piped stdin** the second prompt (EQ folder) loses any bytes the first reader buffered past the first newline.

Plus several WARNING-level robustness gaps (env-expansion order, depth-cap arithmetic, service ordering vs. self-update exit, `set -o pipefail` absence) and INFO items.

## Critical Issues

### CR-01: `--status` always exits 0, so `install.sh` never runs first-time `--setup`

**File:** `cmd/squirebot/main.go:135-138`, `cmd/squirebot/main.go:237-263`, `packaging/linux/install.sh:74-80`
**Issue:** `install.sh` gates onboarding on the exit code of `--status`:

```sh
if "$BIN_DST" --status >/dev/null 2>&1; then
    echo "    already configured — skipping setup."
else
    "$BIN_DST" --setup
fi
```

The comment at `install.sh:71-72` even asserts *"`squirebot --status` exits non-zero when the guild code / EQ folder are not yet configured."* But the implementation does not honor that contract: `runStatus` only prints and returns, and the dispatcher unconditionally calls `os.Exit(0)`:

```go
if len(os.Args) >= 2 && os.Args[1] == "--status" {
    runStatus(cfg, logDir)
    os.Exit(0)   // <-- always 0, even when guild code == "not configured"
}
```

On a fresh box `--status` succeeds, the installer prints "already configured — skipping setup," `--setup` is never invoked, and the started service has no guild code (red forever). This breaks the primary headless install flow (LNX-05/D-06).

**Fix:** Make `runStatus` report configured-ness via the process exit code, e.g. return a bool and exit 1 when unconfigured:

```go
if len(os.Args) >= 2 && os.Args[1] == "--status" {
    if runStatus(cfg, logDir) { // true == fully configured
        os.Exit(0)
    }
    os.Exit(1)
}
```

```go
// runStatus returns true iff both a guild code AND an EQ folder are configured.
func runStatus(cfg *config.Config, logDir string) bool {
    code, err := credstore.Read()
    configured := err == nil && code != "" && (cfg.EQFolder != "" || len(cfg.EQFolders) > 0)
    // ...existing prints...
    return configured
}
```
(Or, if `--status` must stay exit-0 for other callers, change `install.sh` to grep the output instead — but the in-code contract comment says exit code, so fix the code.)

### CR-02: onboarding stdin reader drops buffered input between prompts on non-interactive stdin

**File:** `internal/onboarding/dialog_other.go:33-40`
**Issue:** `readLine` constructs a brand-new `bufio.NewReader(stdin)` on every call:

```go
func readLine() (line string, eofEmpty bool) {
    r := bufio.NewReader(stdin)
    s, err := r.ReadString('\n')
    ...
}
```

`bufio.Reader` reads in chunks from the underlying `io.Reader`. When stdin is a pipe/file/heredoc (e.g. `printf 'CODE\n/path\n' | squirebot --setup`), the first `PromptGuildCode → readLine` call can buffer bytes *past* the first `\n` (the EQ-folder line). That buffered tail is discarded when `PickEQFolder` allocates a **second** `bufio.Reader`, so the second prompt reads EOF/garbage and returns `ErrCancelled`. Interactive TTY input happens to survive (the OS delivers one line at a time), which is why the unit tests — each of which resets `stdin` to a fresh single-line reader — don't catch it. The `install.sh --setup` path is interactive, but any scripted/piped onboarding (CI, docs example, `ssh host squirebot --setup <<EOF`) silently fails the folder step.

**Fix:** Hold one package-level `*bufio.Reader` over the shared `stdin` for the lifetime of the prompts instead of re-wrapping per call:

```go
var stdin io.Reader = os.Stdin
var reader *bufio.Reader

func stdinReader() *bufio.Reader {
    if reader == nil {
        reader = bufio.NewReader(stdin)
    }
    return reader
}

func readLine() (line string, eofEmpty bool) {
    s, err := stdinReader().ReadString('\n')
    if err != nil && s == "" {
        return "", true
    }
    return s, false
}
```
(Tests that swap `stdin` must reset `reader = nil`; expose a small test helper or reset in the test setup.)

## Warnings

### WR-01: `expandPath` runs `os.ExpandEnv` AFTER tilde join, double-expanding and mishandling unset `$HOME`

**File:** `internal/onboarding/dialog_other.go:77-88`
**Issue:** Two problems in one function:
1. For input `"~"` with `$HOME` **unset**, the `if p == "~"` branch falls through without returning, then `os.ExpandEnv("~")` returns `"~"` verbatim — a literal `~` path that will fail `ValidateFolder` with a confusing message. Minor, but the "no panic on unset env" goal is met only by accident.
2. After the `~/` branch rewrites `p` to `filepath.Join(home, ...)`, the function *still* calls `os.ExpandEnv(p)` on the result. If the expanded `$HOME` itself contained a literal `$` (pathological but possible), or if the original path mixed `~/` with `$VAR`, the second pass re-expands. More importantly, the ordering means a path like `~/$FOO` joins home first then expands `$FOO` — generally fine, but the double-pass is surprising and undocumented.

**Fix:** Expand env first, then resolve a leading tilde once, and return early on the bare-`~` unset-HOME case:

```go
func expandPath(p string) string {
    p = os.ExpandEnv(p)
    home := os.Getenv("HOME")
    switch {
    case p == "~" && home != "":
        return home
    case strings.HasPrefix(p, "~/") && home != "":
        return filepath.Join(home, p[2:])
    }
    return p
}
```

### WR-02: heuristic depth cap is `curDepth > max` — admits one extra level vs. the documented "depth 5"

**File:** `internal/eqfind/heuristic_other.go:146-149`
**Issue:** `curDepth := count(path) - rootDepth` then `if curDepth > maxHeuristicDepthOther { SkipDir }`. With `max=5`, a directory at relative depth 6 is *visited* (only its children are pruned). The doc/comment says "depth cap (5)"; the code permits depth 6 entries to be `ValidateFolder`-checked. Not a security hole (timeout still bounds it), but it contradicts the stated bound and the Windows-parity claim. Off-by-one.

**Fix:** Use `>=` if 5 is meant to be the deepest level checked, or update the comment to "depth ≤ 6." Confirm parity with `heuristic_windows.go`'s comparator and match it exactly.

### WR-03: systemd unit self-update restart can hit `start-limit` and stop restarting

**File:** `packaging/linux/squirebot.service:19-30`
**Issue:** `Restart=always` + `RestartSec=5` with no `StartLimitIntervalSec`/`StartLimitBurst` override. The selfupdate path (`main.go:84-91`) does `os.Exit(0)` after swapping, relying on `Restart=always` to relaunch. systemd's default start-limit is 5 starts / 10s; a clean `exit 0` counts as a start for rate-limiting. A rapid swap+exit, or any early-exit crash loop (e.g. a bad config repeatedly failing `config.Load` → `os.Exit(1)`), will trip the limit and systemd will refuse to restart (`start-limit-hit`) — the watcher stays dead until manual `systemctl --user reset-failed`. The comment claims "The self-update swap relaunches via Restart=always" but doesn't account for the limiter.

**Fix:** Add an explicit, lenient limit and ensure a clean post-swap exit is treated as success:

```ini
[Unit]
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Restart=always
RestartSec=5
# exit 0 (post-self-update swap) is a clean restart trigger:
SuccessExitStatus=0
```

### WR-04: `install.sh` uses `set -eu` (no `pipefail`) and never verifies `~/.local/bin` is on `PATH`

**File:** `packaging/linux/install.sh:20`, `:75`, `:93-100`
**Issue:** Two robustness gaps the phase scope explicitly calls out:
1. `set -eu` under `#!/bin/sh` — `pipefail` isn't POSIX, so it's correctly omitted, but there are no pipelines whose failure would be masked *except* the install itself. Acceptable, but the closing-banner commands (`systemctl --user status squirebot`, `squirebot --status` at lines 95) tell the user to run `squirebot ...` bare.
2. There is **no check** that `$HOME/.local/bin` is on `PATH`. On a minimal/SSH box `~/.local/bin` is frequently absent from `PATH`, so the user-facing `squirebot --status` instruction fails with "command not found." The service itself is fine (absolute `ExecStart`), but the onboarding `"$BIN_DST" --setup` line and the printed guidance assume reachability.

**Fix:** After install, warn if not on PATH:

```sh
case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;
  *) echo "warning: $HOME/.local/bin is not on your PATH;" >&2
     echo "         add it (e.g. in ~/.profile) to run 'squirebot --status' directly." >&2 ;;
esac
```

### WR-05: `--setup` ignores SIGINT/SIGTERM — no signal handler on the setup context

**File:** `cmd/squirebot/main.go:139-162`
**Issue:** The `--setup` branch builds `setupCtx` from `context.Background()` with only a deferred `setupCancel()`, and never installs the SIGINT/SIGTERM → cancel handler that `run_other.go` installs for the watcher path. During interactive onboarding the prompts block on `bufio.ReadString` (an uninterruptible blocking read), so Ctrl-C during `--setup` kills the process abruptly rather than cancelling the context cleanly. Minor for an interactive tool, but the blocking `ReadString` also means the `setupCtx` cancellation is never observed by the prompt layer (`PickEQFolder`/`PromptGuildCode` take no ctx). If a backend `Validate` call hangs, there's no way to abort except SIGKILL.

**Fix:** Either install `signal.NotifyContext` for the `--setup` path too, or document that `--setup` is intentionally non-cancellable. At minimum, thread `setupCtx` into `bc.Validate` (already done in `runOnboarding`) so a hung network validation honors Ctrl-C; the stdin read itself is inherently blocking and is acceptable.

### WR-06: `update.Apply()` failure on Linux is swallowed and may loop under `Restart=always`

**File:** `cmd/squirebot/main.go:84-91`
**Issue:** On a Linux box, if a `.new` + sidecar pair exists but `update.Apply()` returns an error (e.g. a partially-written staged binary, or a swap failure), the code logs to stderr and *continues* into the normal watcher run. The staged files are not cleaned up here, so on every restart `update.Apply()` is retried and fails again. Combined with `Restart=always`, a corrupt staged update produces an error on every launch with no self-healing. (The Windows path has the same shape but a human sees the tray; headless Linux only emits a journal line.)

**Fix:** On `update.Apply()` error in the headless path, log via slog (after `logging.Setup`, not just stderr) and consider removing the staged `.new`/`.expected-sha256` pair so a known-bad stage doesn't retry indefinitely. Verify `swap.go` already self-heals; if it does, this is downgradeable to INFO.

## Info

### IN-01: `--uninstall-wipe-credentials` stderr text says "wincred" on Linux

**File:** `cmd/squirebot/main.go:43,46`
**Issue:** The messages "wincred delete (guild code) failed or absent" and "wincred guild-code entry removed" are emitted on all platforms, but on Linux the store is a 0600 file, not wincred. Cosmetic, but misleading in journald.
**Fix:** Use platform-neutral wording ("credential store") or build-tag the message.

### IN-02: `parseVersion` pre-release tail uses lexical compare — `rc10 < rc2`

**File:** `internal/update/manifest.go:207-211`
**Issue:** `strings.Compare(mPre, cPre)` orders `"rc10"` *before* `"rc2"` lexically. The doc acknowledges this is "sufficient for our rcN/betaN scheme," and only finals ship, so it's a dev-only rail — but it is a latent correctness gap if ≥10 RCs of one core are ever published.
**Fix:** Acceptable as-is given the documented constraint; note for future. No change required unless RC counts can reach double digits.

### IN-03: `defaultHeuristicScan` gates on `GOOS == "linux"` but `heuristic_other.go` is `!windows`

**File:** `internal/eqfind/discover.go:91-96`, `internal/eqfind/heuristic_other.go:1`
**Issue:** `heuristic_other.go` compiles on darwin/bsd too (`//go:build !windows`), but `defaultHeuristicScan` only calls `heuristicScan()` when `GOOS == "linux"` (or windows). On macOS the WINE-walk code is compiled-in but dead. Harmless (darwin is explicitly out of scope this phase), just slightly inconsistent — the build tag is broader than the runtime gate.
**Fix:** None required; optionally narrow the build tag to `//go:build linux` to match the runtime gate, or widen the gate to `!= "windows" && GOOS != "darwin"`.

### IN-04: manifest `phase: 2` field is stale/confusing for a Phase-25 release

**File:** `.github/workflows/release.yml:278`
**Issue:** `phase = 2` is hardcoded in `latest.json`. The comment says it's informational and the updater ignores it, which `manifest.go` confirms (no `phase` field). Purely a documentation smell in the emitted manifest.
**Fix:** Drop the field or set it meaningfully; non-blocking.

---

## Confirmed-correct (spot-checks that passed)

- **credstore atomic 0600 write** (`store_other.go:45-58`): tmp+rename in the same dir, `os.WriteFile(tmp, …, 0600)` on a fresh file guarantees mode even over a looser pre-existing file; `Read`/`Delete` surface ENOENT cleanly; no logger import (token never logged). Correct.
- **`binaryAsset` GOOS selection** (`manifest.go:88-93`) + **empty-asset skip** (`check.go:111-115`): a Linux box reading a legacy Windows-only manifest gets `("","")` → no-op skip, never downloads/swaps a PE. SHA-256 verify (`check.go:167-171`) gates every swap. Back-compat intact. Correct.
- **`runStatus` token redaction** (`main.go:237-256`): reads credstore only to print "configured"/"not configured"; the code is never printed. Correct (the exit-code bug CR-01 is separate).
- **SIGINT/SIGTERM handler** (`run_other.go:31-47`): `signal.NotifyContext` + goroutine driving root `cancel()` correctly unwinds `RunApp` on `systemctl --user stop`. Correct.
- **No-op tray controller** (`tray_other.go`): full exported-surface parity, `SetStatus`/`SetIconHealth` are plain slog calls (goroutine-safe, no nil deref). Correct.
- **release.yml Linux build**: `CGO_ENABLED=0`, no `-H=windowsgui`, `-trimpath`, same Version/BackendBaseURL ldflags, tarball hashes the bare ELF (not the tarball) for `binary_sha256_linux`, Windows steps unchanged. Correct.
- **No-symlink walk** (`heuristic_other.go:126-162`): `filepath.WalkDir` does not follow symlinks; permission-denied subtrees pruned via `SkipDir`; unset `$HOME`/`$WINEPREFIX` handled without panic. Correct (aside from the WR-02 off-by-one).

---

_Reviewed: 2026-06-06_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
