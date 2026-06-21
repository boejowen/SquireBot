---
phase: 34-wishlist-rework-per-character-per-slot-upgrades
plan: 02
subsystem: backend
tags: [wishlist, wantmatch, ec-monitor, owner-scoped, idor, read-api, routing, dm-target-invariant, clean-break]

# Dependency graph
requires:
  - phase: 34-01
    provides: "migration 00014 (wishlist_item + alert_log FK rebuilt + wantlist_item DROPPED), store/wishlist.go (AddWishlistTx/RemoveOwnWishlistTx/SetPingedTx + ErrDuplicateWishlist), compute.WishlistFor + WishlistView"
  - phase: 19-wantlist / 20-notify / 28-character-tagged-wantlist
    provides: "the wantlist.go/wantmatch/notify spine cloned + repointed here (owner-scoped CRUD, the 2067 dup idiom, IsCharAssignedToTx, the DM-target-is-owner invariant)"
provides:
  - "wantmatch.ForItem/ForName repointed FROM wishlist_item (pinged=1 gate, INNER JOIN character, no note); store.ECPollSet repointed to wishlist_item"
  - "webadmin/wishlist.go: owner-scoped AddWishlistHandler (in-tx IsCharAssignedToTx → 403) / RemoveOwnWishlistHandler / SetWishlistPingHandler (silent IDOR no-op); mapWishlistErr + validWishlist (21-slot enum)"
  - "readapi/wishlist.go: GET /api/v1/wishlist/{char} over compute.WishlistFor (session uid + ?-bound char)"
  - "main.go: 4 wishlist routes registered (RequireSession), the 5 retired /api/v1/wantlist* routes removed"
  - "the full retired-wantlist-surface test repair (ec/notify/store/webadmin) — go test ./... GREEN again"
affects: [34-03 (web tab consumes WishlistView + the 4 routes), wantlist retirement complete]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Repoint a shared matcher seam in place: keep the func names (ForItem/ForName) + the Hit.WantID FK + the DM-target invariant; change only the SELECT (table, gate polarity, join kind, dropped column)"
    - "Inverse-polarity gate: pinged=1 is the alert-eligible state (the inverse of the wantlist's muted=0, Pitfall 8)"
    - "Pre-filter (ECPollSet) does NOT apply the per-target ping gate — it defines WHAT to poll; the ping gate is applied downstream at the wantmatch seam so a re-pinged target re-alerts without re-polling"
    - "Retire a dropped-table surface by deleting its TEST files (it tested the dropped table) while KEEPING the production handler/store files (unreferenced exported funcs still compile — SQL strings are runtime-only)"

key-files:
  created:
    - internal/backendsrv/webadmin/wishlist.go
    - internal/backendsrv/webadmin/wishlist_test.go
    - internal/backendsrv/readapi/wishlist.go
  modified:
    - internal/backendsrv/wantmatch/match.go
    - internal/backendsrv/wantmatch/match_test.go
    - internal/backendsrv/ec/embed.go
    - internal/backendsrv/ec/ec_test.go
    - internal/backendsrv/store/eccursor.go
    - internal/backendsrv/store/eccursor_test.go
    - internal/backendsrv/store/alertlog_test.go
    - internal/backendsrv/notify/dm_test.go
    - cmd/squirebot-server/main.go
    - cmd/squirebot-server/main_test.go
  deleted:
    - internal/backendsrv/webadmin/wantlist_test.go
    - internal/backendsrv/store/wantlist_test.go

key-decisions:
  - "ECPollSet (store/eccursor.go) was repointed to wishlist_item — a Rule-3 in-scope discovery (production code that 500s on the dropped table; the EC poll spine that feeds wantmatch.ForItem). It does NOT filter pinged (defines what to POLL, not whether to alert)."
  - "ec/embed.go whyWanted now always returns the fixed 'on your wishlist' fallback (the per-slot wishlist has no free-text note; the existing nil-guard made dropping Hit.Note safe — verified at the call site)"
  - "Retired the wantlist TEST surface (deleted webadmin/wantlist_test.go + store/wantlist_test.go) but KEPT the production wantlist.go/store/wantlist.go (plan Task 3 NOTE; store/wantlist.go still declares sqliteConstraintUnique + boolToInt that wishlist.go/assignment.go/guildchannel.go REUSE — deleting it would break the build)"
  - "validWishlist enforces the 21 canonical worn slots as a LITERAL map in webadmin (the validWant precedent) — NOT a compute import, avoiding a webadmin→compute edge; Charm/Power Source correctly rejected (post-Velious, D-04)"

