---
phase: 28-character-tagged-wantlist
verified: 2026-06-09T03:00:00Z
status: human_needed
score: 4/4 truths code-verified (backend); web DOM behavior + live v9→v10 deploy require human verification
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification:
  - test: "Add-want form character <select>"
    expected: "The add-want form on /wantlist shows a character <select> populated from the caller's assigned characters (fetchMyCharacters) with a '(no character)' default; adding with a tag and adding account-level both succeed."
    why_human: "node vitest is DOM-blind (no @testing-library/svelte — the P15/P26/P27 trap); the <select> render + onchange + submit-coercion cannot be proven by npm test."
  - test: "Forged character tag rejected end-to-end"
    expected: "The select lists only the caller's own characters; POSTing a forged body character_id (a char assigned to another member) returns 403 char_not_assigned and persists nothing."
    why_human: "The 403 path is unit-tested server-side (TestAddWant_TaggedUnassignedChar_403, PASS), but the UI-cannot-offer-it + live-server-rejects-forged-body needs a browser/curl smoke on a deployed build."
  - test: "My/Guild toggle"
    expected: "The /wantlist My/Guild segmented toggle switches between the caller's own list and the all-members guildwide roll-up (lazy-loaded on first Guild switch)."
    why_human: "Interactive toggle state + lazy-fetch behavior is DOM-level, invisible to node vitest."
  - test: "Guildwide attribution display (CWANT-04)"
    expected: "The guildwide list shows owner (web_user.username) + character name per want; account-level wants show no character; the list NEVER shows a note column."
    why_human: "Rendered grid columns are DOM output; store excludes note (verified in code) but the visual roll-up needs a browser smoke."
  - test: "Group/filter own wantlist by character (CWANT-06)"
    expected: "The My-only group-by-character <select> narrows the own list to the chosen character; 'All characters' and 'Account-level' options behave."
    why_human: "groupByChar pure helper is node-tested (5 tests PASS); the <select> wiring that drives it is DOM-level."
  - test: "Name escaping (no HTML injection)"
    expected: "Character + owner names render auto-escaped via plain {} braces — never raw HTML."
    why_human: "grep confirms no @html in either component, but visual confirmation of escaping is a browser check."
  - test: "Live v9 → v10 migration + EC embed on prod"
    expected: "Deploy to api.squirebot.quest: goose.Up migrates schema v9→v10 (00010) idempotently with existing wants backfilled to NULL character_id (no data loss); a tagged-want EC match shows the 'For <char>' embed field in a live DM while still DMing the want OWNER."
    why_human: "The migration + EC embed are only exercised against a test DB / fixtures here; the live deploy + a live tagged DM (per P26/P27 deploy doctrine) are ops/UAT items not reachable from the verifier."
---

# Phase 28: Character-Tagged Wantlist Verification Report

**Phase Goal:** Wants gain an optional character dimension — a member can tag a want to one of their assigned characters, those wants roll up into the guildwide wantlist alongside untagged/pre-existing wants, and the member can filter/group their own wantlist by character — while the EC-tunnel monitor DM still targets the character's OWNER.
**Verified:** 2026-06-09
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Optional character tag on add; untagged + pre-existing wants stay valid; 00010 backfills existing rows to NULL character_id with no data loss (CWANT-01/02) | ✓ VERIFIED (code) | `00010_..._wantlist.sql`: `ALTER TABLE wantlist_item ADD COLUMN character_id INTEGER REFERENCES character(id)` — NULLABLE, no NOT NULL/DEFAULT/CASCADE (auto-NULL backfill). `AddWantTx(...characterID *int64...)` persists it (`wantlist.go:77,79-81`). `addWantReq.CharacterID *int64` optional (`webadmin/wantlist.go:57`). `TestMigrate_00010_CharacterTaggedWantlist` PASS. |
| 2 | Tagged wants aggregate into the guildwide list alongside untagged; the list surfaces character/owner attribution, excludes the private note (CWANT-03/04) | ✓ VERIFIED (code) | `ListGuildWants` (`wantlist.go:182`): `JOIN web_user wu` (owner) + `LEFT JOIN character c` (char name), `WHERE w.active=1`, **no note** in SELECT or `GuildWantRow` struct (`:162-173`). `GET /api/v1/wantlist/guild` → `ListGuildWantsHandler` (no caller scope), RequireSession (`main.go:348`). Web: `fetchGuildWants` + `GuildWantRow` (no note) in `api.ts`; `guildWantlistColumns` (Owner·Character·Priority·Item·Reason). Store/webadmin tests PASS. **Display = human.** |
| 3 | A member can filter/group their own wantlist by character (CWANT-06) | ✓ VERIFIED (code) | Pure `groupByChar.ts` (`ACCOUNT_LEVEL` Symbol sentinel; null=passthrough, number=by-char, ACCOUNT_LEVEL=untagged) + 5 node tests PASS. `ListOwnWants` + `WantlistRow` surface `character_id`/`character_name` via LEFT JOIN. WantlistPanel group-by-char `<select>` drives `groupByChar` (`filteredWants = groupByChar(wants, charFilter)`). **`<select>` behavior = human.** |
| 4 | EC-tunnel DM for a tagged want still targets the OWNER (discord_user_id); embed names the character per CWANT-05 | ✓ VERIFIED (code) | `wantmatch.Hit.DiscordUserID` scanned from `w.discord_user_id` — UNCHANGED; `CharacterName` added via `LEFT JOIN character c` in BOTH `ForItem`/`ForName`. Grep: NO `owner_id`/`character_assignment` read in match.go DM path. `ec/embed.go` adds display-only `"For"` field (TrimSpace-guarded), touches no send path. Regression `TestForItem_DMTargetIsWantOwner_NotCharacterOwner` (seeds char owned by another, asserts target=="alice" want owner) PASS. **Live DM = human.** |

