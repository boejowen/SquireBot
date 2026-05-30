package enrich

// Pure parser for P1999 Velious gear-tier wiki pages. Ported 1:1 from
// apps-script/src/lib/wiki-gear-tier-parser.ts (parseGearTierPage,
// splitOnClassHeaders, extractListItems, extractSlotLabel, extractItemNames,
// stripParenNotes) + wiki-gear-tier-types.ts. No side effects, no API calls.
// Verified against the 2 fixtures (Pre-Raid + Raiding) plus the synthetic
// edge-case strings from the TS test.
//
// Iksar tier: the Iksar racial tier has no page of its own — Iksar racial items
// sit inline on the Pre-Raid page within the regular class sections, identified
// by an "Iksar " item-name prefix. On the Pre-Raid page only, such items are
// re-tagged tier="Iksar" (a single emit per item). Mirrors the TS logic.

import (
	"regexp"
	"strings"
)

// Tier is the gear-tier label. The three constants are the exact strings from
// wiki-gear-tier-types.ts.
type Tier string

const (
	TierVeliousPreRaid Tier = "Velious Pre-Raid/Group"
	TierVeliousRaiding Tier = "Velious Raiding"
	TierIksar          Tier = "Iksar"
)

// WikiGearTierRow is one gear-tier recommendation. ItemID is ALWAYS nil — wiki
// transclusions expose no item IDs (the store side therefore uses a full-table
// replace, not an upsert keyed on item_id). Rank is 1-based within a slot.
type WikiGearTierRow struct {
	Tier     Tier
	Class    string
	Slot     string
	ItemID   *int // always nil
	ItemName string
	Rank     int
}

var (
	// == [[ClassName]] == header (multiline). Mirrors the TS classHeaderRe.
	classHeaderRe = regexp.MustCompile(`(?m)^==\s*\[\[([^\]]+)\]\]\s*==\s*$`)
	// Bolded slot label '''SlotName'''. Mirrors the TS extractSlotLabel.
	slotLabelRe = regexp.MustCompile(`'''([^']+)'''`)
	// {{:ItemName}} or {{:ItemName|...}} transclusion. Mirrors the TS itemNameRe.
	itemNameRe = regexp.MustCompile(`\{\{:([^}|]+)(?:\|[^}]*)?\}\}`)
	// ` (parenthetical) ` note within an item name. Mirrors the TS stripParenNotes.
	parenNoteRe = regexp.MustCompile(`\s*\([^)]*\)\s*`)
	multiWsRe   = regexp.MustCompile(`\s+`)
	// <li ...> opening tag (for the manual lookahead-free list-item scan).
	liOpenRe  = regexp.MustCompile(`<li[^>]*>`)
	liCloseRe = regexp.MustCompile(`</li>`)
	ulCloseRe = regexp.MustCompile(`</ul>`)
)

// ParseGearTierPage parses a Velious gear-tier page into (tier, class, slot,
// item) rows. baseTier is the page-level tier; Iksar-prefixed items on a
// Pre-Raid page are re-tagged TierIksar. Returns an error only never (the TS
// returns ok:false for too-short / no-class-sections — for the Go signature
// those yield an empty slice + nil error, which the caller treats as a skip).
func ParseGearTierPage(wikitext string, baseTier Tier) ([]WikiGearTierRow, error) {
	if len(wikitext) < minWikitextLength {
		return nil, nil
	}

	sections := splitOnClassHeaders(wikitext)
	if len(sections) == 0 {
		return nil, nil
	}

	rows := make([]WikiGearTierRow, 0)
	for _, sec := range sections {
		classAbbrev, ok := CLASS_DISPLAY_TO_ABBREV[sec.classDisplay]
		if !ok {
			continue // not a real EQ class — skip noise sections
		}
		for _, li := range extractListItems(sec.body) {
			slot := extractSlotLabel(li)
			if slot == "" {
				continue
			}
			// (The TS also tracks unknown slots not in WIKI_SLOT_TO_INV_SLOTS;
			// it still emits rows for them, so emission is unconditional here.)
			itemNames := extractItemNames(li)
			for i, itemName := range itemNames {
				isIksar := baseTier == TierVeliousPreRaid && strings.HasPrefix(itemName, "Iksar ")
				effectiveTier := baseTier
				if isIksar {
					effectiveTier = TierIksar
				}
				rows = append(rows, WikiGearTierRow{
					Tier:     effectiveTier,
					Class:    classAbbrev,
					Slot:     slot,
					ItemID:   nil,
					ItemName: itemName,
					Rank:     i + 1,
				})
			}
		}
	}
	return rows, nil
}

