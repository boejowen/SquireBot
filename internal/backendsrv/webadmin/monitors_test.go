package webadmin

// monitors_test.go covers the Phase 20 (WANT-08 + D-10) officer monitor handlers
// (20-03 Task 2): the flags GET, the kill-switch flag set + audit, the add-channel
// valid/duplicate-409/invalid-400 paths, the remove, the in-tx officer re-check
// (WR-04 — a non-officer caller rolls back with ErrNotAuthorized), and the D-10
// test-alert branches (bot_unavailable on a nil session; sent / dm_blocked via the
// stubbable sendTestAlert seam). The officer identity is injected via withCaller
// exactly as RequireOfficer would place it; the in-tx store.IsOfficerTx re-check is
// the load-bearing gate these tests exercise.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/notify"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"

	"github.com/bwmarrin/discordgo"
)

func TestMonitorFlags_GET(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	h := withCaller(floor, MonitorFlagsHandler(db))
	rec := getJSON(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Flags    store.MonitorFlags   `json:"flags"`
		Channels []store.GuildChannel `json:"channels"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode flags resp: %v", err)
	}
	// 00007 seed: ec_auction=1, wts=0, raid_target=0.
	if !resp.Flags.EC || resp.Flags.WTS || resp.Flags.Raid {
		t.Fatalf("seeded flags = %+v, want EC on / WTS off / Raid off", resp.Flags)
	}
	if resp.Channels == nil {
		t.Fatalf("channels = nil, want non-nil [] (JSON [])")
	}
}

func TestMonitorFlagSet_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	h := withCaller(floor, SetMonitorFlagHandler(db))
	// Turn WTS on.
	rec := postJSON(t, h, `{"monitor":"wts","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	flags, err := store.GetMonitorFlags(ctx, db)
	if err != nil {
		t.Fatalf("read flags: %v", err)
	}
	if !flags.WTS {
		t.Fatalf("WTS not enabled after flag set")
	}
	if c := auditCount(t, ctx, db, "monitor_flag_set"); c != 1 {
		t.Fatalf("monitor_flag_set audit rows = %d, want 1", c)
	}

	// Invalid monitor enum → 400.
	recBad := postJSON(t, h, `{"monitor":"bogus","enabled":true}`)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("invalid monitor status = %d, want 400", recBad.Code)
	}
}

func TestMonitorFlagSet_NonOfficerRejected(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	stranger := "999999999999999999"
	seedFloorAndUsers(t, ctx, db, floor, map[string]string{stranger: "Stranger"})

	// A non-officer caller → the in-tx IsOfficerTx re-check rolls back → 403.
	h := withCaller(stranger, SetMonitorFlagHandler(db))
	rec := postJSON(t, h, `{"monitor":"wts","enabled":true}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if got := decodeErr(t, rec); got != "not_authorized" {
		t.Fatalf("error = %q, want not_authorized", got)
	}
	// Nothing was written: WTS stays at its dark seed, no audit row.
	flags, _ := store.GetMonitorFlags(ctx, db)
	if flags.WTS {
		t.Fatalf("WTS flipped despite unauthorized caller")
	}
	if c := auditCount(t, ctx, db, "monitor_flag_set"); c != 0 {
		t.Fatalf("monitor_flag_set audit rows = %d, want 0", c)
	}
}

func TestMonitorChannel_AddDuplicateInvalidRemove(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	addH := withCaller(floor, AddGuildChannelHandler(db))

	// Valid add → 200 + audit.
	rec := postJSON(t, addH, `{"label":"Raid Alliance","channel_id":"123456789","monitor":"raid_target"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("add status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if c := auditCount(t, ctx, db, "monitor_channel_add"); c != 1 {
		t.Fatalf("monitor_channel_add audit rows = %d, want 1", c)
	}

	// Exact duplicate (same channel_id + monitor) → 409 duplicate.
	recDup := postJSON(t, addH, `{"label":"Raid Alliance","channel_id":"123456789","monitor":"raid_target"}`)
	if recDup.Code != http.StatusConflict {
		t.Fatalf("dup status = %d, want 409 (body=%s)", recDup.Code, recDup.Body.String())
	}
	if got := decodeErr(t, recDup); got != "duplicate" {
		t.Fatalf("dup error = %q, want duplicate", got)
	}

	// Invalid (non-numeric channel id) → 400.
	recBad := postJSON(t, addH, `{"label":"X","channel_id":"not-a-snowflake","monitor":"wts"}`)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("non-numeric channel status = %d, want 400", recBad.Code)
	}
	// Invalid (blank label) → 400.
	recBlank := postJSON(t, addH, `{"label":"   ","channel_id":"222","monitor":"wts"}`)
	if recBlank.Code != http.StatusBadRequest {
		t.Fatalf("blank label status = %d, want 400", recBlank.Code)
	}

	// Remove the registered channel → removed:true + audit.
	rmH := withCaller(floor, RemoveGuildChannelHandler(db))
	recRm := postJSON(t, rmH, `{"channel_id":"123456789","monitor":"raid_target"}`)
	if recRm.Code != http.StatusOK {
		t.Fatalf("remove status = %d, want 200 (body=%s)", recRm.Code, recRm.Body.String())
	}
	var rmBody struct {
		Removed bool `json:"removed"`
	}
	if err := json.Unmarshal(recRm.Body.Bytes(), &rmBody); err != nil {
		t.Fatalf("decode remove resp: %v", err)
	}
	if !rmBody.Removed {
		t.Fatalf("removed = false, want true")
	}
	if c := auditCount(t, ctx, db, "monitor_channel_remove"); c != 1 {
		t.Fatalf("monitor_channel_remove audit rows = %d, want 1", c)
	}

	channels, _ := store.ListGuildChannels(ctx, db)
	if len(channels) != 0 {
		t.Fatalf("channels after remove = %d, want 0", len(channels))
	}
}

func TestListGuildChannels_NonNil(t *testing.T) {
	db := store.NewTestDB(t)
	floor := "111111111111111111"
	seedFloorAndUsers(t, context.Background(), db, floor, nil)
	h := withCaller(floor, ListGuildChannelsHandler(db))
	rec := getJSON(t, h)
	if body := rec.Body.String(); body != "[]\n" && body != "[]" {
		t.Fatalf("empty channels body = %q, want []", body)
	}
}

func TestTestAlert_NilSession_BotUnavailable(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	// Bot disabled (nil session) → 503 bot_unavailable, no panic; the attempt is
	// still audited.
	h := withCaller(floor, SendTestAlertHandler(db, nil))
	rec := postJSON(t, h, ``)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "bot_unavailable" {
		t.Fatalf("error = %q, want bot_unavailable", got)
	}
	if c := auditCount(t, ctx, db, "monitor_test_alert"); c != 1 {
		t.Fatalf("monitor_test_alert audit rows = %d, want 1", c)
	}
}

func TestTestAlert_Sent(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	// Stub the send seam to succeed (the live notify.Send mechanics are covered by
	// notify's own dm_test.go). The handler only reaches the seam with a non-nil
	// session, so pass a non-nil (empty) *discordgo.Session sentinel.
	restore := sendTestAlert
	var gotSource string
	var gotWantNil bool
	sendTestAlert = func(_ *http.Request, _ *discordgo.Session, _ *sql.DB, a notify.Alert, _ int64) error {
		gotSource = a.Source
		gotWantNil = a.WantID == nil
		return nil
	}
	defer func() { sendTestAlert = restore }()

	sess := &discordgo.Session{}
	h := withCaller(floor, SendTestAlertHandler(db, sess))
	rec := postJSON(t, h, ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode sent resp: %v", err)
	}
	if body.Status != "sent" {
		t.Fatalf("status = %q, want sent", body.Status)
	}
	if gotSource != "test" {
		t.Fatalf("alert source = %q, want test", gotSource)
	}
	if !gotWantNil {
		t.Fatalf("alert WantID was non-nil, want nil (BLOCKER-1 NULL FK)")
	}
	if c := auditCount(t, ctx, db, "monitor_test_alert"); c != 1 {
		t.Fatalf("monitor_test_alert audit rows = %d, want 1", c)
	}
}

func TestTestAlert_DMBlocked(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	floor := "111111111111111111"
	seedFloorAndUsers(t, ctx, db, floor, nil)

	restore := sendTestAlert
	sendTestAlert = func(_ *http.Request, _ *discordgo.Session, _ *sql.DB, _ notify.Alert, _ int64) error {
		return notify.ErrDMBlocked
	}
	defer func() { sendTestAlert = restore }()

	sess := &discordgo.Session{}
	h := withCaller(floor, SendTestAlertHandler(db, sess))
	rec := postJSON(t, h, ``)
	// dm_blocked is a processed-but-blocked outcome: 200 with {"error":"dm_blocked"}.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeErr(t, rec); got != "dm_blocked" {
		t.Fatalf("error = %q, want dm_blocked", got)
	}
	if c := auditCount(t, ctx, db, "monitor_test_alert"); c != 1 {
		t.Fatalf("monitor_test_alert audit rows = %d, want 1", c)
	}
}
