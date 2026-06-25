---
phase: 37-item-enrichment-backbone-flags-effects
reviewed: 2026-06-25T03:58:44Z
resolution: "MD-01 fixed-forward 2026-06-24; LW-01/LW-02/NIT-01/NIT-02 → backlog 999.41"
depth: standard
mode: advisory
files_reviewed: 12
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/enrich/jobs/wiki.go
  - internal/backendsrv/enrich/jobs/wiki_test.go
  - internal/backendsrv/enrich/testdata/wiki-parse-staff-of-temperate-flux.json
  - internal/backendsrv/enrich/wikiitem.go
  - internal/backendsrv/enrich/wikiitem_test.go
  - internal/backendsrv/migrations/00016_item_flags_effects.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/store/backfill.go
  - internal/backendsrv/store/backfill_test.go
  - internal/backendsrv/store/enrich.go
  - internal/backendsrv/store/enrich_test.go
findings:
  blocker: 0
  high: 0
  medium: 1
  low: 2
  nit: 2
  total: 5
status: issues_found
---

# Phase 37: Code Review Report (Advisory)

**Reviewed:** 2026-06-25T03:58:44Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found (advisory — no BLOCKER/HIGH)

## Summary

The Phase 37 item-enrichment backbone (flag booleans + the full flag set + clicky/haste effects) is well-built, heavily tested, and clean on the phase's stated risk surface. `go build`, `go vet`, and the enrich/store/migrations/jobs test suites all pass; the parser is panic-safe on empty/garbage input (verified directly), `MarshalFlags` is deterministic and never emits `null`/`""` (empty → `"[]"`), migration 00016 is additive-nullable + idempotent (goose runs the 9 ADD COLUMNs in one transaction, so partial-apply re-run errors are impossible), all SQL is `?`-parameterized with no concatenation, and the boot backfill is genuinely no-network, idempotent on `flags_json IS NULL`, and wired non-fatally so a hiccup never blocks serving. I reproduced and confirmed the load-bearing D-05 round-trip parity (live RAW-statsblock parse vs. backfill CLEANED-statsblock derivation) holds for all five real fixtures plus the new staff fixture. The one substantive finding is a latent flag-detection divergence between the raw and cleaned statsblock forms for a flag line rendered as a `[[wiki-link]]` — not triggered by any current P1999 flag (MAGIC/LORE/NO DROP/QUEST are plain text in every real fixture), but it can re-write a row on every weekly pass forever if such a flag ever appears. The remainder are minor maintainability notes.

## Medium

### MD-01: Flag-detection diverges between the raw and cleaned statsblock forms for a `[[link]]`-rendered flag line

**File:** `internal/backendsrv/enrich/wikiitem.go:374-402` (`parseStatsblock` / `flagRe`), interacting with `cleanStatsblock:516-540`
**Issue:** The live parser (`ParseItempage`) derives flags from the **raw** `<br>`-separated statsblock, where a flag line that the wiki renders as a wiki-link (e.g. `[[No Drop]]`) does NOT match `flagRe` (`^[A-Z][A-Z\s\-]+$`) because the `[` `]` brackets survive and break the character class. The boot backfill and the weekly freshness compare derive flags from the **stored cleaned** statsblock, where `cleanStatsblock` has already stripped the brackets (`[[No Drop]]` → `No Drop` → uppercased → `NO DROP`), so the flag IS detected. I reproduced this divergence directly:

```
raw     "[[No Drop]]<br>Slot: HEAD"  -> Flags = []          (live parse)
cleaned "No Drop\nSlot: HEAD"        -> Flags = [NO DROP]    (backfill / freshness)
```

Consequences if such a flag line ever appears on a real page:
1. `MarshalFlags(live.Flags)` ≠ stored `flags_json`, so `upsertItemAndQuests` never short-circuits → the row is re-written on **every** weekly pass forever (defeats the D-06 idempotency the whole `MarshalFlags`-everywhere design exists to guarantee).
2. The flag stored after a backfill (`NO DROP`) differs from the flag the live weekly parse produces (`[]`), so the persisted value flip-flops depending on which path last wrote it.

