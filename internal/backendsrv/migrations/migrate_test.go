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

// webAuthTables are the four NEW tables 00004_web_auth.sql creates (Phase 15
// plan 15-01): the Discord-login web user, opaque hashed sessions, the officer
// allowlist keyed by Discord ID, and the singleton key/value config (which
// holds the CLI-seeded owner-floor under key 'owner_floor_discord_id').
var webAuthTables = []string{"web_user", "web_session", "guild_admins", "app_config"}

// characterCoinAndEvictionColumns are the six NEW character columns 00004 adds:
// the four nullable bank-coin columns (ADMIN-05 / D-11) + the eviction
// grace/archive columns (ADMIN-04 / D-10). All extend-only ALTER ADD COLUMN.
var characterCoinAndEvictionColumns = []string{
	"plat", "gold", "silver", "copper", "grace_until", "archived_at",
}

// auditLogGenericColumns are the three generic columns 00004 adds to the
// EXISTING audit_log (D-06: reuse/extend, do NOT invent a parallel log) so
// 15-03's officer/eviction/coin web-write events can be appended alongside the
// existing ingest cross_owner_reject rows.
var auditLogGenericColumns = []string{"actor", "detail", "at"}

// columnSet returns the set of column names for table via PRAGMA table_info.
func columnSet(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	cols := map[string]bool{}
	// #nosec G201 -- table is from a fixed in-test allow-list, not user input.
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s) failed: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scanning table_info(%s) row failed: %v", table, err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating table_info(%s) rows failed: %v", table, err)
	}
	return cols
}

// TestMigrate_00004_AddsWebAuthSchema proves the Phase 15 forward-only migration
// 00004 applied on a fresh DB (NewTestDB runs goose.Up over ALL four migrations):
// the four new tables exist, the six new character columns exist, the three new
// audit_log columns exist, and a second Up is a clean no-op.
func TestMigrate_00004_AddsWebAuthSchema(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00004) + t.Cleanup

	for _, tbl := range webAuthTables {
		if !tableExists(t, db, tbl) {
			t.Errorf("expected table %q to exist after 00004, but it does not", tbl)
		}
	}

	charCols := columnSet(t, db, "character")
	for _, c := range characterCoinAndEvictionColumns {
		if !charCols[c] {
			t.Errorf("expected character to have column %q after 00004 (have: %v)", c, charCols)
		}
	}

	auditCols := columnSet(t, db, "audit_log")
	for _, c := range auditLogGenericColumns {
		if !auditCols[c] {
			t.Errorf("expected audit_log to have generic column %q after 00004 (have: %v)", c, auditCols)
		}
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00004 DB
	// returns nil (goose records applied versions).
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00004 should be a no-op, got error: %v", err)
	}
}

// indexExists reports whether index name exists on table via PRAGMA index_list.
func indexExists(t *testing.T, db *sql.DB, table, name string) bool {
	t.Helper()
	// #nosec G201 -- table is from a fixed in-test literal, not user input.
	rows, err := db.Query(`PRAGMA index_list(` + table + `)`)
	if err != nil {
		t.Fatalf("PRAGMA index_list(%s) failed: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		// PRAGMA index_list columns: seq, name, unique, origin, partial.
		var (
			seq     int
			idxName string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &idxName, &unique, &origin, &partial); err != nil {
			t.Fatalf("scanning index_list(%s) row failed: %v", table, err)
		}
		if idxName == name {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating index_list(%s) rows failed: %v", table, err)
	}
	return false
}

// TestMigrate_00005_AddsSelfServiceLinking proves the Phase 17 forward-only
// migration 00005 applied on a fresh DB (NewTestDB runs goose.Up over ALL five
// migrations): owner gained the discord_user_id FK column, the partial unique
// index owner_discord_user_id_uidx exists, guild_code gained last_seen, and a
// second Up is a clean no-op. The migration must apply WITHOUT goose hitting the
// "Cannot add a UNIQUE column" error (the SQLite ADD-UNIQUE landmine — Pitfall 1):
// a successful NewTestDB here is itself that proof, since NewTestDB runs goose.Up.
func TestMigrate_00005_AddsSelfServiceLinking(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00005) + t.Cleanup

	ownerCols := columnSet(t, db, "owner")
	if !ownerCols["discord_user_id"] {
		t.Errorf("expected owner to have column %q after 00005 (have: %v)", "discord_user_id", ownerCols)
	}

	if !indexExists(t, db, "owner", "owner_discord_user_id_uidx") {
		t.Errorf("expected partial unique index %q on owner after 00005", "owner_discord_user_id_uidx")
	}

	codeCols := columnSet(t, db, "guild_code")
	if !codeCols["last_seen"] {
		t.Errorf("expected guild_code to have column %q after 00005 (have: %v)", "last_seen", codeCols)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00005 DB
	// returns nil (goose records applied versions).
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00005 should be a no-op, got error: %v", err)
	}
}
