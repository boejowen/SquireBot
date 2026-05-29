---
phase: 11-backend-foundation-ingest-api
plan: 04
subsystem: auth
tags: [go, crypto, bearer-token, sha256, crypto-rand, crypto-subtle, sqlite, cli, asvs]

# Dependency graph
requires:
  - phase: 11-backend-foundation-ingest-api (Plan 01)
    provides: "D-01 verdict HAND-ROLLED Go (no PocketBase) — auth is plain stdlib crypto + database/sql, NOT PB auth-records"
  - phase: 11-backend-foundation-ingest-api (Plan 02)
    provides: "guild_code(owner_id, token_hash BLOB UNIQUE, label, disabled_at) + owner table in 00001_init.sql; store.Open + store.NewTestDB shared fixture"
provides:
  - "auth.MintCode(db, ownerLabel) (plaintext, error) — 32-byte crypto/rand token -> base64url plaintext printed ONCE via stdout + SHA-256 hash stored in guild_code.token_hash (hash-only at rest, D-05)"
  - "auth.RevokeCode(db, idOrLabel) error — sets disabled_at WHERE (label OR id) AND disabled_at IS NULL; idempotent (D-09)"
  - "auth.Auth + auth.New(db) — request-time bearer guard; resolveToken(ctx, authHeader) (ownerID, ok) hashes the presented code and constant-time-compares against active guild_code rows; missing/malformed/unknown/revoked -> (0,false) (D-08)"
  - "upsertOwner(db, label) — SELECT-then-INSERT owner by label (reuse, no duplicate owners)"
affects: [11-05, 13-watcher-re-target-onboarding, 15-admin-web-forms-login]

# Tech tracking
tech-stack:
  added: []  # stdlib only — crypto/rand, crypto/sha256, crypto/subtle, encoding/base64 (no new third-party deps)
  patterns:
    - "Hash-only-at-rest bearer token: 32-byte crypto/rand plaintext exists ONCE at mint (stdout + return), only sha256(plaintext) persisted as a SQLite BLOB — mirrors the watcher's internal/auth/store.go DPAPI hash-only discipline"
    - "Constant-time token compare: sha256.Sum256(presented) then subtle.ConstantTimeCompare against each active (disabled_at IS NULL) row — timing-safe, framework-agnostic (no PocketBase apis.RequireAuth)"
    - "Verdict-agnostic auth API: MintCode/RevokeCode/Auth.resolveToken are pure crypto + database/sql, imported UNCHANGED by 11-05 regardless of HTTP shell"
    - "Structural security assertion via AST import-parse (not naive strings.Contains) so a doc comment naming an anti-pattern does not trip a 'must-not-use' check"

key-files:
  created:
    - "internal/backendsrv/auth/mint.go"
    - "internal/backendsrv/auth/store.go"
    - "internal/backendsrv/auth/guard.go"
    - "internal/backendsrv/auth/mint_test.go"
    - "internal/backendsrv/auth/guard_test.go"
  modified: []

key-decisions:
  - "Added an exported auth.New(db) *Auth constructor (Rule 2): the plan's <interfaces> gave Auth{db *sql.DB} with an unexported field but no constructor; 11-05 (a separate package) cannot build an Auth without one. New(db) keeps db unexported while making the guard constructible — required for the verdict-agnostic 11-05 import contract."
  - "upsertOwner is SELECT-then-INSERT (not INSERT ON CONFLICT) because owner.label has no UNIQUE constraint in the D-13 schema (label is a human handle, not an identity key); re-minting for the same guildie reuses the owner row instead of spawning duplicates."
  - "RevokeCode matches EITHER label OR id (D-09 accepts either) and is idempotent via the `AND disabled_at IS NULL` guard — revoking twice is a no-op rather than re-stamping disabled_at."
  - "resolveToken iterates active rows and ConstantTimeCompares each (RESEARCH Pattern 3) rather than a direct token_hash PK lookup — belt-and-braces timing safety matching the ASVS expectation; trivially cheap at ~12 codes."

patterns-established:
  - "Pattern: stdlib-only crypto for the first authenticated network surface — crypto/rand for entropy, crypto/sha256 for at-rest hashing, crypto/subtle for the compare; NEVER hand-rolled, NEVER math/rand (V6)"
  - "Pattern: auth logging carries no token material — slog 'auth_reject' records emit only a reason (and DB err on query failure), never the raw token or Authorization header (V7)"

requirements-completed: [BACKEND-04]

# Metrics
duration: 7 min
completed: 2026-05-29
---

# Phase 11 Plan 04: Bearer-Token Auth (mint/revoke CLI + resolveToken Guard) Summary

