---
name: Google's /token endpoint requires client_secret even with PKCE
description: Hard-won Phase 1 lesson. Google enforces client_secret as a parameter on Desktop OAuth clients even when PKCE is in use. Spec-correct ≠ contract-correct.
type: feedback
originSessionId: 21f7e633-9f81-410d-aaed-1ec2e411a483
---
For Google OAuth Desktop clients: **always include `ClientSecret` in `oauth2.Config`, even when PKCE is in use.**

**Why:** the OAuth 2.0 PKCE spec (RFC 7636) says `client_secret` is optional for public clients — but Google's `https://oauth2.googleapis.com/token` endpoint enforces it as a required parameter regardless. During the Phase 1 smoke (2026-05-01), the watcher hit a hard `oauth2: invalid_request: client_secret is missing` error after the browser consent step. Plan 02's runbook + `01-RESEARCH.md` §4.1 originally said "PKCE replaces client_secret for desktop clients" — true at the spec level, false at Google's enforcement level. Per Google's own docs (https://developers.google.com/identity/protocols/oauth2/native-app): *"when a client runs on a device, the `client_secret` is no longer truly confidential."* So treat the desktop secret as effectively public — bake it into the binary alongside the client ID via `-X main.OAuthClientSecret=...` ldflag.

**How to apply:** any time a future plan, doc, or research output claims that PKCE makes `client_secret` unnecessary for a Google OAuth flow — flag it as wrong and require the secret. Specifically:
- `oauth-config.json` schema_version 2 has `oauth_client_secret` as a load-bearing field
- `cmd/squirebot/build_constants.go` declares `OAuthClientSecret` alongside the other three constants
- `internal/auth/oauthconfig.go` `BuildConstants.Validate()` requires it
- `internal/auth/oauth.go` ~line 187 sets `ClientSecret: bc.OAuthClientSecret` in the `oauth2.Config` literal
- `docs/oauth-setup.md` Step 4 tells future maintainers to COPY (not ignore) the Client secret

This rule is Google-specific. Other OAuth providers (e.g. Auth0, Okta) do honor PKCE-without-secret for public clients per the spec. Don't generalize.
