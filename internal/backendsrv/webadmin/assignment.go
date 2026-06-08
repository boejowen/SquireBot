package webadmin

// assignment.go is the MEMBER character-assignment endpoint backend (Phase 26 /
// ASSIGN-01/02/03/06) — the login-only twin of wantlist.go. Every signed-in guild
// member can list their assigned characters, list the claimable (unassigned, non-
// shared) characters, self-claim an unassigned char, release a char they hold,
// request a contested char (one already assigned to someone else), and cancel a
// pending request. The route layer wraps these in webauth.RequireSession (NOT
// RequireOfficer — these are every member's own actions, D-06/D-08).
//
// Security posture (the wantlist.go precedent, D-02 / Pitfall 1):
//   - The acting identity is ALWAYS the session caller (caller(ctx) =
//     webauth.UserFromContext). The request body carries ONLY character_id — there is
//     NO discord_user_id ACTOR field on any member endpoint, so a spoofed-identity
//     body is structurally ignored (a test asserts it). The store mutators take the
//     callerID the handler supplies from the session.
//   - Release / CancelRequest are owner-scoped in the store (ReleaseCharTx /
//     CancelRequestTx scope `AND discord_user_id=caller` / `AND requester=caller`): a
//     foreign-row mutation affects 0 rows → 200 {"released":false} / {"cancelled":false},
//     a silent IDOR no-op that never leaks the row's existence (T-26-12).
//   - Each mutation composes the store *Tx mutator + AppendAuditTx in ONE withTx
//     (BEGIN IMMEDIATE) — the write and its audit row land atomically (ASSIGN-06 /
//     T-26-13). The audit detail carries character_id ONLY (V7 — no PII).
//   - Typed store errors flow through mapAssignErr (ErrCharShared / ErrCharAlreadyAssigned
//     / ErrDuplicateRequest → 409, ErrNotAuthorized → 403). mapAssignErr is defined HERE
//     and REUSED by the officer handlers (assignment_admin.go) — it covers ErrCharShared
//     which mapOfficerErr does not.
//
// Shared helpers reused verbatim: caller / nowUnix / writeJSON / writeJSONError
// (officers.go), withTx / AppendAuditTx (audit.go). The Phase-26 store funcs
// (store.ClaimCharTx / ReleaseCharTx / RequestTx / CancelRequestTx / ListMyAssignments
// / ListClaimable) are from 26-01.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// assignmentReq is the body for every mutating member endpoint (claim / release /
// request / cancel). It carries ONLY character_id (Pitfall 1 / D-02) — NEVER a
// discord_user_id ACTOR field; the actor is the session caller(ctx). An extra
// is_bank_toon / discord_user_id field in the JSON is simply ignored by the decoder.
type assignmentReq struct {
	CharacterID int64 `json:"character_id"`
}

// mapAssignErr maps the assignment store's typed errors to the exact HTTP codes the
// frontend routes (the mapWantErr twin). It is the SINGLE error mapper for BOTH the
// member endpoints here AND the officer endpoints in assignment_admin.go — so it must
// cover ErrCharShared (a bank/bot-char assign → 409) AND ErrNotAuthorized (the in-tx
// officer re-check failed → 403). Do NOT route officer-assignment errors through
// mapOfficerErr: it lacks the ErrCharShared branch, so a bank/bot assign would fall
// through to a generic 500 instead of a clean 409 char_shared. Matched with errors.Is
// on the TYPED sentinel the store returns — never a string-match.
func mapAssignErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrCharAlreadyAssigned):
		writeJSONError(w, http.StatusConflict, "already_assigned")
	case errors.Is(err, store.ErrCharShared):
		writeJSONError(w, http.StatusConflict, "char_shared")
	case errors.Is(err, store.ErrDuplicateRequest):
		writeJSONError(w, http.StatusConflict, "duplicate_request")
	case errors.Is(err, store.ErrNotAuthorized):
		writeJSONError(w, http.StatusForbidden, "not_authorized")
	default:
		slog.Error("assignment handler", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}

// decodeAssignmentReq decodes the {character_id} body and rejects a non-positive id
// with 400 invalid_input (returning ok=false so the handler returns immediately). The
// body NEVER selects an actor — the caller is the session identity.
func decodeAssignmentReq(w http.ResponseWriter, r *http.Request) (assignmentReq, bool) {
	var req assignmentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_input")
		return assignmentReq{}, false
	}
	return req, true
}

