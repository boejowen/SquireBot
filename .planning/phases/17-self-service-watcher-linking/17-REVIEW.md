---
phase: 17-self-service-watcher-linking
reviewed: 2026-06-02T00:00:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - cmd/squirebot-server/main_test.go
  - internal/backendsrv/auth/guard.go
  - internal/backendsrv/auth/guard_test.go
  - internal/backendsrv/auth/mint.go
  - internal/backendsrv/ingest/handler.go
  - internal/backendsrv/ingest/whoami.go
  - internal/backendsrv/migrations/00005_self_service_linking.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/linking.go
  - internal/backendsrv/store/linking_test.go
  - internal/backendsrv/webadmin/account.go
  - internal/backendsrv/webadmin/account_test.go
  - internal/backendsrv/webadmin/eviction.go
  - web/src/lib/__tests__/api.test.ts
  - web/src/lib/api.ts
  - web/src/lib/components/SiteShell.svelte
  - web/src/lib/components/StateBlock.svelte
  - web/src/lib/components/WatcherCodesPanel.svelte
  - web/src/routes/account/+page.svelte
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 17: Code Review Report

**Reviewed:** 2026-06-02
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

Phase 17 adds self-service watcher-code minting/listing/revoking. I focused hard on the
security-sensitive surfaces called out in the brief: token/secret handling, IDOR
owner-scoping, the resolve-or-create-owner adoption guard, SQL parameterization, the
best-effort `last_seen` stamp, and Svelte XSS.

The core security posture holds up under adversarial reading:

- **Token handling is correct.** Plaintext is generated from `crypto/rand` (32 bytes),
  hashed with SHA-256, and only the hash is persisted. `MintCodeForOwnerTx` does NOT
  print to stdout or slog (unlike the ops `MintCode`), returns the plaintext only via the
  HTTP body, and stores `label` NULL. The audit detail carries `owner_id`/`code_id` only.
- **IDOR is closed.** `RevokeOwnCodeTx` is `WHERE id=? AND owner_id=? AND disabled_at IS NULL`;
  a cross-owner revoke is `RowsAffected=0` → silent no-op (no existence leak). `ListOwnCodes`
  is owner-scoped. Both tested directly.
- **Mis-adoption guard is correct.** `ResolveOrCreateOwnerByDiscordTx` refuses on a
  label-match stamped by a different Discord id and on 2+ unlinked matches, with
  `slog.Warn` and no mint.
- **SQL is fully parameterized.** Every query uses `?` placeholders; no interpolation of
  user-supplied values. The migration uses the documented partial-unique-index pattern to
  dodge the SQLite ADD-UNIQUE landmine.
- **No XSS.** The show-once panel and `StateBlock` render the plaintext, ordinal, dates,
  and error reasons via `{}` auto-escape; no `{@html}` anywhere.

The defects below are correctness/quality issues, not breaches. The most important are a
migration down-migration ordering bug (WR-01), a stale "no inline SQL" invariant violated
by the new `last_seen` stamp that also strands `StampCodeLastSeen` as dead code (WR-02),
and a relative-time gap bug in `formatLastSeen` (WR-03).

## Warnings

### WR-01: Down-migration drops the index before (failing to) drop its column — leaves schema inconsistent

**File:** `internal/backendsrv/migrations/00005_self_service_linking.sql:25-27`
**Issue:** The `+goose Down` block runs `DROP INDEX owner_discord_user_id_uidx;` but never
drops the `owner.discord_user_id` / `guild_code.last_seen` columns (the comment concedes
"column drops are best-effort; forward-only in practice"). A `goose down` therefore leaves
`owner.discord_user_id` present but UN-indexed — i.e. the one-owner-per-Discord-id
uniqueness invariant silently disappears while the column keeps accepting writes. If a
re-`up` is then attempted, `CREATE UNIQUE INDEX` can now fail because duplicate
`discord_user_id` rows may have been inserted in the windowless interval. The down path is
neither a clean revert nor a clean no-op. Since the project is forward-only in practice,
the safer down is to make it an explicit no-op (drop nothing) rather than a partial revert
that removes only the safety index.
**Fix:**
```sql
-- +goose Down
-- Forward-only in practice (mirrors 00004): a partial revert that drops ONLY the
-- unique index would leave owner.discord_user_id writable WITHOUT its uniqueness
-- guard. Make the down an explicit no-op instead.
SELECT 1;
```

