---
phase: 13-watcher-re-target-onboarding
reviewed: 2026-05-30T00:00:00Z
depth: deep
files_reviewed: 17
files_reviewed_list:
  - internal/backend/client.go
  - internal/backend/client_test.go
  - internal/onboarding/dialog.go
  - internal/onboarding/dialog_windows.go
  - internal/onboarding/dialog_other.go
  - internal/onboarding/dialog_test.go
  - internal/credstore/store.go
  - internal/credstore/store_test.go
  - internal/app/runapp.go
  - internal/app/migrate.go
  - internal/app/migrate_test.go
  - cmd/squirebot/main.go
  - cmd/squirebot/build_constants.go
  - cmd/squirebot/console_windows.go
  - internal/backendsrv/ingest/handler.go
  - internal/backendsrv/ingest/whoami.go
  - internal/update/manifest.go
findings:
  critical: 1
  warning: 1
  info: 5
  total: 7
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-05-30
**Depth:** deep (cross-file: watcher sink → backend ingest, onboarding loop, migration, version gates)
**Files Reviewed:** 17 (+ supporting reads: config, tray, watch, eqfind, parse, envelope/version, deleted v1 auth store)
**Status:** issues_found

> Severity scale for this advisory: **HIGH** = bites a guildie on auto-update / data-corruption / crash; **MEDIUM** = real defect, narrower blast radius; **LOW** = cosmetic / pre-existing / test-only. This report does NOT block phase completion — act on it via `/gsd-code-review-fix 13`.

## Summary

The genuinely-new + risky Phase 13 surfaces are, with one exception, in good shape. Specifically these are **clean** and I could not find a defect after tracing them:

- **`internal/backend/client.go`** — the status→error map (204/401/409/426/400/422/5xx/transport) is correct; the `[1s,2s,4s]` retry fires *only* on `errRetryable` (5xx + transport) and never on a terminal sentinel; an unexpected status is non-retryable (cannot spin the loop); ctx-cancellation is honoured in `ctxSleep`; the bearer code and raw content are never logged (proven by `TestIngest_NoSecretInLogs`). The body is marshalled once and a fresh `bytes.Reader` wraps each attempt. `Validate` is correctly one-shot (no retry).
- **`internal/credstore/store.go`** — Store/Read/Delete mechanics are the verbatim-correct salvage of the v1 wincred store; the not-found-on-Read = "needs onboarding" contract holds; plaintext-only-in-DPAPI, never-in-config, never-logged is respected.
- **`internal/app/migrate.go` (`MigrateFromV1`)** — idempotent (re-reads raw JSON; both-keys-empty sentinel no-ops), preserves `EQFolder`/`EQFolders`/mtime maps, and reconstructs the v1 wincred target as `"SquireBot:"+google_email` which **exactly** matches the deleted v1 `auth.CredPrefix+email` (verified against `git show 5e35bc0^:internal/auth/store.go` and the v1 `runapp.go` which keyed `auth.ReadToken(cfg.GoogleEmail)`). The stale-credential delete is best-effort and the whole thing is non-fatal — a migration failure cannot block startup. No way found for it to corrupt a working config or loop onboarding.
- **Version gates** — `ingest.IsOlder` and `update.IsNewer` agree in direction; `IsOlder("2.0.0","2.0.0")==false` (a floor-version watcher is **not** rejected, confirmed by `TestIngest_FloorVersion_204`); empty `watcher_version` is the one intentional 426 exception (`env.WatcherVersion != "" && IsOlder(...)`); the 426 gate is post-decode/pre-store (writes nothing); a future `2.0.0`/`2.1.0` watcher passes. `IsNewer` is defensively false on either-side parse failure, so a corrupt manifest can never trigger an update. The release.yml stamps `-X main.Version=<tag>`, so a `v2.0.0` tag clears the `minWatcherVersion="2.0.0"` floor.
- **`/whoami`** — side-effect-free (only a parameterized `SELECT label ... WHERE id=?`, proven row-count-unchanged), 401 discipline reused verbatim, degrades to empty label rather than 500, never logs the token.
- **Decode-once** — `parse.CP1252Reader` is applied exactly once on the watcher disk-read side (`runapp.go:315/382`); the server feeds `env.Content` into the **bare** `parse.Parse` with no second charmap decode (`handler.go:168`, and `parse/inventory.go` moved the decode off `Parse` per contract A1). `TestIngest_UTF8_ByteFidelity` pins byte fidelity. No mojibake.
- **Win32 memory safety** — `go vet ./internal/onboarding/...` is clean (the previously-caught `uintptr`→pointer bug is gone). The token-indirection registry never reconstructs a Go pointer from a `uintptr`; the in-memory DLGTEMPLATE stays pinned because `DialogBoxIndirectParamW` is modal/blocking; `getDlgItemText` sizes its buffer at `maxCodeLen+1` and `GetDlgItemTextW` truncates rather than overflows.
- **Deletion fallout** — no live `import` of any deleted watcher package (`internal/auth|sheet|scaffold|picker|wizard|heartbeat`) survives (grep-verified; the only matches are server-side `internal/backendsrv/*` packages, which were NOT deleted, plus doc-comments). `go build ./...`, `go vet ./...`, and `go test ./...` are all green (22 packages, 0 failures).

