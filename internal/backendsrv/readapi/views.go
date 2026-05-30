// Package readapi holds the P14 public read handlers — the HTTP surface half of
// BACKEND-05. It composes Plan 01's compute package (View/Bank/GearCheck/
// SpellCheck) and the store's CharFreshness feed behind versioned GET endpoints
// over the existing hand-rolled net/http ServeMux (D-10), and ships a stdlib CORS
// middleware (D-04) so the static SvelteKit site may read the responses.
//
// READ-ONLY, PUBLIC (D-04): these handlers mirror ingest/whoami.go EXACTLY except
// they DROP the bearer guard (the token-resolve / 401 block) — P14 read access is
// public; the Discord gate is P15 (AUTH-08). Every handler is GET-only (405 on
// anything else) and authors ZERO writes — only SELECT-backed compute/store reads.
//
// SECURITY (V7 / T-14.03-05): slog records carry op + view name + row count +
// status + err ONLY — NEVER row content (item/char names, summaries) and never a
// query param, mirroring whoami.go/handler.go. The compute row structs carry RAW
// wiki/user strings (escaping is the client's job per Plan 01's T-14.01-03); this
// is the data layer, not the escaping layer.
//
// The JSON each endpoint returns is the FIXED cross-plan contract: views encode
// the compute structs directly (snake_case tags already on them — compute/
// types.go), so Plan 04's Svelte client consumes those exact field names.
package readapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// ViewsHandler serves one of the four consolidated view endpoints, selected by
// the view field at construction time (so the 4 routes share one handler type,
// one per route). It holds the read-side *store.Store (the compute functions take
// it); there is NO bearer guard (D-04 public read) and NO raw *sql.DB write path.
//
// Construct one per route at server startup and register it on the ServeMux next
// to the ingest/whoami routes:
//
//	mux.Handle("GET /api/v1/views/view",        readapi.NewViews(st, "view"))
//	mux.Handle("GET /api/v1/views/gear_check",  readapi.NewViews(st, "gear_check"))
//	mux.Handle("GET /api/v1/views/spell_check", readapi.NewViews(st, "spell_check"))
//	mux.Handle("GET /api/v1/views/bank",        readapi.NewViews(st, "bank"))
type ViewsHandler struct {
	store *store.Store
	view  string
}

// NewViews builds a view handler bound to one of the four view names ("view",
// "gear_check", "spell_check", "bank"). The view string selects which compute
// function ServeHTTP dispatches to.
func NewViews(s *store.Store, view string) *ViewsHandler {
	return &ViewsHandler{store: s, view: view}
}

// ServeHTTP implements http.Handler. Method routing ("GET") is done by the
// ServeMux pattern in cmd/squirebot-server; this method still guards defensively
// (405 on anything else) so a direct/mis-registered call is safe and a non-GET is
// rejected without touching anything (read-only contract, T-14.03-03).
//
// It dispatches on h.view to the matching Plan 01 compute function, then JSON-
// encodes the result. The bank case encodes the BankView struct (yielding
// {"rows":[...],"coin":null}); the other three encode their row slice directly.
func (h *ViewsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// >>> No bearer-guard / 401 block here (D-04 public read). This is the ONLY
	// structural difference from whoami.go. <<<

	ctx := r.Context()

	var (
		result any
		rows   int
		err    error
	)
	switch h.view {
	case "view":
		var vr []compute.ViewRow
		vr, err = compute.View(ctx, h.store)
		// Coerce a nil slice to a non-nil empty one so the JSON is always an array
		// ([]), never null — a stable shape the thin client can iterate without a
		// nil-guard (the three view cases return a nil slice when no rows match).
		if vr == nil {
			vr = []compute.ViewRow{}
		}
		result, rows = vr, len(vr)
	case "gear_check":
		var gr []compute.GearCheckRow
		gr, err = compute.GearCheck(ctx, h.store)
		if gr == nil {
			gr = []compute.GearCheckRow{}
		}
		result, rows = gr, len(gr)
	case "spell_check":
		var sr []compute.SpellCheckRow
		sr, err = compute.SpellCheck(ctx, h.store)
		if sr == nil {
			sr = []compute.SpellCheckRow{}
		}
		result, rows = sr, len(sr)
	case "bank":
		var bv compute.BankView
		bv, err = compute.Bank(ctx, h.store)
		// The bank's rows array gets the same nil→[] coercion (coin stays null in
		// P14). Encode the BankView struct itself so the JSON is
		// {"rows":[...],"coin":null}.
		if bv.Rows == nil {
			bv.Rows = []compute.BankRow{}
		}
		result, rows = bv, len(bv.Rows)
	default:
		// Defensive: a route was registered with a name this switch doesn't know.
		// Never reachable via the four registered routes.
		slog.Error("views unknown view", "view", h.view, "status", http.StatusInternalServerError)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err != nil {
		// V7: log the op + view + err only — NEVER the row content.
		slog.Error("views read failed", "view", h.view, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		// Header (200) is already flushed; we can't change the status now. Log the
		// encode failure (op + view, never content) for observability.
		slog.Error("views encode failed", "view", h.view, "err", err)
		return
	}
	slog.Info("views ok", "view", h.view, "rows", rows, "status", http.StatusOK)
}