### WR-02: Ingest `last_seen` stamp inlines SQL, violating the handler's "no inline SQL" invariant and orphaning `StampCodeLastSeen`

**File:** `internal/backendsrv/ingest/handler.go:162-165` (and `internal/backendsrv/store/linking.go:218`)
**Issue:** `handler.go`'s package header states the handler "authors NO inline
DELETE/INSERT/character SQL" and touches SQL "only through 11-03's exported Tx functions"
so "there is no second, test-uncovered SQL path." Step [5] breaks that: it inlines
`UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?` directly on `h.db`. Phase
17 added `store.StampCodeLastSeen` for exactly this purpose (its doc comment even says it
"is the helper behind the ingest path's best-effort last_seen stamp"), but the handler
does not call it. Result: (a) the documented single-SQL-path invariant is now false; (b)
`StampCodeLastSeen` is dead in production — exercised only by `linking_test.go`, so the
production stamp has no store-level test coverage; (c) two copies of the same UPDATE can
drift. Not a security bug (the UPDATE is parameterized, scoped to the matched `codeID`,
non-fatal on error), but a maintainability/coverage regression.
**Fix:**
```go
// [5] Best-effort last_seen stamp (D-07 / LINK-05) — via the store helper so the
// ingest path keeps its "no inline SQL" discipline and StampCodeLastSeen's test
// coverage is load-bearing.
if serr := store.StampCodeLastSeen(r.Context(), h.db, codeID); serr != nil {
    slog.Warn("stamp last_seen failed (non-fatal)", "code_id", codeID, "err", serr)
}
```

### WR-03: `formatLastSeen` reports "0 years ago" for last_seen ~360-364 days old

**File:** `web/src/lib/components/StateBlock.svelte` → actually `WatcherCodesPanel.svelte:35-39`
**Issue:** The cascade is `day<30` → days, `mon = floor(day/30)`, `mon<12` → months, else
`yr = floor(day/365)`. The month tier tops out at 12 months = 360 days, but the year tier
divides by 365. For `day` in [360, 364], `mon = 12` (fails `mon < 12`) and
`yr = floor(day/365) = 0`, producing "last used 0 years ago". A 360-day-old watcher reads
as "0 years ago". Low impact (watcher last_seen rarely hits ~1 year), but it is a wrong
string a user can see.
**Fix:** Use a consistent 365-day boundary for the month tier, or floor the year at 1:
```ts
const day = Math.floor(hr / 24);
if (day < 30) return `last used ${day} day${day === 1 ? '' : 's'} ago`;
if (day < 365) {
    const mon = Math.max(1, Math.floor(day / 30));
    return `last used ${mon} month${mon === 1 ? '' : 's'} ago`;
}
const yr = Math.floor(day / 365);
return `last used ${yr} year${yr === 1 ? '' : 's'} ago`;
```

### WR-04: Revoke success message can show a stale ordinal after an optimistic list mutation

