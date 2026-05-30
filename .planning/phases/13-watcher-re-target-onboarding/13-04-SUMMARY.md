---
phase: 13-watcher-re-target-onboarding
plan: 04
subsystem: infra
tags: [go, gofmt, semver, version-compare, auto-update, win32, binary-size, sc-2]

# Dependency graph
requires:
  - phase: 13-watcher-re-target-onboarding
    provides: "13-03 deleted the ~8k-LOC Google stack + go mod tidy (the binary-size/no-secret confirmation requires that deletion to have landed)"
  - phase: 13-watcher-re-target-onboarding
    provides: "13-01 ingest.IsOlder — the server-side SemVer-with-pre-release compare whose DIRECTION this plan's watcher-side IsNewer mirrors (deliberate separate copy)"
provides:
  - "SemVer-pre-release-aware update.IsNewer/parseVersion (999.22) — the watcher-side version-compare truth: a pre-release ranks below its final; corrupt/unparseable version yields false (never update on doubt)"
  - "gofmt-clean cmd/squirebot/console_windows.go with a freeConsole() doc that matches its impl (999.20 + 999.21): ret==0 logs Debug + returns nil (no spurious detach Warn on GUI launches)"
  - "SC-2 evidence: the re-targeted watcher binary is 57% smaller (16.44 MB → 7.07 MB) and carries zero Google OAuth/Sheets/secret string"
affects: [16-cutover-decommission]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "One version-compare doctrine PER SIDE: update.IsNewer (watcher) + ingest.IsOlder (server) are deliberate separate copies, behaviorally consistent in direction, no client<->server internal import"
    - "Watcher-side fail-CLOSED-on-the-UPDATE-decision (never update on a parse-failure of EITHER version) — the inversion of the server gate's fail-closed-on-the-GATE"
    - "Binary-shed verification: build pre-deletion (worktree) vs post-deletion with identical ldflags + byte-accurate ISO-8859-1 substring scan for linked-package string residue"

key-files:
  created: []
  modified:
    - cmd/squirebot/console_windows.go
    - internal/update/manifest.go
    - internal/update/manifest_test.go

key-decisions:
  - "freeConsole() returns nil unconditionally: ret==0 is the benign no-console (GUI/Explorer) case, logged at Debug not Warn — and a genuine detach failure is non-fatal anyway, so it never surfaces as an error return (doc reconciled to impl, 999.21)"
  - "update.IsNewer mirrors ingest.IsOlder's DIRECTION but is a separate copy (no import of internal/backendsrv into the watcher tree — grep-confirmed the only 'backendsrv' token in internal/update is a doc-comment doctrine note)"
  - "pre-release tail comparison is a lexical strings.Compare (sufficient for our only-ever rcN/betaN scheme; the full SemVer §11 dot-identifier rule is overkill for a dev-only safety rail)"
  - "v1 baseline measured by building the pre-deletion commit d12fbaf in a detached worktree with the SAME ldflags (-H=windowsgui -s -w) — apples-to-apples, not the documented-figure-only estimate (came out 16.44 MB, matching the ~17 MB note)"

patterns-established:
  - "Defensive parse contract preserved across the rework: parseVersion still returns ok=false on non-3-numeric core; IsNewer returns false on either input failing — a corrupt manifest can never trigger an update (T-13.04-01)"
  - "TDD gate on a behaviour change to an existing function: RED commit asserts the new pre-release ranking fails against the strict 3-part compare, GREEN commit lands the SemVer-aware impl"

requirements-completed: [WATCH-09, WATCH-11]

# Metrics
duration: 8min
completed: 2026-05-30
---

# Phase 13 Plan 04: Polish + Ship-Prep Summary

**Landed the three ride-along watcher nits (999.20 console gofmt, 999.21 freeConsole doc/impl reconciliation, 999.22 SemVer-pre-release-aware auto-update compare) and confirmed the SC-2 outcome: the re-targeted binary is 57% smaller (16.44 MB → 7.07 MB) and carries zero Google OAuth/Sheets/secret string.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-05-30T06:34:46Z
- **Completed:** 2026-05-30T06:42:13Z
- **Tasks:** 3 (Task 2 TDD RED→GREEN; Task 3 is a read-only confirmation — no production code change)
- **Files modified:** 3 (0 created)

## Accomplishments

