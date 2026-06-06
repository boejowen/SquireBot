package ec

// ec_test.go covers RunMatch (the EC producer) and buildEmbed over the shared
// NewTestDB fixture (00008 creates ec_auction_cursor; 00006/00007 the wantlist +
// notify spine; monitor_flag.ec_auction defaults ON, absent notify_prefs reads
// all-ON — so a seeded want DMs without extra setup).
//
// A fake Fetcher returns canned getdetails bodies (incl. a fetch-failure case); a
// fake Sender (the dm_test.go precedent) records whether the embed branch fired.
// Coverage: new-WTS send, WTB ignored (but cursor advances), BOTH alerts,
// first-sight baseline (no replay), advance-only-on-success, nil-session no-op,
// Send-sentinel tolerance, and buildEmbed field omission.

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
	"github.com/boejowen/SquireBot/internal/backendsrv/wantmatch"
)

// mkAuction builds an enrich.ItemAuctionDetail directly (for the buildEmbed tests,
// which work on the parsed struct, not the JSON body).
func mkAuction(u int, t string, p *int) enrich.ItemAuctionDetail {
	return enrich.ItemAuctionDetail{U: u, I: 100 + u, T: t, P: p}
}

// --- fakes -------------------------------------------------------------------

// fakeSender is the injectable Discord seam (mirrors notify/dm_test.go's fake). It
// counts embed sends separately from plain sends so a test asserts WHICH branch
// fired; sendErr forces a failure on the send step.
type fakeSender struct {
	creates int
	sends   int
	embeds  int
	sendErr error
}

func (f *fakeSender) UserChannelCreate(userID string, _ ...discordgo.RequestOption) (*discordgo.Channel, error) {
	f.creates++
	return &discordgo.Channel{ID: "dm-" + userID}, nil
}

func (f *fakeSender) ChannelMessageSend(_, _ string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.sends++
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &discordgo.Message{ID: "m"}, nil
}

func (f *fakeSender) ChannelMessageSendEmbed(_ string, _ *discordgo.MessageEmbed, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.embeds++
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &discordgo.Message{ID: "e"}, nil
}

// fetchFunc adapts a closure to the politefetch.Fetcher seam.
type fetchFunc func(ctx context.Context, url string, opts politefetch.Options) politefetch.FetchResult

func (f fetchFunc) toFetcher() politefetch.Fetcher {
	return politefetch.Fetcher(f)
}

// okBody builds a 200 FetchResult whose body is the JSON-encoded getdetails detail.
func okBody(items []map[string]any) politefetch.FetchResult {
	body, _ := json.Marshal(map[string]any{
		"items":    items,
		"itemName": "Fungus Covered Scale Tunic",
		"players":  map[string]string{},
	})
	return politefetch.FetchResult{OK: true, Status: 200, Body: body}
}

func auction(u int, t string, p *int) map[string]any {
	m := map[string]any{"u": u, "i": 100 + u, "t": t}
	if p != nil {
		m["p"] = *p
	} else {
		m["p"] = nil
	}
	return m
}

func intp(v int) *int { return &v }

// --- seed helpers ------------------------------------------------------------

func seedUser(t *testing.T, ctx context.Context, db *sql.DB, discordID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, discordID, "user-"+discordID); err != nil {
		t.Fatalf("seed web_user %q: %v", discordID, err)
	}
}

// seedWant inserts an active catalog want for itemID and returns its id.
func seedWant(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, active, muted, created_at)
		 VALUES (?, ?, 'Fungus Covered Scale Tunic', 'buy', 'med', 1, 0, 1)`, discordID, itemID)
	if err != nil {
		t.Fatalf("seed want: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed want id: %v", err)
	}
	return id
}

func cursorOf(t *testing.T, db *sql.DB, itemID int64) (string, bool) {
	t.Helper()
	got, ok, err := store.NewStore(db).GetECCursor(context.Background(), itemID)
	if err != nil {
		t.Fatalf("GetECCursor: %v", err)
	}
	return got, ok
}

func countAlerts(t *testing.T, db *sql.DB, discordID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM alert_log WHERE discord_user_id = ?`, discordID).Scan(&n); err != nil {
		t.Fatalf("count alerts: %v", err)
	}
	return n
}

