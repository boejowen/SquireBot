package store

import (
	"context"
	"database/sql"
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

	got, err := st.SearchCatalog(ctx, "rusty", 25)
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

	got, err := st.SearchCatalog(ctx, "ruby", 25)
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

	got, err := st.SearchCatalog(ctx, "1001", 25)
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

	got, err := st.SearchCatalog(ctx, "2002", 25)
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
	got, err := st.SearchCatalog(ctx, "50%", 25)
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

	got, err := st.SearchCatalog(ctx, "potion", 3)
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

	got, err := st.SearchCatalog(ctx, "anything", 25)
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

	got, err := st.SearchCatalog(ctx, "No Price", 25)
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
