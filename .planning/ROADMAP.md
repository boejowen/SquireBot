# Roadmap: SquireBot

**Created:** 2026-04-30
**Granularity:** coarse (per `config.json`)
**Mode:** yolo
**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

## Phases

- [x] **Phase 1: End-to-End Thin Slice** — Installer + OAuth + raw `inv:<Char>` landing tab; validates three of five existential pitfalls before work compounds. **(SHIPPED 2026-05-02, tagged `phase1-complete`.)**
- [x] **Phase 2: Watcher Robustness + Schema Lock** — Spellbook watcher, autostart, retry/backoff, refresh-token UX, heartbeat, auto-update, code signing, sheet schema frozen at `schema_version=1`. **(SHIPPED 2026-05-09 as v0.2.0 + v0.2.1 hotfix; soak Days 1/4/6/7 PASS; verdict at `docs/phase2-complete-verdict.md`.)**
- [x] **Phase 3: Apps Script Enrichment Foundation** — TypeScript/clasp scaffolding, daily PigParse + weekly wiki summaries, `_item_master`, consolidated `view` tab, cell-note tooltips, `bank` tab. **(CODE-COMPLETE 2026-05-09; watcher v0.3.0 shipped 2026-05-10. Apps Script side deployed per-workbook via `docs/apps-script-deploy.md`. SC-1 end-to-end verification = user smoke test pending; see `.planning/phases/03-apps-script-enrichment-foundation/03-SMOKE-TEST.md`. SC-6 courtesy emails WAIVED per user decision 2026-05-09.)**
- [ ] **Phase 4: Differentiator Features** — Wiki gear-tier and per-class spell scrapes, `gear_check` and `spell_check` consolidated tabs, manual coin sidebar, cell-count monitoring.
- [ ] **Phase 5: Search + Onboarding + Privacy Polish** — HtmlService search sidebar, custom menu, hide system tabs, weekly schema healthcheck, eviction workflow, README + screenshots + SmartScreen video, auto-archive stale chars.

## Phase Details

### Phase 1: End-to-End Thin Slice
**Goal**: A single guildie installs SquireBot, completes OAuth once, points at their EQ folder, runs `/outputfile inventory`, and within seconds sees raw TSV rows appear in `inv:<Char>` of the shared workbook.
**Depends on**: Nothing (first phase).
**Requirements**: INST-01, INST-02, INST-03, AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-06, WATCH-01, WATCH-04, OPS-01, OPS-03
**Success Criteria** (what must be TRUE):
  1. On a clean Windows 11 VM, running the installer `.exe` completes with no UAC prompt and no command-line steps; SquireBot launches automatically and surfaces a tray icon.
  2. The first-run flow opens the user's default browser exactly once for Google OAuth (loopback PKCE on a random port, `drive.file` scope only) and exactly once for Drive Picker workbook selection; the consent screen is in **Production** state, not Testing.
  3. Within 30 seconds of saving a `<Char>-Inventory.txt` to the configured EQ folder, an `inv:<Char>` tab containing the parsed five-column rows appears in the selected workbook, written to per-character non-overlapping ranges.
  4. The OAuth refresh token lives only in Windows Credential Manager (DPAPI via `wincred`) under `SquireBot:<email>`; nothing in `%LOCALAPPDATA%\SquireBot\config.json` would let an attacker impersonate the guildie.
  5. A 10-day-later re-test of the same install (no user action) shows refreshed-token writes still succeeding — proving the Testing-mode 7-day silent expiry has been escaped.
**Plans**: TBD
**Research flag**: not needed (standard installer/OAuth/file-watcher patterns; SUMMARY.md confidence HIGH).

