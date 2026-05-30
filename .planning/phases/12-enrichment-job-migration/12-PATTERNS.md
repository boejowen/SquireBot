# Phase 12: Enrichment Job Migration - Pattern Map

**Mapped:** 2026-05-29
**Files analyzed:** 13 new Go files/units (4 parsers + eqconst + politefetch + 2 jobs + 1 migration + N store methods + scheduler flesh-out + tests)
**Analogs found:** 13 / 13 (every new file has both a TS source AND an existing Go analog)

This is a **TypeScript → Go PORT** phase. RESEARCH §"Go Package Layout" already fixed the target tree; this map adds, per new file: (a) the **TS SOURCE** being ported and (b) the **closest EXISTING Go analog in this repo** whose idioms the executor mirrors, with a verbatim excerpt. The single load-bearing convention to carry through every store method is the **11-05 single-tested-SQL-path rule** (jobs/handlers author NO inline SQL).

## File Classification

| New Go File / Unit | Role | Data Flow | TS Source (port) | Closest Go Analog | Match |
|---|---|---|---|---|---|
| `enrich/pigparse.go` | pure parser | parse (string→struct) | `lib/pigparse-types.ts` (`parseToRows`) | `internal/parse/inventory.go` (`Parse`) | role-match (pure parser) |
| `enrich/wikiitem.go` | pure parser | parse + sha1 | `lib/wiki-parser.ts` (`parseItempage`, `computeSha1Hex`) | `internal/parse/inventory.go` (`Parse`) + `internal/update/check.go` (sha hashing) | role-match |
| `enrich/wikispell.go` | pure parser | parse | `lib/wiki-spell-parser.ts` (`parseClassPage`, `normalizeSpellName`) | `internal/parse/spellbook.go` (`ParseSpellbook`) | role-match |
| `enrich/wikigear.go` | pure parser | parse | `lib/wiki-gear-tier-parser.ts` (`parseGearTierPage`) | `internal/parse/inventory.go` (`Parse`) | role-match |
| `enrich/eqconst.go` | constant/lookup | (none) | `lib/eq-constants.ts` (CLASSES, slot maps) | `internal/parse` package-level vars; `pigparse-types.ts` key-set idiom | partial (data-only) |
| `enrich/politefetch/politefetch.go` | http client | fetch (retry/backoff/304) | `lib/politeFetch.ts` (`politeFetch`) | `internal/update/check.go` (`CheckOnce` net/http + Timeout + UA + stream-hash) | role-match (http client) |
| `enrich/jobs/pigparse.go` | orchestration | fetch→parse→upsert | `triggers/refreshPigparse.ts` (`refreshPigparse`/`runUnderLock`) | `internal/backendsrv/ingest/handler.go` (`bindAndReplace`: compose-over-one-tx, no inline SQL) | role-match |
| `enrich/jobs/wiki.go` | orchestration | fetch→sleep→parse→upsert | `triggers/refreshWikiItems/Spells/GearTier.ts` | `internal/backendsrv/ingest/handler.go` + `internal/update/check.go` (sequential I/O loop) | role-match |
| `migrations/00003_enrich_columns.sql` | migration | DDL | (new — RESEARCH §"Exact 00003") | `migrations/00002_audit.sql` + `embed.go` | exact (goose file shape) |
| `store.UpsertPigparsePrices` | store-SQL | upsert (per-item) | (new) | `store/replace.go` `ReplaceInventoryTx` | exact (tx + parameterized SQL) |
| `store.UpsertItemMaster` | store-SQL | upsert + sha short-circuit | (new) | `store/replace.go` `ReplaceInventoryTx` | exact |
| `store.UpsertWikiSpells` (per-class replace) | store-SQL | DELETE+INSERT/tx | (new) | `store/replace.go` `ReplaceSpellbookTx` (DELETE-all-then-INSERT) | exact |
| `store.ReplaceWikiGearTier` (full-table replace) | store-SQL | DELETE-all+INSERT/tx | (new) | `store/replace.go` `ReplaceInventoryTx` (DELETE-all body) | exact |
| `store.ReplaceQuestItemsForId` (per-id replace) | store-SQL | DELETE-where+INSERT/tx | (new) | `store/replace.go` + `store/binding.go` (WHERE-scoped DELETE/INSERT in tx) | exact |
| `store.Get/SetJobRun`, `store.Get/SetETag` | store-SQL | KV upsert/select | (new) | `store/binding.go` `bindCharacter` (indexed SELECT + INSERT…ON CONFLICT shape) | role-match |
| `scheduler.go` flesh-out (Job registry + tick loop) | scheduler | timer→due-check→run | `triggers/refresh*.ts` cadence + `LockService` | **existing `scheduler/scheduler.go` skeleton** + `internal/heartbeat/heartbeat.go` | exact (same file) |
| `enrich/*_test.go`, `jobs/jobs_test.go`, `politefetch_test.go` | test | fixture-driven | `__tests__/*.test.ts` | `store/replace_test.go` + `ingest/handler_test.go` (httptest) + `store.NewTestDB` | exact |

---

## Pattern Assignments

### `enrich/pigparse.go` (pure parser, parse)

**TS source:** `apps-script/src/lib/pigparse-types.ts` — `parseToRows(body: string): PigparseRowRaw[]`, `isValidRow`, `coerceRow`, `REQUIRED_KEYS`/`NUMERIC_KEYS`, 1% malformation tolerance.
**Go analog:** `internal/parse/inventory.go` — `Parse(r io.Reader) ([][]string, error)`. Mirror its **pure, I/O-free, returns-typed-error, silently-skips-bad-rows** shape and its package doc-comment style.

