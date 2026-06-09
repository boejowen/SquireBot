---
phase: 28-character-tagged-wantlist
reviewed: 2026-06-08T00:00:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/ec/embed.go
  - internal/backendsrv/ec/embed_test.go
  - internal/backendsrv/migrations/00010_character_tagged_wantlist.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/alertlog_test.go
  - internal/backendsrv/store/assignment.go
  - internal/backendsrv/store/assignment_test.go
  - internal/backendsrv/store/wantlist.go
  - internal/backendsrv/store/wantlist_test.go
  - internal/backendsrv/wantmatch/match.go
  - internal/backendsrv/wantmatch/match_test.go
  - internal/backendsrv/webadmin/wantlist.go
  - internal/backendsrv/webadmin/wantlist_test.go
  - web/src/lib/api.ts
  - web/src/lib/columns.ts
  - web/src/lib/components/WantAddForm.svelte
  - web/src/lib/components/WantlistPanel.svelte
  - web/src/lib/wantlist/groupByChar.ts
  - web/src/lib/wantlist/groupByChar.test.ts
findings:
  blocker: 0
  high: 0
  medium: 2
  low: 3
  nit: 2
  total: 7
status: issues_found
---

# Phase 28: Code Review Report

**Reviewed:** 2026-06-08
**Depth:** standard
**Status:** issues_found

## Summary

Phase 28 (Character-Tagged Wantlist) was reviewed adversarially across the live-bound SQL migration, the Go backend (store / handler / wantmatch / EC embed), and the Svelte/TS web layer. The implementation is unusually disciplined: the four highest-risk controls the phase hinges on all hold up under scrutiny.

- **Migration 00010** correctly preserves the 00006 account-level dedup (the `COALESCE(character_id, -1)` sentinel collapses NULL to one value) while allowing the same item for two characters. The `-1` sentinel cannot collide with a real `character.id` (that column is `INTEGER PRIMARY KEY`, a rowid alias — always positive for inserted rows). The migration is extend-only, the Down is a no-op, and 00001-00009 are untouched.
- **The IDOR guard** (`IsCharAssignedToTx`) runs inside the same `withTx` as the insert, before `AddWantTx`, on the only path that supplies a `character_id`; a nil tag skips it (account-level), a non-owned tag returns `ErrCharNotAssigned` → 403 → rollback (no row, no audit). TOCTOU-safe and proven by `TestAddWant_TaggedUnassignedChar_403`.
- **The DM-target invariant (CWANT-05/T-28-06)** holds: `Hit.DiscordUserID` is still scanned from `w.discord_user_id`; the `LEFT JOIN character` only adds a trailing display column; `match.go` reads no `owner_id`/`character_assignment`. The regression test seeds a character owned by one user and assigned to a third, and asserts the DM target is still the want creator — a genuinely load-bearing assertion, not tautological.
- **Guildwide read privacy** is consistent end-to-end: no `note` in the SQL SELECT, the Go `GuildWantRow`, the TS `GuildWantRow`, or `guildWantlistColumns`. The leak test greps the response body for the literal private note.

No BLOCKER or HIGH findings. Two MEDIUM observations concern data-integrity edge cases that are correctness-adjacent rather than vulnerabilities, plus several LOW/NIT polish items. The known DOM-rendering browser-smoke gap is acknowledged in the plan and is not re-raised here.

## Warnings

### MD-01: Dangling-tag wants are silently dropped from `charOptions` group-by filter when the character is removed

