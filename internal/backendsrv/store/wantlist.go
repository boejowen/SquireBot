package store

// wantlist.go is the Phase 19 (WANT-01/02) owner-scoped wantlist store layer. It
// mirrors linking.go's shape exactly: owner-scoped *Tx mutators (Add/Remove) +
// a plain *sql.DB reader (List), %w-wrapped errors, parameterized ? placeholders.
//
// Security posture (the load-bearing parts):
//   - Identity keys on web_user.discord_user_id — the PERSON, NOT the watcher
//     `owner` entity (19-RESEARCH Pitfall 3). A wantlist works for any logged-in
//     member even before they link a watcher, and is the DM target Phase 20 reads.
//   - RemoveOwnWantTx is OWNER-SCOPED (WHERE id=? AND discord_user_id=? AND
//     active=1) — a guildie can never remove another member's want. A cross-owner
//     remove is RowsAffected=0 → (false, nil): a silent IDOR no-op that never
//     leaks the row's existence (the RevokeOwnCodeTx twin — linking.go:198).
//   - The remove is a SOFT-DELETE (active=0) so Phase 20's alert_log.wantlist_item_id
//     FK never dangles (19-RESEARCH Pitfall 4). Every read filters WHERE active=1.
//   - AddWantTx returns the TYPED ErrDuplicateWant on a partial-unique-index
//     conflict, detected via the modernc driver's EXTENDED result code
//     (*sqlite.Error.Code() == 2067), NOT by string-matching "UNIQUE constraint
//     failed" (which couples to the driver's message wording — review MUST-FIX 2).
//   - Parameterized ? placeholders ONLY (V5); the note text is NEVER logged (V7).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// NAMED import (the package otherwise only blank-imports the driver in db.go)
	// so AddWantTx can inspect the concrete *sqlite.Error for its extended result
	// code. This adds NO new dependency — modernc.org/sqlite is already required.
	sqlite "modernc.org/sqlite"
)

// ErrDuplicateWant is returned by AddWantTx when the insert violates one of the
// partial unique indexes (an exact (user,item,reason) or (user,label,reason) re-add
// of an ACTIVE want). The handler (Plan 02) maps it to HTTP 409. Detection is via the
// modernc driver's extended result code, NOT string-matching the message (review MUST-FIX 2).
var ErrDuplicateWant = errors.New("wantlist: duplicate")

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE, the extended result code a
// unique-index violation reports. Hard-coded (with this comment) to avoid importing
// the modernc.org/sqlite/lib subpackage just for the one constant; extended result
// codes are enabled on every connection (modernc conn.go:660), so *sqlite.Error.Code()
// returns this extended value, not the primary SQLITE_CONSTRAINT (19).
const sqliteConstraintUnique = 2067

// WantlistRow is one active row of a guildie's own wantlist (ListOwnWants). ItemID
// is a pointer so a custom want (item_id NULL) marshals as JSON null (D-07); Note is
// a pointer so an absent note marshals as null. The JSON tags match the frontend
// WantlistRow interface (api.ts).
type WantlistRow struct {
	ID            int64   `json:"id"`
	ItemID        *int64  `json:"item_id"` // null ⇒ custom want (D-04, D-07)
	ItemName      string  `json:"item_name"`
	Reason        string  `json:"reason"`
	Priority      string  `json:"priority"`
	Note          *string `json:"note"`
	CreatedAt     int64   `json:"created_at"`
	Muted         bool    `json:"muted"`          // D-09 per-want mute; the mute-bell rendered state reads this
	CharacterID   *int64  `json:"character_id"`   // CWANT-01: null ⇒ account-level want
	CharacterName *string `json:"character_name"` // CWANT-06: LEFT JOIN character.name; null ⇒ account-level (or removed char)
}

