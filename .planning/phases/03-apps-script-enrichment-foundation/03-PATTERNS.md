# Phase 3 Patterns — File-by-File Closest Analogs

**Date:** 2026-05-09
**Method:** Direct survey of `internal/sheet/`, `internal/scaffold/`, `internal/parse/`, `.github/workflows/`. The pattern-mapper subagent timed out; this doc was produced by orchestrator-direct inspection.

---

## §1. Critical drift discovered (read first — fixes CONTEXT.md assumptions)

Phase 2's `internal/scaffold/scaffold.go` is the source of truth for the schema, and it diverges from what I wrote in 03-CONTEXT.md:

### `_pigparse` columns (Phase 2 scaffold)
**Actual:** `item_id, name, current_avg, last_seen, blue_volume, last_refreshed` (6 cols)
**Phase 3 needs:** WTS-vs-WTB direction split + multi-period averages (per RESEARCH §1)
**Resolution:** schema_version=2 migration adds 6 columns at the right edge: `direction` (0/1/2 per PigParse), `t30, a30, t60, a60, t6m, a6m, ty, ay`. Existing `current_avg` becomes a derived alias for `a30` (or gets repurposed; planner decides).

### `_item_master` columns (Phase 2 scaffold)
**Actual:** `item_id, name, wiki_summary, wiki_url, slot, is_quest_item, last_refreshed` (7 cols — INCLUDES is_quest_item)
**Phase 3 needs:** all of these are already there. Add `wikitext_sha1` for change-detection (1 column at right edge, not strictly required for v2 bump but useful).

### `_quest_items` columns (Phase 2 scaffold)
**Actual:** `item_id, quest_name, source_url, last_refreshed` (4 cols — already exists)
**Phase 3 needs:** add `source` column (`'in_game_flag' | 'notes_link'`) at right edge.

### `_meta` rows already populated (Phase 2 scaffold)
**Already present:** `schema_version, canonical_id, bank_toon_name, bank_coin_pp, bank_coin_gp, bank_coin_sp, bank_coin_cp, last_pigparse_refresh, last_wiki_summary_refresh, last_wiki_spell_refresh, last_wiki_gear_refresh, last_quest_items_refresh, last_error`
**Phase 3 must ADD:** `theme` (default `minimalist`), `contact_email` (initially blank — user-deferred per CONTEXT). Append to MetaRows slice in scaffold.go OR write directly from Apps Script's first run (preferred — keeps Phase 3 changes on the TS side).

### View tab `view` columns (Phase 2 scaffold)
**Actual:** `Char, Slot, Item, ID, Count, Wiki, Price, Last Synced` (8 cols — note: `Slot` not `Location`, no `Slots`)
**Phase 3 must respect this.** RESEARCH §3 proposed `Char | Location | Item | ID | Count | Slots | Price | Wiki | Last Synced` — that was WRONG. The scaffold won.

`bank` columns are identical to `view`. Same column set, same order.

### `WatcherMaxSchemaVersion`
**Actual:** `internal/sheet/client.go:40` declares `WatcherMaxSchemaVersion = 1`. Phase 3 bumping `_meta.schema_version` to 2 will trigger `ErrSchemaTooNew` in `ValidateWorkbook` — **the watcher will refuse to write**. Phase 3 MUST coordinate: bump the constant to 2 in the Go side as part of the Apps Script migration plan, ship as v0.3.0.

---

## §2. Files Phase 3 will create — closest analogs

### `apps-script/build.mjs`
**Analog:** none in repo (first JS build script). Recommended structure: see RESEARCH §6 — single `esbuild.build()` call producing IIFE bundle + footer that re-exports trigger functions as globals.

### `apps-script/package.json`
**Analog:** none (no `package.json` exists in repo today). Pin `@google/clasp@^2.4.2` (per RESEARCH §6 — 3.x has breaking changes), `esbuild@^0.20.0`, `@types/google-apps-script@^1.0.91`, `vitest@^1.6.0` for tests.

### `apps-script/.clasp.json.example`
**Analog:** loose analogy to `docs/oauth-setup.md` (per-installation config that lives outside the repo). Contains `{ "scriptId": "<your-workbook-bound-script-id>", "rootDir": "./dist" }`. The real `.clasp.json` is gitignored.

### `apps-script/appsscript.json`
**Analog:** none. Hand-written manifest. Scopes: `spreadsheets.currentonly`, `script.external_request`, `script.scriptapp`, `script.container.ui` (per RESEARCH §6).

### `apps-script/src/Code.ts` (entry, registers triggers)
**Analog:** loose to `cmd/squirebot/main.go` — a thin entry that wires together the actual logic from sub-packages. Re-exports `refreshPigparse`, `refreshWikiItems`, `onChange`, `onOpen`, `buildView`, `buildBank`, `setTheme` for the build-script footer to lift to globals.

