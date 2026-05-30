// Package app is the load-bearing watcher orchestrator. The v1 packages
// produced isolated pieces; package app composes them into a runnable pipeline.
//
// Phase 13 (WATCH-08/09/10/11) re-targeted the SINK from Google Sheets to the
// v2.0 backend ingest API. The cold-start flow is now:
//
//	cold start → config.Load → app.MigrateFromV1 (drop dead v1 state)
//	  ├─ credstore.Read finds no guild code → runOnboarding
//	  │     (native PromptGuildCode → backend.Validate(/whoami) → credstore.Store
//	  │      → native PickEQFolder → eqfind.ValidateFolder → cfg.Save)
//	  └─ guild code present → runWatcher
//	then watch.Run with onChange = read → parse.CP1252Reader → backend.Ingest
//
// The watcher is THINNER than v1: it POSTs the RAW (CP1252-decoded-to-UTF-8)
// /outputfile text as Content and the SERVER parses it (D-1). It NO LONGER calls
// parse.Parse on the upload path. The fsnotify 500ms debounce + always-re-read
// live in internal/watch and are unchanged.
//
// There is NO browser OAuth, NO loopback HTTP listener, and NO Google Sheet
// anywhere in this flow — the entire internal/auth|sheet|scaffold|picker|wizard
// stack was deleted (D-2).
package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/boejowen/SquireBot/internal/backend"
	"github.com/boejowen/SquireBot/internal/config"
	"github.com/boejowen/SquireBot/internal/credstore"
	"github.com/boejowen/SquireBot/internal/eqfind"
	"github.com/boejowen/SquireBot/internal/onboarding"
	"github.com/boejowen/SquireBot/internal/parse"
	"github.com/boejowen/SquireBot/internal/tray"
	"github.com/boejowen/SquireBot/internal/update"
	"github.com/boejowen/SquireBot/internal/watch"
)

// charNameRE extracts <Char> from "<Char>-Inventory.txt". Retained for the
// legacy extractCharName helper + its tests.
var charNameRE = regexp.MustCompile(`^(.+)-Inventory\.txt$`)

// watcherRunning serializes the watcher phase of RunApp (HIGH-01). The tray's
// always-enabled "Enter guild code…" item re-invokes RunApp in a fresh
// goroutine; without this guard a guildie who is already connected and clicks it
// again would start a SECOND watch.Run (duplicate ingest POSTs per file change),
// a second daily-update goroutine, and two goroutines writing the shared
// cfg.LastKnown*Mtime maps with no sync → possible `fatal error: concurrent map
// writes` (an uncatchable crash). The CAS below makes a second concurrent entry
// into the watcher phase a no-op; the flag is reset on watcher exit so a
// legitimate re-onboard after a disconnect still starts a fresh watcher.
//
// It guards ONLY the watcher phase — the onboarding branches above it (prompt
// guild code while NOT yet watching) remain fully re-entrant.
var watcherRunning atomic.Bool

// badFolderMessage is the VERBATIM re-prompt text relocated from the deleted
// wizard's EQ-folder step. Kept identical so guildies see the same wording.
const badFolderMessage = "This folder doesn't look like an EverQuest install (no eqgame.exe found). Pick a different folder."

// RunApp is the background goroutine launched from main.go. It blocks until ctx
// is cancelled. Phase 13 flow:
//
//   - credstore.Read finds no guild code → run onboarding (prompt + validate +
//     store + EQ folder). If the guildie cancels, go red ("Setup needed") and
//     return; the tray "Enter guild code…" item re-invokes RunApp.
//   - a guild code is present but no EQ folder is configured → run just the
//     EQ-folder onboarding step.
//   - then start the watcher.
//
// baseURL is the backend base (config override or the build_constants default);
// version is the watcher build version (threaded into every Ingest + the UA).
func RunApp(ctx context.Context, cfg *config.Config, baseURL, version string, t *tray.Controller) {
	code, err := credstore.Read()
	if err != nil || code == "" {
		// No stored guild code → first-run (or post-migration) onboarding.
		slog.Info("no guild code stored; starting onboarding")
		validated, oErr := runOnboarding(ctx, cfg, baseURL, version, t)
		if oErr != nil {
			slog.Warn("onboarding did not complete", "err", oErr)
			t.SetIconHealth(tray.HealthRed)
			t.SetStatus("Setup needed — click \"Enter guild code…\" in the tray menu")
			return
		}
		code = validated
	} else if !hasEQFolder(cfg) {
		// We have a code but never picked an EQ folder (e.g. interrupted setup).
		if perr := pickAndSaveEQFolder(ctx, cfg, t); perr != nil {
			slog.Warn("EQ folder selection did not complete", "err", perr)
			t.SetIconHealth(tray.HealthRed)
			t.SetStatus("Setup needed — pick your EverQuest folder")
			return
		}
	}

	// HIGH-01: only ONE watcher may run at a time. A re-invocation while the
	// watcher is already up (a guildie re-clicking "Enter guild code…" while
	// connected) loses the CAS and returns early — no second watch.Run, no
	// second daily-update goroutine, no unsynchronized cfg-map writers.
	if !watcherRunning.CompareAndSwap(false, true) {
		slog.Info("watcher already running; ignoring re-invocation")
		t.SetStatus("Already connected — watcher is running")
		return
	}
	defer watcherRunning.Store(false)

	if err := runWatcher(ctx, cfg, baseURL, version, code, t); err != nil {
		slog.Error("watcher exited", "err", err)
		t.SetStatus(fmt.Sprintf("Watcher error: %v", err))
		t.SetIconHealth(tray.HealthRed)
	}
}