// AddWantTx inserts a new wantlist_item for the caller (discordID, resolved from the
// session upstream — NEVER the body, D-02) and returns the new row id. active defaults
// to 1 via the migration DDL. On a partial-unique-index conflict (an exact re-add of an
// active (user,item,reason) catalog want or (user,label,reason) custom want — D-05) it
// returns the TYPED ErrDuplicateWant, detected via the driver's extended result code
// (review MUST-FIX 2) — NOT the raw driver error, and NOT a string-match. itemID is nil
// for a custom want; note is nil when absent.
// characterID is the OPTIONAL character tag (CWANT-01): nil ⇒ an account-level want,
// non-nil ⇒ scoped to that character (the handler authorizes the tag via
// IsCharAssignedToTx BEFORE calling this — Plan 02). The COALESCE(character_id,-1) dedup
// index (00010) still raises 2067 on a duplicate, so the ErrDuplicateWant detection below
// is UNCHANGED.
func AddWantTx(ctx context.Context, tx *sql.Tx, discordID string, itemID *int64, itemName, reason, priority string, note *string, characterID *int64, now int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, note, character_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		discordID, itemID, itemName, reason, priority, note, characterID, now)
	if err != nil {
		// Detect the unique-constraint violation via the modernc driver's extended
		// result code, NOT by string-matching the driver's textual message (brittle —
		// couples to the exact wording the driver emits, which can change).
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintUnique {
			return 0, ErrDuplicateWant
		}
		return 0, fmt.Errorf("add want (user=%s): %w", discordID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("add want last insert id (user=%s): %w", discordID, err)
	}
	return id, nil
}