The **one HIGH** finding is a watcher-side concurrency hazard on the re-entrant "Enter guild code…" path that ships via auto-update and can produce duplicate uploads, duplicate background goroutines, and (worst case) a `fatal error: concurrent map writes` crash. Details below.

## High

### HIGH-01: "Enter guild code…" spawns a second concurrent watcher → duplicate uploads + unsynchronized `cfg` map writes (crash risk)

**File:** `cmd/squirebot/main.go:133-139` (the `OnEnterGuildCode` callback) → `internal/app/runapp.go:68-96` (`RunApp` → `runWatcher`)

**Issue:** The tray's `Enter guild code…` item is always enabled (`internal/tray/tray.go` never `Disable()`s `mEnterGuildCode`), and its handler unconditionally launches a fresh goroutine:

```go
OnEnterGuildCode: func() {
    go app.RunApp(ctx, cfg, baseURL, Version, trayCtl)
},
```

There is already one `RunApp` goroutine running (launched at `main.go:165`), and **nothing guards against a second concurrent `RunApp`/`runWatcher`** (no mutex, no `sync.Once`, no "already running" flag — confirmed by grep across `internal/app`). The re-entrant flow is reachable by a guildie who is *already connected* (green, watching) and clicks the item to re-enter/rotate a code: `RunApp` calls `credstore.Read()`, finds the stored code, sees `hasEQFolder(cfg)==true`, skips both onboarding branches, and falls straight through to `runWatcher` a second time. `runWatcher` then:

1. calls `watch.Run(...)`, which does its own `fsnotify.NewWatcher()` + `w.Add(folder)` (`internal/watch/watcher.go:50/61`). Two independent watchers on the same folders means **every `/outputfile` write now fires both callback chains → two POSTs per file change** to the backend.
2. launches a **second** `update.RunDailyCheck` goroutine (`runapp.go:209`).
3. both callback chains write the shared `cfg.LastKnownInventoryMtime` / `cfg.LastKnownSpellbookMtime` maps (`runapp.go:349-352`, `413-416`) and call `cfg.Save()` with **no synchronization**. Concurrent writes to the same Go map are a hard runtime fault: `fatal error: concurrent map writes` — an uncatchable crash that takes the whole watcher down on a guildie's PC.

Because this binary auto-deploys to ~12 guildies, anyone who clicks the always-visible item a second time (a natural thing to do — "let me re-paste my code") arms the race. The duplicate-POST behavior alone is wasteful; the map-write crash is the real hazard.

**User impact:** Best case, doubled backend traffic + two tray-status writers fighting. Worst case, a `concurrent map writes` panic crashes the watcher; on next launch it recovers (catch-up re-uploads), but the guildie sees the tray vanish and may not know to relaunch.

**Fix:** Serialize `RunApp` so a second invocation is a no-op (or cleanly supersedes the first) instead of running a parallel watcher. Minimal guard — gate the watcher phase on a package-level flag/once, e.g.:

```go
// internal/app/runapp.go
var watcherRunning atomic.Bool

func RunApp(ctx context.Context, cfg *config.Config, baseURL, version string, t *tray.Controller) {
    // ... onboarding branch unchanged ...

    if !watcherRunning.CompareAndSwap(false, true) {
        slog.Info("watcher already running; ignoring re-invocation")
        return
    }
    defer watcherRunning.Store(false)

    if err := runWatcher(ctx, cfg, baseURL, version, code, t); err != nil {
        // ...
    }
}
```

This keeps the legitimate first-run + re-onboard-after-cancel flows working while making a click-while-connected harmless. (Alternative: have the tray `Disable()` `mEnterGuildCode` once the watcher reaches green and only re-enable it on a 401, but the atomic guard is simpler and also covers the OS-signal/manual-fire interleavings.) Whichever is chosen, the shared `cfg` map writes should never be reachable from two goroutines at once.

## Medium

### MED-01: `dialogState` leaks in `byToken` when `DialogBoxIndirectParamW` fails to create the dialog

**File:** `internal/onboarding/dialog_windows.go:162-197` (`PromptGuildCode`) + `122-129` (`registerState`)

**Issue:** `PromptGuildCode` calls `registerState(st)`, which inserts `st` into `dialogRegistry.byToken[tok]`. The only code that removes a token entry is `claimByToken`, which runs **inside `WM_INITDIALOG`** — i.e. only if the dialog is actually created and its proc receives the init message. On the documented creation-failure path:

```go
ret, _, _ := procDialogBoxIndirectParamW.Call(...)
if int(ret) == -1 {
    return "", errDialogCreate   // <- token entry never removed
}
```

`WM_INITDIALOG` never fires, so the `byToken[tok]` entry is never claimed and never deleted — it lingers for the process lifetime. The mutex-guarded map grows by one small `*dialogState` per failed creation. This is a (slow) memory leak, not a correctness bug, and creation failure is rare; but the registry was added precisely to manage these lifetimes, and it has an unhandled exit. (The success and cancel/close paths are fine — `claimByToken` → `releaseHwnd` cleans those up.)

