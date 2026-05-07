// Package app is the load-bearing Phase 1 orchestrator. Plans 01-06
// produce isolated packages; package app composes them into a runnable
// pipeline:
//
//	cold start → config.Load
//	  ├─ if needsWizard → wizard.Server.Run → returns email + spreadsheet + folder + TokenSource
//	  └─ else           → auth.ReadToken → OAuthConfigForRefresh → ReuseTokenSource
//	then sheet.NewClient + ValidateWorkbook
//	then watch.Run with onChange = parse.Parse → sheet.WriteInventory → sheet.UpsertCharOwner
//
// D-04 ChangeWorkbook is a separate entry-point fired from the tray's
// Change Workbook… menu item. It re-runs picker.Server on a fresh
// loopback listener with the existing wincred-backed TokenSource (no
// OAuth re-prompt) and persists the new spreadsheetID on success.
//
// File ownership: Plan 03 ships auth.ReadToken / auth.OAuthConfigForRefresh /
// auth.NewManagerWithListener; this file consumes them by import only and
// does NOT modify internal/auth/oauth.go.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/oauth2"

	"github.com/boejowen/SquireBot/internal/auth"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/heartbeat"
	"github.com/boejowen/SquireBot/internal/parse"
	"github.com/boejowen/SquireBot/internal/picker"
	"github.com/boejowen/SquireBot/internal/scaffold"
	"github.com/boejowen/SquireBot/internal/sheet"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/update"
	"github.com/boejowen/SquireBot/internal/watch"
	"github.com/boejowen/SquireBot/internal/wizard"
)

// charNameRE extracts <Char> from "<Char>-Inventory.txt". Plan 02-02 Task 5
// adds a parallel charNameSpellbookRE for "<Char>-Spellbook.txt".
var charNameRE = regexp.MustCompile(`^(.+)-Inventory\.txt$`)

// charNameSpellbookRE extracts <Char> from "<Char>-Spellbook.txt".
// Mirrors charNameRE for the WATCH-02 spellbook handler.
var charNameSpellbookRE = regexp.MustCompile(`^(.+)-Spellbook\.txt$`)

// changeWorkbookTimeout is how long ChangeWorkbook waits for the user
// to complete a pick before it gives up and tears down the listener.
const changeWorkbookTimeout = 5 * time.Minute

// RunApp is the background goroutine launched from main.go. Blocks
// until ctx is cancelled. Branches on config completeness:
//   - incomplete config → wizard, then watcher
//   - complete config   → re-load wincred token + watcher directly
//
// Tray Continue setup… invokes RunApp again; D-04 Change Workbook…
// invokes ChangeWorkbook (NOT RunApp).
func RunApp(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
	if err := bc.Validate(); err != nil {
		slog.Error("build constants missing", "err", err)
		t.SetStatus("Build error: missing OAuth constants")
		t.SetIconHealth(tray.HealthRed)
		return
	}

	var ts oauth2.TokenSource

	if needsWizard(cfg) {
		t.SetStatus("Setup needed")
		t.SetIconHealth(tray.HealthRed)
		t.ShowContinueSetup()

		ws := wizard.NewServer(cfg, bc)
		res := ws.Run(ctx)
		if res.Err != nil {
			slog.Error("wizard failed", "err", res.Err)
			t.SetStatus(fmt.Sprintf("Setup error: %v", res.Err))
			// Fall through; tray's Continue setup… click triggers a fresh
			// RunApp invocation (main.go wiring).
			return
		}
		t.HideContinueSetup()
		ts = res.TokenSource
		// Wizard already wrote cfg.GoogleEmail (auth.Manager) +
		// cfg.SpreadsheetID (picker.Server) + cfg.EQFolder (handleEQFolderConfirm).
	}

	// Watcher path. If we came through the wizard, ts is live; otherwise
	// (skip-wizard cold start) we rebuild it from wincred.
	if ts == nil {
		built, err := buildTokenSourceFromWincred(ctx, cfg, bc)
		if err != nil {
			slog.Error("token rebuild from wincred failed", "err", err)
			t.SetStatus(fmt.Sprintf("Auth error: %v", err))
			t.SetIconHealth(tray.HealthRed)
			t.ShowContinueSetup()
			return
		}
		ts = built
	}

	if err := runWatcher(ctx, cfg, bc, t, ts); err != nil {
		slog.Error("watcher exited", "err", err)
		t.SetStatus(fmt.Sprintf("Watcher error: %v", err))
		t.SetIconHealth(tray.HealthRed)
	}
}

