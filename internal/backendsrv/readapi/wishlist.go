package readapi

// wishlist.go serves GET /api/v1/wishlist/{char} — one character's per-slot upgrade
// wishlist (WISH-02/03/04), the new Phase 34 read surface. It is the inventory.go
// {char}-handler shape with one delta: it ALSO reads the session viewer id (like
// characters.go) because compute.WishlistFor needs BOTH the {char} path AND the
// caller's discord_user_id (the owner-scoped targets + the EC-hit badge set).
// RequireSession-gated at registration (login-only since P15 — NOT public, NOT
// officer); the gate is the membership boundary, not per-character ownership.
//
// SECURITY:
//   - V4 / D-11 empty-not-404: an unknown char flows through StructuredInventory →
//     empty equipped + (ListOwnWishlist JOINs character by name) zero targets → a
//     200 with the 21 empty slots, NOT a 404.
//   - V5 / T-34-09: the {char} value is passed BY VALUE to compute.WishlistFor →
//     the store reads' `?` binds; the handler builds NO SQL and never string-concats
//     the char or the uid.
//   - V7 / T-34-12: slog carries op + slot count + status + err ONLY — never the
//     char value, the uid, or any item/row content.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// WishlistHandler serves GET /api/v1/wishlist/{char}. It holds the read-side
// *store.Store (compute.WishlistFor takes it). Construct once at startup and
// register UNDER webauth.RequireSession:
//
//	mux.Handle("GET /api/v1/wishlist/{char}", webauth.RequireSession(db, readapi.NewWishlist(st)))
type WishlistHandler struct {
	store *store.Store
}

// NewWishlist builds the wishlist handler from the read-side store.
func NewWishlist(s *store.Store) *WishlistHandler {
	return &WishlistHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise). It reads the {char}
// path wildcard AND the session viewer id, computes that character's per-slot
// wishlist for the viewer, coerces the slot slices nil→[] so the JSON is always
// arrays, and encodes the WishlistView.
func (h *WishlistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	char := r.PathValue("char")           // Go 1.22+ {char} wildcard; only a `?` bind downstream (V5)
	uid, _ := webauth.UserFromContext(ctx) // viewer's discord_user_id; "" → no targets owned/flagged

	view, err := compute.WishlistFor(ctx, h.store, uid, char)
	if err != nil {
		// V7: op + err only — NEVER the char value, the uid, or row content.
		slog.Error("wishlist read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// nil→[] coercion so the arrays are always JSON arrays, never null — a stable
	// shape the client iterates without a nil-guard. WishlistFor builds the fixed
	// 21-slot list, so this is defensive; the inner Targets/Suggestions slices are
	// coerced per-slot.
	if view.Slots == nil {
		view.Slots = []compute.WishlistSlot{}
	}
	for i := range view.Slots {
		if view.Slots[i].Targets == nil {
			view.Slots[i].Targets = []compute.WishlistTarget{}
		}
		if view.Slots[i].Suggestions == nil {
			view.Slots[i].Suggestions = []compute.WishlistSuggestion{}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(view); err != nil {
		// 200 header already flushed; log the encode failure (op + err, never content).
		slog.Error("wishlist encode failed", "err", err)
		return
	}
	// V7: slot count + status only — never the char name or uid.
	slog.Info("wishlist ok", "slots", len(view.Slots), "status", http.StatusOK)
}
