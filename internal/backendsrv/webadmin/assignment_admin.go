package webadmin

// assignment_admin.go is the OFFICER character-assignment endpoint backend (Phase 26 /
// ASSIGN-04/05/06) — the officer-only twin of assignment.go, mirroring officers.go.
// An officer can list every assignment + the pending-request queue, directly
// assign/reassign/remove any char's assignment, approve/deny member requests, and
// designate a char as a guild bank/bot (or clear it). The route layer wraps these in
// webauth.RequireOfficer (the request-time gate); the store *Tx mutators ALSO re-check
// IsOfficerTx as their first in-tx statement (WR-04 authorize-under-transaction), so a
// just-demoted officer cannot land a final mutation (T-26-09 / T-26-10). The hidden
// admin nav is UX only — the route gate + the in-tx re-check are the real boundary.
//
// Security posture:
//   - The ACTOR is ALWAYS the session caller (caller(ctx)) — NEVER a body field. The
//     officer body legitimately carries a TARGET (the assignee for assign; the mode for
//     designate; the character_id / request_id everywhere) but never the actor.
//   - Errors flow through mapAssignErr (NOT mapOfficerErr): mapAssignErr covers
//     ErrCharShared (a bank/bot-char assign → 409 char_shared) which mapOfficerErr does
//     not — routing officer-assignment errors through mapOfficerErr would mis-handle a
//     shared-char assign as a generic 500.
//   - OfficerAssign validates the body assignee with a DEDICATED existence probe
//     (`SELECT 1 FROM web_user WHERE discord_user_id = ?`) → 400 invalid_input ONLY on
//     sql.ErrNoRows. It does NOT use usernameOf: that returns "" both for a missing row
//     AND for a real user whose username is NULL/empty, a false-reject of a valid
//     assignee (T-26-11). The character_assignment.discord_user_id FK → web_user is the
//     DB-level backstop; the probe just yields a clean 400 before the tx.
//   - DesignateChar allow-lists mode ∈ {bank,bot,none} → 400 invalid_input otherwise.
//   - Every mutation composes the store *Tx mutator + AppendAuditTx in ONE withTx
//     (ASSIGN-06 / T-26-13); the audit detail carries character_id (+ target on assign)
//     only (V7).

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// ListAllAssignmentsHandler (GET) returns every live-char assignment + the pending
// request queue for the officer panel (ASSIGN-04): {"assignments":[...],"requests":[...]}.
// Each is a non-nil slice so the JSON is [] (never null) for the empty state.
func ListAllAssignmentsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		assignments, err := store.ListAllAssignments(ctx, db)
		if err != nil {
			slog.Error("list all assignments failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		requests, err := store.ListPendingRequests(ctx, db)
		if err != nil {
			slog.Error("list pending requests failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if assignments == nil {
			assignments = []store.Assignment{}
		}
		if requests == nil {
			requests = []store.PendingRequest{}
		}
		writeJSON(w, map[string]any{"assignments": assignments, "requests": requests})
	}
}

// officerAssignReq is the {character_id, assignee} body for a direct officer assign /
// reassign (D-09). The ACTOR is the session caller, never the body; `assignee` is the
// TARGET who receives the assignment.
type officerAssignReq struct {
	CharacterID int64  `json:"character_id"`
	Assignee    string `json:"assignee"`
}

// OfficerAssignHandler (POST {character_id, assignee}) assigns/reassigns/overrides a
// char to assignee (D-09). Officer-only at the route; authorized in-tx (store
// .OfficerAssignTx re-checks IsOfficerTx first → ErrNotAuthorized → 403). Rejects a
// bank/bot char (ErrCharShared → 409 char_shared). The assignee is validated by a
// dedicated existence probe (NOT usernameOf — T-26-11). Audits "officer_assign" with
// {character_id, target}.
func OfficerAssignHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req officerAssignReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 || req.Assignee == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		// Validate the TARGET assignee exists as a web_user via a dedicated existence
		// probe (T-26-11). A NULL/empty username is still a VALID assignment target — so
		// probe the row's existence, NOT its username (usernameOf would false-reject it).
		// The FK → web_user is the DB backstop; this turns it into a clean 400 pre-tx.
		var exists int
		err := db.QueryRowContext(ctx,
			`SELECT 1 FROM web_user WHERE discord_user_id = ?`, req.Assignee).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		if err != nil {
			slog.Error("officer assign: assignee existence probe failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		callerID := caller(ctx) // actor from session, never the body.
		now := nowUnix()

		err = withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.OfficerAssignTx(ctx, tx, req.CharacterID, req.Assignee, callerID, now); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "officer_assign", callerID,
				map[string]any{"character_id": req.CharacterID, "target": req.Assignee}, now)
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"assigned": true})
	}
}

