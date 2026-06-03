# Quick Task 260602-u7m — Deferred items

These Church of Clean Code findings are NOT quick fixes. Promote to a planned phase
or backlog (`/gsd-add-backlog`). Full detail in `.planning/CLEAN-CODE-REPORT.md`.

## Test work (recommended next — HIGH value)
- **C1** `makeOnSpellbookChange` (`internal/app/runapp.go:389`) has zero tests; its inventory
  twin has four. Add 401 / 426 / cross-owner / empty-file / mtime-persist coverage.
- **C2** `internal/eqfind` real discovery (~15%): test `walkRoot` + sentinel-matching against a
  `t.TempDir()` tree with planted `eqgame.exe`/`eqclient.ini` at varying depths + decoys.
- **Refactor** Collapse the duplicated twins `runapp.go:314`/`:389` into one
  `makeOnFileChange(kind, suffix, mtimeMap …)` + extract `handleIngestErr(...)`. Closes C1 and
  removes ~50 lines of copy-paste in one move. Do the refactor first, then test once.

## Dependencies (needs toolchain — user installs)
- **W1** Run `govulncheck ./...` on both binaries before next release (no CVE cert was possible
  in the audit). Highest priority: `x/crypto`/`minisign` (update signature path), `fsnotify`.
- **W2** Bump `fsnotify` v1.7.0 → v1.10.1 (Windows `ReadDirectoryChangesW` event-loss fixes), then
  re-run the WATCH-07 regression suite.
- INFO: opportunistic minor bumps — `fyne.io/systray` 1.10.0→1.12.1, `wincred` 1.2.0→1.2.3,
  `aead.dev/minisign` 0.2.0→0.3.0 (test the update flow after the minisign bump).

## Optional polish
- **Observability M2** — add an explicit `errors.Is(err, backend.ErrBadPayload)` case in the
  `runapp.go` upload switch (distinguish malformed-envelope bugs from transport blips), or drop the
  unused export.
- **Observability L1/L2** — `update/swap.go:152` `slog.Info` contradicts its own "cannot use slog"
  doc; auto-update *failures* never reach the tray. Low priority for a 12-person guild.
- **Architecture LOW** — add `credstore.DeleteLegacyGoogle(email)` so `app/migrate.go` stops
  importing `wincred` directly; inject an `OnOpenLogFolder` callback so `tray.go` stops shelling
  `explorer.exe` inline.

## Documentation overhaul (bigger than a quick fix)
- CLAUDE.md "Sheet side", "Architecture" (three-layer pancake), and "OAuth scope" sections, plus
  `.planning/research/STACK.md`, still describe the pre-v2.0 Google Sheets system that the v2.0
  "Off Google" milestone replaced with the self-hosted website. Needs a dedicated reconciliation pass.
