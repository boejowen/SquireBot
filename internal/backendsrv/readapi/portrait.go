package readapi

// portrait.go serves GET /api/v1/characters/{name}/portrait — the raw-byte portrait
// stream (Phase 41 / CHARUI-02, plan 41-01). It is the inventory.go request-side scaffold
// (GET-only 405, {name} path wildcard, RequireSession-gated at registration, V7 logging)
// but DIVERGES on the response side: instead of json.NewEncoder it writes the stored image
// bytes directly — the ONLY raw-byte response handler in the API.
//
// SECURITY (D-04 serve hardening):
//   - Content-Type is the STORED sniffed value (image/png|jpeg|webp), NEVER a client claim —
//     the webadmin upload sniffed the magic bytes and persisted the type; this replays it.
//   - X-Content-Type-Options: nosniff so the browser cannot re-interpret the blob as
//     HTML/script (defense against a hypothetical mislabeled blob).
//   - RequireSession at the route already returns 401 for no-session; an absent portrait (or
//     unknown char) → 404, preserving the 404-vs-401 discipline (never an existence leak).
//   - Cache-Control: private, max-age=300 — the ?v=updated_at cache-bust busts it on a new
//     upload, so a short private cache is safe.
//   - V5: {name} binds only as a `?` inside GetPortrait. V7: slog carries byte-count + status
//     + err ONLY — never the char name or the blob content.

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// PortraitHandler serves GET /api/v1/characters/{name}/portrait. It holds the read-side
// *store.Store (GetPortrait streams the blob). Construct once at startup and register UNDER
// webauth.RequireSession:
//
//	mux.Handle("GET /api/v1/characters/{name}/portrait", webauth.RequireSession(db, readapi.NewPortrait(st)))
type PortraitHandler struct {
	store *store.Store
}

// NewPortrait builds the portrait serve handler from the read-side store.
func NewPortrait(s *store.Store) *PortraitHandler {
	return &PortraitHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise). It reads the {name} path
// wildcard, streams that character's stored portrait bytes with the sniffed content_type +
// nosniff, and 404s a portrait-less/unknown char.
func (h *PortraitHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	name := r.PathValue("name") // Go 1.22+ ServeMux {name}; only a `?` bind downstream (V5)

	blob, ct, err := h.store.GetPortrait(ctx, name)
	if errors.Is(err, store.ErrPortraitNotFound) {
		// 404-vs-401 discipline: RequireSession already handled the no-session 401; an
		// absent portrait (or unknown char) is a plain not-found, never an existence leak.
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		// V7: op + err only — NEVER the char name or blob content.
		slog.Error("portrait read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", ct)                      // image/<sniffed>, from the STORED value (never a client claim)
	w.Header().Set("X-Content-Type-Options", "nosniff")     // D-04 serve hardening
	w.Header().Set("Cache-Control", "private, max-age=300")  // the ?v=updated_at busts it on a new upload
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob) // the ONLY raw-byte write in the API

	// V7: byte-count + status only — never the char name.
	slog.Info("portrait ok", "bytes", len(blob), "status", http.StatusOK)
}
