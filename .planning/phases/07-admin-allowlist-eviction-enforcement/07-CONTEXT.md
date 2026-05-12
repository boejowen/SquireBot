# Phase 7: Admin Allowlist + Eviction Enforcement — Context

**Gathered:** 2026-05-11
**Status:** Ready for planning
**Mode:** discuss (user delegated all four gray areas to Claude with the directive "make whichever choice(s) that will make the end-user experience as simple as possible")

---

## Why this phase exists (one paragraph)

The v1.0 eviction workflow is enforced by social convention only: anyone with edit access to the workbook can open `Evict Guildie…` and mark another guildie's characters as removed. That worked for v1.0 because the guild is twelve people who trust each other, but the eviction sidebar is also the only destructive workflow in the entire product — a misclick by a curious non-officer is a 30-day-grace mistake that nobody wants to debug. Phase 7 closes the gap by introducing a server-side allowlist (`_meta.guild_admins`), routing every eviction-related callback through a fail-closed admin check, and giving admins a simple sidebar to add or remove other admins without ever being able to remove the workbook owner. The user experience for the 95% case (non-admin guildies) is unchanged from v1.0: they don't see the menu items in their workflows, and if they wander into "Evict Guildie…" by curiosity they get one clear modal and zero state changes. For the 5% case (officers), they get a working eviction sidebar exactly as before, plus a new "Manage Admins…" menu item that lets them invite peers.

<domain>
## Phase Boundary

**In scope (per ROADMAP §53-65 + REQUIREMENTS.md ADMIN-01/02/03):**

- **`_meta.guild_admins` row** — JSON array of lowercased admin emails, idempotent on re-bootstrap. (ADMIN-01)
- **`_meta.workbook_owner_floor` row** — single-email string captured once at bootstrap; the email that cannot be removed by anyone other than themselves. (Supports ADMIN-03.)
- **Lazy `onOpen` bootstrap** — first time the v1.0.1 apps-script bundle lands and the workbook is opened, `_meta.guild_admins` is seeded with `getActiveSpreadsheet().getOwner().getEmail()` and `_meta.workbook_owner_floor` is written with the same value. Subsequent opens are silent no-ops. (ADMIN-01 bootstrap.)
- **Manual-bootstrap fallback** — new `bootstrapGuildAdmins()` function exposed as a "Initialize Admin Allowlist" menu item under SquireBot menu's "Admin (Phase 7)" subsection. Used if `getOwner()` returns null (consumer-account quirk under `drive.file`); uses `Session.getEffectiveUser().getEmail()` as the seed and gates with a confirmation prompt naming the email being written.
- **Server-side admin guard** — new helper `requireAdminOrThrow(callerEmail)` in `apps-script/src/lib/admin.ts` that reads `_meta.guild_admins`, lowercases + trims comparison, throws `Error('not_authorized')` on miss. Every google.script.run callback in eviction + admin-mgmt sidebars calls it FIRST.
- **Non-admin failure UX for eviction** — `showEvictionSidebar` checks admin status before constructing the sidebar. Non-admin → `SpreadsheetApp.getUi().alert('Not authorized', 'Only guild officers can evict members. Contact a workbook admin if you think this is wrong.', SpreadsheetApp.getUi().ButtonSet.OK)`. The Apps Script modal is the only thing they see — no sidebar opens, no UI state to recover from. (ADMIN-02.)
- **Admin-management sidebar (`showAdminMgmtSidebar`)** — new 300px HtmlService sidebar in `apps-script/src/triggers/showAdminMgmtSidebar.ts`. Follows the inline `SIDEBAR_BODY` String.raw / theme-aware pattern from `showEvictionSidebar.ts`. Shows the current admin list (lowercased emails, sorted, with `(owner)` marker on the floor email), an "Add admin" text input + button, and a per-row "Remove" button next to each non-floor admin. Owner-floor lockout is enforced both client-side (Remove button absent on floor row when caller != floor) AND server-side (`removeAdmin(email)` rejects with `Error('owner_floor_protected')`).
- **Menu integration** — new "Manage Admins…" menu item appended under the existing "Evict Guildie…" item in `onOpen.ts`. Both items are visible to ALL openers; admin-check happens server-side when the item is clicked. No dynamic per-user menu hiding (simple-trigger constraints).
- **Server-side callbacks for admin-mgmt sidebar** — `getAdminList()`, `addAdmin(email)`, `removeAdmin(email)`. All three go through `requireAdminOrThrow` first. `removeAdmin` additionally checks owner-floor protection. All three are LockService-wrapped (mandatory for `_meta` writes).
- **`_meta.admin_log` audit trail** — JSON-array envelope mirroring `_meta.eviction_log` shape; one entry per add/remove, recording `{ at: ISO, action: 'add'|'remove'|'bootstrap', email: string, initiated_by: string }`. Append-only, malformed-existing-log tolerant (same defensive parse as eviction_log).
- **Test coverage at the policy edges** — unit tests for `requireAdminOrThrow` (empty list, single owner, multi-admin, empty caller fail-closed), `addAdmin` (idempotent, lowercases-and-trims, rejects malformed input), `removeAdmin` (rejects floor-by-non-floor, allows self-removal of floor, allows admin-to-remove-other-admin), and `bootstrapGuildAdmins` (idempotent, null-getOwner fallback path). Vitest, mocked SpreadsheetApp — matches existing Phase 4/5 test conventions.

