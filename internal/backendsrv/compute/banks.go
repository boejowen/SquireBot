package compute

// banks.go is the Phase 33 (BANK-01/02) Banks-tab valuation surface — the compute half of
// GET /api/v1/banks. It SURFACES the shipped Phase-29 bank valuation over a WIDENED bank+bot
// scope: the bank/bot roster A-Z, each with a clean per-bank item count + value + nullable
// platinum, plus the guild-wide item-value total and total platinum.
//
// THE IRON LAW (carried from itemrollup.go / inventory.go): this file authors ZERO SQL and
// NEVER re-selects a price. It composes the store's widened bank+bot reads
// (InventoryJoinBanksAndBots + ListBankAndBotToons) and reuses buildBankValuation's
// pickPrice/pricesFromJoin over the name-bridged rows. Identity for any value math is the
// NORMALIZED NAME (the store's pp_rep CTE), NEVER the raw item_id (the EQ-inventory vs
// PigParse-catalog namespace landmine).
//
// Scope asymmetry (deliberate, not a bug): item VALUE includes guild bots (the widened row
// read); PLATINUM stays is_bank_toon-gated (SetCoinTx rejects a non-bank-toon coin write), so
// a guild bot's Plat is nil and contributes 0 to TotalPlatinum. The bot still appears in the
// row list (D-01) — it just carries plat: null.

import (
	"context"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// Banks returns the BanksView for GET /api/v1/banks (BANK-01/02). It reads the WIDENED
// bank+bot scope — InventoryJoinBanksAndBots for the flat item rows (so the guild bot's goods
// count toward the value total) and ListBankAndBotToons for the row list (bots → Plat nil) —
// then delegates to the pure buildBanks transform. It mirrors BankValuationFor's
// public-fn→pure-helper split but over the widened reads — NOT the bank-toons-only
// InventoryJoin bankOnly branch, whose scope feeds the legacy /views/bank grid.
func Banks(ctx context.Context, s *store.Store) (BanksView, error) {
	rows, err := s.InventoryJoinBanksAndBots(ctx) // bank+bot scope (value INCLUDES bots)
	if err != nil {
		return BanksView{}, err
	}
	toons, err := store.ListBankAndBotToons(ctx, s.DB()) // bank+bot row list (bots → Plat nil)
	if err != nil {
		return BanksView{}, err
	}
	return buildBanks(rows, toons), nil
}

// buildBanks is the pure transform. It REUSES buildBankValuation (over the widened rows +
// toons) for the per-bank value + guild totals + unpriced (MR-02: PerBank is seeded from the
// toon list FIRST, so a coin-only bank with zero inventory rows still emits a row), then joins
// each toon's nullable Plat + a per-bank flat item count into a BankRowSummary. It NEVER
// re-selects a price (buildBankValuation already used pickPrice/pricesFromJoin over the
// name-bridged rows). The toon list is already A-Z (COLLATE NOCASE) from the store read, so
// the emitted Banks slice preserves that order (banks are NOT viewer-first — D-01).
func buildBanks(rows []store.InventoryJoinRow, toons []store.BankToon) BanksView {
	bv := buildBankValuation(rows, toons) // PerBank (seeded MR-02) + GuildTotal + TotalPlatinum

	// Per-bank flat item count: Σ rows per char name (the clean-row count, D-02). The flat
	// row list IS the count scope — a bag AND its *-Slot* children each count (their own rows).
	counts := make(map[string]int64, len(toons))
	for _, r := range rows {
		counts[r.Char]++
	}

	out := BanksView{
		Banks:         make([]BankRowSummary, 0, len(toons)),
		GuildValue:    bv.GuildTotal.TotalValue,
		GuildUnpriced: bv.GuildTotal.UnpricedCount,
		TotalPlatinum: bv.TotalPlatinum,
	}
	for _, t := range toons { // toons already A-Z from the store read
		per := bv.PerBank[t.Name]
		out.Banks = append(out.Banks, BankRowSummary{
			Name:      t.Name,
			ItemCount: counts[t.Name],
			Value:     per.TotalValue,
			Unpriced:  per.UnpricedCount,
			Plat:      t.Plat, // nullable — carry nil as nil
		})
	}
	return out
}