// hasEQFolder reports whether any EQ folder is configured (either the legacy
// single EQFolder or the EQFolders slice).
func hasEQFolder(cfg *config.Config) bool {
	return cfg.EQFolder != "" || len(cfg.EQFolders) > 0
}

// runOnboarding is the native "paste your guild code" flow (WATCH-10, D-3). It
// loops the code prompt until a valid code is stored (or the guildie cancels),
// then runs the EQ-folder step. Returns the validated guild code on success.
//
// There is NO browser and NO loopback server — onboarding.PromptGuildCode is a
// native Win32 dialog; backend.Validate hits GET /api/v1/whoami.
func runOnboarding(ctx context.Context, cfg *config.Config, baseURL, version string, t *tray.Controller) (string, error) {
	t.SetIconHealth(tray.HealthRed)
	t.SetStatus("Setup needed — enter your guild code")

	bc := backend.New(baseURL)
	for {
		code, err := onboarding.PromptGuildCode("SquireBot Setup", "Paste your guild code (from your guild officer):")
		if err != nil {
			// Cancelled (or unsupported platform) — leave the tray red; the tray
			// "Enter guild code…" item re-triggers RunApp.
			return "", err
		}
		// Validate against the backend before storing (D-3).
		switch verr := bc.Validate(ctx, code); {
		case verr == nil:
			if serr := credstore.Store(code); serr != nil {
				slog.Error("store guild code", "err", serr)
				t.SetStatus("Could not save the guild code; try again")
				return "", serr
			}
			slog.Info("guild code validated and stored")
			// EQ folder step (only if none configured yet).
			if !hasEQFolder(cfg) {
				if ferr := pickAndSaveEQFolder(ctx, cfg, t); ferr != nil {
					return "", ferr
				}
			}
			return code, nil
		case errors.Is(verr, backend.ErrUnauthorized):
			slog.Info("guild code rejected by backend; re-prompting")
			t.SetStatus("That guild code was rejected — try again")
			continue
		default:
			// Network / server error — surface and abort (don't loop on an
			// unreachable backend).
			slog.Warn("guild code validation error", "err", verr)
			t.SetStatus("Couldn't reach the server to validate — try again later")
			return "", verr
		}
	}
}

// pickAndSaveEQFolder runs the native folder picker and persists a valid EQ
// folder, re-prompting with the verbatim badFolderMessage on a folder that fails
// eqfind.ValidateFolder. Relocated from the deleted wizard.
func pickAndSaveEQFolder(ctx context.Context, cfg *config.Config, t *tray.Controller) error {
	_ = ctx // reserved for future cancellation; the native dialog is modal
	for {
		path, err := onboarding.PickEQFolder("Pick your EverQuest folder")
		if err != nil {
			return err // cancelled / unsupported
		}
		if verr := eqfind.ValidateFolder(path); verr != nil {
			slog.Info("picked folder failed EQ validation; re-prompting", "path", path, "err", verr)
			t.SetStatus(badFolderMessage)
			continue
		}
		cfg.EQFolder = path
		cfg.EQFolders = []string{path}
		if serr := cfg.Save(); serr != nil {
			slog.Error("save cfg after EQ folder pick", "err", serr)
			return serr
		}
		slog.Info("EQ folder configured", "path", path)
		return nil
	}
}

