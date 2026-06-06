# Phase 16: Cutover + Decommission - Context

**Gathered:** 2026-05-31
**Status:** Ready for planning

<domain>
## Phase Boundary

The operational endgame of v2.0 "Off Google": move the guild fully onto the self-hosted website and retire the live Google machinery. No new product surface beyond one small form.

**Reality reframing (load-bearing — finding 04 §4 assumed a *live* Sheet; the world moved):** the guild has been **dark on the Sheet since 2026-05-15** (Google's brand-verification block walled the watchers), and **P13/P14/P15 are already deployed live + verified** at squirebot.quest. So the classic "shadow alongside a live Sheet → reconcile parity → flip ingest" dance is largely void — the Sheet is frozen, the backend is already the only place data can go, and the re-targeted watcher already writes to it. P16 is therefore much **lighter** than the roadmap envisioned.

**The four CUTOVER requirements, as reinterpreted by this discussion** (the milestone audit should read these as deliberately satisfied-in-spirit, not skipped):

| Req | Roadmap wording | This phase actually does |
|---|---|---|
| **CUTOVER-01** | 1–2 wk shadow soak vs the live Sheet | **No formal soak.** Backend live since 2026-05-29; satisfied by a brief "guildies are reporting in on the backend" confirmation folded into the flip gate. |
| **CUTOVER-02** | One-time backfill of human data from the Sheet | **No backfill — clean break / fresh start.** Human data is established *natively* in the Google-free system (new char-meta form + existing bank-coin form); nothing read from the Sheet. |
| **CUTOVER-03** | Single coordinated watcher self-update flip | **As planned:** auto-update + Discord herding; proceed as soon as technically feasible (no migration-% gate). |
| **CUTOVER-04** | Decommission Sheet + Apps Script + OAuth client | **Kill the *live* Google machinery** (disable Apps Script triggers + retire OAuth client); **abandon the Sheet in place** (no export, no delete). |

**In scope:** the char-metadata write form + endpoint (the only real build); minting + distributing ~12 guild codes; publishing the re-targeted binary as a Release + herding the guild onto it; disabling the Apps Script enrichment triggers; retiring the Google OAuth client; a decommission checklist as the proof artifact.

**Out of scope:** any read of / import from the old Sheet; a formal soak window; Sheet export or deletion; per-owner visibility tiers; inventory history; the v2 Wantlist/Discord-pinger (AUTH-09 already pre-paid the identity prerequisite).
</domain>

<decisions>
## Implementation Decisions

### Fresh-start vs backfill (CUTOVER-02)
- **D-01 — No Sheet backfill. Clean break.** Nothing is read from the old Sheet; the Google-free SquireBot starts fresh on all guildie data. This deletes the riskiest P16 work outright (no Sheets export/parsing, no Google-email↔Discord owner-identity reconciliation). Closest to finding 04 §4.1 **Option C** ("hard cutover, no backfill"), extended from inventory-only to *all* human data.
- **D-02 — New char-metadata web form + backend write endpoint** for `class` / `level` / `race` / `is_bank_toon`. These have **no other source** (watcher uploads don't carry them; P15 built no form for them) yet `gear_check` / `spell_check` / `bank` depend on them — so this is required, not optional. The columns **already exist** in `00001_init.sql` (commented "set later / by backfill (P16)"), so no schema migration is needed for storage — just a `POST` handler + the form. ⚠️ **This flips P16's roadmap UI hint from `no` → `yes`** (P16 now has a UI surface).
- **D-03 — Form auth + semantics, locked by the bank-coin precedent (ADMIN-05 / D-12):** login-only, **any signed-in member** may set **any** character's metadata (no officer gate — non-sensitive shared data in a trust-rich guild, and it avoids needing a Discord↔guild-code ownership link that doesn't exist yet). The form operates on characters that **already exist** (created by their first watcher upload binding); **no pre-creation** of character rows. `is_bank_toon` lives on the same form.
- **D-04 — Everything else self-heals or re-enters natively:** inventory/spellbook self-populate from each watcher's first upload; bank coin is re-entered via the **existing P15 BankCoinForm**; the old `_archive` (evicted/stale history) does **not** carry over.

### Soak / go-live confidence (CUTOVER-01)
- **D-05 — No formal soak window.** The backend has run real daily/weekly enrichment cycles since 2026-05-29; P12's jobs were verified against the **live** wiki + PigParse APIs; P14/P15 passed **live human UAT**. The soak's original purpose (don't corrupt the live Sheet, reconcile parity) is void — the Sheet's frozen since 2026-05-15. CUTOVER-01 is satisfied by a brief maintainer confirmation that onboarded guildies are reporting in and their views look right, folded into the flip gate. Thinner safety margin accepted (the specific risks are already independently verified).

### The flip (CUTOVER-03)
- **D-06 — Auto-update + Discord herding.** Publish the re-targeted P13 binary as a GitHub Release (the existing `minio/selfupdate` updater pulls it for any still-running watcher); DM each guildie their minted code + a heads-up about the one-time paste prompt; share the fresh installer link for anyone whose watcher went dormant/uninstalled during the dark weeks.
- **D-07 — Per-guildie unique codes.** The architecture requires it (a shared code would bind every character to one owner). Mint ~12 via `squirebot-server mint-code --owner <label>` (plaintext printed once), distribute individually via Discord DM.
- **D-08 — No migration-percentage gate; proceed as soon as technically feasible** (re-targeted binary published as a Release + ~12 codes minted + char-meta form live). The guildies are already informed and prepared. Stragglers onboard whenever — **decommission strands no one** (the frozen Sheet already gives an un-migrated guildie nothing, and the upgrade path is GitHub, not Google).
- **D-09 — Roll-forward only.** There's no meaningful fallback to preserve: the old Sheet write-path is already dead (Google-blocked), so "revert to the Sheet" was never an option.

### Decommission (CUTOVER-04)
- **D-10 — Disable the Apps Script enrichment triggers.** They very likely still fire on Google's infra (independent of the dark watchers), double-loading the community wiki + PigParse APIs in parallel with the backend and burning Apps Script quota. Stopping them is both a teardown step and good-API-citizen hygiene (the `politeFetch` ethos).
- **D-11 — Retire the Google OAuth client** (the asset CUTOVER-04 explicitly names).
- **D-12 — Abandon the Sheet in place.** Don't touch its data or sharing — no export, no delete, no read-only freeze. Fresh-start already made it worthless to us; deleting it is busywork.
- **D-13 — CUTOVER-04 is satisfied as "no *live* Google machinery / no Google dependency remains."** The system's code-level Google-freedom was already proven in P13 (`go list -deps ./cmd/squirebot` Google-free; binary 57% smaller; no Google secret). The **proof artifact** is a decommission checklist documenting each retired Google asset (triggers disabled, OAuth client retired) plus the existing code-level proofs.

### Claude's Discretion / Planning flags (verify during research/planning — not user decisions)
- **Confirm the P13 re-targeted binary is actually *published as a GitHub Release*** (not just built locally). The entire flip (D-06) rides on this being the live auto-update target.
- **Verify the Apps Script enrichment triggers are actually still running** before treating D-10 as a real action (it's an inference from how Apps Script time-driven triggers work).
- **Decide whether the small char-meta form (D-02) warrants a `/gsd-ui-phase` pass** or a folded-in mini UI-SPEC. It's small and has strong analogs (D-03), so a light touch likely suffices — but the UI safety gate applies since P16 now ships UI.
- **Suggested sequence:** build char-meta endpoint+form → deploy → mint ~12 codes → publish binary + announce → (brief reporting-in confirmation) → disable Apps Script triggers + retire OAuth client → write the decommission checklist → milestone close.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Cutover strategy
- `.planning/explorations/website-milestone/04-data-enrichment-migration.md` §4 — the cutover & backfill research. Note: this discussion chose **§4.1 Option C (hard cutover, no backfill)** extended to *all* human data, NOT the §4.2 hybrid B/C the doc recommended — because the Sheet is already frozen and the system is already live. Read it for context, but the fresh-start decision (D-01) supersedes its backfill plan.

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — CUTOVER-01..04 + the traceability table (P16 rows are `Pending`, filled by planning). See the reinterpretation table in `<domain>` above.
- `.planning/ROADMAP.md` § Phase Details → Phase 16 — success criteria + the "hybrid shadow-mode path" note (now superseded by D-01/D-05).

### Schema & backend wiring (the build target)
- `internal/backendsrv/migrations/00001_init.sql` — `character.class/level/race/is_bank_toon` columns already exist (nullable, "set later / by backfill (P16)"). **No new migration needed for char-meta storage.**
- `cmd/squirebot-server/main.go` — CLI subcommands (`mint-code`/`revoke-code`/`run-job`/`set-owner-floor`/`serve`) + the route-wiring block where the new char-meta endpoint gets registered. **No `backfill`/char-meta endpoint exists yet.**

### Closest analogs for the char-meta form (D-02/D-03)
- `internal/backendsrv/webadmin/coin.go` + `internal/backendsrv/webauth/` (`RequireSession`) — the login-only write-handler pattern (the bank-coin form is the precedent for "any member writes shared non-sensitive data"); audit_log writes; authorize-under-tx store methods.
- `web/src/lib/` BankCoinForm + `FormField.svelte` + `api.ts` (`postJSON`, `classifyAdminError`) — the form/validation/credentialed-fetch pattern, plus the repo's node-only test philosophy (pure helpers, no DOM/@testing-library).
- `internal/backendsrv/compute/gearcheck.go` + `compute/spellcheck.go` + `compute/bank.go` — the read views that consume `class`/`race`/`is_bank_toon`; they'll simply start working once the form populates those fields.

### Flip & ops (CUTOVER-03/04)
- `internal/update/` (watcher) — the `minio/selfupdate` GitHub-Releases auto-updater + 999.22 SemVer pre-release compare that de-risks the coordinated flip; the `internal/onboarding` + `internal/credstore` guild-code paste flow (P13).
- `docs/backend-deploy.md` — on-box deploy posture (systemd/Caddy/cross-compile) for shipping the char-meta endpoint + minting codes.
- `docs/eviction-runbook.md` — runbook style/precedent for the decommission checklist (D-13).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`webadmin` package + `webauth.RequireSession`** — the new char-meta `POST` endpoint is a near-clone of `coin.go` (login-only gate, validate, write, audit). Wire it in `main.go` beside the coin routes.
- **`character.class/level/race/is_bank_toon` columns** — already in the schema; the form writes to existing storage, no migration.
- **BankCoinForm + FormField + `api.ts postJSON`** — the char-meta form is the same shape (login-only, pick a character, validated fields, credentialed POST). Reuse wholesale.
- **`mint-code`/`revoke-code` CLI + `minio/selfupdate` auto-update + on-box systemd/Caddy** — the entire flip rides existing, proven infra; no new tooling.

### Established Patterns
- **Login-only vs officer-only gate (D-01/D-12 from P15):** non-sensitive shared writes = `RequireSession`; this drove D-03.
- **Authorize-under-transaction + audit_log** for every write handler.
- **Node-only web tests** (pure helpers + `.svelte` source assertions; no DOM) — per `web-tests-node-only-blind-to-dom`, code-review/browser-smoke the form before calling it verified.

### Integration Points
- New `POST /api/v1/char/meta` (name TBD) registered in `cmd/squirebot-server/main.go` under `RequireSession`.
- New form route in `web/` (sibling to `/bank-coin`); surfaces the four fields per existing character.
- The read-view compute (`gearcheck`/`spellcheck`/`bank`) is the *consumer* — already built; it just needs the data the form provides.
- Ops touch-points live outside the repo: GitHub Releases (binary), the VPS (mint codes, deploy), Discord (code distribution + herding), the Google console (disable triggers, retire OAuth client).
</code_context>

<specifics>
## Specific Ideas

- User's framing, verbatim intent: *"I want the 'google free' SquireBot to be a fresh start, at least as far as guildie data is concerned"* and *"proceed with the flip as soon as is feasible from a technical standpoint — the guildies know the change is coming and are prepared."* Bias the whole phase toward **minimal fuss, fast close** — the user repeatedly chose the lighter option (no backfill, no soak, abandon-in-place).
- The one place the user accepted *more* work than "do nothing" was the **char-meta form** — because without it the Core Value view (`gear_check` = "what do I still need") ships blank. That trade-off is the phase's center of gravity.
</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (The user's "fresh start" and "abandon the Sheet" calls are *reductions* in scope, captured as decisions D-01/D-12, not deferrals.)
</deferred>

---

*Phase: 16-cutover-decommission*
*Context gathered: 2026-05-31*
