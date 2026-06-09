-- +goose Up
-- Phase 28 plan 28-01 (CWANT-01..04, CWANT-06). The character-tagged wantlist: an
-- OPTIONAL character_id on wantlist_item so a want can be scoped to one of the
-- caller's characters (or left account-level). Forward-only; 00001-00009 are SHIPPED
-- and NOT edited (extend-only).
--
-- Backend-only: the watcher never reads/writes wantlist_item, so there is NO
-- _meta.schema_version bump and NO WatcherMaxSchemaVersion change (the watcher
-- targets the ingest API, not these backend tables). "Schema v10" == goose
-- migration 00010 applied (goose_db_version is the version record).

-- CWANT-01/02: the optional tag. NULLABLE (no NOT NULL, no DEFAULT) ⇒ every EXISTING
-- row auto-backfills to NULL (account-level) with NO separate data-migration step. NO
-- ON DELETE CASCADE: a character row is soft-removed (is_removed=1), never hard-deleted,
-- so a cascade would never fire; a dangling tag is handled by the read-side LEFT JOIN
-- (a removed/missing char ⇒ NULL character_name), not by destroying the want.
ALTER TABLE wantlist_item ADD COLUMN character_id INTEGER REFERENCES character(id);

-- D-05 dedup rewrite (T-28-04). The 00006 partial-unique indexes keyed on
-- (discord_user_id, item_id|item_name, reason). SQLite treats NULL as DISTINCT in a
-- UNIQUE index, so a NAIVE append of character_id to the key would let two account-level
-- (character_id NULL) wants for the same (user, item, reason) coexist collision-free —
-- regressing 00006's account-level dedup. COALESCE(character_id, -1) collapses NULL to a
-- single sentinel (-1) so two account-level wants STILL collide (dedup preserved), while
-- two distinct REAL char ids stay distinct (the same item wanted for two characters is
-- two legitimate rows). The partial-WHERE clauses are UNCHANGED from 00006.
DROP INDEX wantlist_catalog_uidx;
DROP INDEX wantlist_custom_uidx;
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(
  discord_user_id, item_id, reason, COALESCE(character_id, -1)
) WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx ON wantlist_item(
  discord_user_id, item_name, reason, COALESCE(character_id, -1)
) WHERE item_id IS NULL AND active = 1;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00009): explicit no-op. NO DROP COLUMN.
SELECT 1;
