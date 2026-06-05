package webadmin

// monitors.go is the Phase 20 (WANT-08 + the D-10 test-alert / WANT-03) OFFICER
// monitor-controls endpoint backend — the eviction.go/officers.go twin. The route
// layer wraps every handler in webauth.RequireOfficer (the cheap request-time gate);
// the MUTATING handlers ALSO re-check officer status INSIDE their write tx via
// store.IsOfficerTx (the eviction.go WR-04 authorize-under-tx pattern), so a
// just-removed officer cannot land one final flag/channel write on the BEGIN
// IMMEDIATE tx.
//
// Three control surfaces (D-06/D-07/D-08):
//   - the three guild-wide monitor kill-switches (monitor_flag), guild-WIDE state;
//   - the officer-registered source channels (guild_channel) CRUD;
//   - the D-10 "Send me a test alert" bot-pulse, which DMs the CLICKING officer
//     (caller(ctx)) via notify.Send and logs it to their inbox — the WANT-03
//     end-to-end proof.
//
// Security posture:
//   - Every mutator re-checks store.IsOfficerTx in-tx (WR-04 TOCTOU close) and
//     audits via AppendAuditTx in the same tx (detail = ids/flags only, never the
//     bot token or message text, V7).
//   - AddGuildChannelHandler server-validates the inputs (numeric channel id, label
//     non-blank, monitor enum allow-list — the validWant V5 precedent) so officer
//     free-text can't smuggle a non-snowflake channel id into the DB.
//   - The test-alert can ONLY target the calling officer (caller(ctx)) — it is never
//     aimed at another user (T-20-13); source="test" self-targets and is the only
//     gate-bypassing source.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/boejowen/SquireBot/internal/backendsrv/notify"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// validMonitors is the server-side enum allow-list for a monitor name (the DB CHECK
// constraint from 00007 is the second line of defense; this is the first). The three
// monitors map 1:1 to the monitor_flag rows + the guild_channel.monitor column.
var validMonitors = map[string]bool{"ec_auction": true, "wts": true, "raid_target": true}

// numericChannelID matches a Discord channel snowflake: one-or-more ASCII digits and
// nothing else (the AddGuildChannelHandler server-side V5 re-check — never trust the
// form's numeric input alone).
var numericChannelID = regexp.MustCompile(`^[0-9]+$`)

// testAlertBody is the D-10 bot-pulse DM copy (the WANT-03 proof text). It carries no
// PII and is never logged (the Body is V7-sensitive at the notify layer).
const testAlertBody = "SquireBot test alert - your DMs are working. You'll get pinged here when a wanted item shows up."

// sendTestAlert is the notify.Send seam, exposed as a package-private func var so the
// monitors_test.go can stub the live Discord send (the sent / dm_blocked branches)
// without standing up a gateway. Production points it at notify.Send. The handler
// only calls it when botSession != nil (the typed-nil guard), so the *discordgo.Session
// it forwards as the notify.Sender is always a real, non-nil session.
var sendTestAlert = func(r *http.Request, s *discordgo.Session, db *sql.DB, a notify.Alert, now int64) error {
	return notify.Send(r.Context(), s, db, a, now)
}

// MonitorFlagsHandler (GET) returns the three guild-wide kill-switch flags + the
// registered guild_channel list: {flags, channels}. Officer-only at the route.
// Read-only — no tx, no audit.
func MonitorFlagsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		flags, err := store.GetMonitorFlags(ctx, db)
		if err != nil {
			slog.Error("get monitor flags failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		channels, err := store.ListGuildChannels(ctx, db)
		if err != nil {
			slog.Error("list guild channels failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, map[string]any{"flags": flags, "channels": channels})
	}
}

// flagReq is the {monitor, enabled} body for a kill-switch toggle.
type flagReq struct {
	Monitor string `json:"monitor"`
	Enabled bool   `json:"enabled"`
}

// SetMonitorFlagHandler (POST) toggles one guild-wide kill-switch (D-07/D-08).
// Officer-only at the route; validates the monitor enum server-side; authorizes
// INSIDE the tx (store.IsOfficerTx re-check — WR-04); audits "monitor_flag_set".
func SetMonitorFlagHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req flagReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !validMonitors[req.Monitor] {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		err := withTx(ctx, db, func(tx *sql.Tx) error {
			okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
			if e != nil {
				return e
			}
			if !okOfficer {
				return store.ErrNotAuthorized // just-removed officer ⇒ rollback
			}
			if e := store.SetMonitorFlagTx(ctx, tx, req.Monitor, req.Enabled); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "monitor_flag_set", callerID, map[string]any{
				"monitor": req.Monitor, "enabled": req.Enabled,
			}, now)
		})
		if err != nil {
			mapMonitorErr(w, err, "monitor_flag_set")
			return
		}
		writeJSON(w, map[string]any{"monitor": req.Monitor, "enabled": req.Enabled})
	}
}

