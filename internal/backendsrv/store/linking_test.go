package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// linking_test.go covers the Phase 17 self-service-linking store funcs (17-02
// Task 1): the session-derived resolve-or-create-owner algorithm (D-03/D-04, with
// the mis-adoption / ambiguity refuse), the owner-scoped revoke (Pitfall 3 — no
// cross-owner IDOR), the owner-scoped active-codes list (#N source), and the
// best-effort last_seen stamp. The behavioral contract is the plan's <behavior>
// block; the security-critical case is RevokeOwnCodeTx never touching another
// owner's code.

// ownerDiscordID reads owner.discord_user_id for an owner id (NULL → "").
func ownerDiscordID(t *testing.T, ctx context.Context, db *sql.DB, ownerID int64) string {
	t.Helper()
	var v sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT discord_user_id FROM owner WHERE id = ?`, ownerID,
	).Scan(&v); err != nil {
		t.Fatalf("read owner discord_user_id (id=%d): %v", ownerID, err)
	}
	if v.Valid {
		return v.String
	}
	return ""
}

// codeDisabled reports whether the guild_code with the given label is disabled.
func codeDisabled(t *testing.T, ctx context.Context, db *sql.DB, label string) bool {
	t.Helper()
	var disabled sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT disabled_at FROM guild_code WHERE label = ?`, label,
	).Scan(&disabled); err != nil {
		t.Fatalf("read guild_code disabled_at (label=%q): %v", label, err)
	}
	return disabled.Valid
}

