---
phase: 15-admin-web-forms-login
reviewed: 2026-05-31T03:25:10Z
depth: standard
files_reviewed: 37
files_reviewed_list:
  - cmd/squirebot-server/main.go
  - cmd/squirebot-server/ownerfloor.go
  - internal/backendsrv/migrations/00004_web_auth.sql
  - internal/backendsrv/readapi/cors.go
  - internal/backendsrv/scheduler/scheduler.go
  - internal/backendsrv/store/admins.go
  - internal/backendsrv/store/coin.go
  - internal/backendsrv/store/eviction.go
  - internal/backendsrv/store/websession.go
  - internal/backendsrv/webadmin/audit.go
  - internal/backendsrv/webadmin/coin.go
  - internal/backendsrv/webadmin/eviction.go
  - internal/backendsrv/webadmin/officers.go
  - internal/backendsrv/webauth/handlers.go
  - internal/backendsrv/webauth/oauth.go
  - internal/backendsrv/webauth/session.go
  - web/src/app.css
  - web/src/lib/admin.ts
  - web/src/lib/api.ts
  - web/src/lib/auth.ts
  - web/src/lib/coin.ts
  - web/src/lib/components/AdminMgmtForm.svelte
  - web/src/lib/components/AuthGate.svelte
  - web/src/lib/components/BankCoinForm.svelte
  - web/src/lib/components/ConfirmDialog.svelte
  - web/src/lib/components/EvictionForm.svelte
  - web/src/lib/components/FormField.svelte
  - web/src/lib/components/LoginScreen.svelte
  - web/src/lib/components/NotMemberScreen.svelte
  - web/src/lib/components/SessionIndicator.svelte
  - web/src/lib/components/SiteShell.svelte
  - web/src/lib/components/StateBlock.svelte
  - web/src/routes/+layout.svelte
  - web/src/routes/+page.svelte
  - web/src/routes/admin/+page.svelte
  - web/src/routes/bank-coin/+page.svelte
findings:
  critical: 2
  warning: 7
  info: 5
  total: 14
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-05-31T03:25:10Z
**Depth:** standard
**Files Reviewed:** 37
**Status:** issues_found

## Summary

Phase 15 is SquireBot's first authenticated + destructive surface: Discord OAuth2 login, opaque server-side sessions, and the eviction / coin / officer-management write forms. **The security spine is solid.** I traced every property called out in the focus block and the load-bearing ones hold:

- **OAuth/CSRF**: state minted from `crypto/rand`, stored in a short-lived httpOnly+Secure+Lax cookie, verified (present AND equal) before any code exchange; code exchange is server-side; the client secret never reaches the bundle. (`oauth.go`, `handlers.go`)
- **Open-redirect (W-4)**: the callback's final `Location` is built only from the `webOrigin` server constant — never from a request param. Confirmed both redirect targets (`webOrigin+"/?not_member=1"`, `webOrigin+"/"`) and that only `state`/`code` are read from the URL.
- **Membership gate (AUTH-08)**: `IsGuildMember` is fail-closed (empty configured id or empty list → false); a session is minted ONLY for a confirmed member.
- **Sessions**: opaque id stored as SHA-256 only; cookie is httpOnly+Secure+Lax+Domain; regenerate-on-login (fresh id every callback, never adopts a caller id); rolling expiry via `TouchSession`; fail-closed resolve.
- **CORS**: exact-origin echo (never wildcard) + `Allow-Credentials: true` + `Vary: Origin`.
- **Authorization**: officer checks re-verified INSIDE the write tx (`IsOfficerTx` / `AddOfficerTx` / `RemoveOfficerTx` first-statement re-check) on a `BEGIN IMMEDIATE` tx — the v1 WR-04 TOCTOU close is real. Owner-floor protection is checked before any write. Bank-coin is correctly login-only (D-12), not a bug.
- **SQL**: every statement in the changed files uses `?` placeholders. No interpolation. No injection.
- **Frontend XSS**: the reviewed P15 components render all user/Discord-controlled strings via plain `{}` (auto-escaped); no `{@html}` on user data.
- **P14 hardening**: P15 correctly wraps the previously-ungated read routes in `RequireSession` (verified against the `abb92a0^` diff) — a genuine improvement.

