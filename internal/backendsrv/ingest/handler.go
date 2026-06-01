package ingest

// handler.go composes the verdict-agnostic backend pieces (11-02/03/04) into the
// one network-exposed write surface this milestone introduces: POST
// /api/v1/ingest. Per the 11-01 verdict the HTTP shell is stdlib net/http
// (registered on a ServeMux with Go 1.22+ "POST /api/v1/ingest" method+pattern
// routing in cmd/squirebot-server) — there is NO PocketBase.
//
// The request flow is load-bearing and ordered (RESEARCH Architecture diagram):
//
//	[0] cap the body         http.MaxBytesReader(w, r.Body, maxBodyBytes)   (V5 / DoS)
//	[1] bearer guard FIRST   auth.ResolveToken(...) → !ok ⇒ 401, RETURN     (BACKEND-04 / V2)
//	                         BEFORE any store call — 401 writes NOTHING.
//	[2] decode + validate    DecodeAndValidate → 4xx on malformed/bad-kind   (V5)
//	[3] parse content        parse.Parse / ParseSpellbook (UTF-8, A1)        (D-03)
//	[4] ONE transaction:     BeginTx (BEGIN IMMEDIATE) →
//	      store.BindCharacter (first-sighting bind; cross-owner ⇒ 409 + audit, rollback)
//	      store.Replace*Tx    (atomic full-snapshot replace for the bound charID)
//	      Commit              → 204 No Content
//
// SINGLE-TRANSACTION REUSE (the load-bearing constraint, 11-05 WARNING-3 fix):
// the atomic-replace + cross-owner-reject logic is OWNED by 11-03's *sql.Tx-
// based BindCharacter / ReplaceInventoryTx / ReplaceSpellbookTx. This handler
// calls those EXACT functions over ONE *sql.Tx — it authors NO inline
// DELETE/INSERT/character SQL. 11-03's store tests remain the single coverage
// for that logic (the public Store.Replace* methods delegate to the same Tx
// bodies the handler uses), so there is no second, test-uncovered SQL path.
//
// SECURITY (V7): the handler NEVER logs the raw bearer token / Authorization
// header or the raw `content` — only operation + status + char name + err.

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/auth"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/parse"
)

// maxBodyBytes caps the ingest request body at ~1 MB (V5 / T-11.05-02). A maxed-
// out character snapshot is <50 KB (finding 01), so 1 MB is generous headroom
// while still bounding memory against a malicious/oversized payload. The cap is
// enforced via http.MaxBytesReader BEFORE the JSON decode, so an oversized body
// surfaces as a decode error (mapped to a 4xx) rather than being buffered whole.
const maxBodyBytes = 1 << 20 // 1 MiB

// minWatcherVersion is the floor below which the backend refuses an ingest with
// 426 Upgrade Required (SC-5 / CONTEXT D-4). It is the first re-targeted watcher
// version (v2.0.0, the version that POSTs to /api/v1/ingest instead of writing
// the Sheet); bump it ONLY on a real breaking API change. The compare is the
// SemVer-aware ingest.IsOlder (version.go) — the single backend version-compare
// truth, whose behavior twin is the watcher-side 999.22 fix (Plan 04). An empty
// watcher_version is NOT measured against this floor (see the gate in ServeHTTP).
const minWatcherVersion = "2.0.0"

// Handler serves POST /api/v1/ingest. It holds the bearer guard (11-04) and the
// single-writer *sql.DB (11-02) over which it composes the first-sighting bind
// (11-03) and the atomic replace (11-03) in ONE transaction. Construct it once
// at server startup via New and register it on the ServeMux:
//
//	mux.Handle("POST /api/v1/ingest", ingest.New(guard, db))
type Handler struct {
	guard *auth.Auth
	db    *sql.DB
}

// New builds an ingest Handler from the bearer guard and the backend DB handle.
// Both come from server startup (auth.New(db) and store.Open(dbPath)).
func New(guard *auth.Auth, db *sql.DB) *Handler {
	return &Handler{guard: guard, db: db}
}

