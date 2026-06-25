-- +goose Up
-- Phase 37 (ENRICH-12 flags + ENRICH-13 Clicky/Haste effects). Forward-only;
-- 00001-00015 are shipped and NOT edited. SQLite permits only ONE column per
-- ALTER TABLE statement, so each of the nine derived flag/effect fields gets its
-- own ALTER. Every added column is nullable (no DEFAULT) and carries no UNIQUE/PK
-- constraint (the 00012/00013 pattern). These fields were ALREADY parsed by the wiki
-- item parser but discarded by the historical D-8 Sheet-parity scope guard; Plan
-- 37-01 re-surfaced them on ParsedWikiItem and this migration persists them as
-- discrete, individually-queryable columns. NULL until the weekly wiki job re-enriches
-- the row OR the one-time boot backfill re-parses the stored statsblock TEXT (D-05) —
-- a NULL flags_json marks a not-yet-backfilled row (the backfill's idempotency key).
-- flags_json holds the FULL detected all-caps flag SET as a JSON array (D-03), so a
-- future flag (Attunable, No Rent, Artifact, …) needs NO new migration.
--
-- Read-only additive columns: the watcher is OFF the read path (untouched this phase),
-- so NO WatcherMaxSchemaVersion gate is touched (that gate does not exist in the
-- off-Google backend) and goose version() is the version of record (no _meta.schema_version
-- cell exists in this backend).
ALTER TABLE item_master ADD COLUMN is_lore INTEGER;
ALTER TABLE item_master ADD COLUMN is_no_drop INTEGER;
ALTER TABLE item_master ADD COLUMN is_magic INTEGER;
ALTER TABLE item_master ADD COLUMN is_temporary INTEGER;
ALTER TABLE item_master ADD COLUMN is_clicky INTEGER;
ALTER TABLE item_master ADD COLUMN clicky_effect TEXT;
ALTER TABLE item_master ADD COLUMN has_haste INTEGER;
ALTER TABLE item_master ADD COLUMN haste_pct INTEGER;
ALTER TABLE item_master ADD COLUMN flags_json TEXT;   -- D-03: the full detected flag SET as a JSON array; no future migration for a new flag

-- +goose Down
ALTER TABLE item_master DROP COLUMN flags_json;
ALTER TABLE item_master DROP COLUMN haste_pct;
ALTER TABLE item_master DROP COLUMN has_haste;
ALTER TABLE item_master DROP COLUMN clicky_effect;
ALTER TABLE item_master DROP COLUMN is_clicky;
ALTER TABLE item_master DROP COLUMN is_temporary;
ALTER TABLE item_master DROP COLUMN is_magic;
ALTER TABLE item_master DROP COLUMN is_no_drop;
ALTER TABLE item_master DROP COLUMN is_lore;
