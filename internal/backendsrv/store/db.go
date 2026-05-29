// Package store owns the SquireBot backend's SQLite persistence: opening the
// modernc.org/sqlite handle with the correct DSN pragmas, and (in later plans)
// the atomic full-snapshot replace transaction and query helpers.
//
// Single-writer by design. The DB handle is the sole writer to the SQLite file;
// the watcher's internal/sheet/client.go funnels every Sheets mutation through
// one batchMu sync.Mutex so "no two writes can interleave" (client.go:97-104).
// That is the precedent for this server's single-writer discipline — here the
// SQLite mechanism is SetMaxOpenConns(1) + _txlock=immediate (RESEARCH Pattern 5),
// which serializes writes and eliminates SQLITE_BUSY at the guild's ~50–150
// writes/day (T-11.02-02).
package store

import (
	"database/sql"
	"path/filepath"

	// Registers the database/sql driver under the name "sqlite" (modernc, pure
	// Go, no cgo). NOTE: the driver name is "sqlite"; goose's dialect string is
	// "sqlite3" (migrations.RunMigrations) — they differ on purpose
	// (RESEARCH Pitfall 3).
	_ "modernc.org/sqlite"
)

// DSN builds the modernc "sqlite" connection string for dbPath. Exported so
// tests can assert the pragmas are present. The pragma set is verbatim from
// RESEARCH Pattern 5:
//
//   - journal_mode(WAL)    — readers proceed during a write
//   - busy_timeout(5000)   — wait 5s on a lock instead of erroring immediately
//   - foreign_keys(ON)     — NOT persistent; must be in the DSN so EVERY pooled
//     connection enforces FK actions (ON DELETE CASCADE) — T-11.02-01
//   - synchronous(NORMAL)  — safe with WAL, faster than FULL
//   - _txlock=immediate    — BEGIN IMMEDIATE for write txns; avoids the
//     SQLITE_BUSY upgrade deadlock (T-11.02-02)
//
// The path is forward-slashed (filepath.ToSlash) so the file: URI is valid on
// both the Windows dev/test box and the Linux deploy host.
func DSN(dbPath string) string {
	return "file:" + filepath.ToSlash(dbPath) + "?" +
		"_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_txlock=immediate"
}

// Open returns a *sql.DB on the modernc "sqlite" driver with the DSN pragmas
// above and SetMaxOpenConns(1) (single-writer server). sql.Open does not touch
// the file — the connection (and thus the pragmas) is established lazily on
// first use; callers that need eager failure should Ping.
//
// The connection string is intentionally not logged: it carries no secret in
// P11 (just a file path + pragmas), but keeping connection strings out of logs
// is the standing habit (CLAUDE.md / structured-logging convention).
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", DSN(dbPath)) // driver "sqlite" (modernc) — NOT "sqlite3"
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single-writer server; mirrors the watcher's batchMu intent (client.go:97-104)
	return db, nil
}