**However, two CRITICAL correctness bugs ship in the frontend write forms**, both invisible to the (deliberately node-only, DOM-free) test suite:

1. **The bank-coin form crashes on the first keystroke** — Svelte 5 coerces `<input type="number">` bindings to `number`/`null`, but every coin helper calls `.trim()` assuming a string → `TypeError`.
2. **The eviction grace date renders as "Jan 21 1970"** — the backend sends `grace_until` as epoch *seconds* (a JSON number), the frontend feeds it to `new Date()` which expects *milliseconds*, and the TS interface lies (`string`).

Neither is caught by the green build/typecheck/tests because the tests pass string literals straight to the pure helpers and never exercise the DOM binding or the live JSON shape. These are exactly the "security/correctness property that looks tested but isn't" gaps. Both must be fixed before this ships to the ~12 guildies.

---

## Critical Issues

### CR-01: Bank-coin form throws `TypeError` on first keystroke (number-input binding feeds a number to string helpers)

**File:** `web/src/lib/components/BankCoinForm.svelte:159-169` (binding) + `web/src/lib/coin.ts:29-66` (helpers)

**Issue:** The four coin inputs are `<input type="number" bind:value={inputs[f]}>`. In Svelte 5 (this repo: `svelte@5.56.0`), `bind:value` on a number-like input coerces the written-back value through `to_number()` — verified in `node_modules/svelte/src/internal/client/dom/elements/bindings/input.js:31` (`value = is_numberlike_input(input) ? to_number(value) : value`) and `to_number` (line 288: `return value === '' ? null : +value`). So the instant the user types into any field, `inputs[f]` — declared `CoinInputs` = `Record<CoinField, string>` — actually holds a `number` (or `null` when emptied).

Every coin helper assumes a string and calls `.trim()`:

```ts
// coin.ts:30 (validateCoinField), :64 (coinValue)
const trimmed = raw.trim();   // raw is a number at runtime → TypeError: raw.trim is not a function
```

`fieldErrors = $derived(validateCoin(inputs))` re-runs on every keystroke, so the first digit typed throws inside the reactive derivation and breaks the form. `coinValue`/`coinPayload`/`coinChanged` would throw on save too. The `canSave` derived also calls `coinIsValid` → throws.

This is invisible to `coin.test.ts`: every assertion passes a **string literal** (`'5'`, `''`, `'1000'`) directly to the helpers, bypassing the DOM binding entirely. The node-only suite (no `@testing-library/svelte`, no jsdom — a deliberate repo decision) structurally cannot catch a binding-coercion bug. `svelte-check`/`tsc` also miss it: the runtime value diverges from the declared `string` type, and the type system trusts the declaration.

**Fix:** Either keep the field a string (drop `type="number"`, use `inputmode="numeric"` only, which does NOT trigger coercion), or make the helpers number-tolerant. The string-input route preserves the existing strict digits-only validation (`/^\d+$/`):

```svelte
<!-- BankCoinForm.svelte: text input + numeric keypad, no Svelte number coercion -->
<input
  id={`coin-${f}`}
  class="field coin-input"
  class:invalid={!!fieldErrors[f]}
  type="text"
  inputmode="numeric"
  pattern="[0-9]*"
  bind:value={inputs[f]}
  aria-invalid={fieldErrors[f] ? 'true' : undefined}
/>
```

Then add a regression test that drives the helpers with the value types the binding actually produces (number and `null`), e.g. `validateCoinField('plat', 5 as unknown as string)`, to lock the contract. If `type="number"` is kept instead, change `CoinInputs` to `Record<CoinField, number | null>` and rewrite the helpers to stop calling `.trim()`.

---

### CR-02: Eviction grace date renders as "Wed Jan 21 1970" (epoch-seconds value passed to `new Date()` which expects milliseconds)

