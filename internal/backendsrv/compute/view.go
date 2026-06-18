package compute

// view.go is the Go reimplementation of apps-script/src/tabs/buildView.ts — the
// consolidated `view` grid. It joins every (non-empty-slot) inventory_item to its
// wiki enrichment + picked price + quest links and emits one ViewRow each, with
// the tooltip enrichment carried inline (D-03) so the client composes the tooltip
// without a second fetch.
//
// Behavioral parity with buildView.ts, with the deliberate Sheet artifacts DROPPED:
//   - pickPrice (buildView.ts:259-265) is ported below, with the Pitfall-6 TEXT
//     direction fix.
//   - WikiURL is the PLAIN url, NOT the Sheet hyperlink-formula cell string
//     (buildView.ts:107-109) — the web renders a real <a>.
//   - The parseToDate / conditional-format machinery (buildView.ts:267-304) is
//     dropped: LastSynced is the raw ISO string; freshness coloring is client-side.
//   - The store ORDER BY already sorts Char asc → item asc → location asc
//     (buildView.ts:95-99), so compute does NOT re-sort.

import (
	"context"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// Direction encoding (Pitfall 6): pigparse_price.direction is TEXT in SQLite. The
// P12 daily job stores strconv.Itoa(t) where t is 0=WTS / 1=WTB / 2=BOTH
// (internal/backendsrv/enrich/pigparse.go:42-44). So the stored values are the
// STRINGS "0"/"1"/"2" — NOT the numeric 0/1 the v1 buildView used. pickPrice
// compares the stringified direction against these consts.
//
// Runtime confirmation: the actual stored values can be confirmed on the box with
//
//	sqlite3 squirebot.db "SELECT DISTINCT direction FROM pigparse_price"
//
// (expected: "0", and "1"/"2" if any WTB/BOTH rows survive the D-9 WTS filter).
// The encoding above is fixed by the P12 job's source, so the consts are the
// contract; the on-box check is a belt-and-suspenders verification, not a gate.
const (
	directionWTS = "0" // sell-side ask
	directionWTB = "1" // buy-side bid
)

// View computes the consolidated `view` grid over the store: every inventory item
// (with a real item_id) across all non-removed characters, joined to wiki
// enrichment, the picked price, and grouped quest links. Rows are returned in the
// store's Char→item→location order (no re-sort).
func View(ctx context.Context, s *store.Store) ([]ViewRow, error) {
	joinRows, err := s.InventoryJoin(ctx, false)
	if err != nil {
		return nil, err
	}
	links, err := s.QuestLinksByItem(ctx)
	if err != nil {
		return nil, err
	}
	return buildViewRows(joinRows, links), nil
}

// buildViewRows is the pure transform shared by View and Bank: it turns store
// join rows + the quest-link map into ViewRows with enrichment inline. Kept pure
// (no store access) so it is directly unit-testable and so Bank reuses it.
func buildViewRows(joinRows []store.InventoryJoinRow, links map[int64][]store.QuestLinkRow) []ViewRow {
	out := make([]ViewRow, 0, len(joinRows))
	for _, jr := range joinRows {
		row := ViewRow{
			Char:        jr.Char,
			Slot:        jr.Location,
			Item:        jr.ItemName,
			ID:          jr.ItemID,
			Count:       jr.Count,
			WikiURL:     jr.WikiURL, // plain url; the web renders <a>, not a Sheet formula
			LastSynced:  jr.LastSeen,
			WikiSummary: jr.WikiSummary,
			IsQuestItem: jr.IsQuestItem,
			Prices:      pricesFromJoin(jr),
			QuestLinks:  questLinksFor(jr.ItemID, links),
		}
		row.Price = pickPrice(row.Prices)
		out = append(out, row)
	}
	return out
}

// pricesFromJoin builds the inline price detail from a join row. The join yields at
// most one price row per item because the pp_rep CTE (store/readviews.go) collapses
// pigparse_price to ONE representative per normalized name BEFORE the name-keyed LEFT
// JOIN — NOT because item_id is a PK. The price join is by normalized name, not
// item_id (commit 0a169f3). So this is 0-or-1 PriceDetail.
func pricesFromJoin(jr store.InventoryJoinRow) []PriceDetail {
	if !jr.HasPrice {
		return nil
	}
	return []PriceDetail{{
		Direction: jr.Direction,
		A30:       jr.A30,
		T30:       jr.T30,
	}}
}

// questLinksFor maps the store quest links for itemID into the public QuestLink
// shape (nil when the item has no links).
func questLinksFor(itemID int64, links map[int64][]store.QuestLinkRow) []QuestLink {
	src := links[itemID]
	if len(src) == 0 {
		return nil
	}
	out := make([]QuestLink, 0, len(src))
	for _, l := range src {
		out = append(out, QuestLink{QuestName: l.QuestName, Source: l.Source})
	}
	return out
}

// pickPrice ports buildView.ts:259-265 with the TEXT-direction fix (Pitfall 6):
// prefer the WTS direction's a30 (when > 0), then the WTB direction's a30 (when
// > 0), else nil. The v1 returned ” (empty string) as the no-price sentinel for
// the Sheet cell; here the sentinel is a nil *float64 so the JSON encodes `null`
// and the client renders the Price column blank. Direction comparison is on the
// STRINGIFIED value (directionWTS/directionWTB), because the SQLite column is TEXT.
func pickPrice(prices []PriceDetail) *float64 {
	if p := findDirection(prices, directionWTS); p != nil && p.A30 > 0 {
		v := p.A30
		return &v
	}
	if p := findDirection(prices, directionWTB); p != nil && p.A30 > 0 {
		v := p.A30
		return &v
	}
	return nil
}

// findDirection returns the first PriceDetail with the given stringified
// direction, or nil. (Mirrors buildView.ts's rows.find((r) => r.direction === X).)
func findDirection(prices []PriceDetail, direction string) *PriceDetail {
	for i := range prices {
		if prices[i].Direction == direction {
			return &prices[i]
		}
	}
	return nil
}
