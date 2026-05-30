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

func seedPigparse(t *testing.T, db *sql.DB, itemID int64, direction string, a30 float64, t30 int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, blue_volume, last_seen, direction, t30, a30, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,?,datetime('now'))`,
		itemID, "x", a30, t30, "2026-05-09", direction, t30, a30,
	); err != nil {
		t.Fatalf("seed pigparse_price (item_id=%d): %v", itemID, err)
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

	_, charApple := seedOwnerChar(t, db, "owner-a", "Apple")  // non-bank
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
	seedPigparse(t, db, 1234, "0", 4500, 75)
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
