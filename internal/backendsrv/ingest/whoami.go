package ingest

// whoami.go serves GET /api/v1/whoami — the authed, SIDE-EFFECT-FREE validation
// endpoint the watcher's onboarding (Plan 02/03) calls to verify a pasted guild
// code before storing it in DPAPI (CONTEXT D-4). It reuses the SHIPPED bearer
// guard (auth.ResolveToken, 11-04) VERBATIM — no new auth path — so a valid
// active code returns 200 + the owner label and a missing / unknown / revoked
// code returns 401, mirroring the ingest handler's "401 writes nothing"
// discipline.
//
// READ-ONLY contract (T-13.01-04): the only SQL this file authors is a single
// parameterized `SELECT label FROM owner WHERE id = ?` scoped to the already-
// resolved ownerID — a pure read, no DELETE/INSERT/UPDATE. The endpoint performs
// ZERO mutations (proven by the row-count-unchanged test), so it does NOT need
// the single-transaction store helpers the ingest WRITE path is bound to. The
// label is a friendliness ("Connected as <label>") the onboarding can surface;
// validity is the load-bearing 200/401 distinction the watcher actually gates on,
// so a scan failure degrades to an empty label rather than a 500.
//
// SECURITY (V7): the raw Bearer token / Authorization header is NEVER logged —
// the slog records carry only owner_id + status (never the token, never a
// token-vs-label confusion).

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
)

// WhoamiHandler serves GET /api/v1/whoami. It holds the SAME bearer guard (11-04)
// the ingest handler uses and the single-writer *sql.DB (11-02) — but only ever
// READS from it (the owner label for a friendly 200 body). Construct it once at
// server startup via NewWhoami and register it on the ServeMux next to the ingest
// route:
//
//	mux.Handle("GET /api/v1/whoami", ingest.NewWhoami(auth.New(db), db))
type WhoamiHandler struct {
	guard *auth.Auth
	db    *sql.DB
}

// NewWhoami builds a whoami handler from the bearer guard and the backend DB
// handle. The db is needed only for the read-only owner-label lookup; a separate
// auth.New(db) allocation is fine (the guard is a thin stateless wrapper over the
// shared *sql.DB).
func NewWhoami(guard *auth.Auth, db *sql.DB) *WhoamiHandler {
	return &WhoamiHandler{guard: guard, db: db}
}

// ServeHTTP implements http.Handler. The method routing ("GET") is done by the
// ServeMux pattern in cmd/squirebot-server; this method assumes GET but still
// guards defensively (405 on anything else) so a direct/mis-registered call is
// safe.
func (h *WhoamiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Bearer guard — reused VERBATIM from the ingest path. A missing/malformed/
	// unknown/revoked token ⇒ 401, and we RETURN. The endpoint is read-only, so a
	// 401 trivially touches nothing. NEVER log the token (V7).
	ownerID, ok := h.guard.ResolveToken(r.Context(), r.Header.Get("Authorization"))
	if !ok {
		slog.Info("whoami rejected", "reason", "unauthenticated", "status", http.StatusUnauthorized)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Read-only owner-label lookup (the ONLY SQL here — pure read, parameterized,
	// scoped to the resolved ownerID). A scan error (incl. sql.ErrNoRows, which a
	// freshly-resolved owner should never produce) degrades to an empty label: the
	// 200 attests VALIDITY, not the label, so we never 500 a valid code over a
	// label-fetch hiccup.
	var label string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT label FROM owner WHERE id = ?`, ownerID).Scan(&label); err != nil {
		label = ""
		slog.Warn("whoami label lookup failed", "owner_id", ownerID, "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// The watcher only needs the 200/401 distinction; owner_id + owner_label are a
	// nicety the onboarding surfaces as "Connected as <label>".
	_ = json.NewEncoder(w).Encode(map[string]any{
		"owner_id":    ownerID,
		"owner_label": label,
	})
	slog.Info("whoami ok", "owner_id", ownerID, "status", http.StatusOK)
}
