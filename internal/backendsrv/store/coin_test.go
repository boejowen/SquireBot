package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// coin_test.go covers the bank-coin port (15-01 Task 3 / ADMIN-05 / D-11 /
// T-15-04): coin writes are gated to is_bank_toon characters; a non-bank-toon
// (or missing) target returns ErrNotBankToon; nullable coin round-trips as
// *int64 (unset distinguishable from 0).

func TestSetCoinTx_RejectsNonBankToon(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	regular := insertChar(t, ctx, db, ownerID, "Regulartoon", false) // NOT a bank toon

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCoinTx(ctx, tx, regular, 100, 5, 5, 5)
	})
	if !errors.Is(err, ErrNotBankToon) {
		t.Errorf("SetCoinTx on non-bank-toon: err = %v, want ErrNotBankToon", err)
	}

	// Missing character id → also ErrNotBankToon (fail-closed).
	err = commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCoinTx(ctx, tx, 99999, 1, 1, 1, 1)
	})
	if !errors.Is(err, ErrNotBankToon) {
		t.Errorf("SetCoinTx on missing char: err = %v, want ErrNotBankToon", err)
	}
}

func TestSetCoinTx_WritesOntoBankToon(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Banktoon", true)

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCoinTx(ctx, tx, bank, 1234, 9, 8, 7)
	}); err != nil {
		t.Fatalf("SetCoinTx: %v", err)
	}

	got, err := GetCoin(ctx, db, bank)
	if err != nil {
		t.Fatalf("GetCoin: %v", err)
	}
	if got.Plat == nil || *got.Plat != 1234 {
		t.Errorf("plat = %v, want 1234", got.Plat)
	}
	if got.Gold == nil || *got.Gold != 9 {
		t.Errorf("gold = %v, want 9", got.Gold)
	}
	if got.Silver == nil || *got.Silver != 8 {
		t.Errorf("silver = %v, want 8", got.Silver)
	}
	if got.Copper == nil || *got.Copper != 7 {
		t.Errorf("copper = %v, want 7", got.Copper)
	}
}

func TestSetCoinTx_AllowsZero(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Banktoon", true)

	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetCoinTx(ctx, tx, bank, 0, 0, 0, 0)
	}); err != nil {
		t.Fatalf("SetCoinTx zero: %v", err)
	}
	got, err := GetCoin(ctx, db, bank)
	if err != nil {
		t.Fatalf("GetCoin: %v", err)
	}
	// Entered 0 is distinguishable from unset (non-nil pointer to 0).
	if got.Plat == nil || *got.Plat != 0 {
		t.Errorf("plat = %v, want a non-nil pointer to 0 (entered-0 != unset)", got.Plat)
	}
}

func TestListBankToons_OnlyLiveBankToons(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, ctx, db, "Guildie-A")
	bank := insertChar(t, ctx, db, ownerID, "Banktoon", true)
	insertChar(t, ctx, db, ownerID, "Regulartoon", false) // not a bank toon → excluded

	// A removed bank toon is excluded.
	removedBank := insertChar(t, ctx, db, ownerID, "Oldbank", true)
	if _, err := db.ExecContext(ctx, `UPDATE character SET is_removed = 1 WHERE id = ?`, removedBank); err != nil {
		t.Fatalf("mark removed: %v", err)
	}

	toons, err := ListBankToons(ctx, db)
	if err != nil {
		t.Fatalf("ListBankToons: %v", err)
	}
	if len(toons) != 1 || toons[0].CharacterID != bank {
		t.Errorf("ListBankToons = %+v, want exactly the one live bank toon (id=%d)", toons, bank)
	}
	// A bank toon with no coin entered yet has nil coin pointers (pre-fill source).
	if toons[0].Plat != nil {
		t.Errorf("fresh bank toon plat = %v, want nil (never entered)", toons[0].Plat)
	}
}
