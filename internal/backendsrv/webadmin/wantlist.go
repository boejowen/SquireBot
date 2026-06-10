package webadmin

// wantlist.go is the personal-wantlist endpoint backend (Phase 19 / WANT-01/02) —
// the login-only twin of account.go. Every handler derives the acting owner from
// the Discord SESSION (caller(ctx) = webauth.UserFromContext, D-02); the request
// body NEVER carries an owner/discord field. The route layer wraps these in
// webauth.RequireSession (NOT RequireOfficer — every signed-in member manages their
// OWN wantlist, not just officers).
//
// It reuses the shared webadmin helpers verbatim (caller / nowUnix / writeJSON /
// writeJSONError from officers.go, withTx / AppendAuditTx from audit.go) and the
// Phase-19 store funcs (store.AddWantTx / ListOwnWants / RemoveOwnWantTx).
//
// Security posture:
//   - Owner is session-derived (D-02); no body field selects an owner. There is NO
//     owner entity — identity keys directly on the caller's discord_user_id
//     (19-RESEARCH Pitfall 3); the handlers call NO resolve-or-create function.
//   - validWant re-validates the priority enum + the (TRIMMED) 280-rune note
//     cap + the non-blank custom label server-side — never trusting the client
//     <select>/<textarea> (the charmeta precedent). The note is TRIMMED before the
//     rune count so a whitespace-only / 280-spaces note is treated as empty (stored
//     NULL), never as a "real" 280-char note (review WORTH-FIX 6).
//   - List/remove are owner-scoped (no IDOR); a cross-owner remove is a silent no-op
//     (removed:false) that never leaks the row's existence (Pitfall 3).
//   - ErrDuplicateWant (the D-05 exact-duplicate guard) → 409 {"error":"duplicate"}
//     via mapWantErr, matched with errors.Is on the TYPED sentinel the store returns
//     (review MUST-FIX 2) — NOT by string-matching the driver's textual message.
//   - The audit detail carries item_id/want_id ONLY — never the note text (V7).

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// addWantReq is the POST body for adding a want. ItemID is a pointer so a custom
// want (item_id absent/null — D-04/D-07) is distinguishable from a catalog want.
// The body carries NO owner field (D-02) — the owner is the session caller.
type addWantReq struct {
	ItemID   *int64 `json:"item_id"`
	ItemName string `json:"item_name"`
	Priority string `json:"priority"`
	Note     string `json:"note"`
	// CharacterID is the OPTIONAL character tag (CWANT-01). A pointer (mirroring
	// ItemID's optional-pointer idiom) so an absent/null character_id is an
	// account-level want; a non-nil one tags the want to that character. The body is
	// UNTRUSTED — the tag is authorized server-side via store.IsCharAssignedToTx inside
	// AddWantHandler's withTx (NEVER here), so a member cannot tag a want to a character
	// not assigned to them (T-28-05 IDOR).
	CharacterID *int64 `json:"character_id"`
}

// validPriorities is the server-side priority enum allow-list (the DB CHECK
// constraint from Plan 01 is the second line of defense; this is the first).
// NB: there is no reason allow-list any more — the buy/quest reason field was
// removed end-to-end (quick-260610-fm5); a stale client's "reason" key is
// silently ignored by Decode (no DisallowUnknownFields).
var validPriorities = map[string]bool{"low": true, "med": true, "high": true}

// validWant is the server-side V5 re-check (NEVER trust the form's <select>/
// <textarea> — the validCharMeta precedent). If Priority is
// non-empty it must be ∈ {low,med,high} (an empty Priority is allowed and defaults
// to "med" before the store call); the note is TRIMMED FIRST, then capped at 280
// RUNES (utf8.RuneCountInString, NOT len bytes — Pitfall 2; trimming first means
// 280 spaces does NOT pass as a real note — review WORTH-FIX 6); a custom want
// (ItemID nil) requires a non-blank trimmed label.
func validWant(req addWantReq) bool {
	if req.Priority != "" && !validPriorities[req.Priority] {
		return false
	}
	// Trim BEFORE the rune count so a whitespace-only / 280-space note is measured
	// as empty, never as a 280-char note (review WORTH-FIX 6). Runes, not bytes
	// (Pitfall 2) — a multi-byte note must be counted by code point.
	if utf8.RuneCountInString(strings.TrimSpace(req.Note)) > 280 {
		return false
	}
	if req.ItemID == nil && strings.TrimSpace(req.ItemName) == "" {
		// A custom want (no catalog id) needs a non-blank label (D-04).
		return false
	}
	// V5 shape check: a non-nil character_id must be a positive id. AUTHORIZATION
	// (is it assigned to the caller?) is NOT done here — it is the in-tx guard in
	// AddWantHandler (store.IsCharAssignedToTx, T-28-05). This is purely a malformed-id
	// reject so a 0/negative tag is a 400, not a silent FK miss.
	if req.CharacterID != nil && *req.CharacterID <= 0 {
		return false
	}
	return true
}

