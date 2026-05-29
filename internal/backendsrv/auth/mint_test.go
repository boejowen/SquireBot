package auth

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// TestMintCode_StoresHashNotPlaintext is the hash-only-storage contract (D-05 /
// V6): the printed plaintext NEVER lands in the DB; guild_code.token_hash is
// exactly sha256(plaintext). Mirrors the watcher's "the secret never leaves as
// plaintext" discipline (internal/auth/store.go), backed by a SQLite BLOB
// instead of wincred.
func TestMintCode_StoresHashNotPlaintext(t *testing.T) {
	db := store.NewTestDB(t)

	plaintext, err := MintCode(db, "Bob")
	if err != nil {
		t.Fatalf("MintCode: %v", err)
	}
	if plaintext == "" {
		t.Fatal("MintCode returned an empty plaintext")
	}

	// The stored token_hash must equal sha256(plaintext), and the plaintext
	// itself must NOT be present anywhere in the token_hash column.
	want := sha256.Sum256([]byte(plaintext))

	var stored []byte
	if err := db.QueryRow(
		`SELECT token_hash FROM guild_code WHERE label = ?`, "Bob").Scan(&stored); err != nil {
		t.Fatalf("read back token_hash: %v", err)
	}
	if !bytes.Equal(stored, want[:]) {
		t.Fatalf("token_hash mismatch:\n got %x\nwant %x", stored, want[:])
	}
	if len(stored) != sha256.Size {
		t.Fatalf("token_hash is %d bytes, want %d (SHA-256)", len(stored), sha256.Size)
	}
	// Defensive: the raw plaintext bytes must not appear in the stored hash.
	if bytes.Contains(stored, []byte(plaintext)) {
		t.Fatal("plaintext bytes found inside token_hash — hash-only storage violated")
	}
}

// TestMintCode_DistinctTokens proves the 32-byte crypto/rand tokens have real
// entropy: two mints produce two different plaintexts (and two different
// hashes). A non-crypto RNG or a fixed seed would collide (T-11.04-06).
func TestMintCode_DistinctTokens(t *testing.T) {
	db := store.NewTestDB(t)

	a, err := MintCode(db, "Alice")
	if err != nil {
		t.Fatalf("MintCode(Alice): %v", err)
	}
	b, err := MintCode(db, "Alice")
	if err != nil {
		t.Fatalf("MintCode(Alice) again: %v", err)
	}
	if a == b {
		t.Fatalf("two mints produced identical plaintexts %q — no entropy", a)
	}
}

// TestMintCode_RoundTrip asserts the minted plaintext resolves by hash lookup
// against the stored row (the guard-side resolve is exercised in guard_test.go).
func TestMintCode_RoundTrip(t *testing.T) {
	db := store.NewTestDB(t)

	plaintext, err := MintCode(db, "Carol")
	if err != nil {
		t.Fatalf("MintCode: %v", err)
	}
	sum := sha256.Sum256([]byte(plaintext))

	var ownerID int64
	err = db.QueryRow(
		`SELECT owner_id FROM guild_code WHERE token_hash = ? AND disabled_at IS NULL`,
		sum[:]).Scan(&ownerID)
	if err != nil {
		t.Fatalf("round-trip hash lookup failed: %v", err)
	}
	if ownerID == 0 {
		t.Fatal("round-trip lookup returned owner_id 0")
	}
}

// TestMintCode_ReusesOwnerByLabel asserts upsertOwner does not create a new
// owner row for a label that already exists — two mints under the same label
// share one owner, but produce two distinct guild_code rows.
func TestMintCode_ReusesOwnerByLabel(t *testing.T) {
	db := store.NewTestDB(t)

	if _, err := MintCode(db, "Dave"); err != nil {
		t.Fatalf("MintCode(Dave): %v", err)
	}
	if _, err := MintCode(db, "Dave"); err != nil {
		t.Fatalf("MintCode(Dave) again: %v", err)
	}

	var owners int
	if err := db.QueryRow(`SELECT COUNT(*) FROM owner WHERE label = ?`, "Dave").Scan(&owners); err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if owners != 1 {
		t.Fatalf("expected 1 owner row for label Dave, got %d", owners)
	}

	var codes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM guild_code WHERE label = ?`, "Dave").Scan(&codes); err != nil {
		t.Fatalf("count codes: %v", err)
	}
	if codes != 2 {
		t.Fatalf("expected 2 guild_code rows for label Dave, got %d", codes)
	}
}

// TestRevokeCode_DisablesRow proves revocation sets disabled_at so the row is
// excluded from the active-codes candidate set (D-09 / T-11.04-04).
func TestRevokeCode_DisablesRow(t *testing.T) {
	db := store.NewTestDB(t)

	if _, err := MintCode(db, "Erin"); err != nil {
		t.Fatalf("MintCode(Erin): %v", err)
	}

	// Before revoke: disabled_at is NULL (active).
	var before sql.NullString
	if err := db.QueryRow(
		`SELECT disabled_at FROM guild_code WHERE label = ?`, "Erin").Scan(&before); err != nil {
		t.Fatalf("read disabled_at before revoke: %v", err)
	}
	if before.Valid {
		t.Fatalf("disabled_at should be NULL before revoke, got %q", before.String)
	}

	if err := RevokeCode(db, "Erin"); err != nil {
		t.Fatalf("RevokeCode(Erin): %v", err)
	}

	// After revoke: disabled_at is non-NULL.
	var after sql.NullString
	if err := db.QueryRow(
		`SELECT disabled_at FROM guild_code WHERE label = ?`, "Erin").Scan(&after); err != nil {
		t.Fatalf("read disabled_at after revoke: %v", err)
	}
	if !after.Valid {
		t.Fatal("disabled_at is still NULL after RevokeCode — row not disabled")
	}

	// No active row remains for that label.
	var active int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM guild_code WHERE label = ? AND disabled_at IS NULL`,
		"Erin").Scan(&active); err != nil {
		t.Fatalf("count active codes: %v", err)
	}
	if active != 0 {
		t.Fatalf("expected 0 active codes for Erin after revoke, got %d", active)
	}
}
