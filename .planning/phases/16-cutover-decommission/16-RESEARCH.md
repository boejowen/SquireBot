# Phase 16: Cutover + Decommission - Research

**Researched:** 2026-05-31
**Domain:** Operational cutover (no backfill) + one small login-only web form + Google decommission
**Confidence:** HIGH (build target + flip mechanics verified against live code/releases; decommission steps CITED from Google docs)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
**Fresh-start vs backfill (CUTOVER-02)**
- **D-01 — No Sheet backfill. Clean break.** Nothing is read from the old Sheet; the Google-free SquireBot starts fresh on all guildie data. (finding 04 §4.1 Option C extended to *all* human data.)
- **D-02 — New char-metadata web form + backend write endpoint** for `class` / `level` / `race` / `is_bank_toon`. No other source for these; `gear_check`/`spell_check`/`bank` depend on them. Columns **already exist** in `00001_init.sql` — **no schema migration needed for storage**, just a `POST` handler + the form. ⚠️ **Flips P16's roadmap UI hint `no` → `yes`.**
- **D-03 — Form auth + semantics (bank-coin precedent, ADMIN-05 / D-12):** login-only, **any signed-in member** may set **any** character's metadata (no officer gate). Operates on characters that **already exist** (created by first watcher upload); **no pre-creation** of rows. `is_bank_toon` lives on the same form.
- **D-04 — Everything else self-heals or re-enters natively:** inventory/spellbook self-populate from each watcher's first upload; bank coin re-entered via the **existing P15 BankCoinForm**; old `_archive` does **not** carry over.

**Soak / go-live confidence (CUTOVER-01)**
- **D-05 — No formal soak window.** Backend live + verified since 2026-05-29; CUTOVER-01 satisfied by a brief maintainer confirmation that onboarded guildies are reporting in and their views look right, folded into the flip gate. Thinner safety margin accepted.

**The flip (CUTOVER-03)**
- **D-06 — Auto-update + Discord herding.** Publish the re-targeted P13 binary as a GitHub Release; DM each guildie their minted code + a heads-up about the one-time paste prompt; share the fresh installer link for dormant watchers.
- **D-07 — Per-guildie unique codes.** Mint ~12 via `squirebot-server mint-code --owner <label>` (plaintext printed once), distribute individually via Discord DM.
- **D-08 — No migration-percentage gate; proceed as soon as technically feasible** (binary published + ~12 codes minted + char-meta form live). Stragglers onboard whenever — decommission strands no one.
- **D-09 — Roll-forward only.** No fallback to preserve (the old Sheet write-path is already dead).

**Decommission (CUTOVER-04)**
- **D-10 — Disable the Apps Script enrichment triggers.** They very likely still fire on Google's infra, double-loading the wiki + PigParse APIs and burning quota. Teardown + good-API-citizen hygiene.
- **D-11 — Retire the Google OAuth client** (the asset CUTOVER-04 explicitly names).
- **D-12 — Abandon the Sheet in place.** No export, no delete, no read-only freeze.
- **D-13 — CUTOVER-04 satisfied as "no *live* Google machinery / no Google dependency remains."** Code-level Google-freedom already proven in P13. The **proof artifact** is a decommission checklist documenting each retired Google asset plus the existing code-level proofs.

### Claude's Discretion
- **Confirm the P13 re-targeted binary is actually *published as a GitHub Release*** (not just built locally). → **VERIFIED BELOW: it is NOT. This is the #1 plan task.**
- **Verify the Apps Script enrichment triggers are actually still running** before treating D-10 as a real action. → **VERIFIED BELOW: they run on Google's infra independent of the dark watchers; D-10 is a real action.**
- **Decide whether the char-meta form (D-02) warrants a `/gsd-ui-phase` pass** or a folded-in mini UI-SPEC. Small + strong analogs, so a light touch likely suffices — but the UI safety gate applies (config `ui_safety_gate: true`).
- **Suggested sequence:** build char-meta endpoint+form → deploy → mint ~12 codes → publish binary + announce → (brief reporting-in confirmation) → disable Apps Script triggers + retire OAuth client → write decommission checklist → milestone close.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. The "fresh start" and "abandon the Sheet" calls are *reductions* in scope (D-01/D-12), not deferrals.

**Explicitly REJECTED (do NOT research/plan):** any read of/import from the old Sheet (D-01); a formal soak window or parity-tooling (D-05); Sheet export or deletion (D-12); per-owner visibility tiers; inventory history; the v2 Wantlist/Discord-pinger.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description (reinterpreted per CONTEXT) | Research Support |
|----|----------------------------------------|------------------|
| **CUTOVER-01** | **No formal soak.** Brief "guildies reporting in on the backend" confirmation folded into the flip gate. | Backend live + verified since 2026-05-29 (`docs/backend-deploy.md` §6, STATE.md). No build work; a maintainer check + a SQL count of recently-seen characters (`SELECT name,last_seen FROM character WHERE last_seen IS NOT NULL`). |
| **CUTOVER-02** | **Fresh start, NO backfill.** ONE build: a login-only char-meta web form + backend `POST` endpoint for `class`/`level`/`race`/`is_bank_toon`. | Columns exist in `00001_init.sql` (no migration). Clone the `coin.go` handler + `BankCoinForm.svelte` pattern. Full wiring map in **Architecture Patterns** below. |
| **CUTOVER-03** | **Auto-update + Discord herding.** Publish re-targeted P13 binary as a GitHub Release; mint ~12 codes; proceed when technically feasible. | **The binary is NOT yet released** (verified). Flip rides `internal/update` (`minio/selfupdate` + GitHub `/releases/latest/download/latest.json`). `mint-code` CLI exists. Mechanics in **Flip Choreography** below. |
| **CUTOVER-04** | **Kill the live Google machinery** (disable Apps Script enrichment triggers + retire the Google OAuth client); abandon the Sheet in place. | Triggers run on Google's infra independent of watchers (CITED). Concrete decommission steps in **Decommission Runbook** below. Proof artifact = a checklist doc (eviction-runbook style). |
</phase_requirements>

## Summary

Phase 16 is an **operational cutover with exactly one small code build**. The reality reframing in CONTEXT.md holds up under verification: the guild is dark on the Sheet (Google-blocked since 2026-05-15), the backend is live + verified at `api.squirebot.quest`, and the re-targeted watcher already writes to it. The classic shadow-soak/backfill/parity dance is void — there is nothing live to corrupt and nothing irreplaceable to import.

**The single build target** is a login-only char-metadata form (`class`/`level`/`race`/`is_bank_toon`) plus its `POST` endpoint. This is a near-clone of the shipped bank-coin form: backend `internal/backendsrv/webadmin/coin.go` (handler) + `internal/backendsrv/store/coin.go` (store) + `web/src/lib/components/BankCoinForm.svelte` (UI) + the `api.ts postJSON` wrapper. The storage columns already exist (`00001_init.sql`, nullable, commented "set later / by backfill (P16)"), so **no migration is needed**. This form is load-bearing: `compute/gearcheck.go` skips characters with no `class` and adds the Iksar tier only when `race=="IKS"`; `compute/spellcheck.go` skips classless characters. Until the form populates these fields, `gear_check`/`spell_check` ship blank for every character — which is precisely why the user accepted this one piece of "more work."

