---
phase: 12-enrichment-job-migration
plan: 02
subsystem: api
tags: [go, parsers, port, pigparse, mediawiki, sha1, byte-parity, enrichment]

# Dependency graph
requires:
  - phase: 12-01
    provides: "store dimension write methods (WikiSpellRow/WikiGearTierRow/quest_items targets) whose normalized_name join key (lower(trim)) this plan's NormalizeSpellName must match"
provides:
  - "internal/backendsrv/enrich package: 4 pure, I/O-free parsers ported 1:1 from the Apps Script TS sources"
  - "ParseToRows + PigparseRow (PigParse getall → price rows; no t=0/t=1 dedup)"
  - "ParseItempage + ParsedWikiItem + WikiQuestItemLink + sha1Hex (wiki item summary + lowercase-hex SHA-1 + quest links)"
  - "ParseClassPage + WikiSpellRow + NormalizeSpellName (per-class spell rows)"
  - "ParseGearTierPage + WikiGearTierRow + Tier constants (Velious gear tiers, Iksar tagging)"
  - "CLASSES + CLASS_DISPLAY_TO_ABBREV + WIKI_SLOT_TO_INV_SLOTS (eqconst.go)"
  - "12 JSON fixtures copied byte-identical into enrich/testdata/ (+ .gitattributes binary rule)"
affects: [12-04 jobs, 12-03 politefetch, 14 web-frontend]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure parser idiom mirrored from internal/parse (I/O-free, typed return, silently-skip-malformed)"
    - "TS→Go regex port: RE2 has no lookahead — splitMultiStat + extractListItems reimplemented as manual scans"
    - "crypto/sha1 + hex.EncodeToString content fingerprint, NO signed-byte fix-up (Go bytes unsigned)"
    - "Byte-parity proof: same fixture input → exact field values cross-checked against the TS parser run in Node"

key-files:
  created:
    - "internal/backendsrv/enrich/eqconst.go"
    - "internal/backendsrv/enrich/pigparse.go"
    - "internal/backendsrv/enrich/pigparse_test.go"
    - "internal/backendsrv/enrich/wikiitem.go"
    - "internal/backendsrv/enrich/wikiitem_test.go"
    - "internal/backendsrv/enrich/wikispell.go"
    - "internal/backendsrv/enrich/wikispell_test.go"
    - "internal/backendsrv/enrich/wikigear.go"
    - "internal/backendsrv/enrich/wikigear_test.go"
    - "internal/backendsrv/enrich/testdata/ (12 fixtures)"
  modified:
    - ".gitattributes (enrich/testdata/* binary)"

key-decisions:
  - "NormalizeSpellName = lower(trim) (the store's join expression), NOT the TS normalizeSpellName alphanumeric-strip — so the P14 spellbook↔wiki join key matches; spell_name values stay byte-identical (planned per D-12)"
  - "ParsedWikiItem surfaces ONLY the 6 Sheet-persisted fields; ac/weight/effect/classes/is_no_drop deliberately not surfaced (D-8 scope guard; surfacing breaks D-7 parity)"
  - "PigparseRow omits T90/A90/Tc/Ta (Sheet's buildRow never wrote them); parser keeps both t=0/t=1 rows (dedup deferred to the job, D-9)"
  - "pageNameToSlug reimplements JS encodeURIComponent (url.PathEscape escapes apostrophes; encodeURIComponent does not) for wiki-URL byte-parity"

patterns-established:
  - "Lookahead-free list-item scan: find <li...> opens + </li>/</ul> closes, slice each to the earliest terminator (mirrors the TS regex (?=</li>|<li...>|</ul>|$))"
  - "Wiki-fixture test helper loadWikitext: json.Unmarshal the action=parse envelope, pull parse.wikitext['*'] + parse.title, then call the parser"

requirements-completed: [ENRICH-10, ENRICH-11]

# Metrics
duration: 18min
completed: 2026-05-30
---

# Phase 12 Plan 02: Pure Enrichment Parsers (Go Port) Summary

**The 4 pure enrichment parsers (`parseToRows`, `parseItempage`+SHA-1, `parseClassPage`, `parseGearTierPage`) + eq-constants ported 1:1 from TypeScript into a new I/O-free `internal/backendsrv/enrich` package, byte-parity-proven against the live fixtures (PigParse 7240 rows; NEC 171 spells; Pre-Raid 577 gear rows; cloth-cap SHA-1 `5ed737f5…`).**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-05-30T01:41:53Z
- **Completed:** 2026-05-30T01:59:33Z
- **Tasks:** 3
- **Files modified:** 14 (9 Go files + 12 fixtures + `.gitattributes`; some shared)

