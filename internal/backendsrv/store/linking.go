package store

// linking.go is the self-service watcher-linking store layer (Phase 17 / LINK-01
// /02/03/05). It implements the session-derived resolve-or-create-owner algorithm
// (D-03/D-04) plus the owner-scoped code list/revoke and the per-code last_seen
// stamp. It builds on the 00005 migration's owner.discord_user_id FK→web_user and
// guild_code.last_seen.
//
// Security posture (the load-bearing parts):
//   - Owner is ALWAYS derived from the caller's session discord_user_id (D-02); no
//     function here accepts a free-text owner/label from a request.
//   - RevokeOwnCodeTx is OWNER-SCOPED (WHERE id=? AND owner_id=?) — a guildie can
//     never revoke another owner's code (RESEARCH Pitfall 3 / IDOR). Do NOT reuse
//     the intentionally-broad ops-CLI auth.RevokeCode for the web path.
//   - The mis-adoption guard (D-04): 2+ unlinked label matches OR a match already
//     stamped with a DIFFERENT discord_user_id → ErrAmbiguousOwner + slog.Warn,
//     never a silent attach to someone else's characters.
//   - Parameterized ? placeholders ONLY (V5); the token is NEVER logged (V7).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// ErrAmbiguousOwner is returned by ResolveOrCreateOwnerByDiscordTx when the
// caller's Discord username maps ambiguously to existing owner rows — either 2+
// unlinked label matches, or a single label match already stamped with a DIFFERENT
// discord_user_id (the mis-adoption guard, D-04). The handler maps it to HTTP 409
// (mapAccountErr). The string matches the HTTP error code the frontend routes off
// (the v1 sentinel-error convention — admins.go:37). No mint happens on this path.
var ErrAmbiguousOwner = errors.New("ambiguous_owner")

// OwnCode is one row of a guildie's own active-code list (ListOwnCodes). The
// handler assigns the 1-based #N ordinal over the (created_at-ordered) active set;
// it is NOT a stored column (RESEARCH A3 / D-06). LastSeen is NULL until the code's
// watcher next uploads ("never used yet").
type OwnCode struct {
	ID        int64
	CreatedAt string
	LastSeen  sql.NullString
}

