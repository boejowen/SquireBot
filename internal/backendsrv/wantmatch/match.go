// Package wantmatch is the SquireBot backend's SINGLE shared want-matcher seam
// (Phase 20, WANT-03/04 / ARCHITECTURE-v2.2 Pattern 2). It answers one question
// for every alerting monitor: "given an item the guild just saw, which guildies'
// wantlist entries should be alerted?" — and it answers it ONE way so all three
// monitors inherit the same gating.
//
// Two entry points, one for each kind of signal:
//
//	ForItem(db, itemID) — the EC monitor (P21) and the raid monitor (P23, after
//	    it resolves NPC → quest → item) ride this: a STABLE catalog item_id.
//	ForName(db, name)   — the WTS monitor (P22) rides this: free chat text whose
//	    only handle is the item NAME. Matched EXACTLY, case-insensitively — NOT a
//	    LIKE substring (Pitfall 6: a substring LIKE would alert "Sapphire" on
//	    "Black Sapphire Necklace"). Custom wants (item_id NULL) are name-only and
//	    so are reachable ONLY via ForName.
//
// The gates live HERE, at the seam, so NO consumer can forget them:
//   - active=1     — a soft-removed want (active=0, Pitfall 4) never alerts.
//   - the mute gate — a per-want mute (D-09) is "stop pinging me about THIS
//     item"; it is applied at the matcher so every monitor honors it for free.
//
// Both funcs are READ-ONLY (plain *sql.DB, no tx), return a NON-NIL slice (so a
// caller iterates cleanly on no matches), use parameterized ? placeholders (V5),
// and scan the nullable item_id via sql.NullInt64. They match ACROSS ALL USERS —
// an item match is a guild-wide event, fanned out to every wantlister.
package wantmatch

import (
	"context"
	"database/sql"
	"fmt"
)

// Hit is one wantlister to alert: the want row id (the FK notify records into
// alert_log + the dedup key), the owner's discord_user_id (the DM target,
// resolved from the want, NOT a request body), and enough item context for the
// DM body. ItemID is a pointer — a custom want has a NULL item_id.
type Hit struct {
	WantID        int64
	DiscordUserID string
	ItemID        *int64
	ItemName      string
	Reason        string
}

// ForItem returns one Hit per ACTIVE, NON-muted wantlist_item whose item_id
// equals itemID, across ALL users. It is the stable-id path: the EC monitor
// (P21) and the raid monitor (P23, after NPC→quest→item resolution) call it. A
// muted (D-09) or soft-removed (active=0, Pitfall 4) want is excluded at the
// matcher. Returns a non-nil (possibly empty) slice.
func ForItem(ctx context.Context, db *sql.DB, itemID int64) ([]Hit, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, discord_user_id, item_id, item_name, reason
		   FROM wantlist_item
		  WHERE item_id = ? AND active = 1 AND muted = 0`, itemID)
	if err != nil {
		return nil, fmt.Errorf("wantmatch.ForItem (item=%d): %w", itemID, err)
	}
	defer rows.Close()
	return scanHits(rows, fmt.Sprintf("item=%d", itemID))
}

// ForName returns Hits for ACTIVE, NON-muted wants whose item_name matches name
// EXACTLY and case-insensitively (COLLATE NOCASE) — NOT a LIKE substring
// (Pitfall 6). It is the chat-text path: the WTS monitor (P22) calls it, and it
// is the ONLY way a custom want (item_id NULL) is reachable. A muted or
// soft-removed want is excluded at the matcher. Returns a non-nil slice.
func ForName(ctx context.Context, db *sql.DB, name string) ([]Hit, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, discord_user_id, item_id, item_name, reason
		   FROM wantlist_item
		  WHERE item_name = ? COLLATE NOCASE AND active = 1 AND muted = 0`, name)
	if err != nil {
		return nil, fmt.Errorf("wantmatch.ForName (name=%q): %w", name, err)
	}
	defer rows.Close()
	return scanHits(rows, fmt.Sprintf("name=%q", name))
}

// scanHits drains rows into a non-nil []Hit, converting the nullable item_id to
// a *int64. ctxLabel is a short query descriptor for error wrapping.
func scanHits(rows *sql.Rows, ctxLabel string) ([]Hit, error) {
	out := make([]Hit, 0) // non-nil so callers iterate cleanly on no matches
	for rows.Next() {
		var (
			h      Hit
			itemID sql.NullInt64
		)
		if err := rows.Scan(&h.WantID, &h.DiscordUserID, &itemID, &h.ItemName, &h.Reason); err != nil {
			return nil, fmt.Errorf("wantmatch scan hit (%s): %w", ctxLabel, err)
		}
		if itemID.Valid {
			v := itemID.Int64
			h.ItemID = &v
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("wantmatch iterate hits (%s): %w", ctxLabel, err)
	}
	return out, nil
}
