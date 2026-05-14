---
phase: 05-search-onboarding-privacy-polish
plan: 05
subsystem: onboarding-jekyll-pages-and-distribution-handoff
tags: [docs, onboarding, distribution, smoke, pages, milestone-v1]
requires:
  - 05-01 (hideAllSystemTabs + weeklySchemaHealthcheck + protectBankToonName — referenced in troubleshooting.md "Hidden tabs reappeared" + "ErrSchemaTooNew" sections)
  - 05-02 (weeklyEvictionArchive + moveCharToArchive — referenced in eviction-runbook cross-link from troubleshooting.md "I need to remove a guildie")
  - 05-03 (showSearchSidebar + SEARCH-03 Path 2 — REQUIREMENTS.md traceability update for SEARCH-03 lands here)
  - 05-04 (showEvictionSidebar + docs/eviction-runbook.md — DOC-02 runbook is the cross-link target from troubleshooting.md + dev.md)
provides:
  - "Jekyll Pages site source under /docs/ (5 markdown files + assets/.gitkeep placeholder) — Cayman remote_theme@v0.2.0; live URL is https://boejowen.github.io/SquireBot/ once the owner enables Pages (Task 2 user action)"
  - "docs/index.md: 1-paragraph landing per UI-SPEC §Copywriting + locked H1 'SquireBot' + Install / GitHub links"
  - "docs/install.md: chronological 5-step walkthrough per UI-SPEC + locked lede 'Five steps. Takes about three minutes.' + image refs for 6 PNGs + smartscreen.gif"
  - "docs/troubleshooting.md: symptom-keyed recovery covering all 7 H2 sections from UI-SPEC + the new 'Installer won't overwrite a running SquireBot' section (carry-over from CONTEXT deferred backlog — documented workaround not structural fix)"
  - "docs/dev.md: flat link list into existing repo docs (CLAUDE.md, oauth-setup.md, apps-script-deploy.md, build-and-install.md, eviction-runbook.md) + Phase 1..5 SUMMARY index"
  - "README.md shrunk per D-12: 11 lines (was 117); existing long-form content covered by CLAUDE.md (canonical) + docs/dev.md (legacy README notes section)"
  - "REQUIREMENTS.md traceability update for SEARCH-03 Path 2 (note: REQUIREMENTS.md is .gitignored per project convention; update is local-only and recorded in this SUMMARY for the project record)"
affects:
  - "Phase 5 ship gate: DOC-01 + DOC-02 + DOC-03 status remain PENDING-USER-ACTION until Tasks 2 + 3 are completed by the user — assets captured, GIF recorded, Pages enabled, fake-guildie smoke run, distribution report produced, ship authorized. This SUMMARY does NOT mark those requirements Complete unilaterally."
  - "Milestone v1.0 gate: STATE.md is updated to reflect Phase 5 CODE-COMPLETE (5/5 plans landed, 31/31 plans) with the user-action handoff explicit; ship-authorization remains a user decision. The milestone v1.0 status moves from 'phase-5-in-progress' to 'phase-5-code-complete' as a new intermediate state — full 'phase-5-shipped' requires the Task 3 ship signal."
tech-stack:
  added:
    - "GitHub Pages with Jekyll (Cayman remote_theme v0.2.0): no runtime dependency on the watcher or Apps Script side — pure documentation surface. Source under /docs/, served at boejowen.github.io/SquireBot once enabled."
  patterns:
    - "Locked H1 + lede copy from UI-SPEC §Copywriting Contract: each of the 4 pages emits the exact title + lede string from the contract verbatim. No marketing language, no exclamation marks, factual second-person voice. 'SquireBot' / 'Install SquireBot' / 'Troubleshooting' / 'Developer notes' are the locked H1s; the ledes are 1-line each."
    - "Chronological H2-keyed install structure: install.md uses '## N. <Title>' headings (1-5), with image refs inline at each step where visual confirmation matters. Cross-link to troubleshooting at the bottom. Five steps total per the plan + UI-SPEC."
    - "Symptom-keyed H2 troubleshooting structure: each '## <Symptom>' is what a guildie greps for under stress (Ctrl-F → 'tray red' / 'ErrSchemaTooNew' / 'reappeared'). 8 H2 sections total (7 from UI-SPEC + 1 extra 'Installer won't overwrite a running SquireBot' carrying the CONTEXT-deferred installer-upgrade-UX workaround per scope_notes)."
    - "D-12 README shrink: post-shrink README is 11 lines: H1 + 1-paragraph what-is + 3 link lines + horizontal rule + CLAUDE.md pointer. Existing tray-menu reference + filesystem layout content is preserved in git history; canonical replacement is CLAUDE.md (already comprehensive) and docs/build-and-install.md (already covers uninstall + tray menu). The 'Legacy README notes' section in docs/dev.md acknowledges the move."
    - "Image production deferred to user action (Task 2): Claude cannot drive a screen recorder or capture Windows screenshots — that requires a clean Windows install + ScreenToGif (per scope_notes). install.md's image refs land first (Task 1); the binary assets land later via Task 2's user-driven capture session."
    - "USER ACTION REQUIRED handoff pattern: Tasks 2 + 3 are checkpoint:human-action blocks in the plan. This SUMMARY documents what was attempted, what was left for the user, and what the user must do next — without falsely marking DOC-01/DOC-02/DOC-03 as Complete. The smoke is intentionally unverified end-to-end."
