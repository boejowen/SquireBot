// Command spike/pocketbase is a THROWAWAY PocketBase-as-framework spike for
// Phase 11 decision D-01. It is NOT production code — see README.md.
//
// It boots PocketBase as a library (`pocketbase.New()`) and exercises the four
// concrete PASS/FAIL probes from 11-CONTEXT.md §D-01 / 11-RESEARCH.md
// §"PocketBase Spike Probes":
//
//	(a) plain SQL tables (owner/character/inventory_item/spellbook_entry + one
//	    empty dimension table) coexisting with PocketBase's own SQLite file;
//	(b) a custom-bearer-guarded POST /api/v1/ingest route doing an atomic
//	    full-snapshot replace inside app.RunInTransaction — using a custom
//	    crypto/subtle guard, NOT PocketBase's apis.RequireAuth() JWT system
//	    (guild codes are opaque static tokens, not PB auth records);
//	(c) an in-process app.Cron() heartbeat that fires while serving;
//	(d) cross-compiles GOOS=linux GOARCH=amd64 CGO_ENABLED=0 (proven by the
//	    Task 2 build, no runtime code here).
//
// SECURITY (T-11.01-01 / T-11.01-03): the guard compares against a HARDCODED
// TEST token hash and the handler inserts SYNTHETIC rows only. No crypto/rand
// real code is minted, no real character data is touched, and the bearer guard
// is a plain func(*core.RequestEvent) error — it deliberately does NOT model
// PocketBase's auth-record/JWT semantics. The production guard lands in 11-04.
package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"os"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// spikeTestToken is the throwaway plaintext bearer token the harness accepts.
// Present it as `Authorization: Bearer <spikeTestToken>` to drive probe (b).
// This is a TEST credential only — never a real guild code (T-11.01-01).
const spikeTestToken = "spike-test-token-do-not-use-in-prod"

// spikeTestTokenHash is sha256(spikeTestToken). The bearer guard constant-time
// compares the SHA-256 of the presented token against this, mirroring the
// production hash-only-storage discipline (D-08) without any real secret.
var spikeTestTokenHash = sha256.Sum256([]byte(spikeTestToken))

