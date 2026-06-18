package store

import (
	"context"
	"database/sql"
	"testing"
)

// readviews_test.go proves the eight Phase-14 read methods' SQL + grouping
// against a small seeded temp DB (NewTestDB), before compute/ consumes them. It
// reuses the package test helpers seedOwnerChar / seedRaw / i64ptr (defined in
// replace_test.go + itemids_test.go).

// setCharMeta stamps class/level/race/is_bank_toon onto an already-seeded
// character (seedOwnerChar inserts only owner_id + name).
func setCharMeta(t *testing.T, db *sql.DB, charID int64, class string, level int64, race string, isBank bool) {
	t.Helper()
	bank := 0
	if isBank {
		bank = 1
	}
	if _, err := db.Exec(
		`UPDATE character SET class = ?, level = ?, race = ?, is_bank_toon = ?, last_seen = ? WHERE id = ?`,
		class, level, race, bank, "2026-05-09T00:00:00Z", charID,
	); err != nil {
		t.Fatalf("setCharMeta (char_id=%d): %v", charID, err)
	}
}

func seedItemMaster(t *testing.T, db *sql.DB, itemID int64, name, summary, url string, isQuest bool) {
	t.Helper()
	q := 0
	if isQuest {
		q = 1
	}
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		itemID, name, summary, url, "", q, "sha",
	); err != nil {
		t.Fatalf("seed item_master (item_id=%d): %v", itemID, err)
	}
}

// seedPigparse inserts one pigparse_price row. The view/bank price join bridges by
// NORMALIZED NAME (lower(trim(name))), NOT item_id (catalog ids != EQ inventory
// ids), so `name` MUST match the inventory item's name for a price to attach.
func seedPigparse(t *testing.T, db *sql.DB, itemID int64, name, direction string, a30 float64, t30 int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, blue_volume, last_seen, direction, t30, a30, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,?,datetime('now'))`,
		itemID, name, a30, t30, "2026-05-09", direction, t30, a30,
	); err != nil {
		t.Fatalf("seed pigparse_price (item_id=%d, name=%q): %v", itemID, name, err)
	}
}

func seedQuestItem(t *testing.T, db *sql.DB, itemID int64, questName, source string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO quest_items (item_id, quest_name, source_url, source, last_refreshed)
		 VALUES (?,?,?,?,datetime('now'))`,
		itemID, questName, "http://example/q", source,
	); err != nil {
		t.Fatalf("seed quest_items (item_id=%d, quest=%q): %v", itemID, questName, err)
	}
}

func seedWikiGear(t *testing.T, db *sql.DB, tier, class, slot, itemName string, rank int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO wiki_gear_tier (tier, class, slot, item_id, item_name, rank, last_refreshed)
		 VALUES (?,?,?,NULL,?,?,datetime('now'))`,
		tier, class, slot, itemName, rank,
	); err != nil {
		t.Fatalf("seed wiki_gear_tier (%s/%s/%s): %v", tier, class, slot, err)
	}
}

func seedWikiSpell(t *testing.T, db *sql.DB, class string, level int64, name, normalized string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO wiki_spells (class, level, spell_name, normalized_name, last_refreshed)
		 VALUES (?,?,?,?,datetime('now'))`,
		class, level, name, normalized,
	); err != nil {
		t.Fatalf("seed wiki_spells (%s/%d/%s): %v", class, level, name, err)
	}
}

