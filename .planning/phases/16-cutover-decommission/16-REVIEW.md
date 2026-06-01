---
phase: 16-cutover-decommission
reviewed: 2026-05-31T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - internal/backendsrv/enrich/eqconst.go
  - internal/backendsrv/store/charmeta.go
  - internal/backendsrv/store/readviews.go
  - internal/backendsrv/webadmin/charmeta.go
  - internal/backendsrv/webadmin/charmeta_test.go
  - web/src/lib/__tests__/charmeta.test.ts
  - web/src/lib/api.ts
  - web/src/lib/charmeta.ts
  - web/src/lib/components/CharMetaForm.svelte
  - web/src/lib/components/SiteShell.svelte
  - web/src/routes/char-meta/+page.svelte
findings:
  critical: 0
  high: 0
  medium: 1
  low: 2
  info: 3
  total: 6
status: issues
---

# Phase 16: Code Review Report

**Reviewed:** 2026-05-31
**Depth:** standard (per-file + cross-file clone-drift trace against the Phase-15 bank-coin trio)
**Files Reviewed:** 12
**Status:** issues_found (1 medium, 2 low, 3 info — no Critical/High)

## Summary

Phase 16's only source change is the 16-01 char-metadata web form + write endpoint, a deliberate line-by-line clone of the reviewed Phase-15 bank-coin trio (`coin.go` / `coin.ts` / `BankCoinForm.svelte` / the coin handler). I verified the build (`go build ./...` clean), `go vet ./internal/backendsrv/...` (clean), the targeted Go suites (`webadmin`, `store`, `cmd/squirebot-server` all `ok`), and the web vitest (`charmeta.test.ts` 22/22 pass), then traced each new file against its bank-coin sibling for clone-drift.

**Invariants confirmed to hold:**

- **Login-only write (D-03):** both routes wrap `webauth.RequireSession`, never `RequireOfficer` (`main.go:336-337`); the handler consults officer status nowhere (`charmeta.go` — no `IsOfficer`/`RequireOfficer`); the SiteShell nav places the link under `{#if session?.authenticated}`, not the `session?.isOfficer` block (`SiteShell.svelte:54-62`); the page has no officer check (`+page.svelte`). Matches the coin precedent exactly.
- **Parameterized SQL only:** the `UPDATE` and both `SELECT`s use `?` placeholders (`charmeta.go:48`, `readviews.go:288`); no value interpolation.
- **Fail-closed scoping:** the write is scoped `WHERE id = ? AND is_removed = 0` and returns `ErrCharNotFound` on `RowsAffected()==0` → 400 `invalid_input` (`charmeta.go:47-58`); the list reads `WHERE is_removed = 0`. Tests prove a removed/missing char is rejected and writes nothing.
- **Audited transaction idiom:** the meta write + the `char_meta_set` audit row compose in one `withTx` (BEGIN IMMEDIATE) and commit together; a store error rolls both back (`charmeta.go:105-112`, `audit.go:88-107`). Test asserts exactly 1 audit row on success, 0 on rejection.
- **CR-01 (the P15 production crash):** the level input is `type="text" inputmode="numeric"` with no `type="number"` (`CharMetaForm.svelte:186-195`); all user input funnels through `rawToTrimmed`, which tolerates `string|number|null|undefined` (`charmeta.ts:57-61`), so a coerced `number`/`null` cannot throw `.trim is not a function`. The Style-B source assertions + the number-coercion unit tests lock this.
- **XSS (T-16-03):** the character name renders via plain `{}` interpolation in the `<option>` labels (`CharMetaForm.svelte:153`) and the success copy (`res.character`, line 114); no `{@html}` anywhere.
- **Extend-only schema:** `CharMeta` gained `IsBankToon` at the right edge and `CharsWithMeta` selects `is_bank_toon` last (`readviews.go:92-98, 288`); no migration (the columns pre-exist from `00001_init.sql`). `compute.GearCheck`/`SpellCheck` use field access on `Class`/`Level`/`Race` and ignore the new field — confirmed unaffected (`gearcheck.go:84-92`, `spellcheck.go:52-58`).

The one substantive finding (MD-01) is a semantic gap, not a clone defect: char-meta is the **first and only** production writer of `is_bank_toon=true`, and it has no single-bank-toon guard, which the `bank` compute view assumes. Everything else is minor.

