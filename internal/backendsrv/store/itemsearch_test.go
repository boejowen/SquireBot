package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

// itemsearch_test.go proves the Phase 19 D-10 full-catalog SearchCatalog over
// pigparse_price (19-01 Task 3): substring (case-insensitive) match, prefix-first
// ranking, id-equality match, an id-match on a NULL-name row returning WITHOUT
// error (sql.NullString scan — review WORTH-FIX 4), literal %/_ handling (ESCAPE),
// the LIMIT cap, and the empty-corpus → non-nil-empty-slice case. Seeds
// pigparse_price directly (the readviews_test.go:48 seed idiom).

// seedCatalogItem inserts a pigparse_price row. A NULL name is requested by
// passing nameValid=false (the catalog has nullable name — review WORTH-FIX 4).
func seedCatalogItem(t *testing.T, db *sql.DB, itemID int64, name string, nameValid bool, currentAvg float64) {
	t.Helper()
	var nameArg any
	if nameValid {
		nameArg = name
	} else {
		nameArg = nil
	}
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, last_refreshed)
		 VALUES (?, ?, ?, datetime('now'))`,
		itemID, nameArg, currentAvg,
	); err != nil {
		t.Fatalf("seed pigparse_price (item_id=%d): %v", itemID, err)
	}
}

// seedHeldFlag inserts an item_master row (held, EQ-id-keyed) carrying the Phase 37
// (00016) is_clicky/has_haste flags. The catalog facet union reads these by lower(trim(name)),
// NEVER by item_id — so the EQ id here is deliberately distinct from any pigparse_price id.
func seedHeldFlag(t *testing.T, db *sql.DB, itemID int64, name string, clicky, haste bool) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, is_clicky, has_haste, last_refreshed)
		 VALUES (?, ?, ?, ?, datetime('now'))`,
		itemID, name, boolToInt(clicky), boolToInt(haste),
	); err != nil {
		t.Fatalf("seed item_master flag (item_id=%d): %v", itemID, err)
	}
}

// seedCatalogEnrichmentFlag inserts a catalog_enrichment row (catalog-only, norm_name-keyed)
// carrying the Phase 38 (00017) is_clicky/has_haste flags — the catalog-only half of the
// name-keyed flag union (a name with NO item_master row).
func seedCatalogEnrichmentFlag(t *testing.T, db *sql.DB, name string, clicky, haste bool) {
	t.Helper()
	norm := strings.ToLower(strings.TrimSpace(name))
	if _, err := db.Exec(
		`INSERT INTO catalog_enrichment (norm_name, name, is_clicky, has_haste, last_refreshed)
		 VALUES (?, ?, ?, ?, datetime('now'))`,
		norm, name, boolToInt(clicky), boolToInt(haste),
	); err != nil {
		t.Fatalf("seed catalog_enrichment flag (%q): %v", name, err)
	}
}

