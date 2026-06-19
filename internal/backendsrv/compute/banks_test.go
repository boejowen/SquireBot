package compute_test

// banks_test.go is the Phase 33 (BANK-01/02) parity suite for compute.Banks — the
// widened-scope bank+bot valuation surface. It mirrors inventory_test.go's
// BankValuation seed/assert scaffold (newTestDB → store.NewStore → seed helpers →
// call the compute fn → assert the shaped result). The load-bearing assertion is the
// Pitfall-1 regression: a guild BOT's priced item MUST land in the guild value total
// (the scope widen), while its platinum stays nil (coin is bank-toon-gated).

import (
	"context"
	"database/sql"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/compute"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// markGuildBot flips is_guild_bot on an already-seeded character (seedChar only sets
// is_bank_toon). Used to seed a guild bot in the bank+bot scope.
func markGuildBot(t *testing.T, db *sql.DB, charID int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE character SET is_guild_bot = 1 WHERE id = ?`, charID); err != nil {
		t.Fatalf("set is_guild_bot (char_id=%d): %v", charID, err)
	}
}

// bankByName finds a BankRowSummary by name in a BanksView (test helper).
func bankByName(bv compute.BanksView, name string) *compute.BankRowSummary {
	for i := range bv.Banks {
		if bv.Banks[i].Name == name {
			return &bv.Banks[i]
		}
	}
	return nil
}

// TestBanks_GuildBotValueIncluded is the Pitfall-1 regression (the scope widen): a guild
// bot's priced item MUST contribute to GuildValue (BankValuationFor scoped to bank-toons
// only and silently dropped it). The bot's plat stays nil → 0 in TotalPlatinum. Also proves
// per-bank value/unpriced/item-count, the A-Z order, and the nil-plat carry.
func TestBanks_GuildBotValueIncluded(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	// A bank toon: priced item (100) + unpriced item + plat 500. Name "Zbank" (sorts last A-Z).
	bank := seedChar(t, db, "owner-b", "Zbank", "WAR", 60, "HUM", true)
	seedInvFull(t, db, bank, "Bank1", "Gem A", 5001, 1, 0, 1) // priced 100
	seedInvFull(t, db, bank, "Bank2", "Junk", 5099, 1, 0, 2)  // unpriced
	seedPigparse(t, db, 5001, "Gem A", "0", 100, 10)
	if _, err := db.Exec(`UPDATE character SET plat=500 WHERE id=?`, bank); err != nil {
		t.Fatalf("set Zbank plat: %v", err)
	}

	// A guild bot: priced item (40×2=80) + unpriced item. NO plat (bots can't hold coin).
	// Name "Abot" (sorts first A-Z).
	bot := seedChar(t, db, "owner-bo", "Abot", "CLR", 60, "HUM", false)
	markGuildBot(t, db, bot)
	seedInvFull(t, db, bot, "General1", "Gem B", 5002, 2, 0, 1) // priced 40 × 2 = 80
	seedInvFull(t, db, bot, "General2", "Scrap", 5098, 1, 0, 2) // unpriced
	seedPigparse(t, db, 5002, "Gem B", "0", 40, 5)

	bv, err := compute.Banks(ctx, s)
	if err != nil {
		t.Fatalf("Banks: %v", err)
	}

	// GuildValue includes BOTH the bank's 100 AND the bot's 80 (the scope-widen regression).
	if bv.GuildValue != 180 {
		t.Errorf("GuildValue = %v, want 180 (bank 100 + bot 80 — the guild bot's goods MUST count)", bv.GuildValue)
	}
	// GuildUnpriced counts both unpriced rows.
	if bv.GuildUnpriced != 2 {
		t.Errorf("GuildUnpriced = %d, want 2 (the bank's Junk + the bot's Scrap)", bv.GuildUnpriced)
	}
	// TotalPlatinum = the bank toon's 500; the bot contributes 0 (its Plat is nil).
	if bv.TotalPlatinum != 500 {
		t.Errorf("TotalPlatinum = %d, want 500 (bank's plat; bot's nil plat skipped)", bv.TotalPlatinum)
	}

	// A-Z order: "Abot" (the bot) before "Zbank" (the bank).
	if len(bv.Banks) != 2 {
		t.Fatalf("Banks = %d, want 2: %+v", len(bv.Banks), bv.Banks)
	}
	if bv.Banks[0].Name != "Abot" || bv.Banks[1].Name != "Zbank" {
		t.Errorf("order = %q,%q, want Abot,Zbank (A-Z)", bv.Banks[0].Name, bv.Banks[1].Name)
	}

	// The bot row: value 80, unpriced 1, item count 2, Plat nil (NOT 0).
	abot := bankByName(bv, "Abot")
	if abot == nil {
		t.Fatalf("Abot row missing: %+v", bv.Banks)
	}
	if abot.Value != 80 || abot.Unpriced != 1 || abot.ItemCount != 2 {
		t.Errorf("Abot = {value:%v unpriced:%d count:%d}, want 80/1/2", abot.Value, abot.Unpriced, abot.ItemCount)
	}
	if abot.Plat != nil {
		t.Errorf("Abot (guild bot) Plat = %v, want nil (NOT 0 — bots can't hold coin)", abot.Plat)
	}

	// The bank row: value 100, unpriced 1, item count 2, Plat *500.
	zbank := bankByName(bv, "Zbank")
	if zbank == nil {
		t.Fatalf("Zbank row missing: %+v", bv.Banks)
	}
	if zbank.Value != 100 || zbank.Unpriced != 1 || zbank.ItemCount != 2 {
		t.Errorf("Zbank = {value:%v unpriced:%d count:%d}, want 100/1/2", zbank.Value, zbank.Unpriced, zbank.ItemCount)
	}
	if zbank.Plat == nil || *zbank.Plat != 500 {
		t.Errorf("Zbank Plat = %v, want *500", zbank.Plat)
	}
}

// TestBanks_CoinOnlyBankToon is the MR-02 regression on compute.Banks: a bank toon with
// platinum but ZERO inventory rows still emits a BankRowSummary with ItemCount 0 (seeded
// from the toon list FIRST), so its plat in TotalPlatinum has a matching per-bank row.
func TestBanks_CoinOnlyBankToon(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	coinOnly := seedChar(t, db, "owner-c", "Vault", "ENC", 60, "HUM", true)
	if _, err := db.Exec(`UPDATE character SET plat=777 WHERE id=?`, coinOnly); err != nil {
		t.Fatalf("set Vault plat: %v", err)
	}

	bv, err := compute.Banks(ctx, s)
	if err != nil {
		t.Fatalf("Banks: %v", err)
	}

	vault := bankByName(bv, "Vault")
	if vault == nil {
		t.Fatalf("coin-only bank toon Vault missing from Banks (MR-02): %+v", bv.Banks)
	}
	if vault.ItemCount != 0 || vault.Value != 0 || vault.Unpriced != 0 {
		t.Errorf("Vault = {count:%d value:%v unpriced:%d}, want all zero (coin-only)", vault.ItemCount, vault.Value, vault.Unpriced)
	}
	if vault.Plat == nil || *vault.Plat != 777 {
		t.Errorf("Vault Plat = %v, want *777", vault.Plat)
	}
	if bv.TotalPlatinum != 777 {
		t.Errorf("TotalPlatinum = %d, want 777", bv.TotalPlatinum)
	}
}

// TestBanks_Empty proves an empty store returns a zero-value BanksView (no panic): no banks,
// zero totals.
func TestBanks_Empty(t *testing.T) {
	db := newTestDB(t)
	s := store.NewStore(db)
	ctx := context.Background()

	bv, err := compute.Banks(ctx, s)
	if err != nil {
		t.Fatalf("Banks: %v", err)
	}
	if len(bv.Banks) != 0 || bv.GuildValue != 0 || bv.TotalPlatinum != 0 || bv.GuildUnpriced != 0 {
		t.Errorf("empty Banks = %+v, want zero-value", bv)
	}
}