const (
	itemID  = int64(16247)
	tBase   = "2026-06-06T01:00:00+00:00"
	tNewer  = "2026-06-06T02:00:00+00:00"
	tNewest = "2026-06-06T03:00:00+00:00"
)

// --- RunMatch tests ----------------------------------------------------------

// TestRunMatch_NewWTS_Sends: a record with t>cursor, u=0 (WTS) for a wanted item
// → an embed DM via the spine; the cursor advances to that t.
func TestRunMatch_NewWTS_Sends(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedWant(t, ctx, db, "alice", itemID)
	// Baseline the cursor so the new auction is a DIFF, not a first-sight.
	if err := store.NewStore(db).SetECCursor(ctx, itemID, tBase, 1); err != nil {
		t.Fatalf("baseline cursor: %v", err)
	}

	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		return okBody([]map[string]any{auction(0, tNewer, intp(2000))})
	}).toFetcher()
	fs := &fakeSender{}

	if err := RunMatch(ctx, db, fs, fetch); err != nil {
		t.Fatalf("RunMatch: %v", err)
	}
	if fs.embeds != 1 {
		t.Errorf("embeds = %d; want 1 (a new WTS auction DMs a rich embed)", fs.embeds)
	}
	if fs.sends != 0 {
		t.Errorf("plain sends = %d; want 0 (EC always uses the embed path)", fs.sends)
	}
	if got, _ := cursorOf(t, db, itemID); got != tNewer {
		t.Errorf("cursor = %q; want %q (advanced to the seen max(t))", got, tNewer)
	}
}

// TestRunMatch_WTBIgnored_CursorStillAdvances: a u=1 (WTB-only) record with
// t>cursor produces NO send (D-02) but the cursor STILL advances (the auction was
// seen, just not alertable) so it isn't re-diffed next poll.
func TestRunMatch_WTBIgnored_CursorStillAdvances(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedWant(t, ctx, db, "alice", itemID)
	if err := store.NewStore(db).SetECCursor(ctx, itemID, tBase, 1); err != nil {
		t.Fatalf("baseline cursor: %v", err)
	}

	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		return okBody([]map[string]any{auction(1, tNewer, intp(2000))}) // u=1 WTB
	}).toFetcher()
	fs := &fakeSender{}

	if err := RunMatch(ctx, db, fs, fetch); err != nil {
		t.Fatalf("RunMatch: %v", err)
	}
	if fs.embeds != 0 || fs.sends != 0 {
		t.Errorf("WTB-only alerted (embeds=%d sends=%d); want 0/0 (D-02)", fs.embeds, fs.sends)
	}
	if got, _ := cursorOf(t, db, itemID); got != tNewer {
		t.Errorf("cursor = %q; want %q (a seen WTB still advances the cursor)", got, tNewer)
	}
}

// TestRunMatch_BOTHAlerts: u=2 (BOTH) is treated as WTS and DOES send (D-02:
// u∈{0,2}).
func TestRunMatch_BOTHAlerts(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedWant(t, ctx, db, "alice", itemID)
	if err := store.NewStore(db).SetECCursor(ctx, itemID, tBase, 1); err != nil {
		t.Fatalf("baseline cursor: %v", err)
	}

	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		return okBody([]map[string]any{auction(2, tNewer, intp(1500))}) // u=2 BOTH
	}).toFetcher()
	fs := &fakeSender{}

	if err := RunMatch(ctx, db, fs, fetch); err != nil {
		t.Fatalf("RunMatch: %v", err)
	}
	if fs.embeds != 1 {
		t.Errorf("embeds = %d; want 1 (u=2 BOTH alerts as WTS — D-02)", fs.embeds)
	}
}

// TestRunMatch_FirstSightBaseline_NoReplay: a never-cursored item with several
// historical auctions sends ZERO DMs and sets the cursor to max(t) (ROADMAP
// criterion 4 — no history replay on first run / restart).
func TestRunMatch_FirstSightBaseline_NoReplay(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedWant(t, ctx, db, "alice", itemID)
	// NO cursor seeded ⇒ first-sight.

	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		return okBody([]map[string]any{
			auction(0, tBase, intp(2000)),
			auction(0, tNewer, intp(1900)),
			auction(0, tNewest, intp(1800)),
		})
	}).toFetcher()
	fs := &fakeSender{}

	if err := RunMatch(ctx, db, fs, fetch); err != nil {
		t.Fatalf("RunMatch: %v", err)
	}
	if fs.embeds != 0 || fs.sends != 0 {
		t.Errorf("first-sight DMed history (embeds=%d sends=%d); want 0/0 (no replay)", fs.embeds, fs.sends)
	}
	got, ok := cursorOf(t, db, itemID)
	if !ok || got != tNewest {
		t.Errorf("first-sight cursor = (%q, ok=%v); want (%q, true) (baselined to max(t))", got, ok, tNewest)
	}
	if n := countAlerts(t, db, "alice"); n != 0 {
		t.Errorf("first-sight wrote %d alert_log rows; want 0", n)
	}
}