**Analog excerpt — pure parser skeleton + skip-bad-rows (`internal/parse/inventory.go:53-81`):**
```go
func Parse(r io.Reader) (rows [][]string, err error) {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.FieldsPerRecord = -1 // tolerate any column count
	cr.LazyQuotes = true
	all, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	...
	out := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) < 5 {
			continue            // ← skip malformed; never abort whole parse
		}
		if !isIntField(row, 2) {
			continue
		}
		out = append(out, row[:5])
	}
	return out, nil
}
```

**Port mapping:**
- Signature: `func ParseToRows(body []byte) ([]PigparseRow, error)` — takes the raw JSON bytes (the fixture is a flat `[]` array; `os.ReadFile` → straight in).
- `PigparseRowRaw` (TS) → Go struct `PigparseRow` with `json` tags: `I int json:"i"`, `T int json:"t"`, `N string json:"n"`, `L string json:"l"`, `T30/A30/T60/A60/T6m/A6m/Ty/Ay` (omit `T90/A90` and `Tc/Ta` per RESEARCH §2 / A4 — the Sheet never wrote them).
- `parseToRows` throws on non-array body / >1% malformed → Go returns `error`. The 1% tolerance loop (`skipped/len > 0.01 → error`, else log) ports verbatim; `log('warn',…)` → `slog.Warn("pigparse parse", "skipped", n, "total", t)`.

**Go idiom that MUST differ from TS:**
- TS `JSON.parse` + `Array.isArray` guard → Go `json.Unmarshal(body, &[]PigparseRow{})`; a non-array body yields an unmarshal error (map that to the "not an array" message).
- The t=0/t=1 dedup (D-9: keep WTS t=0) is **NOT** in the parser — the parser returns ALL raw rows (parity with TS); the **job** (`jobs/pigparse.go`) filters to `T==0` before upsert. Keep that filter as one isolated step (D-9 reversibility).

---

### `enrich/wikiitem.go` (pure parser, parse + sha1)

**TS source:** `apps-script/src/lib/wiki-parser.ts` — `parseItempage(wikitext, pageTitle): ParseResult`, `computeSha1Hex`, `pageNameToSlug`, `wikiUrlFor`, `extractItempageBody`, `parseTemplateParams`, `parseStatsblock`, `extractSummary`, `parseClasses`; types in `wiki-types.ts` (`ParsedWikiItem`, `WikiQuestItemLink`, `ParseResult` discriminated union).
**Go analog:** `internal/parse/inventory.go` (pure-parser shape) for the parse body; `internal/update/check.go` for the hashing idiom (stdlib `crypto` + `encoding/hex`).

**Analog excerpt — stdlib hashing (`internal/update/check.go:31,145,157`):**
```go
import ("crypto/sha256"; "encoding/hex")
...
h := sha256.New()
mw := io.MultiWriter(f, h)
...
actualHex := hex.EncodeToString(h.Sum(nil))
```

**Port mapping:**
- `ParseResult` discriminated union (`{ok:true,…}` | `{ok:false,reason}`) → Go: return `(ParsedWikiItem, []WikiQuestItemLink, error)` OR a result struct with an `OK bool` + `Reason string`. `MIN_WIKITEXT_LENGTH=200` guard → early typed error/`ok:false` (V5 input validation — port verbatim).
- `is_quest_item`, `slot`, `wiki_url`, `summary`, `wikitext_sha1` are the ONLY fields the Sheet persisted (RESEARCH §1) — **do NOT** surface `ac/weight/effect/classes/is_no_drop` to the store (D-8 scope guard + parity).
- `questLinks` (the `WikiQuestItemLink[]`) is the input to `quest_items` upsert; the caller fills `item_id`.

**Go idiom that MUST differ from TS (load-bearing):**
- `computeSha1Hex` in TS does a **signed-byte fix-up** (`b < 0 ? b + 256`) because Apps Script `Utilities.computeDigest` returns *signed* bytes. **Go does NOT need this** — `crypto/sha1` returns unsigned bytes:
  ```go
  func sha1Hex(s string) string { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:]) }
  ```
  Output is byte-identical lowercase hex → the SHA-1 parity check (D-7 §2) holds. (`crypto/sha1` here is a content fingerprint, NOT a security hash — acceptable per RESEARCH Security V6.)
- `encodeURIComponent`/`.replace(/ /g,'_')` in `pageNameToSlug` → Go `strings.ReplaceAll(name," ","_")` then `url.PathEscape` (verify against the 4 non-redirect fixtures; mind PathEscape vs QueryEscape on `/`).

---

### `enrich/wikispell.go` (pure parser, parse)

**TS source:** `apps-script/src/lib/wiki-spell-parser.ts` — `parseClassPage(...)` (line 37), `splitOnLevelHeaders`, `extractSpellNames`, `extractInlineLevelSpells` (the `{{SpellRow}}` / `{{SongRow}}` / inline-level Bard variants); types in `wiki-spell-types.ts` (`WikiSpellRow` line 6, `normalizeSpellName` line 38).
**Go analog:** `internal/parse/spellbook.go` — `ParseSpellbook(r io.Reader) ([][]string, error)` (the 2-column Level/Name parser; closest domain twin).

**Analog excerpt — normalize-on-the-way + skip-bad-rows (`internal/parse/spellbook.go:47-75`):**
```go
func ParseSpellbook(r io.Reader) (rows [][]string, err error) {
	...
	out := make([][]string, 0, len(all))
	for _, row := range all {
		if len(row) < 2 {
			continue
		}
		if _, err := strconv.Atoi(row[0]); err != nil {
			continue            // non-int Level → skip
		}
		out = append(out, []string{row[0], row[1]})
	}
	return out, nil
}
```

