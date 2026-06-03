---
phase: 19
reviewers: [claude-fresh-session]
review_mode: same-model-independent-context
reviewed_at: 2026-06-03
plans_reviewed: [19-01-PLAN.md, 19-02-PLAN.md, 19-03-PLAN.md]
note: >
  No independent EXTERNAL AI CLI was installed on this machine (only Claude-self
  and the Cursor IDE GUI launcher — no cursor-agent CLI). Per user choice, the
  review was run as a FRESH, context-free `claude -p` session: same model family,
  but independent context (no prior conversation, re-derived everything and
  verified load-bearing claims against the live codebase). This is weaker than a
  true cross-MODEL review — treat the findings as a strong second pass, not as
  multi-model consensus. To get genuine cross-AI coverage, install `gemini` and/or
  `codex` and re-run `/gsd-review 19`.
---

# Cross-AI Plan Review — Phase 19 (Wantlist CRUD)

## Claude Review (fresh `claude -p` session — independent context)

I verified the plans' load-bearing claims against the actual SquireBot codebase (migrations, `store/`, `webadmin/`, `web/src/lib/api.ts`). Findings below distinguish what's confirmed-solid from what's genuinely at risk.

---

### Cross-Set Summary

These are well-constructed composition plans: nearly every new file has a real, verified in-repo twin (`account.go`, `linking.go`, `readviews.go`, `views.go`, `00005_*.sql`, the `/account/codes` api.ts block), and I confirmed the cited helpers actually exist (`caller` officers.go:58, `AppendAuditTx` audit.go:57, `withTx` audit.go:88, `ListOwnCodes` linking.go:162, `RevokeOwnCodeTx` linking.go:198, `mapAccountErr` account.go:211). The security spine (owner-from-session, owner-scoped soft-delete no-op, parameterized SQL, audited mutations, RequireSession gating) is sound and faithfully cloned. The wave ordering (01 store → 02 handlers → 03 UI) is strictly sequential with matching interface contracts — no parallelism hazard.

The plans are **not** blocker-free, though. There is one genuine correctness defect in the phase's signature feature that the node-blind test suite will actively green-light, one under-specified 409 mechanism that contradicts its own cited analog, and a semantic mismatch between the "in bank" contract and what the join actually computes. Details per plan.

---

### Plan 19-01 — Store / Migration

#### Strengths
- **The soft-delete ↔ partial-unique-index re-add interaction is correct.** Both partial indexes are scoped `WHERE … active = 1`, so a removed row (active=0) is a tombstone excluded from the index; re-adding the same `(user,item,reason)` inserts cleanly. This is the trickiest edge in the phase and the plan gets it right, with a `grep -c "active = 1" … >= 2` gate to enforce it.
- **FK target is valid and enforced.** `web_user.discord_user_id` is `TEXT PRIMARY KEY` (00004_web_auth.sql:12), so `REFERENCES web_user(discord_user_id)` is legal, and `foreign_keys(ON)` is set in the DSN (`store/db.go:43`) — so `ON DELETE CASCADE` will actually fire (a common SQLite footgun, avoided here).
- NULL-distinct semantics for custom vs catalog dedupe are correctly handled with two separate partial indexes.
- `pigparse_price` columns (`item_id INTEGER PRIMARY KEY, name TEXT, current_avg REAL`) confirmed (00001_init.sql:60); D-10's "use pigparse_price not item_master" is correctly grounded.