// ServeHTTP implements http.Handler. The method routing ("POST") is done by the
// ServeMux pattern in cmd/squirebot-server; this method assumes POST but still
// guards defensively.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// [0] Cap the body BEFORE any read/decode (V5 / DoS — T-11.05-02).
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	// [1] Bearer guard FIRST — before ANY store call. A missing/malformed/
	// unknown/revoked token ⇒ 401 and we RETURN immediately, having touched the
	// store zero times (the "401 writes nothing" guarantee — BACKEND-04 / V2 /
	// T-11.05-01). NEVER log the token (V7).
	ownerID, codeID, ok := h.guard.ResolveToken(r.Context(), r.Header.Get("Authorization"))
	if !ok {
		slog.Info("ingest rejected", "reason", "unauthenticated", "status", http.StatusUnauthorized)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// [2] Decode + validate the envelope. Map each typed error to a specific 4xx
	// (V5). An oversized body (the cap from [0]) surfaces here as a decode error
	// ⇒ mapped to 400/413 below. No store call has happened yet, so any 4xx here
	// also writes nothing.
	env, err := DecodeAndValidate(r.Body)
	if err != nil {
		h.writeEnvelopeError(w, err)
		return
	}

	// [2.5] min-watcher-version gate (SC-5 / D-4). The bearer guard [1] already
	// ran, so a missing token is 401 (NOT 426). An EMPTY watcher_version is NOT
	// rejected: the envelope documents the field as "accepted now" (forward-compat
	// with a legacy/blank version), so only a PRESENT-but-too-old version trips the
	// 426 — `env.WatcherVersion != ""` is the one intentional exception to
	// IsOlder's "empty present ⇒ older" rule. This runs BEFORE any store call (it
	// is post-decode but pre-parse/pre-bind), so a 426 writes NOTHING. The message
	// is the watcher-facing string the tray mirrors — kept human and reassuring.
	if env.WatcherVersion != "" && IsOlder(env.WatcherVersion, minWatcherVersion) {
		slog.Info("ingest rejected", "reason", "watcher_too_old", "ver", env.WatcherVersion, "status", http.StatusUpgradeRequired)
		http.Error(w, "Your SquireBot is too old for this server; it will auto-update shortly.", http.StatusUpgradeRequired)
		return
	}

	// [3] Parse `content` (UTF-8, contract A1 — feed straight in, NO CP1252
	// decode; the watcher owns that on the disk-read side). An empty content
	// yields (nil, nil) — a valid no-op snapshot that clears the char's rows.
	rows, err := parseContent(env)
	if err != nil {
		// A genuinely malformed payload (e.g. a CSV the parser cannot read) is a
		// client error; never echo the raw content (V7).
		slog.Info("ingest rejected", "reason", "parse_failed", "char", env.Character, "kind", env.Kind, "status", http.StatusUnprocessableEntity)
		http.Error(w, "could not parse content", http.StatusUnprocessableEntity)
		return
	}

	// [4] ONE transaction: first-sighting bind + atomic replace, committed
	// together (a cross-owner reject or a replace failure rolls BOTH back).
	status, err := h.bindAndReplace(r, ownerID, env, rows)
	if err != nil {
		if errors.Is(err, store.ErrCharOwnedByAnother) {
			// Cross-owner upload (D-07 / V4 / T-11.05-04): 409, already audited
			// in BindCharacter; the tx rolled back so the original owner's rows
			// are untouched.
			slog.Warn("ingest rejected", "reason", "cross_owner", "char", env.Character, "status", http.StatusConflict)
			http.Error(w, "character is owned by another guildie", http.StatusConflict)
			return
		}
		// Anything else is a server-side failure (DB error, etc.) — 500. Never
		// echo content or token material (V7).
		slog.Error("ingest failed", "char", env.Character, "kind", env.Kind, "err", err, "status", http.StatusInternalServerError)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// [5] Best-effort last_seen stamp (D-07 / LINK-05). A SEPARATE, non-blocking
	// UPDATE OUTSIDE the atomic replace tx (bindAndReplace above): last_seen is
	// advisory UI metadata ("which PC, is it dead?"), NOT data integrity, so a
	// failed/slow stamp logs and is dropped — it must NEVER change the ingest
	// response (RESEARCH A1). codeID is the matched guild_code.id threaded out of
	// the bearer guard. NEVER log the token (V7) — only code_id + err.
	if _, serr := h.db.ExecContext(r.Context(),
		`UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?`, codeID); serr != nil {
		slog.Warn("stamp last_seen failed (non-fatal)", "code_id", codeID, "err", serr)
	}

	slog.Info("ingest ok", "char", env.Character, "kind", env.Kind, "rows", len(rows), "status", status)
	w.WriteHeader(status)
}

// parseContent runs the verdict-agnostic parser for the envelope's kind. content
// is UTF-8 (A1): feed strings.NewReader(env.Content) straight into the parser —
// NO charmap/CP1252 decode (the watcher decodes on the disk side; double-
// decoding would mojibake). DecodeAndValidate already guaranteed Kind is one of
// the two enum values, so the default branch is unreachable defensive code.
func parseContent(env Envelope) ([][]string, error) {
	switch env.Kind {
	case KindInventory:
		return parse.Parse(strings.NewReader(env.Content))
	case KindSpellbook:
		return parse.ParseSpellbook(strings.NewReader(env.Content))
	default:
		// Unreachable: DecodeAndValidate rejects any other kind. Kept as a guard.
		return nil, ErrInvalidKind
	}
}

// bindAndReplace runs the first-sighting bind and the atomic full-snapshot
// replace in ONE *sql.Tx (BEGIN IMMEDIATE via the _txlock=immediate DSN), then
// commits. It returns the HTTP success status (204 No Content) on commit, or an
// error the caller maps to 409 (ErrCharOwnedByAnother) / 500.
//
// This is the SINGLE place the handler touches SQL — and it does so only through
// 11-03's exported Tx functions (BindCharacter, ReplaceInventoryTx,
// ReplaceSpellbookTx). It authors NO DELETE/INSERT/character SQL of its own (the
// "no second SQL path" constraint). bind and replace share ONE tx so a cross-
// owner reject rolls the (no-op) work back cleanly and the replace sees the
// just-bound charID.
func (h *Handler) bindAndReplace(r *http.Request, ownerID int64, env Envelope, rows [][]string) (int, error) {
	ctx := r.Context()

	tx, err := h.db.BeginTx(ctx, nil) // BEGIN IMMEDIATE (single-writer DSN)
	if err != nil {
		return 0, err
	}

	// First-sighting bind. On a cross-owner attempt, BindCharacter writes an
	// append-only audit_log row INSIDE this tx and returns ErrCharOwnedByAnother
	// (owner_id is never overwritten — binding.go). That audit record is the
	// durable trace of a takeover attempt (D-07 / V4 / T-11.03-05), so we must
	// COMMIT the tx to persist it even though the ingest is refused — mirroring
	// 11-03's own bindInTx test helper ("commit anyway so the audit row is
	// durable … even though the ingest itself is rejected"). A plain rollback
	// here would silently discard the audit trail, which is the bug this path
	// guards against. Since the cross-owner branch performs NO character/row
	// mutation (only the audit INSERT), committing it is safe.
	charID, err := store.BindCharacter(ctx, tx, env.Character, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrCharOwnedByAnother) {
			if cerr := tx.Commit(); cerr != nil {
				// If the audit commit itself fails, surface that (and the tx is
				// already broken); the caller maps a non-409 error to 500.
				return 0, cerr
			}
			return 0, err // 409; audit row persisted
		}
		// Any other bind error (e.g. DB failure): roll back, map to 500.
		_ = tx.Rollback()
		return 0, err
	}

	// Atomic full-snapshot replace for the bound charID, in the SAME tx. The
	// watcher_version travels in the envelope and is stored alongside the rows; a
	// too-old version was already rejected at the [2.5] gate above, so any version
	// reaching here is at/above the floor (or intentionally empty). On failure,
	// roll back BOTH the bind and the (partial) replace.
	uploadedAt := time.Now().UTC()
	switch env.Kind {
	case KindInventory:
		err = store.ReplaceInventoryTx(ctx, tx, charID, rows, uploadedAt, env.WatcherVersion)
	case KindSpellbook:
		err = store.ReplaceSpellbookTx(ctx, tx, charID, rows, uploadedAt, env.WatcherVersion)
	}
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return http.StatusNoContent, nil
}

// writeEnvelopeError maps a DecodeAndValidate error to the right 4xx (V5):
//   - ErrMalformedJSON   → 400 (also covers an oversized body tripping the cap)
//   - ErrMissingCharacter / ErrInvalidKind → 422 (well-formed JSON, bad values)
//
// It never echoes the raw content or any token material (V7).
func (h *Handler) writeEnvelopeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrMissingCharacter), errors.Is(err, ErrInvalidKind):
		slog.Info("ingest rejected", "reason", "invalid_envelope", "status", http.StatusUnprocessableEntity)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, ErrMalformedJSON):
		// http.MaxBytesReader surfaces an oversized body as a decode error here;
		// 400 is the correct, conservative mapping (some servers use 413 — we
		// return 400 uniformly for "the body we got is not a valid envelope").
		slog.Info("ingest rejected", "reason", "malformed_json", "status", http.StatusBadRequest)
		http.Error(w, "malformed JSON envelope", http.StatusBadRequest)
	default:
		slog.Info("ingest rejected", "reason", "bad_request", "status", http.StatusBadRequest)
		http.Error(w, "bad request", http.StatusBadRequest)
	}
}
