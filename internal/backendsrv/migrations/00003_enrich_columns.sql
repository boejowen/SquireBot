-- +goose Up
-- Phase 12 (ENRICH-10/11). Forward-only; 00001/00002 are shipped and NOT edited.
-- SQLite permits only ONE column per ALTER TABLE ADD COLUMN; added columns are
-- nullable (no DEFAULT needed) and carry no UNIQUE/PK constraint.

-- pigparse_price: the 8 canonical price-history columns parseToRows emits but
-- the empty 11-02 table lacks. (current_avg/blue_volume/direction already exist
-- as the Sheet's a30/t30 aliases + WTS-WTB flag.)
ALTER TABLE pigparse_price ADD COLUMN t30 INTEGER;
ALTER TABLE pigparse_price ADD COLUMN a30 REAL;
ALTER TABLE pigparse_price ADD COLUMN t60 INTEGER;
ALTER TABLE pigparse_price ADD COLUMN a60 REAL;
ALTER TABLE pigparse_price ADD COLUMN t6m INTEGER;
ALTER TABLE pigparse_price ADD COLUMN a6m REAL;
ALTER TABLE pigparse_price ADD COLUMN ty  INTEGER;
ALTER TABLE pigparse_price ADD COLUMN ay  REAL;

-- Scheduler last-run bookkeeping (D-2). One row per registered job.
CREATE TABLE job_run (
  job_name     TEXT PRIMARY KEY,           -- 'pigparse_daily' | 'wiki_weekly'
  last_run_at  TEXT,                        -- ISO 8601 UTC; NULL = never run
  last_status  TEXT,                        -- 'ok' | 'error' | 'skipped_unchanged'
  last_detail  TEXT,                        -- short diagnostic (row counts, error)
  updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ETag/Last-Modified cache (D-3). Replaces PropertiesService ETag storage for
-- the 304 short-circuit. Keyed by the exact request URL.
CREATE TABLE etag_cache (
  url            TEXT PRIMARY KEY,
  etag           TEXT,                       -- value of the ETag response header
  last_modified  TEXT,                       -- value of the Last-Modified header
  fetched_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE etag_cache;
DROP TABLE job_run;
ALTER TABLE pigparse_price DROP COLUMN ay;
ALTER TABLE pigparse_price DROP COLUMN ty;
ALTER TABLE pigparse_price DROP COLUMN a6m;
ALTER TABLE pigparse_price DROP COLUMN t6m;
ALTER TABLE pigparse_price DROP COLUMN a60;
ALTER TABLE pigparse_price DROP COLUMN t60;
ALTER TABLE pigparse_price DROP COLUMN a30;
ALTER TABLE pigparse_price DROP COLUMN t30;
