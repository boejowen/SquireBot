---
quick_id: 260621-td4
slug: linux-watcher-polish-surface-cross-owner
date: 2026-06-22
status: complete
files_changed: [internal/app/runapp.go, cmd/squirebot/main.go]
commits: [20a7d45, c2e3926]
---

# Quick Task 260621-td4 Summary

Watcher polish from a friend's (Kim's) Linux debug report. Four reported items: two
were real and are now fixed (one commit each), two were verified non-issues and got
no code/doc change. Watcher is Go; no schema change; **no `v*` tag pushed** (a tag
would fire the watcher release CI — these land in a future release decided separately).

## Task 1 — Surface cross-owner ingest rejection on the tray (item #1) — `20a7d45`

`internal/app/runapp.go` `handleIngestErr` handled a 409 cross-owner reject with only
`slog.Warn` + `return true`. Unlike the `ErrUnauthorized` (401) branch right above it,
it never touched the tray — so the icon stayed green/"Connected" while every upload was
silently rejected (exactly what made Kim's failure invisible).

**Change:** the `case errors.Is(err, backend.ErrCrossOwner):` branch now mirrors the 401
branch's pattern after its existing `slog.Warn`:

```go
t.SetIconHealth(tray.HealthRed)
t.SetStatus("Rejected: " + charName + traySuffix + " is registered to another guildie")
return true // terminal; NO retry (unchanged)
```

`return true` (terminal, no-retry) is unchanged — only the tray surfacing was added.
Uses the `charName`/`traySuffix` already in scope.

**Out of scope (noted, not implemented):** the report also mused about systemd-journal
visibility on Linux. That is a logging-sink change (the Linux watcher logs to a
lumberjack file, not the journal) — a separate, larger decision. The tray surfacing is
the targeted fix and matches the existing 401 pattern. Deferred follow-up.

**Test note:** I did NOT add a tray-state assertion. Neither the Windows nor the headless
`tray.Controller` exposes any read path for the current status/health — `SetStatus`/
`SetIconHealth` are write-only (headless logs via slog; Windows mutates a label/icon),
and there is no exported accessor. The PLAN explicitly forbids inventing test-only
exports, so I relied on the existing `TestMakeOnSpellbookChange_409CrossOwnerNoPersist`
(which asserts one-request + no-mtime-persist and is unaffected by the tray change — it
does not read tray state). The new behavior parallels the well-tested 401 branch.

## Task 2 — `--reconfigure` flag to force re-onboarding (item #2) — `c2e3926`

`squirebot --setup` (`app.RunSetup`) was a no-op once a guild code AND an EQ folder were
configured — it printed "setup complete" and exited without re-prompting, so a guildie
could not re-point the folder or re-enter a code without manually moving `config.json`
aside.

**Changes:**
- `internal/app/runapp.go` `RunSetup` gained a trailing `force bool` parameter. When
  `force` is true it always re-runs `runOnboarding` (re-prompt + re-validate + re-store
  the code) AND re-picks the EQ folder via `pickAndSaveEQFolder` (because `runOnboarding`
  only prompts for the folder when none is configured). Doc comment updated to document
  the new behavior. (D-07 constrains `RunApp` to stay byte-unchanged; `RunSetup` is the
  separate function precisely so it can change — `RunApp` was not touched.)
- `cmd/squirebot/main.go` (the only caller) now accepts `--reconfigure` as a sibling of
  `--setup`, sets `force := os.Args[1] == "--reconfigure"`, skips the `eqfind.Discover()`
  EQ-folder auto-discovery on a forced run (so the user re-points explicitly rather than
  having a stale auto-discovery mask the intent), threads `force` into `app.RunSetup`,
  and prints a distinct completion line (`SquireBot reconfigured.` vs `SquireBot setup
  complete.` — picked with a real `if/else`, since Go has no ternary). The Linux-subcommand
  lead comment was updated to mention `--reconfigure`. Plain `--setup` behavior is unchanged.

**Test gap (documented, not filled):** a `RunSetup(force=true)` unit test that drives the
prompts is NOT cleanly achievable on this Windows dev box. `onboarding.PromptGuildCode`/
`PickEQFolder` have an injectable `stdin` seam only on the `!windows` build
(`dialog_other.go`); the Windows dev box compiles `dialog_windows.go`, which pops a modal
Win32 `DialogBoxIndirectParamW` dialog (and `sqweek/dialog` for the folder picker) with no
headless seam. Adding a cross-platform shim/interface to drive the prompts in a test would
be new test infrastructure, which the PLAN explicitly forbids. Per the PLAN I relied on
`go build ./...` (confirms the single caller + the new signature line up) plus the existing
`internal/onboarding` tests. The force-branch logic is small and exercised via `go build`.

## Investigated — not a bug (items #3 and #4)

No code, doc, or config change was made for these. Evidence captured here so a future
reader doesn't re-investigate.

### #3 — config `"version":1` "migration gap" — NOT a bug

`internal/config/config.go:24` defines `Version` as the config-**file schema** version,
fixed at `=1` (`Load` defaults a `0` → `1`). The app version "2.1.1" is a different number
entirely — it is not stored in `config.json` and they are unrelated. The
`last_known_*_mtime` maps being `null` in Kim's config is expected: those maps are populated
only after an upload **succeeds**, and for the reporter every upload was cross-owner
rejected (item #1), so they were never written. Expected behavior, not a missing migration.

### #4 — install-doc UAC / Google contradictions — NOT real

`docs/install.md:17` says unambiguously "you will **not** see a UAC prompt." The only other
UAC mention lives in the developer runbook `docs/build-and-install.md` (a build/test
checklist), which is a different audience, not a contradiction. The Google/Discord lines in
the install doc describe three distinct true things: the watcher uses no Google; the website
uses Discord login; an upgrade from v1 clears the old v1 Google token. No edit warranted.

## Verification

- `go build ./...` — clean (after both tasks).
- `go test ./internal/app/... ./cmd/squirebot/... ./internal/onboarding/...` — all pass
  (`internal/app` ok, `cmd/squirebot` ok, `internal/onboarding` ok). The existing
  `TestMakeOnSpellbookChange_409CrossOwnerNoPersist` still passes.
- Both commits verified to contain no file deletions (`git diff --diff-filter=D`).
- Scope honored: only `internal/app/runapp.go` + `cmd/squirebot/main.go` changed across the
  two task commits; no `.planning/` files committed by the executor; no `v*` tag pushed.

## Self-Check: PASSED

- `internal/app/runapp.go` — FOUND (modified; ErrCrossOwner branch sets HealthRed + status;
  `RunSetup` has the `force bool` param).
- `cmd/squirebot/main.go` — FOUND (modified; `--reconfigure` sibling dispatch + `force`
  threaded through `app.RunSetup`).
- Commit `20a7d45` — FOUND (Task 1, `fix(watcher): surface cross-owner ingest rejection ...`).
- Commit `c2e3926` — FOUND (Task 2, `feat(watcher): add --reconfigure ...`).