**File:** `web/src/lib/components/EvictionForm.svelte:62-65, 109, 193, 223` + contract mismatch in `web/src/lib/api.ts:338-348`

**Issue:** The backend emits `grace_until` as **unix epoch seconds** (a JSON number):

```go
// webadmin/eviction.go:91 (preview) and store/eviction.go:120 (evict)
graceUntil := nowUnix() + store.EvictionGraceSeconds   // e.g. 1782789805 (seconds)
writeJSON(w, map[string]any{... "grace_until": graceUntil})
```

The frontend's `graceDate` feeds that straight to `new Date()`:

```ts
// EvictionForm.svelte:62
function graceDate(iso: string): string {
  const d = new Date(iso);                       // new Date(1782789805) ⇒ 1.78B ms ⇒ Jan 21 1970
  return Number.isNaN(d.getTime()) ? iso : d.toDateString();
}
```

`new Date(<number>)` interprets the argument as **milliseconds since epoch**. An epoch-*seconds* value (~1.78e9) is ~20 days after 1970-01-01, so the preview line, the success toast, and the confirm-dialog body all show "Grace expires: **Wed Jan 21 1970**" instead of "30 days from today". Verified empirically:

```
backend sends (epoch seconds): 1782789805
graceDate(number) => Wed Jan 21 1970
```

Compounding it, the TS interfaces declare the field as `string` (`EvictionPreview.grace_until: string`, `EvictResult.grace_until: string`, `api.ts:342,348`) while the backend sends a number — so the static types are wrong AND `graceDate`'s `iso: string` param is mistyped. JSON-over-HTTP doesn't enforce the declared type, so the compiler never flags the divergence. The static-narrative tests don't exercise the real JSON shape, so they pass.

This is user-facing and on a **destructive** surface: an officer about to evict a guildie is shown a nonsense grace deadline, undermining the confirm-before-commit safety the dialog is supposed to provide.

**Fix:** Make the contract honest (number of epoch seconds) and convert seconds→ms before constructing the Date:

```ts
// api.ts
export interface EvictionPreview { owner_id: number; characters: string[]; grace_until: number; }
export interface EvictResult { removed_count: number; grace_until: number; }

// EvictionForm.svelte
function graceDate(epochSeconds: number): string {
  const d = new Date(epochSeconds * 1000);
  return Number.isNaN(d.getTime()) ? String(epochSeconds) : d.toDateString();
}
```

Add a unit test asserting `graceDate(<epoch seconds for a known date>)` yields that date, not 1970.

---

## Warnings

### WR-01: Restore can leave a guildie restored-but-codeless on a post-commit failure (no compensating action, no audit)

**File:** `internal/backendsrv/webadmin/eviction.go:217-242`

**Issue:** `RestoreHandler` commits the restore tx (un-set `is_removed`, clear `grace_until`, write the `eviction_restore` audit row) and only AFTER the commit calls `ownerLabelOf` + `auth.MintCode` to issue a fresh guild code (the comment at :222-229 acknowledges this ordering is deliberate because `MintCode` manages its own connection). If either `ownerLabelOf` or `MintCode` fails, the handler returns 500 — but the restore has already committed. Net state: the owner's characters are live again (`is_removed=0`) yet they have **no active guild code** (the old one stays `disabled_at`-revoked from the eviction). Their watcher gets 401s and silently stops uploading, with no UI signal — the officer saw a 500 and assumes nothing happened. The audit log records `eviction_restore` but not the mint failure, so the trail implies a complete restore.

**Fix:** Treat the post-commit mint as recoverable and surface it explicitly rather than as a generic 500: log it at `error` with `owner_id`, and return a distinct success-with-warning shape (e.g. `{restored_count, new_code_issued: false, code_mint_failed: true}`) so the frontend can tell the officer "restored, but re-issue a code via the CLI." Alternatively, resolve the owner label BEFORE the tx (it cannot change during the restore) so the only post-commit fallible step is the mint itself, narrowing the window. Document the operator recovery path (`squirebot-server mint-code --owner <label>`).