// needsWizard reports whether any of the three config values RunApp
// needs is missing. Used by both RunApp and the tray's startup flow.
//
// Plan 02-02 (WATCH-03): folder presence is satisfied by EITHER the legacy
// single-string EQFolder OR the new EQFolders slice; either is enough.
func needsWizard(cfg *config.Config) bool {
	if cfg.GoogleEmail == "" || cfg.SpreadsheetID == "" {
		return true
	}
	return cfg.EQFolder == "" && len(cfg.EQFolders) == 0
}

// swappableTS is an oauth2.TokenSource whose underlying source can be
// replaced while the sheet client is running. onRefresh swaps it when
// Reauthorize has stored a new refresh token in wincred: the old
// ReuseTokenSource cached access token is then stale even though it has
// not expired by wall-clock time, so simply calling ts.Token() returns
// the revoked token from cache and the retry 401s again.
type swappableTS struct {
	mu  sync.Mutex
	cur oauth2.TokenSource
}

func (s *swappableTS) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	cur := s.cur
	s.mu.Unlock()
	return cur.Token()
}

func (s *swappableTS) swap(newTS oauth2.TokenSource) {
	s.mu.Lock()
	s.cur = newTS
	s.mu.Unlock()
}

// buildTokenSourceFromWincred reconstructs an oauth2.TokenSource from
// the wincred-stored refresh token. Used on the skip-wizard cold-start
// path and by ChangeWorkbook. Plan 03's OAuthConfigForRefresh keeps the
// scope set in lockstep with consent-time so the refresh succeeds.
func buildTokenSourceFromWincred(ctx context.Context, cfg *config.Config, bc auth.BuildConstants) (oauth2.TokenSource, error) {
	if cfg.GoogleEmail == "" {
		return nil, fmt.Errorf("no GoogleEmail in config — wizard not yet run")
	}
	st, err := auth.ReadToken(cfg.GoogleEmail)
	if err != nil {
		return nil, fmt.Errorf("read wincred token for %s: %w", cfg.GoogleEmail, err)
	}
	// Prefer the build-time client ID (current Cloud project) over the
	// stored ClientID — survives a Cloud project migration without
	// invalidating the wincred entry. Plan 03's StoredToken.ClientID is
	// retained for diagnostics, not for auth.
	clientID := bc.OAuthClientID
	if clientID == "" {
		clientID = st.ClientID
	}
	oauthCfg := auth.OAuthConfigForRefresh(auth.Config{
		OAuthClientID:     clientID,
		OAuthClientSecret: bc.OAuthClientSecret, // Google requires client_secret on refresh exchanges for desktop apps
	})
	tok := &oauth2.Token{RefreshToken: st.RefreshToken}
	ts := oauth2.ReuseTokenSource(tok, oauthCfg.TokenSource(ctx, tok))
	// Defer-zero our local view of the refresh-token bytes so a later
	// panic / log.Printf cannot leak them. ReuseTokenSource already holds
	// its own copy internally.
	st.RefreshToken = ""
	tok.RefreshToken = ""
	_ = st
	return ts, nil
}

