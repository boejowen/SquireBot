-- +goose Up
-- Phase 41 plan 41-01 (CHARUI-02). The optional per-character portrait photo: a
-- dedicated side table for the image bytes so the hot roster/inventory reads never
-- pull the blob (D-02) — one portrait per char (character_id PK/FK), the sniffed
-- content_type + byte_size, and an ISO updated_at that doubles as the ?v= cache-bust
-- key (D-07). Forward-only; 00001-00018 are shipped and NOT edited.
--
-- Backend-only: the watcher never reads/writes this table (it is OFF the read path),
-- so there is NO WatcherMaxSchemaVersion gate to touch (that gate does not exist in
-- the off-Google backend) and NO _meta.schema_version cell (none exists here) —
-- goose version() is the version of record. "Schema v19" == goose migration 00019
-- applied. The image bytes NEVER cross the ingest API; this is purely a web-write
-- surface (webadmin base64-in-JSON upload) + a login-gated serve endpoint.

-- D-02: one portrait per character, keyed on character_id (PK ⇒ single-row-per-char
-- is the schema, not store logic). ON DELETE CASCADE so an archived/removed character
-- row drops its portrait automatically (the 00009 character_assignment side-table
-- precedent). content_type is SNIFFED server-side from the magic bytes, NEVER the
-- client claim (D-04). updated_at is TEXT ISO to match the character.last_seen/
-- created_at convention (00001_init.sql) and pass straight through to the wire ?v=.
CREATE TABLE character_portrait (
  character_id INTEGER PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE,  -- one portrait per char (side table, D-02)
  image_blob   BLOB NOT NULL,
  content_type TEXT NOT NULL,   -- sniffed server-side, NEVER the client claim (D-04)
  byte_size    INTEGER NOT NULL,
  updated_at   TEXT NOT NULL    -- ISO cache-bust key for ?v= (D-07); matches character.last_seen TEXT convention
);

-- +goose Down
-- Forward-only in practice (mirrors 00004-00018 no-op/drop downs).
DROP TABLE character_portrait;
