-- +goose Up
-- Phase 38 (ENRICH-14/15, D-04 name-keyed). Forward-only; 00001-00016 are shipped and NOT
-- edited. catalog_enrichment holds wiki enrichment for CATALOG-ONLY (unheld) items, keyed
-- by normalized name lower(trim(name)) — the cross-namespace bridge (PigParse catalog ids
-- != EQ inventory ids; join by name, never raw item_id). A held item's enrichment stays in
-- item_master keyed by its EQ item_id; this table holds ONLY names with no held item_master
-- row. Read-only additive table: the watcher is OFF the read path (untouched this phase), so
-- NO WatcherMaxSchemaVersion gate is touched (that gate does not exist in the off-Google
-- backend) and goose version() is the version of record. Mirrors item_master's enrichment
-- shape so Phase 39 can COALESCE held(item_master by id) UNION unheld(catalog_enrichment by
-- name) one row per item.
CREATE TABLE catalog_enrichment (
  norm_name      TEXT PRIMARY KEY,             -- lower(trim(name)) — the cross-namespace key
  name           TEXT,                         -- representative display name (first-seen casing)
  item_id        INTEGER,                      -- representative PigParse id (examine/icon URL); NOT a key
  wiki_summary   TEXT,
  wiki_url       TEXT,
  slot           TEXT,
  is_quest_item  INTEGER NOT NULL DEFAULT 0,
  wikitext_sha1  TEXT,
  icon_id        INTEGER,                       -- lucy_img_ID; NULL/0 = colored-tile fallback (00012 contract)
  statsblock     TEXT,
  is_lore        INTEGER,
  is_no_drop     INTEGER,
  is_magic       INTEGER,
  is_temporary   INTEGER,
  is_clicky      INTEGER,
  clicky_effect  TEXT,
  has_haste      INTEGER,
  haste_pct      INTEGER,
  flags_json     TEXT,                          -- full flag SET as a JSON array (no future per-flag migration — 00016 precedent)
  last_refreshed TEXT
);

-- +goose Down
DROP TABLE catalog_enrichment;