// runWatcher starts the watcher loop and dispatches parse → write →
// upsert per inventory or spellbook event. Returns when ctx is cancelled
// or fsnotify errors fatally.
//
// Plan 02-02 (WATCH-02 + WATCH-03 + WATCH-09): spans every folder in
// cfg.EQFolders (back-compat-shim'd from cfg.EQFolder by config.Load),
// dispatches inventory + spellbook events to separate handlers, and
// runs a startup catch-up scan so files saved while the watcher was
// off are uploaded on the next launch.
func runWatcher(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller, ts oauth2.TokenSource) error {
	sts := &swappableTS{cur: ts}
	sc, err := sheet.NewClient(ctx, sts, cfg.SpreadsheetID)
	if err != nil {
		return fmt.Errorf("sheet client: %w", err)
	}
	// Plan 02-04 (AUTH-05): onRefresh rebuilds the token source on every call.
	// When called after Reauthorize, it drains globalReauthTSCh to get the
	// picker's freshTS (AT2a). After swapping sts, it probes write access in
	// a loop: drive.file batchUpdate returns 401 for 8–25 minutes after picker
	// registration while Google propagates the new grant. The probe sleeps here
	// inside batchMu; that is safe because authSuspended=false causes watcher
	// events to queue on batchMu (not skip), and heartbeat checks
	// globalAuthSuspended (skips when true) — but here it's false, so heartbeat
	// may block on batchMu briefly per 60s probe; the 24h fire rate makes that
	// negligible.
	sc.SetOnRefresh(func() error {
		var freshTS oauth2.TokenSource
		fromReauth := false
		select {
		case freshTS = <-globalReauthTSCh:
			slog.Info("onRefresh: using post-reauth token source")
			fromReauth = true
		default:
			var err error
			freshTS, err = buildTokenSourceFromWincred(ctx, cfg, bc)
			if err != nil {
				return err
			}
		}
		tok, err := freshTS.Token()
		if err != nil {
			return err
		}
		slog.Info("onRefresh: fresh token obtained", "expiry", tok.Expiry)
		sts.swap(freshTS)
		if !fromReauth {
			return nil // non-reauth refresh; no propagation delay expected
		}
		// Probe write access until Google propagates the new grant or timeout.
		const probeInterval = 60 * time.Second
		const probeMaxWait = 25 * time.Minute
		probeDeadline := time.Now().Add(probeMaxWait)
		for attempt := 1; ; attempt++ {
			if perr := sc.PingWriteNoLock(ctx); !errors.Is(perr, sheet.ErrPermanentAuth) {
				if perr == nil {
					slog.Info("onRefresh: write probe succeeded", "attempt", attempt)
				} else {
					slog.Warn("onRefresh: write probe non-auth error; proceeding", "err", perr)
				}
				return nil
			}
			if time.Now().After(probeDeadline) {
				break
			}
			slog.Info("onRefresh: write probe 401 — drive.file propagating",
				"attempt", attempt, "retry_in", probeInterval)
			t.SetStatus(fmt.Sprintf("Reauthorized: waiting for Google propagation (attempt %d)…", attempt))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(probeInterval):
			}
		}
		slog.Error("onRefresh: drive.file write access did not propagate", "after", probeMaxWait)
		return sheet.ErrPermanentAuth
	})

	// Plan 02-01 Task 1: ValidateWorkbook returns one of three states.
	// Wrong → refuse. SchemaTooNew (any state with that error) → refuse.
	// Empty or Matches → proceed to ScaffoldSchemaV1.
	state, vErr := sc.ValidateWorkbook(ctx)
	if errors.Is(vErr, sheet.ErrSchemaTooNew) {
		return fmt.Errorf("validate workbook on startup: %w", vErr)
	}
	if state == sheet.WorkbookStateWrong {
		return fmt.Errorf("validate workbook on startup: %w", vErr)
	}
	// state is Empty or Matches — both proceed to scaffold.

	// Plan 02-01 Task 4: ScaffoldSchemaV1 brings the workbook to v1.
	// Idempotent — no-op on second run. Fresh shared workbook (Empty)
	// gets every dimension + view tab created with locked headers and
	// 13 _meta KV rows including schema_version=1 + canonical_id.
	if err := scaffold.ScaffoldSchemaV1(ctx, sc); err != nil {
		return fmt.Errorf("scaffold schema v1: %w", err)
	}

	// Plan 02-02 (WATCH-03): determine the folders to watch. Prefer
	// cfg.EQFolders (Phase 2 multi-folder); fall back to the legacy
	// cfg.EQFolder for any pre-Phase-2 config that hasn't been re-saved
	// yet. config.Load already shims EQFolder→EQFolders on read, so
	// this fallback is belt-and-braces.
	folders := cfg.EQFolders
	if len(folders) == 0 && cfg.EQFolder != "" {
		folders = []string{cfg.EQFolder}
	}
	if len(folders) == 0 {
		return fmt.Errorf("no EQ folders configured (cfg.EQFolders empty)")
	}

	t.SetSpreadsheetID(cfg.SpreadsheetID)
	t.SetIconHealth(tray.HealthGreen)
	t.SetStatus(fmt.Sprintf("Connected as %s — watching %s", cfg.GoogleEmail, strings.Join(folders, ", ")))

	// Plan 02-05 (WATCH-08 + OPS-05): launch the heartbeat goroutine.
	// Fires immediately, then every 24h. Goes through the same
	// mutex-funneled c.batchUpdate as the watcher writes, so heartbeat
	// fires cannot interleave with WriteInventory / WriteSpellbook.
	// Honors globalAuthSuspended (skips the API call when true so we
	// don't burn quota on doomed requests during a Reauthorize wait).
	go heartbeat.Run(ctx, sc, cfg, cfg.GoogleEmail, bc.WatcherVersion, &globalAuthSuspended)

	// Plan 02-06 (OPS-04): launch the auto-update daily-check goroutine
	// alongside the heartbeat. Independent goroutines: heartbeat goes
	// through Sheets API (mutex-funneled batchUpdate); auto-update goes
	// through direct net/http to GitHub Releases. No coordination needed.
	// On a successful staging the statusFn updates the tray status; the
	// startup-swap (cmd/squirebot/main.go update.Apply) takes effect on
	// the next launch. Owner/repo are hard-coded to the canonical
	// repository — Phase 5+ may switch to ldflag injection if forking
	// becomes a real concern (CONTEXT.md Claude's Discretion).
	if exe, err := os.Executable(); err != nil {
		slog.Warn("os.Executable failed; auto-update goroutine not launched", "err", err)
	} else {
		go update.RunDailyCheck(ctx, "boejowen", "SquireBot", bc.WatcherVersion, exe, func(msg string) { t.SetStatus(msg) })
	}

	// Plan 02-04 (AUTH-05): pass &globalAuthSuspended through to the
	// handlers. They will Store(true) on permanent auth failure and
	// short-circuit subsequent fires until Reauthorize clears the flag.
	onInventory := makeOnInventoryChange(ctx, sc, cfg, bc, t, &globalAuthSuspended)
	onSpellbook := makeOnSpellbookChange(ctx, sc, cfg, bc, t, &globalAuthSuspended)

	// Plan 02-02 (WATCH-09): on startup, walk every folder and synthesize
	// onInventory / onSpellbook calls for any file whose mtime is newer
	// than the cached LastKnown*Mtime. A guildie who runs SquireBot 5
	// minutes a day no longer loses snapshots produced while the watcher
	// was off. Idempotent — re-running with no file changes is a no-op.
	rescanCatchUp(ctx, cfg, folders, onInventory, onSpellbook)

	return watch.Run(ctx, folders, onInventory, onSpellbook)
}

