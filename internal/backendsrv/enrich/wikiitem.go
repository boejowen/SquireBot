package enrich

// Pure parser for P1999 {{Itempage}} wikitext. Ported 1:1 from
// apps-script/src/lib/wiki-parser.ts (parseItempage, computeSha1Hex,
// pageNameToSlug, wikiUrlFor, extractItempageBody, parseTemplateParams,
// parseStatsblock, extractSummary) + the types in wiki-types.ts. No side
// effects, no API calls, no logging. Algorithm verified against the 5 wiki
// fixtures in testdata/ (Cloth Cap, Pearl, Cloak of Flames, Fungus Covered
// Scale Tunic, Fungi Tunic redirect).
//
// SCOPE GUARD (D-8): ParsedWikiItem surfaces ONLY the fields the Sheet
// persisted to _item_master (ItemName/Summary/WikiURL/Slot/IsQuestItem/
// WikitextSHA1). The TS parser also derives ac/weight/effect/classes/is_no_drop/
// is_lore/is_magic/is_temporary — those are NOT surfaced here (the Sheet's
// trigger dropped them; adding them would break the D-7 byte-parity proof).
// is_quest_item only needs the statsblock QUEST-ITEM flag, so we don't parse
// the dropped stats at all.
//
// SHA-1 (replaces Utilities.computeDigest): Go's crypto/sha1 returns UNSIGNED
// bytes, so the signed-byte fix-up the TS computeSha1Hex needed (because Apps
// Script returns signed bytes) is DROPPED here — Go needs no such correction.
// Output is byte-identical lowercase hex of the UTF-8 wikitext → the D-7 §2
// SHA-1 parity check holds. crypto/sha1 here is a content fingerprint, NOT a
// security hash (acceptable per RESEARCH Security V6).

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode/utf8"
)

const minWikitextLength = 200

const maxSummaryLen = 200

// ParsedWikiItem is the item-summary shape parseItempage emits — ONLY the
// fields _item_master persists (D-8 scope guard). ItemID is supplied by the
// caller/job (the inventory union), not the parser, so it is not on this struct.
type ParsedWikiItem struct {
	ItemName     string // {{Itempage|itemname=...}} or the page title
	Summary      string // first ~200 chars of `notes`, links rendered as text
	WikiURL      string // https://wiki.project1999.com/<slug>
	Slot         string // e.g. "HEAD", "CHEST", "BACK"; "" when absent
	IsQuestItem  bool   // statsblock contains "QUEST ITEM"
	WikitextSHA1 string // lowercase hex SHA-1 of the UTF-8 wikitext (change detection)
}

// WikiQuestItemLink is one quest reference harvested from an item's notes (the
// caller fills ItemID). Source is "in_game_flag" (the QUEST-ITEM stat flag) or
// "notes_link" (a [[wiki link]] in the notes body). SourceURL is the derived
// wiki URL for a notes_link, "" for the in_game_flag pseudo-link.
type WikiQuestItemLink struct {
	QuestName string
	Source    string
	SourceURL string
}

// ParseItempage parses {{Itempage}} wikitext into the item summary + its quest
// links. Returns (item, questLinks, ok, reason). ok is false (with a reason)
// when the wikitext is shorter than minWikitextLength (a redirect stub or an
// error page — V5 input validation) or when no {{Itempage}} template is present
// ("no_itempage"). Mirrors the TS ParseResult discriminated union.
func ParseItempage(wikitext, pageTitle string) (ParsedWikiItem, []WikiQuestItemLink, bool, string) {
	if len(wikitext) < minWikitextLength {
		return ParsedWikiItem{}, nil, false, "wikitext_too_short"
	}

	blockBody, found := extractItempageBody(wikitext)
	if !found {
		return ParsedWikiItem{}, nil, false, "no_itempage"
	}

	params := parseTemplateParams(blockBody)
	itemname := strings.TrimSpace(getParam(params, "itemname", pageTitle))
	notesRaw := strings.TrimSpace(getParam(params, "notes", ""))
	statsblockRaw := strings.TrimSpace(getParam(params, "statsblock", ""))

	flags, kv := parseStatsblock(statsblockRaw)
	summary := extractSummary(notesRaw)

	item := ParsedWikiItem{
		ItemName:     itemname,
		Summary:      summary,
		WikiURL:      wikiURLFor(pageTitle),
		Slot:         kv["Slot"], // "" when absent (TS: ?? null)
		IsQuestItem:  flags["QUEST ITEM"],
		WikitextSHA1: sha1Hex(wikitext),
	}

	questLinks := harvestQuestLinks(notesRaw, item)
	return item, questLinks, true, ""
}

