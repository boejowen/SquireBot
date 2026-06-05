-- +goose Up
-- Phase 20 plan 20-01 (WANT-03/04/08). The notification spine's data layer:
-- notify_prefs (per-user opt-in, default-ON D-01) + guild_channel (officer-
-- registered source channels D-07/D-08) + monitor_flag (the three guild-wide
-- kill-switches, EC default-ON D-07) + alert_log.read_at (inbox read-state D-05)
-- + wantlist_item.muted (per-want mute D-09). Forward-only; 00001-00006 are
-- SHIPPED and NOT edited.
--
-- BLOCKER-1: 00006 made alert_log.wantlist_item_id NOT NULL REFERENCES
-- wantlist_item(id). The D-10 test-alert has NO wantlist_item, so the column
-- must be NULLABLE. SQLite can't ALTER away NOT NULL; alert_log has ZERO rows
-- (00006 wrote none — the 00006 migrate test asserts COUNT(*)==0; Phase 20
-- hasn't shipped) so we DROP+CREATE it here with wantlist_item_id NULLABLE +
-- read_at from the start. Forward-only: 00006 is never edited; the change lives
-- entirely in this forward migration.

CREATE TABLE notify_prefs (
  discord_user_id TEXT PRIMARY KEY REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  master INTEGER NOT NULL DEFAULT 1,   -- D-01 default-ON master toggle
  ec     INTEGER NOT NULL DEFAULT 1,   -- per-monitor: EC-tunnel auctions
  wts    INTEGER NOT NULL DEFAULT 1,   -- per-monitor: WTS cross-server
  raid   INTEGER NOT NULL DEFAULT 1,   -- per-monitor: raid targets
  updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE guild_channel (
  id          INTEGER PRIMARY KEY,
  channel_id  TEXT NOT NULL,                  -- Discord snowflake (officer-entered)
  guild_id    TEXT,                           -- best-effort; nullable
  label       TEXT NOT NULL,                  -- human server label (renders via {} auto-escape)
  monitor     TEXT NOT NULL CHECK (monitor IN ('ec_auction','wts','raid_target')),
  enabled     INTEGER NOT NULL DEFAULT 1,
  created_at  INTEGER NOT NULL
);
CREATE UNIQUE INDEX guild_channel_uidx ON guild_channel(channel_id, monitor);

-- The three guild-wide kill-switches (D-07). EC ships ON; WTS/raid ship dark.
CREATE TABLE monitor_flag (
  monitor TEXT PRIMARY KEY CHECK (monitor IN ('ec_auction','wts','raid_target')),
  enabled INTEGER NOT NULL DEFAULT 0
);
INSERT INTO monitor_flag (monitor, enabled) VALUES ('ec_auction', 1), ('wts', 0), ('raid_target', 0);

-- alert_log rebuild (BLOCKER-1): wantlist_item_id becomes NULLABLE (test-alert
-- logs NULL) and read_at is added (NULL = unread, D-05). Zero rows to copy.
DROP INDEX IF EXISTS alert_log_dedup_idx;
DROP TABLE alert_log;
CREATE TABLE alert_log (
  id               INTEGER PRIMARY KEY,
  wantlist_item_id INTEGER REFERENCES wantlist_item(id) ON DELETE CASCADE,  -- NULLABLE (test-alert = NULL)
  discord_user_id  TEXT NOT NULL,
  source           TEXT NOT NULL,
  item_id          INTEGER,
  detail           TEXT,
  sent_at          INTEGER NOT NULL,
  send_status      TEXT NOT NULL,
  read_at          INTEGER                    -- nullable; NULL = unread (D-05)
);
CREATE INDEX alert_log_dedup_idx ON alert_log(wantlist_item_id, source, item_id, sent_at);

ALTER TABLE wantlist_item ADD COLUMN muted INTEGER NOT NULL DEFAULT 0;  -- D-09

-- +goose Down
-- Forward-only in practice (mirrors 00004/00005/00006): explicit no-op.
SELECT 1;
