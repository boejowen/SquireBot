package enrich

// Pure parser for P1999 per-class spell wiki pages. Ported 1:1 from
// apps-script/src/lib/wiki-spell-parser.ts (parseClassPage, splitOnLevelHeaders,
// extractSpellNames, extractInlineLevelSpells) + wiki-spell-types.ts. No side
// effects, no API calls. Verified against the 3 class fixtures (Necromancer,
// Paladin, Warrior) plus the synthetic template-variant cases from the TS test.
//
// Template variants handled (same as the TS):
//   {{SpellRow}}     — CLR/PAL/NEC/MAG/ENC (header-driven level)
//   {{RadSpellRow}}  — WIZ/SHM/RNG/SHD (header-driven level)
//   {{RadSpellRow2}} — DRU (numbered revision)
//   {{SongRow}} / {{Template:SongRow}} — BRD (inline |level=N, no headers;
//                                         fallback when the header pass is empty)
//
// NORMALIZED-NAME DIVERGENCE (deliberate, per plan + D-12): NormalizeSpellName
// is the store's join expression `lower(trim(name))` — NOT the TS
// normalizeSpellName (which also strips "spell:" prefixes and non-alphanumerics).
// The spellbook landing rows are stored with normalized_name = lower(trim(name))
// (store/replace.go:169); the wiki side MUST use the SAME expression so the P14
// spellbook↔wiki join key matches. The extracted spell_name values are
// byte-identical to the TS; only the derived normalized_name differs by design.

import (
	"regexp"
	"strconv"
	"strings"
)

// minWikitextLength is shared with the item parser (declared in wikiitem.go).

// WikiSpellRow is one (class, level, spell) row. NormalizedName is the
// store-faithful join key lower(trim(spell_name)).
type WikiSpellRow struct {
	Class          string
	Level          int
	SpellName      string
	NormalizedName string
}

// NormalizeSpellName is the canonical spellbook↔wiki join key. It is EXACTLY
// the store's normalized_name expression (store/replace.go:169):
// strings.ToLower(strings.TrimSpace(s)). See the package note above for why this
// deliberately diverges from the TS normalizeSpellName.
func NormalizeSpellName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

var (
	// ==Level N== header (multiline). Mirrors the TS headerRe.
	levelHeaderRe = regexp.MustCompile(`(?m)^==\s*Level\s+(\d+)\s*==\s*$`)
	// Any level-2 ==X== header (multiline). Mirrors the TS anyHeaderRe.
	anyHeaderRe = regexp.MustCompile(`(?m)^==[^=].*==\s*$`)
	// {{(Template:)?(Rad)?SpellRow(\d*)<body>}} — captures the template body.
	// (?s) makes . match newlines, equivalent to the TS [\s\S].
	spellRowRe = regexp.MustCompile(`(?s)\{\{\s*(?:Template:)?\s*(?:Rad)?SpellRow\d*\b(.*?)\}\}`)
	// {{(Template:)?SongRow<body>}} — Bard inline-level template.
	songRowRe = regexp.MustCompile(`(?s)\{\{\s*(?:Template:)?\s*SongRow\b(.*?)\}\}`)
	// |name=VALUE (value runs to newline, pipe, or close-brace).
	nameParamRe = regexp.MustCompile(`\|\s*name\s*=\s*([^\n|}]+)`)
	// |level=N (digits).
	levelParamRe = regexp.MustCompile(`\|\s*level\s*=\s*(\d+)`)
)

