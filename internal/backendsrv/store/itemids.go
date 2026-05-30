package store

// itemids.go is the read-side helper the weekly wiki items pass iterates: the
// distinct (item_id, name) union across all of inventory_item. It mirrors the
// Apps Script trigger's collectInventoryItemRefs ("the deduplicated union of
// (item_id, name) pairs across all inv:* tabs"), now a single indexed SELECT
// DISTINCT against the one backend table instead of a per-tab Sheet scan.
//
// It lives in store/ (not the job) to keep the single-tested-SQL-path rule
// intact (11-05 WARNING-3): the wiki job authors ZERO inline SQL — it calls this
// method and the enrich.go *Tx write methods, composing them over one tx. Read
// side, so it is a plain (*Store) method (no tx-composing variant needed).
// Parameterless, so there is nothing to parameterize, but the column projection
// is fixed (never interpolated). slog is silent on the happy path.

import (
	"context"
	"fmt"
)

// ItemRef is one distinct (item_id, name) pair from inventory_item — the unit
// the wiki items pass fetches a wiki page for. ItemID is always > 0 (the query
// excludes 0/NULL empty-slot rows); Name is the inventory display name used to
// build the wiki page title.
type ItemRef struct {
	ItemID int64
	Name   string
}

// DistinctInventoryItemIDs returns one (item_id, name) pair per distinct
// item_id across all of inventory_item where item_id > 0, ordered by item_id.
// Empty-slot rows (item_id = 0 or NULL) are excluded — the wiki has no page for
// "nothing". The dedup key is the item_id ALONE (GROUP BY item_id), exactly like
// the Sheet's collectInventoryItemRefs ("if (!seen.has(id)) seen.set(id, name)"):
// when the same id appears under several names/casings across characters, ONE
// representative name is kept (MIN(name), a deterministic pick) so the wiki items
// pass fetches each item's page exactly once — never twice for the same id (a
// per-(id,name) DISTINCT would refetch and is a politeness regression). The pass
// iterates the result, fetching + parsing + upserting each item's wiki page.
func (s *Store) DistinctInventoryItemIDs(ctx context.Context) ([]ItemRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, MIN(name) AS name
		 FROM inventory_item
		 WHERE item_id IS NOT NULL AND item_id > 0
		 GROUP BY item_id
		 ORDER BY item_id`)
	if err != nil {
		return nil, fmt.Errorf("query distinct inventory item ids: %w", err)
	}
	defer rows.Close()

	var refs []ItemRef
	for rows.Next() {
		var ref ItemRef
		if err := rows.Scan(&ref.ItemID, &ref.Name); err != nil {
			return nil, fmt.Errorf("scan inventory item ref: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory item refs: %w", err)
	}
	return refs, nil
}