// addChannelReq is the {label, channel_id, monitor} body for registering a source
// channel. It carries NO owner (the registration is guild-wide, officer-gated).
type addChannelReq struct {
	Label     string `json:"label"`
	ChannelID string `json:"channel_id"`
	Monitor   string `json:"monitor"`
}

// AddGuildChannelHandler (POST) registers an officer-entered source channel for a
// monitor (D-07). Officer-only at the route; server-validates (label non-blank,
// channel_id numeric, monitor enum — the validWant V5 precedent); authorizes in-tx
// (WR-04); audits "monitor_channel_add". A duplicate (channel_id, monitor) maps to
// 409 {"error":"duplicate"} via the typed conflict sentinel the store returns.
func AddGuildChannelHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req addChannelReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		label := strings.TrimSpace(req.Label)
		// Server-side V5 re-check: non-blank label, numeric channel id, monitor enum.
		if label == "" || !numericChannelID.MatchString(req.ChannelID) || !validMonitors[req.Monitor] {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		err := withTx(ctx, db, func(tx *sql.Tx) error {
			okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
			if e != nil {
				return e
			}
			if !okOfficer {
				return store.ErrNotAuthorized
			}
			if _, e := store.AddGuildChannelTx(ctx, tx, req.ChannelID, label, req.Monitor, now); e != nil {
				return e // the dup-channel sentinel ⇒ mapMonitorErr ⇒ 409 duplicate
			}
			return AppendAuditTx(ctx, tx, "monitor_channel_add", callerID, map[string]any{
				"monitor": req.Monitor, "channel_id": req.ChannelID,
			}, now)
		})
		if err != nil {
			mapMonitorErr(w, err, "monitor_channel_add")
			return
		}
		writeJSON(w, map[string]any{"added": true, "channel_id": req.ChannelID, "monitor": req.Monitor})
	}
}

// ListGuildChannelsHandler (GET) returns the registered source channels (non-nil →
// JSON []). Officer-only at the route. Read-only.
func ListGuildChannelsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		channels, err := store.ListGuildChannels(ctx, db)
		if err != nil {
			slog.Error("list guild channels failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, channels)
	}
}

// removeChannelReq is the {channel_id, monitor} body for removing a registration.
type removeChannelReq struct {
	ChannelID string `json:"channel_id"`
	Monitor   string `json:"monitor"`
}

// RemoveGuildChannelHandler (POST) deletes a registered channel (D-07). Officer-only
// at the route; authorizes in-tx (WR-04); audits "monitor_channel_remove" ONLY on a
// real delete (an absent registration is a clean no-op).
func RemoveGuildChannelHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req removeChannelReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
			!numericChannelID.MatchString(req.ChannelID) || !validMonitors[req.Monitor] {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		callerID := caller(ctx)
		now := nowUnix()

		var removed bool
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
			if e != nil {
				return e
			}
			if !okOfficer {
				return store.ErrNotAuthorized
			}
			var re error
			removed, re = store.RemoveGuildChannelTx(ctx, tx, req.ChannelID, req.Monitor)
			if re != nil {
				return re
			}
			if removed {
				return AppendAuditTx(ctx, tx, "monitor_channel_remove", callerID, map[string]any{
					"monitor": req.Monitor, "channel_id": req.ChannelID,
				}, now)
			}
			return nil
		})
		if err != nil {
			mapMonitorErr(w, err, "monitor_channel_remove")
			return
		}
		writeJSON(w, map[string]any{"removed": removed})
	}
}