type classSection struct {
	classDisplay string
	body         string
}

// splitOnClassHeaders walks == [[ClassName]] == headers; each body runs to the
// next ==X== header (or EOF). Mirrors the TS splitOnClassHeaders.
func splitOnClassHeaders(wikitext string) []classSection {
	var out []classSection

	var anyHeaderPositions []int
	for _, loc := range anyHeaderRe.FindAllStringIndex(wikitext, -1) {
		anyHeaderPositions = append(anyHeaderPositions, loc[0])
	}

	for _, m := range classHeaderRe.FindAllStringSubmatchIndex(wikitext, -1) {
		classDisplay := strings.TrimSpace(wikitext[m[2]:m[3]])
		headerEnd := m[1]
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
		out = append(out, classSection{classDisplay: classDisplay, body: body})
	}
	return out
}

// extractListItems pulls <li>...</li> blocks from the section body. It is a
// lookahead-free reimplementation of the TS regex
// /<li[^>]*>([\s\S]*?)(?=<\/li>|<li[^>]*>|<\/ul>|$)/g: for each <li...> opening
// tag, the content runs from the end of that tag to the EARLIEST of the next
// </li>, the next <li...> open, the next </ul>, or end-of-string. This handles
// the unclosed-<li> case the P1999 wiki sometimes produces.
func extractListItems(body string) []string {
	opens := liOpenRe.FindAllStringIndex(body, -1)
	if len(opens) == 0 {
		return nil
	}
	closes := liCloseRe.FindAllStringIndex(body, -1)
	uls := ulCloseRe.FindAllStringIndex(body, -1)

	var out []string
	for _, open := range opens {
		contentStart := open[1]
		end := len(body)
		// Earliest </li> at or after contentStart.
		for _, c := range closes {
			if c[0] >= contentStart && c[0] < end {
				end = c[0]
				break
			}
		}
		// Earliest next <li...> open strictly after this open tag.
		for _, o := range opens {
			if o[0] > open[0] && o[0] >= contentStart && o[0] < end {
				end = o[0]
				break
			}
		}
		// Earliest </ul> at or after contentStart.
		for _, u := range uls {
			if u[0] >= contentStart && u[0] < end {
				end = u[0]
				break
			}
		}
		out = append(out, body[contentStart:end])
	}
	return out
}

// extractSlotLabel pulls the bolded '''SlotName''' from an <li>. Returns "" if
// not present. Mirrors the TS extractSlotLabel.
func extractSlotLabel(li string) string {
	if m := slotLabelRe.FindStringSubmatch(li); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// extractItemNames pulls all {{:ItemName}} transclusions from an <li>,
// stripping parenthetical notes. Mirrors the TS extractItemNames.
func extractItemNames(li string) []string {
	var out []string
	for _, m := range itemNameRe.FindAllStringSubmatch(li, -1) {
		name := stripParenNotes(strings.TrimSpace(m[1]))
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// stripParenNotes removes ` (anything)` runs and collapses whitespace.
// "Whetstone (Worn)" → "Whetstone". Mirrors the TS stripParenNotes.
func stripParenNotes(s string) string {
	s = parenNoteRe.ReplaceAllString(s, " ")
	s = multiWsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
