package compute_test

// bank_test.go proves compute.Bank returns ONLY the is_bank_toon character's
// rows (same enrichment-inline shape as View) and a nil Coin (P14 — no fabricated
// zeros).

import (
	"context"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

func TestBank_OnlyBankToonAndNilCoin(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	// A regular character and a bank toon, each with one item.
	apple := seedChar(t, db, "owner-a", "Apple", "NEC", 60, "HUM", false)
	bank := seedChar(t, db, "owner-b", "Guildbank", "WAR", 60, "HUM", true)
	seedInv(t, db, apple, "HEAD", "Circlet of Vallon", 1234, 1)
	seedInv(t, db, bank, "GENERAL1", "Bag of Holding", 1038, 1)

	// Enrich the bank item so the inline enrichment path is exercised on bank too.
	seedItemMaster(t, db, 1038, "Bag of Holding", "Holds a lot.", "http://wiki/Bag", false)
	seedPigparse(t, db, 1038, "0", 9000, 40)

	bv, err := compute.Bank(ctx, s)
	if err != nil {
		t.Fatalf("Bank: %v", err)
	}

	// Only the bank toon's row.
	if len(bv.Rows) != 1 {
		t.Fatalf("got %d bank rows, want 1: %+v", len(bv.Rows), bv.Rows)
	}
	r := bv.Rows[0]
	if r.Char != "Guildbank" || r.Item != "Bag of Holding" {
		t.Errorf("bank row = %+v, want Guildbank/Bag of Holding", r)
	}
	if r.Price == nil || *r.Price != 9000 {
		t.Errorf("bank row Price = %v, want *9000", r.Price)
	}
	if r.WikiURL != "http://wiki/Bag" || r.WikiSummary != "Holds a lot." {
		t.Errorf("bank row enrichment = %+v, want url/summary inline", r)
	}

	// Coin is nil in P14 (never fabricated 0pp).
	if bv.Coin != nil {
		t.Errorf("bank Coin = %+v, want nil in P14", bv.Coin)
	}
}