key-files:
  created:
    - docs/_config.yml (8 lines: remote_theme cayman v0.2.0 + plugin allowlist + title/description/show_downloads)
    - docs/index.md (10 lines: layout: default + H1 SquireBot + 1-paragraph lede + Install/GitHub links)
    - docs/install.md (~50 lines: 5-step chronological walkthrough with inline image refs + SmartScreen GIF + cross-link to troubleshooting)
    - docs/troubleshooting.md (~60 lines: 8 symptom-keyed H2 sections + Install Triggers re-hide + drive.file ~50min propagation note + eviction cross-link)
    - docs/dev.md (~30 lines: flat link list into existing repo docs + Phase 1..5 summary index + screenshot redaction note + Legacy README notes section)
    - docs/assets/.gitkeep (empty file so the directory exists pre-binary assets)
    - .planning/phases/05-search-onboarding-privacy-polish/05-05-SUMMARY.md (this file)
  modified:
    - README.md (117 → 11 lines per D-12; shrunk to short Pages pointer; old long-form content covered by CLAUDE.md + dev.md)
    - .planning/REQUIREMENTS.md (SEARCH-03 row marked ✓ Complete via Path 2 + footnote added; LOCAL ONLY — file is .gitignored per project convention; recorded in this SUMMARY for the project record)
decisions:
  - "Task 1 executed exactly as written. Task 2 (asset capture + Pages enable) and Task 3 (DOC-02 fake-guildie smoke + distribution report + ship authorization) are checkpoint:human-action gates that REQUIRE user action; this SUMMARY does NOT pretend they were performed by Claude."
  - "DOC-01 status: PENDING-USER-ACTION. The runbook content is fully written and shipped (install.md + troubleshooting.md); what's missing is the 6 PNG screenshots + the annotated smartscreen.gif. Without those, the Pages site renders with broken image refs. REQUIREMENTS.md DOC-01 must NOT be marked Complete until assets land and the user verifies the Pages site renders correctly."
  - "DOC-02 status: PENDING-USER-ACTION. The sidebar code + runbook shipped in 05-04. The fake-guildie E2E smoke (Plan 05-05 Task 3 Step A) requires opening a live Google Sheet, seeding synthetic _char_owner rows, running the eviction sidebar end-to-end, manually setting grace_until to past, running weeklyEvictionArchive, and verifying the chain. Claude cannot drive a browser-based Google Sheet UI. REQUIREMENTS.md DOC-02 must NOT be marked Complete until the user runs the smoke and reports the outcome."
  - "DOC-03 status: PENDING-USER-ACTION. The image refs are placed in install.md; the binary GIF + PNGs land via Task 2's user-driven capture session. DOC-03 must NOT be marked Complete until the user lands the assets."
  - "Distribution report: NOT YET PRODUCED. The report's heartbeat-freshness counts require querying the live _char_owner table on the dev workbook 'RiverFAIL VanFRAUD'. Claude cannot read live Apps Script state. Task 3 Step B is fully scripted out in the plan (the diagnostic query + the docs/phase5-distribution.md template); the user runs it from the Apps Script editor and fills in the placeholders."
  - "Schema version stays at 3 (CONTEXT D-12 Path A — LOCKED across all of Phase 5). 05-05 ships NO Apps Script code changes (Pages site + README only). internal/sheet/client.go WatcherMaxSchemaVersion = 3 unchanged; no watcher rebuild required."
  - "REQUIREMENTS.md is .gitignored: the SEARCH-03 traceability update was applied to the local working copy. The project convention treats REQUIREMENTS.md as a local planning artifact (it has never been git-tracked per `git log --all -- .planning/REQUIREMENTS.md` returning empty). The Path 2 footnote is preserved here in the SUMMARY frontmatter under the key-files modified list so the change is part of the project record."
  - "README shrink confirmed via wc -l: 11 lines (was 117). Acceptance criterion '<= 30 lines' satisfied with 19 lines to spare. The post-shrink content is the locked structure from <interfaces>: H1 + 1-paragraph + 3 link lines + horizontal rule + CLAUDE.md pointer."