## Medium

### MD-01: char-meta is the first writer of `is_bank_toon=true` and can silently create a second bank-toon, breaking the `bank` view's documented single-toon invariant

**File:** `internal/backendsrv/store/charmeta.go:38-58` (write path); `internal/backendsrv/compute/bank.go:22-39` (consumer)
**Severity:** Medium

**Issue:** A grep across `internal/` confirms `SetCharMetaTx` is the only production code path that sets `is_bank_toon` to a non-default value (`is_bank_toon = ?` at `charmeta.go:48`); the ingest path never writes it. The `bank` compute view is built on `InventoryJoin(ctx, true)` whose `bankOnly` branch is `... AND c.is_bank_toon = 1` (`readviews.go:143`) and explicitly documents the assumption:

> `bank.go:24-25`: "the bankOnly join is scoped to the single is_bank_toon character; Char is constant within it"

Nothing in this new write path — nor in the schema (`00001_init.sql` has no partial-unique index on `is_bank_toon`) nor in `SetCharMetaTx` — prevents two live characters from being flagged `is_bank_toon=1` at once. Any authenticated member can flag a second character via the form. When that happens:

- `InventoryJoin(bankOnly=true)` returns inventory rows from **both** bank-toons; `Char` is no longer constant, silently violating the documented invariant.
- The `bank` view (`/api/v1/views/bank`) then mixes two characters' inventories with no separation; `BankResponse.rows[]` carries multiple `char` values the UI does not expect.
- The `BankCoinForm` picker (`fetchBankToons` → `ListBankToons`, `coin.go:46-51`) lists multiple bank-toons, so "the guild bank's coin" becomes ambiguous.

This does not crash (the builders iterate rows generically), so it is Medium, not a BLOCKER — but it is a real correctness/semantics regression that this build newly makes reachable, and exactly the class of issue the review brief called out. The P15 coin trio never created this exposure because no production writer set the flag.

**Fix:** Enforce single-bank-toon at the store write, inside the existing tx, so the flag is mutually exclusive. Minimal version (demote any other bank-toon when this write sets the flag):

```go
// In SetCharMetaTx, before the UPDATE, when isBankToon is true:
if isBankToon {
    if _, err := tx.ExecContext(ctx,
        `UPDATE character SET is_bank_toon = 0 WHERE is_bank_toon = 1 AND id <> ? AND is_removed = 0`,
        characterID,
    ); err != nil {
        return fmt.Errorf("demote prior bank toon: %w", err)
    }
}
```

Alternatively reject the write with a dedicated error (`ErrBankToonExists` → 400) when another live bank-toon exists, and surface a clear message in the form. Either way, document the chosen rule next to the `is_bank_toon` column and in `bank.go`. (If the product intent is genuinely "exactly one officer-curated bank-toon," consider whether the `is_bank_toon` toggle belongs on a login-only form at all — a design question for the orchestrator, not a code fix.)

## Low

### LR-01: `main_test.go` does not assert the char-meta route gate, contradicting the test file's own docstring

**File:** `internal/backendsrv/webadmin/charmeta_test.go:9-10`; `cmd/squirebot-server/main_test.go:258-336`
**Severity:** Low

**Issue:** `charmeta_test.go` states: "The route-level gate (RequireSession vs RequireOfficer) is asserted in cmd/squirebot-server/main_test.go." That is **not** true — `TestWriteRoutes_Gates` (`main_test.go:258`) wires and asserts the coin routes (`/api/v1/coin`, `/api/v1/coin/bank-toons`) and the officer routes, but it does **not** register or exercise `/api/v1/char/meta` or `/api/v1/char/meta-list`. The D-03 login-only gate for char-meta is therefore proven only at the handler layer — which is gate-agnostic (the handler never checks officer status either way); the actual `RequireSession`-vs-`RequireOfficer` wiring in `runServe` has no route-level regression test. The wiring is correct today (`main.go:336-337`), but a future edit that swapped in `RequireOfficer` would pass every green test, which is precisely the bypass the bank-coin route test exists to prevent.

**Fix:** Extend `TestWriteRoutes_Gates` to mirror the char-meta wiring, exactly as it does for coin:

```go
mux.Handle("GET /api/v1/char/meta-list", webauth.RequireSession(db, webadmin.CharMetaListHandler(db)))
mux.Handle("POST /api/v1/char/meta", webauth.RequireSession(db, webadmin.CharMetaSetHandler(db)))
// ...then assert: anon → 401, and a plain MEMBER session → admitted (NOT 401/403).
```

Either add the assertion or correct the docstring so it does not claim coverage that is absent.

### LR-02: success/error message lifecycle — verified-correct parity, recorded so it is not assumed-skipped

**File:** `web/src/lib/components/CharMetaForm.svelte:87-95, 114`
**Severity:** Low

**Issue:** `onSave` sets `successMsg = `Saved details for ${res.character}.`` (line 114); `onSelect` clears `successMsg`/`errorMsg` (lines 90-91) on the `<select>`'s `onchange`. After a successful save the form intentionally keeps the same selection ("success keeps the select so a second edit is easy"), so the success banner remains visible alongside the now-re-disabled Save button — the intended UX. Changing the picker correctly clears the banner. This is the message/lifecycle area where clone-drift commonly hides; I traced it against `BankCoinForm.svelte` (lines 88-89, 111) and it matches one-for-one. No behavioral defect.

**Fix:** None required — documented as confirmed-correct parity.

## Info

### IN-01: handler echoes `level` as `*int64` while the coin sibling echoes concrete values — harmless, field unused by the client

**File:** `internal/backendsrv/webadmin/charmeta.go:136-142`; `web/src/lib/components/CharMetaForm.svelte:104-114`
**Severity:** Info

**Issue:** The success reply puts `"level": req.Level` (a `*int64`, so JSON `null` when blank) into the response map, and `SaveCharMetaResult.level` is typed `number | null` (`api.ts:316`). This diverges from the coin handler, which echoes concrete `int64`s. It is harmless: the form's optimistic local update (`CharMetaForm.svelte:109-113`) reflects `payload.*` (the locally computed values), not `res.*`, and only consumes `res.character` for the success copy (line 114). The echoed `class`/`level`/`race`/`is_bank_toon` fields are never read by the client. No fix needed; noting because an echo/optimistic-update mismatch is a classic clone-drift bug and here it was verified inert.

### IN-02: `validateLevel`'s `Number.isSafeInteger` guard is unreachable given the preceding `/^\d+$/` + `<= 60` bound

**File:** `web/src/lib/charmeta.ts:78-85`
**Severity:** Info

**Issue:** `validateLevel` checks `/^\d+$/` then `Number.isSafeInteger(n) && n >= 1 && n <= 60`. A value that passed `/^\d+$/` and is `<= 60` is always a safe integer, so the `Number.isSafeInteger` clause can never independently reject. This is faithfully inherited from `coin.ts`'s `validateCoinField` (where it IS load-bearing — coin has no upper bound, so a 20-digit string must be caught by the safe-integer guard). In the char-meta clone the `<= 60` cap makes it dead, but it is defensively harmless and keeps the two helpers structurally identical. Leave as-is for parity.

### IN-03: client/server value-set duplication (`CLASSES`/`RACES` in both `charmeta.ts` and `eqconst.go`) is intentional but unguarded against drift

**File:** `web/src/lib/charmeta.ts:17-30`; `internal/backendsrv/enrich/eqconst.go:26-38`
**Severity:** Info

**Issue:** The 14 class + 14 race abbreviations are hand-duplicated in TypeScript (`charmeta.ts`) and Go (`eqconst.go`), both claiming to mirror `apps-script/src/lib/eq-constants.ts`. The vitest asserts the TS lists have length 14 and contain `WAR`/`ENC`/`IKS`/`VAH` (`charmeta.test.ts:48-58`), but nothing cross-checks the TS set against the Go set, so a future edit to one (e.g. a 15th class) could silently desync the client `<select>` from the server validator. The server is authoritative (`validCharMeta` → 400), so a desync degrades UX (a valid choice rejected, or a client-offered choice 400'd) rather than corrupting data — hence Info. The "IKS"-is-load-bearing comments in both files are correct and present. No change required; flagging the drift surface.

---

_Reviewed: 2026-05-31_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
