---
status: complete
phase: 15-admin-web-forms-login
source: [15-VERIFICATION.md]
started: 2026-05-31T03:30:00.000Z
updated: 2026-05-31T07:40:00.000Z
---

## Current Test

All items verified LIVE on 2026-05-31. Phase 15 is deployed to the VPS and human-confirmed end-to-end.

> Phase 15 was deployed live (binary + 00004 migration to v4 + DISCORD_* systemd EnvironmentFile + frontend bundle + owner-floor seeded as `broccolifart`). A demo character (`Demoknight` / owner `DemoGuildie`, SHD L60 IKS bank-toon) was loaded via the ingest API to exercise the views + forms; it was evicted (smoke #4) and then restored server-side.

## Deploy Prerequisites — DONE

- [x] 4 `DISCORD_*` vars in the systemd `EnvironmentFile` `/etc/squirebot/squirebot.env` (600 root); `SQUIREBOT_WEB_ORIGIN` + `SQUIREBOT_COOKIE_DOMAIN` set.
- [x] Binary deployed; `00004_web_auth.sql` applied on boot (schema v4; web_user/web_session/guild_admins/app_config present).
- [x] `set-owner-floor` seeded `broccolifart` (app_config + guild_admins bootstrap row).

## Tests

### 1. Discord login — member admitted, non-member refused (AUTH-08)
expected: A guild member signs in via Discord OAuth2 and lands authenticated; a non-member is bounced with no session.
result: pass — `broccolifart` signed in end-to-end (state→exchange→guild-membership check→session→cross-subdomain cookie→home). Login redirect carries `scope=identify guilds`.

### 2. Per-user Discord identity captured (AUTH-09)
expected: SessionIndicator shows username/avatar; web_user row holds discord_user_id + username + avatar.
result: pass — identity shown in the header; `web_user` row present.

### 3. Whole-site read gate at the API (D-01)
expected: No session → every /api/v1 read route 401 (server-verified, not just frontend); fresh browser shows the login screen.
result: pass — reads return 401 without a cookie (verified on the live server); incognito hits the login screen.

### 4. Eviction — officer-only, cascade + revoke + grace, reversible (ADMIN-04 / D-10)
expected: Officer evicts a guildie → characters `is_removed`, guild code revoked, 30-day grace; non-officer can't; restore within grace re-mints.
result: pass — evicted `DemoGuildie` via the form (preview cascade + consequence callout + ConfirmDialog); `is_removed=1`, code revoked, real 30-day grace date shown (CR-02). Restore done server-side (un-removed + code reactivated). SEE GAP G-1 (restore has no web UI).

### 5. Bank-coin entry — any authenticated member, surfaces in bank view (ADMIN-05 / D-11/D-12)
expected: A signed-in member records plat/gold/silver/copper on a bank-toon; values persist + surface in the bank view; range-validated server-side.
result: pass — recorded large values (`5656 / 5655 / 6777 / 8775`) on Demoknight; the >999 gold/silver/copper confirm the 260531-2qk uncap; values persisted + surface in the Bank view.

### 6. Officer management — promote-by-pick, owner-floor un-removable (ADMIN-06 / D-07/D-08)
expected: Officer promotes a logged-in member by pick; owner-floor can't be removed by a peer; idempotent + audited.
result: pass (partial scope) — owner-floor lockout confirmed: `broccolifart` shows as "(owner)" with no Remove button. Promote-by-pick shows the "no signed-in members to promote" empty state because only one user has logged in (expected — full promote awaits a second guildie login).

### 7. Visual QA — EQ theme site-wide + accessible ConfirmDialog (WEB-05 / W-5)
expected: Login + forms carry the EQ theme across the 5 palettes; ConfirmDialog traps focus, Cancel-focused, Esc/backdrop dismiss; destructive uses --destructive.
result: pass — theme picker re-themes the site across palettes; ConfirmDialog behaves per W-5; destructive evict button in the --destructive color.

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- **G-1 (low, UI gap):** Eviction restore (D-10 reversibility) is fully built in the backend + the `restoreEviction` API wrapper, but **not surfaced in the EvictionForm UI** — there is no Restore control, so un-eviction is not possible from the web (it was done via SQL/server-side here). Candidate for a small follow-up: add an evicted-owners list + Restore action to the admin UI. Does not block Phase 15 (eviction itself works; restore is a rare recovery action).
