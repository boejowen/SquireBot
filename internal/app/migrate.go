// Package app — first-launch v1→v2 migration (WATCH-11 / CONTEXT D-7).
//
// When a guildie's v1.x watcher auto-updates to the re-targeted (v2) binary, the
// first launch must clean up the dead Google state so no orphaned secret lingers:
//
//  1. Drop the dead config.json fields (`google_email`, `spreadsheet_id`). These
//     were removed from config.Config in Phase 13, so they are now unknown keys
//     that encoding/json silently ignores on Load; rewriting config.json via
//     cfg.Save() physically removes them from disk.
//  2. Delete the stale Google OAuth refresh-token wincred entry, which was stored
//     under the target `SquireBot:<google-email>` by the v1 internal/auth store.
//     (internal/auth is deleted in this same plan, so the delete is done directly
//     against wincred here rather than through the old auth package.)
//
// The migration PRESERVES everything else — EQFolder(s) and the LastKnown*Mtime
// maps are untouched (RESEARCH Pitfall 4: a blunt reset would force a re-pick of
// the EQ folder or a full re-upload storm). It is IDEMPOTENT: once the Google
// fields are gone from config.json, a second run reads them as empty and no-ops.
package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/danieljoos/wincred"

	"github.com/boejowen/SquireBot/internal/config"
)

// MigrateFromV1 performs the one-time v1→v2 cleanup described in the package
// doc. cfg is the already-Loaded config (by the NEW struct, so it carries no
// Google fields). The function:
//
//   - re-reads the RAW config.json to recover the v1-only google_email /
//     spreadsheet_id values (cfg can't surface them — the struct dropped them);
//   - if BOTH are empty, returns nil immediately (the idempotency sentinel:
//     already migrated, or a fresh v2 install);
//   - otherwise deletes the stale SquireBot:<google-email> wincred entry (best
//     effort — a not-found is fine) and calls cfg.Save() to rewrite config.json
//     without the Google keys.
//
// Errors are returned but callers (main.go) treat them as non-fatal (a failed
// migration must not block the watcher from starting). The guild code and any
// secret are NEVER logged (V7) — only the fact of removal.
func MigrateFromV1(cfg *config.Config) error {
	// 1. Re-read the raw config.json to recover the v1-only keys.
	raw, err := os.ReadFile(config.Path())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // fresh install, nothing to migrate
		}
		return err
	}
	// Mirror config.Load's BOM strip so a Notepad-saved file parses.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	var v1 struct {
		GoogleEmail   string `json:"google_email"`
		SpreadsheetID string `json:"spreadsheet_id"`
	}
	if err := json.Unmarshal(raw, &v1); err != nil {
		// A corrupt config is the watcher's problem to surface elsewhere; the
		// migration simply declines to act on an unparseable file.
		return err
	}

	// 2. Idempotency sentinel: both v1 keys absent ⇒ already migrated / fresh v2.
	if v1.GoogleEmail == "" && v1.SpreadsheetID == "" {
		return nil
	}

	// 3. Delete the stale Google refresh-token wincred entry (best effort). The
	//    v1 target was SquireBot:<google-email>. A not-found error is expected on
	//    a machine that never completed the v1 wizard — ignore it.
	if v1.GoogleEmail != "" {
		if cred, gerr := wincred.GetGenericCredential("SquireBot:" + v1.GoogleEmail); gerr == nil {
			if derr := cred.Delete(); derr != nil {
				slog.Warn("v1→v2 migration: could not delete stale Google credential", "err", derr)
			} else {
				slog.Info("v1→v2 migration: stale Google credential removed")
			}
		}
	}

	// 4. Rewrite config.json without the Google keys. cfg was Loaded by the NEW
	//    struct (no google_email/spreadsheet_id fields), so Save() drops them.
	if err := cfg.Save(); err != nil {
		return err
	}
	slog.Info("v1→v2 migration: dropped dead config fields (google_email, spreadsheet_id)")
	return nil
}