**File:** `web/src/lib/components/WantlistPanel.svelte:126-138` (and `internal/backendsrv/store/wantlist.go:104-153`)
**Severity:** MEDIUM
**Issue:** When a tagged character is soft-removed (`is_removed=1`) or hard-deleted, `ListOwnWants`' `LEFT JOIN character` still returns the want but with `character_name = NULL` while `character_id` remains the (now orphaned) id. In `charOptions`, such a row hits the `else if (!m.has(w.character_id))` branch and is offered as a filter option labelled `#<id>` (the `?? \`#${w.character_id}\`` fallback) — a bare numeric id with no name. Meanwhile `groupByChar(rows, <thatId>)` still matches it, so the user CAN select a meaningless "#47" option. This is a UX/data-consistency wart: the migration comment explicitly anticipates removed-char tags resolving to `NULL character_name`, but the UI then surfaces the raw id as if it were a character.
**Why it matters:** Not a security issue (no private data leaks), but a member with a tag to a deleted alt sees a nonsense filter entry and a "phantom character" group. The backend correctly nulls the name; the frontend should treat `character_id != null && character_name == null` as account-level-equivalent (or label it "Removed character") rather than emitting `#<id>`.
**Fix:** In `charOptions`, skip or relabel rows whose `character_name` is null:
```ts
for (const w of wants) {
  if (w.character_id === null || w.character_name === null) { hasAccountLevel ||= w.character_id === null; continue; }
  if (!m.has(w.character_id)) m.set(w.character_id, w.character_name);
}
```
(Decide product-wise whether a dangling tag groups under Account-level or a dedicated "Removed" bucket; either is better than `#47`.) The same applies to the guild grid's Character facet, which renders the orphaned want with a blank Character cell — acceptable there since the accessor already coalesces to `''`.

### MD-02: `ListGuildWants` has no `ORDER BY` tie-break and no pagination ceiling on an all-members, all-time read

**File:** `internal/backendsrv/store/wantlist.go:182-190`
**Severity:** MEDIUM
**Issue:** The guildwide query is `ORDER BY w.created_at DESC` with no secondary key. `created_at` is unix *seconds*; two wants added in the same second (entirely plausible during a bulk add, or seeded with identical timestamps as the tests do — both use `created_at` 1 and 2 but a real burst could collide) have a non-deterministic relative order across calls, which makes the grid order jitter between fetches. Separately, this read returns *every active want across all members for all time* with no cap; at guild scale (~12 members) this is fine today, but it is the one unbounded all-members read added this phase and has no upper bound if the wantlist grows.
**Why it matters:** The ordering non-determinism is a real (if minor) correctness issue — `ListOwnWants` shares it, so this is a pre-existing pattern, but the guild read amplifies it across members. Performance is explicitly out of v1 scope, so the unbounded-read note is informational only.
**Fix:** Add a deterministic tie-break: `ORDER BY w.created_at DESC, w.id DESC`. (Apply the same to `ListOwnWants` for consistency.) No pagination change needed for v1.

## Info / Low

### LOW-01: A forged-tag 403 in the add form shows a generic error, masking the real cause

**File:** `web/src/lib/components/WantAddForm.svelte:161-174`
**Severity:** LOW
**Issue:** `submit()`'s catch only special-cases `code === 'duplicate'`; a `char_not_assigned` 403 (the server rejecting a forged/stale character tag) falls into the generic "Couldn't add that to your wantlist" branch. This is correct security behavior (the select only lists owned characters, so a legit user never hits it), but if a character is unassigned in another tab between page-load and submit, the user gets an opaque generic error instead of "that character is no longer yours."
**Why it matters:** Edge-case UX only — the gate itself is sound. Worth a one-line branch for clarity.
**Fix:** Add `else if (code === 'char_not_assigned') addErrorMsg = "That character isn't assigned to you anymore — pick another or leave it account-level.";` before the generic fallback.

### LOW-02: `charFilter` retains a stale character_id when switching views, but is only read in My view

**File:** `web/src/lib/components/WantlistPanel.svelte:62-69, 328-344`
**Severity:** LOW
**Issue:** `charFilterValue` is not reset when toggling to Guild and back, and the `{#if wantView === 'mine' && ...}` guard hides the `<select>` in Guild view. If a member filters My to character X, switches to Guild, removes/reassigns that character elsewhere, then returns to My, the select may still hold a `character_id` no longer present in `charOptions` — the `<select>` then has a `bind:value` with no matching `<option>`, which most browsers coerce to the first option ("All characters") but Svelte's bound value stays the stale string. `filteredWants` would then filter on a character with zero matching rows, showing the "No wants are tagged to that character" empty state spuriously.
**Why it matters:** Narrow, self-correcting (re-selecting All fixes it), and not a data/security issue.
**Fix:** Reset `charFilterValue = ''` in `setView('guild')`, or clamp `charFilter` to `null` when the selected id is absent from `charOptions.chars`.

