---
phase: 14-web-frontend
audited: 2026-05-30
asvs_level: 1
block_on: high
threats_total: 19
threats_closed: 19
threats_open: 0
status: SECURED
mitigated: 14
accepted: 5
transferred: 0
deploy_gated: 1   # T-14.03-06 (Go side correct; Caddy non-duplication is an operational deploy check)
source_plans:
  - .planning/phases/14-web-frontend/14-01-PLAN.md
  - .planning/phases/14-web-frontend/14-02-PLAN.md
  - .planning/phases/14-web-frontend/14-03-PLAN.md
  - .planning/phases/14-web-frontend/14-04-PLAN.md
corroborating_reviews:
  - .planning/phases/14-web-frontend/14-REVIEW.md      # 0 Critical; XSS/CORS/SQL held under adversarial read
  - .planning/phases/14-web-frontend/14-REVIEW-FIX.md  # WR-01 scheme allow-list pre-resolved (d14e4ab); WR-02/03/04 fixed
---

# Phase 14 (Web Frontend) — Security Verification

**Method:** FORCE stance. Every declared mitigation was treated as absent until proven by a code match in the file cited in the mitigation plan. Implementation files were read-only throughout; no patches applied. Documentation (PLAN/SUMMARY prose) was NOT accepted as evidence — only the actual code (the `escapeHtml`/`safeHttpUrl` calls, the `?`-placeholder SQL, the exact-origin CORS header, the sole `{@html}` sink, the `resolveTheme` whitelist, the noindex/robots files) was accepted. The prior `14-REVIEW.md` (0 Critical) was corroborated, not re-derived.

**Scope:** 19 registered threats across 4 STRIDE registers (14-01..04). ASVS Level 1, `block_on: high`. The two HIGH-severity gates (T-14.02-01 / T-14.04-01, the tooltip stored-XSS chain) were verified end-to-end: the escaping function, the lone injection sink, and the boundary between them.

## Verdict

**All 19 threats CLOSED. 0 open. No implementation gap.** The two HIGH-severity XSS mitigations are present in code (not just asserted): `escapeHtml` runs on every interpolated value in `composeNotes.ts`, the `safeHttpUrl` http(s) allow-list (the WR-01 fix, commit `d14e4ab`) guards both wiki-href sinks, and an app-wide grep proves `{@html}` appears in exactly one place (`ItemTooltip.svelte:106`), fed only by the escaped `composeItemNote`. Phase 14 may ship under `block_on: high`.

## Threat Verification Register

