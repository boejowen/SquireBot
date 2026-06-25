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

// DistinctEnrichmentRefs returns the WIDENED set of items the weekly wiki pass
// enriches (Phase 38, ENRICH-14): the held EQ-id refs (exactly what
// DistinctInventoryItemIDs returns) UNIONed with the catalog-only PigParse refs
// from pigparse_price, deduped by normalized name lower(trim(name)) ACROSS the
// held/catalog boundary so a name that is BOTH held and in the catalog is fetched
// ONCE (held wins) — held and catalog rows for the same item share a name but live
// in DIFFERENT id namespaces (the EQ /outputfile ID vs the PigParse catalog id; the
// catalog↔inventory bridge is the name, never the raw item_id).
//
// The held arm itself groups by item_id (identical to DistinctInventoryItemIDs), so
// two DISTINCT held EQ ids that happen to share a normalized name still yield two
// refs — the pre-existing held-only crawl behavior, NOT a Phase 38 regression. Only
// the catalog arm is name-deduped against the held set; this is exercised by
// TestDistinctEnrichmentRefs_HeldVsHeldSameName.
//
// For a held name ItemID is its EQ id (the MIN(name) representative, unchanged so
// every held reader's item_master row keeps its EQ-id keying); for a catalog-only
// name ItemID is its PigParse id, which the namespace-agnostic
// UpsertItemMasterTx keys an item_master row by.
//
// TWO non-negotiable exclusions guard the catalog arm (D-04 collision guard):
//  1. lower(trim(name)) NOT IN held_names — a name that is BOTH held and in the
//     catalog yields ONE ref (the held EQ id), never a duplicate page fetch and
//     never a second item_master row for the same item (held wins).
//  2. item_id NOT IN (SELECT item_id FROM item_master) — a PigParse id that
//     numerically equals an existing item_master row id is dropped, so the
//     upsert's ON CONFLICT(item_id) DO UPDATE can NEVER overwrite the wrong row.
//
// The catalog arm dedups by lower(trim(name)) (NOT by id): an id-keyed or
// (id,name)-keyed dedup would refetch the same wiki page under multiple casings,
// a politeness regression. Excluded rows simply stay icon-less and surface in the
// items pass's coverage residue. Read side, so a plain (*Store) method; slog is
// silent on the happy path. The job authors ZERO inline SQL (11-05) — this union
// lives HERE.
func (s *Store) DistinctEnrichmentRefs(ctx context.Context) ([]ItemRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`WITH held AS (
		   SELECT item_id, MIN(name) AS name
		   FROM inventory_item
		   WHERE item_id IS NOT NULL AND item_id > 0
		   GROUP BY item_id
		 ),
		 held_names AS (
		   SELECT DISTINCT lower(trim(name)) AS norm
		   FROM inventory_item
		   WHERE item_id IS NOT NULL AND item_id > 0
		 ),
		 catalog AS (
		   SELECT MIN(item_id) AS item_id, MIN(name) AS name
		   FROM pigparse_price
		   WHERE name IS NOT NULL AND trim(name) <> ''
		     AND lower(trim(name)) NOT IN (SELECT norm FROM held_names)
		     AND item_id NOT IN (SELECT item_id FROM item_master)
		   GROUP BY lower(trim(name))
		 )
		 SELECT item_id, name FROM held
		 UNION ALL
		 SELECT item_id, name FROM catalog
		 ORDER BY item_id`)
	if err != nil {
		return nil, fmt.Errorf("query distinct enrichment refs: %w", err)
	}
	defer rows.Close()

	var refs []ItemRef
	for rows.Next() {
		var ref ItemRef
		if err := rows.Scan(&ref.ItemID, &ref.Name); err != nil {
			return nil, fmt.Errorf("scan enrichment item ref: %w", err)
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enrichment item refs: %w", err)
	}
	return refs, nil
}

// IconCoverage is the CURRENT icon-coverage state of item_master for the D-03
// maintainer diagnostic (ENRICH-15): Total enriched rows, how many carry a wiki
// icon (IconCovered), how many are still icon-less (IconLess = icon_id NULL or 0 →
// the client renders the colored-tile fallback), and a bounded, name-ordered sample
// of those icon-less names so a maintainer can SEE which items still lack an icon.
type IconCoverage struct {
	Total         int
	IconCovered   int
	IconLess      int
	ResidueSample []string
}

// ItemMasterIconCoverage reads the WHOLE item_master table (not per-pass deltas) so
// the icon-less residue stays visible on EVERY weekly run — even in steady state
// when most pages 304-skip and a delta-based count would collapse toward zero while
// hundreds of items still render the colored tile. sampleCap bounds the residue name
// list (the slog self-DoS guard, T-38-04); only PUBLIC item names are read (never
// statsblock/wikitext bodies, V7). Read side: a count query (fixed projection) plus
// one bounded-LIMIT sample query (the limit is the sole ? parameter).
func (s *Store) ItemMasterIconCoverage(ctx context.Context, sampleCap int) (IconCoverage, error) {
	var cov IconCoverage
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*),
		        COALESCE(SUM(CASE WHEN icon_id IS NOT NULL AND icon_id > 0 THEN 1 ELSE 0 END), 0)
		 FROM item_master`,
	).Scan(&cov.Total, &cov.IconCovered); err != nil {
		return IconCoverage{}, fmt.Errorf("query item_master icon coverage: %w", err)
	}
	cov.IconLess = cov.Total - cov.IconCovered

	if sampleCap <= 0 || cov.IconLess == 0 {
		return cov, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM item_master
		 WHERE (icon_id IS NULL OR icon_id = 0) AND name IS NOT NULL AND trim(name) <> ''
		 ORDER BY name
		 LIMIT ?`, sampleCap)
	if err != nil {
		return IconCoverage{}, fmt.Errorf("query item_master icon-less sample: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return IconCoverage{}, fmt.Errorf("scan icon-less name: %w", err)
		}
		cov.ResidueSample = append(cov.ResidueSample, n)
	}
	if err := rows.Err(); err != nil {
		return IconCoverage{}, fmt.Errorf("iterate icon-less names: %w", err)
	}
	return cov, nil
}
