package webadmin

// charmeta.go is the char-metadata write backend (CUTOVER-02 / P16, D-02/D-03).
// CRITICAL (D-03): char-meta is gated by LOGIN ONLY — the route wraps these
// handlers in webauth.RequireSession (a valid session), NEVER webauth.RequireOfficer.
// Any authenticated guild member may set any existing character's class/level/race;
// this is non-sensitive shared data in a trust-rich ~12-person guild (the bank-coin
// precedent, D-12). Phase 26 reconciliation (OPEN-3): is_bank_toon is NO LONGER set
// here — guild-bank designation became officer-only (store.DesignateCharTx, the
// /admin/characters/designate route); the member path writes only class/level/race and
// ignores any is_bank_toon body field. This handler does NOT consult officer status
// anywhere (no RequireOfficer, no IsOfficer call) — the non-officer write path is
// explicitly proven by charmeta_test.go's TestCharMetaSet_NonOfficerCanWrite. The
// writer's discord_user_id is recorded in the char_meta_set audit row for
// accountability — that is the only identity use here.
//
// Server-side value-set validation (T-15-29 / Pitfall 5 — never trust the form's
// <select>): class must be an exact uppercase abbreviation in enrich.CLASSES, race
// in enrich.RACES, and level is either blank/omitted (→ SQL NULL) or 1..60. A
// wrong-cased or free-text value ("Warrior", "Iksar") → 400 invalid_input, nothing
// written — a typo would otherwise silently produce zero gear/spell rows
// (gearcheck.go keys the Iksar tier on the literal "IKS"; the joins match exact
// abbreviations). The form operates on EXISTING characters only (D-03 forbids
// pre-creation); a missing/removed character_id → store.ErrCharNotFound → 400. The
// write + its audit row compose in ONE BEGIN IMMEDIATE tx (withTx), so they land
// atomically.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// charMetaReq is the char-meta POST body. Level is *int64 so an omitted/null level
// is distinguishable from an explicit value and maps to SQL NULL (blank = unset; a
// NULL level → spellcheck treats the char as unleveled, the correct behavior). The
// rest are scalars the decoder type-checks.
//
// Phase 26 reconciliation (OPEN-3): is_bank_toon is NO LONGER a member-settable field
// here — guild-bank designation became officer-only (store.DesignateCharTx, via
// POST /api/v1/admin/characters/designate). An is_bank_toon field in the incoming JSON
// is simply ignored by the decoder; the member char-meta path writes only class/level/
// race.
type charMetaReq struct {
	CharacterID int64  `json:"character_id"`
	Class       string `json:"class"`
	Level       *int64 `json:"level"`
	Race        string `json:"race"`
}

// CharMetaListHandler (GET) returns every live (non-removed) character with its
// identity + class/level/race/is_bank_toon — the form's pick-list + pre-fill source.
// Login-only at the route (RequireSession). Empty → [] so the UI shows the no-chars
// state. Mirrors BankToonsHandler (it reaches the store the same way, via a
// *sql.DB-shaped store wrapper).
func CharMetaListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		list, err := store.CharsForMeta(r.Context(), db)
		if err != nil {
			slog.Error("char meta list failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if list == nil {
			list = []store.CharMeta{}
		}
		writeJSON(w, list)
	}
}

// CharMetaSetHandler (POST) sets class/level/race on an existing character. Login-only
// at the route (RequireSession — D-03; this handler NEVER checks officer status).
// Validates the value sets + level range server-side, writes via store.SetCharMetaTx
// (existing non-removed char only, 5-arg — is_bank_toon is NOT written here anymore;
// it is officer-only via store.DesignateCharTx), audits "char_meta_set" with the
// writer's discord id, and returns {character, class, level, race}.
func CharMetaSetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req charMetaReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		// Server-side value-set + range re-validation (T-15-29): NEVER trust the
		// form's <select>. class ∈ CLASSES, race ∈ RACES, level blank or 1..60.
		if !validCharMeta(req) {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}

		// The acting identity is recorded for audit/accountability ONLY — it is NOT
		// an authorization input here (D-03: any authenticated member may write).
		writer := caller(ctx)
		now := nowUnix()

		// ONE tx (BEGIN IMMEDIATE via withTx): SetCharMetaTx (existing-char-gated) +
		// the audit row, committed together.
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.SetCharMetaTx(ctx, tx, req.CharacterID, req.Class, req.Level, req.Race); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "char_meta_set", writer, map[string]any{
				"character_id": req.CharacterID,
			}, now)
		})
		if err != nil {
			if errors.Is(err, store.ErrCharNotFound) {
				// No such (live) character — fail-closed (mirrors coin's ErrNotBankToon).
				writeJSONError(w, http.StatusBadRequest, "invalid_input")
				return
			}
			slog.Error("char meta set failed", "character_id", req.CharacterID, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}

		// Echo the character name + the saved values (the UI's "Saved details for
		// <character>." + the pre-fill refresh). A read-back failure is non-fatal —
		// the write committed; fall back to "" for the name.
		name := ""
		if list, gerr := store.CharsForMeta(ctx, db); gerr == nil {
			for _, c := range list {
				if c.ID == req.CharacterID {
					name = c.Name
					break
				}
			}
		}
		writeJSON(w, map[string]any{
			"character": name,
			"class":     req.Class,
			"level":     req.Level,
			"race":      req.Race,
		})
	}
}

// validCharMeta is the server-side V5 re-check (NEVER trust the form's <select>;
// T-15-29 / Pitfall 5): class ∈ enrich.CLASSES, race ∈ enrich.RACES (exact uppercase
// abbreviations — store the abbreviation, never a display name), level blank/omitted
// (→ NULL, valid) OR 1..60.
//
// A2 decision (documented): a blank/omitted level is allowed ("unset") — a member
// may know a char's class+race before its level, and spellcheck.go treats a NULL
// level (→0) as "skip", the correct unleveled behavior. A non-blank level MUST be
// 1..60.
func validCharMeta(req charMetaReq) bool {
	if !slices.Contains(enrich.CLASSES, req.Class) {
		return false
	}
	if !slices.Contains(enrich.RACES, req.Race) {
		return false
	}
	if req.Level != nil && (*req.Level < 1 || *req.Level > 60) {
		return false
	}
	return true
}
