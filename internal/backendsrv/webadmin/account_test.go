package webadmin

// account_test.go covers the self-service watcher-linking handlers (17-02 Task 2):
// mint derives the owner from the session and returns a plaintext code (hash-only
// at rest), an ambiguous resolve refuses with 409, list is owner-scoped with
// sequential #N ordinals (and [] for a never-minted caller), and a cross-owner
// revoke is a silent no-op that never touches another owner's code (IDOR guard).
//
// The handlers read the acting discord_user_id via webauth.UserFromContext; these
// unit tests inject it with withCaller (officers_test.go) without standing up the
// session machinery (route-level RequireSession is covered by main_test.go).

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// seedWebUser inserts a minimal web_user row (the resolve-or-create username key).
func seedWebUser(t *testing.T, ctx context.Context, db *sql.DB, id, username string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, id, username); err != nil {
		t.Fatalf("insert web_user %q: %v", id, err)
	}
}

// seedOwner inserts an owner row (optionally stamped with a discord_user_id) and
// returns its id. discordID == "" leaves the FK NULL (unlinked).
func seedOwner(t *testing.T, ctx context.Context, db *sql.DB, label, discordID string) int64 {
	t.Helper()
	var res sql.Result
	var err error
	if discordID == "" {
		res, err = db.ExecContext(ctx, `INSERT INTO owner (label) VALUES (?)`, label)
	} else {
		res, err = db.ExecContext(ctx,
			`INSERT INTO owner (label, discord_user_id) VALUES (?, ?)`, label, discordID)
	}
	if err != nil {
		t.Fatalf("insert owner %q: %v", label, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("owner last insert id: %v", err)
	}
	return id
}

// (activeCodeCount and itoa are shared helpers defined in eviction_test.go.)

func TestMintOwnCode_ReturnsPlaintext_HashOnlyAtRest_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-mint"
	seedWebUser(t, ctx, db, caller, "Slampeach")

	h := withCaller(caller, MintOwnCodeHandler(db))
	rec := postJSON(t, h, `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode mint resp: %v", err)
	}
	if resp.Code == "" {
		t.Fatalf("mint returned empty code")
	}

	// A fresh owner was created+stamped, and a guild_code row exists — hash-only:
	// the plaintext must NOT appear in token_hash or label.
	var ownerID int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM owner WHERE discord_user_id = ?`, caller).Scan(&ownerID); err != nil {
		t.Fatalf("resolve created owner: %v", err)
	}
	if activeCodeCount(t, ctx, db, ownerID) != 1 {
		t.Fatalf("expected exactly 1 active code for the new owner")
	}
	var hash []byte
	var label sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT token_hash, label FROM guild_code WHERE owner_id = ?`, ownerID).Scan(&hash, &label); err != nil {
		t.Fatalf("read guild_code: %v", err)
	}
	if string(hash) == resp.Code {
		t.Fatalf("token_hash stored the PLAINTEXT — must be hash-only at rest")
	}
	if label.Valid && label.String == resp.Code {
		t.Fatalf("label stored the PLAINTEXT — must never persist the token")
	}
	if c := auditCount(t, ctx, db, "code_mint"); c != 1 {
		t.Fatalf("code_mint audit rows = %d, want 1", c)
	}

	// A second mint is ADDITIVE (a new code, the first stays active) — LINK-03.
	rec2 := postJSON(t, h, `{}`)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second mint status = %d, want 200", rec2.Code)
	}
	if activeCodeCount(t, ctx, db, ownerID) != 2 {
		t.Fatalf("second mint not additive: active code count = %d, want 2", activeCodeCount(t, ctx, db, ownerID))
	}
}

func TestMintOwnCode_AmbiguousResolve_409(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-amb"
	seedWebUser(t, ctx, db, caller, "Twins")
	// Two unlinked owners whose label matches the username → ambiguous.
	seedOwner(t, ctx, db, "Twins", "")
	seedOwner(t, ctx, db, "twins", "")

	h := withCaller(caller, MintOwnCodeHandler(db))
	rec := postJSON(t, h, `{}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "ambiguous_owner" {
		t.Errorf("error = %q, want ambiguous_owner", got)
	}
	// No code was minted under either owner.
	if c := auditCount(t, ctx, db, "code_mint"); c != 0 {
		t.Errorf("code_mint audit rows = %d, want 0 (refused)", c)
	}
}

