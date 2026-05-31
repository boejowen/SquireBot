package store

import (
	"context"
	"errors"
	"testing"
)

// websession_test.go covers the Discord-login session + web-user store methods
// (15-01 Task 2) over the shared NewTestDB fixture (00004 creates web_user +
// web_session). The load-bearing security assertion is T-15-01: web_session
// stores ONLY sha256(sessionID) — the plaintext is never persisted, so a DB
// leak yields no usable session tokens.

const (
	testDiscordID = "123456789012345678" // a Discord snowflake (string)
	testUsername  = "Slampeach"
	testAvatar    = "abc123avatarhash"
)

func TestHashSession_DeterministicAndNotPlaintext(t *testing.T) {
	id := "deadbeef"
	h1 := HashSession(id)
	h2 := HashSession(id)
	if h1 != h2 {
		t.Fatalf("HashSession not deterministic: %q != %q", h1, h2)
	}
	if h1 == id {
		t.Fatalf("HashSession returned the plaintext unchanged: %q", h1)
	}
	if len(h1) != 64 { // sha256 hex = 32 bytes = 64 hex chars
		t.Fatalf("HashSession length = %d, want 64 (sha256 hex)", len(h1))
	}
}

func TestGenerateSessionID_UniqueAndHex(t *testing.T) {
	a, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID: %v", err)
	}
	b, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID: %v", err)
	}
	if a == b {
		t.Fatalf("GenerateSessionID returned identical ids twice: %q", a)
	}
	if len(a) != 64 { // 32 random bytes hex-encoded
		t.Fatalf("GenerateSessionID length = %d, want 64 (32 bytes hex)", len(a))
	}
}

func TestSessionTTLSeconds_Is30Days(t *testing.T) {
	if SessionTTLSeconds != 2592000 {
		t.Fatalf("SessionTTLSeconds = %d, want 2592000 (30 days)", SessionTTLSeconds)
	}
}

func TestCreateSession_StoresOnlyHashNotPlaintext(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	if err := UpsertWebUser(ctx, db, testDiscordID, testUsername, testAvatar, now); err != nil {
		t.Fatalf("UpsertWebUser: %v", err)
	}
	sid, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID: %v", err)
	}
	if err := CreateSession(ctx, db, testDiscordID, sid, now, SessionTTLSeconds); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// The raw plaintext id must NOT be present in session_hash.
	var nPlain int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM web_session WHERE session_hash = ?`, sid).Scan(&nPlain); err != nil {
		t.Fatalf("count by plaintext: %v", err)
	}
	if nPlain != 0 {
		t.Errorf("found %d rows whose session_hash == the raw session id; plaintext must never be stored", nPlain)
	}

	// The hash IS present.
	var nHash int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM web_session WHERE session_hash = ?`, HashSession(sid)).Scan(&nHash); err != nil {
		t.Fatalf("count by hash: %v", err)
	}
	if nHash != 1 {
		t.Errorf("expected exactly 1 row stored under HashSession(id), got %d", nHash)
	}
}

func TestResolveSession_LiveExpiredMissing(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	if err := UpsertWebUser(ctx, db, testDiscordID, testUsername, testAvatar, now); err != nil {
		t.Fatalf("UpsertWebUser: %v", err)
	}
	sid, _ := GenerateSessionID()
	if err := CreateSession(ctx, db, testDiscordID, sid, now, SessionTTLSeconds); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Live session resolves to the discord_user_id.
	got, err := ResolveSession(ctx, db, sid, now+100)
	if err != nil {
		t.Fatalf("ResolveSession (live): %v", err)
	}
	if got != testDiscordID {
		t.Errorf("ResolveSession returned %q, want %q", got, testDiscordID)
	}

	// Expired session → ErrSessionExpired (fail-closed).
	_, err = ResolveSession(ctx, db, sid, now+SessionTTLSeconds+1)
	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("ResolveSession on expired row: err = %v, want ErrSessionExpired", err)
	}

	// Missing session → ErrSessionNotFound.
	_, err = ResolveSession(ctx, db, "nonexistent-session-id", now)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ResolveSession on missing row: err = %v, want ErrSessionNotFound", err)
	}
}

