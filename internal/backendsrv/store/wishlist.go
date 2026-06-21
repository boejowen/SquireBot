package store

// wishlist.go is the Phase 34 (WISH-02/03/05) owner-scoped wishlist store layer.
// It is a function-for-function clone of wantlist.go's shape: owner-scoped *Tx
// mutators (Add/Remove/SetPinged) + plain *sql.DB readers (ListOwnWishlist,
// AlertedWishlistIDs), %w-wrapped errors, parameterized ? placeholders.
//
// Security posture (the load-bearing parts — carried over verbatim from wantlist):
//   - Identity keys on web_user.discord_user_id — the PERSON, NOT the watcher
//     `owner` entity. A wishlist works for any logged-in member and is the DM
//     target the EC matcher reads (the wantlist precedent, T-28-06).
//   - RemoveOwnWishlistTx / SetPingedTx are OWNER-SCOPED (WHERE id=? AND
//     discord_user_id=? AND active=1). A cross-owner mutation is RowsAffected=0 →
//     (false, nil): a silent IDOR no-op that NEVER leaks the row's existence.
//   - Remove is a SOFT-DELETE (active=0) so alert_log.wantlist_item_id (the FK now
//     targeting wishlist_item per 00014) never dangles. Every read filters active=1.
//   - AddWishlistTx returns the TYPED ErrDuplicateWishlist on a partial-unique-index
//     conflict, detected via the modernc driver's EXTENDED result code
//     (*sqlite.Error.Code() == 2067 == sqliteConstraintUnique, declared in
//     sqliteconstraint.go and REUSED here), NOT by string-matching the driver message.
//   - Parameterized ? placeholders ONLY (V5); the item name is NEVER logged (V7).
//
// NB: sqliteConstraintUnique and boolToInt are declared in sqliteconstraint.go (same
// package) and REUSED here — a duplicate declaration would not compile.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// NAMED import so AddWishlistTx can inspect the concrete *sqlite.Error for its
	// extended result code (the wantlist.go idiom). Adds NO new dependency.
	sqlite "modernc.org/sqlite"
)

// ErrDuplicateWishlist is returned by AddWishlistTx when the insert violates one of
// the partial unique indexes (an exact (user,char,slot,item_id) or
// (user,char,slot,item_name) re-add of an ACTIVE target). The handler (34-02) maps
// it to HTTP 409. Detection is via the modernc driver's extended result code
// (sqliteConstraintUnique == 2067), NOT string-matching the message.
var ErrDuplicateWishlist = errors.New("wishlist: duplicate")

// WishlistTargetRow is one active per-slot upgrade target on a guildie's own
// wishlist (ListOwnWishlist). ItemID is a pointer so a typed/custom or gear-tier
// target (item_id NULL) marshals as JSON null. The JSON tags match the frontend
// WishlistTarget shape (api.ts, 34-03).
type WishlistTargetRow struct {
	ID          int64  `json:"id"`
	ItemID      *int64 `json:"item_id"` // null ⇒ typed/custom OR a gear-tier item (no id)
	ItemName    string `json:"item_name"`
	Slot        string `json:"slot"`         // canonical worn-slot ("Head"/"Finger1"/…)
	CharacterID int64  `json:"character_id"` // NOT NULL (every target is char+slot-scoped)
	Pinged      bool   `json:"pinged"`       // WISH-05 ping toggle (default-ON)
	CreatedAt   int64  `json:"created_at"`
}