metrics:
  duration: ~12min (Task 1 executed; Tasks 2 + 3 handed off to user)
  completed: 2026-05-11T05:35Z (Task 1 only; Phase 5 SHIPPED gate is user-pending)
  tasks_completed: 1 of 3 (1 = Task 1 Pages source + README shrink; 2 = Task 2 user-driven asset capture + Pages enable; 3 = Task 3 user-driven fake-guildie smoke + distribution report + ship authorization)
  commits: 1 (df8c2fd docs: Jekyll Pages site source + README shrink per D-12)
  files_changed: 7 created + 1 modified (+ 1 .gitignored local update to REQUIREMENTS.md)
  tests_added: 0 (no code changes in this plan)
  schema_version_after: 3 (unchanged; Path A confirmed across all 5 Phase-5 plans)
  watcher_rebuild_required: false
---

# Phase 5 Plan 05: Jekyll Pages Site + README Shrink + Distribution Handoff Summary

**One-liner:** Shipped the source half of DOC-01 + DOC-03 — a 5-file Jekyll Pages site under `/docs/` with the locked H1+lede copy from UI-SPEC, plus a D-12-shrunk README (117→11 lines pointing at the Pages site). Tasks 2 (Pages enablement) and Task 3 (DOC-02 fake-guildie smoke + distribution report + Phase-5-shipped authorization) are documented USER ACTION REQUIRED handoffs that Claude literally cannot perform. DOC-01/02/03 remain PENDING until the user enables Pages, runs the smoke, and authorizes ship.

**Scope reduction recorded 2026-05-11 (mid-execution, after Task 1 shipped):** Per user direction, the instructional images + SmartScreen GIF originally specified in D-10 are **DROPPED**. Pages docs are text-only. Task 2 reduces to "enable GitHub Pages" only (Steps A/B/D struck). Task 3 Step 0 (asset audit) struck. `docs/assets/` directory removed. SmartScreen text in install.md describes the dialog elements in enough detail to navigate without a screenshot. CONTEXT.md §Scope Changes records the change next to its peers.

## What shipped (Task 1)

### Jekyll Pages site source

Six new files under `docs/`:

- **`docs/_config.yml`** — Cayman `remote_theme: pages-themes/cayman@v0.2.0` (GitHub Pages allowlist-vetted per RESEARCH §Pitfall P10), `plugins: [jekyll-remote-theme]`, title/description/show_downloads.

- **`docs/index.md`** — 1-paragraph landing page. `layout: default` front matter. H1 `SquireBot` + the locked lede `A small Windows app that streams your EverQuest inventory and spellbook into your guild's shared workbook.` plus two links: `[Install →]` + `[GitHub →]`.

- **`docs/install.md`** — Chronological 5-step walkthrough. Locked H1 `Install SquireBot` + lede `Five steps. Takes about three minutes.` Each step is a `## N. <Title>` header (1=Download, 2=Run the installer, 3=Authorize Google, 4=Pick the workbook and EQ folder, 5=Trigger your first sync). Inline `![alt](assets/<filename>.png)` refs for steps with screenshots; the SmartScreen GIF embedded in Step 2 with the locked alt text from the plan. Cross-link to troubleshooting at the bottom: `If you hit a snag, see [Troubleshooting](...).`

- **`docs/troubleshooting.md`** — Symptom-keyed recovery. Locked H1 `Troubleshooting` + lede `Tray icon turned red? Workbook stopped updating? Start here.` Eight `## <Symptom>` H2 sections: Tray icon is red, Workbook isn't updating, SmartScreen warning won't go away, `ErrSchemaTooNew` in the log, drive.file permission propagation (~50 minute delay), Hidden tabs reappeared, I need to remove a guildie, Installer won't overwrite a running SquireBot. The 8th section is the carry-over from CONTEXT scope_notes (installer-upgrade-UX deferred; document the workaround in troubleshooting.md).

- **`docs/dev.md`** — Flat link list. Locked H1 `Developer notes` + lede `Stack overview, how to build, how to deploy Apps Script.` Links into: CLAUDE.md, oauth-setup.md, build-and-install.md, apps-script-deploy.md, eviction-runbook.md (DOC-02). Phase 1..5 summary index links. A "Screenshot redaction for docs" note (mitigates T-05-05-01 PII risk). A "Legacy README notes" section noting where the old long-form README content lives.

- **`docs/assets/.gitkeep`** — Empty file so `docs/assets/` exists in git pre-binary upload. Task 2's PNG/GIF assets will land here.

### README shrink (D-12)

`README.md` shrunk from 117 lines to 11 lines per D-12:

```markdown
# SquireBot

A small Windows app that streams your EverQuest inventory and spellbook into your guild's shared workbook.

**Install:** https://boejowen.github.io/SquireBot/install/
**GitHub:** https://github.com/boejowen/SquireBot
**Developer notes:** https://boejowen.github.io/SquireBot/dev/

---

For full project context, see `CLAUDE.md`.
```