### `apps-script/src/lib/politeFetch.ts`
**Analog:** `internal/sheet/retry.go` — exponential backoff schedule (Phase 2 uses `2,4,8,16,32,60`s; RESEARCH §5 proposes `2,4,8,16,32`s for politeFetch — slightly tighter since this is courtesy not API-quota). Same general structure: switch on response code, honor `Retry-After`, surface failure after exhausting retries. **Steal:** the pattern of returning a structured `{ status, body, fromCache, retriesUsed }` discriminated-union type.

### `apps-script/src/triggers/refreshPigparse.ts`
**Analog:** `internal/sheet/heartbeat.go` — single function that composes a batchUpdate + writes via the mutex-funneled helper. TS analog: single function that fetches, validates row count, builds row arrays, calls `Range.setValues()` inside `tryLock`/`finally`. **Steal:** the doc-comment-at-top-of-file pattern (heartbeat.go's preamble explains the algorithm and which Pitfalls it defends against — replicate in TS).

### `apps-script/src/triggers/refreshWikiItems.ts`
**Analog:** `internal/sheet/owner.go`'s upsert-by-key pattern (read existing rows by key, decide insert vs update vs no-op). For the resumable cursor: closest analog is `internal/sheet/heartbeat.go`'s `time.AfterFunc(24*time.Hour, fire)` self-rescheduling pattern — TS equivalent is `ScriptApp.newTrigger().timeBased().after(60000).create()`.

### `apps-script/src/tabs/buildView.ts` and `buildBank.ts`
**Analog:** `internal/scaffold/scaffold.go`'s `ScaffoldSchemaV1` — full-snapshot rebuild semantics. Read all input data → compose 2D array → single `Range.setValues()` write. Unlike scaffold's idempotent skip-if-present approach, build is full-replace each time.

### `apps-script/src/lib/themes.ts`
**Analog:** `internal/scaffold/scaffold.go`'s `DimensionTabs` slice — a const array as source of truth that callers iterate. TS: `THEMES: Record<ThemeKey, Theme | null>` as documented in CONTEXT §Theme Registry.

### `apps-script/src/lib/migrations.ts`
**Analog:** `internal/scaffold/scaffold.go`'s `ScaffoldSchemaV1` — idempotent. New function: `migrateToV2()` reads `_meta.schema_version`, no-op if already 2, else extends `_pigparse`/`_item_master`/`_quest_items` headers + writes `_meta.schema_version=2` last.

### `apps-script/src/__tests__/`
**Analog:** `internal/parse/inventory_test.go` + `spellbook_test.go` — table-driven, fixture-loaded, golden-file comparisons. Use vitest. Fixtures already at `apps-script/src/__fixtures__/` (matches Phase 2 convention `internal/parse/testdata/`).

### `.github/workflows/apps-script-build.yml`
**Analog:** `.github/workflows/release.yml` — same setup-node, install-deps, run-build pattern. Difference: this is PR-only (verifies dist matches a clean rebuild), not release-triggered.

---

## §3. Patterns to STEAL from the Go side

| Pattern | Go example | TS application |
|---------|-----------|----------------|
| Doc-comment preamble explaining algorithm + pitfalls defended | `internal/sheet/heartbeat.go` lines 1-37 | Top of every trigger file |
| Single-function-per-file for major operations | `client.go` / `heartbeat.go` / `owner.go` / `meta.go` split | `triggers/refreshPigparse.ts` etc. |
| Mutex-funneled writes through one helper | `internal/sheet/client.go` `c.batchUpdate` | `LockService.getDocumentLock()` wrapping in try/finally |
| Const slice as schema source-of-truth | `scaffold.DimensionTabs` | `THEMES`, `PIGPARSE_HEADERS`, etc. |
| Three-state validate-before-write | `WorkbookState{Empty,Matches,Wrong}` | Migration check: schema-already-v2 vs needs-migration vs unknown |
| Defensive EnsureSheet before any write | `ensure_tab.go` | `getOrCreateSheet(name)` helper before any `getRange` |
| Structured logging via slog with context | `slog.Info(...)` with key-value pairs | `console.log(JSON.stringify({level:'info',op:'refreshPigparse',...}))` — Apps Script Stackdriver inspects strings; structured JSON is greppable later |
| "No comments unless WHY is non-obvious" | enforced via CLAUDE.md, evident in heartbeat.go | Same rule applies to TS |

---

## §4. Patterns to REJECT (don't carry these over)

| Pattern | Why it doesn't apply |
|---------|---------------------|
| `context.Context` as first arg | Apps Script has no equivalent; trigger functions terminate at process-end |
| Channels / goroutines | No concurrency primitives in Apps Script V8 |
| Custom error sentinel types (`ErrSchemaTooNew`, etc.) | TS uses thrown `Error` subclasses or discriminated-union return types |
| `time.AfterFunc` self-rescheduling at the goroutine level | Replaced by `ScriptApp.newTrigger().after()` API |
| `Background ctx` for token refresh hack | No equivalent — Apps Script uses container-bound auth |
| Atomic file rename for autoupdate | N/A — clasp deploys are single-step |

---

## §5. Cross-side coordination requirements

### Schema version coordination (CRITICAL)
- `internal/sheet/client.go:40` declares `WatcherMaxSchemaVersion = 1`.
- `_meta.schema_version > WatcherMaxSchemaVersion` → `ValidateWorkbook` returns `ErrSchemaTooNew` → watcher REFUSES to write.
- **Phase 3 plan must:** (a) bump the constant to 2 in Go side, (b) ship that watcher change as v0.3.0 BEFORE the Apps Script migration runs OR ensure the migration only runs once both sides are deployed.
- Sequencing: ship watcher v0.3.0 with `WatcherMaxSchemaVersion = 2` → THEN deploy Apps Script that runs the migration → migration writes `schema_version=2` → both sides happy.
- If migration runs first, watcher refuses to write, watcher tray goes red, user must update watcher. Acceptable failure mode but should be documented.

### Heartbeat write distinguishability (per CONTEXT §Specifics — Heartbeat-driven onChange)
- Phase 2 heartbeat writes to `_char_owner.last_seen` (col K) + `_status.last_heartbeat` (col F). These fire `onChange` with `e.changeType === 'OTHER'`.
- `view` builder must not full-rebuild on every heartbeat — that's 12 rebuilds/day for no actual data change.
- **Strategy:** Phase 3's `onChange` handler inspects `e.source.getActiveSheet()` (or iterates `e.source.getSheets()` and checks last-edited timestamps via `range.getA1Notation()` if the API exposes it). Only rebuild if the changed range is in `inv:*` or `spell:*`. The `_char_owner` and `_status` writes are skipped.
- Fallback if `e.source.getActiveSheet()` doesn't reliably point at the changed sheet: rely on the 10s `PropertiesService` debounce + the 1h backstop. Worst case: heartbeat triggers a redundant rebuild every 24h × 12 watchers ≈ 12 redundant rebuilds/day. Each ~10s. ~2 min/day waste. Acceptable.

### Tab name conventions
- Watcher writes: `inv:<Char>`, `spell:<Char>`, `_char_owner` (col K only), `_status` (cols A,B,C,F only — Phase 2 mutex-protected the D/E columns)
- Apps Script writes: `_pigparse`, `_item_master`, `_quest_items`, `_meta.theme`, `_meta.last_error`, `view`, `bank`, `_status.cell_count`, `_status.last_full_refresh`
- **Zero overlap.** Watcher and Apps Script never write to the same column. The schema-lock contract holds.

### `MetaRows` extension (per `internal/scaffold/scaffold.go` lines 109-123)
- Append-only. Phase 3's two new rows (`theme`, `contact_email`) go at the END of the slice.
- If we add them to the Go side: requires a watcher rebuild + redeploy. If we add them from Apps Script's first run: keeps Phase 3 isolated to TS.
- **Recommendation:** write the rows from Apps Script's `migrateToV2()` (idempotent: read existing keys, append only if absent). Faster iteration, no Go rebuild.

---

## §6. Test fixture conventions

Phase 1+2 pattern in `internal/parse/testdata/`:
- Real-name files: `Slampeach-Spellbook.txt` (named after the actual character that produced it)
- Generic-name files: `sample-inventory.txt`, `sample-inventory-with-cp1252.txt` (when the test data is synthetic or anonymized)
- One fixture per concept; never a "kitchen sink" fixture
- Encoding-specific fixtures get their encoding in the filename

Phase 3 pattern at `apps-script/src/__fixtures__/` (already in place):
- Live-API fixtures named after their probe: `pigparse-getall-1.json`, `wiki-parse-cloth-cap.json`
- Redirect-handling fixture: `wiki-parse-fungi-tunic-redirect.json` (suffix indicates the test concept it's for)
- Spec snapshots: `pigparse-swagger-v1.json`

This is a CONSISTENT extension of the Phase 1+2 convention. No changes needed.

---

## §7. CI workflow analog

`.github/workflows/release.yml` is the only existing workflow. It builds the Go binary, signs+packages the NSIS installer, uploads to GitHub Releases on `v*` tag push.

**For Phase 3 add `apps-script-build.yml`:**
- Triggers: `pull_request` paths `apps-script/**`
- Runs: `cd apps-script && npm ci && npm run build && npm test && git diff --exit-code dist/` (the last command fails the PR if `dist/` isn't checked in fresh)
- Does NOT push to any workbook (deployment is manual via `clasp push` from the workbook owner's machine — keeps OAuth credentials out of CI)

---

*Phase: 03-apps-script-enrichment-foundation*
*Patterns mapped: 2026-05-09 by orchestrator-direct inspection*
*Critical drift documented in §1 — planner MUST apply these corrections to CONTEXT-derived assumptions*
