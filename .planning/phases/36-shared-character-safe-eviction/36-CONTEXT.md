# Phase 36: Shared-Character-Safe Eviction - Context

**Gathered:** 2026-06-22
**Status:** Ready for planning

> **Mode:** `--auto` (recommended defaults auto-selected; no interactive questions). The user opted into full auto-advance (discuss → plan → execute) for Phase 36 after Phase 35 shipped. Every decision below is a recommended default — review CONTEXT.md before/after planning if you want to adjust.

<domain>
## Phase Boundary

Narrow the per-owner eviction cascade so that evicting a guildie removes **only that member's own characters** — never **shared** characters that other guildies also play and upload. Banks/bots are already eviction-safe (Phase 35's reserved "guild" sentinel owner, id `1000000`); this phase handles **shared NON-bank characters** (OWN-03).

**Scope is backend + a small web change** (NOT backend-only — corrected 2026-06-22 after the plan-checker found that narrowing the preview empties it for an all-shared owner, which disables the Evict button in `EvictionForm.svelte`, so the officer can't revoke that departing member's code). Still: **no schema migration, watcher UNTOUCHED, no `v*` tag** — the web change deploys via the existing web atomic-swap path.

**In scope:** (a) narrowing `store.EvictOwnerTx` (and the preview it feeds) so a character stewarded by the evicted owner survives the eviction when at least one *other* guildie has uploaded it; (b) a preview contract that distinguishes "will be removed (sole-owned)" from "preserved (shared)" so the preview is never empty for an owner who still has live chars; (c) the `EvictionForm.svelte` web change so an all-shared owner stays evictable (code-only revoke) with clear messaging; (d) keeping the guild-code revoke and the grace/restore/archive lifecycle intact for the chars that ARE removed.

**Out of scope:** reworking `audit_log` into a first-class sharing model; backfilling shares from before the `cross_owner_write` trail existed; re-introducing per-character write ownership / cross-owner rejection (explicitly reversed in `260621-u6j`); any watcher change; any change to bank/bot survival (already done in Phase 35).
</domain>

<decisions>
## Implementation Decisions

### Sharing detection (the core question)
- **D-01:** Derive "shared" from the **existing `audit_log` `cross_owner_write` trail** — NO new table, NO migration. `bindCharacter` (store/binding.go) already appends a `cross_owner_write` row `(event, char_name, attempting_owner_id, current_owner_id)` every time a non-steward owner uploads a character (since `260621-u6j`). A character stewarded by the evicted owner `X` is **shared** iff there EXISTS a `cross_owner_write` row with `char_name = <char>.name (COLLATE NOCASE) AND attempting_owner_id <> X`. (Recommended over a new `character_uploader` join table: smallest blast radius, reuses data captured for exactly this purpose, keeps Phase 36 migration-free. Phase 36 adds the **first reader** of `cross_owner_write` — today there are none.)

### Narrowed eviction cascade
- **D-02:** `EvictOwnerTx(X)` flips `is_removed=1 / grace_until` **only** on `X`'s live characters that are NOT shared per D-01. Shared characters are preserved (`is_removed` stays 0, no grace stamp). The **guild-code revoke** (`UPDATE guild_code SET disabled_at=… WHERE owner_id=X`) stays UNCHANGED — the evicted member's watcher still stops uploading regardless of how many chars are removed. `removedCount` reflects only the chars actually flipped (so evicting an owner who only stewards shared chars returns 0 removed but still revokes the code). The Phase-35 sentinel guard (`ErrCannotEvictSentinel`) stays first.

### Steward repoint for surviving shared chars
- **D-03:** When a surviving shared char's steward (`owner_id`) IS the evicted owner `X`, **repoint `owner_id` to a remaining (non-evicted) sharer** so the char keeps a live steward — preferred: the most-recent `attempting_owner_id` from its `cross_owner_write` rows that is neither `X` nor itself an evicted owner. **Acceptable fallback** if the repoint mechanism proves fiddly: leave `owner_id = X` (cosmetic only — `owner_id` is a non-binding steward marker since `260621-u6j`) but still skip removal. The HARD requirement is "the shared char survives"; the repoint is clean-data polish. (Planner/researcher picks the final mechanism that keeps tests simplest.)

