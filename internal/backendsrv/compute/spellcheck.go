package compute

// spellcheck.go is the Go reimplementation of apps-script/src/tabs/buildSpellCheck.ts
// — the consolidated `spell_check` grid and the other WEB-02 parity heart. Per
// character (with a class set and level >= 1), it emits one row per class spell
// the char is eligible for (wiki spell level <= char level) with KNOWN/MISSING
// from the char's spellbook (buildSpellCheck.ts:76-86).
//
// Join key: normalized_name. Both spellbook_entry.normalized_name (replace.go:169)
// and wiki_spells.normalized_name (enrich.go:248) are ALREADY materialized in the
// DB with the identical lower(trim(name)) expression, so the join is a direct set
// membership on the materialized value — NO recompute (cleaner than v1, which
// normalized at read time). Sort: Char asc → level asc → spell asc.

import (
	"context"
	"sort"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// SpellCheck computes the consolidated `spell_check` grid over the store. It
// mirrors buildSpellCheck.ts: classed characters of level >= 1 get KNOWN/MISSING
// rows for each in-range class spell; output is sorted Char asc → level asc →
// spell asc.
func SpellCheck(ctx context.Context, s *store.Store) ([]SpellCheckRow, error) {
	chars, err := s.CharsWithMeta(ctx)
	if err != nil {
		return nil, err
	}
	wiki, err := s.WikiSpells(ctx)
	if err != nil {
		return nil, err
	}
	known, err := s.SpellbookNormalizedByChar(ctx)
	if err != nil {
		return nil, err
	}
	return buildSpellCheckRows(chars, wiki, known), nil
}

// buildSpellCheckRows is the pure transform (no store access) shared with the
// parity tests.
func buildSpellCheckRows(chars []store.CharMeta, wiki []store.WikiSpellRow, known map[string]map[string]bool) []SpellCheckRow {
	// Group wiki spells by class.
	wikiByClass := make(map[string][]store.WikiSpellRow)
	for _, w := range wiki {
		wikiByClass[w.Class] = append(wikiByClass[w.Class], w)
	}

	var out []SpellCheckRow
	for _, c := range chars {
		// Skip chars with no class or level < 1 (buildSpellCheck.ts:77). A NULL
		// level resolves to 0 in CharsWithMeta, which is < 1 → skipped.
		if c.Class == "" || c.Level < 1 {
			continue
		}
		charKnown := known[c.Name] // nil map → membership is always false
		for _, w := range wikiByClass[c.Class] {
			if w.Level > c.Level {
				continue // level-gating: only spells the char's level can learn
			}
			status := "MISSING"
			if charKnown[w.NormalizedName] {
				status = "KNOWN"
			}
			out = append(out, SpellCheckRow{
				Char:   c.Name,
				Class:  c.Class,
				Level:  w.Level,
				Spell:  w.SpellName,
				Status: status,
			})
		}
	}

	sortSpellCheckRows(out)
	return out
}

// sortSpellCheckRows sorts Char asc → level asc → spell asc (buildSpellCheck.ts:89-95).
func sortSpellCheckRows(rows []SpellCheckRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Char != b.Char {
			return a.Char < b.Char
		}
		if a.Level != b.Level {
			return a.Level < b.Level
		}
		return a.Spell < b.Spell
	})
}
