package readapi_test

// itemsearch_test.go is the httptest proof of the D-10 catalog-search handler
// (19-02 Task 2). It proves the HANDLER contract Plan 03 wires to:
//   - GET ?q=2+chars → 200 + the seeded matches;
//   - GET ?q=1char and ?q= → 200 + [] WITHOUT touching the store (short-circuit);
//   - an empty pigparse_price corpus → 200 + [] (never a 500 — Pitfall A2);
//   - a non-GET request → 405.
//
// It seeds a migrated temp DB (store.NewTestDB) with a few pigparse_price rows via
// a self-contained raw INSERT (the store package's own seed helper is package-
// private), mirroring the column layout in migrations/00001_init.sql.

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/readapi"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// seedCatalog inserts a pigparse_price row (the full-Blue-catalog corpus).
func seedCatalog(t *testing.T, db *sql.DB, itemID int64, name string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pigparse_price (item_id, name, current_avg, last_refreshed)
		 VALUES (?, ?, ?, datetime('now'))`,
		itemID, name, 100.0,
	); err != nil {
		t.Fatalf("seed pigparse_price (item_id=%d): %v", itemID, err)
	}
}

// seedHeldFlag inserts an item_master row carrying the Phase 37 (00016) is_clicky/has_haste
// flags — the held half of the name-keyed flag union SearchCatalog reads. The EQ item_id is
// deliberately distinct from any pigparse_price id (the union joins by name, never id).
func seedHeldFlag(t *testing.T, db *sql.DB, itemID int64, name string, clicky, haste int) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, is_clicky, has_haste, last_refreshed)
		 VALUES (?, ?, ?, ?, datetime('now'))`,
		itemID, name, clicky, haste,
	); err != nil {
		t.Fatalf("seed item_master flag (item_id=%d): %v", itemID, err)
	}
}

func decodeItems(t *testing.T, rec *httptest.ResponseRecorder) []store.CatalogItem {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var items []store.CatalogItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode items %q: %v", rec.Body.String(), err)
	}
	return items
}

func TestItemSearch_Matches(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)
	seedCatalog(t, db, 1, "Rusty Dagger")
	seedCatalog(t, db, 2, "Rusty Short Sword")
	seedCatalog(t, db, 3, "Fungi Tunic")

	h := readapi.NewItemSearch(st)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/search?q=rusty", nil))
	items := decodeItems(t, rec)
	if len(items) != 2 {
		t.Fatalf("q=rusty returned %d items, want 2 (%+v)", len(items), items)
	}
}

func TestItemSearch_ShortQuery_ShortCircuitsEmpty(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)
	seedCatalog(t, db, 1, "Rusty Dagger")
	h := readapi.NewItemSearch(st)

	for _, q := range []string{"?q=r", "?q="} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/search"+q, nil))
		items := decodeItems(t, rec)
		if len(items) != 0 {
			t.Fatalf("%s returned %d items, want [] (short-circuit)", q, len(items))
		}
		// The body must be a non-nil array, not null.
		if body := rec.Body.String(); body != "[]\n" && body != "[]" {
			t.Fatalf("%s body = %q, want []", q, body)
		}
	}
}

func TestItemSearch_EmptyCorpus_GracefulEmpty(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db) // no pigparse_price rows
	h := readapi.NewItemSearch(st)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/search?q=anything", nil))
	items := decodeItems(t, rec) // asserts 200, never 500 (Pitfall A2)
	if len(items) != 0 {
		t.Fatalf("empty corpus returned %d items, want []", len(items))
	}
}

func TestItemSearch_NonGet_405(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)
	h := readapi.NewItemSearch(st)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/items/search?q=rusty", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}

// containsID reports whether the decoded result set includes the given catalog item_id.
func containsID(items []store.CatalogItem, itemID int64) bool {
	for _, it := range items {
		if it.ItemID == itemID {
			return true
		}
	}
	return false
}

// TestItemSearch_ClickyParam proves ?clicky=1 reaches SearchCatalog as clicky=true and
// AND-narrows the result to the flagged row (the flag seeded via item_master, joined by name).
func TestItemSearch_ClickyParam(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)
	seedCatalog(t, db, 700, "Shadow Cloak") // clicky
	seedCatalog(t, db, 701, "Plain Cloak")  // not clicky
	seedHeldFlag(t, db, 7, "Shadow Cloak", 1, 0)

	h := readapi.NewItemSearch(st)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/search?q=cloak&clicky=1", nil))
	items := decodeItems(t, rec)
	if !containsID(items, 700) {
		t.Errorf("?clicky=1 = %+v, want the clicky row 700 (Shadow Cloak)", items)
	}
	if containsID(items, 701) {
		t.Errorf("?clicky=1 wrongly included the non-clicky row 701 (Plain Cloak)")
	}
}

// TestItemSearch_GuardWithFacet proves the 2-rune guard still short-circuits to [] BEFORE
// any DB hit even with ?clicky=1 present (resolved Open Q2: no empty-/short-q corpus dump).
func TestItemSearch_GuardWithFacet(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)
	seedCatalog(t, db, 800, "Acrylia Cloak")
	seedHeldFlag(t, db, 8, "Acrylia Cloak", 1, 0)

	h := readapi.NewItemSearch(st)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/search?q=a&clicky=1", nil))
	items := decodeItems(t, rec) // asserts 200
	if len(items) != 0 {
		t.Fatalf("?q=a&clicky=1 returned %d items, want [] (the 2-rune guard fires before the DB)", len(items))
	}
	// The body must be a non-nil array, not null.
	if body := rec.Body.String(); body != "[]\n" && body != "[]" {
		t.Fatalf("?q=a&clicky=1 body = %q, want []", body)
	}
}
