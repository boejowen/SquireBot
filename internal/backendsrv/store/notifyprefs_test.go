package store

import (
	"context"
	"database/sql"
	"testing"
)

// notifyprefs_test.go covers the Phase 20 (WANT-04) per-user notify-prefs store
// funcs (20-01 Task 2): GetPrefs (default-ON D-01 for an ABSENT row) and
// UpsertPrefsTx (owner-scoped round-trip). Reuses insertWebUser / commitTx
// (admins_test.go) + NewTestDB (testhelper.go).

func TestGetPrefs_AbsentRowDefaultsAllOn(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	// No notify_prefs row written → D-01 default-ON: all four toggles true.
	got, err := GetPrefs(ctx, db, "disc-1")
	if err != nil {
		t.Fatalf("GetPrefs (absent row): %v", err)
	}
	if !got.Master || !got.EC || !got.WTS || !got.Raid {
		t.Errorf("GetPrefs absent-row = %+v, want all true (D-01 default-ON)", got)
	}
}

func TestUpsertPrefsTx_RoundTripsAndOverwrites(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")

	// Turn the master toggle OFF, leave the per-monitor ones ON.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return UpsertPrefsTx(ctx, tx, "disc-1", NotifyPrefs{Master: false, EC: true, WTS: true, Raid: true}, 1700)
	}); err != nil {
		t.Fatalf("UpsertPrefsTx (master off): %v", err)
	}

	got, err := GetPrefs(ctx, db, "disc-1")
	if err != nil {
		t.Fatalf("GetPrefs after upsert: %v", err)
	}
	if got.Master {
		t.Errorf("after upsert master=false, GetPrefs.Master = true, want false")
	}
	if !got.EC || !got.WTS || !got.Raid {
		t.Errorf("after upsert master-only-off, per-monitor toggles = %+v, want all true", got)
	}

	// A second upsert (ON CONFLICT) overwrites: flip WTS off, master back on.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return UpsertPrefsTx(ctx, tx, "disc-1", NotifyPrefs{Master: true, EC: true, WTS: false, Raid: true}, 1800)
	}); err != nil {
		t.Fatalf("UpsertPrefsTx (overwrite): %v", err)
	}
	got, err = GetPrefs(ctx, db, "disc-1")
	if err != nil {
		t.Fatalf("GetPrefs after overwrite: %v", err)
	}
	if !got.Master || !got.EC || got.WTS || !got.Raid {
		t.Errorf("after overwrite = %+v, want {Master:true EC:true WTS:false Raid:true}", got)
	}
}

func TestGetPrefs_OwnerScoped(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()
	insertWebUser(t, ctx, db, "disc-1", "Alice")
	insertWebUser(t, ctx, db, "disc-2", "Bob")

	// Alice mutes everything; Bob (no row) must still read default-ON.
	if err := commitTx(t, ctx, db, func(tx *sql.Tx) error {
		return UpsertPrefsTx(ctx, tx, "disc-1", NotifyPrefs{Master: false, EC: false, WTS: false, Raid: false}, 1)
	}); err != nil {
		t.Fatalf("UpsertPrefsTx Alice all-off: %v", err)
	}

	bob, err := GetPrefs(ctx, db, "disc-2")
	if err != nil {
		t.Fatalf("GetPrefs Bob: %v", err)
	}
	if !bob.Master || !bob.EC || !bob.WTS || !bob.Raid {
		t.Errorf("Bob (no row) GetPrefs = %+v, want all true (Alice's all-off must not leak)", bob)
	}
}