### WR-02: Restore's re-minted guild code is unreachable from the web — printed only to server stdout

**File:** `internal/backendsrv/webadmin/eviction.go:236` (calls `auth.MintCode`, which `fmt.Printf`s the plaintext at `internal/backendsrv/auth/mint.go:52`)

**Issue:** When an officer restores an evicted guildie via the web form, the handler re-mints a guild code by calling `auth.MintCode`. `MintCode` writes the one-time plaintext code to the **server process's stdout** (journald), never to the HTTP response (correctly — V7 says don't echo it). The handler returns `{restored_count, new_code_issued: true}`. So the web officer is told a code was issued but has no way to obtain it — they must SSH to the VPS and grep journald, or run the mint CLI again. For a ~12-person guild where the maintainer is the operator this is survivable, but it means the web "restore" flow is functionally incomplete: a non-maintainer officer who restores someone cannot deliver them a working code. This is a design gap, not a leak.

**Fix:** Decide and document the contract. Either (a) explicitly scope web-restore to "the maintainer must hand off the new code out-of-band, watch journald" and reflect that in the success copy, or (b) since restore necessarily issues a NEW code (the old one is permanently revoked), reconsider whether web-initiated restore should mint at all vs. tell the officer to run the CLI. At minimum the success message should not imply the officer now has a deliverable code.

### WR-03: `withTx` does not roll back on a panic inside `fn` (connection/tx leak)

**File:** `internal/backendsrv/webadmin/audit.go:68-81`

**Issue:** `withTx` rolls back only when `fn(tx)` returns a non-nil error:

```go
if ferr := fn(tx); ferr != nil { _ = tx.Rollback(); return ferr }
if cerr := tx.Commit(); cerr != nil { ... }
```

If `fn` (which calls store mutators + `AppendAuditTx`, including `json.Marshal` of arbitrary `detail`) **panics**, neither `Rollback` nor `Commit` runs. With `SetMaxOpenConns(1)` (store/db.go:61), the single pooled connection is left holding an open `BEGIN IMMEDIATE` transaction. Until the panic unwinds and the GC finalizer eventually closes the orphaned `*sql.Tx`, the server's one writer connection is wedged — every subsequent write (and read, since maxconns=1) blocks on `busy_timeout` and then errors. A single panic in any webadmin handler can stall the whole backend's write path.

**Fix:** Add a deferred rollback guarded by a commit flag (the idiomatic Go pattern):

```go
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil { return fmt.Errorf("begin webadmin tx: %w", err) }
    committed := false
    defer func() { if !committed { _ = tx.Rollback() } }()
    if ferr := fn(tx); ferr != nil { return ferr }
    if cerr := tx.Commit(); cerr != nil { return fmt.Errorf("commit webadmin tx: %w", cerr) }
    committed = true
    return nil
}
```

`database/sql` makes a Rollback after a successful Commit a harmless no-op, so the guard flag is the clean form.

### WR-04: `parseOwnerIDQuery` has no overflow guard — a long digit string silently wraps to a wrong (possibly negative) id

**File:** `internal/backendsrv/webadmin/eviction.go:318-335`

**Issue:** The manual parser accumulates `id = id*10 + int64(c-'0')` with no length cap and no overflow check. A query like `?owner_id=99999999999999999999` overflows `int64` and wraps — potentially to a negative or to a small positive that collides with a real owner id. The final `if id <= 0` guard catches the negative-wrap case (returns invalid), but a value that wraps to a *positive* in-range id would be accepted and passed to `PreviewEviction`/`EvictOwnerTx` as a legitimate owner id. The blast radius is bounded (it only previews/evicts whatever owner that wrapped id happens to hit, and the preview is read-only while the evict still goes through the officer gate + floor protection), but it is an unvalidated-input correctness defect on a destructive endpoint.

**Fix:** Use `strconv.ParseInt(raw, 10, 64)` and reject on error (it returns `ErrRange` on overflow), then keep the `> 0` check:

```go
id, err := strconv.ParseInt(raw, 10, 64)
if err != nil || id <= 0 { return 0, false }
return id, true
```

### WR-05: Eviction owner-floor protection silently degrades to "not protected" on a label/username mismatch

**File:** `internal/backendsrv/webadmin/eviction.go:266-303`

**Issue:** `callerMayNotEvictFloor` resolves the floor's protected owner by matching `owner.label == floor web_user's username` (the documented "best-available textual bridge" — there is no FK linking owners to Discord ids). If the floor's Discord `username` does not exactly string-match any `owner.label` (Discord usernames change; `owner.label` is a watcher-supplied handle; case/whitespace differences; the floor never logged in so `username` is the snowflake placeholder per `SetOwnerFloor`), the function returns `(false, nil)` — i.e. **not protected** — and a peer officer CAN evict the maintainer's data. This fails *open* for the floor-protection check specifically (contrast the rest of the auth surface, which fails closed). The owner-floor data protection (D-09) is the one guarantee that the maintainer's own guildie data can't be nuked by a peer, and it rests entirely on a fragile textual coincidence.

**Fix:** This is partly a schema gap (no owner↔discord link), so a full fix may be out of phase scope — but the risk must be recorded and the matching hardened: normalize both sides (trim + `COLLATE NOCASE`, which the query may already get from the column but `floorUsername` should be normalized too) and add a loud `slog.Warn` when a floor is seeded but no matching owner label is found, so the operator knows the protection is inert. Longer term, add an explicit `app_config['owner_floor_owner_id']` (or a column on `owner`) seeded by `set-owner-floor` so the protection keys on an id, not a mutable display name.

### WR-06: `whoami-web` re-resolves and rolls the session via a side-effecting GET (rolling-expiry write on a read endpoint)

**File:** `internal/backendsrv/webauth/handlers.go:211` → `session.go:142-164` (`resolveSessionUser` calls `TouchSession`)

**Issue:** `WhoamiWebHandler` is documented as "the side-effect-free validation endpoint / the always-200 AuthGate feed," and the frontend `fetchSession` calls it on every mount/refresh. But it routes through `resolveSessionUser`, which performs a `TouchSession` UPDATE (bumping `expires_at`) on every call. So a GET that the frontend treats as a pure read mutates the DB. Functionally this is mostly benign (it just extends the rolling window, which is the intent for any authenticated hit), but: (a) it means a page reload silently re-arms a 30-day session even when the user took no action, slightly weakening the "departed guildie's session lapses" property the rolling window is meant to provide; and (b) a side-effecting whoami violates the documented "side-effect-free" contract and the GET-is-safe convention. With `maxconns=1`, it's also an extra serialized write on every gate resolution.

**Fix:** If whoami is meant to be read-only, give it a non-touching resolve path (a `resolveSessionUserReadOnly` that skips `TouchSession`) and let the rolling-window bump happen only on the *gated* API hits (`RequireSession`/`RequireOfficer`), which already call `TouchSession`. If the touch-on-whoami is actually desired, update the handler doc comment to stop claiming "side-effect-free."

### WR-07: `lock_busy` is handled by the frontend but never produced by the backend (dead error path / misleading contract)

**File:** `web/src/lib/api.ts:455-456` + `web/src/lib/admin.ts:46` (handle `lock_busy`); no backend emitter

**Issue:** The frontend's `classifyAdminError` routes a `lock_busy` 403 code to the inline retry copy (`ADMIN_ERROR_COPY['lock-busy']`), and the audit.go doc comment lists `lock_busy` as a v1 error code. But no Go handler ever returns `lock_busy`: the store uses `busy_timeout(5000)` (waits rather than erroring) and `SetMaxOpenConns(1)` (serializes writes), so SQLITE_BUSY is engineered away. The frontend defends against a response the backend cannot send. Harmless at runtime, but it's dead handling that implies a contention surface that doesn't exist, and it can mislead the next maintainer into thinking concurrent-write conflicts are a live concern here.

**Fix:** Either drop the `lock_busy` branch from `classifyAdminError`/`ADMIN_ERROR_COPY` and the audit.go comment, or — if you want defense-in-depth for a future where `busy_timeout` is lowered — add a code comment on both sides noting it's intentionally unreachable today.

---

## Info

### IN-01: 00004 Down migration uses `ALTER TABLE ... DROP COLUMN`, which is fragile across SQLite/goose

**File:** `internal/backendsrv/migrations/00004_web_auth.sql:71-86`

**Issue:** The Down migration `DROP COLUMN`s the seven added columns. SQLite only gained `DROP COLUMN` in 3.35 (2021) and it has restrictions (can't drop a column referenced by an index/FK/generated column). The file's own footer says "Down is best-effort; forward-only in practice," and goose is forward-only here, so this is low-risk — but a Down run on an older bundled SQLite or against a column that later acquires an index would fail mid-rollback. Since rollback is never used in production, this is informational.

**Fix:** None required given the forward-only posture; if you ever need a real Down, switch to the 12-step table-rebuild pattern. Leaving the note so it's a conscious decision.

### IN-02: `EvictionPreview.grace_until` / `EvictResult.grace_until` typed `string` but sent as number

**File:** `web/src/lib/api.ts:342, 348`

**Issue:** Captured under CR-02 as the root cause of the 1970 date bug, but flagging the type-contract drift separately: these interfaces (and several adjacent ones consumed only for display) declare epoch fields as `string` when the backend sends numbers. Even after CR-02's date fix, audit the other timestamp-ish fields for the same `string`-vs-number drift so the TS contract matches the pinned JSON.

**Fix:** Align all `grace_until`/timestamp fields to `number` (epoch seconds) and add a brief comment that they are epoch seconds, mirroring the Go side.

### IN-03: `ConfirmDialog` heading id uses `Math.random()` (non-deterministic, weak uniqueness)

**File:** `web/src/lib/components/ConfirmDialog.svelte:87`

**Issue:** `headingId = \`confirm-dialog-heading-${Math.random().toString(36).slice(2)}\``. `Math.random()` is fine for a DOM id (no security need), but it's non-deterministic (mildly annoying for snapshot/a11y tests) and has a tiny collision chance if multiple dialogs mount. Svelte 5 ships `$props.id()` / there's `crypto.randomUUID()` for a cleaner unique id.

