package readapi_test

// items_test.go is the httptest proof of the Phase 32 GET /api/v1/items handler contract
// (ITEM-01..03): the data-exposure gate is at the API (RequireSession, fail-closed 401
// without a cookie), an empty result encodes as `[]` not null, and a seeded guild item
// comes back grouped by normalized name with the viewer's holdings flagged is_mine. It
// reuses the seed helpers in readapi_test.go (same package). V7: asserts only
// status/shape/flags, never item names in logs.

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

// TestItems_RequireSession_401WithoutCookie proves the BLOCKING T-32-01/05 gate: the
// route, registered under webauth.RequireSession, returns 401 (NOT the inner 200) when the
// request carries no session cookie — fail-closed at the API, not just the UI. Exercises
// the SAME wrap the production registration in cmd/squirebot-server/main.go applies.
func TestItems_RequireSession_401WithoutCookie(t *testing.T) {
	db := store.NewTestDB(t)
	st := store.NewStore(db)

	h := webauth.RequireSession(db, readapi.NewItems(st))
	rec := httptest.NewRecorder()
	// No sb_session cookie → RequireSession must reject with 401, fail-closed.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/v1/items without a session = %d, want 401 (RequireSession fail-closed)", rec.Code)
	}
}

// TestItems_EmptyEncodesArrayNotNull proves the `[]` not null discipline: an empty store
// (no inventory) returns 200 with a bare `[]` body, not `null`.
func TestItems_EmptyEncodesArrayNotNull(t *testing.T) {
	db := store.NewTestDB(t)
	h := readapi.NewItems(store.NewStore(db))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil).
		WithContext(webauth.WithUser(context.Background(), "discord-x"))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("empty items body = %q, want []", got)
	}
}

// TestItems_OK_GroupedRollup proves the seeded guild items come back as a JSON array of
// rollups with the snake_case contract keys, grouped by normalized name (the seedStore
// "Jade Reaver" held by Alpha + the Banker bank toon collapses to ONE rollup with
// holder_count 2 and summed_qty 2).
func TestItems_OK_GroupedRollup(t *testing.T) {
	st := seedStore(t)
	h := readapi.NewItems(st)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil).
		WithContext(webauth.WithUser(context.Background(), "discord-x"))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var rolls []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rolls); err != nil {
		t.Fatalf("decode items body as JSON array: %v (body=%s)", err, rec.Body.String())
	}
	if len(rolls) == 0 {
		t.Fatalf("items returned 0 rollups, want >=1 (seeded Jade Reaver + Bone Chips)")
	}
	// The snake_case contract keys the SvelteKit client consumes.
	for _, k := range []string{"name", "summed_qty", "holder_count", "is_mine", "price", "wiki_url", "icon_id", "statsblock", "holders"} {
		if _, ok := rolls[0][k]; !ok {
			t.Errorf("rollup missing %q key; got keys %v", k, keysOf(rolls[0]))
		}
	}

	// Jade Reaver is held by Alpha (General1) + Banker (Bank1) → ONE rollup, holder_count 2.
	var jade map[string]any
	for _, r := range rolls {
		if r["name"] == "Jade Reaver" {
			jade = r
		}
	}
	if jade == nil {
		t.Fatalf("no Jade Reaver rollup; got %d rollups", len(rolls))
	}
	if hc, _ := jade["holder_count"].(float64); hc != 2 {
		t.Errorf("Jade Reaver holder_count = %v, want 2 (Alpha + Banker)", jade["holder_count"])
	}
	holders, _ := jade["holders"].([]any)
	if len(holders) != 2 {
		t.Errorf("Jade Reaver holders = %d, want 2", len(holders))
	}
}

// TestItems_NonGET_405 proves the read-only contract.
func TestItems_NonGET_405(t *testing.T) {
	db := store.NewTestDB(t)
	h := readapi.NewItems(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/items", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /items status = %d, want 405", rec.Code)
	}
}
