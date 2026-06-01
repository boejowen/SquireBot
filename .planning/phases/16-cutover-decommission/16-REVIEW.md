---
phase: 16-cutover-decommission
reviewed: 2026-05-31T23:40:00Z
depth: standard
scope: "16-01 char-meta build (the only source changes in Phase 16; 16-02/03/04 are ops/docs)"
files_reviewed: 11
files_reviewed_list:
  - internal/backendsrv/webadmin/charmeta.go
  - internal/backendsrv/store/charmeta.go
  - internal/backendsrv/enrich/eqconst.go
  - internal/backendsrv/store/readviews.go
  - cmd/squirebot-server/main.go
  - internal/backendsrv/webadmin/charmeta_test.go
  - web/src/lib/charmeta.ts
  - web/src/lib/components/CharMetaForm.svelte
  - web/src/routes/char-meta/+page.svelte
  - web/src/lib/api.ts
  - web/src/lib/components/SiteShell.svelte
findings:
  critical: 0
  high: 0
  medium: 1
  low: 2
  info: 1
  total: 4
status: issues
---

# Phase 16: Code Review Report (16-01 char-metadata build)

**Reviewed:** 2026-05-31T23:40:00Z
**Depth:** standard (per-file, Go + Svelte/TS, with clone-drift diff against the P15 bank-coin original)
**Files reviewed:** 11 (6 backend incl. one test, 5 frontend; the two `*_test.go`/`*.test.ts` files are context)
**Status:** issues_found

## Summary

16-01 is a faithful, high-quality clone of the already-reviewed P15 bank-coin trio. Every load-bearing invariant the contract called out holds in the source:

- **Login-only (D-03):** both routes are wired under `webauth.RequireSession` (main.go:336-337), never `RequireOfficer`; neither handler nor form consults officer status. The non-officer-write-persists path is proven by `TestCharMetaSet_NonOfficerCanWrite`.
- **Server-side validation:** `validCharMeta` re-checks `class ∈ enrich.CLASSES`, `race ∈ enrich.RACES`, and `level` blank→NULL or 1–60 (charmeta.go:155-166), independent of the form's `<select>`. A blank class/race is rejected server-side (not in the set).
- **Parameterized SQL only:** `SetCharMetaTx` uses `?` placeholders exclusively, `is_removed=0`-scoped, `ErrCharNotFound` fail-closed on `RowsAffected()==0` (charmeta.go:47-58).
- **Audit:** every write emits a `char_meta_set` row in the SAME `withTx` BEGIN IMMEDIATE transaction as the UPDATE (charmeta.go:105-112).
- **CR-01 (DOM-blind):** the level input is `type="text" inputmode="numeric"`, never `type="number"` (CharMetaForm.svelte:186-195); the pure helpers funnel every input through `rawToTrimmed` so a number/null/undefined coercion cannot throw.
- **XSS:** the character name renders via plain `{}` (CharMetaForm.svelte:153, :207); no `{@html}` anywhere; `FormField` renders label/error via `{}` too.
- **Extend-only:** `CharMeta` + `CharsWithMeta` grew `is_bank_toon` at the right edge; the `compute.GearCheck`/`SpellCheck` consumers use field access and are unaffected. The added JSON tags are purely additive (the struct never crossed an API boundary before).

Build + vet + the targeted tests are green (`go build ./...` exit 0; `go vet` clean; CharMeta Go tests pass; 22 web tests pass).

One genuine correctness gap exists that the clone did NOT inherit a guard for, because the bank-coin original never set the flag: the new form is the **first and only** production writer of `is_bank_toon`, and it enforces no single-bank-toon uniqueness — a state the `compute.Bank` view explicitly assumes away (MD-01). Two minor contract/robustness items follow.

## Medium

### MD-01: Char-meta form can flag multiple `is_bank_toon` characters, which the bank view assumes can never happen

**File:** `internal/backendsrv/store/charmeta.go:47-58` (write); `internal/backendsrv/store/readviews.go:142-143` + `internal/backendsrv/compute/bank.go:22-25` (the single-bank-toon assumption)

**Issue:** `SetCharMetaTx` writes `is_bank_toon = ?` with no uniqueness enforcement, and `validCharMeta` does not constrain it (it is a free JSON bool). A grep confirms `SetCharMetaTx` is the ONLY production writer of `is_bank_toon` in the codebase (every other hit is a test helper or a read-side `WHERE is_bank_toon = 1` filter) — so P16's char-meta form is the first reachable path that can set this flag, and a member can set it `true` on any number of characters.

The bank read path is built on a single-bank-toon model: `InventoryJoin(bankOnly=true)` returns rows from ALL `is_bank_toon = 1` characters (`... AND c.is_bank_toon = 1`, ordered by `Char asc`), and `compute.Bank`'s doc states the join is "scoped to the single is_bank_toon character; Char is constant within it" (bank.go:24-25). With two flagged characters that invariant silently breaks: the `bank` view merges rows from multiple characters (Char no longer constant), and the bank-coin form's pick-list (`ListBankToons`) lists several toons with independent coin while the bank view's coin slot (`CoinTotals`, designed around one toon) has no multi-toon story. It does not crash and is user-recoverable (un-check the extra toon), but it is a confusing, unsupported state reachable by a normal member action — a correctness regression introduced by this build, not merely an inherited assumption.

