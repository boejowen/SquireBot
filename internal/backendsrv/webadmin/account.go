package webadmin

// account.go is the self-service watcher-linking endpoint backend (Phase 17 /
// LINK-01/03/05) — the login-only sibling of the officer-only officers.go. Every
// handler derives the acting owner from the Discord SESSION (caller(ctx) =
// webauth.UserFromContext, D-02); the request body NEVER carries an owner/label.
// The route layer wraps these in webauth.RequireSession (NOT RequireOfficer — D-09,
// every signed-in member, not just officers).
//
// It reuses the shared webadmin helpers verbatim (caller / nowUnix / writeJSON /
// writeJSONError from officers.go, withTx / AppendAuditTx from audit.go) and the
// Phase-17 store funcs (store.ResolveOrCreateOwnerByDiscordTx / ListOwnCodes /
// RevokeOwnCodeTx) + auth.MintCodeForOwnerTx.
//
// Security posture:
//   - Owner is session-derived (D-02); no body field selects an owner.
//   - Mint returns the plaintext in the HTTP body EXACTLY ONCE; it is NEVER slog'd
//     and the audit detail carries owner_id/code_id ONLY, never the token (V6/V7).
//   - List/revoke are owner-scoped (no IDOR); a cross-owner revoke is a silent
//     no-op (revoked:false) that never leaks the code's existence (Pitfall 3).
//   - ErrAmbiguousOwner (the D-04 mis-adoption guard) → 409 via mapAccountErr.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// MintOwnCodeHandler (POST) mints a fresh self-service watcher code for the caller
// (LINK-01/03). The owner is derived server-side from the Discord session
// (resolve-or-create, D-02/D-03/D-04); the body carries NO owner. Minting is
// ADDITIVE — a new code never revokes existing ones (LINK-03). The resolve + mint +
// audit run in ONE withTx (BEGIN IMMEDIATE) so they are atomic; the plaintext is
// returned in the body exactly once and is NEVER logged (V7). The audit detail
// carries owner_id ONLY — never the token.
func MintOwnCodeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(ctx) // D-02: owner from session, request body carries NO owner
		now := nowUnix()

		var plaintext string
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			ownerID, derr := store.ResolveOrCreateOwnerByDiscordTx(ctx, tx, callerID)
			if derr != nil {
				return derr // ErrAmbiguousOwner → mapAccountErr → 409 (D-04 refuse)
			}
			var merr error
			plaintext, merr = auth.MintCodeForOwnerTx(ctx, tx, ownerID)
			if merr != nil {
				return merr
			}
			// V7: detail carries owner_id ONLY — never the token/code.
			return AppendAuditTx(ctx, tx, "code_mint", callerID, map[string]any{"owner_id": ownerID}, now)
		})
		if err != nil {
			mapAccountErr(w, err)
			return
		}
		// The plaintext crosses to the page EXACTLY ONCE, in the body, never logged.
		writeJSON(w, map[string]any{"code": plaintext})
	}
}

// ownCodeJSON is one row of the caller's active-code list. ordinal is the 1-based
// #N over the (created_at-ordered) active set (D-06 — not a stored column).
// last_seen is null until the code's watcher next uploads ("never used yet").
type ownCodeJSON struct {
	ID        int64   `json:"id"`
	Ordinal   int     `json:"ordinal"`
	CreatedAt string  `json:"created_at"`
	LastSeen  *string `json:"last_seen"`
}

// ListOwnCodesHandler (GET) returns the caller's OWN active codes (LINK-05),
// owner-scoped and ordered for a stable #N. A caller who has never minted (no owner
// row stamped with their discord_user_id) gets [] — not an error. The JSON is
// always a non-nil array so the empty state keys off [] not null.
func ListOwnCodesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(ctx)

		ownerID, found, err := ownerIDForCaller(ctx, db, callerID)
		if err != nil {
			slog.Error("list own codes: resolve owner failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		out := make([]ownCodeJSON, 0) // non-nil → JSON []
		if found {
			codes, err := store.ListOwnCodes(ctx, db, ownerID)
			if err != nil {
				slog.Error("list own codes failed", "err", err)
				writeJSONError(w, http.StatusInternalServerError, "internal")
				return
			}
			for i, c := range codes {
				row := ownCodeJSON{ID: c.ID, Ordinal: i + 1, CreatedAt: c.CreatedAt}
				if c.LastSeen.Valid {
					v := c.LastSeen.String
					row.LastSeen = &v
				}
				out = append(out, row)
			}
		}
		writeJSON(w, out)
	}
}

// revokeReq is the {id} body for revoke — the CODE id only. The owner is NEVER from
// the body (D-02); it is resolved from the caller's session.
type revokeReq struct {
	ID int64 `json:"id"`
}

// RevokeOwnCodeHandler (POST) revokes a single one of the caller's OWN codes
// (LINK-05 / D-08). The body carries the code id only; the owner is resolved from
// the session. The revoke is owner-scoped (store.RevokeOwnCodeTx — Pitfall 3): a
// code belonging to a different owner is a silent no-op (revoked:false) that never
// leaks its existence and never touches another owner's data. Audits "code_revoke"
// ONLY on a real revoke (idempotent no-op need not spam the log — officers.go:182).
func RevokeOwnCodeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req revokeReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		ownerID, found, err := ownerIDForCaller(ctx, db, callerID)
		if err != nil {
			slog.Error("revoke own code: resolve owner failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if !found {
			// The caller has never minted, so they own no codes — nothing to revoke.
			// Idempotent no-op (never leak existence of someone else's code).
			writeJSON(w, map[string]any{"revoked": false})
			return
		}

		var revoked bool
		err = withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			revoked, e = store.RevokeOwnCodeTx(ctx, tx, req.ID, ownerID)
			if e != nil {
				return e
			}
			if revoked {
				// V7: detail carries owner_id/code_id ONLY — never the token.
				return AppendAuditTx(ctx, tx, "code_revoke", callerID, map[string]any{
					"owner_id": ownerID,
					"code_id":  req.ID,
				}, now)
			}
			return nil
		})
		if err != nil {
			mapAccountErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"revoked": revoked})
	}
}

// ownerIDForCaller resolves the caller's owner id by the FK (no create — that only
// happens on mint). found=false means the caller has no owner stamped with their
// discord_user_id yet (never minted) → the list/revoke handlers treat it as the
// empty case, not an error.
func ownerIDForCaller(ctx context.Context, db *sql.DB, callerID string) (int64, bool, error) {
	var id int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM owner WHERE discord_user_id = ?`, callerID,
	).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, err
	}
	return id, true, nil
}

// mapAccountErr maps the account-handler typed errors to the exact HTTP codes the
// frontend routes (mirrors mapOfficerErr — officers.go:203). ErrAmbiguousOwner (the
// D-04 mis-adoption / ambiguity refuse) → 409 conflict; anything else → 500. The
// frontend routes off the {"error":"code"} shape.
func mapAccountErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrAmbiguousOwner):
		writeJSONError(w, http.StatusConflict, "ambiguous_owner")
	default:
		slog.Error("account write failed", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}
