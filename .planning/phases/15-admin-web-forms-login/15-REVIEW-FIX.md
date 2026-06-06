---
phase: 15-admin-web-forms-login
fixed_at: 2026-05-31T03:53:16Z
review_path: .planning/phases/15-admin-web-forms-login/15-REVIEW.md
iteration: 1
findings_in_scope: 14
fixed: 13
skipped: 1
status: partial
---

# Phase 15: Code Review Fix Report

**Fixed at:** 2026-05-31T03:53:16Z
**Source review:** `.planning/phases/15-admin-web-forms-login/15-REVIEW.md`
**Iteration:** 1
**Scope:** `all` (CRITICAL + WARNING + INFO)

**Summary:**
- Findings in scope: 14 (2 critical, 7 warning, 5 info)
- Fixed: 13
- Skipped: 1 (IN-01 — review prescribes "None required" under the forward-only migration posture)

All 13 applied fixes were committed atomically (one logical fix per commit, except
the two tightly-coupled pairs noted below). The whole tree is GREEN — full
backend + frontend build/vet/test results are at the end of this report. The two
CRITICAL blockers (both invisible to the deliberately node-only, DOM-free suite)
each gained a node-only regression test that would have caught them, per the
environment directive.

---

## Fixed Issues

### CR-01: Bank-coin form `TypeError` on first keystroke (number-input binding feeds a number to string helpers)

**Files modified:** `web/src/lib/components/BankCoinForm.svelte`, `web/src/lib/coin.ts`, `web/src/lib/__tests__/coin.test.ts`
**Commit:** `c51638d`
**Applied fix:** Switched the four coin inputs from `type="number"` to `type="text" inputmode="numeric" pattern="[0-9]*"` — this keeps the on-screen numeric keypad but stops Svelte 5's number-input `to_number()` coercion, so `bind:value` stays a string and the strict `/^\d+$/` validation holds. Additionally hardened the pure helpers (`validateCoinField`, `coinValue`) to accept `string | number | null | undefined` via a `rawToTrimmed()` normalizer (defense-in-depth — crash-proof even if the input is ever switched back to `type="number"`; non-finite numbers collapse to blank). Added a CR-01 regression suite that drives the helpers with the `number`/`null`/`undefined` shapes the DOM binding actually produces (the prior string-literal tests structurally could not catch this in a node-only suite).
**Verification:** Tier 1 re-read; Tier 2 `svelte-check` (0 errors) + `vitest` (21 coin tests pass, incl. the 3 new regression cases).

### CR-02: Eviction grace date renders "Jan 1970" (epoch seconds passed to `new Date()` which expects ms)

