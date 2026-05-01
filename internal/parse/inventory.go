// Package parse decodes <Char>-Inventory.txt files produced by EverQuest's
// /outputfile inventory command into 5-column rows: [Location, Name, ID, Count, Slots].
//
// Encoding lock-in: if a real EQ-produced sample later proves UTF-8 instead of
// CP1252, swap charmap.Windows1252 with utf8 — see Open Question Q4 in
// .planning/phases/01-end-to-end-thin-slice/01-RESEARCH.md §11.
package parse

import (
	"encoding/csv"
	"io"
	"strconv"

	"golang.org/x/text/encoding/charmap"
)

// Parse reads a <Char>-Inventory.txt file (Win-1252, tab-separated, optional header).
// Returns rows of EXACTLY 5 columns each: [Location, Name, ID, Count, Slots].
// Per WATCH-04: tolerate extra trailing columns, decode Windows-1252, accept header row.
// Per RESEARCH.md §9.3: rows with non-int IDs are silently skipped.
//
// Returns (nil, nil) for an empty file (caller should treat as a no-op write).
// Per CLAUDE.md / RESEARCH.md §8.3: this function is encoding-only — it never logs raw
// content (T-04-07) and never trusts fsnotify event payload data (that discipline lives
// in internal/watch).
func Parse(r io.Reader) (rows [][]string, err error) {
	decoded := charmap.Windows1252.NewDecoder().Reader(r)
	cr := csv.NewReader(decoded)
	cr.Comma = '\t'
	cr.FieldsPerRecord = -1 // tolerate any column count
	cr.LazyQuotes = true    // EQ names may contain stray apostrophes (e.g. "Tashan's Lance")
	all, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	// Drop header IF the first row's column 2 (ID position) is NOT numeric.
	if !isIntField(all[0], 2) {
		all = all[1:]
	}
	out := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) < 5 {
			continue
		}
		// ID must parse as int; skip rows where it doesn't.
		if !isIntField(row, 2) {
			continue
		}
		out = append(out, row[:5])
	}
	return out, nil
}

// isIntField reports whether row[col] is a base-10 integer (or "0").
// col is 0-indexed; "column 2" in our schema is the ID at index 2.
func isIntField(row []string, col int) bool {
	if col >= len(row) {
		return false
	}
	_, err := strconv.Atoi(row[col])
	return err == nil
}