func seedSpellbook(t *testing.T, db *sql.DB, charID int64, level int64, name, normalized string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO spellbook_entry (character_id, level, name, normalized_name, uploaded_at)
		 VALUES (?,?,?,?,datetime('now'))`,
		charID, level, name, normalized,
	); err != nil {
		t.Fatalf("seed spellbook_entry (char_id=%d, name=%q): %v", charID, name, err)
	}
}

// TestReadViews_InventoryJoinAndGrouping seeds a 2-character fixture (one bank
// toon) and asserts each read method's rows + grouping.
func TestReadViews_InventoryJoinAndGrouping(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charApple := seedOwnerChar(t, db, "owner-a", "Apple")   // non-bank
	_, charBank := seedOwnerChar(t, db, "owner-b", "Banktoon") // bank toon
	setCharMeta(t, db, charApple, "NEC", 60, "HUM", false)
	setCharMeta(t, db, charBank, "WAR", 60, "HUM", true)

	// Apple: two real items (one enriched + priced + quested) + an empty slot.
	seedRaw(t, db, charApple, "HEAD", "Circlet of Vallon", i64ptr(1234), 1)
	seedRaw(t, db, charApple, "CHEST", "Robe of the Lost Circle", i64ptr(5678), 2)
	seedRaw(t, db, charApple, "GENERAL1", "(empty)", i64ptr(0), 3) // excluded from InventoryJoin

	// Banktoon: one real item.
	seedRaw(t, db, charBank, "GENERAL1", "Bag of Holding", i64ptr(1038), 1)

	// Enrichment for item 1234 only.
	seedItemMaster(t, db, 1234, "Circlet of Vallon", "A fine circlet.", "http://wiki/Circlet", true)
	seedPigparse(t, db, 1234, "Circlet of Vallon", "0", 4500, 75)
	seedQuestItem(t, db, 1234, "Coldain Ring 1", "notes_link")
	seedQuestItem(t, db, 1234, "Coldain Ring 2", "notes_link")
	seedQuestItem(t, db, 9999, "Unrelated Quest", "in_game_flag") // different item_id

	t.Run("InventoryJoin all chars excludes empty slot, orders char/item/location", func(t *testing.T) {
		got, err := s.InventoryJoin(ctx, false)
		if err != nil {
			t.Fatalf("InventoryJoin(false): %v", err)
		}
		// 2 Apple real items + 1 Banktoon item = 3 (empty slot excluded).
		if len(got) != 3 {
			t.Fatalf("got %d rows, want 3: %+v", len(got), got)
		}
		// Order: Apple before Banktoon (char asc); within Apple, "Circlet..." before
		// "Robe..." (item asc).
		if got[0].Char != "Apple" || got[0].ItemName != "Circlet of Vallon" {
			t.Errorf("row[0] = %+v, want Apple/Circlet of Vallon first", got[0])
		}
		if got[1].Char != "Apple" || got[1].ItemName != "Robe of the Lost Circle" {
			t.Errorf("row[1] = %+v, want Apple/Robe second", got[1])
		}
		if got[2].Char != "Banktoon" {
			t.Errorf("row[2] = %+v, want Banktoon last", got[2])
		}
		// Enrichment resolved inline on the Circlet row.
		c := got[0]
		if c.WikiURL != "http://wiki/Circlet" || c.WikiSummary != "A fine circlet." || !c.IsQuestItem {
			t.Errorf("Circlet enrichment = %+v, want url/summary/isquest populated", c)
		}
		if !c.HasPrice || c.Direction != "0" || c.A30 != 4500 || c.T30 != 75 {
			t.Errorf("Circlet price = {has:%t dir:%q a30:%v t30:%d}, want has/0/4500/75", c.HasPrice, c.Direction, c.A30, c.T30)
		}
		if c.LastSeen == "" {
			t.Errorf("Circlet LastSeen empty, want character.last_seen")
		}
		// Robe row has no enrichment → zero-values, HasPrice false.
		r := got[1]
		if r.HasPrice || r.WikiURL != "" || r.WikiSummary != "" || r.IsQuestItem {
			t.Errorf("Robe row = %+v, want no enrichment", r)
		}
	})

	t.Run("InventoryJoin bankOnly returns only the bank toon", func(t *testing.T) {
		got, err := s.InventoryJoin(ctx, true)
		if err != nil {
			t.Fatalf("InventoryJoin(true): %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d bank rows, want 1: %+v", len(got), got)
		}
		if got[0].Char != "Banktoon" || got[0].ItemName != "Bag of Holding" {
			t.Errorf("bank row = %+v, want Banktoon/Bag of Holding", got[0])
		}
	})

	t.Run("QuestLinksByItem groups by item_id", func(t *testing.T) {
		got, err := s.QuestLinksByItem(ctx)
		if err != nil {
			t.Fatalf("QuestLinksByItem: %v", err)
		}
		if len(got[1234]) != 2 {
			t.Fatalf("item 1234 links = %d, want 2: %+v", len(got[1234]), got[1234])
		}
		// Ordered by quest_name.
		if got[1234][0].QuestName != "Coldain Ring 1" || got[1234][1].QuestName != "Coldain Ring 2" {
			t.Errorf("item 1234 links order = %+v, want Ring 1 then Ring 2", got[1234])
		}
		if len(got[9999]) != 1 || got[9999][0].Source != "in_game_flag" {
			t.Errorf("item 9999 links = %+v, want one in_game_flag", got[9999])
		}
	})

	t.Run("InventoryByChar keeps all named items incl empty-slot named rows, grouped", func(t *testing.T) {
		got, err := s.InventoryByChar(ctx)
		if err != nil {
			t.Fatalf("InventoryByChar: %v", err)
		}
		// Apple has 3 rows (incl the id=0 "(empty)" named row — InventoryByChar does
		// NOT filter on item_id, only skips truly empty names).
		if len(got["Apple"]) != 3 {
			t.Errorf("Apple items = %d, want 3: %+v", len(got["Apple"]), got["Apple"])
		}
		if len(got["Banktoon"]) != 1 {
			t.Errorf("Banktoon items = %d, want 1: %+v", len(got["Banktoon"]), got["Banktoon"])
		}
	})

	t.Run("CharsWithMeta returns both non-removed chars ordered by name", func(t *testing.T) {
		got, err := s.CharsWithMeta(ctx)
		if err != nil {
			t.Fatalf("CharsWithMeta: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("chars = %d, want 2: %+v", len(got), got)
		}
		if got[0].Name != "Apple" || got[0].Class != "NEC" || got[0].Level != 60 || got[0].Race != "HUM" {
			t.Errorf("char[0] = %+v, want Apple/NEC/60/HUM", got[0])
		}
		if got[1].Name != "Banktoon" {
			t.Errorf("char[1] = %+v, want Banktoon", got[1])
		}
	})

	t.Run("CharFreshness returns name + last_seen", func(t *testing.T) {
		got, err := s.CharFreshness(ctx)
		if err != nil {
			t.Fatalf("CharFreshness: %v", err)
		}
		if len(got) != 2 || got[0].Name != "Apple" || got[0].LastSeen == "" {
			t.Errorf("CharFreshness = %+v, want Apple with last_seen", got)
		}
	})
}

// TestReadViews_PriceBridgesByNameAcrossNamespaces is the regression for the
// view/bank PRICE-COVERAGE bug: the inventory item_id (EQ /outputfile namespace)
// and the pigparse_price item_id (PigParse catalog namespace) are DIFFERENT, so a
// price must be matched by NORMALIZED NAME (lower(trim(name))), not item_id.
func TestReadViews_PriceBridgesByNameAcrossNamespaces(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charID := seedOwnerChar(t, db, "owner-a", "Findom")
	setCharMeta(t, db, charID, "SHM", 60, "TRO", false)

	// Inventory holds the item under the EQ in-game id 14536 …
	seedRaw(t, db, charID, "GENERAL1", "10 Dose Ant's Potion", i64ptr(14536), 1)
	// … but the PigParse catalog row for the SAME NAME has a DIFFERENT id 19450.
	// Under the old pp.item_id = ii.item_id join this would NOT price (14536 ∉
	// catalog); the name bridge must attach it.
	seedPigparse(t, db, 19450, "10 Dose Ant's Potion", "0", 320, 12)

	// A second item whose name differs only by case/whitespace — proves the
	// lower(trim()) normalization on BOTH sides.
	seedRaw(t, db, charID, "GENERAL2", "Bone Chips", i64ptr(13073), 2)
	seedPigparse(t, db, 88888, "  bone chips  ", "0", 5, 3)

	got, err := s.InventoryJoin(ctx, false)
	if err != nil {
		t.Fatalf("InventoryJoin: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(got), got)
	}

	byName := map[string]InventoryJoinRow{}
	for _, r := range got {
		byName[r.ItemName] = r
	}

	pot := byName["10 Dose Ant's Potion"]
	if !pot.HasPrice || pot.A30 != 320 || pot.T30 != 12 || pot.Direction != "0" {
		t.Errorf("Ant's Potion price = {has:%t a30:%v t30:%d dir:%q}, want has/320/12/0 (name-bridged across 14536↔19450)",
			pot.HasPrice, pot.A30, pot.T30, pot.Direction)
	}
	// The inventory item_id is preserved (the EQ id), not the catalog id.
	if pot.ItemID != 14536 {
		t.Errorf("Ant's Potion ItemID = %d, want 14536 (the EQ inventory id, not the catalog 19450)", pot.ItemID)
	}

	bc := byName["Bone Chips"]
	if !bc.HasPrice || bc.A30 != 5 {
		t.Errorf("Bone Chips price = {has:%t a30:%v}, want has/5 (case+whitespace-normalized name match)", bc.HasPrice, bc.A30)
	}
}

// TestReadViews_PriceNoFanOutOnSharedName guards the CRITICAL fan-out invariant:
// when two DIFFERENT catalog ids share a normalized name, a held inventory row of
// that name must still yield EXACTLY ONE join row (one price), never two — a naive
// ON lower(trim(pp.name)) = lower(trim(ii.name)) would duplicate the row and
// inflate bank counts.
func TestReadViews_PriceNoFanOutOnSharedName(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charID := seedOwnerChar(t, db, "owner-a", "Banktoon")
	setCharMeta(t, db, charID, "WAR", 60, "HUM", true) // bank toon

	seedRaw(t, db, charID, "GENERAL1", "Words of the Spoken", i64ptr(7001), 1)

	// TWO catalog rows, different ids, SAME normalized name (a real PigParse
	// hazard: dupe entries / WTS+WTB rows). The representative-row CTE must collapse
	// these to one before the join.
	seedPigparse(t, db, 7777, "Words of the Spoken", "0", 100, 4)
	seedPigparse(t, db, 9999, "WORDS OF THE SPOKEN", "0", 200, 9)

	// View path (all chars).
	all, err := s.InventoryJoin(ctx, false)
	if err != nil {
		t.Fatalf("InventoryJoin(false): %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("view: got %d rows for one held item with a name shared by 2 catalog ids, want exactly 1 (no fan-out): %+v", len(all), all)
	}
	if !all[0].HasPrice {
		t.Errorf("view: row HasPrice = false, want true (one representative price)")
	}

	// Bank path (the count-inflation risk) — must ALSO be exactly one row.
	bank, err := s.InventoryJoin(ctx, true)
	if err != nil {
		t.Fatalf("InventoryJoin(true): %v", err)
	}
	if len(bank) != 1 {
		t.Fatalf("bank: got %d rows, want exactly 1 (fan-out would inflate bank totals): %+v", len(bank), bank)
	}
}

// seedRawFull is the seedRaw twin that sets count + slots explicitly, so a container
// shell (slots>0) and a stacked item (count>1) are seedable. seedRaw hardcodes
// count=1, slots=0 and cannot express a container.
func seedRawFull(t *testing.T, db *sql.DB, charID int64, location, name string, itemID, count, slots, ordinal int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO inventory_item (character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		charID, location, name, itemID, count, slots, ordinal,
	); err != nil {
		t.Fatalf("seed inventory_item %q: %v", name, err)
	}
}

// TestReadViews_InventoryForChar_KeepsEmptyAndContainers proves the INV-05 surface
// keeps the rows InventoryJoin drops: empty slots (item_id=0), container shells
// (slots>0), and *-Slot* bag-content children — all in row_ordinal order. This is
// the INVERSE of TestReadViews_InventoryJoinAndGrouping's "excludes empty slot".
func TestReadViews_InventoryForChar_KeepsEmptyAndContainers(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charID := seedOwnerChar(t, db, "owner-a", "Slampeach")
	setCharMeta(t, db, charID, "SHM", 60, "TRO", false)

	// Seeded OUT of row_ordinal order to prove the ORDER BY ii.row_ordinal sorts them.
	seedRawFull(t, db, charID, "General4-Slot1", "Diamond", 1071, 5, 0, 2)          // nested general child (stacked)
	seedRawFull(t, db, charID, "General4", "Large Bag", 1038, 1, 10, 1)             // container shell, slots=10
	seedRawFull(t, db, charID, "Finger2", "", 0, 0, 0, 4)                           // EMPTY slot (item_id=0, blank name)
	seedRawFull(t, db, charID, "Bank1-Slot1", "Words of the Spoken", 7001, 1, 0, 3) // bank child

	got, err := s.InventoryForChar(ctx, "Slampeach")
	if err != nil {
		t.Fatalf("InventoryForChar: %v", err)
	}

	// All four rows survive (no item_id>0 filter): empty slot + container + 2 children.
	if len(got) != 4 {
		t.Fatalf("got %d rows, want 4 (empty slot + container shell + 2 children all kept): %+v", len(got), got)
	}

	// Returned in row_ordinal order: General4(1), General4-Slot1(2), Bank1-Slot1(3), Finger2(4).
	wantOrder := []struct {
		loc    string
		itemID int64
	}{
		{"General4", 1038},
		{"General4-Slot1", 1071},
		{"Bank1-Slot1", 7001},
		{"Finger2", 0},
	}
	for i, w := range wantOrder {
		if got[i].Location != w.loc || got[i].ItemID != w.itemID {
			t.Errorf("row[%d] = {loc:%q id:%d}, want {loc:%q id:%d} (row_ordinal order)",
				i, got[i].Location, got[i].ItemID, w.loc, w.itemID)
		}
	}

	// The empty slot (item_id=0) is PRESENT — the inverse of InventoryJoin's exclusion.
	var sawEmpty bool
	for _, r := range got {
		if r.Location == "Finger2" {
			sawEmpty = true
			if r.ItemID != 0 {
				t.Errorf("empty slot ItemID = %d, want 0", r.ItemID)
			}
		}
	}
	if !sawEmpty {
		t.Errorf("empty slot row (Finger2, item_id=0) absent — InventoryForChar must keep it")
	}

	// The container shell carries its capacity (Slots), which InventoryJoinRow omits.
	for _, r := range got {
		if r.Location == "General4" && r.Slots != 10 {
			t.Errorf("container shell Slots = %d, want 10", r.Slots)
		}
	}
}

// TestReadViews_InventoryForChar_LastListedNotCharFreshness proves Pitfall 2: the two
// `last_seen` columns (pigparse_price last-listed vs character upload freshness) are
// NOT crossed. seedPigparse writes last_seen="2026-05-09"; setCharMeta writes
// character.last_seen="2026-05-09T00:00:00Z" — distinct values, so a swap is visible.
func TestReadViews_InventoryForChar_LastListedNotCharFreshness(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charID := seedOwnerChar(t, db, "owner-a", "Slampeach")
	setCharMeta(t, db, charID, "SHM", 60, "TRO", false)

	seedRaw(t, db, charID, "Head", "Crown of Narandi", i64ptr(2050), 1)
	seedPigparse(t, db, 2050, "Crown of Narandi", "0", 4500, 75) // writes last_seen="2026-05-09"

	got, err := s.InventoryForChar(ctx, "Slampeach")
	if err != nil {
		t.Fatalf("InventoryForChar: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(got), got)
	}
	r := got[0]

	if r.LastListed != "2026-05-09" {
		t.Errorf("LastListed = %q, want %q (pigparse_price.last_seen — last-listed-for-sale)", r.LastListed, "2026-05-09")
	}
	if r.LastSeen != "2026-05-09T00:00:00Z" {
		t.Errorf("LastSeen = %q, want %q (character.last_seen — upload freshness)", r.LastSeen, "2026-05-09T00:00:00Z")
	}
	if r.LastListed == r.LastSeen {
		t.Errorf("LastListed (%q) must NOT equal LastSeen (%q) — the two last_seen columns were crossed", r.LastListed, r.LastSeen)
	}
}

// TestReadViews_InventoryForChar_NameJoinHitAndMiss proves the DATA-01 contract on the
// new method: price bridges by normalized name ACROSS namespaces (catalog id != EQ id),
// the EQ inventory item_id is preserved, and an item with no matching catalog row misses.
func TestReadViews_InventoryForChar_NameJoinHitAndMiss(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charID := seedOwnerChar(t, db, "owner-a", "Slampeach")
	setCharMeta(t, db, charID, "SHM", 60, "TRO", false)

	// HIT: inventory holds the EQ id 14536; the PigParse catalog row for the SAME NAME
	// has a DIFFERENT id 19450 — the name bridge must attach the price.
	seedRaw(t, db, charID, "General1", "10 Dose Ant's Potion", i64ptr(14536), 1)
	seedPigparse(t, db, 19450, "10 Dose Ant's Potion", "0", 320, 12)

	// MISS: an item with no matching pigparse_price row.
	seedRaw(t, db, charID, "General2", "Worthless Trinket", i64ptr(9997), 2)

	got, err := s.InventoryForChar(ctx, "Slampeach")
	if err != nil {
		t.Fatalf("InventoryForChar: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(got), got)
	}

	byName := map[string]InventoryRow{}
	for _, r := range got {
		byName[r.ItemName] = r
	}

	pot := byName["10 Dose Ant's Potion"]
	if !pot.HasPrice || pot.A30 != 320 || pot.T30 != 12 || pot.Direction != "0" {
		t.Errorf("Ant's Potion price = {has:%t a30:%v t30:%d dir:%q}, want has/320/12/0 (name-bridged 14536↔19450)",
			pot.HasPrice, pot.A30, pot.T30, pot.Direction)
	}
	if pot.ItemID != 14536 {
		t.Errorf("Ant's Potion ItemID = %d, want 14536 (the EQ inventory id, not the catalog 19450)", pot.ItemID)
	}

	miss := byName["Worthless Trinket"]
	if miss.HasPrice {
		t.Errorf("Worthless Trinket HasPrice = true, want false (no matching pigparse row)")
	}
}

// TestGearTierPrices_NameJoin_HitMiss proves DATA-01 / ROADMAP SC #2: a wiki_gear_tier
// row (item_id ALWAYS NULL — Pitfall 4) resolves its PigParse price + last-listed by
// NORMALIZED ITEM NAME via the pp_rep CTE, NEVER by wgt.item_id. A name-matched row is a
// HIT (HasPrice true, A30 = seeded, LastListed = the pigparse last_seen — Pitfall 2); an
// unmatched row is a MISS (HasPrice false, zero-values), so the consumer renders "no
// price". This is the UNCONDITIONAL gear-tier name-join test that closes SC #2 in Phase 29.
func TestGearTierPrices_NameJoin_HitMiss(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// HIT row: a NULL-item_id gear-tier rec whose item_name matches a pigparse_price row
	// by normalized name — but the catalog row carries a DIFFERENT id (9999) than any EQ
	// id, proving the cross-namespace NAME bridge (the join is lower(trim(item_name)) →
	// pp_rep, never wgt.item_id which is always NULL).
	seedWikiGear(t, db, "Velious Raiding", "WAR", "Head", "Crown of Narandi", 1)
	seedPigparse(t, db, 9999, "Crown of Narandi", "0", 4500, 75) // writes last_seen="2026-05-09"

	// MISS row: a gear-tier rec with NO name-matching pigparse_price row.
	seedWikiGear(t, db, "Velious Raiding", "WAR", "Chest", "Unlisted Relic", 1)

	got, err := s.GearTierPrices(ctx)
	if err != nil {
		t.Fatalf("GearTierPrices: %v", err)
	}

	byName := map[string]GearTierPriceRow{}
	for _, r := range got {
		byName[r.ItemName] = r
	}

	hit := byName["Crown of Narandi"]
	if !hit.HasPrice || hit.A30 != 4500 {
		t.Errorf("Crown of Narandi = {has:%t a30:%v}, want has/4500 (name-bridged, NULL gear-tier id)", hit.HasPrice, hit.A30)
	}
	if hit.LastListed != "2026-05-09" {
		t.Errorf("Crown of Narandi LastListed = %q, want %q (pigparse_price.last_seen — last-listed-for-sale, Pitfall 2)", hit.LastListed, "2026-05-09")
	}

	miss := byName["Unlisted Relic"]
	if miss.HasPrice {
		t.Errorf("Unlisted Relic HasPrice = true, want false (no name-matching pigparse row → consumer renders 'no price')")
	}
	if miss.A30 != 0 || miss.LastListed != "" {
		t.Errorf("Unlisted Relic = {a30:%v lastListed:%q}, want zero-values (nil price resolves to zero)", miss.A30, miss.LastListed)
	}
}

// TestReadViews_GearAndSpellInputs proves the wiki_gear_tier / wiki_spells /
// spellbook read methods feeding gear_check + spell_check.
func TestReadViews_GearAndSpellInputs(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	_, charID := seedOwnerChar(t, db, "owner-a", "Slampeach")
	setCharMeta(t, db, charID, "NEC", 10, "HUM", false)

	seedWikiGear(t, db, "Velious Pre-Raid/Group", "NEC", "Head", "Circlet of Vallon", 1)
	seedWikiGear(t, db, "Velious Pre-Raid/Group", "NEC", "Chest", "Robe of the Lost Circle", 1)
	seedWikiGear(t, db, "Velious Raiding", "NEC", "Head", "Crown of Narandi", 1)

	seedWikiSpell(t, db, "NEC", 1, "Cavorting Bones", "cavorting bones")
	seedWikiSpell(t, db, "NEC", 4, "Disease Cloud", "disease cloud")

	seedSpellbook(t, db, charID, 1, "Cavorting Bones", "cavorting bones")

	t.Run("WikiGearTiers returns all rows ordered", func(t *testing.T) {
		got, err := s.WikiGearTiers(ctx)
		if err != nil {
			t.Fatalf("WikiGearTiers: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("gear tiers = %d, want 3: %+v", len(got), got)
		}
		// ORDER BY tier, class, slot, rank → "Velious Pre-Raid/Group" sorts before
		// "Velious Raiding" (string compare), Chest before Head within Pre-Raid.
		if got[0].Tier != "Velious Pre-Raid/Group" || got[0].Slot != "Chest" {
			t.Errorf("gear[0] = %+v, want Pre-Raid/Chest first", got[0])
		}
	})

	t.Run("WikiSpells returns all rows", func(t *testing.T) {
		got, err := s.WikiSpells(ctx)
		if err != nil {
			t.Fatalf("WikiSpells: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("wiki spells = %d, want 2: %+v", len(got), got)
		}
		if got[0].NormalizedName != "cavorting bones" {
			t.Errorf("spell[0].NormalizedName = %q, want 'cavorting bones'", got[0].NormalizedName)
		}
	})

	t.Run("SpellbookNormalizedByChar builds the known-name set", func(t *testing.T) {
		got, err := s.SpellbookNormalizedByChar(ctx)
		if err != nil {
			t.Fatalf("SpellbookNormalizedByChar: %v", err)
		}
		set := got["Slampeach"]
		if !set["cavorting bones"] {
			t.Errorf("Slampeach known set = %+v, want 'cavorting bones' present", set)
		}
		if set["disease cloud"] {
			t.Errorf("Slampeach known set wrongly contains 'disease cloud': %+v", set)
		}
	})
}
