package store

// charmeta.go is the char-metadata write persistence layer (CUTOVER-02 / P16,
// D-02/D-03). It is the fresh-start replacement for the abandoned Sheet backfill:
// class/level/race already exist as nullable columns on `character`
// (00001_init.sql, commented "set later / by backfill (P16)"), so there is NO
// migration — this just mutates the existing columns onto an already-existing,
// non-removed character (created by its first watcher upload binding; D-03 forbids
// pre-creating rows here). is_bank_toon is no longer written here — it became the
// officer-only "guild bank" designation (Phase 26, store.DesignateCharTx).
//
// Parameterized ? placeholders ONLY (V5). SetCharMetaTx takes the caller's *sql.Tx
// so the webadmin handler composes the meta write + an audit_log row in one tx
// (mirroring coin.go's SetCoinTx). CharsForMeta is a thin *sql.DB wrapper over the
// CharsWithMeta *Store method so the handler (which holds a *sql.DB, like
// BankToonsHandler) can read the pick-list without constructing a Store at the call
// site.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrCharNotFound is returned by SetCharMetaTx when the target character does not
// exist or is soft-removed (UPDATE … WHERE id = ? AND is_removed = 0 affected 0
// rows). The webadmin handler maps it to 400 invalid_input (fail-closed, mirroring
// coin.go's ErrNotBankToon shape) — the form edits live characters only.
var ErrCharNotFound = errors.New("char_not_found")

// SetCharMetaTx writes class/level/race onto an EXISTING, non-removed character
// inside the caller's tx. level is *int64 so a blank/unset level stays NULL (a NULL
// level → 0 → spellcheck skips the char, the correct unleveled behavior; the form
// must not fabricate a 0). The is_removed=0 scoping mirrors CharsWithMeta/
// ListBankToons (the form edits live chars only; D-03 forbids pre-creating rows). A
// RowsAffected()==0 → ErrCharNotFound (fail-closed) so the handler maps it to
// invalid_input. Parameterized ? only (V5).
//
// is_bank_toon is NO LONGER written here (Phase 26 reconciliation, OPEN-2/OPEN-3).
// The bank-toon (now "guild bank") designation became officer-only: the single
// writer of is_bank_toon is store.DesignateCharTx (assignment.go), gated by an
// in-tx officer re-check. SetCharMetaTx stays member-settable (RequireSession) for
// class/level/race — harmless metadata — and is therefore 5-arg (no isBankToon).
//
// The MD-01 single-bank-toon demote that previously lived here is GONE. It existed
// only because the flag was member-settable and an accidental second flag could
// silently merge two members' inventories in the bank view. With the flag now
// officer-only and MULTIPLE guild banks intentional (D-02), the single-bank
// invariant is relaxed: DesignateCharTx does NOT demote, and the consolidated
// Char-column bank view (compute/bank.go) renders N banks cleanly.
func SetCharMetaTx(ctx context.Context, tx *sql.Tx, characterID int64, class string, level *int64, race string) error {
	var levelArg any // nil → SQL NULL (blank/unset level)
	if level != nil {
		levelArg = *level
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE character SET class = ?, level = ?, race = ? WHERE id = ? AND is_removed = 0`,
		class, levelArg, race, characterID,
	)
	if err != nil {
		return fmt.Errorf("set char meta (character_id=%d): %w", characterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCharNotFound
	}
	return nil
}

// CharsForMeta is the *sql.DB-shaped wrapper the char-meta GET handler uses for the
// pick-list / pre-fill (it holds a *sql.DB like BankToonsHandler). It delegates to
// the CharsWithMeta *Store method — every non-removed character with its identity +
// class/level/race/is_bank_toon, ordered by name. Empty → empty slice (the handler
// normalizes nil → []).
func CharsForMeta(ctx context.Context, db *sql.DB) ([]CharMeta, error) {
	return NewStore(db).CharsWithMeta(ctx)
}
