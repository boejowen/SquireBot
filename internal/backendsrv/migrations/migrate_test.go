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