// runWatcher starts the watcher loop and dispatches read → POST per inventory or
// spellbook event. Returns when ctx is cancelled or fsnotify errors fatally.
//
// Phase 13: the sink is a backend.Client (no sheet.NewClient/ValidateWorkbook/
// ScaffoldSchemaV1, no heartbeat goroutine — D-10). The auto-update daily-check
// goroutine survives unchanged (direct net/http to GitHub, never Google). Folder
// resolution + rescanCatchUp + watch.Run are unchanged.
func runWatcher(ctx context.Context, cfg *config.Config, baseURL, version, code string, t *tray.Controller) error {
	bc := backend.New(baseURL)

	// Determine the folders to watch. Prefer cfg.EQFolders (multi-folder); fall
	// back to the legacy cfg.EQFolder. config.Load already shims EQFolder→
	// EQFolders, so the fallback is belt-and-braces.
	folders := cfg.EQFolders
	if len(folders) == 0 && cfg.EQFolder != "" {
		folders = []string{cfg.EQFolder}
	}
	if len(folders) == 0 {
		return fmt.Errorf("no EQ folders configured (cfg.EQFolders empty)")
	}

	t.SetIconHealth(tray.HealthGreen)
	t.SetStatus("Connected — watching " + strings.Join(folders, ", "))

	// Auto-update daily-check goroutine (OPS-04). Independent of the ingest
	// path; direct net/http to GitHub Releases. On a successful staging the
	// statusFn updates the tray; the startup-swap (main.go update.Apply) takes
	// effect on the next launch.
	if exe, err := os.Executable(); err != nil {
		slog.Warn("os.Executable failed; auto-update goroutine not launched", "err", err)
	} else {
		go update.RunDailyCheck(ctx, "boejowen", "SquireBot", version, exe, func(msg string) { t.SetStatus(msg) })
	}

	onInventory := makeOnInventoryChange(ctx, bc, cfg, code, version, t)
	onSpellbook := makeOnSpellbookChange(ctx, bc, cfg, code, version, t)

	// WATCH-09: on startup, walk every folder and synthesize callbacks for any
	// file whose mtime is newer than the cached LastKnown*Mtime. Idempotent.
	rescanCatchUp(ctx, cfg, folders, onInventory, onSpellbook)

	return watch.Run(ctx, folders, onInventory, onSpellbook)
}

// rescanCatchUp walks every folder in `folders`, lists every
// <Char>-Inventory.txt and <Char>-Spellbook.txt, compares mtime against
// cfg.LastKnown*Mtime[char], and synthesizes an onInventory / onSpellbook call
// for each newer file. The OnChange callbacks themselves persist the updated
// mtime once an upload succeeds; rescanCatchUp does NOT update the maps itself
// (accepts a re-upload on a clean restart after a partial failure — idempotent
// re-uploads are cheap).
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

// makeOnInventoryChange wraps the read → POST chain into a watch.OnChange
// callback. Extracted for testability.
//
// Phase 13 (D-1/D-8): on a file event it re-stats (capturing mtime BEFORE the
// read so a same-second re-fire is recognised as "already uploaded"), opens,
// decodes CP1252→UTF-8 ONCE via parse.CP1252Reader, skips an empty body, and
// POSTs the raw UTF-8 to the backend — it does NOT call parse.Parse. On success
// it persists cfg.LastKnownInventoryMtime[char] + cfg.Save() (unchanged from v1).
func makeOnInventoryChange(ctx context.Context, bc *backend.Client, cfg *config.Config, code, version string, t *tray.Controller) watch.OnChange {
	return func(path string) {
		charName := extractCharNameForSuffix(path, watch.InventorySuffix)
		if charName == "" {
			slog.Warn("inventory file with unexpected name; skipping", "path", filepath.Base(path))
			return
		}
		// Per CLAUDE.md / RESEARCH §8.3: re-stat + re-read fresh on every event.
		// Never trust fsnotify event payloads on Windows. Capture mtime BEFORE
		// the read so a same-second re-fire after this upload is recognised as
		// "already uploaded" by catch-up.
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
		// Encoding contract A1/D-8: decode CP1252→UTF-8 ONCE here; the backend
		// receives UTF-8 and does NOT re-decode (double-decoding mojibakes curly
		// apostrophes). The watcher NO LONGER calls parse.Parse — the server
		// parses the raw content (D-1).
		utf8Bytes, rerr := io.ReadAll(parse.CP1252Reader(f))
		_ = f.Close()
		if rerr != nil {
			slog.Error("read inventory", "char", charName, "err", rerr)
			return
		}
		if len(bytes.TrimSpace(utf8Bytes)) == 0 {
			// T-07-05 carry-over: skip an empty/mid-flush file (the server's
			// full-snapshot replace would otherwise clear the character's rows).
			slog.Info("inventory empty; skipping upload", "char", charName)
			return
		}

		err = bc.Ingest(ctx, code, charName, "inventory", string(utf8Bytes), version)
		switch {
		case errors.Is(err, backend.ErrUnauthorized):
			slog.Warn("upload 401 — guild code invalid", "char", charName)
			t.SetIconHealth(tray.HealthRed)
			t.SetStatus("Guild code invalid — re-enter via the tray menu")
			return // terminal; NO retry (D-5 / Pitfall 5)
		case errors.Is(err, backend.ErrVersionTooOld):
			slog.Warn("upload 426 — watcher too old", "char", charName)
			t.SetStatus("Update needed — SquireBot will auto-update")
			return
		case errors.Is(err, backend.ErrCrossOwner):
			slog.Warn("cross-owner reject", "char", charName)
			return
		case err != nil:
			slog.Error("upload inventory", "char", charName, "err", err)
			t.SetStatus("Last upload failed: " + charName)
			return
		}

		// Success → persist the mtime so the next catch-up sees it (UNCHANGED).
		if cfg.LastKnownInventoryMtime == nil {
			cfg.LastKnownInventoryMtime = make(map[string]string)
		}
		cfg.LastKnownInventoryMtime[charName] = fileMtime
		if err := cfg.Save(); err != nil {
			slog.Warn("save cfg after inventory upload", "char", charName, "err", err)
		}
		slog.Info("uploaded inventory", "char", charName)
		t.SetStatus(fmt.Sprintf("Last upload: %s at %s", charName, time.Now().Format("15:04")))
	}
}

