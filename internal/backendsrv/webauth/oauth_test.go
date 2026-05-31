package webauth

// oauth_test.go drives the Discord OAuth2 helpers (Task 1, D-02/D-04) entirely
// against httptest fakes — NO live Discord credentials, NO network. It proves:
//   - AuthCodeURL carries client_id + redirect_uri + response_type=code + the
//     two locked scopes (identify, guilds) + the CSRF state;
//   - Exchange hits the (overridable) token endpoint and returns the access token;
//   - FetchIdentity decodes {id, username, avatar} from /users/@me;
//   - FetchGuilds decodes the guild-id list from /users/@me/guilds;
//   - IsGuildMember is fail-closed: present → true, absent → false, empty → false
//     (the AUTH-08 refusal path).
//
// The seams used here (and ONLY here, restored via t.Cleanup): the oauth2
// endpoint URLs are pointed at the httptest token server, and discordAPIBase is
// pointed at the httptest identity/guilds server.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

// testConfig builds a Config with dummy (non-secret) credentials for the helper
// tests. The "secret" here is a literal test placeholder, never a real value.
func testConfig() Config {
	return Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret-not-real",
		RedirectURI:  "https://api.squirebot.quest/api/v1/auth/callback",
		GuildID:      "111111111111111111",
	}
}

func TestAuthCodeURL_CarriesScopesAndState(t *testing.T) {
	cfg := testConfig()
	got := AuthCodeURL(cfg, "the-state-token")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("AuthCodeURL produced an unparseable URL %q: %v", got, err)
	}
	if u.Host != "discord.com" {
		t.Fatalf("authorize host = %q, want discord.com", u.Host)
	}
	q := u.Query()
	if q.Get("client_id") != cfg.ClientID {
		t.Errorf("client_id = %q, want %q", q.Get("client_id"), cfg.ClientID)
	}
	if q.Get("redirect_uri") != cfg.RedirectURI {
		t.Errorf("redirect_uri = %q, want %q", q.Get("redirect_uri"), cfg.RedirectURI)
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want code", q.Get("response_type"))
	}
	if q.Get("state") != "the-state-token" {
		t.Errorf("state = %q, want the-state-token", q.Get("state"))
	}
	scope := q.Get("scope")
	if !strings.Contains(scope, "identify") || !strings.Contains(scope, "guilds") {
		t.Errorf("scope = %q, want both identify and guilds", scope)
	}
}

func TestExchange_AgainstFakeTokenEndpoint(t *testing.T) {
	// Fake Discord token endpoint: accepts the code, returns an access token.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token endpoint: parse form: %v", err)
		}
		if r.Form.Get("code") != "the-auth-code" {
			t.Errorf("token endpoint code = %q, want the-auth-code", r.Form.Get("code"))
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("token endpoint grant_type = %q, want authorization_code", r.Form.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-access-token","token_type":"Bearer","expires_in":604800}`))
	}))
	defer tokenSrv.Close()

	restore := setOAuthEndpointsForTest("https://discord.com/oauth2/authorize", tokenSrv.URL)
	defer restore()

	tok, err := Exchange(context.Background(), testConfig(), "the-auth-code")
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}
	if tok.AccessToken != "fake-access-token" {
		t.Fatalf("access token = %q, want fake-access-token", tok.AccessToken)
	}
}

func TestFetchIdentity_AndGuilds_AgainstFakeAPI(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer fake-access-token" {
			t.Errorf("Authorization = %q, want Bearer fake-access-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/@me":
			_, _ = w.Write([]byte(`{"id":"222222222222222222","username":"Slampeach","avatar":"abc123"}`))
		case "/users/@me/guilds":
			_, _ = w.Write([]byte(`[{"id":"111111111111111111","name":"The Guild"},{"id":"999"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	restore := setDiscordAPIBaseForTest(apiSrv.URL)
	defer restore()

	tok := &oauth2.Token{AccessToken: "fake-access-token", TokenType: "Bearer"}

	id, username, avatar, err := FetchIdentity(context.Background(), testConfig(), tok)
	if err != nil {
		t.Fatalf("FetchIdentity error: %v", err)
	}
	if id != "222222222222222222" {
		t.Errorf("id = %q, want 222222222222222222", id)
	}
	if username != "Slampeach" {
		t.Errorf("username = %q, want Slampeach", username)
	}
	if avatar != "abc123" {
		t.Errorf("avatar = %q, want abc123", avatar)
	}

	guilds, err := FetchGuilds(context.Background(), testConfig(), tok)
	if err != nil {
		t.Fatalf("FetchGuilds error: %v", err)
	}
	if len(guilds) != 2 || guilds[0] != "111111111111111111" {
		t.Fatalf("guilds = %v, want [111111111111111111 999]", guilds)
	}
}

func TestIsGuildMember_Table(t *testing.T) {
	tests := []struct {
		name       string
		guildIDs   []string
		configured string
		want       bool
	}{
		{"present → member", []string{"a", "111", "b"}, "111", true},
		{"absent → not member (AUTH-08 refusal)", []string{"a", "b", "c"}, "111", false},
		{"empty list → not member (fail-closed)", []string{}, "111", false},
		{"nil list → not member (fail-closed)", nil, "111", false},
		{"empty configured → not member (fail-closed)", []string{"a", "111"}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsGuildMember(tc.guildIDs, tc.configured); got != tc.want {
				t.Errorf("IsGuildMember(%v, %q) = %v, want %v", tc.guildIDs, tc.configured, got, tc.want)
			}
		})
	}
}