**Files modified:** `web/src/lib/eviction.ts` (new), `web/src/lib/components/EvictionForm.svelte`, `web/src/lib/api.ts`, `web/src/lib/__tests__/eviction.test.ts` (new)
**Commit:** `43fea96` (also closes IN-02)
**Applied fix:** Extracted `graceDate()` into a new pure `$lib/eviction.ts` helper (mirroring the repo's "pure logic in `.ts` so it's node-testable" philosophy) and fixed the core bug: multiply the backend's epoch-**seconds** `grace_until` by 1000 (seconds→ms) before constructing the `Date`; non-finite values fall back to the string form rather than "Invalid Date". `EvictionForm` now imports the helper and drops its buggy inline copy. The TS interfaces (`EvictionPreview.grace_until`, `EvictResult.grace_until`) are corrected from `string` to `number` (epoch seconds), matching the Go wire shape. Added `eviction.test.ts` asserting an epoch-seconds value yields its real 2026 date, not 1970 (the JSON-shape gap the prior tests never exercised).
**Verification:** Tier 1 re-read; Tier 2 `svelte-check` (0 errors) + `vitest` (4 new eviction tests pass).

### WR-01: Restore can leave a guildie restored-but-codeless on a post-commit mint failure

**Files modified:** `internal/backendsrv/webadmin/eviction.go`, `web/src/lib/api.ts`
**Commit:** `2340476` (combined with WR-02 — same handler + contract field)
**Applied fix:** Resolve the owner label **before** the restore tx (it cannot change during a restore), narrowing the only post-commit fallible step to `auth.MintCode` itself; a missing owner pre-tx is now a clean `400 invalid_input` with nothing committed. On a post-commit mint failure, the handler no longer returns a generic 500 (which stranded the guildie restored-but-codeless while the officer believed nothing happened) — instead it logs at `error` with `owner_id`/`owner_label` and returns a `200` success-with-warning shape `{restored_count, new_code_issued:false, code_mint_failed:true}`, documenting the operator recovery path (`squirebot-server mint-code --owner <label>`). `RestoreResult` (api.ts) gains the optional `code_mint_failed` flag so the future restore form can branch.
**Verification:** Tier 1 re-read; Tier 2 `go build` + `go vet` + `go test ./internal/backendsrv/webadmin/...` (restore tests pass); `gofmt -w`; `svelte-check` (0 errors).

### WR-02: Restore's re-minted guild code is unreachable from the web (printed only to stdout)

**Files modified:** `internal/backendsrv/webadmin/eviction.go`, `web/src/lib/api.ts`
**Commit:** `2340476` (combined with WR-01)
**Applied fix:** Documented the contract honestly on both sides: the minted plaintext goes to the server's stdout/journald **only** (V7) — never the HTTP response — so `new_code_issued:true` means "a fresh code now EXISTS", NOT "the officer holds a deliverable code". The `RestoreResult` doc comment and the handler doc comment now state the maintainer must hand off the new code out-of-band (read from journald or re-run `mint-code`), and the success contract no longer implies a web-deliverable code.
**Verification:** As WR-01 (same commit).

### WR-03: `withTx` does not roll back on a panic inside `fn` (connection/tx leak)

**Files modified:** `internal/backendsrv/webadmin/audit.go`
**Commit:** `7ead41b`
**Applied fix:** Replaced the inline-only error-path rollback with the idiomatic deferred-rollback-guarded-by-a-`committed`-flag pattern. The deferred `tx.Rollback()` unwinds the BEGIN IMMEDIATE tx as a panic propagates — freeing the store's single (`maxconns=1`) pooled writer connection that would otherwise be wedged until the GC finalizer ran — and is a harmless no-op after a successful Commit. The panic still propagates (not recovered).
**Verification:** Tier 1 re-read; Tier 2 `go build` + `go vet` + `go test ./internal/backendsrv/webadmin/...` (pass).

### WR-04: `parseOwnerIDQuery` has no overflow guard (a long digit string wraps to a wrong id)

**Files modified:** `internal/backendsrv/webadmin/eviction.go`, `internal/backendsrv/webadmin/eviction_test.go`
**Commit:** `9047398` (combined with WR-05 — same file/imports)
**Applied fix:** Replaced the hand-rolled `id = id*10 + digit` accumulator with `strconv.ParseInt(raw, 10, 64)`, which returns `ErrRange` on overflow and `ErrSyntax` on any non-digit/sign — both fail the parse — followed by the existing `> 0` check. An oversized `?owner_id=...` that previously could wrap to a positive in-range id on a destructive endpoint is now rejected.
**Verification:** Tier 1 re-read; Tier 2 `go build` + `go vet` + `go test` (eviction tests pass).

### WR-05: Eviction owner-floor protection silently fails OPEN on a label/username mismatch

**Files modified:** `internal/backendsrv/webadmin/eviction.go`, `internal/backendsrv/webadmin/eviction_test.go`
**Commit:** `9047398` (combined with WR-04)
**Applied fix:** Hardened the `owner.label ↔ floor-username` textual bridge from a plain case-sensitive `label = ?` to `TRIM(label) = TRIM(?) COLLATE NOCASE`, so case/whitespace drift (Discord usernames + watcher-supplied labels) no longer defeats the match and let a peer evict the maintainer's data. Crucially, the function now emits a **loud `slog.Warn`** whenever a floor is seeded but cannot be resolved to an owner row (placeholder snowflake username, or no matching label) — making the inert-protection state visible rather than silently failing open. Added a regression (`TestEvict_PeerCannotEvictFloorData_CaseAndWhitespaceInsensitive`) proving a case+whitespace-only label drift is still protected. The full close (an `owner_floor_owner_id` schema link keyed on id, not a mutable display name) is documented as out-of-phase follow-up, per the review.
**Verification:** Tier 1 re-read; Tier 2 `go build` + `go vet` + `go test` (the new regression passes; the warn was observed firing in `TestEvict_NonOfficerRejected` where the floor username is the snowflake placeholder).
**Note:** This is a security fail-OPEN → fail-CLOSED hardening; the match logic was verified by a behavioral regression test (not merely syntax), so it is committed as a confident fix rather than flagged for human re-verification. The residual schema-level gap (no id link) is a documented, intentional out-of-phase limitation.

### WR-06: `whoami-web` re-resolves and rolls the session via a side-effecting GET

**Files modified:** `internal/backendsrv/webauth/session.go`, `internal/backendsrv/webauth/handlers.go`, `internal/backendsrv/webauth/handlers_test.go`
**Commit:** `5582f9a`
**Applied fix:** Added `resolveSessionUserReadOnly` (same fail-closed `ResolveSession` contract, **no** `TouchSession`) and routed `WhoamiWebHandler` through it. The documented side-effect-free AuthGate feed no longer rolls `expires_at` on every passive mount/refresh (which re-armed a 30-day session and weakened the "departed guildie's session lapses" property, and added a serialized write under `maxconns=1`). The rolling-window bump now happens only on the gated `RequireSession`/`RequireOfficer` hits, which keep the touching `resolveSessionUser`. Added `TestWhoamiWebHandler_DoesNotRollExpiry` asserting `expires_at` is unchanged after a whoami call.
**Verification:** Tier 1 re-read; Tier 2 `go build` + `go vet` + `go test ./internal/backendsrv/webauth/...` (the new regression passes; the existing rolling-TTL test for the gated path still passes).

### WR-07: `lock_busy` handled by the frontend but never produced by the backend

**Files modified:** `internal/backendsrv/webadmin/audit.go`, `web/src/lib/api.ts`, `web/src/lib/admin.ts`
**Commit:** `9868e75`
**Applied fix:** Took the **keep-with-comment** option (not the drop option): documented on both sides that `lock_busy` is intentionally unreachable from the current backend (`busy_timeout(5000)` + `SetMaxOpenConns(1)` serialize writes and wait rather than erroring, so `SQLITE_BUSY` never surfaces), and is retained purely as defense-in-depth for a future where `busy_timeout` is lowered — with a pointer to wire `writeJSONError(403, "lock_busy")` if such a path is ever added. The drop option was rejected because `adminApi.test.ts` and `adminHelpers.test.ts` assert the `lock-busy` classification + copy; dropping would churn green tests for no behavioral gain. Comment-only; no behavior change.
**Verification:** Tier 1 re-read; Tier 2 `go build` (pass) + `svelte-check` (0 errors).

### IN-02: `EvictionPreview` / `EvictResult` `grace_until` typed `string` but sent as number

**Files modified:** `web/src/lib/api.ts` (with CR-02)
**Commit:** `43fea96`
**Applied fix:** Both `grace_until` fields aligned to `number` (epoch seconds) with an explicit comment that they are epoch seconds, mirroring the Go side. (This was the root-cause type drift behind CR-02; fixed in the same change. The other reviewed display interfaces were audited — no further `string`-vs-number `grace_until`/timestamp drift remained on the changed surfaces.)
**Verification:** Folded into CR-02's verification.

### IN-03: `ConfirmDialog` heading id uses `Math.random()`

**Files modified:** `web/src/lib/components/ConfirmDialog.svelte`
**Commit:** `1a001d8`
**Applied fix:** Replaced `Math.random().toString(36).slice(2)` with `crypto.randomUUID()` for collision-free, deterministic per-instance uniqueness on the `aria-labelledby` heading id. Cosmetic; no behavior change.
**Verification:** Tier 1 re-read; Tier 2 `svelte-check` (0 errors) + `vitest` (ConfirmDialog tests pass).

### IN-04: `BankCoinForm` recomputes `coinPayload(inputs)` twice in the save path

**Files modified:** `web/src/lib/components/BankCoinForm.svelte`
**Commit:** `a39434e`
**Applied fix:** Compute `const payload = coinPayload(inputs)` once and reuse it for both the `saveCoin()` POST and the optimistic local update. Cosmetic; no behavior change.
**Verification:** Tier 1 re-read; Tier 2 `svelte-check` (0 errors) + `vitest` (coin tests pass).

### IN-05: CORS sets `Vary: Origin` with `Set` (overwrites) rather than `Add` (appends)

**Files modified:** `internal/backendsrv/readapi/cors.go`
**Commit:** `b082116`
**Applied fix:** Changed `w.Header().Set("Vary", "Origin")` to `w.Header().Add("Vary", "Origin")` — the defensive idiom that won't clobber a `Vary` value a future handler/middleware (e.g. a compressor's `Accept-Encoding`) might add. Behavior-neutral today (nothing upstream sets `Vary`; the `readapi_test.go` `Vary` assertion still passes via `Header().Get`).
**Verification:** Tier 1 re-read; Tier 2 `go build` + `go test ./internal/backendsrv/readapi/...` (pass).

