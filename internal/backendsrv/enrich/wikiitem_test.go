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

// TestParseItempage_Statsblock proves the INV-02 examine stats (2026-06-18): the in-game
// stat block is surfaced as a cleaned, newline-separated string — the slot line preserved,
// all HTML (<br>/<a>) stripped — for the examine panel. (Previously parsed but discarded by
// the D-8 Sheet-parity scope guard.)
func TestParseItempage_Statsblock(t *testing.T) {
	wikitext, title := loadWikitext(t, "wiki-parse-cloth-cap")
	item, _, ok, _ := ParseItempage(wikitext, title)
	if !ok {
		t.Fatal("ParseItempage ok=false")
	}
	if item.Statsblock == "" {
		t.Fatalf("Statsblock empty, want the cleaned in-game stat block")
	}
	// The wiki statsblock leads with the slot line; cleanStatsblock preserves it.
	if !strings.Contains(item.Statsblock, "Slot: HEAD") {
		t.Errorf("Statsblock missing the slot line: %q", item.Statsblock)
	}
	// No HTML survives — it must be plain newline-separated text for the examine.
	if strings.Contains(item.Statsblock, "<") {
		t.Errorf("Statsblock leaked an HTML tag (incl. <br>): %q", item.Statsblock)
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

// TestParseIconID is the INV-04 (D-01/D-02/D-03) unit suite for the pure
// lucy_img_ID → icon-id parse. 0 is the "no icon yet" sentinel (the client falls
// back to the colored tile, D-02), returned for absent/blank/non-numeric/negative
// input. RESEARCH verified live: Cloak of Flames=658, Wurmslayer=736, Ring of the
// Ancients=563. The wiki param arrives with surrounding spaces (`lucy_img_ID = 658`),
// so the parse MUST trim before atoi.
func TestParseIconID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"present numeric", "658", 658},
		{"absent (empty fallback)", "", 0},
		{"blank", "   ", 0},
		{"non-numeric", "abc", 0},
		{"whitespace-wrapped", " 736 ", 736},
		{"negative is rejected", "-5", 0},
		{"filename-ish non-numeric", "Item_658.png", 0},
	}
	for _, c := range cases {
		if got := parseIconID(c.in); got != c.want {
			t.Errorf("parseIconID(%q) = %d, want %d (%s)", c.in, got, c.want, c.name)
		}
	}
}

// TestParseItempage_IconID asserts the lucy_img_ID param flows onto
// ParsedWikiItem.IconID. The cloak-of-flames fixture carries `lucy_img_ID = 658`
// (RESEARCH-verified); a page with no lucy_img_ID param yields IconID == 0 (the
// no-icon sentinel — the fungi-tunic redirect fixture has no {{Itempage}} so it is
// not usable here; instead a long wikitext with an {{Itempage}} but no lucy_img_ID
// proves the absent case).
func TestParseItempage_IconID(t *testing.T) {
	// Present: the cloak-of-flames fixture has lucy_img_ID = 658.
	wikitext, title := loadWikitext(t, "wiki-parse-cloak-of-flames")
	item, _, ok, _ := ParseItempage(wikitext, title)
	if !ok {
		t.Fatal("cloak-of-flames parse failed")
	}
	if item.IconID != 658 {
		t.Errorf("Cloak of Flames IconID = %d, want 658 (lucy_img_ID)", item.IconID)
	}

	// Absent: a synthetic long {{Itempage}} with NO lucy_img_ID → IconID 0.
	noIcon := "{{Itempage\n|itemname=No Icon Item\n|notes=A plain item with no icon param at all here.\n|statsblock=MAGIC ITEM<br>Slot: HEAD\n}}" +
		strings.Repeat(" padding to clear the 200-byte minimum wikitext length guard.", 5)
	item2, _, ok2, reason := ParseItempage(noIcon, "No Icon Item")
	if !ok2 {
		t.Fatalf("synthetic no-icon parse ok=false reason=%q", reason)
	}
	if item2.IconID != 0 {
		t.Errorf("no-lucy_img_ID item IconID = %d, want 0 (no-icon sentinel)", item2.IconID)
	}
}