The existing long-form content (tray menu reference, filesystem layout, known-issues, auto-update notes, code-signing rationale, repository layout, fork instructions) is preserved in git history. The canonical replacements are `CLAUDE.md` (stack/architecture/conventions, already comprehensive) and `docs/build-and-install.md` (tray menu + uninstall + sideload). The `Legacy README notes` section in `docs/dev.md` acknowledges this move.

### REQUIREMENTS.md SEARCH-03 Path 2 update (LOCAL ONLY)

The traceability row for SEARCH-03 was updated locally:

**Before:**
```
| SEARCH-03 | Phase 5 | Pending |
```

**After:**
```
| SEARCH-03 | Phase 5 | ✓ Complete (Path 2: search-button cache-freshness tooltip + existing `view`/`bank` `Last Synced` columns — Plan 05-03) |
```

Plus a footnote added at the bottom of the file:

> **SEARCH-03 Path 2 note:** per CONTEXT scope-change 2026-05-11, the search sidebar does not show per-row staleness; the existing `view` and `bank` tabs' `Last Synced` columns (Phase 3 VIEW-04 conditional formatting) satisfy the requirement on a complementary surface.

**Important caveat:** `.planning/REQUIREMENTS.md` is .gitignored per the project's `.gitignore` line 1 (the entire `.planning/` directory is ignored; only specific files like PROJECT.md, ROADMAP.md, STATE.md, and the phase plan dirs have been force-added historically). `git log --all -- .planning/REQUIREMENTS.md` returns empty — the file has never been git-tracked. The traceability update therefore lives only in the local working copy. This SUMMARY's `key-files.modified` entry preserves the change for the project record.

## What's deferred to USER ACTION (Tasks 2 + 3)

These are not failures — they are `checkpoint:human-action` gates in the plan. The plan acknowledges Claude cannot perform them; the SUMMARY captures the precise handoff.

### Task 2 — Enable GitHub Pages (USER ACTION REQUIRED) — REVISED 2026-05-11

**Scope reduction:** Per user direction 2026-05-11, instructional images + GIF are dropped. Pages docs are text-only. Task 2 reduces to "enable GitHub Pages" — Step C from the original plan. Steps A (capture PNGs), B (record GIF), and D (commit assets) are STRUCK.

**Why Claude can't do this:**
Claude cannot click through the GitHub repo Settings UI to flip the Pages toggle.

**What the user needs to do (revised; only Step C remains):**

**Step A — Capture 6 PNG screenshots** (each ≤200KB; max 1200px wide; use a clean Windows 11 box/VM):

