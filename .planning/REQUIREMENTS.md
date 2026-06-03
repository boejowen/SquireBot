# Milestone v2.1 — Requirements: Self-Service Watcher Linking

**Status:** 🔄 In progress — opened 2026-06-01. Continues phase numbering at **17**.

**Milestone goal:** Let any guildie link their own watcher from squirebot.quest via Discord login — no maintainer hand-minting + DMing codes — while keeping the watcher credential a static, reusable bearer token (no watcher change).

**Why now:** At the v2.0 cutover the maintainer hand-minted 11 codes and DMed them. That doesn't scale and routes every guildie's plaintext credential through the maintainer's hands. The website already has Discord OAuth2 login (AUTH-08) capturing per-user Discord identity (AUTH-09), and `auth.MintCode` already mints hashed bearer tokens — so self-service is a matter of exposing a session-scoped mint endpoint + a "link your watcher" page and retiring the manual CLI path.

**Locked decisions (2026-06-01 discussion):**
- **Link code = the bearer token itself (v2.0 model, reusable).** `auth.MintCode` shows the plaintext exactly once at mint time (hash-only at rest); the watcher pastes it and reuses it as its `Bearer` forever. No expiry. **NO watcher change** — onboarding is identical to v2.0; this milestone is a website + backend-endpoint change only.
- **Re-link = additive.** Minting a new code issues an *additional* valid token without revoking existing ones; a guildie can run the watcher on multiple PCs simultaneously. Revocation is per-token.
- **Manual `mint-code` CLI removed.** Self-service is the sole minting path. The ~11 existing guildies are already linked from the v2.0 cutover, so nobody is stranded. (The `revoke-code` CLI is retained as an ops backstop — see Out of Scope.)
- **HARD CONSTRAINT:** Discord identity is captured at link-time only; **never** put Discord OAuth *in the watcher* (P13 made it browser-free on purpose; OAuth there re-introduces the ~7-day-expiry / public-secret / loopback fragility v2.0 escaped).

**Stack:** unchanged from v2.0 — Go + SQLite backend on the Hetzner VPS (`api.squirebot.quest`), SvelteKit static site (`squirebot.quest`), Discord OAuth2 session, opaque hashed bearer tokens. No new dependencies anticipated.

---

## v2.1 Requirements

### Self-Service Watcher Linking (LINK)

