---
phase: 17-self-service-watcher-linking
plan: 02
subsystem: backend (Go — self-service mint/list/revoke endpoints, resolve-or-create owner, FK eviction floor, CLI removal)
tags: [auth, mint, webadmin, store, resolve-or-create, idor, owner-floor, fk, cli-removal, link-01, link-02, link-03, link-05, link-06]
requires:
  - "17-01: owner.discord_user_id FK→web_user + partial unique index + guild_code.last_seen (00005)"
  - "17-01: bearer guard returning (ownerID, codeID, ok)"
  - "auth.MintCode token-gen shape (sibling source); webadmin officers.go/audit.go helpers (caller/withTx/AppendAuditTx/writeJSON)"
provides:
  - "store.ResolveOrCreateOwnerByDiscordTx (D-03/D-04 adopt/new/refuse) + ErrAmbiguousOwner"
  - "store.ListOwnCodes / RevokeOwnCodeTx (owner-scoped) / StampCodeLastSeen"
  - "auth.MintCodeForOwnerTx (tx-composable, session-owner, returns plaintext, NO stdout)"
  - "webadmin.MintOwnCodeHandler / ListOwnCodesHandler / RevokeOwnCodeHandler + mapAccountErr"
  - "three login-only routes: POST/GET /api/v1/account/codes, POST /api/v1/account/codes/revoke"
  - "eviction owner-floor FK-first resolution (D-05)"
affects:
  - "Wave 3 (frontend /account page) — consumes the three /account/codes endpoints"
  - "ops: mint-code CLI is GONE (LINK-06); revoke-code retained as the only CLI code path"
  - "deploy: maintainer should self-mint once post-deploy so their owner-floor protection becomes FK-backed"
tech-stack:
  added: []
  patterns:
    - "resolve-or-create owner inside ONE BEGIN IMMEDIATE tx; partial unique index is the racing-double-stamp backstop"
    - "owner-scoped revoke (WHERE id=? AND owner_id=?) — cross-owner is a silent RowsAffected=0 no-op (no IDOR, no existence leak)"
    - "sibling tx-function (MintCodeForOwnerTx) rather than changing the live MintCode signature RestoreHandler depends on"
    - "FK-first floor resolution with the legacy label bridge retained as the unlinked-owner fallback"
key-files:
  created:
    - "internal/backendsrv/store/linking.go"
    - "internal/backendsrv/store/linking_test.go"
    - "internal/backendsrv/webadmin/account.go"
    - "internal/backendsrv/webadmin/account_test.go"
  modified:
    - "internal/backendsrv/auth/mint.go"
    - "internal/backendsrv/webadmin/eviction.go"
    - "cmd/squirebot-server/main.go"
    - "cmd/squirebot-server/main_test.go"
decisions:
  - "MintCodeForOwnerTx is a SIBLING of MintCode (signature unchanged) — RestoreHandler still calls auth.MintCode(db, label) on a bare *sql.DB with its stdout disclosure (WR-01/WR-02)"
  - "self-minted guild_code.label = NULL (D-06 — #N/created/last_seen identify a code; the free-text owner label has no role)"
  - "RevokeOwnCodeTx is owner-scoped and NOT auth.RevokeCode (the ops CLI is intentionally owner-unscoped — RESEARCH Pitfall 3)"
  - "#N ordinal is the 1-based handler-assigned index over the created_at-ordered active set (not a stored column — RESEARCH A3)"
  - "eviction floor: FK lookup is authoritative when present (no fail-open); label bridge only when the floor owner is unlinked"
metrics:
  duration: ~10 min
  completed: 2026-06-01
  tasks: 3
  files: 8
  commits: 4
---

# Phase 17 Plan 02: Backend Self-Service Linking Summary

The entire server side of self-service watcher linking: a session-derived
resolve-or-create-owner algorithm, three login-only HTTP handlers (mint / list-own
/ revoke-own) plus the tx-composable `MintCodeForOwnerTx`, the eviction owner-floor
rewired to prefer the new `owner.discord_user_id` FK (D-05), and the deletion of the
v1 `mint-code` CLI (LINK-06). The owner is ALWAYS derived from the Discord session
(D-02); the request body never carries an owner. Minting is additive; list/revoke
are strictly caller-owner-scoped (no IDOR); ambiguity refuses with 409 and a loud
`slog.Warn`; the plaintext crosses to the page once and is never logged.

## What Was Built

