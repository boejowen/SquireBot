package store

import (
	"context"
	"database/sql"
	"testing"
)

// enrich_test.go covers the five Phase 12 dimension-table write methods over the
// shared NewTestDB fixture (which runs 00003, so the 8 pigparse columns + the two
// bookkeeping tables exist). The load-bearing assertions per the plan:
//   - pigparse: all 8 price columns persist; ON CONFLICT(item_id) UPDATES in place;
//     a partial re-upsert leaves untouched ids alone (D-4 graceful degradation).
//   - item_master: SHA-1 getter returns the stored sha; absent id returns "".
//   - wiki_spells: per-class replace drops a removed spell; other classes untouched.
//   - wiki_gear_tier: a second identical replace yields N rows, NOT 2N (Pitfall 1).
//   - quest_items: per-item-id replace; links for other item_ids untouched.

const lastRefreshed = "2026-05-29T00:00:00Z"

// --- pigparse_price ---------------------------------------------------------

func TestUpsertPigparsePrices_PersistsAllPriceColumnsAndUpserts(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	rows := []PigparsePrice{
		{ItemID: 1, Name: "Cloak of Flames", Direction: "0",
			T30: 10, A30: 1000.5, T60: 20, A60: 2000.5, T6m: 30, A6m: 3000.5, Ty: 40, Ay: 4000.5,
			LastSeen: "2026-05-28", LastRefreshed: lastRefreshed},
		{ItemID: 2, Name: "Fungi Tunic", Direction: "0",
			T30: 5, A30: 500.0, T60: 6, A60: 600.0, T6m: 7, A6m: 700.0, Ty: 8, Ay: 800.0,
			LastSeen: "2026-05-28", LastRefreshed: lastRefreshed},
		{ItemID: 3, Name: "Pearl", Direction: "0",
			T30: 1, A30: 1.0, T60: 2, A60: 2.0, T6m: 3, A6m: 3.0, Ty: 4, Ay: 4.0,
			LastSeen: "2026-05-28", LastRefreshed: lastRefreshed},
	}
	if err := s.UpsertPigparsePrices(ctx, rows); err != nil {
		t.Fatalf("UpsertPigparsePrices: %v", err)
	}

	// All 8 price columns + the a30/t30 aliases persisted for item 1.
	var (
		currentAvg        float64
		blueVolume        int
		t30, t60, t6m, ty int
		a30, a60, a6m, ay float64
		direction         string
	)
	err := db.QueryRow(`SELECT current_avg, blue_volume, direction, t30, a30, t60, a60, t6m, a6m, ty, ay
		FROM pigparse_price WHERE item_id = 1`).Scan(
		&currentAvg, &blueVolume, &direction, &t30, &a30, &t60, &a60, &t6m, &a6m, &ty, &ay)
	if err != nil {
		t.Fatalf("query item 1: %v", err)
	}
	if t30 != 10 || a30 != 1000.5 || t60 != 20 || a60 != 2000.5 || t6m != 30 || a6m != 3000.5 || ty != 40 || ay != 4000.5 {
		t.Errorf("price columns mismatch: t30=%d a30=%v t60=%d a60=%v t6m=%d a6m=%v ty=%d ay=%v",
			t30, a30, t60, a60, t6m, a6m, ty, ay)
	}
	// current_avg/blue_volume are the a30/t30 aliases.
	if currentAvg != 1000.5 || blueVolume != 10 {
		t.Errorf("aliases mismatch: current_avg=%v (want 1000.5), blue_volume=%d (want 10)", currentAvg, blueVolume)
	}
	if direction != "0" {
		t.Errorf("direction = %q, want %q", direction, "0")
	}

	// Re-upsert ONLY item 1 with a changed a30 → row UPDATED in place (still 1 row
	// for id 1, new value); items 2 and 3 untouched (truncated/partial response).
	if err := s.UpsertPigparsePrices(ctx, []PigparsePrice{
		{ItemID: 1, Name: "Cloak of Flames", Direction: "0",
			T30: 11, A30: 1111.0, T60: 20, A60: 2000.5, T6m: 30, A6m: 3000.5, Ty: 40, Ay: 4000.5,
			LastSeen: "2026-05-29", LastRefreshed: lastRefreshed},
	}); err != nil {
		t.Fatalf("re-upsert item 1: %v", err)
	}

	var count1 int
	if err := db.QueryRow(`SELECT count(*) FROM pigparse_price WHERE item_id = 1`).Scan(&count1); err != nil {
		t.Fatalf("count item 1: %v", err)
	}
	if count1 != 1 {
		t.Errorf("expected exactly 1 row for item_id 1 after re-upsert, got %d", count1)
	}
	var newA30 float64
	if err := db.QueryRow(`SELECT a30 FROM pigparse_price WHERE item_id = 1`).Scan(&newA30); err != nil {
		t.Fatalf("query a30 item 1: %v", err)
	}
	if newA30 != 1111.0 {
		t.Errorf("item 1 a30 = %v after re-upsert, want 1111.0 (ON CONFLICT did not UPDATE)", newA30)
	}

	// Items 2 and 3 still present, unchanged (partial re-upsert left them alone).
	var a30Item2 float64
	if err := db.QueryRow(`SELECT a30 FROM pigparse_price WHERE item_id = 2`).Scan(&a30Item2); err != nil {
		t.Fatalf("query a30 item 2: %v", err)
	}
	if a30Item2 != 500.0 {
		t.Errorf("item 2 a30 = %v, want 500.0 (partial re-upsert should not touch it)", a30Item2)
	}
	var total int
	if err := db.QueryRow(`SELECT count(*) FROM pigparse_price`).Scan(&total); err != nil {
		t.Fatalf("count total: %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3 total rows after partial re-upsert, got %d", total)
	}
}