// pageNameToSlug converts a page title to a wiki URL path segment: spaces →
// underscores, then percent-encode like JS encodeURIComponent. Mirrors the TS
// encodeURIComponent(name.replace(/ /g,'_')).
//
// Go's url.PathEscape is NOT equivalent: it escapes apostrophes and several
// sub-delims that encodeURIComponent leaves intact (e.g. "Lord_Nagafen's_Lair"
// must keep the bare '). So we re-implement encodeURIComponent's exact
// unreserved set: A-Za-z0-9 and - _ . ! ~ * ' ( ). Everything else is
// percent-encoded byte-by-byte over the UTF-8 encoding.
func pageNameToSlug(name string) string {
	return EncodeURIComponent(strings.ReplaceAll(name, " ", "_"))
}

// EncodeURIComponent mirrors JavaScript's encodeURIComponent exactly: every
// byte of the UTF-8 string is percent-encoded except the unreserved set
// A-Za-z0-9-_.!~*'() (uppercase hex, matching JS). This is required for
// byte-parity of the wiki URLs the Sheet wrote (item wiki_url + quest_items
// source_url) AND for the wiki request URLs the jobs build (jobs.wikiParseURL),
// so there is ONE escaper for wiki page names across the package boundary.
func EncodeURIComponent(s string) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isURIComponentUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperhex[c>>4])
		b.WriteByte(upperhex[c&0x0f])
	}
	return b.String()
}

// isURIComponentUnreserved reports whether b is in JS encodeURIComponent's
// unreserved set: A-Za-z0-9 - _ . ! ~ * ' ( ).
func isURIComponentUnreserved(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
		return true
	}
	return false
}

// wikiURLFor builds the canonical https://wiki.project1999.com/<slug> URL.
func wikiURLFor(pageTitle string) string {
	return "https://wiki.project1999.com/" + pageNameToSlug(pageTitle)
}

// sha1Hex computes the lowercase hex SHA-1 of the UTF-8 bytes of s. NO
// signed-byte fix-up (Go bytes are unsigned) — byte-identical to the TS
// computeSha1Hex output.
func sha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// extractItempageBody finds the {{Itempage template and returns its body
// (between the opening {{Itempage and the matching }}, exclusive), counting
// nested {{...}} so inner templates don't terminate early. Returns ("", false)
// if no Itempage template is present (or it's unbalanced).
func extractItempageBody(wikitext string) (string, bool) {
	openIdx := strings.Index(wikitext, "{{Itempage")
	if openIdx == -1 {
		return "", false
	}
	depth := 1
	i := openIdx + 2 // skip past the opening '{{'
	n := len(wikitext)
	for i < n {
		ch := wikitext[i]
		var next byte
		if i+1 < n {
			next = wikitext[i+1]
		}
		if ch == '{' && next == '{' {
			depth++
			i += 2
			continue
		}
		if ch == '}' && next == '}' {
			depth--
			if depth == 0 {
				return wikitext[openIdx+2 : i], true
			}
			i += 2
			continue
		}
		i++
	}
	return "", false // unbalanced
}

// parseTemplateParams splits a template body on `|` at depth-0 and returns the
// named (key=value) params. segments[0] is the template name itself (skipped).
func parseTemplateParams(body string) map[string]string {
	params := map[string]string{}
	segments := splitAtDepthZero(body, '|')
	for s := 1; s < len(segments); s++ {
		seg := segments[s]
		eq := strings.Index(seg, "=")
		if eq == -1 {
			continue
		}
		key := strings.TrimSpace(seg[:eq])
		value := seg[eq+1:]
		if key != "" {
			params[key] = value
		}
	}
	return params
}