// TestParseItempage_NewFieldsSurfaced is the Phase 37 (ENRICH-12/13) RED→GREEN
// gate for Task 1: ParseItempage must now SURFACE the flag booleans + the full
// flag set + the clicky/haste effects it already computes internally (the D-8
// scope guard previously discarded them). Driven by a synthetic inline
// {{Itempage}} so it needs no fixture (the clicky-positive fixture + the full
// table suite are Task 2). A MAGIC+LORE clicky (Click from Inventory) + Haste:
// exercises every new field in one parse.
func TestParseItempage_NewFieldsSurfaced(t *testing.T) {
	wikitext := "{{Itempage\n|itemname=Synth Clicky\n|notes=A synthetic item used to prove the Phase 37 derived fields are surfaced.\n" +
		"|statsblock=MAGIC ITEM<br>LORE ITEM<br>Slot: PRIMARY<br>Effect: [[Shock of Frost]] (Click from Inventory)<br>Haste: +21%  <br>Class: ALL<br>Race: ALL\n}}" +
		strings.Repeat(" padding to clear the 200-byte minimum wikitext length guard.", 5)
	item, _, ok, reason := ParseItempage(wikitext, "Synth Clicky")
	if !ok {
		t.Fatalf("ParseItempage ok=false reason=%q", reason)
	}
	if !item.IsMagic {
		t.Errorf("IsMagic = false, want true")
	}
	if !item.IsLore {
		t.Errorf("IsLore = false, want true")
	}
	if item.IsNoDrop {
		t.Errorf("IsNoDrop = true, want false")
	}
	if item.IsTemporary {
		t.Errorf("IsTemporary = true, want false")
	}
	if !item.IsClicky {
		t.Errorf("IsClicky = false, want true (Click from Inventory effect)")
	}
	if item.ClickyEffect != "Shock of Frost" {
		t.Errorf("ClickyEffect = %q, want Shock of Frost", item.ClickyEffect)
	}
	if !item.HasHaste {
		t.Errorf("HasHaste = false, want true")
	}
	if item.HastePct != 21 {
		t.Errorf("HastePct = %d, want 21", item.HastePct)
	}
	// Flags carries the FULL detected set (sorted) — every all-caps flag, not just
	// the four queried ones.
	wantFlags := map[string]bool{"MAGIC ITEM": true, "LORE ITEM": true}
	got := map[string]bool{}
	for _, f := range item.Flags {
		got[f] = true
	}
	for f := range wantFlags {
		if !got[f] {
			t.Errorf("Flags missing %q (got %v)", f, item.Flags)
		}
	}
}

