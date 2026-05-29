// Package parse decodes <Char>-Inventory.txt and <Char>-Spellbook.txt files
// produced by EverQuest's /outputfile commands into rows:
// inventory → [Location, Name, ID, Count, Slots]; spellbook → [Level, Name].
//
// ENCODING CONTRACT A1 (locked in Phase 11; RESEARCH "Encoding Note" resolution
// 1/2): Parse and ParseSpellbook treat their io.Reader as ALREADY UTF-8. The
// shared entries do NOT decode CP1252 — they trust the caller.
//
//	P13 HANDOFF — do NOT double-decode: the watcher MUST CP1252→UTF-8 decode
//	(via parse.CP1252Reader) before POSTing `content` to the backend ingest API;
//	the server parser assumes UTF-8 and will mojibake raw CP1252. The existing v1
//	watcher in internal/app/runapp.go already wraps its disk reads in
//	CP1252Reader as of Phase 11 (so it keeps decoding CP1252 off disk with no
//	behavior change). Wrap a CP1252 source exactly once — never feed pre-decoded
//	UTF-8 back through CP1252Reader.
package parse

import (
	"encoding/csv"
	"io"
	"strconv"

	"golang.org/x/text/encoding/charmap"
)

// CP1252Reader wraps r to decode Windows-1252 → UTF-8. The WATCHER (P13 + the
// existing v1 watcher in internal/app/runapp.go) uses this on the disk-read side
// before parsing / POSTing UTF-8 `content`; the server parser does NOT (the wire
// payload is already UTF-8 JSON — RESEARCH "Encoding Note" / contract A1).
//
// This is the SINGLE source of the CP1252 decode after Parse/ParseSpellbook
// stopped decoding: callers that read raw EverQuest .txt bytes off disk must
// wrap their reader in CP1252Reader; callers handed already-UTF-8 bytes (the
// backend ingest path) must NOT (double-decoding produces mojibake).
func CP1252Reader(r io.Reader) io.Reader {
	return charmap.Windows1252.NewDecoder().Reader(r)
}

// Parse reads <Char>-Inventory.txt content (UTF-8, tab-separated, optional header).
// Returns rows of EXACTLY 5 columns each: [Location, Name, ID, Count, Slots].
// Per WATCH-04: tolerate extra trailing columns, accept a header row.
// Per RESEARCH.md §9.3: rows with non-int IDs are silently skipped.
//
// ENCODING (contract A1): r is treated as ALREADY UTF-8 — Parse does NOT decode
// CP1252. Callers reading raw EverQuest .txt bytes off disk must wrap their
// reader in CP1252Reader first (the watcher's path); callers handed UTF-8 bytes
// (the backend ingest path) feed r straight in. Do not double-decode.
//
// Returns (nil, nil) for an empty file (caller should treat as a no-op write).
// Per CLAUDE.md / RESEARCH.md §8.3: this function never logs raw content
// (T-04-07) and never trusts fsnotify event payload data (that discipline lives
// in internal/watch).
func Parse(r io.Reader) (rows [][]string, err error) {
	cr := csv.NewReader(r)
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
