---
phase: 31-characters-tab-in-game-inventory-window
reviewed: 2026-06-18T00:00:00Z
depth: standard
files_reviewed: 27
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/compute/inventory.go
  - internal/backendsrv/compute/inventory_test.go
  - internal/backendsrv/compute/slotconst.go
  - internal/backendsrv/compute/types.go
  - internal/backendsrv/enrich/jobs/wiki.go
  - internal/backendsrv/enrich/jobs/wiki_test.go
  - internal/backendsrv/enrich/wikiitem.go
  - internal/backendsrv/enrich/wikiitem_test.go
  - internal/backendsrv/migrations/00012_item_icon.sql
  - internal/backendsrv/migrations/00013_item_statsblock.sql
  - internal/backendsrv/migrations/migrate_test.go
  - internal/backendsrv/readapi/characters.go
  - internal/backendsrv/readapi/inventory.go
  - internal/backendsrv/readapi/readapi_test.go
  - internal/backendsrv/store/enrich.go
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/store/readviews_test.go
  - web/src/lib/__tests__/examine.test.ts
  - web/src/lib/__tests__/roster.test.ts
  - web/src/lib/api.ts
  - web/src/lib/components/ExaminePanel.svelte
  - web/src/lib/components/InventoryWindow.svelte
  - web/src/lib/components/PaperdollSlot.svelte
  - web/src/lib/examine.ts
  - web/src/lib/roster.ts
  - web/src/routes/characters/+page.svelte
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 31: Code Review Report

**Reviewed:** 2026-06-18T00:00:00Z
**Depth:** standard
**Files Reviewed:** 27
**Status:** issues_found

## Summary

Phase 31 adds the Characters tab, the in-game inventory window, two extend-only migrations
(00012 icon_id / 00013 statsblock), and a weekly wiki-enrichment carry-through. I reviewed
the four threat areas the prompt flagged most heavily — SQL injection on the `{char}` path,
session/auth gating on the new read routes, the `{@html}` XSS sink in the examine panel, and
migration idempotency/extend-only safety — and all four are SOUND:

- **SQL injection:** every user-controlled value (`{char}`, the viewer id) binds only as a
  `?` placeholder. `InventoryForChar` and `RosterFor` build no SQL from input; the only
  dynamic SQL anywhere (`InventoryJoin`'s `bankOnly` branch) is a fixed-string switch on a
  bool, never a value interpolation. No string concatenation reaches a query.
- **Auth gating:** both new routes are registered under `webauth.RequireSession`
  (main.go:362-363), which is fail-closed (no/invalid/expired cookie → 401, inner handler
  never runs). `TestNewRoutes_RequireSession_401WithoutCookie` proves both routes 401 without
  a cookie. The roster's viewer-first ordering is presentation only — never access control.
- **XSS sink:** the single `{@html wikiBodyHtml}` in ExaminePanel routes through
  `composeItemNote`, whose every interpolated value passes `escapeHtml()` and whose href
  passes the `safeHttpUrl()` http(s) scheme allow-list. A `javascript:`/`data:` URL or a
  `<img onerror=…>` item name renders inert. Every other field is plain `{}` interpolation
  (Svelte auto-escapes). PaperdollSlot's `<img src>` uses only an integer (`Item_${iconId}.png`)
  — `parseIconID` guarantees a non-negative int, so no guildie string reaches the URL.
- **Migrations:** 00012/00013 are single-column `ALTER TABLE ADD COLUMN` (nullable, no
  DEFAULT/UNIQUE) — the extend-only pattern; goose records applied versions so re-runs no-op.

The findings below are all WARNING/INFO: a frontend logic divergence that renders an empty
wiki row, a test-coverage gap on 00013, a couple of robustness/consistency nits, and some
documentation/style observations. Nothing blocks ship.

## Warnings

### WR-01: ExaminePanel renders an empty wiki paragraph when the URL fails the safe-scheme check

**File:** `web/src/lib/examine.ts:97-99`, `web/src/lib/components/ExaminePanel.svelte:40-45,59-62`
**Issue:** The `wiki` examine field is decided by `wikiHref(slot)` (examine.ts), which returns
the raw `slot.wiki_url` (or a derived URL) **without** running `safeHttpUrl`. The rendered HTML,
however, comes from a separate `$derived` (`wikiBodyHtml`) that DOES run
`safeHttpUrl(slot.wiki_url || wikiUrlFor(slot.item))` and returns `''` when the scheme is
rejected. So for a slot whose stored `wiki_url` is non-empty but non-http(s) (e.g. a malformed
or `javascript:`/`data:` value), `examineFields` still emits a `wiki` field (passing the
`{#if f.kind === 'wiki'}` branch) while `wikiBodyHtml` is `''` — the template renders
`<p class="ex-wiki">{@html ''}</p>`, an empty bordered paragraph. Not a security hole (the sink
is still sanitized — this is the *absence* of a link, not an injected one), but a visible
broken/empty row. The two code paths use different "is there a usable wiki link?" predicates.
**Fix:** Make the field-presence decision use the same gate as the render. Either run the
href through `safeHttpUrl` in `wikiHref` so a rejected URL drops the field entirely:
```ts
// examine.ts wikiHref — gate the stored URL on the same allow-list the panel uses
import { safeHttpUrl } from './tooltip/composeNotes';
function wikiHref(slot: InventorySlot): string {
	const stored = safeHttpUrl(slot.wiki_url?.trim() ?? '');
	if (stored) return stored;
	const name = slot.item?.trim();
	if (!name) return '';
	return 'https://wiki.project1999.com/' + encodeURIComponent(name.replace(/ /g, '_'));
}
```
or, in ExaminePanel, guard the render on a non-empty body: `{:else if f.kind === 'wiki' && wikiBodyHtml}`.

### WR-02: Migration 00013 (statsblock) has no migration test

**File:** `internal/backendsrv/migrations/migrate_test.go` (00012 covered at line 1024; 00013 absent)
**Issue:** `migrate_test.go` adds `TestMigrate_00012_AddsItemIcon` (column exists, NULL-default
round-trip, idempotent re-run) but there is no parallel `TestMigrate_00013_AddsStatsblock`.
00013 is the LAST migration applied by `NewTestDB`, so `goose.Up` reaching HEAD is exercised
indirectly by every store/compute test — but the column's NULL-default and round-trip behavior
(the things the 00012 test asserts) are unverified for statsblock specifically. A future edit to
00013 (e.g. accidentally adding a NOT NULL / DEFAULT) would not be caught by a dedicated test.
**Fix:** Add a `TestMigrate_00013_AddsStatsblock` mirroring `TestMigrate_00012_AddsItemIcon`:
assert `statsblock` exists on `item_master` (via `columnSet`), a row inserted without it reads
NULL, a row inserted with text round-trips, and a second `RunMigrations` is a no-op.

### WR-03: Down-migrations use `ALTER TABLE DROP COLUMN` — version-dependent, untested

**File:** `internal/backendsrv/migrations/00012_item_icon.sql:14`, `00013_item_statsblock.sql:15`
**Issue:** Both `+goose Down` blocks use `ALTER TABLE item_master DROP COLUMN`. `DROP COLUMN`
requires SQLite ≥ 3.35.0; modernc.org/sqlite bundles a recent engine so this works today, but
(a) it couples the rollback path to the bundled SQLite version, and (b) no test exercises the
Down path (the suite is forward-only, matching production's `goose.Up`-only posture). The risk
is contained — production never runs Down, and the comments correctly call these forward-only —
but a rollback attempted on an older/embedded SQLite would fail silently outside CI. This is a
robustness note, not a defect in the shipped (forward) path.
**Fix:** Either add a comment that Down is dev/CI-only and depends on SQLite ≥ 3.35, or (lower
priority) skip the assertion entirely. No code change strictly required for ship.

### WR-04: `metaLine` joins race + class with a single space, losing the visual separator

**File:** `web/src/routes/characters/+page.svelte:134-140`
**Issue:** `metaLine` builds `["Level 60", "Half Elf", "Ranger"].join(' ')` → `"Level 60 Half
Elf Ranger"`. Race and class are concatenated with the same single space used between every
token, so a two-word race (`Half Elf`, `Dark Elf`, `High Elf`) is visually indistinguishable
from the class that follows it ("...Half Elf Ranger" reads ambiguously). The InventoryWindow
char-head shows only `Last synced` and does not repeat this, so this list-row meta line is the
only place the ambiguity surfaces. Cosmetic, but it degrades scannability of the roster — the
primary value of the Characters tab.
**Fix:** Use a stronger separator between the level/race/class facets, e.g.
`parts.join(' · ')` (middot) or render them as discrete spans, so "Level 60 · Half Elf · Ranger"
is unambiguous.

## Info

### IN-01: Raw ISO timestamp surfaced verbatim in the examine "Last synced" / char-head

**File:** `web/src/lib/examine.ts:104-107`, `web/src/lib/components/InventoryWindow.svelte:130`
**Issue:** `examineFields` renders `Last synced: ${charLastSeen}` and the window head renders
`Last synced ${inventory.last_seen}` using the raw ISO string (e.g.
`2026-06-18T12:00:00Z`). Every other date in the app went through a relative/locale formatter
(the api.ts notes warn about epoch-seconds Date construction); here the value is shown
unformatted. Functionally correct, just not user-friendly.
**Fix:** Run `charLastSeen` / `last_seen` through the existing relative/locale date helper
before display (or `new Date(...).toLocaleString()`), consistent with the rest of the UI.

### IN-02: `parseIconID` clamps negatives to 0 but accepts arbitrarily large ints

**File:** `internal/backendsrv/enrich/wikiitem.go:479-489`
**Issue:** `parseIconID` correctly rejects blank/non-numeric/negative values (returning the
0 "no-icon" sentinel), which is the load-bearing type-safety control for the
`Item_${iconId}.png` URL. It does not bound the upper end, so a wiki page with an absurd
`lucy_img_ID` (e.g. a 12-digit number) would be stored and emitted into the image URL. The
worst case is a 404 image (the `onerror` handler hides it and the colored-tile shows through),
so there is no correctness or security impact — noting only that the value is unbounded.
**Fix:** Optional — none needed; the colored-tile fallback already absorbs a bad id.

### IN-03: `canonicalNumbered` slices by byte length assuming an ASCII prefix

**File:** `internal/backendsrv/compute/inventory.go:93-96`
**Issue:** `canonicalNumbered(prefix, parent)` does `parent[len(prefix):]`. This is safe because
the only callers pass literal ASCII prefixes ("General"/"Bank"/"SharedBank") guarded by the
matching `(?i)` regex, so `len(prefix)` is always a valid byte boundary. The comment already
asserts the invariant. Flagging only so a future caller passing a non-ASCII prefix knows the
slice is byte-based, not rune-based.
**Fix:** None required given current callers; the regex guard guarantees correctness.

### IN-04: `InventoryWindow` keys grids by `location + '#' + i` but the paperdoll keys by canonical slot

**File:** `web/src/lib/components/InventoryWindow.svelte:52-58,257,288`
**Issue:** The general/bank grids key the `{#each}` on `s.location + '#' + i` (the defensive
fix for duplicate-Location data that froze the window). The paperdoll, by contrast, builds
`equipBySlot` keyed on `canonical_slot` and silently keeps only the LAST slot for a duplicated
canonical key (Map.set overwrites). For equipment this is fine because the paired Ear/Wrist/
Finger slots are now numbered upstream into distinct canonical keys (Ear1/Ear2/...), and the
`TestStructuredInventory_PairedSlots` test proves no two share a canonical. So the overwrite
can't drop a real slot today — noting only that the equipment path's de-dup safety lives
entirely in the backend numbering, with no client-side guard.
**Fix:** None required; the backend invariant holds and is tested. A defensive comment in
`equipBySlot` noting "relies on the backend numbering paired slots distinctly" would help.

### IN-05: Comment in `compute/inventory.go` references "CR-01" robustness from a prior review

**File:** `internal/backendsrv/compute/inventory.go:131-144`
**Issue:** The `buildStructuredInventory` doc-comment refers to "CR-01 robustness" (a finding
ID from an earlier review cycle, also referenced in the test names). The dangling-pointer fix it
describes is real, correct, and well-tested (`TestStructuredInventory_OrphanBeforeContainer[_Bank]`).
Noting only that the cross-reference to a prior review's ID is opaque to a reader without that
context.
**Fix:** None required — the code is correct and the tests are excellent. Optionally restate the
invariant ("never retain a pointer into a slice that is later appended to") without the
review-ID reference.

---

_Reviewed: 2026-06-18T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
