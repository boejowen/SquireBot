package jobs

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// gearETagFetcher is a stateful fake Fetcher that reproduces the H-01 staleness
// trap: it models a MediaWiki that returns 304 (FromCache) for any request whose
// If-None-Match (opts.ETag) equals the ETag it last handed out for that page. The
// OLD gear pass sent per-page ETags, so on a second run a single changed page +
// an unchanged (304'd) page meant the full-replace never fired and the change was
// permanently dropped.
//
// The FIX fetches both gear pages UNCONDITIONALLY (empty Options{}), matching the
// TS gear trigger (refreshWikiGearTier.ts:193-194). So for the two gear pages
// this fetcher must observe opts.ETag == "" and must therefore NEVER 304 them —
// the changed Pre-Raid body is always re-fetched and its row lands.
type gearETagFetcher struct {
	t *testing.T

	mu sync.Mutex
	// preRaidBody is the wikitext-envelope JSON served for the Pre-Raid page; it
	// can be swapped to the "changed" body between runs.
	preRaidBody []byte
	raidingBody []byte
	// lastETag is the last ETag handed out per page title (the wiki's view of
	// what the client currently has cached).
	lastETag map[string]string
	// gearETagSeen records whether ANY conditional (If-None-Match) request ever
	// arrived for a gear page — it must stay false under the fix.
	gearETagSeen bool
}

func newGearETagFetcher(t *testing.T) *gearETagFetcher {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile("../testdata/" + name + ".json")
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		return b
	}
	return &gearETagFetcher{
		t:           t,
		preRaidBody: read("wiki-velious-preraid-gear"),
		raidingBody: read("wiki-velious-raiding-gear"),
		lastETag:    map[string]string{},
	}
}

// page extracts the `page=` query value (percent-decoded) from a request URL.
func (f *gearETagFetcher) page(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		f.t.Fatalf("parse url %q: %v", raw, err)
	}
	return u.Query().Get("page")
}

func (f *gearETagFetcher) Fetch(_ context.Context, raw string, opts politefetch.Options) politefetch.FetchResult {
	f.mu.Lock()
	defer f.mu.Unlock()

	page := f.page(raw)
	isGear := strings.HasPrefix(page, "Players:Velious")

	if isGear && opts.ETag != "" {
		// The gear pass sent a conditional header — this is exactly the regression
		// we are guarding against (the fix must fetch gear unconditionally).
		f.gearETagSeen = true
		if opts.ETag == f.lastETag[page] {
			// Wiki says "unchanged" → 304. Under the OLD code this 304'd the
			// unchanged Raiding page and the replace was skipped forever.
			return politefetch.FetchResult{OK: true, FromCache: true, Status: 304, ETag: opts.ETag}
		}
	}

	var body []byte
	switch {
	case strings.Contains(page, "Pre-Raid"):
		body = f.preRaidBody
	case strings.Contains(page, "Raiding"):
		body = f.raidingBody
	default:
		// Non-gear pages (items/spells) are irrelevant to this test — return a
		// MediaWiki-style error envelope so they are skipped without affecting
		// the gear assertions.
		return politefetch.FetchResult{
			OK:     true,
			Status: 200,
			Body:   []byte(`{"error":{"code":"missingtitle","info":"n/a"}}`),
			ETag:   "",
		}
	}

	// Issue a fresh ETag derived from the body length so a changed body yields a
	// different ETag (the wiki's normal behavior).
	etag := page + "-" + strconv.Itoa(len(body))
	f.lastETag[page] = etag
	return politefetch.FetchResult{OK: true, Status: 200, Body: body, ETag: etag}
}

// changedPreRaidBody returns a valid action=parse envelope whose Pre-Raid
// wikitext carries a sentinel Warrior item that does NOT exist in the real
// fixtures, so its presence in wiki_gear_tier proves the changed page's content
// landed.
func changedPreRaidBody(sentinel string) []byte {
	// Minimal but valid (>= 200 chars) gear page: one recognized class header
	// (Warrior) on ITS OWN LINE (classHeaderRe anchors ^==...==$ per-line) + one
	// slot + the sentinel item transclusion.
	wikitext := "Velious pre-raid gear changed-page regression fixture. " +
		"This body is intentionally long enough to clear the minWikitextLength " +
		"guard so the parser does not early-return and finds the class section.\n\n" +
		"== [[Warrior]] ==\n\n" +
		"<ul><li>  '''Head'''  - {{:" + sentinel + "}}</li></ul>\n"
	// JSON-encode the wikitext into the envelope (escape quotes/newlines minimally
	// via the standard envelope shape the decoder reads: parse.wikitext["*"]).
	esc := strings.ReplaceAll(wikitext, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	esc = strings.ReplaceAll(esc, "\n", `\n`)
	return []byte(`{"parse":{"title":"Players:Velious Pre-Raid Gear","wikitext":{"*":"` + esc + `"}}}`)
}

// TestRunWiki_GearSinglePageChangeLands is the H-01 regression: when ONE gear
// page changes and the OTHER is unchanged (and would 304 a conditional request),
// the full-replace must still fire and the changed page's new row must land. The
// OLD per-page-ETag gear pass dropped it permanently; the fix fetches both gear
// pages unconditionally (matching refreshWikiGearTier.ts) so the change always
// lands.
func TestRunWiki_GearSinglePageChangeLands(t *testing.T) {
	restore := setWikiSleepNoop()
	defer restore()

	db := store.NewTestDB(t)
	ctx := context.Background()
	// No inventory refs seeded ⇒ the items pass is a no-op; spells pages 404 in
	// the fake fetcher and are skipped. This isolates the gear pass.

	f := newGearETagFetcher(t)

	// Run #1: both gear pages serve the real fixtures → table populated, and the
	// fake wiki records the ETags it issued for both pages.
	if err := RunWiki(ctx, db, f.Fetch); err != nil {
		t.Fatalf("RunWiki #1: %v", err)
	}
	firstRows := countRows(t, db, "wiki_gear_tier")
	if firstRows == 0 {
		t.Fatalf("wiki_gear_tier empty after run #1")
	}

	// Now the Pre-Raid page changes (a sentinel Warrior helm appears); the
	// Raiding page is unchanged (and, under a conditional fetch, would 304 on its
	// cached ETag from run #1 — the trap).
	const sentinel = "RegressionSentinel Helm Of Velious"
	f.mu.Lock()
	f.preRaidBody = changedPreRaidBody(sentinel)
	f.mu.Unlock()

	// Run #2: the fix must re-fetch BOTH pages unconditionally and full-replace,
	// so the sentinel row lands.
	if err := RunWiki(ctx, db, f.Fetch); err != nil {
		t.Fatalf("RunWiki #2: %v", err)
	}

	// The changed page's sentinel row must now be present.
	var sentinelCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM wiki_gear_tier WHERE item_name = ?`, sentinel,
	).Scan(&sentinelCount); err != nil {
		t.Fatalf("count sentinel rows: %v", err)
	}
	if sentinelCount == 0 {
		t.Errorf("sentinel gear row %q absent after run #2 — single-page change was dropped (H-01 staleness trap)", sentinel)
	}

	// And the gear pass must NEVER have sent a conditional (If-None-Match) header
	// for a gear page — that is the root cause the fix removed.
	f.mu.Lock()
	gearETagSeen := f.gearETagSeen
	f.mu.Unlock()
	if gearETagSeen {
		t.Errorf("gear pass sent an If-None-Match header — it must fetch gear unconditionally (refreshWikiGearTier.ts:193-194)")
	}
}