### Phase 2: Watcher Robustness + Schema Lock
**Goal**: One guildie can install the watcher and not touch it for six months. Sheet schema is frozen at `schema_version=1` with all soft-delete and provenance fields scaffolded — even those not yet exposed by UI — so no later phase forces a destructive migration.
**Depends on**: Phase 1.
**Requirements**: INST-04, INST-05, AUTH-05, WATCH-02, WATCH-03, WATCH-05, WATCH-06, WATCH-07, WATCH-08, WATCH-09, SCHEMA-01, SCHEMA-02, SCHEMA-03, SCHEMA-04, SCHEMA-05, SCHEMA-06, SCHEMA-07, SCHEMA-08, OPS-04, OPS-05
**Success Criteria** (what must be TRUE):
  1. A 7-day continuous watcher run survives deliberate `invalid_grant` injection (tray turns red, click reopens OAuth, no silent retry-loop) and a deliberately corrupted update payload (no broken install state).
  2. After Windows logon SquireBot autostarts via `HKCU\...\Run` (no Task Scheduler, no Windows Service), and writes a once-daily heartbeat to `_char_owner.last_seen` for every active character even when source files have not changed.
  3. The workbook has the full schema present and frozen: per-character `inv:<Char>` and `spell:<Char>` landing tabs, every hidden `_`-prefixed dimension tab (`_meta`, `_char_owner`, `_item_master`, `_pigparse`, `_wiki_spells`, `_wiki_gear_tier`, `_quest_items`, `_audit`, `_status`), and the consolidated mega-tab placeholders (`view`, `gear_check`, `spell_check`, `bank`) — never per-character view tabs. `_meta.schema_version=1`, `_meta.canonical_id` populated.
  4. Sheets API errors trigger exponential backoff (2/4/8/16/32/60s), `Retry-After` is honored on `429`, token is refreshed once on `403`, and persistent failures surface via tray icon — verified by deliberate quota throttling.
  5. Auto-update pipeline pulls `latest.json` from GitHub Releases, verifies SHA-256, and atomically swaps the running binary on next start; either the binary is signed with a code-signing certificate OR the documented "More info → Run anyway" SmartScreen walkthrough completes in under 30 seconds.
  6. Spellbook file format is verified against a real EQ-produced sample before any column commits; soft-delete fields (`is_hidden`, `is_removed`) and the `discord_handle` column exist in `_char_owner` even though no v1 UI populates them.
**Plans**: 10 plans
- [ ] 02-01-PLAN.md — Schema scaffold + canonical-id three-state + Slot→Level rename (SCHEMA-01..08)
- [ ] 02-02-PLAN.md — Spellbook parser + multi-folder watcher + WATCH-09 catch-up (WATCH-02, 03, 05, 06, 09)
- [ ] 02-03-PLAN.md — Sheets retry/backoff + sheet.Client mutex (WATCH-07; closes Pitfall D)
- [ ] 02-04-PLAN.md — Refresh-token UX: invalid_grant detect + tray red + Reauthorize (AUTH-05)
- [ ] 02-05-PLAN.md — Heartbeat goroutine: 24h self-reschedule + _status writes (WATCH-08, OPS-05)
- [ ] 02-06-PLAN.md — Auto-update: startup-swap + latest.json + tray Check for updates (OPS-04)
- [ ] 02-07-PLAN.md — Autostart HKCU Run key + uninstaller checkbox (INST-04)
- [ ] 02-08-PLAN.md — goreleaser migration preserving AUTH-03 PRODUCTION gate
- [ ] 02-09-PLAN.md — SmartScreen walkthrough + SignPath OSS application (INST-05)
- [ ] 02-10-PLAN.md — 7-day soak runbook + injection scripts + assertions
**Research flag**: COMPLETE — see .planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md (code-signing matrix, retry schedule, heartbeat pattern, auto-update startup-swap). Spellbook fixture committed (Slampeach-Spellbook.txt, 49 rows).