// ListMyAssignmentsHandler (GET) returns the caller's assigned characters (ASSIGN-01,
// the "My characters" read), scoped to caller(ctx). The store returns nil for the
// empty state, which the handler normalizes to [] (never null) so the UI shows the
// no-chars state.
func ListMyAssignmentsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		out, err := store.ListMyAssignments(ctx, db, caller(ctx))
		if err != nil {
			slog.Error("list my assignments failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if out == nil {
			out = []store.Assignment{}
		}
		writeJSON(w, out)
	}
}

// ClaimableHandler (GET) returns the unassigned, non-shared, live characters any
// member may self-claim (ASSIGN-02). Not owner-scoped — claimable chars are guild-wide
// visible (the trust-rich-guild ethos). Empty → [] (never null).
func ClaimableHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		out, err := store.ListClaimable(r.Context(), db)
		if err != nil {
			slog.Error("list claimable failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if out == nil {
			out = []store.ClaimableChar{}
		}
		writeJSON(w, out)
	}
}

// ClaimCharHandler (POST {character_id}) self-claims an UNASSIGNED, non-shared char for
// the caller (ASSIGN-02 / D-06). The actor is the session caller (NEVER the body). The
// store rejects a bank/bot char (ErrCharShared → 409 char_shared) and an already-held
// char (ErrCharAlreadyAssigned → 409 already_assigned). Claim + audit run in ONE
// withTx. Audit: "assignment_claim" with character_id only (V7).
func ClaimCharHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		req, ok := decodeAssignmentReq(w, r)
		if !ok {
			return
		}
		callerID := caller(ctx) // Pitfall 1: actor from session, body carries NO actor.
		now := nowUnix()

		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.ClaimCharTx(ctx, tx, req.CharacterID, callerID, now); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "assignment_claim", callerID,
				map[string]any{"character_id": req.CharacterID}, now)
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"claimed": true})
	}
}

// ReleaseCharHandler (POST {character_id}) releases a char the caller holds (ASSIGN-03 /
// D-08), returning it to unassigned. Owner-scoped (store.ReleaseCharTx): a char held by
// another member affects 0 rows → 200 {"released":false}, a silent IDOR no-op that
// never leaks the row's existence (T-26-12). Audits "assignment_release" ONLY on a real
// release (an idempotent no-op need not spam the log — the officers.go precedent),
// detail carrying character_id only (V7).
func ReleaseCharHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		req, ok := decodeAssignmentReq(w, r)
		if !ok {
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var released bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			released, e = store.ReleaseCharTx(ctx, tx, req.CharacterID, callerID)
			if e != nil {
				return e
			}
			if released {
				return AppendAuditTx(ctx, tx, "assignment_release", callerID,
					map[string]any{"character_id": req.CharacterID}, now)
			}
			return nil
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"released": released})
	}
}

// RequestCharHandler (POST {character_id}) files a pending request for a contested char
// (ASSIGN-03 / D-07: a char already assigned to someone else). The actor is the session
// caller. A second pending request from the same member for the same char collides on
// the partial-unique pending index → ErrDuplicateRequest → 409 duplicate_request. A
// bank/bot char is not requestable (ErrCharShared → 409). Request + audit in ONE withTx.
// Audit: "assignment_request" with character_id only (V7).
func RequestCharHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		req, ok := decodeAssignmentReq(w, r)
		if !ok {
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.RequestTx(ctx, tx, req.CharacterID, callerID, now); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "assignment_request", callerID,
				map[string]any{"character_id": req.CharacterID}, now)
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"requested": true})
	}
}

// CancelRequestHandler (POST {character_id}) cancels the caller's OWN pending request
// for a char (D-07). Requester-scoped (store.CancelRequestTx): a foreign or non-pending
// request affects 0 rows → 200 {"cancelled":false}, a silent no-op. Audits
// "request_cancel" ONLY on a real cancel, detail carrying character_id only (V7).
func CancelRequestHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		req, ok := decodeAssignmentReq(w, r)
		if !ok {
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var cancelled bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			cancelled, e = store.CancelRequestTx(ctx, tx, req.CharacterID, callerID, now)
			if e != nil {
				return e
			}
			if cancelled {
				return AppendAuditTx(ctx, tx, "request_cancel", callerID,
					map[string]any{"character_id": req.CharacterID}, now)
			}
			return nil
		})
		if err != nil {
			mapAssignErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"cancelled": cancelled})
	}
}
