# Phase 16: Cutover + Decommission - Pattern Map

**Mapped:** 2026-05-31
**Files analyzed:** 7 new/modified code files (the char-meta form trio) + 1 new doc + 3 ops touch-points (reference-only)
**Analogs found:** 7 / 7 (every code file has an exact in-repo analog — the shipped bank-coin trio)

> **The phase's center of gravity is ONE build: the char-metadata form (CUTOVER-02), a strict clone of the shipped bank-coin trio.** Everything else (publish v2.0.0 Release, mint codes, disable Apps Script triggers, retire OAuth client) is operational/runbook with no code-clone target. The planner should copy the bank-coin patterns verbatim and spend attention on (a) the CR-01 input-type lesson for the `level` field and (b) the ops sequencing.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/backendsrv/webadmin/charmeta.go` | controller (HTTP handler) | request-response (CRUD write) | `internal/backendsrv/webadmin/coin.go` | **exact** (clone) |
| `internal/backendsrv/store/charmeta.go` | model (store mutator) | CRUD write | `internal/backendsrv/store/coin.go` (`SetCoinTx`) | **exact** (clone) |
| `internal/backendsrv/enrich/eqconst.go` (EDIT: add `RACES`) | config (value set) | n/a (constant) | same file's `CLASSES` + apps-script `RACES` | **exact** (port) |
| `cmd/squirebot-server/main.go` (EDIT: 2 route lines) | route (wiring) | request-response | `main.go:328-330` (the `/api/v1/coin` block) | **exact** (sibling insert) |
| `web/src/lib/charmeta.ts` | utility (pure validation helpers) | transform | `web/src/lib/coin.ts` | **exact** (clone) |
| `web/src/lib/api.ts` (EDIT: interfaces + wrappers) | utility (typed fetch) | request-response | `api.ts` (`fetchBankToons`/`saveCoin` + `postJSON`/`getJSON`) | **exact** (sibling insert) |
| `web/src/lib/components/CharMetaForm.svelte` | component | request-response | `web/src/lib/components/BankCoinForm.svelte` | **exact** (clone) |
| `web/src/routes/char-meta/+page.svelte` | route (page) | request-response | `web/src/routes/bank-coin/+page.svelte` | **exact** (clone) |
| `web/src/lib/__tests__/charmeta.test.ts` | test | n/a | `web/src/lib/__tests__/coin.test.ts` + `eviction.test.ts` | **exact** (clone, both styles) |
| `internal/backendsrv/webadmin/charmeta_test.go` | test | n/a | `internal/backendsrv/webadmin/coin_test.go` | **exact** (clone) |
| `docs/decommission-checklist.md` | doc (proof artifact) | n/a | `docs/eviction-runbook.md` | role-match (style only) |

**No migration file.** `internal/backendsrv/migrations/00001_init.sql:12-15` already declares `class TEXT`, `level INTEGER`, `race TEXT`, `is_bank_toon INTEGER NOT NULL DEFAULT 0` (commented "set later / by backfill (P16)"). The form writes to existing storage — a new migration would be churn (RESEARCH Anti-Pattern; D-02).

---

## Pattern Assignments

### `internal/backendsrv/webadmin/charmeta.go` (controller, request-response)

**Analog:** `internal/backendsrv/webadmin/coin.go` — copy `CoinSetHandler` (the POST) + `BankToonsHandler` (the GET pick-list) shape verbatim.

**The handler convention (load-bearing — `audit.go` package doc, lines 9-26):** method-check first → JSON `{"error":"code"}` bodies via `writeJSONError` with EXACT codes the frontend routes (`invalid_input`, etc.) → never log a secret/raw body → every mutating handler opens ONE `*sql.Tx` (DSN is `_txlock=immediate` ⇒ BEGIN IMMEDIATE) composing the store `*Tx` mutator + `AppendAuditTx`, committed atomically.

**Imports pattern** (`coin.go:22-30`):
```go
import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)
```
(char-meta also needs `github.com/boejowen/SquireBot/internal/backendsrv/enrich` for `CLASSES`/`RACES` validation.)

