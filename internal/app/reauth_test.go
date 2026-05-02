package app

// Plan 02-04 Task 3 — auth-suspension flag + Reauthorize OAuth round-trip.
//
// Coverage focus:
//   - globalAuthSuspended starts false on a fresh process.
//   - The inventory + spellbook handlers short-circuit (no API call,
//     no parse, no Save) when authSuspended is true.
//   - Reauthorize refuses to run with no GoogleEmail (wizard not yet
//     completed) and returns an actionable error.
//   - Reauthorize honours its passed ctx cancellation: a context that
//     cancels before the user could ever click "Allow" returns the
//     ctx.Err() and leaves authSuspended unchanged (so the next click
//     still surfaces).
//
// Live OAuth round-trip (browser open → consent → /oauth/callback →
// wincred replace → tray green) cannot be unit-tested without a real
// Google account, so it's covered by Phase 2 final integration testing
// per the plan's <verification> live test.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/sheet"
	"github.com/boejowen/SquireBot/internal/tray"
)

// resetGlobalAuthSuspended ensures tests don't leak flag state between runs.
func resetGlobalAuthSuspended(t *testing.T) {
	t.Helper()
	globalAuthSuspended.Store(false)
	t.Cleanup(func() { globalAuthSuspended.Store(false) })
}

func TestGlobalAuthSuspended_StartsClear(t *testing.T) {
	resetGlobalAuthSuspended(t)
	if globalAuthSuspended.Load() {
		t.Fatal("globalAuthSuspended.Load() = true on fresh process; want false")
	}
}