**Fix:** Use a stable per-instance id source (`crypto.randomUUID()` or Svelte's id helper). Cosmetic.

### IN-04: `BankCoinForm` recomputes `coinPayload(inputs)` twice in the save path

**File:** `web/src/lib/components/BankCoinForm.svelte:102-105`

**Issue:** `onSave` calls `coinPayload(inputs)` inline in the `saveCoin({ character_id, ...coinPayload(inputs) })` argument (line 102) and again into `const payload` on line 105 for the optimistic local update. Minor duplication (and double work). Harmless, but compute once and reuse. (Note: this code path is also implicated by CR-01 — once the binding is fixed to strings, both calls are correct; until then both throw.)

**Fix:** `const payload = coinPayload(inputs);` then `await saveCoin({ character_id: selectedToon.character_id, ...payload });`.

### IN-05: CORS sets `Vary: Origin` with `Set` (overwrites) rather than `Add` (appends)

**File:** `internal/backendsrv/readapi/cors.go:50`

**Issue:** `w.Header().Set("Vary", "Origin")` replaces any existing `Vary` value. Today nothing upstream in the Go handler chain sets `Vary`, so this is correct and harmless. Flagging only because if a future handler (or a compression/middleware layer) adds its own `Vary` (e.g. `Accept-Encoding`), `Set` here would clobber it. Using `Add` is the defensive idiom for `Vary`.

**Fix:** `w.Header().Add("Vary", "Origin")` to be future-proof. Cosmetic given the current chain.

---

_Reviewed: 2026-05-31T03:25:10Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
