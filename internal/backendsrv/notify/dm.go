// Package notify is the SquireBot backend's DM-send + alert-audit core (Phase 20,
// WANT-03/04). It is the proof the bot CAN DM (WANT-03) and that alerts are
// deduped/cooled with a first-class 50007 fallback (WANT-04). Send is the single
// entry point every monitor (P21-23) and the officer test-alert (P03) calls.
//
// What Send does, in order:
//  1. GATE 1 (officer kill-switch): the source's monitor_flag must be ON.
//  2. GATE 2 (user opt-in): the recipient's notify_prefs master + per-source
//     toggle must allow it (D-08 two-gate). A muted want never reaches here — the
//     mute gate lives upstream at the wantmatch seam (D-09).
//  3. DEDUP/COOLDOWN: the store dedup probe (which already filters sent OR
//     dm_blocked, Plan 01 / warning 5) short-circuits a repeat — no re-DM, no
//     re-log. So a DMs-off user does NOT accrue an identical CAN'T-DM inbox row
//     every cycle.
//  4. SEND: open the DM (UserChannelCreate) then send (ChannelMessageSend), and
//     record EVERY first attempt in alert_log (sent | dm_blocked | error, D-04) —
//     50007 maps to dm_blocked (typed ErrDMBlocked), NEVER silently dropped.
//
// The `test` source (the D-10 officer bot-pulse) BYPASSES both gates AND the
// cooldown and carries a nil WantID (→ wantlist_item_id NULL, BLOCKER-1); the
// nil is never dereferenced on the test path.
//
// IMPORT-DIRECTION CONSTRAINT (warning 9): in Plan 03 the web-admin layer imports
// notify (the test-alert handler calls Send), so notify MUST NOT import that layer
// back — reusing its withTx helper would be an import cycle. notify opens its OWN
// local transaction on the *sql.DB it is handed (db.Begin → defer Rollback →
// Commit).
//
// SECURITY (V7): the DM Body + item names are user/wiki-controlled text sent to
// Discord but NEVER logged — the slog line carries source + status + want id only.
package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// Sender is the injectable Discord seam: the two REST calls Send makes.
// *discordgo.Session satisfies it (the real session is injected from the bot
// package in Plan 03); tests inject a fake so no live Discord is required.
type Sender interface {
	UserChannelCreate(userID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// Compile-time proof the real *discordgo.Session satisfies Sender, so the bot's
// shared session injects directly (Plan 03) with no adapter.
var _ Sender = (*discordgo.Session)(nil)

// Alert is one notification to deliver. WantID is *int64 (nullable): the D-10
// test-alert passes nil → wantlist_item_id=NULL (BLOCKER-1); a real wantmatch Hit
// passes the want id. Source ∈ {ec_auction, wts, raid_target, test}. Body is the
// rendered DM text (NEVER logged, V7). ItemID/Detail are nullable.
type Alert struct {
	WantID        *int64
	DiscordUserID string
	Source        string
	ItemID        *int64
	Body          string
	Detail        *string
}

// Send-result sentinels — typed so callers (and tests) can branch precisely.
var (
	// ErrDMBlocked: Discord returned 50007 (cannot send messages to this user).
	// The attempt is recorded as dm_blocked — NEVER silently dropped (D-04).
	ErrDMBlocked = errors.New("notify: dm blocked (50007)")
	// ErrCooledDown: a recent sent OR dm_blocked alert exists inside the window;
	// the send (and the re-log) is suppressed (warning 5).
	ErrCooledDown = errors.New("notify: cooled down")
	// ErrGatedOff: the officer monitor flag or the user pref disallows this
	// source — no send, no alert_log row (the two-gate rule, D-08).
	ErrGatedOff = errors.New("notify: gated off")
)

// Per-source cooldown windows (placeholders — finalized in the P21 soak). The
// test source has a ZERO window so the officer bot-pulse always fires (D-10).
const (
	cooldownEC   = 22 * time.Hour
	cooldownWTS  = 90 * time.Minute
	cooldownRaid = 90 * time.Minute
	cooldownTest = 0
)

// cooldownFor maps a source to its dedup window. An unknown source gets the
// conservative EC window (never zero except for the explicit test source).
func cooldownFor(source string) time.Duration {
	switch source {
	case "wts":
		return cooldownWTS
	case "raid_target":
		return cooldownRaid
	case "test":
		return cooldownTest
	default: // ec_auction + any unexpected source
		return cooldownEC
	}
}

// Send delivers one Alert: it enforces both gates + the dedup/cooldown, opens and
// sends the DM, and records the attempt in alert_log via its OWN local tx. now is
// epoch-seconds (injected so tests are deterministic; production passes
// time.Now().Unix()).
func Send(ctx context.Context, s Sender, db *sql.DB, a Alert, now int64) error {
	isTest := a.Source == "test"

	if !isTest {
		// GATE 1 — officer kill-switch: the source's monitor_flag must be ON.
		flags, err := store.GetMonitorFlags(ctx, db)
		if err != nil {
			return fmt.Errorf("notify: get monitor flags: %w", err)
		}
		if !monitorEnabled(flags, a.Source) {
			return ErrGatedOff
		}

		// GATE 2 — user opt-in: master + the per-source toggle must allow it.
		prefs, err := store.GetPrefs(ctx, db, a.DiscordUserID)
		if err != nil {
			return fmt.Errorf("notify: get prefs: %w", err)
		}
		if !prefAllows(prefs, a.Source) {
			return ErrGatedOff
		}

		// DEDUP/COOLDOWN — a recent sent OR dm_blocked (Plan 01 filter) suppresses
		// a repeat: no re-DM, no re-log (warning 5). A real source always has a
		// non-nil WantID (a wantmatch Hit); guard defensively.
		if a.WantID != nil {
			since := now - int64(cooldownFor(a.Source).Seconds())
			recent, err := store.RecentAlertExists(ctx, db, *a.WantID, a.Source, a.ItemID, since)
			if err != nil {
				return fmt.Errorf("notify: recent alert probe: %w", err)
			}
			if recent {
				return ErrCooledDown
			}
		}
	}

	// SEND — open the DM channel, then send the message.
	ch, err := s.UserChannelCreate(a.DiscordUserID)
	if err != nil {
		if isDMBlocked(err) {
			_ = recordAttempt(ctx, db, a, "dm_blocked", now)
			slog.Warn("notify dm blocked", "source", a.Source, "want", wantIDLog(a), "status", "dm_blocked")
			return ErrDMBlocked
		}
		// A non-50007 open failure is a retryable error attempt.
		_ = recordAttempt(ctx, db, a, "error", now)
		slog.Warn("notify dm open failed", "source", a.Source, "want", wantIDLog(a), "status", "error")
		return fmt.Errorf("notify: open dm: %w", err)
	}

	if _, err := s.ChannelMessageSend(ch.ID, a.Body); err != nil {
		if isDMBlocked(err) {
			_ = recordAttempt(ctx, db, a, "dm_blocked", now)
			slog.Warn("notify dm blocked", "source", a.Source, "want", wantIDLog(a), "status", "dm_blocked")
			return ErrDMBlocked
		}
		_ = recordAttempt(ctx, db, a, "error", now)
		slog.Warn("notify dm send failed", "source", a.Source, "want", wantIDLog(a), "status", "error")
		return fmt.Errorf("notify: send dm: %w", err)
	}

	if err := recordAttempt(ctx, db, a, "sent", now); err != nil {
		return err
	}
	slog.Info("notify dm sent", "source", a.Source, "want", wantIDLog(a), "status", "sent")
	return nil
}

// recordAttempt writes one alert_log row in notify's OWN local transaction (NOT
// the web-admin withTx — warning 9 import cycle). db.Begin honors _txlock=immediate
// DSN (BEGIN IMMEDIATE); the deferred Rollback is a harmless no-op once committed.
func recordAttempt(ctx context.Context, db *sql.DB, a Alert, status string, now int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("notify: begin alert tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful Commit
	if _, err := store.InsertAlertTx(ctx, tx, a.WantID, a.DiscordUserID, a.Source, a.ItemID, a.Detail, now, status); err != nil {
		return fmt.Errorf("notify: record alert (status=%s): %w", status, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("notify: commit alert: %w", err)
	}
	return nil
}

// isDMBlocked reports whether err is the Discord 50007 (cannot send messages to
// this user) REST error. Detected via the typed *discordgo.RESTError code, NOT a
// string-match (the AddWantTx typed-sentinel discipline).
func isDMBlocked(err error) bool {
	var re *discordgo.RESTError
	return errors.As(err, &re) && re.Message != nil &&
		re.Message.Code == discordgo.ErrCodeCannotSendMessagesToThisUser
}

// monitorEnabled maps a source to its guild-wide kill-switch flag.
func monitorEnabled(f store.MonitorFlags, source string) bool {
	switch source {
	case "ec_auction":
		return f.EC
	case "wts":
		return f.WTS
	case "raid_target":
		return f.Raid
	default:
		return false
	}
}

// prefAllows maps a source to the user's per-monitor opt-in (master-gated).
func prefAllows(p store.NotifyPrefs, source string) bool {
	if !p.Master {
		return false
	}
	switch source {
	case "ec_auction":
		return p.EC
	case "wts":
		return p.WTS
	case "raid_target":
		return p.Raid
	default:
		return false
	}
}

// wantIDLog renders a log-safe want id (-1 for the nil test-alert) so the slog
// line never dereferences a nil pointer and never carries the DM Body (V7).
func wantIDLog(a Alert) int64 {
	if a.WantID == nil {
		return -1
	}
	return *a.WantID
}
