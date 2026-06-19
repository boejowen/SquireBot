// Package wantmatch is the SquireBot backend's SINGLE shared want-matcher seam
// (Phase 20, WANT-03/04 / ARCHITECTURE-v2.2 Pattern 2). It answers one question
// for every alerting monitor: "given an item the guild just saw, which guildies'
// wishlist entries should be alerted?" — and it answers it ONE way so all three
// monitors inherit the same gating.
//
// Phase 34 (WISH-05) repointed this seam from the retired item-centric
// wantlist_item to the per-character/per-slot wishlist_item (the D-01 clean
// break). The seam's CONTRACT is unchanged — ForItem/ForName, Hit.WantID (the FK
// notify records into alert_log + the dedup key), and the DM-target invariant all
// carry over verbatim. Only the underlying SELECT changed (FROM wishlist_item, the
// pinged gate, an INNER JOIN, no note column).
//
// Two entry points, one for each kind of signal:
//
//	ForItem(db, itemID) — the EC monitor (P21) and the raid monitor (P23, after
//	    it resolves NPC → quest → item) ride this: a STABLE catalog item_id.
//	ForName(db, name)   — the WTS monitor (P22) rides this: free chat text whose
//	    only handle is the item NAME. Matched EXACTLY, case-insensitively — NOT a
//	    LIKE substring (Pitfall 6: a substring LIKE would alert "Sapphire" on
//	    "Black Sapphire Necklace"). Custom wishlist targets (item_id NULL) are
//	    name-only and so are reachable ONLY via ForName.
//
// The gates live HERE, at the seam, so NO consumer can forget them:
//   - active=1     — a soft-removed target (active=0, Pitfall 4) never alerts.
//   - the ping gate — a per-target ping toggle (WISH-05) is "stop pinging me about
//     THIS upgrade"; pinged=1 is the alert-eligible state (the INVERSE polarity of
//     the wantlist's muted=0, Pitfall 8). It is applied at the matcher so every
//     monitor honors it for free.
//
// Both funcs are READ-ONLY (plain *sql.DB, no tx), return a NON-NIL slice (so a
// caller iterates cleanly on no matches), use parameterized ? placeholders (V5),
// and scan the nullable item_id via sql.NullInt64. They match ACROSS ALL USERS —
// an item match is a guild-wide event, fanned out to every wishlister.
package wantmatch

import (
	"context"
	"database/sql"
	"fmt"
)

// Hit is one wishlister to alert: the wishlist_item row id (the FK notify records
// into alert_log + the dedup key — still named WantID for zero churn to the notify
// spine), the owner's discord_user_id (the DM target, resolved from the wishlist
// row, NOT a request body), and enough item context for the DM body. ItemID is a
// pointer — a custom/gear-tier target has a NULL item_id.
type Hit struct {
	WantID int64
	// DiscordUserID is the DM target — it is ALWAYS wishlist_item.discord_user_id (the
	// wishlist OWNER, the person who created the target), and MUST NOT be derived from
	// character_id, character.owner_id, or any character_assignment. The character tag
	// (CharacterName) is DISPLAY-ONLY; a wishlist target on a character owned/assigned to
	// a DIFFERENT member never changes who gets the DM (T-28-06 / T-34-08).
	DiscordUserID string
	ItemID        *int64
	ItemName      string
	// CharacterName is the tagged character's name, via an INNER JOIN character on
	// wishlist_item.character_id (NOT NULL — every wishlist target is char+slot-scoped).
	// DISPLAY-ONLY (the EC embed's "For <char>" field); it NEVER affects DiscordUserID /
	// the DM recipient.
	CharacterName *string
}

// ForItem returns one Hit per ACTIVE, pinged wishlist_item whose item_id equals
// itemID, across ALL users. It is the stable-id path: the EC monitor (P21) and the
// raid monitor (P23, after NPC→quest→item resolution) call it. A ping-off
// (pinged=0, WISH-05) or soft-removed (active=0, Pitfall 4) target is excluded at
// the matcher. Returns a non-nil (possibly empty) slice.
func ForItem(ctx context.Context, db *sql.DB, itemID int64) ([]Hit, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT w.id, w.discord_user_id, w.item_id, w.item_name, c.name AS character_name
		   FROM wishlist_item w
		   JOIN character c ON c.id = w.character_id
		  WHERE w.item_id = ? AND w.active = 1 AND w.pinged = 1`, itemID)
	if err != nil {
		return nil, fmt.Errorf("wantmatch.ForItem (item=%d): %w", itemID, err)
	}
	defer rows.Close()
	return scanHits(rows, fmt.Sprintf("item=%d", itemID))
}

// ForName returns Hits for ACTIVE, pinged wishlist targets whose item_name matches
// name EXACTLY and case-insensitively (COLLATE NOCASE) — NOT a LIKE substring
// (Pitfall 6). It is the chat-text path: the WTS monitor (P22) calls it, and it is
// the ONLY way a custom/gear-tier target (item_id NULL) is reachable. A ping-off or
// soft-removed target is excluded at the matcher. Returns a non-nil slice.
func ForName(ctx context.Context, db *sql.DB, name string) ([]Hit, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT w.id, w.discord_user_id, w.item_id, w.item_name, c.name AS character_name
		   FROM wishlist_item w
		   JOIN character c ON c.id = w.character_id
		  WHERE w.item_name = ? COLLATE NOCASE AND w.active = 1 AND w.pinged = 1`, name)
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
			h        Hit
			itemID   sql.NullInt64
			charName sql.NullString
		)
		// NB: &h.DiscordUserID is scanned from w.discord_user_id (the wishlist owner) — its
		// scan target is INDEPENDENT of character_id / its owner / its assignee (T-28-06).
		// characterName is the trailing c.name column in both SELECTs (DISPLAY-ONLY).
		if err := rows.Scan(&h.WantID, &h.DiscordUserID, &itemID, &h.ItemName, &charName); err != nil {
			return nil, fmt.Errorf("wantmatch scan hit (%s): %w", ctxLabel, err)
		}
		if itemID.Valid {
			v := itemID.Int64
			h.ItemID = &v
		}
		if charName.Valid {
			v := charName.String
			h.CharacterName = &v
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("wantmatch iterate hits (%s): %w", ctxLabel, err)
	}
	return out, nil
}
