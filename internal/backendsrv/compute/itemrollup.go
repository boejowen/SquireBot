package compute

// itemrollup.go groups compute.View's per-instance rows by NORMALIZED NAME into
// one-row-per-item rollups (D-01) with per-holder detail (ITEM-03) — the backend
// half of the item-centric Inventory tab. The public Items(...) fetches via the
// store, then delegates to a pure buildItemRollups(...) that takes typed slices and
// returns the model with NO ctx/store inside — directly table-testable (the view.go
// public-fn → pure-helper split).
//
// THE IRON LAW (same as the rest of compute): this file authors ZERO SQL. It composes
// View (which already selects the representative price + the name-bridged pp_rep price +
// WikiURL/Prices/IsQuestItem/LastSynced), RosterFor (the is_mine / bank / bot flags), and
// a small item_master icon/stats map. It NEVER re-selects a price — it copies the
// representative ViewRow's Price/Prices (set by View, the only correct name-bridged path).
//
// Group by lower(trim(name)), NEVER item_id: the EQ-inventory ids and the PigParse/
// gear-tier catalog ids are different namespaces (gear-tier rows have no id at all), so
// the normalized name is the only consistent group/join key (memory
// pigparse-vs-ingame-item-id-namespaces; store/readviews.go pp_rep CTE). The
// representative ViewRow.ID is used ONLY for the id-correct item_master icon/stats lookup
// (the watcher's own EQ namespace), NEVER for price.

import (
	"context"
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// Items computes the guild-wide item rollup: every copy of every item held anywhere
// (equipped + general + bag contents + bank) across every character, bank toon, and
// guild bot, collapsed to one ItemRollup per normalized name (D-01). It composes
// compute.View (which carries the representative selected price + inline enrichment),
// store.RosterFor (the per-char is_mine / bank / bot flags, joined by char name), and
// store.ItemMasterIconStats (the id-correct icon/stats lookup). viewerDiscordID is the
// authenticated session id; "" →
// nothing is flagged is_mine, but the list is still complete.
func Items(ctx context.Context, s *store.Store, viewerDiscordID string) ([]ItemRollup, error) {
	viewRows, err := View(ctx, s)
	if err != nil {
		return nil, err
	}
	roster, err := s.RosterFor(ctx, viewerDiscordID)
	if err != nil {
		return nil, err
	}
	iconStats, err := s.ItemMasterIconStats(ctx)
	if err != nil {
		return nil, err
	}
	return buildItemRollups(viewRows, roster, iconStats), nil
}

// buildItemRollups is the pure transform: it groups View rows by normalized name into
// one ItemRollup per name with summed qty, distinct holder count, viewer is_mine,
// name-keyed price/wiki (copied from the representative row — NEVER re-selected), id-
// correct icon/stats, and a per-holder list. Kept pure (no ctx/store) so it is directly
// table-testable. First-seen order is preserved (the client re-sorts viewer-first).
func buildItemRollups(viewRows []ViewRow, roster []store.RosterRow, iconStats map[int64]store.IconStats) []ItemRollup {
	flags := make(map[string]store.RosterRow, len(roster))
	for _, r := range roster {
		flags[r.Name] = r // join holders → flags by char NAME (RosterRow carries no ViewRow id)
	}

	byName := make(map[string]*ItemRollup)
	order := make([]string, 0) // preserve first-seen order before the client re-sorts

	for _, vr := range viewRows {
		key := strings.ToLower(strings.TrimSpace(vr.Item)) // GROUP BY NORMALIZED NAME — never vr.ID
		roll := byName[key]
		if roll == nil {
			ic := iconStats[vr.ID] // representative id-correct icon/stats (item_master EQ namespace)
			roll = &ItemRollup{
				Name:        vr.Item, // first-seen casing
				Price:       vr.Price, // representative price — already selected + name-bridged by View
				Prices:      vr.Prices,
				WikiURL:     vr.WikiURL,
				WikiSummary: vr.WikiSummary,
				IsQuestItem: vr.IsQuestItem,
				IconID:      ic.IconID,
				Statsblock:  ic.Statsblock,
				IsClicky:    ic.IsClicky, // Phase 39 — holdings facet (SC-4), id-correct from item_master
				HasHaste:    ic.HasHaste, // Phase 39
			}
			byName[key] = roll
			order = append(order, key)
		}

		f := flags[vr.Char]
		isBank := f.IsBankToon || f.IsGuildBot
		roll.SummedQty += vr.Count
		if f.IsMine {
			roll.IsMine = true
		}
		roll.Holders = append(roll.Holders, ItemHolder{
			Char:       vr.Char,
			SlotLabel:  slotLabel(vr.Slot),
			Qty:        vr.Count,
			LastSynced: vr.LastSynced,
			IsMine:     f.IsMine,
			IsBank:     isBank,
		})
	}

	// Distinct holder count per rollup (a char holding the same item in two slots counts once).
	for key, roll := range byName {
		seen := make(map[string]struct{}, len(roll.Holders))
		for _, h := range roll.Holders {
			seen[h.Char] = struct{}{}
		}
		byName[key].HolderCount = int64(len(seen))
	}

	out := make([]ItemRollup, 0, len(order))
	for _, key := range order {
		out = append(out, *byName[key])
	}
	return out
}

// slotLabel renders a holder's raw inventory Location (ViewRow.Slot) into the UI-SPEC §F
// display label, reusing the P29 classifySlot taxonomy + splitChild nesting detection — no
// new Location parser. A "*-Slot<N>" child (a bagged copy) labels "Bag": the parent bag's
// display name is NOT on the ViewRow (A2), so the simplest correct label drops it. The
// equipment/general/bank canonical tokens classifySlot returns ("Head", "General4",
// "Bank1") already carry their number, so they read cleanly with a category prefix.
func slotLabel(location string) string {
	if _, isChild := splitChild(location); isChild {
		return "Bag" // bagged copy; the parent bag name is not joined here (A2)
	}
	cat, canonical := classifySlot(location)
	switch cat {
	case SlotEquipment:
		return "Worn · " + canonical
	case SlotBank:
		return "Bank · " + canonical
	default: // SlotGeneral
		return "General · " + canonical
	}
}