**Login-only request struct** (`coin.go:35-41` → swap fields):
```go
type coinReq struct {
	CharacterID int64 `json:"character_id"`
	Plat        int64 `json:"plat"`
	// ...
}
// char-meta twin:
type charMetaReq struct {
	CharacterID int64  `json:"character_id"`
	Class       string `json:"class"`
	Level       int64  `json:"level"`
	Race        string `json:"race"`
	IsBankToon  bool   `json:"is_bank_toon"`  // a JSON bool — the decoder validates the type
}
```

**Core POST handler — the single most load-bearing pattern to clone** (`coin.go:70-128`):
```go
func CoinSetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx := r.Context()
		var req coinReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		if !validCoin(req) {                       // server-side re-validation (T-15-29)
			writeJSONError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		// The acting identity is recorded for AUDIT ONLY — NOT an authorization input
		// (D-12/D-03: any authenticated member may write).
		writer := caller(ctx)
		now := nowUnix()
		err := withTx(ctx, db, func(tx *sql.Tx) error {     // ONE BEGIN IMMEDIATE tx
			if e := store.SetCoinTx(ctx, tx, req.CharacterID, req.Plat, req.Gold, req.Silver, req.Copper); e != nil {
				return e
			}
			return AppendAuditTx(ctx, tx, "coin_set", writer, map[string]any{
				"character_id": req.CharacterID,
			}, now)
		})
		if err != nil {
			if errors.Is(err, store.ErrNotBankToon) {
				writeJSONError(w, http.StatusBadRequest, "not_bank_toon")
				return
			}
			slog.Error("coin set failed", "character_id", req.CharacterID, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, map[string]any{ /* echo name + saved values */ })
	}
}
```
**For char-meta:** rename → `CharMetaSetHandler`; swap `validCoin`→`validCharMeta`; swap `SetCoinTx`→`SetCharMetaTx(ctx, tx, req.CharacterID, req.Class, req.Level, req.Race, req.IsBankToon)`; audit event `"char_meta_set"`. If the store returns a not-found error (RowsAffected==0, see store pattern), map it to `writeJSONError(w, 400, "invalid_input")` (mirrors the `ErrNotBankToon` branch's fail-closed shape).

**Server-side validation helper** (`coin.go:135-142` — `validCoin` walks the four ints and rejects negatives; char-meta needs a richer check):
```go
// validCharMeta — server-side V5 re-check (NEVER trust the form's <select>; T-15-29):
//   class ∈ enrich.CLASSES, race ∈ enrich.RACES, level 1..60 (A2), is_bank_toon is bool (decoder-validated).
func validCharMeta(req charMetaReq) bool {
	if !slices.Contains(enrich.CLASSES, req.Class) { return false }   // exact uppercase abbrev
	if !slices.Contains(enrich.RACES, req.Race)    { return false }
	if req.Level < 1 || req.Level > 60             { return false }
	return true
}
```
> ⚠️ **Pitfall 5 (value-set drift):** `compute/gearcheck.go:35` keys the Iksar tier on the literal string `"IKS"`, and the spell/gear joins match on exact uppercase class abbreviations (`WAR`,…). Validate against `enrich.CLASSES`/`enrich.RACES` and **store the abbreviation, not a display name** — a typo or `"Warrior"` silently produces zero gear/spell rows. (A2: confirm whether blank/`level=0` should be allowed = "unset" → if so, widen to `0..60` or treat blank as NULL; default is 1–60.)

**The GET pick-list handler** clones `BankToonsHandler` (`coin.go:47-64`): method-check GET → call `store.CharsWithMeta(r.Context())` → `if list == nil { list = []store.CharMeta{} }` → `writeJSON(w, list)`. Reuses `store.CharsWithMeta` as-is (see Shared Patterns).

**Out-of-package symbols already provided by the `webadmin` package** (do NOT re-declare — they live in `coin.go`/`audit.go`/siblings): `caller(ctx)`, `nowUnix()`, `withTx`, `AppendAuditTx`, `writeJSON`, `writeJSONError`.

---

### `internal/backendsrv/store/charmeta.go` (model, CRUD write)

**Analog:** `internal/backendsrv/store/coin.go` — `SetCoinTx` is the mutator template.

**Store-layer invariants** (`coin.go` package doc, lines 15-16): parameterized `?` placeholders ONLY (V5); the mutator takes the caller's `*sql.Tx` so the handler composes the write + the audit row in one tx.

**The mutator to clone** (`coin.go:89-110`):
```go
func SetCoinTx(ctx context.Context, tx *sql.Tx, characterID, plat, gold, silver, copper int64) error {
	var isBank int
	err := tx.QueryRowContext(ctx,
		`SELECT is_bank_toon FROM character WHERE id = ?`, characterID,
	).Scan(&isBank)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotBankToon // no such char ⇒ fail-closed
	case err != nil:
		return fmt.Errorf("check bank toon (character_id=%d): %w", characterID, err)
	}
	if isBank != 1 {
		return ErrNotBankToon
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE character SET plat = ?, gold = ?, silver = ?, copper = ? WHERE id = ?`,
		plat, gold, silver, copper, characterID,
	); err != nil {
		return fmt.Errorf("set coin (character_id=%d): %w", characterID, err)
	}
	return nil
}
```
**The char-meta twin is SIMPLER — no bank-toon gate (setting `is_bank_toon` IS one of the columns):**
```go
// SetCharMetaTx writes class/level/race/is_bank_toon onto an EXISTING, non-removed
// character inside the caller's tx. Parameterized ? only (V5). The is_removed=0
// scoping mirrors CharsWithMeta/ListBankToons (the form edits live chars only; D-03
// forbids pre-creating rows). A RowsAffected()==0 → ErrCharNotFound (fail-closed,
// mirroring ErrNotBankToon's shape) so the handler maps it to invalid_input.
var ErrCharNotFound = errors.New("char_not_found")

