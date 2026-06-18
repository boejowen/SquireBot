package readapi

// inventory.go serves GET /api/v1/inventory/{char} — one character's structured
// in-game inventory window (CHAR-03 / INV-01..04), the new Phase 31 read surface.
// It is the views.go handler shape with two deltas: it reads the {char} path
// wildcard (Go 1.22+ ServeMux) and dispatches to compute.StructuredInventory
// (Phase 29 + the Plan 31-01 icon_id / last_seen carry-through) instead of the
// view switch. RequireSession-gated at registration (login-only since P15 — NOT
// public, NOT officer); the gate is the membership boundary, not per-character
// ownership (every member may view any character — the consolidated-views model).
//
// SECURITY:
//   - V4 / D-11 empty-not-404: an unknown char yields an empty CharacterInventory
//     (InventoryForChar's WHERE c.name = ? returns zero rows → empty slices), NOT a
//     404 — so the client renders "no inventory synced yet".
//   - V5 / T-31-06: the {char} value is passed BY VALUE to StructuredInventory →
//     InventoryForChar's `?` bind; the handler builds NO SQL and never
//     string-concats it.
//   - V7 / T-31-08: slog carries op + row count + status + err ONLY — never the
//     char value or any item/row content.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// InventoryHandler serves GET /api/v1/inventory/{char}. It holds the read-side
// *store.Store (compute.StructuredInventory takes it). Construct once at startup
// and register UNDER webauth.RequireSession:
//
//	mux.Handle("GET /api/v1/inventory/{char}", webauth.RequireSession(db, readapi.NewInventory(st)))
type InventoryHandler struct {
	store *store.Store
}

// NewInventory builds the inventory handler from the read-side store.
func NewInventory(s *store.Store) *InventoryHandler {
	return &InventoryHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise). It reads the
// {char} path wildcard, computes that character's structured inventory, coerces
// the three slot slices nil→[] so the JSON is always arrays, and encodes the
// CharacterInventory (which carries the per-slot icon_id + the per-character
// last_seen from Plan 31-01).
func (h *InventoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	char := r.PathValue("char") // Go 1.22+ ServeMux {char} wildcard; only a `?` bind downstream (V5)

	inv, err := compute.StructuredInventory(ctx, h.store, char)
	if err != nil {
		// V7: op + err only — NEVER the char value or row content.
		slog.Error("inventory read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// nil→[] coercion (views.go discipline) so the three arrays are always JSON
	// arrays, never null — a stable shape the client iterates without a nil-guard.
	// An unknown char comes through here with all three nil → all become [] (the
	// V4/D-11 empty-not-404 contract: a 200 with empty arrays, never a 404).
	if inv.Equipment == nil {
		inv.Equipment = []compute.InventorySlot{}
	}
	if inv.General == nil {
		inv.General = []compute.InventorySlot{}
	}
	if inv.Bank == nil {
		inv.Bank = []compute.InventorySlot{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(inv); err != nil {
		// 200 header already flushed; log the encode failure (op + err, never content).
		slog.Error("inventory encode failed", "err", err)
		return
	}
	// V7: row count + status only — never the char name.
	slog.Info("inventory ok",
		"rows", len(inv.Equipment)+len(inv.General)+len(inv.Bank),
		"status", http.StatusOK)
}
