package readapi

// meta.go serves GET /api/v1/meta — the small shell feed the SvelteKit client
// fetches to populate the character list + per-character last-synced timestamp
// (D-01). It is the same read-only, public, no-guard shape as the view handlers
// (D-04), mirroring whoami.go minus the bearer guard. The available-themes list
// is a compile-time client constant (RESEARCH), so this endpoint carries only the
// character/freshness data.
//
// The store's CharFreshness struct has no JSON tags, so meta.go maps it into a
// LOCAL typed response (MetaResponse{characters:[{name,last_seen}]}) that pins the
// snake_case field names — this is the FIXED meta contract Plan 04 consumes.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// metaChar is one character entry in the /api/v1/meta payload: the character name
// + its last-synced ISO timestamp ("" when never seen). snake_case JSON tags pin
// the contract for the client.
type metaChar struct {
	Name     string `json:"name"`
	LastSeen string `json:"last_seen"`
}

// MetaResponse is the /api/v1/meta payload: the list of non-removed characters
// with their freshness. Shape: {"characters":[{"name":...,"last_seen":...}, ...]}.
type MetaResponse struct {
	Characters []metaChar `json:"characters"`
}

// MetaHandler serves GET /api/v1/meta. It holds the read-side *store.Store and,
// like the view handlers, has NO bearer guard (D-04 public read). Construct it
// once at startup: mux.Handle("GET /api/v1/meta", readapi.NewMeta(st)).
type MetaHandler struct {
	store *store.Store
}

// NewMeta builds the meta handler from the read-side store.
func NewMeta(s *store.Store) *MetaHandler {
	return &MetaHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise), read-only, no
// guard. It reads the character freshness list from the store and encodes it as
// the MetaResponse shape.
func (h *MetaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// >>> No bearer-guard / 401 block (D-04 public read). <<<

	rows, err := h.store.CharFreshness(r.Context())
	if err != nil {
		// V7: op + err only — never the character names.
		slog.Error("meta read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Map the store rows into the snake_case contract shape. Pre-size the slice so
	// an empty result encodes as [] (not null) — a stable shape for the client.
	chars := make([]metaChar, 0, len(rows))
	for _, c := range rows {
		chars = append(chars, metaChar{Name: c.Name, LastSeen: c.LastSeen})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(MetaResponse{Characters: chars}); err != nil {
		slog.Error("meta encode failed", "err", err)
		return
	}
	slog.Info("meta ok", "characters", len(chars), "status", http.StatusOK)
}
