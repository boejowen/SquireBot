// Package migrations_test exercises the goose migration through the shared
// store.NewTestDB fixture. It is an EXTERNAL test package (migrations_test, not
// migrations) on purpose: store imports migrations, so an internal test here
// that imported store would form an import cycle. An external test package may
// depend on store, which depends on migrations — no cycle.
package migrations_test

import (
	"database/sql"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/migrations"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// allTables is the full D-13 table set the migration must create (BACKEND-02):
// owner/character/inventory_item/spellbook_entry/guild_code + the five empty
// dimension tables.
var allTables = []string{
	"owner", "character", "inventory_item", "spellbook_entry", "guild_code",
	"item_master", "pigparse_price", "wiki_spells", "wiki_gear_tier", "quest_items",
}

// dimensionTables are created empty by P11; P12 populates them.
var dimensionTables = []string{
	"item_master", "pigparse_price", "wiki_spells", "wiki_gear_tier", "quest_items",
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name,
	).Scan(&got)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		t.Fatalf("sqlite_master lookup for %q failed: %v", name, err)
	}
	return got == name
}

// TestRunMigrations_CreatesAllTables verifies goose.Up on a fresh DB creates
// every D-13 table.
func TestRunMigrations_CreatesAllTables(t *testing.T) {
	db := store.NewTestDB(t) // NewTestDB already ran RunMigrations

	for _, tbl := range allTables {
		if !tableExists(t, db, tbl) {
			t.Errorf("expected table %q to exist after migration, but it does not", tbl)
		}
	}
}

// TestRunMigrations_Idempotent proves re-running goose.Up is a no-op
// (BACKEND-02): the second call returns nil AND the goose_db_version row count
// is unchanged between the two runs.
func TestRunMigrations_Idempotent(t *testing.T) {
	db := store.NewTestDB(t) // first RunMigrations happened inside the helper

	countVersions := func() int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&n); err != nil {
			t.Fatalf("counting goose_db_version failed: %v", err)
		}
		return n
	}

	before := countVersions()

	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations should be a no-op, got error: %v", err)
	}

	after := countVersions()
	if before != after {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", before, after)
	}
}

// TestDimensionTables_Empty asserts the five dimension tables are created empty
// (P11 creates the schema; P12 populates the data).
func TestDimensionTables_Empty(t *testing.T) {
	db := store.NewTestDB(t)

	for _, tbl := range dimensionTables {
		var n int
		// #nosec G201 -- tbl is from a fixed in-test allow-list, not user input.
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + tbl).Scan(&n); err != nil {
			t.Fatalf("counting rows in %q failed: %v", tbl, err)
		}
		if n != 0 {
			t.Errorf("expected dimension table %q to be empty, got %d rows", tbl, n)
		}
	}
}

// pigparsePriceEnrichColumns are the 8 price-history columns 00003 adds to
// pigparse_price (the only dimension table 11-02 left short — RESEARCH §2).
var pigparsePriceEnrichColumns = []string{
	"t30", "a30", "t60", "a60", "t6m", "a6m", "ty", "ay",
}

// enrichTables are the two bookkeeping tables 00003 creates: the scheduler's
// durable last-run cursor (job_run) and the politeFetch ETag/304 state
// (etag_cache).
var enrichTables = []string{"job_run", "etag_cache"}

// TestMigrate_00003_AddsEnrichColumnsAndTables proves the Phase 12 forward-only
// migration 00003 applied on a fresh DB (NewTestDB runs goose.Up over ALL three
// migrations): the 8 price-history columns exist on pigparse_price AND the
// job_run + etag_cache bookkeeping tables exist. This exercises 00003 the same
// way every store/migrations test does — through the shared NewTestDB fixture.
func TestMigrate_00003_AddsEnrichColumnsAndTables(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001, 00002, 00003) + t.Cleanup

	// Collect pigparse_price's column names via PRAGMA table_info into a set.
	cols := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(pigparse_price)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(pigparse_price) failed: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		// PRAGMA table_info columns: cid, name, type, notnull, dflt_value, pk.
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scanning table_info row failed: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating table_info rows failed: %v", err)
	}

	for _, c := range pigparsePriceEnrichColumns {
		if !cols[c] {
			t.Errorf("expected pigparse_price to have column %q after 00003, but it does not (have: %v)", c, cols)
		}
	}

	// Assert job_run + etag_cache both exist via a single count query.
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('job_run','etag_cache')`,
	).Scan(&n); err != nil {
		t.Fatalf("counting enrich tables in sqlite_master failed: %v", err)
	}
	if n != len(enrichTables) {
		t.Errorf("expected %d enrich tables (job_run, etag_cache) to exist, found %d", len(enrichTables), n)
	}

	// A second RunMigrations call is a no-op: goose records applied versions, so
	// re-running over an already-at-00003 DB returns nil (forward-only/idempotent).
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00003 should be a no-op, got error: %v", err)
	}
}
