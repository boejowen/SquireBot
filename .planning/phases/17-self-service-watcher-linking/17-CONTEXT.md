# Phase 17: Self-Service Watcher Linking (web feature) - Context

**Gathered:** 2026-06-01
**Status:** Ready for planning

<domain>
## Phase Boundary

A signed-in guildie can mint, view, and revoke their own watcher codes from squirebot.quest — the owner derived server-side from their Discord session — with **no maintainer involvement** and **no watcher change**. Backend (`internal/backendsrv/{webadmin,store,auth}`) + frontend (`web/`). Reuses the P15 Discord-login session gate, `auth.MintCode`, and the `web_user`/`owner`/`guild_code` tables.

**Locked by the v2.1 milestone scope (NOT re-litigated here — see REQUIREMENTS.md / STATE.md):**
- The link code **is** the reusable bearer token (v2.0 model): shown plaintext exactly once at mint, hash-only at rest, no expiry, watcher pastes-and-reuses forever.
- Re-link is **additive** — a new code never revokes existing ones (multi-PC works); revocation is **per-token**.
- The manual `mint-code` CLI is **removed**; the self-service endpoint is the only mint path. `revoke-code` CLI is **retained** as an ops backstop.
- **HARD CONSTRAINT:** Discord identity is captured at link-time (website) only — **never** Discord OAuth *in the watcher* (re-introduces the ~7-day-expiry / public-secret / loopback fragility v2.0 escaped; P13 made the watcher browser-free on purpose).

</domain>

<decisions>
## Implementation Decisions

### Identity linkage model (LINK-01 / LINK-02) — the load-bearing decision
The schema today has **no FK** between `owner` (keyed by free-text `label`) and `web_user` (keyed by `discord_user_id`). The eviction owner-floor path leans on a loose `owner.label == web_user.username` string match (the documented WR-05 fail-open). This phase creates the real link.

