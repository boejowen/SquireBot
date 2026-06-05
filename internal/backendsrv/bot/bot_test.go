package bot

import (
	"context"
	"testing"
)

// bot_test.go covers the bot package's pure, no-network surface (Phase 20 Task
// 1): ConfigFromEnv's token-gated Enabled, Start's disabled no-op, and the
// recoverBoundary panic firewall. A live Open() is NOT exercised — there is no
// token in CI and the gateway must never be dialed from a unit test.

func TestConfigFromEnv_DisabledWithoutToken(t *testing.T) {
	// Ensure the var is unset for this test (t.Setenv auto-restores after).
	t.Setenv("DISCORD_BOT_TOKEN", "")
	cfg := ConfigFromEnv()
	if cfg.Enabled {
		t.Fatalf("Enabled = true with no token; want false (server must boot bot-less)")
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q; want empty", cfg.Token)
	}
}

func TestConfigFromEnv_EnabledWithToken(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "fake-token-value")
	t.Setenv("DISCORD_GUILD_ID", "guild-123")
	cfg := ConfigFromEnv()
	if !cfg.Enabled {
		t.Fatalf("Enabled = false with a token set; want true")
	}
	if cfg.Token != "fake-token-value" {
		t.Errorf("Token = %q; want the env value", cfg.Token)
	}
	if cfg.GuildID != "guild-123" {
		t.Errorf("GuildID = %q; want guild-123", cfg.GuildID)
	}
}

func TestStart_DisabledIsNoop(t *testing.T) {
	b, err := Start(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("Start(disabled) returned err %v; want nil (non-fatal, no network)", err)
	}
	if b == nil {
		t.Fatalf("Start(disabled) returned nil *Bot; want a non-nil bot-less Bot")
	}
	if b.Session() != nil {
		t.Errorf("disabled Bot has a non-nil session; want nil (no gateway dialed)")
	}
	if b.Connected() {
		t.Errorf("disabled Bot.Connected() = true; want false")
	}
}

func TestRecoverBoundary_SwallowsPanic(t *testing.T) {
	// A function that defers recoverBoundary and then panics must return
	// normally — the panic is recovered, the process survives (Pitfall 7).
	survived := func() (ok bool) {
		defer recoverBoundary("test_panic")
		panic("simulated bot handler explosion")
	}
	// If recoverBoundary did NOT recover, this call would crash the test binary.
	// Reaching the assertion at all proves the panic was swallowed.
	_ = survived()
	// Explicit positive signal:
	if got := survived(); got {
		t.Fatalf("survived() returned true; the named return was never set, want false")
	}
}
