<!-- GSD:project-start source:PROJECT.md -->
## Project

**SquireBot** — a small Windows app that every member of a ~12-person Project 1999 (Classic EverQuest emulator) guild installs on their PC. It watches the EQ folder for the tab-separated text files produced by `/outputfile inventory` and `/outputfile spellbook` and pushes their contents into a single shared Google Sheet. The sheet (Apps Script + TypeScript) joins each guildie's data with the P1999 wiki MediaWiki API and the PigParse REST API to produce per-character inventory views, gear/spell progression checklists vs. Velious tiers, a shared bank with cross-character search, and item tooltips. v2 (deferred) adds wantlist + Discord pinger.

**Core Value:** Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.

See `.planning/PROJECT.md` for the full context, key decisions, and constraints.
<!-- GSD:project-end -->

<!-- GSD:stack-start source:STACK.md -->
## Technology Stack

**Watcher (per-guildie Windows app):** Go 1.24, single statically-linked `.exe`. `google.golang.org/api/sheets/v4` + `golang.org/x/oauth2`, `fsnotify` v1.7+ (500 ms debounce, always re-reads on event), `wincred` for DPAPI-backed token storage, `minio/selfupdate` for auto-update, `fyne.io/systray` for the tray UI, `lumberjack` for log rotation. Distributed via NSIS 3.10+ per-user installer (no UAC), autostart via `HKCU\...\Run`.

**Sheet side:** Google Sheets workbook with Apps Script V8 runtime. Code authored in TypeScript via `clasp` v3.0+ + `esbuild` 0.20+ + `@types/google-apps-script`. `HtmlService` for the search sidebar; cell notes (`Range.setNote`) for tooltips; time-driven triggers for refresh jobs.

**External APIs (NOT scraping targets):**
- **PigParse REST**: `https://pigparse.azurewebsites.net` — Swagger at `/swagger/index.html`. Use `GET /api/item/getall/1` (server=1=Blue) once daily.
- **P1999 MediaWiki API**: `https://wiki.project1999.com/api.php` — `action=parse&prop=wikitext`. Used weekly for item summaries, per-class spell lists, Velious gear tiers, and quest-item mappings.

**OAuth scope:** `https://www.googleapis.com/auth/drive.file` only (non-sensitive — no Google verification audit). The OAuth consent screen MUST be flipped to **Production** before any guildie installs (Testing-mode refresh tokens silently expire every 7 days).

**Never use:** Python+PyInstaller, Electron, service-account JSON keys, the `oob` OAuth flow, the `spreadsheets` or `drive` scope, Apps Script Rhino runtime, HTML scraping of PigParse or the wiki, polling with `time.Tick` instead of `fsnotify` events, trusting `fsnotify` event payloads on Windows.

See `.planning/research/STACK.md` and `.planning/research/SUMMARY.md` for full rationale and alternatives.
<!-- GSD:stack-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

**Three-layer pancake inside the workbook:**

1. **Landing tabs** (watcher-written, one per character per file type): `inv:<CharName>` (cols `Location, Name, ID, Count, Slots, _uploaded_at`), `spell:<CharName>` (cols `Level, Name, _uploaded_at`). Watcher writes full-snapshot replace via `spreadsheets.batchUpdate` (atomic clear `A2:F` + write `A1:F<N+1>`) — never appends, never row-diffs.
2. **Dimension tabs** (Apps Script-written, hidden, `_`-prefixed): `_meta`, `_char_owner`, `_item_master`, `_pigparse`, `_wiki_spells`, `_wiki_gear_tier`, `_quest_items`, `_audit`, `_status`.
3. **View tabs** (Apps Script-written, visible, **CONSOLIDATED filterable mega-tabs** with a leading `Char` column): `view`, `gear_check`, `spell_check`, `bank`. Search is an `HtmlService` sidebar (not a tab).

**Critical, locked decision:** views are CONSOLIDATED, not per-character. Per-character view tabs (`view:<Char>`, `gear_check:<Char>`, `spell_check:<Char>`) would breach Google's hard 200-tab/workbook limit at guild scale (12 × ~10 chars × ~5 view types ≈ 600 tabs). Landing tabs ARE per-character (~120 tabs total — comfortable). If you ever see a plan or doc proposing per-character view tabs, reject it.

**Watcher → Sheet write contract:** atomic `batchUpdate` clear+write per character per file. `valueInputOption=RAW` (never `USER_ENTERED` for hot path — recalc storms). Per-character non-overlapping ranges; no shared mutable ranges from any watcher.

**Identity:** OAuth `userinfo.email` is the canonical identity; the watcher writes it into `_char_owner.owner_email` on first sighting. (`Session.getActiveUser().getEmail()` returns the script owner, NOT the writer — load-bearing distinction.)

**Schema evolution:** extend-only (add columns at right edge, add tabs, add `_meta` rows). Breaking changes require a `_meta.schema_version` bump + idempotent migration + watcher's `WATCHER_MAX_SCHEMA_VERSION` check + script's `SCRIPT_MIN_SCHEMA_VERSION` check.

See `.planning/research/ARCHITECTURE.md` and `.planning/research/SUMMARY.md` for the full schema and dependency graph.
<!-- GSD:architecture-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during Phase 1 / Phase 2 execution.
<!-- GSD:conventions-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, or `.github/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` — do not edit manually.
<!-- GSD:profile-end -->
