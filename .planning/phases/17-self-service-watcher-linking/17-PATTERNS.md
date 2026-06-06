# Phase 17: Self-Service Watcher Linking - Pattern Map

**Mapped:** 2026-06-01
**Files analyzed:** 13 (8 backend, 5 frontend; mix of NEW + CHANGE)
**Analogs found:** 13 / 13 (every file has a same-repo analog — this is a composition phase)

> Read order for the planner: this file maps each new/modified P17 file to the closest existing analog, with the exact lines to copy. Pair it with 17-RESEARCH.md (signatures + the two landmines) and 17-CONTEXT.md (locked D-01..D-10). All line numbers below are from files read this session (2026-06-01).

---

## File Classification

| New/Modified File | New/Chg | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|---------|------|-----------|----------------|---------------|
| `internal/backendsrv/migrations/00005_self_service_linking.sql` | NEW | migration | schema (DDL) | `migrations/00004_web_auth.sql` (extend-only ALTER + index) | exact (role+flow) |
| `internal/backendsrv/store/linking.go` | NEW | store | CRUD + transform | `store/binding.go` (owner upsert/bind in `*sql.Tx`) + `store/admins.go` (fail-closed Tx read) | exact |
| `internal/backendsrv/auth/mint.go` (add `MintCodeForOwnerTx`) | CHANGE | service | request-response (token gen) | `auth/mint.go` `MintCode` (same file — adapt to Tx + ownerID) | exact |
| `internal/backendsrv/auth/guard.go` (thread `codeID`) | CHANGE | middleware | request-response (auth) | `auth/guard.go` `resolveToken` (same file — widen return) | exact |
| `internal/backendsrv/webadmin/account.go` (Mint/List/Revoke handlers) | NEW | controller | request-response (CRUD) | `webadmin/officers.go` (`OfficerAddHandler` skeleton) | exact |
| `internal/backendsrv/webadmin/eviction.go` (`callerMayNotEvictFloor` FK rewire) | CHANGE | controller | request-response | `webadmin/eviction.go` (same func — prefer FK) | exact |
| `internal/backendsrv/ingest/handler.go` (stamp `last_seen`) | CHANGE | controller | request-response (ingest) | `ingest/handler.go` `bindAndReplace` post-commit (same file) | exact |
| `cmd/squirebot-server/main.go` (3 routes; DELETE `runMint`/`mint-code`) | CHANGE | route + config | request-response | `main.go` route block (lines 319-337) + `runMint`/`runRevoke` (97-163) | exact |
| `web/src/routes/account/+page.svelte` | NEW | route (page) | request-response | `web/src/routes/char-meta/+page.svelte` (member page + `.form-card`) | exact |
| `web/src/lib/components/WatcherCodesPanel.svelte` | NEW | component | request-response (CRUD UI) | `web/src/lib/components/EvictionForm.svelte` (load → confirm → commit + state) | exact |
| `web/src/lib/api.ts` (mint/list/revoke wrappers + types) | CHANGE | utility | request-response | `web/src/lib/api.ts` `fetchMeta`/`postJSON` wrappers (same file) | exact |
| `web/src/lib/components/SiteShell.svelte` (Account nav) | CHANGE | component | — | `SiteShell.svelte` `.char-meta-nav` block (lines 54-62) | exact |
| `cmd/squirebot-server/main_test.go` (DELETE mint-dispatch tests) | CHANGE | test | — | existing `TestRun_MintDispatch{,_MissingOwner}` (delete) | exact |

**No "No Analog Found" section** — every file maps to an in-repo pattern.

---

## Pattern Assignments

### `migrations/00005_self_service_linking.sql` (NEW — migration, DDL)

**Analog:** `migrations/00004_web_auth.sql` (extend-only ALTER + index + `+goose Down`).

**LANDMINE (RESEARCH Pitfall 1):** SQLite cannot `ADD COLUMN ... UNIQUE`. Use `ADD COLUMN` (no UNIQUE) + a separate partial `CREATE UNIQUE INDEX ... WHERE ... IS NOT NULL`.

