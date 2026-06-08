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
	seedPigparse(t, db, 1038, "Bag of Holding", "0", 9000, 40)

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

// TestBank_MultipleBankToonsRender (Phase 26, Pitfall 5): the single-bank invariant
// is relaxed — TWO characters flagged is_bank_toon=1 both appear in the consolidated
// bank grid, grouped by the Char column, with nothing merged or dropped. This proves
// the bank view (compute.Bank → InventoryJoin bankOnly `WHERE c.is_bank_toon = 1`)
// already supports N guild banks without a query change.
func TestBank_MultipleBankToonsRender(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	// Two distinct guild-bank characters, each with its own item.
	bankA := seedChar(t, db, "owner-a", "Guildbank", "WAR", 60, "HUM", true)
	bankB := seedChar(t, db, "owner-b", "Alchemybank", "ENC", 60, "HUM", true)
	seedInv(t, db, bankA, "GENERAL1", "Bag of Holding", 1038, 1)
	seedInv(t, db, bankB, "GENERAL1", "Cloth Cap", 2020, 1)

	bv, err := compute.Bank(ctx, s)
	if err != nil {
		t.Fatalf("Bank: %v", err)
	}

	// Both banks' rows appear (nothing merged/dropped).
	if len(bv.Rows) != 2 {
		t.Fatalf("got %d bank rows, want 2 (one per guild bank): %+v", len(bv.Rows), bv.Rows)
	}
	// Collect the (Char, Item) pairs and assert both banks are present, distinct.
	byChar := map[string]string{}
	for _, r := range bv.Rows {
		byChar[r.Char] = r.Item
	}
	if byChar["Guildbank"] != "Bag of Holding" {
		t.Errorf("Guildbank row = %q, want Bag of Holding (rows: %+v)", byChar["Guildbank"], bv.Rows)
	}
	if byChar["Alchemybank"] != "Cloth Cap" {
		t.Errorf("Alchemybank row = %q, want Cloth Cap (rows: %+v)", byChar["Alchemybank"], bv.Rows)
	}
}