- **D-01:** Add a new **nullable, UNIQUE** column `owner.discord_user_id` (FK → `web_user.discord_user_id`). UNIQUE enforces **one owner per Discord identity**. (Schema mechanism is Claude's discretion within "nullable UNIQUE column on `owner`", not a join table.)
- **D-02:** A self-minted code's owner is derived **server-side from the Discord session** (`webauth.UserFromContext`) — never free-typed or client-supplied (the v1 `mint-code --owner <label>` free-text owner is gone).
- **D-03 (adopt-existing-owner):** On a guildie's first self-mint, **attach to their existing owner row** and stamp it with their `discord_user_id` — their existing characters/data stay theirs; subsequent codes attach to the same owner. Data continuity is preserved; we do NOT create a second/orphan identity for an already-present guildie.
- **D-04 (auto-match, new if none):** Resolve "which existing owner is theirs" by `owner.label == Discord username` (TRIM + COLLATE NOCASE, the existing bridge):
  - **Exactly one match** → stamp it with `discord_user_id`, mint against it.
  - **Zero matches** (brand-new guildie, or label ≠ Discord name) → **create a fresh owner** labeled with the Discord username, stamped with `discord_user_id`, mint against it. (Self-service works before the watcher's first upload.)
  - **2+ matches, OR a match already stamped with a *different* `discord_user_id`** → **refuse + log loudly**, never guess (no silent mis-adoption of someone else's data).
- **D-05 (eviction owner-floor rewired to the FK):** Update `callerMayNotEvictFloor` (and the owner-floor owner resolution) in `webadmin/eviction.go` to **prefer `owner.discord_user_id`** when present, falling back to the `owner.label == username` string bridge only for not-yet-linked owners. This closes the WR-05 fail-open for linked guildies as the direct payoff of LINK-02 (the requirement's stated "replacing the loose bridge" intent).

### Code list & identifiability (LINK-05)
- **D-06 (auto-label, no device-name field):** No "name this device" text input at mint time (the richer device-naming UX is explicitly deferred in REQUIREMENTS). A code is identified in the list by an **`#N` ordinal + created date** — plus last-seen (below), which does the "which PC is this?" work.
- **D-07 (per-code last-seen):** Add a new `guild_code.last_seen` column, **stamped on each authenticated ingest** (the bearer path resolves the `guild_code` row, so it knows which code uploaded). Surface it in the list ("last used 5 min ago") so a guildie can tell a dead PC's code from a live one before revoking.
- **D-08 (per-token revoke):** A guildie sees a list of **their own** active codes and can revoke **any single one** without affecting the others (sets `guild_code.disabled_at`; the revoked watcher stops uploading, the rest keep working). Scoped to the caller's owner — never another owner's codes.

### "Link your watcher" page (LINK-04 / LINK-05)
- **D-09 (route + nav):** New route **`/account`** (personal "your account / watcher codes" area), with a **top-nav entry visible to every logged-in member** (NOT officer-gated like `/admin`). Framed broadly so it can later host more self-service (theme pref, 999.5 self-eviction). Gate: `webauth.RequireSession` (any signed-in member), never `RequireOfficer`.
- **D-10 (single combined page):** One page: a **"Generate a new code"** action → the **show-once plaintext panel** (prominent, copy-to-clipboard, clear paste-into-watcher steps) appears inline; **below it the list** of the caller's active codes (`#N`, created, last-seen) each with a **Revoke** control. Reloading/revisiting never re-reveals a plaintext code (hash-only at rest — LINK-04).

### Claude's Discretion
- Schema detail of D-01 (column type/index), and how `guild_code.label` is populated for self-minted codes (currently it stored the owner label; may be left as owner label or null).
- Exact revoke-confirmation UX — **lean: confirm-before-commit** mirroring `EvictionForm`'s pattern, but the planner/UI-design step picks the final detail (instant-with-toast is acceptable given the additive/recoverable model).
- The show-once panel's exact copy and paste-instruction wording (align with the v2.0 watcher onboarding "paste your guild code" copy).
- Mechanics of removing the `mint-code` CLI dispatch in `cmd/squirebot-server/main.go` (the `runMint` func + the `case "mint-code"` switch arm) and its test.
- Whether `whoami-web`/session payload should also expose link status for the frontend — decide during planning if the page needs it.

</decisions>

<specifics>
## Specific Ideas

- The page should feel like a personal settings area (`/account`), not an admin tool — it's for **every** guildie, parallel to but distinct from the officer-only `/admin`.
- Last-seen is the chosen way to answer "which code is this dead PC?" — deliberately preferred over a manual device label.
- Mis-adoption safety is explicit: when the username↔label match is ambiguous or already taken by another Discord identity, **refuse and log** rather than silently attach (no guildie should ever adopt another's characters).

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & locked scope
- `.planning/REQUIREMENTS.md` — LINK-01..06 (the six requirements), the four locked v2.1 decisions, Out-of-Scope table, and the deferred "officer mint-on-behalf" / "device naming UX polish" items.
- `.planning/ROADMAP.md` §"Phase 17" — goal, Depends-on, the 6 Success Criteria (what must be TRUE).
- `.planning/STATE.md` §"Milestone v2.1 Scope (locked 2026-06-01)" — the additive / code-is-bearer-token / CLI-removed / HARD-CONSTRAINT decisions.

### Backend — reusable assets & the exact rewire targets
- `internal/backendsrv/auth/mint.go` — `MintCode(db, ownerLabel)`: token-gen shape (32B crypto/rand → base64 → sha256, hash-only, plaintext printed once). The self-service endpoint adapts this (owner derived from session, plaintext returned to the page not stdout — but NEVER logged, V7).
- `internal/backendsrv/migrations/00001_init.sql` — `owner` (label-keyed, no Discord link), `guild_code` (owner_id FK, token_hash UNIQUE, label, disabled_at), `character` (owner_id FK). The D-01 column + D-07 column are new forward-only migrations on top.
- `internal/backendsrv/migrations/00004_web_auth.sql` — `web_user` (discord_user_id PK), `web_session`, `guild_admins`, `app_config` (owner_floor), the generic `audit_log` columns. Migration convention: forward-only, never edit shipped files; new file `00005_*.sql`.
- `internal/backendsrv/webauth/session.go` — `RequireSession` (the login-only gate for `/account`), `UserFromContext` (server-side caller discord_user_id), cookie/session model.
- `internal/backendsrv/webadmin/eviction.go` — `callerMayNotEvictFloor` + `ownerLabelOf`: **the D-05 rewire target** (the WR-05 string-bridge fail-open to switch to the FK).
- `internal/backendsrv/webadmin/officers.go` — the canonical webadmin handler pattern to mirror: `caller(ctx)`, `withTx`, `AppendAuditTx`, `writeJSON`/`writeJSONError`, in-tx re-check, error mapping.
- `internal/backendsrv/store/` — `websession.go`, `admins.go`, `binding.go` (owner/char binding), `eviction.go` (the re-mint-on-restore + code-revoke logic touching `guild_code`). New store funcs (resolve-or-create owner by session, list/revoke own codes, stamp last-seen) live here.
- `internal/backendsrv/ingest/` — the bearer-auth ingest path that must stamp `guild_code.last_seen` (D-07).
- `cmd/squirebot-server/main.go` — route wiring (where the new `/api/v1/...` endpoints register, login-only via `RequireSession`); `runMint` + the `case "mint-code"` arm to **remove** (LINK-06); `runRevoke` to **keep**.

### Frontend — design system & patterns to mirror
- `web/src/routes/admin/+page.svelte` + `web/src/lib/components/AuthGate.svelte` — the session/officer gating pattern, `getContext(SESSION_KEY)`, two-layer authorization. `/account` is the login-only sibling (no officer check).
- `web/src/lib/components/EvictionForm.svelte` — the confirm-before-commit form pattern + inline error/state handling the revoke flow mirrors; `StateBlock` for empty/refusal states.
- `.planning/phases/14-web-frontend/14-UI-SPEC.md` — the EQ-aesthetic design system (themes, typography, form-card spacing) the `/account` page must conform to.
- `.planning/phases/15-admin-web-forms-login/15-UI-SPEC.md` — the officer-forms IA + form contracts; `/account` reuses these conventions for a non-officer audience.

### Prior-phase context (decisions that constrain this phase)
- `.planning/phases/15-admin-web-forms-login/15-CONTEXT.md` — P15 Discord login + session + owner-floor decisions (D-01..D-12) this phase builds on.
- `.planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md` — P11 mint/token + `guild_code` decisions (D-05/D-09).
- `docs/backend-deploy.md` — the live deploy env (systemd, Caddy, cookie domain, CORS origin) the new routes inherit.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `auth.MintCode` — the token-gen + hash-only persistence logic; adapt for a session-derived owner and a page-returned (never-logged) plaintext.
- `webauth.RequireSession` + `webauth.UserFromContext` — the login-only gate and server-side caller identity (the entire basis for "owner derived from session", D-02).
- `webadmin` handler pattern (`officers.go`): `caller(ctx)`, `withTx` (BEGIN IMMEDIATE), `AppendAuditTx`, `writeJSON`/`writeJSONError`, in-tx re-check — copy this shape for mint/list/revoke handlers.
- `EvictionForm.svelte` + `AuthGate.svelte` + `StateBlock.svelte` — frontend form/gating/empty-state primitives for the `/account` page.

### Established Patterns
- **Forward-only migrations** (goose), never edit shipped files; `_meta`-style version discipline. New `00005_*.sql` adds `owner.discord_user_id` + `guild_code.last_seen`.
- **Hash-only at rest / plaintext shown once / never slog the plaintext** (V6/V7) — extends from the watcher bearer token to the web-returned code.
- **Server is the authorization boundary** (D-01): the `/account` gate is `RequireSession`; the frontend nav/page checks are UX only.
- **Audit every web write** (`AppendAuditTx`) in the same tx — mint/revoke should audit (actor = caller discord_user_id).
- **Per-character non-overlapping writes / atomic tx** — ownership resolution and last-seen stamping must not race the ingest hot path.

### Integration Points
- `cmd/squirebot-server/main.go` route mux — new login-only endpoints (mint / list-own-codes / revoke-own-code) alongside the existing `/api/v1/coin` login-only block; remove the `mint-code` CLI arm.
- The bearer-auth ingest path — add the `guild_code.last_seen` stamp (D-07) without slowing the atomic replace.
- `webadmin/eviction.go` owner-floor resolution — switch to the new FK (D-05).
- The SvelteKit nav/shell — add the all-members `/account` entry (distinct from the officer-gated `/admin` entry).

</code_context>

<deferred>
## Deferred Ideas

- **Officer mint-on-behalf** — minting a code for a guildie who can't log in. Deferred (locked "self-service replaces the CLI entirely / no manual escape hatch"); revisit only if a real can't-log-in case appears.
- **Device naming UX polish** — rename codes, "this PC" auto-detection, richer per-code metadata. LINK-05 ships `#N`/created/last-seen identifiability; fancier device management is deferred.
- **999.5 Self-service eviction** — a departing guildie quitting cleanly without officer action. Adjacent to `/account` self-management (could later live there) but a separate threat-model; deferred.
- **999.12 / WANT-01..08 Wantlist + Discord pinger** — still deferred; this phase's Discord-tied ownership (the `owner.discord_user_id` FK) further pre-pays the per-user-identity prerequisite (the snowflake the pinger DMs).

</deferred>

---

*Phase: 17-self-service-watcher-linking*
*Context gathered: 2026-06-01*