// mapWantErr maps the wantlist store's typed errors to the exact HTTP codes the
// frontend routes (the mapAccountErr twin). ErrDuplicateWant (the D-05 exact
// re-add) → 409 {"error":"duplicate"}; anything else → 500. The match is errors.Is
// on the TYPED sentinel the store returns (review MUST-FIX 2) — NOT a string-match
// of the driver's textual message (which would couple to its exact wording).
func mapWantErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrDuplicateWant):
		writeJSONError(w, http.StatusConflict, "duplicate")
	case errors.Is(err, store.ErrCharNotAssigned):
		// T-28-05: the caller tagged a want to a character NOT assigned to them. The
		// in-tx guard (store.IsCharAssignedToTx) returned the typed sentinel → 403.
		writeJSONError(w, http.StatusForbidden, "char_not_assigned")
	default:
		slog.Error("wantlist write failed", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}

// AddWantHandler (POST) adds a want for the caller (WANT-01/02). The owner is the
// session caller (caller(ctx), D-02) — the body carries NO owner. The add + audit
// run in ONE withTx (BEGIN IMMEDIATE) so they are atomic. On an exact duplicate
// (same item in the same character scope) the store returns ErrDuplicateWant,
// which mapWantErr maps to 409 {"error":"duplicate"}. The audit detail
// carries item_id ONLY — never the note text (V7).
//
// ACCEPTED TRADE-OFF (review JUDGMENT-CALL 8): for catalog wants (ItemID set) the
// handler STORES the client-supplied item_name snapshot rather than re-deriving the
// canonical name from pigparse_price by item_id. This is an integrity smell, NOT a
// vuln: the in-bank join keys on item_id (not the name) and the name renders via
// Svelte {} auto-escape (Plan 03), so a bogus snapshot can neither break the join
// nor inject markup. Re-derive server-side if Phase 20+ needs an authoritative name.
func AddWantHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req addWantReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !validWant(req) {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		// Default the priority when blank (validWant permits an empty Priority).
		priority := req.Priority
		if priority == "" {
			priority = "med"
		}
		// The TRIMMED note is the canonical note: a whitespace-only note is stored as
		// NULL (notePtr nil), never as a row of spaces (review WORTH-FIX 6).
		trimmedNote := strings.TrimSpace(req.Note)
		var notePtr *string
		if trimmedNote != "" {
			notePtr = &trimmedNote
		}
		itemName := strings.TrimSpace(req.ItemName)
		callerID := caller(r.Context()) // D-02: owner from session, body carries NO owner
		now := nowUnix()

		var newID int64
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			// T-28-05 IDOR guard: when a character tag is supplied, AUTHORIZE it UNDER the
			// SAME tx BEFORE the insert (TOCTOU-safe). A character_id in the (untrusted)
			// body that is not assigned to the caller → store.ErrCharNotAssigned →
			// mapWantErr → 403; the rollback leaves no row and no audit. An account-level
			// want (CharacterID nil) skips the guard entirely.
			if req.CharacterID != nil {
				ok, e := store.IsCharAssignedToTx(ctx, tx, *req.CharacterID, callerID)
				if e != nil {
					return e
				}
				if !ok {
					return store.ErrCharNotAssigned
				}
			}
			id, e := store.AddWantTx(ctx, tx, callerID, req.ItemID, itemName, priority, notePtr, req.CharacterID, now)
			if e != nil {
				return e // ErrDuplicateWant → mapWantErr → 409 duplicate
			}
			newID = id
			// V7: detail carries item_id + character_id (IDS ONLY) — never the note text,
			// never the character name.
			return AppendAuditTx(ctx, tx, "wantlist_add", callerID, map[string]any{"item_id": req.ItemID, "character_id": req.CharacterID}, now)
		})
		if err != nil {
			mapWantErr(w, err)
			return
		}
		// Echo the created row, built from the inputs + the returned id. Note uses the
		// trimmed pointer so the response matches what was stored (NULL ⇒ JSON null).
		writeJSON(w, store.WantlistRow{
			ID:        newID,
			ItemID:    req.ItemID,
			ItemName:  itemName,
			Priority:  priority,
			Note:      notePtr,
			CreatedAt: now,
		})
	}
}

