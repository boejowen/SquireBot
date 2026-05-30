---
phase: 13-watcher-re-target-onboarding
plan: 01
subsystem: api
tags: [go, net/http, bearer-auth, semver, version-gate, ingest, sqlite]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api
    provides: "POST /api/v1/ingest, auth.ResolveToken bearer guard, Envelope.watcher_version field, owner table, store.NewTestDB/MintCode/RevokeCode harness"
provides:
  - "GET /api/v1/whoami — authed, side-effect-free validation endpoint (reuses auth.ResolveToken; 200 {owner_id, owner_label} on a valid code, 401 otherwise)"
  - "ingest.IsOlder — SemVer-aware (pre-release-aware) server-side version compare; the single backend version-compare truth"
  - "min-watcher-version 426 gate in the ingest handler (const minWatcherVersion = 2.0.0; rejects a too-old watcher_version, writes nothing)"
affects: [13-02-watcher-foundation, 13-03-retarget-integration, 13-04-polish, 16-cutover-decommission, 15-admin-web-forms-login]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SemVer-with-pre-release compare with asymmetric failure handling (fail-closed on a bad PRESENT version, fail-open on a bad FLOOR const)"
    - "One version-compare truth PER SIDE (server ingest.IsOlder + watcher 999.22 twin are deliberate separate copies; no client<->server internal imports)"
    - "Authed read-only probe endpoint (whoami) reusing the ingest bearer guard verbatim — 200/401 distinction + a friendly label, no side effects"

key-files:
  created:
    - internal/backendsrv/ingest/version.go
    - internal/backendsrv/ingest/version_test.go
    - internal/backendsrv/ingest/whoami.go
    - internal/backendsrv/ingest/whoami_test.go
  modified:
    - internal/backendsrv/ingest/handler.go
    - internal/backendsrv/ingest/handler_test.go
    - cmd/squirebot-server/main.go

key-decisions:
  - "minWatcherVersion floor = 2.0.0 (the first re-targeted watcher version that POSTs to /api/v1/ingest); bump only on a real breaking API change"
  - "Empty watcher_version is NOT version-rejected (env.WatcherVersion != '' guards the gate) — forward-compat with the 'accepted now' envelope contract; the one intentional exception to IsOlder's 'empty present => older' fail-closed rule"
  - "whoami authors one inline read-only SELECT label FROM owner WHERE id=? (the plan's documented exception); it is the single SQL in the file, a pure read, side-effect-free — the single-tested-SQL-path WRITE constraint is unaffected"
  - "A scan failure in whoami's label lookup degrades to an empty label (200 attests validity, not the label), never a 500"

patterns-established:
  - "Gate placement [2.5]: after DecodeAndValidate (so malformed bodies 400 first), before parse/bind/store (so a 426 writes nothing), after the bearer guard (so a missing token is 401, never 426)"
  - "426 Upgrade Required carries the exact watcher-facing string the tray will mirror"

requirements-completed: []  # WATCH-08/09/10 are PARTIALLY advanced (backend half only); see "Requirements" below — not marked complete until the watcher re-target (13-02/03) lands.

# Metrics
duration: 12min
completed: 2026-05-30
---

# Phase 13 Plan 01: Backend Additions (/whoami + 426 version gate) Summary

**`GET /api/v1/whoami` authed validation endpoint + a SemVer-aware `ingest.IsOlder` + a `426 Upgrade Required` min-watcher-version gate in the ingest handler — the ~50-LOC backend half of Phase 13, httptest-proven and static-linux/amd64-cross-compile-ready for the bundled P12 redeploy.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-05-30T05:14:17Z
- **Completed:** 2026-05-30T05:25:00Z
- **Tasks:** 3 (all TDD: RED → GREEN)
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments
- **`ingest.IsOlder`** — the single backend version-compare truth: strips a leading `v`, compares `MAJOR.MINOR.PATCH`, then applies SemVer §11 pre-release precedence (a pre-release sorts BELOW its final). Fail-CLOSED on a bad/empty present version (a forged/blank version cannot pass the gate); fail-OPEN on a bad floor const (our misconfig never rejects a real client). 20 table cases.
- **`GET /api/v1/whoami`** — authed, side-effect-free validation endpoint reusing `auth.ResolveToken` verbatim; returns `200 {owner_id, owner_label}` on a valid active code, `401` on missing/unknown/revoked, `405` on non-GET, and never logs the bearer token (V7). Registered on the ServeMux next to the ingest route.
- **426 gate** — `const minWatcherVersion = "2.0.0"` + an `if env.WatcherVersion != "" && IsOlder(...)` check at `[2.5]` in `ServeHTTP` that returns `426 Upgrade Required` with a clear human message and writes nothing. The pre-existing 11-05 ingest suite (whose bodies carry `watcher_version:"2.0.0"`) stays green.
- Whole-repo `go build ./...` + `go vet ./...` + `go test ./...` all green (zero v1 watcher regression); the off-cloud-suite dep invariant holds; the static `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` build produces a statically-linked x86-64 ELF (the deploy artifact).

## Task Commits

Each task was committed atomically (TDD: test → feat):

1. **Task 1: SemVer-aware `IsOlder`** — `7d3e061` (test RED) → `54b9972` (feat GREEN)
2. **Task 2: `GET /api/v1/whoami` + route** — `5e35bc0` (test RED) → `b520790` (feat GREEN)
3. **Task 3: min-watcher-version 426 gate** — `2eb3efa` (test RED) → `7256e63` (feat GREEN)

**TDD gate compliance:** each task has a `test(...)` commit (RED) followed by a `feat(...)` commit (GREEN). No REFACTOR commits were needed (implementations were clean as written). The Task 1 RED was a compile failure (`undefined: IsOlder`); the Task 3 RED was a runtime assertion failure (a too-old version returned 204 before the gate existed) — neither passed unexpectedly.

