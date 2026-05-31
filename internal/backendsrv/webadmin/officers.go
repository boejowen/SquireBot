package webadmin

// officers.go is the officer-management endpoint backend (ADMIN-06 / D-06/D-07/
// D-08) — the SQLite/HTTP port of v1's admin-mgmt sidebar over admin.ts. The
// route layer (cmd/squirebot-server) wraps all three handlers in
// webauth.RequireOfficer (the cheap REQUEST-TIME gate); the mutators ALSO
// re-authorize INSIDE their *sql.Tx via store.AddOfficerTx / RemoveOfficerTx
// (whose first statement is an officer re-check on the tx snapshot) — this is the
// authorize-under-transaction that closes the v1 WR-04 TOCTOU window (a
// just-removed officer cannot land one final write). The store mutators run with
// BEGIN IMMEDIATE (the store DSN sets _txlock=immediate, so db.BeginTx takes the
// write lock up front) — belt-and-suspenders alongside the in-tx re-check.
//
// Error codes match v1 + the UI-SPEC routing exactly: not_authorized (403,
// store.ErrNotAuthorized — the in-tx re-check failed), owner_floor_protected
// (403, store.ErrOwnerFloorProtected — a peer targeting the floor). Every
// successful write appends an append-only audit_log row in the SAME tx.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// --- shared helpers (used by officers.go / eviction.go / coin.go) ------------

// writeJSONError writes a {"error":"code"} body with the given status — the
// established shape (mirrors ingest + the webauth handlers + the v1 error strings
// the store returns). The frontend routes the code to its inline message.
func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// writeJSON writes a 200 JSON body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// nowUnix is the single timestamp source for the web-write audit/grace stamps
// (unix epoch seconds, UTC). A package var so a test could pin it if needed.
var nowUnix = func() int64 { return time.Now().Unix() }

// caller reads the acting discord_user_id the gate placed in the context. An
// absent identity (should never happen behind RequireOfficer/RequireSession) is
// fail-closed: "" makes the in-tx officer re-check reject (store.IsOfficer "" →
// false), so the write is denied rather than mis-attributed.
func caller(ctx context.Context) string {
	uid, _ := webauth.UserFromContext(ctx)
	return uid
}

// --- handlers ----------------------------------------------------------------

// OfficersListHandler (GET) returns the current officers + the promotable users
// (logged-in web_users not already officers — the D-07 pick source). Wrapped in
// RequireOfficer at the route. {officers:[...], promotable:[...]}.
func OfficersListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		officers, err := store.ListOfficers(ctx, db)
		if err != nil {
			slog.Error("officers list failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		promotable, err := store.ListPromotableUsers(ctx, db)
		if err != nil {
			slog.Error("promotable list failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		// Non-nil empty slices so the JSON is [] not null (the UI's no-promotable-users
		// empty state keys off an empty array).
		if officers == nil {
			officers = []store.Officer{}
		}
		if promotable == nil {
			promotable = []store.Officer{}
		}
		writeJSON(w, map[string]any{"officers": officers, "promotable": promotable})
	}
}

// officerReq is the {discord_user_id} body shared by add/remove.
type officerReq struct {
	DiscordUserID string `json:"discord_user_id"`
}

// OfficerAddHandler (POST) promotes a picked logged-in user (D-07). Officer-only
// at the route; authorizes INSIDE the tx (store.AddOfficerTx re-checks the
// caller's officer status as its first SELECT — WR-04). Idempotent: an
// already-officer target returns 200 {"added":false}. Audits "officer_add".
func OfficerAddHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req officerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DiscordUserID == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var added bool
		// ONE tx: authorize-under-tx (store.AddOfficerTx re-checks IsOfficer first) +
		// the audit row, committed together. The store DSN is _txlock=immediate, so
		// this BeginTx is effectively BEGIN IMMEDIATE (write lock up front).
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			added, e = store.AddOfficerTx(ctx, tx, req.DiscordUserID, callerID, now)
			if e != nil {
				return e
			}
			// Audit only a real promotion (an idempotent no-op need not spam the log,
			// matching v1's appendAdminLogEntry being called only on an actual add).
			if added {
				return AppendAuditTx(ctx, tx, "officer_add", callerID, map[string]any{
					"target": req.DiscordUserID,
				}, now)
			}
			return nil
		})
		if err != nil {
			mapOfficerErr(w, err, "officer_add")
			return
		}

		username := usernameOf(ctx, db, req.DiscordUserID)
		writeJSON(w, map[string]any{"added": added, "username": username})
	}
}

// OfficerRemoveHandler (POST) demotes an officer. Officer-only at the route;
// authorizes INSIDE the tx; owner-floor protected (a peer removing the floor →
// 403 owner_floor_protected, BEFORE any write); idempotent not-found →
// {"removed":false}. Audits "officer_remove".
func OfficerRemoveHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req officerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DiscordUserID == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		// Resolve the username BEFORE the tx (the row still exists pre-delete) so the
		// success response can echo it for the UI's "Officer removed: <username>."
		username := usernameOf(ctx, db, req.DiscordUserID)

		var removed bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			removed, e = store.RemoveOfficerTx(ctx, tx, req.DiscordUserID, callerID, now)
			if e != nil {
				return e
			}
			if removed {
				return AppendAuditTx(ctx, tx, "officer_remove", callerID, map[string]any{
					"target": req.DiscordUserID,
				}, now)
			}
			return nil
		})
		if err != nil {
			mapOfficerErr(w, err, "officer_remove")
			return
		}

		writeJSON(w, map[string]any{"removed": removed, "username": username})
	}
}

// --- error mapping + small helpers ------------------------------------------

// mapOfficerErr maps the store's typed errors to the exact v1 HTTP codes the
// frontend routes; anything else is a 500. T-15-17: a failed write writes no
// audit row (the tx rolled back).
func mapOfficerErr(w http.ResponseWriter, err error, op string) {
	switch {
	case errors.Is(err, store.ErrOwnerFloorProtected):
		writeJSONError(w, http.StatusForbidden, "owner_floor_protected")
	case errors.Is(err, store.ErrNotAuthorized):
		writeJSONError(w, http.StatusForbidden, "not_authorized")
	default:
		slog.Error("officer write failed", "op", op, "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}

// usernameOf looks up a web_user's display username (best-effort — a missing row
// yields "" rather than an error; the id is the load-bearing value, the username
// is only for the success toast).
func usernameOf(ctx context.Context, db *sql.DB, discordUserID string) string {
	var u string
	err := db.QueryRowContext(ctx,
		`SELECT username FROM web_user WHERE discord_user_id = ?`, discordUserID,
	).Scan(&u)
	if err != nil {
		return ""
	}
	return u
}
