# Phase 2: Watcher Robustness + Schema Lock - Research

**Researched:** 2026-05-01
**Domain:** Go Windows watcher hardening (auto-update, retry/backoff, OAuth re-consent UX, autostart, heartbeat scheduling) + code-signing strategy + Google Sheets schema scaffolding
**Confidence:** HIGH for code-signing landscape (multiple sources cross-verified, including a Microsoft-confirmed change to EV behavior); HIGH for `minio/selfupdate` Windows mechanics (verified against `apply.go` source); HIGH for Sheets API retry semantics; HIGH for the schema scaffolding work (this is just disciplined application of existing locked decisions); MEDIUM for `goreleaser` integration shape (we have not run a real build yet).

---

## Summary

Phase 2 has one decision that dominates the planning surface: **code signing**. The 2024-2026 landscape changed materially since `STACK.md` and `PITFALLS.md` were written, and the Phase 1 lesson (SmartScreen MOTW behavior is the real UX risk) is the right framing. Everything else in Phase 2 — autostart, retry/backoff, refresh-token UX, heartbeat, auto-update, schema scaffolding — is disciplined application of patterns that are either already in the codebase or well-established in the Go ecosystem.

**Headline finding (overrides PITFALLS.md and STACK.md):** EV code-signing certificates **no longer grant instant SmartScreen reputation**. Microsoft removed this behavior in March 2024, and in August 2024 removed all EV Code Signing OIDs from existing roots in the Microsoft Trusted Root Program. EV-signed binaries now go through the same reputation-building process as OV-signed binaries — and our 12-user installed base will never accumulate enough downloads to clear that bar. The recommended path for SquireBot in 2026 is therefore **(a) ship unsigned with a polished SmartScreen walkthrough as the budget default**, or **(b) the SignPath Foundation free-for-OSS code-signing path if eligibility is acceptable**. Paid certificates (EV $400-560/yr, OV $200-280/yr, Certum OSS €69 one-time + €30/yr) are a wash on the SmartScreen UX axis and are not recommended for this project's scale.

**Primary recommendation:** Ship Phase 2 unsigned with a 30-second SmartScreen walkthrough video in the README and on the GitHub Release page; apply for SignPath OSS sponsorship in parallel (no-cost path); revisit paid signing only if SignPath eligibility is denied AND a guildie reports the unsigned UX as a blocker. Adopt `goreleaser` to replace the hand-rolled CI stub. Use `minio/selfupdate`'s startup-swap pattern verbatim from its README. Use `google.golang.org/api`'s built-in `gax` exponential backoff via the standard `googleapi.Error.Code` switch — do NOT add `cenkalti/backoff` for the hot-path Sheets writer. Use `time.AfterFunc` self-rescheduling for the once-daily heartbeat — do NOT pull in `robfig/cron` or `go-co-op/gocron` for one job. Schema lock is purely a write-correct-rows-once exercise against a workbook the watcher already owns.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| OAuth re-consent UX (red tray, click reopens browser) | Watcher (Go) | — | Token failure detected by `oauth2.TokenSource.Token()`; UI lives in `internal/tray` |
| Sheets API retry/backoff | Watcher (Go) | — | All hot-path writes originate in the watcher; `gax`-based retry is Google client-internal |
| Autostart on logon | NSIS installer + Windows registry | Watcher (verifies at startup) | `HKCU\...\Run` is set at install time; watcher logs autostart status for support |
| Daily heartbeat to `_char_owner.last_seen` | Watcher (Go) | — | Heartbeat exists to prove "watcher is alive even if files unchanged" — must originate from the watcher |
| Auto-update download + manifest fetch | Watcher (Go) | GitHub Releases (CDN) | `latest.json` published to Releases; watcher polls daily |
| Auto-update binary swap | Watcher (Go, on next startup) | — | Windows file-lock means the running binary cannot replace itself; swap happens at next launch |
| Code signing | CI (`goreleaser`) | Developer machine (cert custody) | Signing belongs in release pipeline, not in development |
| Schema scaffolding (every dimension/view tab + frozen `_meta`) | Watcher (one-time bootstrap) | Apps Script (Phase 3+ owns these tabs after handoff) | Watcher writes the empty scaffold once; Apps Script populates it later |
| Soft-delete fields, `discord_handle`, view-tab placeholders | Watcher (one-time bootstrap) | — | These columns/tabs must exist at `schema_version=1` even if no v1 UI populates them |
| Schema version enforcement | Watcher (read on startup) + Apps Script (read on every trigger, Phase 3+) | — | `WATCHER_MAX_SCHEMA_VERSION` already in `internal/sheet/client.go`; pair with `SCRIPT_MIN_SCHEMA_VERSION` later |
| Spellbook file watcher | Watcher (Go) | — | Symmetric to inventory watcher already in `internal/watch` |

---

## User Constraints (from CONTEXT.md)

> No CONTEXT.md exists yet for Phase 2. The constraints below are extracted from `STATE.md` Decisions Log, `PROJECT.md`, the phase context provided by the orchestrator, and `CLAUDE.md`. Treat them as already-locked.

### Locked Decisions (do not relitigate)

1. **Watcher language and stack:** Go 1.24+ single-binary, `fsnotify` v1.7+ (500ms debounce, always re-read on event, watch parent dir not file), `wincred` for token storage, `golang.org/x/oauth2` + `google.golang.org/api/sheets/v4`, `fyne.io/systray`, `lumberjack` for logs.
2. **Installer:** NSIS 3.10+ per-user (`RequestExecutionLevel user`, install to `%LOCALAPPDATA%\Programs\SquireBot`), autostart via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. **No UAC, no Task Scheduler, no Windows Service.**
3. **OAuth scope:** `https://www.googleapis.com/auth/drive.file` only. Consent screen MUST be Production (already done in Phase 1 Plan 02). Google's `/token` endpoint requires `client_secret` for Desktop OAuth clients **even with PKCE** (Phase 1 lesson; the desktop secret is effectively public per Google's own docs and is baked into the binary alongside the client ID).
4. **Atomic writes only:** `spreadsheets.batchUpdate` clear+write per character per file, `valueInputOption=RAW`, `StringValue` cells (never `NumberValue`), `fields="userEnteredValue"`, per-character non-overlapping ranges. No appends, no row diffs.
5. **Auto-updater library:** `github.com/minio/selfupdate` is locked. Do NOT propose alternatives.
6. **Sheet schema:** workbook is the only persistence; schema evolution is extend-only (add columns at right edge, add tabs, add `_meta` rows). Breaking changes require a `_meta.schema_version` bump + idempotent migration + watcher's `WATCHER_MAX_SCHEMA_VERSION` check + script's `SCRIPT_MIN_SCHEMA_VERSION` check.
7. **CRITICAL — Views are CONSOLIDATED, not per-character.** `view`, `gear_check`, `spell_check`, `bank` mega-tabs with leading `Char` column. Per-character view tabs would breach Google's 200-tab/workbook limit at guild scale (12 × 10 × 5 ≈ 600 tabs). Landing tabs ARE per-character (~120 tabs total — comfortable).
8. **Identity:** OAuth `userinfo.email` is the canonical identity; the watcher writes it into `_char_owner.owner_email` on first sighting. (`Session.getActiveUser().getEmail()` returns the script owner, NOT the writer — load-bearing distinction for Phase 3+, irrelevant in Phase 2 where Apps Script does not exist yet.)
9. **No Apps Script work in this phase.** Phase 3 deals with TypeScript/clasp scaffolding. Phase 2 only writes the empty scaffold tabs and freezes the schema; Apps Script populates them later.
10. **Schema is frozen at `schema_version=1` at the END of this phase.** Every hidden tab and every column the design will ever need must be present, even unused ones, so Phase 3+ never breaks landing-tab consumers.

### Claude's Discretion (within the locked decisions)

- Internal package layout for the new code (`internal/update`, `internal/heartbeat`, `internal/scaffold`, etc. — exact names are mine).
- Whether to introduce `goreleaser` now (recommended: yes — see `## Standard Stack`) or stretch the hand-rolled CI stub.
- The exact failure-detection heuristic for `invalid_grant` (Google returns multiple shapes; `## Code Examples` documents the canonical one).
- Ordering of plans within the phase (research is silent on this; the planner decides).
- Whether to use `cenkalti/backoff` or hand-roll a delay slice for the explicit `2/4/8/16/32/60s` policy required by WATCH-07. Recommendation: hand-roll a slice; we have a fixed schedule, not a configurable strategy.
- Heartbeat write granularity — per-character one-cell write vs. one-cell batch update. Recommendation: one `batchUpdate` per heartbeat fire that touches every active character's `last_seen` cell at once (single API call, single quota debit).

### Deferred Ideas (OUT OF SCOPE)