## Files Created/Modified
- `internal/backendsrv/ingest/version.go` (118 LOC) — `IsOlder` + the internal `parseSemver` helper; the one-truth-per-side doctrine documented in the file header.
- `internal/backendsrv/ingest/version_test.go` (62 LOC) — `TestIsOlder` table (in-package `ingest`, since `IsOlder` is the package-internal gate primitive).
- `internal/backendsrv/ingest/whoami.go` (94 LOC) — `WhoamiHandler` + `NewWhoami`; reuses the bearer guard, one read-only owner-label SELECT, V7 logging.
- `internal/backendsrv/ingest/whoami_test.go` (194 LOC) — `ingest_test` httptest cases: valid-200-no-side-effects, missing/unknown/revoked-401, non-GET-405, no-token-in-logs.
- `internal/backendsrv/ingest/handler.go` (+27/-2) — `minWatcherVersion` const + the `[2.5]` 426 gate; refreshed the now-stale "gated in P13" comment in `bindAndReplace`.
- `internal/backendsrv/ingest/handler_test.go` (+115) — `invBodyVer` helper + 5 gate cases (426-writes-nothing, floor/newer/empty proceed, too-old-no-auth-still-401).
- `cmd/squirebot-server/main.go` (+1 route line, comment refreshed) — `mux.Handle("GET /api/v1/whoami", ingest.NewWhoami(auth.New(db), db))`.

## Decisions Made
- **Floor = `2.0.0`** with a doc comment to bump only on a real breaking API change (the version that re-targets from Sheets to the ingest API).
- **Empty `watcher_version` allowed:** the `env.WatcherVersion != ""` guard makes a legacy/blank version proceed, honoring the envelope's documented "accepted now" forward-compat contract — the one intentional exception to `IsOlder`'s fail-closed-on-empty rule (which still governs the *compare* primitive in isolation, e.g. for a future stricter caller).
- **whoami's label read is inline:** the plan's `<action>` explicitly authorizes the one-line `SELECT label FROM owner WHERE id=?` here. It is the only SQL in the file, a pure parameterized read scoped to the resolved ownerID, and side-effect-free (proven by the row-count-unchanged test). CLAUDE.md's single-tested-SQL-path rule targets the WRITE/replace path, which whoami does not touch — so this is consistent, not a violation.
- **Label-lookup failure degrades to `""`** rather than 500: the 200 attests code validity (the load-bearing 200/401 signal the watcher gates on), not the cosmetic label.

## Deviations from Plan

None — plan executed exactly as written. No Rule 1/2/3 auto-fixes were triggered (the contract files matched the plan's `<interfaces>` exactly, and the floor of `2.0.0` kept every pre-existing test green without adjustment).

One **out-of-scope discovery** was logged (NOT fixed, per the SCOPE BOUNDARY rule): `cmd/squirebot-server/main.go`'s `const defaultAddr` line is not `gofmt -l` clean (a one-extra-space comment-alignment nit at lines ~50-51). Verified present in commit `aabf9b8` (pre-Phase-13) — it is in the `const` block, unrelated to this plan's route-registration edit lower in the file. Logged to `.planning/phases/13-watcher-re-target-onboarding/deferred-items.md`; a future watcher-polish sweep (13-04 already carries gofmt nits 999.20/21) or a `gofmt -w` pass can absorb it. All lines this plan added/edited are gofmt-clean.

## Issues Encountered
None. The TDD RED/GREEN cycles ran cleanly; the existing httptest harness (`store.NewTestDB`, `auth.MintCode`/`RevokeCode`, the `post`/`countInv`/`totalInv` helpers) extended naturally to the gate and whoami cases.

## Requirements

The plan frontmatter lists `[WATCH-08, WATCH-09, WATCH-10]`, but 13-01 delivers only the **backend half** of those requirements (the `/whoami` validation target + the version-gate reject that replaces the retired `WatcherMaxSchemaVersion` Sheets gate). The requirements themselves (re-target the watcher to the backend, native onboarding, DPAPI token storage) are **not satisfied until the watcher work lands in 13-02/13-03**. They are therefore left UNMARKED here and should be marked complete at the end of the watcher integration (or phase completion), not now — marking them complete on the backend half alone would misreport the watcher re-target as done.

## User Setup Required
None — no external service configuration. (Deployment note below is an ops action, not a code/config prerequisite for this plan.)

## Next Phase Readiness
- **DEPLOY-PENDING bundle ready:** these backend additions ship in ONE VPS redeploy bundled with the P12 enrichment binary. The static `linux/amd64` ELF builds; `goose.Up` applies `00003` (P12) on restart; no new systemd timer (the scheduler is in-process). Build stamp per STATE: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X github.com/boejowen/SquireBot/internal/backendsrv/buildinfo.Version=<tag>" -o squirebot-server ./cmd/squirebot-server`.
- **13-02 (watcher foundation)** can now build its `internal/backend` client against a real validation target (`GET /api/v1/whoami`) and a real version-reject contract (426). The watcher-side SemVer twin (999.22, Plan 04) must behave identically to `ingest.IsOlder` — the doctrine is documented in `version.go`'s header.
- No blockers.

## Self-Check: PASSED

- Created files verified on disk: `version.go`, `version_test.go`, `whoami.go`, `whoami_test.go`, `13-01-SUMMARY.md`, `deferred-items.md` — all FOUND.
- Task commits verified in git: `7d3e061`, `54b9972`, `5e35bc0`, `b520790`, `2eb3efa`, `7256e63` — all FOUND.

---
*Phase: 13-watcher-re-target-onboarding*
*Completed: 2026-05-30*
