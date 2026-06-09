# Phase 26: Character Assignment - Context

**Gathered:** 2026-06-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Deliver the **character-assignment data layer + its member and officer surfaces** (ASSIGN-01..06): a member can see "My characters," self-claim/release characters, and request a contested one; an officer can assign/reassign and approve/deny requests; an officer can designate guildwide-shared "guild bank" / "guild bot" characters. All assignment changes are audited. Schema migration `00009` → **v9**.

**In scope:** the assignment data model (new tables/columns), the member claim/release/request API + "My characters" UI, the officer assign/reassign/approve-deny/designate API + admin-panel UI, the auto-seed migration, audit.
**Out of scope (later phases):** the "my characters" *inventory filter* (Phase 27, MYVIEW); the *wantlist* character tag (Phase 28, CWANT). Backend (`internal/backendsrv`) + web (`web/`) ONLY — the Go **watcher is untouched**.
</domain>

<decisions>
## Implementation Decisions

### Assignment model (ASSIGN-05 resolved)
- **D-01:** Normal characters have **exactly one assignee**. NOT many-to-many.
- **D-02:** Sharing is handled by an **officer-only character designation**, not multi-assignment: an officer ("admin/guild leader") can mark a character as a **guild bank** (stores guild money/items) or a **guild bot** (shared utility: rez, ports, etc.). Guild bank/bot characters are **shared guildwide, have NO assignee, and are not claimable** (they're exempt from the one-assignee rule). The user implied **multiple** bank/bot chars are allowed ("characters designated… as either a guild bank or a guild bot").

### Assignment vs. data-ownership
- **D-03:** Assignment is a **separate layer** — a NEW table keyed on the character. `character.owner_id` is LEFT ALONE (it stays the watcher-upload provenance). Ingest first-sighting binding, the eviction owner-floor, and `guild_code.owner_id` are therefore **unaffected** by claims. (Locked: claiming never re-homes `owner_id`.)

### Initial state at migration
- **D-04:** The `00009` migration **auto-seeds** the assignment table: each character whose `owner.discord_user_id` is non-NULL (linked via P17) is auto-assigned to that user. Legacy / NULL-owner characters start **unassigned**. No data loss; idempotent.
- **D-05:** Guild bank/bot designations are NOT auto-derived from data — officers set them. (See OPEN-2 for whether existing `is_bank_toon` chars pre-seed the guild-bank designation.)

### Claim / reassign policy (ASSIGN-02/03/04)
- **D-06:** Self-claim is **open and instant for an UNASSIGNED character** — any signed-in member can claim it (matches the guild's trust-rich, universal-visibility ethos).
- **D-07:** Claiming a character **already assigned to another member** files a **request** that an **officer approves or denies** (a small request/approval workflow — NOT a silent take, NOT a hard block). The requester sees pending/approved/denied status; can cancel a pending request.
- **D-08:** A member can **release/unclaim** a character they hold (returns it to unassigned).
- **D-09:** **Officers** can directly **assign / reassign / remove** any character's assignment at any time (no request needed), and **approve/deny** member requests, from the admin panel. Officers also **designate/clear** guild bank/bot status.

### Audit (ASSIGN-06)
- **D-10:** Every assignment change (self-claim, release, request create/cancel, officer assign/reassign/remove, request approve/deny, bank/bot designate) is recorded via the existing audit path (`AppendAuditTx`) with actor + character + action + time. V7 logging discipline (ids/action only, no PII beyond the discord_user_id already keyed).

### Claude's Discretion
- The exact table/column shapes, request-state machine encoding, and endpoint routing are the planner's to design (within D-01..D-10 + the OPEN items below).
- Discovery UX (pick-from-list vs search) for finding a claimable character is the planner/UI's call — a list of claimable (unassigned) characters is the obvious default.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone + phase scope
- `.planning/PROJECT.md` § "Current Milestone: v2.3" — locked decisions + scope.
- `.planning/REQUIREMENTS.md` § "v2.3 Requirements" — ASSIGN-01..06 (the requirements this phase delivers) + v2.3 Traceability.
- `.planning/ROADMAP.md` § "Phase 26 — Character Assignment" — goal + success criteria.

### Project rules
- `./CLAUDE.md` — schema evolution (extend-only, version-stamped, idempotent; `_meta.schema_version` write is the LAST migration step); consolidated-views LOCKED (don't regress); structured logging (slog Go side); officer-gate is server-truth.

### Existing code to build on (no external ADRs for this phase)
- `internal/backendsrv/migrations/00001_init.sql` — `owner`, `character` (note existing `is_bank_toon`, `is_hidden`, `is_removed`), `guild_code`.
- `internal/backendsrv/migrations/00005_self_service_linking.sql` — `owner.discord_user_id` FK→`web_user` (the link the auto-seed uses) + the partial-unique-index pattern (SQLite can't ALTER ADD UNIQUE).
- `internal/backendsrv/migrations/00006_wantlist.sql` — the CHECK-constraint + soft-delete + partial-unique-index conventions to mirror.
- `internal/backendsrv/webadmin/wantlist.go` — the member-CRUD endpoint precedent: session-derived caller (`caller(ctx)`, D-02 owner-from-session, never body), `withTx` + `AppendAuditTx`, typed store errors → HTTP codes, owner-scoped no-IDOR.
- `internal/backendsrv/webadmin/officers.go` / `audit.go` — `caller`, `nowUnix`, `writeJSON`/`writeJSONError`, `withTx`, `AppendAuditTx`; the `RequireOfficer` gate + `guild_admins` (officer endpoints) vs `RequireSession` (member endpoints).
- `internal/backendsrv/webadmin/charmeta.go` (+ `store.SetCharMetaTx`, `web/src/routes/char-meta/`) — precedent for editing character attributes; the single-bank-toon invariant lives here (P16 MD-01). **Reconcile guild-bank designation with this** (OPEN-2).
- `web/src/routes/account/` + `WatcherCodesPanel.svelte` — member self-service page rhythm (form-card, show-once, confirm-before-commit) to mirror for "My characters."
- `web/src/routes/admin/` + `MonitorAdminPanel.svelte` — officer admin-panel surface to extend for assign/reassign/approve-deny/designate.
- `web/src/lib/components/SettingsMenu.svelte` — the member nav already routes to /account, /char-meta; "My characters" likely lands beside them.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Member-CRUD spine:** `wantlist.go` is the closest template — copy its session-derived-owner + `withTx`+audit + typed-error + owner-scoped-no-IDOR shape for the member claim/release/request endpoints.
- **Officer spine:** the `RequireOfficer` gate + `guild_admins` + `eviction`/`officers` handlers are the template for the officer assign/reassign/approve-deny/designate endpoints.
- **Auto-seed source:** `owner.discord_user_id` (P17) already maps most characters → a Discord user; the migration backfills the assignment table from it.
- **Audit:** `AppendAuditTx` already exists — ASSIGN-06 is a compose, not new infra.
- **UI:** `WatcherCodesPanel` (member) + `MonitorAdminPanel` (officer) + `ConfirmDialog` are the reuse targets; node-only vitest pure-helper pattern for any logic.

### Established Patterns
- Schema migrations: goose, forward-only, version-stamped LAST; SQLite can't ALTER-ADD-UNIQUE → use a partial `CREATE UNIQUE INDEX` (00005 precedent).
- Owner/identity from the session, never the request body (D-02). Owner-scoped reads/writes (no IDOR; cross-actor = silent no-op).
- Web tests are node-only (no DOM) — browser-smoke the assign/claim UI before calling it verified.

### Integration Points
- New tables/columns via `00009` → schema v9 (single migration).
- New route group under the existing `webadmin` mux (member = `RequireSession`, officer = `RequireOfficer`).
- New web routes/components for "My characters" + the officer admin-panel section.
- The deploy path is the web build→tarball→atomic-swap + a backend redeploy (`scp` binary + `systemctl restart`; `goose.Up` applies `00009` on boot) — NOT a watcher release.
</code_context>

<specifics>
## Specific Ideas

- "Guild bank = stores money and items; guild bot = a specific shared utility (resurrecting dead characters, teleporting, etc.) — these don't need to be assigned to any SquireBot user because they're shared guildwide" (user, verbatim intent).
- Contested claim = a **request an officer approves**, mirroring a lightweight approval queue.
- Auto-seed so already-linked guildies see their characters under "My characters" on day one with zero manual claiming.
</specifics>

<deferred>
## Deferred Ideas

- **MYVIEW (Phase 27):** filtering the inventory views to "my characters" + single-char drill-down — depends on this phase's assignment data, but is its own phase.
- **CWANT (Phase 28):** tagging wantlist items to a character — depends on this phase.
- **OPEN-1 (for spec/plan):** final table shape — likely `character_assignment(character_id UNIQUE → assignee discord_user_id, assigned_at, assigned_by)` + `assignment_request(id, character_id, requester, current_assignee, status pending|approved|denied, created_at, resolved_at, resolved_by)` + a guild-bank/guild-bot designation (columns on `character` vs a small role table). Planner/researcher to finalize.
- **OPEN-2 (for spec/plan):** reconcile the new officer-only **guild bank** designation with the EXISTING `is_bank_toon` column — which is currently **member-settable via `/char-meta`** and drives the `bank` view + the **single-bank-toon invariant** (`compute.Bank`, P16 MD-01). Decide: does "guild bank" == `is_bank_toon` (move it officer-only, and likely **relax the single-bank invariant** since the user implied multiple guild banks)? Is "**guild bot**" a brand-new flag? Does designating bank/bot clear any existing assignment? This reconciliation is load-bearing — flag it for the researcher.
- **OPEN-3 (for spec/plan):** whether `is_bank_toon` stays member-settable on `/char-meta` or becomes officer-only under the new designation model.
</deferred>

---

*Phase: 26-character-assignment*
*Context gathered: 2026-06-08*
