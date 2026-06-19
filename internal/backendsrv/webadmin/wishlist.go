package webadmin

// wishlist.go is the per-character / per-slot wishlist write API (Phase 34,
// WISH-02/03/05) — the owner-scoped twin of the retired wantlist.go, line-for-line.
// Every handler derives the acting owner from the Discord SESSION
// (caller(ctx) = webauth.UserFromContext, D-02); the request body NEVER carries an
// owner/discord field. The route layer wraps these in webauth.RequireSession (NOT
// RequireOfficer — every signed-in member manages their OWN wishlist).
//
// It reuses the shared webadmin helpers verbatim (caller / nowUnix / writeJSON /
// writeJSONError from officers.go, withTx / AppendAuditTx from audit.go) and the
// Phase-34 store funcs (store.AddWishlistTx / RemoveOwnWishlistTx / SetPingedTx).
//
// Security posture (the load-bearing parts — carried over verbatim from wantlist):
//   - Owner is session-derived (D-02); no body field selects an owner.
//   - character_id is REQUIRED (every target is char+slot-scoped) and UNTRUSTED:
//     AddWishlistHandler authorizes it UNDER the SAME tx via store.IsCharAssignedToTx
//     BEFORE the insert (T-34-07/T-28-05 IDOR) → store.ErrCharNotAssigned → 403, so a
//     member can NEVER tag a wishlist target to a character not assigned to them.
//   - validWishlist re-validates the slot ∈ the 21 canonical worn slots (V5) + a
//     non-blank ≤200-rune item label for a typed/custom target — never trusting the
//     client. An unknown slot → 400.
//   - Remove/ping are owner-scoped (no IDOR); a cross-owner mutation is a silent
//     no-op (removed:false / pinged echoed) that never leaks the row's existence.
//   - ErrDuplicateWishlist (the exact-duplicate guard) → 409 {"error":"duplicate"}
//     via mapWishlistErr, matched with errors.Is on the TYPED sentinel — NOT a
//     string-match of the driver's textual message.
//   - The audit detail carries item_id/character_id/slot/want_id (IDS ONLY) — never
//     the item label or character name (V7).

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

// wishlistSlots is the server-side V5 allow-list of the 21 canonical worn slots
// (Charm/Power Source omitted — post-Velious, never hold items; the P31 paperdoll
// taxonomy). Declared as a literal HERE (the validWant precedent) rather than
// importing compute, to avoid a webadmin→compute dependency edge. The vocabulary
// mirrors compute.slotconst's Title-case worn-slot tokens.
var wishlistSlots = map[string]bool{
	"Head": true, "Face": true, "Ear1": true, "Ear2": true, "Neck": true,
	"Shoulders": true, "Arms": true, "Back": true, "Wrist1": true, "Wrist2": true,
	"Hands": true, "Finger1": true, "Finger2": true, "Chest": true, "Legs": true,
	"Feet": true, "Waist": true, "Primary": true, "Secondary": true, "Range": true,
	"Ammo": true,
}

// addWishlistReq is the POST body for adding a wishlist target. character_id is
// REQUIRED (not a pointer) — every target is char+slot-scoped. ItemID is a pointer
// so a typed/custom or gear-tier target (item_id absent/null) is distinguishable
// from a catalog target. The body carries NO owner field (D-02).
type addWishlistReq struct {
	CharacterID int64  `json:"character_id"`
	Slot        string `json:"slot"`
	ItemID      *int64 `json:"item_id"`
	ItemName    string `json:"item_name"`
}

// validWishlist is the server-side V5 re-check (NEVER trust the client form). The
// character_id must be a positive id (AUTHORIZATION — is it assigned to the caller?
// — is the in-tx guard in AddWishlistHandler, NOT here); the slot must be a known
// canonical worn slot (an unknown slot → 400); a typed/custom target (ItemID nil)
// requires a non-blank trimmed label, capped at 200 RUNES (utf8.RuneCountInString,
// not bytes — the validWant precedent; trimmed first so a row of spaces is empty).
func validWishlist(req addWishlistReq) bool {
	if req.CharacterID <= 0 {
		return false
	}
	if !wishlistSlots[req.Slot] {
		return false
	}
	name := strings.TrimSpace(req.ItemName)
	if req.ItemID == nil && name == "" {
		// A typed/custom target (no catalog id) needs a non-blank label.
		return false
	}
	if utf8.RuneCountInString(name) > 200 {
		return false
	}
	return true
}

// mapWishlistErr maps the wishlist store's typed errors to the exact HTTP codes the
// frontend routes (the mapWantErr twin). ErrDuplicateWishlist (the exact re-add of
// an active (user,char,slot,item) target) → 409 {"error":"duplicate"};
// ErrCharNotAssigned (T-34-07: a target tagged to a non-owned character) → 403;
// anything else → 500. The match is errors.Is on the TYPED sentinel — NOT a
// string-match of the driver's textual message.
func mapWishlistErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrDuplicateWishlist):
		writeJSONError(w, http.StatusConflict, "duplicate")
	case errors.Is(err, store.ErrCharNotAssigned):
		writeJSONError(w, http.StatusForbidden, "char_not_assigned")
	default:
		slog.Error("wishlist write failed", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}

