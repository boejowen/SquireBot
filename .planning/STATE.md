---
gsd_state_version: 1.0
milestone: v2.3
milestone_name: Character Assignment & Per-Character Wantlists
status: planning
last_updated: "2026-06-08T05:30:00.000Z"
last_activity: 2026-06-08
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# State: SquireBot

**Initialized:** 2026-04-30
**Last updated:** 2026-06-02 (v2.2 "Wantlist + Discord Pinger" roadmap created — Phases 19–23)

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-06-02 after v2.1 shipped)

- **Core value:** Every guildie can answer "what does my character still need, and where in the guild is it?" — delivered via the self-hosted website (squirebot.quest).
- **Current focus:** Phase 21 — EC-Tunnel Auction Monitor COMPLETE (21-03 ec.RunMatch + ec_auction_match scheduler job + main.go reorder shipped; WANT-05 delivered, Track-1 finale done; next = `/gsd-discuss-phase 22` (invite-gated) or close the milestone)
- **Mode:** yolo
- **Granularity:** coarse

## Current Position

Phase: 27 — My-Characters Inventory Filter (EXECUTED 2026-06-08 — code-complete + verified(human_needed); browser-smoke UAT pending)
Execution: 1/1 plan complete (27-01, 3 commits 1d400bf test-RED → 972da5c feat-GREEN → 9bd86b2 wire +page.svelte; all on master, web/ only). NEW `web/src/lib/myview.ts` (pure `myCharNameSet`+`applyMyFilter`, DOM-free, type-only import, no {@html}) + `myview.test.ts` (9 cases green); `web/src/routes/+page.svelte` adds fetchMyCharacters() to the Promise.all, `mineOnly=$state(false)` (default OFF), one EQ-theme `<select>` (All members/My characters/per-char), four grids fed `data={filtered*Rows}`, SearchBox stays full `viewRows`, zero-claimed disabled/hint. Gates GREEN: npm run check 0/0 (484 files), npm test 296/296 (incl 9 new), npm run build ok. Name-join VERIFIED in source: readviews.go:150 ≡ assignment.go:448 both `SELECT c.name` raw (COLLATE NOCASE is ORDER-BY-only) → MyCharacter.name ≡ ViewRow.char byte-for-byte; predicate also case-insensitive (defensive). CONSOLIDATED-VIEWS LOCK held: DataGrid.svelte + columns.ts + api.ts UNTOUCHED, no per-char tabs, no backend/route/migration (git diff = 3 files all under web/src/), no ?mine=1 server scope (negative security property T-27-01). Verifier=human_needed (3/3 success criteria + MYVIEW-01/02 code-verified; 27-VERIFICATION.md). REMAINING before "verified": browser-smoke UAT on a DEPLOYED build or full local stack (NOT npm run dev vs prod — login bounces) + `/gsd-ui-review 27` — node vitest is DOM-blind (web-tests-node-only-blind-to-dom; same gap as P15/P26). Smoke checklist (8 items) in 27-01-PLAN <browser_smoke_gap> + 27-VERIFICATION. CODE REVIEW: `/gsd-code-review 27` ran — 27-REVIEW.md, 0 BLOCKER/0 HIGH/0 MEDIUM, 2 LOW + 2 NIT; LOW/NIT FIXED in 25c76c1 (IN-01 reset stale drill-down `selectedChar` on refetch; IN-02 clarifying comment that the shared bank-coin summary stays guild-wide by design; IN-03 +2 predicate tests → 298 green; IN-04 noted impossible — character.name is UNIQUE). Gates re-green (check 0/0, test 298, build ok). DEPLOYED LIVE 2026-06-08 (web-only, NO backend restart — zero server change): npm run build → tarball → scp → extract to /var/www/squirebot.new → chmod u=rwX,go=rX → atomic mv-swap (squirebot.old kept for rollback) per docs/backend-deploy.md §7.5. External verify GREEN: apex 200, entry JS content-type text/javascript (fresh bundle start.BPXF6Psz.js; NOT the blank-screen text/html bug), api 401 (backend healthy/untouched). Rollback: `mv /var/www/squirebot /var/www/squirebot.bad && mv /var/www/squirebot.old /var/www/squirebot`. REMAINING: browser-smoke UAT on prod (8-item checklist in 27-01-PLAN <browser_smoke_gap>) + optional `/gsd-ui-review 27` — node vitest is DOM-blind. Next: browser-smoke → `/gsd-plan-phase 28` (Character-Tagged Wantlist — the 00009 character_id touch; final v2.3 phase; stacks on this + P26).

