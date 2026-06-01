package store

// charmeta.go is the char-metadata write persistence layer (CUTOVER-02 / P16,
// D-02/D-03). It is the fresh-start replacement for the abandoned Sheet backfill:
// class/level/race/is_bank_toon already exist as nullable columns on `character`
// (00001_init.sql, commented "set later / by backfill (P16)"), so there is NO
// migration — this just mutates the existing columns onto an already-existing,
// non-removed character (created by its first watcher upload binding; D-03 forbids
// pre-creating rows here).
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

// SetCharMetaTx writes class/level/race/is_bank_toon onto an EXISTING, non-removed
// character inside the caller's tx. level is *int64 so a blank/unset level stays
// NULL (a NULL level → 0 → spellcheck skips the char, the correct unleveled
// behavior; the form must not fabricate a 0). The is_removed=0 scoping mirrors
// CharsWithMeta/ListBankToons (the form edits live chars only; D-03 forbids
// pre-creating rows). A RowsAffected()==0 → ErrCharNotFound (fail-closed) so the
// handler maps it to invalid_input. Parameterized ? only (V5).
//
// MD-01 (P16 review): this is the FIRST and ONLY production writer of
// is_bank_toon=true, and the bank compute view (compute/bank.go) documents — and
// the bankOnly InventoryJoin branch (readviews.go: `... AND c.is_bank_toon = 1`)
// relies on — the invariant that at most ONE live character is the bank toon
// ("Char is constant within it"). Nothing in the schema enforces it (00001_init.sql
// has no partial-unique index on is_bank_toon), so without a guard the login-only
// form could flag a second character and silently mix two characters' inventories
// in the bank view. We enforce single-bank-toon HERE, inside the caller's tx, by
// DEMOTING every other live bank toon to 0 immediately before the set. The demote +
// set (+ the handler's audit row) commit atomically, so the invariant holds at
// every committed state. Setting is_bank_toon=false performs no demote (it only
// clears this character's own flag, which can never violate the at-most-one rule).
func SetCharMetaTx(ctx context.Context, tx *sql.Tx, characterID int64, class string, level *int64, race string, isBankToon bool) error {
	bt := 0
	if isBankToon {
		bt = 1
	}
	var levelArg any // nil → SQL NULL (blank/unset level)
	if level != nil {
		levelArg = *level
	}
	// Single-bank-toon invariant (MD-01): when promoting this character to the bank
	// toon, demote any OTHER live bank toon first, inside this same tx. Scoped to
	// live rows (is_removed=0) and excludes self (id <> ?) so re-saving the current
	// bank toon is a no-op. Parameterized ? only (V5).
	if isBankToon {
		if _, err := tx.ExecContext(ctx,
			`UPDATE character SET is_bank_toon = 0 WHERE is_bank_toon = 1 AND id <> ? AND is_removed = 0`,
			characterID,
		); err != nil {
			return fmt.Errorf("demote prior bank toon (keeping character_id=%d): %w", characterID, err)
		}
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE character SET class = ?, level = ?, race = ?, is_bank_toon = ? WHERE id = ? AND is_removed = 0`,
		class, levelArg, race, bt, characterID,
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