### Phase 3: Apps Script Enrichment Foundation
**Goal**: A watcher upload becomes a tooltipped, priced, wiki-linked row in the consolidated `view` tab within ~30 seconds. PigParse pricing refreshes daily; per-item wiki summaries refresh weekly. The `bank` tab shows the bank toon's inventory with cell-note tooltips.
**Depends on**: Phase 2 (frozen schema; landing tabs producing data). Technically parallelizable with Phase 2 (Go vs TS, no shared code paths) — with one contributor, sequential is correct.
**Requirements**: ENRICH-01, ENRICH-02, ENRICH-05, ENRICH-06, ENRICH-07, ENRICH-08, ENRICH-09, VIEW-01, VIEW-02, VIEW-03, VIEW-04, TIP-01, TIP-02, TIP-03, OPS-02
**Success Criteria** (what must be TRUE):
  1. End-to-end test on a sample character: a watcher upload triggers `onChange` → consolidated `view` row appears within ~30s with hyperlink to P1999 wiki, current PigParse price, and a hover cell-note composing wiki summary + price summary + "Quest item: used in X, Y, Z" line where applicable.
  2. The daily PigParse trigger (~03:00 PT) hits `GET /api/item/getall/1` and writes the full price list to `_pigparse`; the weekly wiki trigger (Sunday ~04:00 PT) populates `_item_master` (per-item summaries) and `_quest_items`. Each scrape's row count assertion catches a deliberately truncated response and preserves last-known-good data while writing the failure to `_meta.last_error`.
  3. A long-running scrape interrupted at the 5-minute mark resumes correctly from the cursor stored in `PropertiesService` after the self-rescheduled trigger fires.
  4. All outbound HTTP goes through `politeFetch(url)` with identifying User-Agent, ETag/`If-Modified-Since`, `CacheService`, exponential backoff on `429/503/504`, and the 1-second `Utilities.sleep` between wiki requests — verified by inspecting Apps Script logs across one full refresh cycle.
  5. The `Last Synced` cell on every `view` row uses conditional formatting (green ≤7d, orange ≤30d, red >30d), and `LockService.getDocumentLock().tryLock(30000)` in `try/finally` guards every aggregate write that touches a shared range.
  6. Courtesy contact emails to the PigParse operator and the P1999 wiki admins are sent and acknowledged **before** the daily/weekly triggers fire against live infrastructure.
**Plans**: TBD
**UI hint**: yes
**Research flag**: needed — `/gsd-research-phase` should probe (a) actual JSON shape of `GET /api/item/getall/1` end-to-end with a real curl against PigParse, and (b) MediaWiki `api.php?action=parse&prop=wikitext` template shapes for per-item summary pages (infobox layout, summary paragraph extraction). Produces parser specs before code is written.

### Phase 4: Differentiator Features
**Goal**: Every guildie sees a per-slot "shopping list" of missing Velious gear and a level-aware list of trainable spells they don't yet know. **The reason SquireBot exists.** The shared bank toon's coin balance is editable through a guarded sidebar form.
**Depends on**: Phase 3 (`_item_master`, scrape harness, consolidated-view rendering pattern, `politeFetch`).
**Requirements**: ENRICH-03, ENRICH-04, BANK-01, BANK-02, BANK-03, BANK-04, CHECK-01, CHECK-02, CHECK-03, CHECK-04, CHECK-05, OPS-07
**Success Criteria** (what must be TRUE):
  1. For the developer's own characters, the `gear_check` consolidated tab shows accurate `MISSING` rows against `Velious Pre-Raid/Group`, `Velious Raiding`, and (for Iksar characters only) the `Iksar` racial tier — joined per-character per-slot against `_wiki_gear_tier`, with `Status` = `OK | MISSING | OTHER`.
  2. The `spell_check` consolidated tab shows accurate `KNOWN | MISSING` rows for every spell each character's class can train at or below their current level, joined on normalized spell name (since spellbook has no IDs); the wiki scrape covers all 14 classes within the trigger budget.
  3. A sidebar form sets `_char_owner.class` and `_char_owner.level` on first sighting of a character, and both `gear_check` and `spell_check` rebuild on `onChange` of any `inv:*` or `spell:*` landing tab and on every weekly `_wiki_*` refresh.
  4. The `bank` tab is visible to every guildie, displays the inventory of the character named in `_meta.bank_toon_name`, and shows a coin row (PP/GP/SP/CP) populated only via the custom-menu sidebar — raw cell edits to `bank_coin_*` are blocked by `Range.protect()`.
  5. Workbook cell count is monitored on the weekly trigger and surfaced in `_status`; an alarm threshold of 5M (vs the 10M cap) is configured and verified to trigger when synthetically loaded.
