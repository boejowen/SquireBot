package store

// binding.go is the SQLite port of the v1 watcher's identity policy
// (internal/sheet/owner.go UpsertCharOwner — first-write-wins), TIGHTENED for
// the backend. In v1, ~12 racing watchers each wrote to the shared Sheet and an
// OAuth email was advisory, so a cross-owner mismatch was logged-but-ALLOWED
// (slog.Warn + an _audit row, the write itself ungated). In v2 the BACKEND — not
// racing watchers — owns the write, so that race class disappears and a
// cross-owner upload is REJECTED (ErrCharOwnedByAnother) instead of merely
// warned (D-07 / V4). owner_id is NEVER overwritten on a mismatch; the attempt
// is recorded in audit_log (append-only) so a takeover attempt leaves a trace
// (T-11.03-05). Reassignment is a P15 admin action.
//
// bindCharacter takes a *sql.Tx (not *sql.DB) on purpose: 11-05 composes the
// first-sighting bind and the atomic ReplaceInventory/ReplaceSpellbook in ONE
// transaction, so a rejected upload rolls back cleanly and a successful one
// commits the bind + the rows together.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// ErrCharOwnedByAnother is returned by bindCharacter when a character name is
// already bound to a DIFFERENT owner than the uploading token's. The ingest
// handler (11-05) maps it to HTTP 409 with a clear message.
var ErrCharOwnedByAnother = errors.New("character owned by another owner")

// bindCharacter resolves the character named charName for the uploading token's
// owner (tokenOwnerID), enforcing the first-sighting single-owner policy:
//
//   - name unseen   → INSERT a character bound to tokenOwnerID; return the new id.
//   - name owned by tokenOwnerID → return the existing id (no insert, no error).
//   - name owned by a DIFFERENT owner → write an audit_log row and return
//     ErrCharOwnedByAnother (owner_id is NOT overwritten).
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
	case ownerID != tokenOwnerID: // CROSS-OWNER → reject + audit (v2 tightens v1's warn-and-allow)
		if aerr := auditCrossOwnerReject(ctx, tx, charName, tokenOwnerID, ownerID); aerr != nil {
			return 0, aerr
		}
		// Never log the bearer token (V7); char name + owner ids only.
		slog.Warn("cross-owner upload rejected",
			"char_name", charName, "attempting_owner_id", tokenOwnerID, "current_owner_id", ownerID)
		return 0, ErrCharOwnedByAnother // handler (11-05) maps to 409 + clear message
	default: // owner matches → proceed with the existing charID
		return charID, nil
	}
}

// auditCrossOwnerReject appends a durable record of a cross-owner upload attempt
// to audit_log (append-only). Written inside the same transaction as the bind so
// it commits with the rejection record even though the ingest is refused.
func auditCrossOwnerReject(ctx context.Context, tx *sql.Tx, charName string, attemptingOwner, currentOwner int64) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO audit_log (event, char_name, attempting_owner_id, current_owner_id)
		 VALUES ('cross_owner_reject', ?, ?, ?)`,
		charName, attemptingOwner, currentOwner); err != nil {
		return fmt.Errorf("write cross_owner_reject audit row (char=%q): %w", charName, err)
	}
	return nil
}