| Threat ID | Category | Disposition | Status | Evidence (file:line — verified in code) |
|-----------|----------|-------------|--------|------------------------------------------|
| T-14.01-01 | Tampering (SQL injection, store reads) | mitigate | **CLOSED** | `store/readviews.go` — all 8 methods use `?`-placeholder / fixed-literal SQL; `bankOnly` is a fixed-string branch (`:133-137`, two complete query literals, no value interpolation); grep for `fmt.Sprintf`/`+`-concat/`strings.Join` returned **0**. Every query is a `SELECT`. |
| T-14.01-02 | Info disclosure (store logs) | mitigate | **CLOSED** | `store/readviews.go` — no happy-path logging; errors are `%w`-wrapped only (`:141,165,180,...`), never row content (V7). Header documents the discipline (`:17-18`). |
| T-14.01-03 | Tampering (downstream XSS — raw strings) | accept (boundary owned downstream) | **CLOSED** (accepted risk #1) | `compute/view.go:73` copies `WikiSummary` raw; `compute/types.go:60` documents the structs carry RAW wiki/user strings. Escaping is owned downstream and **verified present** there (composeNotes `escapeHtml`, Svelte `{}`). Deliberate trust-boundary split — logged in Accepted-Risks. |
| T-14.01-04 | Elevation (read methods) | mitigate | **CLOSED** | `store/readviews.go` — every method is a pure `SELECT`; grep for `INSERT/UPDATE/DELETE/Exec` in `readapi/` hit **only** `readapi_test.go` seed helpers, zero in production handlers/reads. |
| T-14.02-01 | Tampering/Elevation (stored XSS, composeNotes) | mitigate (**HIGH**) | **CLOSED** | `web/src/lib/tooltip/composeNotes.ts` — `escapeHtml` (`:53-60`, `&`-first, ASVS V5) applied to item name (`:98`), wiki href (`:103`), summary (`:111`), prices (`:122-129`), quest names (`:146`); `safeHttpUrl` http(s) allow-list (`:69-72`) gates the href (`:99-103`) — the **WR-01 fix, `d14e4ab`**; wiki anchor carries `rel="noopener" target="_blank"` (`:103`). |
| T-14.02-02 | Tampering (tab-nabbing, tooltip wiki `<a>`) | mitigate | **CLOSED** | `composeNotes.ts:103` — `target="_blank" rel="noopener"` on the wiki anchor. |
| T-14.02-03 | Spoofing (theme value injection) | mitigate | **CLOSED** | `web/src/lib/theme/themes.ts:119-123` — `resolveTheme` whitelists against the 5-key `THEME_KEYS`, falls back to `DEFAULT_THEME='velious'` (`:27`); `applyTheme` routes through it (`:148-149`). No arbitrary value reaches `[data-theme]`. |
| T-14.02-04 | Info disclosure (search-engine indexing) | mitigate | **CLOSED** (D-05) | `web/src/app.html:8` `<meta name="robots" content="noindex" />`; `web/static/robots.txt:3-4` `User-agent: *` / `Disallow: /`. |
| T-14.02-05 | Info disclosure (Off-Google supply chain, fonts) | mitigate | **CLOSED** | `web/src/app.css:12-21` — 10 self-hosted `@fontsource/*` woff2 imports; grep for `fonts.googleapis.com`/`fonts.gstatic.com` returned **0** runtime links. |
| T-14.03-01 | Spoofing/Info disclosure (CORS) | mitigate | **CLOSED** | `readapi/cors.go:37-40` — `Access-Control-Allow-Origin` echoes the exact `allowOrigin` param (never `*`), `Vary: Origin`, methods/headers scoped; **no** `Access-Control-Allow-Credentials`. `main.go:57` locks the default to `https://app.squirebot.quest`. |
| T-14.03-02 | Info disclosure (data public by design) | accept (time-boxed) | **CLOSED** (accepted risk #2) | `readapi/views.go:70-71` + `meta.go:57` — bearer guard deliberately dropped (D-04); `cors.go:6-9` documents public posture; P15 AUTH-08 gates it; D-05 noindex active. Logged in Accepted-Risks. |
| T-14.03-03 | Elevation (handlers) | mitigate | **CLOSED** | `readapi/views.go:65-68` + `meta.go:52-55` — GET-only 405 guard; all dispatch to SELECT-backed compute/store; grep for `INSERT/UPDATE/DELETE` in `views.go`/`meta.go` = **0**. |
| T-14.03-04 | Tampering (SQL injection, handlers) | mitigate | **CLOSED** | `readapi/views.go`/`meta.go` take no server-side user input in P14 (no query-param filters); all SQL is parameterized in `store/readviews.go` (inherits T-14.01-01, verified). |
| T-14.03-05 | Info disclosure (handler logs) | mitigate | **CLOSED** | `readapi/views.go:118,125,135,138` + `meta.go:62,77,80` — slog carries op/view/rows/status/err only, never row content (V7); header documents it (`views.go:13-16`). |
| T-14.03-06 | Tampering (CORS header duplication, Caddy) | mitigate (**deploy-time**) | **CLOSED** (deploy-gate item — see below) | `main.go:283-290` — CORS set **once** in Go (`Handler: readapi.CORS(*corsOrigin, mux)`); inline comment + `cors.go:20-24` flag the Caddy-must-not-also-emit check. Go side is correct in code; the on-box Caddy non-duplication is an operational deploy verification, not a code gap. |
| T-14.04-01 | Tampering/Elevation (stored XSS, ItemTooltip `{@html}`) | mitigate (**HIGH**) | **CLOSED** | `web/src/lib/components/ItemTooltip.svelte:106` `{@html bodyHtml}` is the **sole** `{@html}` directive app-wide (grep of `web/src` — every other hit is a comment); `bodyHtml` is `composeItemNote(...)` (`:45`), fully escaped (T-14.02-01). |
| T-14.04-02 | Tampering (reflected XSS, search query) | mitigate | **CLOSED** | `SearchResults.svelte:100` passes `{query}` (plain interpolation) to StateBlock; `StateBlock.svelte:69` renders `No matches for "{query}"` via Svelte auto-escape; the "Did you mean" affordance is a `<button onclick>` (`SearchResults.svelte:103`), not an href/`{@html}`. Zero `{@html}` in either file. |
| T-14.04-03 | Tampering (tab-nabbing, grid + tooltip wiki `<a>`) | mitigate | **CLOSED** | `cells/WikiCell.svelte:18` `target="_blank" rel="noopener"` + `safeHttpUrl` `$derived` guard (`:14`, renders only `{#if safeUrl}`); `columns.ts:90` delegates the Wiki cell to `WikiCell`; tooltip anchor inherits `composeNotes.ts:103`. |
| T-14.04-04 | Spoofing (theme injection) | mitigate | **CLOSED** | `themes.ts:148-153` `applyTheme` resolves through `resolveTheme` (5-key whitelist → velious fallback) before the single `[data-theme]` write (inherits T-14.02-03). |
| T-14.04-05 | Info disclosure (data public by design) | accept (time-boxed) | **CLOSED** (accepted risk #2, inherited) | Inherits T-14.03-02 (D-04 public read, P15 AUTH-08 gate, D-05 noindex). Logged in Accepted-Risks. |

**Disposition tally:** 14 mitigate (all verified in code) · 5 accept (all logged below) · 0 transfer. Of the 14 mitigations, 1 (T-14.03-06) additionally carries a deploy-time operational verification.

## Accepted Risks Log

These are the explicit, time-boxed / by-design accepted risks declared in the threat registers. Recording them here counts them CLOSED (per the phase's accept disposition); they are tracked, not gaps.

### AR-1 — Read API returns raw, un-escaped strings (downstream XSS boundary)
- **Threat:** T-14.01-03 (Tampering / downstream XSS).
- **Why accepted:** The Go data/compute layer is deliberately NOT the escaping layer (mirrors the v1 `searchIndex.ts` trust-boundary note). `compute` row structs carry raw wiki/user strings (`compute/view.go:73`, documented in `compute/types.go:60`).
- **Compensating control (verified present, not just promised):** the escaping obligation lands on the client and **is implemented** — `composeNotes.ts` `escapeHtml`/`safeHttpUrl` for the tooltip HTML, and Svelte `{}` auto-escaping for every other rendered string (search query, grid cells). The boundary is honored end-to-end.
- **Status:** Accepted. Not an oversight.

### AR-2 — P14 read endpoints + rendered site are unauthenticated (public by design)
- **Threats:** T-14.03-02 and T-14.04-05 (Info disclosure — data public by design).
- **Why accepted:** Per D-04, the low-sensitivity EQ inventory (character names + item lists) is public in P14 to unblock the ~12-guildie roll-out now, consistent with the guild's universal-visibility ethos. The bearer guard is intentionally dropped on the read handlers (`readapi/views.go:70-71`, `meta.go:57`).
- **Time-box / exit:** P15's **AUTH-08** (Discord login) walls read access to guild members; the exact-origin CORS echo keeps the credentialed upgrade a one-line change (`cors.go:11-18`).
- **Compensating controls (verified):** exact-origin CORS, no `Allow-Credentials` (T-14.03-01); `noindex` + `robots Disallow` keep it off search engines (T-14.02-04).
- **Status:** Accepted, time-boxed to P15.

## Deploy-Time Verification Gate

### DG-1 — Caddy must not also emit `Access-Control-Allow-Origin`
- **Threat:** T-14.03-06 (Tampering / CORS header duplication).
- **Code state (CLOSED):** CORS is set exactly once, in Go (`main.go:290` wraps the whole mux in `readapi.CORS`; `cors.go` is the single emitter). The build artifact is correct.
- **Operational step the maintainer performs at deploy:** on the Hetzner VPS, confirm the Caddyfile `reverse_proxy` block fronting 443 → `127.0.0.1:8090` adds **no** `Access-Control-Allow-Origin` header — a duplicated header ("origin, origin") makes the browser reject the response. Flagged in `main.go:283-286` and `cors.go:20-24`.
- **Classification:** This is a deploy-gate item, NOT an open code gap. The mitigation lives correctly in the code; the non-duplication check is an environment verification the maintainer runs when dropping the binary + the public Cloudflare Pages cutover (mirroring P11's manual-deploy posture). Recorded here so it is not lost.

## Unregistered Flags (new attack surface with no threat mapping)

**None.** Every `## Threat Flags` / `## Threat Surface` section across the four SUMMARYs maps cleanly to a registered threat ID:
- 14-01 SUMMARY "Threat Surface" → T-14.01-03 (no new HTTP/auth/migration surface).
- 14-02 SUMMARY → the three ported modules map to T-14.02-01..05.
- 14-03 SUMMARY "Threat Surface" → "No threat flags (no surface outside the threat model)"; the 5 new endpoints are all covered by T-14.03-01..06.
- 14-04 SUMMARY "Threat Flags" → "None"; components map to T-14.04-01..05.

The 5 public read endpoints (the only genuinely new external surface in the phase) are fully enumerated in the T-14.03 register — registered, not unregistered.

## Corroboration with Prior Review

`14-REVIEW.md` (standard depth, 47 files, **0 Critical**) independently confirmed the four high-risk areas hold up under adversarial reading: the single `{@html}` sink, the never-wildcard CORS, the `?`-only SQL with the fixed-string `bankOnly` branch, and v1 parity. `14-REVIEW-FIX.md` confirms the WR-01 wiki-URL scheme allow-list was already resolved (`d14e4ab`) before that pass — and this audit independently re-verified `safeHttpUrl` is present and wired at **both** sinks (`composeNotes.ts:69-72,99-103` and `WikiCell.svelte:14,18`). The remaining open Info items (IN-02..IN-05) are cosmetic/UX and carry no security disposition. Nothing in the review contradicts a CLOSED verdict.

## Audit Trail

- Loaded all 8 required-reading artifacts (4 PLAN `<threat_model>` blocks, 4 SUMMARYs) + both prior review files before any verification.
- Verified Go layer in code: `store/readviews.go` (parameterized SQL, fixed bankOnly branch, SELECT-only, error-only logging), `readapi/cors.go` (exact origin, no credentials, OPTIONS 204), `readapi/views.go` + `meta.go` (405 guard, no writes, V7 logging), `cmd/squirebot-server/main.go` (single CORS wrap + Caddy comment + locked origin).
- Verified frontend in code: `composeNotes.ts` (escapeHtml + safeHttpUrl on every value/href), `ItemTooltip.svelte` (sole `{@html}`, proven by an app-wide grep), `WikiCell.svelte` + `columns.ts` (rel=noopener + scheme guard), `SearchResults.svelte` + `StateBlock.svelte` (query auto-escaped, no `{@html}`), `themes.ts` (resolveTheme/applyTheme whitelist), `app.html` + `robots.txt` (noindex + Disallow), `app.css` (self-hosted fonts, no Google CDN).
- No implementation file modified. Only this file (`14-SECURITY.md`) was written.

---

## SECURED

**Phase:** 14 — Web Frontend
**Threats Closed:** 19/19 (14 mitigated in code · 5 accepted-risk logged · 1 of the mitigated also deploy-gated) · **threats_open: 0**
**ASVS Level:** 1 · **block_on:** high

**Rationale:** Every declared mitigation was located in the cited implementation code — including both HIGH-severity XSS gates (the `escapeHtml`/`safeHttpUrl` chain in `composeNotes.ts` and the single `{@html}` sink in `ItemTooltip.svelte`, proven sole by an app-wide grep), the exact-origin/no-credentials CORS middleware, and the `?`-only parameterized reads with a fixed-string `bankOnly` branch — while the five public-data / downstream-boundary risks are recorded as explicit time-boxed accepted risks and the lone Caddy CORS-duplication item is correct in Go with its non-duplication check recorded as a deploy-time gate. No implementation gap; nothing to escalate; Phase 14 clears the `block_on: high` bar.
