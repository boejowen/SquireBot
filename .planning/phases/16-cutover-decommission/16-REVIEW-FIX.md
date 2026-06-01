---
phase: 16-cutover-decommission
fixed_at: 2026-06-01T00:09:00Z
review_path: .planning/phases/16-cutover-decommission/16-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 4
status: all_fixed
---

# Phase 16: Code Review Fix Report

**Fixed at:** 2026-06-01
**Source review:** `.planning/phases/16-cutover-decommission/16-REVIEW.md`
**Iteration:** 1

**Scope note:** The review uses a 5-tier severity scale (critical / high / medium /
low / info). With `fix_scope: critical_warning`, in-scope = Critical + High + Medium
+ Low; Info is out-of-scope. The report has 0 Critical, 0 High, 1 Medium (MD-01),
2 Low (LR-01 fixable, LR-02 verify-only), 3 Info (IN-01/02/03). So **2 findings were
actionable** (MD-01, LR-01); LR-02 is explicitly "no fix required" and the three Info
findings were explicitly "no change required / leave as-is for parity."

**Summary:**
- Findings in scope: 2 (MD-01, LR-01 — the actionable Medium + Low)
- Fixed: 2
- Skipped: 4 (LR-02 verify-only by design; IN-01/02/03 out-of-scope Info, no-fix-recommended)

**Verification:** Each fix was verified with `go build ./...` (clean), `go vet`
(clean), and `go test ./internal/backendsrv/... ./cmd/squirebot-server/...` (all
suites `ok`) before its commit. The targeted MD-01 regression and the LR-01 route-gate
subtests were also run individually (`-v`) and confirmed green. Nothing was pushed.

## Fixed Issues

### MD-01: char-meta could silently create a second `is_bank_toon=1`, breaking the bank view's single-toon invariant

**Files modified:** `internal/backendsrv/store/charmeta.go`,
`internal/backendsrv/compute/bank.go`,
`internal/backendsrv/store/charmeta_test.go` (new)
**Commit:** `0e31023`
**Applied fix:** Enforced the single-bank-toon invariant at the store seam — the
recommended demote-prior-bank-toon approach. Inside `SetCharMetaTx`, when
`isBankToon` is true, a parameterized `UPDATE character SET is_bank_toon = 0 WHERE
is_bank_toon = 1 AND id <> ? AND is_removed = 0` runs in the same `tx` immediately
before the per-character set, so the demote + set + the handler's `char_meta_set`
audit row commit atomically (every committed state has at most one live bank toon).
Setting the flag to false performs no demote (it only clears the character's own
flag, which can't violate the rule). Documented the chosen rule in the `SetCharMetaTx`
doc comment and next to the assumption in `compute/bank.go`. Added a store-package
regression test file with four cases: `TestSetCharMetaTx_SingleBankToonInvariant`
(seed an existing bank toon, promote a second via the write path, assert exactly one
`is_bank_toon=1` remains and it is the new one), `TestSetCharMetaTx_ReSaveSelfIsNoOpDemote`
(re-saving the current bank toon does not self-demote — the `id <> ?` exclusion),
`TestSetCharMetaTx_DemoteToFalseLeavesNoBankToon`, plus happy-path write +
removed/missing-char fail-closed coverage matching the store's existing
`coin_test.go` idioms (`insertOwner`/`insertChar`/`commitTx`, `NewTestDB`).

### LR-01: `main_test.go` did not assert the char-meta route gate (contradicting `charmeta_test.go`'s docstring)

**Files modified:** `cmd/squirebot-server/main_test.go`,
`internal/backendsrv/webadmin/charmeta_test.go`
**Commit:** `9b608a4`
**Applied fix:** Took the stronger fix (the test, not just a docstring correction).
Extended `TestWriteRoutes_Gates` to register the char-meta routes exactly as
`runServe` does — `GET /api/v1/char/meta-list` and `POST /api/v1/char/meta` wrapped in
`webauth.RequireSession` — and added two subtests mirroring the coin assertions:
`char/meta anon→401` (no session → 401) and `char/meta-list member→admitted` (a plain
non-officer member session is admitted, NOT 401/403, proving the route is
`RequireSession` and not `RequireOfficer`). A future edit swapping in `RequireOfficer`
now fails this route-level test, which the gate-agnostic handler layer cannot catch.
Also corrected the `charmeta_test.go` docstring to point at the now-real coverage
(`TestWriteRoutes_Gates`) instead of vaguely claiming coverage that did not exist.

## Skipped Issues

### LR-02: success/error message lifecycle — verified-correct parity

**File:** `web/src/lib/components/CharMetaForm.svelte:87-95, 114`
**Reason:** skipped by design — the review itself concludes "None required —
documented as confirmed-correct parity." The success-banner/select lifecycle was
traced one-for-one against `BankCoinForm.svelte` and matches; there is no behavioral
defect to fix. No code change.
**Original finding:** The form intentionally keeps the selection after a successful
save (banner stays visible alongside the re-disabled Save button); changing the picker
clears the banner. Verified-correct clone parity.

### IN-01: handler echoes `level` as `*int64` while the coin sibling echoes concrete values

**File:** `internal/backendsrv/webadmin/charmeta.go:136-142`;
`web/src/lib/components/CharMetaForm.svelte:104-114`
**Reason:** out-of-scope (Info) and the reviewer explicitly recommended "No fix
needed." The echoed `class`/`level`/`race`/`is_bank_toon` fields are never read by the
client (the form's optimistic update reflects locally-computed `payload.*`, and only
consumes `res.character`); the divergence is inert. No code change.

### IN-02: `validateLevel`'s `Number.isSafeInteger` guard is unreachable given `/^\d+$/` + `<= 60`

**File:** `web/src/lib/charmeta.ts:78-85`
**Reason:** out-of-scope (Info); reviewer recommended "Leave as-is for parity." The
guard is faithfully inherited from `coin.ts`'s `validateCoinField` (where it IS
load-bearing — coin has no upper bound). In the char-meta clone the `<= 60` cap makes
it dead but defensively harmless, and it keeps the two helpers structurally identical.
No code change.

### IN-03: client/server value-set duplication (`CLASSES`/`RACES` in both `charmeta.ts` and `eqconst.go`)

**File:** `web/src/lib/charmeta.ts:17-30`;
`internal/backendsrv/enrich/eqconst.go:26-38`
**Reason:** out-of-scope (Info); reviewer recommended "No change required." The
duplication is intentional (TS `<select>` source + Go authoritative validator). A
desync degrades UX (a valid choice rejected, or a client-offered choice 400'd) rather
than corrupting data, since the server is authoritative. Flagged as a drift surface
only. No code change.

---

_Fixed: 2026-06-01_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
