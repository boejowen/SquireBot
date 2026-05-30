// Package jobs holds the two backend enrichment jobs (ENRICH-10/11) — the daily
// PigParse price pull and the weekly P1999 wiki scrape — that replace the Apps
// Script time-driven refresh* triggers. Each job COMPOSES the Wave-1 pieces and
// authors ZERO inline SQL, exactly like ingest/handler.go::bindAndReplace:
//
//	politefetch.Fetch  (12-03, the polite net/http client + ETag/304)
//	  → enrich.Parse*   (12-02, the pure parsers)
//	    → store.*Tx     (12-01, the single tested SQL path, composed over ONE tx)
//
// The Sheets-specific machinery the triggers carried — the 6-minute
// resumable-position self-reschedule, the 10M-cell watchdog, the expected-tab
// schema watchdog, the Apps Script document lock, the script-properties store,
// and the post-run gear/spell view rebuilds — is DELETED, not ported (D-5/D-8):
// a backend job has no execution cap (one uninterrupted run), the single-writer
// DB + a per-job mutex replace the Apps Script lock, and the views belong to P14.
package jobs

import (
	"strings"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
)

// PigparseURL is the daily price source: PigParse's getall for server=1 (Blue).
// A hardcoded constant, never user-supplied — that is the SSRF mitigation
// (T-12.04-04 accept): no caller controls the host or scheme. Mirrors
// refreshPigparse.ts PIGPARSE_URL.
const PigparseURL = "https://pigparse.azurewebsites.net/api/item/getall/1"

// WikiAPIBase is the P1999 MediaWiki action API base. Also a hardcoded constant
// (the wikiParseURL builder only ever appends an escaped page title to it, so the
// host/scheme are never attacker-controlled). Mirrors the triggers' WIKI_API_BASE.
const WikiAPIBase = "https://wiki.project1999.com/api.php"

// wikiParseURL builds the action=parse&prop=wikitext request URL for one page
// title, mirroring the TS request shape used by all three wiki triggers
// (refreshWikiItems.ts:176, refreshWikiSpells.ts:170, refreshWikiGearTier.ts:193):
//
//	`${WIKI_API_BASE}?action=parse&prop=wikitext&format=json&page=${encodeURIComponent(name.replace(/ /g, '_'))}&redirects=true`
//
// The page title has spaces replaced with underscores (matching the TS
// `.replace(/ /g, '_')`) and is then escaped with the SAME enrich.EncodeURIComponent
// the parser uses for stored wiki_url/source_url values — NOT url.QueryEscape,
// which over-escapes apostrophes and parentheses (`Lord_Nagafen's_Lair` →
// `%27`, `Cloak_of_Flames_(Quest)` → `%28..%29`) and so diverges byte-for-byte
// from the TS encodeURIComponent. There is now ONE escaper for wiki page names.
// redirects=true asks MediaWiki to resolve a redirect page (e.g. "Fungi Tunic"
// → "Fungus Covered Scale Tunic") to its canonical target — the `parse.title` in
// the response is the resolved title the parser uses for the wiki_url. The
// returned URL is ALSO the etag_cache key (one cache row per page), so matching
// the TS byte-for-byte keeps the cache key stable across the port.
func wikiParseURL(pageTitle string) string {
	escaped := enrich.EncodeURIComponent(strings.ReplaceAll(pageTitle, " ", "_"))
	return WikiAPIBase + "?action=parse&prop=wikitext&format=json&page=" + escaped + "&redirects=true"
}