**Column-type convention (copy from `00001_init.sql`, NOT 00004):** `guild_code` is a 00001 table whose timestamp columns are **TEXT `datetime('now')`** — `created_at TEXT NOT NULL DEFAULT (datetime('now'))` (00001:55), `disabled_at TEXT` (00001:54). So `guild_code.last_seen` must be **`TEXT`** (stamp with `datetime('now')`), NOT the epoch INTEGER the 00004 web columns use. FK target `web_user(discord_user_id)` is `TEXT PRIMARY KEY` (00004:12), so `owner.discord_user_id` is `TEXT` — types match.

**Goose Up/Down shape to copy** (00004:1-6 header style, 00004:51-59 ALTER style, 00004:71-86 Down style):
```sql
-- +goose Up
-- Phase 17 plan 17-0X. owner.discord_user_id FK→web_user (LINK-01/02, D-01) +
-- guild_code.last_seen (D-07). Forward-only; 00001-00004 shipped, NOT edited.
ALTER TABLE owner ADD COLUMN discord_user_id TEXT REFERENCES web_user(discord_user_id);
CREATE UNIQUE INDEX owner_discord_user_id_uidx
  ON owner(discord_user_id) WHERE discord_user_id IS NOT NULL;  -- one owner per Discord id; many NULLs ok
ALTER TABLE guild_code ADD COLUMN last_seen TEXT;               -- TEXT/datetime('now') — matches 00001 guild_code cols

-- +goose Down
DROP INDEX owner_discord_user_id_uidx;
-- (column drops best-effort; forward-only in practice — mirror 00004:86 comment)
```
**FK caveat (RESEARCH:203):** the DSN sets `_pragma=foreign_keys(ON)` per connection, so the FK is enforced at runtime; existing `owner` rows get NULL (NULL is FK-exempt). Add a `migrate_test.go` `table_info`/`index_list` assertion mirroring 00004's test convention.

---

### `store/linking.go` (NEW — store, CRUD + the resolve-or-create algorithm)

**Analogs:** `store/binding.go` (owner-bind in a `*sql.Tx`, exported delegates to a lowercase impl, audit-on-the-tx) + `store/admins.go` (fail-closed `*Tx` reads, typed sentinel errors with v1-matching strings).

**Funcs to add:** `ResolveOrCreateOwnerByDiscordTx`, `ListOwnCodes`, `RevokeOwnCodeTx`, `StampCodeLastSeen` (+ a new `ErrAmbiguousOwner` sentinel).

**Sentinel-error + exported-wrapper convention** (copy from binding.go:27-42 / admins.go:34-42):
```go
// binding.go:30
var ErrCharOwnedByAnother = errors.New("character owned by another owner")
// admins.go:37 — typed, fail-closed, string matches the HTTP error code
var ErrNotAuthorized = errors.New("not_authorized")
```
→ add `var ErrAmbiguousOwner = errors.New("ambiguous_owner")` (the D-04 refuse-and-log case; the handler maps it to a 409/422 + `slog.Warn`).

**The resolve-or-create core (D-03/D-04)** — model on `bindCharacter` (binding.go:55-84): a single indexed SELECT, a `switch` on `sql.ErrNoRows`, parameterized `?` only, `slog.Warn` + audit on the refuse path (NEVER the token, V7). The existing label bridge to reuse verbatim is the eviction.go one:
```go
// eviction.go:371-372 — the TRIM + COLLATE NOCASE bridge to reuse
`SELECT id FROM owner WHERE TRIM(label) = TRIM(?) COLLATE NOCASE`
```
Algorithm (run ALL inside the ONE mint `withTx` `BEGIN IMMEDIATE` tx — RESEARCH Pattern 2):
1. `SELECT id FROM owner WHERE discord_user_id = ?(caller)` → 1 row ⇒ return it (D-03 subsequent codes).
2. resolve caller username (see `usernameOf` officers.go:218 — `SELECT username FROM web_user WHERE discord_user_id = ?`).
3. `SELECT id FROM owner WHERE TRIM(label)=TRIM(?username) COLLATE NOCASE AND discord_user_id IS NULL` → exactly 1 ⇒ `UPDATE owner SET discord_user_id=? WHERE id=?` (D-04 adopt); 0 ⇒ `INSERT owner(label, discord_user_id)` (D-04 new); 2+ ⇒ `ErrAmbiguousOwner`.
4. ALSO refuse if a label match is already stamped with a *different* `discord_user_id` (mis-adoption guard).