This does NOT bite today: every flag line in all five real fixtures (`MAGIC ITEM`, `LORE ITEM`, `QUEST ITEM`, `NO DROP`, `TEMPORARY`) is plain text, and round-trip parity (live-RAW vs. backfill-CLEANED) passes for all of them. It is latent, bounded to a wiki convention change, hence Medium not High.
**Fix:** Make the two paths agree by stripping `[[ ]]` link brackets BEFORE flag classification in `parseStatsblock` (render `[[target|display]]`→`display`, `[[target]]`→`target` per line, reusing `summaryLinkRe`, the same rendering `cleanStatsblock`/`parseClicky` already use), so the raw and cleaned forms classify a bracketed flag line identically. Alternatively, add a round-trip-parity regression test that feeds a `[[link]]`-as-flag statsblock through both paths and asserts equal `Flags` — that converts this latent divergence into a guarded invariant.

## Low

### LW-01: Boot backfill runs on `context.Background()` and is uninterruptible by SIGTERM

**File:** `cmd/squirebot-server/main.go:235` (`store.NewStore(db).BackfillItemFlags(context.Background())`)
**Issue:** The backfill runs before the signal-tied context is created (line 243), using `context.Background()`, inside a single `BEGIN IMMEDIATE` transaction over every candidate row (`backfill.go:55-105`). The code comment acknowledges the table is "small — held items only, pre-P38," so today this is a fast local pass. But if P38 grows the catalog to thousands of rows, the boot path holds the write lock for the whole UPDATE loop with no cancellation seam — a SIGTERM during boot cannot unwind it, and startup is delayed. (Performance is out of v1 scope; flagged only for the cancellation/startup-blocking aspect.)
**Fix:** Thread the signal-tied `ctx` (move the `signal.NotifyContext` setup above the backfill call) so a SIGTERM during a future large backfill aborts cleanly. Optionally batch the UPDATEs across multiple transactions when the candidate count is large.

### LW-02: `GetItemMasterSHA1Tx` is production-dead (superseded by `GetItemMasterFreshnessTx`)

**File:** `internal/backendsrv/store/enrich.go:254-265`
**Issue:** `GetItemMasterSHA1Tx` has no production caller — `upsertItemAndQuests` reads freshness exclusively via `GetItemMasterFreshnessTx` (`wiki.go:227`). It is only referenced by `enrich_test.go`. (This predates Phase 37: the freshness getter superseded it when the icon/statsblock backfill arrived; Phase 37 merely extends `GetItemMasterFreshnessTx` with `flags_json`, leaving the SHA1 getter further orphaned.) An exported function whose only consumer is its own test is dead surface and a stale doc reference (`enrich.go:233` still describes the old SHA1-getter short-circuit path).
**Fix:** Either delete `GetItemMasterSHA1Tx` (and fold its "absent → ''" assertion into the freshness-getter test) or document it as test-only. Update the `UpsertItemMasterTx` doc comment at `enrich.go:231-233` to reference `GetItemMasterFreshnessTx`, not the dead SHA1 getter.

## Nit

### NIT-01: `parseClicky` silently drops trailing text after the qualifier

**File:** `internal/backendsrv/enrich/wikiitem.go:649-677`
**Issue:** The clicky name is computed as `raw[:open]` (everything before the LAST `(`), so any text AFTER the qualifier is dropped: `"Effect Name (Click) extra"` → name `"Effect Name"`. Real P1999 Effect lines always place the qualifier last, so this is fine in practice, but the behavior is implicit. (Verified directly; not a defect for current input.)
**Fix:** No change needed for current data. If hardening is desired, document the "qualifier is assumed last" assumption in the function comment.

### NIT-02: `parseHastePct` accepts a nonsense `+-`/`-+` prefix combination

**File:** `internal/backendsrv/enrich/wikiitem.go:624-638`
**Issue:** It strips a leading `+` then a leading `-` independently, so a malformed `"+-5%"` parses to `(true, 5)` rather than rejecting. The wiki only ever writes `+NN%`, so this is purely defensive cosmetics, but the double-TrimPrefix is slightly surprising.
**Fix:** Optional. Strip at most one sign (e.g. `strings.TrimLeft(s, "+-")` is equally permissive but clearer about intent, or validate a single optional leading sign).

---

_Reviewed: 2026-06-25T03:58:44Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard (advisory)_