**Out of scope (deferred to later phases / backlog):**

- **JSDOM sidebar inline-JS tests for admin-mgmt sidebar** — covered by Phase 8 TEST-01/02 (every shipping sidebar gets a `__tests__/*-sidebar.test.ts` companion; the admin-mgmt sidebar is one of those, but the test-infrastructure work lands in Phase 8).
- **Dynamic per-user menu hiding** — would require `onOpen` to call `Session.getEffectiveUser().getEmail()`, which has simple-trigger constraints (may return empty, runs without authorization in some contexts). Defer to a polish phase if non-admin guildies complain about seeing greyed-out items.
- **Workbook ownership transfer UX** — `_meta.workbook_owner_floor` is captured once at bootstrap and never auto-updated. If ownership of the workbook transfers, the new owner can manually edit the `_meta.workbook_owner_floor` cell value (or run "Initialize Admin Allowlist" again after deleting the existing row). Not enough load to justify a UX surface in v1.0.1.
- **Auto-remove evicted guildies from `_meta.guild_admins`** — would be nice hygiene (if someone is evicted, they probably shouldn't still be an admin), but it crosses the eviction → admin policy boundary and the user can do it manually via the admin-mgmt sidebar. Capture as deferred idea below.
- **Email format validation beyond non-empty** — admins are trusted; garbage-in just means useless allowlist entries that they can remove. Not worth a regex.
- **Watcher-side change** — none. `_meta.guild_admins` is apps-script-owned. The watcher continues writing only to `_char_owner` / landing tabs. `WatcherMaxSchemaVersion` stays at 3.
- **Schema version bump** — none. `_meta.guild_admins` and `_meta.workbook_owner_floor` are extend-only row additions. `_meta.schema_version` stays at 3.
- **Eviction sidebar UX redesign** — the existing 300px sidebar is fine; this phase ONLY adds an admin-check at the entry point and at each callback. Layout, theming, lock semantics all unchanged.
- **Self-eviction protection** — an admin can still evict their own email if they want to; no special-case logic. (Edge case nobody has hit; deferred indefinitely.)

**Explicitly NOT a Phase 7 ambiguity (defaulted by Claude per user directive):**

All four gray areas surfaced during scout were defaulted toward end-user simplicity. See `<decisions>` below for the full rationale; the short version:

- **Bootstrap mechanism:** lazy `onOpen`, idempotent, with a manual-fallback menu item. Zero clicks for the 95% case.
- **Storage shape + owner-floor tracking:** JSON array for `guild_admins` (matches `eviction_log` pattern); separate `_meta.workbook_owner_floor` row for the lockout floor (unambiguous, decoupled from list ordering).
- **Non-admin failure UX:** Apps Script modal `getUi().alert(...)`. One screen, one message, no sidebar lurking.
- **Admin management UX shape:** dedicated `showAdminMgmtSidebar` matching every other v1.0 sidebar's pattern. New "Manage Admins…" menu item.

</domain>

<decisions>
## Implementation decisions (locked)

### D-01: Bootstrap mechanism — lazy `onOpen` with manual-fallback menu item

**Decision:** Add `bootstrapGuildAdmins()` to `apps-script/src/lib/admin.ts`. The function is idempotent and runs in two paths:

1. **Lazy automatic path:** Called from the top of `onOpen` (after the simple-trigger guards). Checks `_meta.guild_admins` — if it parses to a non-empty array, return immediately (no-op). Otherwise, reads `SpreadsheetApp.getActiveSpreadsheet().getOwner().getEmail()`. If non-empty, writes `JSON.stringify([ownerEmail.toLowerCase()])` to `_meta.guild_admins`, writes `ownerEmail.toLowerCase()` to `_meta.workbook_owner_floor`, and appends a `{ at, action: 'bootstrap', email: ownerEmail, initiated_by: 'onOpen' }` entry to `_meta.admin_log`. Wrapped in `LockService.getDocumentLock().tryLock(30000)`; on lock contention, silently skip (next open retries). Errors are logged at `warn` level — they do NOT throw out of `onOpen` (would break the menu for everyone).

2. **Manual-fallback path:** New menu item "Initialize Admin Allowlist (manual)" appended under "Admin (Phase 7)" sub-section of the SquireBot menu. Calls a wrapper `bootstrapGuildAdminsManual()` that does what the lazy path does, but uses `Session.getEffectiveUser().getEmail()` as the seed and shows a `getUi().alert('About to add {email} as the first admin and owner-floor. Continue?', ...ButtonSet.OK_CANCEL)` confirmation before writing. Designed for the case where `getOwner()` returns null (consumer-account quirk under `drive.file`).

**Rationale (vs. alternatives):**

- **`installTriggers` hook:** would require the workbook owner to remember to re-run `installTriggers` after every clasp push that includes a Phase 7 change. Brittle (Phase 5's existing menu structure relies on owners running it once, and there's no enforcement). Lazy onOpen runs every single open, idempotent — works even if the owner never touches `installTriggers` again.
- **Dedicated menu item only:** non-zero friction (owner has to remember to click it, has to find it in the menu). Bootstrap should "just happen."
- **Piggyback on `migrateToV3`:** would couple admin bootstrap to schema migrations. Bad coupling — admin bootstrap is a permissions concern, not a schema concern. Migrations are also supposed to be one-time-per-major-version events; admin bootstrap is one-time-per-workbook.
- **Lazy onOpen (chosen):** zero friction for the 95% case. Idempotent. Self-healing — even if the lazy path silently fails once (lock contention, transient API hiccup), the next open retries.

**Edge cases handled:**

- `getOwner()` returns null (consumer-account / shared-drive scenarios under `drive.file`): log a warning, set `_meta.admin_log` to `[{ at, action: 'bootstrap_failed', reason: 'owner_null', initiated_by: 'onOpen' }]` so the owner can see something happened, and rely on the manual-fallback menu item.
- Lock contention (two opens at the same instant): one wins, one no-ops via `tryLock` returning false. Next open is idempotent.
- Multiple admins exist when the manual-fallback fires (unexpected — `guild_admins` was non-empty all along): the function returns immediately because the idempotent check sees the existing list. No double-write.
- The workbook owner is a guildie who already has a `_char_owner` row: irrelevant — admin status is orthogonal to character ownership. The same email can be both an admin and have characters.

### D-02: Storage shape — JSON array for guild_admins; separate floor row

**Decision:**

- `_meta.guild_admins` value cell = `JSON.stringify(sortedLowercasedEmails)`. Example: `["alice@example.com","bob@example.com","jbowen@mncivic.com"]`. Always lowercased + trimmed + sorted before write (idempotent diffs; readers don't need to normalize).
- `_meta.workbook_owner_floor` value cell = single email string, lowercased + trimmed. Example: `jbowen@mncivic.com`. Written once at bootstrap; not auto-updated.
- `_meta.admin_log` value cell = `JSON.stringify(entries[])` where each entry is `{ at: ISO8601, action: 'add'|'remove'|'bootstrap'|'bootstrap_failed', email: string, initiated_by: string }`. Append-only, malformed-existing-log tolerant (same defensive parse-and-fall-through-to-[] pattern as `_meta.eviction_log` in `showEvictionSidebar.ts:170-176`).

**Rationale (vs. alternatives):**

- **Comma/newline-delimited string:** breaks if any email contains a comma/newline (unlikely but real); also requires every reader to do its own parsing. JSON.parse is one call, one shape, idiomatic.
- **Own sheet (`_admins` tab):** would breach the project's `_`-prefixed dimension-tab convention only if the sheet has columns — but here we'd have a single-column-of-emails sheet, which is exactly what a JSON array in a single cell is for. Sheet would also be reachable via the unhide-all-system-tabs flow that admins use for debugging, leaking the admin list more prominently than needed. Cell value in `_meta` keeps the data co-located with the other workbook-state data.
- **First element of `guild_admins` is implicitly the owner-floor:** too clever. Future readers would have to know this convention. Separate row is explicit.
- **Compute owner-floor live from `DriveApp.getOwner()` on every check:** brittle — `getOwner()` can return null under `drive.file` (the very reason D-01 needs a manual fallback). Cached value is safer and survives ownership-transfer ambiguity.
- **JSON array + separate floor row (chosen):** matches the existing `_meta.eviction_log` pattern (JSON.stringify in a single cell, defensive parse on read). Decoupled concerns: list membership and floor identity. Each is independently extensible (e.g., future "demote to read-only admin" would add a `role` field to entries WITHOUT touching the floor row).

**Why lowercase + trim?** Gmail addresses are case-insensitive (`Alice@Gmail.com` == `alice@gmail.com` per Gmail's policy) and `drive.file`-authorized emails arrive with various casings depending on which Google product surfaced them. Normalizing on write prevents false-negatives at compare time. Sort-on-write means `git diff`-style audits of the workbook (via clasp pull) produce deterministic output.

### D-03: Non-admin failure UX — Apps Script modal alert; no sidebar

**Decision:** `showEvictionSidebar` and `showAdminMgmtSidebar` both check admin status BEFORE constructing the sidebar HTML. On non-admin:

```typescript
SpreadsheetApp.getUi().alert(
  'Not authorized',
  'Only guild officers can {evict members | manage admins}. ' +
    'Contact a workbook admin if you think this is wrong.',
  SpreadsheetApp.getUi().ButtonSet.OK,
);
return;  // no sidebar opens
```

Server-side, `getEvictionEmails`, `previewEviction`, `commitEviction`, `getAdminList`, `addAdmin`, and `removeAdmin` ALL call `requireAdminOrThrow(callerEmail)` as their first statement. A non-admin who manages to invoke a callback (e.g., via `google.script.run` from a stale sidebar) gets `Error('not_authorized')` thrown and the action is rejected. Client-side check is the UX; server-side check is the security boundary.

**Caller identity:** `Session.getEffectiveUser().getEmail()` — same call the eviction sidebar already uses for `initiated_by` (A5 sandbox-empty fallback). For the AUTHORIZATION check, however, empty string is treated as "not admin" (fail-closed); we do NOT fall back to 'unknown' for authorization decisions. The audit-log fallback to 'unknown' is separate and only applies to recording who did something.

**Rationale (vs. alternatives):**

- **Sidebar opens with banner + disabled controls:** two pieces of UI state for non-admins to look at (the sidebar AND the banner). Modal is one piece. Modal is also un-dismissable until they click OK — they read the message. A disabled sidebar can be ignored.
- **Sidebar opens normally; server-side reject only:** UX confusion (sidebar loads, user picks an email, clicks Evict, sees an error toast). Modal cuts that to one screen.
- **Hide menu item from non-admins via per-user `onOpen` logic:** requires `Session.getEffectiveUser().getEmail()` in simple-trigger `onOpen`, which may return empty (CLAUDE.md A5) and runs without authorization in some contexts (unreliable). Also adds per-user state to a function that's currently stateless. Defer to a polish phase.
- **Modal alert (chosen):** one screen, one message, one button. Zero state to recover from. Matches native Google Sheets UX (Sheets uses modals for permission errors). Cheap to implement (`getUi().alert`).

### D-04: Admin management UX shape — dedicated sidebar + new menu item

**Decision:** New file `apps-script/src/triggers/showAdminMgmtSidebar.ts` with three google.script.run callbacks: `getAdminList()`, `addAdmin(email)`, `removeAdmin(email)`. Follows the `showEvictionSidebar.ts` template verbatim: inline `SIDEBAR_BODY` String.raw constant, `themeStyleBlock` + `buildSidebarHtml` helpers, 300px width, `escapeHtml` for every interpolation, `aria-live="polite"` status div.

**Sidebar layout:**
```
SquireBot — Manage admins                [300px]
─────────────────────────────────────────────────
Manage who can evict guildies. Owner-floor
email cannot be removed.

Current admins (3):
  • alice@example.com           [Remove]
  • bob@example.com             [Remove]
  • jbowen@mncivic.com (owner)
                                (no remove)

Add admin
  ┌─────────────────────────────────┐
  │                                 │
  └─────────────────────────────────┘
  [Add admin]

[status / error message area, aria-live]
```

**Menu item:** new "Manage Admins…" item appended to the SquireBot menu in `onOpen.ts`, placed immediately AFTER "Evict Guildie…" and BEFORE "Set Theme…". Visible to all openers; server-side admin-check handles authorization.

**Owner-floor enforcement:**

- **Client-side:** the "Remove" button is not rendered next to the floor email when the caller's email != floor (the sidebar's initial `getAdminList()` call returns `{ admins: string[], floor: string, callerEmail: string }`).
- **Server-side:** `removeAdmin(targetEmail)` reads `_meta.workbook_owner_floor`, gets caller via `Session.getEffectiveUser().getEmail()` + lowercase + trim, and rejects with `Error('owner_floor_protected')` if `targetEmail === floor && callerEmail !== floor`.
- **Self-removal of floor by the floor user:** allowed. The floor user CAN remove themselves (e.g., they're stepping down). The floor row stays at the old value (orphaned — refers to an email no longer in the admin list). This is fine; the floor row is the "who is protected from non-self removal" pointer, not the "who is currently an admin" pointer. Comment in `removeAdmin` documents this.

**Add-admin validation:** `addAdmin(email)` trims, lowercases, rejects empty string, rejects strings without `@` (single-char sanity check — not full RFC 5322 validation). If the email is already in the list, returns `{ added: false, alreadyExists: true }` (idempotent; no error). On success, appends `{ at, action: 'add', email, initiated_by }` to `_meta.admin_log`.

**Remove-admin behavior:** `removeAdmin(email)` trims, lowercases. If the email is not in the list, returns `{ removed: false, notFound: true }` (idempotent; no error). On success, appends `{ at, action: 'remove', email, initiated_by }` to `_meta.admin_log`.

**Rationale (vs. alternatives):**

- **Browser.inputBox prompts (Add Admin… / Remove Admin… menu items):** quick to ship, but no visibility into the current list, two clicks per action, fragile UX (typo in `Browser.inputBox` and you've written `aliice@example.com` to the allowlist). Sidebar shows the list and lets admins act on it directly.
- **Expand eviction sidebar with admin-mgmt section:** conflates two distinct workflows. Eviction is destructive and 30-day-grace; admin-mgmt is a quiet allowlist edit. Different mental models, different sidebars.
- **Dedicated sidebar (chosen):** matches every other v1.0 sidebar (Search, Eviction, Bank-Coin, Char-Info, Theme Picker). Code-reuse from `showEvictionSidebar.ts` is direct (the sidebar shape is nearly identical: list of items, action button, error/status area). Discoverable via menu. Five-minute UX for an admin.

### D-05: Server-side admin guard — new `lib/admin.ts` module

**Decision:** New file `apps-script/src/lib/admin.ts` exports:

- `getAdminList(): { admins: string[]; floor: string }` — reads both `_meta` rows, parses JSON, returns normalized arrays. Tolerates malformed `guild_admins` JSON (returns `{ admins: [], floor: '' }` — fail-closed; nobody is admin if the cell is corrupt).
- `isAdmin(email: string): boolean` — convenience; lowercases + trims input, returns whether it's in the list.
- `requireAdminOrThrow(email: string): void` — throws `Error('not_authorized')` if `!isAdmin(email)`. Empty/null email also throws (fail-closed). Used by every protected callback.
- `bootstrapGuildAdmins(opts?: { seedEmail?: string; initiatedBy?: string }): { bootstrapped: boolean; seedEmail?: string }` — the function called from `onOpen` (no opts; uses `getOwner()`) and from the manual-fallback menu item (opts.seedEmail = `Session.getEffectiveUser().getEmail()`).
- `addAdmin(email: string, callerEmail: string): { added: boolean; alreadyExists?: boolean }` — protected by `requireAdminOrThrow(callerEmail)`. Lock-wrapped.
- `removeAdmin(email: string, callerEmail: string): { removed: boolean; notFound?: boolean }` — protected by `requireAdminOrThrow(callerEmail)`. Owner-floor-protected. Lock-wrapped.
- `appendAdminLogEntry(entry: AdminLogEntry): void` — internal helper; same defensive parse-existing pattern as eviction_log.

**Rationale:** centralizing the admin-policy primitives in one file (a) makes the unit tests trivial (one file under test), (b) prevents the eviction sidebar from re-implementing the same admin-check pattern with a subtle bug, (c) lets future phases (999.1 bank-coin permission lock; 999.5 self-service eviction) import the same primitives.

### D-06: `Session.getEffectiveUser` empty-fallback policy

**Decision:** Two distinct fallback policies for `Session.getEffectiveUser().getEmail()`:

- **For authorization checks (`requireAdminOrThrow`):** empty string is treated as "not admin" — `requireAdminOrThrow` throws `Error('not_authorized')`. Fail-closed.
- **For audit-log `initiated_by` recording:** fall back to `'unknown'` — load-bearing fields (`at`, `action`, `email`) are still recorded so the audit trail isn't lost. Matches `showEvictionSidebar.ts:151-153` (A5 sandbox-quirk fallback).

**Rationale:** authorization requires evidence; absence of evidence is absence of authorization. Audit-logging is about recording what happened — `'unknown'` is still useful information ("an admin action ran in a context where we couldn't identify the actor").

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 7 contract
- `.planning/ROADMAP.md` § "Phase 7: Admin Allowlist + Eviction Enforcement" — the 5 success criteria are the ship gate. Especially criterion 2 ("safe no-op for non-admins") which is the security boundary.
- `.planning/REQUIREMENTS.md` § ADMIN-01 / ADMIN-02 / ADMIN-03 — the three requirements this phase covers. ADMIN-03's "owner-floor lockout protection" is the load-bearing nuance.
- `.planning/STATE.md` § "Decisions Log (v1.0.1)" — milestone-level decisions: no schema bump, no watcher rebuild, single ship gate = `clasp push`.

### Files being modified
- `apps-script/src/triggers/showEvictionSidebar.ts` — admin guard goes in the opener AND in all three callbacks (`getEvictionEmails`, `previewEviction`, `commitEviction`). Lines 48-57 (opener), 59-75, 82-106, 113-187 (callbacks). The inline `SIDEBAR_BODY` template at line 228 is the pattern the new admin-mgmt sidebar will mirror verbatim.
- `apps-script/src/triggers/onOpen.ts` lines 7-29 — menu definition. New "Manage Admins…" item between "Evict Guildie…" and "Set Theme…" (after line 22). New "Admin (Phase 7)" sub-separator with "Initialize Admin Allowlist (manual)" item below the existing migration items (after line 26).
- `apps-script/src/Code.ts` lines 1-77 — re-export footer must include the new exports (`showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`). Apps Script trigger system finds globals by name; missing re-export = silent menu failure.

### Files being created
- `apps-script/src/lib/admin.ts` — central admin-policy module (D-05).
- `apps-script/src/triggers/showAdminMgmtSidebar.ts` — admin-management sidebar (D-04).
- `apps-script/src/__tests__/admin.test.ts` — unit tests for `requireAdminOrThrow`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`, `isAdmin`.
- `apps-script/src/__tests__/adminMgmtSidebar.test.ts` — tests for `getAdminList` callback shape, owner-floor server-side enforcement. (Inline-JS DOM tests are deferred to Phase 8 TEST-02.)

### Reference patterns (read before implementing)
- `apps-script/src/triggers/showEvictionSidebar.ts` — closest existing sidebar shape. New admin-mgmt sidebar clones (a) the `themeStyleBlock` + `buildSidebarHtml` + `SIDEBAR_BODY` String.raw triplet, (b) the inline `escapeHtml` helper at line 257 (verbatim — do not re-implement), (c) the lock-wrapped write pattern at lines 122-187, (d) the JSON-array malformed-tolerant parse at lines 166-176.
- `apps-script/src/lib/sheet-helpers.ts` lines 26-56 — `readMetaRows` + `writeMetaRow` are the only primitives the admin module needs for `_meta` access. No raw `getRange` calls in `lib/admin.ts` — go through these.
- `apps-script/src/lib/migrations.ts` lines 85-95 (the `LockService.getDocumentLock().tryLock(30000)` envelope) — every multi-write in `lib/admin.ts` must use this exact pattern. The "schema_version write is last" rule does NOT apply here (no migration; no `schema_version` write).
- `apps-script/src/triggers/showCharInfoSidebar.ts` — secondary reference for a sidebar that has both read+write callbacks with form-style input (closest shape to the admin-mgmt sidebar's "Add admin" text input). Cell-note tooltip pattern there is not needed.

### Project-wide constraints
- `CLAUDE.md` § "Architecture" — `_meta.guild_admins` and `_meta.workbook_owner_floor` are extend-only `_meta` row additions; no schema_version bump. `Session.getActiveUser().getEmail()` returns the SCRIPT owner, NOT the caller — load-bearing distinction; use `getEffectiveUser` everywhere.
- `CLAUDE.md` § "Conventions" — Apps Script TypeScript lives in `apps-script/src/` (libs in `lib/`, triggers in `triggers/`, tests in `__tests__/`). Structured logging via `log('level', op, fields)`.
- `.planning/research/PITFALLS.md` § Pitfall P6 — every multi-cell `_meta` write MUST be `LockService.getDocumentLock().tryLock(30000)`-wrapped; missing lock causes lost-write races under concurrent admin actions.

### External docs
- No ADRs for this scope. No external library docs needed (no new dependencies). `google-apps-script` types for `SpreadsheetApp.getUi().alert(...)` signature are in `@types/google-apps-script` (already a dep).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `apps-script/src/lib/sheet-helpers.ts` — `readMetaRows('_meta')` + `writeMetaRow('_meta', key, value)` cover every `_meta` read/write the admin module needs. JSON.stringify the array on write; JSON.parse + defensive `[]` fallback on read.
- `apps-script/src/triggers/showEvictionSidebar.ts:122-187` — lock-wrapped, malformed-tolerant, audit-log-appending write pattern. Copy this shape into `addAdmin`/`removeAdmin`.
- `apps-script/src/triggers/showEvictionSidebar.ts:228-323` — inline `SIDEBAR_BODY` String.raw template. Clone for the admin-mgmt sidebar; swap the body content; keep the `<style>`, `escapeHtml`, and `aria-live` patterns verbatim.
- `apps-script/src/lib/log.ts` — `log('info'|'warn'|'error', op, fields)` helper. Every admin-module function emits at least one structured log call (op = function name; fields include `email`, `callerEmail` for traceability).
- `apps-script/src/lib/themes.ts` — `getActiveTheme()` + `THEMES[themeKey]` + `Theme` type. Admin-mgmt sidebar consumes these identically to the eviction sidebar.

### Established Patterns
- **`_meta` KV shape** — every dimension/state value lives in a single `(key, value)` row on the `_meta` tab. Structured values are JSON-stringified into the value cell. Defensive parse-and-fall-through-to-`[]` on read (handles human-edited cells gracefully).
- **LockService envelope for `_meta` writes** — `const lock = LockService.getDocumentLock(); if (!lock.tryLock(30000)) throw new Error('lock_busy'); try { ... } finally { lock.releaseLock(); }`. Mandatory for any multi-step `_meta` write.
- **Caller identity = `Session.getEffectiveUser().getEmail()`** — the eviction sidebar's `initiated_by` audit field (line 151) is the project's existing answer to "who did this." Phase 7 extends this to authorization with a fail-closed policy on empty.
- **Inline `SIDEBAR_BODY` String.raw template (Option A)** — no companion `.html` file. 999.7 backlog item plans to extract these to `.html` files; until then, every new sidebar follows the inline pattern.
- **Re-export footer in `Code.ts`** — every new trigger function must appear in both the import block (top) and the export block (bottom) of `Code.ts`; build.mjs's footer lifts them to top-level globals. Apps Script's menu system finds them by global name.

### Integration Points
- **`onOpen` menu** — `apps-script/src/triggers/onOpen.ts:7-29`. Insert "Manage Admins…" between line 22 ("Evict Guildie…") and line 23 ("Set Theme…"). Add a new sub-separator + "Initialize Admin Allowlist (manual)" after the existing migration items (around line 26).
- **`onOpen` lazy bootstrap** — top of `onOpen` (after the SpreadsheetApp.getUi() call, before the menu chain) calls `bootstrapGuildAdmins()`. Wrapped in try/catch + `log('warn', 'bootstrap_failed', { error })` — never throws out of `onOpen`.
- **`showEvictionSidebar` opener** — first statement after the `themeKey` const becomes:
  ```typescript
  const callerEmail = (Session.getEffectiveUser().getEmail() ?? '').toLowerCase().trim();
  if (!isAdmin(callerEmail)) {
    SpreadsheetApp.getUi().alert('Not authorized', 'Only guild officers can evict members. Contact a workbook admin if you think this is wrong.', SpreadsheetApp.getUi().ButtonSet.OK);
    return;
  }
  ```
- **Eviction callbacks (`getEvictionEmails`, `previewEviction`, `commitEviction`)** — each gains `requireAdminOrThrow(...)` as its first statement. No other changes to these callbacks (the existing 30-day-grace, lock-wrapped, JSON-envelope behavior is unchanged).
- **`installTriggers`** — UNCHANGED. The new admin-mgmt sidebar opens via menu only; no time-driven trigger. No `installTriggers` change means existing workbooks don't need to re-run "Install Triggers" to pick up Phase 7.
- **`Code.ts` re-exports** — add `showAdminMgmtSidebar`, `getAdminList`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdminsManual`. Import them from `'./triggers/showAdminMgmtSidebar'` and `'./lib/admin'`. Add to the export block.

</code_context>

<specifics>
## Specific Ideas

- **The "Manage Admins…" menu item label** — match the existing "…" pattern (every sidebar-opening item in the SquireBot menu ends with an ellipsis: "Set Character Info…", "Set Bank Coin…", "Search…", "Evict Guildie…", "Set Theme…"). Visual consistency.
- **The non-admin modal copy** — `"Only guild officers can evict members. Contact a workbook admin if you think this is wrong."` matches the existing v1.0 voice (terse, factual, suggests a next step). For admin-mgmt: `"Only guild officers can manage admins. Contact a workbook admin if you think this is wrong."`
- **The `(owner)` floor marker in the admin-mgmt sidebar** — display the floor email with a trailing ` (owner)` annotation. The owner can still remove themselves; the marker is just a visual cue that this row is special. Tooltip: `"This is the workbook owner. The owner-floor lockout protection prevents anyone else from removing this email."`
- **`_meta.admin_log` envelope shape** — mirrors `_meta.eviction_log` exactly: `{ at: ISO8601, action: 'add'|'remove'|'bootstrap'|'bootstrap_failed', email: string, initiated_by: string }`. Note `reason` is absent (eviction_log has `reason: 'evicted'` for future extensibility; admin_log doesn't need it yet).
- **Lowercase + trim normalization** — apply at THREE points: (1) on read from `_meta.guild_admins` (defensive — handles workbooks where an admin manually edited the cell), (2) on write (idempotent diffs), (3) on every comparison (`isAdmin` input, `requireAdminOrThrow` input, addAdmin/removeAdmin input). Centralize in a `normalizeEmail(s: string): string` helper inside `lib/admin.ts`.

</specifics>

<deferred>
## Deferred Ideas

| Idea | Where it goes |
|------|---------------|
| **Auto-remove evicted guildies from `_meta.guild_admins`** — hygiene: when `commitEviction(email)` runs, if `email` is also in `guild_admins`, remove it. Crosses the eviction → admin policy boundary; admins can do this manually via the admin-mgmt sidebar. | Backlog candidate; revisit after a few real evictions to see if anyone forgot. |
| **Dynamic per-user menu hiding** — `onOpen` calls `Session.getEffectiveUser().getEmail()` and only adds "Evict Guildie…" + "Manage Admins…" if the caller is an admin. Blocked by simple-trigger auth constraints (may return empty). | Defer to a polish phase IF non-admin guildies complain about seeing items they can't use. |
| **Workbook ownership transfer UX** — `_meta.workbook_owner_floor` is captured once at bootstrap and never auto-updated. New owner can manually edit the cell. | Defer indefinitely; not enough load to justify a UI. |
| **Email format validation beyond non-empty + `@` check** — full RFC 5322 regex; verify domain MX records; etc. Admins are trusted; garbage-in just means a useless allowlist entry they can remove. | Defer indefinitely. |
| **"Promote to owner-floor" UX** — let an admin change the `_meta.workbook_owner_floor` row from the sidebar (with confirmation modal). Currently the only way to change the floor is to hand-edit the cell. | v1.1 candidate if ownership-transfer happens in practice. |
| **Role-based admin tiers** — read-only admin vs. full admin vs. owner-floor. The current model is binary (admin or not). | v2 candidate; not needed for a 12-person guild. |
| **Admin-log retention/rotation** — `_meta.admin_log` grows unboundedly. At ~50 bytes per entry and a few admin actions per year, this is fine for ~years. `_meta.eviction_log` has the same property. | Backlog candidate; revisit when the cell value exceeds 50KB (the Apps Script `Range.setValue` warning threshold). |
| **Cross-workbook admin sync** — if/when SquireBot supports multiple workbooks per guild (it doesn't), admin lists would need to sync. Out of scope for v1.x. | v2 candidate. |
| **Self-eviction protection** — currently an admin can evict their own email (or be evicted by another admin). Edge case; nobody has hit it. | Defer indefinitely. |

</deferred>

<scope_changes>
## Scope changes during this discussion

None. The user delegated all four gray-area decisions to Claude with the directive "make whichever choice(s) that will make the end-user experience as simple as possible." All six locked decisions follow that brief directly:

- **D-01 (lazy onOpen bootstrap + manual fallback):** zero clicks for the 95% case; a single discoverable menu item for the consumer-account fallback.
- **D-02 (JSON array + separate floor row):** matches existing `_meta.eviction_log` pattern; readers don't need new mental model.
- **D-03 (Apps Script modal for non-admins):** one screen, one message, zero state to recover from.
- **D-04 (dedicated `showAdminMgmtSidebar` + new menu item):** consistent with every other v1.0 sidebar; admins discover via the same menu they already use.
- **D-05 (central `lib/admin.ts` module):** every admin-policy primitive in one place; future phases reuse rather than re-implement.
- **D-06 (fail-closed for auth, soft-fallback for audit-log):** the existing eviction-sidebar A5 fallback pattern extended carefully — audit logs survive sandbox quirks, but authorization decisions don't.

Phase 7 ships ADMIN-01 + ADMIN-02 + ADMIN-03 only. No watcher change. No schema bump. No new triggers (no `installTriggers` re-run required for existing workbooks).

</scope_changes>

<verification_hooks>
## Verification hooks (planner: these are the criteria the executor must satisfy)

From ROADMAP §57-62 (Phase 7 success criteria):

1. **`_meta.guild_admins` exists + contains workbook owner after first deploy; re-bootstrap is idempotent** — Unit-testable: `bootstrapGuildAdmins()` on an empty `_meta` writes the owner; second call no-ops (idempotent check returns early). Integration: `clasp push` the bundle, open the dev workbook, verify the `_meta` tab has both `guild_admins` and `workbook_owner_floor` rows with the expected values; close and reopen — no double-write.
2. **Non-admin opening `Evict Guildie…` sees "not authorized" modal; clicking the action is a safe no-op** — Unit-testable: `requireAdminOrThrow('nonadmin@example.com')` throws `'not_authorized'`. Integration: open the dev workbook as a non-admin Google account (or temporarily remove your email from `_meta.guild_admins`), click "Evict Guildie…", verify the modal appears and no `_char_owner.is_removed` flips happen and no `_meta.eviction_log` entry appears.
3. **Admin can open admin-mgmt UI and add another guildie; new admin's eviction calls succeed** — Unit-testable: `addAdmin('newadmin@example.com', 'existingadmin@example.com')` returns `{ added: true }`; subsequent `isAdmin('newadmin@example.com')` returns true. Integration: admin opens "Manage Admins…", types a new email, clicks Add; admin-mgmt sidebar reloads list; new admin opens the workbook (in a second browser session), opens "Evict Guildie…", verifies the sidebar opens normally and the email selector populates.
4. **Admin can remove another admin; owner-floor email cannot be removed by non-floor admins** — Unit-testable: `removeAdmin('alice@example.com', 'bob@example.com')` (where alice is admin, bob is admin, neither is floor) returns `{ removed: true }`. `removeAdmin(floor, 'bob@example.com')` (where bob is not floor) throws `'owner_floor_protected'`. `removeAdmin(floor, floor)` (self-removal of floor) succeeds. Integration: admin opens "Manage Admins…", verifies the owner-floor row has no "Remove" button (caller is non-floor), clicks Remove on a non-floor admin, verifies that admin's row disappears.
5. **`_meta.schema_version` stays at 3 and `WatcherMaxSchemaVersion` stays at 3** — Plain grep: `grep -n "schema_version" apps-script/src/lib/migrations.ts` shows no new bump. `grep -n "WatcherMaxSchemaVersion" internal/sheet/client.go` shows the constant is still `3`. No watcher rebuild for Phase 7.

Planner should structure plans so each success criterion maps to at least one plan with a clear ship gate.

</verification_hooks>

---

## Plan-phase entry signal

This phase is **ready for planning**. Suggested invocation:

```
/clear
/gsd-plan-phase 7 --skip-research
```

Research is optional — the patterns are all already in the codebase. The eviction sidebar is the reference implementation for the admin-mgmt sidebar; the eviction_log envelope is the reference for the admin_log envelope; the LockService pattern is the reference for every multi-step write. If the planner wants a research pass for `SpreadsheetApp.getUi().alert()` semantics under different consent screens, or `getOwner()` behavior under `drive.file` for consumer accounts, `/gsd-plan-phase 7` (no `--skip-research`) is fine — but the manual-fallback path (D-01) is designed to absorb any `getOwner()` weirdness, so research is more belt-and-suspenders than load-bearing.

Estimated plan count: **3-4 plans**.

- Plan 1: `lib/admin.ts` module + `__tests__/admin.test.ts` — `requireAdminOrThrow`, `isAdmin`, `addAdmin`, `removeAdmin`, `bootstrapGuildAdmins`, `getAdminList`, `appendAdminLogEntry`. The policy primitives, unit-tested in isolation. (ADMIN-01 partial, ADMIN-03 partial.)
- Plan 2: `triggers/showAdminMgmtSidebar.ts` + `__tests__/adminMgmtSidebar.test.ts` + `Code.ts` re-exports — the new sidebar, callback shapes, owner-floor server-side enforcement, audit-log writes. (ADMIN-03 full.)
- Plan 3: `triggers/showEvictionSidebar.ts` guard + `triggers/onOpen.ts` menu integration + `triggers/onOpen.ts` lazy bootstrap call — wire the policy module into the existing eviction sidebar, add the new menu items, call `bootstrapGuildAdmins()` from `onOpen`. (ADMIN-01 bootstrap, ADMIN-02 full.)
- Plan 4 (optional, planner's call to merge into 3): `clasp push` + dev-workbook smoke — push the bundle, verify the four success criteria interactively, append confirmation to STATE.md.

The planner may consolidate Plans 1+2 if the test surface is small enough, OR consolidate Plans 3+4 if the smoke is fast. Either is fine.

---

*Phase: 7-admin-allowlist-eviction-enforcement*
*Context gathered: 2026-05-11*