**Plans**: TBD
**UI hint**: yes
**Research flag**: needed — `/gsd-research-phase` must produce a parser spec for the Velious gear-tier wiki pages (`Players:Velious_Pre-Raid_Gear`, `Players:Velious_Raiding_Gear`, plus the Iksar racial tier) from real wikitext samples. Per SUMMARY.md, "the gear progression checklist is the headline differentiator; a parser regression here is high-blast-radius."

### Phase 5: Search + Onboarding + Privacy Polish
**Goal**: All 12 guildies are running SquireBot. Cross-character "who has Lustrous Russet Coat?" is answered in a sidebar in <2 seconds. The eviction workflow for a guildie leaving the guild is documented and tested. SmartScreen onboarding is non-scary.
**Depends on**: Phase 4 (consolidated views complete; `_char_owner` populated with class/level).
**Requirements**: SEARCH-01, SEARCH-02, SEARCH-03, SEARCH-04, TIP-04, VIEW-05, OPS-06, DOC-01, DOC-02, DOC-03
**Success Criteria** (what must be TRUE):
  1. The HtmlService search sidebar (~300px wide) opens from a custom `onOpen` menu item, and a query returns matches across every `inv:*` tab in the workbook in under 2 seconds, formatted as `<Item> (id <id>) → <Char>: <Location>, count <N>`, with the source character's last-sync staleness inline. Results are cached in `CacheService` for 60 seconds.
  2. All hidden `_`-prefixed system tabs are hidden by default; `Range.protect()` guards every cell whose mutation could break the build (notably `_meta.bank_toon_name` and the bank coin cells).
  3. A weekly Apps Script healthcheck verifies all expected tabs exist by ID and writes any missing-tab errors to `_meta.last_error`; characters with `inventory_mtime > 90d` are auto-archived to a hidden `_archive` tab.
  4. The eviction workflow is tested end-to-end on a fake guildie account: their email is removed from workbook share → all their characters are marked `is_removed` via the sidebar → 30-day grace observed → archive is automatic. Documented in DOC-02.
  5. README documents install flow, SmartScreen walkthrough (with screenshots/video link), OAuth flow, EQ folder picker, and the "tray turned red, what now?" recovery; onboarding screenshots and a short SmartScreen video are linked from the download page; all 12 guildies are installed and writing data.
**Plans**: 5 plans
- [x] 05-01-PLAN.md — System tab hide + bank_toon_name protect + weekly schema healthcheck (OPS-06) ✅ SHIPPED 2026-05-11 (3 commits c85586b, ae57d61, a9562b6; 229/229 tests green; trigger count 7→8; schema_version=3 unchanged)
- [x] 05-02-PLAN.md — Archive lib + weeklyStaleCharArchive + weeklyEvictionArchive cron (VIEW-05) ✅ SHIPPED 2026-05-11 (5 commits 32f8cfa, 434adf6, ab10732, 4c3b339, 2530de3; 246/246 tests green; trigger count 8→10; schema_version=3 unchanged; Path A held)
- [ ] 05-03-PLAN.md — Cross-character search sidebar (SEARCH-01..04, TIP-04)
- [ ] 05-04-PLAN.md — Eviction sidebar + DOC-02 runbook (DOC-02)
- [ ] 05-05-PLAN.md — Jekyll Pages site + README shrink + 12-guildie rollout smoke (DOC-01, DOC-03)
**UI hint**: yes
**Research flag**: not needed (standard sidebar/HtmlService/onboarding patterns; SUMMARY.md confidence HIGH).