PRIOR (planned 2026-06-08):
Plan: 1 plan / 1 wave / 2 tasks (27-01). Gates chosen: plan-directly (no CONTEXT.md — design fully locked in ROADMAP), **--skip-ui** (additive filter on the existing design system), light research first. Flow ran: research (HIGH conf) → pattern-map (4 files, all exact analogs) → planner → plan-checker **VERIFICATION PASSED** (0 blockers / 0 warnings, first iteration). Key finding: **ZERO backend change, ZERO migration** — purely a `web/` composition of already-shipped parts (consolidated view payloads + P26 `fetchMyCharacters()` / `GET /api/v1/assignments/mine`). 27-01 = NEW pure `web/src/lib/myview.ts` `applyMyFilter` + node test (mirrors assignments.ts), then ONE EQ-theme `<select>` (All members[default-off] / My characters / per-char drill-down) wired into `web/src/routes/+page.svelte` feeding the four grids pre-filtered `data` (WantlistPanel upstream-filter pattern); SearchBox stays guild-wide; DataGrid/columns untouched (consolidated-views LOCK preserved by construction). Locked sub-decisions: single `<select>`, grid-only, join on NAME (verified: readviews.go:150 ≡ assignment.go:448 both `SELECT c.name`, no normalization; predicate also case-insensitive), pure helper. Threat model T-27-01 = the NEGATIVE property (filter is NOT access control — API already serves all-members rows to every session). Browser-smoke gap flagged (node-only vitest is DOM-blind — the P15/P26 trap): post-exec `/gsd-ui-review 27` + deploy-then-smoke (or full local stack) mandatory before verified. Docs: 27-RESEARCH/PATTERNS/01-PLAN written (committed; commit_docs=false otherwise). Next: `/gsd-execute-phase 27`.

---

