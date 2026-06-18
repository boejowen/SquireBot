package readapi

// items.go serves GET /api/v1/items — the guild-wide item-centric Inventory tab rollup
// (ITEM-01..03), the new Phase 32 read surface. Like characters.go it needs the VIEWER's
// discord_user_id (read from the request context that RequireSession injects) so the
// rollup can flag is_mine on items held by the viewer's characters (D-02/ITEM-02). It
// returns compute.ItemRollup directly (one row per normalized item name, with per-holder
// detail), pre-coerced so an empty result encodes as `[]` not null.
//
// RequireSession-gated at registration (login-only since P15 — NOT public, NOT officer).
// The rollup is a GUILD-WIDE read: every signed-in member sees every item + every holder;
// the gate is membership, not ownership — the list is ORDERED viewer-first (client-side)
// but never SCOPED to the viewer (V4 / T-32-05). NEVER RequireOfficer.
//
// DISTINCT from GET /api/v1/items/search (itemsearch.go, the P19 wantlist CATALOG search) —
// a separate Go 1.22+ ServeMux pattern; this handler does NOT reuse the P19 catalog-search
// constructor or store read (it serves guild HOLDINGS via compute.Items, not the catalog).
//
// SECURITY:
//   - V5: no user input server-side (no query params); the viewer id binds only as
//     RosterFor's single `?` placeholder downstream (no SQL built here, no string-concat
//     of item/char names — T-32-02).
//   - V7: slog carries op + row count + status + err ONLY — never an item or char name
//     (T-32-04).

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// ItemsHandler serves GET /api/v1/items. It holds the read-side *store.Store; the viewer
// identity comes from the RequireSession-populated context, so no *sql.DB is needed in the
// handler. Construct once at startup and register UNDER webauth.RequireSession:
//
//	mux.Handle("GET /api/v1/items", webauth.RequireSession(db, readapi.NewItems(st)))
type ItemsHandler struct {
	store *store.Store
}

// NewItems builds the item-rollup handler from the read-side store.
func NewItems(s *store.Store) *ItemsHandler {
	return &ItemsHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise). It reads the viewer's
// discord_user_id from context (RequireSession guarantees it in production; an absent id →
// "" → nothing flagged is_mine, still a valid guild-wide list), computes the item rollup,
// and encodes it as a JSON array.
func (h *ItemsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	uid, _ := webauth.UserFromContext(ctx) // viewer's discord_user_id; "" → nothing flagged is_mine

	rows, err := compute.Items(ctx, h.store, uid)
	if err != nil {
		// V7: op + err only — never an item/char name.
		slog.Error("items read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := rows
	if out == nil {
		out = []compute.ItemRollup{} // [] not null — a stable client shape
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		slog.Error("items encode failed", "err", err)
		return
	}
	slog.Info("items ok", "rows", len(out), "status", http.StatusOK) // V7: count + status only
}
