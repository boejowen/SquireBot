package store

// coin.go is the bank-coin entry persistence layer (ADMIN-05 / D-11). It ports
// v1's "manual platinum for the bank toon" intent: the /outputfile format
// carries NO coin, so manual entry is the only honest path. Coin values are
// nullable plat/gold/silver/copper INTEGER columns on character (00004), written
// ONLY onto is_bank_toon characters.
//
// Data-layer invariant (T-15-04): SetCoinTx refuses a write to a non-bank-toon
// character (ErrNotBankToon) — the D-11 "coin only on bank toons" rule is
// enforced at the store layer too, not just the handler. Numeric range
// validation (e.g. 0–999 for gold/silver/copper) lives at the 15-03 handler
// layer; the store rejects only a non-bank-toon target.
//
// Parameterized ? placeholders ONLY (V5). SetCoinTx takes the caller's *sql.Tx
// so 15-03 composes the coin write + an audit_log row in one tx.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotBankToon is returned by SetCoinTx when the target character is not flagged
// is_bank_toon=1. The 15-03 handler maps it to a 4xx (coin entry is bank-toon-only).
var ErrNotBankToon = errors.New("not_bank_toon")

// BankToon is one bank-toon row with its current coin (BankToon list + GetCoin).
// Coin fields are *int64 so an unset (NULL) value is distinguishable from 0 —
// the form pre-fills from these (UI-SPEC), so "never entered" vs "entered as 0"
// must not collapse. snake_case JSON tags (crosses the API boundary in 15-03).
type BankToon struct {
	CharacterID int64  `json:"character_id"`
	Name        string `json:"name"`
	Plat        *int64 `json:"plat"`
	Gold        *int64 `json:"gold"`
	Silver      *int64 `json:"silver"`
	Copper      *int64 `json:"copper"`
}

// ListBankToons returns the live bank-toon characters (is_bank_toon=1 AND
// is_removed=0) with their current coin — the pre-fill source for the bank-coin
// form and the values the bank view surfaces (replacing P14's null/0 placeholder).
// Ordered by name.
func ListBankToons(ctx context.Context, db *sql.DB) ([]BankToon, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, plat, gold, silver, copper
		   FROM character
		  WHERE is_bank_toon = 1 AND is_removed = 0
		  ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list bank toons: %w", err)
	}
	defer rows.Close()

	var out []BankToon
	for rows.Next() {
		bt, scanErr := scanBankToon(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, bt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bank toons: %w", err)
	}
	return out, nil
}

// GetCoin returns one bank-toon's row by id (the form's pre-fill / read-back). A
// missing character id returns a %w-wrapped sql.ErrNoRows so the caller can
// errors.Is it. Note this does NOT gate on is_bank_toon — it is a read; the
// bank-toon gate is a write-time concern (SetCoinTx).
func GetCoin(ctx context.Context, db *sql.DB, characterID int64) (*BankToon, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, plat, gold, silver, copper FROM character WHERE id = ?`, characterID)
	bt, err := scanBankToonRow(row)
	if err != nil {
		return nil, fmt.Errorf("get coin (character_id=%d): %w", characterID, err)
	}
	return &bt, nil
}

// SetCoinTx writes plat/gold/silver/copper onto characterID inside the caller's
// tx — ONLY if the target is a bank toon (T-15-04). It SELECTs is_bank_toon
// first; if !=1 → ErrNotBankToon (no write). Then UPDATEs the four columns.
// Range validation is the handler's job (15-03); this enforces the bank-toon gate.
func SetCoinTx(ctx context.Context, tx *sql.Tx, characterID, plat, gold, silver, copper int64) error {
	var isBank int
	err := tx.QueryRowContext(ctx,
		`SELECT is_bank_toon FROM character WHERE id = ?`, characterID,
	).Scan(&isBank)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotBankToon // no such char ⇒ certainly not a bank toon (fail-closed)
	case err != nil:
		return fmt.Errorf("check bank toon (character_id=%d): %w", characterID, err)
	}
	if isBank != 1 {
		return ErrNotBankToon
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character SET plat = ?, gold = ?, silver = ?, copper = ? WHERE id = ?`,
		plat, gold, silver, copper, characterID,
	); err != nil {
		return fmt.Errorf("set coin (character_id=%d): %w", characterID, err)
	}
	return nil
}

// scanBankToon scans one *sql.Rows row into a BankToon, resolving the four
// nullable coin columns to *int64 (nil when NULL).
func scanBankToon(rows *sql.Rows) (BankToon, error) {
	var bt BankToon
	var plat, gold, silver, copper sql.NullInt64
	if err := rows.Scan(&bt.CharacterID, &bt.Name, &plat, &gold, &silver, &copper); err != nil {
		return BankToon{}, fmt.Errorf("scan bank toon row: %w", err)
	}
	bt.Plat = nullableToPtr(plat)
	bt.Gold = nullableToPtr(gold)
	bt.Silver = nullableToPtr(silver)
	bt.Copper = nullableToPtr(copper)
	return bt, nil
}

// scanBankToonRow is the single-row (*sql.Row) twin of scanBankToon for GetCoin.
func scanBankToonRow(row *sql.Row) (BankToon, error) {
	var bt BankToon
	var plat, gold, silver, copper sql.NullInt64
	if err := row.Scan(&bt.CharacterID, &bt.Name, &plat, &gold, &silver, &copper); err != nil {
		return BankToon{}, err // caller wraps (preserves sql.ErrNoRows for errors.Is)
	}
	bt.Plat = nullableToPtr(plat)
	bt.Gold = nullableToPtr(gold)
	bt.Silver = nullableToPtr(silver)
	bt.Copper = nullableToPtr(copper)
	return bt, nil
}

// nullableToPtr maps a sql.NullInt64 to *int64 (nil when not valid) so an unset
// coin column stays distinguishable from an entered 0.
func nullableToPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
