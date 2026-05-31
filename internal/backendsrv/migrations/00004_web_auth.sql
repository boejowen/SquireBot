-- +goose Up
-- Phase 15 plan 15-01. Discord-login web users + opaque sessions + the
-- guild_admins allowlist (keyed by Discord ID) + CLI-seeded owner-floor +
-- bank coin columns + eviction grace/archive + generic audit_log columns.
-- Forward-only; 00001/00002/00003 are shipped and NOT edited. AUTH-08/09,
-- ADMIN-04/05/06.

-- web_user: one row per Discord identity captured at login (AUTH-09). The
-- snowflake is the stable key the deferred v2 pinger will DM. Upserted each
-- login (username/avatar refresh). Timestamps = unix epoch seconds (UTC).
CREATE TABLE web_user (
  discord_user_id TEXT PRIMARY KEY,   -- Discord snowflake (string; 64-bit safe)
  username        TEXT NOT NULL,
  avatar          TEXT,               -- Discord avatar hash (nullable)
  first_seen      INTEGER NOT NULL,   -- unix epoch seconds, UTC
  last_login      INTEGER NOT NULL
);

-- web_session: opaque server-side sessions (D-05). Only the SHA-256 hash of
-- the session id is stored — never the plaintext. Rolling 30-day expiry.
CREATE TABLE web_session (
  id              INTEGER PRIMARY KEY,
  session_hash    TEXT NOT NULL UNIQUE,  -- sha256 hex of the opaque session id
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  created_at      INTEGER NOT NULL,      -- unix epoch seconds, UTC
  expires_at      INTEGER NOT NULL       -- rolling; bumped on each authenticated hit
);
CREATE INDEX idx_web_session_user ON web_session (discord_user_id);

-- guild_admins: the officer allowlist (ADMIN-06), keyed by Discord ID. Ported
-- from v1's _meta.guild_admins JSON to real rows. Idempotent add/remove +
-- owner-floor protection enforced in the store layer.
CREATE TABLE guild_admins (
  discord_user_id TEXT PRIMARY KEY REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  added_at        INTEGER NOT NULL,      -- unix epoch seconds, UTC
  added_by        TEXT NOT NULL          -- the promoting officer's discord_user_id, or 'cli'
);

-- app_config: singleton key/value for ops-seeded config. The owner-floor
-- Discord ID (D-08, CLI-seeded via set-owner-floor) lives here under key
-- 'owner_floor_discord_id'. Generic so future single-value config reuses it.
CREATE TABLE app_config (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at INTEGER NOT NULL            -- unix epoch seconds, UTC
);

-- Bank coin (ADMIN-05 / D-11). Nullable — only is_bank_toon characters ever
-- get values; the /outputfile format carries no coin so manual entry is the
-- only path. Extend-only ALTER (mirrors 00003).
ALTER TABLE character ADD COLUMN plat   INTEGER;
ALTER TABLE character ADD COLUMN gold   INTEGER;
ALTER TABLE character ADD COLUMN silver INTEGER;
ALTER TABLE character ADD COLUMN copper INTEGER;

-- Eviction grace/archive (ADMIN-04 / D-10). Set on the owner's characters at
-- eviction; the archive job hard-archives after grace_until passes. Epoch secs.
ALTER TABLE character ADD COLUMN grace_until INTEGER;  -- unix epoch; eviction grace deadline
ALTER TABLE character ADD COLUMN archived_at INTEGER;  -- set by the archive job once past grace

-- Generic audit_log columns (D-06: REUSE/EXTEND the existing audit_log, do not
-- invent a parallel log). The 00002 table is ingest-specific (event/char_name/
-- attempting_owner_id/current_owner_id). These add the generic web-write fields
-- the officer/eviction/coin events need: actor=acting discord_user_id, detail=
-- a small JSON blob, at=unix epoch seconds (the web writes use epoch; the
-- existing created_at TEXT stays for the ingest rows).
ALTER TABLE audit_log ADD COLUMN actor  TEXT;     -- acting discord_user_id (web writes)
ALTER TABLE audit_log ADD COLUMN detail TEXT;     -- JSON detail blob (web writes)
ALTER TABLE audit_log ADD COLUMN at     INTEGER;  -- unix epoch seconds (web writes)

-- +goose Down
ALTER TABLE audit_log DROP COLUMN at;
ALTER TABLE audit_log DROP COLUMN detail;
ALTER TABLE audit_log DROP COLUMN actor;
ALTER TABLE character DROP COLUMN archived_at;
ALTER TABLE character DROP COLUMN grace_until;
ALTER TABLE character DROP COLUMN copper;
ALTER TABLE character DROP COLUMN silver;
ALTER TABLE character DROP COLUMN gold;
ALTER TABLE character DROP COLUMN plat;
DROP TABLE app_config;
DROP TABLE guild_admins;
DROP INDEX idx_web_session_user;
DROP TABLE web_session;
DROP TABLE web_user;
-- (Down is best-effort; forward-only in practice.)