Phase: 26 — Character Assignment (EXECUTED 2026-06-08 — code-complete; UAT + 2 advisory MEDIUMs pending)
Execution: 3/3 plans complete (10 commits aff1553..c08b4e6, all on master, code only). 26-01 store/migration (00009→v9), 26-02 API (12 routes), 26-03 web UI. Gates: go build/vet/test green; web check(0 warn)/build/test(287) green. Verifier=human_needed (6/6 criteria + 6/6 ASSIGN code-verified; web interactive + live-DB deploy = UAT). Code review=issues_found, 0 BLOCKER/0 HIGH; 2 MEDIUM + 2 NIT FIXED via /gsd-code-review-fix (commits b1852eb/29170a6/55d06d3, ff-merged to master; gates re-green). MD-01: RequestTx now rejects non-contested chars (unassigned/self-held) → 409 not_contested (ErrCharNotContested). MD-02: added ListMyPendingRequests + GET /api/v1/assignments/requests/mine + panel hydration (Request→Cancel survives reload). 3 LOW deferred (out-of-tx assignee probe [FK-backstopped], unused current_assignee UI plumbing, post-approval re-request). SECURITY: /gsd-secure-phase 26 PASSED — 26-SECURITY.md threats_open:0, 18/18 closed (17 mitigate + 1 accept AR-26-07 multiple-guild-banks). Phase 26 = code-complete + verified(human_needed) + review-clean + threat-secure. DEPLOYED LIVE 2026-06-08: R2 backup (squirebot-2026-06-08.db.gz) → backend binary swapped (.bak kept) → goose.Up applied 00009 (schema v8→v9; `OK 00009_character_assignment.sql`; bot reconnected; scheduler 4 jobs) → AUTO-SEED 4/4 eligible chars assigned → web tarball+atomic-swap (squirebot.old kept). External verify GREEN: apex 200, JS content-type canary passes, /my-characters serves 200, whoami-web 200, api healthy. Rollback: cp /usr/local/bin/squirebot-server.bak → restart (old binary runs against v9 DB — extend-only); web mv squirebot.old back. BROWSER-SMOKE UAT 2026-06-08: **PASS** (see 26-HUMAN-UAT.md). Blocks A (member claim/release/re-claim + inline 409)/B (officer assign/reassign/remove/designate)/C (My-characters nav link + de-banked CharMetaForm) all green; live-DB seed verified directly on-box (4/4 assignments, real bank toon Findom preserved). Found 1 MEDIUM UI bug → **backlog 999.33** (commit 9c3022f): designating a char bank/bot clears its character_assignment row so it drops out of ListAllAssignments — the only list AssignmentAdminPanel renders + the host of the designate/reassign controls — AND it's excluded from member claimable, so bank/bot is a UI one-way door (data layer correct/reversible; fix = surface designated chars in the panel). Surfaced live: tester designated all 4 assigned chars → panel emptied → scoped direct-DB recovery (R2 backup first; cleared flags on ids 1/14/15/16 + re-seed assigned_by='migration'; Findom untouched; +assignment_restore audit rows). DEFERRED (tester's discretion): non-officer /admin 403 UI-collapse (needs a 2nd account; server gate already code-verified via TestOfficerEndpoints_NonOfficer_Rejected) + the Request→Cancel (A5)/Approve-Deny (B4) interactive flows (UI trigger can't occur today — claimable=unassigned-only; API test-covered). Optional: /gsd-ui-review 26. Next: /gsd-plan-phase 27 (stacks on committed code). v2.3 Phase 26 SHIPPED + UAT-verified.
PRIOR planning state:
Plan: 3 plans (26-01 store/migration → 26-02 API → 26-03 web), 3 waves, ASSIGN-01..06 covered. RESEARCH.md + PATTERNS.md + 3 PLAN.md written; plan-checker: 0 blockers / 4 warnings (all fixed in a revision pass).
Status: Planning complete. Locked resolutions from research: guild bank SUBSUMES is_bank_toon (officer-only; single-bank invariant RELAXED — multiple banks OK, bank view unchanged, 2-bank test added); guild bot = NEW is_guild_bot column; character_assignment PK on character_id = single-assignee; assignment_request (pending|approved|denied|cancelled) + partial-unique-pending; idempotent INSERT OR IGNORE auto-seed from owner.discord_user_id; goose v9, NO _meta.schema_version cell; owner_id never re-homed; watcher UNTOUCHED. Each plan has a STRIDE threat_model. Browser-smoke gap flagged for /gsd-ui-review (web tests node-only). NOT YET EXECUTED — Phase 26 writes a LIVE-backend schema migration (00009) + API + web; execution + deploy is a deliberate next step. Next: `/gsd-execute-phase 26`.
Last activity: 2026-06-08 — Phase 26 planned (research → pattern-map → 3 plans → checked → revised). Next: `/gsd-execute-phase 26`.

## v2.3 Phase Plan (created 2026-06-08)

Phases continue from v2.2 (which ran 19–25). Phase dirs 11–25 exist on disk — never reuse them. v2.3 starts at **26**.

**Execution order:** strict dependency chain **26 → 27 → 28**. The character-assignment data layer (P26) is the foundation BOTH the my-characters filter (P27) and the character-tagged wantlist (P28) build on — each needs the authoritative "which characters are mine" answer.

| Phase | Name | Requirements | Success Criteria | UI |
|-------|------|--------------|------------------|----|
| 26 | Character Assignment | ASSIGN-01, ASSIGN-02, ASSIGN-03, ASSIGN-04, ASSIGN-05, ASSIGN-06 | 6 | yes |
| 27 | My-Characters Inventory Filter | MYVIEW-01, MYVIEW-02 | 3 | yes |
| 28 | Character-Tagged Wantlist | CWANT-01, CWANT-02, CWANT-03, CWANT-04, CWANT-05, CWANT-06 | 4 | yes |

**Phase 26 — Character Assignment (foundation):** `00009` goose migration → schema **v9** adding the `character_assignment` layer over the existing `character.owner_id` upload provenance; member self-claim/release + officer assign/reassign API (IDOR-safe, owner/officer-scoped via the ADMIN-04..06 gate, audited); the "My characters" + officer admin-panel UI. **Open sub-decision:** ASSIGN-05 single- vs multi-owner for shared bank toons (resolve in spec/plan; likely many-to-many).

**Phase 27 — My-Characters Inventory Filter:** an ADDITIVE "my characters" quick-filter + single-character drill-down over the existing all-members consolidated views (inventory/bank/gear/spell). Visibility stays all-members — the CLAUDE.md consolidated-views rule is intact; this is NOT access control. Read-only/client-side over the existing DataGrid; no per-character view tabs.

**Phase 28 — Character-Tagged Wantlist:** the wantlist gains an OPTIONAL character dimension — NULLABLE `character_id` on `wantlist_item` (00009 backfills existing wants to NULL, no data loss); character-tagged wants roll up into the guildwide wantlist; per-user filter/group-by-character; the EC-tunnel monitor DM still targets the character's OWNER (keys on `discord_user_id`). **Open sub-decisions:** CWANT-04 guildwide attribution display; CWANT-05 whether the EC embed names the character.

## Milestone v2.3 Scope (locked 2026-06-08)

**Goal:** Associate SquireBot users with specific characters, let them view those characters' inventory, and create character-tagged wantlist items that roll up to the guildwide wantlist.

**Locked decisions:**
- **Assignment = BOTH** — member self-claim (incl. characters under an unlinked/legacy owner) AND officer assign/override/reassign.
- **Inventory = ALL-MEMBERS + FILTER** — keep the all-members consolidated views (CLAUDE.md LOCKED); ADD a "my characters" filter/drill-down. ADDITIVE, not an access-control restriction.
- **Wantlist = CHARACTER-TAGGED** — wants gain an optional character dimension and aggregate into the guildwide wantlist; the EC-monitor DM still targets the OWNER.

**Scope:** backend (`internal/backendsrv`) + web (`web/`). The Go **watcher is untouched** (backend-only schema; no `WatcherMaxSchemaVersion` concern). New `00009` migration → schema **v9** (extend-only, version-stamped, idempotent; `character_id` NULLABLE).

**Open sub-decisions (resolve in spec/plan):** ASSIGN-05 (multi-user shared bank toons), CWANT-04 (guildwide attribution display), CWANT-05 (EC embed names the character).

<details><summary>v2.2 history (parked — Track 2 invite-gated; preserved below)</summary>

Phase: 25 — Linux Watcher — **COMPLETE (3/3 plans)**. v2.1.1 watcher SHIPPED 2026-06-08 (tag pushed; release.yml green; latest.json permalink → 2.1.1; win+linux assets reachable; bundles Linux watcher P25 + eqfind EQLite autodetect quick 260607-vgb). Earlier 2026-06-08: 260607-sdh web gear-menu IA cleanup DEPLOYED to prod via tarball+atomic-swap. v2.2 Track 1 (P19-21) SHIPPED LIVE; Track 2 (22-23 Discord pinger) parked on the 3 Raid Alliance bot invites. Detailed P21/P25 activity logs retained below.
</details>

Phase 25 post-execution (2026-06-06): verified (6/6 criteria code-verified, human_needed — 4 on-machine UATs in 25-HUMAN-UAT.md). Code review found 2 BLOCKERS + 6 warnings — both blockers were install-flow bugs (CR-01: `--status` always exit 0 → install.sh skipped first-run `--setup` so a fresh box sat red; CR-02: per-prompt bufio.Reader dropped piped stdin) — ALL 8 FIXED (25-REVIEW-FIX.md; commits bebe961/b12f013/0ac0bc0/f7f8736/bd197d7/c47efdc/81b4b28), gates re-green. Local snapshot tarball assembled + inspected (static ELF + README + systemd unit + install.sh; `install -Dm755` sets the exec bit on install). HANDOFF: ready for a Linux guildie to smoke-test (run install.sh → --setup → play P99 → confirm upload on squirebot.quest); canonical release tarball builds via release.yml on a tagged release.

Phase 25 activity (25-03, FINAL): 2026-06-06 — 3 task commits (6e31ec3, 1a3c239, 5b09a4f). (1) `internal/update/manifest.go` gained additive `BinaryURLLinux`/`BinarySHA256Linux` (omitempty, new-fields-only — old Windows-only latest.json still parses) + a `Manifest.binaryAsset() (url,sha)` method keyed on `runtime.GOOS` (windows → the `.exe` pair, else → the linux pair); `check.go` does `binURL,binSHA := m.binaryAsset()` and routes all four bare-binary read sites through it, so a linux box reading a windows-only manifest gets `("","")` → the existing "missing binary_url; skip" no-op fires (it NEVER downloads the `.exe` — Pitfall 3/T-25-10); the mandatory SHA-256 verify is unchanged. Tests: linux-field parse, legacy back-compat parse, GOOS selection, wrong-OS-asset-only no-op. (2) `packaging/linux/`: systemd USER unit (`Type=simple`, `ExecStart=%h/.local/bin/squirebot`, `Restart=always`, `RestartSec=5`, `NoNewPrivileges=true`, `WantedBy=default.target`, NO $HOME-protection — must read the EQ folder); idempotent POSIX `install.sh` (`set -eu`; `install -Dm755` binary → `~/.local/bin`, `install -Dm644` unit → `~/.config/systemd/user`, daemon-reload, first-run `--setup` only when `--status` reports unconfigured, `enable --now`; opt-in `--linger` runs `loginctl enable-linger` ONLY in the flag branch — T-25-13); Linux README. `.gitattributes` forces LF on `packaging/linux/*.sh`+`*.service` (a CRLF shebang would break `#!/bin/sh` — Rule 2 deviation; staged blob verified CR=0). (3) `release.yml`: additive CGO-free linux build (`GOOS=linux`, no `-H=windowsgui`), `squirebot-linux-amd64.tar.gz` assembly (binary+README+unit+install.sh), bare-binary `binary_sha256_linux`, `binary_url_linux`/`binary_sha256_linux` in the manifest, both linux assets uploaded — Windows NSIS/installer/`.exe` steps UNTOUCHED (squirebot.exe ×12, makensis ×12, windows binary_url present, -H=windowsgui ×2). Gates: `go test ./internal/update/...` + whole-module `go test ./...` PASS on the host; `GOOS=linux go vet ./internal/update/...` clean; cross-compile → static ELF (magic `\x7fELF`); linux closure systray=0/sqweek=0; Windows build OK; `sh -n install.sh` clean; `ProtectHome` count 0; release.yml YAML valid (yaml.v3). Phase 25 COMPLETE (3/3); LNX-01..06 code-complete; the live "watches WINE EQ folder + uploads + autostarts + self-updates" remains a human UAT on a real Linux+WINE box.

Phase 25 activity (25-02): 2026-06-06 — 3 task commits (09d6d72, 66eac47, 6aa2d16). (1) credstore split: `git mv store.go/_test.go → store_windows.go/_test.go` (//go:build windows, wincred body byte-unchanged) + new `store_other.go` (!windows) 0600-file Store/Read/Delete via atomic tmp+rename under XDG config (dir 0700), with a TestFileStore that asserts exact `Perm()==0o600` AND that re-Store over a pre-existing 0644 file tightens to 0600 (Pitfall 4 / T-25-05). (2) eqfind: relaxed `defaultHeuristicScan`'s GOOS guard to `!= windows && != linux` (darwin still no-op); moved `defaultKnownPaths` OUT of discover.go into `knownpaths_windows.go` (C:\ literals) / `knownpaths_other.go` ($WINEPREFIX/~/.wine direct-hits, ZERO C:\ on linux); `heuristic_other.go` is now a real bounded WINE walk (WINEPREFIX→~/.wine→Lutris incl Flatpak→Bottles incl Flatpak→Steam/Proton compatdata; depth 5, 30s timeout, no-symlink, prune users/ProgramData/Recycle but NOT Program Files — RESEARCH A1/T-25-07); discover.go residual os/filepath/runtime imports stay referenced (vet-confirmed, no stranded import). (3) onboarding: `dialog_other.go` is now real stdin PromptGuildCode/PickEQFolder (trim, empty/EOF→ErrCancelled, ~/$VAR expansion) over a `var stdin io.Reader` test seam — no browser/HTTP surface; dropped the now-false TestNonWindows_Unsupported; ErrUnsupported sentinel retained. NEW additive `app.RunSetup` (onboarding-then-return, never watch.Run) beside the byte-unchanged RunApp (D-07 grep-asserted). main.go gained `--setup` (eqfind.Discover pre-fill → RunSetup → exit) + `--status` (health, configured/not-configured boolean, NEVER the code — V7/T-25-06) dispatch after logging.Setup+config.Load. Gates: linux build/vet clean, systray=0 + sqweek=0 in the linux closure, Windows `go test ./...` 0 failures. The three `!windows` test files are compile-verified here (Windows host can't run an ELF) + run-verified on linux CI; store_windows_test stays run-verified on the Windows box (real DPAPI). Deferred fix in 25-03: packaging (tarball/systemd/install.sh) + manifest OS-asset selection so a linux box doesn't self-update to the Windows .exe (Pitfall 3).

Phase 25 activity: 2026-06-06 — 25-01 executed (3 task commits: bb8e214, e84c6c4, 52b96c6). The highest-risk structural item of the phase: `fyne.io/systray` (CGO/GTK on Linux) was imported at TWO un-tagged sites — `internal/tray/tray.go` AND `cmd/squirebot/main.go`. Both build-tag-split: (1) `git mv tray.go→tray_windows.go` (//go:build windows, bytes unchanged) + new `tray_other.go` (//go:build !windows) reproducing the IDENTICAL exported `*tray.Controller` API as slog no-ops (RunApp signature byte-identical, D-07); test files paired the same way. (2) main.go drops the systray import; its shutdown-listener goroutine + `systray.Run` tail extracted into `run_windows.go` (//go:build windows, byte-equivalent) / `run_other.go` (//go:build !windows: `signal.NotifyContext(ctx, SIGINT, SIGTERM)` → cancel() → `<-ctx.Done()`, no systray). (3) `config.defaultPath()` + new `logging.defaultLogDir()` branch on `runtime.GOOS` — XDG config/state on Linux, `%LOCALAPPDATA%` on Windows; new XDG-branch unit tests. Gates all green: linux closure systray count = 0, dialog/sqweek count = 0, `go.mod` still lists systray (windows uses it), Windows build OK, host `go test ./...` 0 failures, `GOOS=linux go vet ./...` clean.

Track-1 closeout 2026-06-06: cross-compiled linux/amd64 → scp → install (kept `.bak`) → `systemctl restart` per `docs/backend-deploy.md`; `goose.Up` auto-applied `00008_ec_cursor` (schema **v7 → v8**, `ec_auction_cursor` table created); pre-deploy R2 backup taken. Startup logs clean: `bot connected` (guild 1483502186351562925), `listening 127.0.0.1:8090`, `scheduler started interval 10m0s jobs:4` (the +1 = `ec_auction_match`). UAT #1 (live EC DM) tracked in `21-HUMAN-UAT.md`, confirms ORGANICALLY on the first real matching tunnel auction (D-07 coverage-dependent; send path already proven live in P20). UAT #2 (live scheduler poll) confirmed at the ~10-min tick. Next: pursue the 3 Raid Alliance invites (user) to unblock Track 2 → `/gsd-discuss-phase 22`; otherwise v2.2 Track 1 stands as the delivered value.

Phase 21 activity: 2026-06-06 -- 21-03 executed (the integration finale, WANT-05). NEW `internal/backendsrv/ec` package: `RunMatch` composes poll→diff→match→embed→send — polls PigParse `getdetails/0/{itemname}` per wanted item (SPIKE: server=0 live feed, NAME key form), diffs new WTS auctions (`u∈{0,2}`; WTB never alerts) against the per-item `ec_auction_cursor` with first-sight baseline (no replay) + advance-only-on-success, matches via `wantmatch.ForItem`, and DMs a discordgo rich embed through `notify.Send` (Source `ec_auction` + WantID + Embed) — re-implementing NONE of the P20 spine (both gates + dedup + cooldownEC=22h + alert_log inherited). Registered as the `scheduler.ec_auction_match` job (~10-min `dueEC` cadence); `scheduler.Start` gained a `botSession *discordgo.Session` param and `main.go` was reordered to start the bot BEFORE the scheduler so the live session is threaded `main.go → scheduler → ec`. nil-session no-op (typed-nil-interface guard). `go test ./...` + `go vet ./internal/backendsrv/...` + `go build ./...` all green. 3 task commits (bec919c, 38412ec, 5dcd384). Next: `/gsd-discuss-phase 22` (WTS Cross-Server, invite-gated) or close v2.2 if Track 2 stays gated.
Earlier: 21-02 executed. Extended the P20 notify spine with a discordgo rich-embed send-path (D-04): added `ChannelMessageSendEmbed` to the `Sender` interface (the real `*discordgo.Session` already satisfies it — assertion holds) + an `Embed` field on `Alert`; the single SEND step branches on `a.Embed != nil` so the embed rides the EXACT same two-gate + dedup/cooldown + alert_log core (verified: `grep -c GetMonitorFlags` = 1, no duplicate gate path). Also added `wantmatch.Hit.Note *string` (D-05 "why you wanted it"), carried by the shared `scanHits` for BOTH `ForItem` and `ForName` (one scanner change). Backend-only, no web/. notify + wantmatch package tests + `go build ./...` + `go vet ./internal/backendsrv/...` green. 2 task commits (72edf87, 92b8fa8). Next: `/gsd-execute-phase 21` plan 21-03 (ec package RunMatch poll→diff→match→embed→send + scheduler ec_auction_match job + main.go bot/scheduler reorder).
Earlier: 21-01 executed (the GATING plan). Mandatory PigParse feasibility spike ran live (no checkpoint, D-08): **path chosen = per-auction `getdetails`** (presence + freshness threshold met). Two research corrections recorded in 21-SPIKE.md: (1) the LIVE Blue getdetails feed is **server=0**, NOT server=1; (2) the only working query key is the item **NAME** (the id form 400s). Delivered: `enrich.ParseItemDetail`, migration `00008_ec_cursor.sql`, and `store.GetECCursor`/`SetECCursor`/`ECPollSet`. 3 task commits (0f29fcc, ff01c77, 486cff2).
Progress: Phase 21 — 3/3 plans complete (COMPLETE)

## v2.2 Phase Plan (created 2026-06-02)

Phases continue from v2.1 (which ended at Phase 18). Phase dirs 11–18 exist on disk — never reuse them. v2.2 starts at **19**.

**Execution order:** 19 → 20 → 21 (Track 1, UNBLOCKED — strict dependency chain) → 22 → 23 (Track 2, INVITE-GATED). 22/23 depend on the Phase 20 spine but are otherwise gated purely on the external Raid Alliance invites; if invites land early they can slot earlier, and if they never land Track 1 still delivers a complete, valuable feature.

| Phase | Name | Track | Requirements | Success Criteria | UI |
|-------|------|-------|--------------|------------------|----|
| 19 | Wantlist CRUD | 1 (unblocked) | WANT-01, WANT-02 | 4 | yes |
| 20 | Bot + DM + Notification Infrastructure | 1 (unblocked) | WANT-03, WANT-04, WANT-08 | 5 | no |
| 21 | EC-Tunnel Auction Monitor | 1 (unblocked) | WANT-05 | 4 | no |
| 22 | WTS Cross-Server Monitor | 2 (invite-gated) | WANT-06 | 3 | no |
| 23 | Quest-Target Raid Monitor | 2 (invite-gated) | WANT-07 | 3 | no |

**Phase 19 — Wantlist CRUD:** pure web feature, the product surface — `00006_wantlist.sql` (≥ `wantlist_item`, `alert_log`); `webadmin/wantlist.go` (the `account.go` twin: login-only, IDOR-safe, audited); `web/src/routes/wantlist/` reusing the existing item catalog for add-item search; already-in-bank flag. No Discord yet.

**Phase 20 — Bot + DM + Notification Infrastructure (keystone):** in-process discordgo gateway goroutine (`recover()`-isolated, non-fatal start, reconnect, `DISCORD_BOT_TOKEN` env); `notify` DM open+send + 50007 → `dm_blocked`; `wantmatch` shared matcher (`ForItem`/`ForName`); `alert_log` dedup/cooldown; opt-in/prefs + in-site notification inbox; per-monitor enable/disable + `guild_channel` config feature flags. The spine all three monitors ride.

**Phase 21 — EC-Tunnel Auction Monitor:** first real alert. MANDATORY first task = the PigParse feasibility spike (confirm timestamps advance + coverage; decides per-auction `getdetails` vs coarser `lastWTSSeen` trigger). `scheduler.ec_auction_match` poll-and-diff on `ec_auction_cursor` → `wantmatch.ForItem` → `notify`. Advance-cursor-only-after-success; no backlog replay on restart.

**Phase 22 — WTS Cross-Server Monitor (INVITE-GATED):** entry-precondition = 3 Raid Alliance bot invites confirmed in writing + `MESSAGE_CONTENT` enabled. Build/test the name/alias matcher (`wantmatch.ForName`) against a fixture corpus first; `bot/wts.go` MESSAGE_CREATE + content-non-empty smoke test; reuse `notify`/`alert_log`. Stays dark behind feature flag until invites land.

**Phase 23 — Quest-Target Raid Monitor (INVITE-GATED):** entry-precondition = invites + `MESSAGE_CONTENT` AND the curated `quest → NPC` table populated (curation can start in parallel during Track 1). `bot/raidtarget.go` MESSAGE_CREATE → NPC detect → `quest_target` → existing `quest_items` → `wantmatch.ForItem` → `notify`. Chain: NPC → quest → item → wantlister.

## Milestone v2.2 Scope (locked 2026-06-02)

**Goal:** Guildies maintain a personal wantlist on squirebot.quest and get DMed on Discord when a wanted item appears — at EC-tunnel auction, in cross-server WTS channels, or as a raid-target tied to a wanted item's quest.

**Locked decisions (v2.2 research):**

- **Delivery/reading split** — DM delivery is UNBLOCKED (bot in the guild's own Discord reaches every guildie); only WTS/raid *reading* is invite-gated. Drives Track 1 (P19–21) vs Track 2 (P22–23).
- **In-process bot goroutine, not a separate process** — avoids two SQLite writers; `recover()`-isolated + non-fatal start + `Restart=always`. Lib: `bwmarrin/discordgo` v0.29.0 (CGO-free, the only new dependency).
- **EC = PigParse poll-and-diff** (~10-min cadence; no push feed) — live spike first (Phase 21's first task).
- **One match seam, three sources** — shared `wantmatch` + `notify` + `alert_log` (dedup/cooldown), built once in Phase 20; EC/WTS/raid fan in.
- **50007 (can't-DM) first-class** — in-site notification inbox is table stakes, the fallback that makes the unreliable DM channel acceptable.
- **`MESSAGE_CONTENT`** is a self-serve dev-portal toggle (no Discord audit under 100 servers) — gates Track 2 reading only.
- **HARD CONSTRAINT (carried):** never put a Discord bot or OAuth in the watcher — untouched this milestone.

**New schema:** one goose migration `00006_wantlist.sql` — `wantlist_item`, `alert_log`, `quest_target`, `guild_channel`, `ec_auction_cursor`. `quest_target` adds only the inverse quest → raid-NPC hop; reuses the existing `quest_items` table.

## External prerequisites (gate Track 2 only)

| # | Prerequisite | Gates | Status |
|---|--------------|-------|--------|
| 1 | 3 Raid Alliance Discord bot invites (WTS-channel read + `MESSAGE_CONTENT`) | WANT-06 (P22), WANT-07 (P23) | Un-negotiated (external/human) |
| 2 | Curated `quest → raid-target NPC(s)` lookup populated (seeded from wiki) | WANT-07 (P23) | Not started; can curate in parallel now |

## Deferred Items

Carried forward at v2.1 close (non-blocking):

| Category | Item | Status |
|----------|------|--------|
| ops | Decommission the Azure PAYG test VM (the `0.4.0-rc1` box) to stop billing | ✅ done 2026-06-02 (user decommissioned the VM) |
| signing | 999.9 — SignPath Foundation OSS approval | in flight (lands as a hotfix when approved) |
| uat | P12/P14/P15 HUMAN-UAT live-smoke checklists (from v2.0) | ✅ closed 2026-06-06 — P14 (5/5) + P15 (7/7) were already `complete`; P12's lone Sheet-parity test marked **obsolete** (the Sheet was decommissioned in P16; enrichment is proven live on the prod scheduler) |

## Files of Record

- `.planning/PROJECT.md` — core value, requirements, constraints, Key Decisions (updated 2026-06-02 with the v2.2 milestone).
- `.planning/REQUIREMENTS.md` — v2.2 requirements + Traceability (8/8 mapped to Phases 19–23).
- `.planning/ROADMAP.md` — phase structure (v2.2 Phases 19–23 appended; all prior-milestone history preserved) + Progress tables + Backlog.
- `.planning/research/SUMMARY-v2.2.md` — synthesized v2.2 research (authoritative technical guidance + the 5-phase shape).
- `.planning/research/ARCHITECTURE-v2.2.md` — component/table/build-order detail.
- `.planning/MILESTONES.md` — historical record (v1.0 + v1.0.1 + v2.0 + v2.1).
- `.planning/milestones/v2.1-{ROADMAP,REQUIREMENTS}.md` — the v2.1 archive.
- `.planning/config.json` — granularity (coarse), mode (yolo), workflow toggles.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260602-u7m | Church of Clean Code audit follow-up — save report + low-risk fixes (.gitignore, CLAUDE.md dep doctrine, enrich naming) | 2026-06-03 | 922b441 | [260602-u7m-church-audit-follow-up-save-report-low-r](./quick/260602-u7m-church-audit-follow-up-save-report-low-r/) |
| 260607-sdh | Reorganize web top bar into an accessible header settings dropdown (gear menu): theme picker + Watcher-codes + Character-details + officer Admin + identity/Sign-out fold in; header slims to Wantlist + Notifications + gear. Absorbed & deleted SessionIndicator. | 2026-06-08 | bdde646 | [260607-sdh-reorganize-web-top-bar-into-accessible-s](./quick/260607-sdh-reorganize-web-top-bar-into-accessible-s/) |
| 260607-vgb | eqfind autodetect: add EQLite-bundle + system-location coverage (Linux `/opt/...EQLite` direct-hit + `systemCandidateRoots` walk; HOME `~/Desktop/EQLite`; Windows `%USERPROFILE%\Desktop\EQLite` etc.). Fixes a guildie's `/opt/everquest/EQLite` being invisible (Linux scan was WINE-`drive_c`-only). Ships to guildies only via a tagged release (auto-update). | 2026-06-08 | add17e9 | [260607-vgb-add-eqlite-bundle-system-location-covera](./quick/260607-vgb-add-eqlite-bundle-system-location-covera/) |

---

*State updated: 2026-06-02 for v2.2 "Wantlist + Discord Pinger". Roadmap created 2026-06-02 (Phases 19–23; continues numbering from 18). Prior: v1.0 (2026-05-11), v1.0.1 (2026-05-12), v1.0.2 binary (2026-05-13, superseded), v2.0 (2026-05-31), v2.1 (2026-06-02).*
