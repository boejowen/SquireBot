package readapi

// characters.go serves GET /api/v1/characters — the viewer-aware Characters-tab
// roster (CHAR-01/02, D-10), the new Phase 31 read surface. Unlike views.go/meta.go
// it needs the VIEWER's discord_user_id (read from the request context that
// RequireSession injects) to flag is_mine and order the viewer's characters first.
// It composes store.RosterFor (the single tested SQL path) behind a local typed
// snake_case response (the meta.go shape), pre-sized so an empty roster encodes as
// `[]` not null.
//
// RequireSession-gated at registration (login-only since P15 — NOT public, NOT
// officer). The roster is a GUILD-WIDE read: every member sees every character; the
// gate is membership, not ownership — the response is ORDERED viewer-first but never
// SCOPED to the viewer's own chars (V4 / T-31-07). NEVER RequireOfficer.
//
// SECURITY:
//   - V5: the viewer id binds only as RosterFor's single `?` placeholder (no SQL
//     built here, no string-concat).
//   - V7: slog carries op + row count + status + err ONLY — never a char name.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// rosterChar is one row of the /api/v1/characters payload: a character's identity +
// metadata + the bank/bot designation flags + the per-char upload freshness +
// whether the row is assigned to the viewer. snake_case JSON tags pin the contract
// the SvelteKit client consumes (mirroring the Go RosterRow / compute snake_case).
type rosterChar struct {
	Name       string `json:"name"`
	Level      int64  `json:"level"`
	Race       string `json:"race"`
	Class      string `json:"class"`
	IsMine     bool   `json:"is_mine"`
	IsBankToon bool   `json:"is_bank_toon"`
	IsGuildBot bool   `json:"is_guild_bot"`
	LastSeen   string `json:"last_seen"`
}

// CharactersHandler serves GET /api/v1/characters. It holds the read-side
// *store.Store; the viewer identity comes from the RequireSession-populated
// context, so no *sql.DB is needed in the handler. Construct once at startup and
// register UNDER webauth.RequireSession:
//
//	mux.Handle("GET /api/v1/characters", webauth.RequireSession(db, readapi.NewCharacters(st)))
type CharactersHandler struct {
	store *store.Store
}

// NewCharacters builds the roster handler from the read-side store.
func NewCharacters(s *store.Store) *CharactersHandler {
	return &CharactersHandler{store: s}
}

// ServeHTTP implements http.Handler — GET-only (405 otherwise). It reads the
// viewer's discord_user_id from context (RequireSession guarantees it in
// production; an absent id → "" → nothing flagged is_mine, still a valid roster),
// fetches the band-tagged viewer-first roster, and encodes it as a JSON array.
func (h *CharactersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	uid, _ := webauth.UserFromContext(ctx) // viewer's discord_user_id; "" → no rows flagged mine

	rows, err := h.store.RosterFor(ctx, uid)
	if err != nil {
		// V7: op + err only — never a char name.
		slog.Error("characters read failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Pre-size so an empty roster marshals as [] (not null) — a stable client shape.
	out := make([]rosterChar, 0, len(rows))
	for _, c := range rows {
		out = append(out, rosterChar{
			Name:       c.Name,
			Level:      c.Level,
			Race:       c.Race,
			Class:      c.Class,
			IsMine:     c.IsMine,
			IsBankToon: c.IsBankToon,
			IsGuildBot: c.IsGuildBot,
			LastSeen:   c.LastSeen,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		slog.Error("characters encode failed", "err", err)
		return
	}
	slog.Info("characters ok", "rows", len(out), "status", http.StatusOK)
}
