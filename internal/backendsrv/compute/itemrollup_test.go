package compute_test

// itemrollup_test.go proves the Phase 32 item-centric rollup (ITEM-01..03): grouping
// View's per-instance rows by NORMALIZED NAME (never item_id) into one rollup per name
// with summed qty, distinct holder count, viewer is_mine, name-keyed price, id-correct
// icon/stats, and a per-holder list with classifySlot labels.
//
// It mirrors view_test.go's seeded-temp-DB pattern (external compute_test package over
// store.NewTestDB) and adds the icon/stats + character_assignment seeds the rollup needs.
// Pure buildItemRollups assertions run via Items(ctx, store, viewer) end-to-end so the
// grouping/flag-propagation/slot-label logic is exercised over real store reads.

import (
	"context"
	"database/sql"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// seedWebUser inserts a web_user row (the character_assignment FK target).
func seedWebUser(t *testing.T, db *sql.DB, discordUserID, username string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?,?,NULL,0,0)`, discordUserID, username,
	); err != nil {
		t.Fatalf("seed web_user %q: %v", discordUserID, err)
	}
}

// seedAssignment assigns charID to discordUserID (the viewer "yours"/is_mine source).
func seedAssignment(t *testing.T, db *sql.DB, charID int64, discordUserID string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?,?,0,'test')`, charID, discordUserID,
	); err != nil {
		t.Fatalf("seed character_assignment (char=%d, %s): %v", charID, discordUserID, err)
	}
}

