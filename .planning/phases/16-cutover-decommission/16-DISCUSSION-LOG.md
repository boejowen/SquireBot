# Phase 16: Cutover + Decommission - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-31
**Phase:** 16-cutover-decommission
**Areas discussed:** Backfill data + tool, Soak + go/no-go, Flip + guild codes, Decommission order

---

## Backfill data + tool (CUTOVER-02)

### Q1 — How to extract human-supplied data out of the frozen Sheet?

| Option | Description | Selected |
|--------|-------------|----------|
| Manual export | Download tabs as CSV/TSV from the Sheet UI, parse offline (zero Google OAuth) | |
| Read-only Sheets API | One-off script with read-only OAuth (finding 04 §4.3) | |
| Hand-enter | Re-type the ~120 chars' metadata directly | |

**User's choice:** *Free-text* — "Do not extract the human-supplied data from the old/frozen sheet. I want the 'google free' SquireBot to be a fresh start, at least as far as guildie data is concerned."
**Notes:** Rejected the entire backfill premise. Reframes CUTOVER-02 from "import from Sheet" to "establish natively." Eliminated the biggest risk surface (Sheets export + Google-email↔Discord owner reconciliation). Closest to finding 04 §4.1 Option C, extended to all human data.

### Q2 — With no backfill, how do class/level/race/is_bank_toon get set?

| Option | Description | Selected |
|--------|-------------|----------|
| Add a web form | Lightweight authenticated char-metadata form | ✓ |
| CLI setter / SQL | `set-char-meta` subcommand / raw SQL, maintainer does entry | |
| Infer + defer rest | Infer class/level from spellbook; leave race/is_bank_toon unset; accept degraded views | |

**User's choice:** Add a web form
**Notes:** Required (not optional) — gear_check/spell_check/bank depend on these fields and there's no other source. Restores the v1 char-info-sidebar capability v2 never rebuilt. Columns already exist in `00001_init` so no migration. Flips P16 UI hint no→yes. Edit-model + creation semantics locked by the bank-coin ADMIN-05/D-12 precedent (login-only, any member, existing chars only) without a separate question.

---

## Soak + go/no-go (CUTOVER-01)

### Q1 — Soak model, given fresh-start + the Sheet already frozen?

| Option | Description | Selected |
|--------|-------------|----------|
| Canary-first | Maintainer + 1–2 volunteers soak ~1 wk, then guild flip | |
| Flip-all then soak | Everyone flips at once, then soak 1–2 wk | |
| Minimal / no formal soak | Backend already live+verified; just flip + confirm reporting in | ✓ |

**User's choice:** Minimal / no formal soak
**Notes:** CUTOVER-01 reinterpreted (not dropped). Backend live since 2026-05-29; P12 verified vs live wiki/PigParse; P14/P15 live UAT; Sheet frozen since 2026-05-15 so nothing to shadow. Satisfied by a brief "guildies reporting in" confirmation folded into the flip gate. Thinner margin accepted.

---

## Flip + guild codes (CUTOVER-03)

### Q1 — How should the flip reach all ~12 guildies?

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-update + herding | Publish Release + self-update + DM codes + installer link for dormant watchers | ✓ |
| Passive auto-update only | Publish + post codes, rely on self-updater + paste prompt | |
| Clean reinstall for all | Everyone reinstalls fresh from the installer | |

**User's choice:** Auto-update + herding
**Notes:** Realistic for 12 known people; actively covers guildies whose watchers went dormant during the dark weeks.

### Q2 — Go/no-go threshold for proceeding to decommission?

| Option | Description | Selected |
|--------|-------------|----------|
| Majority + chase | Proceed at ~10/12 + chase stragglers after | |
| Wait for 100% | Hold teardown until all ~12 report in | |
| Hard deadline | Announce a date; decommission on it | |

**User's choice:** *Free-text* — "Proceed with the flip as soon as is feasible from a technical standpoint. The guildies know that the change is coming and are prepared to migrate."
**Notes:** No migration-% gate. Technical readiness is the gate (binary published + codes minted + char-meta form live). Stragglers onboard anytime; decommission strands no one. Planning flag: confirm the P13 binary is actually published as a GitHub Release.

---

## Decommission order (CUTOVER-04)

### Q1 — What happens to the Google Sheet once the flip is confirmed?

| Option | Description | Selected |
|--------|-------------|----------|
| Export then delete | One-time XLSX archive offline, then delete Sheet + Apps Script | |
| Freeze + grace, then delete | Read-only ~30 days, then delete | |
| Delete outright | Delete Sheet + Apps Script, no archive | |

**User's choice:** *Free-text* — "Don't bother touching the Google Sheet once the flip is confirmed."
**Notes:** Prompted a scope-clarification follow-up (Q2) because two neighbors of the Sheet aren't inert.

### Q2 — How far does "don't bother" extend for CUTOVER-04?

| Option | Description | Selected |
|--------|-------------|----------|
| Kill live bits, abandon rest | Disable Apps Script triggers + retire OAuth client; leave Sheet in place | ✓ |
| Total benign neglect | Touch nothing; everything dormant | |
| Full teardown after all | Delete Sheet + Apps Script + OAuth client + Cloud project | |

**User's choice:** Kill live bits, abandon rest
**Notes:** Disable the still-running Apps Script enrichment triggers (stops double-load on community APIs + quota burn) + retire the OAuth client; abandon the Sheet's data in place. Satisfies CUTOVER-04 as "no live Google machinery remains" (code-level Google-freedom already proven in P13). Proof artifact = a decommission checklist. Planning flag: verify the triggers are actually still running.

## Claude's Discretion

- Char-meta form auth/semantics (login-only, any member, existing chars only) — locked by the bank-coin ADMIN-05/D-12 precedent rather than asked.
- Whether the char-meta form needs `/gsd-ui-phase` — deferred to the planner (small form, strong analogs).
- Build/ops sequencing — recommended order captured in CONTEXT planning flags.

## Deferred Ideas

None — the discussion stayed within phase scope. The "fresh start" and "abandon the Sheet" calls are scope *reductions* captured as decisions, not deferrals.
