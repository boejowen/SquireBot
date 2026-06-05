// Package bot is the SquireBot backend's Discord gateway client (Phase 20,
// WANT-03/04): a single shared *discordgo.Session whose lifetime is tied to the
// server's root context, started in a NON-BLOCKING, NON-FATAL, recover-isolated
// goroutine that clones the scheduler's grain (internal/backendsrv/scheduler).
//
// Why a long-lived gateway connection (not a token-per-REST-call): discordgo
// opens a single WSS gateway, auto-reconnects, and shares one auto-rate-limited
// REST client. notify rides that same *Session (injected via bot.Session()) so
// every DM goes out under one connection — there is no per-send login.
//
// LOCKED invariants (Pitfall 7 / T-20-06):
//   - Start is NON-BLOCKING: it returns immediately; the server's main goroutine
//     keeps owning the HTTP listener (the scheduler.Start precedent).
//   - Start is NON-FATAL: a missing token disables the bot (the server boots
//     bot-less); an Open() error is RETURNED for the caller to log-and-continue,
//     never a process exit.
//   - Every goroutine + (future P22/P23) gateway handler defers recoverBoundary:
//     a panic in the bot loop is logged and SWALLOWED so the website / scheduler /
//     ingest survive. A bot panic must NEVER propagate.
//   - ctx-tied lifetime: a goroutine waits on the cancelled root context then
//     dg.Close(), mirroring scheduler's clean-shutdown contract (SIGINT/SIGTERM
//     via the server's signal.NotifyContext).
//
// SECURITY (V7 / T-20-07/08): the bot token is read ONLY from
// os.Getenv(DISCORD_BOT_TOKEN) and is NEVER logged (no slog field carries it),
// never committed, never in the build. No MESSAGE_CONTENT intent is requested in
// Phase 20 (DM-send + guild membership only); that privileged intent is a
// Phase-22 toggle.
package bot

import (
	"context"
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
)

// Config is the Discord bot configuration, read from the process environment
// (the squirebot-server systemd EnvironmentFile, Plan 03). Token in particular
// NEVER reaches a log line. Enabled gates the whole gateway connection: with no
// token the server boots with the bot disconnected (a valid, supported state —
// the read/write API and scheduler run regardless).
type Config struct {
	Token   string
	GuildID string
	Enabled bool
}

// ConfigFromEnv reads the bot config from the environment. DISCORD_BOT_TOKEN is
// the gateway credential; DISCORD_GUILD_ID is the guild the bot serves. Enabled
// is derived: the bot is OFF unless a token is set, so a build/CI/local run with
// no token starts cleanly with the bot disconnected (never a fatal). The token
// value is read but NEVER logged (V7).
func ConfigFromEnv() Config {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	return Config{
		Token:   token,
		GuildID: os.Getenv("DISCORD_GUILD_ID"),
		Enabled: os.Getenv("DISCORD_BOT_TOKEN") != "",
	}
}

// Bot wraps the shared *discordgo.Session. A disabled bot (no token) carries a
// nil session — Session() returns nil and Connected() returns false, so notify
// and /healthz (Plan 03) degrade cleanly rather than panicking on a nil deref.
type Bot struct {
	session *discordgo.Session
}

// Session returns the shared *discordgo.Session (nil when the bot is disabled).
// notify (Plan 03 wiring) injects this into its Sender seam; a nil session means
// DM attempts surface a clean error rather than a panic (the caller guards nil).
func (b *Bot) Session() *discordgo.Session {
	return b.session
}

// Connected reports whether the gateway websocket has reached READY. It is the
// /healthz bot-state signal (Plan 03, Pitfall 7): a disabled or not-yet-ready
// bot returns false. discordgo flips session.DataReady true on the READY event
// and false on disconnect, so it tracks the live connection state.
func (b *Bot) Connected() bool {
	return b.session != nil && b.session.DataReady
}

// Start brings up the Discord gateway client NON-BLOCKING and NON-FATAL, then
// returns the shared *Bot for the caller (runServe, Plan 03) to wire into notify
// and /healthz. It clones the scheduler's grain: build the session, hand its
// lifetime to ctx, return immediately.
//
//   - Disabled (no token): returns (&Bot{}, nil) with a NIL session — the server
//     runs bot-less. No network is touched.
//   - Enabled: discordgo.New("Bot "+token); request ONLY the Guilds +
//     DirectMessages intents (DM-send + guild membership — NO MESSAGE_CONTENT in
//     Phase 20); register a Ready handler (recover-isolated) that logs the
//     connect; dg.Open() (non-blocking — it spawns the gateway goroutines). An
//     Open() error is RETURNED (the caller logs and continues — NON-FATAL), never
//     a panic/exit. A goroutine waits for the root context to cancel then
//     dg.Close() so the connection unwinds on shutdown (the scheduler ctx-tied
//     precedent).
//
// SECURITY: the token is used only to construct the session; it is NEVER logged.
func Start(ctx context.Context, cfg Config) (*Bot, error) {
	if !cfg.Enabled {
		slog.Info("bot disabled (no DISCORD_BOT_TOKEN); server running bot-less")
		return &Bot{}, nil
	}

	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		// NON-FATAL: return the error; the caller logs and continues bot-less.
		return &Bot{}, err
	}

	// Phase 20 needs only DM-send + guild membership. MESSAGE_CONTENT is a
	// privileged Phase-22 intent and is deliberately NOT requested here.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsDirectMessages

	// Ready handler: recover-isolated like every (future) gateway handler. Logs
	// the connect WITHOUT the token (V7). Phase 22/23 message handlers will each
	// `defer recoverBoundary("message_create")` the same way.
	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		defer recoverBoundary("ready")
		slog.Info("bot connected", "guild", cfg.GuildID)
	})

	if err := dg.Open(); err != nil {
		// NON-FATAL: the gateway couldn't connect (bad token / network). Return
		// the error; the caller logs and the server keeps running bot-less.
		return &Bot{}, err
	}

	// ctx-tied lifetime: close the gateway when the root context is cancelled
	// (SIGINT/SIGTERM). recover-isolated so a Close-path panic can't crash the
	// process (Pitfall 7).
	go func() {
		defer recoverBoundary("shutdown")
		<-ctx.Done()
		slog.Info("bot stopping", "reason", ctx.Err())
		if cerr := dg.Close(); cerr != nil {
			slog.Warn("bot close failed", "err", cerr)
		}
	}()

	return &Bot{session: dg}, nil
}

// recoverBoundary is the deferable panic firewall EVERY bot goroutine + (future
// P22/P23) gateway handler installs. A recovered panic is logged (op + the panic
// value, never a secret) and SWALLOWED so a bot-side panic NEVER propagates into
// the HTTP listener / scheduler / ingest (T-20-06, LOCKED — Pitfall 7).
//
// Usage: `defer recoverBoundary("message_create")` as the FIRST line of every
// handler/goroutine body.
func recoverBoundary(op string) {
	if r := recover(); r != nil {
		slog.Error("bot handler panic recovered", "op", op, "panic", r)
	}
}