// --- item_master ------------------------------------------------------------

// TestMarshalFlags pins the canonical flags-array encoder's contract (00016 / D-06):
// a nil OR empty slice → the literal "[]" (NEVER null/""), and a non-empty slice →
// a deterministic JSON array. The empty→"[]" rule is the load-bearing one: the
// upsert, the boot backfill, and the weekly job's freshness compare ALL produce
// flags_json through this one helper, so a flagless item is written exactly once
// (an empty set encoded as null at one site and "[]" at another would re-write the
// row on every weekly pass forever).
func TestMarshalFlags(t *testing.T) {
	if got := MarshalFlags(nil); got != "[]" {
		t.Errorf(`MarshalFlags(nil) = %q, want "[]" (never null/"" — D-06 idempotency)`, got)
	}
	if got := MarshalFlags([]string{}); got != "[]" {
		t.Errorf(`MarshalFlags([]string{}) = %q, want "[]" (never null/"" — D-06 idempotency)`, got)
	}
	// A non-empty (already-sorted) slice round-trips to a deterministic 2-element array.
	if got := MarshalFlags([]string{"LORE ITEM", "MAGIC ITEM"}); got != `["LORE ITEM","MAGIC ITEM"]` {
		t.Errorf(`MarshalFlags(["LORE ITEM","MAGIC ITEM"]) = %q, want ["LORE ITEM","MAGIC ITEM"]`, got)
	}
}

func TestUpsertItemMaster_AndSHA1Getter(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	item := ItemMaster{
		ItemID: 13128, Name: "Fungus Covered Scale Tunic", WikiSummary: "A quest tunic.",
		WikiURL: "https://wiki.project1999.com/Fungus_Covered_Scale_Tunic", Slot: "CHEST",
		IsQuestItem: true, WikitextSHA1: "abc123def456", LastRefreshed: lastRefreshed,
	}
	if err := s.UpsertItemMaster(ctx, item); err != nil {
		t.Fatalf("UpsertItemMaster: %v", err)
	}

	// is_quest_item stored as 1.
	var quest int
	if err := db.QueryRow(`SELECT is_quest_item FROM item_master WHERE item_id = ?`, item.ItemID).Scan(&quest); err != nil {
		t.Fatalf("query is_quest_item: %v", err)
	}
	if quest != 1 {
		t.Errorf("is_quest_item = %d, want 1", quest)
	}

	// GetItemMasterSHA1Tx returns the stored sha for a present row.
	if err := withTx(t, db, func(tx *sql.Tx) error {
		sha, err := GetItemMasterSHA1Tx(ctx, tx, int64(item.ItemID))
		if err != nil {
			return err
		}
		if sha != "abc123def456" {
			t.Errorf("GetItemMasterSHA1Tx = %q, want %q", sha, "abc123def456")
		}
		// Absent id returns "".
		absent, err := GetItemMasterSHA1Tx(ctx, tx, 999999)
		if err != nil {
			return err
		}
		if absent != "" {
			t.Errorf("GetItemMasterSHA1Tx(absent) = %q, want empty string", absent)
		}
		return nil
	}); err != nil {
		t.Fatalf("withTx: %v", err)
	}

	// Upserting the same id with a new sha UPDATES in place (still one row).
	item.WikitextSHA1 = "newsha"
	item.WikiSummary = "Updated."
	if err := s.UpsertItemMaster(ctx, item); err != nil {
		t.Fatalf("UpsertItemMaster (update): %v", err)
	}
	var count, summaryRows int
	if err := db.QueryRow(`SELECT count(*) FROM item_master WHERE item_id = ?`, item.ItemID).Scan(&count); err != nil {
		t.Fatalf("count item_master: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 item_master row after update, got %d", count)
	}
	if err := db.QueryRow(`SELECT count(*) FROM item_master WHERE item_id = ? AND wikitext_sha1 = 'newsha'`, item.ItemID).Scan(&summaryRows); err != nil {
		t.Fatalf("count updated sha: %v", err)
	}
	if summaryRows != 1 {
		t.Errorf("expected the row's wikitext_sha1 to be updated to 'newsha'")
	}
}