### Preview / picker parity + the all-shared-owner UI (success criterion 3 + the load-bearing edge case)
- **D-04:** The eviction cascade and the preview MUST be backed by ONE shared SQL fragment so the preview can never claim a different set than the action removes (the explicit lesson from Phase 35's CR-01, where the picker read and the write path diverged). The preview must surface BOTH the chars that **will be removed** (the D-02 narrowed, sole-owned set) AND the fact that **shared chars are preserved + the code will be revoked** — concretely, the preview response carries the removed-set PLUS a signal (a preserved-shared count, or the full live set flagged removed-vs-preserved) so the UI is **never empty for an owner who still has live characters**. `ListEvictableOwners` keeps listing any owner with ≥1 **live** character. The **restore path is unaffected** (shared chars never enter grace).
- **D-06 (web — the all-shared-owner fix):** `web/src/lib/components/EvictionForm.svelte` currently disables Evict when `preview.characters.length === 0` (`cascadeEmpty`). After the preview narrows, an **all-shared owner** (every live char shared) produces an empty removed-set — the button must STILL be enabled so the officer can revoke their code. Update the form so eviction is allowed when the removed-set is empty BUT the owner still has live (preserved-shared) characters, rendering the explicit "0 characters removed; {N} shared characters preserved; guild code will be revoked" framing. Keep the genuine "owner has zero live chars at all" case disabled (unchanged). **Browser-smoke the eviction flow on prod** after deploy (node vitest is DOM-blind — [[web-tests-node-only-blind-to-dom]]); officer-auth required ([[web-local-dev-cant-auth-against-prod]]).

### Migration + deploy
- **D-05:** Phase 36 ships with **NO schema migration** — detection is computed on read from `audit_log` + `character`. Schema stays **v15**. **Watcher UNTOUCHED**; **no `v*` tag** (consistent with v2.3/v2.4/v2.5 framing — the milestone's one migration was `00015` in Phase 35). The change is **backend + web** (NOT backend-only): the backend binary swaps in and the web build deploys via the existing web atomic-swap path (Claude drives the prod deploy via the ssh-agent workaround; no goose-run since no migration).

### Claude's Discretion
- Exact SQL shape of the "shared" predicate (a `NOT EXISTS` correlated subquery vs an anti-join), and whether to add a covering index on `audit_log(char_name)` — research/plan decide; evictions are rare officer actions over a ~12-person guild, so an unindexed `char_name` scan is acceptable if simpler.
- Whether the narrowed predicate lives in one reusable SQL fragment shared by `EvictOwnerTx` + `PreviewEviction` (preferred — single source of truth so preview can never drift from the action, the lesson from Phase 35's CR-01) or is duplicated.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirement + roadmap
- `.planning/REQUIREMENTS.md` — OWN-03 (the phase's sole requirement) + the v2.5 out-of-scope list (no watcher change; do NOT re-introduce cross-owner rejection).
- `.planning/ROADMAP.md` §"Phase 36: Shared-character-safe eviction" — goal + the 4 success criteria.

### Code this phase modifies / reads
- `internal/backendsrv/store/eviction.go` — `EvictOwnerTx` (the `WHERE owner_id=? AND is_removed=0` cascade to narrow), `PreviewEviction`, `ListEvictableOwners`, `RestoreOwnerTx`, `ListRestorableOwners`, `ArchiveExpiredEvictions`, `EvictionGraceSeconds`, the Phase-35 `ErrCannotEvictSentinel` guard.
- `internal/backendsrv/store/binding.go` — the `cross_owner_write` audit source (`auditCrossOwnerWrite`); confirms `owner_id` is a non-binding steward marker and cross-owner uploads are allowed + audited.
- `internal/backendsrv/migrations/00002_audit.sql` — the `audit_log` schema (`id, event, char_name, attempting_owner_id, current_owner_id, created_at`); `char_name` is plain `TEXT` (mind COLLATE NOCASE when joining `character.name`).
- `internal/backendsrv/store/owner.go` — `GuildSentinelOwnerID` (banks/bots are already eviction-safe; Phase 36 does not re-litigate bank survival).
- `internal/backendsrv/webadmin/eviction.go` — `EvictHandler` / the eviction-preview handler / `RestoreHandler` and `mapEvictionErr` (the HTTP surface; the preview JSON contract gains the preserved-shared signal — confirm the new field is additive/snake_case).
- `internal/backendsrv/store/eviction_test.go` — the `insertOwner/insertChar/insertGuildCode/charState/commitTx` helpers + `TestEvictOwnerTx_*` shapes to mirror for the new shared-survival tests.
- `web/src/lib/components/EvictionForm.svelte` — the officer eviction form; `cascadeEmpty`/`canEvict` (lines 69–73) gate the Evict button on a non-empty preview; D-06 must keep an all-shared owner evictable (code-only revoke) with messaging.
- `web/src/lib/api.ts` (the `EvictionPreview`/`EvictableOwner` interfaces) + `web/src/lib/eviction.ts` (any pure preview helpers) — mirror the new preview field field-for-field (snake_case); add node tests for any new pure logic.

### Phase-35 foundation
- `.planning/phases/35-owner-less-guild-banks-bots/35-01-SUMMARY.md` — the sentinel-owner model Phase 36 builds on (banks/bots already skip the cascade by construction).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`audit_log` `cross_owner_write` rows** — already capture (char_name, attempting_owner_id, current_owner_id) for every cross-owner upload since `260621-u6j`. This IS the sharing signal; no new write path needed.
- **`EvictOwnerTx` tx shape** — already takes the caller's `*sql.Tx` and composes evict + code-revoke + (handler-side) audit row in one tx; the narrowed predicate slots into the existing `UPDATE … WHERE owner_id=?` and the same tx.
- **`charState` / `commitTx` test helpers** (eviction_test.go) — directly reusable for asserting a shared char stays `is_removed=0` while a sole-owned char flips to 1.

### Established Patterns
- **Parameterized `?` placeholders only (V5).** No string-interpolated SQL.
- **Single source of truth for a predicate** — Phase 35's CR-01 BLOCKER came from the eviction *picker* and the *write path* diverging. Apply the lesson: the "is this char removable?" predicate should be shared by `EvictOwnerTx` and `PreviewEviction` so the preview can never claim a different set than the action removes.
- **Forward-only, extend-only** schema convention (if any migration were added — but D-05 says none).

### Integration Points
- `webadmin/eviction.go` `EvictHandler`/`PreviewEviction` consume the store functions; narrowing the store layer flows up without a handler signature change (confirm the JSON preview shape still satisfies the UI).
- The grace → `ArchiveExpiredEvictions` lifecycle is untouched (shared chars never enter grace, so they never archive via eviction).
</code_context>

<specifics>
## Specific Ideas

- **Concrete acceptance scenario (the originating incident):** a character played by two guildies (e.g. the Kim/Aenriel-class shared-login case, or any char with a `cross_owner_write` row). Evicting the first-uploader must leave that character live (`is_removed=0`), while a character ONLY that member ever uploaded is removed + grace-stamped as today. Banks like Findom are already covered by Phase 35's sentinel and are not in scope here.
- **Key edge case for research/plan:** an owner whose *every* live char is shared. They must still be evictable (the eviction revokes their guild_code) but `removedCount` = 0 and the preview says "0 characters removed; guild code revoked." Do NOT let a narrowed `char_count` drop them out of `ListEvictableOwners`.
</specifics>

<deferred>
## Deferred Ideas

- **First-class sharing model** (a `character_uploader` table populated at bind time + backfilled) — a cleaner long-term representation than deriving from `audit_log`, but unnecessary for OWN-03 and out of scope. Note for a future milestone if sharing needs richer queries.
- **Backfill of pre-`260621-u6j` shares** — the `cross_owner_write` trail only exists since 2026-06-21, so shares from before then aren't recorded. Accepted: the guild is ~12 people, the feature is days old, and the known incidents are already handled (Findom is now a sentinel bank via Phase 35). Not worth a historical reconstruction.
- **Self-service eviction** (backlog 999.5) — already deferred at the milestone level; unrelated.

### Reviewed Todos (not folded)
None — no pending todos matched Phase 36's scope.
</deferred>

---

*Phase: 36-shared-character-safe-eviction*
*Context gathered: 2026-06-22*