func TestTouchSession_RollsExpiry(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	if err := UpsertWebUser(ctx, db, testDiscordID, testUsername, testAvatar, now); err != nil {
		t.Fatalf("UpsertWebUser: %v", err)
	}
	sid, _ := GenerateSessionID()
	if err := CreateSession(ctx, db, testDiscordID, sid, now, SessionTTLSeconds); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Bump the expiry far into the future, then a far-future resolve still works.
	newExpiry := now + SessionTTLSeconds + 100000
	if err := TouchSession(ctx, db, sid, newExpiry); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}
	if _, err := ResolveSession(ctx, db, sid, now+SessionTTLSeconds+1); err != nil {
		t.Errorf("ResolveSession after TouchSession should succeed (rolled window), got %v", err)
	}
}

func TestUpsertWebUser_IdempotentFirstSeenUnchanged(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	firstNow := int64(1700000000)
	laterNow := int64(1700009999)

	if err := UpsertWebUser(ctx, db, testDiscordID, testUsername, testAvatar, firstNow); err != nil {
		t.Fatalf("UpsertWebUser (first): %v", err)
	}
	// Second login updates username/avatar/last_login; first_seen stays.
	if err := UpsertWebUser(ctx, db, testDiscordID, "RenamedToon", "newavatar", laterNow); err != nil {
		t.Fatalf("UpsertWebUser (second): %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM web_user WHERE discord_user_id = ?`, testDiscordID).Scan(&count); err != nil {
		t.Fatalf("count web_user: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 web_user row after two upserts, got %d", count)
	}

	var firstSeen, lastLogin int64
	var username string
	if err := db.QueryRowContext(ctx,
		`SELECT username, first_seen, last_login FROM web_user WHERE discord_user_id = ?`, testDiscordID,
	).Scan(&username, &firstSeen, &lastLogin); err != nil {
		t.Fatalf("read web_user: %v", err)
	}
	if firstSeen != firstNow {
		t.Errorf("first_seen = %d, want %d (must NOT change on re-login)", firstSeen, firstNow)
	}
	if lastLogin != laterNow {
		t.Errorf("last_login = %d, want %d (must update on re-login)", lastLogin, laterNow)
	}
	if username != "RenamedToon" {
		t.Errorf("username = %q, want %q (must update on re-login)", username, "RenamedToon")
	}
}

func TestGetWebUser_ReturnsUsernameAndAvatar(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	if err := UpsertWebUser(ctx, db, testDiscordID, testUsername, testAvatar, now); err != nil {
		t.Fatalf("UpsertWebUser: %v", err)
	}
	username, avatar, err := GetWebUser(ctx, db, testDiscordID)
	if err != nil {
		t.Fatalf("GetWebUser: %v", err)
	}
	if username != testUsername {
		t.Errorf("username = %q, want %q", username, testUsername)
	}
	if avatar == nil || *avatar != testAvatar {
		t.Errorf("avatar = %v, want %q", avatar, testAvatar)
	}

	// Missing user → ErrSessionNotFound-style; we expect sql.ErrNoRows-wrapped.
	if _, _, err := GetWebUser(ctx, db, "no-such-user"); err == nil {
		t.Errorf("GetWebUser on missing user: err = nil, want non-nil")
	}
}

func TestDeleteSession_RemovesMatchingRow(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	now := int64(1700000000)

	if err := UpsertWebUser(ctx, db, testDiscordID, testUsername, testAvatar, now); err != nil {
		t.Fatalf("UpsertWebUser: %v", err)
	}
	sid, _ := GenerateSessionID()
	if err := CreateSession(ctx, db, testDiscordID, sid, now, SessionTTLSeconds); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := DeleteSession(ctx, db, sid); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := ResolveSession(ctx, db, sid, now); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("after DeleteSession, ResolveSession err = %v, want ErrSessionNotFound", err)
	}
}
