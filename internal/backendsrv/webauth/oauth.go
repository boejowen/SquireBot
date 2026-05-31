// Package webauth is the SquireBot v2 human-session auth layer (Phase 15,
// AUTH-08/AUTH-09): a hand-rolled Discord OAuth2 authorization-code flow
// (golang.org/x/oauth2 — the 11-01 spike verdict, NOT PocketBase), an opaque
// server-side session backed by the 15-01 store (web_session, hash-only at
// rest), the session-gate middleware that walls the read API (D-01), and the
// login/callback/whoami-web/logout HTTP handlers. It is a SIBLING of the
// watcher's bearer guard (internal/backendsrv/auth) — the bearer path stays the
// ingest/whoami contract; this cookie path gates the human-facing read + write
// API.
//
// oauth.go holds the Discord-specific OAuth2 mechanics (D-02/D-04):
//   - the oauth2.Config builder (scopes = identify+guilds, exactly — D-02);
//   - GenerateState (crypto/rand, mirrors auth/mint.go) for the CSRF state param;
//   - AuthCodeURL / Exchange (server-side code exchange — the client secret is
//     BACKEND-ONLY, never in the static bundle, never logged);
//   - FetchIdentity (/users/@me → id/username/avatar — AUTH-09) and FetchGuilds
//     (/users/@me/guilds → the guild-id list), both bounded by io.LimitReader
//     (the politefetch discipline) so a runaway Discord response can't OOM the VPS;
//   - IsGuildMember — fail-closed membership = the configured guild id present in
//     the user's guilds list (no bot, no allowlist — AUTH-08).
//
// SECURITY (T-15-09 / V7): the Discord client secret crosses ONLY into the
// oauth2.Config built here from env (ConfigFromEnv) and is used ONLY in the
// server-side token exchange. It is NEVER logged (no slog of the secret/token)
// and NEVER returned to a client.
package webauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
)

// Config is the Discord OAuth2 application configuration. All four values are
// backend-only env vars (D-04 / §10 maintainer prerequisite); ClientSecret in
// particular NEVER reaches the static frontend bundle and is NEVER logged.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	GuildID      string
}

// ConfigFromEnv reads the Discord OAuth2 config from the process environment
// (set on the box via the squirebot-server systemd unit — root-only
// EnvironmentFile, chmod 600). The four vars are the ones the maintainer
// provisions in the Discord Developer Portal (§10):
//
//	DISCORD_CLIENT_ID, DISCORD_CLIENT_SECRET, DISCORD_GUILD_ID, DISCORD_REDIRECT_URI
//
// Empty values are tolerated here (the server still starts; the OAuth handlers
// simply can't complete a login) so a build/CI/local run without the secret is
// fine — the secret is required only for the live login, deferred to deploy.
func ConfigFromEnv() Config {
	return Config{
		ClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("DISCORD_REDIRECT_URI"),
		GuildID:      os.Getenv("DISCORD_GUILD_ID"),
	}
}

// discordAuthURL / discordTokenURL are the Discord OAuth2 endpoints (D-02). They
// are package vars (not consts) ONLY so oauth_test.go can repoint them at an
// httptest token server via setOAuthEndpointsForTest; production never mutates
// them. The token URL is the /api/oauth2/token form (Discord requires the
// client_secret in the body even for confidential clients — fine, it's
// backend-only).
var (
	discordAuthURL  = "https://discord.com/oauth2/authorize"
	discordTokenURL = "https://discord.com/api/oauth2/token"
)

// discordAPIBase is the base URL for the identity + guilds REST calls. A package
// var ONLY so the test can point it at an httptest server (setDiscordAPIBaseForTest);
// production is always the real Discord API base.
var discordAPIBase = "https://discord.com/api"

// discordScopes are the EXACTLY two locked scopes (D-02): identify (→ /users/@me
// id/username/avatar) and guilds (→ /users/@me/guilds membership). No `email`,
// no bot, no `guilds.members.read`.
var discordScopes = []string{"identify", "guilds"}

// identityClient bounds a single Discord identity/guilds call (30s timeout, TLS
// verification ON via the Go default — no custom tls.Config). Mirrors
// politefetch's shared client discipline.
var identityClient = &http.Client{Timeout: 30 * time.Second}

// maxDiscordResponseBytes caps the identity/guilds body read (1 MB — these
// responses are tiny) so a runaway/hostile response can't OOM the small VPS
// (mirrors politefetch.maxResponseBytes + the ingest MaxBytesReader discipline).
const maxDiscordResponseBytes int64 = 1 << 20

// oauthConfig builds the per-call *oauth2.Config from the Discord Config and the
// (possibly test-overridden) endpoint URLs. Building it on demand keeps the
// endpoints a simple package-var seam and avoids a package-level mutable config.
func oauthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       discordScopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  discordAuthURL,
			TokenURL: discordTokenURL,
		},
	}
}