### Task 1 (TDD) — store funcs + `MintCodeForOwnerTx`
- **`auth.MintCodeForOwnerTx(ctx, tx, ownerID) (string, error)`** — a sibling of
  `MintCode` (which is UNTOUCHED — `RestoreHandler` still calls it). Copies the
  token-gen shape verbatim (32B `crypto/rand` → `base64.RawURLEncoding` → `sha256`),
  INSERTs the HASH on the caller's `*sql.Tx` with `label = NULL` (D-06), and returns
  the plaintext for the HTTP body ONLY — no `fmt.Printf`, no `slog` of the code (V6/V7).
- **`store/linking.go`** — `ErrAmbiguousOwner` sentinel +:
  - `ResolveOrCreateOwnerByDiscordTx` — (a) already-linked owner returned (D-03);
    (b) resolve username via `web_user`; (c) scan `TRIM(label) COLLATE NOCASE`
    matches: exactly-one-unlinked → stamp & return (D-04 adopt), zero → INSERT fresh
    owner (D-04 new), 2+ unlinked OR a match stamped by a different discord id →
    `slog.Warn` + `ErrAmbiguousOwner` (D-04 refuse / mis-adoption guard). All on the tx.
  - `ListOwnCodes` — owner-scoped active codes, `ORDER BY created_at ASC, id ASC`,
    non-nil slice (so JSON `[]`).
  - `RevokeOwnCodeTx` — `UPDATE … WHERE id=? AND owner_id=? AND disabled_at IS NULL`;
    RowsAffected>0 → revoked. Cross-owner / already-revoked → `(false, nil)` no-op.
  - `StampCodeLastSeen` — best-effort `UPDATE guild_code SET last_seen=datetime('now')`.
- **`store/linking_test.go`** — covers already-linked, adopt (with case/whitespace
  drift), create-fresh, multiple-match-ambiguous, stamped-by-other-ambiguous,
  owner-scoped list + non-nil-empty, cross-owner revoke no-op (IDOR) + own revoke +
  idempotent re-revoke, and the last_seen stamp.

### Task 2 — the three login-only handlers + `mapAccountErr`
- **`webadmin/account.go`** — `MintOwnCodeHandler` (POST; resolve-or-create →
  `MintCodeForOwnerTx` → audit `code_mint` with owner_id ONLY → `{code}` body once),
  `ListOwnCodesHandler` (GET; owner-by-FK → `ListOwnCodes` → `{id, ordinal, created_at,
  last_seen}` with sequential `#N`, `[]` for never-minted), `RevokeOwnCodeHandler`
  (POST `{id}`; owner-by-FK → `RevokeOwnCodeTx` → audit `code_revoke` only on a real
  revoke). `mapAccountErr`: `ErrAmbiguousOwner` → 409, default → 500. `ownerIDForCaller`
  resolves owner by `discord_user_id` (no create — that only happens on mint).
- **`webadmin/account_test.go`** — mint returns plaintext + hash-only at rest +
  audited + additive second mint; ambiguous resolve → 409 (no mint); list owner-scoped
  with sequential ordinals + `[]` empty + `last_seen` null; cross-owner revoke
  `revoked:false` no-op (not audited) + own revoke `revoked:true` (audited).

### Task 3 — routes, eviction FK rewire, CLI removal
- **`main.go`** — three `RequireSession`-gated routes registered alongside the
  char-meta login-only block. `mint-code` dispatch arm + `runMint` deleted; usage
  comment + run() doc updated; `revoke-code` + `splitFlagsAndPositionals` retained.
- **`eviction.go`** `callerMayNotEvictFloor` — FK-first: `SELECT id FROM owner WHERE
  discord_user_id = ?` (the floor's id); a hit is authoritative (closes WR-05 for
  linked owners — the LINK-02 payoff); falls back to the legacy `TRIM(label) COLLATE
  NOCASE` bridge (with its inert-protection `slog.Warn`s) only when the floor owner
  is unlinked.
- **`main_test.go`** — `TestRun_MintDispatch{,_MissingOwner}` deleted;
  `TestRun_RevokeDispatch` reseeds a code directly via the store (new `seedActiveCode`
  helper) instead of the gone `mint-code` CLI; `TestWriteRoutes_Gates` extended with
  account-route anon→401 + member→admitted (proves `RequireSession`, not `RequireOfficer`).

## Verification

- `go build ./...` (repo root) — PASS (routes wired, mint-code removed cleanly, `auth`
  import still used by `runRevoke`/`runServe`).
- `go test ./auth/... ./store/...` — PASS (Task 1: every resolve-or-create branch,
  IDOR no-op, stamp).
- `go test ./webadmin/...` — PASS (Task 2: hash-only mint, 409 ambiguous, owner-scoped
  list/`[]`, cross-owner-revoke no-op).
- `go test ./cmd/... ./internal/backendsrv/...` — PASS (full backend + cmd suite green;
  no dangling reference to runMint; account routes gated correctly).