**Port mapping:**
- `parseClassPage(wikitext, className)` → `func ParseClassPage(wikitext, class string) ([]WikiSpellRow, error)`. `WikiSpellRow{Class, Level, SpellName, NormalizedName}`.
- `normalizeSpellName(s)` (TS) ≡ the DB's existing `lower(trim(name))` convention already used in `ReplaceSpellbookTx` (`strings.ToLower(strings.TrimSpace(name))`, replace.go:169) — **reuse that exact expression** so the P14 spellbook↔wiki join key matches.
- Warrior fixture → **0 rows** (degenerate no-spell case); Bard `{{SongRow}}` inline-level fallback has NO fixture — replicate the synthetic-string cases from `apps-script/src/__tests__/wiki-spell-parser.test.ts` (RESEARCH §"Fixture Reuse" flags this).

---

### `enrich/wikigear.go` (pure parser, parse)

**TS source:** `apps-script/src/lib/wiki-gear-tier-parser.ts` — `parseGearTierPage(...)` (line 26); types in `wiki-gear-tier-types.ts` (`Tier` union line 5: `'Velious Pre-Raid/Group'|'Velious Raiding'|'Iksar'`; `WikiGearTierRow` line 7, `item_id` **always null**, Iksar tagging).
**Go analog:** `internal/parse/inventory.go` (pure-parser shape; same skip-bad-rows discipline).

**Port mapping:**
- `func ParseGearTierPage(wikitext string, tier Tier) ([]WikiGearTierRow, error)`. `WikiGearTierRow{Tier, Class, Slot, ItemID *int /* always nil */, ItemName, Rank int}`.
- `Tier` union → Go typed string constants (`TierVeliousPreRaid`, `TierVeliousRaiding`, `TierIksar`).
- `rank` is 1-based; Iksar-tagged items carry `Tier=Iksar` (D-7 §4 verifies this).

**Go idiom note:** because `ItemID` is always nil, the **store** side uses **full-table replace** (not upsert) — see `store.ReplaceWikiGearTier` (the broken `UNIQUE(…,item_id)` hazard, Pitfall 1). The parser itself is unaffected.

---

### `enrich/eqconst.go` (constant/lookup, no I/O)

**TS source:** `apps-script/src/lib/eq-constants.ts` — `CLASSES` (line 8, 14 classes), `CLASS_DISPLAY_TO_ABBREV` (line 17), `WIKI_SLOT_TO_INV_SLOTS` (line 54).
**Go analog:** package-level `var`/`const` declarations as in `internal/parse` (e.g. the `REQUIRED_KEYS`/`NUMERIC_KEYS` slice-of-keys idiom from `pigparse-types.ts` → Go `[]string` / `map[string][]string`).

**Port mapping:**
- `CLASSES` (TS `as const` tuple) → `var CLASSES = [...]string{...}` or `[14]string`.
- `CLASS_DISPLAY_TO_ABBREV: Record<string,ClassAbbrev>` → `map[string]string`.
- `WIKI_SLOT_TO_INV_SLOTS: Record<string,string[]>` → `map[string][]string`.
- No logic, no tests beyond a compile-time sanity (optional table-count assert). This package is imported by `wikispell.go`/`wikigear.go` (class validation) — keep it dependency-free to avoid an import cycle (RESEARCH §"Naming the parser package enrich").

---

### `enrich/politefetch/politefetch.go` (http client, fetch with retry/backoff/304)

**TS source:** `apps-script/src/lib/politeFetch.ts` — `politeFetch(url, opts): FetchResult` (discriminated union `FetchSuccess|FetchError`), `RETRY_DELAYS_MS=[2000,4000,8000,16000,32000]`, `RETRY_STATUSES={429,503,504}`, `parseRetryAfterMs` (0–600s clamp), `DEFAULT_USER_AGENT`.
**Go analog:** `internal/update/check.go` — the watcher's existing **net/http** client with `http.NewRequestWithContext`, an explicit `User-Agent`, `&http.Client{Timeout:…}`, streamed read, and a package-level mutex/test-seam pattern.

**Analog excerpt — net/http request + UA + timeout + status guard (`internal/update/check.go:125-138`):**
```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.BinaryURL, nil)
if err != nil {
	return fmt.Errorf("CheckOnce download req: %w", err)
}
req.Header.Set("User-Agent", "SquireBot-AutoUpdate")
c := &http.Client{Timeout: 5 * time.Minute}
resp, err := c.Do(req)
if err != nil {
	return fmt.Errorf("CheckOnce download: %w", err)
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
	return fmt.Errorf("CheckOnce download HTTP %d", resp.StatusCode)
}
```

**Analog excerpt — test-seam + reschedule-sleep that respects ctx (`internal/update/check.go:53-62`), mirror for the inter-request sleep & retry waits:**
```go
func realCheckSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
```

**Port mapping (per RESEARCH §"politeFetch Go Port Spec" — all 12 controls):**
- TS `FetchResult` union → Go `FetchResult` struct (RESEARCH §"Return shape"): `OK, Status, Body []byte, ETag, LastModified, FromCache bool, RetriesUsed int, Err error`.
- UA: `"SquireBot/"+Version+" (+https://github.com/boejowen/SquireBot)"`; `Version` is a new backend `var Version` (D-11; default fallback `"SquireBot/dev (+…)"`). Mirror the watcher's `req.Header.Set("User-Agent", …)`.
- ETag/304: `If-None-Match` + (ADD) `If-Modified-Since` headers from `etag_cache`; on `resp.StatusCode==304` return `FromCache:true, Body:nil`.
- Backoff loop: transliterate `RETRY_DELAYS_MS` → `[]time.Duration{2*time.Second,…,32*time.Second}`; `for attempt:=0; attempt<=len(retryDelays); attempt++`. `Retry-After` parse → integer seconds, clamp 0–600 (TS handles delta-seconds only — match it). Sleep via the ctx-aware timer seam above (NOT bare `time.Sleep`, so shutdown is clean).
- Non-retriable status (not in {200,304,429,503,504}) → return error immediately, no sleep (TS `non-retriable ${status}`).