A related observation while in this file: `dialogProc` is a Go callback invoked across the `DialogBoxIndirectParamW` syscall boundary. A Go panic propagating out of that callback (e.g. from inside `setDlgItemText`/`getDlgItemText`) is undefined/fatal. The current callees can't realistically panic (`UTF16PtrFromString` returns an error rather than panicking, slice indexing is bounded), so this is not a live bug — but any future edit that adds a panicking path inside the proc would crash hard. Worth a one-line comment that `dialogProc` must never panic.

**User impact:** Negligible in practice (a few leaked structs only if the native dialog repeatedly fails to instantiate, which itself would already be breaking onboarding). Flagged as MEDIUM only because it is a real lifetime-management gap in the most safety-sensitive (syscall-boundary) file.

**Fix:** Clean up the token on the creation-failure path:

```go
ret, _, _ := procDialogBoxIndirectParamW.Call(...)
if int(ret) == -1 {
    dialogRegistry.mu.Lock()
    delete(dialogRegistry.byToken, tok)
    dialogRegistry.mu.Unlock()
    return "", errDialogCreate
}
```

(Or add an `unregisterToken(tok)` helper symmetric to `registerState`.) Optionally add `// dialogProc MUST NOT panic — it runs across the syscall callback boundary.` above `dialogProc`.

## Info

### IN-01: Two known gofmt nits (cosmetic — already acknowledged)

**File:** `internal/onboarding/dialog_windows.go`, `internal/backend/client_test.go`

`gofmt -l` flags both files (column-alignment in the const block / struct literals). These are the two cosmetic nits the maintainer already called out; the other Phase-13 files (`client.go`, `migrate.go`, `runapp.go`, `credstore/store.go`, `manifest.go`, `handler.go`, `whoami.go`) are gofmt-clean. **Fix:** `gofmt -w internal/onboarding/dialog_windows.go internal/backend/client_test.go`. No behavior change.

### IN-02: A >512-char paste is silently truncated with no user feedback

**File:** `internal/onboarding/dialog_windows.go:262-271` (`getDlgItemText`, `maxCodeLen = 512`)

The edit control has no `EM_LIMITTEXT`, and `GetDlgItemTextW` is read into a fixed `maxCodeLen+1` buffer, so a pasted code longer than 512 chars is silently truncated (then likely fails `Validate` with a confusing 401). This is memory-safe (no overflow — `GetDlgItemTextW` honours the buffer length), purely a UX nicety. Guild codes are short, so this is unlikely to ever trigger. **Fix (optional):** post `EM_SETLIMITTEXT` to the edit control on `WM_INITDIALOG`, or just leave it — 512 is generous.

### IN-03: `config.Save()` remove-then-rename window is now hit on every v1→v2 first launch

**File:** `internal/config/config.go:115-122` (pre-existing), exercised by `internal/app/migrate.go:91` (`cfg.Save()`)

`Save()`'s fallback does `os.Remove(p)` then a second `os.Rename(tmp, p)`. If the process dies (or the disk fills) between the remove and the successful rename, `config.json` is gone and the guildie loses their EQ folder + mtime maps on next launch (recoverable only by re-onboarding). This is **pre-existing** behavior, not introduced by Phase 13 — but `MigrateFromV1` makes every v1→v2 first launch call `Save()`, so the (narrow) window is now traversed by all ~12 guildies exactly once during the cutover. **Fix (optional, out of Phase-13 scope):** make `Save()` rename-over-without-remove (the tmp+atomic-rename already handles the common case; the `os.Remove` fallback is the risky branch and modern Windows `os.Rename` overwrites). Noted for awareness, not a Phase-13 regression.

### IN-04: Test-only `readAll` helper can short-read the request body

**File:** `internal/app/runapp_test.go:232-239`

```go
func readAll(r *http.Request) (string, error) {
    buf := make([]byte, r.ContentLength)
    ...
    _, err := r.Body.Read(buf)   // single Read may return < ContentLength
    return string(buf), err
}
```

A single `Body.Read` is not guaranteed to fill `buf`; this should be `io.ReadAll`. It happens to pass today because the test bodies are tiny and arrive in one chunk, but it's a latent flaky-test pattern. **Fix:** `b, err := io.ReadAll(r.Body); return string(b), err`. Test-only — no production impact.

### IN-05: `extractCharName` + `charNameRE` are dead-ish (retained only for a legacy test)

**File:** `internal/app/runapp.go:50` (`charNameRE`) + `427-434` (`extractCharName`)

The production sink uses `extractCharNameForSuffix`; `extractCharName`/`charNameRE` are kept solely for `TestExtractCharName`. Harmless, but it's parallel name-parsing logic that no production path calls (the regex requires `-Inventory.txt` and silently does nothing for spellbooks). **Fix (optional):** drop both and the test, or fold the test into `extractCharNameForSuffix`. Pure tidiness.

---

_Reviewed: 2026-05-30_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