- Apps Script TypeScript scaffolding — Phase 3.
- Any consumer of the dimension tabs (`_pigparse`, `_wiki_*`, `_quest_items`) — Phase 3-4.
- View tab CONTENT (`view`, `gear_check`, etc.) — only the empty placeholder tabs ship in Phase 2.
- Search sidebar / `HtmlService` work — Phase 5.
- The Day-10 token-survival check (already scheduled via `trig_01Uog2muQ22CBsjZfqPiSH9r`, fires 2026-05-13).
- Per-character "hide from guild" UI — schema scaffolding only (`is_hidden` column exists; no Phase 2 UI populates it).
- Eviction workflow documentation — Phase 5.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INST-04 | Autostart via `HKCU\...\Run` | `## Code Examples` §1 — NSIS Run-key snippet; `## Architecture Patterns` §2 |
| INST-05 | Code-signed OR documented SmartScreen walkthrough <30s | `## Code-Signing Recommendation Matrix` (the headline section) |
| AUTH-05 | `invalid_grant` → red tray → click reopens OAuth, no silent retry-loop | `## Code Examples` §3 — `googleapi.Error` matching; `## Common Pitfalls` Pitfall A |
| WATCH-02 | Spellbook file watcher | `## Architecture Patterns` §3 — symmetric to existing inventory watcher |
| WATCH-03 | Configurable list of EQ folders | `## Architecture Patterns` §3 — already covered by `cfg.EQFolder` extension to `cfg.EQFolders []string` |
| WATCH-05 | Spellbook 2-col format verified before schema lock | Inline (developer collects sample); not a research deliverable |
| WATCH-06 | Atomic clear+write `batchUpdate` for spellbook | Already implemented for inventory; mirror for spellbook |
| WATCH-07 | Backoff 2/4/8/16/32/60s, `Retry-After` on 429, refresh once on 403 | `## Code Examples` §2 — `googleapi.Error` switch + hand-rolled delay slice |
| WATCH-08 | Daily heartbeat to `_char_owner.last_seen` | `## Code Examples` §4 — `time.AfterFunc` self-reschedule; `## Don't Hand-Roll` confirms cron lib is overkill |
| WATCH-09 | On restart, rescan folders + upload files newer than `last_known_inventory_mtime` | `## Architecture Patterns` §4 — startup catch-up |
| SCHEMA-01..04 | Per-char `inv:`, `spell:`, hidden dim tabs, consolidated mega-tabs | `## Architecture Patterns` §5 — schema scaffolder |
| SCHEMA-05 | `_char_owner` full column set incl. `discord_handle`, `is_hidden`, `is_removed` | Locked column list in `## Architecture Patterns` §5 |
| SCHEMA-06 | Tabs by `getSheetById`, never by name | Already enforced in `internal/sheet` (caches sheetId per name on first lookup) |
| SCHEMA-07 | `_meta`: `schema_version`, `canonical_id`, `bank_toon_name`, `bank_coin_*`, `last_*_refresh` | `## Architecture Patterns` §5 — `_meta` row list |
| SCHEMA-08 | Extend-only, with version checks on both sides | Locked in `## User Constraints`; pattern established in `internal/sheet/meta.go` |
| OPS-04 | Auto-update: `latest.json`, SHA-256, atomic startup swap | `## Architecture Patterns` §6 — `minio/selfupdate` startup-swap; `## Code Examples` §5 |
| OPS-05 | Watcher writes its version into `_status` | `## Architecture Patterns` §5 — `_status` shape |

---

## Standard Stack

### Code-Signing Recommendation Matrix

> This is the highest-blast-radius decision in Phase 2. It is presented as a matrix because the user makes the final call.

**Critical correction to PITFALLS.md and STACK.md:** As of **March 2024**, EV certificates no longer grant instant SmartScreen reputation. Microsoft removed the "EV-signed = no warning" behavior; in August 2024 they removed all EV Code Signing OIDs from existing roots in the Microsoft Trusted Root Program. Both EV and OV now go through the same reputation-building curve, which is unreachable for a 12-user audience. `[CITED: learn.microsoft.com/answers/questions/417016]` `[CITED: support.sectigo.com/PS_KnowledgeDetailPageFaq?Id=kA01N000000zFJx]`

This invalidates the central premise of PITFALLS.md Pitfall #2 (which claimed EV grants immediate SmartScreen trust) and SUMMARY.md's "EV preferred ~$300-600/yr" recommendation. The new reality:

| Path | Up-front cost | Ongoing cost | SmartScreen on first download | Hardware token? | Effort to integrate | Recommendation |
|------|--------------|-------------|-------------------------------|----------------|--------------------|----|
| **A. Unsigned + walkthrough** | $0 | $0 | Yes (full blue panel) | No | None — walkthrough is a 60-line README + a 30s screen recording | **DEFAULT — recommended** |
| **B. SignPath Foundation OSS** | $0 (eligibility-gated) | $0 | Yes initially, builds reputation faster than self-bought OV (SignPath has aggregate reputation) | No (cloud-HSM-backed; SignPath signs in their cloud) | Medium — apply, await approval (~1-4 weeks), wire `goreleaser` to call SignPath's CI integration | **Apply in parallel with A** |
| **C. Certum OSS Code Signing** | €69 one-time (cert + smartcard reader) + €35 shipping | €30/yr | Yes (same reputation curve as OV) | Yes — physical smartcard you must have plugged in to sign | Medium — adds a developer-machine step or requires cloud HSM mirroring | Only if A+B both fail |
| **D. OV cert (Sectigo/SSL.com/etc.)** | $200-280/yr | $200-280/yr | Yes (same reputation curve as anything else for our scale) | Yes — USB hardware token (mandatory since June 2023) | High — token logistics, GitHub Actions integration via Azure Key Vault or self-hosted runner | **Not recommended** |
| **E. EV cert (DigiCert/Sectigo)** | $400-560/yr | $400-560/yr | **Same as OV since March 2024** | Yes — USB hardware token | High — same as D | **Not recommended** — buying EV in 2026 for a non-driver Windows app is value destruction |

**Assumptions and caveats:**
- `[VERIFIED: SSL.com FAQ]` and `[VERIFIED: Microsoft Q&A]` confirm both EV and OV require hardware tokens since June 2023.
- `[VERIFIED: signpath.org]` SignPath Foundation provides free OSS code signing via a shared certificate; eligibility requires a public GitHub repo (we have one), code-review process, and MFA.
- `[VERIFIED: certum.store + community blog posts]` Certum's OSS Code Signing is the cheapest paid option but requires a physical smartcard.
- `[CITED: cabforum.org via SSL.com 2026 changelog]` Effective March 1, 2026 (already past), max validity for publicly-trusted code-signing certificates is **458 days** — annual renewal is now the only option for paid certs.
- `[ASSUMED]` SignPath's aggregate reputation builds faster than a single 12-user project's would on its own. The premise is sound (SignPath signs many OSS projects under one cert) but I have not confirmed a specific timeline. Real-world UX should be validated before declaring Path B "solved."

**Default decision (Phase 2 ships against this unless user overrides):**
1. Apply for SignPath Foundation sponsorship (Plan in Phase 2 should include this as a parallel async task — application + waiting period happens off the critical path).
2. Ship Phase 2 unsigned with a walkthrough README section + 30-second screen recording linked from the GitHub Release notes.
3. If SignPath approves before Phase 5 completion, retrofit signing into the `goreleaser` pipeline (this is a single config-block change, not a re-architecture).
4. If SignPath denies AND a guildie reports the unsigned UX as a blocker, fall back to Certum OSS (€69 + €30/yr).
5. Never buy EV; almost certainly never buy OV.

### Core Stack (verified against current `go.mod` 2026-05)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/minio/selfupdate` | v0.6.0 (latest) | Auto-update binary swap | Active fork of `inconshreveable/go-update`; handles Windows running-binary swap via the .new/.old rename pattern (verified against `apply.go` source). `[VERIFIED: github.com/minio/selfupdate tags]` |
| `golang.org/x/oauth2` | v0.36.0 (already pinned) | Token refresh + `invalid_grant` detection | Already in use; `oauth2.RetrieveError` is the canonical type for matching `invalid_grant`. |
| `google.golang.org/api/sheets/v4` | latest (already pinned) | Sheets writes incl. backoff via gax | Built-in retry uses `gax.ExponentialBackoff`; standard pattern is to wrap the call and inspect `googleapi.Error.Code`. `[CITED: blog.salrashid.dev/articles/2021/exponential_backoff_retry]` |
| `googleapis/gax-go/v2` | v2.18.0 (already pinned, transitive) | Exponential backoff | Used by Sheets client internally; we read `googleapi.Error` returned to us. |
| `gopkg.in/natefinch/lumberjack.v2` | v2.2.1 (already pinned) | Log rotation | Already used; new code logs through the same `slog` handler. |