// adminCharReq is the {character_id} body for the officer endpoints that act on a char
// without a target (remove). The actor is the session caller.
type adminCharReq struct {
	CharacterID int64 `json:"character_id"`
}

// OfficerRemoveAssignHandler (POST {character_id}) removes a char's assignment entirely
// (D-09). Officer-only at the route; authorized in-tx. Idempotent: a missing assignment
// → 200 {"removed":false}. Audits "assignment_remove" ONLY on a real removal.
func OfficerRemoveAssignHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req adminCharReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var removed bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			removed, e = store.RemoveAssignTx(ctx, tx, req.CharacterID, callerID)
			if e != nil {
				return e
			}
			if removed {
				return AppendAuditTx(ctx, tx, "assignment_remove", callerID,
					map[string]any{"character_id": req.CharacterID}, now)
			}
			return nil
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"removed": removed})
	}
}

// adminRequestReq is the {request_id} body for approve / deny. The actor is the session
// caller.
type adminRequestReq struct {
	RequestID int64 `json:"request_id"`
}

// ApproveRequestHandler (POST {request_id}) approves a pending request: reassigns the
// char to the requester AND denies all sibling pending requests for that char, in the
// store tx (Pitfall 3). Officer-only at the route; authorized in-tx. A missing/non-
// pending request → 200 {"approved":false} (no assignment write). Audits
// "request_approve" ONLY on a real approval.
func ApproveRequestHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req adminRequestReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RequestID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var approved bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			approved, e = store.ApproveRequestTx(ctx, tx, req.RequestID, callerID, now)
			if e != nil {
				return e
			}
			if approved {
				return AppendAuditTx(ctx, tx, "request_approve", callerID,
					map[string]any{"request_id": req.RequestID}, now)
			}
			return nil
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"approved": approved})
	}
}

// DenyRequestHandler (POST {request_id}) marks a pending request denied (D-09). Officer-
// only at the route; authorized in-tx. A missing/non-pending request → 200
// {"denied":false}. Audits "request_deny" ONLY on a real deny.
func DenyRequestHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req adminRequestReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RequestID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var denied bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			denied, e = store.DenyRequestTx(ctx, tx, req.RequestID, callerID, now)
			if e != nil {
				return e
			}
			if denied {
				return AppendAuditTx(ctx, tx, "request_deny", callerID,
					map[string]any{"request_id": req.RequestID}, now)
			}
			return nil
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"denied": denied})
	}
}

// designateReq is the {character_id, mode} body for the officer designate endpoint. mode
// ∈ {bank, bot, none}. The actor is the session caller.
type designateReq struct {
	CharacterID int64  `json:"character_id"`
	Mode        string `json:"mode"`
}

// designateModes maps the allow-listed mode string to the store's DesignateMode (the
// allow-list IS the validation: any other value → 400 invalid_input).
var designateModes = map[string]store.DesignateMode{
	"none": store.DesignateNeither,
	"bank": store.DesignateBank,
	"bot":  store.DesignateBot,
}

// DesignateCharHandler (POST {character_id, mode}) sets a char's guild-bank/bot
// designation (D-09, OPEN-3 — officer-only). mode ∈ {bank,bot,none} (allow-listed → 400
// otherwise). Officer-only at the route; authorized in-tx. Designating bank/bot makes
// the char SHARED → the store clears any assignment + denies pending requests in the
// same tx (Pitfall 6). A missing/removed char → ErrCharNotFound → 400 invalid_input.
// Audits "char_designate" with {character_id}.
func DesignateCharHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req designateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		mode, ok := designateModes[req.Mode]
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.DesignateCharTx(ctx, tx, req.CharacterID, mode, callerID, now); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "char_designate", callerID,
				map[string]any{"character_id": req.CharacterID}, now)
		})
		if err != nil {
			// A missing/removed char is a client error (the char must exist + be live).
			if errors.Is(err, store.ErrCharNotFound) {
				writeJSONError(w, http.StatusBadRequest, "invalid_input")
				return
			}
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"designated": true})
	}
}
