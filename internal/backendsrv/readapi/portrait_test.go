package readapi_test

// portrait_test.go is the httptest proof of the Phase 41 GET /api/v1/characters/{name}/
// portrait serve handler (CHARUI-02, plan 41-01): it streams the STORED blob with the STORED
// sniffed content_type + X-Content-Type-Options: nosniff, 404s a portrait-less/unknown char,
// and 405s a non-GET. Raw-byte response (the only such handler in the API). Self-contained
// raw INSERT seeding over a migrated temp DB (store package seed helpers are package-private).

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/readapi"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/webauth"
)

// seedPortraitChar inserts a character and (optionally) a portrait row with the given blob +
// content_type. Returns the character id.
func seedPortraitChar(t *testing.T, db *sql.DB, name string, blob []byte, contentType string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.Exec(`INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		t.Fatalf("seed char %q: %v", name, err)
	}
	charID, _ := res.LastInsertId()
	if blob != nil {
		if _, err := db.Exec(
			`INSERT INTO character_portrait (character_id, image_blob, content_type, byte_size, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			charID, blob, contentType, len(blob), "2026-07-15T00:00:00Z"); err != nil {
			t.Fatalf("seed portrait (char=%d): %v", charID, err)
		}
	}
	return charID
}

// serveReq builds a GET request with the {name} path value set (the production route binds it
// from the ServeMux wildcard).
func serveReq(method, name string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/characters/"+name+"/portrait", nil)
	req.SetPathValue("name", name)
	return req
}

// TestPortraitServe_StreamsStoredBytes: a char with a portrait streams the exact bytes with
// the STORED content_type + nosniff.
func TestPortraitServe_StreamsStoredBytes(t *testing.T) {
	db := store.NewTestDB(t)
	blob := []byte{0x89, 0x50, 0x4E, 0x47, 0xDE, 0xAD, 0xBE, 0xEF}
	seedPortraitChar(t, db, "Slampeach", blob, "image/png")

	h := readapi.NewPortrait(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, serveReq(http.MethodGet, "Slampeach"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png (the stored sniffed value)", ct)
	}
	if ns := rec.Header().Get("X-Content-Type-Options"); ns != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", ns)
	}
	if !bytes.Equal(rec.Body.Bytes(), blob) {
		t.Errorf("served bytes = %v, want the stored blob %v", rec.Body.Bytes(), blob)
	}
}

// TestPortraitServe_NotFound: a char with no portrait → 404.
func TestPortraitServe_NotFound(t *testing.T) {
	db := store.NewTestDB(t)
	seedPortraitChar(t, db, "NoPortrait", nil, "") // char exists, no portrait row

	h := readapi.NewPortrait(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, serveReq(http.MethodGet, "NoPortrait"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("portrait-less char status = %d, want 404", rec.Code)
	}
}

// TestPortraitServe_UnknownChar: an unknown char name → 404 (never an existence leak).
func TestPortraitServe_UnknownChar(t *testing.T) {
	db := store.NewTestDB(t)

	h := readapi.NewPortrait(store.NewStore(db))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, serveReq(http.MethodGet, "NoSuchChar"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown char status = %d, want 404", rec.Code)
	}
}

// TestPortraitServe_WrongMethod405: a non-GET method → 405.
func TestPortraitServe_WrongMethod405(t *testing.T) {
	db := store.NewTestDB(t)
	seedPortraitChar(t, db, "Slampeach", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png")

	h := readapi.NewPortrait(store.NewStore(db))
	rec := httptest.NewRecorder()
	req := serveReq(http.MethodPost, "Slampeach")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST on serve handler = %d, want 405", rec.Code)
	}
}

// TestPortraitServe_RequireSession401 proves the BLOCKING gate: registered under
// webauth.RequireSession, the route returns 401 (not the inner 200/404) with no session
// cookie — fail-closed at the API, the SAME wrap production applies.
func TestPortraitServe_RequireSession401(t *testing.T) {
	db := store.NewTestDB(t)
	seedPortraitChar(t, db, "Slampeach", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png")

	// The SAME wrap production applies (cmd/squirebot-server/main.go).
	h := webauth.RequireSession(db, readapi.NewPortrait(store.NewStore(db)))
	rec := httptest.NewRecorder()
	// No sb_session cookie → RequireSession must reject with 401.
	h.ServeHTTP(rec, serveReq(http.MethodGet, "Slampeach"))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no-session serve = %d, want 401 (RequireSession fail-closed)", rec.Code)
	}
}
