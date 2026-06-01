package compute

// bank.go is the Go reimplementation of apps-script/src/tabs/buildBank.ts — the
// consolidated `bank` grid. It is the same join/shape as View but scoped to the
// bank toon's inventory, returning a BankView (rows + a nullable Coin object).
//
// Two v1 facts changed for the DB world:
//   - Bank-toon identity: v1 read _meta.bank_toon_name (buildBank.ts:54-55); the
//     DB equivalent is character.is_bank_toon = 1 (the store's InventoryJoin
//     bankOnly branch). There is no _meta row in SQLite.
//   - Coin: the composeCoinRow / writeCoinRow machinery (buildBank.ts:135-163) is
//     NOT ported. Coin is nil in P14 — ADMIN-05 (P15) adds the admin web form
//     that records it. Returning nil (not fabricated 0pp) lets the client render
//     "Coin: not yet recorded" honestly.

import (
	"context"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// Bank computes the consolidated `bank` grid: the bank toon's inventory rows
// (same enrichment-inline shape as View), plus a nil Coin (P14). Rows are in the
// store's item→location order (the bankOnly join is scoped to the single
// is_bank_toon character; Char is constant within it).
//
// The "single is_bank_toon character" assumption is upheld by the write side:
// store.SetCharMetaTx (the only production writer of is_bank_toon=true) demotes any
// other live bank toon in the same tx when it promotes one, so at most one live
// character is ever flagged (MD-01, P16 review). There is no schema-level
// partial-unique index; the store mutator is the enforcement point.
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
