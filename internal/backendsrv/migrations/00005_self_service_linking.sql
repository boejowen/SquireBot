-- +goose Up
-- Phase 17 plan 17-01. owner.discord_user_id FK→web_user (LINK-02, D-01) +
-- guild_code.last_seen (D-07, LINK-05). Forward-only; 00001-00004 shipped, NOT edited.
--
-- LANDMINE (17-RESEARCH Pitfall 1): SQLite cannot add a UNIQUE column via ALTER
-- (it would require building an index over existing rows). Uniqueness comes from a SEPARATE
-- partial CREATE UNIQUE INDEX ... WHERE discord_user_id IS NOT NULL, which both
-- enforces one-owner-per-Discord-id AND lets the many existing owners stay NULL.
--
-- Column types match conventions: discord_user_id is TEXT (web_user.discord_user_id
-- is a TEXT PRIMARY KEY — 00004:12 — so the FK types match); last_seen is TEXT to
-- match the sibling guild_code TEXT/datetime('now') columns (00001:54-55), NOT the
-- epoch INTEGER the 00004 web-side columns use.
--
-- FK caveat: the DSN sets _pragma=foreign_keys(ON) per connection so the FK is
-- enforced at runtime, but SQLite does NOT retroactively scan existing rows on ADD
-- COLUMN — existing owner rows get NULL discord_user_id (NULL is FK-exempt). New
-- stamps reference a real web_user row (a session only exists for a logged-in
-- member), so the FK holds.
ALTER TABLE owner ADD COLUMN discord_user_id TEXT REFERENCES web_user(discord_user_id);
CREATE UNIQUE INDEX owner_discord_user_id_uidx
  ON owner(discord_user_id) WHERE discord_user_id IS NOT NULL;  -- one owner per Discord id; many NULLs ok
ALTER TABLE guild_code ADD COLUMN last_seen TEXT;               -- TEXT/datetime('now') — matches 00001 guild_code cols

-- +goose Down
DROP INDEX owner_discord_user_id_uidx;
-- (column drops are best-effort; forward-only in practice — mirror 00004:86 comment)
