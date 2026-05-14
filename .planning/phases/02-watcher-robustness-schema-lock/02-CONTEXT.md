# Phase 2: Watcher Robustness + Schema Lock — Context

**Gathered:** 2026-05-01
**Status:** Ready for planning
**Source:** Synthesized from interactive chat decisions (in lieu of formal `/gsd-discuss-phase` run) + Phase 2 RESEARCH.md findings

<domain>
## Phase Boundary

**In scope (per ROADMAP.md Phase 2):**
- Spellbook watcher (parser + fsnotify glob for `<CharName>-Spellbook.txt`, atomic batchUpdate writes to `spell:<Char>` landing tabs)
- Windows logon autostart via `HKCU\...\Run` (no Task Scheduler, no Service)
- Sheets API exponential backoff (2/4/8/16/32/60s), `Retry-After` honoring on 429, single token refresh on 403, persistent-failure surfacing via tray icon
- Refresh-token UX: detect `invalid_grant`, turn tray red, click reopens consent — no silent retry loops
- Once-daily heartbeat write to `_char_owner.last_seen` (and `_status.last_heartbeat` / `_status.watcher_version`) per active character even when no source file changed
- Auto-update: `latest.json` on GitHub Releases, SHA-256 verify, atomic startup-swap (never in-process)
- Code-signing path: ship UNSIGNED + SmartScreen walkthrough (README/video), apply for SignPath OSS in parallel, do not block release
- Sheet schema FROZEN at `schema_version=1` — every tab and every column the design will ever use is present (even unused ones), so Phase 3+ never breaks landing-tab consumers
- `goreleaser` adopted to replace the Phase 1 hand-rolled GitHub Actions release stub
- Spellbook file format verification against committed real EQ sample (`internal/parse/testdata/Slampeach-Spellbook.txt`)
- NSIS uninstaller behavior: checkbox to wipe `config.json` + wincred entry, default = preserve

**Out of scope (deferred to later phases):**
- Apps Script / TypeScript / clasp scaffolding (Phase 3)
- PigParse / wiki scrape harness (Phase 3)
- Consolidated `view`, `gear_check`, `spell_check`, `bank` rendering (Phase 3 / Phase 4)
- Code-signing migration (whenever SignPath OSS is approved — handled as a follow-up plan or v1.x patch)
- README / SmartScreen video polish (Phase 5 — this phase produces the walkthrough text but not the polished video)

</domain>

<decisions>
## Implementation Decisions

### Code Signing (Q1 — answered: ship unsigned, apply in parallel)
- **Default release path is UNSIGNED.** Documented SmartScreen "More info → Run anyway" walkthrough is the deliverable. Walkthrough must complete in under 30 seconds (per ROADMAP success criterion 5).
- **Apply for SignPath Foundation OSS** code signing in parallel (free, eligibility-gated, 1–4 week wait). Tracked as a Phase 2 task but does NOT block Phase 2 completion.
- **Never buy EV.** Microsoft removed EV's instant-SmartScreen-reputation perk in March 2024 (OIDs removed from Trusted Root Program August 2024). EV ≡ OV on UX axis now. Locked.
- **Paid fallback (only if SignPath OSS denied AND user opts in later):** Certum OSS (€69 one-time + €30/yr smartcard).
- The auto-update pipeline must work with both unsigned and signed binaries — no signing-aware code paths in `selfupdate` integration.

### Heartbeat Cadence (Q2 — answered: interval, not wall-clock fixed)
- **24-hour rolling interval** from process start, NOT a fixed wall-clock daily time (e.g. not "every day at 03:00").
- Implementation: `time.AfterFunc(24*time.Hour, fire)` self-rescheduling pattern (per RESEARCH.md §6).
- Heartbeat batchUpdate writes BOTH `_char_owner.last_seen` AND `_status.last_heartbeat` + `_status.watcher_version` in the same call (no extra cost; surfaces watcher version centrally).
- Persistence across restarts: a guildie who runs SquireBot 5 minutes a day still gets a heartbeat each session — fire on startup if last_seen > 23h ago, then schedule next 24h fire.