## Accomplishments
- New `internal/backendsrv/enrich` package — 4 pure parsers + the EQ constant tables, transliterated 1:1 from `apps-script/src/lib/*`. No `net/http`, no `database/sql`, no `os` beyond test fixture reads (purity grep clean across all non-test files).
- **SHA-1 byte-parity** (the strongest D-7 signal): `sha1Hex` is `crypto/sha1` + `hex.EncodeToString` with the TS signed-byte fix-up dropped — emits byte-identical lowercase hex (`sha1Hex("test") == a94a8fe5…`; cloth-cap wikitext `== 5ed737f5…`, matching Node's `crypto.createHash('sha1')`).
- **Exact byte-parity** for every parser, cross-checked by running the TS parsers in Node against the same fixtures: PigParse 7240 rows (item 19450 retained twice, t=0 + t=1); NEC 171 spells / 23 levels, PAL 66 / 15, Warrior 0; Pre-Raid 577 gear rows / 14 classes / 4 Iksar, Raiding 606 / 14 / 0; item Summary/Slot/quest-links identical (incl. the `…` ellipsis truncation).
- 28 Go tests pass; `go build ./...` + `go vet ./...` clean; full `go test ./...` green (every watcher + backend package — zero regression).
- All 12 `__fixtures__` copied byte-identical into `testdata/`, pinned `binary` in `.gitattributes` so an autocrlf rewrite can't flip the wikitext SHA-1 on a fresh checkout.

## Task Commits

Each task was committed atomically (TDD `auto`; faithful-port test + impl co-committed):

1. **Task 1: eqconst.go + pigparse.go + fixtures** — `012d082` (feat)
2. **Task 2: wikiitem.go (item summary + SHA-1 + quest links)** — `1a2fc15` (feat)
3. **Task 3: wikispell.go + wikigear.go (spell + gear-tier)** — `d519f37` (feat)

**Plan metadata:** the `docs(12-02): complete pure-parsers plan` commit (this SUMMARY + STATE/ROADMAP/REQUIREMENTS updates).

## Files Created/Modified
- `internal/backendsrv/enrich/eqconst.go` — CLASSES (14), CLASS_DISPLAY_TO_ABBREV, WIKI_SLOT_TO_INV_SLOTS; dependency-free (imported by wikispell/wikigear, no import cycle).
- `internal/backendsrv/enrich/pigparse.go` — `ParseToRows` + `PigparseRow`; ports `parseToRows`/`isValidRow`/`coerceRow` (1% malformation tolerance via a per-row `json.RawMessage` typeof-check); surfaces only Sheet-persisted price fields.
- `internal/backendsrv/enrich/wikiitem.go` — `ParseItempage` + `ParsedWikiItem` + `WikiQuestItemLink` + `sha1Hex` + `encodeURIComponent`; ports the full {{Itempage}} parser; surfaces only the 6 persisted fields.
- `internal/backendsrv/enrich/wikispell.go` — `ParseClassPage` + `WikiSpellRow` + `NormalizeSpellName`; SpellRow/RadSpellRow/RadSpellRow2 header pass + Bard SongRow inline-level fallback.
- `internal/backendsrv/enrich/wikigear.go` — `ParseGearTierPage` + `WikiGearTierRow` + `Tier` consts; lookahead-free `extractListItems`; Iksar re-tagging on Pre-Raid; `ItemID` always nil.
- `internal/backendsrv/enrich/*_test.go` — fixture-driven byte-parity tests transliterated from the TS test expectations + the synthetic Bard/template-variant/gear edge cases.
- `internal/backendsrv/enrich/testdata/*.json` — the 12 fixtures (byte-identical copies).
- `.gitattributes` — `internal/backendsrv/enrich/testdata/* binary`.

## Decisions Made
- **NormalizeSpellName = `lower(trim)` (NOT the TS variant).** The TS `normalizeSpellName` additionally strips a `spell:` prefix and all non-alphanumerics (e.g. "Numb the Dead" → "numbthedead"). The plan + D-12 + the 12-01 store deliberately use `lower(trim(name))` (store/replace.go:169) as the canonical `normalized_name` join key on BOTH the spellbook landing rows and the wiki spell rows — so the P14 spellbook↔wiki join matches. The extracted `spell_name` values are byte-identical to the TS; only the derived `normalized_name` differs, by design. (Followed the plan's explicit acceptance criterion.)
- **Scope guard honored (D-8):** the wiki item parser internally needs only the QUEST-ITEM flag + Slot to produce its 6 surfaced fields, so ac/weight/effect/classes/is_no_drop/is_lore/is_magic/is_temporary are never parsed or surfaced — exactly what the Sheet persisted, preserving D-7 parity.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `pageNameToSlug` initial port used `url.PathEscape`, which over-escapes apostrophes**
- **Found during:** Task 2 (wikiitem)
- **Issue:** The TS `pageNameToSlug` uses `encodeURIComponent`, which leaves `'` (and `!~*()`) unescaped; Go's `url.PathEscape` percent-encodes the apostrophe (`Lord_Nagafen%27s_Lair`), so the wiki URLs (`item.wiki_url` + `quest_items.source_url`) would NOT be byte-identical to the Sheet's.
- **Fix:** Reimplemented `encodeURIComponent` exactly (unreserved set `A-Za-z0-9-_.!~*'()`, uppercase-hex percent-encoding over UTF-8 bytes); dropped the `net/url` import.
- **Files modified:** internal/backendsrv/enrich/wikiitem.go
- **Verification:** `TestPageNameToSlug` (incl. "Lord Nagafen's Lair") passes; full URL byte-parity confirmed against the TS parser output.
- **Committed in:** `1a2fc15` (Task 2 commit)

**2. [Rule 3 - Blocking] `enrich/testdata/*` fixtures not pinned `binary` — autocrlf would corrupt the wikitext SHA-1**
- **Found during:** Task 1 (staging fixtures)
- **Issue:** Git warned "LF will be replaced by CRLF" on the copied fixtures. A CRLF rewrite on a fresh checkout changes the wikitext bytes → flips the SHA-1 → breaks the byte-parity test on another machine. The byte-parity proof is load-bearing for this plan.
- **Fix:** Added `internal/backendsrv/enrich/testdata/* binary` to `.gitattributes`, mirroring the existing `internal/parse/testdata/* binary` precedent; re-staged the blobs as binary (verified `attr/-text`).
- **Files modified:** .gitattributes
- **Verification:** `git ls-files --eol` shows `attr/-text`; staged blobs `cmp`-identical to the apps-script sources.
- **Committed in:** `012d082` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking). Plus 3 cosmetic doc-comment rewordings (eqconst/pigparse/wikiitem) so the plan's `grep -E "net/http|database/sql"` and `grep -E "\+ 256|< 0 \?"` acceptance checks return 0 — the comments described the absent constructs; no behavior change.
**Impact on plan:** Both auto-fixes were necessary for byte-parity correctness (the plan's entire acceptance proof). No scope creep — every change stayed inside the 4-parser + eqconst + fixtures boundary. Plan executed as written otherwise.

## Issues Encountered
- Verifying true byte-parity (not just the TS tests' `>=` thresholds) required running the TS parsers in Node. Bundled each TS source with the existing `esbuild` (in `apps-script/node_modules`) via a throwaway `.mjs` + a `vm` sandbox with an Apps-Script `Utilities` SHA-1 shim, dumped the exact field values, and diffed against a throwaway Go dump test. All matched exactly; the throwaway scripts/tests were removed before committing.

## User Setup Required
None — no external service configuration required. These are pure functions with no network, DB, or credential dependencies.

## Next Phase Readiness
- **12-04 (jobs)** can now compose these parsers: daily PigParse (`ParseToRows` → filter `T==0` per D-9 → `store.UpsertPigparsePricesTx`), weekly wiki (`ParseItempage`/`ParseClassPage`/`ParseGearTierPage` → the 12-01 store write methods). The parser output structs already line up with the 12-01 store input structs (PigparseRow price fields, WikiSpellRow/WikiGearTierRow shapes, WikiQuestItemLink incl. SourceURL).
- **12-03 (politefetch)** is independent and unaffected (this plan touched no HTTP).
- No blockers. Wave-1 parsers complete; the only remaining Wave-1 item is 12-03 (politeFetch).

## Self-Check: PASSED

- All 9 enrich Go files + SUMMARY.md exist on disk.
- All 12 fixtures present in `internal/backendsrv/enrich/testdata/` (byte-identical to source).
- All 3 task commits exist in git history: `012d082`, `1a2fc15`, `d519f37`.
- `go build ./...` + `go vet ./...` clean; `go test ./...` green (no regression); 28 enrich tests pass.

---
*Phase: 12-enrichment-job-migration*
*Completed: 2026-05-30*
