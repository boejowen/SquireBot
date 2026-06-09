---
phase: 28-character-tagged-wantlist
plan: 02
subsystem: api
tags: [go, sqlite, wantlist, idor, discord-embed, wantmatch]

# Dependency graph
requires:
  - phase: 28-character-tagged-wantlist (Plan 01)
    provides: "AddWantTx(...characterID *int64...), ListGuildWants, IsCharAssignedToTx, ErrCharNotAssigned (schema v10 / 00010 migration)"
  - phase: 26-character-assignment
    provides: "character_assignment table + the discord_user_id↔character ownership model"
provides:
  - "POST /api/v1/wantlist optional character_id with a server-side in-tx IDOR guard (403 on a non-owned char)"
  - "GET /api/v1/wantlist/guild — login-gated all-members wantlist roll-up (note excluded)"
  - "wantmatch.Hit.CharacterName via LEFT JOIN character (DM target unchanged)"
  - "EC embed display-only 'For <char>' field"
affects: [28-03 (web frontend consuming the guild route + tagged add), notify, ec-monitor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Authorize-under-transaction IDOR guard: IsCharAssignedToTx runs inside the same withTx BEFORE the insert (TOCTOU-safe), mirroring the officer in-tx re-check"
    - "Display-only LEFT JOIN: a presentation field is carried on the matcher Hit without ever re-deriving the DM target"

key-files:
  created:
    - internal/backendsrv/ec/embed_test.go
  modified:
    - internal/backendsrv/webadmin/wantlist.go
    - internal/backendsrv/webadmin/wantlist_test.go
    - cmd/squirebot-server/main.go
    - internal/backendsrv/wantmatch/match.go
    - internal/backendsrv/wantmatch/match_test.go
    - internal/backendsrv/ec/embed.go

key-decisions:
  - "The character tag is authorized in-tx (IsCharAssignedToTx) BEFORE the insert, not in handler glue — the body character_id is untrusted"
  - "ErrCharNotAssigned maps to 403 char_not_assigned (mirrors the ErrDuplicateWant→409 case)"
  - "GET /api/v1/wantlist/guild is RequireSession (login-only since P15), NOT RequireOfficer"
  - "Hit.DiscordUserID stays w.discord_user_id; CharacterName is a display-only LEFT JOIN that never touches the DM target"
  - "validWant rejects a non-positive character_id (400) as a shape check; authorization stays the in-tx guard"

patterns-established:
  - "In-tx IDOR authorization for an optional body-supplied resource id"
  - "Regression test that seeds a tagged char owned/assigned to a DIFFERENT user and asserts the DM target is still the want creator"

requirements-completed: [CWANT-01, CWANT-03, CWANT-04, CWANT-05]

# Metrics
duration: 22min
completed: 2026-06-09
---

# Phase 28 Plan 02: Character-Tagged Wantlist — API + EC Embed Summary

**Server-authorized character tag on POST /api/v1/wantlist (403 on a non-owned char), a login-gated guildwide wantlist roll-up, and a display-only "For <char>" EC embed field — with the DM target proven to stay the want owner regardless of the tag.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-06-09T03:18:01Z
- **Completed:** 2026-06-09T03:40:00Z
- **Tasks:** 3
- **Files modified:** 6 (1 created)

## Accomplishments
- `AddWantHandler` authorizes the optional `character_id` via `store.IsCharAssignedToTx` INSIDE the existing `withTx` BEFORE the insert; a forged tag (a character assigned to another member) returns 403 `char_not_assigned` and persists nothing (verified by the negative IDOR test).
- New `ListGuildWantsHandler` + route `GET /api/v1/wantlist/guild` (RequireSession, login-only) returning every member's active wants with the private `note` excluded (T-28-07).
- `wantmatch.Hit` gained `CharacterName *string` via a `LEFT JOIN character c ON c.id = w.character_id` in BOTH `ForItem` and `ForName`; `Hit.DiscordUserID` is structurally unchanged (still scanned from `w.discord_user_id`).
- EC rich embed gained a display-only "For <char>" field (omitted when the tag is nil or whitespace-only) with no change to the send path.
- The two load-bearing regression tests pass: the IDOR-403 (`TestAddWant_TaggedUnassignedChar_403`) and the DM-target-unchanged proof (`TestForItem_DMTargetIsWantOwner_NotCharacterOwner`, which seeds a char owned by one user and assigned to another, asserting the DM target is still the want creator).

## Task Commits

Each task was committed atomically (TDD: tests + impl committed together per task):

1. **Task 1: AddWantHandler IDOR guard + ListGuildWantsHandler + route** - `5fa2838` (feat)
2. **Task 2: wantmatch.Hit.CharacterName via LEFT JOIN (DM target unchanged)** - `3a5bc5f` (feat)
3. **Task 3: EC embed display-only "For" field** - `1570bda` (feat)

_commit_docs is false — no planning docs were committed in the task commits._

## Files Created/Modified
- `internal/backendsrv/webadmin/wantlist.go` - `addWantReq.CharacterID`; `validWant` positive-id shape check; in-tx `IsCharAssignedToTx` guard threading `req.CharacterID` into `AddWantTx`; audit detail `character_id` (id only); `mapWantErr` → 403; new `ListGuildWantsHandler`.
- `internal/backendsrv/webadmin/wantlist_test.go` - assigned-char (200 + persist + audit), forged-char (403 + no persist + no audit), account-level, guild-list all-members + note-excluded, guild-list method-guard.
- `cmd/squirebot-server/main.go` - `GET /api/v1/wantlist/guild` wrapped in `webauth.RequireSession`.
- `internal/backendsrv/wantmatch/match.go` - `Hit.CharacterName`; LEFT JOIN + `c.name` in both SELECTs; `charName` scan target appended last; `DiscordUserID` scan unchanged.
- `internal/backendsrv/wantmatch/match_test.go` - tagged/untagged `CharacterName` for ForItem/ForName; the DM-target-unchanged regression.
- `internal/backendsrv/ec/embed.go` - conditional "For" field (TrimSpace-guarded) after "Why you wanted it".
- `internal/backendsrv/ec/embed_test.go` (created) - tagged → For=<name>, untagged → omitted, whitespace-only → omitted.

## Decisions Made
None beyond the plan — followed the plan's load-bearing constraints exactly (in-tx guard before insert, 403 mapping, RequireSession route, display-only embed, DM target untouched).

## Deviations from Plan

None - plan executed exactly as written.

The plan's `files_modified` listed `internal/backendsrv/ec/embed_test.go` as a modified file; it did not yet exist, so it was created (the existing `buildEmbed` tests live in `ec_test.go`, which was left untouched). This matches the plan's intent (the file appears in the plan's `<files>` for Task 3) and is not a behavioral deviation.

## Issues Encountered
None.

## Threat Surface Scan
No new security-relevant surface beyond the plan's `<threat_model>`. The two mitigations the register marks `mitigate` for this plan are implemented and asserted:
- **T-28-05 (IDOR):** `IsCharAssignedToTx` called in-tx before `AddWantTx`; `ErrCharNotAssigned`→403; negative test asserts 403 + no persistence.
- **T-28-06 (wrong DM recipient):** `Hit.DiscordUserID` unchanged; regression test asserts the DM target == want owner with a char owned/assigned to other users.
- **T-28-07 (info disclosure):** guildwide route is RequireSession and the store read excludes `note`; test asserts the note is absent from the response.

## Known Stubs
None — all data paths are wired (the guild route hits the real store; the embed reads the real Hit; the tag is authorized against the real assignment table).

## User Setup Required
None - no external service configuration required. (Schema v10 / 00010 migration shipped in Plan 01; no new migration here.)

## Next Phase Readiness
- The backend half of the character-tagged wantlist is complete and gated.
- Plan 03 (web frontend) can consume `GET /api/v1/wantlist/guild` and POST a `character_id` on the tagged-add path.
- The EC monitor will display "For <char>" on tagged matches automatically (no further wiring needed).

## Self-Check: PASSED

All created/modified files present on disk; all three task commits (`5fa2838`, `3a5bc5f`, `1570bda`) exist in git history.

---
*Phase: 28-character-tagged-wantlist*
*Completed: 2026-06-09*