1. Download latest installer from https://github.com/boejowen/SquireBot/releases/latest, screenshot the installer dialog → save as `docs/assets/01-installer.png`.
2. When SmartScreen appears, screenshot the initial prompt showing `More info` → `docs/assets/02-smartscreen-more-info.png`.
3. Click `More info`, screenshot the expanded panel showing `Run anyway` → `docs/assets/03-smartscreen-run-anyway.png`.
4. After install, when the Google OAuth consent screen appears, screenshot it showing the `drive.file` scope → `docs/assets/04-oauth-consent.png`.
5. When the EQ folder picker dialog appears, screenshot it → `docs/assets/05-folder-picker.png`.
6. Trigger a tray-red state (revoke OAuth at https://myaccount.google.com/permissions, wait for next watcher poll), screenshot → `docs/assets/tray-red.png`.

**Pre-publish PII check (T-05-05-01):** Blur or crop any visible guildie emails, character names, OAuth client IDs, or paths containing personal info. Prefer using a synthetic test character on a fresh workbook.

**Step B — Record the SmartScreen GIF** (≤5MB):

1. Install ScreenToGif (https://www.screentogif.com/) or LICEcap / ShareX.
2. Record the SmartScreen sequence: initial warning → `More info` → expanded panel → `Run anyway` → installer launches. ~10-15 seconds.
3. Annotate with text callouts pointing at `More info` and `Run anyway`.
4. Export at ≤720p, ≤15fps, single-color-replacement compression.
5. Verify size ≤ 5,242,880 bytes. If over budget, re-export at lower fps or shorter duration; if persistently over budget, fallback to YouTube unlisted per CONTEXT D-10 (then edit install.md to embed the YouTube URL instead of the GIF, and add a one-line update to UI-SPEC §Image format).
6. Save as `docs/assets/smartscreen.gif`.

**Step C — Enable GitHub Pages:**

1. Open https://github.com/boejowen/SquireBot/settings/pages.
2. Under `Source`, select `Deploy from a branch`.
3. Set `Branch: main` and `Folder: /docs`. Save.
4. Wait ~30-90 seconds for the first `pages-build-deployment` Actions workflow to complete (visible at https://github.com/boejowen/SquireBot/actions).
5. Visit https://boejowen.github.io/SquireBot/ — verify the Cayman-themed index renders with the `SquireBot` H1 and the install link.
6. Click `Install →` — verify install.md renders, screenshots appear, the GIF plays inline, links work.

**Step D — Commit assets:**

```bash
git add docs/assets/01-installer.png docs/assets/02-smartscreen-more-info.png docs/assets/03-smartscreen-run-anyway.png docs/assets/04-oauth-consent.png docs/assets/05-folder-picker.png docs/assets/tray-red.png docs/assets/smartscreen.gif
git commit -m "docs(05-05): land DOC-01 + DOC-03 assets (6 PNG + smartscreen GIF)"
git push
```

**After Task 2:** DOC-01 + DOC-03 can be marked Complete in REQUIREMENTS.md by the user.

### Task 3 — DOC-02 fake-guildie smoke + distribution report + ship authorization (USER ACTION REQUIRED)

**Why Claude can't do this:**
Claude cannot open the live `RiverFAIL VanFRAUD` Google Sheet, seed `_char_owner` rows via the Sheets UI, run the eviction sidebar's `commitEviction` callback through `google.script.run`, manually edit `_meta.eviction_log` cell JSON, or run `weeklyEvictionArchive` from the Apps Script editor. The smoke is end-to-end against a live deployed workbook.

**What the user needs to do:**

**Step 0 — Pre-flight asset audit (BLOCKING):**

After Task 2 completes, run this to ensure all assets exist and the GIF is within budget:

```bash
node -e "const fs=require('fs'); const pngs=['docs/assets/01-installer.png','docs/assets/02-smartscreen-more-info.png','docs/assets/03-smartscreen-run-anyway.png','docs/assets/04-oauth-consent.png','docs/assets/05-folder-picker.png','docs/assets/tray-red.png']; const gif='docs/assets/smartscreen.gif'; let ok=true; for(const f of pngs){if(!fs.existsSync(f)){console.error('MISSING PNG:',f);ok=false;}} if(!fs.existsSync(gif)){console.error('MISSING GIF:',gif);ok=false;} else {const size=fs.statSync(gif).size; if(size>5242880){console.error('GIF OVER 5MB BUDGET:',size,'bytes');ok=false;} else {console.log('gif size OK:',size,'bytes');}} process.exit(ok?0:1);"
```

(Relaxes the GIF assertion if YouTube fallback was invoked in Task 2.)

**Step A — DOC-02 fake-guildie smoke** (run against `RiverFAIL VanFRAUD` dev workbook via the Apps Script editor):

1. **Seed.** In `_char_owner`, append two synthetic rows:
   - `char_name=SmokeTestChar, owner_email=fake-guildie@example.com, is_removed=FALSE, last_seen=<now ISO>`
   - `char_name=SmokeTestAlt, owner_email=fake-guildie@example.com, is_removed=FALSE`

   Create synthetic `inv:SmokeTestChar` (3 rows of dummy data with the standard 6-column schema) and `spell:SmokeTestAlt` (2 rows with `Level | Name | _uploaded_at`).

2. **Open sidebar.** `SquireBot → Evict Guildie…`. Verify:
   - `fake-guildie@example.com` appears in the dropdown.
   - Selecting it shows `Characters affected (2): SmokeTestChar, SmokeTestAlt` (sorted alphabetically — Alt before Char if a quirk shows).
   - Grace-expires shows `<30 days from today>`.

3. **Click Evict.** Confirm in the native browser modal. Verify:
   - Success message renders inline: `Marked 2 character(s) as removed. Grace until <date>.`
   - `_char_owner.is_removed=TRUE` for both rows.
   - `_meta.eviction_log` JSON has a new entry: `{at, email:'fake-guildie@example.com', initiated_by:<your-email or 'unknown'>, grace_until, chars:['SmokeTestChar','SmokeTestAlt'], reason:'evicted'}` (chars in row-scan insertion order per 05-04's `commitEviction`).

4. **Simulate grace expiry.** In the Apps Script editor (or directly in `_meta.eviction_log` cell), edit the entry's `grace_until` to `'1970-01-01T00:00:00.000Z'`. Save.

5. **Run weeklyEvictionArchive manually.** Apps Script editor → select `weeklyEvictionArchive` → Run. Verify:
   - `_archive` tab exists (lazy-created or pre-existing); 7-column header `archived_at, char_name, tab_type, row_count, uploaded_at, reason, snapshot_json`.
   - Two `_archive` rows appended for SmokeTestChar (`inv` + `spell` where present; missing source tab → row_count=0, snapshot_json='[]' per 05-02's helper).
   - Two `_archive` rows appended for SmokeTestAlt (or one row if only `spell:SmokeTestAlt` exists — per archive.test.ts behavior).
   - `_meta.archive_log` has 2 new entries (one per archived char).
   - `_meta.eviction_log` has the processed entry REMOVED.
   - `inv:SmokeTestChar` and `spell:SmokeTestAlt` are HIDDEN (not deleted).
   - `_status.last_eviction_archive_run` = current timestamp; `_status.last_eviction_archive_count` = `'2'`.

6. **Un-evict verification (recovery).** In `_char_owner`, flip both rows' `is_removed=FALSE` manually. Unhide `inv:SmokeTestChar` + `spell:SmokeTestAlt`. Verify the data is unchanged (the `_archive` snapshot persists for audit; live tabs restored).

7. **Cleanup.** Delete the two synthetic `_char_owner` rows; delete `inv:SmokeTestChar` + `spell:SmokeTestAlt`; remove the test entries from `_meta.archive_log`; optionally delete the `_archive` tab if no other real entries exist.

**Step B — Produce the distribution report:**

From the Apps Script editor, run this diagnostic (paste into a temporary function, run, capture the two counts, then delete the function):

```typescript
function _phase5DistributionProbe() {
  const ss = SpreadsheetApp.getActiveSpreadsheet();
  const sheet = ss.getSheetByName('_char_owner');
  const values = sheet.getRange(2, 1, sheet.getLastRow() - 1, 13).getValues();
  const sevenDaysAgoMs = Date.now() - 7 * 24 * 60 * 60 * 1000;
  const distinctActiveEmails = new Set();
  const distinctRecentEmails = new Set();
  for (const r of values) {
    const isRemoved = r[8] === true || String(r[8]) === 'true' || String(r[8]).toLowerCase() === 'true';
    if (isRemoved) continue;
    const email = String(r[1] ?? '').trim();
    if (!email) continue;
    distinctActiveEmails.add(email);
    const lastSeenRaw = r[10];
    const lastSeenMs = typeof lastSeenRaw === 'string' ? Date.parse(lastSeenRaw) : (lastSeenRaw instanceof Date ? lastSeenRaw.getTime() : NaN);
    if (Number.isFinite(lastSeenMs) && lastSeenMs > sevenDaysAgoMs) distinctRecentEmails.add(email);
  }
  console.log('Distinct active emails:', distinctActiveEmails.size);
  console.log('Distinct emails seen in last 7d:', distinctRecentEmails.size);
}
```

Then create `docs/phase5-distribution.md` with the template from the plan's `<interfaces>` block, filling in:
- `distinctActiveEmails.size` → the count under "Distinct active guildie emails (non-is_removed)"
- `distinctRecentEmails.size` → the count under "Distinct emails with last_seen in the last 7 days"
- The per-email heartbeat table (sorted by last_seen desc)
- The notable-gaps narrative (free-form, list any expected emails missing or stale)

**Automated sanity check (the only one):** if `distinctRecentEmails.size <= 1`, treat as a likely deployment failure (only the dev box is reporting). Surface in the TL;DR as `⚠ Likely smoke regression — only N guildie(s) writing data; recommend investigating before authorizing ship`. Otherwise: `N of ~12 guildies writing data in the last 7 days. Ship decision is the user's call.`

**Step C — Surface to user for ship authorization:**

User replies with one of:
- `ship: phase-5-shipped` — authorize ship; proceed to Step D
- `ship: hold, reason: <reason>` — hold ship; document the reason as a blocker
- `ship: smoke-regression-investigate` — only if count ≤ 1; investigate before deciding

**Step D — On `ship: phase-5-shipped`:**

1. Update `.planning/STATE.md`:
   - `status: phase-5-in-progress` → `status: phase-5-shipped`
   - `completed_phases: 4` → `completed_phases: 5`
   - `completed_plans: 30` → `completed_plans: 31`
   - `percent: 97` → `percent: 100`
   - Append a "Phase 5 SHIPPED" entry to the session log with the smoke outcome + distribution counts

2. Update `.planning/ROADMAP.md`: tick the Phase 5 box at line 14; tick 05-05-PLAN.md at line 99.

3. Mark REQUIREMENTS.md DOC-01, DOC-02, DOC-03 as `✓ Complete (Plan 05-05)`.

4. Tag the repo: `git tag phase5-complete` + `git tag v1.0.0` (or whichever versioning is preferred). Push tags.

5. Optionally: trigger a milestone retrospective.

## Threat-register coverage

All 6 STRIDE items from the plan's `<threat_model>` are addressed at the SOURCE level (Task 1):

- **T-05-05-01 (PII in screenshots):** MITIGATED by the `docs/dev.md` "Screenshot redaction for docs" note + Task 2 Step A pre-publish PII check note in this SUMMARY. Reinforced by the user-side guidance to prefer synthetic test characters.
- **T-05-05-02 (malicious PR replaces GIF):** MITIGATED by the single-maintainer repo model + Pages serves only static content (no code execution from a binary asset).
- **T-05-05-03 (GIF >5MB bloats clones):** MITIGATED by the 5MB budget in Task 2 Step B + the YouTube fallback path documented for over-budget cases.
- **T-05-05-04 (SmartScreen GIF legitimizes unsigned binaries broadly):** ACCEPTED. install.md copy is scoped to SquireBot specifically; no "More info → Run anyway" advice outside the SquireBot context.
- **T-05-05-05 (repudiation — guildie disputes install ask):** ACCEPTED. Out-of-band; install instructions are public + reproducible via Pages.
- **T-05-05-06 (Pages site reveals project structure to attackers):** ACCEPTED. Repo is already public; Pages just makes the docs nicer. Trusted-guild posture.

## Deviations from Plan

**One material deviation, documented inline:**

1. **[Rule 4 — Architectural / scope] Tasks 2 + 3 cannot be performed by Claude.**
   - **Found during:** Plan execution (immediately on reading the plan).
   - **Issue:** Both Task 2 (capture 6 screenshots + record annotated GIF + enable Pages in GitHub repo Settings) and Task 3 (run a live fake-guildie eviction smoke through the Google Sheets UI + Apps Script editor + produce a heartbeat-freshness distribution report from live `_char_owner` data) require GUI-driven actions in a browser, a screen recorder, the Google Sheets web app, and the GitHub repo Settings page. Claude has no GUI access to any of these surfaces.
   - **Fix (per plan + user prompt's `<critical_warning>`):** Execute Task 1 to completion (Pages site source + README shrink + REQUIREMENTS.md SEARCH-03 update). Document Tasks 2 + 3 as USER ACTION REQUIRED handoffs in this SUMMARY with step-by-step checklists matching the plan's `<how-to-verify>` text. Do NOT mark DOC-01, DOC-02, or DOC-03 as Complete unilaterally. Do NOT mark Phase 5 as SHIPPED unilaterally.
   - **Tradeoff:** Phase 5 is CODE-COMPLETE (5/5 plans' code + docs landed) but SHIP-PENDING (assets + smoke + distribution report + user authorization remain). The plan explicitly anticipates this via the `checkpoint:human-action` gates; this is correctly-followed plan behavior, not a planning error.
   - **Documented in:** This SUMMARY's `affects` frontmatter + `What's deferred to USER ACTION` body section + the explicit STATE.md update strategy below.

**No other deviations from Task 1's `<action>` text.**

## STATE.md + ROADMAP.md update strategy

Per the plan and the user prompt, STATE.md/ROADMAP.md updates have two phases:

**Now (Task 1 complete, Tasks 2-3 pending):**
- `.planning/STATE.md` status field stays `phase-5-in-progress` (NOT flipped to `phase-5-shipped` yet — that's a user decision after Task 3).
- The Phase 5 plan-execution log adds a row for 05-05 noting "Task 1 complete; Tasks 2 + 3 PENDING USER ACTION" with commit hash `df8c2fd`.
- `completed_plans: 30` becomes `completed_plans: 31` because the plan's deliverable (the Pages site source + README shrink) HAS landed; the user-action portions are tracked as handoffs, not as plan-incompletions.
- The progress bar moves 30/31 → 31/31 (the plan IS complete to the extent Claude can execute it). Phase 5 status moves from `phase-5-in-progress` to a new intermediate `phase-5-code-complete` flagging that the ship gate is user-pending.

**After user ship signal (Task 3 complete):**
- Status → `phase-5-shipped`. Completed phases → 5. Percent → 100.
- ROADMAP.md Phase 5 row ticked; 05-05-PLAN.md ticked.
- REQUIREMENTS.md DOC-01, DOC-02, DOC-03 marked Complete.
- Tag `phase5-complete` + `v1.0.0`.

## Self-Check

**Files exist (8 created + 1 modified + 1 local-only update):**

- FOUND: `docs/_config.yml`
- FOUND: `docs/index.md`
- FOUND: `docs/install.md`
- FOUND: `docs/troubleshooting.md`
- FOUND: `docs/dev.md`
- FOUND: `docs/assets/.gitkeep`
- FOUND: `README.md` (11 lines per `wc -l README.md`)
- FOUND: `.planning/phases/05-search-onboarding-privacy-polish/05-05-SUMMARY.md` (this file)
- LOCAL-ONLY: `.planning/REQUIREMENTS.md` SEARCH-03 row updated to ✓ Complete via Path 2 + footnote added (file is .gitignored)

**Commit exists:**

- FOUND: `df8c2fd` — `docs(05-05): add Jekyll Pages site + shrink README per D-12`

**Acceptance-criteria grep gates (re-verified after commit):**

- `grep -nF "remote_theme: pages-themes/cayman@v0.2.0" docs/_config.yml` → 1 hit (PASS)
- `grep -nF "jekyll-remote-theme" docs/_config.yml` → 1 hit (PASS)
- `grep -nF "title: SquireBot" docs/_config.yml` → 1 hit (PASS)
- `grep -nF "layout: default" docs/index.md` → 1 hit (PASS)
- `grep -nF "# SquireBot" docs/index.md` → 1 hit (PASS)
- `grep -nF "boejowen/SquireBot" docs/index.md` → ≥1 hit (PASS)
- `grep -nF "# Install SquireBot" docs/install.md` → 1 hit (PASS)
- `grep -nF "Five steps. Takes about three minutes." docs/install.md` → 1 hit (PASS)
- `grep -nE "^## [12345]\. " docs/install.md` → 5 hits (PASS)
- `grep -nF "smartscreen.gif" docs/install.md` → ≥1 hit (PASS)
- `grep -nF "More info" docs/install.md` → ≥1 hit (PASS)
- `grep -nF "/outputfile inventory" docs/install.md` → ≥1 hit (PASS)
- `grep -nF "# Troubleshooting" docs/troubleshooting.md` → 1 hit (PASS)
- `grep -nE "^## " docs/troubleshooting.md` → 8 hits (PASS — ≥6 required)
- `grep -nF "Tray icon is red" docs/troubleshooting.md` → ≥1 hit (PASS)
- `grep -nF "ErrSchemaTooNew" docs/troubleshooting.md` → ≥1 hit (PASS)
- `grep -nF "drive.file" docs/troubleshooting.md` → ≥1 hit (PASS)
- `grep -nF "Install Triggers" docs/troubleshooting.md` → ≥1 hit (PASS)
- `grep -nF "eviction-runbook" docs/troubleshooting.md` → ≥1 hit (PASS)
- `grep -nF "# Developer notes" docs/dev.md` → 1 hit (PASS)
- `grep -nF "eviction-runbook.md" docs/dev.md` → ≥1 hit (PASS)
- `grep -nF "oauth-setup.md" docs/dev.md` → ≥1 hit (PASS)
- `wc -l README.md` → 11 (≤30 required, PASS)
- `grep -nF "boejowen.github.io/SquireBot" README.md` → ≥1 hit (PASS)
- `grep -nF "SEARCH-03 | Phase 5 | ✓ Complete" .planning/REQUIREMENTS.md` → 1 hit (PASS — local-only file)
- `grep -nF "Path 2" .planning/REQUIREMENTS.md` → ≥1 hit (PASS — local-only file)

## Self-Check: PASSED (for Task 1)

All Task-1 acceptance gates pass. Tasks 2 + 3 are intentionally not self-checked here — they are USER ACTION REQUIRED handoffs and will self-check via the user's resume signal per the plan's `<resume-signal>` blocks.

## Deferred backlog at v1.0 boundary (carried forward)

These are CONTEXT-deferred items that survive Phase 5 and become v1.0.x or v2 candidates:

- **Bank-coin permission lock** (only bank-toon-owner can use Set Bank Coin sidebar) — Phase 4 deferred backlog; not bitten by usage yet.
- **Polished theme picker tile UI** (6-tile grid per `docs/design/mockups/eq-aesthetic-picker.html`) — Phase 4 deferred backlog.
- **Sidebar HTML inline-JS unit tests** — Phase 4 deferred backlog; requires JSDOM setup not currently in vitest.
- **Installer-driven upgrade UX** (NSIS can't overwrite running .exe; current workaround = stop process → run installer; documented in `docs/troubleshooting.md` "Installer won't overwrite a running SquireBot" section) — v1.0.1 candidate.
- **Self-service eviction** (departing guildie quits cleanly without officer action) — threat-model deferred to v2.
- **`_meta.guild_admins` allowlist** (eviction sidebar enforces officer-only by code, not by trust) — 05-04 v1.0.x polish item.
- **Extract `SIDEBAR_BODY` constants to `apps-script/src/sidebars/*.html`** — uniform deferral across 05-03 + 05-04; would be a single coordinated refactor.
- **PropertiesService migration for recent-query persistence** (CacheService 25-min cap means recent-history TTL is shorter than ideal) — 05-03 v1.0.1 candidate.

## Next

1. **USER ACTION:** Run Task 2 (capture assets + record GIF + enable Pages) per the step-by-step checklist above.
2. **USER ACTION:** Run Task 3 Steps A+B+C (DOC-02 smoke + distribution report + ship authorization).
3. **On ship: phase-5-shipped:** Apply the STATE.md/ROADMAP.md/REQUIREMENTS.md updates from Step D, tag `phase5-complete` + `v1.0.0`, push tags.
4. **Roadmap audit TODO from CLAUDE.md:** reconcile "56 v1 requirements" summary vs literal 69 REQ-IDs in REQUIREMENTS.md. Phase 5 IS the milestone audit point per CLAUDE.md note; this can be resolved during the milestone retrospective.
5. **Day-10 token-survival check** (routine `trig_01Uog2muQ22CBsjZfqPiSH9r`) fires 2026-05-13T15:00:00Z — independent of Phase 5 ship; leave it alone.