// rescanCatchUp walks every folder in `folders`, lists every
// <Char>-Inventory.txt and <Char>-Spellbook.txt, compares mtime against
// cfg.LastKnownInventoryMtime[char] / cfg.LastKnownSpellbookMtime[char],
// and synthesizes an onInventory / onSpellbook call for each newer file.
//
// Per Plan 02-02 Task 5: the OnChange callbacks themselves persist the
// updated mtime once a sheet write succeeds (see makeOnInventoryChange /
// makeOnSpellbookChange). rescanCatchUp does NOT update the maps itself —
// this avoids the false-positive "already uploaded" state if a transient
// sheet failure happens during catch-up but accepts a re-upload on a
// clean restart after a partial failure (idempotent re-uploads are cheap).
//
// ctx is forwarded to the callbacks via closure. The function does not
// block on the callbacks — they run synchronously here on the catch-up
// goroutine; if a callback blocks, catch-up blocks. Acceptable because
// catch-up runs once at startup and the callbacks are bounded by sheet
// API latency.
func rescanCatchUp(ctx context.Context, cfg *config.Config, folders []string, onInventory, onSpellbook watch.OnChange) {
	if ctx.Err() != nil {
		return
	}
	for _, folder := range folders {
		entries, err := os.ReadDir(folder)
		if err != nil {
			slog.Warn("catch-up: read folder", "folder", folder, "err", err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			var charName, suffix string
			var lastMap map[string]string
			var cb watch.OnChange
			switch {
			case strings.HasSuffix(name, watch.InventorySuffix):
				suffix = watch.InventorySuffix
				lastMap = cfg.LastKnownInventoryMtime
				cb = onInventory
			case strings.HasSuffix(name, watch.SpellbookSuffix):
				suffix = watch.SpellbookSuffix
				lastMap = cfg.LastKnownSpellbookMtime
				cb = onSpellbook
			default:
				continue
			}
			charName = strings.TrimSuffix(name, suffix)
			info, err := e.Info()
			if err != nil {
				continue
			}
			currentMtime := info.ModTime().UTC().Format(time.RFC3339)
			prevMtime := ""
			if lastMap != nil {
				prevMtime = lastMap[charName]
			}
			if currentMtime == prevMtime {
				continue // already uploaded the latest version
			}
			slog.Info("catch-up upload",
				"folder", folder, "char", charName, "type", suffix,
				"prev_mtime", prevMtime, "current_mtime", currentMtime)
			cb(filepath.Join(folder, name))
		}
	}
}

// makeOnInventoryChange wraps the parse → WriteInventory → UpsertCharOwner
// chain into a watch.OnChange callback. Extracted for testability.
//
// Plan 02-02 (WATCH-09): on a successful write, the file's source mtime
// (captured before parse) is persisted to cfg.LastKnownInventoryMtime[char]
// + cfg.Save() so a watcher restart's catch-up scan correctly recognises
// "already uploaded" without re-firing.
func makeOnInventoryChange(ctx context.Context, sc *sheet.Client, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller, authSuspended *atomic.Bool) watch.OnChange {
	return func(path string) {
		// Plan 02-04 (AUTH-05): if the watcher is suspended (refresh
		// token died, awaiting Reauthorize click), skip BEFORE the
		// stat/parse/write chain. CONTEXT.md (locked): no silent
		// retry-loop after invalid_grant.
		if authSuspended != nil && authSuspended.Load() {
			slog.Info("auth suspended; skipping inventory", "path", filepath.Base(path))
			return
		}
		charName := extractCharNameForSuffix(path, watch.InventorySuffix)
		if charName == "" {
			slog.Warn("inventory file with unexpected name; skipping",
				"path", filepath.Base(path))
			return
		}
		// Per CLAUDE.md / RESEARCH.md §8.3: re-stat + re-read fresh on
		// every event. Never trust fsnotify event payloads on Windows.
		// Capture mtime BEFORE parse so a same-second re-fire after this
		// upload is recognised as "already uploaded" by catch-up.
		fi, statErr := os.Stat(path)
		if statErr != nil {
			slog.Error("stat inventory", "char", charName, "err", statErr)
			return
		}
		fileMtime := fi.ModTime().UTC().Format(time.RFC3339)

		f, err := os.Open(path)
		if err != nil {
			slog.Error("open inventory", "char", charName, "err", err)
			return
		}
		rows, perr := parse.Parse(f)
		_ = f.Close()
		if perr != nil {
			slog.Error("parse inventory", "char", charName, "err", perr)
			return
		}
		if len(rows) == 0 {
			// T-07-05 mitigation: don't WriteInventory with 0 rows (would
			// clear the tab). EQ flush mid-write or empty char → skip.
			slog.Info("inventory empty; skipping write", "char", charName)
			return
		}
		uploadedAt := time.Now().UTC().Format(time.RFC3339)
		if err := sc.WriteInventory(ctx, charName, sheet.InventoryHeader, rows, uploadedAt); err != nil {
			// Plan 02-04 (AUTH-05): permanent auth failure → suspend
			// writes, surface tray red + Reauthorize menu, return WITHOUT
			// upserting char_owner (we are not actually writing).
			if isPermanentAuthErr(err) {
				suspendForAuth(authSuspended, t, charName, "inventory", err)
				return
			}
			slog.Error("write inventory", "char", charName, "err", err)
			t.SetStatus(fmt.Sprintf("Last upload failed: %s", charName))
			return
		}
		if err := sc.UpsertCharOwner(ctx, charName, cfg.GoogleEmail, bc.WatcherVersion); err != nil {
			// Non-fatal — inv:Char write succeeded; surface warning, continue.
			slog.Warn("upsert char_owner", "char", charName, "err", err)
		}
		// WATCH-09: persist the mtime so the next catch-up sees it.
		if cfg.LastKnownInventoryMtime == nil {
			cfg.LastKnownInventoryMtime = make(map[string]string)
		}
		cfg.LastKnownInventoryMtime[charName] = fileMtime
		if err := cfg.Save(); err != nil {
			slog.Warn("save cfg after inventory upload", "char", charName, "err", err)
		}
		slog.Info("uploaded", "char", charName, "rows", len(rows))
		t.SetStatus(fmt.Sprintf("Last upload: %s at %s", charName, time.Now().Format("15:04")))
	}
}

// makeOnSpellbookChange wraps the parse → WriteSpellbook → UpsertCharOwner
// chain into a watch.OnChange callback. Mirrors makeOnInventoryChange but
// for <Char>-Spellbook.txt files.
//
// Plan 02-02 (WATCH-02): UpsertCharOwner runs after spellbook events too,
// so last_seen is refreshed on every signal of life — Plan 02-01 Task 3
// already extended UpsertCharOwner to refresh last_seen on every match.
func makeOnSpellbookChange(ctx context.Context, sc *sheet.Client, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller, authSuspended *atomic.Bool) watch.OnChange {
	return func(path string) {
		// Plan 02-04 (AUTH-05): mirrors makeOnInventoryChange. Skip when
		// suspended.
		if authSuspended != nil && authSuspended.Load() {
			slog.Info("auth suspended; skipping spellbook", "path", filepath.Base(path))
			return
		}
		charName := extractCharNameForSuffix(path, watch.SpellbookSuffix)
		if charName == "" {
			slog.Warn("spellbook file with unexpected name; skipping",
				"path", filepath.Base(path))
			return
		}
		fi, statErr := os.Stat(path)
		if statErr != nil {
			slog.Error("stat spellbook", "char", charName, "err", statErr)
			return
		}
		fileMtime := fi.ModTime().UTC().Format(time.RFC3339)

		f, err := os.Open(path)
		if err != nil {
			slog.Error("open spellbook", "char", charName, "err", err)
			return
		}
		rows, perr := parse.ParseSpellbook(f)
		_ = f.Close()
		if perr != nil {
			slog.Error("parse spellbook", "char", charName, "err", perr)
			return
		}
		if len(rows) == 0 {
			// Same T-07-05-style mitigation as inventory: don't write zero
			// rows (would clear the tab). EQ flush mid-write or empty
			// spellbook → skip.
			slog.Info("spellbook empty; skipping write", "char", charName)
			return
		}
		uploadedAt := time.Now().UTC().Format(time.RFC3339)
		if err := sc.WriteSpellbook(ctx, charName, sheet.SpellbookHeader, rows, uploadedAt); err != nil {
			// Plan 02-04 (AUTH-05): same permanent-auth handling as inventory.
			if isPermanentAuthErr(err) {
				suspendForAuth(authSuspended, t, charName, "spellbook", err)
				return
			}
			slog.Error("write spellbook", "char", charName, "err", err)
			t.SetStatus(fmt.Sprintf("Last upload failed: %s spellbook", charName))
			return
		}
		if err := sc.UpsertCharOwner(ctx, charName, cfg.GoogleEmail, bc.WatcherVersion); err != nil {
			slog.Warn("upsert char_owner", "char", charName, "err", err)
		}
		if cfg.LastKnownSpellbookMtime == nil {
			cfg.LastKnownSpellbookMtime = make(map[string]string)
		}
		cfg.LastKnownSpellbookMtime[charName] = fileMtime
		if err := cfg.Save(); err != nil {
			slog.Warn("save cfg after spellbook upload", "char", charName, "err", err)
		}
		slog.Info("uploaded spellbook", "char", charName, "rows", len(rows))
		t.SetStatus(fmt.Sprintf("Last upload: %s spellbook at %s", charName, time.Now().Format("15:04")))
	}
}

// extractCharName returns "<Char>" for "<Char>-Inventory.txt" or "" for
// any other basename. Retained for the existing TestExtractCharName cases
// and any external callers; Plan 02-02 callbacks use
// extractCharNameForSuffix to disambiguate inventory vs. spellbook.
func extractCharName(path string) string {
	base := filepath.Base(path)
	m := charNameRE.FindStringSubmatch(base)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// extractCharNameForSuffix returns "<Char>" for a basename matching
// "<Char>"+suffix, or "" otherwise. Plan 02-02 Task 5 introduces this
// helper so the inventory and spellbook handlers can share extraction
// logic without each owning a regex. Suffixes are watch.InventorySuffix
// and watch.SpellbookSuffix.
func extractCharNameForSuffix(path, suffix string) string {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, suffix) {
		return ""
	}
	name := strings.TrimSuffix(base, suffix)
	if name == "" {
		return ""
	}
	return name
}

// isPermanentAuthErr returns true if err is the boundary signal Plan
// 02-03's withRetry produces on a second auth-flavored 403
// (sheet.ErrPermanentAuth) OR the canonical Google refresh-token-dead
// shape Plan 02-04 Task 1's IsRevokedRefreshToken matches against. Both
// trigger the same UX: tray red + suspend writes + Reauthorize click.
func isPermanentAuthErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sheet.ErrPermanentAuth) {
		return true
	}
	return auth.IsRevokedRefreshToken(err)
}

