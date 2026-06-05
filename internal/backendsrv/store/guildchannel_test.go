package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// guildchannel_test.go covers the Phase 20 (WANT-08) officer-scoped store funcs
// (20-01 Task 3): GetMonitorFlags (reads the migration-seeded EC=on/wts=off/
// raid=off, D-07), SetMonitorFlagTx (idempotent toggle), AddGuildChannelTx (+ the
// TYPED ErrDuplicateChannel on the (channel_id,monitor) unique conflict),
// ListGuildChannels (non-nil), and RemoveGuildChannelTx (removed bool). The
// monitor_flag table + its three seed rows are created by the 00007 migration,
// NOT this layer. Reuses commitTx (admins_test.go) + NewTestDB (testhelper.go).

func TestGuildChannel_MonitorFlagDefaultsFromSeed(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// The 00007 migration seeds EC=on, WTS=off, raid=off (D-07 ships-dark).
	mf, err := GetMonitorFlags(ctx, db)
	if err != nil {
		t.Fatalf("GetMonitorFlags: %v", err)
	}
	if !mf.EC || mf.WTS || mf.Raid {
		t.Errorf("seeded MonitorFlags = %+v, want {EC:true WTS:false Raid:false} (D-07)", mf)
	}
}

func TestGuildChannel_SetMonitorFlagToggles(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// Flip WTS on (it ships off).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetMonitorFlagTx(ctx, tx, "wts", true)
	}); err != nil {
		t.Fatalf("SetMonitorFlagTx wts on: %v", err)
	}
	mf, err := GetMonitorFlags(ctx, db)
	if err != nil {
		t.Fatalf("GetMonitorFlags after toggle: %v", err)
	}
	if !mf.WTS {
		t.Errorf("after SetMonitorFlagTx(wts,true), WTS = false, want true")
	}
	if !mf.EC || mf.Raid {
		t.Errorf("toggling wts changed other flags: %+v, want EC=true Raid=false", mf)
	}

	// Idempotent: re-set the same value is a clean no-op.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetMonitorFlagTx(ctx, tx, "wts", true)
	}); err != nil {
		t.Fatalf("idempotent SetMonitorFlagTx: %v", err)
	}

	// Flip EC off (the kill-switch).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return SetMonitorFlagTx(ctx, tx, "ec_auction", false)
	}); err != nil {
		t.Fatalf("SetMonitorFlagTx ec off: %v", err)
	}
	mf, err = GetMonitorFlags(ctx, db)
	if err != nil {
		t.Fatalf("GetMonitorFlags after EC off: %v", err)
	}
	if mf.EC {
		t.Errorf("after SetMonitorFlagTx(ec_auction,false), EC = true, want false")
	}
}

func TestGuildChannel_AddListRemoveAndDuplicate(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	// Empty registry → non-nil empty slice.
	if got := mustChannels(t, ctx, db); got == nil {
		t.Fatalf("ListGuildChannels on empty returned nil, want non-nil empty slice")
	} else if len(got) != 0 {
		t.Fatalf("ListGuildChannels on empty len = %d, want 0", len(got))
	}

	// Add a channel.
	var id int64
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		id, e = AddGuildChannelTx(ctx, tx, "12345", "Blue EC", "ec_auction", 100)
		return e
	}); err != nil {
		t.Fatalf("AddGuildChannelTx: %v", err)
	}
	if id <= 0 {
		t.Fatalf("AddGuildChannelTx returned non-positive id %d", id)
	}

	got := mustChannels(t, ctx, db)
	if len(got) != 1 {
		t.Fatalf("ListGuildChannels len = %d, want 1", len(got))
	}
	c := got[0]
	if c.ChannelID != "12345" || c.Label != "Blue EC" || c.Monitor != "ec_auction" || !c.Enabled {
		t.Errorf("registered channel = %+v, want {ChannelID:12345 Label:Blue EC Monitor:ec_auction Enabled:true}", c)
	}

	// A duplicate (channel_id, monitor) → TYPED ErrDuplicateChannel.
	dupErr := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddGuildChannelTx(ctx, tx, "12345", "Blue EC again", "ec_auction", 200)
		return e
	})
	if !errors.Is(dupErr, ErrDuplicateChannel) {
		t.Fatalf("duplicate AddGuildChannelTx err = %v, want ErrDuplicateChannel", dupErr)
	}

	// SAME channel, DIFFERENT monitor → inserts fine (the unique key is the pair).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		_, e := AddGuildChannelTx(ctx, tx, "12345", "Blue EC", "wts", 300)
		return e
	}); err != nil {
		t.Fatalf("same channel different monitor should succeed, got: %v", err)
	}

	// Remove the ec_auction registration → (true), leaving the wts one.
	var removed bool
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveGuildChannelTx(ctx, tx, "12345", "ec_auction")
		return e
	}); err != nil {
		t.Fatalf("RemoveGuildChannelTx: %v", err)
	}
	if !removed {
		t.Fatalf("RemoveGuildChannelTx on existing = false, want true")
	}
	if got := mustChannels(t, ctx, db); len(got) != 1 || got[0].Monitor != "wts" {
		t.Fatalf("after remove, channels = %+v, want only the wts registration", got)
	}

	// Removing an absent registration → silent (false, nil).
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		var e error
		removed, e = RemoveGuildChannelTx(ctx, tx, "99999", "raid_target")
		return e
	}); err != nil {
		t.Fatalf("RemoveGuildChannelTx absent errored: %v (want no-op)", err)
	}
	if removed {
		t.Errorf("RemoveGuildChannelTx on absent = true, want false (no-op)")
	}
}

func mustChannels(t *testing.T, ctx context.Context, db *sql.DB) []GuildChannel {
	t.Helper()
	rows, err := ListGuildChannels(ctx, db)
	if err != nil {
		t.Fatalf("ListGuildChannels: %v", err)
	}
	return rows
}
