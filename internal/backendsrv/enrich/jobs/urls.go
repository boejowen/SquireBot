// Package jobs holds the two backend enrichment jobs (ENRICH-10/11) — the daily
// PigParse price pull and the weekly P1999 wiki scrape — that replace the Apps
// Script time-driven refresh* triggers. Each job COMPOSES the Wave-1 pieces and
// authors ZERO inline SQL, exactly like ingest/handler.go::bindAndReplace:
//
//	politefetch.Fetch  (12-03, the polite net/http client + ETag/304)
//	  → enrich.Parse*   (12-02, the pure parsers)
//	    → store.*Tx     (12-01, the single tested SQL path, composed over ONE tx)
//
// The Sheets-specific machinery the triggers carried — the 6-minute resumable
// cursor (refreshWikiItems's CURSOR_KEY self-reschedule), monitorCellCount,
// weeklySchemaHealthcheck, LockService, PropertiesService, and the post-run
// buildSpellCheck/buildGearCheck VIEW rebuilds — is DELETED, not ported (D-5/D-8):
// a backend job has no execution cap (one uninterrupted run), the single-writer
// DB + a per-job mutex replace LockService, and the views belong to P14.
package jobs

import (
	"net/url"
	"strings"
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
// title, mirroring the TS request shape used by all three wiki triggers:
//
//	WIKI_API_BASE?action=parse&prop=wikitext&format=json&page=<title>&redirects=...
//
// The page title has spaces replaced with underscores (matching the TS
// `.replace(/ /g, '_')`) and is then query-escaped, so a title like
// "Players:Velious Pre-Raid Gear" becomes "Players%3AVelious_Pre-Raid_Gear"
// (byte-faithful to the TS encodeURIComponent for the colon/underscore cases the
// real titles use). redirects=1 asks MediaWiki to resolve a redirect page (e.g.
// "Fungi Tunic" → "Fungus Covered Scale Tunic") to its canonical target — the
// `parse.title` in the response is the resolved title the parser uses for the
// wiki_url. The returned URL is ALSO the etag_cache key (one cache row per page).
func wikiParseURL(pageTitle string) string {
	escaped := url.QueryEscape(strings.ReplaceAll(pageTitle, " ", "_"))
	return WikiAPIBase + "?action=parse&prop=wikitext&format=json&redirects=1&page=" + escaped
}