**Go idiom that MUST differ from TS (REQUIRED — security):**
- TS `getContentText()` had Apps Script's implicit response cap; Go `io.ReadAll(resp.Body)` is **unbounded**. **Wrap with `io.LimitReader(resp.Body, maxResponseBytes)`** (~16 MB; PigParse fixture is 1.27 MB) — mirrors the ingest handler's `http.MaxBytesReader(1<<20)` (handler.go:79). See Shared Patterns §Bounded-reads.
- TLS verification ON (Go default) — do NOT set `InsecureSkipVerify` (TS `validateHttpsCertificates:true`).
- **1-second inter-request sleep stays OUT of the client** (TS comment lines 5-7 is explicit) — it lives in the **wiki job** between page fetches.

---

### `enrich/jobs/pigparse.go` (orchestration, fetch→parse→upsert)

**TS source:** `apps-script/src/triggers/refreshPigparse.ts` — `refreshPigparse()` (line 43) / `runUnderLock(startMs)` (line 59) / `buildRow` / `PIGPARSE_HEADERS` (15 cols) / `ROW_COUNT_FLOOR_PCT` truncation guard. Port the **flow**, NOT the Sheets I/O.
**Go analog:** `internal/backendsrv/ingest/handler.go` — `bindAndReplace` (the **compose-store-calls-over-one-tx, authors-no-inline-SQL** pattern). This is the canonical 11-05 single-SQL-path example.

**Analog excerpt — orchestration composes EXPORTED store Tx methods, no inline SQL (`internal/backendsrv/ingest/handler.go:165-216`):**
```go
func (h *Handler) bindAndReplace(r *http.Request, ownerID int64, env Envelope, rows [][]string) (int, error) {
	ctx := r.Context()
	tx, err := h.db.BeginTx(ctx, nil) // BEGIN IMMEDIATE (single-writer DSN)
	if err != nil {
		return 0, err
	}
	charID, err := store.BindCharacter(ctx, tx, env.Character, ownerID)  // ← store method, not inline SQL
	...
	err = store.ReplaceInventoryTx(ctx, tx, charID, rows, uploadedAt, env.WatcherVersion)  // ← store method
	...
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return http.StatusNoContent, nil
}
```

**TS source excerpt — the flow to port (fetch→parse→guard→write), `refreshPigparse.ts:59-82`:**
```ts
function runUnderLock(startMs: number): void {
  const result = politeFetch(PIGPARSE_URL);
  if (!result.ok) { writeError(...); return; }            // → return error / slog + job_run status
  let rows; try { rows = parseToRows(result.body); } catch (e) { writeError(...); return; }
  const lastCount = readMetaRowInt('_status', 'last_pigparse_row_count') ?? 0;
  if (lastCount > 0 && rows.length < lastCount * ROW_COUNT_FLOOR_PCT) {
    writeError(... 'truncated_response' ...);
    log('warn', 'refreshPigparse', { abandoned: 'truncated', ... });
    return;                                                // ⚠ Sheet ABORTS here…
  }
  ...
}
```

**Port mapping:**
- Job signature: `func RunPigparse(ctx context.Context, db *sql.DB, fetch politefetch.Fetcher) error`.
- Flow: `store.GetETag(url)` → `politefetch.Fetch(ctx, url, etag, lastMod)` → on 304 `slog.Info("skipped_unchanged")` + `store.SetJobRun(...,"skipped_unchanged")` and **return** (Pitfall 6: never parse an empty 304 body) → else `ParseToRows(body)` → **filter `T==0`** (D-9) → `store.UpsertPigparsePrices(ctx, tx, rows)` (in ONE tx) → `store.SetETag` + `store.SetJobRun(...,"ok",detail)`.
- `LockService.getDocumentLock()` (lines 45-56) → the scheduler's per-job `sync.Mutex` (gone from the job body; lives in the registry).
- `writeError`/`writeMetaRow`/`clearError` (Sheets `_meta`/`_status`) → `slog` + `job_run.last_status`/`last_detail`.

**Go idiom that MUST differ from TS (D-4):**
- The truncation guard **ABORTS** in the Sheet (`return` without writing). In Go it is a **LOG only** — compute `today < 0.90*last` → `slog.Warn("pigparse truncation guard", "today", n, "last", last)` then **proceed with the upsert** (graceful degradation). The last-known count rides in `job_run.last_detail`.
- All SQL lives in `store.*` — the job authors NO `INSERT`/`DELETE` (mirror `bindAndReplace`).

---

### `enrich/jobs/wiki.go` (orchestration, fetch→sleep→parse→upsert, sequential)

**TS source:** `apps-script/src/triggers/refreshWikiItems.ts` (`refreshWikiItems` line 67, `ITEM_MASTER_HEADERS`, `QUEST_ITEMS_HEADERS`, SHA-1 short-circuit, **6-min cursor — DELETE**), `refreshWikiSpells.ts` (`refreshWikiSpells` line 65, per-class full-replace, **`buildSpellCheck` — DROP**), `refreshWikiGearTier.ts` (`refreshWikiGearTier` line 72, `replaceAllWikiGearTier` full-replace, `WIKI_GEAR_TIER_HEADERS`).
**Go analog:** `internal/backendsrv/ingest/handler.go` `bindAndReplace` (compose store methods over a tx) + `internal/update/check.go` `runDailyCheckWithURL` (sequential I/O loop, errors logged-not-fatal).