// makeOnSpellbookChange mirrors makeOnInventoryChange for <Char>-Spellbook.txt
// files (kind "spellbook").
func makeOnSpellbookChange(ctx context.Context, bc *backend.Client, cfg *config.Config, code, version string, t *tray.Controller) watch.OnChange {
	return func(path string) {
		charName := extractCharNameForSuffix(path, watch.SpellbookSuffix)
		if charName == "" {
			slog.Warn("spellbook file with unexpected name; skipping", "path", filepath.Base(path))
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
		utf8Bytes, rerr := io.ReadAll(parse.CP1252Reader(f))
		_ = f.Close()
		if rerr != nil {
			slog.Error("read spellbook", "char", charName, "err", rerr)
			return
		}
		if len(bytes.TrimSpace(utf8Bytes)) == 0 {
			slog.Info("spellbook empty; skipping upload", "char", charName)
			return
		}

		err = bc.Ingest(ctx, code, charName, "spellbook", string(utf8Bytes), version)
		switch {
		case errors.Is(err, backend.ErrUnauthorized):
			slog.Warn("upload 401 — guild code invalid", "char", charName)
			t.SetIconHealth(tray.HealthRed)
			t.SetStatus("Guild code invalid — re-enter via the tray menu")
			return
		case errors.Is(err, backend.ErrVersionTooOld):
			slog.Warn("upload 426 — watcher too old", "char", charName)
			t.SetStatus("Update needed — SquireBot will auto-update")
			return
		case errors.Is(err, backend.ErrCrossOwner):
			slog.Warn("cross-owner reject", "char", charName)
			return
		case err != nil:
			slog.Error("upload spellbook", "char", charName, "err", err)
			t.SetStatus("Last upload failed: " + charName + " spellbook")
			return
		}

		if cfg.LastKnownSpellbookMtime == nil {
			cfg.LastKnownSpellbookMtime = make(map[string]string)
		}
		cfg.LastKnownSpellbookMtime[charName] = fileMtime
		if err := cfg.Save(); err != nil {
			slog.Warn("save cfg after spellbook upload", "char", charName, "err", err)
		}
		slog.Info("uploaded spellbook", "char", charName)
		t.SetStatus(fmt.Sprintf("Last upload: %s spellbook at %s", charName, time.Now().Format("15:04")))
	}
}

// extractCharName returns "<Char>" for "<Char>-Inventory.txt" or "" for any
// other basename. Retained for the existing TestExtractCharName cases.
func extractCharName(path string) string {
	base := filepath.Base(path)
	m := charNameRE.FindStringSubmatch(base)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// extractCharNameForSuffix returns "<Char>" for a basename matching
// "<Char>"+suffix, or "" otherwise. Suffixes are watch.InventorySuffix and
// watch.SpellbookSuffix.
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