// TestParseItempage_FlagsAndEffects drives three real fixtures as a table,
// proving ENRICH-12 (the four queried flags + the full Flags set) and ENRICH-13
// (clicky-vs-worn classification + haste %) end-to-end through ParseItempage:
//   - Cloak of Flames: MAGIC + Haste:+36%, NOT a clicky (a haste cloak is worn).
//   - Fungus tunic:    LORE + Effect (Worn) — the (Worn) effect is NOT a click.
//   - Staff of Temperate Flux: MAGIC + LORE + Effect (Click from Inventory) — the
//     clicky-positive case (ClickyEffect == the spell name, links/qualifier stripped).
func TestParseItempage_FlagsAndEffects(t *testing.T) {
	cases := []struct {
		fixture      string
		isMagic      bool
		isLore       bool
		isNoDrop     bool
		isTemporary  bool
		isClicky     bool
		clickyEffect string
		hasHaste     bool
		hastePct     int
		wantFlags    []string // a subset that MUST be present in the full set
	}{
		{
			fixture:   "wiki-parse-cloak-of-flames",
			isMagic:   true,
			isLore:    false,
			isNoDrop:  false,
			isClicky:  false, // a Haste cloak is worn, NOT an activatable clicky
			hasHaste:  true,
			hastePct:  36,
			wantFlags: []string{"MAGIC ITEM"},
		},
		{
			fixture:      "wiki-parse-fungus-covered-scale-tunic",
			isMagic:      false,
			isLore:       true,
			isClicky:     false, // the (Worn) effect is NOT a click
			clickyEffect: "",
			hasHaste:     false,
			wantFlags:    []string{"LORE ITEM"},
		},
		{
			fixture:      "wiki-parse-staff-of-temperate-flux",
			isMagic:      true,
			isLore:       true,
			isClicky:     true, // (Click from Inventory) IS a clicky
			clickyEffect: "Shock of Frost",
			hasHaste:     false,
			wantFlags:    []string{"MAGIC ITEM", "LORE ITEM"},
		},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			wikitext, title := loadWikitext(t, c.fixture)
			item, _, ok, reason := ParseItempage(wikitext, title)
			if !ok {
				t.Fatalf("ParseItempage ok=false reason=%q", reason)
			}
			if item.IsMagic != c.isMagic {
				t.Errorf("IsMagic = %v, want %v", item.IsMagic, c.isMagic)
			}
			if item.IsLore != c.isLore {
				t.Errorf("IsLore = %v, want %v", item.IsLore, c.isLore)
			}
			if item.IsNoDrop != c.isNoDrop {
				t.Errorf("IsNoDrop = %v, want %v", item.IsNoDrop, c.isNoDrop)
			}
			if item.IsTemporary != c.isTemporary {
				t.Errorf("IsTemporary = %v, want %v", item.IsTemporary, c.isTemporary)
			}
			if item.IsClicky != c.isClicky {
				t.Errorf("IsClicky = %v, want %v", item.IsClicky, c.isClicky)
			}
			if item.ClickyEffect != c.clickyEffect {
				t.Errorf("ClickyEffect = %q, want %q", item.ClickyEffect, c.clickyEffect)
			}
			if item.HasHaste != c.hasHaste {
				t.Errorf("HasHaste = %v, want %v", item.HasHaste, c.hasHaste)
			}
			if item.HastePct != c.hastePct {
				t.Errorf("HastePct = %d, want %d", item.HastePct, c.hastePct)
			}
			got := map[string]bool{}
			for _, f := range item.Flags {
				got[f] = true
			}
			for _, want := range c.wantFlags {
				if !got[want] {
					t.Errorf("Flags missing %q (got %v)", want, item.Flags)
				}
			}
		})
	}
}

// TestParseClicky is the D-01 classification unit suite: only an activatable
// (Click...) qualifier yields a clicky; (Worn) passives and (Combat) procs do
// not. The effect NAME has its [[wiki-link]] brackets stripped and the trailing
// "(...)" qualifier removed. Empty / no-qualifier input → (false, "").
func TestParseClicky(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantClick bool
		wantName  string
	}{
		{"click bare", "[[Shock of Frost]] (Click)", true, "Shock of Frost"},
		{"click from inventory", "[[Shock of Frost]] (Click from Inventory)", true, "Shock of Frost"},
		{"worn is not a click", "[[Fungal Regrowth]] (Worn)", false, ""},
		{"combat is not a click", "[[Lifetap]] (Combat)", false, ""},
		{"empty", "", false, ""},
		{"no qualifier", "[[Some Effect]]", false, ""},
		{"display-text link", "[[Spell Page|Frost Bolt]] (Click)", true, "Frost Bolt"},
	}
	for _, c := range cases {
		gotClick, gotName := parseClicky(c.in)
		if gotClick != c.wantClick || gotName != c.wantName {
			t.Errorf("parseClicky(%q) = (%v, %q), want (%v, %q) [%s]",
				c.in, gotClick, gotName, c.wantClick, c.wantName, c.name)
		}
	}
}

// TestParseHastePct is the haste-% parse unit suite: "+36%"/"21%" → the integer
// magnitude; blank/garbage → (false, 0). Defensive like parseIconID (T-37-02).
func TestParseHastePct(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantHaste bool
		wantPct   int
	}{
		{"signed percent", "+36%", true, 36},
		{"bare percent", "21%", true, 21},
		{"whitespace-wrapped", "  +10%  ", true, 10},
		{"no percent sign", "15", true, 15},
		{"empty", "", false, 0},
		{"non-numeric", "abc", false, 0},
		{"percent only", "%", false, 0},
	}
	for _, c := range cases {
		gotHaste, gotPct := parseHastePct(c.in)
		if gotHaste != c.wantHaste || gotPct != c.wantPct {
			t.Errorf("parseHastePct(%q) = (%v, %d), want (%v, %d) [%s]",
				c.in, gotHaste, gotPct, c.wantHaste, c.wantPct, c.name)
		}
	}
}

