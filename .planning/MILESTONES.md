# SquireBot Milestones

Historical record of shipped versions. Each entry links to the milestone archive in `.planning/milestones/`.

---

## v1.0 — Watcher + Workbook + Onboarding (initial release)

**Shipped:** 2026-05-11
**Tag:** `v1.0.0` (additionally `phase5-complete`)
**Archive:** [`milestones/v1.0-ROADMAP.md`](milestones/v1.0-ROADMAP.md) · [`milestones/v1.0-REQUIREMENTS.md`](milestones/v1.0-REQUIREMENTS.md) · [`milestones/v1.0-MILESTONE-AUDIT.md`](milestones/v1.0-MILESTONE-AUDIT.md)

**Stats:** 5 phases · 31 plans · 203 commits · 14k LOC Go (watcher) + 11k LOC TypeScript (apps-script) · 11 days kickoff to ship (2026-04-30 → 2026-05-11)

**Phases:** All shipped sequentially.

| Phase | Name | Shipped As | Date |
|-------|------|-----------|------|
| 1 | End-to-End Thin Slice | v0.1.0 | 2026-05-02 |
| 2 | Watcher Robustness + Schema Lock | v0.2.0 / v0.2.1 hotfix | 2026-05-09 |
| 3 | Apps Script Enrichment Foundation | v0.3.0 | 2026-05-10 |
| 4 | Differentiator Features | v0.4.0 | 2026-05-11 |
| 5 | Search + Onboarding + Privacy Polish | v1.0.0 | 2026-05-11 |

**Key accomplishments:**

1. **End-to-end thin slice** — installer + OAuth + watcher + workbook in 5-step user flow, no UAC, ~12 MB binary, Azure VM smoke 16/17 PASS
2. **Hardened watcher** — spellbook + multi-folder + retry/backoff + heartbeat + auto-update + schema lock at `schema_version=1`; survived 7-day soak with deliberate `invalid_grant` injection
3. **Apps Script enrichment layer** — daily PigParse + weekly P1999 wiki summary triggers; consolidated `view` tab with hyperlinks, prices, conditional Last-Synced formatting, cell-note tooltips; `politeFetch` helper with ETag/If-Modified-Since
4. **Gear + spell differentiators** — wiki gear-tier scraper (Velious Pre-Raid + Raiding + Iksar) + per-class spell-list scraper (all 4 P1999 template variants, 11 caster classes, 1,562 spells); `gear_check` / `spell_check` consolidated tabs with OK/MISSING/OTHER status; manual bank coin sidebar with `Range.protect()`
5. **Search + eviction + onboarding** — 300px HtmlService cross-character search sidebar (Wagner-Fischer Levenshtein "Did you mean?"); lock-guarded eviction workflow with 30-day grace + lazy `_archive` tab + `_meta.eviction_log` envelope contract; Jekyll Pages onboarding site live at https://boejowen.github.io/SquireBot/
6. **End-to-end validation** — chained live smokes v0.1.0 → v0.4.0 + DOC-02 fake-guildie eviction smoke (all 7 checkpoints PASS) + 297/297 vitest tests at code-complete

**Effective requirements coverage:** 69 / 69 (66 satisfied + 2 partial via user-authorized scope reductions + 1 waived by user). See archive for full reconciliation.

**Status:** SHIPPED. Tech-debt acknowledged in audit (REQUIREMENTS.md drift, Phases 3+4 SUMMARY.md gaps, no VERIFICATION.md workflow). Cleanup carried into archive; not blocking.

**Deferred to v1.0.1 / v2:** Self-service eviction, `_meta.guild_admins` allowlist, polished theme picker tile UI, sidebar inline-JS unit tests, installer-driven upgrade UX (NSIS overwrite-running shim), PropertiesService recent-query persistence, SignPath OSS approval (in flight), v2 Wantlist + Discord pinger (WANT-01..08).

---

*This file accumulates one entry per shipped milestone. Next entry will be v1.0.1 (patch) or v1.1 (feature) — start via `/gsd-new-milestone`.*