**Score:** 4/4 truths code-verified (backend layer fully testable; the web DOM + live deploy are the human items).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `migrations/00010_character_tagged_wantlist.sql` | nullable character_id + COALESCE dedup rewrite | ✓ VERIFIED | Contains `COALESCE(character_id, -1)` (x2 indexes), `ADD COLUMN character_id INTEGER REFERENCES character(id)` (no CASCADE/NOT NULL/DEFAULT), `DROP INDEX` x2, Down=`SELECT 1;`. git log confirms 00001–00009 untouched this phase. |
| `store/wantlist.go` | AddWantTx +char, ListOwnWants +char, NEW ListGuildWants (no note) | ✓ VERIFIED | All three present; `GuildWantRow` has no Note field; dedup `sqliteConstraintUnique` detection intact. |
| `store/assignment.go` | IsCharAssignedToTx + ErrCharNotAssigned | ✓ VERIFIED | `func IsCharAssignedToTx(...tx *sql.Tx...)` SELECT-1 probe; `ErrCharNotAssigned = errors.New("char_not_assigned")`. |
| `webadmin/wantlist.go` | IDOR guard + ListGuildWantsHandler + 403 | ✓ VERIFIED | In-tx `IsCharAssignedToTx` BEFORE `AddWantTx`; `mapWantErr` → StatusForbidden; `ListGuildWantsHandler` (no caller scope); audit detail = ids only. |
| `cmd/squirebot-server/main.go` | GET /api/v1/wantlist/guild (RequireSession) | ✓ VERIFIED | `main.go:348` RequireSession-wrapped (not RequireOfficer). |
| `wantmatch/match.go` | Hit.CharacterName LEFT JOIN, DM target unchanged | ✓ VERIFIED | Field added; both SELECTs LEFT JOIN; DiscordUserID scan unchanged; no owner_id/char_assignment read. |
| `ec/embed.go` | display-only "For" field | ✓ VERIFIED | TrimSpace-guarded conditional MessageEmbedField, no send-path change. |
| `web/src/lib/api.ts` | char fields + fetchGuildWants + GuildWantRow (no note) | ✓ VERIFIED | WantlistRow.character_id/_name; addWant body character_id; GuildWantRow + fetchGuildWants. |
| `web/src/lib/wantlist/groupByChar.ts` (+ .test.ts) | pure helper + node test | ✓ VERIFIED | Pure DOM-free; 5 node tests PASS. |
| `WantAddForm.svelte` | char <select> from fetchMyCharacters | ✓ WIRED (code) | `<select bind:value={charSelect}>` with "(no character)" default, options from `fetchMyCharacters()`. Render = human. |
| `WantlistPanel.svelte` | My/Guild toggle (one grid) + group-by-char | ✓ WIRED (code) | Segmented toggle (one control, one DataGrid — consolidated lock), lazy fetchGuildWants, groupByChar filter, no @html. Behavior = human. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| AddWantTx | wantlist_item.character_id | INSERT character_id | ✓ WIRED | INSERT column list includes character_id; persisted. |
| ListGuildWants | web_user.username + character.name | JOIN + LEFT JOIN | ✓ WIRED | Both joins present; owner + char name surfaced. |
| AddWantHandler | store.IsCharAssignedToTx | in-tx guard before AddWantTx | ✓ WIRED | Guard precedes insert in same withTx; 403 on fail. |
| ec/embed.go buildEmbed | hit.CharacterName | conditional MessageEmbedField | ✓ WIRED | TrimSpace-guarded "For" field. |
| WantAddForm | fetchMyCharacters() | <select> options | ✓ WIRED (code) | Options bound to fetchMyCharacters result. Render unverified (human). |
| WantlistPanel | fetchGuildWants / groupByChar | My/Guild toggle + group select | ✓ WIRED (code) | Both imported + driven. Interactive behavior unverified (human). |

