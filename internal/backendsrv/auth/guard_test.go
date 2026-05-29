package auth

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// TestResolveToken_Table exercises every authentication branch the guard must
// distinguish (D-08 / V2). All non-match branches MUST return (0, false) so the
// 11-05 handler maps them to 401 writing nothing, and no branch may leak which
// failure occurred.
func TestResolveToken_Table(t *testing.T) {
	db := store.NewTestDB(t)
	a := &Auth{db: db}
	ctx := context.Background()

	// A valid, active code for owner "Valid".
	validCode, err := MintCode(db, "Valid")
	if err != nil {
		t.Fatalf("mint valid code: %v", err)
	}

	// A code we mint then revoke — must NOT authenticate.
	revokedCode, err := MintCode(db, "Revoked")
	if err != nil {
		t.Fatalf("mint code to revoke: %v", err)
	}
	if err := RevokeCode(db, "Revoked"); err != nil {
		t.Fatalf("revoke code: %v", err)
	}

	cases := []struct {
		name      string
		header    string
		wantOK    bool
		wantOwner bool // true => expect a non-zero ownerID
	}{
		{"valid bearer code", "Bearer " + validCode, true, true},
		{"missing bearer prefix", validCode, false, false},
		{"empty header", "", false, false},
		{"bearer prefix only, no code", "Bearer ", false, false},
		{"unknown code", "Bearer " + strings.Repeat("A", 43), false, false},
		{"revoked code", "Bearer " + revokedCode, false, false},
		{"wrong scheme", "Basic " + validCode, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotOwner, gotOK := a.resolveToken(ctx, tc.header)
			if gotOK != tc.wantOK {
				t.Fatalf("resolveToken(%q) ok = %v, want %v", tc.header, gotOK, tc.wantOK)
			}
			if tc.wantOwner && gotOwner == 0 {
				t.Fatalf("resolveToken(%q) ownerID = 0, want non-zero", tc.header)
			}
			if !tc.wantOwner && gotOwner != 0 {
				t.Fatalf("resolveToken(%q) ownerID = %d, want 0", tc.header, gotOwner)
			}
		})
	}
}

// TestResolveToken_ReturnsMintingOwner proves a valid code resolves to the
// EXACT owner that minted it (not just "some" owner) — the basis for the
// first-sighting bind in 11-05.
func TestResolveToken_ReturnsMintingOwner(t *testing.T) {
	db := store.NewTestDB(t)
	a := &Auth{db: db}
	ctx := context.Background()

	// Mint codes for two distinct owners so a wrong-owner bug would show.
	if _, err := MintCode(db, "Bob"); err != nil {
		t.Fatalf("mint Bob: %v", err)
	}
	aliceCode, err := MintCode(db, "Alice")
	if err != nil {
		t.Fatalf("mint Alice: %v", err)
	}

	// Alice's expected owner id, read straight from the schema.
	var wantOwner int64
	if err := db.QueryRow(
		`SELECT id FROM owner WHERE label = ?`, "Alice").Scan(&wantOwner); err != nil {
		t.Fatalf("read Alice owner id: %v", err)
	}

	gotOwner, ok := a.resolveToken(ctx, "Bearer "+aliceCode)
	if !ok {
		t.Fatal("Alice's valid code did not resolve")
	}
	if gotOwner != wantOwner {
		t.Fatalf("resolveToken returned owner %d, want Alice's %d", gotOwner, wantOwner)
	}
}

// TestResolveToken_UsesConstantTimeCompare is a structural guard (timing is not
// deterministically unit-testable): the source MUST use
// subtle.ConstantTimeCompare, MUST filter to active rows (disabled_at IS NULL),
// MUST hash the presented code (sha256.Sum256), and the package MUST NOT import
// PocketBase's apis (guild codes are opaque static tokens, not PB auth records —
// so apis.RequireAuth is never used). The import check is parsed from the AST so
// a comment that merely NAMES apis.RequireAuth (to document that we avoid it)
// does not trip the assertion — only a real dependency would.
func TestResolveToken_UsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("guard.go")
	if err != nil {
		t.Fatalf("read guard.go: %v", err)
	}
	s := string(src)
	for _, want := range []string{
		"subtle.ConstantTimeCompare",
		"disabled_at IS NULL",
		"sha256.Sum256",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("guard.go must contain %q (security control)", want)
		}
	}

	// Parse the file and assert no PocketBase import path appears — robust
	// against the guard's own doc comment, which intentionally references
	// apis.RequireAuth to explain what it deliberately does NOT use.
	f, err := parser.ParseFile(token.NewFileSet(), "guard.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse guard.go imports: %v", err)
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(path, "pocketbase") {
			t.Errorf("guard.go must NOT import PocketBase (%q) — guild codes are opaque static tokens, not PB auth records", path)
		}
	}
}