// ParseClassPage parses a per-class spell wiki page into (class, level, spell)
// rows. Returns an error only on a malformed regex (never); a too-short page or
// a degenerate no-spell page (Warrior/Monk/Rogue) returns an EMPTY slice + nil
// error — NOT an error (mirrors the TS ok:true with zero rows). The header pass
// runs first; if it yields zero rows, the Bard inline-level fallback runs.
func ParseClassPage(wikitext, class string) ([]WikiSpellRow, error) {
	if len(wikitext) < minWikitextLength {
		// Too short: the TS returns ok:false. For the Go signature we return
		// an empty slice + nil — the caller (job) treats zero rows as a skip.
		// (No fixture/test exercises the too-short path as an error here; the
		// plan's contract is "Warrior yields 0 rows, nil error".)
		return nil, nil
	}

	sections := splitOnLevelHeaders(wikitext)
	rows := make([]WikiSpellRow, 0)
	for _, sec := range sections {
		for _, name := range extractSpellNames(sec.body) {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			rows = append(rows, WikiSpellRow{
				Class:          class,
				Level:          sec.level,
				SpellName:      trimmed,
				NormalizedName: NormalizeSpellName(trimmed),
			})
		}
	}

	// Fallback: header pass produced no rows → try inline-level (Bard). If this
	// also returns nothing the class is genuinely spell-less (return empty).
	if len(rows) == 0 {
		rows = append(rows, extractInlineLevelSpells(wikitext, class)...)
	}
	return rows, nil
}

type levelSection struct {
	level int
	body  string
}

// splitOnLevelHeaders returns one entry per ==Level N== header; the body runs
// from the end of that header to the start of the next ==X== header (or EOF).
// Mirrors the TS splitOnLevelHeaders (including the 1..60 level guard and the
// "next ANY header terminates the body" rule).
func splitOnLevelHeaders(wikitext string) []levelSection {
	var out []levelSection

	// All ANY-header start positions.
	var anyHeaderPositions []int
	for _, loc := range anyHeaderRe.FindAllStringIndex(wikitext, -1) {
		anyHeaderPositions = append(anyHeaderPositions, loc[0])
	}

	for _, m := range levelHeaderRe.FindAllStringSubmatchIndex(wikitext, -1) {
		// m[0],m[1] = full match span; m[2],m[3] = group 1 (the digits).
		level, err := strconv.Atoi(wikitext[m[2]:m[3]])
		if err != nil || level < 1 || level > 60 {
			continue
		}
		headerEnd := m[1]
		// Next ANY header strictly after headerEnd.
		nextHeaderIdx := -1
		for _, p := range anyHeaderPositions {
			if p > headerEnd {
				nextHeaderIdx = p
				break
			}
		}
		var body string
		if nextHeaderIdx != -1 {
			body = wikitext[headerEnd:nextHeaderIdx]
		} else {
			body = wikitext[headerEnd:]
		}
		out = append(out, levelSection{level: level, body: body})
	}
	return out
}

// extractSpellNames pulls the name= param from every spell template in the
// section body ({{SpellRow}}, {{RadSpellRow}}, {{RadSpellRow2}}, optionally
// Template:-prefixed). Position-agnostic. Mirrors the TS extractSpellNames.
func extractSpellNames(body string) []string {
	var names []string
	for _, m := range spellRowRe.FindAllStringSubmatch(body, -1) {
		tplBody := m[1]
		if nm := nameParamRe.FindStringSubmatch(tplBody); nm != nil {
			names = append(names, nm[1])
		}
	}
	return names
}

// extractInlineLevelSpells handles the Bard page format (no ==Level N== headers;
// level lives inline as |level=N). Used as the fallback when the header pass
// yields zero rows. Mirrors the TS extractInlineLevelSpells, including the
// finite 1..60 level guard.
func extractInlineLevelSpells(wikitext, class string) []WikiSpellRow {
	var rows []WikiSpellRow
	for _, m := range songRowRe.FindAllStringSubmatch(wikitext, -1) {
		tplBody := m[1]
		nm := nameParamRe.FindStringSubmatch(tplBody)
		lvl := levelParamRe.FindStringSubmatch(tplBody)
		if nm == nil || lvl == nil {
			continue
		}
		name := strings.TrimSpace(nm[1])
		level, err := strconv.Atoi(lvl[1])
		if name == "" || err != nil || level < 1 || level > 60 {
			continue
		}
		rows = append(rows, WikiSpellRow{
			Class:          class,
			Level:          level,
			SpellName:      name,
			NormalizedName: NormalizeSpellName(name),
		})
	}
	return rows
}