- **999.20 + 999.21 (`cmd/squirebot/console_windows.go`):** `gofmt -w` re-aligned the `var (kernel32 … procFreeConsole …)` block (the extra space before `=` is gone — `gofmt -l` now prints nothing). `freeConsole()` no longer logs `slog.Warn` on `ret==0`: it logs at `Debug` and returns `nil`, killing the spurious detach Warn on every GUI/Explorer launch. The doc comment is reconciled to the impl ("Returns nil unconditionally … a no-console launch is the common benign case … logged at Debug") and the stale "log at Warn level on any failure" sentence is removed. The now-unused `"fmt"` import was dropped.
- **999.22 (`internal/update/manifest.go`):** `IsNewer`/`parseVersion` are now SemVer-pre-release-aware. `parseVersion` returns `(core [3]int, pre string, ok bool)` — strips a leading `v`, splits the pre-release tail on the FIRST `-`, requires exactly 3 numeric core parts (preserving the defensive `ok=false`). `IsNewer` compares MAJOR.MINOR.PATCH numerically, then applies SemVer §11 pre-release precedence on a core tie: a pre-release ranks BELOW its final (an rc updates to the final; a final never downgrades to an rc; `rcN` vs `rcM` is a lexical tail compare). The defensive contract is preserved — a corrupt manifest OR an unparseable current version yields `false` (never update on doubt). The file documents the "one version-compare doctrine, per side" rule (this is the deliberate separate-copy twin of `ingest.IsOlder`).
- **SC-2 confirmation (read-only):** the re-targeted watcher binary is **materially smaller and carries no Google secret** — measured, not asserted (evidence table below).
- **Whole-repo gate green:** `go build ./...` + `go vet ./...` + `go test ./...` all pass (22 packages, 0 failures — no v1/backend regression from the nits); `gofmt -l` clean on all three touched files.

## Task Commits

Each task was committed atomically:

1. **Task 1: 999.20 gofmt + 999.21 freeConsole doc/impl** — `c930fc2` (fix)
2. **Task 2: 999.22 SemVer-pre-release-aware IsNewer** — `e758fb0` (test RED) → `3e8e53b` (feat GREEN)
3. **Task 3: SC-2 binary-smaller / no-secret confirmation** — no commit (read-only confirmation; evidence recorded here, no production code change)

**Plan metadata:** committed separately with this SUMMARY + STATE + ROADMAP.

**TDD gate compliance (Task 2):** a `test(...)` RED commit (`e758fb0`) precedes the `feat(...)` GREEN commit (`3e8e53b`). The RED run failed exactly on the new pre-release cases (`IsNewer("2.0.0-rc1","2.0.0") = false, want true`; `rc1→rc2`; higher-core-prerelease), proving the test exercises the gap — it did not pass unexpectedly. No REFACTOR commit was needed (the impl mirrors the proven `ingest.IsOlder` structure and was clean as written).

## Files Created/Modified

- `cmd/squirebot/console_windows.go` (+22/−21) — gofmt-aligned var block (999.20); `freeConsole()` ret==0 → `slog.Debug` + `return nil` (999.21); doc reconciled to "returns nil unconditionally / Debug"; dropped `"fmt"` import.
- `internal/update/manifest.go` (+93/−22) — `parseVersion` now returns `(core, pre, ok)`; `IsNewer` is SemVer-pre-release-aware with the defensive false-on-either-parse-failure preserved; package doc updated from "3-part" to "SemVer-pre-release-aware"; one-truth-per-side doctrine documented.
- `internal/update/manifest_test.go` (+69) — `TestIsNewer_SemVerPreRelease` (11 cases: core compares + rc-below-final + final-no-downgrade + rcN/rcM lexical + higher-core-prerelease-beats-lower-final) + `TestIsNewer_DefensiveCorruptVersion` (8 cases: corrupt manifest AND corrupt current both → false). All pre-existing `IsNewer`/`Fetch`/`Manifest` tests kept passing. (Also incidentally restored this file to `gofmt -l`-clean — it failed gofmt before the rewrite.)

## SC-2 Evidence (the verifier reads this)

**Binary size — materially smaller (measured apples-to-apples with identical ldflags `-H=windowsgui -s -w -X main.Version=2.0.0-dev`):**