**Analog excerpt — sequential loop, log-but-continue on per-item failure (`internal/update/check.go:197-206`):**
```go
for {
	if err := checkSleepFn(ctx, checkInterval); err != nil {
		slog.Info("auto-update goroutine exiting", "err", err)
		return
	}
	if err := checkOnceWithURL(ctx, manifestURL, currentVersion, exePath, statusFn); err != nil {
		slog.Warn("auto-update check failed", "err", err)   // ← log, keep going
	}
}
```

**Port mapping:**
- `func RunWiki(ctx context.Context, db *sql.DB, fetch politefetch.Fetcher) error` — runs items, then spells, then gear-tier sequentially (ONE uninterrupted run — the 6-min cursor is DELETED, D-5).
- **1-second inter-request sleep** between every wiki page fetch (`INTER_REQUEST_SLEEP_MS=1000`) via the ctx-aware timer seam (politefetch analog) — SC-4 politeness.
- Per-item SHA-1 short-circuit (items): read existing `wikitext_sha1` (a `store` getter) → if unchanged, skip the upsert ("unchanged"), mirroring the Sheet's `readItemMasterSha` early-return.
- Per-item failure → accumulate + `slog.Warn`, continue (mirror the auto-update loop) — do NOT abort the whole weekly run on one bad page.

**What to DROP (D-5 / D-8 — REJECT if seen):** the `CURSOR_KEY` resumable machinery, `monitorCellCount`, `weeklySchemaHealthcheck`, `LockService`, `PropertiesService` cursor, and the post-run `buildSpellCheck()`/`buildGearCheck()` view rebuilds (P14 owns views).

---

### `migrations/00003_enrich_columns.sql` (migration, DDL)

**TS source:** none (new DDL; exact SQL is in RESEARCH §"Exact 00003 migration SQL").
**Go analog:** `migrations/00002_audit.sql` (goose annotated-SQL file shape) + `migrations/embed.go` (auto-include mechanism).

**Analog excerpt — goose file shape (`migrations/00002_audit.sql`):**
```sql
-- +goose Up
-- audit_log is an append-only record of security-relevant events. ...
-- Forward-only: this is the SECOND goose migration; 00001_init.sql is owned by
-- 11-02 and is NOT edited (goose is forward-only and the //go:embed *.sql glob
-- auto-includes this file). goose stays idempotent across both migrations.
CREATE TABLE audit_log (
  id                   INTEGER PRIMARY KEY,
  event                TEXT NOT NULL,
  ...
  created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE audit_log;
```

**Analog excerpt — the auto-include + dialect FOOT-GUN (`migrations/embed.go:25-39`):**
```go
//go:embed *.sql
var embedMigrations embed.FS
...
func RunMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil { // ⚠️ "sqlite3" dialect, NOT the "sqlite" driver name
		return err
	}
	return goose.Up(db, ".")
}
```

**Port mapping:**
- Drop the new file at `internal/backendsrv/migrations/00003_enrich_columns.sql`; the `//go:embed *.sql` glob picks it up — **do NOT touch embed.go** (RESEARCH Pitfall 4).
- Content: 8 × single-column `ALTER TABLE pigparse_price ADD COLUMN …` (SQLite forbids multi-column add — Pitfall 3), + `CREATE TABLE job_run(...)` + `CREATE TABLE etag_cache(...)` exactly as RESEARCH §"Exact 00003" specifies.
- Verification: `store.NewTestDB(t)` runs `goose.Up` over all migrations, so a new `migrate_test.go` assertion (`pragma table_info(pigparse_price)` has the 8 cols; `job_run`+`etag_cache` exist) exercises 00003 automatically.

---

### Store methods (store-SQL) — ALL author exported, tested SQL (11-05 single-SQL-path rule)

> **The load-bearing convention for EVERY method below:** the upsert/replace SQL lives HERE in exported `store` functions with `_test.go` coverage; `enrich/jobs/*` call these and author **NO** inline `INSERT`/`DELETE`/`UPDATE`. This is the exact discipline `ingest/handler.go` follows by calling `store.ReplaceInventoryTx` rather than writing SQL inline (handler.go doc-comment lines 21-27). Provide both a public `Store.X(ctx,…)` (begins+commits its own tx) and an `XTx(ctx, tx, …)` body where a caller may need to compose — symmetric with `ReplaceInventory`/`ReplaceInventoryTx`.

