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

## v1.0.1 — Installer + Permissions Hardening

**Shipped:** 2026-05-12
**Tag:** `v1.0.1` (binary release pushed 2026-05-11 by Phase 6 ship gate)
**Archive:** [`milestones/v1.0.1-ROADMAP.md`](milestones/v1.0.1-ROADMAP.md) · [`milestones/v1.0.1-REQUIREMENTS.md`](milestones/v1.0.1-REQUIREMENTS.md) · _no audit (skipped per yolo-mode close; phase verifier already PASSED 5/5 must-haves on Phase 8 + 5/5 hooks on Phase 7 dev-workbook smoke)_

**Stats:** 3 phases · 12 plans · 63 commits · 2 calendar days kickoff to ship (2026-05-11 → 2026-05-12) · +318 LOC Go watcher (14,507 total) · +~2k LOC TypeScript (13,266 total) · +39 vitest tests (297 → 336)

**Phases:** All shipped sequentially.

| Phase | Name | Shipped As | Date |
|-------|------|-----------|------|
| 6 | Installer Overwrite-Running Shim | Watcher binary `v1.0.1` (GitHub Release) | 2026-05-11 |
| 7 | Admin Allowlist + Eviction Enforcement | `clasp push` to dev workbook (5/5 hooks PASS) | 2026-05-12 |
| 8 | Test Infra + Persistence + Docs Backfill | `clasp push` + 336/336 vitest green | 2026-05-12 |

**Key accomplishments:**

1. **Installer upgrade path hardened** — NSIS pre-install shim signals the running watcher to exit gracefully (new `internal/system` Windows named-event IPC package), polls for process exit up to 10s, falls back to `taskkill /F`. Manual stop workaround retired from `docs/troubleshooting.md`. Shipped + UAT-verified as tag `v1.0.1` on Azure VM (both `--quit` and legacy paths exercised against real running watchers).
2. **Officer-only eviction enforced by code** — `_meta.guild_admins` allowlist + `_meta.workbook_owner_floor` + `_meta.admin_log` extend-only `_meta` rows; eviction sidebar opener + all 3 `google.script.run` callbacks gated by `requireAdminOrThrow` as first statement. Non-admins see "Not authorized" modal + zero writes (verified live in dev-workbook).
3. **Admin-management UX with owner-floor lockout protection** — `showAdminMgmtSidebar` HtmlService sidebar; workbook owner cannot be removed by anyone other than themselves (server-side `'owner_floor_protected'` throw + client-side Remove-button suppression + visual `(owner)` annotation).
4. **Sidebar inline-JS test infrastructure** — JSDOM in vitest at top level; `mountSidebar()` helper with indirect-eval + nested-`<script>` extraction; 4 net-new sidebar inline-JS test files (Search, Eviction, Bank-Coin, Char-Info) → 5/5 shipping sidebars covered (Admin-Mgmt via Phase 7's trigger-call test).
5. **Recent-search MRU survives CacheService TTL** — `searchIndex.ts` MRU migrated from `CacheService.getDocumentCache()` (25-min, document-scoped) to `PropertiesService.getUserProperties()` (durable per-user); confirmed by new `vi.useFakeTimers()` 25-min persistence test.
6. **v1.0 documentation debt retired** — 8 retroactive Phase 3+4 SUMMARY.md files following the Phase 5 template byte-for-byte; every `metrics.commits` SHA resolves under `git cat-file -e` (0 invented hashes).
7. **Latent v1.0 OAuth-scope bug retired** — `apps-script/appsscript.json` was missing `userinfo.email`; under consumer @gmail.com `Session.getEffectiveUser().getEmail()` silently returned empty. Fix in commit `544bef8`. Side effect: closes the v1.0-era silent `initiated_by='unknown'` audit-log fallback for all post-Phase-7 deploys.

**Requirements coverage:** 8 / 8 (all complete; zero partials; zero waivers; zero orphans). See archive for full reconciliation.

**Status:** SHIPPED. Schema gates untouched throughout (`_meta.schema_version=3`, `WatcherMaxSchemaVersion=3`). Code review: 0 critical, 4 warning, 6 info (test-quality advisory only; none blocking).

**Deferred to v1.0.2 / v1.1 / v2:** 4 v1.0.2 candidates surfaced during Phase 6 UAT (999.13–999.16: Reauthorize on boot-time `invalid_grant`, tray `OnReady` queuing, UTF-8 BOM stripping in config loader, console-detach or `Start-Process` documentation); Admin-Mgmt sidebar inline-JS test coverage (999.17); Phase 8 advisory test-quality findings (999.18); SignPath OSS approval still in flight (999.9); v1.1 polish (bank-coin permission lock 999.1, theme picker tile UI 999.2, `SIDEBAR_BODY` extraction 999.7); v2 Wantlist + Discord pinger (999.12 / WANT-01..08).

---

*This file accumulates one entry per shipped milestone. Next entry will be v1.0.2 (patch) or v1.1 (feature) or v2 (Wantlist + Discord) — start via `/gsd-new-milestone`.*
