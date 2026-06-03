-- +goose Up
-- Phase 19 plan 19-01 (WANT-01, WANT-02). The personal, owner-scoped wantlist:
-- wantlist_item (the CRUD surface) + alert_log (Phase-20 stub, ZERO rows written
-- this phase). Forward-only; 00001-00005 are SHIPPED and NOT edited.
--
-- Identity is web_user.discord_user_id (the PERSON, not the watcher `owner` —
-- 19-RESEARCH Pitfall 3): a wantlist belongs to a logged-in member and is the DM
-- target Phase 20 keys on, so it must work even before that member links a watcher.
--
-- D-05 dedupe (19-RESEARCH Pitfall 1): SQLite treats NULL as DISTINCT in a UNIQUE
-- index, so a single UNIQUE(discord_user_id, item_id, reason) would NOT dedupe
-- custom wants (item_id NULL). Two PARTIAL unique indexes split the cases —
-- catalog wants key on item_id, custom wants key on item_name — BOTH scoped
-- `WHERE active = 1` so a removed-then-re-added want never collides with its own
-- soft-deleted tombstone.
--
-- The reason/priority CHECK constraints are DB-level defense-in-depth (review #5):
-- even if a future non-handler writer (Phase 20+) bypasses the handler's validWant,
-- a bad enum (reason='maybe', priority='urgent') is rejected at the database.
CREATE TABLE wantlist_item (
  id              INTEGER PRIMARY KEY,
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  item_id         INTEGER,                       -- NULL ⇒ custom want (D-04)
  item_name       TEXT NOT NULL,                 -- snapshot: catalog name OR custom label
  reason          TEXT NOT NULL CHECK (reason IN ('buy','quest')),                 -- 'buy' | 'quest' (D-01); CHECK = DB-level defense-in-depth (review #5)
  priority        TEXT NOT NULL DEFAULT 'med' CHECK (priority IN ('low','med','high')),   -- 'low' | 'med' | 'high' (D-01/D-02); CHECK = DB-level defense-in-depth (review #5)
  note            TEXT,                          -- optional, ≤280 chars (handler-enforced)
  active          INTEGER NOT NULL DEFAULT 1,    -- soft-delete (Pitfall 4)
  created_at      INTEGER NOT NULL               -- unix epoch secs (nowUnix())
);
CREATE INDEX wantlist_user_idx    ON wantlist_item(discord_user_id);
CREATE INDEX wantlist_item_id_idx ON wantlist_item(item_id);
CREATE UNIQUE INDEX wantlist_catalog_uidx ON wantlist_item(discord_user_id, item_id, reason)   WHERE item_id IS NOT NULL AND active = 1;
CREATE UNIQUE INDEX wantlist_custom_uidx  ON wantlist_item(discord_user_id, item_name, reason) WHERE item_id IS NULL     AND active = 1;

-- alert_log: created here at its FULL Phase-20 shape so Phase 20 adds zero
-- migration churn, but Phase 19 writes ZERO rows to it (the migrate test asserts
-- COUNT(*) == 0).
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wantlist_item_id INTEGER NOT NULL REFERENCES wantlist_item(id) ON DELETE CASCADE,
  discord_user_id  TEXT NOT NULL,
  source           TEXT NOT NULL,
  item_id          INTEGER,
  detail           TEXT,
  sent_at          INTEGER NOT NULL,
  send_status      TEXT NOT NULL
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wantlist_item_id, source, item_id, sent_at);

-- pigparse_name_idx: NOTE this index does NOT speed up the SearchCatalog query.
-- The search uses a leading-wildcard `name LIKE '%…%'` plus `CAST(item_id AS TEXT) = ?`,
-- neither of which a B-tree on name(COLLATE NOCASE) can serve — SQLite full-scans
-- pigparse_price regardless (review WORTH-FIX 7). It is kept ONLY as a mild assist for
-- any future prefix/exact-name lookup; it is NOT a DoS mitigation. If you ever need real
-- substring perf, move to FTS5 — do not assume this index helps the current query.
CREATE INDEX pigparse_name_idx ON pigparse_price(name COLLATE NOCASE);

-- +goose Down
-- Forward-only in practice (mirrors 00004/00005): explicit no-op.
SELECT 1;
