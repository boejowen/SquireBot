# Phase 19: Wantlist CRUD - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-03
**Phase:** 19-Wantlist CRUD
**Areas discussed:** Entry fields, Add & dedupe, In-bank indicator

(Area "List presentation" was offered but not selected — defaulted by Claude; see Claude's Discretion.)

---

## Entry fields

| Option | Description | Selected |
|--------|-------------|----------|
| Reason + priority + note | Reason required (buy/quest), Low/Med/High priority (default med), optional note ≤280 chars | ✓ |
| Reason + note, no priority | Lean v1: reason + optional note, defer priority | |
| Reason + numeric rank + note | Reason + numeric priority rank + optional note | |

**User's choice:** Reason + priority + note.
**Notes:** Priority is a simple Low/Med/High enum (default med), drives the default list sort. Note capped at 280 chars.

---

## Add & dedupe

### Add scope

| Option | Description | Selected |
|--------|-------------|----------|
| Catalog-only | Every want must resolve to a real catalog item_id (name+ID search, fuzzy "did you mean") | |
| Catalog + custom text | Also allow a freeform custom want with no item_id — display-only, not auto-matched by monitors | ✓ |

**User's choice:** Catalog + custom text.
**Notes:** Custom wants (NULL item_id) are display-only — excluded from the in-bank join and from the later EC/WTS/raid monitors. UI must mark them "custom — won't trigger alerts".

### Dedupe

| Option | Description | Selected |
|--------|-------------|----------|
| One row per item | Unique (user, item); reason is an editable property | |
| Allow buy + quest both | Unique (user, item, reason); same item can appear twice with different reasons | ✓ |

**User's choice:** Allow buy + quest both.
**Notes:** Uniqueness key becomes (web_user, item_id, reason) for catalog items; custom wants dedupe on (web_user, custom_label, reason) or in-handler — planner's call.

---

## In-bank indicator

| Option | Description | Selected |
|--------|-------------|----------|
| Where + who holds it | Show which character(s) across the guild hold it + count (the core value); joins consolidated all-character inventory | ✓ |
| Bank toon only (yes/no) | Literal WANT-02 reading — yes/no against the bank toon only | |
| Any guildie (yes/no) | Yes/no if any character has it, without naming the holder | |

**User's choice:** Where + who holds it.
**Notes:** Joins the existing consolidated all-character inventory; keyed on stable item_id. Only catalog wants resolve; custom wants show "—".

---

## Claude's Discretion

- **List presentation (not discussed — defaulted):** reuse the existing filterable/sortable DataGrid + HTML item tooltips (twin of the 4 views); default sort priority (high→low) then in-bank status; friendly empty state. Planner may adjust columns.
- **Security/identity shape:** locked upstream (account.go twin — login-only, server-derived owner per v2.1 D-02, IDOR-safe, audited); not re-litigated.
- **Custom-want storage detail** (separate custom_label column vs reusing note): planner's call, provided custom wants are visually distinct and excluded from matching.

## Deferred Ideas

- Guild-wide "who wants what" roll-up — separate future feature (already in REQUIREMENTS.md Future Requirements).
- WTB monitoring / price-threshold alerts — alert-side refinements, Phases 21+.
- Auto-promoting a custom text want to a real item_id once the catalog learns it — future polish.
