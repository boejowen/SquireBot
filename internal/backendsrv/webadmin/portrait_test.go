package webadmin

// portrait_test.go — Phase 41 plan 41-01 (TDD). Proves the portrait upload/delete write
// backend (CHARUI-02): a base64 PNG/JPEG/WebP is decoded, SNIFFED (content_type set from the
// magic bytes, never the client claim), size-capped at 256KB, stored + audited under one tx;
// SVG/GIF/oversize/malformed input is rejected with distinct 4xx codes; the delete audits
// "portrait_removed"; and the store's ErrNotAuthorized/ErrCharNotFound map to 403/400. The
// route-level RequireSession gate is asserted separately (readapi serve test + main_test);
// here the caller is injected via withCaller (a seeded assignee) and the char is seeded with
// a character_assignment row so the store gate passes.
//
// Shared helpers reused from officers_test.go / coin_test.go (same package): withCaller,
// decodeErr, auditCount, seedPlainMember, store.NewTestDB.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// portraitSeedChar inserts a plain character under a throwaway owner + assigns it to
// assignee (a seeded web_user) so the store's assignee-OR-officer gate passes. Returns the
// character id.
func portraitSeedChar(t *testing.T, ctx context.Context, db *sql.DB, name, assignee string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
	if err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	ownerID, _ := res.LastInsertId()
	res, err = db.ExecContext(ctx, `INSERT INTO character (owner_id, name) VALUES (?, ?)`, ownerID, name)
	if err != nil {
		t.Fatalf("insert char %q: %v", name, err)
	}
	charID, _ := res.LastInsertId()
	// Seed the assignee web_user + the assignment so store.SetPortraitTx's gate admits them.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, assignee, "Assignee"); err != nil {
		t.Fatalf("insert web_user %q: %v", assignee, err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO character_assignment (character_id, discord_user_id, assigned_at, assigned_by)
		 VALUES (?, ?, 0, 'test')`, charID, assignee); err != nil {
		t.Fatalf("seed assignment (char=%d): %v", charID, err)
	}
	return charID
}

// postPortrait POSTs a base64 body to the {name}-path upload handler with the caller
// injected. It sets the {name} path value directly (the production route binds it from the
// ServeMux wildcard).
func postPortrait(t *testing.T, handler http.Handler, name, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/characters/"+name+"/portrait", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("name", name)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// deletePortrait sends a DELETE to the {name}-path delete handler with the caller injected.
func deletePortrait(t *testing.T, handler http.Handler, name string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/characters/"+name+"/portrait", nil)
	req.SetPathValue("name", name)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// b64 encodes raw bytes as standard base64 (the browser's encoding).
func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// storedPortrait reads back a char's stored blob + content_type (proves the sniffed type
// landed, not the client claim).
func storedPortrait(t *testing.T, ctx context.Context, db *sql.DB, name string) ([]byte, string) {
	t.Helper()
	var blob []byte
	var ct string
	err := db.QueryRowContext(ctx,
		`SELECT p.image_blob, p.content_type FROM character_portrait p
		   JOIN character c ON c.id = p.character_id WHERE c.name = ?`, name).Scan(&blob, &ct)
	if err != nil {
		t.Fatalf("read stored portrait (%q): %v", name, err)
	}
	return blob, ct
}

// validPNG / validJPEG / validWebP are minimal buffers with the real magic-byte header (the
// sniff only reads the prefix; a header + a few bytes suffices).
func validPNG() []byte  { return append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, 0x00, 0x01, 0x02, 0x03) }
func validJPEG() []byte { return append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, 0x00, 0x10, 0x4A, 0x46) }
func validWebP() []byte {
	b := []byte("RIFF")
	b = append(b, 0x24, 0x00, 0x00, 0x00) // file size (little-endian; value irrelevant to the sniff)
	b = append(b, []byte("WEBP")...)
	b = append(b, []byte("VP8 ")...)
	return b
}

func TestPortraitSet_ValidPNG(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(validPNG())+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	blob, ct := storedPortrait(t, ctx, db, "Slampeach")
	if ct != "image/png" {
		t.Errorf("stored content_type = %q, want image/png (sniffed, not client)", ct)
	}
	if len(blob) == 0 {
		t.Errorf("stored blob is empty")
	}
	if c := auditCount(t, ctx, db, "portrait_set"); c != 1 {
		t.Errorf("portrait_set audit rows = %d, want 1", c)
	}
}

func TestPortraitSet_ValidJPEG(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(validJPEG())+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if _, ct := storedPortrait(t, ctx, db, "Slampeach"); ct != "image/jpeg" {
		t.Errorf("stored content_type = %q, want image/jpeg", ct)
	}
}

func TestPortraitSet_ValidWebP(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(validWebP())+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if _, ct := storedPortrait(t, ctx, db, "Slampeach"); ct != "image/webp" {
		t.Errorf("stored content_type = %q, want image/webp", ct)
	}
}

func TestPortraitSet_RejectsSVG(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(svg)+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (SVG rejected)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_image" {
		t.Errorf("error = %q, want invalid_image", got)
	}
	if c := auditCount(t, ctx, db, "portrait_set"); c != 0 {
		t.Errorf("rejected SVG audited %d rows, want 0", c)
	}
}

func TestPortraitSet_RejectsGIF(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	gif := append([]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, 0x00, 0x01) // "GIF89a"
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(gif)+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (GIF rejected)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_image" {
		t.Errorf("error = %q, want invalid_image", got)
	}
}

func TestPortraitSet_RejectsOversize(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	// A valid-PNG header followed by > 256KB of body → decoded length exceeds the cap.
	big := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, maxPortraitBytes+1)...)
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(big)+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (oversize)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "too_large" {
		t.Errorf("error = %q, want too_large", got)
	}
}

func TestPortraitSet_RejectsMalformedBase64(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"!!!not-base64!!!"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (malformed base64)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
}

func TestPortraitSet_RejectsEmptyBody(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	// Missing image_base64.
	rec := postPortrait(t, h, "Slampeach", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing image_base64)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
}

func TestPortraitSet_StrangerMapped403(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	stranger := "999999999999999999"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)
	seedPlainMember(t, ctx, db, stranger, "Stranger")

	h := withCaller(stranger, PortraitSetHandler(db))
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(validPNG())+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (stranger)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "not_authorized" {
		t.Errorf("error = %q, want not_authorized", got)
	}
}

func TestPortraitSet_UnknownCharMapped400(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	member := "555555555555555555"
	seedPlainMember(t, ctx, db, member, "PlainMember")

	h := withCaller(member, PortraitSetHandler(db))
	rec := postPortrait(t, h, "NoSuchChar", `{"image_base64":"`+b64(validPNG())+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (unknown char)", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Errorf("error = %q, want invalid_input", got)
	}
}

func TestPortraitDelete_AuditsRemoval(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	setH := withCaller(assignee, PortraitSetHandler(db))
	if rec := postPortrait(t, setH, "Slampeach", `{"image_base64":"`+b64(validPNG())+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("seed upload status = %d, want 200", rec.Code)
	}

	delH := withCaller(assignee, PortraitDeleteHandler(db))
	rec := deletePortrait(t, delH, "Slampeach")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if c := auditCount(t, ctx, db, "portrait_removed"); c != 1 {
		t.Errorf("portrait_removed audit rows = %d, want 1", c)
	}
	// The row is gone.
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM character_portrait p JOIN character c ON c.id = p.character_id WHERE c.name = ?`,
		"Slampeach").Scan(&n); err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if n != 0 {
		t.Errorf("portrait rows after delete = %d, want 0", n)
	}
}

func TestPortraitSet_WrongMethod405(t *testing.T) {
	db := store.NewTestDB(t)
	h := withCaller("x", PortraitSetHandler(db))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/characters/Slampeach/portrait", nil)
	req.SetPathValue("name", "Slampeach")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET on set handler = %d, want 405", rec.Code)
	}
}

// unusedJSON keeps json imported if a future assertion decodes the success body; the set
// handler returns {character, updated_at}. Assert its shape here.
func TestPortraitSet_SuccessBodyShape(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	assignee := "555555555555555555"
	portraitSeedChar(t, ctx, db, "Slampeach", assignee)

	h := withCaller(assignee, PortraitSetHandler(db))
	rec := postPortrait(t, h, "Slampeach", `{"image_base64":"`+b64(validPNG())+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Character string `json:"character"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode success body: %v", err)
	}
	if body.Character != "Slampeach" || body.UpdatedAt == "" {
		t.Errorf("success body = %+v, want character=Slampeach + non-empty updated_at", body)
	}
}
