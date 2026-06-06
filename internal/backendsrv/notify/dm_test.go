package notify

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// dm_test.go covers notify.Send's every branch (Phase 20 Task 3) against a fake
// Sender (no live Discord): send-success, 50007→dm_blocked, generic error, the
// cooldown skip, the dm_blocked repeat-suppression (warning 5), both gates
// (officer flag + user pref), and the test-source bypass with a NULL want id.

// fakeSender is the injectable Discord seam. createErr/sendErr let a test force a
// failure on either step; calls counts the sends so a "no send" assertion bites.
type fakeSender struct {
	createErr error
	sendErr   error
	creates   int
	sends     int
	embeds    int // count of ChannelMessageSendEmbed calls (the rich-embed path)
}

func (f *fakeSender) UserChannelCreate(userID string, _ ...discordgo.RequestOption) (*discordgo.Channel, error) {
	f.creates++
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &discordgo.Channel{ID: "dm-chan-" + userID}, nil
}

func (f *fakeSender) ChannelMessageSend(channelID, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.sends++
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &discordgo.Message{ID: "msg-1"}, nil
}

// ChannelMessageSendEmbed is the rich-embed seam (D-04). It shares sendErr so a
// test can force a 50007/generic failure on the embed path too; it counts embeds
// separately from sends so a test can assert WHICH branch fired.
func (f *fakeSender) ChannelMessageSendEmbed(channelID string, _ *discordgo.MessageEmbed, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.embeds++
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return &discordgo.Message{ID: "embed-1"}, nil
}

// rest50007 is the *discordgo.RESTError discord returns when the recipient can't
// be DM'd (50007 / cannot send messages to this user).
func rest50007() error {
	return &discordgo.RESTError{
		Message: &discordgo.APIErrorMessage{
			Code:    discordgo.ErrCodeCannotSendMessagesToThisUser,
			Message: "Cannot send messages to this user",
		},
	}
}

func seedUser(t *testing.T, ctx context.Context, db *sql.DB, discordID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO web_user (discord_user_id, username, avatar, first_seen, last_login)
		 VALUES (?, ?, NULL, 0, 0)`, discordID, "user-"+discordID); err != nil {
		t.Fatalf("seed web_user %q: %v", discordID, err)
	}
}

// seedWant inserts an active catalog want and returns its id (the FK + dedup key
// for a non-test alert).
func seedWant(t *testing.T, ctx context.Context, db *sql.DB, discordID string, itemID int64) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		`INSERT INTO wantlist_item (discord_user_id, item_id, item_name, reason, priority, active, muted, created_at)
		 VALUES (?, ?, 'Fungi Tunic', 'buy', 'med', 1, 0, 1)`, discordID, itemID)
	if err != nil {
		t.Fatalf("seed want: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed want id: %v", err)
	}
	return id
}

func i64(v int64) *int64 { return &v }

// countAlerts returns the number of alert_log rows for a user (the
// repeat-suppression assertion keys on this staying at 1).
func countAlerts(t *testing.T, db *sql.DB, discordID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM alert_log WHERE discord_user_id = ?`, discordID).Scan(&n); err != nil {
		t.Fatalf("count alerts: %v", err)
	}
	return n
}

// lastStatus returns the newest alert_log send_status for a user.
func lastStatus(t *testing.T, db *sql.DB, discordID string) string {
	t.Helper()
	var s string
	if err := db.QueryRow(
		`SELECT send_status FROM alert_log WHERE discord_user_id = ? ORDER BY id DESC LIMIT 1`,
		discordID).Scan(&s); err != nil {
		t.Fatalf("last status: %v", err)
	}
	return s
}

// ecAlert builds a real (non-test) EC alert for an opted-in user's want.
func ecAlert(wantID int64, discordID string, itemID int64) Alert {
	return Alert{
		WantID:        i64(wantID),
		DiscordUserID: discordID,
		Source:        "ec_auction",
		ItemID:        i64(itemID),
		Body:          "WTS Fungi Tunic in EC",
	}
}

func TestSend_Success_RecordsSent(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	fs := &fakeSender{}

	if err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000); err != nil {
		t.Fatalf("Send(success): %v", err)
	}
	if fs.creates != 1 || fs.sends != 1 {
		t.Errorf("creates=%d sends=%d; want 1/1", fs.creates, fs.sends)
	}
	if got := lastStatus(t, db, "alice"); got != "sent" {
		t.Errorf("send_status = %q; want sent", got)
	}
}