// suspendForAuth trips authSuspended, turns the tray red, surfaces the
// Reauthorize menu item, and logs a structured event. Called from both
// inventory + spellbook handlers on a permanent-auth failure. Safe with
// a nil authSuspended (test plumbing).
func suspendForAuth(authSuspended *atomic.Bool, t *tray.Controller, charName, kind string, err error) {
	if authSuspended != nil {
		authSuspended.Store(true)
	}
	slog.Error("permanent auth failure — suspending writes",
		"char", charName, "kind", kind, "err", err)
	t.SetIconHealth(tray.HealthRed)
	t.SetStatus("Reauthorize: refresh token died. Click Reauthorize…")
	t.ShowReauthorize()
}

// ChangeWorkbook is the D-04 tray flow. Re-runs picker.Server on a
// fresh loopback listener with a refresh-only TokenSource (no OAuth
// re-prompt). On successful pick + ValidateWorkbook the picker writes
// the new spreadsheetID to config.json; this function then surfaces
// the change in tray status.
//
// Phase 1 limitation (T-07-09 / threat register): the running watcher
// keeps using the OLD spreadsheetID until the user restarts the app.
// Phase 2 polish will hot-swap. Documented as accepted.
func ChangeWorkbook(ctx context.Context, cfg *config.Config, bc auth.BuildConstants, t *tray.Controller) {
	slog.Info("Change Workbook flow start", "email", cfg.GoogleEmail)
	if cfg.GoogleEmail == "" {
		slog.Warn("Change Workbook with no GoogleEmail — wizard not yet run; ignoring")
		t.SetStatus("Setup needed")
		t.ShowContinueSetup()
		return
	}
	if err := bc.Validate(); err != nil {
		slog.Error("Change Workbook: build constants missing", "err", err)
		return
	}

	ts, err := buildTokenSourceFromWincred(ctx, cfg, bc)
	if err != nil {
		slog.Error("Change Workbook: token rebuild", "err", err)
		t.SetStatus("Change Workbook: re-auth required")
		return
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("Change Workbook: listen", "err", err)
		return
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	sc, err := sheet.NewClient(ctx, ts, "")
	if err != nil {
		slog.Error("Change Workbook: sheet client", "err", err)
		return
	}

	pickerSrv := picker.NewServer(sc, ts, cfg, bc)
	pickerSrv.SetRedirectAfterPick("/changed")
	done := make(chan struct{}, 1)
	pickerSrv.OnPicked(func() {
		select {
		case done <- struct{}{}:
		default:
		}
	})
	pickerSrv.AttachRoutes(mux)

	mux.HandleFunc("/changed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:system-ui;text-align:center;margin-top:4em">
<h1 style="color:#2e7d32">&#10003; Workbook changed</h1>
<p>You can close this tab. SquireBot will pick up the new workbook on the next watcher restart.</p>
</body></html>`))
	})

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("Change Workbook: serve", "err", err)
		}
	}()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	startURL := fmt.Sprintf("http://127.0.0.1:%d/picker", port)
	if err := auth.OpenBrowser(startURL); err != nil {
		slog.Warn("Change Workbook: open browser", "err", err, "url", startURL)
	}

	select {
	case <-ctx.Done():
		return
	case <-done:
		t.SetSpreadsheetID(cfg.SpreadsheetID)
		short := cfg.SpreadsheetID
		if len(short) > 8 {
			short = short[:8] + "…"
		}
		t.SetStatus(fmt.Sprintf("Workbook changed: %s", short))
		slog.Info("Change Workbook complete", "spreadsheet_id_set", cfg.SpreadsheetID != "")
	case <-time.After(changeWorkbookTimeout):
		slog.Warn("Change Workbook: timeout waiting for pick")
		t.SetStatus("Change Workbook: cancelled")
	}
}
