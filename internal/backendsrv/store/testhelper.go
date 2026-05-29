package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/migrations"
)

// NewTestDB opens a temp-file modernc SQLite DB (via Open, so it gets the full
// WAL/busy_timeout/foreign_keys/_txlock pragma set + SetMaxOpenConns(1)), runs
// goose.Up to apply the D-13 schema, and returns the live handle. It registers
// a t.Cleanup that closes the handle; the temp directory itself is removed
// automatically by t.TempDir().
//
// This is the SHARED backend test fixture (RESEARCH "Wave 0 Gaps"): store,
// ingest, and auth tests in 11-03/11-04/11-05 all spin up a migrated DB through
// this one helper. It deliberately lives in a non-_test.go file so it is
// importable from other packages' test code (the same pattern as
// net/http/httptest); the testing import is intentional test-support tooling,
// not production runtime code.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "squirebot-test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("NewTestDB: open temp DB at %s: %v", dbPath, err)
	}
	t.Cleanup(func() {
		if cerr := db.Close(); cerr != nil {
			t.Errorf("NewTestDB cleanup: close DB: %v", cerr)
		}
	})

	if err := migrations.RunMigrations(db); err != nil {
		t.Fatalf("NewTestDB: run migrations: %v", err)
	}

	return db
}
