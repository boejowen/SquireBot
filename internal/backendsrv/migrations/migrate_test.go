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
	db := store.NewTestDB(t) // Open + goose.Up (00001..00006) + t.Cleanup

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

	// Forward-only/idempotent: a second RunMigrations over an already-at-00006 DB
	// returns nil (goose records applied versions).
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00006 should be a no-op, got error: %v", err)
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
	db := store.NewTestDB(t) // Open + goose.Up (00001..00007) + t.Cleanup

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

	// Forward-only/idempotent: a second RunMigrations over an already-at-00007 DB
	// returns nil (goose records applied versions).
	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations after 00007 should be a no-op, got error: %v", err)
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
