// Package migrations embeds the forward-only goose SQL migrations for the
// SquireBot backend and applies them to a *sql.DB on startup.
//
// The migration files (00001_init.sql, …) are compiled into the binary via
// //go:embed, so "deploy = drop the new binary + restart" (D-10): there is no
// loose migrations directory to ship alongside the executable.
//
// FOOT-GUN (RESEARCH Pitfall 3): the database/sql DRIVER name is "sqlite"
// (modernc.org/sqlite, registered in store.Open) but the goose DIALECT string
// is "sqlite3". They live in independent namespaces and deliberately differ —
// passing "sqlite" to goose.SetDialect yields "unknown dialect", and opening
// the driver as "sqlite3" yields "unknown driver". Keep them distinct.
package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

// embedMigrations holds every *.sql goose migration co-located in this package
// directory. goose.SetBaseFS points goose at this FS instead of the real disk.
//
//go:embed *.sql
var embedMigrations embed.FS

// RunMigrations applies all forward (goose Up) migrations to db.
//
// It is idempotent (BACKEND-02): goose records applied versions in its own
// goose_db_version table, so a second call on an already-migrated database is a
// no-op and returns nil. Safe to call on every server startup.
func RunMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil { // ⚠️ "sqlite3" dialect, NOT the "sqlite" driver name
		return err
	}
	// "." = the embed FS root (migrations are co-located with this file).
	return goose.Up(db, ".")
}

// UpTo applies forward migrations up to AND including version (goose.UpTo) over
// the SAME embedded FS RunMigrations uses. TEST-SUPPORT ONLY: migration tests
// use it to stop at an intermediate schema version (e.g. seed pre-00011 rows,
// then resume to prove a data pass) — production startup always calls
// RunMigrations (HEAD). It intentionally mirrors RunMigrations' setup so the
// dialect/FS foot-guns stay in one shape.
func UpTo(db *sql.DB, version int64) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil { // ⚠️ "sqlite3" dialect, NOT the "sqlite" driver name
		return err
	}
	return goose.UpTo(db, ".", version)
}