## Phase Dependencies

```
Phase 1 (Thin Slice)
   │
   ▼
Phase 2 (Watcher Robustness + Schema Lock)
   │
   ▼
Phase 3 (Apps Script Enrichment Foundation)  ◀── parallelizable with Phase 2 (Go vs TS, no shared code paths)
   │                                              with single contributor: sequential is correct
   ▼
Phase 4 (Differentiator Features)
   │
   ▼
Phase 5 (Search + Onboarding + Polish)
```

**Note on Phase 2 / Phase 3 parallelization:** The watcher (Go) and Apps Script (TypeScript) share no code paths. With two contributors these phases could run concurrently. With one contributor (current state), sequential execution in the order shown is correct.

## Coverage

**Total v1 requirements:** 69 (counted across 12 categories: INST 5, AUTH 6, WATCH 9, SCHEMA 8, ENRICH 9, VIEW 5, TIP 4, BANK 4, SEARCH 4, CHECK 5, OPS 7, DOC 3).
**Mapped to phases:** 69
**Unmapped:** 0 ✓
**Duplicates:** 0 ✓

> **Note on count discrepancy:** The instruction header and `REQUIREMENTS.md` summary state "56 v1 requirements," but a literal count of bulleted REQ-IDs in the file totals 69. The roadmap maps every REQ-ID present in the file. The "56" figure should be reconciled in `REQUIREMENTS.md` at next milestone audit.

### Coverage Map (REQ-ID → Phase)

| Phase | Requirements |
|-------|--------------|
| Phase 1 | INST-01, INST-02, INST-03, AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-06, WATCH-01, WATCH-04, OPS-01, OPS-03 |
| Phase 2 | INST-04, INST-05, AUTH-05, WATCH-02, WATCH-03, WATCH-05, WATCH-06, WATCH-07, WATCH-08, WATCH-09, SCHEMA-01, SCHEMA-02, SCHEMA-03, SCHEMA-04, SCHEMA-05, SCHEMA-06, SCHEMA-07, SCHEMA-08, OPS-04, OPS-05 |
| Phase 3 | ENRICH-01, ENRICH-02, ENRICH-05, ENRICH-06, ENRICH-07, ENRICH-08, ENRICH-09, VIEW-01, VIEW-02, VIEW-03, VIEW-04, TIP-01, TIP-02, TIP-03, OPS-02 |
| Phase 4 | ENRICH-03, ENRICH-04, BANK-01, BANK-02, BANK-03, BANK-04, CHECK-01, CHECK-02, CHECK-03, CHECK-04, CHECK-05, OPS-07 |
| Phase 5 | SEARCH-01, SEARCH-02, SEARCH-03, SEARCH-04, TIP-04, VIEW-05, OPS-06, DOC-01, DOC-02, DOC-03 |

### Counts per Phase

| Phase | Req Count |
|-------|-----------|
| Phase 1 | 12 |
| Phase 2 | 20 |
| Phase 3 | 15 |
| Phase 4 | 12 |
| Phase 5 | 10 |
| **Total** | **69** |

## Progress Table

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. End-to-End Thin Slice | 1/8 | Executing (01-01 ✓) | - |
| 2. Watcher Robustness + Schema Lock | 0/? | Not started | - |
| 3. Apps Script Enrichment Foundation | 0/? | Not started | - |
| 4. Differentiator Features | 0/? | Not started | - |
| 5. Search + Onboarding + Polish | 0/? | Not started | - |

### Phase 1 Plan Status

| Plan | Name | Status | Commits | Completed |
|------|------|--------|---------|-----------|
| 01-01 | Repo skeleton | ✓ Complete | 1abb22a, 4900420, ddb594e | 2026-05-01 |
| 01-02 | (next) | Pending | — | — |

---

*Roadmap created: 2026-04-30*
*Last updated: 2026-05-01 after Plan 01-01 execution*