**Three verification results the planner must act on:**
1. **The re-targeted P13 binary is NOT published as a GitHub Release.** The latest release is `v1.0.2` (2026-05-13) carrying the OLD 16.99 MB Google-Sheets binary and a `latest.json` pointing at `v1.0.2`. No v2.x tag, no draft, no prerelease exists. Until a newer non-prerelease tag (e.g. `v2.0.0`) is pushed, still-running v1.0.2 watchers fetch v1.0.2's manifest, compute `IsNewer("1.0.2","1.0.2")==false`, and never update. **Publishing the binary is the #1 plan task and the literal trigger for the whole flip.**
2. **The Apps Script enrichment triggers are almost certainly still firing.** They are time-driven on Google's infrastructure, independent of the (dark) watchers and of the OAuth client; `installTriggers.ts` created 7 of them (incl. daily PigParse + 3 weekly wiki). They keep double-loading the community wiki + PigParse APIs in parallel with the backend. D-10 is a real teardown action, not a hypothetical.
3. **Both Google decommission operations are simple, maintainer-run, console actions** (delete the OAuth client; delete the project triggers) — documented below with citations.

**Primary recommendation:** Build the char-meta form as a strict clone of the bank-coin trio (login-only `RequireSession`, validate against the canonical `enrich.CLASSES`/`apps-script RACES` value sets, `level` 1–60, write under one audited tx). Then run the operational sequence: deploy → mint ~12 codes → **push a `v2.0.0` tag to fire `release.yml`** (publishing the bare binary + a fresh `latest.json`) → Discord-herd → brief reporting-in confirmation → delete the Apps Script triggers + the Google OAuth client → write the decommission checklist → close the milestone.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Char-meta write (`class`/`level`/`race`/`is_bank_toon`) | API / Backend (`webadmin` + `store`) | — | Authorization + validation + audit belong on the server (D-03 says login-only; the SERVER is the gate, the form only gates UX — exactly the P15 D-01 posture). |
| Char-meta form (pick a char, set fields) | Frontend (SvelteKit `web/`) | — | A thin renderer over the `postJSON` API + pure validation helpers; mirrors `/bank-coin`. |
| Char-meta value-set validation (valid classes/races) | API / Backend | Frontend (UX) | Server re-validates (defense-in-depth); the form's `<select>` options are UX only (the CR-01/T-15-29 lesson — never trust the disabled-button UX). |
| Watcher self-update flip | Watcher (`internal/update`) + GitHub Releases (CDN) | Discord (out-of-band herding) | The flip is a published-artifact event: `minio/selfupdate` pulls `/releases/latest/download/latest.json`. No backend change. |
| Guild-code minting | API / Backend CLI (`squirebot-server mint-code`) | Discord (DM distribution) | Plaintext printed once on the box; distributed out-of-band. |
| Apps Script trigger teardown | External: Google Apps Script console (maintainer) | — | Lives in the Apps Script project on Google's infra, outside this repo. |
| Google OAuth client retirement | External: Google Cloud Console (maintainer) | — | A Cloud project asset, outside this repo. |
| Decommission proof artifact | Repo docs (`docs/`) | — | A checklist markdown; the CUTOVER-04 evidence. |

## Standard Stack

This phase introduces **NO new dependencies**. Everything rides proven, in-repo infrastructure. The "stack" here is the set of existing components the build + ops reuse.

### Core (existing — reuse wholesale)
| Component | Where | Purpose in P16 | Why standard |
|-----------|-------|----------------|--------------|
| `webadmin` package | `internal/backendsrv/webadmin/` | The new char-meta `POST` handler clones `coin.go` here | Established login-only write-handler pattern (method-check → decode → validate → one audited tx → JSON reply) [VERIFIED: codebase] |
| `webauth.RequireSession` | `internal/backendsrv/webauth/session.go` | The login-only route gate (D-03) | The exact gate bank-coin uses; `caller(ctx)` reads the acting discord id for the audit row [VERIFIED: codebase] |
| `store` coin/char methods | `internal/backendsrv/store/` | New `SetCharMetaTx` clones `SetCoinTx`; `CharsWithMeta` already lists pick-able chars | Parameterized `?` placeholders, authorize-under-tx, `*sql.Tx`-composable [VERIFIED: codebase] |
| `webadmin.withTx` + `AppendAuditTx` | `internal/backendsrv/webadmin/audit.go` | Atomic write + audit row in one `BEGIN IMMEDIATE` tx | The established write-handler tx idiom (deferred-rollback guard) [VERIFIED: codebase] |
| `enrich.CLASSES` | `internal/backendsrv/enrich/eqconst.go` | The 14-class server-side validation set | Canonical Go constant, dependency-free [VERIFIED: codebase] |
| `BankCoinForm.svelte` + `FormField.svelte` | `web/src/lib/components/` | The char-meta form clones this shape | Login-only, pick-a-char, validated, credentialed POST, no ConfirmDialog [VERIFIED: codebase] |
| `api.ts postJSON` + `getJSON` | `web/src/lib/api.ts` | Typed credentialed fetch + the `Unauthenticated`/`Forbidden` mapping | The pinned client contract; new `fetchCharsForMeta`/`saveCharMeta` wrappers sit beside `fetchBankToons`/`saveCoin` [VERIFIED: codebase] |
| `internal/update` (`minio/selfupdate`) | watcher | The auto-update flip transport (CUTOVER-03) | Already proven; 999.22 SemVer pre-release compare de-risks it [VERIFIED: codebase] |
| `squirebot-server mint-code` | `cmd/squirebot-server/main.go` | Mint ~12 guild codes (D-07) | Existing CLI subcommand; prints plaintext ONCE [VERIFIED: codebase] |
| `release.yml` | `.github/workflows/release.yml` | Publishes the bare binary + `latest.json` on a `v*` tag | Already re-targeted for v2.0 (Google secrets stripped, guild-code dialog mentioned) [VERIFIED: codebase] |

