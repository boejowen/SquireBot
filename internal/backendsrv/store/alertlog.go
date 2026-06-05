package store

// alertlog.go is the Phase 20 (WANT-03/04) alert_log store layer: the inbox
// read path (ListInbox / UnreadCount), the owner-scoped read-state mutators
// (MarkAlertReadTx / MarkAllAlertsReadTx), the write path notify uses
// (InsertAlertTx), and the dedup/cooldown probe (RecentAlertExists). It mirrors
// wantlist.go's grain: owner-scoped *Tx mutators + plain *sql.DB readers,
// %w-wrapped errors, parameterized ? placeholders, non-nil list slices.
//
// D-04 inbox = full history: every alert ATTEMPT is logged — sent, dm_blocked
// (50007), and error — so the inbox is both the "what was I pinged about?" log
// AND the can't-DM safety net. D-05: read_at NULL = unread; the unread badge
// keys on it.
//
// BLOCKER-1: InsertAlertTx takes a NULLABLE wantID (*int64). The D-10 test-alert
// has NO wantlist_item, so it logs wantlist_item_id=NULL (00007 made the column
// nullable). Catalog/custom alerts pass a real id.
//
// Owner scoping (the IDOR guard): MarkAlertReadTx matches id AND discord_user_id,
// so a cross-owner mark-read is RowsAffected=0 → (false,nil): a silent no-op that
// never leaks the row's existence (the RemoveOwnWantTx twin).

import (
	"context"
	"database/sql"
	"fmt"
)

// AlertLogRow is one inbox row for a guildie (ListInbox). ItemID/Detail/ReadAt
// are pointers so a NULL column marshals as JSON null. ReadAt NULL ⇒ unread
// (D-05). The JSON tags match the frontend AlertLogRow interface (api.ts).
type AlertLogRow struct {
	ID         int64   `json:"id"`
	Source     string  `json:"source"`
	ItemID     *int64  `json:"item_id"`
	Detail     *string `json:"detail"`
	SentAt     int64   `json:"sent_at"`
	SendStatus string  `json:"send_status"`
	ReadAt     *int64  `json:"read_at"` // null ⇒ unread (D-05)
}

// ListInbox returns the caller's alert history, newest-first (ORDER BY sent_at
// DESC), OWNER-SCOPED (WHERE discord_user_id = ?). Always a NON-NIL slice so the
// handler emits JSON [] not null. discordID is session-derived upstream (D-02).
func ListInbox(ctx context.Context, db *sql.DB, discordID string) ([]AlertLogRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, source, item_id, detail, sent_at, send_status, read_at
		   FROM alert_log
		  WHERE discord_user_id = ?
		  ORDER BY sent_at DESC`, discordID)
	if err != nil {
		return nil, fmt.Errorf("list inbox (user=%s): %w", discordID, err)
	}
	defer rows.Close()

	out := make([]AlertLogRow, 0) // non-nil → JSON []
	for rows.Next() {
		var (
			r      AlertLogRow
			itemID sql.NullInt64
			detail sql.NullString
			readAt sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &r.Source, &itemID, &detail, &r.SentAt, &r.SendStatus, &readAt); err != nil {
			return nil, fmt.Errorf("scan inbox row (user=%s): %w", discordID, err)
		}
		if itemID.Valid {
			v := itemID.Int64
			r.ItemID = &v
		}
		if detail.Valid {
			v := detail.String
			r.Detail = &v
		}
		if readAt.Valid {
			v := readAt.Int64
			r.ReadAt = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox (user=%s): %w", discordID, err)
	}
	return out, nil
}

// MarkAlertReadTx marks a single alert read, OWNER-SCOPED to the caller (the
// IDOR guard — RemoveOwnWantTx twin). The UPDATE matches id AND discord_user_id
// AND read_at IS NULL, so:
//   - the caller's own unread row → RowsAffected=1 → (true, nil);
//   - another owner's row → RowsAffected=0 → (false, nil): a silent no-op that
//     never leaks the row's existence;
//   - an already-read own row → RowsAffected=0 → (false, nil).
//
// discordID MUST be resolved from the session upstream, never the body (D-02).
func MarkAlertReadTx(ctx context.Context, tx *sql.Tx, alertID int64, discordID string, now int64) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE alert_log SET read_at = ? WHERE id = ? AND discord_user_id = ? AND read_at IS NULL`,
		now, alertID, discordID)
	if err != nil {
		return false, fmt.Errorf("mark alert read (id=%d): %w", alertID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mark alert read rows-affected (id=%d): %w", alertID, err)
	}
	return n > 0, nil
}