| Binary | Commit | Bytes | Size |
|--------|--------|-------|------|
| v1 baseline (pre-deletion, full Google stack) | `d12fbaf` (13-02 complete; built in a detached worktree) | 17,242,112 | 16.44 MB |
| v2 re-targeted (post-deletion) | `3e8e53b` (this plan's HEAD) | 7,408,640 | 7.07 MB |
| **Delta** | | **9,833,472** | **9.38 MB smaller (57%)** |

The documented "~17 MB v1" baseline from `release.yml` is corroborated by the measured 16.44 MB pre-deletion build. The 9.38 MB / 57% drop is the shed Google dependency tree (oauth2 + google.golang.org/api + grpc + genproto + cloud.google.com/go/auth) deleted in 13-03.

**No Google secret / OAuth string baked in (byte-accurate ISO-8859-1 whole-blob substring scan of the re-targeted binary):**

| Pattern | Matches |
|---------|---------|
| `oauth2.googleapis.com` | 0 |
| `accounts.google.com` | 0 |
| `client_secret` | 0 |
| `spreadsheets.googleapis.com` | 0 |
| `sheets.googleapis.com` | 0 |
| `googleapis.com/auth` | 0 |
| `GOCSPX-` (Google OAuth client-secret prefix) | 0 |
| `golang.org/x/oauth2` | 0 |
| `google.golang.org/api` | 0 |
| **TOTAL** | **0** |

Not even the `golang.org/x/oauth2` / `google.golang.org/api` package-path strings are present — those would embed if any of that code were still linked, so their absence confirms the tree is genuinely gone (not merely dead code).

**Dependency tree Google-free:** `go list -deps ./cmd/squirebot` contains **zero** packages whose import path contains "google" (so no `golang.org/x/oauth2`, no `google.golang.org/api`, no `cloud.google.com`; the benign `database/sql/driver` "drive" substring is also absent — this watcher binary does not even pull `database/sql`).

**All three SC-2 sub-checks pass — no Google package survived the deletion, so there is nothing to re-open back in Plan 03.**

## Decisions Made

- **`freeConsole()` returns nil unconditionally (999.21):** `ret==0` is the benign no-console case on a GUI/Explorer launch; it is logged at `Debug` so it stops spamming `Warn` on every such launch, and a genuine detach failure is non-fatal anyway (the watcher continues), so it does not surface as an error return. The doc was rewritten to state exactly this — doc and impl now agree.
- **`update.IsNewer` is a separate copy of the compare doctrine, not a shared import (999.22):** it mirrors `ingest.IsOlder`'s *direction* (a pre-release of a given core is older than that core's final) but the watcher must not import `internal/backendsrv/*`. Grep-confirmed the only `backendsrv` token in `internal/update/` is the doc-comment doctrine note, not an `import`.
- **Watcher-side fail-CLOSED-on-the-update-decision:** unlike the server's `IsOlder` (fail-closed on the *gate* — an unparseable present version is treated as below the floor so it is rejected), the watcher's `IsNewer` returns `false` when *either* version is unparseable — i.e. never update on doubt. Opposite mechanics, same "doubt is safe" spirit; documented in both the impl doc and the test.
- **v1 baseline built, not estimated:** rather than cite the documented ~17 MB figure alone, I built the pre-deletion commit (`d12fbaf`) in a detached `git worktree` with identical ldflags for an apples-to-apples byte delta (16.44 MB measured, corroborating the documented figure). The worktree was removed and the gitignored `dist/` artifacts pruned afterward.

## Deviations from Plan

None — plan executed exactly as written. No Rule 1/2/3 auto-fixes were triggered: the three target files matched the plan's `<interfaces>` snapshots, the SemVer rework mirrored the already-proven `ingest.IsOlder`, and all three SC-2 confirmations passed on the first build (no Plan-03 regression to re-open).

**Path note (not a deviation, recorded for traceability):** the orchestrator prompt and `13-CONTEXT.md` D-9 referred to the console file as `internal/system/console_windows.go`, but the plan frontmatter, the plan's `<interfaces>`, and the actual repo all place it at `cmd/squirebot/console_windows.go` (confirmed via Glob — `internal/system/` exists as a package but contains no `console_windows.go`). The plan frontmatter is authoritative; the correct file was edited. The pre-existing `cmd/squirebot-server/main.go` gofmt nit logged in `deferred-items.md` (by 13-01) is a *different* file (server side) and not one of this plan's three nits — left untouched (out of scope; 999.20 is specifically the watcher `console_windows.go`).

## Issues Encountered

- **Cross-shell quoting:** the size/secret-scan PowerShell one-liners were mangled when passed through the Bash tool (single quotes and `-` operators stripped). Resolved by writing small `.ps1` scripts under the gitignored `dist/` and running them with `-File` — clean output, no repo pollution. Scripts pruned after use.

## User Setup Required

None — no external service configuration. (This plan is hygiene + a P16-de-risking version-compare fix + a read-only SC-2 confirmation.)

## Next Phase Readiness

- **Phase 13 is COMPLETE (4/4).** The watcher is re-targeted (13-03), the Google stack is gone, the auto-update compare is now pre-release-safe (this plan), and the SC-2 "materially smaller, no Google secret" milestone claim is measured and recorded.
- **P16 (Cutover) de-risked:** `update.IsNewer` and the server's `ingest.IsOlder` now agree on pre-release ranking, so the coordinated self-update flip (CUTOVER-03) will not be surprised by a pre-release safety manifest — a final `v2.x.0` tag is correctly seen as newer by every watcher, including any parked on a pre-release, and a stray `-rcN` manifest can never make a final watcher downgrade.
- **No blockers.** The re-targeted binary builds clean with the CI ldflags; the auto-updater contract (`binary_url`/`binary_sha256` in `release.yml`) is intact from 13-03.

## Self-Check: PASSED

- Modified files verified on disk: `cmd/squirebot/console_windows.go`, `internal/update/manifest.go`, `internal/update/manifest_test.go` — all FOUND.
- Task commits verified in git: `c930fc2` (Task 1), `e758fb0` (Task 2 RED), `3e8e53b` (Task 2 GREEN) — all FOUND.
- gofmt -l clean on all three touched files; `go build`/`go vet`/`go test ./...` green (22 pkg, 0 fail); 999.20/21/22 fixed; SemVer pre-release test-proven; SC-2 binary-smaller (57%) + zero-Google-string evidence captured.

---
*Phase: 13-watcher-re-target-onboarding*
*Completed: 2026-05-30*