---

## Skipped Issues

### IN-01: 00004 Down migration uses `ALTER TABLE ... DROP COLUMN`

**File:** `internal/backendsrv/migrations/00004_web_auth.sql:71-86`
**Reason:** skipped — the review itself prescribes **"None required given the forward-only posture"** and explicitly leaves the note "so it's a conscious decision." goose is forward-only here, the migration file already documents "Down is best-effort; forward-only in practice," and rollback is never used in production. Switching to the 12-step table-rebuild pattern would add risk/complexity for a code path that is never exercised, with no behavioral benefit. The conscious decision is recorded: if a real Down is ever needed, rebuild via the 12-step pattern.
**Original issue:** SQLite only gained `DROP COLUMN` in 3.35 (2021) with restrictions (can't drop a column referenced by an index/FK/generated column), so a Down run on an older bundled SQLite or a later-indexed column could fail mid-rollback.

---

## Full-Tree Verification (post-all-fixes)

Per the critical verification directive, the whole project was rebuilt and retested after all fixes. Every command is GREEN:

| # | Command | Result |
|---|---------|--------|
| 1 | `go build ./...` | PASS (rc=0) |
| 2 | `GOOS=linux GOARCH=amd64 go build ./cmd/squirebot-server` | PASS (rc=0) — Linux server binary cross-compiles |
| 3 | `go vet ./...` | PASS (rc=0) — no diagnostics |
| 4 | `go test ./...` | PASS — every package `ok` (webadmin 14.4s, webauth 9.6s, readapi, store, scheduler, ingest, … all pass; no failures) |
| 5 | `npm run check` (from `web/`) | PASS — 432 files, **0 errors, 0 warnings** |
| 6 | `npm run test:unit -- --run` (from `web/`) | PASS — **14 test files, 172 tests passed** (incl. new `eviction.test.ts` + augmented `coin.test.ts`) |
| 7 | `npm run build` (from `web/`) | PASS — `✓ built in 15.75s`, adapter-static wrote the site to `build/` |

No source files are left in a broken or uncommitted state (verified `git status` clean for `internal/`, `cmd/`, `web/`). All 13 fixes are committed across 10 atomic commits (`c51638d`, `43fea96`, `7ead41b`, `9047398`, `2340476`, `5582f9a`, `9868e75`, `1a001d8`, `a39434e`, `b082116`).

**Security semantics preserved:** all fixes keep the fail-CLOSED posture (WR-05 specifically converts a fail-OPEN floor-eviction gap toward fail-CLOSED + loud operator warning), parameterized SQL, hashed opaque sessions, the in-tx officer re-check / TOCTOU close, and owner-floor protection. No new test toolchain was installed (the node-only philosophy was honored); the two blockers gained node-only regression coverage via pure helpers.

---

_Fixed: 2026-05-31T03:53:16Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
