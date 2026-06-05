package webadmin

// notifications_test.go covers the Phase 20 (WANT-04) login-only notification
// handlers (20-03 Task 1): prefs default-ON GET; SetPrefs round-trip + audit row;
// inbox newest-first; unread-count; mark-read flips read_at; mark-read on another
// owner's alert is a silent no-op (read:false, IDOR); read-all. Every handler
// derives the owner from the session (webauth.UserFromContext, injected via
// withCaller) — the body never carries an owner (D-02).

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// seedAlert inserts an alert_log row directly (the inbox/mark-read fixture). source
// is the monitor source; sentAt drives the newest-first ordering; read controls the
// read_at column (nil ⇒ unread). Returns the new row id.
func seedAlert(t *testing.T, ctx context.Context, db *sql.DB, discordID, source string, sentAt int64, read bool) int64 {
	t.Helper()
	var readAt any
	if read {
		readAt = sentAt + 1
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO alert_log (wantlist_item_id, discord_user_id, source, item_id, detail, sent_at, send_status, read_at)
		 VALUES (NULL, ?, ?, NULL, NULL, ?, 'sent', ?)`,
		discordID, source, sentAt, readAt)
	if err != nil {
		t.Fatalf("seed alert_log (%s): %v", discordID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed alert last insert id: %v", err)
	}
	return id
}

// getJSON runs a GET against the handler (the prefs/inbox/unread-count read path).
func getJSON(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec
}

func TestPrefs_DefaultOn_GET(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-prefs-default"
	seedWebUser(t, ctx, db, callerID, "PrefsDefault")

	h := withCaller(callerID, GetPrefsHandler(db))
	rec := getJSON(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var p store.NotifyPrefs
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode prefs: %v", err)
	}
	// D-01: an absent prefs row reads all-ON.
	if !p.Master || !p.EC || !p.WTS || !p.Raid {
		t.Fatalf("default prefs = %+v, want all true (D-01 default-ON)", p)
	}
}

func TestPrefs_SetRoundTrip_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-prefs-set"
	seedWebUser(t, ctx, db, callerID, "PrefsSetter")

	setH := withCaller(callerID, SetPrefsHandler(db))
	// Turn the WTS toggle off, keep the rest on.
	rec := postJSON(t, setH, `{"master":true,"ec":true,"wts":false,"raid":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("set status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var stored store.NotifyPrefs
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode set resp: %v", err)
	}
	if !stored.Master || !stored.EC || stored.WTS || !stored.Raid {
		t.Fatalf("echoed prefs = %+v, want WTS off / rest on", stored)
	}
	if c := auditCount(t, ctx, db, "notify_prefs_set"); c != 1 {
		t.Fatalf("notify_prefs_set audit rows = %d, want 1", c)
	}

	// Round-trip: a fresh GET reflects the stored state.
	getH := withCaller(callerID, GetPrefsHandler(db))
	rec2 := getJSON(t, getH)
	var reread store.NotifyPrefs
	if err := json.Unmarshal(rec2.Body.Bytes(), &reread); err != nil {
		t.Fatalf("decode reread: %v", err)
	}
	if reread.WTS {
		t.Fatalf("reread WTS = true, want false (persisted)")
	}
}

func TestInbox_NewestFirst_OwnerScoped(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-inbox"
	seedWebUser(t, ctx, db, callerID, "Inboxer")

	// Two rows for the caller (older then newer) + one for another member (excluded).
	seedAlert(t, ctx, db, callerID, "ec_auction", 1700000000, false)
	seedAlert(t, ctx, db, callerID, "wts", 1700000100, false)
	seedWebUser(t, ctx, db, "disc-other", "Other")
	seedAlert(t, ctx, db, "disc-other", "raid_target", 1700000200, false)

	h := withCaller(callerID, ListInboxHandler(db))
	rec := getJSON(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var rows []store.AlertLogRow
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode inbox: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("inbox len = %d, want 2 (other member's row excluded)", len(rows))
	}
	// Newest-first: the wts (1700000100) row precedes the ec_auction (1700000000).
	if rows[0].Source != "wts" || rows[1].Source != "ec_auction" {
		t.Fatalf("inbox order = [%s,%s], want [wts,ec_auction] (newest-first)", rows[0].Source, rows[1].Source)
	}
}

func TestInbox_Empty_NonNil(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-inbox-empty"
	seedWebUser(t, ctx, db, callerID, "InboxEmpty")

	h := withCaller(callerID, ListInboxHandler(db))
	rec := getJSON(t, h)
	if body := rec.Body.String(); body != "[]\n" && body != "[]" {
		t.Fatalf("empty inbox body = %q, want []", body)
	}
}

func TestUnreadCount(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-unread"
	seedWebUser(t, ctx, db, callerID, "Unreader")

	// 2 unread + 1 already-read for the caller; 1 unread for another member.
	seedAlert(t, ctx, db, callerID, "ec_auction", 1700000000, false)
	seedAlert(t, ctx, db, callerID, "wts", 1700000100, false)
	seedAlert(t, ctx, db, callerID, "raid_target", 1700000200, true)
	seedWebUser(t, ctx, db, "disc-other", "Other")
	seedAlert(t, ctx, db, "disc-other", "ec_auction", 1700000300, false)

	h := withCaller(callerID, UnreadCountHandler(db))
	rec := getJSON(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode unread-count: %v", err)
	}
	if body.Count != 2 {
		t.Fatalf("unread count = %d, want 2 (own unread only)", body.Count)
	}
}

func TestNotif_MarkRead_OwnFlips_CrossOwnerNoOp(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-markread"
	seedWebUser(t, ctx, db, callerID, "MarkReader")

	ownID := seedAlert(t, ctx, db, callerID, "ec_auction", 1700000000, false)

	// Another member's alert (the IDOR fixture).
	seedWebUser(t, ctx, db, "disc-other", "Other")
	otherID := seedAlert(t, ctx, db, "disc-other", "wts", 1700000100, false)

	h := withCaller(callerID, MarkReadHandler(db))

	// Cross-owner mark-read → read:false, the other member's row untouched, no audit.
	recX := postJSON(t, h, `{"id":`+itoa(otherID)+`}`)
	if recX.Code != http.StatusOK {
		t.Fatalf("cross-owner status = %d, want 200", recX.Code)
	}
	if decodeRead(t, recX) {
		t.Fatalf("cross-owner mark-read returned read=true — IDOR")
	}
	if unreadOf(t, ctx, db, "disc-other") != 1 {
		t.Fatalf("other member's alert was marked read — IDOR")
	}
	if c := auditCount(t, ctx, db, "notify_read"); c != 0 {
		t.Fatalf("notify_read audit rows = %d, want 0 (no-op not audited)", c)
	}

	// Own mark-read → read:true, read_at set, audited.
	recOwn := postJSON(t, h, `{"id":`+itoa(ownID)+`}`)
	if recOwn.Code != http.StatusOK {
		t.Fatalf("own status = %d, want 200 (body=%s)", recOwn.Code, recOwn.Body.String())
	}
	if !decodeRead(t, recOwn) {
		t.Fatalf("own mark-read returned read=false, want true")
	}
	if unreadOf(t, ctx, db, callerID) != 0 {
		t.Fatalf("own alert not marked read")
	}
	if c := auditCount(t, ctx, db, "notify_read"); c != 1 {
		t.Fatalf("notify_read audit rows = %d, want 1", c)
	}
}

func TestNotif_MarkRead_BadID_400(t *testing.T) {
	db := store.NewTestDB(t)
	callerID := "disc-markread-bad"
	seedWebUser(t, context.Background(), db, callerID, "BadMark")
	h := withCaller(callerID, MarkReadHandler(db))
	rec := postJSON(t, h, `{"id":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := decodeErr(t, rec); got != "invalid_input" {
		t.Fatalf("error = %q, want invalid_input", got)
	}
}

func TestNotif_MarkAllRead_OwnerScoped_Audits(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()
	callerID := "disc-readall"
	seedWebUser(t, ctx, db, callerID, "ReadAller")

	seedAlert(t, ctx, db, callerID, "ec_auction", 1700000000, false)
	seedAlert(t, ctx, db, callerID, "wts", 1700000100, false)
	// Another member's unread row that must NOT be flipped.
	seedWebUser(t, ctx, db, "disc-other", "Other")
	seedAlert(t, ctx, db, "disc-other", "raid_target", 1700000200, false)

	h := withCaller(callerID, MarkAllReadHandler(db))
	rec := postJSON(t, h, ``)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode read-all: %v", err)
	}
	if body.Count != 2 {
		t.Fatalf("read-all count = %d, want 2 (own unread only)", body.Count)
	}
	if unreadOf(t, ctx, db, callerID) != 0 {
		t.Fatalf("caller still has unread after read-all")
	}
	if unreadOf(t, ctx, db, "disc-other") != 1 {
		t.Fatalf("other member's unread was flipped — owner scope breach")
	}
	if c := auditCount(t, ctx, db, "notify_read_all"); c != 1 {
		t.Fatalf("notify_read_all audit rows = %d, want 1", c)
	}
}

// unreadOf reads the unread (read_at IS NULL) alert count for a discord_user_id.
func unreadOf(t *testing.T, ctx context.Context, db *sql.DB, discordID string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM alert_log WHERE discord_user_id = ? AND read_at IS NULL`, discordID).Scan(&n); err != nil {
		t.Fatalf("count unread (%s): %v", discordID, err)
	}
	return n
}

// decodeRead extracts the {"read":bool} field.
func decodeRead(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var body struct {
		Read bool `json:"read"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode read body %q: %v", rec.Body.String(), err)
	}
	return body.Read
}
