# Phase 7: Admin Allowlist + Eviction Enforcement - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-11
**Phase:** 07-admin-allowlist-eviction-enforcement
**Areas discussed:** Bootstrap mechanism, Storage shape + owner-floor tracking, Non-admin failure UX, Admin management UX shape (all delegated to Claude)

---

## Gray-area selection

Claude presented four phase-specific gray areas for Phase 7 (Admin Allowlist + Eviction Enforcement). User declined to pick any to discuss individually:

> "I have no preference in any of those four areas. Please make whichever choice(s) that will make the end-user experience as simple as possible."

The four areas, each with the options Claude would have explored:

### Bootstrap mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Lazy `onOpen` bootstrap | First open after deploy seeds `_meta.guild_admins` with `getOwner().getEmail()`; idempotent on re-open. Zero clicks. | ✓ |
| `installTriggers` hook | Bootstrap runs as part of the existing one-time install step. Requires owner to remember to re-run on deploy. | |
| Dedicated menu item | "Initialize Admin Allowlist" menu item the owner must remember to click. | (kept as fallback) |
| Piggyback on `migrateToV3` | Bootstrap added to the migration. Couples permissions to schema. | |

**Claude's choice:** Lazy `onOpen` (primary) + manual-fallback menu item (secondary).
**Why:** Zero friction for the 95% case. Idempotent. Self-healing. Manual fallback covers the `getOwner()`-returns-null edge case under `drive.file` for consumer accounts.

### Storage shape + owner-floor tracking

| Option | Description | Selected |
|--------|-------------|----------|
| JSON array + separate floor row | `_meta.guild_admins` = JSON array; `_meta.workbook_owner_floor` = single email row. Matches eviction_log pattern. | ✓ |
| Comma/newline-delimited string | Single cell, delimiter-parsed. Breaks if emails contain commas. | |
| Own sheet (`_admins` tab) | Single-column tab. Cell value is the same data; tab is more visible than needed. | |
| First element of guild_admins = owner | Implicit convention. Too clever; future readers would have to know. | |
| Compute owner-floor live from `DriveApp.getOwner()` | Brittle (returns null under `drive.file`); ownership-transfer-ambiguous. | |

**Claude's choice:** JSON array for guild_admins; separate `_meta.workbook_owner_floor` row.
**Why:** Matches existing `_meta.eviction_log` pattern (JSON.stringify + defensive parse). Decoupled concerns: list membership and floor identity. Each extensible independently.

### Non-admin failure UX

| Option | Description | Selected |
|--------|-------------|----------|
| Apps Script modal (`getUi().alert`) | One screen, one message, one button. Sidebar never opens. | ✓ |
| Sidebar opens with banner + disabled controls | Two pieces of UI state; user can ignore disabled banner. | |
| Sidebar opens normally; server-side reject only | UX confusion (load sidebar, pick email, click Evict, see error). | |
| Hide menu item from non-admins | Requires per-user `onOpen` logic; blocked by simple-trigger auth constraints. | |

**Claude's choice:** Apps Script modal alert.
**Why:** One screen, one message, zero state to recover from. Matches native Sheets permission-error UX. Cheap to implement.

### Admin management UX shape

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated `showAdminMgmtSidebar` | New 300px sidebar matching every other v1.0 sidebar pattern. List + add + remove. | ✓ |
| `Browser.inputBox` prompts | Quick to ship, no visibility into current list, fragile (typo → bad allowlist entry). | |
| Expand eviction sidebar with admin-mgmt section | Conflates two distinct workflows. Different mental models. | |

**Claude's choice:** Dedicated `showAdminMgmtSidebar` + new "Manage Admins…" menu item.
**Why:** Matches every other v1.0 sidebar (Search, Eviction, Bank-Coin, Char-Info, Theme Picker). Code-reuse from `showEvictionSidebar.ts` is direct. Five-minute UX for an admin.

---

## Claude's Discretion

User delegated ALL four gray areas to Claude with the directive "make whichever choice(s) that will make the end-user experience as simple as possible." Claude resolved them as follows (full rationale in CONTEXT.md `<decisions>`):

- **D-01 Bootstrap mechanism:** Lazy `onOpen` + manual-fallback menu item
- **D-02 Storage shape:** JSON array (guild_admins) + separate single-email row (workbook_owner_floor) + JSON-array audit log (admin_log)
- **D-03 Non-admin failure UX:** Apps Script modal alert before sidebar construction; server-side `requireAdminOrThrow` on every callback (defense in depth)
- **D-04 Admin management UX:** Dedicated `showAdminMgmtSidebar`; new menu item "Manage Admins…" placed between "Evict Guildie…" and "Set Theme…"
- **D-05 Server-side admin guard module:** New `apps-script/src/lib/admin.ts` centralizing every admin-policy primitive (added by Claude as a structural decision, not from the four presented areas)
- **D-06 `Session.getEffectiveUser` empty-fallback policy:** Fail-closed for authorization decisions; soft-fallback to `'unknown'` for audit-log `initiated_by` (matches the existing eviction sidebar A5 pattern)

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` block. Highlights:

- Auto-remove evicted guildies from guild_admins (hygiene)
- Dynamic per-user menu hiding (blocked by simple-trigger auth)
- Workbook ownership transfer UX (rare; manual cell edit suffices)
- "Promote to owner-floor" UX (v1.1 candidate)
- Role-based admin tiers (v2 candidate)
- Admin-log retention/rotation (revisit at 50KB cell value)
- Cross-workbook admin sync (v2)
- Self-eviction protection (defer indefinitely)

No scope creep. No new ideas surfaced that fell outside Phase 7 scope.
