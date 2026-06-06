package ec

// ec.go is the EC-tunnel auction monitor's producer body (Phase 21, WANT-05):
// RunMatch composes poll → diff → match → embed → send. It is the scheduler
// ec_auction_match job's Run target. The flow, per wanted item:
//
//	ECPollSet → getdetails/0/{name} → ParseItemDetail → per-item cursor diff
//	  (advance-only-on-success; first-sight baseline = record-but-don't-DM)
//	  → for each NEW WTS auction (t>cursor AND u∈{0,2}): wantmatch.ForItem(item_id)
//	  → buildEmbed → notify.Send(Source:"ec_auction", WantID, Embed)
//
// It re-implements NONE of the spine: both gates, dedup, cooldownEC=22h, and the
// alert_log audit are inherited by routing every send through notify.Send. It
// NEVER calls the discordgo embed-send method directly — notify.Send owns the
// single Discord seam.
//
// Best-effort posture (D-07): a per-item fetch/parse/match failure logs + continues
// — one bad item never aborts the whole poll loop. A failing item's cursor does NOT
// advance (advance-only-on-success), so its next poll retries the same window.
//
// SECURITY (V7): every log line carries source + item_id + want id + status ONLY —
// never the DM body, the embed content, raw item names beyond the id, or the
// players map.

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/notify"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/wantmatch"
)

// source is the alert source key for the EC monitor — it selects cooldownEC=22h
// (D-10) and the monitor_flag.ec / notify_prefs.ec gates inside notify.Send.
const source = "ec_auction"

// RunMatch is the EC auction producer (the ec_auction_match job body). It derives
// the poll set, polls PigParse getdetails per wanted item, diffs new WTS auctions
// against the per-item cursor, matches via wantmatch.ForItem, and DMs a rich embed
// through notify.Send. sender is the live *discordgo.Session (nil when the bot is
// disabled — a clean no-op); fetch is injected (politefetch.Fetch in production, a
// fake in tests). It returns a non-nil error only on an unexpected setup failure
// (a poll-set read error); per-item failures are logged and survived (D-07).
func RunMatch(ctx context.Context, db *sql.DB, sender notify.Sender, fetch politefetch.Fetcher) error {
	// nil session ⇒ the bot is disabled (no token). Clean no-op — nothing to send.
	// Guard the typed-nil interface too (a nil *discordgo.Session boxed into the
	// interface): notify.Send would deref it, so treat it as disabled.
	if sender == nil || isNilSender(sender) {
		slog.Info("ec_auction_match: no bot session, skipping", "source", source)
		return nil
	}

	s := store.NewStore(db)
	poll, err := s.ECPollSet(ctx)
	if err != nil {
		return err // a poll-set read failure is a real setup error (the caller logs + advances the job cursor)
	}
	if len(poll) == 0 {
		slog.Info("ec_auction_match: empty poll set", "source", source)
		return nil
	}

	for _, item := range poll {
		// Best-effort per item: a single item's failure logs + continues (D-07).
		pollItem(ctx, db, s, sender, fetch, item)
	}
	return nil
}