// TestRunMatch_AdvanceOnlyOnSuccess: a fetch failure for the FIRST item does NOT
// advance its cursor and does NOT abort the loop — the SECOND item still polls and
// DMs (D-07 best-effort).
func TestRunMatch_AdvanceOnlyOnSuccess(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	const failItem = int64(2001)
	const okItem = int64(2002)
	seedWant(t, ctx, db, "alice", failItem)
	seedWant(t, ctx, db, "alice", okItem)
	s := store.NewStore(db)
	if err := s.SetECCursor(ctx, failItem, tBase, 1); err != nil {
		t.Fatalf("baseline failItem cursor: %v", err)
	}
	if err := s.SetECCursor(ctx, okItem, tBase, 1); err != nil {
		t.Fatalf("baseline okItem cursor: %v", err)
	}

	// The fetcher fails for failItem (its escaped name segment is identical here —
	// both wants share the seed name — so distinguish by a per-call counter: the
	// first call fails, the second succeeds). DISTINCT poll order is not guaranteed,
	// so fail on the FIRST fetch and succeed on the rest, then assert at least one
	// embed fired and the okItem cursor moved while a failed item's did not.
	var calls int
	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		calls++
		if calls == 1 {
			return politefetch.FetchResult{OK: false, Status: 503} // fetch failure
		}
		return okBody([]map[string]any{auction(0, tNewer, intp(1234))})
	}).toFetcher()
	fs := &fakeSender{}

	if err := RunMatch(ctx, db, fs, fetch); err != nil {
		t.Fatalf("RunMatch: %v", err)
	}
	if calls != 2 {
		t.Errorf("fetch called %d times; want 2 (loop survived the first item's failure)", calls)
	}
	if fs.embeds != 1 {
		t.Errorf("embeds = %d; want 1 (the second item still polled + alerted after the first failed)", fs.embeds)
	}
	// Exactly one of the two cursors stayed at the baseline (the failed item); the
	// other advanced (the succeeded item). Assert one each.
	c1, _ := cursorOf(t, db, failItem)
	c2, _ := cursorOf(t, db, okItem)
	advanced, stayed := 0, 0
	for _, c := range []string{c1, c2} {
		switch c {
		case tNewer:
			advanced++
		case tBase:
			stayed++
		}
	}
	if advanced != 1 || stayed != 1 {
		t.Errorf("cursors failItem=%q okItem=%q; want exactly one advanced (%q) and one stayed (%q)", c1, c2, tNewer, tBase)
	}
}

// TestRunMatch_NilSession_NoOp: RunMatch with a nil Sender returns nil without
// panicking and sends nothing (bot disabled — no token).
func TestRunMatch_NilSession_NoOp(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	seedWant(t, ctx, db, "alice", itemID)

	// A typed-nil *discordgo.Session boxed into the interface — the nil-interface
	// gotcha main.go can hand us. RunMatch must treat it as disabled, not deref it.
	var sess *discordgo.Session
	fetched := false
	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		fetched = true
		return okBody(nil)
	}).toFetcher()

	if err := RunMatch(ctx, db, sess, fetch); err != nil {
		t.Fatalf("RunMatch(nil session): %v", err)
	}
	if fetched {
		t.Error("RunMatch polled with a nil session; want a clean no-op before any fetch")
	}
}

