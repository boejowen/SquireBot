# Requirements: SquireBot — Milestone v2.5 "Ownership Cleanup"

**Milestone goal:** Reconcile character-ownership semantics for a guild that shares P99 logins — make guild banks/bots owner-less, and stop eviction from over-deleting shared characters. Backend-only; one schema migration; watcher untouched (no `v*` tag).

**Origin:** Promotes backlog items 999.35 + 999.36, both deferred from quick task `260621-u6j` (which dropped the single-owner write gate so any guild member can upload any character). Context: memory `cross-owner-character-misbinding.md`; code in `internal/backendsrv/store/{binding.go,eviction.go,assignment.go,charmeta.go}` + `internal/backendsrv/webadmin/{eviction.go,assignment_admin.go}`.

## v2.5 Requirements

### Ownership (OWN)

- [ ] **OWN-01** — An officer can designate any character as a guild bank or bot without first "owning" it; designation is decoupled from upload-ownership (no "claim" step).
- [ ] **OWN-02** — A designated guild bank/bot is owner-less (guild-held) and is NOT removed or archived when the member who first uploaded it is evicted.
- [ ] **OWN-03** — Evicting a guildie removes only that member's own characters, not shared characters that other guildies also play and upload.
- [ ] **OWN-04** — Existing designated banks/bots bound to an individual owner (e.g. `Findom` → owner 9) migrate to the owner-less model automatically, with no manual fixup.

## Future Requirements (deferred)

- **Self-service eviction** (backlog 999.5) — a departing guildie leaves cleanly without an officer action; threat-model deferred. Not in v2.5.

## Out of Scope (v2.5)

- **Any watcher change** — backend-only milestone (the single-owner write gate was already relaxed in `260621-u6j`, shipped in watcher `v2.1.2`).
- **Re-introducing per-character write ownership / cross-owner rejection** — explicitly reversed; the guild shares characters by design.
- **The v2.2 Track-2 Discord pinger** (invite-gated) — unrelated.

## Traceability

| REQ-ID | Phase | Status |
|--------|-------|--------|
| OWN-01 | 35 | Pending |
| OWN-02 | 35 | Pending |
| OWN-04 | 35 | Pending |
| OWN-03 | 36 | Pending |

*Created 2026-06-22 for milestone v2.5 (promotes backlog 999.35 / 999.36).*