### Supporting (existing — reference, no change)
| Component | Where | Why relevant |
|-----------|-------|--------------|
| `compute/gearcheck.go` + `spellcheck.go` + `bank.go` | `internal/backendsrv/compute/` | The CONSUMERS of the form's output; they already work — they just need `class`/`race`/`is_bank_toon` populated. NO change needed. [VERIFIED: codebase] |
| `store.CharsWithMeta` | `internal/backendsrv/store/readviews.go` | Already returns `id,name,class,level,race` for every non-removed char — the natural pick-list + pre-fill source for the form. [VERIFIED: codebase] |
| `apps-script RACES` | `apps-script/src/lib/eq-constants.ts:44-47` | The 14-race validation set (NOT yet ported to Go — see Open Questions). [VERIFIED: codebase] |
| `internal/onboarding` + `internal/credstore` | watcher | The "paste guild code" first-run flow a flipped watcher runs (DPAPI store, native dialog, `/whoami` validate). NO change needed. [VERIFIED: codebase] |
| `docs/backend-deploy.md` | repo | The deploy runbook (cross-compile → scp → restart; the apex `file_server` Caddy block for the frontend bundle; `mint-code` on the box). [VERIFIED: codebase] |
| `docs/eviction-runbook.md` | repo | The style/precedent for the CUTOVER-04 decommission checklist. [VERIFIED: codebase] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Cloning `coin.go` for char-meta | A generic "set any character column" endpoint | Rejected by analogy — the codebase favors small, explicit, audited handlers per write surface (coin/eviction/officer are each separate). A generic mutator would be a security smell and break the audit-event naming convention. |
| Officer-only char-meta gate | `RequireOfficer` | Rejected by D-03 (locked): non-sensitive shared data, trust-rich guild, and no Discord↔guild-code ownership link exists. Use `RequireSession`. |
| A `/gsd-ui-phase` pass for the form | Folded-in mini UI-SPEC | Discretion (CONTEXT). The form is tiny with a near-identical shipped analog; a light touch likely suffices, but the `ui_safety_gate` still applies. |
| New migration for char-meta storage | — | Rejected — columns already exist (`00001_init.sql`). A new migration would be churn for zero schema change. |

**Installation:** none. `go build ./...` (backend) + `cd web && npm run build` (frontend) use the existing toolchain. No `npm install`/`go get`.

