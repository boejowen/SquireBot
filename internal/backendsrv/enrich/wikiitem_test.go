package enrich

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// loadWikitext reads a wiki fixture (the MediaWiki action=parse envelope) and
// returns the inner wikitext (parse.wikitext["*"]) + the page title
// (parse.title). The enrichment wiki parsers consume the inner wikitext string,
// not the envelope — exactly as the Apps Script triggers do. Shared by the
// wikiitem / wikispell / wikigear tests (same package).
func loadWikitext(t *testing.T, fixture string) (wikitext, title string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + fixture + ".json")
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	var env struct {
		Parse struct {
			Title    string `json:"title"`
			Wikitext struct {
				Star string `json:"*"`
			} `json:"wikitext"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", fixture, err)
	}
	return env.Parse.Wikitext.Star, env.Parse.Title
}

func TestPageNameToSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Cloth Cap", "Cloth_Cap"},
		{"Cloak_of_Flames", "Cloak_of_Flames"},
		{"Lord Nagafen's Lair", "Lord_Nagafen's_Lair"},
	}
	for _, c := range cases {
		if got := pageNameToSlug(c.in); got != c.want {
			t.Errorf("pageNameToSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := wikiURLFor("Cloth Cap"); got != "https://wiki.project1999.com/Cloth_Cap" {
		t.Errorf("wikiURLFor(Cloth Cap) = %q, want https://wiki.project1999.com/Cloth_Cap", got)
	}
}

func TestParseItempage_ClothCap(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-cloth-cap")
	item, links, ok, reason := ParseItempage(wikitext, title)
	if !ok {
		t.Fatalf("ParseItempage ok=false reason=%q", reason)
	}
	if item.ItemName != "Cloth Cap" {
		t.Errorf("ItemName = %q, want Cloth Cap", item.ItemName)
	}
	if item.WikiURL != "https://wiki.project1999.com/Cloth_Cap" {
		t.Errorf("WikiURL = %q", item.WikiURL)
	}
	if !item.IsQuestItem {
		t.Errorf("IsQuestItem = false, want true")
	}
	if item.Slot != "HEAD" {
		t.Errorf("Slot = %q, want HEAD", item.Slot)
	}
	if !isHex40(item.WikitextSHA1) {
		t.Errorf("WikitextSHA1 = %q, want 40-char lowercase hex", item.WikitextSHA1)
	}
	// Quest links must include the in-game flag pseudo-link.
	var flag *WikiQuestItemLink
	for i := range links {
		if links[i].Source == "in_game_flag" {
			flag = &links[i]
			break
		}
	}
	if flag == nil {
		t.Fatal("no in_game_flag quest link")
	}
	if flag.QuestName != "[in-game QUEST flag]" {
		t.Errorf("flag.QuestName = %q, want [in-game QUEST flag]", flag.QuestName)
	}
	if flag.SourceURL != "" {
		t.Errorf("in_game_flag SourceURL = %q, want empty", flag.SourceURL)
	}
}

func TestParseItempage_Pearl(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-pearl")
	item, links, ok, _ := ParseItempage(wikitext, title)
	if !ok {
		t.Fatal("ParseItempage ok=false")
	}
	if item.ItemName != "Pearl" {
		t.Errorf("ItemName = %q, want Pearl", item.ItemName)
	}
	if item.IsQuestItem {
		t.Errorf("IsQuestItem = true, want false (Pearl has no QUEST ITEM flag)")
	}
	if !strings.Contains(item.Summary, "Call of the Hero") {
		t.Errorf("Summary missing 'Call of the Hero': %q", item.Summary)
	}
	if !strings.Contains(item.Summary, "Death Pact") {
		t.Errorf("Summary missing 'Death Pact': %q", item.Summary)
	}
	// No in_game_flag link (statsblock didn't have QUEST ITEM).
	for _, l := range links {
		if l.Source == "in_game_flag" {
			t.Errorf("unexpected in_game_flag link for Pearl")
		}
	}
	// notes_link targets for the 3 spell references.
	noteTargets := map[string]bool{}
	for _, l := range links {
		if l.Source == "notes_link" {
			noteTargets[l.QuestName] = true
		}
	}
	for _, want := range []string{"Call of the Hero", "Death Pact", "Thicken Mana"} {
		if !noteTargets[want] {
			t.Errorf("notes_link targets missing %q (got %v)", want, noteTargets)
		}
	}
}

func TestParseItempage_CloakOfFlames(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-cloak-of-flames")
	item, _, ok, _ := ParseItempage(wikitext, title)
	if !ok {
		t.Fatal("ParseItempage ok=false")
	}
	if item.ItemName != "Cloak of Flames" {
		t.Errorf("ItemName = %q", item.ItemName)
	}
	if item.IsQuestItem {
		t.Errorf("IsQuestItem = true, want false (Cloak of Flames is MAGIC, not QUEST)")
	}
	if item.Slot != "BACK" {
		t.Errorf("Slot = %q, want BACK", item.Slot)
	}
}

func TestParseItempage_FungusCoveredScaleTunic(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-fungus-covered-scale-tunic")
	item, _, ok, _ := ParseItempage(wikitext, title)
	if !ok {
		t.Fatal("ParseItempage ok=false")
	}
	if item.ItemName != "Fungus Covered Scale Tunic" {
		t.Errorf("ItemName = %q", item.ItemName)
	}
	// LORE ITEM, not QUEST ITEM (the TS test asserts is_quest_item=false here).
	if item.IsQuestItem {
		t.Errorf("IsQuestItem = true, want false (LORE, not QUEST)")
	}
	if item.Slot != "CHEST" {
		t.Errorf("Slot = %q, want CHEST", item.Slot)
	}
}

func TestParseItempage_Redirect(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-fungi-tunic-redirect")
	_, _, ok, reason := ParseItempage(wikitext, title)
	if ok {
		t.Fatal("ParseItempage ok=true, want false for redirect stub")
	}
	if reason != "wikitext_too_short" {
		t.Errorf("reason = %q, want wikitext_too_short (redirect stub is 40 bytes)", reason)
	}
}

func TestParseItempage_EdgeCases(t *testing.T) {
	// Empty + tiny input → wikitext_too_short.
	if _, _, ok, reason := ParseItempage("", "Empty"); ok || reason != "wikitext_too_short" {
		t.Errorf("empty input: ok=%v reason=%q, want false/wikitext_too_short", ok, reason)
	}
	if _, _, ok, reason := ParseItempage(strings.Repeat("a", 100), "Short"); ok || reason != "wikitext_too_short" {
		t.Errorf("tiny input: ok=%v reason=%q, want false/wikitext_too_short", ok, reason)
	}
	// Long but no {{Itempage}} template → no_itempage.
	if _, _, ok, reason := ParseItempage(strings.Repeat("a", 500), "NoTpl"); ok || reason != "no_itempage" {
		t.Errorf("no-template input: ok=%v reason=%q, want false/no_itempage", ok, reason)
	}
}

// TestSHA1Hex_MatchesTS is the D-7 §2 parity signal: sha1Hex emits byte-identical
// lowercase hex to the TS computeSha1Hex. SHA1("test") is a well-known constant;
// the cloth-cap value was precomputed from the fixture's wikitext with Node's
// crypto.createHash('sha1') over the UTF-8 bytes — identical to the TS path.
func TestSHA1Hex_MatchesTS(t *testing.T) {
	if got := sha1Hex("test"); got != "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3" {
		t.Errorf(`sha1Hex("test") = %q, want a94a8fe5ccb19ba61c4c0873d391e987982fbbd3`, got)
	}
	if got := sha1Hex("hello"); got != "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d" {
		t.Errorf(`sha1Hex("hello") = %q, want aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d`, got)
	}
	// SHA-1 over the actual cloth-cap wikitext (precomputed via Node crypto).
	wikitext, title := loadWikitext(t, "wiki-parse-cloth-cap")
	item, _, ok, _ := ParseItempage(wikitext, title)
	if !ok {
		t.Fatal("cloth-cap parse failed")
	}
	if item.WikitextSHA1 != "5ed737f5859888582c5cf08e2d0721ff830b2cb1" {
		t.Errorf("cloth-cap WikitextSHA1 = %q, want 5ed737f5859888582c5cf08e2d0721ff830b2cb1", item.WikitextSHA1)
	}
}

// TestSHA1Hex_Stability mirrors the TS deterministic + change-sensitive checks.
func TestSHA1Hex_Stability(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-cloth-cap")
	a, _, _, _ := ParseItempage(wikitext, title)
	b, _, _, _ := ParseItempage(wikitext, title)
	if a.WikitextSHA1 != b.WikitextSHA1 {
		t.Errorf("SHA-1 not deterministic: %q != %q", a.WikitextSHA1, b.WikitextSHA1)
	}
	c, _, _, _ := ParseItempage(wikitext+" modified", title)
	if a.WikitextSHA1 == c.WikitextSHA1 {
		t.Errorf("SHA-1 unchanged after wikitext change: %q", a.WikitextSHA1)
	}
}

// TestExtractSummary_ASCIIShort is the byte==rune baseline: pure-ASCII input
// under the limit is returned verbatim (no regression vs the old byte path).
func TestExtractSummary_ASCIIShort(t *testing.T) {
	in := "A short note about a Cloak of Flames."
	if got := extractSummary(in); got != in {
		t.Errorf("extractSummary(%q) = %q, want verbatim", in, got)
	}
}

// TestExtractSummary_RuneTruncationValidUTF8 is the M-01 regression: a multi-byte
// rune straddling rune-position 200 must NOT be sliced mid-sequence. The TS
// measures/slices by string.length + slice (UTF-16 units); the Go port now uses
// []rune, matching wiki-parser.ts:237-241. The old byte path (text[:200]) cut the
// snowman's first byte off, emitting invalid UTF-8 ("… e2 | e2 80 a6").
func TestExtractSummary_RuneTruncationValidUTF8(t *testing.T) {
	// 199 ASCII 'a' + ☃ (U+2603, 3 bytes) as rune #200, then filler past 200.
	// No spaces, so the word-boundary branch is skipped (lastSpace == -1) and the
	// full 200-rune cut + "…" is returned — exactly the TS `cut + '…'` path.
	in := strings.Repeat("a", 199) + "☃" + strings.Repeat("b", 50)
	got := extractSummary(in)

	if !utf8.ValidString(got) {
		t.Fatalf("extractSummary produced invalid UTF-8: % x", []byte(got))
	}
	if !strings.ContainsRune(got, '☃') {
		t.Errorf("output dropped/mangled the boundary rune ☃: %q", got)
	}
	// 200 kept runes + the trailing ellipsis = 201 runes.
	if n := utf8.RuneCountInString(got); n != 201 {
		t.Errorf("rune count = %d, want 201 (200 runes + …)", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("output missing trailing ellipsis: %q", got)
	}
	// The 200th rune (the snowman) must be the last char BEFORE the ellipsis,
	// fully intact (proves it wasn't truncated mid-sequence).
	if !strings.HasSuffix(got, "☃…") {
		t.Errorf("boundary rune not preserved intact before ellipsis: %q", got)
	}
}

// TestExtractSummary_RuneWordBoundary proves the word-boundary trim still fires
// on rune-cut input: a space past rune 170 (within the first 200 runes) trims
// there, matching the TS `lastSpace > MAX_SUMMARY_LEN - 30` branch, and the
// multi-byte rune before the cut survives intact + valid.
func TestExtractSummary_RuneWordBoundary(t *testing.T) {
	// Layout (runes): [0..179] 'a', [180] 'é' (2 bytes), [181] ' ', [182..249] 'b'.
	// The last space within the first 200 runes is at rune-index 181 (> 170), so
	// the result is trimmed to runes [0..180] (ending in 'é') + "…".
	in := strings.Repeat("a", 180) + "é" + " " + strings.Repeat("b", 68)
	got := extractSummary(in)

	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: % x", []byte(got))
	}
	if !strings.HasSuffix(got, "é…") {
		t.Errorf("want trim at the space after 'é' (…é…), got %q", got)
	}
	// 181 kept runes (indices 0..180) + the ellipsis = 182 runes; no 'b' leaks in.
	if n := utf8.RuneCountInString(got); n != 182 {
		t.Errorf("rune count = %d, want 182 (181 runes + …)", n)
	}
	if strings.ContainsRune(got, 'b') {
		t.Errorf("trailing 'b' run leaked past the word-boundary trim: %q", got)
	}
}

// isHex40 reports whether s is exactly 40 lowercase hex chars.
func isHex40(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