requirements-completed: [WISH-01, WISH-05, WISH-07]

# Metrics
duration: 16min
completed: 2026-06-19
---

# Phase 34 Plan 02: Wishlist Matcher Repoint + Owner-Scoped Write/Read API Summary

**The EC matcher repointed to the new `wishlist_item` table (one SELECT each in `wantmatch.ForItem`/`ForName` + the `ECPollSet` pre-filter), the owner-scoped write API (`webadmin/wishlist.go` add/remove/ping — the `wantlist.go` clone, with the in-tx `IsCharAssignedToTx` 403 + 409 dup + silent IDOR no-op), the per-character read route (`GET /api/v1/wishlist/{char}` over `compute.WishlistFor`), and the full clean-break test repair so `go test ./...` is GREEN again. The DM-target-is-owner invariant (T-34-08) is preserved + regression-tested. NO web/watcher change; NO new migration.**

## Performance

- **Duration:** ~16 min
- **Started:** 2026-06-19T03:46:51Z
- **Completed:** 2026-06-19T04:02:18Z
- **Tasks:** 3
- **Files:** 15 changed (3 created, 10 modified, 2 deleted)

## Accomplishments

- **Task 1 — wantmatch repoint (`cbbc208`):** `ForItem`/`ForName` now `SELECT … FROM wishlist_item` with the `pinged = 1` gate (the inverse of the wantlist's `muted = 0`, Pitfall 8), an INNER `JOIN character` (`character_id` is NOT NULL now), and the `note` column dropped (the `Hit.Note` field + scan removed). `DiscordUserID` stays scanned from `w.discord_user_id` (the wishlist owner — the load-bearing T-34-08 DM-target invariant). The sole live caller `ec/ec.go:211` compiles unchanged. **Also repointed `store.ECPollSet` to `wishlist_item`** (the EC poll-set pre-filter — Rule-3: production code that 500s on the dropped table). `ec/embed.go whyWanted` degrades to the fixed `"on your wishlist"` fallback (the nil-guard made dropping the note safe). The ported `TestForItem_DMTargetIsWishOwner_NotCharacterOwner` regression passes (the char is owned by one user, assigned to another, yet the DM target is the wishlist owner).
- **Task 2 — owner-scoped write API (`58587e1`):** `webadmin/wishlist.go` (the `wantlist.go` clone): `AddWishlistHandler` derives the owner from the session (`caller(ctx)`), ALWAYS authorizes the REQUIRED `character_id` in-tx via `store.IsCharAssignedToTx` BEFORE the insert (`ErrCharNotAssigned` → 403, T-34-07), detects the 2067 dup → 409, re-validates the slot against the 21-canonical-worn-slot allow-list + a ≤200-rune label → 400, and audits IDs/slot only (V7). `RemoveOwnWishlistHandler` + `SetWishlistPingHandler` are the owner-scoped silent-no-op twins (a cross-owner id flips no row, returns `removed:false` / echoes the requested ping). Retired the dropped-table test surface: deleted `webadmin/wantlist_test.go` + `store/wantlist_test.go`; reseeded `store/alertlog_test.go`, `store/eccursor_test.go`, `notify/dm_test.go` on `wishlist_item`.
- **Task 3 — read route + main.go (`03de8af`):** `readapi/wishlist.go` serves `GET /api/v1/wishlist/{char}` via `compute.WishlistFor(ctx, st, uid, char)` — the session uid from `webauth.UserFromContext` + the `?`-bound `{char}` path value; nil→[] coercion on the slot/target/suggestion slices; empty-not-404 (D-11); V7 slog (slots + status only). `main.go` registers the 4 wishlist routes under `RequireSession` and REMOVES the 5 retired `/api/v1/wantlist*` registrations (GET, POST, /remove, /guild, /mute — they would 500 on the dropped table, Pitfall 7). `main_test.go`'s route-level RequireSession 401/admitted proof repointed to the wishlist route.

## Task Commits

1. **Task 1: wantmatch + ECPollSet repoint** — `cbbc208` (feat)
2. **Task 2: owner-scoped write API + retire wantlist test surface** — `58587e1` (feat)
3. **Task 3: read route + 4-routes-registered/5-removed in main.go** — `03de8af` (feat)

## Files Created/Modified
- `internal/backendsrv/wantmatch/match.go` — repointed to wishlist_item (pinged gate, INNER JOIN, no note)
- `internal/backendsrv/wantmatch/match_test.go` — reseeded on wishlist_item; ported the DM-target-is-owner regression
- `internal/backendsrv/ec/embed.go` — whyWanted → fixed "on your wishlist" fallback (note dropped)
- `internal/backendsrv/ec/ec_test.go` — seedWant reseeded on wishlist_item; the embed test asserts the fallback
- `internal/backendsrv/store/eccursor.go` — ECPollSet repointed to wishlist_item (the Rule-3 runtime fix)
- `internal/backendsrv/store/eccursor_test.go` — insWant reseeded on wishlist_item (per-call char)
- `internal/backendsrv/store/alertlog_test.go` — seedWant reseeded on wishlist_item (per-call char)
- `internal/backendsrv/notify/dm_test.go` — seedWant reseeded on wishlist_item (per-call char)
- `internal/backendsrv/webadmin/wishlist.go` — owner-scoped add/remove/ping (the wantlist.go clone)
- `internal/backendsrv/webadmin/wishlist_test.go` — add(200)/non-owned(403)/dup(409)/bad-slot(400)/silent-no-op
- `internal/backendsrv/readapi/wishlist.go` — GET /api/v1/wishlist/{char} over compute.WishlistFor
- `cmd/squirebot-server/main.go` — 4 wishlist routes registered, 5 wantlist routes removed
- `cmd/squirebot-server/main_test.go` — RequireSession route proof repointed to wishlist
- `internal/backendsrv/webadmin/wantlist_test.go` — DELETED (retired surface)
- `internal/backendsrv/store/wantlist_test.go` — DELETED (retired surface)

## Threat Model — Mitigation Verification
- **T-34-07 (IDOR, write API):** MITIGATED — owner = `caller(ctx)`; `IsCharAssignedToTx` authorizes `character_id` in-tx BEFORE the add → 403 on a non-owned char; remove/ping are owner-scoped silent no-ops. Tested green (`TestAddWishlist_NonOwnedChar_403`, `TestRemoveOwnWishlist_CrossOwnerNoOp_OwnRemoved`, `TestSetWishlistPing_CrossOwnerNoOp_OwnFlips`).
- **T-34-08 (Spoofing, DM target):** MITIGATED — `DiscordUserID` is always `wishlist_item.discord_user_id`; CharacterName is display-only. Ported regression `TestForItem_DMTargetIsWishOwner_NotCharacterOwner` passes.
- **T-34-09 (SQLi, {char}/matcher):** MITIGATED — `{char}`/uid bind only as `?` placeholders downstream; the matcher SELECTs are `?`-bound; never name-concatenated.
- **T-34-10 (input, slot enum + label):** MITIGATED — `validWishlist` re-checks the 21-slot allow-list + a ≤200-rune label server-side (`TestAddWishlist_InvalidSlot_400`, incl. the omitted Charm/empty-slot/missing-char_id cases).
- **T-34-11 (DoS, stale routes):** MITIGATED — the 5 `/api/v1/wantlist*` registrations removed; `go build` rc=0; `main_test.go` proves the wishlist route is RequireSession-gated.
- **T-34-12 (Info disclosure, audit/slog):** MITIGATED — audit detail carries item_id/character_id/slot/want_id only (asserted no label/name leak); slog carries op + count + status only.

## Deviations from Plan

**[Rule 3 - Blocking] Repointed `store.ECPollSet` to `wishlist_item`.**
- **Found during:** Task 1 (the `ec` package tests failed with `no such table: wantlist_item` from the production `ECPollSet` query, not just the test seeds).
- **Issue:** `store/eccursor.go:ECPollSet` (the EC monitor's "which item_ids does anyone want, poll those" pre-filter — the spine that resolves the set `wantmatch.ForItem` then matches) still `SELECT … FROM wantlist_item`. The D-01 clean break dropped that table, so the live EC monitor would 500 at runtime.
- **Fix:** Repointed the SELECT to `wishlist_item` (active=1, item_id NOT NULL; the ping gate is NOT applied here — it defines what to POLL, applied downstream at the matcher). Reseeded `eccursor_test.go`.
- **Why in-scope:** the plan's `<objective>` is "wire the 34-01 data layer to the EC/notification spine"; the deferred-items handoff lists `store` as a 34-02 repair package. `ECPollSet` is the same matcher spine `match.go` repoints — leaving it pointed at the dropped table would be a live prod bug.
- **Commit:** `cbbc208`

**[Rule 3 - Blocking] `ec/embed.go whyWanted` dropped the `Hit.Note` read.**
- **Found during:** Task 1 (dropping `Hit.Note` from the struct broke `embed.go:98`).
- **Issue:** the per-slot wishlist model has no free-text note (the plan dropped the note column from the matcher); `whyWanted` read `hit.Note`.
- **Fix:** `whyWanted` now returns the fixed `"on your wishlist"` fallback (the field was already nil-guarded → degrades cleanly per the plan's verified-safe note). Updated `TestBuildEmbed_OmitsNullPriceAndSeller` to assert the fallback.
- **Commit:** `cbbc208`

**[In-scope cleanup] Deleted the retired wantlist TEST files; KEPT the production wantlist.go/store/wantlist.go.**
- The plan Task 3 NOTE explicitly directs leaving the production wantlist files in place (they still compile — SQL strings are runtime-only), and `store/wantlist.go` declares `sqliteConstraintUnique` + `boolToInt` that `wishlist.go`/`assignment.go`/`guildchannel.go` REUSE (deleting it would break the build). Only the TEST files that seed the dropped `wantlist_item` table (`webadmin/wantlist_test.go`, `store/wantlist_test.go`) were deleted — their coverage is replaced by the wishlist tests.

## Issues Encountered
None unresolved. All reseeded tests required a NOT-NULL `character_id` (a throwaway owner+character per seed) and a unique `character.name` (UNIQUE COLLATE NOCASE) — handled with a per-call name counter in each touched test file.

## Verification
- `go test ./internal/backendsrv/wantmatch/... ./internal/backendsrv/ec/...` — green (repoint + ported DM-target-is-owner regression + the pinged gate + ECPollSet).
- `go test ./internal/backendsrv/webadmin/... -run Wishlist` — green (add 200 / non-owned 403 / dup 409 / bad-slot 400 / silent IDOR no-op).
- `go test ./internal/backendsrv/readapi/... ./cmd/squirebot-server/...` — green (the {char} read route + the route-level RequireSession 401/admitted proof).
- `go test ./internal/backendsrv/store/... ./internal/backendsrv/notify/...` — green (the reseeded alertlog/eccursor/dm tests).
- **`go test ./...` — GREEN** (all the retired-wantlist-surface packages repaired; no FAIL).
- `go vet ./...` — clean. `go build ./...` — rc=0.

## User Setup Required
None — no external service configuration. (The 4 new routes go live at the 34-04 deploy; watcher untouched, no migration.)

## Next Phase Readiness
- **34-03 unblocked:** the web Wishlist tab consumes `GET /api/v1/wishlist/{char}` (WishlistView snake_case contract) + `POST /api/v1/wishlist` / `/remove` / `/ping` (the owner-scoped writes), and the per-character search corpus (WISH-07) is the per-character `WishlistView` the web lazily fetches across non-bank/bot chars.
- No git tag (watcher UNTOUCHED).

## Self-Check: PASSED

All 3 created files exist on disk (`webadmin/wishlist.go`, `webadmin/wishlist_test.go`, `readapi/wishlist.go`); the 2 deleted files are gone; all 3 task commits (`cbbc208`, `58587e1`, `03de8af`) are present in git history; `go test ./...` + `go build ./...` + `go vet ./...` all green.

---
*Phase: 34-wishlist-rework-per-character-per-slot-upgrades*
*Completed: 2026-06-19*