**Canonical analog excerpt — DELETE-all-then-prepared-INSERT inside a caller tx, parameterized `?` only, structured-log-without-raw-content (`store/replace.go:90-123`):**
```go
func ReplaceInventoryTx(ctx context.Context, tx *sql.Tx, charID int64, rows [][]string, uploadedAt time.Time, watcherVer string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM inventory_item WHERE character_id = ?`, charID); err != nil {
		slog.Error("inventory replace: delete", "char_id", charID, "err", err)
		return fmt.Errorf("delete inventory_item (char_id=%d): %w", charID, err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO inventory_item
		(character_id, location, name, item_id, count, slots, row_ordinal, uploaded_at)
		VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare inventory insert (char_id=%d): %w", charID, err)
	}
	defer stmt.Close()
	for i, r := range rows {
		itemID, _ := strconv.Atoi(r[2])
		...
		if _, err := stmt.ExecContext(ctx, charID, r[0], r[1], itemID, cnt, slots, i, uploadedStr); err != nil {
			slog.Error("inventory replace: insert", "char_id", charID, "row_ordinal", i, "err", err)
			return fmt.Errorf("insert inventory_item (char_id=%d, row_ordinal=%d): %w", charID, i, err)
		}
	}
	return nil
}
```

**Canonical analog excerpt — public wrapper begins+commits its own tx (`store/replace.go:63-75`):**
```go
func (s *Store) ReplaceInventory(ctx context.Context, charID int64, rows [][]string, uploadedAt time.Time, watcherVer string) error {
	tx, err := s.db.BeginTx(ctx, nil) // _txlock=immediate DSN ⇒ BEGIN IMMEDIATE
	if err != nil {
		return fmt.Errorf("begin inventory replace tx (char_id=%d): %w", charID, err)
	}
	defer tx.Rollback() // no-op after Commit; rolls back on any error
	if err := ReplaceInventoryTx(ctx, tx, charID, rows, uploadedAt, watcherVer); err != nil {
		return err
	}
	return tx.Commit()
}
```

Per-method mapping (write strategy from RESEARCH §"Upsert / Conflict-Key Per Table" + D-12):

| Store method | Strategy | Conflict / scope key | Analog to mirror |
|---|---|---|---|
| `UpsertPigparsePrices(ctx,tx,rows)` | per-row `INSERT … ON CONFLICT(item_id) DO UPDATE` | `item_id` (PK) | prepared-INSERT loop of `ReplaceInventoryTx`; add `ON CONFLICT … DO UPDATE SET …=excluded.…` (RESEARCH §"Code Examples" has the exact 15-col statement) |
| `UpsertItemMaster(ctx,tx,item)` | `ON CONFLICT(item_id) DO UPDATE` + SHA-1 short-circuit (caller skips when unchanged) | `item_id` (PK) | `ReplaceInventoryTx` insert shape; single-row |
| `UpsertWikiSpells(ctx,tx,class,rows)` | **per-class DELETE+INSERT in one tx** (D-12, Sheet-faithful) | `DELETE WHERE class=?` then prepared INSERT | `ReplaceSpellbookTx` (DELETE-all-then-INSERT) but scoped `WHERE class=?` |
| `ReplaceWikiGearTier(ctx,tx,rows)` | **full-table replace** (DELETE all + INSERT all) — the `UNIQUE(…,item_id)` is broken (item_id always NULL) | none (replace whole table) | `ReplaceInventoryTx` DELETE-all body, but `DELETE FROM wiki_gear_tier` (no WHERE) |
| `ReplaceQuestItemsForId(ctx,tx,itemID,links)` | per-`item_id` DELETE+INSERT (D-12) | `DELETE WHERE item_id=?` then INSERT | `ReplaceSpellbookTx` DELETE+INSERT scoped `WHERE item_id=?` |
| `GetJobRun(ctx,name)` / `SetJobRun(ctx,name,ts,status,detail)` | SELECT one row / `INSERT … ON CONFLICT(job_name) DO UPDATE` | `job_name` (PK) | `bindCharacter` indexed `SELECT … WHERE name=?` (binding.go:57-58) for the getter; upsert shape for the setter |
| `GetETag(ctx,url)` / `SetETag(ctx,url,etag,lastMod)` | SELECT one row / `INSERT … ON CONFLICT(url) DO UPDATE` | `url` (PK) | same as job_run |

**Go idioms that MUST hold (security V5 / Tampering):** parameterized `?` placeholders ONLY — never string-concat parsed item names / wikitext into SQL (the analog uses `?` exclusively). modernc supports SQLite UPSERT (≥3.24). Never log raw response content — log counts/keys/err only (mirror `slog.Error("… replace: insert", "char_id", …, "err", err)`).

---

### `scheduler/scheduler.go` flesh-out (scheduler, timer→due-check→run)

**TS source:** the cadence semantics of `triggers/refresh*.ts` (daily / weekly-Sunday) + `LockService` (→ per-job mutex). No verbatim port — RESEARCH §"Scheduler Design" is the spec.
**Go analog:** the **existing skeleton in this same file** (the `time.Ticker` + `select ctx.Done()/ticker.C` loop) + `internal/heartbeat/heartbeat.go` (the immediate-fire-then-reschedule precedent its own doc-comment cites).

**Analog excerpt — the skeleton loop to KEEP (ctx.Done clean-shutdown is already correct; replace only the body, `scheduler/scheduler.go:43-59`):**
```go
func run(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()
	slog.Info("scheduler started", "interval", HeartbeatInterval.String(), "jobs", 0)
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			// SKELETON: no real jobs until P12. Just prove the loop fires.
			slog.Info("scheduler heartbeat")
		}
	}
}
```

**Analog excerpt — immediate-first-fire + self-reschedule (the "due-on-startup" precedent, `internal/heartbeat/heartbeat.go:90-101`):**
```go
// Immediate first fire.
tick()