// splitAtDepthZero splits input on delim, but only where {{ }} nesting depth is
// zero (so nested templates aren't shredded). Mirrors the TS splitAtDepthZero.
func splitAtDepthZero(input string, delim byte) []string {
	var out []string
	depth := 0
	start := 0
	n := len(input)
	for i := 0; i < n; i++ {
		ch := input[i]
		var next byte
		if i+1 < n {
			next = input[i+1]
		}
		if ch == '{' && next == '{' {
			depth++
			i++
			continue
		}
		if ch == '}' && next == '}' {
			depth--
			i++
			continue
		}
		if depth == 0 && ch == delim {
			out = append(out, input[start:i])
			start = i + 1
		}
	}
	out = append(out, input[start:])
	return out
}

var (
	// <br>, <br/>, <br /> (case-insensitive) — statsblock line separator.
	brRe = regexp.MustCompile(`(?i)<br\s*/?>`)
	// A standalone flag line: all uppercase letters, spaces, hyphens.
	flagRe = regexp.MustCompile(`^[A-Z][A-Z\s\-]+$`)
)

// parseStatsblock splits the statsblock (HTML-in-wikitext) on <br>, then
// classifies each line as a standalone flag (no colon, all-uppercase) or a
// key:value pair. Multi-stat lines ("STR: +2  DEX: -10") are split further.
// Returns (flags set, kv map). Mirrors the TS parseStatsblock.
func parseStatsblock(raw string) (map[string]bool, map[string]string) {
	flags := map[string]bool{}
	kv := map[string]string{}
	for _, l := range brRe.Split(raw, -1) {
		line := strings.TrimSpace(l)
		if line == "" {
			continue
		}
		if !strings.Contains(line, ":") {
			upper := strings.ToUpper(line)
			if flagRe.MatchString(upper) {
				flags[upper] = true
			}
			continue
		}
		for _, piece := range splitMultiStat(line) {
			colonAt := strings.Index(piece, ":")
			if colonAt == -1 {
				continue
			}
			key := strings.TrimSpace(piece[:colonAt])
			value := strings.TrimSpace(piece[colonAt+1:])
			if key != "" {
				kv[key] = value
			}
		}
	}
	return flags, kv
}

// splitMultiStat splits a line on runs of 2+ spaces that precede a short
// stat-key+colon (1-5 letters then ':'). RE2 has no lookahead, so this is a
// manual scan equivalent to the TS regex
// /\s{2,}(?=[A-Za-z][A-Za-z]?[A-Za-z]?[A-Za-z]?[A-Za-z]?:)/. For a simple
// "Slot: HEAD" line it returns ["Slot: HEAD"] (single piece).
func splitMultiStat(line string) []string {
	var out []string
	start := 0
	i := 0
	n := len(line)
	for i < n {
		if line[i] != ' ' {
			i++
			continue
		}
		// Count the run of spaces.
		j := i
		for j < n && line[j] == ' ' {
			j++
		}
		if j-i >= 2 && isStatKeyAhead(line[j:]) {
			out = append(out, line[start:i])
			start = j
			i = j
			continue
		}
		i = j
	}
	out = append(out, line[start:])
	return out
}