- All Task 1/2/3 acceptance greps pass (signatures, sentinels, owner-scoped SQL, route
  patterns, `runMint`/`mint-code` count 0, `revoke-code` retained, mint tests removed).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Duplicate test helpers in webadmin package**
- **Found during:** Task 2 (first `go test ./webadmin/...` build).
- **Issue:** My `account_test.go` declared `activeCodeCount` and `itoa`, which already
  exist in `eviction_test.go` (same package) — redeclaration build error. Also
  `account.go` referenced `context` without importing it.
- **Fix:** Removed the two duplicate helpers from `account_test.go` (reusing the
  eviction_test.go ones) and added the `context` import to `account.go`.
- **Files modified:** internal/backendsrv/webadmin/account.go, account_test.go
- **Commit:** 573ac9c

**2. [Rule 1 - Bug] FK violation in account_test.go seed**
- **Found during:** Task 2 (second `go test ./webadmin/...` run).
- **Issue:** `seedOwner(..., "disc-other")` stamped an owner with a `discord_user_id`
  that had no `web_user` row → `FOREIGN KEY constraint failed`. (The FK from 00005 is
  enforced at runtime — `_pragma=foreign_keys(ON)`.)
- **Fix:** Seed `web_user("disc-other", …)` before the "other" owner in both affected
  tests.
- **Files modified:** internal/backendsrv/webadmin/account_test.go
- **Commit:** 573ac9c

**3. [Rule 3 - Blocking] `TestRun_RevokeDispatch` depended on the deleted mint-code CLI**
- **Found during:** Task 3 (deleting the mint-code arm).
- **Issue:** The retained revoke-code dispatch test seeded its code via
  `run(["mint-code", …])`, which Task 3 removes.
- **Fix:** Added a `seedActiveCode` test helper that inserts an owner + active
  guild_code directly via the store (with a `migrations` import), so the revoke-code
  ops-backstop test still proves the dispatch without the removed CLI.
- **Files modified:** cmd/squirebot-server/main_test.go
- **Commit:** fc2b3db

### Added (beyond the plan's explicit task list, in scope)

- Extended `TestWriteRoutes_Gates` with the two `/api/v1/account/codes` gate cases
  (anon→401, member→admitted) — the route-level proof that the new endpoints are
  `RequireSession` not `RequireOfficer` (mirrors the existing char-meta/coin gate
  cases; the handler layer is gate-agnostic so only this catches a future
  RequireOfficer swap). Commit fc2b3db.

## TDD Gate Compliance

- RED: commit `2846b68` — `test(17-02): add failing tests …` (build failed:
  undefined `ResolveOrCreateOwnerByDiscordTx`/`ErrAmbiguousOwner`/`ListOwnCodes`/etc.).
- GREEN: commit `93fb723` — `feat(17-02): add MintCodeForOwnerTx + … store funcs`
  (`go test ./auth/... ./store/...` PASS).
- REFACTOR: none required — the GREEN implementation was already clean.

## Notes for Downstream Plans (Wave 3 frontend)

- The three endpoints return: mint → `{"code": "<plaintext>"}` (shown once — never
  re-fetched, never `localStorage`); list → `[{id, ordinal, created_at, last_seen}]`
  (`last_seen` is JSON `null` until the code's watcher next uploads); revoke (POST
  `{id}`) → `{"revoked": <bool>}` (false = not the caller's / already revoked — treat
  as a successful no-op, do not surface as an error).
- Ambiguous resolve surfaces as HTTP 409 `{"error":"ambiguous_owner"}` — the frontend
  should route this to a clear "we couldn't match your guildie data; ask an officer"
  message (it means a label collision, not a transient failure).
- `web/` vitest is node-only/DOM-blind (MEMORY): browser-smoke the show-once panel +
  clipboard copy; do not call the page verified on unit tests alone.

## Deploy Note (carry into the Phase 17 ship checklist)

After redeploying the backend (goose applies nothing new — 00005 shipped in Wave 1),
the maintainer should visit `/account` and self-mint ONCE. That stamps their
`owner.discord_user_id`, flipping their owner-floor protection from the legacy label
bridge to the FK path (D-05). Until then floor protection stays on the hardened label
bridge (no regression — that is the current live behavior).

## Self-Check: PASSED

- FOUND: internal/backendsrv/store/linking.go
- FOUND: internal/backendsrv/store/linking_test.go
- FOUND: internal/backendsrv/webadmin/account.go
- FOUND: internal/backendsrv/webadmin/account_test.go
- FOUND: .planning/phases/17-self-service-watcher-linking/17-02-SUMMARY.md
- FOUND commit 2846b68 (RED), 93fb723 (GREEN/Task 1), 573ac9c (Task 2), fc2b3db (Task 3)
