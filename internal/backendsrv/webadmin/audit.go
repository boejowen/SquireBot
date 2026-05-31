// Package webadmin holds the three authenticated write-form backends (15-03):
// eviction (ADMIN-04, officer-only), bank-coin (ADMIN-05, any authenticated
// member per D-12), and officer management (ADMIN-06, officer-only). It composes
// the 15-01 store *Tx mutators under an audited transaction and the 15-02
// webauth gates — RequireSession / RequireOfficer at the route, the acting
// discord_user_id read via webauth.UserFromContext, and the in-tx officer
// re-check that closes the v1 WR-04 TOCTOU window.
//
// Handler convention (mirrors internal/backendsrv/ingest/handler.go + the
// webauth handlers): method-check first; JSON {"error":"code"} bodies via
// writeJSONError with the EXACT v1 error codes the frontend routes
// (not_authorized / owner_floor_protected / not_bank_toon / invalid_input /
// grace_expired); never log a secret or raw body (V7). Every
//
// WR-07: `lock_busy` is INTENTIONALLY NOT emitted by any handler here. The store
// uses busy_timeout(5000) (the writer waits rather than erroring on contention)
// and SetMaxOpenConns(1) (writes are serialized), so SQLITE_BUSY is engineered
// away — there is no contention surface to surface. The frontend keeps a
// `lock_busy` branch (classifyAdminError → 'lock-busy') purely as defense-in-depth
// for a hypothetical future where busy_timeout is lowered; it is unreachable from
// this backend today. If you ever add a code path that CAN return lock_busy, wire
// it through writeJSONError(w, 403, "lock_busy") so the existing frontend handling
// activates. Every
// mutating handler opens ONE *sql.Tx (the store DSN is _txlock=immediate ⇒ BEGIN
// IMMEDIATE), composes the store *Tx mutator + AppendAuditTx in that single tx,
// and commits — so the write and its audit row land atomically or not at all.
package webadmin

// audit.go is the append-only audit helper every webadmin write path uses. It
// REUSES/EXTENDS the existing audit_log table (D-06) — it does NOT invent a
// parallel log. The generic actor/detail/at columns were added in the 00004
// migration (15-01); this writes into them. The 00002-era `event` column carries
// the web-write event name; char_name/attempting_owner_id/current_owner_id stay
// NULL for these rows (they are ingest-specific). Append-only by construction:
// this is the ONLY audit_log writer in webadmin and it does INSERT exclusively —
// never UPDATE or DELETE (T-15-17 repudiation mitigation).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// AppendAuditTx writes one append-only audit_log row inside the caller's tx, so
// the audit record is committed atomically with the write it describes (a rolled-
// back write leaves no orphan audit row, and a committed write always has its
// trail). event is the web-write event name (officer_add / officer_remove /
// eviction / eviction_restore / coin_set); actor is the acting discord_user_id
// (from webauth.UserFromContext); detail is a small struct JSON-marshalled into
// the detail column. at is unix epoch seconds (the web-write timestamp convention;
// the existing ingest rows keep their created_at TEXT default).
//
// detail must marshal to a small JSON blob (never raw request bodies or secrets —
// V7); a marshal failure is returned (the caller rolls back the tx). Parameterized
// ? placeholders only (V5).
func AppendAuditTx(ctx context.Context, tx *sql.Tx, event, actor string, detail any, now int64) error {
	blob, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail (event=%q): %w", event, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO audit_log (at, event, detail, actor) VALUES (?, ?, ?, ?)`,
		now, event, string(blob), actor,
	); err != nil {
		return fmt.Errorf("append audit_log row (event=%q): %w", event, err)
	}
	return nil
}

// withTx is the single tx-composition helper every webadmin mutating handler
// uses: open a transaction, run fn, COMMIT on success / ROLLBACK on any error
// (so a store-error or audit-error rolls BOTH the write and its audit row back —
// the write and its audit trail are atomic). The store DSN sets _txlock=immediate,
// so db.BeginTx issues BEGIN IMMEDIATE — the write lock is taken up front, which
// (together with the store mutators' in-tx officer re-check) closes the v1 WR-04
// TOCTOU window. A commit failure is returned for the handler to map to 500.
//
// WR-03: the rollback is a DEFERRED guard keyed on a `committed` flag, not an
// inline rollback on the error path. If fn PANICS (it marshals arbitrary detail
// + runs store mutators), an inline-only rollback would never run and — with the
// store's SetMaxOpenConns(1) — the single pooled writer connection would be left
// holding an open BEGIN IMMEDIATE tx, wedging every subsequent write (and read)
// until the GC finalizer eventually closed the orphaned *sql.Tx. The deferred
// rollback unwinds the tx as the panic propagates, freeing the connection.
// database/sql makes a Rollback after a successful Commit a harmless no-op, so the
// flag is the clean idiom (the panic still propagates — we do not recover it).
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE
	if err != nil {
		return fmt.Errorf("begin webadmin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback() // no-op after a successful Commit; frees the conn on a panic/error
		}
	}()
	if ferr := fn(tx); ferr != nil {
		return ferr
	}
	if cerr := tx.Commit(); cerr != nil {
		return fmt.Errorf("commit webadmin tx: %w", cerr)
	}
	committed = true
	return nil
}
