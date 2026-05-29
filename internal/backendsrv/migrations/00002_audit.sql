-- +goose Up
-- audit_log is an append-only record of security-relevant events. The first
-- writer is the cross-owner-reject path (binding.go auditCrossOwnerReject):
-- when a guild code attempts to upload data for a character already bound to a
-- DIFFERENT owner, the attempt is rejected (ErrCharOwnedByAnother) and a row is
-- written here so a takeover attempt leaves a durable trace (D-07 / V4 / T-11.03-05).
--
-- Forward-only: this is the SECOND goose migration; 00001_init.sql is owned by
-- 11-02 and is NOT edited (goose is forward-only and the //go:embed *.sql glob
-- auto-includes this file). goose stays idempotent across both migrations.
CREATE TABLE audit_log (
  id                   INTEGER PRIMARY KEY,
  event                TEXT NOT NULL,            -- e.g. 'cross_owner_reject'
  char_name            TEXT,                     -- character involved (nullable)
  attempting_owner_id  INTEGER,                  -- owner whose token tried the write
  current_owner_id     INTEGER,                  -- owner that currently owns the char
  created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE audit_log;