**Version verification (the ONE that matters):** the re-targeted binary's release version. The shipped watcher default is `Version = "0.1.0-dev"` (`cmd/squirebot/build_constants.go`), overridden at release time by the pushed tag. The latest published release is **`v1.0.2`** [VERIFIED: `gh release list` 2026-05-31]. The flip requires pushing a tag whose SemVer is **strictly newer than every guildie's running version** — practically `v2.0.0` (so `IsNewer("1.0.2","2.0.0")==true` for any current watcher, and the new release becomes GitHub's "Latest", which `ManifestURL` resolves).

## Architecture Patterns

### System Architecture Diagram

```
                       ┌──────────────────────── THE ONE BUILD (CUTOVER-02) ────────────────────────┐
                       │                                                                              │
  Browser (member,     │   POST /api/v1/char/meta {character_id,class,level,race,is_bank_toon}        │
  logged in via         ──┐                                                                            │
  Discord, P15 AuthGate) │  GET /api/v1/char/meta-list  (pick-list + pre-fill: CharsWithMeta)          │
                         │                                                                              │
                         ▼                                                                              │
              ┌─────────────────────┐   credentialed (sb_session cookie, CORS-creds)                   │
              │  web/ CharMetaForm   │ ── postJSON / getJSON (api.ts) ──┐                               │
              │ (clone BankCoinForm) │                                   │                              │
              └─────────────────────┘                                   ▼                              │
                                                          ┌──────────────────────────────┐            │
                                                          │ webauth.RequireSession (gate) │            │
                                                          │  → caller(ctx)=discord_user_id │            │
                                                          └───────────────┬────────────────┘           │
                                                                          ▼                             │
                                              ┌────────────────────────────────────────────┐           │
                                              │ webadmin.CharMetaSetHandler (clone coin.go) │           │
                                              │  validate class∈CLASSES, race∈RACES,         │          │
                                              │  level 1..60, is_bank_toon bool              │          │
                                              │  withTx{ store.SetCharMetaTx + AppendAuditTx}│           │
                                              └───────────────┬──────────────────────────────┘          │
                                                              ▼                                          │
                                                  ┌────────────────────────┐                            │
                                                  │ character table (cols   │ ◀── columns ALREADY exist  │
                                                  │ class/level/race/       │     (00001_init.sql)       │
                                                  │ is_bank_toon)           │                            │
                                                  └───────────┬─────────────┘                            │
                       └──────────────────────────────────────┼──────────────────────────────────────┘
                                                              │ (read path — ALREADY BUILT, unchanged)
                                                              ▼
                       compute.gearcheck (skips classless; Iksar tier iff race=="IKS")
                       compute.spellcheck (skips classless)        →  gear_check / spell_check views
                       compute.bank / readviews (is_bank_toon=1)   →  bank view
                       (these go from BLANK → populated once the form runs)

  ════════════════════════ THE OPS CUTOVER (CUTOVER-01/03/04 — no build) ════════════════════════

  CUTOVER-03 flip:
     dev box ── git tag v2.0.0 ──▶ GitHub Actions release.yml ──▶ GitHub Release v2.0.0
                                       (builds bare squirebot.exe + latest.json)        │
                                                                                         ▼
     running v1.0.2 watcher ──(24h or manual)──▶ GET /releases/latest/download/latest.json
                                       IsNewer("1.0.2","2.0.0")==true ─▶ download+SHA256 verify
                                       ─▶ stage .new ─▶ next launch update.Apply() swaps ─▶ re-exec
                                       ─▶ MigrateFromV1 (drop stale Google wincred) ─▶ PromptGuildCode
                                       ─▶ /whoami validate ─▶ credstore.Store (DPAPI) ─▶ ingest to backend
     (dormant/uninstalled watcher: fresh installer link from the Release, then paste code)

     maintainer ── squirebot-server mint-code --owner "<guildie>" ──▶ plaintext ONCE ──▶ Discord DM (x12)

  CUTOVER-04 decommission (maintainer, external consoles):
     Google Apps Script console  ──▶ delete the 7 time-driven triggers  (stops the parallel wiki/PigParse load)
     Google Cloud Console        ──▶ delete the OAuth 2.0 Client ID      (the named CUTOVER-04 asset)
     (Sheet: ABANDON IN PLACE — no export, no delete — D-12)
     ──▶ write docs/decommission-checklist.md  (the proof artifact, D-13)
```

### Recommended file touch map
```
internal/backendsrv/
├── store/charmeta.go         # NEW: SetCharMetaTx (clone SetCoinTx) + (optional) CharMetaListItem
│                             #      reuse store.CharsWithMeta for the pick-list/pre-fill
├── webadmin/charmeta.go      # NEW: CharMetaSetHandler + CharMetaListHandler (clone coin.go shape)
└── (no migration file — columns exist)

cmd/squirebot-server/main.go  # EDIT: register the 2 routes under RequireSession,
                              #       beside the existing `POST /api/v1/coin` lines

web/src/lib/
├── charmeta.ts               # NEW: pure validation helpers (clone coin.ts shape) — node-testable
├── api.ts                    # EDIT: add CharMeta interfaces + fetchCharsForMeta/saveCharMeta wrappers
└── components/CharMetaForm.svelte  # NEW: clone BankCoinForm.svelte

web/src/routes/
└── char-meta/+page.svelte    # NEW: clone bank-coin/+page.svelte (member-accessible, no officer check)

docs/
└── decommission-checklist.md # NEW: the CUTOVER-04 proof artifact (eviction-runbook style)
```

### Pattern 1: Login-only audited write handler (clone of `coin.go`)
**What:** Method-check → JSON decode (+ trivial guard) → server-side range/value validation → one `withTx` composing the store mutator + `AppendAuditTx` → typed JSON reply. The acting `discord_user_id` is recorded for audit ONLY, never as an authorization input (D-03: any member may write).
**When to use:** the char-meta `POST`. This is the single most load-bearing pattern to copy verbatim.
**Example (the template to clone — verbatim from the shipped bank-coin handler):**
```go
// Source: internal/backendsrv/webadmin/coin.go (CoinSetHandler) — VERIFIED in repo
func CoinSetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
		ctx := r.Context()
		var req coinReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CharacterID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid_input"); return
		}
		if !validCoin(req) { writeJSONError(w, http.StatusBadRequest, "invalid_input"); return }
		writer := caller(ctx)   // acting discord id — AUDIT ONLY (D-12: any member may write)
		now := nowUnix()
		err := withTx(ctx, db, func(tx *sql.Tx) error {
			if e := store.SetCoinTx(ctx, tx, req.CharacterID, req.Plat, req.Gold, req.Silver, req.Copper); e != nil { return e }
			return AppendAuditTx(ctx, tx, "coin_set", writer, map[string]any{"character_id": req.CharacterID}, now)
		})
		if err != nil {
			if errors.Is(err, store.ErrNotBankToon) { writeJSONError(w, http.StatusBadRequest, "not_bank_toon"); return }
			slog.Error("coin set failed", "character_id", req.CharacterID, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal"); return
		}
		writeJSON(w, map[string]any{ /* echo name + saved values */ })
	}
}
```
For char-meta: rename to `CharMetaSetHandler`, swap `coinReq`→`charMetaReq {CharacterID, Class, Level, Race, IsBankToon}`, swap `validCoin`→`validCharMeta` (class∈`enrich.CLASSES`, race∈RACES, level 1–60, is_bank_toon is a bool so the JSON decoder validates it), swap `SetCoinTx`→`SetCharMetaTx`, audit event `"char_meta_set"`.

### Pattern 2: Bank-toon-gated store mutator → char-meta store mutator
**What:** A `*sql.Tx`-composable mutator with parameterized `?` placeholders. `SetCoinTx` first SELECTs a guard column then UPDATEs. `SetCharMetaTx` is simpler — it just UPDATEs the four columns by id (no bank-toon gate; setting `is_bank_toon` IS one of the columns).
**Example:**
```go
// Source: internal/backendsrv/store/coin.go (SetCoinTx) — VERIFIED in repo
func SetCoinTx(ctx context.Context, tx *sql.Tx, characterID, plat, gold, silver, copper int64) error {
	var isBank int
	err := tx.QueryRowContext(ctx, `SELECT is_bank_toon FROM character WHERE id = ?`, characterID).Scan(&isBank)
	// ... ErrNotBankToon gate ...
	_, err = tx.ExecContext(ctx, `UPDATE character SET plat=?,gold=?,silver=?,copper=? WHERE id=?`, plat,gold,silver,copper,characterID)
	return err
}
```
The char-meta twin: `UPDATE character SET class=?, level=?, race=?, is_bank_toon=? WHERE id=? AND is_removed=0` (mirror the `is_removed=0` scoping `CharsWithMeta`/`ListBankToons` use). Consider returning a not-found error when `RowsAffected()==0` (mirrors `ErrNotBankToon`'s fail-closed shape) so the handler can map it to `invalid_input`.

### Pattern 3: Login-only Svelte form (clone of `BankCoinForm.svelte`)
**What:** `onMount` fetch the pick-list → `<select>` a character → pre-fill fields from the chosen char → pure validation helpers → Save (disabled until valid AND changed) → success keeps the select. NO `ConfirmDialog` (non-destructive). A mid-session 401 hands off to the whole-site `AuthGate` via `authGuard(err)`.
**Critical pitfall baked into the template (CR-01):** the four coin inputs use `type="text" inputmode="numeric"`, **NOT** `type="number"`. Svelte 5's `bind:value` on a number input coerces the written-back value through `to_number()` (→ `number|null`), which crashed the node-blind suite when a helper called `.trim()`. The char-meta `level` field MUST follow the same rule. See `web/src/lib/components/BankCoinForm.svelte:160-177` and `web/src/lib/coin.ts:rawToTrimmed`.
**Value-set fields:** `class` and `race` should be `<select>` dropdowns sourced from a shared option list (mirror the server's `CLASSES`/`RACES`), `is_bank_toon` a checkbox, `level` the text+numeric input above.

### Pattern 4: Route page (clone of `bank-coin/+page.svelte`)
**What:** A member-accessible route — NO officer check, just the layout `AuthGate`. Contrast `/admin/+page.svelte` (Layer-1 officer refusal). The char-meta page mirrors `/bank-coin`: a `--panel` card under a Heading-20px title wrapping `<CharMetaForm />`.
**Example:** `web/src/routes/bank-coin/+page.svelte` (10 lines of logic; no `getContext` officer gate). Wire the route into the shell nav alongside "Record bank coin."

### Pattern 5: Route registration (in `main.go`)
**What:** Register both new routes under `webauth.RequireSession` (login-only, D-03), beside the existing coin routes. The whole mux is CORS-wrapped once.
**Example:**
```go
// Source: cmd/squirebot-server/main.go — the existing login-only block to extend
mux.Handle("GET /api/v1/coin/bank-toons", webauth.RequireSession(db, webadmin.BankToonsHandler(db)))
mux.Handle("POST /api/v1/coin",           webauth.RequireSession(db, webadmin.CoinSetHandler(db)))
// ADD (login-only, D-03):
mux.Handle("GET /api/v1/char/meta-list",  webauth.RequireSession(db, webadmin.CharMetaListHandler(db)))
mux.Handle("POST /api/v1/char/meta",      webauth.RequireSession(db, webadmin.CharMetaSetHandler(db)))
```
(Route names are a planning choice; `/api/v1/char/meta` + `/api/v1/char/meta-list` are suggested. Keep them under `/api/v1/` and login-only.)

### Anti-Patterns to Avoid
- **type="number" on the level input.** The CR-01 crash. Use `type="text" inputmode="numeric" pattern="[0-9]*"`.
- **Trusting the form for authorization or validity.** The SERVER re-validates (T-15-29). The `<select>` options + disabled Save are UX only.
- **A new migration for storage.** The columns exist. Adding one is churn.
- **An officer gate on char-meta.** D-03 locks it login-only.
- **Pre-creating character rows from the form.** D-03: the form only edits chars that already exist (created by their first watcher upload). No "add a character" affordance.
- **Reading anything from the Sheet.** D-01 forbids it. There is no Sheets client in the backend and none should be added.
- **Pushing a prerelease tag (`-rcN`) for the flip.** `release.yml` auto-marks any tag containing `-` as a prerelease, and GitHub's `/releases/latest/` ignores prereleases — a running watcher would never see it. Use a clean `v2.0.0`.
- **Deleting/exporting the Sheet.** D-12: abandon in place.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic write + audit row | A bespoke tx + manual rollback | `webadmin.withTx` + `AppendAuditTx` | The deferred-rollback-on-panic guard (WR-03) is subtle; reuse it. [VERIFIED: webadmin/audit.go] |
| Credentialed fetch + typed auth errors | A raw `fetch` in the form | `api.ts postJSON`/`getJSON` | Carries `credentials:'include'`, the 401→`Unauthenticated`/403→`Forbidden(code)` mapping, and the malformed-JSON guard. [VERIFIED: api.ts] |
| Form field validation | Inline `.svelte` logic | A pure `charmeta.ts` (clone `coin.ts`) | The repo's tests are node-only (no DOM); pure helpers are the ONLY way to unit-test the contract. [VERIFIED: coin.ts + web-tests-node-only memory] |
| The pick-list of characters | A new "list all chars" query | `store.CharsWithMeta` | Already returns `id,name,class,level,race` for non-removed chars — exactly the pick-list + pre-fill. [VERIFIED: readviews.go] |
| Class/race validity | A typed-in free-text field | `enrich.CLASSES` (server) + a ported RACES set | Canonical value sets exist; free text would corrupt the gear/spell joins. [VERIFIED: eqconst.go] |
| The watcher auto-update flip | A new updater / manual binary push | `internal/update` + `git tag v2.0.0` → `release.yml` | The whole `minio/selfupdate` + manifest + SemVer-compare path is built + proven. Just publish the artifact. [VERIFIED: internal/update + release.yml] |
| Minting guild codes | A new code generator | `squirebot-server mint-code --owner <label>` | Existing CLI; prints plaintext once, stores only the SHA-256 hash. [VERIFIED: main.go] |
| Disabling Apps Script triggers | A code change to `installTriggers.ts` | The Apps Script Triggers console (delete) | A maintainer console action; no repo change needed (the project is being abandoned). [CITED: developers.google.com] |

**Key insight:** P16's build surface is a single clone of an already-shipped, code-reviewed, regression-covered form. The risk is NOT in writing code — it is in (a) remembering the CR-01 input-type lesson so the new `level` field doesn't crash the node-blind tests, and (b) the *ops* sequencing (publish the binary BEFORE herding; disable the triggers as a real teardown step). Treat the build as low-risk-by-precedent and spend the planning attention on the cutover choreography + the decommission checklist.

## Runtime State Inventory

> P16 is a cutover/decommission phase, so this section is required. The canonical question: *after the build is deployed and the flip fires, what runtime systems still carry old/Google state?*

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| **Stored data** | Backend SQLite `character` rows: `class`/`level`/`race`/`is_bank_toon` are NULL/0 for every char (no backfill, D-01) — the form populates them going forward. The OLD Google Sheet still holds the only copy of the human-supplied metadata + `_archive` history — **deliberately abandoned** (D-01/D-12). | **Code edit** (the form) for go-forward data. **No data migration** (fresh start). |
| **Live service config** | (1) **Apps Script time-driven triggers (7)** still firing on Google's infra — daily PigParse + 3 weekly wiki + onChange + hourly view + watchdogs — independent of the dark watchers. (2) The **Google OAuth 2.0 Client ID** (the v1.0.2 desktop client) still exists in the Cloud project. (3) The **Google Sheet** itself (data + Drive ACLs) — abandoned in place. | **Maintainer console action:** delete the 7 triggers (CUTOVER-04 / D-10) + delete the OAuth client (D-11). Sheet: **no action** (D-12). |
| **OS-registered state** | Each guildie's Windows watcher: a `HKCU\...\Run` autostart entry (unchanged across the update) + a DPAPI wincred entry. WATCH-11's `MigrateFromV1` already deletes the **stale Google** wincred (`SquireBot:<google-email>`) on first launch of the re-targeted binary; the new bearer-token entry is written by onboarding. | **None new** — handled by the already-shipped P13 `MigrateFromV1` + onboarding when the flip lands. Verified by `git log`/13-03. |
| **Secrets / env vars** | Backend systemd `EnvironmentFile` (`/etc/squirebot/squirebot.env`): `DISCORD_*` + origins (P15, unchanged). The backend handles **no Google secret** (P13 stripped it). The R2 backup token (unchanged). The minted guild codes (~12) — stored hashed; plaintext is transient (printed once, DM'd). | **None** for char-meta (it adds no secret). **New ops act:** mint ~12 codes (D-07) — transient secrets distributed out-of-band. |
| **Build artifacts / published binaries** | **The published GitHub Release `v1.0.2` carries the OLD 16.99 MB Google-Sheets binary + a `latest.json` pinned to v1.0.2.** No v2.x release exists. Running v1.0.2 watchers will NOT auto-update until a newer non-prerelease tag is published. | **Plan task (the #1):** push `v2.0.0` (or similar) to fire `release.yml`, publishing the re-targeted 7.07 MB binary + a fresh `latest.json`. This IS the flip trigger (CUTOVER-03). |

**Nothing found in category that requires more than the above** — explicitly: there is **no n8n/Datadog/Tailscale/Cloudflare-Tunnel/Task-Scheduler** state for this project (it's a Go binary + SvelteKit static + a Hetzner VPS + GitHub Releases). The only "live service config" outside the repo is the two Google assets (triggers + OAuth client) and the abandoned Sheet.

## Common Pitfalls

### Pitfall 1: Assuming the re-targeted binary is the live update target
**What goes wrong:** The plan treats "publish the binary" as already-done and jumps to herding; watchers never update because GitHub's "Latest" is still v1.0.2.
**Why it happens:** P13/14/15 are "complete" and the frontend is deployed live, creating the impression the watcher binary is too. But the deploy that happened is the *backend server* + *frontend bundle* on the VPS — NOT a GitHub Release of the watcher.
**How to avoid:** Make "push `v2.0.0` and confirm the Release published the bare `squirebot.exe` + a `latest.json` with `version: 2.0.0` and a `binary_url` pointing at the v2.0.0 asset" an explicit, verifiable plan task. Then confirm `IsNewer` semantics: `/releases/latest/download/latest.json` resolves to the newest **non-prerelease** release.
**Warning signs:** `gh release list` top entry is `v1.0.2`; `gh release download v1.0.2 -p latest.json -O -` shows `"version":"1.0.2"`. [VERIFIED both, 2026-05-31.]

### Pitfall 2: The `level` input re-introduces the CR-01 crash
**What goes wrong:** Using `type="number"` for `level` makes Svelte 5 write `number|null` back into a string-typed binding; a pure helper calling `.trim()` throws — and the node-only test suite is blind to it (165 green tests + 2 crashing blockers was the P15 reality).
**Why it happens:** `type="number"` is the "obvious" choice for a numeric field; the coercion is non-obvious; the tests can't see DOM behavior.
**How to avoid:** Copy `BankCoinForm`'s input pattern verbatim — `type="text" inputmode="numeric" pattern="[0-9]*"` — and the `rawToTrimmed` normalization in the validation helper. **Code-review + browser-smoke the form before calling it verified** (per the `web-tests-node-only-blind-to-dom` memory). Live smoke is deferred-to-deploy in this repo's convention, but the form MUST be eyeballed in a browser.
**Warning signs:** a `<input type="number">` in the new form; a helper that calls `.trim()` on a `bind:value` without a `string|number|null` normalizer.

### Pitfall 3: Treating Apps Script triggers as already-dead
**What goes wrong:** D-10 is skipped because "the watchers are dark, so nothing's running." But time-driven Apps Script triggers fire on Google's schedulers regardless of watcher state — the daily PigParse + weekly wiki scrapes keep hitting the community APIs, doubling the polite-fetch load the backend now also generates.
**Why it happens:** Conflating "watchers can't write to the Sheet (OAuth-blocked)" with "the Sheet's server-side automation stopped." They are independent — the OAuth block walls the *watcher's* writes, not the *script's* time-driven triggers (which use the script owner's authorization, already granted).
**How to avoid:** Make "delete the 7 project triggers" a concrete CUTOVER-04 step with a verification (the Apps Script Executions log shows no further scheduled runs). This is both teardown and `politeFetch`-ethos hygiene (don't double-load the volunteer-run wiki/PigParse).
**Warning signs:** the Apps Script project's Executions dashboard still shows recent `refreshPigparse`/`refreshWikiItems` runs.

### Pitfall 4: A stray prerelease tag silently breaks the flip
**What goes wrong:** Tagging `v2.0.0-rc1` to "test the release pipeline" publishes a **prerelease** (`release.yml` auto-marks any `-`-containing tag), which GitHub's `/releases/latest/` ignores — no watcher sees it — but it *looks* like the binary shipped.
**Why it happens:** The natural instinct to rc a big release; the prerelease/latest interaction is non-obvious.
**How to avoid:** For the actual flip, push a clean `v2.0.0`. If a dry-run is wanted, use `workflow_dispatch` (builds artifacts without creating a Release) or accept that an rc won't be the auto-update target. The 999.22 SemVer compare correctly ranks a final above its own rc, so even a running rc watcher updates to the final.
**Warning signs:** the only v2 release is marked "Pre-release"; `gh release view` shows `isPrerelease: true`.

### Pitfall 5: Char-meta value sets drift from the wiki join keys
**What goes wrong:** The form accepts free-text or a wrong-cased class/race, so `gear_check`/`spell_check` silently produce zero rows for that char (the join to `wiki_spells.class` / the `race=="IKS"` check fails).
**Why it happens:** The compute layer matches on exact uppercase abbreviations (`WAR`, `IKS`, …); a typo or display-name (`Warrior`) won't match.
**How to avoid:** Validate `class` against `enrich.CLASSES` (14 entries) and `race` against the RACES set (14 entries) server-side; drive the form's `<select>` from the same lists. Store the abbreviation, not a display name. (Note: `compute/gearcheck.go` keys the Iksar tier on the literal string `"IKS"`.)
**Warning signs:** a free-text class input; a char with a class set but zero gear/spell rows.

## Code Examples

Verified patterns from the repo (the templates to clone):

### Login-only route registration (the block to extend)
```go
// Source: cmd/squirebot-server/main.go:328-330 — VERIFIED
// Bank-coin — LOGIN-ONLY (D-12): RequireSession, NOT RequireOfficer.
mux.Handle("GET /api/v1/coin/bank-toons", webauth.RequireSession(db, webadmin.BankToonsHandler(db)))
mux.Handle("POST /api/v1/coin", webauth.RequireSession(db, webadmin.CoinSetHandler(db)))
```

### The pick-list / pre-fill source (reuse as-is)
```go
// Source: internal/backendsrv/store/readviews.go:279 — VERIFIED
// CharsWithMeta returns every non-removed character with id/name/class/level/race.
func (s *Store) CharsWithMeta(ctx context.Context) ([]CharMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, class, level, race FROM character WHERE is_removed = 0 ORDER BY name`)
	// ... scans class/level/race as NullString/NullInt64 → zero-values ...
}
```

### The audited tx idiom (reuse via webadmin helpers)
```go
// Source: internal/backendsrv/webadmin/audit.go — VERIFIED
err := withTx(ctx, db, func(tx *sql.Tx) error {
	if e := store.SetCharMetaTx(ctx, tx, req.CharacterID, req.Class, req.Level, req.Race, req.IsBankToon); e != nil { return e }
	return AppendAuditTx(ctx, tx, "char_meta_set", caller(ctx), map[string]any{"character_id": req.CharacterID}, nowUnix())
})
```

### The canonical value set (server-side validation)
```go
// Source: internal/backendsrv/enrich/eqconst.go:26 — VERIFIED
var CLASSES = []string{"WAR","CLR","PAL","RNG","SHD","DRU","MNK","BRD","ROG","SHM","NEC","WIZ","MAG","ENC"}
// RACES is NOT yet in Go — port from apps-script/src/lib/eq-constants.ts:44-47:
//   HUM BAR ERU ELF HIE DEF HEF DWF TRL OGR HFL GNM IKS VAH
```

### The flip trigger (ops, not code)
```bash
# Source: .github/workflows/release.yml (on: push tags v*) — VERIFIED
# This publishes the bare squirebot.exe + a fresh latest.json (version=2.0.0):
git tag v2.0.0 && git push origin v2.0.0
# Then verify the Release published correctly:
gh release view v2.0.0 --json tagName,isPrerelease,assets   # expect isPrerelease:false, squirebot.exe + latest.json present
gh release download v2.0.0 -p latest.json -O -              # expect "version":"2.0.0", binary_url → v2.0.0/squirebot.exe
```

### Minting the guild codes (ops)
```bash
# Source: docs/backend-deploy.md §5 + cmd/squirebot-server/main.go:runMint — VERIFIED
# On the VPS, once per guildie (plaintext printed ONCE — DM it, never log it):
sudo -u squirebot /usr/local/bin/squirebot-server mint-code --owner "<guildie-label>"
```

## State of the Art

| Old Approach (roadmap / finding 04 §4.2) | Current Approach (CONTEXT, this phase) | When Changed | Impact |
|------------------------------------------|----------------------------------------|--------------|--------|
| 1–2 week shadow soak vs the live Sheet | No formal soak; brief reporting-in confirmation | 2026-05-31 discussion (D-05) | Removes the whole parity-tooling/soak-window scope; relies on independently-verified P12/P14/P15. |
| One-time backfill from the Sheet (owners/chars/coin/archive) | Fresh start; ONE form for `class/level/race/is_bank_toon`; everything else self-heals | 2026-05-31 (D-01/D-02) | Deletes the riskiest work (Sheets read, email↔Discord owner reconciliation). The form is the only build. |
| `_meta.schema_version` ↔ `WatcherMaxSchemaVersion` handshake | Forward-only `goose` migrations + `/api/v1/` API version | P11/P13 (already shipped) | The watcher's Sheets schema gate is gone; the flip rides SemVer auto-update, not a schema handshake. |
| Set Sheet read-only, keep as frozen archive, retire later | Abandon the Sheet in place (no export/delete/freeze) | 2026-05-31 (D-12) | No Sheet-side work at all; the frozen-since-2026-05-15 Sheet is already worthless to the system. |

**Deprecated/outdated:**
- **finding 04 §4.2 hybrid B/C backfill plan** — superseded by D-01 (fresh start). Read §4 for context only; do NOT implement its backfill mechanics (§4.3).
- **The roadmap's Phase 16 "hybrid shadow-mode path" note** (`ROADMAP.md:182`) — superseded by D-01/D-05.
- **CUTOVER-01/02 roadmap wording** in `REQUIREMENTS.md:59-60` — the discussion deliberately satisfies these in spirit (see the reinterpretation table); the traceability rows are `Pending`, filled by planning.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The Apps Script enrichment triggers are *currently still firing* (D-10 inference). High confidence from how time-driven triggers work, but not directly observed (no access to the live Apps Script Executions dashboard in this session). | Pitfalls 3 / Decommission | LOW — if they happen to be already stopped, D-10 becomes a no-op verification rather than an action. Either way the checklist step is "confirm no scheduled runs remain." |
| A2 | `level` valid range is 1–60 (classic P1999 EQ level cap). | Architecture / Pattern 1 | LOW — if the guild wants 0/"unset" allowed, widen to 0–60 or treat blank as NULL. A planning-time confirmation; cheap to change. The bank-coin precedent treats blank as a real "unset" via nullable columns, which suggests blank-level → NULL is acceptable. |
| A3 | A clean `v2.0.0` tag is the right version for the flip (strictly newer than every running v1.0.2 watcher). | Flip / Pitfall 4 | LOW — any `vX.Y.Z` > `1.0.2` works; `2.0.0` matches the "v2.0 milestone" framing. If a guildie is somehow on a higher version, bump accordingly. |
| A4 | The form should edit only existing characters (no "add character" affordance), per D-03. | Anti-Patterns | NONE — this is a locked decision (D-03), restated for the planner; listed here only because it shapes the form's UX. |
| A5 | Route names `/api/v1/char/meta` + `/api/v1/char/meta-list` (TBD per CONTEXT). | Pattern 5 | NONE — a naming choice; any `/api/v1/`-prefixed, login-only pair is fine. The planner/UI-spec picks the final names. |

## Open Questions

1. **Is the RACES list ported to Go, or does the handler import from where?**
   - What we know: `enrich.CLASSES` exists in Go (`eqconst.go`); the RACES list exists only in TypeScript (`apps-script/src/lib/eq-constants.ts:44-47` — 14 entries: `HUM BAR ERU ELF HIE DEF HEF DWF TRL OGR HFL GNM IKS VAH`).
   - What's unclear: the backend has no Go `RACES` constant. The char-meta handler needs one for server-side validation.
   - Recommendation: add a small `RACES` slice to `internal/backendsrv/enrich/eqconst.go` (or a new tiny constants file in the handler's package) ported verbatim from the TS source. Trivial, no new dependency. The frontend `<select>` mirrors it.

2. **Does the form's "Save" need a confirm step for `is_bank_toon`?**
   - What we know: `BankCoinForm` is non-destructive → no `ConfirmDialog` (D-12 precedent). Setting `is_bank_toon` changes which view a char appears in (bank view) but destroys no data.
   - What's unclear: whether flipping `is_bank_toon` off (removing a char from the bank view) warrants a confirm.
   - Recommendation: follow the bank-coin precedent — no ConfirmDialog (non-destructive metadata). If the UI-spec pass wants a confirm on `is_bank_toon` un-set, that's a small addition; default to none.

3. **Should the maintainer confirmation for CUTOVER-01 be a doc artifact or just a checklist line?**
   - What we know: D-05 says "a brief maintainer confirmation … folded into the flip gate." There's no formal soak deliverable.
   - What's unclear: whether the milestone audit wants a written "guildies reporting in" note.
   - Recommendation: fold it into the decommission checklist as one line ("Confirmed N of ~12 guildies have uploaded since the flip — `SELECT COUNT(*) FROM character WHERE last_seen > <flip-date>`") rather than a separate doc. Cheap, satisfies the audit, no soak-window scope.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build the char-meta endpoint + cross-compile the watcher binary | ✓ (assumed on dev box; CI uses 1.25) | 1.24+ | — (user installs toolchains themselves per memory) |
| Node + npm | Build the char-meta form (`npm run build` / `vitest`) | ✓ (web/ already builds) | 20+ / vitest 4.1.7 | — |
| `gh` CLI | Verify/inspect GitHub Releases; (optional) refresh release notes | ✓ | authenticated (used this session) | git tag push fires `release.yml` without `gh` |
| GitHub Actions (`release.yml`) | Publish the re-targeted binary + `latest.json` on a `v*` tag | ✓ (workflow present, runs on tag push) | windows-latest, Go 1.25, NSIS ≥3.10 | `workflow_dispatch` with a version input |
| SSH access to the Hetzner VPS | Deploy the char-meta endpoint (scp + restart); mint codes on the box | ✓ (documented; ssh-agent + `id_ed25519` per memory) | — | — |
| Google Cloud Console access | Delete the OAuth 2.0 Client ID (CUTOVER-04 / D-11) | ✓ (maintainer's Google account owns the project) | — | Disabling vs deleting both satisfy "no live Google machinery" |
| Google Apps Script access | Delete the 7 project triggers (CUTOVER-04 / D-10) | ✓ (maintainer owns the container-bound script) | — | Triggers dashboard `script.google.com/home/triggers` OR the project editor's Triggers panel |
| Discord (DMs) | Distribute the ~12 minted codes + herd the guild (D-06/D-07) | ✓ (out-of-band; the guild Discord exists — it's the P15 login gate) | — | — |

**Missing dependencies with no fallback:** none — every dependency this phase needs is already in use (the backend is live, the frontend builds, releases have shipped, the VPS is reachable, the Google consoles are the maintainer's).

**Missing dependencies with fallback:** none blocking. The only "not-yet-done" item is the *artifact* (the unpublished v2.0.0 binary), which is a plan task, not a missing tool.

## Security Domain

> `security_enforcement: true`, `security_asvs_level: 1`, `security_block_on: high` (config.json). The char-meta form is a new authenticated write surface, so the V-categories below apply to it; the decommission steps are a net security *reduction* (removing a Google attack surface).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control (in this codebase) |
|---------------|---------|-----------------------|
| V2 Authentication | yes | The form rides the existing Discord-OAuth2 session (P15). No new auth code — `webauth.RequireSession` gates the route. [VERIFIED] |
| V3 Session Management | yes | Existing opaque `sb_session` cookie (HttpOnly + Secure + SameSite=Lax + Domain-scoped, hashed server-side). No change. [VERIFIED: webauth/session.go] |
| V4 Access Control | yes | Login-only (D-03): `RequireSession`, NOT `RequireOfficer`. The SERVER is the gate; the form gates UX only (the P15 D-01 posture). The acting `discord_user_id` is audited (`AppendAuditTx`) for accountability. |
| V5 Input Validation | **yes (the focus)** | Server-side: `character_id > 0`; `class ∈ enrich.CLASSES`; `race ∈ RACES`; `level` 1–60 (A2); `is_bank_toon` is a JSON bool (decoder-validated). Parameterized `?` placeholders ONLY (no string-built SQL). Mirror `validCoin`'s server re-check (T-15-29: never trust the client). |
| V6 Cryptography | no (for the form) | The form handles no secrets. (The guild codes minted in this phase are SHA-256-hashed at rest by the existing `mint-code` path — no new crypto.) |

### Known Threat Patterns for {Go net/http + SQLite + SvelteKit, this phase}

| Pattern | STRIDE | Standard Mitigation (here) |
|---------|--------|---------------------------|
| SQL injection via char-meta fields | Tampering | Parameterized `?` placeholders in `SetCharMetaTx` (the store-layer V5 invariant) — never interpolate `class`/`race`/`level`. |
| Stored XSS via a character name echoed in success copy | Tampering / (client) | The name renders via plain `{}` (Svelte auto-escapes), NEVER the `{@html}` directive — the exact T-15-28 mitigation `BankCoinForm` documents. |
| CSRF on the `POST` (cookie-authed) | Spoofing | `SameSite=Lax` session cookie + the API requires a JSON `Content-Type` + the CORS allow-origin is the exact static-site origin (no wildcard). The existing posture; no change. [VERIFIED: session.go + readapi.CORS] |
| Privilege confusion (member writes officer-only data) | Elevation of Privilege | N/A by design — char-meta IS login-only shared data (D-03). No officer-only surface is touched. |
| Repudiation of who set a char's metadata | Repudiation | `AppendAuditTx("char_meta_set", caller(ctx), …)` — append-only audit row in the same tx (T-15-17). |
| Forged/oversized request body | DoS / Tampering | `json.NewDecoder(r.Body).Decode` + the trivial-guard reject; the values are tiny scalars (no large-body surface). |
| **Decommission: deleted OAuth client restorable for 30 days** | Info disclosure (residual) | CITED Google behavior — a deleted OAuth client is restorable for 30 days then permanently gone. Acceptable: the watcher OAuth code is already deleted (P13), so the client has no consumer; deletion immediately revokes any outstanding tokens. Note it in the checklist. |
| **Decommission: triggers keep loading the volunteer wiki/PigParse** | (good-citizen, not STRIDE) | Deleting the 7 triggers stops the parallel load — the `politeFetch` ethos (D-10). |

**Net security posture:** P16 *reduces* the system's attack surface (removes the Google OAuth client + the Sheet's server-side automation). The one new surface — the char-meta form — is a strict clone of an already-security-reviewed, regression-covered form (P15 had its 2 node-blind blockers fixed + the WR-05 fail-open→fail-closed fix). No `security_block_on: high` concern is introduced provided the V5 server-side validation + parameterized SQL + plain-`{}` name rendering are honored (all inherited from the bank-coin template).

## Sources

### Primary (HIGH confidence — codebase, verified this session)
- `internal/backendsrv/webadmin/coin.go` + `officers.go` + `audit.go` — the login-only write-handler + audited-tx pattern (the clone template)
- `internal/backendsrv/store/coin.go` + `admins.go` + `readviews.go` (`CharsWithMeta`) — the store mutator + pick-list source
- `internal/backendsrv/webauth/session.go` — `RequireSession`/`RequireOfficer`/`caller(ctx)`
- `internal/backendsrv/migrations/00001_init.sql` — confirms `class/level/race/is_bank_toon` columns exist (no migration needed)
- `internal/backendsrv/compute/gearcheck.go` + `spellcheck.go` + `eqconst.go` (`CLASSES`) — the consumers + the class value set; `race=="IKS"` literal
- `internal/backendsrv/enrich/eqconst.go` (`CLASSES`) + `apps-script/src/lib/eq-constants.ts:44-47` (`RACES`) — validation value sets
- `cmd/squirebot-server/main.go` — route-wiring block + `mint-code` CLI
- `web/src/lib/components/BankCoinForm.svelte` + `FormField.svelte`, `web/src/lib/coin.ts`, `web/src/lib/api.ts`, `web/src/routes/bank-coin/+page.svelte` + `admin/+page.svelte` — the form/validation/route templates + the CR-01 input-type lesson
- `internal/update/check.go` + `manifest.go` + `swap.go`; `cmd/squirebot/build_constants.go` + `main.go` — the auto-update flip mechanics + `IsNewer`/`ManifestURL`/version constant
- `.github/workflows/release.yml` — the publish-on-tag pipeline (already re-targeted for v2.0)
- `docs/backend-deploy.md` (§5 mint, §7 deploy) + `docs/eviction-runbook.md` (checklist style) + `docs/apps-script-deploy.md` (trigger install) + `apps-script/src/triggers/installTriggers.ts` (the 7 triggers)
- `gh release list` / `gh release view v1.0.2 --json …` / `git tag` (2026-05-31) — **confirmed no v2.x release/draft/tag; latest is v1.0.2 (16.99 MB Google binary); `latest.json` pinned to v1.0.2**
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` §4 — the cutover/backfill research (now superseded by D-01)
- `.planning/config.json` — `nyquist_validation:false`, `security_enforcement:true` (level 1), `ui_phase/ui_safety_gate:true`

### Secondary (MEDIUM — official Google docs, decommission steps)
- [Manage OAuth Clients — Google Cloud Console Help](https://support.google.com/cloud/answer/15549257?hl=en) — delete an OAuth 2.0 Client ID (Clients page → checkbox → DELETE; immediately revokes tokens; restorable 30 days)
- [Setting up OAuth 2.0 — API Console Help](https://support.google.com/googleapi/answer/6158849?hl=en) — Credentials page location
- [Installable Triggers — Apps Script](https://developers.google.com/apps-script/guides/triggers/installable) — programmatic create/delete via `ScriptApp`; triggers run on Google's schedulers independent of clients
- [Troubleshooting Triggers — Mixed Analytics](https://mixedanalytics.com/knowledge-base/troubleshooting-triggers/) — the per-user Triggers dashboard at `script.google.com/home/triggers`

### Tertiary (LOW — none relied upon)
- (none — every load-bearing claim is verified against the codebase or cited from official Google docs)

## Metadata

**Confidence breakdown:**
- Standard stack / build target: **HIGH** — the char-meta form is a line-by-line clone of shipped, code-reviewed code; the columns/consumers/value-sets are all verified in-repo.
- Flip mechanics (CUTOVER-03): **HIGH** — `internal/update` + `release.yml` read end-to-end; the "binary unpublished" finding is directly verified via `gh`.
- Decommission steps (CUTOVER-04): **MEDIUM-HIGH** — the operations are simple maintainer console actions, cited from Google docs; A1 (triggers currently firing) is an unobserved-but-well-grounded inference.
- Pitfalls: **HIGH** — drawn from this repo's own recorded incidents (CR-01 node-blind blockers, the prerelease/latest interaction, the OAuth-block-vs-trigger distinction).

**Research date:** 2026-05-31
**Valid until:** 2026-06-30 for the codebase claims (stable repo); the GitHub-Release state is a point-in-time snapshot — re-run `gh release list` at planning time to confirm no v2.x release has been published in the interim.
