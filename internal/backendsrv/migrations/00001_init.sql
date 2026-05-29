-- +goose Up
CREATE TABLE owner (
  id          INTEGER PRIMARY KEY,
  label       TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE character (
  id            INTEGER PRIMARY KEY,
  owner_id      INTEGER NOT NULL REFERENCES owner(id),
  name          TEXT NOT NULL UNIQUE COLLATE NOCASE,  -- char name unique (D-13)
  class         TEXT,    -- nullable; set later / by backfill (P16)
  level         INTEGER,
  race          TEXT,
  is_bank_toon  INTEGER NOT NULL DEFAULT 0,
  is_hidden     INTEGER NOT NULL DEFAULT 0,
  is_removed    INTEGER NOT NULL DEFAULT 0,
  last_seen     TEXT,
  watcher_version TEXT,
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX character_owner_idx ON character(owner_id);

CREATE TABLE inventory_item (
  id           INTEGER PRIMARY KEY,
  character_id INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
  location     TEXT NOT NULL,
  name         TEXT NOT NULL,
  item_id      INTEGER,            -- 0/NULL for empty slot
  count        INTEGER NOT NULL DEFAULT 1,
  slots        INTEGER,
  row_ordinal  INTEGER NOT NULL,   -- file line order; stable display sort
  uploaded_at  TEXT NOT NULL
);
CREATE INDEX inventory_char_idx ON inventory_item(character_id);
CREATE INDEX inventory_item_idx ON inventory_item(item_id);

CREATE TABLE spellbook_entry (
  id              INTEGER PRIMARY KEY,
  character_id    INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
  level           INTEGER NOT NULL,
  name            TEXT NOT NULL,
  normalized_name TEXT NOT NULL,   -- lower(trim(name)) — P12/P14 join key to wiki_spells
  uploaded_at     TEXT NOT NULL
);
CREATE INDEX spellbook_char_idx ON spellbook_entry(character_id);
CREATE INDEX spellbook_norm_idx ON spellbook_entry(normalized_name);

CREATE TABLE guild_code (
  id          INTEGER PRIMARY KEY,
  owner_id    INTEGER NOT NULL REFERENCES owner(id),
  token_hash  BLOB NOT NULL UNIQUE,  -- sha256(plaintext); 32 bytes
  label       TEXT,
  disabled_at TEXT,                  -- NULL = active; non-NULL = revoked (D-09)
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- EMPTY dimension tables (P12 populates; P11 only creates).
CREATE TABLE item_master    (item_id INTEGER PRIMARY KEY, name TEXT, wiki_summary TEXT, wiki_url TEXT, slot TEXT, is_quest_item INTEGER NOT NULL DEFAULT 0, wikitext_sha1 TEXT, last_refreshed TEXT);
CREATE TABLE pigparse_price (item_id INTEGER PRIMARY KEY, name TEXT, current_avg REAL, blue_volume INTEGER, last_seen TEXT, direction TEXT, last_refreshed TEXT);
CREATE TABLE wiki_spells     (id INTEGER PRIMARY KEY, class TEXT NOT NULL, level INTEGER NOT NULL, spell_name TEXT NOT NULL, normalized_name TEXT NOT NULL, last_refreshed TEXT, UNIQUE(class, level, spell_name));
CREATE TABLE wiki_gear_tier  (id INTEGER PRIMARY KEY, tier TEXT NOT NULL, class TEXT NOT NULL, slot TEXT NOT NULL, item_id INTEGER, item_name TEXT, rank INTEGER, last_refreshed TEXT, UNIQUE(tier, class, slot, item_id));
CREATE TABLE quest_items     (id INTEGER PRIMARY KEY, item_id INTEGER NOT NULL, quest_name TEXT NOT NULL, source_url TEXT, source TEXT, last_refreshed TEXT, UNIQUE(item_id, quest_name));

-- +goose Down
DROP TABLE quest_items; DROP TABLE wiki_gear_tier; DROP TABLE wiki_spells;
DROP TABLE pigparse_price; DROP TABLE item_master; DROP TABLE guild_code;
DROP TABLE spellbook_entry; DROP TABLE inventory_item; DROP TABLE character; DROP TABLE owner;
