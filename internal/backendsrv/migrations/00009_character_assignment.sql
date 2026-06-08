-- +goose Up
-- Phase 26 plan 26-01 (ASSIGN-01..06). The character→user assignment layer over
-- the untouched character.owner_id upload provenance (D-03): a single-assignee
-- table, a contested-claim request queue, the new is_guild_bot designation, and
-- the idempotent auto-seed from owner.discord_user_id (D-04). Forward-only;
-- 00001-00008 are SHIPPED and NOT edited.
--
-- Backend-only: the watcher never reads/writes these tables, so there is NO
-- _meta.schema_version bump and NO WatcherMaxSchemaVersion change (the watcher
-- targets the ingest API, not these backend tables). "Schema v9" == goose
-- migration 00009 applied (goose_db_version is the version record).

-- D-02: "guild bot" is a NEW designation (no existing analog). "guild bank" reuses
-- the EXISTING is_bank_toon (now officer-only). A guild bank/bot char is shared,
-- has NO assignee, and is not claimable (exemption enforced in the store layer).
ALTER TABLE character ADD COLUMN is_guild_bot INTEGER NOT NULL DEFAULT 0;

-- D-01/D-03: exactly one assignee per NORMAL character, keyed on character_id (PK
-- ⇒ the single-assignee invariant is the schema, not store logic). discord_user_id
-- is the assignee (the PERSON, the wantlist/notify identity — NOT owner_id). The
-- claim/release/officer-assign mutators write here; owner_id is never touched.
CREATE TABLE character_assignment (
  character_id    INTEGER PRIMARY KEY REFERENCES character(id) ON DELETE CASCADE,  -- one row per char ⇒ single assignee (D-01)
  discord_user_id TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  assigned_at     INTEGER NOT NULL,                 -- unix epoch secs (nowUnix())
  assigned_by     TEXT NOT NULL                     -- 'self' for self-claim, else the officer's discord_user_id (D-06/D-09); auto-seed uses 'migration'
);
CREATE INDEX character_assignment_user_idx ON character_assignment(discord_user_id);

-- D-07: a contested claim (char already assigned to someone else) files a request
-- an officer approves/denies. State machine: pending → approved | denied | cancelled.
-- The requester can cancel a pending request; the officer approve/deny resolves it.
CREATE TABLE assignment_request (
  id               INTEGER PRIMARY KEY,
  character_id     INTEGER NOT NULL REFERENCES character(id) ON DELETE CASCADE,
  requester        TEXT NOT NULL REFERENCES web_user(discord_user_id) ON DELETE CASCADE,
  current_assignee TEXT,                            -- snapshot of who held it at request time (nullable: may be unassigned by approval time)
  status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','approved','denied','cancelled')),  -- DB-level defense-in-depth (the wantlist CHECK precedent)
  created_at       INTEGER NOT NULL,
  resolved_at      INTEGER,                         -- NULL until approve/deny/cancel
  resolved_by      TEXT                             -- the officer (approve/deny) or the requester (cancel); NULL while pending
);
CREATE INDEX assignment_request_char_idx ON assignment_request(character_id);
CREATE INDEX assignment_request_requester_idx ON assignment_request(requester);
-- At most ONE pending request per (character, requester) — SQLite treats NULL as
-- DISTINCT in a UNIQUE index, so scope the partial index to status='pending'
-- (the 00006 wantlist partial-unique precedent). A second pending request from the
-- same member for the same char collides; resolved rows never collide.
CREATE UNIQUE INDEX assignment_request_pending_uidx
  ON assignment_request(character_id, requester) WHERE status = 'pending';

-- D-04: idempotent auto-seed. Each character whose owner is linked (owner.discord_
-- user_id non-NULL via P17) AND is not already assigned AND is not a guild bank/bot
-- is auto-assigned to that user. INSERT OR IGNORE on the character_id PK makes a
-- re-run a no-op (goose already guards re-run, but OR IGNORE is belt-and-suspenders
-- and lets the same SELECT be safe if ever re-applied). Legacy/NULL-owner chars and
-- guild banks/bots are excluded ⇒ they start unassigned. assigned_by='migration'.
INSERT OR IGNORE INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
SELECT c.id, o.discord_user_id, strftime('%s','now'), 'migration'
  FROM character c
  JOIN owner o ON o.id = c.owner_id
 WHERE o.discord_user_id IS NOT NULL
   AND c.is_removed = 0
   AND c.is_bank_toon = 0
   AND c.is_guild_bot = 0;

-- +goose Down
-- Forward-only in practice (mirrors 00004-00008): explicit no-op.
SELECT 1;