### NSIS Uninstaller (Q3 — answered: checkbox, default preserve)
- Uninstaller presents a **checkbox**: "Also delete saved configuration and Google account credentials"
- **Default = unchecked (preserve).** Re-installing later resumes work without re-OAuth.
- When checked: deletes `%LOCALAPPDATA%\SquireBot\config.json` AND removes the wincred entry under `SquireBot:<email>`.
- Always (regardless of checkbox): removes binary, autostart `HKCU\...\Run` entry, Start Menu shortcut, log files in `%LOCALAPPDATA%\SquireBot\logs\`.

### Spellbook File Format (Q4 — answered: real sample committed, schema rename)
- File pattern: `<CharName>-Spellbook.txt` (e.g., `Slampeach-Spellbook.txt`).
- Two columns, tab-separated: **`Level`** (integer 1–60), **`Name`** (string).
- **`Slot` → `Level` rename is a Phase 2 task**: rename column across CLAUDE.md (architecture overview), `.planning/research/ARCHITECTURE.md` (sheet schema spec), and `.planning/research/SUMMARY.md` (stack overview). Phase 1 docs/code that say `Slot` for spellbook column are wrong; the values prove level-not-slot.
- Test fixture: `internal/parse/testdata/Slampeach-Spellbook.txt` (committed `7f814d2`, 49 spells, level range 9–53). Parser must round-trip this fixture.
- Spellbook file has NO spell IDs — Phase 4 `spell_check` join is by normalized spell name (case/whitespace-folded).
- Spellbook file has NO mem-slot information — it's the scribed list, not the 8 active mem slots.

### Schema Lock (`schema_version=1`)
- **Extend-only forever after this point.** Any later breaking change requires `_meta.schema_version` bump + idempotent migration + watcher's `WATCHER_MAX_SCHEMA_VERSION` check + script's `SCRIPT_MIN_SCHEMA_VERSION` check.
- **All landing tabs:** `inv:<Char>` (cols `Location, Name, ID, Count, Slots, _uploaded_at`), `spell:<Char>` (cols `Level, Name, _uploaded_at`).
- **All hidden `_`-prefixed dimension tabs created with full column scaffold even though Phase 2 doesn't populate most of them:**
  - `_meta` (cols: `key, value` — populated rows include `schema_version=1`, `canonical_id`, `created_at`, `bank_toon_name`, `last_error`)
  - `_char_owner` (cols: `char_name, owner_email, class, level, last_seen, is_hidden, is_removed, discord_handle` — soft-delete and discord-handle columns scaffolded even though no v1 UI populates them)
  - `_item_master` (cols: `item_id, name, wiki_url, summary, last_synced` — empty in Phase 2, Phase 3 populates)
  - `_pigparse` (cols: `item_id, name, price_pp, last_synced` — empty in Phase 2)
  - `_wiki_spells` (cols: `class, level, spell_name, normalized_name, wiki_url` — empty in Phase 2)
  - `_wiki_gear_tier` (cols: `tier, slot, item_name, item_id, class_filter, race_filter, wiki_url` — empty in Phase 2)
  - `_quest_items` (cols: `item_id, item_name, quest_name, wiki_url` — empty in Phase 2)
  - `_audit` (cols: `timestamp, actor, action, details` — empty in Phase 2)
  - `_status` (cols: `key, value` — populated rows include `last_heartbeat`, `watcher_version`, `cell_count`, `last_full_refresh`)
- **Consolidated mega-tab placeholders created (visible, leading `Char` column, otherwise empty):**
  - `view`, `gear_check`, `spell_check`, `bank`
- **NEVER per-character view tabs.** Hard architectural rule (Google's 200-tab/workbook limit).
- Workbook canonical-id state machine: three states must be handled explicitly — `MatchesCanonical`, `Empty` (fresh shared workbook — Phase 2 scaffolds it), `WrongCanonical` (picker chose wrong sheet — refuse to write, prompt re-pick). Phase 1's picker only handled `MatchesCanonical`; Phase 2 must add `Empty` and `WrongCanonical` paths.

### Concurrency
- **New pitfall (D from RESEARCH.md):** Heartbeat goroutine and watcher goroutine share a single `sheet.Client`. Phase 2 MUST add a `sync.Mutex` around `Spreadsheets.BatchUpdate` calls. The existing `client.go` doc-comment says "single Client is safe for serial use only" but doesn't enforce it. This is a gate task in the plan that introduces the heartbeat goroutine.

### Sheets Retry/Backoff
- **Do NOT add `cenkalti/backoff` or any external backoff library.** Use the WATCH-07-mandated fixed slice `2/4/8/16/32/60s` + switch on `googleapi.Error.Code` and `googleapi.Error.Errors[0].Reason`.
- The `google.golang.org/api/sheets/v4` library already does internal gax retries; we only handle surfaced errors at the boundary (after gax has given up).
- `Retry-After` header on 429: parse and honor (overrides our backoff slice for that one retry).
- `403` with reason `userRateLimitExceeded` or `rateLimitExceeded` → backoff per slice. `403` with reason `forbidden` or `permissionDenied` → permanent, surface via tray.
- `401` (and `oauth2.RetrieveError` with `error="invalid_grant"`) → permanent, surface via tray (red), click reopens consent flow.

### Auto-Update (`minio/selfupdate`)
- **Startup-swap pattern, NEVER in-process.** Windows refuses to replace a running .exe; library uses `.target.new` / `.target.old` rename dance + hides `.target.old` (OS won't let it be deleted while any handle is open).
- Update flow: on tray "Check for updates" or once-daily background check, fetch `latest.json` from GitHub Releases → compare semver → fetch new binary → SHA-256 verify → write to `<exepath>.new` → set "pending update" flag in `config.json` → on next startup, before main goroutine starts, do the swap and continue with new binary.
- `latest.json` schema: `{ "version": "1.x.y", "url": "https://github.com/.../squirebot.exe", "sha256": "...", "released": "2026-..." }`
- Rollback: if startup-swap fails (corrupt download), keep `.target.old` and surface via tray red icon. Manual recovery = uninstall/reinstall.

### Release Pipeline (`goreleaser`)
- Adopt `goreleaser` to replace the Phase 1 hand-rolled `release.yml` GitHub Actions stub.
- Preserve the **AUTH-03 PRODUCTION consent-screen gate** as a workflow PRE-STEP outside `goreleaser` — the gate is a manual checkpoint (a `gh workflow run` input or a labeled-PR check) that ensures the OAuth consent screen is not in Testing mode at release time. `goreleaser init` would clobber the existing release.yml; the migration plan must preserve this gate explicitly.
- Sign step in goreleaser config is left as a no-op for now (unsigned path); when SignPath OSS is approved, the sign step gets wired in via SignPath's GitHub Action without restructuring the release pipeline.

### Refresh-Token UX
- Distinguish transient `403` (refresh once and retry) from permanent `invalid_grant` (force re-consent). Implement via switch on `oauth2.RetrieveError.Response.Body` (parse JSON, check `error` field).
- On `invalid_grant`: tray icon → red, log structured event, suspend all watcher writes (don't keep replaying), surface "Reauthorize" menu item that opens the OAuth loopback flow again.
- On successful re-auth: replace wincred entry, resume watcher, log structured "reauthorized" event with timestamp.

### Claude's Discretion
- Concrete error log structure and field names (use Phase 1's existing structured logging; extend as needed).
- Tray icon red/green/yellow asset choices (use existing Phase 1 icons; add a red variant if missing).
- Specific exit codes for the auto-update startup-swap success/fail (any sane convention).
- Exact menu wording in tray (defer to copy-microcopy guidance; "Check for updates", "Reauthorize", "Open log folder", "Quit" are all acceptable starting points).
- Whether to commit `Slampeach-Inventory.txt` as a matched-pair fixture for the inventory parser. RESEARCH.md flagged the inventory fixture (`sample-inventory.txt`) as still using Phase 1's abstract naming — Phase 2 planner can decide whether to rename it for consistency or defer.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase-local
- `.planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md` — Code-signing matrix, retry/backoff schedule, heartbeat scheduling pattern, auto-update startup-swap mechanics, schema-scaffold table, new pitfall D (sheet.Client mutex)

### Project-wide
- `CLAUDE.md` — Project conventions and locked decisions (NOTE: documents `Slot` column for spellbook — Phase 2 must update to `Level`)
- `.planning/PROJECT.md` — Core value, constraints, key decisions
- `.planning/REQUIREMENTS.md` — REQ-IDs covered: INST-04, INST-05, AUTH-05, WATCH-02, WATCH-03, WATCH-05–09, SCHEMA-01–08, OPS-04, OPS-05
- `.planning/ROADMAP.md` Phase 2 section — Six locked success criteria
- `.planning/STATE.md` — Phase 1 lessons, locked decisions log (especially decision #10 — Google /token requires client_secret with PKCE)

### Phase 2 RESEARCH.md sections to read explicitly
- `## Standard Stack` — Libraries to use
- `## Architecture Patterns` — Task structure
- `## Don't Hand-Roll` — Anti-patterns
- `## Common Pitfalls` — Verification triggers
- `## Code Examples` — Reference patterns

