---
created: 2026-05-29T16:01:50.354Z
title: Add api.squirebot.quest A-record after host (Hetzner) provision
area: deploy
files:
  - .planning/phases/11-backend-foundation-ingest-api/11-CONTEXT.md:41
  - .planning/phases/11-backend-foundation-ingest-api/11-06-PLAN.md:138
---

## Problem

Phase 11's backend serves at **`api.squirebot.quest`** (CONTEXT D-14). Caddy issues its
Let's Encrypt TLS cert via the HTTP-01 challenge, which requires the hostname to resolve to
the server — i.e. a DNS **A-record `api` → the backend VPS's public IPv4**. That record
**cannot be created yet**: it needs the box's public IP, and the **Hetzner Cloud VPS** isn't
provisioned yet.

> **Host note:** the backend host was switched **Oracle Cloud Always Free → Hetzner Cloud VPS
> on 2026-05-29** (see CONTEXT D-12 host-change banner). The original blocker was Oracle's
> maintenance window; that's moot now — the only thing left is to provision the Hetzner box
> and point DNS at it. (Filename still says "oracle" — slug kept stable; the host is Hetzner.)

Without this record, Wave 5 (plan 11-06, on-box deploy) cannot complete — Caddy will fail to
obtain the cert and the HTTPS ship gate (11-07) can't pass.

Domain `squirebot.quest` is already registered at **Porkbun** (2026-05-29). The apex + `www`
are deliberately reserved for the P14 SvelteKit frontend (Cloudflare/Pages), so only the
`api` subdomain points at the backend box.

## Solution

**Trigger:** the Hetzner Cloud VPS is provisioned (Phase 11, Wave 5 / plan 11-06 Task 2 —
`autonomous: false`, execute-phase pauses here for human action).

1. Provision the Hetzner Cloud VPS (shared-vCPU, ~2 vCPU / 4 GB, **US** location → x86/AMD
   `CPX` line per D-12); note its public IPv4. Always-on — no idle-reclamation to manage.
2. Add the DNS **A-record `api` → `<that public IPv4>`** wherever DNS is managed — Porkbun
   now, or Cloudflare later if nameservers move for the P14 frontend.
3. Proceed with 11-06: Caddy then auto-issues the `api.squirebot.quest` cert via HTTP-01.
   **No Porkbun/registrar API token is needed** (DNS-01 / wildcard not used).

Cross-ref: `11-CONTEXT.md` D-12/D-14 and `11-06-PLAN.md` Task 2 (resume-signal expects the
public IP + domain).
