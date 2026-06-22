package store

// binding.go is the SQLite port of the v1 watcher's identity policy
// (internal/sheet/owner.go UpsertCharOwner — first-write-wins). In v1, ~12 racing
// watchers each wrote to the shared Sheet and an OAuth email was advisory, so a
// cross-owner mismatch was logged-but-ALLOWED (slog.Warn + an _audit row, the
// write itself ungated). v2 originally TIGHTENED this: with the BACKEND (not
// racing watchers) owning the write, a cross-owner upload was REJECTED
// (D-07 / V4) instead of merely warned.
//
// As of 2026-06-21 (quick 260621-u6j) that rejection is REVERSED. This guild
// shares P99 logins (multiple guildies playing the same character) and runs guild
// banks with no real owner, so first-uploader-wins broke legitimate uploads (the
// Kim/Aenriel and lern41/Findom incidents). The model is now: ANY valid
// (non-revoked) guild code may upload ANY character. A cross-owner write is
// ALLOWED — it does NOT overwrite owner_id (the first uploader stays as a
// NON-BINDING steward record) and is recorded in audit_log (append-only) as a
// `cross_owner_write` event for traceability. owner_id is a first-sighting
// steward marker, NOT a write gate; there is no anti-overwrite guard (accepted
// trade-off for a trusted ~12-person guild).
//
// bindCharacter takes a *sql.Tx (not *sql.DB) on purpose: 11-05 composes the
// first-sighting bind and the atomic ReplaceInventory/ReplaceSpellbook in ONE
// transaction, so the cross-owner audit row and the replaced rows commit together
// and a real DB failure rolls back cleanly.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// BindCharacter is the exported entry point to the first-sighting owner bind,
// called by the ingest handler (11-05, a different package) INSIDE the same
// *sql.Tx as the atomic replace. It delegates UNCHANGED to bindCharacter (the
// package-internal implementation that 11-03's binding_test.go covers directly),
// so there is one tested bind SQL path. It returns the resolved charID on a
// first-sighting INSERT, a same-owner match, OR a cross-owner write (which the
// handler then replaces for; owner_id is left untouched and a cross_owner_write
// audit row is appended). The only non-nil error is a real DB failure.
func BindCharacter(ctx context.Context, tx *sql.Tx, charName string, tokenOwnerID int64) (charID int64, err error) {
	return bindCharacter(ctx, tx, charName, tokenOwnerID)
}

// bindCharacter resolves the character named charName for the uploading token's
// owner (tokenOwnerID). Cross-owner uploads are ALLOWED (shared chars/banks,
// 260621-u6j) — the only gate is the bearer guard upstream in the handler:
//
//   - name unseen   → INSERT a character bound to tokenOwnerID; return the new id.
//   - name owned by tokenOwnerID → return the existing id (no insert, no error).
//   - name owned by a DIFFERENT owner → append a `cross_owner_write` audit row and
//     return the EXISTING id (owner_id is NOT overwritten — the first uploader
//     stays as a non-binding steward record). The handler then replaces the rows
//     for this charID in the same tx.
//
// Lookup is a single indexed SELECT by NAME (the name column is UNIQUE COLLATE
// NOCASE, so the match is case-insensitive) — never a linear scan, never by row
// index. Parameterized ? placeholders only (V5).
func bindCharacter(ctx context.Context, tx *sql.Tx, charName string, tokenOwnerID int64) (charID int64, err error) {
	var ownerID int64
	err = tx.QueryRowContext(ctx,
		`SELECT owner_id, id FROM character WHERE name = ?`, charName).Scan(&ownerID, &charID)
	switch {
	case errors.Is(err, sql.ErrNoRows): // FIRST SIGHTING → bind char to this token's owner
		res, ierr := tx.ExecContext(ctx,
			`INSERT INTO character (owner_id, name) VALUES (?, ?)`, tokenOwnerID, charName)
		if ierr != nil {
			return 0, fmt.Errorf("insert character %q (owner_id=%d): %w", charName, tokenOwnerID, ierr)
		}
		charID, ierr = res.LastInsertId()
		if ierr != nil {
			return 0, fmt.Errorf("last insert id for character %q: %w", charName, ierr)
		}
		return charID, nil
	case err != nil:
		return 0, fmt.Errorf("lookup character %q: %w", charName, err)
	case ownerID != tokenOwnerID: // CROSS-OWNER → allow + audit (shared chars/banks, 260621-u6j)
		if aerr := auditCrossOwnerWrite(ctx, tx, charName, tokenOwnerID, ownerID); aerr != nil {
			return 0, aerr
		}
		// Never log the bearer token (V7); char name + owner ids only.
		slog.Info("cross-owner upload allowed (shared character)",
			"char_name", charName, "attempting_owner_id", tokenOwnerID, "current_owner_id", ownerID)
		// owner_id is NOT overwritten: the first uploader stays as a non-binding
		// steward record. Return the existing charID so the handler replaces its
		// rows in the same tx.
		return charID, nil
	default: // owner matches → proceed with the existing charID
		return charID, nil
	}
}

// auditCrossOwnerWrite appends a durable record of a cross-owner upload to
// audit_log (append-only). Written inside the same transaction as the bind so it
// commits together with the replaced rows. The event string is `cross_owner_write`
// (renamed from the pre-260621-u6j `cross_owner_reject` — same columns); the write
// is ALLOWED now, the row is purely a traceability marker.
func auditCrossOwnerWrite(ctx context.Context, tx *sql.Tx, charName string, attemptingOwner, currentOwner int64) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO audit_log (event, char_name, attempting_owner_id, current_owner_id)
		 VALUES ('cross_owner_write', ?, ?, ?)`,
		charName, attemptingOwner, currentOwner); err != nil {
		return fmt.Errorf("write cross_owner_write audit row (char=%q): %w", charName, err)
	}
	return nil
}
