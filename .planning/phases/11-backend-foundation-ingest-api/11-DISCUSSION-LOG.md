# Phase 11: Backend Foundation + Ingest API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-28
**Phase:** 11-backend-foundation-ingest-api
**Areas discussed:** Server foundation, Ingest payload shape, Guild-code lifecycle, Deploy & backup

**How decided:** The user was presented all four gray areas and responded *"I have no preference for any of the questions you asked me. Please use your recommendations for each."* All four were therefore locked at Claude's recommendation in one pass (per the user's standing delegation pattern). Options considered are preserved below.

---

## Server foundation — PocketBase vs hand-rolled Go

| Option | Description | Selected |
|--------|-------------|----------|
| 1-day PocketBase spike first | Time-box evaluating PocketBase (pre-built SQLite+auth+REST+admin UI); could save ~5–8 days across P11+P15; hand-rolled Go as fallback | ✓ |
| Commit to hand-rolled Go now | Full control, mirrors the watcher's shape, zero framework dependency | |
| Commit to PocketBase now | Fastest, least code, but framework lock-in unverified against bearer-token + enrichment-hook needs | |

**User's choice:** Delegated to Claude's recommendation → spike-first (Plan 11-01), hand-rolled Go fallback, with explicit 4-point pass criteria. Host (Oracle Always Free) + DB (SQLite) stand either way.
**Notes:** Hand-rolled fallback shape also locked: Go stdlib `net/http` ServeMux + pure-Go `modernc.org/sqlite` (no cgo).

---

## Ingest payload shape

| Option | Description | Selected |
|--------|-------------|----------|
| Raw file text, parse server-side | Thinnest watcher; one parser shared with P12 enrichment | ✓ |
| Pre-parsed rows | Watcher keeps its parser; server just stores | |

**User's choice:** Delegated → raw file text in a JSON envelope `{character, kind, content, watcher_version}`; server parses + atomically replaces.
**Notes:** Char name travels in the envelope (it comes from the watcher's filename, not the file body).

---

## Guild-code lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| CLI mint + Discord DM + first-sighting bind + hash-revoke | Server CLI prints plaintext once, stores SHA-256 hash; first upload binds char→owner; revoke by disabling the hash | ✓ |
| Admin HTTP endpoint for minting | Web-based minting in P11 | (deferred to P15) |

**User's choice:** Delegated → CLI mint + Discord DM + first-sighting bind + disable-by-hash revoke. Bearer auth; 401 on missing/bad/unknown/revoked.
**Notes:** Admin HTTP forms (incl. character reassignment) are P15, not P11.

---

## Deploy & backup

| Option | Description | Selected |
|--------|-------------|----------|
| Bare binary + systemd + Caddy; nightly cron → Object Storage | Single-binary ethos; embedded goose migrations on startup; nightly `sqlite3 .backup` to Oracle Object Storage | ✓ |
| Docker/compose; Litestream continuous replication | More moving parts; tighter RPO | (Litestream noted as future upgrade) |

**User's choice:** Delegated → bare binary + systemd + Caddy; nightly cron backup to Oracle Object Storage (Always Free); documented restore.
**Notes:** Oracle instance locked at `VM.Standard.A1.Flex` 1 OCPU / 6 GB, US home region.

## Claude's Discretion

All four areas above were delegated to Claude's recommendation. Supporting technical choices also locked at discretion: `modernc.org/sqlite` driver, stdlib ServeMux, embedded `goose` migrations, the SQLite schema shape (owner/character split + empty dimension tables), and the `/api/v1/ingest` envelope contract.

## Deferred Ideas

- Admin HTTP minting / character reassignment → P15
- Read API + views → P14 (BACKEND-05)
- Enrichment cron jobs + dimension population → P12
- Watcher re-target / OAuth deletion / onboarding → P13
- Litestream continuous replication → future backup upgrade