### LOW-03: `validWant` rejects `character_id <= 0` but the audit still records a rejected-shape attempt only when it reaches the tx

**File:** `internal/backendsrv/webadmin/wantlist.go:95-97, 184`
**Severity:** LOW
**Issue:** A `character_id <= 0` is rejected at `validWant` (400 `invalid_input`) before the tx, so no audit row is written — fine. But note the audit detail map records `"character_id": req.CharacterID` as a raw pointer; for an account-level want this serializes the key with a `null` value into the audit JSON. That's harmless (ids-only, V7-compliant) but means every account-level add now carries a `"character_id":null` in its audit detail where pre-phase it had none. Confirm downstream audit consumers tolerate the new key.
**Why it matters:** No vuln; a schema-of-audit-detail observation. The test asserts the key is present for a tagged add but does not assert its absence/null for account-level.
**Fix:** Optional — omit the key when nil: `detail := map[string]any{"item_id": req.ItemID}; if req.CharacterID != nil { detail["character_id"] = *req.CharacterID }`. This also avoids logging a bare `null`.

### NIT-01: `buildEmbed` "For" field placed after "Why you wanted it" — minor ordering inconsistency

**File:** `internal/backendsrv/ec/embed.go:146-163`
**Severity:** NIT
**Issue:** The conditional "For" field is appended last, after the always-present "Why you wanted it" field. The inline (true) "For" field rendering after a full-width (Inline:false) field produces a slightly awkward embed layout (a lone inline field on its own row). Cosmetic; the plan specified this position.
**Fix:** Consider appending "For" among the other inline fields (Price/Seen/Seller) before the full-width "Why you wanted it", so inline fields group on one row. Cosmetic only.

### NIT-02: `seedWantChar` test helper omits `note` column but relies on positional defaults

**File:** `internal/backendsrv/wantmatch/match_test.go` (seedWantChar)
**Severity:** NIT
**Issue:** The helper inserts an explicit column list without `note`, relying on the column's nullable default — correct, but the adjacent `seedWant`/`seedWantNote` helpers vary in which columns they name, making the fixtures slightly inconsistent to read. No behavioral impact.
**Fix:** None required; noted for fixture readability.

---

## Verification notes (controls that PASSED adversarial review)

- **Migration dedup correctness:** `COALESCE(character_id, -1)` preserves NULL-vs-NULL collision and permits two distinct real char ids — proven by `TestMigrate_00010` cases 2/3/4/6/7. The `-1` sentinel is collision-safe (`character.id` is a positive rowid alias).
- **Migration idempotency / extend-only:** Down is `SELECT 1;`; no edits to 00001-00009; second `RunMigrations` asserted a no-op (`goose_db_version` count unchanged). No partial-failure replay hazard (single forward DDL block).
- **IDOR guard placement:** `IsCharAssignedToTx` is called in-tx, before `AddWantTx`, only when `req.CharacterID != nil`; `ErrCharNotAssigned` → `mapWantErr` → 403; rollback leaves no row/audit. `TestAddWant_TaggedUnassignedChar_403` asserts 403 + zero persisted + zero audit.
- **DM-target invariant:** `Hit.DiscordUserID` scan target unchanged; LEFT JOIN adds only a trailing display column; no `owner_id`/`character_assignment` read in `match.go`. `TestForItem_DMTargetIsWantOwner_NotCharacterOwner` is non-tautological (distinct owner + assignee).
- **Privacy:** `note` absent from `ListGuildWants` SELECT, `GuildWantRow` (Go + TS), and `guildWantlistColumns`. Leak test greps the raw response body.
- **XSS:** No `{@html}` in either component; names render via Svelte `{}` auto-escape and TanStack text-node accessors; the EC "For" value is a plain (non-URL, non-markdown) embed field.
- **SQL injection:** All queries parameterized; no string interpolation of ids/names.
- **Route gating:** `GET /api/v1/wantlist/guild` wrapped in `RequireSession` (login-only, not `RequireOfficer`) — matches the post-P15 read-API posture.

---

_Reviewed: 2026-06-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