- [x] **LINK-01**: A signed-in guildie can mint a new watcher code for themselves via a session-scoped backend endpoint, with the owner derived server-side from the Discord session (never free-typed or passed by the client). *(Backend done in 17-02: `MintOwnCodeHandler` + `ResolveOrCreateOwnerByDiscordTx`; frontend trigger in 17-03.)*
- [x] **LINK-02**: A minted code's owner is associated with the guildie's Discord identity (`web_user`), unifying website-login identity with watcher ownership — replacing the loose `owner.label == web_user.username` bridge the v2.0 eviction/owner-floor path relies on. *(17-01 FK + 17-02 resolve-or-create stamp + D-05 eviction-floor FK rewire.)*
- [x] **LINK-03**: Minting is additive — a new code is issued without revoking the guildie's existing codes, so the watcher can run on multiple PCs at once. *(17-02: `MintCodeForOwnerTx` only INSERTs; verified additive in `TestMintOwnCode_…`.)*
- [x] **LINK-04**: A "Link your watcher" page (behind the Discord login) shows the freshly minted code's plaintext exactly once, with copy-to-clipboard and clear paste-into-watcher instructions; the plaintext is never retrievable again (hash-only at rest). *(17-03: `/account` + `WatcherCodesPanel` show-once panel; plaintext in component state only, never persisted/re-fetched/logged. Browser-smoke verified live — reload never re-reveals.)*
- [x] **LINK-05**: A guildie can view their own active watcher codes (each identifiable, e.g. by label / created date / last-seen) and revoke any one individually, without affecting their other codes (per-token revocation). *(17-03: active-codes list #N/created/last-seen + confirm-before-commit per-code revoke w/ optimistic collapse; browser-smoke verified scoped revoke + additive mint live.)*
- [x] **LINK-06**: The manual `mint-code` CLI subcommand is removed; the self-service endpoint is the only path to mint a watcher code. *(17-02: `runMint` + `case "mint-code"` deleted; `revoke-code` retained.)*

### Watcher cleanups carried forward (WATCH)

> ⚠ **Verify-or-close:** the v2.0 archive records 999.20/21/22 as RESOLVED in Plan 13-04 (commits `c930fc2`/`e758fb0`/`3e8e53b`). These requirements are framed to **confirm the live state and close any residual**, not to re-do completed work. If confirmed done, the only real residual is WATCH-14's stuck-watcher reinstall (an ops action, not code).

- [x] **WATCH-12**: `cmd/squirebot/console_windows.go` is `gofmt`-clean (999.20) — confirmed live (`gofmt -l` clean). ✅ Phase 18
- [x] **WATCH-13**: `freeConsole()`'s doc matches its implementation and a no-console launch logs at Debug (not a spurious Warn on every GUI/Explorer launch) (999.21) — confirmed live (`slog.Debug`=1, `slog.Warn`=0; doc/impl reconciled). ✅ Phase 18
- [x] **WATCH-14**: The auto-updater compares versions SemVer-aware, so a watcher parked on a pre-release (e.g. `0.4.0-rc1`) correctly updates to a final release (999.22) — confirmed live (`TestIsNewer_SemVerPreRelease` green). Ops residual resolved: no production watcher was stuck (the `0.4.0-rc1` install is the disposable Azure test VM; this PC + all 7 reporting toons on 2.0.0). ✅ Phase 18

---

## Future Requirements (deferred, not this milestone)

- **Officer mint-on-behalf** — an officer minting a code for a guildie who can't log in. Deferred per the locked "self-service replaces the CLi entirely / no manual escape hatch" decision; revisit only if a real can't-log-in case appears.
- **Code labels / device naming UX polish** — richer per-code metadata (rename, "this PC" detection). LINK-05 ships basic identifiability; fancier device management is deferred.
- **999.5 Self-service eviction** — departing guildie quits cleanly without officer action. Adjacent to identity self-management but a separate threat-model; deferred.
- **999.12 / WANT-01..08 Wantlist + Discord pinger** — still deferred; this milestone's Discord-tied ownership further pre-pays the identity prerequisite.

## Out of Scope (explicit exclusions)

| Exclusion | Reason |
|-----------|--------|
| Discord OAuth *in the watcher* | HARD CONSTRAINT — re-introduces the ~7-day-expiry / public-secret / loopback fragility v2.0 deliberately escaped; P13 made the watcher browser-free on purpose. Discord is the identity at link-time (website) only. |
| Changing the watcher onboarding flow | The code-is-the-bearer-token model means the watcher's "paste your guild code" onboarding is unchanged. No watcher rebuild for the core feature. |
| Token expiry / rotation schedules | Tokens stay long-lived and reusable (no expiry); revocation is the only invalidation path. |
| Removing the `revoke-code` CLI | Only `mint-code` is replaced by self-service. `revoke-code` is retained as an ops/officer backstop (e.g. revoking a departed guildie's tokens). |
| SignPath OSS signing (999.9) | Orthogonal, still in flight; lands as a hotfix when approved, not part of this milestone. |

---

## Traceability

_Maps each REQ-ID to exactly one phase. Phases continue at 17. 9/9 mapped — no orphans, no duplicates._

| REQ-ID | Phase | Status |
|--------|-------|--------|
| LINK-01 | Phase 17 | ✅ Done (17-02 backend) |
| LINK-02 | Phase 17 | ✅ Done (17-01 FK + 17-02 resolve/floor) |
| LINK-03 | Phase 17 | ✅ Done (17-02) |
| LINK-04 | Phase 17 | ✅ Done (17-03 frontend; browser-smoke verified) |
| LINK-05 | Phase 17 | ✅ Done (17-03 UI; browser-smoke verified) |
| LINK-06 | Phase 17 | ✅ Done (17-02) |
| WATCH-12 | Phase 18 | Done |
| WATCH-13 | Phase 18 | Done |
| WATCH-14 | Phase 18 | Done |

**Coverage:** 9/9 v2.1 requirements mapped ✓ · Phase 17: 6 (LINK-01..06) · Phase 18: 3 (WATCH-12/13/14).

---

*Requirements defined: 2026-06-01 for v2.1 "Self-Service Watcher Linking". Traceability filled 2026-06-01 (Phases 17–18). Prior milestones: v1.0 (`milestones/v1.0-REQUIREMENTS.md`), v1.0.1 (`milestones/v1.0.1-REQUIREMENTS.md`), v2.0 (`milestones/v2.0-REQUIREMENTS.md`).*