// GenerateState returns a fresh high-entropy OAuth CSRF state token: 32 bytes
// from crypto/rand (NOT math/rand — mirrors auth/mint.go), hex-encoded. The
// caller stores it in a short-lived httpOnly cookie and verifies equality on the
// callback (T-15-06 OAuth CSRF guard).
func GenerateState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil { // crypto/rand (NOT math/rand)
		return "", fmt.Errorf("generate oauth state entropy: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// AuthCodeURL returns the Discord authorize URL the login handler 302-redirects
// to: response_type=code, client_id, redirect_uri, scope=identify+guilds, and
// the supplied CSRF state.
func AuthCodeURL(cfg Config, state string) string {
	return oauthConfig(cfg).AuthCodeURL(state)
}

// Exchange performs the server-side authorization-code → token exchange against
// the Discord token endpoint (the client secret rides the request body; Discord
// requires it). The secret never leaves this server-side call.
func Exchange(ctx context.Context, cfg Config, code string) (*oauth2.Token, error) {
	tok, err := oauthConfig(cfg).Exchange(ctx, code)
	if err != nil {
		// NB: do NOT %w-wrap-and-log the secret; the oauth2 error never contains
		// it, and we never slog this error with the token/secret attached.
		return nil, fmt.Errorf("discord code exchange failed: %w", err)
	}
	return tok, nil
}

// discordUser is the decoded /users/@me shape we capture (AUTH-09 / D-03). The
// snowflake id is the stable key + the deferred v2-pinger DM handle; avatar is a
// hash (or "") the frontend turns into a CDN URL.
type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// partialGuild is one element of /users/@me/guilds — we need only the id for the
// membership test.
type partialGuild struct {
	ID string `json:"id"`
}

// FetchIdentity calls GET {discordAPIBase}/users/@me with the bearer access
// token and returns the user's snowflake id, username, and avatar hash
// (AUTH-09). The body read is bounded by io.LimitReader (politefetch discipline).
func FetchIdentity(ctx context.Context, cfg Config, tok *oauth2.Token) (id, username, avatar string, err error) {
	var u discordUser
	if err := getDiscordJSON(ctx, tok, discordAPIBase+"/users/@me", &u); err != nil {
		return "", "", "", fmt.Errorf("fetch discord identity: %w", err)
	}
	return u.ID, u.Username, u.Avatar, nil
}

// FetchGuilds calls GET {discordAPIBase}/users/@me/guilds with the bearer access
// token and returns the list of guild snowflake ids the user belongs to. The
// body read is bounded by io.LimitReader.
func FetchGuilds(ctx context.Context, cfg Config, tok *oauth2.Token) ([]string, error) {
	var guilds []partialGuild
	if err := getDiscordJSON(ctx, tok, discordAPIBase+"/users/@me/guilds", &guilds); err != nil {
		return nil, fmt.Errorf("fetch discord guilds: %w", err)
	}
	ids := make([]string, 0, len(guilds))
	for _, g := range guilds {
		ids = append(ids, g.ID)
	}
	return ids, nil
}

// IsGuildMember reports whether the configured guild id is present in the user's
// guild-id list — the AUTH-08 membership boundary. FAIL-CLOSED: an empty
// configured id or an empty/nil list yields false (no membership, no session).
func IsGuildMember(guildIDs []string, configured string) bool {
	if configured == "" || len(guildIDs) == 0 {
		return false // fail-closed
	}
	for _, id := range guildIDs {
		if id == configured {
			return true
		}
	}
	return false
}

// getDiscordJSON performs a bounded, bearer-authenticated GET and decodes the
// JSON body into dst. It injects Authorization: Bearer <access_token> directly
// (the access token, not the secret), bounds the body with io.LimitReader, and
// treats any non-200 as an error (a 429 surfaces as an error the interactive
// caller turns into a "try again" — no retry loop on the login path). The token
// is NEVER logged (V7).
func getDiscordJSON(ctx context.Context, tok *oauth2.Token, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build discord request: %w", err)
	}
	req.Header.Set("Authorization", tok.Type()+" "+tok.AccessToken)
	resp, err := identityClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord request transport error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discord API returned HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDiscordResponseBytes))
	if err != nil {
		return fmt.Errorf("read discord response body: %w", err)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode discord response: %w", err)
	}
	return nil
}

// setOAuthEndpointsForTest repoints the authorize + token endpoint URLs (used by
// oauth_test.go to aim Exchange at an httptest token server). It returns a
// restore func. TEST-ONLY — production never calls it.
func setOAuthEndpointsForTest(authURL, tokenURL string) func() {
	prevAuth, prevToken := discordAuthURL, discordTokenURL
	discordAuthURL, discordTokenURL = authURL, tokenURL
	return func() { discordAuthURL, discordTokenURL = prevAuth, prevToken }
}

// setDiscordAPIBaseForTest repoints the identity/guilds API base (used by
// oauth_test.go to aim FetchIdentity/FetchGuilds at an httptest server). It
// returns a restore func. TEST-ONLY.
func setDiscordAPIBaseForTest(base string) func() {
	prev := discordAPIBase
	discordAPIBase = base
	return func() { discordAPIBase = prev }
}
