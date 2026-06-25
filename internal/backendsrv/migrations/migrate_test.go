// Package migrations_test exercises the goose migration through the shared
// store.NewTestDB fixture. It is an EXTERNAL test package (migrations_test, not
// migrations) on purpose: store imports migrations, so an internal test here
// that imported store would form an import cycle. An external test package may
// depend on store, which depends on migrations — no cycle.
package migrations_test

import (
	"database/sql"
	"path/filepath"
	"strings"
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

// openAtVersion opens a fresh raw DB and applies goose Up only THROUGH version
// (via migrations.UpTo, the same embedded-FS helper RunMigrations uses), then
// registers a t.Cleanup close. Migration tests for tables that a LATER migration
// drops (e.g. wantlist_item, retired by 00014's D-01 clean break) pin to the
// version where the table still existed — store.NewTestDB always migrates to
// HEAD, where the table is gone. The historical assertions are unchanged; only
// the schema version they run against is pinned.
func openAtVersion(t *testing.T, version int64) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pinned.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open raw DB pinned at v%d: %v", version, err)
	}
	t.Cleanup(func() {
		if cerr := db.Close(); cerr != nil {
			t.Errorf("close raw DB pinned at v%d: %v", version, cerr)
		}
	})
	if err := migrations.UpTo(db, version); err != nil {
		t.Fatalf("UpTo(%d): %v", version, err)
	}
	return db
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

// wantlistItemColumns are the eight columns 00006 creates on wantlist_item
// (WANT-01/02): the FK identity, the nullable catalog item_id + snapshot
// item_name, the reason/priority enums, the optional note, the soft-delete
// active flag, and the epoch created_at.
var wantlistItemColumns = []string{
	"discord_user_id", "item_id", "item_name", "reason", "priority", "note", "active", "created_at",
}

// TestMigrate_00006_AddsWantlist proves the Phase 19 forward-only migration 00006
// applied on a fresh DB (NewTestDB runs goose.Up over ALL six migrations):
// wantlist_item + alert_log exist, wantlist_item has the eight expected columns,
// BOTH partial unique indexes (catalog + custom) exist, alert_log is created
// empty (Phase 19 writes zero alert rows), the reason/priority CHECK constraints
// reject a bad enum (review #5 — DB-level defense-in-depth), a valid-enum insert
// succeeds, and a second Up is a clean no-op.
func TestMigrate_00006_AddsWantlist(t *testing.T) {
	// Pinned at v6: 00014 (D-01 clean break) DROPs wantlist_item at HEAD, so this
	// historical test runs against a raw handle stopped at v6 where it still exists.
	db := openAtVersion(t, 6)

	for _, tbl := range []string{"wantlist_item", "alert_log"} {
		if !tableExists(t, db, tbl) {
			t.Errorf("expected table %q to exist after 00006, but it does not", tbl)
		}
	}

	wantCols := columnSet(t, db, "wantlist_item")
	for _, c := range wantlistItemColumns {
		if !wantCols[c] {
			t.Errorf("expected wantlist_item to have column %q after 00006 (have: %v)", c, wantCols)
		}
	}

	for _, idx := range []string{"wantlist_catalog_uidx", "wantlist_custom_uidx"} {
		if !indexExists(t, db, "wantlist_item", idx) {
			t.Errorf("expected partial unique index %q on wantlist_item after 00006", idx)
		}
	}

	// alert_log is created at full shape but Phase 19 writes ZERO rows.
	var alertN int
	if err := db.QueryRow(`SELECT COUNT(*) FROM alert_log`).Scan(&alertN); err != nil {
		t.Fatalf("counting alert_log rows failed: %v", err)
	}
	if alertN != 0 {
		t.Errorf("expected alert_log to be empty after 00006, got %d rows", alertN)
	}

	// Seed a web_user so the wantlist_item FK holds for the CHECK-constraint probes.
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-chk", "Chk"); err != nil {
		t.Fatalf("seed web_user: %v", err)
	}

	// The reason CHECK bites: a bad-enum reason ('maybe') must be rejected at the DB.
	if _, err := db.Exec(
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, created_at)
		 VALUES (?, NULL, ?, 'maybe', 'med', NULL, 0)`, "disc-chk", "BadReason"); err == nil {
		t.Errorf("expected reason='maybe' insert to fail the CHECK constraint, but it succeeded")
	}

	// The priority CHECK bites: a bad-enum priority ('urgent') must be rejected.
	if _, err := db.Exec(
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, created_at)
		 VALUES (?, NULL, ?, 'buy', 'urgent', NULL, 0)`, "disc-chk", "BadPriority"); err == nil {
		t.Errorf("expected priority='urgent' insert to fail the CHECK constraint, but it succeeded")
	}

	// A valid-enum insert succeeds (the CHECK only rejects bad enums).
	if _, err := db.Exec(
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, created_at)
		 VALUES (?, NULL, ?, 'buy', 'high', NULL, 0)`, "disc-chk", "GoodWant"); err != nil {
		t.Errorf("expected a valid-enum (reason='buy', priority='high') insert to succeed, got: %v", err)
	}

	// Forward-only/idempotent: a second UpTo(6) over an already-at-00006 DB returns
	// nil (goose records applied versions). NB: RunMigrations would advance to HEAD
	// and drop wantlist_item — this test stays pinned at v6.
	if err := migrations.UpTo(db, 6); err != nil {
		t.Fatalf("second UpTo(6) after 00006 should be a no-op, got error: %v", err)
	}
}

// notifyTables are the three NEW tables 00007 creates (Phase 20 plan 20-01,
// WANT-03/04/08): the per-user opt-in prefs (default-ON D-01), the officer-
// registered source channels (D-07/D-08), and the three guild-wide kill-switch
// flags (D-07, EC ships ON / WTS+raid ship dark).
var notifyTables = []string{"notify_prefs", "guild_channel", "monitor_flag"}

// TestMigrate_00007_AddsNotify proves the Phase 20 forward-only migration 00007
// applied on a fresh DB (NewTestDB runs goose.Up over ALL seven migrations):
//   - notify_prefs + guild_channel + monitor_flag exist;
//   - alert_log gained read_at and wantlist_item gained muted (columnSet);
//   - alert_log was rebuilt with a NULLABLE wantlist_item_id — a NULL-FK insert
//     succeeds (the D-10 test-alert identity, BLOCKER-1);
//   - a notify_prefs row inserted with only discord_user_id reads master/ec/wts/
//     raid all = 1 (DEFAULT 1, D-01);
//   - monitor_flag is seeded with exactly three rows: ec_auction=1, wts=0,
//     raid_target=0 (D-07 ships-dark);
//   - the guild_channel/monitor CHECK rejects a bogus monitor and accepts a valid
//     one; and a second Up is a clean no-op.
func TestMigrate_00007_AddsNotify(t *testing.T) {
	// Pinned at v7: 00014 (D-01 clean break) DROPs wantlist_item at HEAD, and this
	// test reads wantlist_item.muted — run against a raw handle stopped at v7.
	db := openAtVersion(t, 7)

	for _, tbl := range notifyTables {
		if !tableExists(t, db, tbl) {
			t.Errorf("expected table %q to exist after 00007, but it does not", tbl)
		}
	}

	alertCols := columnSet(t, db, "alert_log")
	if !alertCols["read_at"] {
		t.Errorf("expected alert_log to have column %q after 00007 (have: %v)", "read_at", alertCols)
	}
	wantCols := columnSet(t, db, "wantlist_item")
	if !wantCols["muted"] {
		t.Errorf("expected wantlist_item to have column %q after 00007 (have: %v)", "muted", wantCols)
	}

	// Seed a web_user so any discord_user_id-bearing inserts have an FK target.
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-np", "NotifyProbe"); err != nil {
		t.Fatalf("seed web_user: %v", err)
	}

	// BLOCKER-1: alert_log.wantlist_item_id is NULLABLE after the rebuild — a row
	// with wantlist_item_id=NULL inserts cleanly (the D-10 test-alert path, which
	// has no wantlist_item). Under 00006's NOT NULL this would have FK/NOT-NULL
	// failed; the rebuild is the fix.
	if _, err := db.Exec(
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, sent_at, send_status)
		 VALUES (NULL, ?, 'test', 0, 'sent')`, "disc-np"); err != nil {
		t.Errorf("expected a NULL wantlist_item_id alert_log insert (test-alert) to succeed, got: %v", err)
	}

	// notify_prefs DEFAULT 1 (D-01): a row inserted with only discord_user_id reads
	// master/ec/wts/raid all = 1 (absent-row default-ON semantics in the DDL).
	if _, err := db.Exec(`INSERT INTO notify_prefs (discord_user_id) VALUES (?)`, "disc-np"); err != nil {
		t.Fatalf("insert notify_prefs default row: %v", err)
	}
	var master, ec, wts, raid int
	if err := db.QueryRow(
		`SELECT master, ec, wts, raid FROM notify_prefs WHERE discord_user_id = ?`, "disc-np",
	).Scan(&master, &ec, &wts, &raid); err != nil {
		t.Fatalf("read notify_prefs defaults: %v", err)
	}
	if master != 1 || ec != 1 || wts != 1 || raid != 1 {
		t.Errorf("notify_prefs defaults = master=%d ec=%d wts=%d raid=%d, want all 1 (D-01)", master, ec, wts, raid)
	}

	// monitor_flag is seeded with exactly three rows: EC=1, wts=0, raid=0 (D-07).
	var flagCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM monitor_flag`).Scan(&flagCount); err != nil {
		t.Fatalf("counting monitor_flag rows failed: %v", err)
	}
	if flagCount != 3 {
		t.Errorf("expected exactly 3 seeded monitor_flag rows, got %d", flagCount)
	}
	wantFlags := map[string]int{"ec_auction": 1, "wts": 0, "raid_target": 0}
	for monitor, want := range wantFlags {
		var got int
		if err := db.QueryRow(`SELECT enabled FROM monitor_flag WHERE monitor = ?`, monitor).Scan(&got); err != nil {
			t.Fatalf("read monitor_flag %q: %v", monitor, err)
		}
		if got != want {
			t.Errorf("monitor_flag[%q].enabled = %d, want %d (D-07 ships-dark seed)", monitor, got, want)
		}
	}

	// The guild_channel monitor CHECK bites: a bogus monitor must be rejected.
	if _, err := db.Exec(
		`INSERT INTO guild_channel (channel_id, label, monitor, created_at)
		 VALUES ('111', 'Server A', 'bogus', 0)`); err == nil {
		t.Errorf("expected monitor='bogus' guild_channel insert to fail the CHECK constraint, but it succeeded")
	}
	// A valid monitor inserts fine.
	if _, err := db.Exec(
		`INSERT INTO guild_channel (channel_id, label, monitor, created_at)
		 VALUES ('111', 'Server A', 'ec_auction', 0)`); err != nil {
		t.Errorf("expected a valid monitor='ec_auction' guild_channel insert to succeed, got: %v", err)
	}

	// Forward-only/idempotent: a second UpTo(7) over an already-at-00007 DB returns
	// nil. NB: RunMigrations would advance to HEAD and drop wantlist_item — pinned at v7.
	if err := migrations.UpTo(db, 7); err != nil {
		t.Fatalf("second UpTo(7) after 00007 should be a no-op, got error: %v", err)
	}
}

// ecCursorColumns are the three columns 00008 creates on ec_auction_cursor
// (Phase 21 plan 21-01, WANT-05): the item_id PK join key, the RFC3339 last-seen
// auction timestamp, and the epoch updated_at.
var ecCursorColumns = []string{"item_id", "last_seen_t", "updated_at"}

// TestMigrate_00008_AddsECCursor proves the Phase 21 forward-only migration 00008
// applied on a fresh DB (NewTestDB runs goose.Up over ALL eight migrations):
// ec_auction_cursor exists with its three columns, an upsert round-trips on the
// item_id PK (insert-then-conflict-update keeps one row), and a second Up is a
// clean no-op (idempotent). Backend-only table — the watcher never touches it.
func TestMigrate_00008_AddsECCursor(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00008) + t.Cleanup

	if !tableExists(t, db, "ec_auction_cursor") {
		t.Errorf("expected table %q to exist after 00008, but it does not", "ec_auction_cursor")
	}

	cols := columnSet(t, db, "ec_auction_cursor")
	for _, c := range ecCursorColumns {
		if !cols[c] {
			t.Errorf("expected ec_auction_cursor to have column %q after 00008 (have: %v)", c, cols)
		}
	}

	// item_id is the PK: a second insert with the same item_id must conflict on the
	// PK (ON CONFLICT(item_id) is the upsert grain the store relies on) — proven
	// here by an INSERT OR REPLACE keeping exactly one row.
	if _, err := db.Exec(
		`INSERT INTO ec_auction_cursor (item_id, last_seen_t, updated_at) VALUES (?,?,?)`,
		int64(16247), "2026-06-06T01:00:00+00:00", int64(100)); err != nil {
		t.Fatalf("first ec_auction_cursor insert: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO ec_auction_cursor (item_id, last_seen_t, updated_at) VALUES (?,?,?)
		 ON CONFLICT(item_id) DO UPDATE SET last_seen_t=excluded.last_seen_t, updated_at=excluded.updated_at`,
		int64(16247), "2026-06-06T02:00:00+00:00", int64(200)); err != nil {
		t.Fatalf("ec_auction_cursor upsert: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ec_auction_cursor`).Scan(&n); err != nil {
		t.Fatalf("count ec_auction_cursor: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 ec_auction_cursor row after upsert on the PK, got %d", n)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00008 DB
	// returns nil (goose records applied versions).
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00008 should be a no-op, got error: %v", err)
	}
}

