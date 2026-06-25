package store

import (
	"context"
	"database/sql"
	"testing"
)

// catalogenrich_test.go mirrors enrich_test.go's TestUpsertItemMaster_AndSHA1Getter
// for the NAME-KEYED catalog path (Phase 38, D-04 name-keyed): upsert round-trip by
// norm_name, the 4-field freshness getter (present + absent), ON CONFLICT(norm_name)
// update-in-place, and the icon/flags self-heal that proves the weekly job will re-write.

// TestUpsertCatalogEnrichment_RoundTrip upserts a CatalogEnrichment and asserts it
// round-trips by norm_name, that booleans store as 0/1, and that the freshness getter
// returns the stored (sha, icon, statsblock, flags) — with zero-values for an absent
// norm_name (Behaviors 1 & 2).
func TestUpsertCatalogEnrichment_RoundTrip(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	e := CatalogEnrichment{
		NormName: "cloak of flames", Name: "Cloak of Flames", ItemID: 90950,
		WikiSummary: "A flaming cloak.", WikiURL: "https://wiki.project1999.com/Cloak_of_Flames",
		Slot: "BACK", IsQuestItem: false,
		WikitextSHA1: "cofsha1", LastRefreshed: lastRefreshed,
		IconID: 567, Statsblock: "AC 11 HP 35",
		IsLore: true, FlagsJSON: MarshalFlags([]string{"LORE"}),
	}
	if err := s.UpsertCatalogEnrichment(ctx, e); err != nil {
		t.Fatalf("UpsertCatalogEnrichment: %v", err)
	}

	// is_lore stored as 1; the representative name + PigParse id round-trip.
	var isLore int
	var gotName string
	var gotID int64
	if err := db.QueryRow(
		`SELECT is_lore, name, item_id FROM catalog_enrichment WHERE norm_name = ?`, e.NormName,
	).Scan(&isLore, &gotName, &gotID); err != nil {
		t.Fatalf("query catalog_enrichment row: %v", err)
	}
	if isLore != 1 {
		t.Errorf("is_lore = %d, want 1 (b2i(true))", isLore)
	}
	if gotName != "Cloak of Flames" {
		t.Errorf("name = %q, want %q", gotName, "Cloak of Flames")
	}
	if gotID != 90950 {
		t.Errorf("item_id = %d, want 90950 (representative PigParse id)", gotID)
	}

	// GetCatalogEnrichmentFreshnessTx returns the stored 4-tuple for a present row,
	// and zero-values for an absent norm_name.
	if err := withTx(t, db, func(tx *sql.Tx) error {
		sha, icon, stats, flags, ferr := GetCatalogEnrichmentFreshnessTx(ctx, tx, e.NormName)
		if ferr != nil {
			return ferr
		}
		if sha != "cofsha1" {
			t.Errorf("freshness sha = %q, want %q", sha, "cofsha1")
		}
		if icon != 567 {
			t.Errorf("freshness icon = %d, want 567", icon)
		}
		if stats != "AC 11 HP 35" {
			t.Errorf("freshness statsblock = %q, want %q", stats, "AC 11 HP 35")
		}
		if flags != `["LORE"]` {
			t.Errorf("freshness flags_json = %q, want %q", flags, `["LORE"]`)
		}

		// Absent norm_name → all zero-values, nil error.
		sha2, icon2, stats2, flags2, ferr2 := GetCatalogEnrichmentFreshnessTx(ctx, tx, "no such item")
		if ferr2 != nil {
			return ferr2
		}
		if sha2 != "" || icon2 != 0 || stats2 != "" || flags2 != "" {
			t.Errorf("absent freshness = (%q,%d,%q,%q), want (\"\",0,\"\",\"\")", sha2, icon2, stats2, flags2)
		}
		return nil
	}); err != nil {
		t.Fatalf("withTx: %v", err)
	}
}