func main() {
	app := pocketbase.New()

	// Probe (c) — in-process cron skeleton. A 1-minute heartbeat proves
	// app.Cron() schedules jobs that fire while the server runs (D-01 c).
	// PocketBase auto-starts the scheduler when it serves.
	app.Cron().MustAdd("spike-heartbeat", "*/1 * * * *", func() {
		log.Println("spike: cron fired (probe c)")
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Probe (a) — plain SQL tables created via raw DDL run on the same
		// SQLite handle PocketBase manages (A5: plain tables are fine for the
		// spike). These coexist with PB's own system tables / collections.
		if err := createSpikeTables(se.App); err != nil {
			return err
		}

		// Probe (b) — custom-bearer-guarded ingest route doing atomic replace.
		// The guard is bound per-route via Route.Bind with a hook.Handler whose
		// Func is our own constant-time bearer check — NOT apis.RequireAuth().
		se.Router.POST("/api/v1/ingest", ingestHandler).
			Bind(&hook.Handler[*core.RequestEvent]{
				Id:   "spike-bearer-guard",
				Func: bearerGuard,
			})

		log.Println("spike: probes a/b/c wired (tables created, guarded route bound, cron scheduled)")
		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatalf("spike: app.Start failed: %v", err)
		os.Exit(1)
	}
}

// createSpikeTables runs raw DDL for the four core tables plus one empty
// dimension table (item_master), proving plain SQL tables coexist with
// PocketBase's SQLite file (probe a). DDL is IF NOT EXISTS so re-serving is
// idempotent. Shapes mirror 11-RESEARCH.md §"Migration SQL Sketch" (trimmed).
func createSpikeTables(app core.App) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS owner (
			id          INTEGER PRIMARY KEY,
			label       TEXT NOT NULL,
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS character (
			id        INTEGER PRIMARY KEY,
			owner_id  INTEGER NOT NULL REFERENCES owner(id),
			name      TEXT NOT NULL UNIQUE COLLATE NOCASE
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_item (
			id           INTEGER PRIMARY KEY,
			character_id INTEGER NOT NULL,
			location     TEXT NOT NULL,
			name         TEXT NOT NULL,
			item_id      INTEGER,
			count        INTEGER NOT NULL DEFAULT 1,
			slots        INTEGER,
			row_ordinal  INTEGER NOT NULL,
			uploaded_at  TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS spellbook_entry (
			id           INTEGER PRIMARY KEY,
			character_id INTEGER NOT NULL,
			level        INTEGER NOT NULL,
			name         TEXT NOT NULL,
			uploaded_at  TEXT NOT NULL
		)`,
		// One empty dimension table (P12 would populate; here only created).
		`CREATE TABLE IF NOT EXISTS item_master (
			item_id      INTEGER PRIMARY KEY,
			name         TEXT,
			wiki_summary TEXT
		)`,
	}
	for _, sql := range stmts {
		if _, err := app.DB().NewQuery(sql).Execute(); err != nil {
			return err
		}
	}
	// Ensure a character row exists so the atomic-replace probe has a target.
	if _, err := app.DB().NewQuery(
		`INSERT OR IGNORE INTO owner (id, label) VALUES (1, 'spike-owner')`).
		Execute(); err != nil {
		return err
	}
	if _, err := app.DB().NewQuery(
		`INSERT OR IGNORE INTO character (id, owner_id, name) VALUES (1, 1, 'SpikeChar')`).
		Execute(); err != nil {
		return err
	}
	return nil
}

// bearerGuard is the custom auth middleware for probe (b). It reads the
// Authorization header, strips the "Bearer " prefix, SHA-256-hashes the
// presented token, and constant-time compares it against the hardcoded test
// hash. On miss it returns 401 (writing nothing); on hit it calls e.Next() to
// proceed to ingestHandler. This is deliberately NOT apis.RequireAuth() — see
// the package doc and D-01 criterion (b).
func bearerGuard(e *core.RequestEvent) error {
	const prefix = "Bearer "
	authz := e.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authz, prefix) {
		return e.UnauthorizedError("missing or malformed bearer token", nil)
	}
	presented := strings.TrimPrefix(authz, prefix)
	sum := sha256.Sum256([]byte(presented))
	if subtle.ConstantTimeCompare(sum[:], spikeTestTokenHash[:]) != 1 {
		return e.UnauthorizedError("bad token", nil)
	}
	return e.Next()
}

// ingestHandler performs the atomic full-snapshot replace inside a single
// transaction (probe b): DELETE all rows for the character, then INSERT the
// new (synthetic) snapshot. A shrinking snapshot drops removed rows for free.
// Mirrors the watcher's clear+write contract reimplemented as one SQLite tx.
func ingestHandler(e *core.RequestEvent) error {
	const charID = 1
	err := e.App.RunInTransaction(func(txApp core.App) error {
		if _, err := txApp.DB().NewQuery(
			`DELETE FROM inventory_item WHERE character_id = {:cid}`).
			Bind(dbx.Params{"cid": charID}).Execute(); err != nil {
			return err
		}
		// Synthetic two-row snapshot — no real character data (T-11.01-01).
		rows := [][]any{
			{"General1", "Spider Silk", 10001, 1, 0, 0},
			{"General2", "Bone Chips", 10002, 5, 0, 1},
		}
		for _, r := range rows {
			if _, err := txApp.DB().NewQuery(
				`INSERT INTO inventory_item
					(character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
				 VALUES ({:cid}, {:loc}, {:name}, {:iid}, {:cnt}, {:slots}, {:ord}, datetime('now'))`).
				Bind(dbx.Params{
					"cid":   charID,
					"loc":   r[0],
					"name":  r[1],
					"iid":   r[2],
					"cnt":   r[3],
					"slots": r[4],
					"ord":   r[5],
				}).Execute(); err != nil {
				return err
			}
		}
		return nil // commit; any returned error rolls the whole tx back
	})
	if err != nil {
		return e.InternalServerError("atomic replace failed", err)
	}
	return e.JSON(200, map[string]any{"ok": true, "replaced": 2})
}