func TestSend_50007_RecordsDMBlocked(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	fs := &fakeSender{sendErr: rest50007()}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrDMBlocked) {
		t.Fatalf("Send(50007) err = %v; want ErrDMBlocked", err)
	}
	if got := lastStatus(t, db, "alice"); got != "dm_blocked" {
		t.Errorf("send_status = %q; want dm_blocked (never silently dropped)", got)
	}
}

func TestSend_50007_OnChannelCreate_RecordsDMBlocked(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	// 50007 can surface on the DM-open step too (recipient blocks DMs).
	fs := &fakeSender{createErr: rest50007()}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrDMBlocked) {
		t.Fatalf("Send(create 50007) err = %v; want ErrDMBlocked", err)
	}
	if fs.sends != 0 {
		t.Errorf("ChannelMessageSend called after a create 50007; want 0")
	}
	if got := lastStatus(t, db, "alice"); got != "dm_blocked" {
		t.Errorf("send_status = %q; want dm_blocked", got)
	}
}

func TestSend_GenericError_RecordsError(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	boom := errors.New("discord 500")
	fs := &fakeSender{sendErr: boom}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000)
	if err == nil || errors.Is(err, ErrDMBlocked) {
		t.Fatalf("Send(generic) err = %v; want a wrapped non-blocked error", err)
	}
	if got := lastStatus(t, db, "alice"); got != "error" {
		t.Errorf("send_status = %q; want error (retryable)", got)
	}
}

func TestSend_Cooldown_RecentSent_Skips(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	// Pre-seed a recent 'sent' row inside the EC window.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, item_id, sent_at, send_status)
		 VALUES (?, 'alice', 'ec_auction', 5000, ?, 'sent')`, want, 1000); err != nil {
		t.Fatalf("seed recent sent: %v", err)
	}
	before := countAlerts(t, db, "alice")
	fs := &fakeSender{}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1001) // 1s later — well inside the window
	if !errors.Is(err, ErrCooledDown) {
		t.Fatalf("Send(cooldown) err = %v; want ErrCooledDown", err)
	}
	if fs.creates != 0 || fs.sends != 0 {
		t.Errorf("sender called during cooldown; creates=%d sends=%d want 0/0", fs.creates, fs.sends)
	}
	if after := countAlerts(t, db, "alice"); after != before {
		t.Errorf("alert_log grew during cooldown (%d→%d); want no new row", before, after)
	}
}

func TestSend_DMBlockedRepeat_Suppressed_NoNewRow(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	// Pre-seed a recent 'dm_blocked' row — a DMs-off user already surfaced once.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, item_id, sent_at, send_status)
		 VALUES (?, 'alice', 'ec_auction', 5000, ?, 'dm_blocked')`, want, 1000); err != nil {
		t.Fatalf("seed recent dm_blocked: %v", err)
	}
	before := countAlerts(t, db, "alice")
	if before != 1 {
		t.Fatalf("setup: expected 1 alert row, got %d", before)
	}
	fs := &fakeSender{sendErr: rest50007()}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1001)
	if !errors.Is(err, ErrCooledDown) {
		t.Fatalf("Send(dm_blocked repeat) err = %v; want ErrCooledDown (warning 5)", err)
	}
	if fs.creates != 0 || fs.sends != 0 {
		t.Errorf("sender called on a suppressed repeat; creates=%d sends=%d want 0/0", fs.creates, fs.sends)
	}
	if after := countAlerts(t, db, "alice"); after != 1 {
		t.Errorf("a repeat dm_blocked added a new row (%d→%d); the inbox must show it ONCE per window", before, after)
	}
}

func TestSend_UserPrefOff_Gated(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	// Master OFF → the user-pref gate closes.
	commitPrefs(t, ctx, db, "alice", store.NotifyPrefs{Master: false, EC: true, WTS: true, Raid: true})
	fs := &fakeSender{}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrGatedOff) {
		t.Fatalf("Send(pref off) err = %v; want ErrGatedOff", err)
	}
	if fs.creates != 0 || fs.sends != 0 {
		t.Errorf("sender called when gated off; want 0/0")
	}
	if n := countAlerts(t, db, "alice"); n != 0 {
		t.Errorf("alert_log row written when gated off (%d); want 0", n)
	}
}

