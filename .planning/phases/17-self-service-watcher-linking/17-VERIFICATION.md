---
phase: 17-self-service-watcher-linking
verified: 2026-06-02T00:00:00Z
status: passed
score: 15/15 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: "Initial verification."
---

# Phase 17: Self-Service Watcher Linking Verification Report

**Phase Goal:** Let any guildie self-serve linking their watcher via Discord login — session-scoped mint endpoint (owner derived server-side from the Discord session), Discord-identity↔owner linkage, additive minting, per-token list/revoke, the "Link your watcher" /account page with show-once plaintext / copy-to-clipboard / paste instructions, and removal of the mint-code CLI. The watcher credential stays a static bearer token (NO watcher change).
**Verified:** 2026-06-02
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth (source plan)                                                                                                                         | Status     | Evidence |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | -------- |
| 1   | owner.discord_user_id column (TEXT, FK→web_user) + partial unique index (many NULLs, one owner per Discord id) [17-01]                       | ✓ VERIFIED | `00005_self_service_linking.sql:20-22` — `ALTER TABLE owner ADD COLUMN discord_user_id TEXT REFERENCES web_user(discord_user_id)` + `CREATE UNIQUE INDEX owner_discord_user_id_uidx ON owner(discord_user_id) WHERE discord_user_id IS NOT NULL`. migrate_test.go:293-298 asserts column + index. No inline `UNIQUE` on ADD COLUMN. |
| 2   | guild_code.last_seen column (TEXT, datetime('now')) [17-01]                                                                                  | ✓ VERIFIED | `00005:23` `ALTER TABLE guild_code ADD COLUMN last_seen TEXT`; migrate_test.go:302 asserts it. |
| 3   | Bearer guard returns matched guild_code.id alongside owner_id [17-01]                                                                        | ✓ VERIFIED | `guard.go:45,75` both signatures `(ownerID, codeID int64, ok bool)`; `SELECT id, owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL` (:84); `subtle.ConstantTimeCompare` retained (:97); all miss paths `return 0, 0, false`. guard_test.go:55,68 asserts non-zero codeID on match. |
| 4   | Authenticated ingest stamps guild_code.last_seen best-effort, outside the atomic replace tx [17-01]                                          | ✓ VERIFIED | `handler.go:162-164` `UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?` runs AFTER `bindAndReplace` (:139) + its error block, BEFORE `w.WriteHeader(status)` (:168); failure only `slog.Warn` non-fatal. whoami.go:68 discards codeID with `_`. |
| 5   | Self-mint derives owner server-side from the Discord session (never client-supplied) [17-02]                                                 | ✓ VERIFIED | `account.go:49` `callerID := caller(ctx)` (session); `:54` `ResolveOrCreateOwnerByDiscordTx(ctx, tx, callerID)`; request body carries no owner. |
| 6   | First self-mint adopts a single label-matched owner; zero → fresh owner; 2+ matches or other-stamped → refuse + log [17-02]                  | ✓ VERIFIED | `linking.go:65-155` — step (a) FK lookup, (b) username resolve, (c) `TRIM(label)=TRIM(?) COLLATE NOCASE`: 1 unlinked→adopt (UPDATE), 0→INSERT fresh, 2+→`ErrAmbiguousOwner`+slog.Warn, other-stamped→`ErrAmbiguousOwner`+slog.Warn. |
| 7   | Minting is additive — a new code never revokes existing codes [17-02]                                                                        | ✓ VERIFIED | `mint.go:76` `MintCodeForOwnerTx` only `INSERT INTO guild_code` (no disable of others); crypto/rand 32B, sha256, label NULL, no stdout/slog of code. |
| 8   | Guildie lists only their own active codes (#N, created, last_seen) and revokes any single one, owner-scoped [17-02]                          | ✓ VERIFIED | `linking.go:162` `ListOwnCodes` `WHERE owner_id = ? AND disabled_at IS NULL ORDER BY created_at ASC, id ASC`; `:198` `RevokeOwnCodeTx` `WHERE id = ? AND owner_id = ? AND disabled_at IS NULL`; cross-owner revoke = RowsAffected 0 → (false,nil) no-op. account.go:113 assigns `Ordinal: i+1`. |
| 9   | Eviction owner-floor resolves via owner.discord_user_id when present, falling back to the label bridge [17-02]                               | ✓ VERIFIED | `eviction.go:349` FK-first `SELECT id FROM owner WHERE discord_user_id = ?` (floor); `:372` label-bridge fallback `SELECT username FROM web_user WHERE discord_user_id = ?`. |
| 10  | mint-code CLI subcommand no longer exists; revoke-code retained [17-02 / LINK-06]                                                            | ✓ VERIFIED | main.go: `grep -c "func runMint"`=0, `grep -c case "mint-code"`=0; remaining `mint-code` strings (:72,138,311) are comments documenting removal. `case "revoke-code"` + `runRevoke` (:80,104) retained. main_test.go: `TestRun_MintDispatch` count=0. |
| 11  | Signed-in member sees an "Account" nav entry (not officer-gated) → /account [17-03]                                                          | ✓ VERIFIED | SiteShell.svelte:63 `<a href="/account" class="char-meta-nav">Account</a>` inside `{#if session?.authenticated}` (:54), NOT the `session?.isOfficer` block (:47). account/+page.svelte has no isOfficer/RequireOfficer gate. |
| 12  | Generate mints a code, shows plaintext once in a copy-to-clipboard panel + paste instructions + irreversibility warning [17-03 / LINK-04]   | ✓ VERIFIED | WatcherCodesPanel.svelte:118 `mintOwnCode()`, `mintedPlaintext` state only; :135 `navigator.clipboard.writeText`; :401 `user-select: all` manual fallback; NO `{@html}`, NO `localStorage` (grep both empty). 554 lines (> 80 min). |
| 13  | Reload/revisit never re-reveals plaintext (list shows only #N / created / last-seen) [17-03]                                                 | ✓ VERIFIED | Plaintext lives only in `mintedPlaintext` component state; list re-fetched via `fetchOwnCodes()` (:104,123) returns #N/created/last-seen only. Human browser-smoke step 4 confirmed reload does not re-reveal (17-03-SUMMARY:92-96 APPROVED). |
| 14  | Minting again adds a new #N row without removing existing (additive) [17-03 / LINK-03]                                                       | ✓ VERIFIED | Backend additive (truth 7); UI re-fetches after mint (:123). Human browser-smoke step 5 confirmed both rows present (SUMMARY APPROVED). |
| 15  | Member can revoke any single one of their own codes via confirm-before-commit dialog; row collapses on success [17-03]                       | ✓ VERIFIED | WatcherCodesPanel.svelte:169 `revokeOwnCode(target.id)`, ConfirmDialog reuse (:68,314), optimistic collapse. Human browser-smoke step 6 confirmed scoped revoke + Cancel default-focus (SUMMARY APPROVED). |

**Score:** 15/15 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/backendsrv/migrations/00005_self_service_linking.sql` | FK + partial unique index + last_seen | ✓ VERIFIED | All three DDL statements present; SQLite ADD-UNIQUE landmine avoided. |
| `internal/backendsrv/auth/guard.go` | ResolveToken (ownerID, codeID, ok) | ✓ VERIFIED | Both signatures widened; constant-time compare retained; wired by ingest + whoami. |
| `internal/backendsrv/auth/mint.go` | MintCodeForOwnerTx (tx, no stdout) | ✓ VERIFIED | Sibling added; MintCode unchanged (still used by RestoreHandler). No stdout/slog of code. |
| `internal/backendsrv/store/linking.go` | resolve/list/revoke/stamp + ErrAmbiguousOwner | ✓ VERIFIED | All funcs present; owner-scoped revoke; parameterized; wired by account.go + ingest. |
| `internal/backendsrv/webadmin/account.go` | 3 handlers + mapAccountErr | ✓ VERIFIED | Mint/List/Revoke handlers; session-derived owner; ErrAmbiguousOwner→409; wired in main.go. |
| `internal/backendsrv/webadmin/eviction.go` | FK-preferred owner-floor | ✓ VERIFIED | FK-first lookup + label fallback. |
| `cmd/squirebot-server/main.go` | 3 RequireSession routes; mint-code CLI removed | ✓ VERIFIED | Routes :312-314 under RequireSession; runMint + dispatch arm gone; revoke-code kept. |
| `web/src/lib/api.ts` | fetchOwnCodes/mintOwnCode/revokeOwnCode + OwnCode | ✓ VERIFIED | :542-562, reuse getJSON/postJSON. |
| `web/src/routes/account/+page.svelte` | /account member page w/ WatcherCodesPanel | ✓ VERIFIED | Renders panel; no officer gate. |
| `web/src/lib/components/WatcherCodesPanel.svelte` | generate→show-once→list→confirm-revoke | ✓ VERIFIED | 554 lines; clipboard + fallback; no @html/localStorage. |
| `web/src/lib/components/SiteShell.svelte` | member-visible Account nav | ✓ VERIFIED | href="/account" under authenticated block. |
| `web/src/lib/components/StateBlock.svelte` | no-codes kind | ✓ VERIFIED | kind union :25, branch :93. |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| guard.go | guild_code.id | `SELECT id, owner_id, token_hash` | ✓ WIRED |
| handler.go | guild_code.last_seen | post-bindAndReplace UPDATE | ✓ WIRED |
| account.go | store.ResolveOrCreateOwnerByDiscordTx | withTx→resolve→MintCodeForOwnerTx→AppendAuditTx | ✓ WIRED |
| linking.go | guild_code (caller-scoped revoke) | `WHERE id = ? AND owner_id = ?` | ✓ WIRED |
| eviction.go | owner.discord_user_id | `WHERE discord_user_id = ?` (FK-first) | ✓ WIRED |
| main.go | account endpoints | RequireSession mux.Handle account/codes | ✓ WIRED |
| WatcherCodesPanel.svelte | /api/v1/account/codes | mint/fetch/revokeOwnCode in $lib/api | ✓ WIRED |
| SiteShell.svelte | /account | member-visible nav under authenticated | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Full Go tree compiles | `go build ./...` | exit 0 | ✓ PASS |
| Backend + CLI test suites | `go test ./cmd/... ./internal/backendsrv/...` | all ok (auth, store, webadmin, migrations, ingest, cmd) | ✓ PASS |
| Web typecheck (regression) | `cd web && npm run check` | 443 FILES 0 ERRORS 0 WARNINGS | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
| ----------- | ----------- | ------ | -------- |
| LINK-01 | 17-02, 17-03 | ✓ SATISFIED | MintOwnCodeHandler, session-derived owner, /account generate trigger (truths 5,12). |
| LINK-02 | 17-01, 17-02 | ✓ SATISFIED | owner.discord_user_id FK + resolve-or-create stamp + eviction FK rewire (truths 1,6,9). |
| LINK-03 | 17-02 | ✓ SATISFIED | Additive mint, INSERT-only (truths 7,14). |
| LINK-04 | 17-03 | ✓ SATISFIED | Show-once panel, plaintext in state only, never re-revealed (truths 12,13); browser-smoke approved. |
| LINK-05 | 17-02, 17-03 | ✓ SATISFIED | Owner-scoped list + per-code revoke + last_seen (truths 8,15); browser-smoke approved. |
| LINK-06 | 17-02 | ✓ SATISFIED | mint-code CLI removed, revoke-code retained (truth 10). |

All 6 LINK-0x IDs declared in plan frontmatter are present in REQUIREMENTS.md and accounted for. No orphaned Phase-17 requirements. (WATCH-12/13/14 are mapped to Phase 18, not this phase.)

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `internal/backendsrv/ingest/handler.go` | 162-164 | Inline `UPDATE guild_code SET last_seen` instead of calling `store.StampCodeLastSeen` (WR-02) | ℹ️ Info | `StampCodeLastSeen` is dead in production (exercised only by linking_test.go); the inline UPDATE is functionally correct — parameterized, scoped to the matched codeID, non-fatal on error. Maintainability/coverage observation only; does NOT fail the goal (confirmed per brief). |
| `web/src/lib/components/WatcherCodesPanel.svelte` | ~35-39 | `formatLastSeen` "0 years ago" for ~360-364-day-old last_seen (WR-03) | ℹ️ Info | Low-impact display string; watcher last_seen rarely approaches 1 year. Advisory. |
| `web/src/lib/components/WatcherCodesPanel.svelte` | 170-178 | Stale ordinal after optimistic revoke filter (WR-04) | ℹ️ Info | UI-consistency only; revoke is server-scoped by stable code id. Advisory. |
| `internal/backendsrv/migrations/00005...sql` | 25-27 | Down-migration drops index but not columns (WR-01) | ℹ️ Info | Project is forward-only in practice; down path never run in prod. Advisory. |

No blocker or warning-severity anti-patterns affecting goal achievement. The four review warnings (WR-01..04) are all advisory/quality, consistent with 17-REVIEW.md (0 critical).

### Human Verification Required

None outstanding. The single non-autonomous gate — 17-03 Task 3 browser-smoke of /account (clipboard, show-once reveal, reload-never-re-reveals, additive mint, scoped revoke, member-only nav, Heavy-theme contrast) — was performed against the LIVE deployed site (api.squirebot.quest schema v5, squirebot.quest /account) and the human replied "approved" with all 7 steps passing (17-03-SUMMARY §"Task 3: Browser-Smoke Checkpoint — PASSED"). This gate is closed, not pending.

### Gaps Summary

No gaps. All 15 must-have truths across the three plans are verified against the codebase: schema columns + index exist and are asserted by migrate_test; the bearer guard threads the code id with the constant-time compare intact; the ingest path stamps last_seen best-effort outside the replace tx; the session-derived resolve-or-create-owner algorithm (adopt/new/refuse) is implemented with the mis-adoption guard; minting is INSERT-only (additive); list/revoke are strictly owner-scoped (IDOR closed); the eviction floor prefers the FK; the mint-code CLI is gone while revoke-code is retained; and the /account page delivers the member-visible nav, show-once copy-to-clipboard panel, owner-scoped list, and confirm-before-commit revoke with no plaintext persistence and no `{@html}`. Go build + full backend/CLI test suites pass; web typecheck is clean (0 errors / 0 warnings — doubling as the cross-phase regression check). The feature is deployed live and the DOM-blind frontend surface was human-approved.

---

_Verified: 2026-06-02_
_Verifier: Claude (gsd-verifier)_