**Stdlib-only bearer-token auth for the backend's first network surface: a `mint-code` that emits a 32-byte `crypto/rand` base64url plaintext exactly once and persists only its SHA-256 hash, a `revoke-code` that disables the row, and a `resolveToken` guard that SHA-256-hashes a presented `Bearer` value and `crypto/subtle.ConstantTimeCompare`s it against active `guild_code` rows — verdict-agnostic, imported unchanged by 11-05, no PocketBase coupling.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-05-29T21:57:37Z
- **Completed:** 2026-05-29T22:04:58Z
- **Tasks:** 2 (both TDD: RED -> GREEN; no REFACTOR needed)
- **Files created:** 5

## Accomplishments

- **Hash-only mint (D-05 / V6):** `MintCode(db, ownerLabel)` generates 32 bytes from `crypto/rand`, base64url-encodes them as the plaintext (shown ONCE via `fmt.Printf` to stdout + returned), and stores ONLY `sha256(plaintext)` in `guild_code.token_hash` (a 32-byte BLOB). The plaintext is never persisted and never logged — exactly the watcher's `internal/auth/store.go` "the secret never leaves as plaintext" discipline, backed by SQLite instead of wincred. `upsertOwner` reuses an owner by label so re-minting for a guildie does not spawn duplicate owners.
- **Idempotent revoke (D-09 / T-11.04-04):** `RevokeCode(db, idOrLabel)` sets `disabled_at` on the matching active row(s) (`WHERE (label = ? OR id = ?) AND disabled_at IS NULL`); a revoked code is then excluded from the guard's candidate set. Revoking twice is a no-op.
- **Timing-safe guard (D-08 / V2 / T-11.04-02):** `Auth.resolveToken(ctx, authHeader)` strips the `Bearer ` prefix (or fails), `sha256.Sum256`-hashes the presented code, queries active rows (`disabled_at IS NULL`), and `subtle.ConstantTimeCompare`s each — returning `(ownerID, true)` on the first match and `(0, false)` for missing/malformed/unknown/revoked/wrong-scheme. The presented value is hashed before any DB use (no SQL interpolation of the token — V5/T-11.04-05) and never reaches `slog` (V7).
- **Verdict-agnostic & framework-free:** the auth API is pure `crypto/*` + `database/sql`; the package imports zero third-party deps and does NOT use PocketBase's `apis.RequireAuth()`/JWT auth-record system (guild codes are opaque static tokens). 11-05 imports `MintCode`/`RevokeCode`/`Auth`/`New` UNCHANGED whether it wires `net/http` (the 11-01 HAND-ROLLED verdict) or any other shell.
- **Full green, no regression:** `go build ./...`, `go vet ./...` clean; `go test ./internal/backendsrv/...` exit 0 (auth + migrations + store); full `go test ./...` exit 0 (no v1 watcher regression).

## Task Commits

Each task was committed atomically (both TDD, RED -> GREEN):

1. **Task 1 (RED): failing mint/revoke tests** — `b06395c` (test)
2. **Task 1 (GREEN): mint-code/revoke-code + hash-only storage** — `b63c080` (feat)
3. **Task 2 (RED): failing bearer-guard tests** — `5bacdc0` (test)
4. **Task 2 (GREEN): resolveToken + constant-time compare** — `f5b4b66` (feat)

_No REFACTOR commits — both implementations were minimal and clean at GREEN. Plan metadata committed separately._

## Files Created/Modified

- `internal/backendsrv/auth/mint.go` — `MintCode(db, ownerLabel) (string, error)`: `crypto/rand` 32-byte token -> `base64.RawURLEncoding` plaintext (shown once) -> `sha256.Sum256` -> `INSERT INTO guild_code` storing `sum[:]`. The token-gen shape mirrors `internal/auth/pkce.go:27-34`.
- `internal/backendsrv/auth/store.go` — package docstring (hash-only-at-rest discipline) + `upsertOwner(db, label) (int64, error)` (SELECT-then-INSERT) + `RevokeCode(db, idOrLabel) error` (`UPDATE ... SET disabled_at`).
- `internal/backendsrv/auth/guard.go` — `Auth{db *sql.DB}` + `New(db) *Auth` + `resolveToken(ctx, authHeader) (ownerID int64, ok bool)` (RESEARCH Pattern 3: prefix-strip, `sha256.Sum256`, active-rows query, `subtle.ConstantTimeCompare`).
- `internal/backendsrv/auth/mint_test.go` — `TestMintCode_StoresHashNotPlaintext`, `_DistinctTokens`, `_RoundTrip`, `_ReusesOwnerByLabel`, `TestRevokeCode_DisablesRow` (via `store.NewTestDB`).
- `internal/backendsrv/auth/guard_test.go` — `TestResolveToken_Table` (7 cases), `_ReturnsMintingOwner`, `_UsesConstantTimeCompare` (AST import-parse + source grep for the security controls).