**Race note (RESEARCH:141):** composing resolve+mint in ONE `withTx` is what makes it atomic; the partial unique index is the backstop (a racing double-stamp surfaces as a constraint error, not corruption). The store is `SetMaxOpenConns(1)` + `_txlock=immediate` so writes already serialize.

**`ListOwnCodes` query** (caller-scoped, stable `#N` ordering — RESEARCH Code Examples; mirror the SELECT shape in admins.go):
```sql
SELECT id, created_at, last_seen
  FROM guild_code
 WHERE owner_id = ? AND disabled_at IS NULL
 ORDER BY created_at ASC, id ASC
```
`#N` = handler-assigned 1-based index over the active set (not a stored column — RESEARCH A3).

**`RevokeOwnCodeTx` — caller-scoped (RESEARCH Pitfall 3, the security-critical one):** MUST scope to the caller's owner; do NOT reuse `auth.RevokeCode` (the ops CLI is intentionally owner-unscoped).
```sql
UPDATE guild_code SET disabled_at = datetime('now')
 WHERE id = ? AND owner_id = ? AND disabled_at IS NULL
```
`RowsAffected = 0` ⇒ not theirs / already revoked ⇒ idempotent no-op (never leak existence).

**`StampCodeLastSeen`** — plain `UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?` (best-effort; see ingest below).

---

### `auth/mint.go` — add `MintCodeForOwnerTx` (CHANGE — service, token gen)

**Analog:** `MintCode` in the SAME file (mint.go:30-54). **Do NOT change `MintCode`'s signature** — `RestoreHandler` (eviction.go:292) still calls `auth.MintCode(db, ownerLabel)` on a bare `*sql.DB` and relies on its stdout print (WR-01/WR-02). Add a sibling (RESEARCH Open Question 1: "Add the sibling").