// TestDeriveFlagsAndEffects_NewlineForm proves the one-derivation contract (D-05):
// DeriveFlagsAndEffects re-parses the STORED cleaned statsblock — newline-separated
// (not <br>) and with the [[wiki-link]] brackets already stripped — and produces the
// SAME flags/effects the live parser does. This is the exact input store.BackfillItemFlags
// feeds it at boot, so a green test here is the no-network backfill's correctness proof.
func TestDeriveFlagsAndEffects_NewlineForm(t *testing.T) {
	t.Run("clicky from no-bracket cleaned Effect line", func(t *testing.T) {
		// The stored 00013 form: newline-separated, links rendered to display text.
		sb := "MAGIC ITEM\nSlot: PRIMARY\nEffect: Shock of Frost (Click from Inventory)\nClass: ALL"
		d := DeriveFlagsAndEffects(sb)
		if !d.IsMagic {
			t.Errorf("IsMagic = false, want true (MAGIC ITEM flag on the newline form)")
		}
		if !d.IsClicky {
			t.Errorf("IsClicky = false, want true (a no-bracket cleaned 'Effect: ... (Click ...)' must classify)")
		}
		if d.ClickyEffect != "Shock of Frost" {
			t.Errorf("ClickyEffect = %q, want %q (the clicky name from the bracket-stripped Effect line)", d.ClickyEffect, "Shock of Frost")
		}
		if len(d.Flags) != 1 || d.Flags[0] != "MAGIC ITEM" {
			t.Errorf("Flags = %v, want [\"MAGIC ITEM\"]", d.Flags)
		}
	})

	t.Run("haste, no effect", func(t *testing.T) {
		sb := "MAGIC ITEM\nSlot: BACK\nHaste: +36%\nClass: ALL"
		d := DeriveFlagsAndEffects(sb)
		if !d.HasHaste || d.HastePct != 36 {
			t.Errorf("HasHaste/HastePct = (%v, %d), want (true, 36)", d.HasHaste, d.HastePct)
		}
		if d.IsClicky {
			t.Errorf("IsClicky = true, want false (no Effect line ⇒ not a clicky)")
		}
	})

	t.Run("empty block ⇒ zero value, no panic", func(t *testing.T) {
		d := DeriveFlagsAndEffects("")
		if d.IsMagic || d.IsClicky || d.HasHaste || d.HastePct != 0 || d.Flags != nil {
			t.Errorf("DeriveFlagsAndEffects(\"\") = %+v, want the zero value", d)
		}
	})

	t.Run("newline form matches the <br> form", func(t *testing.T) {
		// The SAME logical block, separated by <br> vs '\n' (no brackets either way),
		// must derive identically — proves the brOrNlRe union seam is sound.
		brForm := "LORE ITEM<br>MAGIC ITEM<br>Slot: HEAD<br>Haste: +21%"
		nlForm := "LORE ITEM\nMAGIC ITEM\nSlot: HEAD\nHaste: +21%"
		db := DeriveFlagsAndEffects(brForm)
		dn := DeriveFlagsAndEffects(nlForm)
		if db.IsLore != dn.IsLore || db.IsMagic != dn.IsMagic ||
			db.HasHaste != dn.HasHaste || db.HastePct != dn.HastePct ||
			strings.Join(db.Flags, "|") != strings.Join(dn.Flags, "|") {
			t.Errorf("br form %+v != newline form %+v (the separator union must be transparent)", db, dn)
		}
		if !dn.IsLore || !dn.IsMagic || dn.HastePct != 21 {
			t.Errorf("newline form derived wrong: %+v", dn)
		}
	})

	t.Run("[[link]]-rendered flag: raw form matches cleaned form (MD-01)", func(t *testing.T) {
		// A flag the wiki renders as a link: the RAW statsblock keeps the [[ ]] brackets,
		// the stored CLEANED statsblock (cleanStatsblock) has already stripped them. Both
		// MUST detect the same flag — otherwise the live parse (raw) and the
		// backfill/freshness (cleaned) disagree on flags_json and the row re-writes on every
		// weekly pass forever (the D-06 idempotency the MarshalFlags-everywhere design exists
		// to guarantee). renderWikiLinks in the flag branch makes the two forms converge.
		rawForm := "[[No Drop]]<br>Slot: HEAD"
		cleanedForm := "No Drop\nSlot: HEAD"
		draw := DeriveFlagsAndEffects(rawForm)
		dclean := DeriveFlagsAndEffects(cleanedForm)
		if !draw.IsNoDrop {
			t.Errorf("raw [[No Drop]] form: IsNoDrop = false, want true (the bracketed flag must render+classify)")
		}
		if !dclean.IsNoDrop {
			t.Errorf("cleaned 'No Drop' form: IsNoDrop = false, want true")
		}
		if draw.IsNoDrop != dclean.IsNoDrop ||
			strings.Join(draw.Flags, "|") != strings.Join(dclean.Flags, "|") {
			t.Errorf("raw form %+v != cleaned form %+v ([[link]] flag must classify identically across both — MD-01)", draw.Flags, dclean.Flags)
		}
		if len(dclean.Flags) != 1 || dclean.Flags[0] != "NO DROP" {
			t.Errorf("Flags = %v, want [\"NO DROP\"]", dclean.Flags)
		}
	})
}

