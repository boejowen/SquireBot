package readapi_test

// banks_test.go is the httptest proof of the Phase 33 GET /api/v1/banks handler contract
// (BANK-01/02): the data-exposure gate is at the API (RequireSession, fail-closed 401
// without a cookie), an empty result encodes the BanksView with `banks: []` not null, and a
// seeded bank + guild bot come back A-Z with the guild summary (guild_value + total_platinum).
// It reuses the seed helpers in readapi_test.go (same package). V7: asserts only
// status/shape/totals, never bank/item names in logs.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/readapi"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// seedBanksStore builds a bank toon (with a priced item + plat) and a guild bot (with a
// priced item, no plat) so /api/v1/banks returns a populated BanksView whose guild value
// includes the bot's goods (the Phase 33 scope widen).
func seedBanksStore(t *testing.T) *store.Store {
	t.Helper()
	db := store.NewTestDB(t)

	// A bank toon "Zbank" with a priced item + plat 500.
	bankID := seedChar(t, db, "owner-b", "Zbank", "Warrior", 60, true)
	if _, err := db.Exec(`UPDATE character SET plat = 500 WHERE id = ?`, bankID); err != nil {
		t.Fatalf("set Zbank plat: %v", err)
	}
	seedItemMaster(t, db, 5001, "Gem A", "A gem.", "https://wiki.project1999.com/Gem_A", false)
	seedPigparse(t, db, 5001, "Gem A", "0", 100, 10)
	seedInv(t, db, bankID, "Bank1", "Gem A", 5001, 1)

	// A guild bot "Abot" with a priced item, NO plat (coin is bank-toon-gated).
	botID := seedChar(t, db, "owner-bo", "Abot", "Cleric", 60, false)
	if _, err := db.Exec(`UPDATE character SET is_guild_bot = 1 WHERE id = ?`, botID); err != nil {
		t.Fatalf("set is_guild_bot: %v", err)
	}
	seedItemMaster(t, db, 5002, "Gem B", "A gem.", "https://wiki.project1999.com/Gem_B", false)
	seedPigparse(t, db, 5002, "Gem B", "0", 80, 4)
	seedInv(t, db, botID, "General1", "Gem B", 5002, 1)

	return store.NewStore(db)
}

// TestBanks_RequireSession_401WithoutCookie proves the BLOCKING T-33-02 gate: the route,
// registered under webauth.RequireSession, returns 401 (NOT the inner 200) when the request
// carries no session cookie — fail-closed at the API, not just the UI. Exercises the SAME
// wrap the production registration in cmd/squirebot-server/main.go applies.
func TestBanks_RequireSession_401WithoutCookie(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)

	h := webauth.RequireSession(db, readapi.NewBanks(st))
	rec := httptest.NewRecorder()
	// No sb_session cookie → RequireSession must reject with 401, fail-closed.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/banks", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/v1/banks without a session = %d, want 401 (RequireSession fail-closed)", rec.Code)
	}
}

// TestBanks_Empty_EncodesArrayNotNull proves the `[]` not null discipline: an empty store
// (no bank toons) returns 200 with a BanksView whose `banks` field is `[]`, not `null`.
func TestBanks_Empty_EncodesArrayNotNull(t *testing.T) {
	db := store.NewTestDB(t)
	h := readapi.NewBanks(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/banks", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	// The body is an OBJECT carrying the guild summary; its `banks` field must be [] not null.
	var body struct {
		Banks         json.RawMessage `json:"banks"`
		GuildValue    float64         `json:"guild_value"`
		TotalPlatinum int64           `json:"total_platinum"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode banks body as object: %v (body=%s)", err, rec.Body.String())
	}
	if strings.TrimSpace(string(body.Banks)) != "[]" {
		t.Fatalf("banks field = %q, want [] (not null)", string(body.Banks))
	}
}

// TestBanks_OK_EncodesView proves the seeded bank + guild bot come back as a BanksView whose
// `banks` array carries both A-Z, with the guild summary (guild_value includes the bot's
// goods; total_platinum is the bank toon's plat only).
func TestBanks_OK_EncodesView(t *testing.T) {
	h := readapi.NewBanks(seedBanksStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/banks", nil).
		WithContext(webauth.WithUser(context.Background(), "discord-x"))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Banks []struct {
			Name      string  `json:"name"`
			ItemCount int64   `json:"item_count"`
			Value     float64 `json:"value"`
			Unpriced  int64   `json:"unpriced"`
			Plat      *int64  `json:"plat"`
		} `json:"banks"`
		GuildValue    float64 `json:"guild_value"`
		GuildUnpriced int64   `json:"guild_unpriced"`
		TotalPlatinum int64   `json:"total_platinum"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode banks body: %v (body=%s)", err, rec.Body.String())
	}

	if len(body.Banks) != 2 {
		t.Fatalf("banks = %d, want 2 (bank + bot): %+v", len(body.Banks), body.Banks)
	}
	// A-Z: "Abot" (the bot) before "Zbank".
	if body.Banks[0].Name != "Abot" || body.Banks[1].Name != "Zbank" {
		t.Errorf("order = %q,%q, want Abot,Zbank (A-Z)", body.Banks[0].Name, body.Banks[1].Name)
	}
	// The guild value includes the bot's 80 + the bank's 100 (the scope widen).
	if body.GuildValue != 180 {
		t.Errorf("guild_value = %v, want 180 (bank 100 + bot 80 — bot goods included)", body.GuildValue)
	}
	// Total platinum is the bank toon's 500; the bot's plat is null → 0.
	if body.TotalPlatinum != 500 {
		t.Errorf("total_platinum = %d, want 500 (bank plat only; bot plat null)", body.TotalPlatinum)
	}
	// The bot row carries plat: null (NOT 0).
	if body.Banks[0].Plat != nil {
		t.Errorf("Abot (bot) plat = %v, want null", body.Banks[0].Plat)
	}
	// The bank row carries its plat.
	if body.Banks[1].Plat == nil || *body.Banks[1].Plat != 500 {
		t.Errorf("Zbank plat = %v, want 500", body.Banks[1].Plat)
	}
}

// TestBanks_NonGET_405 proves the read-only contract.
func TestBanks_NonGET_405(t *testing.T) {
	db := store.NewTestDB(t)
	h := readapi.NewBanks(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/banks", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /banks status = %d, want 405", rec.Code)
	}
}
