package store

import (
	"context"
	"strings"
	"testing"
)

// seedPre00016Row inserts an item_master row carrying a stored (cleaned, no-bracket,
// newline-separated) statsblock but with flags_json forced back to NULL — exactly the
// state of a row enriched BEFORE migration 00016 added the flag/effect columns. The
// upsert always writes flags_json (via MarshalFlags), so we null it afterward to
// simulate the pre-migration row the backfill must heal (mirrors the wiki icon-backfill
// test's `UPDATE ... SET icon_id=0`).
func seedPre00016Row(t *testing.T, s *Store, itemID int, name, statsblock string) {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertItemMaster(ctx, ItemMaster{
		ItemID: itemID, Name: name, Statsblock: statsblock,
		WikitextSHA1: "sha-" + name, LastRefreshed: lastRefreshed,
		// FlagsJSON left "" → MarshalFlags("") path is irrelevant; we null it next.
	}); err != nil {
		t.Fatalf("seed item_master %q: %v", name, err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE item_master SET flags_json = NULL WHERE item_id = ?`, itemID); err != nil {
		t.Fatalf("null flags_json for %q: %v", name, err)
	}
}

// TestBackfillItemFlags_ClickyFromStoredStatsblock proves the D-05 no-network backfill:
// a row whose STORED statsblock is the cleaned, bracket-stripped form INCLUDING an Effect
// line re-derives is_magic + is_clicky + the clicky NAME from that no-bracket line (the
// exact field SEARCH-04 needs), stores flags_json as a real array, and a SECOND pass is a
// no-op (idempotent — keyed on flags_json IS NULL).
func TestBackfillItemFlags_ClickyFromStoredStatsblock(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	const id = 7001
	// The CLEANED, NO-BRACKET stored form (cleanStatsblock ran before storage):
	// "Effect: Shock of Frost (Click from Inventory)" — NOT the raw "[[Shock of Frost]] ...".
	seedPre00016Row(t, s, id, "Staff of Temperate Flux",
		"MAGIC ITEM\nSlot: PRIMARY\nEffect: Shock of Frost (Click from Inventory)\nClass: ALL")

	scanned, updated, err := s.BackfillItemFlags(ctx)
	if err != nil {
		t.Fatalf("BackfillItemFlags: %v", err)
	}
	if scanned < 1 || updated < 1 {
		t.Fatalf("first backfill scanned=%d updated=%d, want >= 1 each", scanned, updated)
	}

	var isMagic, isClicky int
	var clickyEffect, flagsJSON string
	if err := db.QueryRow(
		`SELECT is_magic, is_clicky, clicky_effect, flags_json FROM item_master WHERE item_id = ?`, id,
	).Scan(&isMagic, &isClicky, &clickyEffect, &flagsJSON); err != nil {
		t.Fatalf("read backfilled row: %v", err)
	}
	if isMagic != 1 {
		t.Errorf("is_magic = %d, want 1 (MAGIC ITEM flag from the stored statsblock)", isMagic)
	}
	if isClicky != 1 {
		t.Errorf("is_clicky = %d, want 1 (the no-bracket 'Effect: ... (Click ...)' line must classify, D-05)", isClicky)
	}
	if clickyEffect != "Shock of Frost" {
		t.Errorf("clicky_effect = %q, want %q (the clicky NAME from the bracket-stripped Effect line — SEARCH-04)", clickyEffect, "Shock of Frost")
	}
	if !strings.Contains(flagsJSON, "MAGIC ITEM") {
		t.Errorf("flags_json = %q, want it to contain \"MAGIC ITEM\"", flagsJSON)
	}

	// Idempotent: a SECOND pass re-writes nothing (flags_json is now populated, so the
	// row drops out of the candidate set — D-06).
	_, updated2, err := s.BackfillItemFlags(ctx)
	if err != nil {
		t.Fatalf("second BackfillItemFlags: %v", err)
	}
	if updated2 != 0 {
		t.Errorf("second backfill updated = %d, want 0 (idempotent — flags_json IS NULL key)", updated2)
	}
}

// TestBackfillItemFlags_HasteNoEffect covers the haste-positive / clicky-negative path:
// a stored statsblock with a Haste line and NO Effect line backfills to has_haste=1,
// haste_pct=36, is_clicky=0.
func TestBackfillItemFlags_HasteNoEffect(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	const id = 7002
	seedPre00016Row(t, s, id, "Cloak of Flames",
		"MAGIC ITEM\nSlot: BACK\nHaste: +36%\nClass: ALL")

	if _, _, err := s.BackfillItemFlags(ctx); err != nil {
		t.Fatalf("BackfillItemFlags: %v", err)
	}

	var hasHaste, hastePct, isClicky int
	if err := db.QueryRow(
		`SELECT has_haste, haste_pct, is_clicky FROM item_master WHERE item_id = ?`, id,
	).Scan(&hasHaste, &hastePct, &isClicky); err != nil {
		t.Fatalf("read backfilled row: %v", err)
	}
	if hasHaste != 1 || hastePct != 36 {
		t.Errorf("has_haste/haste_pct = %d/%d, want 1/36", hasHaste, hastePct)
	}
	if isClicky != 0 {
		t.Errorf("is_clicky = %d, want 0 (no Effect line ⇒ not a clicky)", isClicky)
	}
}

// TestBackfillItemFlags_FlaglessStoresEmptyArray proves a statsblock with NO recognized
// flags backfills flags_json to the literal "[]" (NOT NULL, NOT "null") — so the row
// counts as backfilled and is never re-scanned, and so it byte-matches the weekly
// freshness compare's MarshalFlags(nil) (D-06).
func TestBackfillItemFlags_FlaglessStoresEmptyArray(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	const id = 7003
	// A plain stat block: only key:value lines, no all-caps flag line.
	seedPre00016Row(t, s, id, "Plain Item", "Slot: HANDS\nAC: 5\nClass: ALL")

	if _, _, err := s.BackfillItemFlags(ctx); err != nil {
		t.Fatalf("BackfillItemFlags: %v", err)
	}

	var flagsJSON string
	if err := db.QueryRow(`SELECT flags_json FROM item_master WHERE item_id = ?`, id).Scan(&flagsJSON); err != nil {
		t.Fatalf("read flags_json: %v", err)
	}
	if flagsJSON != "[]" {
		t.Errorf("flagless flags_json = %q, want \"[]\" (never NULL/null — D-06 idempotency)", flagsJSON)
	}

	// And it is now out of the candidate set: a second pass updates nothing.
	if _, updated2, err := s.BackfillItemFlags(ctx); err != nil {
		t.Fatalf("second BackfillItemFlags: %v", err)
	} else if updated2 != 0 {
		t.Errorf("second backfill updated = %d, want 0 (the flagless row's \"[]\" drops it from the candidate set)", updated2)
	}
}