// TestMigrate_00009_CharacterAssignment proves the Phase 26 forward-only migration
// 00009 applied on a fresh DB (NewTestDB runs goose.Up over ALL nine migrations):
//   - character gained the is_guild_bot column (columnSet);
//   - character_assignment + assignment_request tables exist (tableExists);
//   - the partial unique index assignment_request_pending_uidx exists (indexExists);
//   - the assignment_request status CHECK rejects a bogus enum ('bogus');
//   - the partial-unique pending index collides on a second pending request for the
//     same (character_id, requester) but a resolved (denied) + a new pending do NOT;
//   - the auto-seed assigned a linked-owner non-bank non-removed char to that user
//     (assigned_by='migration'), and skipped a NULL-owner char, an is_bank_toon=1
//     char, and an is_removed=1 char; and
//   - a second Up is a clean no-op (idempotent — goose_db_version row count
//     unchanged, mirroring TestRunMigrations_Idempotent).
//
// Backend-only table set — the watcher never touches it.
func TestMigrate_00009_CharacterAssignment(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00009) + t.Cleanup

	// is_guild_bot column exists on character.
	charCols := columnSet(t, db, "character")
	if !charCols["is_guild_bot"] {
		t.Errorf("expected character to have column %q after 00009 (have: %v)", "is_guild_bot", charCols)
	}

	// Both new tables exist.
	for _, tbl := range []string{"character_assignment", "assignment_request"} {
		if !tableExists(t, db, tbl) {
			t.Errorf("expected table %q to exist after 00009, but it does not", tbl)
		}
	}

	// The partial-unique pending index exists.
	if !indexExists(t, db, "assignment_request", "assignment_request_pending_uidx") {
		t.Errorf("expected partial unique index %q on assignment_request after 00009", "assignment_request_pending_uidx")
	}

	// Seed a web_user so the assignment_request.requester FK holds for the probes.
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-req", "Requester"); err != nil {
		t.Fatalf("seed web_user: %v", err)
	}
	// Seed a character to hang the request probes on.
	probeOwner := mustInsertOwner(t, db, "ProbeOwner", nil)
	probeChar := mustInsertChar(t, db, probeOwner, "ProbeChar", false, false, false)

	// The status CHECK bites: a bogus status must be rejected at the DB.
	if _, err := db.Exec(
		`INSERT INTO assignment_request (character_id, requester, status, created_at)
		 VALUES (?, ?, 'bogus', 0)`, probeChar, "disc-req"); err == nil {
		t.Errorf("expected status='bogus' assignment_request insert to fail the CHECK constraint, but it succeeded")
	}

	// A first pending request inserts fine.
	if _, err := db.Exec(
		`INSERT INTO assignment_request (character_id, requester, status, created_at)
		 VALUES (?, ?, 'pending', 0)`, probeChar, "disc-req"); err != nil {
		t.Fatalf("first pending request insert: %v", err)
	}
	// A SECOND pending request for the same (char, requester) collides on the
	// partial-unique pending index.
	if _, err := db.Exec(
		`INSERT INTO assignment_request (character_id, requester, status, created_at)
		 VALUES (?, ?, 'pending', 0)`, probeChar, "disc-req"); err == nil {
		t.Errorf("expected a second pending request for the same (char, requester) to collide on the partial-unique index, but it succeeded")
	}
	// Resolving the first (denied) frees the partial index: a NEW pending no longer
	// collides (the index is scoped WHERE status='pending', so resolved rows drop out).
	if _, err := db.Exec(
		`UPDATE assignment_request SET status = 'denied', resolved_at = 1, resolved_by = ?
		   WHERE character_id = ? AND requester = ? AND status = 'pending'`,
		"disc-req", probeChar, "disc-req"); err != nil {
		t.Fatalf("deny the first pending request: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO assignment_request (character_id, requester, status, created_at)
		 VALUES (?, ?, 'pending', 0)`, probeChar, "disc-req"); err != nil {
		t.Errorf("expected a new pending request after the prior one was resolved (denied) NOT to collide, got: %v", err)
	}

	// Auto-seed inclusion/exclusion. The 00009 seed ran during NewTestDB over an
	// empty DB (no rows), so it backfilled nothing then. To exercise the SELECT
	// inclusion/exclusion logic deterministically, seed the four character classes
	// here and re-run the SAME seed statement (it is INSERT OR IGNORE — replay-safe).
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-seed", "SeedOwner"); err != nil {
		t.Fatalf("seed web_user for owner: %v", err)
	}
	seedDisc := "disc-seed"
	linkedOwner := mustInsertOwner(t, db, "LinkedOwner", &seedDisc) // owner.discord_user_id non-NULL
	nullOwner := mustInsertOwner(t, db, "NullOwner", nil)           // owner.discord_user_id NULL

	linkedChar := mustInsertChar(t, db, linkedOwner, "LinkedChar", false, false, false) // → assigned
	nullChar := mustInsertChar(t, db, nullOwner, "NullChar", false, false, false)       // → excluded (NULL owner)
	bankChar := mustInsertChar(t, db, linkedOwner, "BankChar", true, false, false)      // → excluded (is_bank_toon=1)
	removedChar := mustInsertChar(t, db, linkedOwner, "RemovedChar", false, false, true) // → excluded (is_removed=1)

	// Re-run the auto-seed SELECT (the exact 00009 statement; INSERT OR IGNORE so a
	// replay is safe). This is the same SQL the migration ran on boot.
	if _, err := db.Exec(
		`INSERT OR IGNORE INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 SELECT c.id, o.discord_user_id, strftime('%s','now'), 'migration'
		   FROM character c
		   JOIN owner o ON o.id = c.owner_id
		  WHERE o.discord_user_id IS NOT NULL
		    AND c.is_removed = 0
		    AND c.is_bank_toon = 0
		    AND c.is_guild_bot = 0`); err != nil {
		t.Fatalf("re-run auto-seed: %v", err)
	}

	assignedTo := func(charID int64) (string, string, bool) {
		t.Helper()
		var who, by string
		err := db.QueryRow(
			`SELECT discord_user_id, assigned_by FROM character_assignment WHERE character_id = ?`, charID,
		).Scan(&who, &by)
		if err == sql.ErrNoRows {
			return "", "", false
		}
		if err != nil {
			t.Fatalf("read assignment (character_id=%d): %v", charID, err)
		}
		return who, by, true
	}

	// The linked-owner char is assigned to that user with assigned_by='migration'.
	if who, by, ok := assignedTo(linkedChar); !ok || who != "disc-seed" || by != "migration" {
		t.Errorf("linkedChar assignment = (%q, %q, %v), want (disc-seed, migration, true)", who, by, ok)
	}
	// The NULL-owner / bank / removed chars are NOT assigned.
	if _, _, ok := assignedTo(nullChar); ok {
		t.Errorf("nullChar (NULL owner) should NOT have an assignment, but it does")
	}
	if _, _, ok := assignedTo(bankChar); ok {
		t.Errorf("bankChar (is_bank_toon=1) should NOT have an assignment, but it does")
	}
	if _, _, ok := assignedTo(removedChar); ok {
		t.Errorf("removedChar (is_removed=1) should NOT have an assignment, but it does")
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00009 DB
	// returns nil AND the goose_db_version row count is unchanged.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00009 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// TestMigrate_00010_CharacterTaggedWantlist proves the Phase 28 forward-only
// migration 00010 applied on a fresh DB (NewTestDB runs goose.Up over ALL ten
// migrations):
//   - wantlist_item gained the nullable character_id column (columnSet);
//   - the COALESCE(character_id,-1)-keyed dedup rewrite PRESERVES the 00006
//     account-level (NULL character_id) dedup: a second account-level want for the
//     same (user, item_id) collides on wantlist_catalog_uidx;
//   - the SAME (user, item_id) tagged to two DIFFERENT character_id values
//     does NOT collide (the COALESCE sentinel keeps real char ids distinct ⇒ the
//     same item can be wanted for two characters);
//   - the SAME assertions hold on the custom path (item_id NULL, dedup on item_name
//     via wantlist_custom_uidx);
//
// NB (quick-260610-fm5): 00011 later dropped reason from BOTH unique keys but kept
// the COALESCE(character_id,-1) term, so every assertion below (all single-reason
// pairs) holds UNCHANGED at HEAD — this test regression-guards the per-char dedup.
//   - existing rows backfill to NULL character_id (the ADD COLUMN default — no data
//     loss, CWANT-02); and
//   - a second Up is a clean no-op (idempotent — goose_db_version row count
//     unchanged).
//
// Backend-only: the watcher never reads/writes wantlist_item, so there is NO
// WatcherMaxSchemaVersion change. "Schema v10" == goose 00010 applied.
func TestMigrate_00010_CharacterTaggedWantlist(t *testing.T) {
	// Pinned at v10: 00014 (D-01 clean break) DROPs wantlist_item at HEAD — this
	// historical test seeds + reads wantlist_item, so run against a raw handle at v10.
	db := openAtVersion(t, 10)

	// character_id column exists on wantlist_item.
	wlCols := columnSet(t, db, "wantlist_item")
	if !wlCols["character_id"] {
		t.Errorf("expected wantlist_item to have column %q after 00010 (have: %v)", "character_id", wlCols)
	}

	// Seed a web_user (the wantlist_item.discord_user_id FK) and two characters so the
	// character_id FK holds for the tagged inserts.
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-cw", "CWUser"); err != nil {
		t.Fatalf("seed web_user: %v", err)
	}
	cwOwner := mustInsertOwner(t, db, "CWOwner", nil)
	charA := mustInsertChar(t, db, cwOwner, "CharA", false, false, false)
	charB := mustInsertChar(t, db, cwOwner, "CharB", false, false, false)

	// addWant is a small raw INSERT helper returning the error (so a collision is
	// observable). characterID is *int64 so a NULL (account-level) want is expressible.
	addWant := func(itemID *int64, itemName, reason string, characterID *int64) error {
		var itemArg, charArg any
		if itemID != nil {
			itemArg = *itemID
		}
		if characterID != nil {
			charArg = *characterID
		}
		_, err := db.Exec(
			`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, character_id, created_at)
			 VALUES (?, ?, ?, ?, 'med', ?, 0)`,
			"disc-cw", itemArg, itemName, reason, charArg)
		return err
	}

	// --- Catalog path (item_id NOT NULL, dedup on item_id) ---
	catItem := int64(5001)

	// 1) An existing-row backfill check: a first account-level (character_id NULL) want.
	if err := addWant(&catItem, "Catalog Item", "buy", nil); err != nil {
		t.Fatalf("first account-level catalog want: %v", err)
	}
	// Its character_id reads NULL (the ADD COLUMN backfill — CWANT-02).
	var charIDNull sql.NullInt64
	if err := db.QueryRow(
		`SELECT character_id FROM wantlist_item WHERE discord_user_id = ? AND item_id = ? AND reason = 'buy' AND active = 1`,
		"disc-cw", catItem,
	).Scan(&charIDNull); err != nil {
		t.Fatalf("read backfilled character_id: %v", err)
	}
	if charIDNull.Valid {
		t.Errorf("first account-level want character_id = %d, want NULL (backfill)", charIDNull.Int64)
	}

	// 2) A SECOND account-level (NULL) want for the same (user,item_id) COLLIDES
	//    (the COALESCE sentinel preserves 00006 account-level dedup — T-28-04).
	if err := addWant(&catItem, "Catalog Item", "buy", nil); err == nil {
		t.Errorf("expected a second account-level (NULL) catalog want for the same (user,item) to collide on wantlist_catalog_uidx, but it succeeded")
	}

	// 3) The SAME (user,item_id) tagged to charA, then charB — BOTH succeed
	//    (same item wanted for two characters ⇒ two rows; real char ids stay distinct).
	if err := addWant(&catItem, "Catalog Item", "buy", &charA); err != nil {
		t.Errorf("expected the catalog want tagged to charA to succeed (distinct from the NULL want), got: %v", err)
	}
	if err := addWant(&catItem, "Catalog Item", "buy", &charB); err != nil {
		t.Errorf("expected the catalog want tagged to charB to succeed (distinct from charA), got: %v", err)
	}
	// 4) A SECOND want for the same item tagged to charA AGAIN collides (per-char dedup).
	if err := addWant(&catItem, "Catalog Item", "buy", &charA); err == nil {
		t.Errorf("expected a second catalog want for the same (user,item,charA) to collide, but it succeeded")
	}

	// --- Custom path (item_id NULL, dedup on item_name) ---
	// 5) A first account-level (NULL) custom want.
	if err := addWant(nil, "Custom Label", "quest", nil); err != nil {
		t.Fatalf("first account-level custom want: %v", err)
	}
	// 6) A SECOND account-level (NULL) custom want for the same (user,item_name)
	//    COLLIDES on wantlist_custom_uidx.
	if err := addWant(nil, "Custom Label", "quest", nil); err == nil {
		t.Errorf("expected a second account-level (NULL) custom want for the same (user,label) to collide on wantlist_custom_uidx, but it succeeded")
	}
	// 7) The SAME (user,item_name) tagged to two DIFFERENT chars — both succeed.
	if err := addWant(nil, "Custom Label", "quest", &charA); err != nil {
		t.Errorf("expected the custom want tagged to charA to succeed (distinct from the NULL want), got: %v", err)
	}
	if err := addWant(nil, "Custom Label", "quest", &charB); err != nil {
		t.Errorf("expected the custom want tagged to charB to succeed (distinct from charA), got: %v", err)
	}

	// Forward-only/idempotent: a second UpTo(10) over an already-at-00010 DB returns
	// nil AND the goose_db_version row count is unchanged. NB: RunMigrations would
	// advance to HEAD and drop wantlist_item — pinned at v10.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.UpTo(db, 10); err != nil {
		t.Fatalf("second UpTo(10) after 00010 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// TestMigrate_00011_WantlistDropReasonDedup proves the quick-260610-fm5 forward-only
// migration 00011 (drop the buy/quest reason from the wantlist dedup key):
//
//	(a) NEITHER recreated unique index keys on reason any more — their
//	    sqlite_master SQL contains no 'reason' but KEEPS the load-bearing
//	    COALESCE(character_id, -1) term from 00010 (T-fm5-01);
//	(b) at HEAD a 'buy' + 'quest' insert pair for the same (user, item, NULL char)
//	    collides on the SECOND insert (reason left the key; the reason COLUMN
//	    persists — its NOT NULL CHECK cannot be altered away in SQLite — but it no
//	    longer differentiates rows), on the catalog AND custom paths; and
//	(c) the DATA pass: migrating a v10 database carrying cross-reason duplicates up
//	    to v11 soft-deletes (active=0 — NEVER DELETE; alert_log FKs these rows)
//	    every colliding row EXCEPT MIN(id), per (user, item|label,
//	    COALESCE(character_id,-1)) — and a char-tagged row in a COALESCE scope of
//	    its own SURVIVES the pass (the COALESCE pin in the dedupe GROUP BYs).
//
// (c) cannot run through store.NewTestDB (it always migrates to HEAD), so it opens
// a raw store.Open handle and drives migrations.UpTo (the test-support helper over
// the SAME embedded FS RunMigrations uses) to v10, seeds, then resumes to v11.
func TestMigrate_00011_WantlistDropReasonDedup(t *testing.T) {
	// Pinned at v11: 00014 (D-01 clean break) DROPs wantlist_item + its unique
	// indexes at HEAD — parts (a)/(b) read those indexes + seed wantlist_item, so
	// run against a raw handle stopped at v11. Part (c) already opens its own v10→v11
	// handle below.
	db := openAtVersion(t, 11)

	// (a) Index-SQL assert: no 'reason' in either recreated unique index; the
	// COALESCE(character_id, -1) term is retained.
	rows, err := db.Query(
		`SELECT name, sql FROM sqlite_master WHERE name IN ('wantlist_catalog_uidx','wantlist_custom_uidx')`)
	if err != nil {
		t.Fatalf("read index SQL from sqlite_master: %v", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var name, sqlText string
		if err := rows.Scan(&name, &sqlText); err != nil {
			t.Fatalf("scan sqlite_master row: %v", err)
		}
		seen++
		if strings.Contains(sqlText, "reason") {
			t.Errorf("index %q still keys on reason after 00011: %s", name, sqlText)
		}
		if !strings.Contains(sqlText, "COALESCE(character_id, -1)") {
			t.Errorf("index %q lost the COALESCE(character_id, -1) term after 00011 (the 00010 per-char dedup pin): %s", name, sqlText)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master rows: %v", err)
	}
	if seen != 2 {
		t.Fatalf("found %d of the 2 wantlist unique indexes in sqlite_master, want 2", seen)
	}

	// (b) Cross-reason collision at HEAD. Seed a web_user for the FK, then insert
	// the same (user, item, NULL char) as 'buy' and again as 'quest' — the second
	// MUST collide now that reason left the key.
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-r11", "ReasonProbe"); err != nil {
		t.Fatalf("seed web_user: %v", err)
	}
	insWant := func(itemID any, itemName, reason string) error {
		_, err := db.Exec(
			`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, character_id, created_at)
			 VALUES (?, ?, ?, ?, 'med', NULL, 0)`, "disc-r11", itemID, itemName, reason)
		return err
	}
	if err := insWant(int64(6001), "Cross Reason Item", "buy"); err != nil {
		t.Fatalf("first catalog ('buy') insert: %v", err)
	}
	if err := insWant(int64(6001), "Cross Reason Item", "quest"); err == nil {
		t.Errorf("expected the 'quest' re-add of the same (user,item,NULL char) to collide after 00011, but it succeeded")
	}
	// Same on the custom path (item_id NULL, keyed on item_name).
	if err := insWant(nil, "Cross Reason Label", "buy"); err != nil {
		t.Fatalf("first custom ('buy') insert: %v", err)
	}
	if err := insWant(nil, "Cross Reason Label", "quest"); err == nil {
		t.Errorf("expected the 'quest' re-add of the same (user,label,NULL char) custom want to collide after 00011, but it succeeded")
	}

	// (c) The dedupe-data pass, on a SEPARATE raw handle stopped at v10.
	rawPath := filepath.Join(t.TempDir(), "dedupe-proof.db")
	raw, err := store.Open(rawPath)
	if err != nil {
		t.Fatalf("open raw v10 DB: %v", err)
	}
	t.Cleanup(func() {
		if cerr := raw.Close(); cerr != nil {
			t.Errorf("close raw v10 DB: %v", cerr)
		}
	})
	if err := migrations.UpTo(raw, 10); err != nil {
		t.Fatalf("UpTo(10): %v", err)
	}

	// Seed cross-reason duplicate pairs that were LEGAL at v10 (reason was in the
	// key), plus a char-tagged row in its own COALESCE scope.
	if _, err := raw.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-dd", "DedupeUser"); err != nil {
		t.Fatalf("seed web_user (v10): %v", err)
	}
	ddOwner := mustInsertOwner(t, raw, "DedupeOwner", nil)
	ddChar := mustInsertChar(t, raw, ddOwner, "DedupeChar", false, false, false)

	seedV10 := func(itemID any, itemName, reason string, charID any) int64 {
		t.Helper()
		res, err := raw.Exec(
			`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, character_id, created_at)
			 VALUES (?, ?, ?, ?, 'med', ?, 0)`, "disc-dd", itemID, itemName, reason, charID)
		if err != nil {
			t.Fatalf("seed v10 want (%s, %s): %v", itemName, reason, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("seed v10 want id: %v", err)
		}
		return id
	}
	catKeep := seedV10(int64(7001), "Dup Item", "buy", nil)   // MIN(id), catalog NULL-char group → stays active
	catDrop := seedV10(int64(7001), "Dup Item", "quest", nil) // newer cross-reason dup → deactivated
	cusKeep := seedV10(nil, "Dup Label", "buy", nil)          // MIN(id), custom NULL-char group → stays active
	cusDrop := seedV10(nil, "Dup Label", "quest", nil)        // newer cross-reason dup → deactivated
	// The SAME item tagged to a character is its OWN COALESCE scope — it must
	// SURVIVE the pass (proves the dedupe GROUP BYs kept COALESCE(character_id,-1)).
	charKeep := seedV10(int64(7001), "Dup Item", "quest", ddChar)

	if err := migrations.UpTo(raw, 11); err != nil {
		t.Fatalf("UpTo(11) over the seeded v10 DB: %v", err)
	}

	activeOf := func(id int64) int {
		t.Helper()
		var a int
		if err := raw.QueryRow(`SELECT active FROM wantlist_item WHERE id = ?`, id).Scan(&a); err != nil {
			t.Fatalf("read active (id=%d): %v", id, err)
		}
		return a
	}
	if got := activeOf(catKeep); got != 1 {
		t.Errorf("catalog MIN(id) row active = %d, want 1 (the keeper)", got)
	}
	if got := activeOf(catDrop); got != 0 {
		t.Errorf("catalog cross-reason dup active = %d, want 0 (soft-deleted by the 00011 data pass)", got)
	}
	if got := activeOf(cusKeep); got != 1 {
		t.Errorf("custom MIN(id) row active = %d, want 1 (the keeper)", got)
	}
	if got := activeOf(cusDrop); got != 0 {
		t.Errorf("custom cross-reason dup active = %d, want 0 (soft-deleted by the 00011 data pass)", got)
	}
	if got := activeOf(charKeep); got != 1 {
		t.Errorf("char-tagged row active = %d, want 1 — the dedupe GROUP BYs must keep COALESCE(character_id,-1) (T-fm5-01)", got)
	}
	// Soft-delete ONLY: every seeded row still exists (alert_log FKs them).
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM wantlist_item`).Scan(&n); err != nil {
		t.Fatalf("count wantlist_item rows: %v", err)
	}
	if n != 5 {
		t.Errorf("wantlist_item row count = %d, want 5 (the data pass must soft-delete, never DELETE)", n)
	}
}