// muteReq is the {id, muted} body for the per-want mute toggle (D-09): the want id
// + the new mute state. The owner is NEVER from the body (D-02); it is the session
// caller.
type muteReq struct {
	ID    int64 `json:"id"`
	Muted bool  `json:"muted"`
}

// MuteWantHandler (POST) toggles a single one of the caller's OWN wants' mute flag
// (D-09 "stop pinging me about THIS item") — the RemoveOwnWantHandler twin. The body
// carries the want id + the desired state only; the owner is the session caller. The
// toggle is owner-scoped (store.SetMutedTx — Pitfall 3): a want belonging to a
// different member is a silent no-op that never leaks its existence and changes
// nothing. The response echoes the REQUESTED muted state (mirroring removed:false —
// a cross-owner id returns muted as requested but the store flipped no row). Audits
// "wantlist_mute" with want_id ONLY (V7) when a row actually flipped.
func MuteWantHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req muteReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(r.Context())
		now := nowUnix()

		var ok bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			ok, e = store.SetMutedTx(ctx, tx, req.ID, callerID, req.Muted)
			if e != nil {
				return e
			}
			if ok {
				// V7: detail carries want_id ONLY.
				return AppendAuditTx(ctx, tx, "wantlist_mute", callerID, map[string]any{"want_id": req.ID}, now)
			}
			return nil
		})
		if err != nil {
			mapWantErr(w, err)
			return
		}
		// Echo the requested state (the silent-no-op contract: a cross-owner id
		// returns muted as requested but flipped nothing — mirrors removed:false).
		writeJSON(w, map[string]any{"muted": req.Muted})
	}
}

// ListOwnWantsHandler (GET) returns the caller's OWN active wants (WANT-01),
// owner-scoped to caller(ctx). The store returns a non-nil slice, so the JSON is
// always [] (never null) for the empty state.
func ListOwnWantsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(r.Context())
		out, err := store.ListOwnWants(ctx, db, callerID)
		if err != nil {
			slog.Error("list own wants failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, out) // non-nil from the store → JSON []
	}
}

// ListGuildWantsHandler (GET) returns the GUILDWIDE wantlist — every active want
// across ALL members (CWANT-03/04, the "what does the guild want" read). Unlike
// ListOwnWantsHandler it is NOT caller-scoped: it calls store.ListGuildWants(ctx, db)
// with NO callerID. The store read JOINs the owner's username + LEFT JOINs the tagged
// character name and EXCLUDES the private note (T-28-07) — so a login-gated member can
// see the roll-up without leaking anyone's private "why you wanted it" note. The route
// wraps this in webauth.RequireSession (login-only, NOT RequireOfficer — the read API
// has been login-only since P15). The store returns a non-nil slice ⇒ JSON [] for empty.
func ListGuildWantsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		out, err := store.ListGuildWants(ctx, db) // guild-wide — NO caller scope
		if err != nil {
			slog.Error("list guild wants failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, out) // non-nil from the store → JSON []
	}
}

// removeWantReq is the {id} body for remove — the WANT id only. The owner is NEVER
// from the body (D-02); it is the session caller.
type removeWantReq struct {
	ID int64 `json:"id"`
}

// RemoveOwnWantHandler (POST) soft-removes a single one of the caller's OWN wants
// (the RevokeOwnCodeHandler twin). The body carries the want id only; the owner is
// the session caller. The remove is owner-scoped (store.RemoveOwnWantTx — Pitfall
// 3): a want belonging to a different member is a silent no-op (removed:false) that
// never leaks its existence. Audits "wantlist_remove" ONLY on a real remove (an
// idempotent no-op need not spam the log — the officers.go precedent), detail
// carrying want_id only (V7).
func RemoveOwnWantHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req removeWantReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(r.Context())
		now := nowUnix()

		var removed bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			removed, e = store.RemoveOwnWantTx(ctx, tx, req.ID, callerID)
			if e != nil {
				return e
			}
			if removed {
				// V7: detail carries want_id ONLY.
				return AppendAuditTx(ctx, tx, "wantlist_remove", callerID, map[string]any{"want_id": req.ID}, now)
			}
			return nil
		})
		if err != nil {
			mapWantErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"removed": removed})
	}
}
