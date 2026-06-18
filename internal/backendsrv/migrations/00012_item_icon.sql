-- +goose Up
-- Phase 31 (INV-04, D-01/D-02/D-03). Forward-only; 00001-00011 are shipped and NOT
-- edited. SQLite permits only ONE column per ALTER TABLE ADD COLUMN; the added
-- column is nullable (no DEFAULT needed) and carries no UNIQUE/PK constraint
-- (the 00003 pattern). icon_id is the P1999 wiki icon id (lucy_img_ID), NULL until
-- the weekly wiki job enriches the row — coverage grows incrementally (D-02), and
-- a NULL surfaces as 0 in compute, which the client renders as the colored-tile
-- fallback. Read-only additive column: the watcher is OFF the read path (untouched
-- this phase), so NO WatcherMaxSchemaVersion gate is touched and goose version() is
-- the version of record (no _meta.schema_version cell exists in this backend).
ALTER TABLE item_master ADD COLUMN icon_id INTEGER;

-- +goose Down
ALTER TABLE item_master DROP COLUMN icon_id;