// TestRunMatch_SendSentinelTolerated: a cooled-down send (ErrCooledDown from the
// spine) is swallowed and the cursor STILL advances (the auction was processed).
func TestRunMatch_SendSentinelTolerated(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", itemID)
	if err := store.NewStore(db).SetECCursor(ctx, itemID, tBase, 1); err != nil {
		t.Fatalf("baseline cursor: %v", err)
	}
	// Pre-seed a recent 'sent' alert_log row inside the EC window so the dedup probe
	// short-circuits the next send (ErrCooledDown).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, item_id, sent_at, send_status)
		 VALUES (?, 'alice', 'ec_auction', ?, ?, 'sent')`, want, itemID, time.Now().Unix()); err != nil {
		t.Fatalf("seed recent sent: %v", err)
	}

	fetch := fetchFunc(func(_ context.Context, _ string, _ politefetch.Options) politefetch.FetchResult {
		return okBody([]map[string]any{auction(0, tNewer, intp(2000))})
	}).toFetcher()
	fs := &fakeSender{}

	if err := RunMatch(ctx, db, fs, fetch); err != nil {
		t.Fatalf("RunMatch: %v", err)
	}
	if fs.embeds != 0 || fs.creates != 0 {
		t.Errorf("a cooled-down want still hit Discord (embeds=%d creates=%d); want 0/0", fs.embeds, fs.creates)
	}
	if got, _ := cursorOf(t, db, itemID); got != tNewer {
		t.Errorf("cursor = %q; want %q (the auction was processed despite the cooldown)", got, tNewer)
	}
}

// --- buildEmbed tests --------------------------------------------------------

// fieldByName returns the embed field with the given name, or nil.
func fieldByName(e *discordgo.MessageEmbed, name string) *discordgo.MessageEmbedField {
	for _, f := range e.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// TestBuildEmbed_OmitsNullPriceAndSeller: a nil price omits the Price field (never
// "0pp"); an unresolved seller omits the Seller field; the Title carries the WTS
// tag, the URL is the wiki link, and "Why you wanted it" reflects Reason (+ Note).
func TestBuildEmbed_OmitsNullPriceAndSeller(t *testing.T) {
	note := "tank twink, save up to 2k"
	hit := wantmatch.Hit{
		WantID:        7,
		DiscordUserID: "alice",
		ItemName:      "Flowing Black Silk Sash",
		Reason:        "buy",
		Note:          &note,
	}
	now := time.Date(2026, 6, 6, 2, 3, 0, 0, time.UTC)
	a := mkAuction(0, "2026-06-06T02:00:00+00:00", nil) // nil price
	e := buildEmbed(hit, a, "", seenAgo(a.T, now))      // unresolved seller

	if fieldByName(e, "Price") != nil {
		t.Error("Price field present for a nil price; want OMITTED (never 0pp)")
	}
	if fieldByName(e, "Seller") != nil {
		t.Error("Seller field present when unresolved; want OMITTED (best-effort)")
	}
	if got, want := e.Title, "Flowing Black Silk Sash — WTS"; got != want {
		t.Errorf("Title = %q; want %q (item + WTS tag)", got, want)
	}
	if got, want := e.URL, "https://wiki.project1999.com/Flowing_Black_Silk_Sash"; got != want {
		t.Errorf("URL = %q; want the wiki link %q", got, want)
	}
	why := fieldByName(e, "Why you wanted it")
	if why == nil {
		t.Fatal("Why-you-wanted-it field missing; want present")
	}
	if want := "buy — " + note; why.Value != want {
		t.Errorf("Why = %q; want %q (Reason + Note)", why.Value, want)
	}
}

// TestBuildEmbed_PriceAndSellerPresent: a non-nil price renders the Price field
// (not "0pp") and a resolved seller renders the Seller field.
func TestBuildEmbed_PriceAndSellerPresent(t *testing.T) {
	hit := wantmatch.Hit{WantID: 1, DiscordUserID: "bob", ItemName: "Rubicite Helm", Reason: "quest"}
	now := time.Date(2026, 6, 6, 2, 3, 0, 0, time.UTC)
	a := mkAuction(0, "2026-06-06T02:00:00+00:00", intp(2000))
	e := buildEmbed(hit, a, "Seller1", seenAgo(a.T, now))

	price := fieldByName(e, "Price")
	if price == nil || price.Value != "~2000 pp" {
		t.Errorf("Price field = %v; want %q", price, "~2000 pp")
	}
	seller := fieldByName(e, "Seller")
	if seller == nil || seller.Value != "Seller1" {
		t.Errorf("Seller field = %v; want Seller1", seller)
	}
	seen := fieldByName(e, "Seen")
	if seen == nil || seen.Value != "~3 min ago" {
		t.Errorf("Seen field = %v; want %q", seen, "~3 min ago")
	}
}