// --- wiki_spells ------------------------------------------------------------

func TestUpsertWikiSpellsForClass_ReplacesPerClassAndLeavesOthers(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// Seed 3 NEC spells and 1 PAL spell.
	nec := []WikiSpell{
		{Class: "NEC", Level: 1, SpellName: "Disease Cloud", LastRefreshed: lastRefreshed},
		{Class: "NEC", Level: 8, SpellName: "Lifedrain", LastRefreshed: lastRefreshed},
		{Class: "NEC", Level: 12, SpellName: "Heat Blood", LastRefreshed: lastRefreshed},
	}
	if err := s.UpsertWikiSpellsForClass(ctx, "NEC", nec); err != nil {
		t.Fatalf("seed NEC: %v", err)
	}
	pal := []WikiSpell{
		{Class: "PAL", Level: 9, SpellName: "Courage", LastRefreshed: lastRefreshed},
	}
	if err := s.UpsertWikiSpellsForClass(ctx, "PAL", pal); err != nil {
		t.Fatalf("seed PAL: %v", err)
	}

	// normalized_name is lower(trim(spell_name)).
	var norm string
	if err := db.QueryRow(`SELECT normalized_name FROM wiki_spells WHERE class='NEC' AND spell_name='Heat Blood'`).Scan(&norm); err != nil {
		t.Fatalf("query normalized: %v", err)
	}
	if norm != "heat blood" {
		t.Errorf("normalized_name = %q, want %q", norm, "heat blood")
	}

	// Replace NEC with 2 rows (one removed) → exactly 2 NEC rows remain.
	necShrunk := []WikiSpell{
		{Class: "NEC", Level: 1, SpellName: "Disease Cloud", LastRefreshed: lastRefreshed},
		{Class: "NEC", Level: 8, SpellName: "Lifedrain", LastRefreshed: lastRefreshed},
	}
	if err := s.UpsertWikiSpellsForClass(ctx, "NEC", necShrunk); err != nil {
		t.Fatalf("replace NEC: %v", err)
	}

	var necCount int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_spells WHERE class = 'NEC'`).Scan(&necCount); err != nil {
		t.Fatalf("count NEC: %v", err)
	}
	if necCount != 2 {
		t.Errorf("expected 2 NEC rows after per-class replace (stale 'Heat Blood' deleted), got %d", necCount)
	}
	// PAL untouched.
	var palCount int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_spells WHERE class = 'PAL'`).Scan(&palCount); err != nil {
		t.Fatalf("count PAL: %v", err)
	}
	if palCount != 1 {
		t.Errorf("expected PAL rows to be untouched by a NEC replace, got %d (want 1)", palCount)
	}
}

// --- wiki_gear_tier ---------------------------------------------------------