// codeIDByLabel returns the guild_code id for a label.
func codeIDByLabel(t *testing.T, ctx context.Context, db *sql.DB, label string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM guild_code WHERE label = ?`, label,
	).Scan(&id); err != nil {
		t.Fatalf("read guild_code id (label=%q): %v", label, err)
	}
	return id
}

func TestResolveOrCreateOwnerByDiscordTx_AlreadyLinked(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertWebUser(t, ctx, db, "disc-1", "Slampeach")
	ownerID := insertOwner(t, ctx, db, "Slampeach")
	// Stamp it already-linked.
	if _, err := db.ExecContext(ctx,
		`UPDATE owner SET discord_user_id = ? WHERE id = ?`, "disc-1", ownerID); err != nil {
		t.Fatalf("pre-link owner: %v", err)
	}

	var got int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		got, e = ResolveOrCreateOwnerByDiscordTx(ctx, tx, "disc-1")
		return e
	}); err != nil {
		t.Fatalf("resolve already-linked: %v", err)
	}
	if got != ownerID {
		t.Fatalf("already-linked: got owner %d, want %d", got, ownerID)
	}
}

func TestResolveOrCreateOwnerByDiscordTx_AdoptExactlyOneLabelMatch(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertWebUser(t, ctx, db, "disc-2", "Slampeach")
	// An unlinked owner whose label matches the username (case/whitespace drift).
	ownerID := insertOwner(t, ctx, db, "  slampeach ")

	var got int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		got, e = ResolveOrCreateOwnerByDiscordTx(ctx, tx, "disc-2")
		return e
	}); err != nil {
		t.Fatalf("resolve adopt: %v", err)
	}
	if got != ownerID {
		t.Fatalf("adopt: got owner %d, want %d (the existing label match)", got, ownerID)
	}
	if d := ownerDiscordID(t, ctx, db, ownerID); d != "disc-2" {
		t.Fatalf("adopt: owner not stamped with discord_user_id; got %q want %q", d, "disc-2")
	}
}

func TestResolveOrCreateOwnerByDiscordTx_CreateFreshOnZeroMatch(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertWebUser(t, ctx, db, "disc-3", "Newbie")
	// No owner with a matching label.
	insertOwner(t, ctx, db, "SomeoneElse")

	var got int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		got, e = ResolveOrCreateOwnerByDiscordTx(ctx, tx, "disc-3")
		return e
	}); err != nil {
		t.Fatalf("resolve create-fresh: %v", err)
	}
	if got == 0 {
		t.Fatalf("create-fresh: got zero owner id")
	}
	// The fresh owner is labeled with the username and stamped with the caller.
	var label string
	if err := db.QueryRowContext(ctx,
		`SELECT label FROM owner WHERE id = ?`, got).Scan(&label); err != nil {
		t.Fatalf("read fresh owner label: %v", err)
	}
	if label != "Newbie" {
		t.Fatalf("create-fresh: label = %q, want %q", label, "Newbie")
	}
	if d := ownerDiscordID(t, ctx, db, got); d != "disc-3" {
		t.Fatalf("create-fresh: discord_user_id = %q, want %q", d, "disc-3")
	}
}

func TestResolveOrCreateOwnerByDiscordTx_AmbiguousMultipleMatches(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertWebUser(t, ctx, db, "disc-4", "Twins")
	insertOwner(t, ctx, db, "Twins")
	insertOwner(t, ctx, db, "twins") // a second unlinked label match (case-insensitive)

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := ResolveOrCreateOwnerByDiscordTx(ctx, tx, "disc-4")
		return e
	})
	if !errors.Is(err, ErrAmbiguousOwner) {
		t.Fatalf("multiple matches: got err %v, want ErrAmbiguousOwner", err)
	}
}

func TestResolveOrCreateOwnerByDiscordTx_RefusesLabelMatchStampedByOther(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	insertWebUser(t, ctx, db, "disc-5", "Shared")
	insertWebUser(t, ctx, db, "disc-other", "OtherName")
	ownerID := insertOwner(t, ctx, db, "Shared")
	// The only label match is already stamped by a DIFFERENT discord id.
	if _, err := db.ExecContext(ctx,
		`UPDATE owner SET discord_user_id = ? WHERE id = ?`, "disc-other", ownerID); err != nil {
		t.Fatalf("pre-stamp owner: %v", err)
	}

	err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := ResolveOrCreateOwnerByDiscordTx(ctx, tx, "disc-5")
		return e
	})
	if !errors.Is(err, ErrAmbiguousOwner) {
		t.Fatalf("stamped-by-other: got err %v, want ErrAmbiguousOwner", err)
	}
}

func TestListOwnCodes_OwnerScopedActiveOnly(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	a := insertOwner(t, ctx, db, "OwnerA")
	b := insertOwner(t, ctx, db, "OwnerB")
	insertGuildCode(t, ctx, db, a, "a1")
	insertGuildCode(t, ctx, db, a, "a2")
	insertGuildCode(t, ctx, db, b, "b1")
	// A revoked code for A must not show.
	insertGuildCode(t, ctx, db, a, "a3-revoked")
	if _, err := db.ExecContext(ctx,
		`UPDATE guild_code SET disabled_at = datetime('now') WHERE label = ?`, "a3-revoked"); err != nil {
		t.Fatalf("revoke a3: %v", err)
	}

	codes, err := ListOwnCodes(ctx, db, a)
	if err != nil {
		t.Fatalf("list own codes: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("list: got %d codes, want 2 (active, owner-A-only)", len(codes))
	}
	// Never-minted owner → non-nil empty slice (so the handler emits []).
	empty, err := ListOwnCodes(ctx, db, 99999)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if empty == nil {
		t.Fatalf("list empty: got nil slice, want non-nil empty so JSON is [] not null")
	}
	if len(empty) != 0 {
		t.Fatalf("list empty: got %d codes, want 0", len(empty))
	}
}

func TestRevokeOwnCodeTx_CallerScopedNoOpForForeignCode(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	a := insertOwner(t, ctx, db, "OwnerA")
	b := insertOwner(t, ctx, db, "OwnerB")
	insertGuildCode(t, ctx, db, a, "a-code")
	insertGuildCode(t, ctx, db, b, "b-code")
	bCodeID := codeIDByLabel(t, ctx, db, "b-code")

	// Owner A attempts to revoke owner B's code → no-op, no error, B's code stays active.
	var revoked bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		revoked, e = RevokeOwnCodeTx(ctx, tx, bCodeID, a)
		return e
	}); err != nil {
		t.Fatalf("cross-owner revoke errored (should be a silent no-op): %v", err)
	}
	if revoked {
		t.Fatalf("cross-owner revoke: got revoked=true, want false (must not touch another owner's code)")
	}
	if codeDisabled(t, ctx, db, "b-code") {
		t.Fatalf("cross-owner revoke: B's code was disabled — IDOR")
	}

	// Owner A revokes its OWN code → revoked=true, code disabled.
	aCodeID := codeIDByLabel(t, ctx, db, "a-code")
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		revoked, e = RevokeOwnCodeTx(ctx, tx, aCodeID, a)
		return e
	}); err != nil {
		t.Fatalf("own revoke: %v", err)
	}
	if !revoked {
		t.Fatalf("own revoke: got revoked=false, want true")
	}
	if !codeDisabled(t, ctx, db, "a-code") {
		t.Fatalf("own revoke: A's code not disabled")
	}

	// Revoking an already-revoked code → idempotent no-op (false, nil).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		revoked, e = RevokeOwnCodeTx(ctx, tx, aCodeID, a)
		return e
	}); err != nil {
		t.Fatalf("re-revoke errored: %v", err)
	}
	if revoked {
		t.Fatalf("re-revoke: got revoked=true, want false (idempotent no-op)")
	}
}

func TestStampCodeLastSeen_SetsLastSeen(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	a := insertOwner(t, ctx, db, "OwnerA")
	insertGuildCode(t, ctx, db, a, "a-code")
	codeID := codeIDByLabel(t, ctx, db, "a-code")

	// Pre-stamp last_seen is NULL.
	var before sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT last_seen FROM guild_code WHERE id = ?`, codeID).Scan(&before); err != nil {
		t.Fatalf("read last_seen before: %v", err)
	}
	if before.Valid {
		t.Fatalf("last_seen should start NULL")
	}

	if err := StampCodeLastSeen(ctx, db, codeID); err != nil {
		t.Fatalf("stamp last_seen: %v", err)
	}

	var after sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT last_seen FROM guild_code WHERE id = ?`, codeID).Scan(&after); err != nil {
		t.Fatalf("read last_seen after: %v", err)
	}
	if !after.Valid {
		t.Fatalf("last_seen still NULL after stamp")
	}
}
