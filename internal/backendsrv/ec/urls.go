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
// server segment is 0 (the LIVE Blue tunnel — the spike pinned this: server=1 is
// ~11h stale; see 21-SPIKE.md). The NAME form is the ONLY working lookup key (the
// bare-id form 400s; an id in the name slot returns empty).
const getDetailsBase = "https://pigparse.azurewebsites.net/api/item/getdetails/0/"

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
