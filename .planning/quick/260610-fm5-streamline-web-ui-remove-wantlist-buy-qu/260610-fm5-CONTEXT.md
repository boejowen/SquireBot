# Quick Task 260610-fm5: Streamline web UI - Context

**Gathered:** 2026-06-10 (from a completed 4-agent audit: extraneous-info sweep, placement audit, reason-removal impact map, deploy-procedure verification)
**Status:** Ready for planning. ALL decisions below are LOCKED — do not revisit, do not re-audit.

<domain>
## Task Boundary

Three workstreams, all user-requested ("make the UI as simple and streamlined as possible; no extraneous info e.g. no itemIDs; similar features in similar places; remove the want/buy designation when adding a wantlist item"):

- **WS1 — Remove the wantlist buy/quest "Reason" field end-to-end** (web UI + Go API + SQLite migration 00011 + all affected tests).
- **WS2 — Strip extraneous technical info from member-facing UI** (six item-ID surfaces, raw enum leaks, jargon).
- **WS3 — Clear-win feature-placement consistency fixes** (nav, scope filters, grid toolbar, cross-links, headings, dead-end page).

Deploy to prod is handled by the ORCHESTRATOR after execution — the plan/executors cover code + local gates only. Do NOT include deploy steps in the plan tasks.
</domain>

<decisions>
## Implementation Decisions (ALL LOCKED)

### WS1 — Reason removal: exact file-by-file change map

**Web (SvelteKit, Svelte 5 runes):**

1. `web/src/lib/components/WantAddForm.svelte`
   - L6 comment: 'Reason/Priority/Note detail fields' → 'Priority/Note detail fields'.
   - L49: delete `let reason = $state<'' | 'buy' | 'quest'>('');`
   - L80-82: `canSubmit` drops the reason term → `let canSubmit = $derived(staged && !adding);` (update comment).
   - L132 (resetStaging): delete `reason = '';`
   - L154: delete `reason: reason as 'buy' | 'quest',` from the addWant body.
   - L166-168: duplicate-error copy → `"That's already on your wantlist."` (delete the '(The same item can be on twice — once to buy, once for a quest.)' parenthetical).
   - L242-248: delete the entire Reason `<FormField>` block (select with Buy/Quest options).
2. `web/src/lib/columns.ts`
   - L38: delete `import ReasonCell from '$lib/components/cells/ReasonCell.svelte';`
   - L173 + L293-294 doc comments: drop '· Reason'.
   - wantlistColumns L228-234: delete the whole `{ id: 'reason', ... ReasonCell ... }` entry.
   - guildWantlistColumns L321-327: delete the trailing `{ id: 'reason', ... }` entry (last array element — fix the preceding comma).
3. `web/src/lib/components/cells/ReasonCell.svelte` — DELETE the file (only importer is columns.ts L38, verified).
4. `web/src/lib/api.ts` — delete `reason: 'buy' | 'quest';` from `WantlistRow` (L585), `GuildWantRow` (L632), and the `addWant` body param type (L658). No wrapper logic changes.
5. `web/src/lib/wantlist/groupByChar.test.ts` — L18: delete `reason: 'buy',` from the `want()` fixture factory (type error otherwise). Only web test carrying reason (verified).
6. `web/src/lib/components/WantlistPanel.svelte` — L142 comment-only: drop '· Reason'.

**Go backend:**

