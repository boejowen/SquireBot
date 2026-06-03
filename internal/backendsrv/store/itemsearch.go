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

// SearchCatalog returns up to `limit` pigparse_price catalog items whose name
// contains q (case-insensitively) OR whose item_id equals q, ranked prefix-first.
// q is bound through ? placeholders ONLY (never concatenated); the LIKE wildcards
// are built in Go and bound as values with ESCAPE so a user-typed %/_ is literal.
// Always returns a non-nil slice (possibly empty). A NULL-name row is scanned
// safely (sql.NullString → "") so an id-match on it never errors (review WORTH-FIX 4).
func (s *Store) SearchCatalog(ctx context.Context, q string, limit int) ([]CatalogItem, error) {
	like := "%" + escapeLike(q) + "%"
	prefix := escapeLike(q) + "%"
	rows, err := s.db.QueryContext(ctx,
		"SELECT item_id, name, current_avg FROM pigparse_price "+
			"WHERE name LIKE ? ESCAPE '\\' OR CAST(item_id AS TEXT) = ? "+
			"ORDER BY (name LIKE ? ESCAPE '\\') DESC, length(name), name COLLATE NOCASE "+
			"LIMIT ?",
		like, q, prefix, limit)
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
