package webadmin

// notifications.go is the Phase 20 (WANT-04) login-only notification-preferences +
// inbox endpoint backend — the account.go/wantlist.go twin. Every handler derives
// the acting owner from the Discord SESSION (caller(ctx) = webauth.UserFromContext,
// D-02); the request body NEVER carries an owner/discord field. The route layer
// wraps these in webauth.RequireSession (NOT RequireOfficer — every signed-in
// member manages their OWN prefs + reads their OWN inbox).
//
// It reuses the shared webadmin helpers verbatim (caller / nowUnix / writeJSON /
// writeJSONError from officers.go, withTx / AppendAuditTx from audit.go) and the
// Phase-20 store funcs (store.GetPrefs / UpsertPrefsTx / ListInbox / UnreadCount /
// MarkAlertReadTx / MarkAllAlertsReadTx).
//
// Security posture:
//   - Owner is session-derived (D-02); no body field selects an owner. A cross-owner
//     target is a silent no-op (the owner-scoped store mutators return RowsAffected=0
//     → false), never leaking the row's existence (the MarkAlertReadTx IDOR guard).
//   - GetPrefs is default-ON (D-01): an absent prefs row reads as all-true.
//   - The audit detail carries flags/ids/counts ONLY — never the alert Body text (V7).

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// GetPrefsHandler (GET) returns the caller's notification prefs (default-ON for a
// new user — the store reader returns all-true on an absent row, D-01). Owner from
// the session (caller(ctx), D-02). Read-only — no tx, no audit.
func GetPrefsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(r.Context()) // D-02: owner from session, body carries NO owner
		prefs, err := store.GetPrefs(ctx, db, callerID)
		if err != nil {
			slog.Error("get prefs failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, prefs)
	}
}

// prefsReq is the POST body for setting prefs. It carries the four booleans and NO
// owner field (D-02) — the owner is the session caller.
type prefsReq struct {
	Master bool `json:"master"`
	EC     bool `json:"ec"`
	WTS    bool `json:"wts"`
	Raid   bool `json:"raid"`
}

// SetPrefsHandler (POST) upserts the caller's notification prefs (owner from the
// session, D-02 — the body carries NO owner). The upsert + audit run in ONE withTx
// (BEGIN IMMEDIATE) so they are atomic; the handler re-reads + echoes the stored
// prefs. The audit detail carries the four flags ONLY — never any message text (V7).
func SetPrefsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req prefsReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(r.Context()) // D-02: owner from session
		now := nowUnix()
		p := store.NotifyPrefs{Master: req.Master, EC: req.EC, WTS: req.WTS, Raid: req.Raid}

		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.UpsertPrefsTx(ctx, tx, callerID, p, now); e != nil {
				return e
			}
			// V7: detail carries the flags ONLY — never message text.
			return AppendAuditTx(ctx, tx, "notify_prefs_set", callerID, map[string]any{
				"master": req.Master, "ec": req.EC, "wts": req.WTS, "raid": req.Raid,
			}, now)
		})
		if err != nil {
			slog.Error("set prefs failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		// Re-read the stored prefs so the response is the authoritative state.
		stored, err := store.GetPrefs(ctx, db, callerID)
		if err != nil {
			slog.Error("set prefs re-read failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, stored)
	}
}

// ListInboxHandler (GET) returns the caller's OWN alert_log rows newest-first,
// owner-scoped to caller(ctx). The store returns a non-nil slice so the JSON is
// always [] (never null) for the empty state.
func ListInboxHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(r.Context())
		out, err := store.ListInbox(ctx, db, callerID)
		if err != nil {
			slog.Error("list inbox failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, out) // non-nil from the store → JSON []
	}
}

// UnreadCountHandler (GET) returns {count:N} — the caller's unread-alert count (the
// nav-badge number, D-05). Owner from the session.
func UnreadCountHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(r.Context())
		n, err := store.UnreadCount(ctx, db, callerID)
		if err != nil {
			slog.Error("unread count failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, map[string]any{"count": n})
	}
}

// markReadReq is the {id} body for mark-read — the ALERT id only. The owner is NEVER
// from the body (D-02); it is the session caller.
type markReadReq struct {
	ID int64 `json:"id"`
}

// MarkReadHandler (POST) marks a single one of the caller's OWN alerts read,
// owner-scoped (store.MarkAlertReadTx — the IDOR guard): an alert belonging to a
// different member is a silent no-op (read:false) that never leaks its existence.
// Audits "notify_read" ONLY on a real flip (an idempotent no-op need not spam the
// log — the officers.go precedent), detail carrying the alert id only (V7).
func MarkReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req markReadReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(r.Context())
		now := nowUnix()

		var read bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			read, e = store.MarkAlertReadTx(ctx, tx, req.ID, callerID, now)
			if e != nil {
				return e
			}
			if read {
				// V7: detail carries the alert id ONLY.
				return AppendAuditTx(ctx, tx, "notify_read", callerID, map[string]any{"id": req.ID}, now)
			}
			return nil
		})
		if err != nil {
			slog.Error("mark read failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, map[string]any{"read": read})
	}
}

// MarkAllReadHandler (POST) marks ALL the caller's unread alerts read (owner-scoped,
// D-05). Returns {count:N} — the number of rows flipped. Audits "notify_read_all"
// with the count (V7: a count, never message text) only when something was flipped.
func MarkAllReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(r.Context())
		now := nowUnix()

		var count int64
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			count, e = store.MarkAllAlertsReadTx(ctx, tx, callerID, now)
			if e != nil {
				return e
			}
			if count > 0 {
				return AppendAuditTx(ctx, tx, "notify_read_all", callerID, map[string]any{"count": count}, now)
			}
			return nil
		})
		if err != nil {
			slog.Error("mark all read failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, map[string]any{"count": count})
	}
}
