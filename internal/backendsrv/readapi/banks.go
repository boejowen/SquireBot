package readapi

// banks.go serves GET /api/v1/banks — the guild-wide Banks tab valuation (BANK-01/02), the
// new Phase 33 read surface. Unlike items.go/characters.go it needs NO viewer id: banks are
// not viewer-scoped (D-01 is plain A-Z, every signed-in member sees the same guild banks), so
// this handler never reads the session identity from the request context. It returns the
// whole compute.BanksView object (the A-Z bank+bot rows + the guild summary), with the rows
// pre-coerced so an empty result encodes as `[]` not null.
//
// Session-gated at registration (login-only since P15 — never public, never an officer-scoped
// gate). The read is GUILD-WIDE: every signed-in member sees every bank; the gate is
// membership, not ownership (V4 / T-33-02). The route is wired ONLY under the session gate.
//
// SECURITY:
//   - V5: no user input server-side (no query params, no viewer id, no body); the bank-set
//     predicate downstream is a fixed-string WHERE (no SQL built here, no string-concat of
//     bank/item names — T-33-01).
//   - V7: slog carries op + row count + status + err ONLY — never a bank name, item name,
//     value, or platinum figure (T-33-04).

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// BanksHandler serves GET /api/v1/banks. It holds the read-side *store.Store; there is no
// viewer identity to read (banks are guild-wide, not viewer-scoped). Construct once at
// startup and register UNDER webauth.RequireSession:
//
//	mux.Handle("GET /api/v1/banks", webauth.RequireSession(db, readapi.NewBanks(st)))
type BanksHandler struct {
	store *store.Store
}

// NewBanks builds the Banks-tab valuation handler from the read-side store.
func NewBanks(s *store.Store) *BanksHandler {
	return &BanksHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise). It computes the widened
// bank+bot valuation and encodes the whole BanksView object (the A-Z rows + the guild
// summary), with the rows coerced to `[]` not null on empty.
func (h *BanksHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bv, err := compute.Banks(r.Context(), h.store)
	if err != nil {
		// V7: op + err only — never a bank/item name, value, or plat.
		slog.Error("banks read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if bv.Banks == nil {
		bv.Banks = []compute.BankRowSummary{} // [] not null — a stable client shape
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(bv); err != nil {
		slog.Error("banks encode failed", "err", err)
		return
	}
	slog.Info("banks ok", "rows", len(bv.Banks), "status", http.StatusOK) // V7: count + status only
}
