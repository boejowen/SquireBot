-- +goose Up
-- Phase 31 (INV-02 examine stats). Forward-only; 00001-00012 are shipped and NOT
-- edited. SQLite permits only ONE column per ALTER TABLE ADD COLUMN; the added
-- column is nullable (no DEFAULT) and carries no UNIQUE/PK constraint (the 00003 /
-- 00012 pattern). statsblock is the item's in-game stat block (Slot/AC/STR.../WT/
-- class/race + flags) cleaned from the wiki {{Itempage|statsblock=...}} param —
-- previously parsed but discarded by the D-8 Sheet-parity scope guard. NULL until
-- the weekly wiki job re-enriches the row; a NULL surfaces as "" in compute and the
-- examine simply omits the stats line (D-09 graceful omission). Read-only additive
-- column: the watcher is OFF the read path (untouched), so NO WatcherMaxSchemaVersion
-- gate is touched and goose version() is the version of record.
ALTER TABLE item_master ADD COLUMN statsblock TEXT;

-- +goose Down
ALTER TABLE item_master DROP COLUMN statsblock;