**File:** `web/src/lib/components/WatcherCodesPanel.svelte:170-178`
**Issue:** Ordinals are server-assigned 1-based indices over the active set
(`account.go:113`, `i + 1`). After revoking code `#2` of `[#1, #2, #3]`, `doRevoke`
optimistically filters the revoked row out of `codes` but does NOT re-derive the remaining
ordinals, and it does not re-fetch the list (unlike `generate`, which calls
`fetchOwnCodes()` after the mint). The surviving rows keep their old `ordinal` values
(`#1`, `#3`), while the next server `GET` would renumber them `#1`, `#2`. So the displayed
ordinals diverge from the server's until the next full reload, and the success copy
("Code #3 revoked…") can reference an ordinal that no longer matches what the user sees.
This is a UI-consistency bug, not a security issue (revoke is server-scoped by code `id`,
which is stable). Re-fetching after a successful revoke (mirroring `generate`) keeps the
ordinals authoritative.
**Fix:** After a successful revoke, re-load from the server instead of (or in addition to)
the optimistic filter:
```ts
if (revoked) {
    revokeSuccessMsg = `Code #${target.ordinal} revoked. That watcher will stop uploading on its next attempt.`;
    codes = await fetchOwnCodes(); // re-derive authoritative #N (mirror generate())
}
```

## Info

### IN-01: `reason()` only special-cases `ambiguous_owner`; a 409 with no recognized code falls through to a generic string but routes nowhere

**File:** `web/src/lib/components/WatcherCodesPanel.svelte:124-130, 195-216`
**Issue:** `mintOwnCode` can 409 with `ambiguous_owner` (handled) — but `route()` only
re-routes `Unauthenticated`/`officers-only`. A 409 `Forbidden`-less `ApiError` (status 409)
is not caught by `route()` (returns false), so it correctly falls to the inline
`mintErrorMsg` with `reason()` → "The server rejected the request." That is acceptable, but
the inline copy claims "No code was created — try again," which is true for `ambiguous_owner`
(the resolve refused pre-mint) yet would be misleading for any future 500 that occurs
*after* a partial side effect. Currently safe because mint is atomic in one `withTx`; flag
as a latent coupling between copy and server atomicity.
**Fix:** Keep the atomic-mint invariant (already true) and add a brief comment in `generate`
noting the "No code was created" copy depends on the server's all-or-nothing mint tx.

### IN-02: `ownerIDForCaller` duplicates the step-(a) lookup in `ResolveOrCreateOwnerByDiscordTx`

**File:** `internal/backendsrv/webadmin/account.go:193-205` vs `internal/backendsrv/store/linking.go:67-70`
**Issue:** `ownerIDForCaller` (handler-local, on `*sql.DB`) runs the identical
`SELECT id FROM owner WHERE discord_user_id = ?` that `ResolveOrCreateOwnerByDiscordTx`
step (a) runs (on `*sql.Tx`). Two copies of the owner-by-discord-id resolution. They cannot
drift in behavior today, but a future change to the linkage key (e.g. normalizing the id)
must be made in both places. Consider a single shared `store.OwnerIDByDiscordID` the handler
and the resolve func both call.
**Fix:** Extract the lookup into one store function; have both call sites use it.

### IN-03: `ListOwnCodesHandler`/`RevokeOwnCodeHandler` re-query the owner outside any tx, racing a concurrent mint adoption

**File:** `internal/backendsrv/webadmin/account.go:98, 152`
**Issue:** List and revoke resolve the owner via `ownerIDForCaller` on the bare `*sql.DB`
(no tx), then revoke runs in its own `withTx`. Because the store is single-writer
(`SetMaxOpenConns(1)`, `_txlock=immediate`), there is no true concurrency hazard today —
writes serialize. Noted only as defense-in-depth: if the connection cap is ever raised, the
read-then-write gap (resolve owner, then revoke) is non-atomic. Currently benign.
**Fix:** None required while `maxconns=1`; if that changes, fold the owner resolve into the
revoke tx.

### IN-04: Migration comment says `last_seen` matches "00001:54-55" guild_code cols, but the file is the source of truth — verify on schema evolution

**File:** `internal/backendsrv/migrations/00005_self_service_linking.sql:10-13, 23`
**Issue:** The column-type rationale references specific line numbers in `00001_init.sql`
and `00004_web_auth.sql`. Line-number references rot as files change. The actual types are
correct (`discord_user_id TEXT REFERENCES web_user(discord_user_id)` matches the TEXT PK;
`last_seen TEXT` matches the sibling `datetime('now')` columns). Purely a comment-durability
nit.
**Fix:** Reference column names, not line numbers, in migration rationale comments.

### IN-05: `copyCode` swallows clipboard errors silently with an empty `catch` (intentional, but undocumented at the catch site)

**File:** `web/src/lib/components/WatcherCodesPanel.svelte:140-144`
**Issue:** The `catch {}` on the clipboard write is intentional (the token stays
`user-select:all` for manual copy, and the comment explains it) — so this is NOT the
empty-catch anti-pattern. Calling it out only to confirm it was reviewed: the failure mode
(clipboard denied) degrades gracefully to manual select-copy, and `copied` is reset to
false. No fix needed; documented here so the empty catch is not later "fixed" into a
blocking error.
**Fix:** None — verified correct. Optionally `console.debug` the denial for support triage.

---

_Reviewed: 2026-06-02_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
