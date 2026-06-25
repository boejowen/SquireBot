package readapi

// itemsearch.go is the Phase 19 D-10 full-catalog item-search HTTP handler — the
// views.go twin. It composes Plan 01's (*store.Store).SearchCatalog over
// pigparse_price (the FULL Blue catalog, NOT the guild-seen subset) behind a
// session-gated GET endpoint, so the wantlist add form (Plan 03) can pin a want by
// item_id from the authoritative catalog.
//
// SECURITY (V7 / DoS): a q shorter than 2 runes short-circuits to [] BEFORE any DB
// hit (the empty-query guard + the DoS mitigation for a full-scan LIKE — Pitfall
// A4); LIMIT 25 caps the result. The q string is NEVER logged — the slog records
// carry the op + result-count + qlen ONLY, mirroring views.go's "never the query
// param" discipline. An empty corpus degrades to [] (never a 500 — Pitfall A2).

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// searchLimit caps the result set (the DoS bound alongside the len-guard; the
// underlying query is a full-scan of pigparse_price — the index does not help).
const searchLimit = 25

// ItemSearch serves GET /api/v1/items/search. It holds the read-side *store.Store
// (SearchCatalog is a method on it); there is no write path. Construct one at
// server startup and register it behind webauth.RequireSession (login-only, like
// the view endpoints).
type ItemSearch struct {
	st *store.Store
}

// NewItemSearch builds the search handler bound to the read-side store.
func NewItemSearch(st *store.Store) *ItemSearch {
	return &ItemSearch{st: st}
}

// ServeHTTP implements http.Handler. GET-only (405 on anything else); a q shorter
// than 2 runes short-circuits to [] without touching the store; otherwise it calls
// SearchCatalog(q, LIMIT), coerces a nil result to [], and JSON-encodes. The q
// string is never logged (V7).
func (h *ItemSearch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) < 2 {
		// Empty-query guard + DoS mitigation: never hit the store for a tiny q.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]store.CatalogItem{})
		return
	}
	// Phase 39 facets (SEARCH-04/05): "1" encoding (Assumption A1); absent/other → false.
	// These are coerced to bool BEFORE the store call — no user string reaches SQL.
	clicky := r.URL.Query().Get("clicky") == "1"
	haste := r.URL.Query().Get("haste") == "1"
	items, err := h.st.SearchCatalog(r.Context(), q, clicky, haste, searchLimit)
	if err != nil {
		// V7: no q in the log — only the op + err.
		slog.Error("item search failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []store.CatalogItem{} // nil → [] coercion (views.go:87)
	}
	// V7: count + qlen + the facet booleans (NOT PII) only, NEVER the q value.
	slog.Info("item search ok", "rows", len(items), "qlen", utf8.RuneCountInString(q),
		"clicky", clicky, "haste", haste, "status", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		// Header (200) already flushed; log the encode failure (no content, no q).
		slog.Error("item search encode failed", "err", err)
	}
}