**Fix:** Enforce the single-bank-toon model in the SAME transaction as the write. Demote all other bank toons when one is set (matches v1's `_meta.bank_toon_name` single-value semantics):

```go
func SetCharMetaTx(ctx context.Context, tx *sql.Tx, characterID int64, class string, level *int64, race string, isBankToon bool) error {
	// ... existing bt / levelArg setup ...
	res, err := tx.ExecContext(ctx,
		`UPDATE character SET class = ?, level = ?, race = ?, is_bank_toon = ? WHERE id = ? AND is_removed = 0`,
		class, levelArg, race, bt, characterID)
	if err != nil { /* ... */ }
	n, _ := res.RowsAffected()
	if n == 0 { return ErrCharNotFound }
	// Single-bank-toon invariant (compute.Bank assumes exactly one): if THIS char is
	// now the bank toon, clear the flag on every other character in the same tx.
	if isBankToon {
		if _, err := tx.ExecContext(ctx,
			`UPDATE character SET is_bank_toon = 0 WHERE id != ? AND is_bank_toon = 1`,
			characterID); err != nil {
			return fmt.Errorf("demote prior bank toons (keep=%d): %w", characterID, err)
		}
	}
	return nil
}
```

Alternatively, reject the write with a dedicated `ErrBankToonExists` (→ 409/400) when another `is_bank_toon=1` character exists and the target is not it — but auto-demote is the less surprising UX (the form's checkbox reads as "this character IS the bank"). Either way, add a regression test seeding an existing bank toon, then flagging a second, and asserting exactly one `is_bank_toon=1` row remains.

## Low

### LO-01: GET `meta-list` declares `level: number | null` but the Go struct can never emit `null` (clone-drift from the coin path)

**File:** `internal/backendsrv/store/readviews.go:96` (`Level int64`) vs `web/src/lib/api.ts:307` (`level: number | null`)

**Issue:** `CharMeta.Level` is `int64` (NULL→0 in the scan, readviews.go:310), so a character with an unset level serializes as `"level": 0` on the wire — never `"level": null`. The TS `CharMetaItem.level` is typed `number | null`, implying `null` is reachable when it is not. This is clone-drift: the bank-coin original preserved the null-vs-0 distinction with pointer columns (`BankToon.Plat *int64` → genuine `null`, coin.go:33-40), but the char-meta path collapsed level to `int64`. It is currently benign because 0 is not a valid level and the helpers normalize both: `levelToInput` maps `0|null → ''` (charmeta.ts:142) and `charMetaChanged` maps `c.level === 0 → null` (charmeta.ts:162). But the type contract is misleading and could bite a future consumer that trusts the declared `null`.

**Fix:** Either make the Go side honest (read `level` into `sql.NullInt64` and expose `Level *int64` with a `json:"level"` tag, mirroring `BankToon`'s pointer-coin shape so an unset level is a real JSON `null`) — or tighten the TS type to `level: number` and drop the `| null` from `CharMetaItem`/`SaveCharMetaResult` (api.ts:307,316) since the server provably never sends it. The pointer approach is the more faithful clone of the coin original and removes the 0≡unset coupling.

### LO-02: Success copy reads "Saved details for ." when the post-write read-back fails

**File:** `internal/backendsrv/webadmin/charmeta.go:127-138` + `web/src/lib/components/CharMetaForm.svelte:114`

**Issue:** The POST reply's `character` name comes from a best-effort read-back; on failure it falls back to `""` (charmeta.go:127, "fall back to '' for the name"). The form then renders `successMsg = `Saved details for ${res.character}.`` (CharMetaForm.svelte:114), producing the empty-name string **"Saved details for ."**. The write committed, so the message is not wrong, just ugly. (The bank-coin original has the identical latent shape with "Coin saved for ." — this is inherited, not newly introduced — but it is worth fixing in the clone.)

**Fix:** Guard the interpolation in the form, falling back to a generic noun when the name is blank:

```svelte
successMsg = res.character ? `Saved details for ${res.character}.` : 'Saved character details.';
```

(Or have the handler fall back to the selected character's name it already has in `req` context, though the form is the cleaner fix since it holds `selectedChar.name`.)

## Info

### IN-01: `CharsForMeta` pick-list ordering is case-sensitive (`ORDER BY name`), inconsistent with the bank-toon list (`ORDER BY name COLLATE NOCASE`)

**File:** `internal/backendsrv/store/readviews.go:291` (`ORDER BY name`) vs `internal/backendsrv/store/coin.go:51` (`ORDER BY name COLLATE NOCASE`)

**Issue:** The char-meta pick-list reuses the pre-existing `CharsWithMeta` query, which orders by `name` case-sensitively, so a lowercase-initial name would sort after all uppercase names (ASCII ordering). The sibling bank-toon list uses `COLLATE NOCASE`. EQ/P99 character names are conventionally capitalized, so this is cosmetic and unlikely to surface — but the two member-facing pick-lists sorting differently is a minor inconsistency. Not introduced by P16 (`CharsWithMeta` predates this phase); flagged only because P16 newly surfaces this query in a user-facing dropdown.

**Fix (optional):** Add `COLLATE NOCASE` to the `CharsWithMeta` `ORDER BY` to match the bank-toon list. Verify the existing `compute` consumers (which re-key by name) are order-insensitive first — they are, so the change is safe.

---

_Reviewed: 2026-05-31T23:40:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
