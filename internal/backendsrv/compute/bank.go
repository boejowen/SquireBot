package compute

// bank.go is the Go reimplementation of apps-script/src/tabs/buildBank.ts — the
// consolidated `bank` grid. It is the same join/shape as View but scoped to the
// bank toon's inventory, returning a BankView (rows + a nullable Coin object).
//
// Two v1 facts changed for the DB world:
//   - Bank-toon identity: v1 read _meta.bank_toon_name (buildBank.ts:54-55) — a
//     SINGLE bank toon; the DB equivalent is character.is_bank_toon = 1 (the store's
//     InventoryJoin bankOnly branch). There is no _meta row in SQLite. Phase 26
//     relaxed the single-bank invariant: is_bank_toon is now the officer-only "guild
//     bank" designation (store.DesignateCharTx) and MULTIPLE characters may carry it.
//   - Coin: the composeCoinRow / writeCoinRow machinery (buildBank.ts:135-163) is
//     NOT ported. Coin is nil in P14 — ADMIN-05 (P15) adds the admin web form
//     that records it. Returning nil (not fabricated 0pp) lets the client render
//     "Coin: not yet recorded" honestly.

import (
	"context"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// Bank computes the consolidated `bank` grid: ALL guild-bank characters' inventory
// rows (same enrichment-inline shape as View), plus a nil Coin (P14). Rows carry
// their Char (consolidated, multiple banks supported) in the store's
// char→item→location order (the bankOnly join is `WHERE c.is_bank_toon = 1`).
//
// Phase 26 relaxed the single-bank invariant (OPEN-2/OPEN-3): is_bank_toon became
// the officer-only "guild bank" designation (store.DesignateCharTx), which does NOT
// demote other banks, so MULTIPLE live characters may be flagged. The consolidated
// Char-column grid (buildViewRows — the same shape as the main view) disambiguates
// them cleanly; this query needs NO change to support N banks.
func Bank(ctx context.Context, s *store.Store) (BankView, error) {
	joinRows, err := s.InventoryJoin(ctx, true) // bankOnly
	if err != nil {
		return BankView{}, err
	}
	links, err := s.QuestLinksByItem(ctx)
	if err != nil {
		return BankView{}, err
	}
	return BankView{
		Rows: buildViewRows(joinRows, links),
		Coin: nil, // P14: never fabricate 0pp; ADMIN-05 fills this in P15
	}, nil
}
