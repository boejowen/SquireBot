# Phase 33: Banks Tab + Valuation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-18
**Phase:** 33-banks-tab-valuation
**Areas discussed:** Bank roster scope & order, Valuation display, Item search behavior, Selected-bank detail
**Mode:** default (interactive); all 4 areas selected; all locked to the recommended option (the user's delegate-and-lock pattern).

---

## Bank roster scope & ordering (BANK-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Bank toons + bots, A-Z | List every `IsBankToon` AND `IsGuildBot` char, alphabetically; both count toward the valuation totals | ✓ |
| Bank toons only, A-Z | Only `IsBankToon`; exclude bots (their holdings would also drop from the totals) | |
| You decide | Lock to recommended | |

**User's choice:** Bank toons + bots, A-Z (D-01)
**Notes:** Both designations hold shared guild goods; "same ordering style as Characters" reduces to A-Z because banks aren't anyone's viewer/assigned chars.

---

## Valuation display (BANK-02)

| Option | Description | Selected |
|--------|-------------|----------|
| Guild-wide summary header | One total item value + one total platinum at the top; rows stay clean | ✓ |
| Summary + per-row subtotals | Top summary AND each bank's own value/platinum on its row | |
| You decide | Lock to recommended | |

**User's choice:** Guild-wide summary header (D-02)
**Notes:** Per-bank number moves to the detail header instead (D-04), keeping list rows clean.

---

## Item search behavior (BANK-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Item-centric (P32-style) | Name → which bank(s) hold it + qty/slot → click a holder opens that bank's window | ✓ |
| Bank-list filter | Name → narrows the bank list to banks holding a match | |
| You decide | Lock to recommended | |

**User's choice:** Item-centric, P32-style (D-03)
**Notes:** Reuses the Phase 32 rollup scoped to `is_bank` holders; consistent with the Inventory tab's search + cross-tab jump.

---

## Selected-bank detail header

| Option | Description | Selected |
|--------|-------------|----------|
| Per-bank value/platinum header | THIS bank's own value + platinum above the reused P31 window | ✓ |
| Just the window | Open the reused window unchanged; totals only in the top summary | |
| You decide | Lock to recommended | |

**User's choice:** Per-bank value/platinum header (D-04)
**Notes:** Pairs with D-02 — guild-wide totals at top, per-bank slice in the detail.

---

## Claude's Discretion
- Backend shape (bank-scoped endpoint vs composing existing reads: `BankValuationFor`/`TotalPlatinum` + `RosterFor` band-2 + the P31 window route + the P32 item rollup).
- Item-search wiring (reuse P32 `/api/v1/items` client-filtered to `is_bank` vs a new bank-scoped endpoint).
- Per-bank value/platinum sourcing for D-04.
- Whether a migration is needed (expected: none — designation/bank-coin shipped in v2.3, icon/statsblock in P31; schema v13).
- Exact layout / mobile reflow → UI-SPEC.

## Deferred Ideas
- Sort/filter controls on the bank list beyond A-Z + item search (future polish).
- Per-item value column inside the bank window (the window is reused as-is from P31).
- Wishlist (Phase 34).
