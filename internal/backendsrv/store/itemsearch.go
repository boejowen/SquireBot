package store

// itemsearch.go is the Phase 19 D-10 full-catalog item search. It is a plain
// (*Store) read method (no tx), mirroring readviews.go's InventoryJoin idiom:
// build a fixed-string query with ? placeholders, s.db.QueryContext, defer
// rows.Close(), scan loop, rows.Err().
//
// The corpus is pigparse_price — the FULL daily-ingested Blue item catalog — NOT
// the guild-SEEN-only item table (searching that subset is the trap D-10 exists to
// avoid — 19-RESEARCH Pitfall A1). Pinning a want from pigparse_price.item_id makes
// it both in-bank-joinable and alert-matchable in Phase 21+ (same id space —
// InventoryJoin already LEFT JOINs inventory.item_id = pigparse_price.item_id).
//
// SQL discipline:
//   - The search term q and the built LIKE wildcards are bound as ? VALUES with
//     ESCAPE '\' — q is NEVER concatenated into the SQL (V5/Tampering). A user-typed
//     %/_/\ is escaped to a literal, not a wildcard (Pitfall A5).
//   - SQLite's LIKE is ALREADY ASCII-case-insensitive, so COLLATE NOCASE is NOT put
//     on the LIKE terms (it would be a no-op there — review WORTH-FIX 7). It appears
//     ONLY on the ORDER BY name tiebreak, where it genuinely affects sort order. A
//     future maintainer must NOT "fix" case-insensitivity by adding COLLATE to the
//     LIKE — the LIKE keyword itself provides it.
//   - name is scanned via sql.NullString (the column is nullable — review WORTH-FIX 4)
//     so an id-match on a NULL-name row never errors; current_avg via sql.NullFloat64.
//   - On error the wrap carries len(q) ONLY — never the q value (V7 / no PII in logs).

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// CatalogItem is one pigparse_price catalog hit. CurrentAvg (price) is optional
// display polish (omitempty; nil when the row has no current_avg). item_id + name
// are the load-bearing fields (the pin + the snapshot label). The JSON tags match
// the frontend CatalogItem interface (api.ts).
type CatalogItem struct {
	ItemID     int64    `json:"item_id"`
	Name       string   `json:"name"`
	CurrentAvg *float64 `json:"current_avg,omitempty"`
}

// escapeLike escapes the SQL LIKE metacharacters (\, %, _) with a leading
// backslash so a user-typed wildcard is matched LITERALLY when the term is bound
// with `ESCAPE '\'`. The backslash itself is escaped FIRST so it does not double-
// escape the % and _ replacements.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// flagUnion is the name-keyed flag source for the Phase 39 catalog facets: the
// item_master (held, EQ-id-keyed) ∪ catalog_enrichment (catalog-only, norm_name-keyed)
// is_clicky/has_haste columns, joined to pigparse_price by lower(trim(name)). It mirrors
// CatalogIconCoverage (store/itemids.go:193-205), swapping icon_id for the two flag columns.
// item_id is NEVER the join key (PigParse vs EQ namespace — Pitfall 1 / memory
// pigparse-vs-ingame-item-id-namespaces). Added to the query ONLY when a facet is active so
// the no-facet path stays byte-identical to the original single-table search (Pitfall 3 —
// the join is LEFT and conditional, never an unconditional/always-on join that would drop
// unenriched catalog rows when no facet is active).
//
// The inner union is GROUP BY norm with MAX(flag) so the subquery yields EXACTLY ONE row per
// normalized name. item_master.name is NOT unique (the table is keyed by item_id), so N
// distinct held EQ ids can share a normalized name (e.g. same-name spell scrolls / quest
// turn-ins). Without the GROUP BY this LEFT JOIN would fan a single catalog row out to N
// duplicate result rows (BL-01: duplicate Svelte {#each} keys → each_key_duplicate crash +
// LIMIT 25 undercount). MAX(flag) = "the name carries the flag if ANY same-name item does" —
// the correct name-keyed-facet semantics — and it also collapses any (invariant-violating)
// cross-table norm overlap defensively (NULL flags sort below 1, so MAX prefers a real flag).
const flagUnion = `
  LEFT JOIN (
    SELECT norm, MAX(is_clicky) AS is_clicky, MAX(has_haste) AS has_haste FROM (
      SELECT lower(trim(name)) AS norm, is_clicky, has_haste FROM item_master
      UNION ALL
      SELECT norm_name          AS norm, is_clicky, has_haste FROM catalog_enrichment
    ) GROUP BY norm
  ) f ON f.norm = lower(trim(pigparse_price.name))`

// SearchCatalog returns up to `limit` pigparse_price catalog items whose name
// contains q (case-insensitively) OR whose item_id equals q, ranked prefix-first.
// The optional clicky/haste facets (Phase 39, SEARCH-04/05) AND-narrow the result to
// rows whose name carries the matching flag in the item_master ∪ catalog_enrichment
// name-keyed union (joined by lower(trim(name)), NEVER item_id). With NEITHER facet active
// the query is byte-identical to the original single-table search (the LEFT JOIN is absent).
// q is bound through ? placeholders ONLY (never concatenated); the LIKE wildcards
// are built in Go and bound as values with ESCAPE so a user-typed %/_ is literal. The facet
// bools select a FIXED predicate fragment — no user string reaches SQL. Always returns a
// non-nil slice (possibly empty). A NULL-name row is scanned safely (sql.NullString → "")
// so an id-match on it never errors (review WORTH-FIX 4). Resolved-default Open Q1: the
// prefix-first ORDER BY is kept for catalog scope (NOT re-sorted viewer-first).
func (s *Store) SearchCatalog(ctx context.Context, q string, clicky, haste bool, limit int) ([]CatalogItem, error) {
	like := "%" + escapeLike(q) + "%"
	prefix := escapeLike(q) + "%"

	var facet strings.Builder
	if clicky {
		facet.WriteString(" AND COALESCE(f.is_clicky,0) = 1")
	}
	if haste {
		facet.WriteString(" AND COALESCE(f.has_haste,0) = 1")
	}

	query := "SELECT pigparse_price.item_id, pigparse_price.name, pigparse_price.current_avg " +
		"FROM pigparse_price"
	if clicky || haste {
		query += flagUnion // join ONLY when a facet is active (keep the no-facet path identical)
	}
	query += " WHERE (pigparse_price.name LIKE ? ESCAPE '\\' OR CAST(pigparse_price.item_id AS TEXT) = ?)" +
		facet.String() +
		" ORDER BY (pigparse_price.name LIKE ? ESCAPE '\\') DESC, length(pigparse_price.name), pigparse_price.name COLLATE NOCASE" +
		" LIMIT ?"

	rows, err := s.db.QueryContext(ctx, query, like, q, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("search catalog (len=%d): %w", len(q), err)
	}
	defer rows.Close()

	out := make([]CatalogItem, 0) // non-nil → JSON []
	for rows.Next() {
		var (
			it    CatalogItem
			nameN sql.NullString
			avgN  sql.NullFloat64
		)
		if err := rows.Scan(&it.ItemID, &nameN, &avgN); err != nil {
			return nil, fmt.Errorf("scan catalog row (len=%d): %w", len(q), err)
		}
		if nameN.Valid {
			it.Name = nameN.String
		}
		if avgN.Valid {
			v := avgN.Float64
			it.CurrentAvg = &v
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog rows (len=%d): %w", len(q), err)
	}
	return out, nil
}