func TestSend_OfficerFlagOff_Gated(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	// Turn the EC officer monitor flag OFF.
	commitFlag(t, ctx, db, "ec_auction", false)
	fs := &fakeSender{}

	err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrGatedOff) {
		t.Fatalf("Send(officer flag off) err = %v; want ErrGatedOff", err)
	}
	if fs.creates != 0 || fs.sends != 0 {
		t.Errorf("sender called when the officer flag is off; want 0/0")
	}
	if n := countAlerts(t, db, "alice"); n != 0 {
		t.Errorf("alert_log row written when officer-gated (%d); want 0", n)
	}
}

func TestSend_TestSource_BypassesGatesAndCooldown_NullWantID(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")

	// Both gates CLOSED for a real source — the test source must ignore them.
	commitPrefs(t, ctx, db, "alice", store.NotifyPrefs{Master: false, EC: false, WTS: false, Raid: false})
	commitFlag(t, ctx, db, "ec_auction", false)

	fs := &fakeSender{}
	a := Alert{
		WantID:        nil, // BLOCKER-1: the test-alert has NO want → NULL FK
		DiscordUserID: "alice",
		Source:        "test",
		Body:          "SquireBot test alert",
	}
	if err := Send(ctx, fs, db, a, 1000); err != nil {
		t.Fatalf("Send(test) err = %v; the test source must always send", err)
	}
	if fs.creates != 1 || fs.sends != 1 {
		t.Errorf("test source did not send; creates=%d sends=%d want 1/1", fs.creates, fs.sends)
	}
	if got := lastStatus(t, db, "alice"); got != "sent" {
		t.Errorf("test send_status = %q; want sent", got)
	}
	// The row's wantlist_item_id is NULL (BLOCKER-1).
	var wantNull sql.NullInt64
	if err := db.QueryRow(
		`SELECT wantlist_item_id FROM alert_log WHERE discord_user_id = 'alice' ORDER BY id DESC LIMIT 1`).
		Scan(&wantNull); err != nil {
		t.Fatalf("read back test row: %v", err)
	}
	if wantNull.Valid {
		t.Errorf("test-alert wantlist_item_id = %d; want NULL", wantNull.Int64)
	}

	// And a SECOND test alert sends again (cooldown bypassed — the bot pulse).
	if err := Send(ctx, fs, db, a, 1001); err != nil {
		t.Fatalf("second Send(test): %v", err)
	}
	if fs.sends != 2 {
		t.Errorf("second test alert did not send; sends=%d want 2 (no cooldown on test)", fs.sends)
	}
}

// ecEmbedAlert builds a real (non-test) EC alert carrying a rich Embed (D-04).
// Body stays set so a regression can prove the embed branch is chosen OVER the
// string branch when both are present.
func ecEmbedAlert(wantID int64, discordID string, itemID int64) Alert {
	a := ecAlert(wantID, discordID, itemID)
	a.Embed = &discordgo.MessageEmbed{Title: "WTS Fungi Tunic"}
	return a
}

func TestSendEmbed_Success_UsesEmbedPath_RecordsSent(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	fs := &fakeSender{}

	if err := Send(ctx, fs, db, ecEmbedAlert(want, "alice", 5000), 1000); err != nil {
		t.Fatalf("Send(embed success): %v", err)
	}
	if fs.embeds != 1 {
		t.Errorf("embeds=%d; want 1 (the embed branch must fire)", fs.embeds)
	}
	if fs.sends != 0 {
		t.Errorf("sends=%d; want 0 (a non-nil Embed must NOT take the string branch)", fs.sends)
	}
	if got := lastStatus(t, db, "alice"); got != "sent" {
		t.Errorf("send_status = %q; want sent", got)
	}
}

func TestSendEmbed_NilEmbed_FallsBackToString(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	fs := &fakeSender{}

	// ecAlert leaves Embed nil — the P20 plain-string path must be unchanged.
	if err := Send(ctx, fs, db, ecAlert(want, "alice", 5000), 1000); err != nil {
		t.Fatalf("Send(nil embed): %v", err)
	}
	if fs.sends != 1 {
		t.Errorf("sends=%d; want 1 (nil Embed → plain string)", fs.sends)
	}
	if fs.embeds != 0 {
		t.Errorf("embeds=%d; want 0 (no embed call when Embed is nil)", fs.embeds)
	}
}