### Supporting (CI / build only)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `goreleaser` | v2.x (latest stable) | Multi-target Go release pipeline | Replace the hand-rolled GitHub Actions release stub; gives single-config sign hook for SignPath/AzureSignTool when we adopt signing. `[CITED: goreleaser.com/customization/sign/binary_sign/]` |
| AzureSignTool (only if path C/D/E chosen) | v6+ | CLI signtool that talks to Azure Key Vault / cloud HSM | Standard tool for cloud-backed code signing in CI; not required for default path A or path B (SignPath provides its own integration). `[CITED: github.com/vcsjones/AzureSignTool]` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `time.AfterFunc` self-reschedule for heartbeat | `github.com/robfig/cron/v3` v3.0.1 | `[VERIFIED: github.com/robfig/cron tags]` Library is real and stable but adds a dep + goroutine pool for ONE recurring job. Robfig's repo also has 50+ open PRs and panic bugs since 2020 maintenance slowed `[CITED: github.com/netresearch/go-cron readme]`. **Use `time.AfterFunc`.** |
| `time.AfterFunc` self-reschedule for heartbeat | `github.com/go-co-op/gocron/v2` | Active, fluent API, support for distributed leader election. Same dep-bloat objection — we have ONE job. **Reject.** |
| Hand-rolled delay slice for retries | `github.com/cenkalti/backoff/v4` | Mature, jitter, context-aware. But WATCH-07 specifies a fixed `2/4/8/16/32/60s` schedule — a 6-element `[]time.Duration` is simpler than configuring backoff to match. **Hand-roll the slice.** |
| Hand-rolled `latest.json` fetch | `github.com/blang/semver/v4` for version compare | We need to compare `1.2.3` < `1.2.4`. semver is correct but stdlib `strings.Split` + `strconv.Atoi` is fine for a 3-part numeric scheme. **Stdlib.** |
| `goreleaser` | Keep hand-rolled CI stub | Stub works but every sign-related change becomes a YAML rewrite. `goreleaser` consolidates: ldflags injection, NSIS invocation, SHA-256, manifest, signing hook — all in one `.goreleaser.yaml`. **Adopt goreleaser** in Phase 2. |
| SignPath OSS (path B) | Open Source Developer Code Signing from Certum (path C) | Certum is paid but immediate; SignPath is free but eligibility-gated and slower to provision. **Run them in parallel.** |

### Installation (additions to existing `go.mod`)

```bash
go get github.com/minio/selfupdate@v0.6.0
# Nothing else; cron / backoff libs explicitly NOT added (see Don't Hand-Roll table above)
```

### Build pipeline migration

```bash
# Add to repo root
brew install goreleaser  # or scoop install goreleaser on Windows dev box; CI gets the GH Action
# Bootstrap config
goreleaser init
# Hand-edit .goreleaser.yaml to:
#   - mirror current ldflags injection (-X main.OAuthClientID etc.)
#   - call makensis as a post-build hook
#   - emit latest.json
#   - leave a `signs:` block stubbed-but-disabled for path-B/C wiring later
```

**Version verification:** `[VERIFIED]` (2026-05-01)
- `minio/selfupdate v0.6.0` — current latest tag per GitHub API.
- `robfig/cron v3.0.1` — current latest tag per GitHub API (referenced only as "considered and rejected").
- `goreleaser v2.x` — `[CITED: goreleaser.com]` v2 is the active major.

---

## Architecture Patterns

### System Architecture Diagram

```
                          +-----------------------------+
                          |  Windows logon (HKCU\Run)   |
                          +--------------+--------------+
                                         |
                                         v
+--------------------------+  startup    +---------------------+
| %LOCALAPPDATA%\Programs\ +------------>| squirebot.exe       |
| SquireBot\               |             |                     |
|   squirebot.exe          |             | 1. update.Apply()    |
|   squirebot.exe.new ?    |  swap on    |    (if .new exists, |
|   squirebot.exe.old ?    |  startup    |    rename, hide old)|
+--------------------------+             | 2. config.Load       |
                                         | 3. wincred.Read     |
                                         | 4. sheet.Validate   |
                                         | 5. ScaffoldSchema   |
                                         |    (idempotent)     |
                                         | 6. Start watchers   |
                                         |    a. inventory     |
                                         |    b. spellbook     |
                                         | 7. Start heartbeat  |
                                         |    (24h tick)       |
                                         | 8. Start update     |
                                         |    poll (24h tick)  |
                                         | 9. Tray loop        |
                                         +----+--------+-------+
                                              |        |
                       fsnotify event          |        | googleapi.Error
                       (debounced)             |        | (transient or invalid_grant)
                                               v        v
                     +-------------------------+  +-----+------------------+
                     | parse + WriteInventory  |  | Retry decision tree    |
                     | / WriteSpellbook (atomic|  | - 429 -> Retry-After    |
                     |   batchUpdate)          |  | - 5xx -> exp backoff   |
                     +-----+--------------------+  | - 403 once -> refresh |
                           |                       | - invalid_grant ->    |
                           v                       |   tray RED, prompt   |
                     +-----+----------------+      +------------------------+
                     | UpsertCharOwner +    |
                     | _audit row            |
                     +-----------------------+

                     Daily, regardless of file events:
                     +-----------------------+      +------------------------+
                     | heartbeat.Tick        +----->| batchUpdate              |
                     |  (one-call write,     |      |   _char_owner.last_seen |
                     |   all active chars)   |      |   for every active char |
                     +-----------------------+      +------------------------+

                     Daily, GitHub Releases:
                     +-----------------------+      +------------------------+
                     | update.Check          +----->| GET latest.json         |
                     |  (compare embedded    |      | verify SHA-256          |
                     |   Version vs latest)  |      | download to .new        |
                     |                       |      | (do NOT swap; swap     |
                     |                       |      |  happens at next start)|
                     +-----------------------+      +------------------------+
```

### Recommended Project Structure

```
internal/
  app/                # exists; gains heartbeat + update startup wiring
  auth/               # exists; gains invalid_grant detection helper (oauth.go)
  config/             # exists; extend cfg.EQFolders []string for WATCH-03; add LastKnownInventoryMtime map[string]time.Time for WATCH-09
  heartbeat/          # NEW; one job, one ticker, one batchUpdate-per-tick
  parse/              # exists; add spellbook.go + spellbook_test.go (after WATCH-05 sample collection)
  scaffold/           # NEW; ScaffoldSchemaV1(client) — idempotent; touched once at startup
  sheet/              # exists; add WriteSpellbook, WriteHeartbeat, WriteStatus, EnsureAllTabs; extend meta.go for new _meta rows
  tray/               # exists; add HealthState=NeedsReauth (red + click reopens OAuth)
  update/             # NEW; minio/selfupdate consumer; latest.json fetch + SHA-256 verify + .new staging + startup swap
  watch/              # exists; extend to multi-folder + spellbook suffix
installer/
  squirebot.nsi       # exists; ensure HKCU\...\Run write + uninstall removal
.goreleaser.yaml      # NEW; replaces hand-rolled .github/workflows/release.yml steps
```

### Pattern 1: Idempotent schema scaffold (one-time, runs every startup, no-ops if complete)

**What:** A single function `scaffold.ScaffoldSchemaV1(ctx, sc)` that ensures every Phase 2-frozen tab and every required `_meta` row exists. Runs unconditionally on startup. Cheap (single batched read of all sheet IDs + at-most-N writes for missing pieces).

**When to use:** Every cold start of the watcher, after `ValidateWorkbook` succeeds.

**Why:** Schema lock at end of Phase 2 means the workbook layout is fixed forever; this function is the bootstrap that gets a freshly-shared workbook to that fixed layout. After the first successful run, the function is a no-op (every `EnsureSheet` returns the cached sheetId; every `_meta` row already exists). Idempotency matters because a guildie may re-share or re-pick the workbook, and the bootstrap must not corrupt existing data.

### Pattern 2: Startup binary swap (NEVER replace running binary)

**What:** On startup, before doing anything else, check for `squirebot.exe.new` adjacent to the running binary. If present and SHA-256-verifies against the embedded `expected_sha256` value (passed in via `latest.json` saved alongside the .new file), invoke `selfupdate.Apply` with the `.new` reader. The library renames running .exe → .old, renames .new → .exe, then **on Windows attempts to hide (not delete) the .old file** because Windows holds the lock on the running process even via that path. `[VERIFIED: minio/selfupdate apply.go]`

**When to use:** Every cold start. Skip if no `.new` file present (the common path).

**Why:** `[VERIFIED]` Windows file-locking semantics block in-process replacement. The library's pattern works around this by deferring the actual swap to a moment when no process owns the running binary's handle (i.e., the next process launch, before we do work). The hand-off relies on three guarantees: (a) the `.new` file was written to disk completely (atomic write contract), (b) the SHA-256 matches the manifest (corruption check), (c) the rename is atomic (Windows MoveFile is atomic for same-volume renames).

### Pattern 3: Multi-folder watcher with two suffix dispatchers

**What:** Replace the single-folder fsnotify loop in `internal/watch/watcher.go` with: one fsnotify watcher per folder in `cfg.EQFolders`, each routing events through the existing 500ms debouncer, with the dispatch function inspecting the basename suffix and routing to either `inventoryHandler(path)` or `spellbookHandler(path)`.

**When to use:** Always — replaces the Phase 1 single-folder, inventory-only watcher.

