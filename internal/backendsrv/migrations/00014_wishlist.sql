-- +goose Up
-- Phase 34 (WISH-01..07). The per-character / per-slot upgrade wishlist replaces the retired
-- item-centric wantlist_item (D-01 clean break). Forward-only; 00001-00013 unedited.
-- Identity = web_user.discord_user_id (the PERSON, the DM target — the wantlist precedent).
-- character_id is NOT NULL (every target is char+slot-scoped). slot is the canonical Title-case
-- worn-slot token ("Head"/"Finger1"/"Primary") — the compute.classifySlot vocabulary.
CREATE TABLE wishlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  character_id    INTEGER NOT NULL REFERENCES character(id),
  slot            TEXT NOT NULL,
  item_id         INTEGER,                       -- NULL ⇒ typed/custom OR a gear-tier item (no id)
  item_name       TEXT NOT NULL,
  pinged          INTEGER NOT NULL DEFAULT 1,    -- WISH-05 ping toggle (default-ON, inverse of muted; Pitfall 8)
  active          INTEGER NOT NULL DEFAULT 1,    -- soft-delete (explicit remove)
  created_at      INTEGER NOT NULL
);
CREATE INDEX wishlist_user_idx    ON wishlist_item(discord_user_id);
CREATE INDEX wishlist_char_idx    ON wishlist_item(character_id);
CREATE INDEX wishlist_item_id_idx ON wishlist_item(item_id);
CREATE UNIQUE INDEX wishlist_catalog_uidx ON wishlist_item(discord_user_id, character_id, slot, item_id)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wishlist_custom_uidx  ON wishlist_item(discord_user_id, character_id, slot, item_name) WHERE item_id IS NULL     AND active = 1;

-- D-01 / Pitfall 4: rebuild alert_log to FK wishlist_item(id). 00007 made it FK wantlist_item(id).
-- KEEP the column name wantlist_item_id (Pitfall 6 option B) so store/alertlog.go needs NO edit;
-- only the FK TARGET table changes. Clean break: copy no rows. SQLite can't ALTER a FK → DROP+CREATE.
DROP INDEX IF EXISTS alert_log_dedup_idx;
DROP TABLE alert_log;
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wantlist_item_id INTEGER REFERENCES wishlist_item(id) ON DELETE CASCADE,  -- target now wishlist_item; NULLABLE (test-alert)
  discord_user_id  TEXT NOT NULL,
  source           TEXT NOT NULL,
  item_id          INTEGER,
  detail           TEXT,
  sent_at          INTEGER NOT NULL,
  send_status      TEXT NOT NULL,
  read_at          INTEGER
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wantlist_item_id, source, item_id, sent_at);

-- D-01: drop the retired wantlist_item + its indexes (AFTER the alert_log FK no longer points at it).
DROP INDEX IF EXISTS wantlist_user_idx;
DROP INDEX IF EXISTS wantlist_item_id_idx;
DROP INDEX IF EXISTS wantlist_catalog_uidx;
DROP INDEX IF EXISTS wantlist_custom_uidx;
DROP TABLE wantlist_item;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00013): explicit no-op.
SELECT 1;