func TestListOwnCodes_OwnerScoped_SequentialOrdinals_AndEmpty(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-list"
	seedWebUser(t, ctx, db, caller, "Lister")

	// Never minted → [].
	listH := withCaller(caller, ListOwnCodesHandler(db))
	rec := httptest.NewRecorder()
	listH.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("empty list status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "[]\n" && body != "[]" {
		t.Fatalf("never-minted list body = %q, want []", body)
	}

	// Mint two codes for the caller, plus a code for ANOTHER owner that must not show.
	mintH := withCaller(caller, MintOwnCodeHandler(db))
	if r := postJSON(t, mintH, `{}`); r.Code != http.StatusOK {
		t.Fatalf("mint 1 status = %d", r.Code)
	}
	if r := postJSON(t, mintH, `{}`); r.Code != http.StatusOK {
		t.Fatalf("mint 2 status = %d", r.Code)
	}
	seedWebUser(t, ctx, db, "disc-other", "Other")
	other := seedOwner(t, ctx, db, "Other", "disc-other")
	if _, err := db.ExecContext(ctx,
		`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)`,
		other, []byte("hash-other"), nil); err != nil {
		t.Fatalf("seed other code: %v", err)
	}

	rec2 := httptest.NewRecorder()
	listH.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec2.Code)
	}
	var codes []struct {
		ID       int64   `json:"id"`
		Ordinal  int     `json:"ordinal"`
		LastSeen *string `json:"last_seen"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &codes); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("list returned %d codes, want 2 (caller-scoped, other owner excluded)", len(codes))
	}
	if codes[0].Ordinal != 1 || codes[1].Ordinal != 2 {
		t.Errorf("ordinals = %d,%d; want 1,2 (sequential #N)", codes[0].Ordinal, codes[1].Ordinal)
	}
	if codes[0].LastSeen != nil {
		t.Errorf("last_seen = %v, want null (never used yet)", *codes[0].LastSeen)
	}
}

func TestRevokeOwnCode_CrossOwnerNoOp_OwnCodeRevoked(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	caller := "disc-rev"
	seedWebUser(t, ctx, db, caller, "Revoker")

	// Caller mints one code.
	mintH := withCaller(caller, MintOwnCodeHandler(db))
	if r := postJSON(t, mintH, `{}`); r.Code != http.StatusOK {
		t.Fatalf("mint status = %d", r.Code)
	}
	var callerOwner int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM owner WHERE discord_user_id = ?`, caller).Scan(&callerOwner); err != nil {
		t.Fatalf("resolve caller owner: %v", err)
	}

	// Another owner with an active code.
	seedWebUser(t, ctx, db, "disc-other", "Other")
	other := seedOwner(t, ctx, db, "Other", "disc-other")
	if _, err := db.ExecContext(ctx,
		`INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)`,
		other, []byte("hash-other"), nil); err != nil {
		t.Fatalf("seed other code: %v", err)
	}
	var otherCodeID int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM guild_code WHERE owner_id = ?`, other).Scan(&otherCodeID); err != nil {
		t.Fatalf("read other code id: %v", err)
	}

	// Caller tries to revoke the OTHER owner's code → revoked:false, other code stays active.
	revH := withCaller(caller, RevokeOwnCodeHandler(db))
	rec := postJSON(t, revH, `{"id":`+itoa(otherCodeID)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("cross-owner revoke status = %d, want 200", rec.Code)
	}
	if got := decodeRevoked(t, rec); got {
		t.Fatalf("cross-owner revoke returned revoked=true — IDOR")
	}
	if activeCodeCount(t, ctx, db, other) != 1 {
		t.Fatalf("other owner's code was disabled — IDOR")
	}
	if c := auditCount(t, ctx, db, "code_revoke"); c != 0 {
		t.Errorf("code_revoke audit rows = %d, want 0 (no-op was not audited)", c)
	}

	// Caller revokes its OWN code → revoked:true, audited.
	var ownCodeID int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM guild_code WHERE owner_id = ?`, callerOwner).Scan(&ownCodeID); err != nil {
		t.Fatalf("read own code id: %v", err)
	}
	rec2 := postJSON(t, revH, `{"id":`+itoa(ownCodeID)+`}`)
	if rec2.Code != http.StatusOK {
		t.Fatalf("own revoke status = %d, want 200", rec2.Code)
	}
	if !decodeRevoked(t, rec2) {
		t.Fatalf("own revoke returned revoked=false, want true")
	}
	if activeCodeCount(t, ctx, db, callerOwner) != 0 {
		t.Fatalf("own code not revoked")
	}
	if c := auditCount(t, ctx, db, "code_revoke"); c != 1 {
		t.Errorf("code_revoke audit rows = %d, want 1", c)
	}
}

// decodeRevoked extracts the {"revoked":bool} field.
func decodeRevoked(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var body struct {
		Revoked bool `json:"revoked"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode revoked body %q: %v", rec.Body.String(), err)
	}
	return body.Revoked
}