// pollItem polls one wanted item: fetch → parse → diff → match → send, advancing
// the per-item cursor only on a successful poll. All failure paths log (ids/status
// only, V7) and return WITHOUT advancing the cursor (so the next poll retries).
func pollItem(
	ctx context.Context,
	db *sql.DB,
	s *store.Store,
	sender notify.Sender,
	fetch politefetch.Fetcher,
	item store.ECPollItem,
) {
	cursor, ok, err := s.GetECCursor(ctx, item.ItemID)
	if err != nil {
		slog.Warn("ec_auction_match: cursor read failed", "source", source, "item_id", item.ItemID, "status", "error")
		return
	}

	res := fetch(ctx, getDetailsURL(item.ItemName), politefetch.Options{})
	if !res.OK {
		// Advance-only-on-success: do NOT move the cursor on a fetch failure.
		slog.Warn("ec_auction_match: fetch failed", "source", source, "item_id", item.ItemID, "http_status", res.Status, "status", "fetch_failed")
		return
	}
	if res.FromCache {
		// 304 — nothing changed; skip parse + leave the cursor where it is.
		slog.Info("ec_auction_match: unchanged (304)", "source", source, "item_id", item.ItemID, "status", "skipped_unchanged")
		return
	}

	detail, err := enrich.ParseItemDetail(res.Body)
	if err != nil {
		slog.Warn("ec_auction_match: parse failed", "source", source, "item_id", item.ItemID, "status", "parse_failed")
		return
	}
	if len(detail.Items) == 0 {
		// Nothing to advance to (the API returns items:null for an unseen item).
		slog.Info("ec_auction_match: no auctions", "source", source, "item_id", item.ItemID, "status", "empty")
		return
	}

	maxT := maxTimestamp(detail.Items)

	// FIRST-SIGHT BASELINE (ROADMAP criterion 4 / Pitfall 5): a never-cursored item
	// records max(t) WITHOUT DMing any of its history — so a standing auction is
	// never carpet-bombed and a restart never replays a backlog.
	if !ok {
		if serr := s.SetECCursor(ctx, item.ItemID, maxT, time.Now().Unix()); serr != nil {
			slog.Warn("ec_auction_match: baseline cursor write failed", "source", source, "item_id", item.ItemID, "status", "error")
			return
		}
		slog.Info("ec_auction_match: first-sight baseline", "source", source, "item_id", item.ItemID, "status", "baselined")
		return
	}

	// Diff: for each NEW WTS auction (t>cursor AND u∈{0,2}; WTB u==1 NEVER alerts —
	// D-02), fan out to every wantlister via wantmatch.ForItem and DM through the
	// spine. The cursor still advances past WTB-only records (they were SEEN, just
	// not alertable), so they aren't re-diffed next poll.
	now := time.Now()
	for _, a := range detail.Items {
		if a.T <= cursor {
			continue // already past the cursor — seen before
		}
		if a.U != 0 && a.U != 2 {
			continue // WTB-only (u==1) — never alerts (D-02)
		}
		hits, err := wantmatch.ForItem(ctx, db, item.ItemID)
		if err != nil {
			slog.Warn("ec_auction_match: wantmatch failed", "source", source, "item_id", item.ItemID, "status", "match_error")
			continue
		}
		for _, hit := range hits {
			sendHit(ctx, db, sender, item.ItemID, hit, a, detail.Players, now)
		}
	}

	// Advance the cursor ONLY after a successful poll (D-07; advance-only-on-success).
	// max(t) advances even past WTB-only records (seen, not alerted) so they aren't
	// re-processed; only WTS records ever DM.
	if serr := s.SetECCursor(ctx, item.ItemID, maxT, time.Now().Unix()); serr != nil {
		slog.Warn("ec_auction_match: cursor advance failed", "source", source, "item_id", item.ItemID, "status", "error")
	}
}

// sendHit builds the embed for one wantlister and routes it through notify.Send.
// The expected gate/dedup sentinels (ErrCooledDown / ErrGatedOff / ErrDMBlocked)
// are SWALLOWED at debug — they are normal flow, not a poll failure. The auction
// was still processed; the cursor advance in pollItem is independent of any single
// send's outcome.
func sendHit(
	ctx context.Context,
	db *sql.DB,
	sender notify.Sender,
	itemID int64,
	hit wantmatch.Hit,
	a enrich.ItemAuctionDetail,
	players map[string]string,
	now time.Time,
) {
	seenStr := seenAgo(a.T, now)
	seller := resolveSeller(a, players)
	embed := buildEmbed(hit, a, seller, seenStr)
	detailStr := buildDetail(a, seenStr)

	wantID := hit.WantID
	alert := notify.Alert{
		WantID:        &wantID,
		DiscordUserID: hit.DiscordUserID,
		Source:        source,
		ItemID:        &itemID,
		Detail:        &detailStr,
		Embed:         embed,
	}

	err := notify.Send(ctx, sender, db, alert, now.Unix())
	switch {
	case err == nil:
		// sent — notify.Send already logged it (V7).
	case errors.Is(err, notify.ErrCooledDown),
		errors.Is(err, notify.ErrGatedOff),
		errors.Is(err, notify.ErrDMBlocked):
		// All EXPECTED — cooldown/gate/blocked are normal flow, not a failure.
		slog.Debug("ec_auction_match: send suppressed", "source", source, "item_id", itemID, "want", hit.WantID, "status", "suppressed")
	default:
		// A real send error (DB/transport). Log + continue the loop (best-effort).
		slog.Warn("ec_auction_match: send failed", "source", source, "item_id", itemID, "want", hit.WantID, "status", "send_error")
	}
}

// maxTimestamp returns the lexically-greatest t over the auctions. Lexical compare
// is a sound monotonic cursor here because the getdetails t is fixed-shape with a
// constant +00:00 offset (21-SPIKE.md) — no time.Parse needed for the diff.
func maxTimestamp(items []enrich.ItemAuctionDetail) string {
	maxT := ""
	for _, a := range items {
		if a.T > maxT {
			maxT = a.T
		}
	}
	return maxT
}

// isNilSender reports whether sender is a typed-nil *discordgo.Session boxed into
// the notify.Sender interface — the classic Go nil-interface gotcha. main.go
// declares `var botSession *discordgo.Session` and threads it through
// scheduler.Start → the EC job closure → RunMatch's notify.Sender param; when the
// bot is disabled that pointer is nil but the interface value is NON-nil (it
// carries the *discordgo.Session type), so `sender == nil` is false. Guarding the
// typed-nil here keeps RunMatch a clean no-op rather than panicking inside
// notify.Send when it dereferences the session. A test fake (a non-nil concrete
// type) is never a nil session, so it passes through.
func isNilSender(sender notify.Sender) bool {
	sess, ok := sender.(*discordgo.Session)
	return ok && sess == nil
}
