// Package ec is the SquireBot backend's EC-tunnel auction monitor (Phase 21,
// WANT-05) — the first real end-to-end alert. It is a THIN PRODUCER that composes
// the Plan-01 data layer (the getdetails parser + the per-item ec_auction_cursor +
// the poll set) and the Plan-02 spine extension (the notify embed send-path +
// wantmatch.Hit.Note) into a single poll → diff → match → embed → send flow.
//
// It re-implements NOTHING of the spine: no DM send, no dedup, no cooldown, no
// gate logic. Every send routes through notify.Send with Source:"ec_auction" + a
// non-nil WantID, so it inherits both gates (officer monitor_flag + user
// notify_prefs), the RecentAlertExists dedup, the cooldownEC=22h window, and the
// alert_log audit. It NEVER calls the discordgo embed-send method directly —
// notify.Send owns the single Discord seam.
//
// The package lives OUTSIDE enrich/jobs to avoid an import cycle: ec composes
// enrich + store + wantmatch + notify, but the enrich parsers must NOT depend on
// notify/wantmatch (import-direction). The pure getdetails parser (pigdetails.go)
// stays in enrich; the orchestration lives here.
package ec

import (
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
)

// getDetailsBase is the per-auction getdetails feed base — a hardcoded
// host/scheme constant (T-21-08 SSRF mitigation: no caller controls the host or
// scheme; only the catalog-sourced item NAME is interpolated, escaped). The
// server segment is 1 — the LIVE P99 Blue tunnel (matches the catalog convention
// getall/1 = Blue). The NAME form is the ONLY working lookup key (the bare-id form
// 400s; an id in the name slot returns empty).
//
// CORRECTED 2026-06-09: the Phase 21 spike's critical finding "server=0 = LIVE
// Blue" was DISPROVEN — server=0 is GREEN. A guildie who plays Blue got a real
// false-ping for a Green seller, and a live probe showed server=1 is the fresher
// live Blue feed (e.g. Cloak of Flames server=1 max_t 2026-06-08T23:02 > server=0).
// server=0 merely LOOKED "fresher" in the spike because Green has more parsers. See
// 21-SPIKE.md correction. The getdetails records carry no per-auction server field,
// so the URL server segment is the only lever for which P99 server we poll.
const getDetailsBase = "https://pigparse.azurewebsites.net/api/item/getdetails/1/"

// getDetailsURL builds the getdetails poll URL for one wanted item, keyed on its
// catalog NAME (the only working query key — 21-SPIKE.md). The name segment is
// escaped with enrich.EncodeURIComponent (the project's single wiki/PigParse
// escaper — NOT url.QueryEscape, which over-escapes apostrophes/parentheses and
// diverges from the JS encodeURIComponent byte-for-byte). An item name with
// special characters cannot break the URL or traverse the path: the host/scheme
// are a constant and the name is fully percent-encoded.
func getDetailsURL(itemName string) string {
	return getDetailsBase + enrich.EncodeURIComponent(itemName)
}
