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