// Self-reschedule loop.
for {
	if err := sleepFn(ctx, Interval); err != nil {
		slog.Info("heartbeat goroutine exiting", "err", err)
		return
	}
	tick()
}
```

**Port mapping (RESEARCH §"Scheduler Design"):**
- Replace `HeartbeatInterval=time.Hour` body with a `checkInterval` (~10 min) **poll-and-check** loop; KEEP the `select ctx.Done()/ticker.C` shutdown verbatim (it's already tested by `scheduler_test.go::TestRun_ReturnsOnContextCancel`).
- Add a `Job{Name, Due func(last,now time.Time) bool, Run func(ctx) error, mu sync.Mutex}` registry (2 jobs: `pigparse_daily`, `wiki_weekly`).
- On `Start`: `store.GetJobRun` each job's `last_run_at` → **immediate check pass** (so a missed job fires within seconds of restart, mirroring heartbeat's immediate first fire) → then the ticker loop.
- `Due` predicates (D-10, no cron lib): PigParse `now.Sub(last) >= 24h`; wiki `now.Weekday()==time.Sunday && last.Before(startOfSundayUTC(now))`.
- `runJob`: `job.mu.TryLock()` (skip-not-queue; replaces `LockService`) → `job.Run(ctx)` → `store.SetJobRun(name, now, statusFrom(err), detail)` **after** the run (advance-always, even on error — A2 — so a failing fetch doesn't hot-loop).

**Go idiom note:** the per-job `sync.Mutex` replaces the Sheet's `LockService.getDocumentLock()`; the DB's `SetMaxOpenConns(1)` already serializes writes, so the mutex only prevents a redundant overlapping fetch+parse cycle, not DB races (RESEARCH §"Per-job mutex"). The two jobs have separate mutexes and may run concurrently (both due on a Sunday) — their short upsert txs serialize harmlessly through the single connection.

---

### Tests (test, fixture-driven)

**TS source:** `apps-script/src/__tests__/{pigparse-types,wiki-parser,wiki-spell-parser,wiki-gear-tier-parser,politeFetch}.test.ts` (transliterate the expected-value assertions for byte parity).
**Go analogs:** `store/replace_test.go` (table-driven store SQL tests over `NewTestDB`), `ingest/handler_test.go` (httptest end-to-end), `store/testhelper.go` (`NewTestDB`), `scheduler/scheduler_test.go` (ctx-cancel goroutine test).

**Analog excerpt — store test over the shared migrated temp DB (`store/replace_test.go:29-49`):**
```go
func TestReplaceInventory_InsertsAllRows(t *testing.T) {
	db := NewTestDB(t)            // Open + goose.Up + t.Cleanup (shared fixture)
	s := NewStore(db)
	_, charID := seedOwnerChar(t, db, "owner-a", "Aragorn")
	rows := [][]string{
		{"General1", "Cloth Cap", "1001", "1", "0"},
		...
	}
	ctx := context.Background()
	if err := s.ReplaceInventory(ctx, charID, rows, time.Now().UTC(), "0.3.0"); err != nil {
		t.Fatalf("ReplaceInventory: %v", err)
	}
	got, err := db.Query(`SELECT location, name, item_id, count, slots, row_ordinal
		FROM inventory_item WHERE character_id = ? ORDER BY row_ordinal`, charID)
	...
}
```

**Analog excerpt — httptest harness for an I/O surface (`ingest/handler_test.go:27-45`):**
```go
func newHandler(t *testing.T) (*ingest.Handler, *sql.DB) {
	t.Helper()
	db := store.NewTestDB(t)
	h := ingest.New(auth.New(db), db)
	return h, db
}
func post(t *testing.T, h *ingest.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(body))
	...
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
```

**Port mapping:**
- **Parser tests** (`enrich/*_test.go`): `os.ReadFile("testdata/<fixture>.json")` → for wiki, first `json.Unmarshal` and pull `parse.wikitext["*"]` (Shape B, RESEARCH §"Fixture Reuse") → call parser → assert field-for-field against the TS test's expected values. Copy the 12 fixtures from `apps-script/src/__fixtures__/` into `internal/backendsrv/enrich/testdata/` (RESEARCH says COPY, do not embed across the repo).
- **politefetch test** (`politefetch_test.go`): drive an `httptest.Server` returning 200/304/429-with-Retry-After/503/404 to exercise every control (mirror the `update/check_test.go` httptest seam pattern). Inject a small `retryDelays` + a fake sleep for fast tests.
- **store tests** (`*_test.go`): `NewTestDB(t)` + seed → call the new `Upsert*`/`Replace*` method → query back. Cover the load-bearing cases: PigParse t=0/t=1 dedup → one row per id; gear-tier full-replace → no duplicate growth on a second call; per-class spell replace drops a removed spell.
- **jobs test** (`jobs/jobs_test.go`): `NewTestDB(t)` + an `httptest.Server` serving the fixtures → run the job → assert dimension rows + `job_run` status; assert a 304 response leaves rows untouched (Pitfall 6).

---

## Shared Patterns

### Single-tested-SQL-path (11-05 WARNING-3) — applies to ALL jobs + store methods
**Source:** `internal/backendsrv/ingest/handler.go` (doc-comment lines 21-27 + `bindAndReplace`).
**Apply to:** `enrich/jobs/pigparse.go`, `enrich/jobs/wiki.go`, and every `store.*` method.
Jobs compose EXPORTED store methods over a `*sql.Tx`; they author **zero** inline `INSERT`/`DELETE`/`UPDATE`. All SQL is exported + `_test.go`-covered in `store/`. There is never a second, test-uncovered SQL copy.
```go
// orchestration calls the store method; it does NOT write SQL:
err = store.ReplaceInventoryTx(ctx, tx, charID, rows, uploadedAt, env.WatcherVersion)
```

### Atomic tx discipline (`Store.X` begins+commits; `XTx` composes)
**Source:** `internal/backendsrv/store/replace.go` (`ReplaceInventory` ↔ `ReplaceInventoryTx`).
**Apply to:** every new store write method.
`BeginTx(ctx,nil)` (the `_txlock=immediate` DSN makes it `BEGIN IMMEDIATE`) + `defer tx.Rollback()` (no-op after Commit) + `tx.Commit()`. Provide a `*sql.Tx`-taking body so a caller can compose multiple writes in one tx.

### Parameterized SQL only (security V5 / Tampering)
**Source:** `internal/backendsrv/store/binding.go` + `replace.go` (every query uses `?`).
**Apply to:** all store methods.
`?` placeholders exclusively; never string-concat parsed item names / wikitext into SQL. modernc SQLite supports UPSERT (`ON CONFLICT … DO UPDATE SET col=excluded.col`).

### Bounded reads (security DoS) — net/http + LimitReader
**Source:** `internal/backendsrv/ingest/handler.go:50,79` (`maxBodyBytes = 1<<20`; `http.MaxBytesReader`).
**Apply to:** `enrich/politefetch/politefetch.go`.
Go's `io.ReadAll` is unbounded → wrap `resp.Body` in `io.LimitReader(resp.Body, ~16MB)` before reading. (TS `getContentText()` had Apps Script's implicit cap; the Go port must add this.)

### Structured logging without raw content (V7)
**Source:** `internal/backendsrv/store/replace.go:110-111`, `binding.go:78-79`, and `internal/update/check.go` throughout.
**Apply to:** all jobs, the client, and store methods.
`slog.Info/Warn/Error(op, "key", val, …)` JSON to stdout; log **counts / ids / status / err only** — never the raw wikitext, raw PigParse body, or any token. Map the Sheet's `log('level', op, {fields})` → `slog.Level(op, "field", val)`.

### ctx-aware sleep seam (clean shutdown + testability)
**Source:** `internal/update/check.go:53-62` (`realCheckSleep`: `time.NewTimer` + `select ctx.Done()/t.C`), also `internal/heartbeat/heartbeat.go:50-59`.
**Apply to:** politefetch retry/backoff waits, the wiki job's 1s inter-request sleep, and the scheduler tick. Never bare `time.Sleep` in a long-running path — use a timer that unblocks on `ctx.Done()` (so SIGTERM unwinds promptly), and make it a package-level seam tests can override for speed.

### goose forward-only / never-edit-shipped + embed glob
**Source:** `internal/backendsrv/migrations/embed.go:25-39` + `00002_audit.sql` header.
**Apply to:** `00003_enrich_columns.sql`.
Drop a new `00003_*.sql`; the `//go:embed *.sql` glob auto-includes it; **never edit 00001/00002**. Dialect is `"sqlite3"` (NOT the `"sqlite"` driver name) — already handled by `RunMigrations`; do not touch embed.go.

### Shared backend test fixture
**Source:** `internal/backendsrv/store/testhelper.go` (`NewTestDB(t)`: Open + goose.Up + t.Cleanup).
**Apply to:** every new store test and the jobs test. Importable from other packages' `_test.go` (it lives in a non-`_test.go` file on purpose, like `httptest`).

---

## No Analog Found

None. Every new file maps to BOTH a TS source and an existing Go analog. Two units have only a **partial** Go analog (data/spec-driven rather than code-mirrored), noted for the planner:

| File / Unit | Role | Why partial |
|---|---|---|
| `enrich/eqconst.go` | constant/lookup | Pure data tables — the "analog" is the Go package-level `var`/`map` idiom (no behavioral twin). Ports `eq-constants.ts` 1:1 into Go literals. |
| `scheduler/scheduler.go` Job-registry design | scheduler | No existing job-registry in the repo (the skeleton is a no-op ticker). The shape comes from RESEARCH §"Scheduler Design"; the loop/shutdown/immediate-fire idioms are mirrored from the skeleton itself + `heartbeat.go`. |

---

## Metadata

**Analog search scope:** `internal/backendsrv/{store,scheduler,migrations,ingest,auth,logging}`, `internal/{parse,heartbeat,update,sheet}`, `apps-script/src/{lib,triggers,__tests__,__fixtures__}`.
**Files scanned:** ~22 (8 Go analogs read in full, 6 TS sources read/grepped for signatures, DDL + 2 tests).
**Pattern extraction date:** 2026-05-29

## PATTERN MAPPING COMPLETE

**Phase:** 12 - Enrichment Job Migration
**Files classified:** 17 units (13 distinct new files + the multi-method store group + scheduler flesh-out + the test group)
**Analogs found:** 13 / 13 (100% — every new file has a TS source AND a Go analog)

### Coverage
- Files with exact analog: 9 (migration, all store-SQL methods, scheduler flesh-out, tests)
- Files with role-match analog: 6 (4 parsers, politefetch, both jobs)
- Files with partial/data-only analog: 2 (eqconst, scheduler registry design)
- Files with no analog: 0

### Key Patterns Identified
- **Single-tested-SQL-path is the spine:** jobs compose exported `store.*Tx` methods over one `*sql.Tx` (mirroring `ingest/handler.go::bindAndReplace`); ALL upsert/replace SQL lives in tested `store/` methods modeled on `ReplaceInventoryTx`/`ReplaceSpellbookTx` (parameterized `?`, DELETE-then-INSERT, `defer tx.Rollback()`).
- **Pure parsers mirror `internal/parse`:** I/O-free, return typed errors, silently skip malformed rows, never log raw content — exactly `Parse`/`ParseSpellbook`.
- **politefetch mirrors `internal/update/check.go`'s net/http client** (request + UA + Timeout + status guard + ctx-aware sleep seam) and adds the REQUIRED `io.LimitReader` cap from `ingest`'s `MaxBytesReader` discipline.
- **Scheduler flesh-out keeps the existing skeleton's `ctx.Done()` shutdown verbatim** (already tested) and adds heartbeat's immediate-first-fire for deterministic due-on-startup.

### Go-vs-TS idiom flags (called out per file)
- `crypto/sha1` needs **NO** signed-byte fix-up (TS `b<0?b+256` is dropped) — `wikiitem.go`.
- `io.LimitReader` body cap is REQUIRED (TS had Apps Script's implicit cap) — `politefetch.go`.
- `database/sql` uses `?` placeholders ONLY — all store methods.
- Truncation guard is a **LOG, not an abort** (TS `return`s) — `jobs/pigparse.go` (D-4).
- PigParse t=0/t=1 dedup (keep WTS) lives in the **job**, not the parser — `jobs/pigparse.go` (D-9).
- `wiki_gear_tier` uses **full-table replace**, not upsert (broken `UNIQUE`-on-NULL) — `store.ReplaceWikiGearTier`.

### File Created
`.planning/phases/12-enrichment-job-migration/12-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. The planner can reference, per new file: the exact TS source function to port, the exact existing Go analog file + line-anchored excerpt to mirror, and the precise Go-vs-TS idiom deviations — and can cite the single-tested-SQL-path rule for every store method.