// setItemIconStats stamps icon_id + statsblock onto an already-seeded item_master row
// (seedItemMaster leaves both NULL — the P31 00012/00013 columns).
func setItemIconStats(t *testing.T, db *sql.DB, itemID, iconID int64, statsblock string) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE item_master SET icon_id = ?, statsblock = ? WHERE item_id = ?`,
		iconID, statsblock, itemID,
	); err != nil {
		t.Fatalf("set icon/stats on %d: %v", itemID, err)
	}
}

// setItemFlags stamps the Phase 37 (00016) is_clicky/has_haste booleans onto an
// already-seeded item_master row (seedItemMaster leaves both NULL). 1 = true, 0 = false;
// these are the columns the Phase 39 holdings facet reads through the ItemRollup.
func setItemFlags(t *testing.T, db *sql.DB, itemID int64, clicky, haste bool) {
	t.Helper()
	bit := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	if _, err := db.Exec(
		`UPDATE item_master SET is_clicky = ?, has_haste = ? WHERE item_id = ?`,
		bit(clicky), bit(haste), itemID,
	); err != nil {
		t.Fatalf("set flags on %d: %v", itemID, err)
	}
}

// setItemDisplayFlags stamps the Phase 40 (00016) is_no_drop/is_lore/is_magic booleans onto
// an already-seeded item_master row (seedItemMaster leaves all three NULL). These are the
// columns the ITEMUI-01 tile/examine outline reads through the ItemRollup (rollup source).
func setItemDisplayFlags(t *testing.T, db *sql.DB, itemID int64, noDrop, lore, magic bool) {
	t.Helper()
	bit := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	if _, err := db.Exec(
		`UPDATE item_master SET is_no_drop = ?, is_lore = ?, is_magic = ? WHERE item_id = ?`,
		bit(noDrop), bit(lore), bit(magic), itemID,
	); err != nil {
		t.Fatalf("set display flags on %d: %v", itemID, err)
	}
}

// TestItems_GroupsByNameWithHoldersAndFlags is the end-to-end ITEM-01..03 proof over a
// seeded DB: the SAME item held by two chars in different slots collapses to ONE rollup
// (summed_qty = Σ, holder_count = distinct chars), an unpriced item has nil Price, a
// priced item carries its pickPrice, an icon/stats item populates IconID/Statsblock, a
// bagged + loose copy of one name sum into one rollup (the bagged holder labels "Bag"),
// the viewer's holder propagates is_mine, and no coin/empty-name row leaks.
func TestItems_GroupsByNameWithHoldersAndFlags(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	const viewer = "discord-viewer-1"
	seedWebUser(t, db, viewer, "Viewer")

	// Two chars: Apple is the viewer's (assigned), Banktoon is a guild bank toon.
	apple := seedChar(t, db, "owner-a", "Apple", "NEC", 60, "HUM", false)
	bank := seedChar(t, db, "owner-b", "Banktoon", "WAR", 60, "HUM", true)
	seedAssignment(t, db, apple, viewer)

	// "Jade Reaver" held by BOTH chars in different slots (Apple worn Primary, Banktoon
	// in Bank1) → ONE rollup, summed_qty = 2, holder_count = 2, is_mine = true (Apple).
	seedItemMaster(t, db, 1001, "Jade Reaver", "A fine blade.", "https://wiki/Jade", false)
	setItemIconStats(t, db, 1001, 560, "MAGIC ITEM\nDMG: 14")
	setItemFlags(t, db, 1001, true, false)                      // Phase 39: is_clicky=1, has_haste=0 → rollup booleans propagate
	setItemDisplayFlags(t, db, 1001, true, false, true)         // Phase 40: is_no_drop=1, is_lore=0, is_magic=1 → rollup booleans propagate
	seedQuest(t, db, 1001, "Jade Reaver Turn-In", "notes_link") // ITEMUI-02: named quest with source_url="http://example/q"
	seedPigparse(t, db, 9001, "Jade Reaver", "0", 1500, 7)      // WTS, name-bridged (catalog id 9001 != EQ 1001)
	seedInv(t, db, apple, "Primary", "Jade Reaver", 1001, 1)
	seedInv(t, db, bank, "Bank1", "Jade Reaver", 1001, 1)

	// "Bone Chips" held by Apple as a LOOSE copy AND a BAGGED copy (in a General4 bag)
	// → ONE rollup, summed_qty = 2 (seedInv counts each copy as 1), holder_count = 1,
	// the bagged copy labels "Bag" (proving bag-vs-loose copies of one name collapse).
	seedInv(t, db, apple, "General1", "Bone Chips", 13073, 2)
	seedInv(t, db, apple, "General4-Slot1", "Bone Chips", 13073, 3)

	// "Worthless Trinket" held only by Banktoon, NO matching pigparse row → Price nil.
	seedItemMaster(t, db, 9997, "Worthless Trinket", "Junk.", "https://wiki/Trinket", false)
	seedInv(t, db, bank, "General2", "Worthless Trinket", 9997, 1)

	rolls, err := compute.Items(ctx, s, viewer)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}

	byName := map[string]compute.ItemRollup{}
	for _, r := range rolls {
		byName[r.Name] = r
		if r.Name == "" {
			t.Errorf("rollup with empty name leaked (coin/empty-slot row): %+v", r)
		}
	}

	// Exactly three distinct items (Jade Reaver, Bone Chips, Worthless Trinket).
	if len(rolls) != 3 {
		t.Fatalf("got %d rollups, want 3 (one per normalized name): %+v", len(rolls), rolls)
	}

	// Jade Reaver: ONE rollup across two chars/slots.
	jade := byName["Jade Reaver"]
	if jade.SummedQty != 2 {
		t.Errorf("Jade Reaver summed_qty = %d, want 2 (Apple Primary + Banktoon Bank1)", jade.SummedQty)
	}
	if jade.HolderCount != 2 {
		t.Errorf("Jade Reaver holder_count = %d, want 2 (Apple + Banktoon)", jade.HolderCount)
	}
	if !jade.IsMine {
		t.Errorf("Jade Reaver is_mine = false, want true (Apple is the viewer's)")
	}
	if jade.Price == nil || *jade.Price != 1500 {
		t.Errorf("Jade Reaver price = %v, want 1500 (name-bridged pickPrice)", jade.Price)
	}
	if jade.IconID != 560 || jade.Statsblock != "MAGIC ITEM\nDMG: 14" {
		t.Errorf("Jade Reaver icon/stats = {%d, %q}, want 560 / the stat block", jade.IconID, jade.Statsblock)
	}
	// Phase 39: the holdings-facet booleans propagate from item_master (00016) through the rollup.
	if !jade.IsClicky || jade.HasHaste {
		t.Errorf("Jade Reaver flags = {clicky:%t haste:%t}, want clicky / not-haste (is_clicky=1, has_haste=0)", jade.IsClicky, jade.HasHaste)
	}
	// Phase 40: the display flags (is_no_drop=1, is_lore=0, is_magic=1) propagate id-correctly.
	if !jade.IsNoDrop || jade.IsLore || !jade.IsMagic {
		t.Errorf("Jade Reaver display flags = {noDrop:%t lore:%t magic:%t}, want true/false/true (item_master 00016)", jade.IsNoDrop, jade.IsLore, jade.IsMagic)
	}
	// Phase 40: the named quest link (copied from the representative ViewRow) carries source_url.
	if len(jade.QuestLinks) != 1 || jade.QuestLinks[0].QuestName != "Jade Reaver Turn-In" || jade.QuestLinks[0].SourceURL != "http://example/q" {
		t.Errorf("Jade Reaver quest_links = %+v, want 1 (Jade Reaver Turn-In / http://example/q)", jade.QuestLinks)
	}
	if len(jade.Holders) != 2 {
		t.Fatalf("Jade Reaver holders = %d, want 2: %+v", len(jade.Holders), jade.Holders)
	}
	holderByChar := map[string]compute.ItemHolder{}
	for _, h := range jade.Holders {
		holderByChar[h.Char] = h
	}
	if h := holderByChar["Apple"]; h.SlotLabel != "Worn · Primary" || !h.IsMine || h.IsBank {
		t.Errorf("Apple holder = {slot:%q mine:%t bank:%t}, want Worn · Primary / mine / not-bank", h.SlotLabel, h.IsMine, h.IsBank)
	}
	if h := holderByChar["Banktoon"]; h.SlotLabel != "Bank · Bank1" || h.IsMine || !h.IsBank {
		t.Errorf("Banktoon holder = {slot:%q mine:%t bank:%t}, want Bank · Bank1 / not-mine / bank", h.SlotLabel, h.IsMine, h.IsBank)
	}

	// Bone Chips: bagged + loose copies sum into ONE rollup, holder_count = 1.
	bc := byName["Bone Chips"]
	if bc.SummedQty != 2 {
		t.Errorf("Bone Chips summed_qty = %d, want 2 (loose copy + bagged copy, each count 1)", bc.SummedQty)
	}
	if bc.HolderCount != 1 {
		t.Errorf("Bone Chips holder_count = %d, want 1 (only Apple)", bc.HolderCount)
	}
	if len(bc.Holders) != 2 {
		t.Fatalf("Bone Chips holders = %d, want 2 (loose + bagged copy): %+v", len(bc.Holders), bc.Holders)
	}
	var sawBag, sawGeneral bool
	for _, h := range bc.Holders {
		switch h.SlotLabel {
		case "Bag":
			sawBag = true
		case "General · General1":
			sawGeneral = true
		}
	}
	if !sawBag {
		t.Errorf("Bone Chips: bagged copy did not label \"Bag\": %+v", bc.Holders)
	}
	if !sawGeneral {
		t.Errorf("Bone Chips: loose copy did not label \"General · General1\": %+v", bc.Holders)
	}

	// Worthless Trinket: no matching pigparse → Price nil; not the viewer's → is_mine false.
	trinket := byName["Worthless Trinket"]
	if trinket.Price != nil {
		t.Errorf("Worthless Trinket price = %v, want nil (no matching pigparse row)", trinket.Price)
	}
	if trinket.IsMine {
		t.Errorf("Worthless Trinket is_mine = true, want false (only the bank toon holds it)")
	}
	// Phase 39: a never-flagged (NULL is_clicky/has_haste) row scans to false, false.
	if trinket.IsClicky || trinket.HasHaste {
		t.Errorf("Worthless Trinket flags = {clicky:%t haste:%t}, want false/false (NULL flag columns)", trinket.IsClicky, trinket.HasHaste)
	}
	// Phase 40: a never-flagged, non-quest item carries all-false display flags + nil quest_links.
	if trinket.IsNoDrop || trinket.IsLore || trinket.IsMagic {
		t.Errorf("Worthless Trinket display flags = {noDrop:%t lore:%t magic:%t}, want all false (NULL flag columns)", trinket.IsNoDrop, trinket.IsLore, trinket.IsMagic)
	}
	if trinket.QuestLinks != nil {
		t.Errorf("Worthless Trinket quest_links = %+v, want nil (no quest_items row)", trinket.QuestLinks)
	}
}