### Init research (Phase 0)
- `.planning/research/STACK.md` — Locked stack (NOTE: §code-signing recommends EV — that's now wrong; defer to Phase 2 RESEARCH.md §1)
- `.planning/research/ARCHITECTURE.md` — Sheet schema spec (NOTE: documents `Slot` column for spellbook — Phase 2 must update to `Level`)
- `.planning/research/PITFALLS.md` — 27-pitfall catalogue (NOTE: Pitfall #2 is now wrong; defer to Phase 2 RESEARCH.md §1)
- `.planning/research/SUMMARY.md` — Phase 0 synthesis (NOTE: documents `Slot` column for spellbook — Phase 2 must update to `Level`)

### Test fixtures
- `internal/parse/testdata/sample-inventory.txt` — Phase 1 inventory fixture (Phase 2 may rename for naming consistency)
- `internal/parse/testdata/sample-inventory-with-cp1252.txt` — Phase 1 CP-1252 encoding fixture
- `internal/parse/testdata/Slampeach-Spellbook.txt` — Phase 2 spellbook fixture (Slampeach SK, 49 spells)

</canonical_refs>

<specifics>
## Specific Ideas

- The schema-lock plan should produce ONE idempotent migration function in Apps Script-or-watcher (TBD which side owns scaffolding) that can be run safely against any existing workbook to bring it to `schema_version=1`. Phase 1 workbooks (already populated with `inv:<Char>` tabs and `_meta.canonical_id`) must scaffold the missing tabs/columns without disturbing existing data.
- The schema-scaffold logic must be safe to run against an EMPTY shared workbook chosen in the picker — fresh first-time setup creates every tab and writes `_meta.schema_version=1` + `_meta.canonical_id`.
- Watcher writes the full schema scaffold itself on first contact with a workbook (since Phase 3 hasn't shipped Apps Script yet) — this means the scaffolding code lives in Go for Phase 2.
- Spellbook fixture parser test: parse `internal/parse/testdata/Slampeach-Spellbook.txt`, assert exactly 49 rows, assert column 1 values are valid integers in [1, 60], assert column 2 values are non-empty strings, assert no duplicate (Level, Name) pairs.
- The `goreleaser` migration plan should produce a working `release.yml` that builds + uploads + generates `latest.json` on a `v*` tag push, retains the AUTH-03 PRODUCTION-mode pre-step from the existing stub, and emits SHA-256 sums for the auto-updater to verify.

</specifics>

<deferred>
## Deferred Ideas

- **SignPath OSS application** — file in Phase 2 but approval/integration is async. If approval lands during Phase 2 execution, integrate via a follow-up plan; otherwise it ships in a 1.x patch.
- **Workbook eviction workflow** (`is_removed` UI) — Phase 5 owns the UI. Phase 2 only scaffolds the columns.
- **Discord pinger** — v2-deferred per PROJECT.md. Phase 2 only scaffolds the `_char_owner.discord_handle` column.
- **Cell-count alarm threshold (5M)** — Phase 4 owns the alarm. Phase 2 only scaffolds the `_status.cell_count` row.
- **Inventory-fixture rename for naming consistency** — flagged but planner can defer.

</deferred>

---

*Phase: 02-watcher-robustness-schema-lock*
*Context gathered: 2026-05-01 via interactive chat synthesis (post-research)*