**Why:** WATCH-02 (spellbook) and WATCH-03 (multi-folder) are coupled in implementation: the natural extension point is the existing fsnotify loop. Splitting into two completely separate watcher goroutines (one per file type) buys nothing because they share zero state. Splitting into N watchers (one per folder) is necessary because a single `fsnotify.Watcher` can `Add` multiple paths but you still want one debouncer queue per path — easier to reason about with one per folder.

### Pattern 4: Startup catch-up (WATCH-09)

**What:** On startup, after `ScaffoldSchemaV1` and before starting the live watcher, walk every folder in `cfg.EQFolders`, list every `<Char>-Inventory.txt` and `<Char>-Spellbook.txt`, compare each file's mtime against `cfg.LastKnownInventoryMtime[char]` / `cfg.LastKnownSpellbookMtime[char]`. For each newer file, synthesize an `onChange` call into the same handler the live watcher uses. Update `LastKnownInventoryMtime` after each successful upload.

**When to use:** Every cold start, after the schema scaffold and before entering the fsnotify loop.

**Why:** A guildie who runs SquireBot 5 minutes a day and `/outputfile inventory`s while it's not running would otherwise lose those snapshots forever. The catch-up loop closes that gap without requiring polling. mtime on Windows is reliable for "did this file change since we last saw it" within ~1s precision, which is more than adequate for our cadence.

### Pattern 5: Schema scaffold contents (the freeze list)

**What gets created if absent during `ScaffoldSchemaV1`:**

**Hidden dimension tabs (`_`-prefixed, `sheet.hideSheet()`):**

| Tab | Columns at v1 freeze (extend right only after this point) |
|-----|----------------------------------------------------------|
| `_meta` | A: key, B: value (KV style) — required keys: `schema_version=1`, `canonical_id=squirebot-v1-workbook-2026`, `bank_toon_name`, `bank_coin_pp`, `bank_coin_gp`, `bank_coin_sp`, `bank_coin_cp`, `last_pigparse_refresh`, `last_wiki_summary_refresh`, `last_wiki_spell_refresh`, `last_wiki_gear_refresh`, `last_quest_items_refresh`, `last_error` |
| `_char_owner` | `char_name, owner_email, display_name, discord_handle, class, level, is_bank_toon, is_hidden, is_removed, first_seen, last_seen, server, watcher_version` (13 cols — locked at v1) |
| `_item_master` | `item_id, name, wiki_summary, wiki_url, slot, is_quest_item, last_refreshed` (7 cols) |
| `_pigparse` | `item_id, name, current_avg, last_seen, blue_volume, last_refreshed` (6 cols) |
| `_wiki_spells` | `class, level, spell_name, normalized_name, last_refreshed` (5 cols) |
| `_wiki_gear_tier` | `tier, class, slot, item_id, item_name, rank, last_refreshed` (7 cols) |
| `_quest_items` | `item_id, quest_name, source_url, last_refreshed` (4 cols) |
| `_audit` | `timestamp, owner_email, char_name, file_type, rows_written, watcher_version` (6 cols) |
| `_status` | `owner_email, char_name, watcher_version, last_inventory_upload, last_spellbook_upload, last_heartbeat` (6 cols) |

**Visible consolidated mega-tab placeholders (empty for Phase 2; Phase 3+ populates):**

