package store

// replace.go is the SQLite port of the v1 watcher's full-snapshot clear+write
// contract (internal/sheet/write.go WriteInventory/WriteSpellbook). The watcher
// did one atomic Sheets batchUpdate that cleared the range and wrote the new
// rows in a single request; here that becomes ONE SQLite transaction:
//
//	BEGIN IMMEDIATE  ──►  DELETE FROM <tbl> WHERE character_id = ?
//	                 ──►  INSERT … (all parsed rows)
//	                 ──►  UPDATE character SET last_seen, watcher_version
//	                 ──►  COMMIT
//
// This is the BACKEND-03 contract: never append, never row-diff — full-snapshot
// replace only (CLAUDE.md write contract). A shrinking snapshot drops removed
// rows for free via the DELETE. The transaction is genuinely atomic (BEGIN
// IMMEDIATE via the _txlock=immediate DSN + defer Rollback), so no reader ever
// sees a partial/empty intermediate state — strictly stronger than the Sheet's
// single-batchUpdate approximation.
//
// TWO Sheets carve-outs are deliberately DROPPED here (PATTERNS "do NOT copy"):
//   - The Pitfall #8 toRowData "always StringValue, never NumberValue" hack
//     (write.go) does NOT carry over — SQLite has real INTEGER columns; item_id/
//     count/slots are parsed explicitly with strconv.Atoi.
//   - The watcher's defensive 5-col padding does NOT carry over — parse.Parse
//     already filters <5-col rows, so every row here is well-formed.

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Store wraps the *sql.DB from store.Open and owns the backend's write-side
// transactions (the atomic full-snapshot replace). It is the single writer to
// the SQLite file (Open sets SetMaxOpenConns(1) + _txlock=immediate).
type Store struct {
	db *sql.DB
}

// NewStore wraps a *sql.DB (from store.Open or store.NewTestDB) in a Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB exposes the underlying handle for read queries / composing transactions in
// later plans (11-05 wires the ingest handler over this).
func (s *Store) DB() *sql.DB { return s.db }

// ReplaceInventory atomically replaces ALL inventory_item rows for charID with
// rows, in one transaction (DELETE-all-then-INSERT). rows are parse.Parse
// output: [Location, Name, ID, Count, Slots]. row_ordinal is the file line
// order (stable display sort for P14). last_seen + watcher_version are updated
// in the same tx. A shrinking snapshot drops removed rows (BACKEND-03).
//
// item_id/count/slots are parsed with strconv.Atoi into real INTEGER columns
// (the Sheets StringValue hack is dropped); parse.Parse already guarantees r[2]
// (ID) is an int, so a non-numeric value would have been filtered upstream — the
// ignored Atoi error here defensively yields 0 (e.g. count/slots blanks).
func (s *Store) ReplaceInventory(ctx context.Context, charID int64, rows [][]string, uploadedAt time.Time, watcherVer string) error {
	tx, err := s.db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE
	if err != nil {
		return fmt.Errorf("begin inventory replace tx (char_id=%d): %w", charID, err)
	}
	defer tx.Rollback() // no-op after Commit; rolls back the DELETE on any INSERT error

	if _, err := tx.ExecContext(ctx, `DELETE FROM inventory_item WHERE character_id = ?`, charID); err != nil {
		slog.Error("inventory replace: delete", "char_id", charID, "err", err)
		return fmt.Errorf("delete inventory_item (char_id=%d): %w", charID, err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO inventory_item
		(character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare inventory insert (char_id=%d): %w", charID, err)
	}
	defer stmt.Close()

	uploadedStr := uploadedAt.UTC().Format(time.RFC3339)
	for i, r := range rows {
		itemID, _ := strconv.Atoi(r[2]) // parser guarantees r[2] is int; blank ⇒ 0
		cnt, _ := strconv.Atoi(r[3])
		slots, _ := strconv.Atoi(r[4])
		if _, err := stmt.ExecContext(ctx, charID, r[0], r[1], itemID, cnt, slots, i, uploadedStr); err != nil {
			// NEVER log raw content (V7); operation + char_id + ordinal + err only.
			slog.Error("inventory replace: insert", "char_id", charID, "row_ordinal", i, "err", err)
			return fmt.Errorf("insert inventory_item (char_id=%d, row_ordinal=%d): %w", charID, i, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE character SET last_seen = ?, watcher_version = ? WHERE id = ?`,
		uploadedStr, watcherVer, charID); err != nil {
		slog.Error("inventory replace: update character", "char_id", charID, "err", err)
		return fmt.Errorf("update character last_seen (char_id=%d): %w", charID, err)
	}

	return tx.Commit()
}

// ReplaceSpellbook atomically replaces ALL spellbook_entry rows for charID with
// rows, in one transaction. rows are parse.ParseSpellbook output: [Level, Name].
// normalized_name is computed at insert as lower(trim(name)) — the P12/P14 wiki
// join key (RESEARCH Migration Sketch). level is stored as a real INTEGER.
// Symmetric with ReplaceInventory: DELETE-all-then-INSERT, same UPDATE tail.
func (s *Store) ReplaceSpellbook(ctx context.Context, charID int64, rows [][]string, uploadedAt time.Time, watcherVer string) error {
	tx, err := s.db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE
	if err != nil {
		return fmt.Errorf("begin spellbook replace tx (char_id=%d): %w", charID, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM spellbook_entry WHERE character_id = ?`, charID); err != nil {
		slog.Error("spellbook replace: delete", "char_id", charID, "err", err)
		return fmt.Errorf("delete spellbook_entry (char_id=%d): %w", charID, err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO spellbook_entry
		(character_id, level, name, normalized_name, uploaded_at)
		VALUES (?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare spellbook insert (char_id=%d): %w", charID, err)
	}
	defer stmt.Close()

	uploadedStr := uploadedAt.UTC().Format(time.RFC3339)
	for i, r := range rows {
		level, _ := strconv.Atoi(r[0]) // parser guarantees r[0] is int
		name := r[1]
		normalized := strings.ToLower(strings.TrimSpace(name))
		if _, err := stmt.ExecContext(ctx, charID, level, name, normalized, uploadedStr); err != nil {
			slog.Error("spellbook replace: insert", "char_id", charID, "row_ordinal", i, "err", err)
			return fmt.Errorf("insert spellbook_entry (char_id=%d, row_ordinal=%d): %w", charID, i, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE character SET last_seen = ?, watcher_version = ? WHERE id = ?`,
		uploadedStr, watcherVer, charID); err != nil {
		slog.Error("spellbook replace: update character", "char_id", charID, "err", err)
		return fmt.Errorf("update character last_seen (char_id=%d): %w", charID, err)
	}

	return tx.Commit()
}