// TestUpsertCatalogEnrichment_ConflictUpdatesInPlace re-upserts the SAME norm_name
// with a different icon_id → still exactly one row, updated in place (Behavior 3,
// ON CONFLICT(norm_name)).
func TestUpsertCatalogEnrichment_ConflictUpdatesInPlace(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	base := CatalogEnrichment{
		NormName: "manastone", Name: "Manastone", ItemID: 90004,
		WikitextSHA1: "msha", LastRefreshed: lastRefreshed,
		IconID: 100, FlagsJSON: MarshalFlags(nil),
	}
	if err := s.UpsertCatalogEnrichment(ctx, base); err != nil {
		t.Fatalf("UpsertCatalogEnrichment (first): %v", err)
	}
	base.IconID = 999
	base.Name = "Manastone (updated)"
	if err := s.UpsertCatalogEnrichment(ctx, base); err != nil {
		t.Fatalf("UpsertCatalogEnrichment (update): %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM catalog_enrichment WHERE norm_name = ?`, "manastone").Scan(&count); err != nil {
		t.Fatalf("count catalog_enrichment: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 catalog_enrichment row after update, got %d (ON CONFLICT(norm_name) must update in place)", count)
	}
	var gotIcon int64
	var gotName string
	if err := db.QueryRow(`SELECT icon_id, name FROM catalog_enrichment WHERE norm_name = ?`, "manastone").Scan(&gotIcon, &gotName); err != nil {
		t.Fatalf("read updated row: %v", err)
	}
	if gotIcon != 999 || gotName != "Manastone (updated)" {
		t.Errorf("after update icon_id=%d name=%q, want 999 / \"Manastone (updated)\"", gotIcon, gotName)
	}
}

// TestUpsertCatalogEnrichment_SelfHeal proves the 4-field freshness compare will
// re-write: a row written with IconID 0 + FlagsJSON "[]" then re-upserted with IconID
// 567 + FlagsJSON ["LORE"] → the freshness getter now returns the NEW icon/flags
// (Behavior 4). This is the 00012-icon self-heal, by name: a row written before a
// successful enrichment backfills on the next weekly pass.
func TestUpsertCatalogEnrichment_SelfHeal(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// Write a "not yet fully enriched" row: icon 0, empty flag set.
	pre := CatalogEnrichment{
		NormName: "rubicite breastplate", Name: "Rubicite Breastplate", ItemID: 90100,
		WikitextSHA1: "presha", LastRefreshed: lastRefreshed,
		IconID: 0, FlagsJSON: MarshalFlags(nil),
	}
	if err := s.UpsertCatalogEnrichment(ctx, pre); err != nil {
		t.Fatalf("UpsertCatalogEnrichment (pre): %v", err)
	}

	// Confirm the pre-backfill freshness reads the icon-0 / "[]" state.
	if err := withTx(t, db, func(tx *sql.Tx) error {
		_, icon, _, flags, ferr := GetCatalogEnrichmentFreshnessTx(ctx, tx, "rubicite breastplate")
		if ferr != nil {
			return ferr
		}
		if icon != 0 || flags != "[]" {
			t.Errorf("pre-backfill freshness icon=%d flags=%q, want 0 / []", icon, flags)
		}
		return nil
	}); err != nil {
		t.Fatalf("withTx (pre): %v", err)
	}

	// Re-upsert with a real icon + a flag set (the next weekly pass after a successful fetch).
	post := pre
	post.IconID = 567
	post.FlagsJSON = MarshalFlags([]string{"LORE"})
	if err := s.UpsertCatalogEnrichment(ctx, post); err != nil {
		t.Fatalf("UpsertCatalogEnrichment (post): %v", err)
	}

	// The freshness getter now reflects the backfilled icon + flags — proving the job's
	// 4-field compare (which uses these values) would have re-written.
	if err := withTx(t, db, func(tx *sql.Tx) error {
		_, icon, _, flags, ferr := GetCatalogEnrichmentFreshnessTx(ctx, tx, "rubicite breastplate")
		if ferr != nil {
			return ferr
		}
		if icon != 567 {
			t.Errorf("post-backfill freshness icon=%d, want 567 (self-heal)", icon)
		}
		if flags != `["LORE"]` {
			t.Errorf("post-backfill freshness flags=%q, want [\"LORE\"] (self-heal)", flags)
		}
		return nil
	}); err != nil {
		t.Fatalf("withTx (post): %v", err)
	}
}
