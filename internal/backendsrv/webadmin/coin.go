package webadmin

// coin.go is the bank-coin entry backend (ADMIN-05 / D-11/D-12). CRITICAL (D-12,
// B-1): bank-coin is gated by LOGIN ONLY — the route wraps these handlers in
// webauth.RequireSession (a valid session), NEVER webauth.RequireOfficer. Any
// authenticated guild member may record the shared bank's coin; this handler does
// NOT consult officer status anywhere (no RequireOfficer, no IsOfficer call). The
// writer's discord_user_id is recorded in the coin_set audit row for
// accountability — that is the only identity use here. (Contrast eviction.go /
// officers.go, which ARE officer-gated.) The non-officer write path is explicitly
// proven by coin_test.go's TestCoinSet_NonOfficerCanWrite.
//
// Server-side range validation (T-15-15 — never trust the client's disabled-button
// UX): plat >= 0; gold/silver/copper each in [0,999]. Out-of-range → 400
// invalid_input. The store's SetCoinTx additionally refuses a non-bank-toon target
// (ErrNotBankToon → 400 not_bank_toon) — the D-11 bank-toon gate enforced at the
// data layer too. The write + its audit row compose in ONE BEGIN IMMEDIATE tx
// (withTx), so they land atomically.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// coinReq is the bank-coin POST body. The four coin fields are plain int64 so an
// explicit 0 is a real value (the form pre-fills from the stored coin, where 0 and
// "unset" differ — see store.BankToon's *int64 read side).
type coinReq struct {
	CharacterID int64 `json:"character_id"`
	Plat        int64 `json:"plat"`
	Gold        int64 `json:"gold"`
	Silver      int64 `json:"silver"`
	Copper      int64 `json:"copper"`
}

// BankToonsHandler (GET) returns the live is_bank_toon characters with their
// current coin — the form pre-fill source and the values the bank view surfaces
// (replacing P14's null/0 placeholder, D-11). Login-only at the route
// (RequireSession). Empty → [] so the UI shows the no-bank-toons state.
func BankToonsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		toons, err := store.ListBankToons(r.Context(), db)
		if err != nil {
			slog.Error("bank toons list failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		if toons == nil {
			toons = []store.BankToon{}
		}
		writeJSON(w, toons)
	}
}

// CoinSetHandler (POST) records coin on a bank toon. Login-only at the route
// (RequireSession — D-12; this handler NEVER checks officer status). Validates the
// ranges, writes via store.SetCoinTx (bank-toon-gated), audits "coin_set" with the
// writer's discord id, and returns {character, coin}.
func CoinSetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req coinReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		// Server-side range validation (T-15-15): plat >= 0; g/s/c in [0,999].
		if !validCoin(req) {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}

		// The acting identity is recorded for audit/accountability ONLY — it is NOT
		// an authorization input here (D-12: any authenticated member may write).
		writer := caller(ctx)
		now := nowUnix()

		// ONE tx (BEGIN IMMEDIATE via withTx): SetCoinTx (bank-toon-gated) + the audit
		// row, committed together.
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.SetCoinTx(ctx, tx, req.CharacterID, req.Plat, req.Gold, req.Silver, req.Copper); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "coin_set", writer, map[string]any{
				"character_id": req.CharacterID,
			}, now)
		})
		if err != nil {
			if errors.Is(err, store.ErrNotBankToon) {
				writeJSONError(w, http.StatusBadRequest, "not_bank_toon")
				return
			}
			slog.Error("coin set failed", "character_id", req.CharacterID, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}

		// Echo the character name + the saved coin (the UI's "Coin saved for
		// <character>." + the pre-fill refresh). A read-back failure is non-fatal —
		// the write committed; fall back to the request values.
		name := ""
		if bt, gerr := store.GetCoin(ctx, db, req.CharacterID); gerr == nil {
			name = bt.Name
		}
		writeJSON(w, map[string]any{
			"character": name,
			"coin": map[string]int64{
				"plat": req.Plat, "gold": req.Gold, "silver": req.Silver, "copper": req.Copper,
			},
		})
	}
}

// validCoin enforces the D-11/UI-SPEC ranges: plat >= 0 (a whole number, 0 or
// more) and gold/silver/copper each in [0,999]. Returns true iff all four pass.
func validCoin(req coinReq) bool {
	if req.Plat < 0 {
		return false
	}
	for _, v := range []int64{req.Gold, req.Silver, req.Copper} {
		if v < 0 || v > 999 {
			return false
		}
	}
	return true
}
