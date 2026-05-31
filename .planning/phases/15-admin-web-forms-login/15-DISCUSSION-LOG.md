# Phase 15: Admin Web Forms + Login - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-30
**Phase:** 15-admin-web-forms-login
**Areas discussed:** Login gate scope, Officer & owner-floor, Eviction behavior, Bank-coin model
**Area selection:** user selected ALL FOUR offered gray areas.

---

## Login gate scope — Read gate

| Option | Description | Selected |
|--------|-------------|----------|
| Whole site | Every visitor signs in with Discord; read API itself requires a valid session; non-members refused. Matches AUTH-08 + P14 D-04. | ✓ |
| Read public, gate writes | Keep P14's "anyone with the link" read; only write forms require login. Data stays world-readable. | |
| Members optional, gate writes | Read public; login optional for members; only writes gated. Three states. | |

**User's choice:** Whole site
**Notes:** Read API (not just frontend) enforces the session — frontend-only gating is bypassable. Closes P14's public-but-unlisted stopgap.

---

## Officer & owner-floor — Add officer

| Option | Description | Selected |
|--------|-------------|----------|
| Pick a logged-in user | Form lists Discord identities captured at login; click to promote. No snowflakes typed. Must have signed in once. | ✓ |
| Type a Discord handle/ID | Promote before first login, but typo-prone (wrong ID = silent no-op officer). | |
| Both | Pick from known users + manual entry fallback. | |

**User's choice:** Pick a logged-in user

## Officer & owner-floor — Owner-floor seeding

| Option | Description | Selected |
|--------|-------------|----------|
| CLI seed your Discord ID | `squirebot-server set-owner-floor <discord-id>`, run once at deploy (mirrors P11 mint-code CLI). Un-removable by peers. | ✓ |
| First login auto-floor | First user to sign in becomes floor. Zero ops, but racy/ambiguous. | |
| Hardcode in config | Bake Discord ID into server config/env. Couples identity to deploy config. | |

**User's choice:** CLI seed your Discord ID
**Notes:** Seeded ID is also the first/bootstrap admin. Replaces v1's gone `onOpen`/`getOwner()` bootstrap.

---

## Eviction behavior — Evict target

| Option | Description | Selected |
|--------|-------------|----------|
| Whole guildie | Pick a guildie (owner) → cascade is_removed across all their characters. Exactly v1. | ✓ |
| Single character | Evict one character at a time. New behavior; tedious for a fully-departed guildie. | |
| Both | Default whole-guildie with per-character option. | |

**User's choice:** Whole guildie

## Eviction behavior — Evict effect

| Option | Description | Selected |
|--------|-------------|----------|
| Revoke code + 30-day grace | Immediately revoke guild code (watcher stops uploading) AND keep v1's 30-day grace + archive (reversible during grace). | ✓ |
| Keep v1 exactly | Mark is_removed + grace + archive only; don't touch the guild code. | |
| Immediate hard removal | Revoke code + remove data now, no grace. | |

**User's choice:** Revoke code + 30-day grace
**Notes:** Aligns with the roadmap Note ("access revocation = one app-controlled action" in the DB world). Reversal during grace = un-remove + re-mint.

---

## Bank-coin model — Coin scope

| Option | Description | Selected |
|--------|-------------|----------|
| Bank toons only | Coin entry limited to is_bank_toon characters; nullable plat/gold/silver/copper columns on `character`. Matches v1. | ✓ |
| Any character | Coin for any character. Not meaningful on a random alt. | |
| One guild-wide total | Single guild total, not per-character. Diverges from v1. | |

**User's choice:** Bank toons only
**Notes:** Surfaces in the bank view, replacing P14's null/0 placeholder. Per requirement wording, the coin form is authenticated-member (not officer-only) — captured as D-12 in CONTEXT.

---

## Claude's Discretion

- Session/cookie exact attributes + OAuth redirect-callback route + cross-subdomain cookie/CORS-credentials mechanics (posture locked, mechanics → research).
- Exact `00004_*` goose migration DDL (web-user / web_session / guild_admins / coin columns / eviction grace+archive).
- `guild_admins` as a table vs an is_admin/role column; owner-floor as a singleton row vs config.
- Coin input validation/units; eviction archive as a scheduled job vs lazy-on-read.
- All login-screen + form visual/interaction layout → `/gsd-ui-phase 15`.

## Deferred Ideas

- v2 Wantlist + Discord pinger (this phase only captures AUTH-09 identity).
- Per-member visibility tiers (universal visibility remains).
- Tightening bank-coin to officer-only (currently authenticated-member per ADMIN-05).
- Discord-app provisioning (create app, register redirect URI, client id+secret) = maintainer prerequisite → flag in research.
- Shadow-soak / human-data backfill / coordinated watcher flip / Sheet + Apps Script + Google-OAuth decommission → P16.