// AddWishlistHandler (POST) adds a per-slot upgrade target for the caller
// (WISH-02/03). The owner is the session caller (caller(ctx), D-02) — the body
// carries NO owner. The add + audit run in ONE withTx (BEGIN IMMEDIATE) so they are
// atomic. The character tag is ALWAYS authorized in-tx BEFORE the insert (T-34-07
// IDOR) — a character_id in the (untrusted) body that is not assigned to the caller
// → store.ErrCharNotAssigned → 403; the rollback leaves no row and no audit. On an
// exact duplicate the store returns ErrDuplicateWishlist → 409. The audit detail
// carries IDs/slot ONLY — never the item label (V7).
func AddWishlistHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req addWishlistReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !validWishlist(req) {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		itemName := strings.TrimSpace(req.ItemName)
		callerID := caller(r.Context()) // D-02: owner from session, body carries NO owner
		now := nowUnix()

		var newID int64
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			// T-34-07 IDOR guard: AUTHORIZE the character tag UNDER the SAME tx BEFORE the
			// insert (TOCTOU-safe). For the wishlist character_id is REQUIRED, so the guard
			// ALWAYS runs. A character_id not assigned to the caller → ErrCharNotAssigned →
			// mapWishlistErr → 403; the rollback leaves no row and no audit.
			ok, e := store.IsCharAssignedToTx(ctx, tx, req.CharacterID, callerID)
			if e != nil {
				return e
			}
			if !ok {
				return store.ErrCharNotAssigned
			}
			id, e := store.AddWishlistTx(ctx, tx, callerID, req.CharacterID, req.Slot, req.ItemID, itemName, now)
			if e != nil {
				return e // ErrDuplicateWishlist → mapWishlistErr → 409 duplicate
			}
			newID = id
			// V7: detail carries item_id + character_id + slot (IDS ONLY) — never the item
			// label, never the character name.
			return AppendAuditTx(ctx, tx, "wishlist_add", callerID, map[string]any{
				"item_id": req.ItemID, "character_id": req.CharacterID, "slot": req.Slot,
			}, now)
		})
		if err != nil {
			mapWishlistErr(w, err)
			return
		}
		// Echo the created row, built from the inputs + the returned id.
		writeJSON(w, store.WishlistTargetRow{
			ID:          newID,
			ItemID:      req.ItemID,
			ItemName:    itemName,
			Slot:        req.Slot,
			CharacterID: req.CharacterID,
			Pinged:      true, // default-ON (Pitfall 8)
			CreatedAt:   now,
		})
	}
}

// removeWishlistReq is the {id} body for remove — the wishlist target id only. The
// owner is NEVER from the body (D-02); it is the session caller.
type removeWishlistReq struct {
	ID int64 `json:"id"`
}

// RemoveOwnWishlistHandler (POST) soft-removes a single one of the caller's OWN
// wishlist targets (the RemoveOwnWantHandler twin). The body carries the target id
// only; the owner is the session caller. The remove is owner-scoped
// (store.RemoveOwnWishlistTx — T-34-07): a target belonging to a different member is
// a silent no-op (removed:false) that never leaks its existence. Audits
// "wishlist_remove" ONLY on a real remove, detail carrying want_id only (V7).
func RemoveOwnWishlistHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req removeWishlistReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(r.Context())
		now := nowUnix()

		var removed bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			removed, e = store.RemoveOwnWishlistTx(ctx, tx, req.ID, callerID)
			if e != nil {
				return e
			}
			if removed {
				// V7: detail carries want_id ONLY.
				return AppendAuditTx(ctx, tx, "wishlist_remove", callerID, map[string]any{"want_id": req.ID}, now)
			}
			return nil
		})
		if err != nil {
			mapWishlistErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"removed": removed})
	}
}

// pingReq is the {id, pinged} body for the per-target ping toggle (WISH-05): the
// target id + the desired ping state. The owner is NEVER from the body (D-02).
type pingReq struct {
	ID     int64 `json:"id"`
	Pinged bool  `json:"pinged"`
}

// SetWishlistPingHandler (POST) toggles a single one of the caller's OWN wishlist
// targets' ping flag (WISH-05 "ping me / stop pinging me about THIS upgrade") — the
// MuteWantHandler twin (inverted polarity: pinged default-ON). The body carries the
// target id + the desired state only; the owner is the session caller. The toggle is
// owner-scoped (store.SetPingedTx — T-34-07): a target belonging to a different
// member is a silent no-op that never leaks its existence. The response echoes the
// REQUESTED pinged state (mirroring the silent-no-op contract). Audits
// "wishlist_ping" with want_id ONLY (V7) when a row actually flipped.
func SetWishlistPingHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req pingReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(r.Context())
		now := nowUnix()

		var ok bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			var e error
			ok, e = store.SetPingedTx(ctx, tx, req.ID, callerID, req.Pinged)
			if e != nil {
				return e
			}
			if ok {
				// V7: detail carries want_id ONLY.
				return AppendAuditTx(ctx, tx, "wishlist_ping", callerID, map[string]any{"want_id": req.ID}, now)
			}
			return nil
		})
		if err != nil {
			mapWishlistErr(w, err)
			return
		}
		// Echo the requested state (the silent-no-op contract: a cross-owner id returns
		// pinged as requested but flipped nothing).
		writeJSON(w, map[string]any{"pinged": req.Pinged})
	}
}