func TestSendEmbed_50007_RecordsDMBlocked(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)
	fs := &fakeSender{sendErr: rest50007()}

	err := Send(ctx, fs, db, ecEmbedAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrDMBlocked) {
		t.Fatalf("Send(embed 50007) err = %v; want ErrDMBlocked", err)
	}
	if fs.embeds != 1 {
		t.Errorf("embeds=%d; want 1 (the embed branch was attempted)", fs.embeds)
	}
	if got := lastStatus(t, db, "alice"); got != "dm_blocked" {
		t.Errorf("send_status = %q; want dm_blocked (never silently dropped)", got)
	}
}

func TestSendEmbed_OfficerFlagOff_Gated(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	commitFlag(t, ctx, db, "ec_auction", false)
	fs := &fakeSender{}

	err := Send(ctx, fs, db, ecEmbedAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrGatedOff) {
		t.Fatalf("Send(embed, officer flag off) err = %v; want ErrGatedOff", err)
	}
	if fs.creates != 0 || fs.embeds != 0 {
		t.Errorf("sender called when officer-gated; creates=%d embeds=%d want 0/0", fs.creates, fs.embeds)
	}
	if n := countAlerts(t, db, "alice"); n != 0 {
		t.Errorf("alert_log row written when officer-gated (%d); want 0", n)
	}
}

func TestSendEmbed_UserPrefOff_Gated(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	commitPrefs(t, ctx, db, "alice", store.NotifyPrefs{Master: false, EC: true, WTS: true, Raid: true})
	fs := &fakeSender{}

	err := Send(ctx, fs, db, ecEmbedAlert(want, "alice", 5000), 1000)
	if !errors.Is(err, ErrGatedOff) {
		t.Fatalf("Send(embed, pref off) err = %v; want ErrGatedOff", err)
	}
	if fs.creates != 0 || fs.embeds != 0 {
		t.Errorf("sender called when pref-gated; creates=%d embeds=%d want 0/0", fs.creates, fs.embeds)
	}
}

func TestSendEmbed_Cooldown_RecentSent_Skips(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	seedUser(t, ctx, db, "alice")
	want := seedWant(t, ctx, db, "alice", 5000)

	// Pre-seed a recent 'sent' row inside the EC window — the embed path must
	// inherit the SAME dedup/cooldown (no second code path).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, item_id, sent_at, send_status)
		 VALUES (?, 'alice', 'ec_auction', 5000, ?, 'sent')`, want, 1000); err != nil {
		t.Fatalf("seed recent sent: %v", err)
	}
	before := countAlerts(t, db, "alice")
	fs := &fakeSender{}

	err := Send(ctx, fs, db, ecEmbedAlert(want, "alice", 5000), 1001)
	if !errors.Is(err, ErrCooledDown) {
		t.Fatalf("Send(embed cooldown) err = %v; want ErrCooledDown", err)
	}
	if fs.creates != 0 || fs.embeds != 0 {
		t.Errorf("sender called during cooldown; creates=%d embeds=%d want 0/0", fs.creates, fs.embeds)
	}
	if after := countAlerts(t, db, "alice"); after != before {
		t.Errorf("alert_log grew during cooldown (%d→%d); want no new row", before, after)
	}
}

// commitPrefs writes a user's prefs via UpsertPrefsTx in a committed tx.
func commitPrefs(t *testing.T, ctx context.Context, db *sql.DB, discordID string, p store.NotifyPrefs) {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin prefs tx: %v", err)
	}
	if err := store.UpsertPrefsTx(ctx, tx, discordID, p, 1); err != nil {
		_ = tx.Rollback()
		t.Fatalf("upsert prefs: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit prefs: %v", err)
	}
}

// commitFlag toggles a monitor flag via SetMonitorFlagTx in a committed tx.
func commitFlag(t *testing.T, ctx context.Context, db *sql.DB, monitor string, enabled bool) {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin flag tx: %v", err)
	}
	if err := store.SetMonitorFlagTx(ctx, tx, monitor, enabled); err != nil {
		_ = tx.Rollback()
		t.Fatalf("set monitor flag: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit flag: %v", err)
	}
}