**Copy the token-gen shape verbatim** (mint.go:31-49) — 32B `crypto/rand` → `base64.RawURLEncoding` → `sha256`, hash-only INSERT, parameterized `?`:
```go
raw := make([]byte, 32)
rand.Read(raw)                                  // crypto/rand, NOT math/rand (V6)
code := base64.RawURLEncoding.EncodeToString(raw) // plaintext, shown ONCE
sum := sha256.Sum256([]byte(code))
// INSERT INTO guild_code (owner_id, token_hash, label) VALUES (?, ?, ?)  — sum[:], never the plaintext
```
**The two deltas for the Tx variant:**
1. Signature `MintCodeForOwnerTx(ctx context.Context, tx *sql.Tx, ownerID int64) (string, error)` — takes a resolved `ownerID` (D-02: NO free-text owner), runs on the caller's `*sql.Tx` (composes into the handler's `withTx`).
2. **NO `fmt.Printf` to stdout** (mint.go:52 — that is the CLI's one-time disclosure). The web variant *returns* the plaintext for the HTTP body ONLY; it is **never** logged (V7). `label` for self-minted codes: store the resolved owner label or NULL (RESEARCH Open Question 2 — either safe; NULL cleanest).

---

### `auth/guard.go` — thread `codeID` out (CHANGE — middleware; the most invasive task)

**Analog:** `resolveToken`/`ResolveToken` in the SAME file (guard.go:41, 69-102). **LANDMINE (RESEARCH Pitfall 2):** today it `SELECT owner_id, token_hash` and returns `(ownerID int64, ok bool)` — no code id, so `last_seen` can't be stamped.

**Change:** widen the SELECT to `SELECT id, owner_id, token_hash FROM guild_code WHERE disabled_at IS NULL` (guard.go:78) and return `(ownerID, codeID int64, ok bool)` from BOTH `resolveToken` (guard.go:69) and the exported `ResolveToken` (guard.go:41). **Keep** the `subtle.ConstantTimeCompare` (guard.go:91) and the never-log-the-token discipline (guard.go:64) unchanged. Update the two call sites: ingest `handler.go:94` and `ingest/whoami.go`. Treat as a discrete task with its own `guard_test.go` updates (RESEARCH flags this).

---

### `webadmin/account.go` (NEW — controller; Mint/List/Revoke handlers)

**Analog:** `webadmin/officers.go` `OfficerAddHandler` (officers.go:108-150). This is the canonical skeleton — copy it exactly. (RESEARCH A5: adding to `webadmin` is consistent — it already mixes login-only and officer-only handlers; or a new `webaccount` package, planner's call.)

**The `caller→withTx→*Tx→AppendAuditTx→writeJSON` skeleton** (officers.go:108-149, helpers at officers.go:37-61, audit.go:88-107):
```go
func MintOwnCodeHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        ctx := r.Context()
        callerID := caller(ctx)   // officers.go:58 = webauth.UserFromContext (D-02 — owner from session, NOT the body)
        now := nowUnix()          // officers.go:52
        var plaintext string
        err := withTx(ctx, db, func(tx *sql.Tx) error {            // audit.go:88 — BEGIN IMMEDIATE + deferred rollback (WR-03)
            ownerID, derr := store.ResolveOrCreateOwnerByDiscordTx(ctx, tx, callerID)
            if derr != nil { return derr }                          // ErrAmbiguousOwner ⇒ refuse (D-04)
            var merr error
            plaintext, merr = auth.MintCodeForOwnerTx(ctx, tx, ownerID)
            if merr != nil { return merr }
            return AppendAuditTx(ctx, tx, "code_mint", callerID, map[string]any{"owner_id": ownerID}, now) // audit.go:57 — detail carries owner_id/code_id ONLY, never the token (V7)
        })
        if err != nil { mapAccountErr(w, err); return }
        writeJSON(w, map[string]any{"code": plaintext})            // officers.go:44 — plaintext in the body EXACTLY ONCE
    }
}
```
**Shared helpers to REUSE (do not re-author):** `caller` (officers.go:58), `nowUnix` (officers.go:52), `writeJSON`/`writeJSONError` (officers.go:43-48), `withTx` (audit.go:88), `AppendAuditTx` (audit.go:57). **Error mapping:** add a `mapAccountErr` mirroring `mapOfficerErr` (officers.go:203-213) — map `ErrAmbiguousOwner`→409/422, default→500. The frontend routes off the `{"error":"code"}` shape.

**List handler** mirrors `OfficersListHandler` (officers.go:68-97): method-check, GET store call, **non-nil empty slice** so JSON is `[]` not `null` (officers.go:88-94 — the empty-state keys off `[]`), assign `#N` ordinals over the active set. **Revoke handler** mirrors `OfficerRemoveHandler` (officers.go:156-196): decode `{id}` body, `withTx` → `store.RevokeOwnCodeTx(ctx, tx, id, callerOwnerID)` → audit `code_revoke` only if `revoked` (officers.go:182-187 idempotent-no-audit pattern).

---

### `webadmin/eviction.go` — `callerMayNotEvictFloor` FK rewire (CHANGE — D-05)

**Analog:** the SAME function (eviction.go:333-386). Today it resolves the floor's protected owner purely via the label bridge (eviction.go:371-372) and is documented fail-OPEN (WR-05, eviction.go:344-366).

**Change (D-05):** prefer `owner.discord_user_id` when present. Add a SELECT-by-FK first: `SELECT id FROM owner WHERE discord_user_id = ?(floor)` — if found, that IS the protected owner (closes WR-05 for linked owners). Fall back to the existing `TRIM(label) COLLATE NOCASE` bridge (eviction.go:371-372) only when the floor's owner is not-yet-linked. Keep the `slog.Warn` inert-protection warnings (eviction.go:363, 378) for the still-unlinked fallback path. This is the direct LINK-02 payoff.

---

### `ingest/handler.go` — stamp `last_seen` post-commit (CHANGE — D-07)

**Analog:** `ServeHTTP` + `bindAndReplace` in the SAME file (handler.go:81-158, 188-242).

**Pattern 3 (RESEARCH) — stamp OUTSIDE the replace tx, best-effort:** the bearer guard now returns `codeID` (see guard.go change). After `bindAndReplace` returns the 204 (handler.go:139), fire a separate non-blocking `UPDATE` that NEVER fails the ingest:
```go
ownerID, codeID, ok := h.guard.ResolveToken(r.Context(), r.Header.Get("Authorization")) // handler.go:94 — now 3 returns
// ... existing [2]..[4] flow unchanged ...
status, err := h.bindAndReplace(r, ownerID, env, rows)   // handler.go:139
if err != nil { /* existing 409/500 mapping, handler.go:140-154 */ return }
if _, serr := h.db.ExecContext(r.Context(),
    `UPDATE guild_code SET last_seen = datetime('now') WHERE id = ?`, codeID); serr != nil {
    slog.Warn("stamp last_seen failed (non-fatal)", "code_id", codeID, "err", serr) // NEVER the token (V7)
}
w.WriteHeader(status)                                    // handler.go:157
```
**Why outside the tx (RESEARCH:145):** the replace tx (handler.go:188 `bindAndReplace`) is the load-bearing atomic snapshot; folding an auth-bookkeeping write into it couples advisory metadata to data integrity. `last_seen` is advisory UI — a failed stamp logs and is dropped. No coalescing needed at ~12-guildie volume.

---

### `cmd/squirebot-server/main.go` — 3 routes + DELETE `runMint` (CHANGE — LINK-06)

**Analog:** the existing route block (main.go:319-337) and the CLI dispatch (main.go:77-126).

**Add 3 login-only routes** mirroring the char-meta login-only block (main.go:336-337 — `RequireSession`, NOT `RequireOfficer`):
```go
// main.go:336 is the precedent: webauth.RequireSession(db, webadmin.CharMetaSetHandler(db))
mux.Handle("POST /api/v1/account/codes",        webauth.RequireSession(db, webadmin.MintOwnCodeHandler(db)))
mux.Handle("GET  /api/v1/account/codes",        webauth.RequireSession(db, webadmin.ListOwnCodesHandler(db)))
mux.Handle("POST /api/v1/account/codes/revoke", webauth.RequireSession(db, webadmin.RevokeOwnCodeHandler(db)))
```
(Note: a single `mux.Handle` per method+path — the mux pattern carries the verb, e.g. main.go:329-330.)

**DELETE (LINK-06):** the `case "mint-code": return runMint(args[1:])` arm (main.go:79-80), the `runMint` func (main.go:97-126), and its `main_test.go` tests `TestRun_MintDispatch{,_MissingOwner}`. **KEEP:** `runRevoke` + the `case "revoke-code"` arm (main.go:81-82, 136-163) as the ops backstop, and the shared `splitFlagsAndPositionals` helper (used by `runRevoke`).

---

### `web/src/routes/account/+page.svelte` (NEW — route, member page)

**Analog:** `web/src/routes/char-meta/+page.svelte` (the entire file, 1-41). `/account` is the login-only sibling — same shell, no officer check.

**Copy the `.form-card` page shell verbatim** (char-meta/+page.svelte:18-41): `<section class="form-card">` (`max-width:720px`, `padding:24px`, `--panel`, `--border`, `6px` radius, `display:flex; flex-direction:column; gap:24px`) + the `.form-title` style (20px `--font-display`, `--weight-display`). Swap the title to **"Your watcher codes"** (17-UI-SPEC Copywriting) and render `<WatcherCodesPanel />` in place of `<CharMetaForm />`. No `+page.ts` (inherits SPA fallback, char-meta:9). AuthGate-gated at the layout (no page-level gate).

---

### `web/src/lib/components/WatcherCodesPanel.svelte` (NEW — component)

**Analog:** `web/src/lib/components/EvictionForm.svelte` (1-130+). This is the richest pattern source: Svelte 5 runes, `getContext` auth-guard, the load→confirm→commit lifecycle, optimistic-collapse, and `ConfirmDialog` reuse.

**Copy these patterns:**
- **Imports + context** (EvictionForm.svelte:19-38): `import { onMount, getContext } from 'svelte'`, lucide icon imports, `StateBlock`/`ConfirmDialog`, `AUTH_GUARD_KEY`/`AuthGuard` from `AuthGate.svelte`, api wrappers from `$lib/api`; `const authGuard = getContext<AuthGuard>(AUTH_GUARD_KEY)`.
- **Runes state shape** (EvictionForm.svelte:43-66): `let phase = $state<'loading'|'error'|'ready'>('loading')`, list `$state`, `dialogOpen`/`-ing` flags, `successMsg`/`errorMsg`. For P17: `codes`, `minting`, `revokeTarget`, `revokeDialogOpen`, `revoking`, plus a transient `mintedPlaintext` (the show-once panel state).
- **`load()` on mount** (EvictionForm.svelte:79-94): `phase='loading'` → `await fetchOwnCodes()` → `phase='ready'`; `if (route(err)) return` (the 401→AuthGate bubble) else `phase='error'`.
- **Confirm-before-commit** (EvictionForm.svelte:113-130): `openConfirm()` sets `dialogOpen=true`; `doRevoke()` closes the dialog, calls `revokeOwnCode(id)`, then **optimistic-collapse** the row (`codes = codes.filter(...)` — EvictionForm:128) + a `--status-ok` success line; on failure keep the row + inline `--status-missing` error.
- **Trust boundary** (EvictionForm.svelte:15-17): every interpolation via plain `{}` (auto-escape) — NEVER `{@html}`. The plaintext code, `#N`, dates all render via `{}`.

**GENUINELY NEW (no in-repo analog) — the show-once copy-to-clipboard:** `Grep` confirms no existing `navigator.clipboard`/`writeText` pattern in `web/src`. Build per 17-UI-SPEC § Show-Once Panel: `navigator.clipboard.writeText(code)` → swap to `check`+"Copied!" (`--status-ok`) for ~2s; graceful fallback = the token stays `user-select:all` so manual select-copy works if the Clipboard API is denied. **Plaintext lives ONLY in component state for the panel's lifetime — never `localStorage`, never re-fetched, never logged** (RESEARCH Pitfall 4 / LINK-04). **Browser-smoke this** — `web/` vitest is node-only and blind to the DOM/clipboard (RESEARCH Pitfall 5 / MEMORY web-tests-node-only); the P15 precedent shipped 2 crashing BLOCKERs behind 165 green tests.

---

### `web/src/lib/api.ts` — mint/list/revoke wrappers (CHANGE — utility)

**Analog:** the public-wrapper section of the SAME file (api.ts:198-223 GET wrappers, the `postJSON` core at api.ts:240). Copy the one-liner wrapper convention (`fetchMeta`, api.ts:221). `getJSON`/`postJSON` already carry `credentials:'include'` (api.ts:163, 247) + the `Unauthenticated`/`Forbidden` typed-error subclasses (api.ts:46, 54) the `AuthGate` catches — reuse them, do not re-author fetch.
```ts
export interface OwnCode { id: number; ordinal: number; created_at: string; last_seen: string | null; }
export function fetchOwnCodes(f: typeof fetch = fetch): Promise<OwnCode[]> {
    return getJSON<OwnCode[]>('/api/v1/account/codes', f);           // 401 → Unauthenticated → AuthGate
}
export function mintOwnCode(f: typeof fetch = fetch): Promise<{ code: string }> {
    return postJSON<{ code: string }>('/api/v1/account/codes', {}, f); // {} body — owner is session-derived (D-02)
}
export function revokeOwnCode(id: number, f: typeof fetch = fetch): Promise<{ revoked: boolean }> {
    return postJSON<{ revoked: boolean }>('/api/v1/account/codes/revoke', { id }, f);
}
```
The `classifyAdminError` helper (api.ts:517) exists if the panel needs to branch the mint/revoke error code.

---

### `web/src/lib/components/SiteShell.svelte` — Account nav (CHANGE)

**Analog:** the `.char-meta-nav` block in the SAME file (SiteShell.svelte:54-62). Add the **Account** link **inside the `{#if session?.authenticated}` block** (member-visible), NOT the `{#if session?.isOfficer}` Admin block (SiteShell.svelte:47-52). Reuse `.char-meta-nav` styling verbatim (D-09 / 17-UI-SPEC):
```svelte
{#if session?.authenticated}
    <a href="/account" class="char-meta-nav">Account</a>          <!-- NEW — member-visible, plain <a>, no officer marker -->
    <a href="/char-meta" class="char-meta-nav">Character details</a>  <!-- existing -->
    <SessionIndicator {session} />
{/if}
```

---

## Shared Patterns

### Authenticated-write tx + audit (apply to: all 3 new `account.go` handlers)
**Source:** `webadmin/audit.go` — `withTx` (audit.go:88-107, BEGIN IMMEDIATE + deferred-rollback panic-safety WR-03) + `AppendAuditTx` (audit.go:57-69, `INSERT INTO audit_log (at, event, detail, actor)`). Compose the store `*Tx` mutator + the audit row in ONE tx so write+trail are atomic. Detail blob carries `owner_id`/`code_id` ONLY — never the token (V7).

### Caller identity from the session (apply to: every account handler — D-02)
**Source:** `caller(ctx)` (officers.go:58) = `webauth.UserFromContext` (session.go:112). The gate (`RequireSession`) is the ONLY legitimate identity setter (session.go:124 warns against forging). The request body NEVER carries an owner.

### Login-only gate, not officer (apply to: the 3 new routes — D-09)
**Source:** `webauth.RequireSession(db, handler)` (session.go:11-13; precedent main.go:336-337). `/account` is `RequireSession`, NEVER `RequireOfficer`. The hidden nav is UX only; the server gate is the boundary.

### Hash-only / shown-once / never-log the plaintext (apply to: mint.go new fn, account.go mint handler, the Svelte panel — V6/V7 / LINK-04)
**Source:** `auth/mint.go` token-gen (mint.go:31-49) + the ingest never-log-token discipline (handler.go:29-30, 93). Plaintext crosses to the page in the HTTP body EXACTLY once; never slog, never the audit detail, never `localStorage`.

### Parameterized SQL only (apply to: linking.go, the migration, every query — V5)
**Source:** every store file uses `?` placeholders exclusively (binding.go:58, admins.go:70, eviction.go:372). No interpolation of user input into SQL; the resolve-or-create reuses `TRIM(label) COLLATE NOCASE` (eviction.go:372) as a parameterized bridge.

### Confirm-before-commit + optimistic-collapse (apply to: WatcherCodesPanel revoke)
**Source:** `EvictionForm.svelte` (openConfirm:113-116 → ConfirmDialog → doEvict:118-130, list-filter at :128) + the shared `ConfirmDialog` (focus-trap, Esc/backdrop dismiss, destructive styling — reused as-is). Default-focus Cancel (17-UI-SPEC).

### Credentialed fetch + typed errors (apply to: api.ts wrappers, the panel)
**Source:** `web/src/lib/api.ts` `getJSON`/`postJSON` (`credentials:'include'`, api.ts:163/247) + `Unauthenticated`/`Forbidden` (api.ts:46/54) → `AuthGate` re-route. Reuse; never raw `fetch`.

---

## No Analog Found

| Concern | Role | Reason | Planner guidance |
|---------|------|--------|------------------|
| Copy-to-clipboard + show-once reveal (inside `WatcherCodesPanel`) | component (sub-pattern) | `Grep` found NO existing `navigator.clipboard`/`writeText`/`user-select` copy pattern in `web/src`. This is the one genuinely-new frontend surface. | Build per 17-UI-SPEC § Show-Once Panel; `user-select:all` fallback; browser-smoke (vitest is DOM-blind — RESEARCH Pitfall 5). |

Everything else maps to an in-repo analog (this is a composition phase).

---

## Metadata

**Analog search scope:** `internal/backendsrv/{auth,webadmin,store,ingest,webauth,migrations}`, `cmd/squirebot-server`, `web/src/{routes,lib,lib/components}`.
**Files read this session:** officers.go, mint.go, guard.go, eviction.go, audit.go, 00001_init.sql, 00004_web_auth.sql, binding.go, admins.go (1-120), ingest/handler.go, session.go (1-140), main.go (route + CLI sections), char-meta/+page.svelte, EvictionForm.svelte (1-130), api.ts (150-260 + grep), SiteShell.svelte (40-80) + the 3 phase docs (CONTEXT/RESEARCH/UI-SPEC).
**Pattern extraction date:** 2026-06-01
