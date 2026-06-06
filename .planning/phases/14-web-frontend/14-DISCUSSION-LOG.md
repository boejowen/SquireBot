# Phase 14: Web Frontend - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-30
**Phase:** 14-web-frontend
**Areas discussed:** Read access posture (1 of 4 surfaced; the other 3 delegated to Claude's recommendation)

---

## Area selection (multiSelect)

Four gray areas surfaced (each with a previewed recommendation): Compute split (server vs client), Read access posture, Theme catalog + default, Tooltip + search behavior.

| Option | Selected |
|--------|----------|
| Compute split: server vs client | (delegated → locked at lean) |
| Read access posture | ✓ |
| Theme catalog + default | (delegated → locked at lean) |
| Tooltip + search behavior | (delegated → locked at lean) |

**User's choice:** discussed *Read access posture* only; delegated the rest (standing `feedback_delegate_gray_areas` pattern).

---

## Read access posture — P14 lockdown level

| Option | Description | Selected |
|--------|-------------|----------|
| Public — open read | No gate; CORS-open read API; anyone with the URL sees the 4 views. | |
| Public now → Discord-gate in P15 | Ship open in P14 (unblock the guild), then P15's Discord login walls reads to guild members. | ✓ |
| Shared read-password now | API enforces one shared secret; site prompts once, stores in localStorage. | |

**User's choice:** Public now → Discord-gate in P15.
**Notes:** Fast availability now + guild-only later; adds a forward-dependency note to P15 scope, no extra P14 work. → CONTEXT D-04.

## Read access posture — search-engine indexing

| Option | Description | Selected |
|--------|-------------|----------|
| Keep it out (noindex + robots disallow) | `noindex` meta + `robots.txt` disallow; "public" = anyone with the link, not anyone who searches. | ✓ |
| Allow indexing | Let search engines crawl/index the site. | |

**User's choice:** Keep it out (noindex + robots disallow). → CONTEXT D-05.

---

## Claude's Discretion (delegated areas, locked at recommendation)

- **Compute split** → server-computed per-view read endpoints; gear/spell logic reimplemented in Go with v1 TS tests as oracle; search + tooltip presentation stay client-side via ported TS. (CONTEXT D-01/D-02/D-03)
- **Theme catalog + default** → 5 EQ themes (drop `sheets-default`), default `velious`, per-user localStorage, THEMES → CSS custom properties, Heavy textures via real CSS; self-host webfonts. (CONTEXT D-06/D-07)
- **Tooltip + search behavior** → hover-on-desktop + tap-on-touch rich-HTML popover with real wiki link; inline clickable "did you mean?" single-best fuzzy hit on no-exact-match; fix bugs 999.28 + 999.30 during the port. (CONTEXT D-08/D-09)
- Read-endpoint URL shapes / JSON field names / pagination; TanStack Table specifics (→ `/gsd-ui-phase 14`); self-host vs CDN fonts fallback.

## Deferred Ideas

- Discord-login gate on read access → **P15** (forward-dependency from D-04).
- Admin write forms (eviction / bank-coin / admin-mgmt) → **P15**; `bank` view coin is null/0 in P14 until ADMIN-05.
- Fancy theme-picker live-preview tiles → optional polish (P14 can ship a simpler picker).
- Detailed visual/interaction layout → `/gsd-ui-phase 14`.
- Cutover / shadow-soak / Sheet decommission → **P16**.
