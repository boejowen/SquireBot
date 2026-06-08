package jobs

import "testing"

// TestWikiParseURL_EncodeURIComponentParity proves wikiParseURL escapes the page
// title with encodeURIComponent semantics (bare ' and () in the unreserved set),
// NOT url.QueryEscape (which would emit %27/%28/%29). This is the byte-for-byte
// match to the TS request URL built in refreshWikiItems.ts:176 /
// refreshWikiSpells.ts:170 / refreshWikiGearTier.ts:193:
//
//	`...&page=${encodeURIComponent(name.replace(/ /g, '_'))}&redirects=true`
//
// The exact-bug-class the slug-escape fix in 12-02 already corrected once.
func TestWikiParseURL_EncodeURIComponentParity(t *testing.T) {
	cases := []struct {
		name      string
		pageTitle string
		want      string
	}{
		{
			// Apostrophe stays bare (encodeURIComponent: ' is unreserved);
			// url.QueryEscape would emit %27.
			name:      "apostrophe",
			pageTitle: "Lord Nagafen's Lair",
			want:      WikiAPIBase + "?action=parse&prop=wikitext&format=json&page=Lord_Nagafen's_Lair&redirects=true",
		},
		{
			// Parens stay bare (encodeURIComponent: ( and ) are unreserved);
			// url.QueryEscape would emit %28..%29.
			name:      "parens",
			pageTitle: "Cloak of Flames (Quest)",
			want:      WikiAPIBase + "?action=parse&prop=wikitext&format=json&page=Cloak_of_Flames_(Quest)&redirects=true",
		},
		{
			// Apostrophe AND parens together (the prompt's combined case).
			name:      "apostrophe_and_parens",
			pageTitle: "Foo's Bar (Quest)",
			want:      WikiAPIBase + "?action=parse&prop=wikitext&format=json&page=Foo's_Bar_(Quest)&redirects=true",
		},
		{
			// Colon DOES percent-encode (not in the unreserved set) — matches the
			// real Velious gear page titles.
			name:      "colon_encodes",
			pageTitle: "Players:Velious Pre-Raid Gear",
			want:      WikiAPIBase + "?action=parse&prop=wikitext&format=json&page=Players%3AVelious_Pre-Raid_Gear&redirects=true",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wikiParseURL(c.pageTitle); got != c.want {
				t.Errorf("wikiParseURL(%q)\n got = %q\nwant = %q", c.pageTitle, got, c.want)
			}
		})
	}
}

// TestPigparseURL_BlueOnly pins the daily price-catalog source to PigParse's
// getall for server=1 (P99 Blue). This is a deliberate INVARIANT: SquireBot must
// pull prices from ONLY the Blue server, NEVER Green.
//
// PigParse server numbering for getall: 1 = Blue. If this constant ever changes
// (e.g. to /getall/2 or any other segment), this test FAILS on purpose — you have
// changed WHICH P99 server SquireBot queries. Update this test ONLY if that
// widening/switch was intentional.
func TestPigparseURL_BlueOnly(t *testing.T) {
	const wantBlue = "https://pigparse.azurewebsites.net/api/item/getall/1"
	if PigparseURL != wantBlue {
		t.Fatalf("PigparseURL = %q, want the Blue (server=1) catalog endpoint %q.\n"+
			"SquireBot must pull prices from ONLY P99 Blue (getall server 1=Blue). A change here "+
			"changes which P99 server is queried — update this test ONLY if that was intended.",
			PigparseURL, wantBlue)
	}
}