#### Concerns
- **[MEDIUM] `pigparse_name_idx` is essentially dead weight for the actual query.** The search is `name LIKE '%…%'` (leading wildcard) plus `CAST(item_id AS TEXT) = ?`. A leading-`%` LIKE and a `CAST(...)` both **defeat B-tree index usage** — SQLite will full-scan `pigparse_price` on every search regardless of the index. The migration creates an index the query can't use, and the threat model (T-19-04) cites it as a DoS mitigation. At ~thousands of rows behind RequireSession for a 12-person guild this is harmless in practice, but the claim is false and the index is misleading. Either drop it or switch to FTS5 if substring perf ever matters.
- **[LOW] `CatalogItem.Name string` vs nullable `pigparse_price.name`.** The column has no `NOT NULL` (00001:60). The substring branch excludes NULL names (NULL LIKE → not-true), but the `OR CAST(item_id AS TEXT) = ?` branch **can** return a NULL-name row, and scanning NULL into a `string` will error → the handler returns 500, contradicting the "empty corpus → graceful []" promise for an id-exact search. Scan `name` via `sql.NullString` defensively, or add `WHERE name IS NOT NULL`.
- **[LOW] `COLLATE NOCASE` on the `LIKE` term is a no-op.** SQLite `LIKE` is already ASCII-case-insensitive by default and ignores column collation; the real case-insensitivity comes from `LIKE` itself, not `COLLATE NOCASE`. The COLLATE only matters on the `ORDER BY name` and the `=` comparison. Harmless for ASCII EQ item names, but the plan's stated rationale ("COLLATE NOCASE = the case-insensitive idiom") is misattributed and could mislead a future maintainer who removes the wrong piece.
- **[LOW] No `CHECK` constraints on the `reason`/`priority` enums.** Enum validity rests entirely on handler code. Phase 20 (or any direct write) could insert `reason='maybe'`. `CHECK (reason IN ('buy','quest'))` / `CHECK (priority IN ('low','med','high'))` is free defense-in-depth on a fresh `CREATE TABLE`.
- **[LOW] `alert_log` full Phase-20 shape is a forward guess.** Creating it now is mandated by success criterion #4, so it's in-contract — but the column set (`source`, `detail`, `send_status`, the dedup index) is speculative and may need a 00007 migration when Phase 20 actually lands. Acceptable (extend-only), worth stating explicitly so nobody treats it as frozen.

#### Suggestions
- Drop `pigparse_name_idx` (or replace with FTS5) and correct T-19-04 to say the real bound is "full-scan of N≈few-thousand rows + LIMIT, acceptable at guild scale."
- Scan `name` as `NullString`; add the enum `CHECK`s; trim `note` before the 280-rune check (280 spaces currently passes).

---

### Plan 19-02 — HTTP Handlers / Routes

#### Strengths
- IDOR discipline is exemplary: owner from `caller(ctx)`, grep-gated to zero `owner_id`/`req.Owner`, cross-owner remove proven as a `{removed:false}` no-op via a real `NewTestDB`-backed test. This correctly mirrors v2.1 D-02.
- Rune-aware note validation (`utf8.RuneCountInString`), audit detail carrying ids-only (no note/PII), and `len(q) < 2` short-circuit before any DB hit are all correct and gated.
- Handler tests use a real DB, so the 409 path is exercised end-to-end rather than mocked.