// SendTestAlertHandler (POST) is the D-10 bot-pulse — the WANT-03 end-to-end proof.
// It DMs the CALLING officer (caller(ctx)) a sample alert via notify.Send and logs it
// to THEIR inbox, then maps the outcome to a status the frontend renders as the three
// toasts (sent / dm_blocked / bot_unavailable).
//
// TYPED-NIL GUARD: the handler takes a CONCRETE *discordgo.Session (NOT the
// notify.Sender interface) precisely so the `botSession == nil` check is a real
// nil-pointer check — a nil *discordgo.Session boxed into a notify.Sender interface
// would be a non-nil interface (the typed-nil trap). The route wiring (Task 3) passes
// b.Session(), which is a real nil when the bot is disabled. Only when non-nil do we
// forward it (the *discordgo.Session satisfies notify.Sender directly).
//
// The Alert carries a nil want id (the alert_log row logs wantlist_item_id=NULL —
// BLOCKER-1; NEVER a 0 or a literal int) and the self-targeting test source (the only
// gate-bypassing source — T-20-13). The mutation is audited regardless of the send
// outcome; a just-removed officer's test-alert rolls back at the in-tx re-check (WR-04).
func SendTestAlertHandler(db *sql.DB, botSession *discordgo.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		callerID := caller(ctx)
		now := nowUnix()

		// Bot offline (no token / failed connect) ⇒ a clean 503, never a nil-deref
		// panic. The test-alert is the only path that touches the live session. The
		// attempt is still audited (with an in-tx officer re-check, WR-04) so a
		// just-removed officer can neither fire nor log a test alert.
		if botSession == nil {
			err := withTx(ctx, db, func(tx *sql.Tx) error {
				okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
				if e != nil {
					return e
				}
				if !okOfficer {
					return store.ErrNotAuthorized
				}
				return AppendAuditTx(ctx, tx, "monitor_test_alert", callerID, map[string]any{"status": "bot_unavailable"}, now)
			})
			if errors.Is(err, store.ErrNotAuthorized) {
				writeJSONError(w, http.StatusForbidden, "not_authorized")
				return
			}
			writeJSONError(w, http.StatusServiceUnavailable, "bot_unavailable")
			return
		}

		alert := notify.Alert{
			WantID:        nil, // BLOCKER-1: wantlist_item_id=NULL (the test row); NEVER 0
			DiscordUserID: callerID,
			Source:        "test", // self-targets the caller; bypasses the two gates + cooldown
			Body:          testAlertBody,
		}
		sendErr := sendTestAlert(r, botSession, db, alert, now)

		// Classify the outcome for the frontend toasts + the audit row.
		var status string
		switch {
		case sendErr == nil:
			status = "sent"
		case errors.Is(sendErr, notify.ErrDMBlocked):
			status = "dm_blocked" // the inbox already logged the dm_blocked row
		default:
			status = "bot_unavailable"
		}

		// Audit the test-alert (status only — never the Body/token, V7) with the in-tx
		// officer re-check (WR-04) so a just-removed officer's test row never lands.
		_ = withTx(ctx, db, func(tx *sql.Tx) error {
			okOfficer, e := store.IsOfficerTx(ctx, tx, callerID)
			if e != nil {
				return e
			}
			if !okOfficer {
				return store.ErrNotAuthorized
			}
			return AppendAuditTx(ctx, tx, "monitor_test_alert", callerID, map[string]any{"status": status}, now)
		})

		switch status {
		case "sent":
			writeJSON(w, map[string]any{"status": "sent"})
		case "dm_blocked":
			// 200: the send was processed + logged; the DM was blocked at Discord (50007).
			writeJSON(w, map[string]any{"error": "dm_blocked"})
		default:
			slog.Error("test alert send failed", "err", sendErr)
			writeJSONError(w, http.StatusBadGateway, "bot_unavailable")
		}
	}
}

// mapMonitorErr maps the monitor store's typed errors to the exact HTTP codes the
// frontend routes (the mapEvictionErr twin). ErrNotAuthorized (the in-tx re-check
// failed) → 403; the duplicate-channel conflict → 409; anything else → 500. A failed
// write writes no audit row (the tx rolled back).
func mapMonitorErr(w http.ResponseWriter, err error, op string) {
	switch {
	case errors.Is(err, store.ErrNotAuthorized):
		writeJSONError(w, http.StatusForbidden, "not_authorized")
	case errors.Is(err, store.ErrDuplicateChannel):
		writeJSONError(w, http.StatusConflict, "duplicate")
	default:
		slog.Error("monitor write failed", "op", op, "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
	}
}
