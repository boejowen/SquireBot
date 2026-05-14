---
name: When Google OAuth suddenly stops working, check Verification status first
description: Debugging guidance for sudden OAuth failures on a previously-working Google OAuth client — check consent-screen Verification status BEFORE chasing client_id, client_secret, redirect_uri, or scope theories.
type: feedback
originSessionId: 956afeb5-fe4c-4f34-b6f4-683d331ae7b2
---
When `Access blocked: Authorization Error / Error 400: invalid_request` or `Error 401: invalid_client` errors hit Google OAuth from a SquireBot watcher OR from a manually-constructed diagnostic URL, the first thing to check is **OAuth consent screen → Verification status** in Google Cloud Console — NOT the client_id, client_secret, redirect_uri, or scope configuration.

**Why:** During the v1.0.2 ship on 2026-05-13, the watcher OAuth stopped working overnight. I (Claude) spent ~2 hours chasing three false leads — (1) deleted client_id, (2) multiple-secrets-warning policy enforcement, (3) loopback-IP-vs-localhost redirect URI migration — before discovering that the actual cause was Google's brand-verification gate flipping enforcement on this client. The Verification status field showed "Your branding needs to be verified before it can be shown to users." That single field would have saved hours if checked first. The `consent_screen_status: PRODUCTION` field in `oauth-config.json` does NOT imply verification status — those are independent Google workflows.

**How to apply:**
1. For any "OAuth was working yesterday and isn't today" incident on a SquireBot watcher: open Cloud Console → APIs & Services → OAuth consent screen → scroll to **Verification status**. If it says anything other than "Verified," that is almost certainly the cause regardless of which OAuth error code Google returned.
2. The OAuth client_id and client_secret being baked into the binary correctly can be verified in seconds via `grep -aoE "[0-9]+-[a-z0-9]+\.apps\.googleusercontent\.com" <binary>`. Always do this BEFORE theorizing about other causes — it rules out the build pipeline definitively.
3. When constructing a diagnostic OAuth URL for a user to test, beware: error pages render URLs with `<wbr>` zero-width word-break opportunities at `.` and `:`, which appear as visible spaces on copy-paste. The actual on-wire URL is fine; do not treat the displayed-space as evidence of URL corruption.