// TestDeriveFlagsAndEffects_ClusteredFlagLine is the regression for the 2026-06-26
// Phase-40 ring smoke bug: the P1999 wiki packs multiple flags onto ONE statsblock
// line separated by SINGLE spaces ("MAGIC ITEM NO DROP", "MAGIC ITEM LORE ITEM",
// "LORE ITEM NO DROP"), and parseStatsblock stores that whole line as a single
// all-caps map key. The old exact-match lookups (flags["NO DROP"]) MISSED every
// clustered line, zeroing is_no_drop/is_lore for ~95% of held flag-bearing items
// (160/168 no-drop, 330/360 lore unset in prod). hasFlag's substring containment
// must now resolve EACH flag out of the cluster. The statsblocks below are verbatim
// prod item_master rows (Cryosilk Pantaloons / Mountain Death Belt / Cloak of Spiroc
// Feathers / Savant's Cap).
func TestDeriveFlagsAndEffects_ClusteredFlagLine(t *testing.T) {
	cases := []struct {
		name                       string
		statsblock                 string
		wantMagic, wantNoDrop, wantLore bool
	}{
		{
			name:       "MAGIC ITEM NO DROP (single-space cluster) — both resolve, lore stays false",
			statsblock: "MAGIC ITEM NO DROP\nSlot: LEGS\nAC: 5\nClass: NEC WIZ MAG ENC",
			wantMagic:  true, wantNoDrop: true, wantLore: false,
		},
		{
			name:       "MAGIC ITEM LORE ITEM — magic+lore resolve, no-drop stays false",
			statsblock: "MAGIC ITEM LORE ITEM\nSlot: WAIST\nAC: 8\nClass: WAR CLR PAL ROG",
			wantMagic:  true, wantNoDrop: false, wantLore: true,
		},
		{
			name:       "LORE ITEM NO DROP — lore+no-drop resolve, magic stays false",
			statsblock: "LORE ITEM NO DROP\nSlot: BACK\nAC: 6\nClass: NEC",
			wantMagic:  false, wantNoDrop: true, wantLore: true,
		},
		{
			name:       "MAGIC ITEM (standalone) — no regression, only magic",
			statsblock: "MAGIC ITEM\nSlot: HEAD\nAC: 2\nClass: ALL",
			wantMagic:  true, wantNoDrop: false, wantLore: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := DeriveFlagsAndEffects(c.statsblock)
			if d.IsMagic != c.wantMagic {
				t.Errorf("IsMagic = %v, want %v", d.IsMagic, c.wantMagic)
			}
			if d.IsNoDrop != c.wantNoDrop {
				t.Errorf("IsNoDrop = %v, want %v", d.IsNoDrop, c.wantNoDrop)
			}
			if d.IsLore != c.wantLore {
				t.Errorf("IsLore = %v, want %v", d.IsLore, c.wantLore)
			}
		})
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
