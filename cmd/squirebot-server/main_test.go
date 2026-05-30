package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// TestRun_MintDispatch drives the mint-code subcommand through run() against a
// temp DB and asserts exit code 0 plus a guild_code row persisted (the dispatch
// opens the DB, runs goose.Up so a fresh box can mint before the first serve,
// and mints a code). This is the unit-testable proof of the os.Args dispatch
// (the build-level check covers serve; the on-box run is 11-06/11-07).
func TestRun_MintDispatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-mint.db")

	code := run([]string{"mint-code", "--owner", "alice", "--db", dbPath})
	if code != 0 {
		t.Fatalf("run(mint-code) exit = %d, want 0", code)
	}

	// The mint must have created the schema (goose.Up) and inserted a guild_code
	// row for the owner label. Open the same DB and assert exactly one active code.
	db := openForAssert(t, dbPath)
	defer db.Close()

	var n int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM guild_code gc
		JOIN owner o ON o.id = gc.owner_id
		WHERE o.label = ? AND gc.disabled_at IS NULL`, "alice").Scan(&n); err != nil {
		t.Fatalf("query guild_code: %v", err)
	}
	if n != 1 {
		t.Errorf("active guild_code rows for alice = %d, want 1", n)
	}
}

// TestRun_MintDispatch_MissingOwner: mint-code without --owner is a usage error
// (exit 2), and no DB mutation happens.
func TestRun_MintDispatch_MissingOwner(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-mint.db")
	if code := run([]string{"mint-code", "--db", dbPath}); code != 2 {
		t.Fatalf("run(mint-code without --owner) exit = %d, want 2", code)
	}
}

// TestRun_RevokeDispatch: revoke-code after a mint disables the code (exit 0) and
// the row is no longer active.
func TestRun_RevokeDispatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-revoke.db")

	if code := run([]string{"mint-code", "--owner", "bob", "--db", dbPath}); code != 0 {
		t.Fatalf("mint exit = %d, want 0", code)
	}
	if code := run([]string{"revoke-code", "bob", "--db", dbPath}); code != 0 {
		t.Fatalf("revoke exit = %d, want 0", code)
	}

	db := openForAssert(t, dbPath)
	defer db.Close()
	var active int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM guild_code gc
		JOIN owner o ON o.id = gc.owner_id
		WHERE o.label = ? AND gc.disabled_at IS NULL`, "bob").Scan(&active); err != nil {
		t.Fatalf("query: %v", err)
	}
	if active != 0 {
		t.Errorf("active guild_code rows for bob after revoke = %d, want 0", active)
	}
}

// TestRun_RevokeDispatch_MissingArg: revoke-code without an id/label is a usage
// error (exit 2).
func TestRun_RevokeDispatch_MissingArg(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-revoke.db")
	if code := run([]string{"revoke-code", "--db", dbPath}); code != 2 {
		t.Fatalf("run(revoke-code without arg) exit = %d, want 2", code)
	}
}

// TestRun_RunJob_BadName: run-job with an unknown job name is a usage error
// (exit 2) and makes no live fetch. The live pigparse/wiki success paths are
// covered by Plan 04's job tests (httptest-backed) + the manual D-7 parity check;
// here we assert only the dispatch/arg-handling layer so the unit test needs no
// network. (The bad name is rejected BEFORE the DB is opened, so the --db value
// is irrelevant — but we pass a temp path to mirror real invocation.)
func TestRun_RunJob_BadName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "bogus", "--db", dbPath}); code != 2 {
		t.Fatalf("run(run-job bogus) exit = %d, want 2", code)
	}
}

// TestRun_RunJob_MissingName: run-job with no positional job name is a usage error
// (exit 2).
func TestRun_RunJob_MissingName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "--db", dbPath}); code != 2 {
		t.Fatalf("run(run-job with no job name) exit = %d, want 2", code)
	}
}

// TestRun_RunJob_ExtraPositional: run-job with TWO job names is a usage error
// (exit 2). Documented choice: exactly one job name is required, so
// `run-job pigparse wiki` is rejected rather than silently running only the first
// (avoids an ambiguous "which one ran?" surprise in the D-7 parity check).
func TestRun_RunJob_ExtraPositional(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "pigparse", "wiki", "--db", dbPath}); code != 2 {
		t.Fatalf("run(run-job pigparse wiki) exit = %d, want 2 (exactly one job name required)", code)
	}
}

// TestRun_RunJob_NameAroundFlag: the job-name positional may appear AFTER the --db
// flag (splitFlagsAndPositionals handles ordering, same as revoke-code). A bad
// name in that position still exits 2 — proving the arg split, not a live run.
func TestRun_RunJob_NameAroundFlag(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "squirebot-runjob.db")
	if code := run([]string{"run-job", "--db", dbPath, "bogus"}); code != 2 {
		t.Fatalf("run(run-job --db X bogus) exit = %d, want 2", code)
	}
}

// openForAssert opens the DB the CLI wrote (it already has the schema via the
// subcommand's goose.Up) for read-back assertions, registering cleanup.
func openForAssert(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db for assert: %v", err)
	}
	return db
}