// MarkAllAlertsReadTx marks ALL the caller's unread alerts read (the "mark all
// read" action, D-05). OWNER-SCOPED (WHERE discord_user_id = ?); returns the
// number of rows flipped. A user with no unread rows gets 0.
func MarkAllAlertsReadTx(ctx context.Context, tx *sql.Tx, discordID string, now int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE alert_log SET read_at = ? WHERE discord_user_id = ? AND read_at IS NULL`,
		now, discordID)
	if err != nil {
		return 0, fmt.Errorf("mark all alerts read (user=%s): %w", discordID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mark all alerts read rows-affected (user=%s): %w", discordID, err)
	}
	return n, nil
}

// UnreadCount returns the caller's unread-alert count (read_at IS NULL),
// OWNER-SCOPED. This is the nav badge's number (D-05) — the load-bearing signal
// that an undeliverable alert actually gets noticed.
func UnreadCount(ctx context.Context, db *sql.DB, discordID string) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM alert_log WHERE discord_user_id = ? AND read_at IS NULL`,
		discordID).Scan(&n); err != nil {
		return 0, fmt.Errorf("unread count (user=%s): %w", discordID, err)
	}
	return n, nil
}

// InsertAlertTx records ONE alert attempt in alert_log (read_at is omitted ⇒ NULL
// ⇒ unread, D-05). It is notify's write path: send_status ∈ {sent, dm_blocked,
// error} so EVERY attempt is logged (D-04), never silently dropped.
//
// BLOCKER-1: wantID is *int64 (nullable). The D-10 test-alert passes nil →
// wantlist_item_id=NULL (it has no wantlist_item); catalog/custom alerts pass a
// real id. itemID and detail are likewise nullable. Returns the new row id.
func InsertAlertTx(ctx context.Context, tx *sql.Tx, wantID *int64, discordID, source string, itemID *int64, detail *string, sentAt int64, status string) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, item_id, detail, sent_at, send_status)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		wantID, discordID, source, itemID, detail, sentAt, status)
	if err != nil {
		return 0, fmt.Errorf("insert alert (user=%s, source=%s): %w", discordID, source, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert alert last insert id (user=%s): %w", discordID, err)
	}
	return id, nil
}

// RecentAlertExists is the dedup/cooldown probe notify checks before sending: it
// reports whether a recent alert for the same (wantlist_item_id, source, item_id)
// already landed inside the per-source window (sent_at >= sinceUnix). It serves
// the alert_log_dedup_idx.
//
// Warning 5 — the send_status IN ('sent','dm_blocked') filter means a recent
// CAN'T-DM (50007 → dm_blocked) ALSO suppresses a repeat, so a DMs-off user does
// NOT accrue an identical dm_blocked inbox row every cycle — the 50007 is
// surfaced ONCE per window. (An 'error' row does NOT suppress: a transient send
// error should be retried.)
//
// The item_id match is nullable: `(item_id IS ? OR item_id = ?)` with itemID
// passed twice handles both a NULL item_id (matches NULL) and a concrete id.
//
// NOTE: the D-10 test-alert never calls this (the `test` source bypasses dedup in
// notify), and its NULL wantlist_item_id rows can never match a
// `wantlist_item_id = ?` probe anyway — so a NULL test row is correctly
// never-deduped.
func RecentAlertExists(ctx context.Context, db *sql.DB, wantID int64, source string, itemID *int64, sinceUnix int64) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM alert_log
		  WHERE wantlist_item_id = ? AND source = ?
		    AND (item_id IS ? OR item_id = ?)
		    AND send_status IN ('sent','dm_blocked')
		    AND sent_at >= ?
		  LIMIT 1`,
		wantID, source, itemID, itemID, sinceUnix).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("recent alert exists (want=%d, source=%s): %w", wantID, source, err)
	}
	return true, nil
}