## Decisions Made

- **Exported `New(db) *Auth` constructor (deviation Rule 2 — see below):** the plan's `<interfaces>` specified `Auth{db *sql.DB}` (unexported field) and `resolveToken`, but provided no way for the separate 11-05 package to construct an `Auth`. Added `New(db)` so the guard is constructible from outside the package while keeping `db` unexported. Necessary for the verdict-agnostic 11-05 import contract.
- **`upsertOwner` = SELECT-then-INSERT** (not `INSERT ... ON CONFLICT`): `owner.label` has no UNIQUE constraint in D-13 (a label is a human handle, not an identity key), so a conflict target does not exist; reusing the row by label prevents duplicate owners on re-mint.
- **`RevokeCode` accepts label OR id and is idempotent:** D-09 says "disable the hashed row" and accepts either identifier; the `AND disabled_at IS NULL` guard makes a second revoke a no-op rather than re-stamping the timestamp.
- **Iterate-and-ConstantTimeCompare over a direct PK lookup:** RESEARCH Pattern 3's belt-and-braces timing safety (matching the ASVS expectation), trivially cheap at the guild's ~12 codes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added exported `auth.New(db) *Auth` constructor**
- **Found during:** Task 2 (bearer guard)
- **Issue:** The plan's `<interfaces>` defined `type Auth struct { db *sql.DB }` with an unexported field and `resolveToken`, but no constructor. 11-05 lives in a different package and cannot populate the unexported `db` field, so it could not build an `Auth` to call the guard — the guard would be unreachable from its only consumer.
- **Fix:** Added `func New(db *sql.DB) *Auth { return &Auth{db: db} }`, keeping `db` unexported (encapsulation preserved) while making the guard constructible. No behavior change to `resolveToken`.
- **Files modified:** `internal/backendsrv/auth/guard.go`
- **Verification:** `go build ./...` + `go vet ./...` clean; the guard tests construct `&Auth{db: db}` directly (same package) and pass; 11-05 will use `auth.New(...)`.
- **Committed in:** `f5b4b66` (Task 2 GREEN commit)