7. `internal/backendsrv/webadmin/wantlist.go`
   - L48: delete `Reason string` json field from addWantReq (encoding/json silently ignores stale clients' "reason" key — Decode is used WITHOUT DisallowUnknownFields, verified L141).
   - L62-65: delete `validReasons` (keep validPriorities).
   - validWant L74-77: delete the validReasons check; update doc comments (L18/L25/L67-68/L124 mention reason enums and "SAME item with the OTHER reason is a distinct row").
   - L177: AddWantTx call drops `req.Reason` arg.
   - L196: delete `Reason: req.Reason,` from the echoed store.WantlistRow literal.
8. `internal/backendsrv/store/wantlist.go`
   - L56: delete `Reason` field from WantlistRow struct; L166: same for GuildWantRow.
   - AddWantTx (L77): drop the `reason` param → `func AddWantTx(ctx, tx, discordID string, itemID *int64, itemName, priority string, note *string, characterID *int64, now int64)`.
   - INSERT L79-81: KEEP the reason column, write the literal `'buy'`: `INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, character_id, created_at) VALUES (?, ?, ?, 'buy', ?, ?, ?, ?)` — the column is NOT NULL CHECK with no DEFAULT, so it MUST still be supplied ('buy' satisfies the retained CHECK). **Forgetting this = instant 500 on every add.**
   - ErrDuplicateWant detection (SQLite extended code 2067, L86-89): mechanically UNCHANGED.
   - ListOwnWants SELECT L106: drop `w.reason,`; Scan L127: drop `&r.Reason`. ListGuildWants SELECT L184 + Scan L204: same.
   - L36-38 ErrDuplicateWant doc: '(user,item,reason)' → '(user,item[,char])'.
9. `internal/backendsrv/wantmatch/match.go` — L48: delete `Reason string` from Hit. ForItem L67 + ForName L85 SELECTs: drop `w.reason,`. scanHits L110: drop `&h.Reason` (keep order: id, discord_user_id, item_id, item_name, note, character_name).
10. `internal/backendsrv/ec/embed.go` — whyWanted (L93-110) becomes Note-only:
    `if hit.Note != nil { if note := strings.TrimSpace(*hit.Note); note != "" { return note } }; return "on your wantlist"` — keep the never-empty contract so the 'Why you wanted it' embed field stays always-present (L145-150 unchanged). Update doc comments L12-13 and L93-95.

**Migration (NEW file `internal/backendsrv/migrations/00011_wantlist_drop_reason_dedup.sql`):**

Current state at v10 (verified): 00010 already recreated the two partial unique indexes as `wantlist_catalog_uidx(discord_user_id, item_id, reason, COALESCE(character_id,-1)) WHERE item_id IS NOT NULL AND active=1` and `wantlist_custom_uidx(discord_user_id, item_name, reason, COALESCE(character_id,-1)) WHERE item_id IS NULL AND active=1`. The `muted` column exists (00007). The `reason` column STAYS (SQLite can't drop a CHECK-referenced column); store writes 'buy' forever.

**CRITICAL PIN: the recreated indexes and the dedupe pass MUST keep the `COALESCE(character_id,-1)` term.** Dropping it would deactivate legitimate per-character-tagged wants, break CWANT-01..06, and fail TestMigrate_00010 steps 3/4/7.

Exact SQL (goose style mirrors 00006-00010: plain statements, `-- +goose Up` / `-- +goose Down` with `SELECT 1;` no-op Down):

```sql
-- +goose Up
-- Drop the buy/quest reason from the wantlist dedup key. Forward-only; 00001-00010
-- are SHIPPED and NOT edited. The reason COLUMN stays (NOT NULL CHECK cannot be
-- altered away in SQLite); the store now always writes 'buy'. Backend-only: NO
-- _meta.schema_version bump and NO WatcherMaxSchemaVersion change (watcher never
-- touches wantlist_item — the 00008/00010 precedent).
--
-- 1) DATA: deactivate (soft-delete, active=0 — never DELETE; alert_log FKs these
-- rows) the newer of any active rows that collide once reason leaves the key,
-- keeping MIN(id) per (user, item, COALESCE(character_id,-1)) — catalog — and per
-- (user, item_name, COALESCE(character_id,-1)) — custom. MUST run BEFORE the new
-- unique indexes are created.
UPDATE wantlist_item SET active = 0
 WHERE active = 1 AND item_id IS NOT NULL
   AND id NOT IN (
     SELECT MIN(id) FROM wantlist_item
      WHERE active = 1 AND item_id IS NOT NULL
      GROUP BY discord_user_id, item_id, COALESCE(character_id, -1));

UPDATE wantlist_item SET active = 0
 WHERE active = 1 AND item_id IS NULL
   AND id NOT IN (
     SELECT MIN(id) FROM wantlist_item
      WHERE active = 1 AND item_id IS NULL
      GROUP BY discord_user_id, item_name, COALESCE(character_id, -1));

-- 2) INDEXES: recreate WITHOUT reason, KEEPING the 00010 COALESCE(character_id,-1)
-- term and the unchanged partial WHERE clauses.
DROP INDEX wantlist_catalog_uidx;
DROP INDEX wantlist_custom_uidx;
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(
  discord_user_id, item_id, COALESCE(character_id, -1)
) WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx ON wantlist_item(
  discord_user_id, item_name, COALESCE(character_id, -1)
) WHERE item_id IS NULL AND active = 1;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00010): explicit no-op.
SELECT 1;
```

**Go tests:**

11. `internal/backendsrv/webadmin/wantlist_test.go` — drop `"reason":"…",` from every postJSON body (L99, L138, L158, L181, L185, L227, L263, L300-304, L335, L339, L347, L353, L356, L384, L429, L496). DELETE the `{"bad reason", …, 400}` validation case (L299). L235: drop `row.Reason != "buy"` from the echo assertion. seedWant helper L31-36: drop the reason param, hardcode `'buy'` in the raw INSERT (callers L388, L409, L474). REWRITE TestAddWant_Duplicate_409_OtherReason_200 (L327-363): same-item-other-reason re-add now expects 409 (rename to TestAddWant_Duplicate_409); custom-path dup subcase unchanged except bodies. Header comment L7-9 updated.
12. `internal/backendsrv/store/wantlist_test.go` — 16 AddWantTx call sites drop the reason arg (L41, L76, L79, L83, L110, L174, L182, L191, L209, L235, L296, L324, L367, L371, L423, L432). L56: drop `r.Reason != "buy"`. TestAddWantTx_DuplicateReturnsTypedSentinel L189-199: the 'SAME item, DIFFERENT reason → inserts fine' block INVERTS — second add of same (user,item) must now return ErrDuplicateWant; ListOwnWants count expectation 2 → 1. Comment rewording L180/L186/L429/L436.
13. `internal/backendsrv/wantmatch/match_test.go` — L151: drop `|| h.Reason != "buy"`. Comment rewording L122-123/L235-236. Raw-SQL seed helpers keep their `'buy'` literal (column persists) — no functional change.
14. `internal/backendsrv/ec/embed_test.go` — delete `Reason:` from wantmatch.Hit literals (L27, L50, L69). No assertion changes.
15. `internal/backendsrv/ec/ec_test.go` — delete `Reason:` from Hit literals (L521, L552). Why-you-wanted-it expectation `"buy — " + note` → just `note` (L544-546); update comment L514. Verify no assertion depends on 'quest'. seedWant raw SQL keeps `'buy'` — no change.
16. `internal/backendsrv/migrations/migrate_test.go`
    - TestMigrate_00006 and TestMigrate_00010 pass UNCHANGED given the COALESCE pin (verified); reword reason-mentioning comments in TestMigrate_00010 (L718-725, L780, L789-792, L813-818).
    - ADD TestMigrate_00011: (a) `SELECT sql FROM sqlite_master WHERE name IN ('wantlist_catalog_uidx','wantlist_custom_uidx')` — assert neither contains 'reason'; (b) a 'buy' + 'quest' insert pair for same (user,item,NULL char) collides on the second insert; (c) dedupe-data proof: migrate UpTo v10 → seed cross-reason duplicates → UpTo v11 → assert only MIN(id) stays active=1. NewTestDB always migrates to HEAD, so (c) REQUIRES calling goose directly (goose.SetBaseFS/SetDialect + UpTo) or adding a small test-only UpTo helper to migrations/embed.go — either is acceptable; follow whichever migrate_test.go's existing style supports best.

**Verified NOT impacted (do not touch):** notify/dm_test.go, store/eccursor_test.go + eccursor.go (raw 'buy'/'quest' literals stay valid; optional vestigial-comment cleanup ONLY if trivial), bot/bot.go + scheduler/scheduler.go (slog fields named 'reason' are unrelated), cmd/squirebot-server/main_test.go, web holders.ts/.test.ts, api.test.ts, columns.test.ts, MonitorAdminPanel.test.ts. The 'reason' identifiers in AdminMgmtForm/EvictionForm/BankCoinForm/CharMetaForm/MyCharactersPanel/AssignmentAdminPanel/WatcherCodesPanel/MonitorAdminPanel are local error-copy helpers — UNRELATED.

### WS2 — Extraneous-info strip (member-facing only)

1. `web/src/lib/columns.ts` — remove viewColumns `id` column (L90; bankColumns shares it). Update the header doc comments (L68, L74-80) that mention the ID column and its global-filter exclusion.
2. `web/src/lib/columns.ts` — remove wantlistColumns `item_id` column (L220-227). Update doc comments mentioning it (L173-176, L193-197).
3. `web/src/lib/components/SearchResults.svelte` — remove the `#{g.itemId}` span (L79) + now-unused `.item-id` style.
4. `web/src/lib/components/cells/WantItemCell.svelte` — remove the `#{row.item_id}` rendering (L22) + its style; keep the custom-want chip branch untouched.
5. `web/src/lib/components/WantAddForm.svelte` — remove the result `#{item.item_id}` span (L208) and staged `#{pickedItem.item_id}` span (L229) + `.result-id`/`.staged-id` styles.
6. `web/src/lib/components/NotificationRow.svelte` —
   - L101 fallback `Alert from ${row.source}` → friendly map: `ec_auction` → 'EC auction alert', `wts` → 'WTS alert', `raid_target` → 'Raid target alert', unknown → 'SquireBot alert'.
   - L42 deliveryBadge: 'ERROR' → 'NOT SENT' (keep the --status-other treatment).
   - Check any NotificationRow/inbox tests for these strings and update.
7. `web/src/lib/components/DataGrid.svelte` — facet default option `{col.id} (all)` (L127) → use the display header: `String(col.columnDef.header ?? col.id)`; apply the same to BOTH aria-labels (L122, L141).
8. `web/src/lib/tooltip/composeNotes.ts` — L151 'Quest item: yes (in-game flag)' → 'Quest item'. **Update `web/src/lib/__tests__/composeNotes.test.ts` assertions on that string.**

**Explicitly KEEP (decided):** StatusCell OK/MISSING/OTHER vocabulary, LastSyncedCell date format, WatcherCodesPanel '#1' ordinals, MonitorAdminPanel channel snowflakes (admin diagnostic), eviction mint-code ops copy, PigParse mention in tooltips. **DEFERRED (not this task):** AssignmentAdminPanel snowflake→username (needs a server-side web_user join / members API — backlog follow-up).

### WS3 — Placement consistency (clear wins)

1. `SiteShell.svelte` — add a labeled nav link to `/` (label **Inventory**), styled identically to the existing nav links, ordered Inventory → Wantlist → Notifications. Wordmark keeps linking home.
2. `web/src/routes/+page.svelte` — restyle the home scope filter to the wantlist's pattern: a segmented two-button toggle (`My characters` | `Guild`) + a separate character `<select>` shown only for My characters. Default = Guild (preserves today's all-members default). Reuse the wantlist's `.seg`/`.seg-btn` styling (extract shared CSS only if trivial; duplicating the small style block is acceptable). Behavior is presentation-only — same filtering semantics as the current merged 'Show' select.
3. `web/src/lib/components/DataGrid.svelte` — hide the per-column filter row behind a 'Filters' disclosure toggle (default hidden; min-height 44px button; aria-expanded). Relabel the global filter placeholder 'Filter all columns…' → 'Filter this table…'.
4. `web/src/lib/components/NotificationPrefsPanel.svelte` — add one line under the monitor rows: "To mute alerts for a single item, use the bell next to it on your wantlist." with a link to /wantlist.
5. Purpose lines (match existing `.form-purpose` style): `/char-meta` ("Set class, level, and race so gear and spell checks work for that character.") and `/bank-coin`; add an h1 'Admin' above the /admin cards.
6. Cross-links: on `/my-characters` add 'Set class & level' link → /char-meta; on `/char-meta` add 'Claim characters' link → /my-characters. In `SettingsMenu.svelte` rename menu item 'Character details' → 'Set class & level'.
7. `/bank-coin` — add a 'Back to bank' link → `/?view=bank`; home `+page.svelte` reads the `?view=` query param on mount to seed the active tab (validate against the page's actual tab ids; ignore unknown values).
8. `web/src/routes/wantlist/+page.svelte` (+ WantlistPanel if needed) — keep intro + WantAddForm in the 720px card, but let the filter bar + grids break out to full width (mirror the home layout). Markup/CSS only.
9. `WantlistPanel.svelte` — move the remove/mute announce messages to sit directly above the grid they affect (message-adjacent-to-control standardization).

**DEFERRED (not this task):** folding /char-meta into /my-characters (medium redesign), MyCharactersPanel message reshuffle, full toolbar redesign.

### Claude's Discretion
Exact wording of purpose lines/cross-links, disclosure button styling details, and minor comment cleanups — follow existing EQ-theme tokens (`--accent`, `--panel`, `--font-display`, 44px min touch targets) and the surrounding code's comment density/idiom.
</decisions>

<specifics>
## Constraints and conventions

- Svelte 5 runes ($state/$derived/$props) + plain `{}` interpolation ONLY for names/labels (XSS boundary — the only raw-HTML sink stays ItemTooltip).
- Server-truth pattern: never optimistic-mutate; grids reload from the server.
- Go: structured slog, typed sentinel errors, withTx(BEGIN IMMEDIATE) — do not change these patterns.
- Migration is forward-only; 00001-00010 are SHIPPED and must NOT be edited.
- NO _meta.schema_version bump, NO WatcherMaxSchemaVersion change (wantlist_item is backend-only; the watcher never touches it — 00008/00010 precedent).
- Commit style: conventional commits, e.g. `feat(260610-fm5): …` / `refactor(260610-fm5): …`; atomic commit per task; code only (commit_docs=false — never commit .planning/ files).
- Work directly on master (branching_strategy none).

## Verification gates (run after the final code task)

- `cd web; npm run check` → 0 errors / 0 warnings
- `cd web; npm test` → all green (count was 303 before this task; new total may differ)
- `cd web; npm run build` → success
- repo root: `go build ./...`, `go vet ./internal/backendsrv/... ./cmd/squirebot-server/...`, `go test ./internal/backendsrv/...` → all green
</specifics>

<canonical_refs>
## Canonical References

- `docs/backend-deploy.md` — deploy runbook (orchestrator-only; not part of the plan)
- `CLAUDE.md` — project conventions
- 19-UI-SPEC / CWANT decisions referenced in code comments — the consolidated-views LOCK (single DataGrid, never per-character tabs) must hold
</canonical_refs>
