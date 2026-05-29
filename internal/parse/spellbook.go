package parse

// ParseSpellbook decodes <Char>-Spellbook.txt files produced by EverQuest's
// /outputfile spellbook command into 2-column rows: [Level, Name].
//
// File format (CONTEXT.md §Spellbook File Format, 02-CONTEXT.md):
//   - Tab-separated, Win-1252 encoded.
//   - Optional header row "Level\tName" (detected by non-numeric column 0).
//   - Two columns: Level (integer 1-60) and Name (non-empty string).
//   - NO spell IDs (Phase 4 spell_check joins by normalized spell name).
//   - NO mem-slot info (this is the scribed list, not active mem slots).
//
// This file mirrors the inventory parser at internal/parse/inventory.go.
// Encoding matches inventory (contract A1): ParseSpellbook treats r as ALREADY
// UTF-8 and does NOT decode CP1252 — the watcher wraps its disk reader in
// CP1252Reader (defined in inventory.go) first. csv.Reader with Comma='\t' and
// LazyQuotes=true (EQ spell names may contain stray apostrophes — e.g.
// "Cazic-Thule's Wrath"), FieldsPerRecord=-1 to tolerate extra trailing columns
// per WATCH-05.

import (
	"encoding/csv"
	"io"
	"strconv"
)

// ParseSpellbook reads a <Char>-Spellbook.txt file (Win-1252, tab-separated,
// optional header). Returns rows of EXACTLY 2 columns each: [Level, Name].
//
// Level is preserved as a string but validated as an integer; rows with a
// non-integer Level or fewer than 2 columns are silently skipped (mirrors
// inventory's row-with-bad-ID behavior). The level range 1-60 is informational
// only — the parser does not reject 0 or 100 (the EQ format documents 1-60
// but we tolerate).
//
// Per WATCH-05: extra trailing columns are tolerated and truncated to 2.
//
// ENCODING (contract A1): r is treated as ALREADY UTF-8 — ParseSpellbook does
// NOT decode CP1252. The watcher wraps its disk reader in CP1252Reader first;
// the backend ingest path feeds UTF-8 content straight in. Do not double-decode.
//
// Returns (nil, nil) for empty input — caller treats as a no-op write.
//
// Per CLAUDE.md / RESEARCH.md §8.3: this function never logs raw content
// (T-04-07) and never trusts fsnotify event payload data (that discipline lives
// in internal/watch).
func ParseSpellbook(r io.Reader) (rows [][]string, err error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.FieldsPerRecord = -1 // tolerate any column count
	cr.LazyQuotes = true    // EQ names may contain stray apostrophes
	all, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	// Drop header IF the first row's column 0 (Level position) is NOT numeric.
	// isIntField is defined in inventory.go (same package); reuse it.
	if !isIntField(all[0], 0) {
		all = all[1:]
	}
	out := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) < 2 {
			continue
		}
		// Level must parse as a base-10 integer; skip rows where it doesn't.
		if _, err := strconv.Atoi(row[0]); err != nil {
			continue
		}
		out = append(out, []string{row[0], row[1]})
	}
	return out, nil
}
