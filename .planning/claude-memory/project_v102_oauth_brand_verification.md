---
name: v1.0.2 shipped + Google OAuth brand-verification incident
description: Phase 9 / v1.0.2 shipped 2026-05-13. Google flipped brand-verification enforcement on the OAuth client mid-ship, blocking auth for ALL watcher versions (v0.4.0-rc1, v1.0.1, v1.0.2) uniformly. Not a code bug. Resubmitted to Google review queue with new privacy policy + Search Console ownership proof.
type: project
originSessionId: 956afeb5-fe4c-4f34-b6f4-683d331ae7b2
---
**v1.0.2 SHIPPED 2026-05-13** — tag pushed, GitHub Release published (4 artifacts), Phase 9 verifier passed 5/5 must-haves programmatically. Phase 9 close-out committed and pushed.

**Active blocker: 999.19 Google OAuth brand verification.** Submitted to Google review queue 2026-05-13. ETA 3–5 business days. While in review, every SquireBot watcher in the guild loses auth on next token refresh (~1h TTL). Will unblock automatically when Google approves. No code fix needed.

**Why:** Google flipped brand-verification enforcement on the SquireBot Desktop Client OAuth client (`262087828393-8obvbca97eb1q73f2kna7nef4adhq7bu`) between v1.0.1 ship (2026-05-11) and v1.0.2 ship (2026-05-13). The `consent_screen_status: PRODUCTION` JSON field in `oauth-config.json` and the release.yml AUTH-03 gate only confirm publishing status, NOT Google's verification state — these are independent workflows. The release pipeline is innocent; the build pipeline correctly baked the same client_id/secret as v1.0.1.

**How to apply:** If a future session sees `Access blocked: Authorization Error / Error 400: invalid_request` or `invalid_client` errors hitting Google OAuth from the watcher OR from any diagnostic URL constructed by hand:
1. Check **OAuth consent screen → Verification status** FIRST in Google Cloud Console — this is almost always the cause for an existing-and-working app that suddenly stops working.
2. Do NOT chase: multiple-secrets warning, loopback-IP-vs-localhost theory, redirect_uri formatting. Those are red herrings I burned ~2 hours on during this incident.
3. The fix lives in Cloud Console (consent screen fields + Search Console ownership proof) — not in the watcher binary. Resubmit for verification and wait.
4. The privacy policy lives at `docs/privacy-policy.md` (served at `boejowen.github.io/SquireBot/privacy-policy/`). The Search Console verification HTML token is at `docs/google7ea0696846f966ed.html`. The homepage `docs/index.md` MUST link to the privacy policy or Google rejects.
5. Full incident trail (with all rejected hypotheses documented): `.planning/debug/v1-0-2-oauth-invalid-client-incident.md`.

**Hardening backlog items captured during this incident:** 999.19 (this), 999.20 (WR-01 gofmt drift), 999.21 (WR-02 FreeConsole doc/impl), 999.22 (SemVer-aware auto-update — dev `0.4.0-rc1` thinks it's newer than `1.0.x`), 999.23 (graceful tray messaging when Google's OAuth client is policy-blocked).