func SetCharMetaTx(ctx context.Context, tx *sql.Tx, characterID int64, class string, level int64, race string, isBankToon bool) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE character SET class = ?, level = ?, race = ?, is_bank_toon = ? WHERE id = ? AND is_removed = 0`,
		class, level, race, boolToInt(isBankToon), characterID,
	)
	if err != nil {
		return fmt.Errorf("set char meta (character_id=%d): %w", characterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCharNotFound
	}
	return nil
}
```
(`is_bank_toon` is an `INTEGER NOT NULL DEFAULT 0` column — write `1`/`0`, not a Go bool. Use a tiny `boolToInt` helper or inline.)

**No new read query needed** — the pick-list reuses `store.CharsWithMeta` (Shared Patterns).

---

### `internal/backendsrv/enrich/eqconst.go` (config — EDIT: add `RACES`)

**Analog:** the same file's `CLASSES` slice (`eqconst.go:26-29`) + the apps-script `RACES` source (`apps-script/src/lib/eq-constants.ts:44-47`). This resolves **Open Question 1** (RACES is not yet in Go).

**Existing Go `CLASSES` (server-side validation set — `eqconst.go:26-29`):**
```go
// CLASSES is the canonical ordered list of the 14 P1999 class abbreviations,
// ported verbatim from eq-constants.ts (`CLASSES`).
var CLASSES = []string{
	"WAR", "CLR", "PAL", "RNG", "SHD", "DRU", "MNK", "BRD",
	"ROG", "SHM", "NEC", "WIZ", "MAG", "ENC",
}
```

**The `RACES` to PORT VERBATIM** — the TS source (`apps-script/src/lib/eq-constants.ts:44-47`):
```typescript
export const RACES = [
  'HUM', 'BAR', 'ERU', 'ELF', 'HIE', 'DEF', 'HEF', 'DWF',
  'TRL', 'OGR', 'HFL', 'GNM', 'IKS', 'VAH',
] as const;
```
**Add to `eqconst.go` (1:1 port, follows the `CLASSES` precedent in the same file):**
```go
// RACES is the canonical ordered list of the 14 P1999 race abbreviations,
// ported verbatim from eq-constants.ts (`RACES`). Used for char-meta server-side
// validation; the frontend <select> mirrors it. ("IKS" is load-bearing — compute/
// gearcheck.go keys the Iksar tier on this exact literal.)
var RACES = []string{
	"HUM", "BAR", "ERU", "ELF", "HIE", "DEF", "HEF", "DWF",
	"TRL", "OGR", "HFL", "GNM", "IKS", "VAH",
}
```
The frontend `<select>` option lists for `class` and `race` mirror these 14+14 abbreviations (UX only; the server is authoritative).

