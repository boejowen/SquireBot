-- +goose Up
-- Phase 35 (OWN-01/02/04). Make a designated guild bank/bot GUILD-HELD rather than
-- tied to whoever first uploaded it. Resolved design = Option A: a reserved "guild"
-- sentinel owner row (id 1000000, label 'guild') that holds owner-less banks/bots,
-- NOT a nullable character.owner_id (Option B would require a 12-table NOT-NULL-drop
-- rebuild + every owner_id consumer learning to handle NULL). Forward-only;
-- 00001-00014 unedited.
--
-- The sentinel id is the FIXED literal 1000000 — one million, far above the organic
-- owner autoincrement range at a guild scale of ~12 owners, so it can never collide
-- with a real INSERTed owner row (owner.id is INTEGER PRIMARY KEY, so an explicit id
-- is allowed). This literal MUST equal store.GuildSentinelOwnerID.
--
-- Backend-only: the watcher never reads/writes owner/character.owner_id directly (it
-- targets the ingest API), so there is NO _meta.schema_version bump and NO
-- WatcherMaxSchemaVersion change (that gate does not exist in the off-Google backend).
-- "Schema v15" == goose migration 00015 applied (goose_db_version is the record).
--
-- IRREVERSIBLE (WR-02): step 2's backfill OVERWRITES character.owner_id for every
-- bank/bot, DISCARDING the original first-uploader binding. That pre-backfill owner_id
-- is the ONLY state this migration destroys, and it is NOT recoverable from the DB:
-- the Down step is the project's forward-only `SELECT 1;` no-op (mirrors 00004-00014),
-- so `goose down` does NOT restore the old binding — it only moves goose_db_version
-- backward while the overwritten owner_ids stay overwritten. Recovery of a
-- mis-designated bank's original owner is "restore from the R2 backup taken before the
-- 00015 deploy", NOT `goose down`. Do not trust `down` to roll this back.

-- 1. Seed the reserved guild sentinel owner (replay-safe on the PK). Do NOT set
--    discord_user_id: the sentinel maps to no Discord user — 00005's partial-unique
--    index only constrains non-NULL values, so a NULL is the correct "owner-less"
--    semantics.
INSERT OR IGNORE INTO owner (id, label) VALUES (1000000, 'guild');

-- 2. Backfill (OWN-04): repoint every existing designated bank/bot to the sentinel,
--    automatically — no manual fixup (the Findom->owner 9 case). The `owner_id <> 1000000`
--    guard makes the UPDATE touch zero rows on replay (idempotent). NO is_removed filter:
--    a soft-removed bank is still guild-held, and later re-designation still works.
UPDATE character
   SET owner_id = 1000000
 WHERE (is_bank_toon = 1 OR is_guild_bot = 1)
   AND owner_id <> 1000000;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00014): explicit no-op. NOTE this is NOT a
-- rollback — see the IRREVERSIBLE note in the Up header: the original first-uploader
-- owner_id is gone after the Up backfill and is NOT restored here (recovery = R2 backup).
SELECT 1;
