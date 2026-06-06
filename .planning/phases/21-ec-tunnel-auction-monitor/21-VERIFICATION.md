---
phase: 21-ec-tunnel-auction-monitor
verified: 2026-06-05T00:00:00Z
status: human_needed
score: 4/4 code truths verified (1 deploy-time UAT pending)
overrides_applied: 0
human_verification:
  - test: "Deploy the new server binary and trigger a real EC-tunnel WTS auction of a wanted item (or wait for one), then confirm the wantlister receives the rich-embed DM (item + price + WTS tag, seller best-effort) on the guild's own Discord."
    expected: "A discordgo rich embed DM arrives carrying the item name + ' — WTS' title, a ~price pp field (or omitted when unknown), a 'Seen ~N min ago' field, optional seller, a 'Why you wanted it' field, and a wiki link; alert_log records a 'sent' row; a repeat poll within 22h does not re-DM."
    why_human: "ROADMAP criterion 3's live-delivery guarantee ('the wantlister receives a DM') cannot be proven programmatically — it requires a real deploy + a live PigParse-fed auction + a live Discord session. The code path is fully unit-tested (TestRunMatch_NewWTS_Sends, TestBuildEmbed_*) and the P20 spine DM path was already proven live, but the EC embed end-to-end is a deploy-time UAT (21-03-SUMMARY §'Post-Deploy Smoke — pending deploy')."
  - test: "On the deployed box, confirm the ec_auction_match scheduler job actually runs on its ~10-min cadence."
    expected: "Journal/logs show periodic 'ec_auction_match' poll lines (source=ec_auction) and the per-item ec_auction_cursor advances; the HTTP API + ingest stay up (bot/scheduler panic isolation holds)."
    why_human: "Scheduler cadence + live polling against PigParse can only be observed on the running box; the unit suite proves the Due predicate and RunMatch logic but not live execution."
---

# Phase 21: EC-Tunnel Auction Monitor Verification Report