func names(items []CatalogItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

func contains(items []CatalogItem, itemID int64) bool {
	for _, it := range items {
		if it.ItemID == itemID {
			return true
		}
	}
	return false
}

func TestSearchCatalog_SubstringCaseInsensitive(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	seedCatalogItem(t, db, 1, "Rusty Dagger", true, 1.0)
	seedCatalogItem(t, db, 2, "Encrusted Ruby", true, 2.0) // mid-string "rust"... actually "rus" in "Encrusted"
	seedCatalogItem(t, db, 3, "Cloak of Flames", true, 3.0)

	got, err := st.SearchCatalog(ctx, "rusty", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if !contains(got, 1) {
		t.Errorf("SearchCatalog(\"rusty\") = %v, want it to include item 1 (Rusty Dagger)", names(got))
	}
	if contains(got, 3) {
		t.Errorf("SearchCatalog(\"rusty\") wrongly included item 3 (Cloak of Flames)")
	}
}

func TestSearchCatalog_PrefixRankedAhead(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// Both contain "ruby"; only "Ruby Crown" STARTS WITH it.
	seedCatalogItem(t, db, 10, "Encrusted Ruby", true, 1.0)
	seedCatalogItem(t, db, 11, "Ruby Crown", true, 1.0)

	got, err := st.SearchCatalog(ctx, "ruby", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("SearchCatalog(\"ruby\") len = %d, want 2 (%v)", len(got), names(got))
	}
	if got[0].ItemID != 11 {
		t.Errorf("prefix match should rank first: got order %v, want item 11 (Ruby Crown) first", names(got))
	}
}

func TestSearchCatalog_IdEqualityMatch(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	seedCatalogItem(t, db, 1001, "Some Item", true, 1.0)
	seedCatalogItem(t, db, 9999, "Other Item", true, 1.0)

	got, err := st.SearchCatalog(ctx, "1001", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if !contains(got, 1001) {
		t.Errorf("SearchCatalog(\"1001\") = %v, want it to match item_id 1001 by id", names(got))
	}
}

func TestSearchCatalog_IdMatchOnNullNameRow(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// A pigparse_price row with a NULL name (review WORTH-FIX 4) and a known id.
	seedCatalogItem(t, db, 2002, "", false /* NULL name */, 0)

	got, err := st.SearchCatalog(ctx, "2002", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog on a NULL-name id match errored: %v (want no error)", err)
	}
	if !contains(got, 2002) {
		t.Errorf("SearchCatalog(\"2002\") = %v, want it to return the NULL-name row 2002", got)
	}
	// The NULL name resolves to an empty string, not a panic/error.
	for _, it := range got {
		if it.ItemID == 2002 && it.Name != "" {
			t.Errorf("NULL-name row 2002 Name = %q, want empty string", it.Name)
		}
	}
}

func TestSearchCatalog_LiteralWildcardChars(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	seedCatalogItem(t, db, 1, "50% Off Token", true, 1.0)
	seedCatalogItem(t, db, 2, "Plain Cloak", true, 1.0)

	// A literal "%" must match only the row that actually contains "%", NOT every
	// row (which a bare unescaped "%" wildcard would).
	got, err := st.SearchCatalog(ctx, "50%", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if !contains(got, 1) {
		t.Errorf("SearchCatalog(\"50%%\") = %v, want it to match the literal-%% row 1", names(got))
	}
	if contains(got, 2) {
		t.Errorf("SearchCatalog(\"50%%\") wrongly matched row 2 (the %% was treated as a wildcard)")
	}
}

func TestSearchCatalog_LimitCap(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	for i := int64(1); i <= 10; i++ {
		seedCatalogItem(t, db, i, "Potion of Healing", true, 1.0)
	}

	got, err := st.SearchCatalog(ctx, "potion", false, false, 3)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if len(got) > 3 {
		t.Errorf("SearchCatalog limit=3 returned %d rows, want at most 3", len(got))
	}
}

func TestSearchCatalog_EmptyCorpusReturnsNonNilSlice(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)

	got, err := st.SearchCatalog(ctx, "anything", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog on empty corpus errored: %v", err)
	}
	if got == nil {
		t.Fatalf("SearchCatalog on empty corpus returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("SearchCatalog on empty corpus len = %d, want 0", len(got))
	}
}

func TestSearchCatalog_CurrentAvgNullable(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// current_avg is REAL and nullable; insert one without it.
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, last_refreshed) VALUES (?, ?, datetime('now'))`,
		3003, "No Price Item"); err != nil {
		t.Fatalf("seed null-price row: %v", err)
	}

	got, err := st.SearchCatalog(ctx, "No Price", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if !contains(got, 3003) {
		t.Fatalf("SearchCatalog(\"No Price\") = %v, want item 3003", names(got))
	}
	for _, it := range got {
		if it.ItemID == 3003 && it.CurrentAvg != nil {
			t.Errorf("row 3003 CurrentAvg = %v, want nil (NULL current_avg)", *it.CurrentAvg)
		}
	}
}

// ── Phase 39 facet cases (SEARCH-04/05) ────────────────────────────────────────

func TestSearchCatalog_ClickyOnly(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// Two "cloak" catalog rows; flag ONE clicky (held via item_master, EQ id 1 != catalog 100).
	seedCatalogItem(t, db, 100, "Shadow Cloak", true, 1.0)
	seedCatalogItem(t, db, 101, "Plain Cloak", true, 1.0)
	seedHeldFlag(t, db, 1, "Shadow Cloak", true, false)

	got, err := st.SearchCatalog(ctx, "cloak", true, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog clicky: %v", err)
	}
	if !contains(got, 100) {
		t.Errorf("clicky filter = %v, want it to include the clicky row 100 (Shadow Cloak)", names(got))
	}
	if contains(got, 101) {
		t.Errorf("clicky filter wrongly included the non-clicky row 101 (Plain Cloak)")
	}
}

func TestSearchCatalog_HasteOnly(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	seedCatalogItem(t, db, 200, "Haste Belt", true, 1.0)
	seedCatalogItem(t, db, 201, "Slow Belt", true, 1.0)
	seedHeldFlag(t, db, 2, "Haste Belt", false, true)

	got, err := st.SearchCatalog(ctx, "belt", false, true, 25)
	if err != nil {
		t.Fatalf("SearchCatalog haste: %v", err)
	}
	if !contains(got, 200) {
		t.Errorf("haste filter = %v, want it to include the haste row 200 (Haste Belt)", names(got))
	}
	if contains(got, 201) {
		t.Errorf("haste filter wrongly included the non-haste row 201 (Slow Belt)")
	}
}

func TestSearchCatalog_BothFacets(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// Ring A: both clicky AND haste; Ring B: clicky only; Ring C: haste only.
	seedCatalogItem(t, db, 300, "Both Ring", true, 1.0)
	seedCatalogItem(t, db, 301, "Clicky Ring", true, 1.0)
	seedCatalogItem(t, db, 302, "Haste Ring", true, 1.0)
	seedHeldFlag(t, db, 3, "Both Ring", true, true)
	seedHeldFlag(t, db, 4, "Clicky Ring", true, false)
	seedHeldFlag(t, db, 5, "Haste Ring", false, true)

	got, err := st.SearchCatalog(ctx, "ring", true, true, 25)
	if err != nil {
		t.Fatalf("SearchCatalog both: %v", err)
	}
	if !contains(got, 300) {
		t.Errorf("both-facet filter = %v, want the intersection row 300 (Both Ring)", names(got))
	}
	if contains(got, 301) || contains(got, 302) {
		t.Errorf("both-facet filter = %v, want ONLY the intersection (not the clicky-only/haste-only rows)", names(got))
	}
}

func TestSearchCatalog_CatalogOnlyFlag(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// "Etched Cloak" is flagged ONLY in catalog_enrichment (no item_master row) — the
	// catalog-only half of the union. "Drab Cloak" has no enrichment at all.
	seedCatalogItem(t, db, 400, "Etched Cloak", true, 1.0)
	seedCatalogItem(t, db, 401, "Drab Cloak", true, 1.0)
	seedCatalogEnrichmentFlag(t, db, "Etched Cloak", true, false)

	got, err := st.SearchCatalog(ctx, "cloak", true, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog catalog-only flag: %v", err)
	}
	if !contains(got, 400) {
		t.Errorf("catalog-only flag = %v, want the enrichment-flagged row 400 (Etched Cloak)", names(got))
	}
	if contains(got, 401) {
		t.Errorf("catalog-only flag wrongly included the unenriched row 401 (Drab Cloak)")
	}
}

func TestSearchCatalog_SameNameHeldNoFanout(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// BL-01 regression: item_master is keyed by item_id, so two DISTINCT held EQ ids can
	// share a normalized name (same-name spell scrolls / quest turn-ins). The name-keyed
	// facet LEFT JOIN must NOT fan the single catalog row out to one row per same-name held
	// row — duplicate rows crash the web {#each} (each_key_duplicate) and undercount LIMIT.
	seedCatalogItem(t, db, 500, "Words of Dimension", true, 1.0)
	seedHeldFlag(t, db, 10, "Words of Dimension", true, false) // EQ id 10, clicky
	seedHeldFlag(t, db, 11, "Words of Dimension", true, false) // EQ id 11, SAME name, also clicky

	got, err := st.SearchCatalog(ctx, "words", true, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog same-name held: %v", err)
	}
	n := 0
	for _, it := range got {
		if it.ItemID == 500 {
			n++
		}
	}
	if n != 1 {
		t.Errorf("same-name held fan-out: catalog row 500 returned %d times, want exactly 1 (duplicate rows crash the web {#each}); got %v", n, names(got))
	}
}

func TestSearchCatalog_NoFacetRegression(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	st := NewStore(db)
	// A clicky row, a plain row, and a row with NO enrichment row at all. With no facet
	// active the LEFT JOIN is absent and EVERY matching row appears — including the
	// unenriched one (the Pitfall 3 / INNER-JOIN-drop guard) — in the original prefix-first order.
	seedCatalogItem(t, db, 500, "Cloak of Flames", true, 1.0) // STARTS WITH "cloak" → prefix-first
	seedCatalogItem(t, db, 501, "Black Cloak", true, 1.0)
	seedCatalogItem(t, db, 502, "Plain Cloak", true, 1.0) // never enriched
	seedHeldFlag(t, db, 6, "Cloak of Flames", true, false)

	got, err := st.SearchCatalog(ctx, "cloak", false, false, 25)
	if err != nil {
		t.Fatalf("SearchCatalog no-facet: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("no-facet search len = %d, want 3 (all matching rows incl. the unenriched one): %v", len(got), names(got))
	}
	for _, id := range []int64{500, 501, 502} {
		if !contains(got, id) {
			t.Errorf("no-facet search dropped row %d: %v (an unenriched row must still appear)", id, names(got))
		}
	}
	// Prefix-first ordering preserved (Open Q1 default): "Cloak of Flames" (prefix) ranks first.
	if got[0].ItemID != 500 {
		t.Errorf("no-facet ordering: got[0] = %q, want Cloak of Flames (prefix-first) first", got[0].Name)
	}
}