**2. [Rule 1 - Bug] Fixed a false-positive structural security assertion**
- **Found during:** Task 2 (bearer guard, GREEN)
- **Issue:** `TestResolveToken_UsesConstantTimeCompare` initially used `strings.Contains(src, "apis.RequireAuth")` to assert the guard does NOT use PocketBase's JWT auth. That matched the guard's own DOC COMMENT, which intentionally names `apis.RequireAuth()` to document what it deliberately avoids — so the test failed on correct code (a test-logic false positive, not a guard defect).
- **Fix:** Rewrote the negative check to parse `guard.go`'s imports from the AST (`go/parser` + `go/token`, `parser.ImportsOnly`) and assert no import path contains `pocketbase` — the real invariant, robust against comment prose. The positive controls (`subtle.ConstantTimeCompare`, `disabled_at IS NULL`, `sha256.Sum256`) remain source greps.
- **Files modified:** `internal/backendsrv/auth/guard_test.go`
- **Verification:** `TestResolveToken_UsesConstantTimeCompare` now PASSES; `go list` confirms the auth package imports only stdlib (no `pocketbase`/`apis`).
- **Committed in:** `f5b4b66` (Task 2 GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 missing-critical, 1 test-logic bug).
**Impact on plan:** No scope creep. The constructor is the minimal addition needed for 11-05 to consume the guard (the plan's stated contract). The test fix corrected a false positive on correct code — the security invariant it checks (no PocketBase auth coupling) is now enforced more robustly via AST parsing. No production behavior changed beyond adding the constructor.

## Issues Encountered

None beyond the test-logic false positive documented as Deviation 2 (resolved within the same task). The stdout `mint` message ("Guild code for ... store now — not shown again") surfaces in test logs by design (the plaintext is printed exactly once at mint, per D-05) — that is the intended contract, not a leak: it goes to stdout via `fmt.Printf`, never to `slog`, and never to the DB.

## Threat Model Compliance

All `mitigate`-disposition threats from the plan's STRIDE register are covered and test-proven:

- **T-11.04-01 (token brute-force):** 32-byte (256-bit) `crypto/rand` tokens; `TestMintCode_DistinctTokens` proves entropy; a non-match returns `(0, false)` revealing nothing.
- **T-11.04-02 (timing attack):** `subtle.ConstantTimeCompare` on the hash bytes (`guard.go:76`); `TestResolveToken_UsesConstantTimeCompare` enforces its presence structurally.
- **T-11.04-03 (token leakage in logs):** only `sha256(plaintext)` is persisted; the plaintext crosses to the maintainer once via stdout; no token/Authorization material reaches `slog` (the four `auth_reject` records carry only a reason + DB err) — `TestMintCode_StoresHashNotPlaintext` proves the plaintext is absent from the DB.
- **T-11.04-04 (revoked code still authenticates):** `WHERE disabled_at IS NULL` excludes revoked rows; `RevokeCode` sets `disabled_at`; the `TestResolveToken_Table` revoked case proves `(0, false)`.
- **T-11.04-05 (SQLi via the presented token):** the token is `sha256`-hashed before any DB use; the active-rows query has no user-supplied SQL fragment; mint/revoke/upsert use `?` placeholders only.
- **T-11.04-06 (weak/predictable token):** `crypto/rand` only — the auth package imports `crypto/rand`, never `math/rand` (`go list` confirms).

ASVS L1 coverage delivered at the build/test tier: V2 (opaque high-entropy token, hash-at-rest, constant-time compare, not-authenticated reveals nothing), V6 (`crypto/rand` + `crypto/sha256` + `crypto/subtle`, never hand-rolled), V7 (never log the token). The 401-writes-nothing wiring is completed in 11-05 (the handler returns before any store call when `ok == false`).

## Known Stubs

None. The guard, mint, and revoke logic are complete and test-proven. The only thing intentionally deferred is the HTTP/CLI WIRING (11-05): `MintCode`/`RevokeCode` are not yet dispatched from `cmd/squirebot-server`, and `resolveToken` is not yet bound to a `net/http` middleware — that is 11-05's job per the plan's verdict-agnostic boundary, not an unfinished stub here.

## Next Phase Readiness

- **BACKEND-04 satisfied at the build/test tier.** `auth.MintCode`, `auth.RevokeCode`, `auth.New`, and `(*auth.Auth).resolveToken` are exported (resolveToken is package-internal but reachable via the guard `Auth` 11-05 constructs) and ready for 11-05 to wire.
- **11-05 (ingest + HTTP/CLI shell)** composes: `cmd/squirebot-server mint-code/revoke-code` -> `auth.MintCode`/`auth.RevokeCode`; the `POST /api/v1/ingest` middleware -> `auth.New(db)` + `resolveToken` (map `ok == false` to 401 writing nothing), then 11-03's `bindCharacter` + `ReplaceInventory`/`ReplaceSpellbook` in one tx. 11-05 still carries the two cleanup chores (delete `spike/pocketbase/` + remove the `pocketbase` dep, then `go mod tidy`).
- **resolveToken is package-unexported by design.** 11-05 calls it through the `*Auth` value it builds with `auth.New(db)`; if 11-05's middleware needs to call it across the package boundary, it will be exported then (a one-line change) — the plan's `<interfaces>` deliberately kept it lowercase for now.
- **No blockers.**

## Self-Check: PASSED

- Files on disk: `auth/mint.go` FOUND, `auth/store.go` FOUND, `auth/guard.go` FOUND, `auth/mint_test.go` FOUND, `auth/guard_test.go` FOUND, `11-04-SUMMARY.md` FOUND.
- Commits exist: `b06395c` (Task 1 RED test) FOUND, `b63c080` (Task 1 GREEN feat) FOUND, `5bacdc0` (Task 2 RED test) FOUND, `f5b4b66` (Task 2 GREEN feat) FOUND.
- Plan `<verification>`: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./internal/backendsrv/...` exit 0; full `go test ./...` exit 0 (no regression). `mint.go` contains `rand.Read` + `base64.RawURLEncoding` + `sha256.Sum256` + `INSERT INTO guild_code` storing `sum[:]`; `guard.go` contains `subtle.ConstantTimeCompare` + `WHERE disabled_at IS NULL` + `sha256.Sum256`. `go list` confirms the auth package imports stdlib only — no `math/rand`, no `pocketbase`/`apis`.

## TDD Gate Compliance

Both tasks followed the RED -> GREEN gate sequence in git history:
- **Task 1:** `test(11-04)` RED `b06395c` (fails to compile — `MintCode`/`RevokeCode` undefined) precedes `feat(11-04)` GREEN `b63c080` (5/5 mint/revoke tests pass).
- **Task 2:** `test(11-04)` RED `5bacdc0` (fails to compile — `Auth` undefined) precedes `feat(11-04)` GREEN `f5b4b66` (3/3 resolveToken tests pass).

No REFACTOR commits (both implementations were minimal and clean at GREEN). No test passed unexpectedly during RED (both RED states failed to compile — the cleanest possible RED).

---
*Phase: 11-backend-foundation-ingest-api*
*Completed: 2026-05-29*
