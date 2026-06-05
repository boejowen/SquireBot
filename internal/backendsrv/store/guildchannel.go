package store

// guildchannel.go is the Phase 20 (WANT-08) OFFICER-scoped store layer for the
// guild-wide monitor controls: the per-monitor kill-switch flags (monitor_flag,
// D-07/D-08) and the officer-registered source channels (guild_channel). Unlike
// the owner-scoped wantlist/notify funcs, these are NOT owner-scoped: a monitor
// flag and a channel registration are guild-WIDE state (D-08). Authorization is
// enforced at the route (RequireOfficer) + re-checked in-tx (store.IsOfficerTx)
// by the Plan-03 handlers; these store funcs are unreachable except behind that
// gate. Parameterized ? placeholders ONLY (V5); typed conflict via the modernc
// extended result code, not a string-match (the AddWantTx idiom).
//
// The monitor_flag table + its three seed rows (ec_auction=1, wts=0,
// raid_target=0) are CREATED in the 00007 migration, NOT here — this layer only
// reads/toggles them.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// NAMED import so AddGuildChannelTx can inspect the concrete *sqlite.Error for
	// its extended result code (the AddWantTx idiom). Adds NO new dependency —
	// modernc.org/sqlite is already required.
	sqlite "modernc.org/sqlite"
)

// ErrDuplicateChannel is returned by AddGuildChannelTx when the insert violates
// the guild_channel(channel_id, monitor) unique index (the same channel already
// registered for that monitor). The Plan-03 handler maps it to HTTP 409.
// Detection is via the modernc driver's extended result code (==2067 /
// sqliteConstraintUnique, defined in wantlist.go), NOT a string-match.
var ErrDuplicateChannel = errors.New("guild_channel: duplicate")

// GuildChannel is one officer-registered source channel (ListGuildChannels). The
// JSON tags match the frontend GuildChannel interface (api.ts). Enabled maps to
// the INTEGER 0/1 column.
type GuildChannel struct {
	ID        int64  `json:"id"`
	ChannelID string `json:"channel_id"`
	Label     string `json:"label"`
	Monitor   string `json:"monitor"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
}

// MonitorFlags is the guild-wide kill-switch state for the three monitors (D-07/
// D-08). EC ships ON; WTS/raid ship dark (the 00007 seed). The JSON tags match
// the frontend MonitorFlags interface (api.ts).
type MonitorFlags struct {
	EC   bool `json:"ec"`
	WTS  bool `json:"wts"`
	Raid bool `json:"raid"`
}

// GetMonitorFlags reads the three guild-wide kill-switch flags into a struct,
// mapping each monitor_flag row by its monitor name. The three rows are seeded by
// the 00007 migration (ec_auction=1, wts=0, raid_target=0) so every key is
// present; an unexpected extra row is ignored.
func GetMonitorFlags(ctx context.Context, db *sql.DB) (MonitorFlags, error) {
	rows, err := db.QueryContext(ctx, `SELECT monitor, enabled FROM monitor_flag`)
	if err != nil {
		return MonitorFlags{}, fmt.Errorf("get monitor flags: %w", err)
	}
	defer rows.Close()

	var mf MonitorFlags
	for rows.Next() {
		var (
			monitor string
			enabled int
		)
		if err := rows.Scan(&monitor, &enabled); err != nil {
			return MonitorFlags{}, fmt.Errorf("scan monitor_flag row: %w", err)
		}
		on := enabled != 0
		switch monitor {
		case "ec_auction":
			mf.EC = on
		case "wts":
			mf.WTS = on
		case "raid_target":
			mf.Raid = on
		}
	}
	if err := rows.Err(); err != nil {
		return MonitorFlags{}, fmt.Errorf("iterate monitor_flag rows: %w", err)
	}
	return mf, nil
}

// SetMonitorFlagTx toggles a single guild-wide kill-switch (D-07/D-08). It is
// idempotent: re-setting the same value is a clean no-op. The three rows are
// seeded by the migration, so an unknown monitor affects 0 rows (the Plan-03
// caller validates the enum against the allow-list before calling). enabled is
// stored as INTEGER 0/1.
func SetMonitorFlagTx(ctx context.Context, tx *sql.Tx, monitor string, enabled bool) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE monitor_flag SET enabled = ? WHERE monitor = ?`,
		boolToInt(enabled), monitor)
	if err != nil {
		return fmt.Errorf("set monitor flag (monitor=%s): %w", monitor, err)
	}
	return nil
}

// AddGuildChannelTx registers an officer-entered source channel for a monitor. On
// the (channel_id, monitor) unique conflict it returns the TYPED
// ErrDuplicateChannel, detected via the driver's extended result code (the
// AddWantTx idiom), NOT a string-match. enabled defaults to 1 via the DDL.
// Returns the new row id. Officer authorization is enforced upstream (the route +
// in-tx re-check), so this func does not owner-scope.
func AddGuildChannelTx(ctx context.Context, tx *sql.Tx, channelID, label, monitor string, now int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO guild_channel (channel_id, label, monitor, created_at)
		 VALUES (?, ?, ?, ?)`,
		channelID, label, monitor, now)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqliteConstraintUnique {
			return 0, ErrDuplicateChannel
		}
		return 0, fmt.Errorf("add guild channel (channel=%s, monitor=%s): %w", channelID, monitor, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("add guild channel last insert id (channel=%s): %w", channelID, err)
	}
	return id, nil
}

// ListGuildChannels returns all registered source channels, newest-first (ORDER
// BY created_at DESC). Always a NON-NIL slice so the handler emits JSON [] not
// null. Guild-wide (not owner-scoped) — officer-gated upstream.
func ListGuildChannels(ctx context.Context, db *sql.DB) ([]GuildChannel, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, channel_id, label, monitor, enabled, created_at
		   FROM guild_channel
		  ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list guild channels: %w", err)
	}
	defer rows.Close()

	out := make([]GuildChannel, 0) // non-nil → JSON []
	for rows.Next() {
		var (
			c       GuildChannel
			enabled int
		)
		if err := rows.Scan(&c.ID, &c.ChannelID, &c.Label, &c.Monitor, &enabled, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan guild_channel row: %w", err)
		}
		c.Enabled = enabled != 0
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate guild_channel rows: %w", err)
	}
	return out, nil
}

// RemoveGuildChannelTx deletes a registered channel by (channel_id, monitor) and
// reports whether a row was removed (RowsAffected > 0). Removing an absent
// registration is a clean no-op → (false, nil). Officer-gated upstream.
func RemoveGuildChannelTx(ctx context.Context, tx *sql.Tx, channelID, monitor string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`DELETE FROM guild_channel WHERE channel_id = ? AND monitor = ?`,
		channelID, monitor)
	if err != nil {
		return false, fmt.Errorf("remove guild channel (channel=%s, monitor=%s): %w", channelID, monitor, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("remove guild channel rows-affected (channel=%s): %w", channelID, err)
	}
	return n > 0, nil
}