| Tab | Header row only at Phase 2 freeze |
|-----|-----------------------------------|
| `view` | `Char, Slot, Item, ID, Count, Wiki, Price, Last Synced` |
| `gear_check` | `Char, Class, Tier, Slot, Have, Recommended, Status` |
| `spell_check` | `Char, Class, Level, Spell, Status` |
| `bank` | `Char, Slot, Item, ID, Count, Wiki, Price, Last Synced` (mirrors `view` shape; populated only with bank toon's data in Phase 4) |

**Per-character landing tab template (created on first sighting of `<Char>` by the watcher):**

| Tab | Columns |
|-----|---------|
| `inv:<Char>` | `Location, Name, ID, Count, Slots, _uploaded_at` (already shipped Phase 1) |
| `spell:<Char>` | `Slot, Name, _uploaded_at` (NEW Phase 2; depends on WATCH-05 verification of the actual `/outputfile spellbook` shape) |

**Anti-pattern:** Do NOT pre-create `inv:<Char>` and `spell:<Char>` tabs for unknown characters in `ScaffoldSchemaV1`. Landing tabs are created lazily on first sighting (already the Phase 1 pattern). Pre-creation would either (a) require knowing the character roster up front (we don't), or (b) waste cells in the 10M cap.

### Pattern 6: Auto-update startup-swap (concrete sequence)

```
On startup, before everything else:
1. exePath := os.Executable()
2. Check for $exePath.new — if absent, return (nothing to do).
3. Read $exePath.expected-sha256 (a sidecar file written when the .new was downloaded).
4. Compute SHA-256 of $exePath.new.
5. If mismatch:
     - Delete .new and .expected-sha256
     - Log + return (treat as no update; next daily check will re-attempt)
6. Open $exePath.new for read.
7. Call selfupdate.Apply(reader, selfupdate.Options{TargetPath: exePath}).
   The library:
     a. Writes new content to .target.new (we already had it — minor redundancy)
     b. Renames running .exe to .target.old
     c. Renames .target.new -> .exe
     d. On Windows attempts to delete .old; on failure, hides it.
8. Re-exec self via syscall.Exec or os.StartProcess + os.Exit so the
   newly-named binary takes over (the goroutine in this process is still
   the OLD binary).
9. Done — control passes to the new binary.

Daily update check (separate goroutine on a 24h ticker):
1. GET https://github.com/<owner>/SquireBot/releases/latest/download/latest.json
2. Parse {version, installer_url, installer_sha256, released_at}.
3. If parse.version <= embedded Version, return (already current).
4. Download the binary archive from installer_url to $exePath.new.tmp.
5. Verify SHA-256.
6. Atomically rename .tmp -> .new.
7. Write expected SHA-256 sidecar.
8. Surface "Update ready, restart to apply" in tray status.
9. Do NOT swap now (running binary is locked). Swap happens at next launch.
```

### Anti-Patterns to Avoid

- **Hot-swap of the running .exe in-process** (without selfupdate's startup-swap pattern): on Windows this returns `ERROR_SHARING_VIOLATION`. `[VERIFIED: PITFALLS.md Pitfall 14]`
- **`USER_ENTERED` valueInputOption on hot-path writes:** triggers recalc storms and can drop leading zeros on item IDs. Already enforced in `internal/sheet/write.go`; hold the line for spellbook + heartbeat + scaffold writes.
- **Per-character heartbeat = N batchUpdate calls per fire:** burns quota and creates 12-watcher fan-out concerns. Use ONE batchUpdate that touches every active character's `last_seen` cell.
- **Polling for autostart correctness:** don't have the watcher edit `HKCU\...\Run` at startup to "ensure autostart is on." The installer owns that registry key; the uninstaller removes it. If a guildie disables it, that's deliberate.
- **Hand-rolled cron for one job:** see `## Don't Hand-Roll`. `time.AfterFunc` self-rescheduled.
- **Synchronous `signtool.exe` invocation in `goreleaser` from a developer machine:** signing belongs in CI with the cert in a vault, not on a laptop where a stolen laptop = stolen signing key. (Moot if we go with default path A — no signing.)
- **Treating spellbook 2-col format as confirmed before WATCH-05 sample collection:** every Phase 2 plan that depends on the spellbook format MUST be gated on the developer's sample-collection task. If the actual format is different (e.g., 3 columns including category), the schema scaffold shape changes.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Replacing a running .exe on Windows | A custom .new/.old rename routine | `github.com/minio/selfupdate.Apply` | Multi-step Windows file-lock dance; library handles file-handle close, rename ordering, hide-instead-of-delete on the .old residue, and rollback-on-failure. `[VERIFIED: apply.go source]` |
| Detecting `invalid_grant` from a generic error | `if strings.Contains(err.Error(), "invalid_grant")` | `errors.As(err, &oauth2.RetrieveError{})` then check `.ErrorCode` | The string-match approach silently breaks on i18n or library wording changes; `oauth2.RetrieveError` is the typed shape `[CITED: pkg.go.dev/golang.org/x/oauth2]` |
| Sheets API exponential backoff for transient errors | A `for-range delays { sleep; try; if err == nil break }` loop | `googleapi.Error` switch + the WATCH-07-mandated fixed delay slice | The Sheets client already returns `*googleapi.Error` with `.Code` and headers; we only need a switch to decide retry vs. surface. Don't reimplement gax. `[CITED: github.com/googleapis/google-api-go-client]` |
| `Retry-After` header parsing on 429 | A custom string-to-duration | `net/http.Header.Get("Retry-After")` + `strconv.Atoi` (seconds) or `http.ParseTime` (HTTP-date) | RFC 7231 specifies two forms; both are short stdlib parses. |
| Cron / scheduled job scheduling for the heartbeat | `robfig/cron`, `gocron`, or a goroutine pool | `time.NewTicker(24*time.Hour)` or `time.AfterFunc` self-reschedule | One job, no cron expression complexity, no DST concerns (UTC ticks are fine), no panic-recovery middleware needed (we own the goroutine). |
| Cross-platform code-signing wrapper | Custom signtool.exe invocation script | `goreleaser`'s `signs:` block + `AzureSignTool` (path C/D/E) or `signpath-action` (path B) | Mature CI integrations exist; rolling our own loses the abstraction over the credential storage layer (cloud HSM vs. smartcard vs. local cert). |
| HTTP client for GitHub Releases manifest fetch | A bespoke `net/http` client | Stdlib `net/http.Client` with a timeout | Single GET, JSON unmarshal — no library needed. The temptation is "let's add `go-github`"; resist. |
| SHA-256 verification | A streaming hash-and-buffer routine | `crypto/sha256.Sum256` for files <100 MB; `io.Copy(hasher, file)` otherwise | Stdlib. We'll never hit the >100 MB threshold (binary is ~12 MB, installer ~25 MB). |
| Version comparison | `golang.org/x/mod/semver` | `strings.Split` + `strconv.Atoi` for our 3-part numeric scheme | We control the version format. A 6-line compare is more readable than the import. |

**Key insight:** Phase 2 is full of "tempting library, real cost is one screen of code" choices. Default to stdlib unless the library handles a non-trivial Windows ceremony (`minio/selfupdate`) or is already in the project (`oauth2`, `sheets/v4`, `lumberjack`, `wincred`).

---

## Common Pitfalls

### Pitfall A: `invalid_grant` is multi-shape

**What goes wrong:** PITFALLS.md Pitfall #1 (Testing-mode 7-day expiry) and #7 (six-month inactivity, password change, user-revoked, refresh-token cap) all surface as `invalid_grant`. Different root causes, identical wire response. If we only detect "the literal string `invalid_grant`," we'll miss the moments Google returns `unauthorized_client` (revoked OAuth client), `invalid_client` (client deleted from GCP project), or wraps the error with extra context. The watcher silently retries forever; the guildie sees nothing on the tray.

**Why it happens:** The OAuth 2 RFC defines a small enumeration of error codes; Google extends it; client libraries variously expose them as a typed error or as an `*url.Error`-wrapping. Naive `errors.Is` against a sentinel misses all of them.

**How to avoid:**
- Use `errors.As(err, &retErr)` against `*oauth2.RetrieveError` (the canonical typed shape `[CITED: pkg.go.dev/golang.org/x/oauth2#RetrieveError]`).
- Match `retErr.ErrorCode` against `"invalid_grant"`, `"unauthorized_client"`, `"invalid_client"` — all are non-recoverable, all should drive the same UX (red tray, click reopens OAuth).
- For ANY other error, fall through to the WATCH-07 backoff schedule.

**Warning signs:** Watcher logs show `invalid_grant` but tray remains green; guildie's `last_seen` stops advancing while the process is still running; `cmdkey /list` shows the wincred entry still present (it WILL still be there — the refresh token is no longer accepted, but it's not deleted).

### Pitfall B: 403 might be quota OR might be revoked-scope

**What goes wrong:** WATCH-07 says "refresh token once on `403`." But Google returns 403 for both "your access token has expired (transient — refresh and retry)" AND "this OAuth client no longer has the `drive.file` scope on this file (permanent — re-OAuth needed)." Refreshing once and retrying on the latter silently retries forever, drains quota, and never surfaces the real problem.

**Why it happens:** HTTP 403 is generic; the discriminator is in the JSON body's `error.errors[0].reason` field, which varies (`authError`, `forbidden`, `insufficientPermissions`, `userRateLimitExceeded`).

**How to avoid:**
- After one refresh+retry on a 403, if the second attempt also returns 403, check `googleapi.Error.Errors[0].Reason`.
- `authError` or `insufficientPermissions` → permanent; treat as `invalid_grant` (red tray, prompt re-OAuth).
- `userRateLimitExceeded` → fall through to backoff schedule.
- `forbidden` (no further detail) → treat as permanent (defensive default; better to surface than to silently spin).

**Warning signs:** Repeated 403s in `squirebot.log` with no tray transition; CPU/network usage proportional to retry count; sheet reads succeed in browser but watcher writes fail (suggests scope drift, not workbook deletion).

### Pitfall C: Scaffold runs against the wrong workbook

**What goes wrong:** A guildie clicks "Change Workbook…" and picks an unrelated spreadsheet. The schema scaffold runs against it and creates 13 hidden tabs in the guildie's vacation-planning sheet. That sheet now looks like a SquireBot workbook to the canonical-id check, but it has none of the actual data.

**Why it happens:** Scaffold is unconditional after `ValidateWorkbook` succeeds, but `ValidateWorkbook` is exactly what `_meta.canonical_id` is supposed to gate — and on a fresh-shared workbook from the guild leader, `_meta.canonical_id` does not exist yet either. The scaffold has to decide: do we run on a workbook that has neither matching nor mismatching canonical_id?

**How to avoid:**
- `ValidateWorkbook` returns one of three states: `MatchesCanonical` (run scaffold no-op), `Empty` (run scaffold, write canonical_id), `WrongCanonical` (refuse — `ErrWrongWorkbook`).
- "Empty" requires positive evidence: the sheet has no `_meta` tab AT ALL (not just no `canonical_id` row). A workbook that has a `_meta` tab without `canonical_id` is suspect — refuse to scaffold and surface the same `ErrWrongWorkbook` message.
- The wizard's Picker step (Phase 1) already validates pre-existing-canonical-id workbooks; Phase 2 needs to add the "fresh shared workbook" path explicitly.

**Warning signs:** A guildie reports their personal sheet now has a hidden `_meta` tab; `_audit` writes succeeding to a workbook that has no `inv:*` tabs from anyone.

### Pitfall D: Heartbeat batchUpdate races with an inventory write

**What goes wrong:** The heartbeat tick fires at the same instant a guildie saves an inventory file. Two batchUpdate calls race; the heartbeat write touches a `_char_owner` row, the inventory write touches an `inv:<Char>` tab. Different tabs, but the Sheets API can serialize them differently than the call order. Worst case: both observe the same `_char_owner` snapshot, both write, the heartbeat write wins (because it's smaller and finishes faster), and the inventory `last_inventory_upload` field is stale by ~24h.

**Why it happens:** The Sheets API guarantees per-request atomicity but NOT cross-request ordering. Without a serialization point in the watcher, two goroutines hitting the API "simultaneously" produce non-deterministic interleaving.

**How to avoid:**
- All sheet writes go through a single `sheet.Client` instance, and `sheet.Client` adopts a `sync.Mutex` around the `*sheets.Service.Spreadsheets.BatchUpdate` call. (Currently the `client.go` doc-comment says "single Client is safe for serial use only" — Phase 2 needs to enforce that with a mutex now that two goroutines exist: the watcher loop and the heartbeat ticker.)
- The mutex serializes API calls, not in-process work — no concurrency benefit lost on parsing/marshalling.
- The single mutex is also the right place to enforce WATCH-07's backoff: a transient error → re-acquire delay → retry, with the mutex preventing a second goroutine from blowing through the same backoff.

**Warning signs:** `_status.last_heartbeat` newer than `_status.last_inventory_upload` despite a guildie just having uploaded; race-condition reports in the log when the slog handler interleaves "heartbeat write" and "inventory write" with the same start timestamp.

### Pitfall E: SmartScreen MOTW behavior diverges across browsers

**What goes wrong:** PITFALLS.md Pitfall #2 documents the SmartScreen wall, but the precise UX depends on Mark-of-the-Web (MOTW) and which browser/process wrote the zone identifier. Edge + default settings = full blue panel. Chrome on the same machine = a different "Keep / Discard" prompt. Firefox = no MOTW at all (so SmartScreen never engages, but Defender SmartScreen can still flag at runtime). Our walkthrough video can therefore be wrong for any guildie not using Edge.

**Why it happens:** MOTW is a Windows alternate-data-stream tag set by browsers when they download from "internet" zones. Different browsers tag with different verbosity; some don't tag at all. Microsoft's SmartScreen flow is gated on MOTW presence + signing reputation.

**How to avoid:**
- The walkthrough must say "if you see a 'Windows protected your PC' blue panel, click More info → Run anyway" — and ALSO cover the non-blue prompt shapes (Chrome's Keep/Discard, Firefox's no-prompt).
- The README should recommend Edge or Chrome explicitly so the walkthrough video is accurate for the most-likely path.
- INST-05 acceptance is "documented walkthrough completes in <30 seconds" — the documentation must include screenshots of all three browser flows OR explicitly limit support to one browser.

**Warning signs:** A guildie reports "I didn't get a SmartScreen prompt" (most likely Firefox / no MOTW path — install proceeds silently but Defender may still quarantine later); guildies on macOS attempting to install (out-of-scope, but document the rejection).

### Pitfall F: `goreleaser init` overwrites the existing release.yml

**What goes wrong:** `goreleaser init` generates `.goreleaser.yaml` AND optionally a `.github/workflows/release.yml`. If we run init naively, it clobbers the carefully-built Phase 1 release stub with the consent-screen check, ldflags injection, NSIS step, and SHA-256 manifest emission.

**Why it happens:** `goreleaser init` is designed for greenfield projects.

**How to avoid:**
- Do NOT run `goreleaser init` against the existing `.github/workflows/release.yml`.
- Hand-author `.goreleaser.yaml` referencing the existing structure; commit the workflow's pre-build oauth-config validation step verbatim.
- The migration is: replace the `Build squirebot.exe` + `Build NSIS installer` + `Compute SHA-256` + `Write latest.json` steps with a single `goreleaser release` invocation. Keep the consent-screen gate and the OAUTH_CONFIG_JSON materialization OUTSIDE goreleaser as workflow steps.

**Warning signs:** A diff that touches release.yml and removes the AUTH-03 PRODUCTION gate is a STOP-WORK signal.

---

## Code Examples

### Example 1: NSIS Run-key for autostart (INST-04)

```nsis
; In Section "Install"
; Per-user autostart -- HKCU, no UAC.
WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot" "$INSTDIR\squirebot.exe"

; In Section "Uninstall"
DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "SquireBot"
```

`[CITED: nsis.sourceforge.io WriteRegStr docs]` — Standard NSIS pattern; the `HKCU` (not `HKLM`) is the load-bearing detail per Locked Decision #2.

### Example 2: Sheets API retry with WATCH-07 schedule

```go
// internal/sheet/retry.go (NEW)
package sheet

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

// retrySchedule is the exact 6-step backoff WATCH-07 mandates.
var retrySchedule = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	32 * time.Second,
	60 * time.Second,
}

// withRetry runs op with the WATCH-07 retry policy. Returns the FINAL
// error if every attempt failed; nil on first success. The reauth callback
// is invoked at most once on a 403 with reason=authError|insufficientPermissions
// before a single retry; if the second attempt also fails as permanent,
// returns ErrPermanentAuth.
type AuthTransition int

const (
	AuthOK AuthTransition = iota
	AuthRequired // surface to tray as red
)

var ErrPermanentAuth = errors.New("permanent auth failure -- re-OAuth required")

func withRetry(ctx context.Context, op func() error, onRefresh func() error) error {
	refreshed := false
	for attempt := 0; ; attempt++ {
		err := op()
		if err == nil {
			return nil
		}
		var ge *googleapi.Error
		if !errors.As(err, &ge) {
			// Non-Google error (network, etc.) -- treat as transient.
			if attempt >= len(retrySchedule) {
				return fmt.Errorf("non-googleapi error after %d attempts: %w", attempt, err)
			}
			if waitErr := sleep(ctx, retrySchedule[attempt]); waitErr != nil {
				return waitErr
			}
			continue
		}
		switch ge.Code {
		case http.StatusTooManyRequests: // 429
			d := retrySchedule[min(attempt, len(retrySchedule)-1)]
			if ra := parseRetryAfter(ge.Header.Get("Retry-After")); ra > 0 {
				d = ra
			}
			if waitErr := sleep(ctx, d); waitErr != nil {
				return waitErr
			}
		case http.StatusForbidden: // 403
			reason := ""
			if len(ge.Errors) > 0 {
				reason = ge.Errors[0].Reason
			}
			if reason == "authError" || reason == "insufficientPermissions" || reason == "forbidden" {
				if refreshed {
					return ErrPermanentAuth // surface to tray
				}
				refreshed = true
				if rerr := onRefresh(); rerr != nil {
					return fmt.Errorf("token refresh after 403: %w", rerr)
				}
				continue // retry immediately after refresh, don't burn a backoff slot
			}
			// userRateLimitExceeded -- transient, fall through
			if attempt >= len(retrySchedule) {
				return fmt.Errorf("403 (%s) after %d attempts", reason, attempt)
			}
			if waitErr := sleep(ctx, retrySchedule[attempt]); waitErr != nil {
				return waitErr
			}
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 5xx
			if attempt >= len(retrySchedule) {
				return fmt.Errorf("5xx after %d attempts: %w", attempt, err)
			}
			if waitErr := sleep(ctx, retrySchedule[attempt]); waitErr != nil {
				return waitErr
			}
		default:
			// 400, 404, anything else -- not transient, surface immediately.
			return err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
```

`[CITED: pkg.go.dev/google.golang.org/api/googleapi#Error]` confirms `*googleapi.Error.Header`, `.Code`, and `.Errors[].Reason` are the documented inspection points. The library does not expose `Retry-After` parsing; we use stdlib.

### Example 3: `invalid_grant` detection at the TokenSource level

```go
// internal/auth/refresh.go (NEW)
package auth

import (
	"errors"
	"strings"

	"golang.org/x/oauth2"
)

// IsRevokedRefreshToken returns true if err is the canonical Google
// "this refresh token is permanently dead, prompt re-OAuth" shape. It
// covers all four invalidation modes documented in PITFALLS.md Pitfall #7:
// Testing-mode 7-day expiry, six-month inactivity, user-revoked, password
// change with token-revoke setting; plus the OAuth-spec sibling errors
// returned when the client itself has been deleted/suspended.
func IsRevokedRefreshToken(err error) bool {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		switch re.ErrorCode {
		case "invalid_grant", "unauthorized_client", "invalid_client":
			return true
		}
	}
	// Defensive fallback: some wrapping paths produce a generic error
	// whose message contains the OAuth code. Last-resort match; the typed
	// path above should catch nearly all real cases.
	if err != nil {
		s := strings.ToLower(err.Error())
		if strings.Contains(s, "invalid_grant") ||
			strings.Contains(s, "unauthorized_client") ||
			strings.Contains(s, "invalid_client") {
			return true
		}
	}
	return false
}
```

`[CITED: pkg.go.dev/golang.org/x/oauth2#RetrieveError]` is the canonical typed shape. The `RetrieveError.ErrorCode` field has been stable since v0.10.0.

### Example 4: Daily heartbeat with `time.AfterFunc` self-reschedule

```go
// internal/heartbeat/heartbeat.go (NEW)
package heartbeat

import (
	"context"
	"log/slog"
	"time"

	"github.com/boejowen/SquireBot/internal/sheet"
)

const interval = 24 * time.Hour

// Run blocks until ctx is cancelled. Fires WriteHeartbeat once on entry
// (so a watcher started after a long downtime updates _char_owner.last_seen
// promptly), then every 24 hours thereafter.
//
// Implementation note: time.AfterFunc with self-reschedule (rather than
// time.Ticker) so a hung WriteHeartbeat doesn't queue ticks. WATCH-08
// tolerates a missed tick more easily than a piled-up backlog.
func Run(ctx context.Context, sc *sheet.Client) {
	tick := func() {
		if err := sc.WriteHeartbeat(ctx); err != nil {
			slog.Warn("heartbeat write failed", "err", err)
			// Non-fatal: WATCH-07 retry already happened inside WriteHeartbeat.
			// The next tick will re-attempt.
		} else {
			slog.Info("heartbeat written")
		}
	}
	tick() // immediate first fire
	for {
		t := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			tick()
		}
	}
}
```

### Example 5: Auto-update startup-swap

```go
// internal/update/swap.go (NEW)
package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/selfupdate"
)

// Apply checks for a staged update (squirebot.exe.new + .expected-sha256
// sidecar) adjacent to the running binary, verifies the hash, and uses
// minio/selfupdate to atomically swap the running .exe at startup. Caller
// MUST invoke Apply before any other startup work; if Apply returns nil
// AND swapped==true, caller must os.Exit(0) so the swapped binary takes
// over on next launch. If swapped==false, normal startup proceeds.
//
// On any error the staging files are deleted (so a corrupted .new doesn't
// re-trigger every launch) and the function returns the error -- caller
// logs and continues with the OLD binary.
func Apply() (swapped bool, err error) {
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("os.Executable: %w", err)
	}
	stagedPath := exe + ".new"
	hashPath := exe + ".expected-sha256"

	if _, err := os.Stat(stagedPath); errors.Is(err, os.ErrNotExist) {
		return false, nil // common path -- no update staged
	}

	expected, err := os.ReadFile(hashPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		return false, fmt.Errorf("read sidecar hash: %w", err)
	}
	expectedHex := strings.TrimSpace(string(expected))

	staged, err := os.Open(stagedPath)
	if err != nil {
		return false, fmt.Errorf("open staged: %w", err)
	}
	defer staged.Close()

	h := sha256.New()
	if _, err := io.Copy(h, staged); err != nil {
		return false, fmt.Errorf("hash staged: %w", err)
	}
	actualHex := hex.EncodeToString(h.Sum(nil))
	if actualHex != expectedHex {
		_ = os.Remove(stagedPath)
		_ = os.Remove(hashPath)
		return false, fmt.Errorf("staged hash mismatch: have %s, want %s", actualHex, expectedHex)
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("rewind staged: %w", err)
	}

	if err := selfupdate.Apply(staged, selfupdate.Options{TargetPath: exe}); err != nil {
		// Library performs its own rollback on failure; if we're here,
		// rollback also failed -- leave the .new in place for next attempt.
		return false, fmt.Errorf("selfupdate.Apply: %w", err)
	}

	// Cleanup -- the .new is consumed; the sidecar is no longer relevant.
	_ = os.Remove(stagedPath)
	_ = os.Remove(hashPath)

	// Best-effort: the library hides the .old on Windows; we attempt a
	// delete (will succeed once the OS releases the lock at next launch).
	_ = os.Remove(filepath.Clean(exe + ".old"))

	slog.Info("auto-update applied", "exe", exe)
	return true, nil
}
```

`[CITED: pkg.go.dev/github.com/minio/selfupdate#Apply]` confirms the `Apply(io.Reader, Options)` signature; `[VERIFIED: apply.go source]` confirms the .new/.old rename pattern and the Windows hide-instead-of-delete behavior.

---

## State of the Art

| Old Approach (PITFALLS.md / STACK.md said...) | Current Approach (2026-05) | When Changed | Impact |
|-----------------------------------------------|---------------------------|--------------|--------|
| EV cert grants instant SmartScreen reputation | EV and OV are equivalent on SmartScreen since March 2024 | March 2024; OIDs removed from MS Trusted Root Program August 2024 | **Inverts the recommended cert path.** Default now is unsigned + walkthrough or SignPath OSS. |
| EV cert costs $300-600/yr and is "preferred" | Same price band, but no longer worth paying for | March 2024 | $300-600/yr saved. |
| OV cert hardware-token storage is "optional" | Mandatory since June 2023 (CA/B Forum requirement) | June 2023 | Token logistics cost is now a fixed barrier to OV/EV adoption — adds to the case for SignPath/unsigned. |
| Code-signing certs can be issued for 2-3 years | Max 458 days since March 2026 | March 2026 (CA/B Forum) | If we ever buy a cert, plan for annual renewal as a recurring task. |
| `inconshreveable/go-update` is the auto-update library | `minio/selfupdate` is the active fork | ~2020 (minio fork); STACK.md already reflects this | None — already locked. |
| Apps Script Rhino runtime | V8 (Rhino EOL 2026-01-31) | 2026-01-31 | Phase 3 concern, not Phase 2. Mentioned for completeness. |

**Deprecated/outdated:**
- "EV preferred" cert recommendation in `PITFALLS.md` Pitfall #2 — **outdated; should be amended in a Phase 2 docs sweep.**
- "EV (~$300-600/yr) is overkill for 12 users" framing in `STACK.md` "Code-signing certificate" row — partially right (EV is overkill), partially wrong-for-the-wrong-reason (it's not just overkill; it gives no SmartScreen advantage). Same docs sweep.
- "self-signed certs are *worse* than unsigned" (PITFALLS.md Pitfall #2) — still accurate `[VERIFIED: Microsoft SmartScreen docs]`; do NOT use a self-signed cert.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | SignPath Foundation eligibility for SquireBot is plausible (public GitHub repo, real OSS license, MFA on GitHub account) | Code-Signing Recommendation Matrix path B | If denied, fallback path is unsigned + walkthrough OR Certum OSS €69. No critical-path blocker. |
| A2 | SignPath's aggregate reputation builds SmartScreen trust faster than a single 12-user project's would | Code-Signing Recommendation Matrix path B | If false, path B becomes equivalent to path A on the SmartScreen UX axis. Still no cost, still recommended for the brand-recognition signal in the consent dialog, but the "faster reputation" claim should be removed. |
| A3 | The hand-rolled CI release.yml can be cleanly migrated to `goreleaser` while preserving the AUTH-03 consent-screen gate as a pre-step | Standard Stack §goreleaser | If the migration is more invasive than expected, fall back to keeping the stub and adding signing-only steps. Stretch decision; not a phase blocker. |
| A4 | The spellbook file format is 2-column `Slot \t Name` | Architecture Patterns §5 | WATCH-05 explicitly calls this out as needing verification before schema lock. If 3+ columns, `spell:<Char>` schema must extend at the right edge before being frozen. |
| A5 | `_meta` as KV (column A=key, B=value) is the right shape for the row count we'll have at v1 | Architecture Patterns §5 | Alternative is a flat-row schema. KV is more flexible for `last_*_refresh` proliferation in later phases; flat is faster to read. KV is the right call at our scale. |
| A6 | A 24-hour heartbeat interval matches WATCH-08's "once daily" intent | Code Examples §4 | If "daily" means "at a fixed wall-clock time" (e.g., 09:00 local), the implementation needs cron-like semantics. Recommend confirming with user that interval-based is acceptable. |
| A7 | `googleapi.Error.Errors[0].Reason` is reliably populated for 403 responses in 2026 | Common Pitfalls Pitfall B | If Google starts returning 403s with empty `Errors` in some paths, the reason-based dispatch needs a fallback (default to permanent on empty reason, which is what the example code does). |
| A8 | A SmartScreen walkthrough README + 30-second screen recording is sufficient for INST-05 acceptance | Code-Signing Recommendation Matrix path A | If guildies can't make it through unsigned (high-trust-radius assumption), fallback chain is SignPath then Certum. |
| A9 | `time.AfterFunc` self-reschedule is preferable to `time.Ticker` for the heartbeat | Code Examples §4 | Both work; Ticker is simpler. AfterFunc avoids tick-pile-up on a hung write. The choice is a judgment call; either is correct. |

---

## Open Questions

1. **SignPath OSS application timeline.** The application form takes "minutes" but approval can be "1-4 weeks" per blog posts. Should Phase 2 block on signing, or ship unsigned and retrofit? **Recommendation:** ship unsigned, apply in parallel; retrofit if approved by Phase 5.

2. **Heartbeat write content beyond `last_seen`.** WATCH-08 says "one-cell write." Given we're already paying for a batchUpdate, should we also refresh `_status.last_heartbeat` and `_status.watcher_version` in the same batch? **Recommendation:** yes — same API call, no extra cost; surfaces watcher liveness in two tabs (one keyed on character, one keyed on owner_email), useful for OPS-05.

3. **`_meta.canonical_id` rotation.** What is the documented procedure if the canonical_id constant ever needs to bump (e.g., a v2 workbook layout that's not extend-compatible)? **Recommendation:** out of scope for Phase 2; document as a Phase 5+ migration concern.

4. **NSIS uninstaller scope.** Does the uninstaller delete `%LOCALAPPDATA%\SquireBot\config.json` and the wincred entry, or preserve them for a possible re-install? **Recommendation:** offer a checkbox in the uninstaller; default to "preserve" to avoid surprising guildies who reinstall after Windows reset.

5. **WATCH-09 catch-up interaction with WATCH-08 heartbeat.** If startup catch-up uploads 5 characters' worth of inventory, that already updates each `last_seen` indirectly (via `UpsertCharOwner`). Does the heartbeat tick still need to fire immediately, or skip until the next 24h boundary? **Recommendation:** still fire immediately; heartbeat is the source of truth for "watcher is alive," and a missed first-fire violates the WATCH-08 contract.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Local builds | ✓ | 1.26.2 (verified `go version`) | — |
| `go.mod` deps already pinned | Existing watcher build | ✓ | See go.mod listing | — |
| NSIS 3.10+ | Installer build | ✓ (CI installs via choco; local dev uses installed copy or remote build) | 3.10+ verified in CI workflow | — |
| `goreleaser` | Replacement CI | ✗ (not yet in repo / dev box) | Needs `scoop install goreleaser` or GH Action | Keep hand-rolled `release.yml` with sign-step grafted on |
| `signtool.exe` (only if path C/D/E) | Local code signing | ✗ (default path A/B doesn't need it) | Windows SDK | N/A for default |
| AzureSignTool (only if path D/E) | Cloud-HSM signing | ✗ | `dotnet tool install azuresigntool` | N/A for default |
| GitHub Releases as `latest.json` host | Auto-update manifest fetch | ✓ | Already in use | — |
| Test EQ-output sample for spellbook (WATCH-05) | Locking spell:<Char> schema | ✗ (developer collects inline) | — | Block schema-lock plan on collection |

**Missing dependencies with no fallback:** None for default path A. `goreleaser` is recommended-but-optional.

**Missing dependencies with fallback:**
- `goreleaser` — fallback is the existing hand-rolled CI stub. Adopting goreleaser is a quality-of-life improvement, not a hard requirement.
- Spellbook sample file — fallback is to gate the spellbook-related plans on the developer collecting one. Phase 2 itself is not blocked; the schema-lock plan within it is.

---

## Security Domain

> Phase 2 inherits the Phase 1 OAuth threat model unchanged. Adding auto-update + signing surfaces a new threat class (compromised release pipeline) that Phase 1 did not have.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | OAuth 2.0 PKCE loopback (Phase 1, locked); `client_secret` baked in is acceptable per Google docs |
| V3 Session Management | yes | Refresh token in DPAPI-backed wincred (Phase 1, locked); permanent failure → forced re-OAuth (this phase, AUTH-05) |
| V4 Access Control | partial | `drive.file` scope confines access to user-picked workbook; no per-character ACLs in v1 (universal-visibility, accepted) |
| V5 Input Validation | yes | TSV parser already enforces 5-column inventory shape; spellbook parser must do the same on first sample |
| V6 Cryptography | yes | SHA-256 verification of staged update binary (this phase, OPS-04); NEVER hand-roll — use `crypto/sha256` stdlib |
| V8 Data Protection at Rest | yes | wincred + DPAPI; `%LOCALAPPDATA%\SquireBot\config.json` MUST NOT contain refresh token (Phase 1, locked) |
| V9 Communications | yes | All Sheets API + GitHub Releases calls over HTTPS via stdlib `net/http`; no cert pinning needed (the system root trust store is the right scope at our threat model) |
| V10 Malicious Code | yes | Self-update verifies SHA-256 against a manifest signed by... the GitHub Releases TLS cert. Without code signing (path A), the integrity chain ends at "you trust the manifest." With path B (SignPath), integrity chain extends to a signed `.exe`. |
| V14 Configuration | yes | `_meta.schema_version=1` + `WATCHER_MAX_SCHEMA_VERSION` constant; mismatch refuses to write (Phase 1, locked; reapply on every Phase 2 startup after scaffold) |

### Known Threat Patterns for {Go watcher + GitHub Releases + Sheets API}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Compromised release: attacker pushes a malicious tag and writes a `latest.json` pointing to their binary | Tampering / Elevation of Privilege | Repo branch protection on `main`; required reviewers on release tag creation; the SHA-256 in `latest.json` is only as trustworthy as the `latest.json` itself — code signing closes this gap, the unsigned default does NOT. **Risk accepted at our scale; document explicitly.** |
| Compromised CI: stolen `OAUTH_CONFIG_JSON` secret → attacker mints valid SquireBot tokens for any guildie | Spoofing | Repo secret access is GitHub-managed (encrypted at rest, redacted in logs); the consent screen still forces user click for first-run OAuth; rotation procedure documented |
| Spoofed `latest.json` via DNS hijack | Tampering | HTTPS to GitHub Releases CDN; domain pinning is GitHub's job |
| Auto-update downgrade attack: attacker presents a stale, vulnerable manifest | Tampering | Compare embedded `Version` against manifest `version`; only swap if NEWER. (The example code in §Auto-update startup-swap implements this.) |
| Local file-system attacker with read access to `%LOCALAPPDATA%\SquireBot\config.json` | Information Disclosure | Refresh token never written there (Phase 1 locked); only spreadsheet ID, EQ folder, email — all low-sensitivity |
| Local file-system attacker with read access to `%LOCALAPPDATA%\SquireBot\squirebot.exe.new` between download and swap | Tampering | SHA-256 verification at swap time catches tampering; window is <24h; mitigation acceptable at our threat model |
| Heartbeat misuse: malicious watcher writes false `last_seen` for another guildie | Spoofing | Watchers can only write to their own per-character ranges; `_char_owner.owner_email` first-write-wins; mismatch surfaces in `_audit`. (Phase 1 already enforces this for inventory; extend to heartbeat in this phase.) |
| Schema scaffold runs against attacker-shared workbook | Tampering | `ValidateWorkbook` three-state machine (Pitfall C above) refuses to scaffold against a workbook with a mismatched `_meta` |

---

## Sources

### Primary (HIGH confidence)

- [SSL.com FAQ — EV vs OV code signing](https://www.ssl.com/faqs/which-code-signing-certificate-do-i-none-ev-ov/) — confirms hardware-token mandate since June 2023, 458-day validity cap from March 2026
- [Microsoft Q&A — EV reputation removal](https://learn.microsoft.com/en-us/answers/questions/417016/reputation-with-ov-certificates-and-are-ev-certifi) — confirms EV no longer grants instant SmartScreen reputation as of March 2024
- [Microsoft Learn — SmartScreen reputation for Windows app developers](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation) — canonical Microsoft documentation on the reputation system
- [Microsoft Learn — Code signing options](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options) — overview of cert types and use cases
- [SignPath Foundation (free OSS code signing)](https://signpath.org/) — recommended path B for SquireBot
- [SignPath Foundation terms for OSS projects](https://signpath.org/terms.html) — eligibility requirements
- [Certum Open Source Code Signing](https://certum.store/open-source-code-signing-code.html) — recommended fallback path C; €69 one-time + €30/yr
- [github.com/minio/selfupdate](https://github.com/minio/selfupdate) — locked auto-update library; v0.6.0 verified latest tag
- [minio/selfupdate apply.go (source)](https://raw.githubusercontent.com/minio/selfupdate/master/apply.go) — directly verified the .new/.old rename pattern, hide-on-Windows behavior, fp.Close() requirement
- [pkg.go.dev/golang.org/x/oauth2 — RetrieveError](https://pkg.go.dev/golang.org/x/oauth2#RetrieveError) — typed error shape for `invalid_grant` detection
- [pkg.go.dev/google.golang.org/api/googleapi — Error](https://pkg.go.dev/google.golang.org/api/googleapi#Error) — `.Code`, `.Header`, `.Errors[].Reason` for retry decisions
- [GoReleaser — Signing binaries](https://goreleaser.com/customization/sign/binary_sign/) — `signs:` block contract for path C/D/E integration
- [GitHub: vcsjones/AzureSignTool](https://github.com/vcsjones/AzureSignTool) — cloud-HSM signtool wrapper for path D/E

### Secondary (MEDIUM confidence — verified but pricing/timelines volatile)

- [DigiCert EV Code Signing pricing](https://signmycode.com/digicert-ev-code-signing) — $559.99/yr current (verified 2026-05); volatile
- [Sectigo Code Signing pricing](https://signmycode.com/sectigo-ev-code-signing) — $279.99/yr EV, $211.46/yr OV (verified 2026-05); volatile
- [Certum Open Source Code Signing review (community blog)](https://www.msz.it/a-cheap-code-signing-certificate-for-open-source-projects-by-certum-asseco-an-honest-review-walkthrough/) — confirms the smartcard + reader fulfillment flow
- [github.com/robfig/cron tags](https://github.com/robfig/cron/tags) — v3.0.1 latest; library is stable but maintenance is slow
- [github.com/netresearch/go-cron README](https://github.com/netresearch/go-cron) — fork that documents robfig's 50+ open PR backlog
- [Salrashid Jain blog on Google API Go retry](https://blog.salrashid.dev/articles/2021/exponential_backoff_retry/) — explanation of gax exponential backoff in the Google client library

### Tertiary (LOW confidence — single source, validate before relying)

- SignPath OSS approval timeline ("1-4 weeks") — based on community reports; no official Service Level commitment from SignPath. Validate by tracking application date if Phase 2 plans include applying.
- "SignPath's aggregate reputation builds SmartScreen trust faster than a single project's" — plausible (shared cert across many projects), but no quantitative data found. Treat as an extra reason to apply, not as a guarantee.

### Sibling research (already-locked decisions consulted)

- `.planning/research/STACK.md` — locked Phase 1 stack; Phase 2 must respect
- `.planning/research/ARCHITECTURE.md` — sheet schema design (per-character view tab proposal SUPERSEDED by SUMMARY.md's consolidated mega-tab decision; Phase 2 freezes the consolidated layout)
- `.planning/research/PITFALLS.md` — 27-pitfall catalogue; Pitfall #2 (SmartScreen) needs amendment per State of the Art table above; Pitfalls #3, #4, #5, #7, #11, #12, #13, #14, #16 are directly relevant to Phase 2 plans
- `.planning/research/SUMMARY.md` — synthesis; confirmed consolidated-view-tab decision and Phase 2 scope
- `.planning/STATE.md` — Decisions Log entry #10 (client_secret with PKCE) is the load-bearing Phase 1 lesson preserved into Phase 2

---

## Metadata

**Confidence breakdown:**
- Code-signing recommendation matrix: HIGH — multiple primary sources cross-verify the EV/OV equivalence change, the hardware-token mandate, the 458-day cap; SignPath and Certum eligibility are well-documented.
- `minio/selfupdate` Windows mechanics: HIGH — `apply.go` source directly inspected; matches PITFALLS.md Pitfall #14 description.
- Sheets API retry semantics: HIGH — `googleapi.Error` typed shape + `RetrieveError` typed shape are stable Go OAuth2 / Google API conventions.
- Schema scaffold contents: HIGH — directly derived from already-locked decisions in REQUIREMENTS.md (SCHEMA-01..08) and ARCHITECTURE.md (with SUMMARY.md's consolidated-view override applied).
- `goreleaser` migration: MEDIUM — pattern is well-documented but we have not run it against the existing release.yml; risk that the AUTH-03 consent-screen gate doesn't slot cleanly into the goreleaser shape.
- Heartbeat semantics: MEDIUM — choice of `time.AfterFunc` vs `time.Ticker` is a judgment call; "once daily" interpretation is ambiguous (interval vs wall-clock); A6 + Open Question 5.

**Research date:** 2026-05-01
**Valid until:** 2026-06-01 for the cert-vendor pricing rows; 2026-08-01 for the rest. (Cert pricing is the most volatile data point; everything else is more stable.)