**Phase Goal:** The first real end-to-end alert ships — when a wanted item is auctioned in the EC tunnel, the wantlister gets a DM (price + WTS tag; seller best-effort), all on the guild's own Discord. Delivered via a `scheduler.ec_auction_match` job that polls PigParse per wanted item (~10-min), diffs on an auction-timestamp cursor, exact-item-ID matches against wantlists, and DMs via the Phase 20 notify/wantmatch/alert_log spine. Gated by an upfront mandatory PigParse feasibility spike.
**Verified:** 2026-06-05
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP criterion) | Status | Evidence |
|---|---------------------------|--------|----------|
| 1 | Mandatory PigParse feasibility spike ran, recorded a go-decision + coverage, and the poll code uses the spike-corrected `getdetails/0/{name}` (server=0, NAME key) — NOT the stale server=1/id form | ✓ VERIFIED | `21-SPIKE.md` is a 78-line live-run record: verdict "Adopt per-auction `getdetails`", critical finding "live Blue tunnel is server=0 NOT server=1", "NAME, not ID" key decision. `ec/urls.go:30` `getDetailsBase = ".../getdetails/0/"`; `getDetailsURL(itemName)` keys on `item.ItemName` (`ec/ec.go:95`). NO id-form / server=1 in production code. |
| 2 | A `scheduler.ec_auction_match` job polls per wanted item ~10-min, diffs the per-item `ec_auction_cursor`, exact-item-ID matches via `wantmatch.ForItem`; poll set = active catalog wants (item_id NOT NULL), both buy+quest (D-01), WTS-only (D-02), custom NULL skipped (D-03) | ✓ VERIFIED | `scheduler.go:179-184` registers `ec_auction_match` with `Due: dueEC` (`>= 10*time.Minute`, scheduler.go:94-96). `ec.RunMatch` → `s.ECPollSet` (`store/eccursor.go`: `SELECT DISTINCT item_id, item_name FROM wantlist_item WHERE active=1 AND item_id IS NOT NULL` — reason NOT filtered, NULL skipped). Diff `a.T <= cursor` skip + `a.U != 0 && a.U != 2` skip (ec.go:138-143). `wantmatch.ForItem(ctx, db, item.ItemID)` (ec.go:144). Tests: `TestECPollSet`, `TestRunMatch_NewWTS_Sends`. |
| 3 | On a new auction the DM carries item + price + WTS tag (seller best-effort), routed THROUGH notify.Send (Source "ec_auction") inheriting both gates + dedup + cooldown + alert_log — NOT a direct ChannelMessageSendEmbed from the job | ✓ VERIFIED (code) / ⚠ live delivery pending | Every send goes through `notify.Send` with `Alert{Source:"ec_auction", WantID:&…, Embed:…}` (ec.go:183-192). `grep ChannelMessageSendEmbed` in `ec/` production = NONE (only the test fake). `notify/dm.go`: single gate block (`grep -c GetMonitorFlags` = 1), `a.Embed != nil` branch (dm.go:179), `ec_auction` → cooldownEC=22h + monitor_flag.EC + notify_prefs.EC gates. `buildEmbed`: Title `name + " — WTS"`, price omitted when nil, seller best-effort. **Live DM delivery is a deploy-time UAT — see human_verification.** |
| 4 | Per-item cursor advances ONLY after a successful poll; job does NOT replay backlog on restart (first-sight baseline) — covered by a test | ✓ VERIFIED | `pollItem`: fetch fail / parse fail / 304 all `return` WITHOUT SetECCursor (ec.go:96-116); first-sight (`!ok`) sets cursor to max(t) and returns WITHOUT DMing (ec.go:123-130); advance to maxT only after processing (ec.go:157). Tests: `TestRunMatch_FirstSightBaseline_NoReplay` (asserts 0 embeds, 0 sends, 0 alert_log rows, cursor=max(t)) + `TestRunMatch_AdvanceOnlyOnSuccess` (failed item's cursor stays at baseline, loop survives). |

**Score:** 4/4 code truths verified. Truth #3's live-delivery half is a deploy-time UAT (the code path is fully unit-tested; only the real DM-on-the-box is unverifiable here).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/backendsrv/enrich/pigdetails.go` | `ParseItemDetail` + t/u-collision-aware types | ✓ VERIFIED | 153 lines; `ParseItemDetail` present, import-pure, malformed-tolerant; 7 tests incl. `TestParseItemDetail_TCollisionRegression`, `_NullablePrice`, `_TruncatedBodyErrorsNoPanic`. |
| `internal/backendsrv/enrich/__fixtures__/pigparse-getdetails-fungi.json` | real server=0 NAME-keyed fixture | ✓ VERIFIED | Valid JSON; `u=0` WTS records with `t` timestamps from the live 02:xx feed + `p:null` (nullable price). |
| `internal/backendsrv/migrations/00008_ec_cursor.sql` | `ec_auction_cursor` table, extend-only | ✓ VERIFIED | `CREATE TABLE ec_auction_cursor (item_id PK, last_seen_t TEXT, updated_at INTEGER)`; forward-only Down no-op; no schema_version bump (backend-only). `TestMigrate_00008_AddsECCursor` asserts table + cols + idempotent re-apply. 00001-00007 byte-unchanged (git-confirmed). |
| `internal/backendsrv/store/eccursor.go` | GetECCursor / SetECCursor / ECPollSet | ✓ VERIFIED | 104 lines; absent-is-(",false,nil) first-sight signal; ON CONFLICT upsert; DISTINCT poll-set query. Tests: `TestECCursor`, `TestECPollSet`. |
| `internal/backendsrv/notify/dm.go` | embed send-path on the SAME gate/dedup core | ✓ VERIFIED | `ChannelMessageSendEmbed` on Sender + `Embed` on Alert + single send branch; `var _ Sender = (*discordgo.Session)(nil)` compiles; exactly 1 gate block. |
| `internal/backendsrv/wantmatch/match.go` | `Hit.Note *string` via shared scanHits | ✓ VERIFIED | `Note *string` on Hit; `reason, note` in BOTH SELECTs (count=2); `h.Note = &v` in shared scanHits. |
| `internal/backendsrv/ec/ec.go` | RunMatch (poll→diff→match→embed→send) | ✓ VERIFIED | 232 lines; `RunMatch` + `pollItem` + `sendHit`; nil/typed-nil session no-op; best-effort per-item isolation. |
| `internal/backendsrv/ec/embed.go` | buildEmbed (D-04/D-05) | ✓ VERIFIED | 176 lines; WTS-tagged title, wiki URL (D-06), price omitted when nil (never 0pp), seller best-effort, why-you-wanted-it from Reason+Note. |
| `internal/backendsrv/scheduler/scheduler.go` | ec_auction_match registry + botSession param | ✓ VERIFIED | `Start(ctx, db, botSession *discordgo.Session)`; `ec_auction_match` job with `dueEC` (~10-min); recover-isolated run loop preserved; two-cursor distinction documented. |
| `cmd/squirebot-server/main.go` | bot before scheduler, session threaded, non-fatal | ✓ VERIFIED | `bot.Start` (line 241) before `scheduler.Start(ctx, db, botSession)` (line 255); non-fatal bot-start (line 243 log+continue); nil-session guarded. |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `ec.RunMatch` | `wantmatch.ForItem` | per new WTS auction → ForItem(item_id) → notify.Send | ✓ WIRED (ec.go:144,192) |
| `ec.RunMatch` | `store.SetECCursor` | advance to max(t) only after item poll succeeds | ✓ WIRED (ec.go:124,157) |
| `scheduler.Start` | `ec.RunMatch` | registry entry passes botSession + politefetch.Fetch | ✓ WIRED (scheduler.go:182) |
| `main.go bot.Start` | `scheduler.Start` | bot started first so botSession is live | ✓ WIRED (main.go:241→255) |
| `notify.Send` embed branch | `discordgo.ChannelMessageSendEmbed` | `a.Embed != nil` → ChannelMessageSendEmbed | ✓ WIRED (dm.go:179-180) |
| `wantmatch.ForItem` | `wantlist_item.note` | SELECT … note → Hit.Note | ✓ WIRED (match.go, count=2) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whole tree compiles | `go build ./...` | exit 0 | ✓ PASS |
| Full test suite | `go test ./...` | all packages ok (ec, notify, wantmatch, scheduler, migrations, store) | ✓ PASS |
| Static analysis | `go vet ./internal/backendsrv/ec/... notify/... scheduler/...` | exit 0 | ✓ PASS |
| No direct embed send in ec/ prod | `grep -rl ChannelMessageSendEmbed ec/ --exclude=*_test.go` | NONE | ✓ PASS |
| Migrations 00001-00007 byte-unchanged | `git diff --name-only` over phase-21 range | only 00008 + migrate_test.go | ✓ PASS |
| Live EC embed DM on the box | (deploy + real auction) | not run | ? SKIP → human |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WANT-05 | 21-01/02/03 (frontmatter) | EC monitor polls PigParse per wanted item (~10-min, diff on auction-timestamp cursor) + DMs on auction (price + WTS/WTB; seller best-effort); includes upfront feasibility spike | ✓ SATISFIED (code) | REQUIREMENTS.md:37 marks WANT-05 `[x]` Phase 21; line 81 traceability "Complete (2026-06-06)"; 8/8 mapped. All 4 ROADMAP criteria have code + tests; live delivery is the pending deploy UAT. No orphaned requirements for Phase 21. |

### Locked Decisions (D-01..D-11) Honored

| Decision | Honored | Evidence |
|----------|---------|----------|
| D-01 both buy+quest fire | ✓ | ECPollSet does NOT filter reason. |
| D-02 WTS-only, WTB never alerts | ✓ | `a.U != 0 && a.U != 2` skip; `TestRunMatch_WTBIgnored`. |
| D-03 custom NULL wants skipped | ✓ | `item_id IS NOT NULL` in ECPollSet. |
| D-04 rich embed | ✓ | `buildEmbed` → discordgo.MessageEmbed. |
| D-05 fields (price/seller/note best-effort) | ✓ | price omitted when nil, seller omitted when unresolved, whyWanted from Reason+Note. |
| D-06 wiki link (backend-only, no new web route) | ✓ | `wikiURLFor` replicates composeNotes idiom; zero web/ files changed in phase 21. |
| D-07 ship on thin coverage, document | ✓ | 21-SPIKE coverage caveat + best-effort per-item isolation. |
| D-08 no checkpoint, threshold applied | ✓ | spike ran yolo, path chosen, proceeded. |
| D-09 polite (only poll wanted items, sane cadence) | ✓ | poll set from wantlist only; 10-min dueEC. |
| D-10 cooldown per-source constant | ✓ | cooldownEC=22h in dm.go. |
| D-11 time-based re-list re-alert | ✓ | cursor diff is time-only; price not in dedup key. |

### Anti-Patterns Found

None. No TODO/FIXME/placeholder in the phase files; no stub returns; no hardcoded-empty data flowing to output. The `return ""` paths in embed.go (seller/seenAgo) are intentional best-effort omissions (D-05), not stubs. The `urls.go` server=0/NAME form correctly overrides the plan's stale id-form text per the spike.

### Human Verification Required

1. **Live EC embed DM** — After deploy, trigger/await a real EC-tunnel WTS auction of a wanted item and confirm the wantlister receives the rich-embed DM (item + price + WTS tag, seller best-effort) on the guild's Discord; confirm `alert_log` records a 'sent' row and a repeat within 22h does not re-DM.
   - Why human: ROADMAP criterion 3's live-delivery guarantee needs a real deploy + live PigParse feed + live Discord session. Code path fully unit-tested; this is the documented deploy-time smoke (21-03-SUMMARY §"Post-Deploy Smoke — pending deploy").

2. **Scheduler cadence on the box** — Confirm the `ec_auction_match` job runs ~every 10 min and the per-item cursor advances, with the HTTP API + ingest staying up (panic isolation).
   - Why human: live execution cadence is only observable on the running box.

### Gaps Summary

No blocking gaps. All four ROADMAP success criteria are implemented in committed, building, passing code, with dedicated behavioral tests for the load-bearing guarantees (first-sight baseline / no-replay, advance-only-on-success, WTS-only matching, embed-through-notify.Send). The single most important spike output — the server=0 + NAME-key correction — is correctly reflected in the actual poll code (`getdetails/0/{name}`), overriding the plan's stale id-form text. WANT-05 is code-satisfied. The phase goal ("the first real end-to-end alert ships") includes a live DM delivery that can only be confirmed by a deploy-time UAT — hence `human_needed` rather than `passed`, per the verification decision tree.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