// ListOwnWants returns the caller's ACTIVE (active = 1) wants, newest first. It is
// OWNER-SCOPED (WHERE discord_user_id = ?) — owner resolved from the session upstream.
// Always returns a NON-NIL slice (possibly empty) so the handler emits JSON [] not null
// (the ListOwnCodes precedent — linking.go:162). Nullable item_id/note are scanned via
// sql.Null* and converted to pointers (NULL ⇒ JSON null).
func ListOwnWants(ctx context.Context, db *sql.DB, discordID string) ([]WantlistRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT w.id, w.item_id, w.item_name, w.reason, w.priority, w.note, w.created_at, w.muted,
		        w.character_id, c.name AS character_name
		   FROM wantlist_item w
		   LEFT JOIN character c ON c.id = w.character_id
		  WHERE w.discord_user_id = ? AND w.active = 1
		  ORDER BY w.created_at DESC`, discordID)
	if err != nil {
		return nil, fmt.Errorf("list own wants (user=%s): %w", discordID, err)
	}
	defer rows.Close()

	out := make([]WantlistRow, 0) // non-nil → JSON []
	for rows.Next() {
		var (
			r       WantlistRow
			itemID  sql.NullInt64
			note    sql.NullString
			muted   int
			charID  sql.NullInt64
			charNme sql.NullString
		)
		if err := rows.Scan(&r.ID, &itemID, &r.ItemName, &r.Reason, &r.Priority, &note, &r.CreatedAt, &muted, &charID, &charNme); err != nil {
			return nil, fmt.Errorf("scan own-want row (user=%s): %w", discordID, err)
		}
		if itemID.Valid {
			v := itemID.Int64
			r.ItemID = &v
		}
		if note.Valid {
			v := note.String
			r.Note = &v
		}
		r.Muted = muted != 0 // INTEGER 0/1 → bool (the mute-bell read path, D-09)
		if charID.Valid {
			v := charID.Int64
			r.CharacterID = &v
		}
		if charNme.Valid {
			v := charNme.String
			r.CharacterName = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate own wants (user=%s): %w", discordID, err)
	}
	return out, nil
}

// GuildWantRow is one active want in the GUILDWIDE roll-up (ListGuildWants — CWANT-03/04,
// the "what does the guild want" read). It surfaces the owner's username (web_user.username,
// JOINed) and the OPTIONAL tagged character name (LEFT JOIN character; NULL ⇒ account-level).
//
// SECURITY (T-28-02): there is intentionally NO Note field. The per-want note is private to
// its owner; the guildwide read MUST NOT expose it. ListOwnWants is the only read that
// returns note (owner-scoped). Do NOT add a note column to this struct or its SELECT.
type GuildWantRow struct {
	ID            int64   `json:"id"`
	ItemID        *int64  `json:"item_id"`
	ItemName      string  `json:"item_name"`
	Reason        string  `json:"reason"`
	Priority      string  `json:"priority"`
	DiscordUserID string  `json:"discord_user_id"`
	Owner         string  `json:"owner"`          // web_user.username
	CharacterID   *int64  `json:"character_id"`   // null ⇒ account-level want
	CharacterName *string `json:"character_name"` // null ⇒ account-level (or removed char)
	// NB: NO note field — private, excluded (Security recommendation, T-28-02).
}

// ListGuildWants returns EVERY active want across ALL members (CWANT-03/04, the guildwide
// roll-up), newest first. Distinct from ListOwnWants: it is NOT owner-scoped (no
// discord_user_id filter) and it JOINs web_user for the owner's username + LEFT JOINs
// character for the optional tag name. It EXCLUDES the private note (T-28-02). Plain
// *sql.DB (read-only, no tx). Always returns a NON-NIL slice (possibly empty) so the
// handler emits JSON [] not null. Nullable item_id/character_id/character_name are scanned
// via sql.Null* → pointers (NULL ⇒ JSON null).
func ListGuildWants(ctx context.Context, db *sql.DB) ([]GuildWantRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT w.id, w.item_id, w.item_name, w.reason, w.priority,
		        w.discord_user_id, wu.username AS owner, w.character_id, c.name AS character_name
		   FROM wantlist_item w
		   JOIN web_user wu ON wu.discord_user_id = w.discord_user_id
		   LEFT JOIN character c ON c.id = w.character_id
		  WHERE w.active = 1
		  ORDER BY w.created_at DESC, w.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list guild wants: %w", err)
	}
	defer rows.Close()

	out := make([]GuildWantRow, 0) // non-nil → JSON []
	for rows.Next() {
		var (
			r       GuildWantRow
			itemID  sql.NullInt64
			charID  sql.NullInt64
			charNme sql.NullString
		)
		if err := rows.Scan(&r.ID, &itemID, &r.ItemName, &r.Reason, &r.Priority,
			&r.DiscordUserID, &r.Owner, &charID, &charNme); err != nil {
			return nil, fmt.Errorf("scan guild-want row: %w", err)
		}
		if itemID.Valid {
			v := itemID.Int64
			r.ItemID = &v
		}
		if charID.Valid {
			v := charID.Int64
			r.CharacterID = &v
		}
		if charNme.Valid {
			v := charNme.String
			r.CharacterName = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate guild wants: %w", err)
	}
	return out, nil
}

// RemoveOwnWantTx soft-removes a single want, OWNER-SCOPED to the caller (the
// security-critical IDOR guard — RevokeOwnCodeTx twin, linking.go:198). The UPDATE
// matches id AND discord_user_id AND active = 1, so:
//   - removing the caller's own active want → RowsAffected=1 → (true, nil).
//   - removing a want owned by a DIFFERENT member → RowsAffected=0 → (false, nil),
//     a silent idempotent no-op that NEVER leaks the want's existence.
//   - removing an already-removed own want → RowsAffected=0 → (false, nil).
//
// discordID MUST be resolved from the caller's session upstream, never the body
// (Pitfall 3 — keyed on the person's discord_user_id, NOT on a watcher entity).
func RemoveOwnWantTx(ctx context.Context, tx *sql.Tx, wantID int64, discordID string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE wantlist_item SET active = 0 WHERE id = ? AND discord_user_id = ? AND active = 1`,
		wantID, discordID)
	if err != nil {
		return false, fmt.Errorf("remove own want (id=%d): %w", wantID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove own want rows-affected (id=%d): %w", wantID, err)
	}
	return n > 0, nil
}

// SetMutedTx toggles a single want's mute flag (D-09 "stop pinging me about THIS
// item"), OWNER-SCOPED to the caller — line-for-line the RemoveOwnWantTx IDOR
// guard. The UPDATE matches id AND discord_user_id AND active = 1, so:
//   - muting/unmuting the caller's own active want → RowsAffected=1 → (true, nil);
//   - a want owned by a DIFFERENT member → RowsAffected=0 → (false, nil): a silent
//     no-op that never leaks the want's existence;
//   - an already-removed (active=0) own want → RowsAffected=0 → (false, nil).
//
// muted is stored as INTEGER 0/1. discordID MUST be resolved from the session
// upstream, never the body (D-02 / Pitfall 3).
func SetMutedTx(ctx context.Context, tx *sql.Tx, wantID int64, discordID string, muted bool) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE wantlist_item SET muted = ? WHERE id = ? AND discord_user_id = ? AND active = 1`,
		boolToInt(muted), wantID, discordID)
	if err != nil {
		return false, fmt.Errorf("set muted (id=%d): %w", wantID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set muted rows-affected (id=%d): %w", wantID, err)
	}
	return n > 0, nil
}

// boolToInt converts a bool to the INTEGER 0/1 the muted column uses.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