---

### `cmd/squirebot-server/main.go` (route — EDIT: 2 sibling lines)

**Analog:** the login-only `/api/v1/coin` block (`main.go:328-330`). Register the 2 new routes under `webauth.RequireSession` (login-only, D-03), beside the coin routes. The whole mux is CORS-wrapped once below (lines 332-336), so no CORS change.

**The block to extend** (`main.go:328-330`):
```go
// Bank-coin — LOGIN-ONLY (D-12): RequireSession, NOT RequireOfficer.
mux.Handle("GET /api/v1/coin/bank-toons", webauth.RequireSession(db, webadmin.BankToonsHandler(db)))
mux.Handle("POST /api/v1/coin", webauth.RequireSession(db, webadmin.CoinSetHandler(db)))

// ADD (login-only, D-03 — char-meta is non-sensitive shared data, any member writes):
mux.Handle("GET /api/v1/char/meta-list", webauth.RequireSession(db, webadmin.CharMetaListHandler(db)))
mux.Handle("POST /api/v1/char/meta",     webauth.RequireSession(db, webadmin.CharMetaSetHandler(db)))
```
> ⚠️ **Use `RequireSession`, NOT `RequireOfficer`** (D-03, A5). The officer-only block is directly above (lines 319-326) — char-meta belongs with the login-only coin block, not there. Route names `/api/v1/char/meta` + `/api/v1/char/meta-list` are a planning choice (A5); any `/api/v1/`-prefixed login-only pair is fine. The CLI subcommand dispatch (`mint-code`, `main.go:79-80`) is unchanged.

---

### `web/src/lib/charmeta.ts` (utility — pure validation helpers)

