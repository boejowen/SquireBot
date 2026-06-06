-- +goose Up
-- Phase 21 plan 21-01 (WANT-05). The EC-tunnel auction diff cursor:
-- ec_auction_cursor (per-item last-seen auction timestamp). The producer job
-- (Plan 03) reads it before a poll (absent row ⇒ first-sight baseline, no DM)
-- and advances it ONLY after a successful per-item poll (advance-only-on-success,
-- no backlog replay on restart). Forward-only; 00001-00007 are SHIPPED and NOT
-- edited.
--
-- Backend-only: the watcher never reads/writes this table, so there is NO
-- _meta.schema_version bump and NO WatcherMaxSchemaVersion change (the watcher
-- targets the ingest API, not these backend tables).

CREATE TABLE ec_auction_cursor (
  item_id     INTEGER PRIMARY KEY,   -- the stable join key (CLAUDE.md); one row per polled item
  last_seen_t TEXT NOT NULL,         -- max(ItemAuctionDetail.t) seen, RFC3339 date-time string
  updated_at  INTEGER NOT NULL       -- unix epoch secs
);

-- +goose Down
-- Forward-only in practice (mirrors 00004/00005/00006/00007): explicit no-op.
SELECT 1;
