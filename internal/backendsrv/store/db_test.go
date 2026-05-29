package store

import (
	"strings"
	"testing"
)

// TestOpen_ForeignKeysEnabled proves the foreign_keys pragma in the DSN takes
// effect on a fresh connection. SQLite's foreign_keys pragma is NOT persistent
// (it resets per connection), so this guards T-11.02-01: without it, the
// ON DELETE CASCADE actions on inventory_item/spellbook_entry would be silently
// ignored. We open via NewTestDB (which routes through Open) and read the pragma
// back through the pooled connection.
func TestOpen_ForeignKeysEnabled(t *testing.T) {
	db := NewTestDB(t)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys query failed: %v", err)
	}
	if fk != 1 {
		t.Fatalf("expected foreign_keys=1 on a fresh connection, got %d", fk)
	}
}

// TestDSN_ContainsPragmas asserts the connection string carries the exact
// load-bearing pragmas from RESEARCH Pattern 5 — WAL (concurrent read-during-
// write), busy_timeout (wait, don't error), foreign_keys (FK enforcement), and
// _txlock=immediate (BEGIN IMMEDIATE to avoid the SQLITE_BUSY upgrade deadlock).
func TestDSN_ContainsPragmas(t *testing.T) {
	dsn := DSN("/var/lib/squirebot/squirebot.db")

	for _, want := range []string{
		"journal_mode(WAL)",
		"busy_timeout(5000)",
		"foreign_keys(ON)",
		"synchronous(NORMAL)",
		"_txlock=immediate",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN missing %q; got %q", want, dsn)
		}
	}
}
