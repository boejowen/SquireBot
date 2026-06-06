package compute_test

// view_test.go proves compute.View parity over a seeded temp DB, mirroring a v1
// buildView.test.ts scenario (a character with a few inventory items, one
// enriched + priced + quested), plus a focused pickPrice unit test.
//
// It is an external test package (compute_test) using store.NewTestDB. The store
// package's own _test.go seed helpers are package-private, so this file defines
// its own raw-insert helpers in fixtures_test.go.

import (
	"context"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

func TestView_EnrichmentInlineAndOrdering(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	apple := seedChar(t, db, "owner-a", "Apple", "NEC", 60, "HUM", false)

	// 2 real items: the Circlet is fully enriched (wiki + WTS price + quest);
	// the Robe is bare. Plus an empty slot (id=0) that must NOT appear in view.
	seedInv(t, db, apple, "HEAD", "Circlet of Vallon", 1234, 1)
	seedInv(t, db, apple, "CHEST", "Robe of the Lost Circle", 5678, 2)
	seedInv(t, db, apple, "GENERAL1", "(empty)", 0, 3)

	seedItemMaster(t, db, 1234, "Circlet of Vallon", "A fine circlet of Vallon.", "http://wiki/Circlet", true)
	seedPigparse(t, db, 1234, "Circlet of Vallon", "0", 4500, 75) // WTS, a30=4500, t30=75
	seedQuest(t, db, 1234, "Coldain Ring 1", "notes_link")

	rows, err := compute.View(ctx, s)
	if err != nil {
		t.Fatalf("View: %v", err)
	}

	// Empty slot excluded → 2 rows. Ordered Char→item→location: "Circlet..."
	// before "Robe..." (item asc).
	if len(rows) != 2 {
		t.Fatalf("got %d view rows, want 2: %+v", len(rows), rows)
	}
	if rows[0].Item != "Circlet of Vallon" || rows[1].Item != "Robe of the Lost Circle" {
		t.Fatalf("row order = [%q, %q], want Circlet then Robe", rows[0].Item, rows[1].Item)
	}

	c := rows[0]
	// Picked price is the WTS a30.
	if c.Price == nil || *c.Price != 4500 {
		t.Errorf("Circlet Price = %v, want *4500", c.Price)
	}
	// Plain wiki URL — NOT an =HYPERLINK formula.
	if c.WikiURL != "http://wiki/Circlet" {
		t.Errorf("Circlet WikiURL = %q, want plain url", c.WikiURL)
	}
	// Enrichment carried inline (D-03).
	if c.WikiSummary != "A fine circlet of Vallon." {
		t.Errorf("Circlet WikiSummary = %q, want populated", c.WikiSummary)
	}
	if !c.IsQuestItem {
		t.Errorf("Circlet IsQuestItem = false, want true")
	}
	if len(c.QuestLinks) != 1 || c.QuestLinks[0].QuestName != "Coldain Ring 1" {
		t.Errorf("Circlet QuestLinks = %+v, want one 'Coldain Ring 1'", c.QuestLinks)
	}
	// Raw price detail present for the tooltip.
	if len(c.Prices) != 1 || c.Prices[0].Direction != "0" || c.Prices[0].A30 != 4500 || c.Prices[0].T30 != 75 {
		t.Errorf("Circlet Prices = %+v, want one {0,4500,75}", c.Prices)
	}
	if c.LastSynced == "" {
		t.Errorf("Circlet LastSynced empty, want character.last_seen")
	}

	// The bare Robe row: no enrichment, nil price, no quest links.
	r := rows[1]
	if r.Price != nil {
		t.Errorf("Robe Price = %v, want nil", r.Price)
	}
	if r.WikiURL != "" || r.WikiSummary != "" || r.IsQuestItem || len(r.Prices) != 0 || len(r.QuestLinks) != 0 {
		t.Errorf("Robe row = %+v, want bare (no enrichment)", r)
	}
}

// TestView_PriceBridgesByNameAcrossNamespaces is the compute-level regression for
// the PRICE-COVERAGE bug: a held item whose PigParse catalog id differs from its
// EQ inventory id (same name) must still get its picked price + inline Prices, and
// a name shared by two catalog ids must NOT fan out the view into duplicate rows.
func TestView_PriceBridgesByNameAcrossNamespaces(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	findom := seedChar(t, db, "owner-a", "Findom", "SHM", 60, "TRO", false)

	// Held under EQ id 14536; catalog row for the same NAME is id 19450.
	seedInv(t, db, findom, "GENERAL1", "10 Dose Ant's Potion", 14536, 1)
	seedPigparse(t, db, 19450, "10 Dose Ant's Potion", "0", 320, 12)

	// Same normalized name under TWO catalog ids — the representative-row CTE must
	// collapse to one, so this item yields exactly ONE view row with one price.
	seedInv(t, db, findom, "GENERAL2", "Words of the Spoken", 7001, 2)
	seedPigparse(t, db, 7777, "Words of the Spoken", "0", 100, 4)
	seedPigparse(t, db, 9999, "WORDS OF THE SPOKEN", "0", 200, 9)

	rows, err := compute.View(ctx, s)
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d view rows, want 2 (no fan-out on the shared-name item): %+v", len(rows), rows)
	}

	byItem := map[string]compute.ViewRow{}
	for _, r := range rows {
		byItem[r.Item] = r
	}

	pot := byItem["10 Dose Ant's Potion"]
	if pot.Price == nil || *pot.Price != 320 {
		t.Errorf("Ant's Potion Price = %v, want *320 (name-bridged 14536↔19450)", pot.Price)
	}
	if len(pot.Prices) != 1 || pot.Prices[0].A30 != 320 || pot.Prices[0].T30 != 12 {
		t.Errorf("Ant's Potion Prices = %+v, want one {a30:320 t30:12}", pot.Prices)
	}
	if pot.ID != 14536 {
		t.Errorf("Ant's Potion ID = %d, want 14536 (EQ inventory id preserved)", pot.ID)
	}

	words := byItem["Words of the Spoken"]
	if words.Price == nil {
		t.Errorf("Words of the Spoken Price = nil, want a single representative price (no fan-out)")
	}
	if len(words.Prices) != 1 {
		t.Errorf("Words of the Spoken Prices = %+v, want exactly one (fan-out guard)", words.Prices)
	}
}