// ResolveOrCreateOwnerByDiscordTx maps the caller's session discord_user_id to an
// owner.id, stamping the FK on first link (D-03/D-04). It runs ENTIRELY on the
// passed *sql.Tx (the handler's BEGIN IMMEDIATE mint tx) so resolve+mint are atomic
// and replayable; the partial unique index owner_discord_user_id_uidx is the
// backstop against a racing double-stamp.
//
// Algorithm:
//
//	(a) caller already linked → SELECT id FROM owner WHERE discord_user_id=? → return it (D-03).
//	(b) not linked → resolve username via web_user.
//	(c) label-bridge match on TRIM(label)=TRIM(?username) COLLATE NOCASE (the existing
//	    eviction.go:372 bridge), scanning ALL matches:
//	      - a match stamped with a DIFFERENT discord_user_id → ErrAmbiguousOwner (mis-adoption guard).
//	      - exactly ONE unlinked match → stamp it (UPDATE) and return it (D-04 adopt).
//	      - ZERO matches → INSERT a fresh owner(label=username, discord_user_id=caller) (D-04 new).
//	      - 2+ unlinked matches → ErrAmbiguousOwner (refuse + log).
//
// All on the tx, parameterized ? only; the token is never involved here (this maps
// identity, not codes). NEVER logs a token (V7).
func ResolveOrCreateOwnerByDiscordTx(ctx context.Context, tx *sql.Tx, callerDiscordID string) (int64, error) {
	// (a) Already linked? Subsequent codes attach to the same owner (D-03).
	var existingID int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM owner WHERE discord_user_id = ?`, callerDiscordID,
	).Scan(&existingID)
	switch {
	case err == nil:
		return existingID, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("resolve owner by discord_user_id: %w", err)
	}

	// (b) First link — resolve the caller's Discord username (the label-bridge key;
	// usernameOf precedent — officers.go:218).
	var username string
	err = tx.QueryRowContext(ctx,
		`SELECT username FROM web_user WHERE discord_user_id = ?`, callerDiscordID,
	).Scan(&username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// No web_user row for an authenticated caller should not happen (a session is
		// only minted for a logged-in member), but fail closed: treat it as ambiguous
		// rather than silently inventing an owner under an empty label.
		slog.Warn("ambiguous_owner", "reason", "no_web_user_for_caller")
		return 0, ErrAmbiguousOwner
	case err != nil:
		return 0, fmt.Errorf("resolve caller username: %w", err)
	}

	// (c) Label-bridge: scan ALL owners whose label matches the username, trimmed +
	// case-insensitive (the existing WR-05 bridge — eviction.go:372). Parameterized.
	// IN-02: the reserved sentinel owner (GuildSentinelOwnerID, label 'guild') is NEVER a
	// link target — a guildie whose Discord username is literally "guild" must not adopt
	// the guild-held bank/bot owner. Exclude it by id.
	rows, err := tx.QueryContext(ctx,
		`SELECT id, discord_user_id FROM owner WHERE TRIM(label) = TRIM(?) COLLATE NOCASE AND id <> ?`,
		username, GuildSentinelOwnerID)
	if err != nil {
		return 0, fmt.Errorf("scan label matches for %q: %w", username, err)
	}
	defer rows.Close()

	var unlinkedMatches []int64
	for rows.Next() {
		var id int64
		var disc sql.NullString
		if err := rows.Scan(&id, &disc); err != nil {
			return 0, fmt.Errorf("scan label match row: %w", err)
		}
		switch {
		case !disc.Valid:
			unlinkedMatches = append(unlinkedMatches, id)
		case disc.String != callerDiscordID:
			// A label match already belongs to a DIFFERENT Discord identity → refuse
			// (never silently adopt another guildie's characters — D-04 mis-adoption guard).
			slog.Warn("ambiguous_owner", "reason", "label_match_stamped_by_other")
			return 0, ErrAmbiguousOwner
		default:
			// disc.String == callerDiscordID: already-linked match. Step (a) should have
			// caught this, but if it surfaces here, return it (idempotent).
			return id, nil
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate label matches: %w", err)
	}

	switch len(unlinkedMatches) {
	case 1:
		// D-04 adopt: stamp the single unlinked label match with the caller's id.
		if _, err := tx.ExecContext(ctx,
			`UPDATE owner SET discord_user_id = ? WHERE id = ?`, callerDiscordID, unlinkedMatches[0]); err != nil {
			return 0, fmt.Errorf("stamp adopted owner %d: %w", unlinkedMatches[0], err)
		}
		return unlinkedMatches[0], nil
	case 0:
		// D-04 new: no label match → create a fresh owner labeled with the username,
		// stamped with the caller. Self-service works before the watcher's first upload.
		res, err := tx.ExecContext(ctx,
			`INSERT INTO owner (label, discord_user_id) VALUES (?, ?)`, username, callerDiscordID)
		if err != nil {
			return 0, fmt.Errorf("create fresh owner for %q: %w", username, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("fresh owner last insert id: %w", err)
		}
		return id, nil
	default:
		// 2+ unlinked label matches → ambiguous, never guess (D-04 refuse + log).
		slog.Warn("ambiguous_owner", "reason", "multiple_label_matches")
		return 0, ErrAmbiguousOwner
	}
}

// ListOwnCodes returns the caller-owner's ACTIVE (disabled_at IS NULL) codes,
// ordered created_at ASC, id ASC for a stable #N ordinal (the handler assigns the
// 1-based index — RESEARCH A3 / D-06). Always returns a NON-NIL slice (possibly
// empty) so the handler emits [] not null (officers.go:88-94 precedent). Owner is
// resolved by the caller's session upstream; this query is owner-scoped.
func ListOwnCodes(ctx context.Context, db *sql.DB, ownerID int64) ([]OwnCode, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, created_at, last_seen
		   FROM guild_code
		  WHERE owner_id = ? AND disabled_at IS NULL
		  ORDER BY created_at ASC, id ASC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list own codes for owner %d: %w", ownerID, err)
	}
	defer rows.Close()

	out := make([]OwnCode, 0) // non-nil → JSON []
	for rows.Next() {
		var c OwnCode
		if err := rows.Scan(&c.ID, &c.CreatedAt, &c.LastSeen); err != nil {
			return nil, fmt.Errorf("scan own-code row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate own codes: %w", err)
	}
	return out, nil
}

// RevokeOwnCodeTx revokes a single code, OWNER-SCOPED to the caller (RESEARCH
// Pitfall 3 — the security-critical IDOR guard). The UPDATE matches id AND
// owner_id AND disabled_at IS NULL, so:
//   - revoking the caller's own active code → RowsAffected=1 → (true, nil).
//   - revoking a code owned by a DIFFERENT owner → RowsAffected=0 → (false, nil),
//     a silent idempotent no-op that NEVER leaks the code's existence and NEVER
//     touches another owner's data.
//   - revoking an already-revoked own code → RowsAffected=0 → (false, nil).
//
// Do NOT reuse auth.RevokeCode (the ops CLI is intentionally owner-unscoped). The
// owner_id MUST be resolved from the caller's session upstream, never the body.
func RevokeOwnCodeTx(ctx context.Context, tx *sql.Tx, codeID, ownerID int64) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE guild_code SET disabled_at = datetime('now') WHERE id = ? AND owner_id = ? AND disabled_at IS NULL`,
		codeID, ownerID)
	if err != nil {
		return false, fmt.Errorf("revoke own code (code=%d owner=%d): %w", codeID, ownerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("revoke own code rows-affected (code=%d owner=%d): %w", codeID, ownerID, err)
	}
	return n > 0, nil
}

// StampCodeLastSeen records that the code with id codeID just uploaded (D-07). It
// is the helper behind the ingest path's best-effort last_seen stamp (the bearer
// guard resolves the matched guild_code.id; see auth/guard.go + ingest/handler.go).
// last_seen is advisory UI metadata — the ingest path calls this OUTSIDE the atomic
// replace tx and treats a failure as non-fatal (RESEARCH Pattern 3). Plain
// parameterized UPDATE; never logs the token (V7).
func StampCodeLastSeen(ctx context.Context, db *sql.DB, codeID int64) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?`, codeID); err != nil {
		return fmt.Errorf("stamp last_seen (code=%d): %w", codeID, err)
	}
	return nil
}