#### Concerns
- **[MEDIUM] The duplicate→409 detection mechanism is under-specified and contradicts its cited analog.** `mapAccountErr` (account.go:211) maps via `errors.Is(err, store.ErrAmbiguousOwner)` — a **typed sentinel**. But Plan 01 instructs `AddWantTx` to "return the unique-index conflict **raw-wrapped**," and Plan 02 says "clone `mapAccountErr` to map a UNIQUE-constraint violation → 409." A raw-wrapped `modernc.org/sqlite` error will **not** match any `errors.Is` sentinel — so a verbatim clone of the analog returns 500, not 409. The two instructions are mutually inconsistent. The likely executor "fix" is brittle string-matching on `"UNIQUE constraint failed"`, which couples the handler to driver wording. **Resolve explicitly:** have the store detect the violation (modernc exposes `*sqlite.Error` with `.Code()` == 2067 `SQLITE_CONSTRAINT_UNIQUE`) and return a typed `store.ErrDuplicateWant`, then the handler's `errors.Is` works exactly like the analog. The TDD duplicate test will catch the broken path, but at the cost of a debugging cycle the plan could pre-empt.
- **[LOW/MEDIUM] Server trusts client-supplied `item_name` for catalog wants.** `validWant` only requires a non-blank label when `item_id == nil`. A catalog want with `item_id=1001` and `item_name="anything"` (or blank) is accepted and stored as the "snapshot." The server never re-derives the canonical name from `pigparse_price` by `item_id`. The in-bank join keys on `item_id` (so it isn't broken), and `{}` escaping prevents XSS — this is purely an integrity/trust smell, not a vuln. Consider looking up the catalog name server-side for `item_id`-bearing wants instead of trusting the body.

#### Suggestions
- Specify the 409 path concretely: `var ErrDuplicateWant = errors.New("wantlist: duplicate")`; in `AddWantTx`, detect the constraint via the driver error code and return `ErrDuplicateWant`; in `mapWantErr`, `errors.Is(err, store.ErrDuplicateWant) → 409`. Remove the "return it raw-wrapped" wording.
- Add a handler test asserting the duplicate response is **exactly 409 with `{"error":"duplicate"}`**, and that the *other reason* still returns 200 — both against the real DB.

---

### Plan 19-03 — SvelteKit UI

#### Strengths
- The mandatory `checkpoint:human-verify` browser-smoke (Task 3, blocking) directly honors the documented P15 node-blind lesson, with a concrete 9-point script including XSS, debounce-in-Network-tab, and cross-owner checks.
- XSS boundary is gated to zero `{@html}` on user data; server-truth reload (never optimistic-mutate) keeps the grid authoritative.
- DOM-free logic (`holdersFor`, `priorityRank`) is correctly isolated for node testing.

#### Concerns
- **[HIGH] `holdersFor` double-counts characters that hold an item in more than one row — and the node test will pass anyway.** The `view` data is built from raw `inventory_item` rows (one per location/stack — confirmed in `InventoryJoin`, readviews.go:127-138: `FROM inventory_item ii … WHERE ii.item_id > 0`, no GROUP BY). A character very commonly holds the same `item_id` in multiple rows (worn + bank, or several stacks of bone chips/words/etc.). The spec'd `holdersFor` (Plan 01 behavior + RESEARCH 502-514) **maps** matching rows to `{char, count}` and sorts by char — it does **not** group-by-char and sum. Result: the deep display renders `↳ Borticus: 1` / `↳ Borticus: 1` (two lines, same char) instead of the UI-SPEC's `↳ Borticus: 2`. This is a visible defect in the phase's signature core-value surface, and because the node test mirrors the same map-not-reduce spec, the suite will be **green while the display is wrong** — the exact P15 trap the plan claims to defend against. **Fix:** `holdersFor` must reduce by char (sum counts), then sort. Add a node test with two view rows for the same `(char, item_id)` asserting a single summed line.
- **[MEDIUM] "In bank" badge vs the all-inventory join — semantic mismatch with success criterion #3.** Success criterion #3 and the badge copy say "already in the **guild bank**?", but `fetchView()` hits `/api/v1/views/view` (api.ts:201) = the **all-character, all-location** consolidated inventory (worn + carried + bank), not bank-toon-only (`bankOnly=false`). So an item only *worn* by one guildie, never banked, will display "In bank." D-06 deliberately chose all-inventory ("where in the guild is it"), which is arguably the better behavior — but then the **word "In bank" overstates it** and should read "In guild" / "Held by." Nobody reconciled the inherited D-06 wording against criterion #3's "bank" wording. Pick one: relabel the badge, or scope the join to `bank`.
- **[LOW] Holder grouping is O(wants × viewRows) per render** over the full guild inventory pulled client-side. Fine at this scale (tens of wants × low-thousands of rows), but pre-build a `Map<item_id, {char→count}>` from `viewRows` once on load rather than re-filtering per row if the grid re-renders on sort/filter.

#### Suggestions
- Make `holdersFor` reduce-by-char; cover the multi-row-same-char case in `holders.test.ts` (it currently only asserts the null-itemId case).
- Reconcile the "In bank" copy with the actual join semantics before the browser-smoke, so the human verifier isn't validating a mislabeled cell.

---

### Focused Answers to the Review Prompt
- **Soft-delete + partial-index dedupe / re-add:** Correct. Tombstones (active=0) drop out of the `active=1` partial index; re-add works. ✓
- **NULL item_id semantics:** Handled via two partial indexes; custom path excluded from join. ✓ (One gap: catalog `item_name` is client-trusted — integrity, not correctness.)
- **D-10 SQL injection / ESCAPE / DoS:** Injection is properly bound (`?` only, grep-gated); `ESCAPE '\'` + `escapeLike` is correct. **But** the `COLLATE NOCASE`-on-LIKE rationale is wrong (no-op), and the `pigparse_name_idx`/DoS-mitigation claim is false (leading-wildcard LIKE can't use it). Real DoS risk is nil at this scale, so the *mitigation story* is wrong but the *outcome* is safe.
- **IDOR / owner-scoping:** Correct and well-tested.
- **In-bank join scaling / key:** `item_id` is the right key (stable; join confirmed same id-space via `pigparse_price.item_id`/`inventory_item.item_id`). Scaling is fine. **The group-by-char defect (HIGH) is the real issue** — many characters/rows per item are mishandled.
- **Scope creep:** Minimal. `alert_log` is mandated by criterion #4, not creep. The search ranking (prefix + id-exact) is justified by D-10's full-catalog scope.
- **Achieves WANT-01/02:** Yes, modulo the holder-summing fix and the "In bank" wording.
- **What the node-blind tests hide:** The `holdersFor` summing bug (tests assert the buggy spec), the entire debounce/disabled-button/ConfirmDialog/chip rendering surface, and the 409-vs-500 mapping if it regresses at the wire (Go handler tests cover the latter — good).

---

### Overall Risk Assessment: **MEDIUM**

The foundation (security shape, migration, FK/cascade, dedupe/re-add, parameterized SQL, wave ordering) is verified-solid and faithfully cloned — low structural risk. Risk is concentrated in three correctable items: one **HIGH** correctness defect in the core-value holder display that the node suite will not catch (`holdersFor` must reduce-by-char), one **MEDIUM** spec/mechanism ambiguity (409 detection contradicts the cited `errors.Is` analog), and one **MEDIUM** "In bank" label-vs-join mismatch against success criterion #3. None are architectural; all are point fixes that should be pinned in the plans before execution rather than discovered in the browser-smoke.

---

## Consensus Summary

> Single reviewer (fresh-session Claude). Not multi-model consensus — see the `note` in the frontmatter. The items below are this reviewer's prioritized findings.

### Must-fix before execution (pin into the plans)
1. **[HIGH] `holdersFor` must group-by-char and SUM counts**, not map one line per inventory row (Plan 19-03). A character holding an item in multiple stacks/locations would render duplicate `↳ Char: 1` lines instead of `↳ Char: N`. The node test asserts the same buggy spec, so it will be green while the signature in-bank display is wrong — the exact P15 node-blind trap. Add a multi-row-same-char test.
2. **[MEDIUM] Pin the duplicate→409 mechanism** (Plans 19-01 + 19-02). "Return the unique-index error raw-wrapped" + "clone `mapAccountErr`" are contradictory: the cited analog uses `errors.Is` on a typed sentinel, which a raw driver error won't match → 500 not 409. Have the store detect `SQLITE_CONSTRAINT_UNIQUE` (modernc code 2067) and return a typed `ErrDuplicateWant`; map that in the handler.
3. **[MEDIUM] Reconcile "In bank" wording vs the all-inventory join** (Plan 19-03 / success criterion #3). `fetchView()` is all-character/all-location, so "In bank" overstates a worn-only item. Either relabel ("In guild" / "Held by") or scope the join to `bank`.

### Worth fixing (cheap, defense-in-depth)
- Scan `pigparse_price.name` as `sql.NullString` (id-exact branch can hit a NULL name → 500) — Plan 19-01.
- Add `CHECK` constraints on `reason`/`priority` enums in the migration — Plan 19-01.
- Trim `note` before the 280-rune check (280 spaces currently passes) — Plan 19-02.
- Drop or correct the `pigparse_name_idx` DoS claim — a leading-`%` LIKE can't use it (harmless at guild scale, but the threat-model mitigation story is false) — Plan 19-01.
- Consider server-side re-deriving the catalog name from `item_id` instead of trusting the client `item_name` snapshot — Plan 19-02.

### Confirmed-solid (no action)
- Owner-from-session IDOR discipline + cross-owner no-op (real-DB tested).
- Soft-delete ↔ partial-unique-index re-add interaction.
- FK target + `ON DELETE CASCADE` actually firing (foreign_keys ON).
- Parameterized SQL + `ESCAPE '\'` (no injection).
- Wave ordering 01→02→03 with matching interface contracts.
- Mandatory blocking browser-smoke honoring the P15 lesson.

### Divergent views
- None (single reviewer). Re-run with `gemini`/`codex` installed for true cross-model divergence.