// AddWishlistTx inserts a new wishlist_item for the caller (discordID, resolved
// from the session upstream — NEVER the body) and returns the new row id. pinged +
// active default to 1 via the migration DDL (Pitfall 8 default-ON). itemID is nil
// for a typed/custom or gear-tier target. On a partial-unique-index conflict (an
// exact re-add of an active (user,char,slot,item) target) it returns the TYPED
// ErrDuplicateWishlist, detected via the driver's extended result code — NOT the
// raw driver error, NOT a string-match.
func AddWishlistTx(ctx context.Context, tx *sql.Tx, discordID string, characterID int64, slot string, itemID *int64, itemName string, now int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO wishlist_item (discord_user_id, character_id, slot, item_id, item_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		discordID, characterID, slot, itemID, itemName, now)
	if err != nil {
		// Detect the unique-constraint violation via the modernc driver's extended
		// result code, NOT by string-matching the driver's textual message.
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintUnique {
			return 0, ErrDuplicateWishlist
		}
		return 0, fmt.Errorf("add wishlist (user=%s): %w", discordID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("add wishlist last insert id (user=%s): %w", discordID, err)
	}
	return id, nil
}

// ListOwnWishlist returns the caller's ACTIVE (active = 1) targets for the named
// character, newest first. It is OWNER-SCOPED (WHERE discord_user_id = ?) — owner
// resolved from the session upstream — and joins character by NAME so the route
// `{char}` path value binds ONLY as a ? placeholder (V5, never concatenated).
// Always returns a NON-NIL slice (possibly empty) so the handler emits JSON [] not
// null. The nullable item_id is scanned via sql.NullInt64 → *int64. NB: the D-02
// auto-removal is NOT applied here — that is the compute layer's job (held-name
// join); this store read returns the raw active targets.
func ListOwnWishlist(ctx context.Context, db *sql.DB, discordID, char string) ([]WishlistTargetRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT w.id, w.item_id, w.item_name, w.slot, w.character_id, w.pinged, w.created_at
		   FROM wishlist_item w
		   JOIN character c ON c.id = w.character_id
		  WHERE w.discord_user_id = ? AND c.name = ? AND w.active = 1
		  ORDER BY w.created_at DESC`, discordID, char)
	if err != nil {
		return nil, fmt.Errorf("list own wishlist (user=%s): %w", discordID, err)
	}
	defer rows.Close()

	out := make([]WishlistTargetRow, 0) // non-nil → JSON []
	for rows.Next() {
		var (
			r      WishlistTargetRow
			itemID sql.NullInt64
			pinged int
		)
		if err := rows.Scan(&r.ID, &itemID, &r.ItemName, &r.Slot, &r.CharacterID, &pinged, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan own-wishlist row (user=%s): %w", discordID, err)
		}
		if itemID.Valid {
			v := itemID.Int64
			r.ItemID = &v
		}
		r.Pinged = pinged != 0 // INTEGER 0/1 → bool (the ping-toggle read path)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate own wishlist (user=%s): %w", discordID, err)
	}
	return out, nil
}

// RemoveOwnWishlistTx soft-removes a single target, OWNER-SCOPED to the caller (the
// RemoveOwnWantTx twin — the security-critical IDOR guard). The UPDATE matches id
// AND discord_user_id AND active = 1, so:
//   - removing the caller's own active target → RowsAffected=1 → (true, nil).
//   - removing a target owned by a DIFFERENT member → RowsAffected=0 → (false, nil),
//     a silent idempotent no-op that NEVER leaks the target's existence.
//   - removing an already-removed own target → RowsAffected=0 → (false, nil).
//
// discordID MUST be resolved from the caller's session upstream, never the body.
func RemoveOwnWishlistTx(ctx context.Context, tx *sql.Tx, wishID int64, discordID string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE wishlist_item SET active = 0 WHERE id = ? AND discord_user_id = ? AND active = 1`,
		wishID, discordID)
	if err != nil {
		return false, fmt.Errorf("remove own wishlist (id=%d): %w", wishID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove own wishlist rows-affected (id=%d): %w", wishID, err)
	}
	return n > 0, nil
}

// SetPingedTx toggles a single target's ping flag (WISH-05 "ping me / stop pinging
// me about THIS upgrade"), OWNER-SCOPED to the caller — line-for-line the
// RemoveOwnWishlistTx IDOR guard, inverting only the column. pinged is the INVERSE
// of the wantlist's muted (default-ON; the user toggles it OFF to silence). The
// UPDATE matches id AND discord_user_id AND active = 1, so:
//   - setting the caller's own active target → RowsAffected=1 → (true, nil);
//   - a target owned by a DIFFERENT member → RowsAffected=0 → (false, nil): a silent
//     no-op that never leaks the target's existence;
//   - an already-removed (active=0) own target → RowsAffected=0 → (false, nil).
//
// pinged is stored as INTEGER 0/1 (boolToInt, declared in sqliteconstraint.go). discordID
// MUST be resolved from the session upstream, never the body.
func SetPingedTx(ctx context.Context, tx *sql.Tx, wishID int64, discordID string, pinged bool) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE wishlist_item SET pinged = ? WHERE id = ? AND discord_user_id = ? AND active = 1`,
		boolToInt(pinged), wishID, discordID)
	if err != nil {
		return false, fmt.Errorf("set pinged (id=%d): %w", wishID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set pinged rows-affected (id=%d): %w", wishID, err)
	}
	return n > 0, nil
}

// AlertedWishlistIDs returns the SET of wishlist_item ids the caller has at least
// one alert_log row for (WISH-05 EC-hit badge), OWNER-SCOPED. The alert_log FK
// column is still named wantlist_item_id (00014 kept the column name and only
// repointed its FK target to wishlist_item(id) — Pitfall 6 option B), so this read
// keys on that column. The compute layer attaches pinged_hit:true to each target
// whose id is in the returned map. Always returns a NON-NIL map.
func AlertedWishlistIDs(ctx context.Context, db *sql.DB, discordID string) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT wantlist_item_id FROM alert_log
		  WHERE discord_user_id = ? AND wantlist_item_id IS NOT NULL`, discordID)
	if err != nil {
		return nil, fmt.Errorf("alerted wishlist ids (user=%s): %w", discordID, err)
	}
	defer rows.Close()

	out := make(map[int64]bool) // non-nil
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan alerted wishlist id (user=%s): %w", discordID, err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerted wishlist ids (user=%s): %w", discordID, err)
	}
	return out, nil
}
