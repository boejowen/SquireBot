package store

import (
	"context"
	"fmt"
	"testing"
)

// eccursor_test.go covers the Phase 21 EC monitor data layer (21-01 Task 3) over
// the shared NewTestDB fixture (00008 creates ec_auction_cursor):
//   - GetECCursor on an unseen item ⇒ ("", false, nil) — the first-sight baseline
//     signal (the producer must NOT DM history for a never-cursored item).
//   - SetECCursor then GetECCursor round-trips with ok=true; a second Set with a
//     later t overwrites in place (ON CONFLICT upsert — one row per item).
//   - ECPollSet returns DISTINCT active catalog wants ONLY: a buy want, a quest
//     want (D-01 reason NOT filtered), a custom NULL-item want skipped (D-03), an
//     inactive want skipped, and a duplicate item_id across two users polled once.

func TestECCursor_GetAbsentIsFirstSight(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	lastT, ok, err := s.GetECCursor(ctx, 16247)
	if err != nil {
		t.Fatalf("GetECCursor on empty: %v", err)
	}
	if ok {
		t.Errorf("GetECCursor on unseen item: ok = true, want false (first-sight baseline)")
	}
	if lastT != "" {
		t.Errorf("GetECCursor on unseen item: lastT = %q, want empty string", lastT)
	}
}

func TestECCursor_SetThenGetRoundTripsAndUpserts(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	const itemID = int64(16247)
	const t1 = "2026-06-06T01:00:00+00:00"
	if err := s.SetECCursor(ctx, itemID, t1, 100); err != nil {
		t.Fatalf("SetECCursor (first): %v", err)
	}

	got, ok, err := s.GetECCursor(ctx, itemID)
	if err != nil {
		t.Fatalf("GetECCursor after set: %v", err)
	}
	if !ok {
		t.Errorf("GetECCursor after set: ok = false, want true")
	}
	if got != t1 {
		t.Errorf("GetECCursor lastT = %q, want %q", got, t1)
	}

	// A second Set with a LATER t advances in place (upsert — one row, not append).
	const t2 = "2026-06-06T02:00:00+00:00"
	if err := s.SetECCursor(ctx, itemID, t2, 200); err != nil {
		t.Fatalf("SetECCursor (second): %v", err)
	}
	got2, _, err := s.GetECCursor(ctx, itemID)
	if err != nil {
		t.Fatalf("GetECCursor after second set: %v", err)
	}
	if got2 != t2 {
		t.Errorf("GetECCursor lastT = %q after advance, want %q", got2, t2)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ec_auction_cursor`).Scan(&count); err != nil {
		t.Fatalf("count ec_auction_cursor: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 ec_auction_cursor row after two SetECCursor calls (upsert), got %d", count)
	}
}

func TestECPollSet_DistinctActiveCatalogWantsOnly(t *testing.T) {
	db := NewTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	// Two users so a duplicate item_id across users can be deduped.
	insertWebUser(t, ctx, db, "disc-a", "Alice")
	insertWebUser(t, ctx, db, "disc-b", "Bob")

	// Helper: insert a wishlist_item directly (Phase 34 repoint — ECPollSet now reads
	// wishlist_item; character_id is NOT NULL, so a throwaway character is created per
	// call). The old `reason` param is retained but ignored (no buy/quest concept on the
	// wishlist). active=0 ⇒ excluded by the active=1 gate.
	var seq int
	insWant := func(disc string, itemID *int64, name, _ string, active int) {
		t.Helper()
		res, err := db.Exec(`INSERT INTO owner (label) VALUES (?)`, "owner-"+name)
		if err != nil {
			t.Fatalf("seed owner (%s): %v", name, err)
		}
		ownerID, _ := res.LastInsertId()
		seq++
		res, err = db.Exec(`INSERT INTO character (owner_id, name) VALUES (?, ?)`,
			ownerID, fmt.Sprintf("Toon-%s-%d", disc, seq))
		if err != nil {
			t.Fatalf("seed character (%s): %v", name, err)
		}
		charID, _ := res.LastInsertId()
		if _, err := db.Exec(
			`INSERT INTO wishlist_item (discord_user_id, character_id, slot, item_id, item_name, pinged, active, created_at)
			 VALUES (?, ?, 'Chest', ?, ?, 1, ?, 0)`, disc, charID, itemID, name, active); err != nil {
			t.Fatalf("seed wishlist_item (%s/%s): %v", disc, name, err)
		}
	}
	id := func(v int64) *int64 { return &v }

	insWant("disc-a", id(100), "Fungus Covered Scale Tunic", "buy", 1) // active catalog
	insWant("disc-a", id(200), "Rod of Annihilation", "quest", 1)      // active catalog (no reason filter)
	insWant("disc-a", nil, "My Custom Thing", "buy", 1)                // active custom (item_id NULL) — D-03 skip
	insWant("disc-b", id(300), "Manastone", "buy", 0)                  // INACTIVE catalog — skip
	insWant("disc-b", id(100), "Fungus Covered Scale Tunic", "buy", 1) // DUPLICATE item_id across users — dedupe

	got, err := s.ECPollSet(ctx)
	if err != nil {
		t.Fatalf("ECPollSet: %v", err)
	}

	// Expect exactly {100, 200} — custom (NULL) skipped, inactive (300) skipped,
	// duplicate 100 polled once.
	gotIDs := map[int64]string{}
	for _, it := range got {
		if prev, dup := gotIDs[it.ItemID]; dup {
			t.Errorf("ECPollSet returned item_id %d twice (DISTINCT failed): %q and %q", it.ItemID, prev, it.ItemName)
		}
		gotIDs[it.ItemID] = it.ItemName
	}
	if len(gotIDs) != 2 {
		t.Fatalf("ECPollSet returned %d distinct items, want 2 (got: %v)", len(gotIDs), gotIDs)
	}
	if _, ok := gotIDs[100]; !ok {
		t.Errorf("ECPollSet missing active buy catalog item 100 (got: %v)", gotIDs)
	}
	if _, ok := gotIDs[200]; !ok {
		t.Errorf("ECPollSet missing active QUEST catalog item 200 — D-01 reason must NOT be filtered (got: %v)", gotIDs)
	}
	if _, ok := gotIDs[300]; ok {
		t.Errorf("ECPollSet included inactive item 300 — active=0 must be skipped (got: %v)", gotIDs)
	}
}