**Analog:** `web/src/lib/coin.ts`. Extract ALL validation/change-detection logic into a pure `.ts` so it is node-unit-testable WITHOUT a DOM (the repo's established philosophy — `coin.ts` header, lines 1-7; the form `.svelte` is a thin renderer over these).

**The CR-01 input contract — the single most important thing to clone** (`coin.ts:18-40`). The form binds a TEXT input, but Svelte's number coercion (or a future `type="number"`) can write `number`/`null` back into the bound store, so every helper accepts a union and normalizes at one choke point:
```typescript
export type CoinRaw = string | number | null | undefined;

// Normalize whatever the binding produced to the trimmed string the logic expects.
// null/undefined → ''; a number → its string form (non-finite → '', fractional keeps
// its decimal so /^\d+$/ still rejects it). The single choke point that makes every
// helper crash-proof regardless of the input element.
function rawToTrimmed(raw: CoinRaw): string {
	if (raw === null || raw === undefined) return '';
	if (typeof raw === 'number') return Number.isFinite(raw) ? String(raw) : '';
	return raw.trim();
}
```

**The per-field validator** (`coin.ts:59-74`) — for char-meta `level`, clone the integer-range check (digits-only `/^\d+$/`, blank handling, `Number.isSafeInteger`) but enforce **1–60** (A2):
```typescript
export function validateCoinField(field: CoinField, raw: CoinRaw): string | undefined {
	const trimmed = rawToTrimmed(raw);
	if (trimmed === '') return undefined;                 // blank = unset (or 0)
	if (!/^\d+$/.test(trimmed)) return /* error copy */;  // no sign/decimal/exponent
	const n = Number(trimmed);
	if (Number.isSafeInteger(n)) return undefined;        // (char-meta: AND 1 <= n <= 60)
	return /* error copy */;
}
```
**char-meta helpers to author** (clone the coin.ts shape): `LEVEL_FIELD` constants; `validateLevel(raw)` (1–60, blank rule per A2); `validateClass`/`validateRace` (membership in the mirrored `CLASSES`/`RACES` lists; a `<select>` already constrains UX but validate for the gate); `charMetaIsValid(inputs)`; `charMetaPayload(inputs)`; `inputsFromChar(char)` (pre-fill — clone `inputsFromToon`, `coin.ts:117-125`); `charMetaChanged(inputs, char)` (the Save gate — clone `coinChanged`, `coin.ts:133-135`).

---

### `web/src/lib/api.ts` (utility — EDIT: interfaces + wrappers)

**Analog:** `api.ts` `fetchBankToons`/`saveCoin` (lines 384-395) + the `postJSON`/`getJSON` cores (lines 154-269). **Do NOT hand-roll fetch** — reuse the cores (credential + typed-error contract).

**The credentialed-fetch contract (reuse, do not reinvent — `api.ts:240-269`):** `postJSON` carries `credentials: 'include'`, `Content-Type: application/json`, the 401→`Unauthenticated`/403→`Forbidden(code)`/other-non-2xx→`ApiError(status, code)` mapping, and the malformed-2xx-JSON guard (WR-04). `getJSON` is the GET twin (lines 154-196). The `{error}` code rides along on EVERY non-2xx so a `400 invalid_input` is branchable.

**The wrappers to clone** (`api.ts:384-395`):
```typescript
/** GET /api/v1/coin/bank-toons → BankToon[] ([] when none). Login-only. */
export function fetchBankToons(fetchFn: typeof fetch = fetch): Promise<BankToon[]> {
	return getJSON<BankToon[]>('/api/v1/coin/bank-toons', fetchFn);
}

/** POST /api/v1/coin → { character, coin }. Login-only (D-12). */
export function saveCoin(
	body: { character_id: number; plat: number; gold: number; silver: number; copper: number },
	fetchFn: typeof fetch = fetch
): Promise<SaveCoinResult> {
	return postJSON<SaveCoinResult>('/api/v1/coin', body, fetchFn);
}
```
**Add for char-meta** (beside these): a `CharMetaItem` interface (mirror `BankToon`, lines 273-281 — `{ character_id, name, class, level, race, is_bank_toon }`); a `SaveCharMetaResult` interface; `fetchCharsForMeta(fetchFn)` → `getJSON<CharMetaItem[]>('/api/v1/char/meta-list', …)`; `saveCharMeta(body, fetchFn)` → `postJSON<SaveCharMetaResult>('/api/v1/char/meta', body, …)`. Keep the `fetchFn` injectable seam (the test hook).

---

### `web/src/lib/components/CharMetaForm.svelte` (component, request-response)

**Analog:** `web/src/lib/components/BankCoinForm.svelte`. **Non-destructive ⇒ NO `ConfirmDialog`** (D-12 precedent; resolves Open Question 2 — default to no confirm even for `is_bank_toon` un-set).

**The flow + auth-handoff to clone** (`BankCoinForm.svelte:40-137`): `getContext<AuthGuard>(AUTH_GUARD_KEY)` → `onMount` fetch the pick-list → `<select>` a character → pre-fill from the chosen char (`onSelect`) → `$derived` field errors + `canSave` (valid AND changed AND !saving) → `onSave` POSTs, reflects the saved values into the local list so the gate re-disables, sets `successMsg` → a mid-session 401 catches `Unauthenticated` and calls `authGuard(err)` (hands off to the whole-site `AuthGate`).

> ⚠️ **CR-01 — the `level` input MUST be `type="text" inputmode="numeric"`, NEVER `type="number"`** (RESEARCH Pitfall 2 / the P15 crash). Clone the input markup verbatim (`BankCoinForm.svelte:161-177`):
```svelte
<!-- CR-01: a text input + numeric keypad, NOT type="number". Svelte 5's bind:value
     on a number-like input coerces the written-back value through to_number()
     (→ number|null), but inputs[f] is typed/used as a string (the helpers call
     .trim()). type="text" + inputmode="numeric" keeps the numeric keypad WITHOUT
     the coercion, so the binding stays a string and the strict /^\d+$/ holds. -->
<input
	id={`...`}
	class="field"
	class:invalid={!!fieldError}
	type="text"
	inputmode="numeric"
	pattern="[0-9]*"
	bind:value={levelInput}
	aria-invalid={fieldError ? 'true' : undefined}
/>
```

**SECURITY — clone two invariants** (`BankCoinForm.svelte:14-17`): (1) the character name is user-controlled, so render it ONLY via plain `{}` (Svelte auto-escapes), NEVER `{@html}` (T-15-28); (2) client validation is UX defense-in-depth only — the SERVER re-validates (T-15-29).

**Field types in the form:** `class` and `race` are `<select>` dropdowns sourced from the mirrored `CLASSES`/`RACES` lists (like the `selectedId` char `<select>`, lines 149-154); `level` is the text+numeric input above; `is_bank_toon` is a checkbox. Wrap each in `FormField` (see Shared Patterns). Use `StateBlock` for loading/error/empty (clone lines 140-145).

---

### `web/src/routes/char-meta/+page.svelte` (route — page)

**Analog:** `web/src/routes/bank-coin/+page.svelte` (a 10-line member-accessible page — NO officer `getContext` check; contrast `/admin/+page.svelte`'s officer refusal).

**Clone verbatim** (`bank-coin/+page.svelte:9-20`), swapping component + title:
```svelte
<script lang="ts">
	import CharMetaForm from '$lib/components/CharMetaForm.svelte';
</script>

<svelte:head>
	<title>SquireBot — set character details</title>
</svelte:head>

<section class="form-card">
	<h1 class="form-title">Set character details</h1>
	<CharMetaForm />
</section>
<!-- + the .form-card / .form-title <style> block (lines 22-40), copied as-is -->
```
Wire the route into the shell nav alongside "Record bank coin" (the same place `/bank-coin` is surfaced).

---

### `web/src/lib/__tests__/charmeta.test.ts` (test) + `internal/backendsrv/webadmin/charmeta_test.go` (test)

> ⚠️ **The web test suite is node-only and BLIND to DOM behavior** (`web-tests-node-only-blind-to-dom` memory; P15 shipped 165 green tests with 2 crashing blockers). Green tests ≠ works in a browser. **Code-review + browser-smoke the form before calling it verified.** Two complementary node-only styles exist — clone BOTH:

**Style A — pure-helper unit tests** (analog: `coin.test.ts`). Drive the `charmeta.ts` helpers directly. **MUST include the CR-01 regression block** (`coin.test.ts:160-199`) that feeds helpers `number`/`null`/`undefined` (the shapes `bind:value` actually writes back) to prove no `.trim()`-on-a-number crash even if the input is ever switched to `type="number"`:
```typescript
it('validate* does NOT throw on a number / null / undefined', () => {
	expect(validateLevel(5 as unknown as string)).toBeUndefined();
	expect(validateLevel(null as unknown as string)).toBeUndefined();
	expect(validateLevel(undefined as unknown as string)).toBeUndefined();
	// ...
});
```

**Style B — `.svelte` source-assertion** (analog: `eviction.test.ts:14-21`). The node suite can't mount the component, so `readFileSync` the form source and assert on the markup string — this is how the repo catches the input-type lesson without a DOM:
```typescript
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
const CHARMETA_FORM_SOURCE = readFileSync(
	fileURLToPath(new URL('../components/CharMetaForm.svelte', import.meta.url)), 'utf8'
);
it('the level input is type="text" inputmode="numeric", never type="number" (CR-01)', () => {
	expect(CHARMETA_FORM_SOURCE).toContain('inputmode="numeric"');
	expect(CHARMETA_FORM_SOURCE).not.toContain('type="number"');
});
```

**Backend test** (analog: `coin_test.go`). Clone `TestCoinSet_NonOfficerCanWrite` (`coin_test.go:81-` + the `seedPlainMember` helper, lines 69-79) — the D-03/D-12 proof that a **plain member (no `guild_admins` row)** can POST char-meta AND the columns actually change (read back, not just the response). Use `store.NewTestDB(t)` and the `withCaller` injection. Add cases: `class`/`race` not in the value set → 400 `invalid_input`; `level` out of 1–60 → 400; a non-existent/removed `character_id` → 400 (the `ErrCharNotFound` path). The route-level gate assertion (`RequireSession` not `RequireOfficer`) goes in `cmd/squirebot-server/main_test.go` (per `coin_test.go`'s header note, lines 8-9).

---

## Shared Patterns

### Login-only authorization gate
**Source:** `cmd/squirebot-server/main.go:328-330` (`webauth.RequireSession`) + `coin.go` package doc (lines 3-11).
**Apply to:** the char-meta route registration AND the handler.
**The rule (D-03, locked):** char-meta is non-sensitive shared data → `RequireSession` (any signed-in member), NEVER `RequireOfficer`. The handler must NOT consult officer status anywhere (no `IsOfficer` call). The acting `discord_user_id` (`caller(ctx)`) is recorded in the audit row for accountability — that is its ONLY use, never an authorization input.

### Atomic write + audit row (the tx idiom)
**Source:** `internal/backendsrv/webadmin/audit.go` — `withTx` (lines 88-107) + `AppendAuditTx` (lines 57-69).
**Apply to:** the char-meta POST handler.
```go
err := withTx(ctx, db, func(tx *sql.Tx) error {
	if e := store.SetCharMetaTx(ctx, tx, req.CharacterID, req.Class, req.Level, req.Race, req.IsBankToon); e != nil {
		return e
	}
	return AppendAuditTx(ctx, tx, "char_meta_set", caller(ctx), map[string]any{"character_id": req.CharacterID}, nowUnix())
})
```
**Don't hand-roll the tx** — `withTx`'s deferred-rollback-on-panic guard (WR-03, keyed on a `committed` flag) is subtle; the store DSN's `_txlock=immediate` makes `BeginTx` issue `BEGIN IMMEDIATE`. `AppendAuditTx` is the ONLY `audit_log` writer in `webadmin` and INSERTs exclusively (append-only, T-15-17). `detail` must marshal to a SMALL JSON blob (never raw bodies/secrets, V7) — use `map[string]any{"character_id": …}`.

### The character pick-list / pre-fill source (reuse as-is)
**Source:** `internal/backendsrv/store/readviews.go:279-310` (`CharsWithMeta`) + the `CharMeta` struct (lines 86-92).
**Apply to:** the char-meta GET handler (pick-list) AND it's already the input `compute.GearCheck`/`SpellCheck` consume.
```go
type CharMeta struct {
	ID    int64
	Name  string
	Class string   // "" when NULL
	Level int64    // 0 when NULL
	Race  string   // "" when NULL
}
// CharsWithMeta: SELECT id, name, class, level, race FROM character WHERE is_removed = 0 ORDER BY name
// (nullable columns resolve to zero-values; does NOT filter on class/level/race — the
//  caller decides). This is EXACTLY the form's pick-list + pre-fill source.
```
**Don't write a new "list all chars" query** — `CharsWithMeta` already returns every non-removed character with `id/name/class/level/race`. (For the GET handler reply, you may need `is_bank_toon` too for pre-fill; if so, extend `CharMeta` + the SELECT by one column at the right edge — extend-only — or add a thin sibling read. The planner picks.)

### Canonical value sets (server-side validation)
**Source:** `internal/backendsrv/enrich/eqconst.go:26-29` (`CLASSES`, Go) + `apps-script/src/lib/eq-constants.ts:44-47` (`RACES`, TS — port to Go).
**Apply to:** the handler's `validCharMeta` AND the form's `<select>` options.
Validate `class ∈ CLASSES` (14) + `race ∈ RACES` (14), store the **abbreviation** (`WAR`/`IKS`), never a display name. Free text or wrong-casing silently breaks the gear/spell joins (Pitfall 5).

### The shared form field (label + control + inline error)
**Source:** `web/src/lib/components/FormField.svelte` (the whole file, 38 lines).
**Apply to:** every field in `CharMetaForm` (the char `<select>`, the class/race `<select>`s, the `level` input, the `is_bank_toon` checkbox).
The caller owns the native `<select>`/`<input>` (passed as the `children` snippet, wires `id`↔`for`); `error` renders via plain `{}` (auto-escaped, T-15-28). Reuse unchanged.

### How the view consumers read the form's output (confirms the form makes views work)
**Source:** `compute/gearcheck.go`, `compute/spellcheck.go`, `compute/bank.go`. **These are CONSUMERS — NO change needed**; they go from BLANK → populated once the form runs (the reason D-02 is required, not optional).
- `gearcheck.go:84-93` — `if c.Class == "" { continue }` (skips classless); `if c.Race == iksarRace { … }` where `iksarRace = "IKS"` (line 35) appends the Iksar tier.
- `spellcheck.go:55` — `if c.Class == "" || c.Level < 1 { continue }` (skips classless or unleveled; a NULL `level`→0→skipped — relevant to the A2 blank-level decision).
- `bank.go:27,37` — scoped to `is_bank_toon = 1` via `InventoryJoin(ctx, true)`; `Coin: nil` with the comment "ADMIN-05 (P15) adds the admin web form that records it" (the bank-coin form, already shipped — char-meta sets the `is_bank_toon` flag that puts a char into this view).

---

## No Analog Found

None for the **code** build — every code file clones a shipped bank-coin sibling.

The **ops/decommission** work (CUTOVER-01/03/04) has no code-clone target by design — it is operational/runbook. Reference-only analogs (do NOT clone as code):

| Work | Reference (style/mechanism only) | Note |
|------|----------------------------------|------|
| Publish v2.0.0 Release (the flip trigger, CUTOVER-03) | `.github/workflows/release.yml` (publishes the bare binary + `latest.json` on a `v*` tag); `internal/update` (`minio/selfupdate` + `IsNewer` + `ManifestURL`) | **No code change** — push a clean `v2.0.0` tag. ⚠️ NOT a `-`-containing prerelease (Pitfall 4: `/releases/latest/` ignores prereleases). The #1 plan task (binary is NOT yet published; latest is v1.0.2). |
| Mint ~12 guild codes (CUTOVER-03) | `cmd/squirebot-server/main.go:79-122` (`runMint`) + `docs/backend-deploy.md` §5 | Existing CLI: `squirebot-server mint-code --owner <label>` (plaintext printed ONCE; SHA-256 stored). Distribute via Discord DM. |
| Disable Apps Script triggers + retire OAuth client (CUTOVER-04) | (external Google consoles) | Maintainer console actions, no repo change. RESEARCH §Decommission Runbook has the cited steps. |
| The decommission checklist (CUTOVER-04 proof artifact, D-13) | `docs/eviction-runbook.md` (checklist STYLE only) + `docs/backend-deploy.md`/`docs/apps-script-deploy.md` | New `docs/decommission-checklist.md`. Markdown prose, not code — fold in the CUTOVER-01 "guildies reporting in" SQL count line (RESEARCH Open Question 3). |

---

## Metadata

**Analog search scope:** `internal/backendsrv/{webadmin,store,enrich,compute}/`, `cmd/squirebot-server/`, `web/src/lib/{,components,__tests__}/`, `web/src/routes/`, `apps-script/src/lib/`, `internal/backendsrv/migrations/`, `docs/`, `.github/workflows/`.
**Files scanned:** ~22 (11 read in full/targeted, the rest existence-confirmed).
**Pattern extraction date:** 2026-05-31
**Note:** RESEARCH.md pre-identified every analog with file:line precision; this map pulls the concrete excerpts the planner copies from. The build is low-risk-by-precedent (a line-by-line clone of code-reviewed, regression-covered work) — the two real risks are the CR-01 `level`-input-type lesson and the ops sequencing (publish the binary BEFORE herding; the triggers are a real teardown step).
