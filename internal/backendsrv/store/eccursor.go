package store

// eccursor.go holds the EC-tunnel auction monitor's data-layer reads/writes
// (Phase 21, WANT-05), created by the 00008 migration:
//
//	GetECCursor → read the per-item last-seen auction timestamp BEFORE a poll.
//	              An ABSENT row ⇒ ok=false ⇒ "never seen" ⇒ the producer (Plan 03)
//	              baselines the cursor on first sight and does NOT DM history
//	              (advance-only-on-success / no-backlog-replay — RESEARCH Pattern 3).
//	SetECCursor → advance (upsert) the per-item cursor AFTER a successful poll.
//	ECPollSet   → the poll set: DISTINCT active catalog wants (item_id present).
//	              D-01 reason NOT filtered (buy AND quest); D-03 NULL item_id
//	              (custom wants) skipped — they belong to wantmatch.ForName (P22).
//
// Plain (*Store) methods over a single Exec / single-row SELECT, mirroring
// jobstate.go's Get/SetJobRun absent-is-zero + ON CONFLICT upsert discipline.
// Parameterized ? placeholders only (V5); non-nil slice + rows.Err() discipline
// for the poll set (the wantmatch.scanHits precedent).

import (
	"context"
	"database/sql"
	"fmt"
)

// GetECCursor reads the per-item last-seen auction timestamp for itemID. It
// returns:
//   - lastT: the stored last_seen_t (the RFC3339 string the diff compares against);
//   - ok: true iff a cursor row exists for itemID;
//   - err: a non-nil error only on an unexpected DB failure.
//
// An absent row is the load-bearing "never seen ⇒ first-sight baseline" signal:
// sql.ErrNoRows ⇒ ("", false, nil). The producer must NOT DM a never-cursored
// item's history — it baselines the cursor to max(t) of the first poll instead.
func (s *Store) GetECCursor(ctx context.Context, itemID int64) (lastT string, ok bool, err error) {
	var t sql.NullString
	qerr := s.db.QueryRowContext(ctx,
		`SELECT last_seen_t FROM ec_auction_cursor WHERE item_id = ?`, itemID,
	).Scan(&t)
	switch {
	case qerr == sql.ErrNoRows:
		return "", false, nil // never seen ⇒ first-sight baseline, do NOT DM history
	case qerr != nil:
		return "", false, fmt.Errorf("read ec_auction_cursor (item=%d): %w", itemID, qerr)
	}
	return t.String, true, nil
}

// SetECCursor upserts the per-item cursor for itemID, storing lastT (the max
// ItemAuctionDetail.t seen) and now (unix epoch secs). It is an upsert
// (ON CONFLICT(item_id) DO UPDATE) so there is exactly ONE row per item: the
// first poll inserts, every subsequent advance updates in place. Called ONLY
// after the item's poll succeeds (advance-only-on-success — a fetch/parse failure
// must NOT advance the cursor, else a missed window is silently skipped).
func (s *Store) SetECCursor(ctx context.Context, itemID int64, lastT string, now int64) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO ec_auction_cursor (item_id, last_seen_t, updated_at)
		 VALUES (?,?,?)
		 ON CONFLICT(item_id) DO UPDATE SET
		   last_seen_t=excluded.last_seen_t, updated_at=excluded.updated_at`,
		itemID, lastT, now,
	); err != nil {
		return fmt.Errorf("upsert ec_auction_cursor (item=%d): %w", itemID, err)
	}
	return nil
}

// ECPollItem is one item to poll: the stable catalog item_id (the wantmatch join
// key + the cursor key) plus the snapshot item_name (the getdetails NAME lookup
// key — the spike pinned the name form as the only working query key).
type ECPollItem struct {
	ItemID   int64
	ItemName string
}

// ECPollSet returns the DISTINCT set of active catalog wishlist targets to poll:
// every wishlist_item with active=1 AND a non-NULL item_id, deduped so a popular
// item polls once across all users. Phase 34 (WISH-05) repointed this from the
// retired wantlist_item to wishlist_item (the D-01 clean break) — the ping toggle
// (pinged) is NOT filtered here (a ping-off target still defines what to POLL; the
// per-target ping gate is applied downstream at the wantmatch seam, so a temporarily
// un-pinged target re-alerts immediately when re-pinged without re-polling). Custom/
// gear-tier targets (item_id IS NULL) are skipped here — they are name-only and
// reachable solely via wantmatch.ForName (P22), never an exact-id EC poll. Returns a
// non-nil (possibly empty) slice.
func (s *Store) ECPollSet(ctx context.Context) ([]ECPollItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT item_id, item_name
		   FROM wishlist_item
		  WHERE active = 1 AND item_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("ec poll set query: %w", err)
	}
	defer rows.Close()

	out := make([]ECPollItem, 0) // non-nil so the caller iterates cleanly on no wants
	for rows.Next() {
		var it ECPollItem
		if err := rows.Scan(&it.ItemID, &it.ItemName); err != nil {
			return nil, fmt.Errorf("ec poll set scan: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ec poll set iterate: %w", err)
	}
	return out, nil
}