// newStubSheetClient builds a *sheet.Client wired to an httptest server
// that counts incoming requests. The handler always returns 500 — the
// tests verify the handler is NEVER called when authSuspended is true.
func newStubSheetClient(t *testing.T) (*sheet.Client, *httptest.Server, *int64) {
	t.Helper()
	var calls int64
	srv := httptest.NewServer(httpHandlerCounting(&calls))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fake"})
	sc, err := sheet.NewClient(ctx, ts, "SHEET1",
		option.WithEndpoint(srv.URL),
		option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return sc, srv, &calls
}

// httpHandlerCounting returns a handler that increments *calls on every
// request and replies 500. Tests assert *calls stays at 0.
func httpHandlerCounting(calls *int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(calls, 1)
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"stub-should-not-be-called"}}`))
	})
}

// TestMakeOnInventoryChange_SkipsWhenSuspended verifies the inventory
// handler short-circuits the moment globalAuthSuspended is true: no
// stat, no parse, no WriteInventory call, no cfg.LastKnownInventoryMtime
// update.
func TestMakeOnInventoryChange_SkipsWhenSuspended(t *testing.T) {
	resetGlobalAuthSuspended(t)

	dir := t.TempDir()
	invPath := filepath.Join(dir, "Foo-Inventory.txt")
	if err := os.WriteFile(invPath, []byte("Location\tName\tID\tCount\tSlots\nbank\tBag\t1234\t1\t10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		GoogleEmail:             "user@example.com",
		EQFolders:               []string{dir},
		LastKnownInventoryMtime: map[string]string{},
	}
	bc := auth.BuildConstants{WatcherVersion: "test"}
	trayCtl := tray.NewController(tray.Config{})
	sc, _, calls := newStubSheetClient(t)

	// Trip the suspension flag BEFORE creating the handler — the handler
	// reads the flag on every fire.
	globalAuthSuspended.Store(true)

	handler := makeOnInventoryChange(context.Background(), sc, cfg, bc, trayCtl, &globalAuthSuspended)
	handler(invPath)

	if got := atomic.LoadInt64(calls); got != 0 {
		t.Errorf("stub HTTP calls = %d; want 0 (handler must skip when suspended)", got)
	}
	if v, ok := cfg.LastKnownInventoryMtime["Foo"]; ok {
		t.Errorf("LastKnownInventoryMtime[\"Foo\"] = %q; want absent (handler must skip)", v)
	}
}

// TestMakeOnSpellbookChange_SkipsWhenSuspended is the spellbook twin.
func TestMakeOnSpellbookChange_SkipsWhenSuspended(t *testing.T) {
	resetGlobalAuthSuspended(t)

	dir := t.TempDir()
	spbPath := filepath.Join(dir, "Foo-Spellbook.txt")
	if err := os.WriteFile(spbPath, []byte("Level\tName\n9\tLifetap\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		GoogleEmail:             "user@example.com",
		EQFolders:               []string{dir},
		LastKnownSpellbookMtime: map[string]string{},
	}
	bc := auth.BuildConstants{WatcherVersion: "test"}
	trayCtl := tray.NewController(tray.Config{})
	sc, _, calls := newStubSheetClient(t)

	globalAuthSuspended.Store(true)

	handler := makeOnSpellbookChange(context.Background(), sc, cfg, bc, trayCtl, &globalAuthSuspended)
	handler(spbPath)

	if got := atomic.LoadInt64(calls); got != 0 {
		t.Errorf("stub HTTP calls = %d; want 0 (handler must skip when suspended)", got)
	}
	if v, ok := cfg.LastKnownSpellbookMtime["Foo"]; ok {
		t.Errorf("LastKnownSpellbookMtime[\"Foo\"] = %q; want absent (handler must skip)", v)
	}
}

// TestReauthorize_NoGoogleEmailReturnsError covers the early-exit when
// the wizard hasn't run yet — the OAuth loopback flow has no email to
// re-confirm against.
func TestReauthorize_NoGoogleEmailReturnsError(t *testing.T) {
	resetGlobalAuthSuspended(t)

	cfg := &config.Config{}
	bc := auth.BuildConstants{
		OAuthClientID:     "x",
		OAuthClientSecret: "y",
		PickerAPIKey:      "z",
		GCPProjectNumber:  "1",
	}
	trayCtl := tray.NewController(tray.Config{})

	err := Reauthorize(context.Background(), cfg, bc, trayCtl, &globalAuthSuspended)
	if err == nil {
		t.Fatal("Reauthorize with empty GoogleEmail returned nil; want error")
	}
	if !strings.Contains(err.Error(), "GoogleEmail") {
		t.Errorf("Reauthorize error = %q; want it to mention GoogleEmail", err)
	}
}

// TestReauthorize_TimeoutLeavesSuspendedUnchanged covers the locked
// CONTEXT.md invariant: a failed re-auth (user closes browser, network
// error, etc.) does NOT silently un-suspend the watcher. Tray must stay
// red, menu item must stay visible — only a successful re-auth resumes
// writes.
func TestReauthorize_TimeoutLeavesSuspendedUnchanged(t *testing.T) {
	resetGlobalAuthSuspended(t)
	globalAuthSuspended.Store(true)

	cfg := &config.Config{GoogleEmail: "user@example.com"}
	bc := auth.BuildConstants{
		OAuthClientID:     "x",
		OAuthClientSecret: "y",
		PickerAPIKey:      "z",
		GCPProjectNumber:  "1",
	}
	trayCtl := tray.NewController(tray.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := Reauthorize(ctx, cfg, bc, trayCtl, &globalAuthSuspended)
	if err == nil {
		t.Fatal("Reauthorize with already-cancelled ctx returned nil; want error")
	}
	if !globalAuthSuspended.Load() {
		t.Error("globalAuthSuspended cleared on failed re-auth; want still true (no silent resume)")
	}
}

// TestRunReauthorize_SmokesWithoutPanic exercises the package-global
// wrapper that the tray.Config.OnReauthorize closure invokes. The
// underlying Reauthorize will fail fast (no GoogleEmail) — RunReauthorize
// must log + return without panicking, leaving the flag state alone so a
// later real-config run can succeed.
func TestRunReauthorize_SmokesWithoutPanic(t *testing.T) {
	resetGlobalAuthSuspended(t)
	globalAuthSuspended.Store(true)

	cfg := &config.Config{} // no email → underlying Reauthorize errors immediately
	bc := auth.BuildConstants{}
	trayCtl := tray.NewController(tray.Config{})

	// Should not panic, should not deadlock.
	RunReauthorize(context.Background(), cfg, bc, trayCtl)

	if !globalAuthSuspended.Load() {
		t.Error("globalAuthSuspended cleared by failed Reauthorize; want still true")
	}
}