### Behavioral Spot-Checks (gate commands)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Backend compiles | `go build ./...` | exit 0 | ✓ PASS |
| Backend suites | `go test ./internal/backendsrv/{migrations,store,webadmin,wantmatch,ec}/ -count=1` | all 5 ok | ✓ PASS |
| 00010 migration | `TestMigrate_00010_CharacterTaggedWantlist` | within store/migrations PASS | ✓ PASS |
| 403 forged-tag | `TestAddWant_TaggedUnassignedChar_403` | asserts 403 | ✓ PASS |
| DM-target regression | `TestForItem_DMTargetIsWantOwner_NotCharacterOwner` | asserts owner==want creator | ✓ PASS |
| Web typecheck | `cd web && npm run check` | 486 files, 0 errors / 0 warnings | ✓ PASS |
| Web tests | `cd web && npm test` | 303 passed (24 files) incl. 5 groupByChar | ✓ PASS |
| Web build | `cd web && npm run build` | built in ~31s, site written | ✓ PASS |
| No @html | grep WantAddForm + WantlistPanel | no matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CWANT-01 | 28-01/02/03 | Optional character tag on add | ✓ SATISFIED (code) | AddWantTx +char, addWantReq.CharacterID, in-tx IDOR guard, WantAddForm <select>. UI render = human. |
| CWANT-02 | 28-01 | Tag optional; 00010 backfills existing to NULL, no data loss (schema bump) | ✓ SATISFIED | Nullable ADD COLUMN auto-backfills NULL; migrate test asserts backfill. (Schema → v10, not v9 — RESOLVED below.) |
| CWANT-03 | 28-01/02/03 | Tagged wants roll up into guildwide list | ✓ SATISFIED (code) | ListGuildWants all-members read + /wantlist/guild route + Guild toggle. Display = human. |
| CWANT-04 | 28-01/02/03 | Guildwide list surfaces character/owner attribution (note excluded) | ✓ SATISFIED (code) | owner + character_name in GuildWantRow; note excluded (struct+SELECT). Display = human. |
| CWANT-05 | 28-02 | EC DM targets OWNER; embed names character | ✓ SATISFIED (code) | DiscordUserID unchanged + "For" embed field; regression test. Live DM = human. |
| CWANT-06 | 28-01/03 | Member can filter/group own wantlist by character | ✓ SATISFIED (code) | groupByChar pure helper + 5 tests; ListOwnWants char name; group-by-char <select>. <select> behavior = human. |

**Coverage:** 6/6 CWANT requirements satisfied in code. No orphaned requirements (ROADMAP maps CWANT-01..06 → Phase 28; all claimed across the 3 plans).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | TODO/FIXME/placeholder/not-implemented scan over all 9 modified files | — | Clean — no stubs, no placeholders, no dangling work. |

### Note on the schema-version wording

ROADMAP/REQUIREMENTS prose in a few places says the `character_id` touch is the `00009` migration → schema **v9** (CWANT-02 carries the stale "00009"/"v9" wording from the pre-split plan). The ACTUAL ship is correct: `00009` shipped as `character_assignment` in P26, and the `character_id` column lands in **`00010` → schema v10** (the ROADMAP Phase-28 detail and Plan 28-01 both state this; the prose IDs are stale, not a defect). The migration file, store doctrine comment, and migrate test all consistently say v9→v10. **Not a gap** — wording drift only; the extend-only doctrine is intact (00001–00009 untouched, verified by git log).

### Human Verification Required

The backend (migration, store, handlers, IDOR guard, wantmatch/EC embed, DM-target regression) is fully node/go-testable and PASSES. The web layer's interactive DOM (the character `<select>`, the My/Guild toggle, the guildwide attribution display, the group-by-char control) is invisible to node-only vitest (no @testing-library/svelte — the documented P15/P26/P27 trap), and the LIVE v9→v10 deploy + a live tagged-want DM are ops/UAT items. Per the P26/P27 doctrine, these land as human verification:

1. **Browser-smoke on a DEPLOYED build** (`npm run dev` bounces login against prod) — run the 8-point checklist in 28-03-SUMMARY (char select populated + tag/account-level add; forged char impossible in UI + 403 on forged body; My/Guild toggle; guildwide owner+char attribution; no note column; group-by-char filter; escaped names).
2. **`/gsd-ui-review 28`** — flagged (UI hint: yes).
3. **Live v9 → v10 deploy** to api.squirebot.quest — goose.Up idempotent migration with existing wants backfilled to NULL (no data loss) + a live tagged-want EC match showing "For <char>" while DMing the want OWNER.

### Gaps Summary

No code gaps. All 6 CWANT requirements are satisfied in the codebase: the 00010 migration is extend-only with the COALESCE dedup rewrite and NULL backfill; the store threads the optional tag, adds the note-excluding guildwide read, and the in-tx IDOR guard; the API authorizes the tag (403 on forged char), serves the login-gated guildwide route, and proves the EC DM target stays the want owner while naming the character display-only; the web surfaces the tag select, the My/Guild toggle, and the group-by-char filter (all node-testable parts green, no @html). All Go + web gates pass. The only open items are the inherently-non-node-verifiable web DOM behavior and the live deploy — routed to human verification, matching the established P26/P27 close pattern.

---

_Verified: 2026-06-09_
_Verifier: Claude (gsd-verifier)_