// isStatKeyAhead reports whether s begins with 1-5 ASCII letters followed by a
// colon — the lookahead the TS splitMultiStat regex asserts.
func isStatKeyAhead(s string) bool {
	letters := 0
	for letters < len(s) && letters < 5 && isASCIILetter(s[letters]) {
		letters++
	}
	if letters == 0 {
		return false
	}
	return letters < len(s) && s[letters] == ':'
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

var (
	// [[target]] or [[target|display]] — used by extractSummary (render as
	// display ?? target) and indirectly mirrored in harvestQuestLinks.
	summaryLinkRe = regexp.MustCompile(`\[\[([^|\]]+)(?:\|([^\]]+))?\]\]`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
	templateRe    = regexp.MustCompile(`\{\{[^}]+\}\}`)
	wsRe          = regexp.MustCompile(`\s+`)
	// [[target]] with optional #anchor and optional |display — quest-link
	// harvest (captures the bare target only). Mirrors the TS
	// /\[\[([^|\]#]+)(?:#[^|\]]*)?(?:\|[^\]]+)?\]\]/g.
	questLinkRe = regexp.MustCompile(`\[\[([^|\]#]+)(?:#[^|\]]*)?(?:\|[^\]]+)?\]\]`)
)

// extractSummary strips wiki-links (rendered as display text), HTML tags, and
// leftover {{templates}} from the notes body, collapses whitespace, and
// truncates to ~maxSummaryLen on a word boundary with an ellipsis. Mirrors the
// TS extractSummary exactly (including the ellipsis character "…").
func extractSummary(notes string) string {
	if notes == "" {
		return ""
	}
	// Render [[target|display]] as display, [[target]] as target.
	text := summaryLinkRe.ReplaceAllStringFunc(notes, func(m string) string {
		sub := summaryLinkRe.FindStringSubmatch(m)
		if sub[2] != "" {
			return sub[2]
		}
		return sub[1]
	})
	text = htmlTagRe.ReplaceAllString(text, " ")
	text = templateRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(wsRe.ReplaceAllString(text, " "))

	// Length + truncation are measured in RUNES, not bytes, to match the TS
	// `text.length` (UTF-16 code units) + `text.slice(0, MAX_SUMMARY_LEN)`. A
	// byte slice (text[:maxSummaryLen]) would cut a multi-byte UTF-8 rune in
	// half on a summary whose 200th byte falls inside a non-ASCII character
	// (em-dash, curly quote, accented zone/mob name) → invalid UTF-8 stored in
	// the summary. P1999 notes are all BMP, where 1 rune == 1 UTF-16 unit, so
	// the rune count matches the TS string.length exactly.
	r := []rune(text)
	if len(r) <= maxSummaryLen {
		return text
	}
	cut := string(r[:maxSummaryLen])
	lastSpace := strings.LastIndex(cut, " ")
	// The boundary check is on the RUNE-cut prefix (TS: cut.lastIndexOf(' ') vs
	// MAX_SUMMARY_LEN-30). lastSpace is a BYTE offset; convert it to a rune index
	// (== the TS UTF-16 index for BMP text) before comparing. lastSpace == -1
	// (no space in cut) ⇒ keep the full cut, exactly like the TS `-1 > 170` =
	// false branch (guarded here so cut[:-1] never panics).
	if lastSpace >= 0 && utf8.RuneCountInString(cut[:lastSpace]) > maxSummaryLen-30 {
		cut = cut[:lastSpace]
	}
	return strings.TrimSpace(cut) + "…"
}

// harvestQuestLinks emits one WikiQuestItemLink per unique [[wiki link]] target
// in the notes. If the item carries the in-game QUEST-ITEM flag, a separate
// source="in_game_flag" pseudo-link is prepended. Mirrors the TS
// harvestQuestLinks. SourceURL is the wiki URL for a notes_link, "" for the
// in_game_flag link (the Sheet wrote an empty source_url for the flag).
func harvestQuestLinks(notes string, item ParsedWikiItem) []WikiQuestItemLink {
	var links []WikiQuestItemLink
	if item.IsQuestItem {
		links = append(links, WikiQuestItemLink{
			QuestName: "[in-game QUEST flag]",
			Source:    "in_game_flag",
			SourceURL: "",
		})
	}
	if notes == "" {
		return links
	}
	seen := map[string]bool{}
	for _, m := range questLinkRe.FindAllStringSubmatch(notes, -1) {
		target := strings.TrimSpace(m[1])
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		links = append(links, WikiQuestItemLink{
			QuestName: target,
			Source:    "notes_link",
			SourceURL: wikiURLFor(target),
		})
	}
	return links
}

// getParam returns params[key] trimmed of nothing here (caller trims) — it just
// applies the TS `?? fallback` default when the key is absent.
func getParam(params map[string]string, key, fallback string) string {
	if v, ok := params[key]; ok {
		return v
	}
	return fallback
}