// TestMigrate_00012_AddsItemIcon proves the Phase 31 forward-only migration 00012
// applied on a fresh DB (NewTestDB runs goose.Up over ALL twelve migrations):
//   - item_master gained the nullable icon_id column (columnSet);
//   - a row inserted WITHOUT icon_id reads NULL (the extend-only ADD COLUMN default,
//     no DEFAULT/UNIQUE — coverage ships incrementally, D-02/D-03);
//   - a row inserted WITH an icon_id round-trips that integer; and
//   - a second Up is a clean no-op (idempotent — goose_db_version row count
//     unchanged).
//
// Backend-only additive column — the watcher never reads/writes item_master, so
// there is NO WatcherMaxSchemaVersion change. "Schema v12" == goose 00012 applied.
func TestMigrate_00012_AddsItemIcon(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00012) + t.Cleanup

	// icon_id column exists on item_master.
	cols := columnSet(t, db, "item_master")
	if !cols["icon_id"] {
		t.Errorf("expected item_master to have column %q after 00012 (have: %v)", "icon_id", cols)
	}

	// A row inserted WITHOUT icon_id reads NULL (extend-only ADD COLUMN, no DEFAULT).
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		int64(1000), "No Icon Item", "", "", "", 0, "sha-noicon"); err != nil {
		t.Fatalf("insert item_master without icon_id: %v", err)
	}
	var iconNull sql.NullInt64
	if err := db.QueryRow(`SELECT icon_id FROM item_master WHERE item_id = ?`, int64(1000)).Scan(&iconNull); err != nil {
		t.Fatalf("read icon_id (no-icon row): %v", err)
	}
	if iconNull.Valid {
		t.Errorf("item_master.icon_id for a row inserted without it = %d, want NULL (extend-only default)", iconNull.Int64)
	}

	// A row inserted WITH an icon_id round-trips the integer.
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed, icon_id)
		 VALUES (?,?,?,?,?,?,?,datetime('now'),?)`,
		int64(1001), "Cloak of Flames", "", "", "BACK", 0, "sha-cof", int64(658)); err != nil {
		t.Fatalf("insert item_master with icon_id: %v", err)
	}
	var icon int64
	if err := db.QueryRow(`SELECT icon_id FROM item_master WHERE item_id = ?`, int64(1001)).Scan(&icon); err != nil {
		t.Fatalf("read icon_id (icon row): %v", err)
	}
	if icon != 658 {
		t.Errorf("item_master.icon_id round-trip = %d, want 658 (Cloak of Flames lucy_img_ID)", icon)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00012 DB
	// returns nil AND the goose_db_version row count is unchanged.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00012 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// itemFlagsEffectsColumns are the nine discrete flag/effect columns 00016 adds to
// item_master (ENRICH-12 + ENRICH-13): the four queried flags, the clicky boolean +
// name, the haste boolean + %, and the full-flag-set JSON array. A column-name typo
// (e.g. is_nodrop vs is_no_drop) is caught here in CI, not at runtime in the upsert.
var itemFlagsEffectsColumns = []string{
	"is_lore", "is_no_drop", "is_magic", "is_temporary",
	"is_clicky", "clicky_effect", "has_haste", "haste_pct", "flags_json",
}

// TestMigrate_00016_AddsItemFlagsEffects proves the Phase 37 forward-only migration
// 00016 applied on a fresh DB (NewTestDB runs goose.Up over ALL sixteen migrations):
//   - item_master gained ALL NINE discrete flag/effect columns (columnSet) — the
//     assertion that catches a mistyped column name before it reaches the store upsert;
//   - a row inserted WITHOUT the new columns reads NULL flags_json (the extend-only
//     ADD COLUMN default — that NULL is the boot backfill's idempotency key, D-05);
//   - a row inserted WITH the new columns round-trips them (is_magic=1, haste_pct=36,
//     flags_json the stored array); and
//   - a second Up is a clean no-op (idempotent — goose_db_version row count unchanged).
//
// Backend-only additive columns — the watcher never reads/writes item_master, so there
// is NO WatcherMaxSchemaVersion change (that gate does not exist in the off-Google
// backend). "Schema v16" == goose 00016 applied.
func TestMigrate_00016_AddsItemFlagsEffects(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00016) + t.Cleanup

	// All nine new columns exist on item_master.
	cols := columnSet(t, db, "item_master")
	for _, c := range itemFlagsEffectsColumns {
		if !cols[c] {
			t.Errorf("expected item_master to have column %q after 00016 (have: %v)", c, cols)
		}
	}

	// A row inserted WITHOUT the new columns reads NULL flags_json (extend-only ADD
	// COLUMN, no DEFAULT) — the not-yet-backfilled marker the boot backfill keys on.
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed)
		 VALUES (?,?,?,?,?,?,?,datetime('now'))`,
		int64(2000), "Pre-00016 Item", "", "", "", 0, "sha-pre16"); err != nil {
		t.Fatalf("insert item_master without the new columns: %v", err)
	}
	var flagsNull sql.NullString
	if err := db.QueryRow(`SELECT flags_json FROM item_master WHERE item_id = ?`, int64(2000)).Scan(&flagsNull); err != nil {
		t.Fatalf("read flags_json (pre-00016 row): %v", err)
	}
	if flagsNull.Valid {
		t.Errorf("item_master.flags_json for a row inserted without it = %q, want NULL (extend-only default / backfill key)", flagsNull.String)
	}

	// A row inserted WITH the new columns round-trips them.
	if _, err := db.Exec(
		`INSERT INTO item_master (item_id, name, wiki_summary, wiki_url, slot, is_quest_item, wikitext_sha1, last_refreshed,
		    is_lore, is_no_drop, is_magic, is_temporary, is_clicky, clicky_effect, has_haste, haste_pct, flags_json)
		 VALUES (?,?,?,?,?,?,?,datetime('now'),?,?,?,?,?,?,?,?,?)`,
		int64(2001), "Cloak of Flames", "", "", "BACK", 0, "sha-cof16",
		0, 0, 1, 0, 0, "", 1, 36, `["MAGIC ITEM"]`); err != nil {
		t.Fatalf("insert item_master with the new columns: %v", err)
	}
	var isMagic, hasHaste, hastePct int
	var flagsJSON string
	if err := db.QueryRow(
		`SELECT is_magic, has_haste, haste_pct, flags_json FROM item_master WHERE item_id = ?`, int64(2001),
	).Scan(&isMagic, &hasHaste, &hastePct, &flagsJSON); err != nil {
		t.Fatalf("read flag/effect columns (flagged row): %v", err)
	}
	if isMagic != 1 || hasHaste != 1 || hastePct != 36 || flagsJSON != `["MAGIC ITEM"]` {
		t.Errorf("flag/effect round-trip = is_magic=%d has_haste=%d haste_pct=%d flags_json=%q, want 1/1/36/[\"MAGIC ITEM\"]",
			isMagic, hasHaste, hastePct, flagsJSON)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00016 DB
	// returns nil AND the goose_db_version row count is unchanged.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00016 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// catalogEnrichmentColumns are the 20 columns 00017 creates on the NEW
// catalog_enrichment table (Phase 38 ENRICH-14/15, D-04 name-keyed): the
// norm_name PK (the cross-namespace key), the representative name + PigParse
// item_id, and the full item_master enrichment column set re-keyed on the name.
// A column-name typo (e.g. is_nodrop vs is_no_drop) is caught here in CI, not at
// runtime in the catalog upsert.
var catalogEnrichmentColumns = []string{
	"norm_name", "name", "item_id", "wiki_summary", "wiki_url", "slot", "is_quest_item",
	"wikitext_sha1", "icon_id", "statsblock", "is_lore", "is_no_drop", "is_magic",
	"is_temporary", "is_clicky", "clicky_effect", "has_haste", "haste_pct",
	"flags_json", "last_refreshed",
}

// TestMigrate_00017_AddsCatalogEnrichment proves the Phase 38 forward-only migration
// 00017 applied on a fresh DB (NewTestDB runs goose.Up over ALL seventeen migrations):
//   - the NEW catalog_enrichment table exists (additive — no ALTER of item_master);
//   - it carries ALL 20 columns (columnSet) — the assertion that catches a mistyped
//     column name before it reaches the catalog upsert;
//   - it is created EMPTY (the crawl populates it; nothing is seeded by the migration);
//   - norm_name is the PK: a second insert with the SAME norm_name conflicts on the PK
//     (ON CONFLICT(norm_name) is the upsert grain the store relies on), keeping one row;
//   - a second Up is a clean no-op (idempotent — goose_db_version row count unchanged).
//
// Backend-only additive table — the watcher never reads/writes catalog_enrichment, so there
// is NO WatcherMaxSchemaVersion change (that gate does not exist in the off-Google backend).
// "Schema v17" == goose 00017 applied.
func TestMigrate_00017_AddsCatalogEnrichment(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00017) + t.Cleanup

	// The new table exists.
	if !tableExists(t, db, "catalog_enrichment") {
		t.Errorf("expected table %q to exist after 00017, but it does not", "catalog_enrichment")
	}

	// All 20 columns exist on catalog_enrichment.
	cols := columnSet(t, db, "catalog_enrichment")
	for _, c := range catalogEnrichmentColumns {
		if !cols[c] {
			t.Errorf("expected catalog_enrichment to have column %q after 00017 (have: %v)", c, cols)
		}
	}

	// Created empty (the weekly crawl populates it — the migration seeds nothing).
	var initial int
	if err := db.QueryRow(`SELECT count(*) FROM catalog_enrichment`).Scan(&initial); err != nil {
		t.Fatalf("count catalog_enrichment (post-migration): %v", err)
	}
	if initial != 0 {
		t.Errorf("expected catalog_enrichment to be created empty, got %d rows", initial)
	}

	// norm_name is the PK: a second insert with the same norm_name must conflict on the
	// PK (ON CONFLICT(norm_name) is the upsert grain the store relies on) — proven here
	// by an INSERT ... ON CONFLICT(norm_name) DO UPDATE keeping exactly one row.
	if _, err := db.Exec(
		`INSERT INTO catalog_enrichment (norm_name, name, item_id, icon_id, flags_json) VALUES (?,?,?,?,?)`,
		"cloak of flames", "Cloak of Flames", int64(1234), int64(567), "[]"); err != nil {
		t.Fatalf("first catalog_enrichment insert: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO catalog_enrichment (norm_name, name, item_id, icon_id, flags_json) VALUES (?,?,?,?,?)
		 ON CONFLICT(norm_name) DO UPDATE SET icon_id=excluded.icon_id`,
		"cloak of flames", "Cloak of Flames", int64(1234), int64(890), "[]"); err != nil {
		t.Fatalf("catalog_enrichment upsert: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM catalog_enrichment`).Scan(&n); err != nil {
		t.Fatalf("count catalog_enrichment after upsert: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 catalog_enrichment row after upsert on the PK, got %d", n)
	}
	// The conflict UPDATE took effect (icon_id 567 → 890).
	var gotIcon int64
	if err := db.QueryRow(`SELECT icon_id FROM catalog_enrichment WHERE norm_name = ?`, "cloak of flames").Scan(&gotIcon); err != nil {
		t.Fatalf("read catalog_enrichment icon_id after upsert: %v", err)
	}
	if gotIcon != 890 {
		t.Errorf("catalog_enrichment icon_id = %d after ON CONFLICT(norm_name) update, want 890", gotIcon)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00017 DB
	// returns nil AND the goose_db_version row count is unchanged.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00017 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// wishlistItemColumns are the nine columns 00014 creates on wishlist_item
// (WISH-02/03): the FK identity (discord_user_id, the PERSON), the NOT-NULL
// character_id + canonical worn-slot, the nullable catalog item_id + snapshot
// item_name, the default-ON pinged toggle (Pitfall 8), the soft-delete active
// flag, and the epoch created_at.
var wishlistItemColumns = []string{
	"id", "discord_user_id", "character_id", "slot", "item_id", "item_name", "pinged", "active", "created_at",
}

// TestMigrate_00014_AddsWishlist proves the Phase 34 forward-only migration 00014
// applied on a fresh DB (NewTestDB runs goose.Up over ALL fourteen migrations) —
// the D-01 clean break:
//   - wishlist_item exists with its nine expected columns (columnSet);
//   - wantlist_item is GONE (the clean break dropped the retired item-centric
//     table — tableExists returns false);
//   - alert_log was REBUILT (its FK now targets wishlist_item(id), but the column
//     name is KEPT wantlist_item_id per Pitfall 6 option B so store/alertlog.go
//     needs no edit): a NULL-FK alert_log insert still succeeds (the test-alert
//     path — proves the rebuild kept the column nullable + named wantlist_item_id);
//   - a real wishlist_item row + an alert_log row that FKs it both insert (proves
//     the rebuilt FK targets wishlist_item, not the dropped wantlist_item); and
//   - a second Up is a clean no-op (idempotent — goose_db_version row count
//     unchanged).
//
// Backend-only: the watcher never touches wishlist_item/wantlist_item, so there is
// NO WatcherMaxSchemaVersion change (that gate does not exist in the off-Google
// backend). "Schema v14" == goose 00014 applied.
func TestMigrate_00014_AddsWishlist(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00014) + t.Cleanup

	// wishlist_item exists with its nine columns.
	if !tableExists(t, db, "wishlist_item") {
		t.Errorf("expected table %q to exist after 00014, but it does not", "wishlist_item")
	}
	wlCols := columnSet(t, db, "wishlist_item")
	for _, c := range wishlistItemColumns {
		if !wlCols[c] {
			t.Errorf("expected wishlist_item to have column %q after 00014 (have: %v)", c, wlCols)
		}
	}

	// D-01 clean break: the retired wantlist_item is GONE.
	if tableExists(t, db, "wantlist_item") {
		t.Errorf("expected wantlist_item to be DROPPED after 00014 (D-01 clean break), but it still exists")
	}

	// Seed a web_user + an owner + a character so the FKs hold for the probes.
	if _, err := db.Exec(
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, "disc-wl", "WishlistProbe"); err != nil {
		t.Fatalf("seed web_user: %v", err)
	}
	wlOwner := mustInsertOwner(t, db, "WishlistOwner", nil)
	wlChar := mustInsertChar(t, db, wlOwner, "WishlistChar", false, false, false)

	// alert_log was rebuilt: the FK now targets wishlist_item(id) but the column
	// name is KEPT wantlist_item_id (Pitfall 6 option B). A NULL-FK insert still
	// succeeds (the test-alert path — proves the rebuild kept the column nullable
	// + named wantlist_item_id, so store/alertlog.go needs no edit).
	if _, err := db.Exec(
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, sent_at, send_status)
		 VALUES (NULL, ?, 'test', 0, 'sent')`, "disc-wl"); err != nil {
		t.Errorf("expected a NULL wantlist_item_id alert_log insert (test-alert) to succeed after the 00014 rebuild, got: %v", err)
	}

	// A real wishlist_item row inserts (pinged + active default-ON via DDL).
	res, err := db.Exec(
		`INSERT INTO wishlist_item (discord_user_id, character_id, slot, item_id, item_name, created_at)
		 VALUES (?, ?, 'Primary', NULL, 'Some Sword', 0)`, "disc-wl", wlChar)
	if err != nil {
		t.Fatalf("insert wishlist_item row: %v", err)
	}
	wishID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("wishlist_item last insert id: %v", err)
	}
	// Its pinged + active default to 1 (the DDL defaults — Pitfall 8 default-ON).
	var pinged, active int
	if err := db.QueryRow(
		`SELECT pinged, active FROM wishlist_item WHERE id = ?`, wishID,
	).Scan(&pinged, &active); err != nil {
		t.Fatalf("read wishlist_item defaults: %v", err)
	}
	if pinged != 1 || active != 1 {
		t.Errorf("wishlist_item defaults = pinged=%d active=%d, want both 1 (default-ON, D-04)", pinged, active)
	}

	// An alert_log row that FKs the real wishlist_item id inserts cleanly (proves
	// the rebuilt FK targets wishlist_item(id), not the dropped wantlist_item).
	if _, err := db.Exec(
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, sent_at, send_status)
		 VALUES (?, ?, 'ec_auction', 0, 'sent')`, wishID, "disc-wl"); err != nil {
		t.Errorf("expected an alert_log insert FK'ing a real wishlist_item id to succeed, got: %v", err)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00014 DB
	// returns nil AND the goose_db_version row count is unchanged.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00014 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// mustInsertOwner inserts an owner with the given label and (nullable)
// discord_user_id, returning its id. discordUserID is *string so a NULL-owner
// (legacy/unlinked) is distinguishable from a linked one.
func mustInsertOwner(t *testing.T, db *sql.DB, label string, discordUserID *string) int64 {
	t.Helper()
	var arg any
	if discordUserID != nil {
		arg = *discordUserID
	}
	res, err := db.Exec(`INSERT INTO owner (label, discord_user_id) VALUES (?, ?)`, label, arg)
	if err != nil {
		t.Fatalf("insert owner %q: %v", label, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("owner last insert id: %v", err)
	}
	return id
}

// mustInsertChar inserts a character for ownerID with the bank/bot/removed flags,
// returning its id.
func mustInsertChar(t *testing.T, db *sql.DB, ownerID int64, name string, isBank, isBot, isRemoved bool) int64 {
	t.Helper()
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	res, err := db.Exec(
		`INSERT INTO character (owner_id, name, is_bank_toon, is_guild_bot, is_removed) VALUES (?, ?, ?, ?, ?)`,
		ownerID, name, b(isBank), b(isBot), b(isRemoved))
	if err != nil {
		t.Fatalf("insert character %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("character last insert id: %v", err)
	}
	return id
}

// guildSentinelOwnerID is the FIXED reserved "guild" owner id seeded by 00015. It
// is BOUND to the Go source of truth (store.GuildSentinelOwnerID) rather than a
// duplicated literal (IN-01) so an accidental edit to the const is caught at compile
// time here and asserted against the documented contract by
// TestGuildSentinelOwnerID_MatchesContract. Only the migration .sql literal remains
// an unavoidable independent copy (SQL cannot import the Go const).
const guildSentinelOwnerID = store.GuildSentinelOwnerID

// backfillBankOwnerSQL is the EXACT backfill UPDATE 00015 runs. The test re-runs it
// after seeding rows because NewTestDB migrated over an EMPTY DB, so the in-migration
// backfill touched nothing (the 00009 re-run pattern). The statement is idempotent:
// the `owner_id <> ?` guard makes a re-run touch zero rows.
const backfillBankOwnerSQL = `UPDATE character
   SET owner_id = ?
 WHERE (is_bank_toon = 1 OR is_guild_bot = 1)
   AND owner_id <> ?`

// ownerIDOfChar reads a character's owner_id by name.
func ownerIDOfChar(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	var got int64
	if err := db.QueryRow(`SELECT owner_id FROM character WHERE name = ?`, name).Scan(&got); err != nil {
		t.Fatalf("read owner_id for char %q: %v", name, err)
	}
	return got
}

// TestMigrate_00015_SeedsGuildOwnerAndBackfillsBanks proves the Phase 35 forward-only
// migration 00015 (OWN-01/02/04):
//   - the reserved sentinel owner (id 1000000, label 'guild') is seeded;
//   - the backfill repoints every is_bank_toon=1 OR is_guild_bot=1 char to the sentinel
//     EVEN when it was bound to an individual owner before (the Findom->owner 9 case,
//     OWN-04) — proven by seeding a real-owner bank/bot then re-running the migration's
//     EXACT backfill UPDATE (NewTestDB migrated over an empty DB, so it backfilled
//     nothing — the 00009 re-run pattern);
//   - a NORMAL (non-bank, non-bot) char's owner_id is UNCHANGED by the backfill;
//   - the backfill is idempotent (the `owner_id <> sentinel` guard) and a second
//     RunMigrations is a clean no-op (goose_db_version row count unchanged).
//
// Backend-only: the watcher never touches owner/character.owner_id directly, so there
// is NO WatcherMaxSchemaVersion change. "Schema v15" == goose 00015 applied.
func TestMigrate_00015_SeedsGuildOwnerAndBackfillsBanks(t *testing.T) {
	db := store.NewTestDB(t) // Open + goose.Up (00001..00015) + t.Cleanup

	// The sentinel owner row exists with label 'guild'.
	var label string
	if err := db.QueryRow(`SELECT label FROM owner WHERE id = ?`, guildSentinelOwnerID).Scan(&label); err != nil {
		t.Fatalf("read sentinel owner (id=%d): %v", guildSentinelOwnerID, err)
	}
	if label != "guild" {
		t.Errorf("sentinel owner label = %q, want 'guild'", label)
	}

	// Seed a real owner + an owner-bound bank (Findom), bot (Botchar), and a normal char.
	realOwner := mustInsertOwner(t, db, "RealGuildie", nil)
	mustInsertChar(t, db, realOwner, "Findom", true /*isBank*/, false, false)
	mustInsertChar(t, db, realOwner, "Botchar", false, true /*isBot*/, false)
	mustInsertChar(t, db, realOwner, "Normalchar", false, false, false)

	// Sanity: before the backfill they all sit under realOwner.
	if got := ownerIDOfChar(t, db, "Findom"); got != realOwner {
		t.Fatalf("pre-backfill Findom owner_id = %d, want realOwner %d", got, realOwner)
	}

	// Re-run the migration's EXACT backfill (idempotent; NewTestDB backfilled nothing).
	if _, err := db.Exec(backfillBankOwnerSQL, guildSentinelOwnerID, guildSentinelOwnerID); err != nil {
		t.Fatalf("re-run backfill: %v", err)
	}

	// OWN-04: the owner-bound bank + bot repoint to the sentinel.
	if got := ownerIDOfChar(t, db, "Findom"); got != guildSentinelOwnerID {
		t.Errorf("after backfill Findom owner_id = %d, want sentinel %d (OWN-04)", got, guildSentinelOwnerID)
	}
	if got := ownerIDOfChar(t, db, "Botchar"); got != guildSentinelOwnerID {
		t.Errorf("after backfill Botchar owner_id = %d, want sentinel %d (OWN-04)", got, guildSentinelOwnerID)
	}
	// The normal char is UNTOUCHED (the backfill must not sweep normal chars).
	if got := ownerIDOfChar(t, db, "Normalchar"); got != realOwner {
		t.Errorf("after backfill Normalchar owner_id = %d, want realOwner %d (untouched)", got, realOwner)
	}

	// Idempotency: a second backfill re-run is harmless (the prior assertions still hold).
	if _, err := db.Exec(backfillBankOwnerSQL, guildSentinelOwnerID, guildSentinelOwnerID); err != nil {
		t.Fatalf("second backfill re-run: %v", err)
	}
	if got := ownerIDOfChar(t, db, "Findom"); got != guildSentinelOwnerID {
		t.Errorf("after second backfill Findom owner_id = %d, want sentinel %d", got, guildSentinelOwnerID)
	}
	if got := ownerIDOfChar(t, db, "Normalchar"); got != realOwner {
		t.Errorf("after second backfill Normalchar owner_id = %d, want realOwner %d (still untouched)", got, realOwner)
	}

	// Forward-only/idempotent: a second RunMigrations over an already-at-00015 DB
	// returns nil AND the goose_db_version row count is unchanged.
	var beforeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count goose_db_version before re-run: %v", err)
	}
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00015 should be a no-op, got error: %v", err)
	}
	var afterVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM goose_db_version`).Scan(&afterVersions); err != nil {
		t.Fatalf("count goose_db_version after re-run: %v", err)
	}
	if beforeVersions != afterVersions {
		t.Fatalf("goose_db_version row count changed on re-run: before=%d after=%d (not idempotent)", beforeVersions, afterVersions)
	}
}

// TestGuildSentinelOwnerID_MatchesContract pins the Go-side sentinel id to the
// documented contract value (1000000). IN-01: with the migration test now BOUND to
// store.GuildSentinelOwnerID, an accidental edit to that const would silently change
// every test alongside it — this single literal assertion catches such a drift
// against the value the migration .sql literal and PROJECT docs are pinned to.
func TestGuildSentinelOwnerID_MatchesContract(t *testing.T) {
	if store.GuildSentinelOwnerID != 1000000 {
		t.Errorf("store.GuildSentinelOwnerID = %d, want 1000000 (the reserved-sentinel contract; the 00015 .sql literal is pinned to it)", store.GuildSentinelOwnerID)
	}
}

// TestMigrate_00015_BackfillRunsOverPreExistingData drives the ACTUAL embedded 00015
// backfill over pre-existing rows (WR-01) — the highest-stakes correctness claim in
// the phase (it silently re-homes real guildie-owned banks). It mirrors
// TestMigrate_00011's part (c): pin a fresh raw handle at v14, seed an owner-bound
// bank + bot + a normal char while the sentinel does NOT yet exist, then UpTo(15) so
// the migration's OWN line — `UPDATE character SET owner_id=1000000 WHERE
// (is_bank_toon=1 OR is_guild_bot=1) AND owner_id<>1000000` — runs against non-empty
// data. The bank/bot must repoint to the sentinel; the normal char must be UNTOUCHED.
// This exercises the shipped SQL itself (NOT a hand-copied string), removing the
// drift risk WR-01 flags in the sibling test's re-run.
func TestMigrate_00015_BackfillRunsOverPreExistingData(t *testing.T) {
	// Pin a fresh raw handle at v14 — BEFORE 00015 seeds the sentinel + backfills.
	raw := openAtVersion(t, 14)

	// Seed a real owner + an owner-bound bank, an owner-bound bot, and a normal char,
	// ALL bound to the real owner. (At v14 the sentinel owner row does not yet exist.)
	realOwner := mustInsertOwner(t, raw, "RealGuildie", nil)
	mustInsertChar(t, raw, realOwner, "Findom", true /*isBank*/, false, false)
	mustInsertChar(t, raw, realOwner, "Botchar", false, true /*isBot*/, false)
	mustInsertChar(t, raw, realOwner, "Normalchar", false, false, false)

	// Sanity: pre-migration they all sit under the real owner.
	if got := ownerIDOfChar(t, raw, "Findom"); got != realOwner {
		t.Fatalf("pre-00015 Findom owner_id = %d, want realOwner %d", got, realOwner)
	}

	// Drive the ACTUAL embedded 00015 (seed sentinel + run the backfill) over the
	// seeded rows — NOT a hand-copied SQL string.
	if err := migrations.UpTo(raw, 15); err != nil {
		t.Fatalf("UpTo(15) over the seeded v14 DB: %v", err)
	}

	// The sentinel owner row now exists (step 1 of 00015) so the FK repoint is valid.
	var label string
	if err := raw.QueryRow(`SELECT label FROM owner WHERE id = ?`, guildSentinelOwnerID).Scan(&label); err != nil {
		t.Fatalf("read sentinel owner after 00015 (id=%d): %v", guildSentinelOwnerID, err)
	}
	if label != "guild" {
		t.Errorf("sentinel owner label = %q, want 'guild'", label)
	}

	// OWN-04: the embedded backfill repointed the owner-bound bank + bot to the sentinel.
	if got := ownerIDOfChar(t, raw, "Findom"); got != guildSentinelOwnerID {
		t.Errorf("after embedded 00015 backfill Findom owner_id = %d, want sentinel %d (OWN-04)", got, guildSentinelOwnerID)
	}
	if got := ownerIDOfChar(t, raw, "Botchar"); got != guildSentinelOwnerID {
		t.Errorf("after embedded 00015 backfill Botchar owner_id = %d, want sentinel %d (OWN-04)", got, guildSentinelOwnerID)
	}
	// The normal char is UNCHANGED — the backfill must NOT sweep non-bank/non-bot chars.
	if got := ownerIDOfChar(t, raw, "Normalchar"); got != realOwner {
		t.Errorf("after embedded 00015 backfill Normalchar owner_id = %d, want realOwner %d (untouched)", got, realOwner)
	}
}