func TestReplaceWikiGearTier_FullReplaceDoesNotDuplicate(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	rows := []WikiGearTier{
		{Tier: "Velious Raiding", Class: "WAR", Slot: "Chest", ItemID: sql.NullInt64{}, ItemName: "Breastplate of the Plagueborn", Rank: 1, LastRefreshed: lastRefreshed},
		{Tier: "Velious Raiding", Class: "WAR", Slot: "Chest", ItemID: sql.NullInt64{}, ItemName: "ToV Chest", Rank: 2, LastRefreshed: lastRefreshed},
		{Tier: "Iksar", Class: "SHM", Slot: "Head", ItemID: sql.NullInt64{}, ItemName: "Iksar Hide Helm", Rank: 1, LastRefreshed: lastRefreshed},
	}
	n := len(rows)

	if err := s.ReplaceWikiGearTier(ctx, rows); err != nil {
		t.Fatalf("ReplaceWikiGearTier (first): %v", err)
	}
	var after1 int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_gear_tier`).Scan(&after1); err != nil {
		t.Fatalf("count after first: %v", err)
	}
	if after1 != n {
		t.Fatalf("expected %d rows after first replace, got %d", n, after1)
	}

	// Second identical replace → still N rows (NOT 2N). This is the Pitfall-1
	// guard: a per-row upsert would duplicate because UNIQUE(...,item_id) never
	// fires on NULL item_id; full-table replace keeps the count at N.
	if err := s.ReplaceWikiGearTier(ctx, rows); err != nil {
		t.Fatalf("ReplaceWikiGearTier (second): %v", err)
	}
	var after2 int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_gear_tier`).Scan(&after2); err != nil {
		t.Fatalf("count after second: %v", err)
	}
	if after2 != n {
		t.Errorf("expected %d rows after a second identical replace, got %d (full-replace must not duplicate)", n, after2)
	}

	// item_id is NULL for every gear-tier row.
	var nullCount int
	if err := db.QueryRow(`SELECT count(*) FROM wiki_gear_tier WHERE item_id IS NULL`).Scan(&nullCount); err != nil {
		t.Fatalf("count null item_id: %v", err)
	}
	if nullCount != n {
		t.Errorf("expected all %d gear-tier rows to have NULL item_id, got %d", n, nullCount)
	}
}

// --- quest_items ------------------------------------------------------------

func TestReplaceQuestItemsForID_ScopedReplace(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// 2 links for item 100, 1 link for item 200.
	if err := s.ReplaceQuestItemsForID(ctx, 100, []QuestItem{
		{ItemID: 100, QuestName: "Quest A", SourceURL: "https://wiki/QuestA", Source: "notes_link", LastRefreshed: lastRefreshed},
		{ItemID: 100, QuestName: "Quest B", SourceURL: "", Source: "in_game_flag", LastRefreshed: lastRefreshed},
	}); err != nil {
		t.Fatalf("seed item 100: %v", err)
	}
	if err := s.ReplaceQuestItemsForID(ctx, 200, []QuestItem{
		{ItemID: 200, QuestName: "Quest C", SourceURL: "https://wiki/QuestC", Source: "notes_link", LastRefreshed: lastRefreshed},
	}); err != nil {
		t.Fatalf("seed item 200: %v", err)
	}

	// Replace item 100 with a single link → exactly 1 row for 100.
	if err := s.ReplaceQuestItemsForID(ctx, 100, []QuestItem{
		{ItemID: 100, QuestName: "Quest A", SourceURL: "https://wiki/QuestA", Source: "notes_link", LastRefreshed: lastRefreshed},
	}); err != nil {
		t.Fatalf("replace item 100: %v", err)
	}
	var count100 int
	if err := db.QueryRow(`SELECT count(*) FROM quest_items WHERE item_id = 100`).Scan(&count100); err != nil {
		t.Fatalf("count item 100: %v", err)
	}
	if count100 != 1 {
		t.Errorf("expected 1 quest link for item 100 after replace (Quest B dropped), got %d", count100)
	}
	// Item 200 untouched.
	var count200 int
	if err := db.QueryRow(`SELECT count(*) FROM quest_items WHERE item_id = 200`).Scan(&count200); err != nil {
		t.Fatalf("count item 200: %v", err)
	}
	if count200 != 1 {
		t.Errorf("expected item 200's links untouched by an item-100 replace, got %d (want 1)", count200)
	}
}

// withTx runs fn inside a single transaction over db, rolling back afterward
// (read-only test usage). Mirrors the Store.X/XTx split: tests that exercise the
// *Tx getters directly need a *sql.Tx to pass.
func withTx(t *testing.T, db *sql.DB, fn func(tx *sql.Tx) error) error {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(tx)
}
