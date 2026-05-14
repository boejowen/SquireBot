# Pitfalls Research

**Domain:** Windows desktop watcher + per-user Google OAuth + shared Google Sheet + light scraping (P1999 wiki, PigParse REST), 12-user guild scale
**Researched:** 2026-04-30
**Confidence:** HIGH for OAuth/SmartScreen/Sheets-API pitfalls (verified against current Google docs and Microsoft Learn); HIGH for `drive.file` and consent-screen verification thresholds; HIGH for Apps Script quotas and Lock Service patterns (verified against current Apps Script docs); MEDIUM for the EQ-specific file-format and patch-history pitfalls (community forum + wiki sources, not Daybreak/Rogean); MEDIUM for OneDrive-DPAPI interaction (documented Microsoft KB plus community reports, but not directly probed on a P99 guildie's machine).

---

## TL;DR — The Five Existential Pitfalls

If only five things go wrong, these are the ones that *kill the product*. Everything else is recoverable.

1. **OAuth consent screen stuck in Testing mode** → every guildie's refresh token expires every 7 days, watcher silently stops, sheet goes stale. Fix is "publish the app to Production *before* shipping," missed if you don't know it exists.
2. **SmartScreen "Unknown publisher" wall** → guildies see a blue full-screen "Windows protected your PC" panel on first run. ~30%+ click "Don't run" and the install ceiling is breached on day one. Fix is code-signing **or** a pre-rehearsed "Click More Info → Run Anyway" walkthrough that ships *with* the installer.
3. **Concurrent writes from 12 watchers using `values.update`** → lost-update bug where the last write wins and earlier guildies' inventory rows vanish. Fix is per-guildie row ranges (no shared range) **and** Apps Script Lock Service for any aggregate writes.
4. **Stale-data trust collapse** → a guildie hasn't run `/outputfile inventory` in 4 months, but their character still appears in the sheet looking current. Officers loot-allocate based on data that's 3 patches old, somebody gets burned, and trust in the sheet evaporates. Fix is a per-character "last updated" timestamp surfaced prominently in every view.
5. **Scope creep into `spreadsheets` or `drive` scope** → shipping with `drive.file` is the only way to avoid Google's restricted-scope verification (third-party security audit, ~$15-75K, 4-6 weeks). One PR that "just adds `spreadsheets` for convenience" turns the project into a months-long compliance exercise.

---

## Critical Pitfalls

### Pitfall 1: OAuth consent screen left in "Testing" — refresh tokens die in 7 days

**What goes wrong:**
Google Cloud Console OAuth apps have two publishing statuses: **Testing** and **Production**. While in Testing with an external user type, refresh tokens issued to non-Google-Workspace users **expire 7 days after consent**, regardless of how short the requested scope is. The watcher silently fails on day 8 with `invalid_grant: Token has been expired or revoked`. Each guildie has to re-OAuth weekly. After two cycles, half the guild stops bothering and the sheet goes stale.

**Why it happens:**
Developers default to Testing because it bypasses the consent-screen warning and feels safer. The 7-day refresh-token expiration is **not** displayed in the Cloud Console alongside the Testing-mode toggle — you have to read the OAuth docs to discover it. Easy to miss in a 12-user guild that *technically* fits inside the Testing-mode 100-user cap, so "we don't need to verify" is a tempting but wrong conclusion.

**How to avoid:**
- Set publishing status to **Production** before shipping the v1 installer.
- With `drive.file` scope (non-sensitive), Production publishing does **not** require Google verification — there is no third-party audit, no demo video, no security questionnaire. You just flip the status. This is the single biggest reason `drive.file` was the right scope choice.
- Confirm post-flip: a fresh OAuth grant on a test guildie's machine, then come back 10 days later and verify the watcher still writes successfully.
- If for any reason you must stay in Testing, surface this fact prominently in setup docs ("re-OAuth weekly is expected") — but don't.

**Warning signs:**
- `invalid_grant` errors in `squirebot.log` exactly 7 days after a guildie's first OAuth.
- Multiple guildies all losing auth at roughly the same calendar time.
- A guildie's data freezes on a date that maps to 7 days post-install.

**Phase to address:**
**Phase 1 (foundation/OAuth setup)** — must be in place before the first guildie installs. This is a "configure once" pitfall, but configuring it wrong is silent for a week, which means easily missed during early test installs.

---

### Pitfall 2: SmartScreen "Unknown publisher" warning blocks installation for non-technical guildies

**What goes wrong:**
When a Windows user double-clicks an unsigned `.exe` downloaded from the internet, Windows attaches the Mark-of-the-Web (MotW) NTFS attribute, and SmartScreen presents a full-screen blue panel: **"Windows protected your PC. Microsoft Defender SmartScreen prevented an unrecognized app from starting."** The default button is "Don't run." A small "More info" link reveals "Run anyway." For a non-technical guildie, this panel reads as malware accusation, not friction. Some percentage abandon the install. The "click installer, click Allow once" UX promise is broken at the first click.

**Why it happens:**
- No code-signing certificate → no publisher identity → SmartScreen has no reputation to evaluate.
- Even *with* a standard OV (Organization Validation) cert, SmartScreen reputation is built from *cumulative installs across all users*, and our total user base is 12. We will never accumulate the install count needed to be "auto-trusted."
- An EV (Extended Validation) cert is the only cert type that gets *immediate* SmartScreen trust, and it costs ~$300-600/yr plus hardware-token logistics.
- Microsoft's bar for what triggers SmartScreen has tightened over time; even some signed binaries from established vendors trip it on initial release.

**How to avoid:**
**Layered approach, in order of preference:**
1. **EV code-signing certificate** — solves SmartScreen on first download. Best UX. Verify cost at purchase time; expect $300-600/yr plus a hardware token (HSM/USB) you have to keep accessible to whatever signs releases. Worth it if budget allows because it converts a scary blue screen into nothing.
2. **OV cert + SmartScreen reputation submission** — cheaper (~$100-200/yr), but you'll see SmartScreen warnings until reputation accrues, which for a 12-user product is approximately *never*. Useful primarily because OV certs prevent the worse "unsigned" warning in some configurations and identify the publisher in the consent dialog.
3. **No cert** — only acceptable if (a) the installer ships with a clear, friendly "When Windows asks, click 'More info' then 'Run anyway' — this is normal for small-team apps" walkthrough, and (b) you accept that one or two guildies will need real-time hand-holding through the warning. Pre-record a 30-second screen capture showing the click path, link it from the download page.

**Don't:**
- Ship unsigned and not warn guildies — guaranteed setup failures.
- Ship signed with a brand-new cert and assume reputation will materialize — it won't.
- Use self-signed certs — *worse* than unsigned (SmartScreen treats them as outright malicious).

**Warning signs:**
- Test installs on clean Windows 11 VMs all hit the SmartScreen wall (this is the *expected* outcome for unsigned, so if you don't see it, something's wrong with the test).
- Guildies report "I clicked Don't Run, was that right?" — they should never see this dialog if signing works.
- Guildies who downloaded via Edge see the warning more aggressively than Chrome guildies (Edge is more conservative).

**Phase to address:**
**Phase 1 (installer/distribution)** — this is the single highest-risk install-time UX issue. Decide on the cert path (EV / OV / unsigned-with-walkthrough) before building the release pipeline.

---

### Pitfall 3: Concurrent writes from 12 watchers cause lost-update bugs

**What goes wrong:**
Twelve watchers, all logged in as different Google users, all writing to the same shared workbook. The naive pattern — "watcher reads the inventory tab, computes the new state, writes back the whole tab via `spreadsheets.values.update`" — will *eventually* drop data when two guildies' watchers fire within the same second: watcher A reads at T=0, watcher B reads at T=100ms (sees the same state), A writes its update at T=500ms, B writes its update at T=550ms (overwriting A's changes). A's character data silently disappears from the sheet. This is a classic read-modify-write race condition. At 12 watchers writing intermittently, collisions are rare-but-real and will accumulate as a slow trust drain.

**Why it happens:**
- Sheets API doesn't enforce optimistic concurrency on `values.update` — there's no per-cell version token in the basic write API.
- Apps Script Lock Service and Sheets API write requests live in *different* worlds — a Lock acquired in Apps Script does NOT block direct Sheets API writes from the watcher.
- Developers prototyping with one user never see the bug; it only emerges at multi-user scale.

**How to avoid:**
**Architectural pattern: each watcher writes only to its own non-overlapping range.**
- Reserve one tab (or one row range within a shared tab) per character or per guildie. The watcher writes only to "its" range. No two watchers ever touch the same cells.
- Use the Sheets API `batchUpdate` with explicit `range` per character (e.g., `Inventory_Bobalt!A1:E2000`) — clobbering your own tab is fine; clobbering somebody else's is the bug.
- For *aggregate* views ("guild-wide search index", "all characters consolidated"), do **not** have watchers write to them directly. Compute aggregates inside Apps Script (which can use Lock Service), triggered on a schedule (every 5-10 min), reading from the per-character ranges.
- For any spot where a watcher *must* mutate shared state (rare — maybe a "last-seen heartbeat" column), call an Apps Script Web App endpoint that wraps the write in `LockService.getDocumentLock().tryLock(30000)` + `try/finally`. Apps Script Web App enforces the serialization the raw Sheets API doesn't.

**Sheets API write quotas** (verified March 2026): **60 write requests per minute per user per project**. With per-character ranges, a guildie running `/outputfile` on 5 alts in quick succession produces 5 writes — well under the limit. The shared limit (300 writes/min/project) is not at risk for 12 users.

**Warning signs:**
- "My Bobalt's inventory disappeared" reports, especially right after another guildie ran `/outputfile`.
- Sheet revision history showing two near-simultaneous edits from different guildies on the same range.
- Apps Script logs showing `tryLock` returning `false` (locks contending). At our scale this should be rare; if frequent, the architecture is wrong.

**Phase to address:**
**Phase 2 (sheet write architecture)** — establish the per-character-range convention before *any* watcher write code is shipped, because retrofitting from a shared-write model to per-range is painful (existing data must be migrated).

---

### Pitfall 4: Stale-character data with no freshness signal — silent rot, then trust collapse

**What goes wrong:**
A guildie quits playing their Necro for 3 months but doesn't uninstall SquireBot. The character's inventory rows last updated in January are still in the sheet, looking identical to inventory updated yesterday. An officer plans loot allocation around "Bobalt has the Robe of Living Fungus, he doesn't need this drop" — except Bobalt sold that robe two patches ago and nobody knows. Worse: guildie multiboxes 4 alts, plays Alt #1 weekly, but Alts #2-4 haven't been logged in since last expansion. All four alts appear equally "current" in the sheet. The first time loot allocation goes wrong because of stale data, *every* piece of data in the sheet becomes suspect.

**Why it happens:**
- File-watcher pattern updates only when the user actively runs `/outputfile`. There's no negative-existence signal.
- Sheets default rendering treats all rows equally — no inherent "last modified" surfacing.
- Players forget characters exist between expansions.

**How to avoid:**
- **Last-updated timestamp per character, surfaced prominently.** Every character tab has a header cell: `Last sync: 2026-04-12 (18 days ago)`. Use Apps Script conditional formatting to color this orange after 14 days, red after 30.
- **Watcher writes its own heartbeat** — even when the inventory file hasn't changed, the watcher pings a "last-seen" timestamp once per day (cheap one-cell write). Distinguishes "watcher running, data unchanged" from "watcher dead, data ancient."
- **Two timestamps per character:** `inventory file mtime` (when did the guildie last `/outputfile`) AND `last sync attempt` (when did the watcher last try). Both visible. The combination tells the truth: file mtime old + sync recent = guildie isn't outputting; file mtime old + sync old = watcher is dead.
- **Cross-character search results show staleness inline** — when a guildie searches "Lustrous Russet Coat" and gets a hit on Bobalt, the result row shows "Bobalt (last updated 47 days ago)" so the searcher knows whether to trust it.
- **Auto-archive after a threshold** — characters with `inventory file mtime` older than, say, 90 days move to an "Archived Characters" tab. Easy to restore by re-running `/outputfile`.

**Warning signs:**
- Officer reports "I tried to allocate based on the sheet and was wrong."
- Guildies asking "is X really still on Bobalt?"
- Guildies discovering their *own* old alts in the sheet and being surprised.

**Phase to address:**
**Phase 2 (sheet schema)** — bake last-updated timestamps into the schema from the start. **Phase 3 (UI/views)** for the conditional formatting and search-result staleness display. **Cheap to add at start, expensive to retrofit** because every existing view query needs to learn the timestamp column.

---

### Pitfall 5: `drive.file` scope confusion → Picker UX mistake breaks first-run

**What goes wrong:**
`drive.file` scope grants access *only* to files the user explicitly creates with the app or selects via a Google Picker. A common misimplementation: developer assumes "drive.file means all files user opens" or skips the Picker entirely and tries to access the workbook by hard-coded ID. Result: `403 insufficient permissions` on the first write, even though OAuth succeeded. Worse: the *first* guildie to set up creates a Picker flow that works for them, but they unknowingly need to share the workbook with each *new* guildie's email AND each new guildie has to use the Picker to acknowledge that specific workbook. Skipping the Picker results in `drive.file` not having access to that file ID for that user. The error message looks like an OAuth bug; it's actually a scope-semantics bug.

**Why it happens:**
- `drive.file` is documented but rarely used compared to `spreadsheets` — most OAuth tutorials assume the broader scope.
- The Picker requirement is non-obvious; "the user OAuth'd, why can't we read the file?" is a natural-feeling but wrong assumption.
- The Picker API requires its own setup (additional API key, OAuth client config flag, JS library load) which is a lot of plumbing for what looks like a one-line check.

**How to avoid:**
- **First-run flow MUST include the Picker step:**
  1. Sheet owner shares the workbook with the guildie's Gmail (manual step, one-time per guildie).
  2. Guildie runs SquireBot. First-run page opens browser to Google OAuth.
  3. After OAuth, watcher redirects to a tiny embedded HTML page hosting the Google Picker, configured to filter to spreadsheets the guildie has access to.
  4. Guildie clicks the workbook in the Picker → watcher receives the file ID + grant.
  5. Watcher stores `(workbook_file_id, guildie_email)` and writes via Sheets API.
- **Test the unhappy paths:** what if the guildie doesn't have access yet? Picker shows nothing useful, watcher hangs. Detect this — show a clear "Ask your guild lead to share the SquireBot workbook with you, then click Retry."
- **Document the share-list management** — sheet owner needs a process for adding new guildie emails. This is operational tax (see Pitfall 16).
- **Don't try to be clever and bypass the Picker** by using `drive.metadata` or hard-coded file IDs. `drive.file` is intentionally restrictive. The Picker IS the consent.

**Warning signs:**
- Setup logs showing OAuth success followed immediately by 403 on first Sheets API call.
- Guildie reports "I clicked Allow on Google but it doesn't work."
- A guildie's email isn't on the workbook share list (check via Sheet → Share dialog).

**Phase to address:**
**Phase 1 (OAuth + setup flow)** — designed in from the start. Picker is part of the OAuth ceremony, not an afterthought.

---

### Pitfall 6: Per-machine installer triggering UAC — silent setup-ceiling violation

**What goes wrong:**
NSIS or any installer that defaults to per-machine install (`Program Files\SquireBot`) requires admin elevation. Windows shows the UAC prompt: "Do you want to allow this app to make changes to your device?" with publisher info. For an unsigned app, publisher reads "Unknown" and the dialog is yellow/orange (warning color). Some guildies don't have admin on their PC (work laptop, shared family PC, school machine). Some guildies have admin but find the UAC + publisher-unknown combo so alarming they bail. The install ceiling spec says "one Allow click for Windows, one for Google" — a UAC prompt + a SmartScreen prompt is *two* Windows clicks before they even reach Google.

**Why it happens:**
- Installer generators default to per-machine because that's the "professional" pattern.
- Per-user installs are technically slightly newer and feel less canonical to developers.
- "Just need admin one time" feels acceptable to the developer; for a non-technical user it's a dealbreaker.

**How to avoid:**
- **Per-user install only.** NSIS: `RequestExecutionLevel user`, install to `$LOCALAPPDATA\Programs\SquireBot`. No UAC prompt at all.
- Autostart via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` (per-user registry, no admin needed) — NOT Task Scheduler, NOT a Windows Service.
- **Test the no-admin path explicitly** — install on a Windows 11 VM as a Standard User account. If anything trips UAC, fix it before shipping.

**Warning signs:**
- A test install on Standard User account asks for admin password — bug.
- Installer fails silently on a guildie's locked-down work laptop — same bug.

**Phase to address:**
**Phase 1 (installer)** — locked decision per STACK.md, but easy to regress on if someone "improves" the installer to be per-machine for a single feature like Windows Service autostart.

---

### Pitfall 7: Refresh-token rotation surprises — "I'm signed out and I don't know why"

**What goes wrong:**
Beyond the 7-day Testing-mode trap (Pitfall 1), refresh tokens have several other failure modes that look identical to the user — silent watcher death — but require different fixes:
1. **Six-month inactivity expiration** — refresh tokens not used for 6 months are revoked. Affects guildies who took a break from EQ.
2. **User revoked access from their Google account** — guildie clicked "Remove access" in security.google.com to clean up, didn't realize SquireBot was the cool guild thing they actually wanted.
3. **Password change with revoke-tokens-on-change setting** — Google Workspace admins can configure this; consumer Gmail does it on suspicious-activity password resets.
4. **Hit refresh-token issue limit** — Google allows up to 100 outstanding refresh tokens per (client, user) pair; the 101st invalidates the oldest. Unlikely at our scale but possible if a guildie reinstalls SquireBot many times.
5. **Hit refresh-token issued-token limit per app** — Google has a system-wide cap (~50 per OAuth client per user per scope set); same as above.

**Why it happens:**
The watcher logs `invalid_grant` and exits or sleeps. No notification surfaces to the guildie. They check the sheet a week later, see their data hasn't updated, try `/outputfile` again, sheet still doesn't update. Frustrated, they ask in Discord; debugging takes someone with access to the watcher logs.

**How to avoid:**
- **Detect `invalid_grant` and surface it to the guildie immediately** via the systray icon: change icon to red, set tooltip to "SquireBot needs you to sign in to Google again — click here." Click reopens the browser OAuth flow.
- **Apps Script consumer-side detection** — when the workbook hasn't received a write from a guildie's account in N days, surface a "Last seen" cell on a "Status" tab visible to all guildies. Officers can nudge the guildie.
- **Don't auto-retry forever** — exponential backoff with a cap; after 24h of failed refresh, stop retrying and rely on the systray prompt.
- **Document the failure path** — README has a "If the watcher icon turns red" section.

**Warning signs:**
- `invalid_grant` in `squirebot.log`.
- Specific guildie's Last Seen column not advancing.
- Guildie says "I think SquireBot stopped working."

**Phase to address:**
**Phase 2 (watcher robustness/error handling)** — the OAuth refresh path is built in Phase 1, but the *error UX* (systray icon state, prompt-to-reauth) needs explicit work in Phase 2.

---

### Pitfall 7a: `drive.file` write-access propagates ~50 minutes after Reauthorize — silent post-recovery failure

**What goes wrong:**
After Reauthorize succeeds (token refreshed, picker re-registers the workbook), the watcher correctly swaps to the new TokenSource and the next file event triggers a `batchUpdate`. That `batchUpdate` returns 401 — and continues to return 401 for **~50 minutes**. Read calls (`Spreadsheets.Get`) succeed immediately under the new grant; only writes are blocked. If the watcher treats post-Reauthorize 401s as terminal (i.e., calls `suspendForAuth`), the tray turns red, the user clicks Reauthorize again, the picker re-registers, the next write 401s again, and the loop never resolves — the workbook never reaches a state where writes succeed. Empirical window observed during the Day-4 soak (2026-05-07) was 51 minutes (50 probe attempts at 60-second intervals) on a clean throwaway account.

**Why it happens:**
- `drive.file` scope is per-(file, grant). When the picker re-registers the workbook under a new OAuth grant, the registration must propagate to every Sheets API backend before writes will succeed against any of them.
- Read access propagates via a fast path and is effectively immediate.
- Write access propagates via a slower path. Successive `batchUpdate` calls hit different backends that haven't yet received the registration.
- Google's `drive.file` documentation does not mention this propagation delay or the read/write asymmetry. The natural assumption — that Reauthorize → next-write-succeeds is synchronous — is wrong.

**How to avoid:**
- **Don't treat post-Reauthorize 401s as terminal.** Set a `globalPostReauthPending` flag when the picker phase completes; while it's true, route 401s to a propagation-wait state instead of `suspendForAuth`.
- **Probe write access in a background goroutine, never holding the `batchUpdate` mutex.** A naive probe loop inside `onRefresh` that holds `batchMu` for tens of minutes blocks heartbeat and every other API call (this was tried in commit `c9aef96` and was wrong on two counts: it held the mutex, and 25 minutes wasn't long enough). Spawn a separate goroutine that calls `PingWriteNoLock` (an empty `batchUpdate`, no mutex acquisition) every 60s.
- **Cap the wait at 90 minutes.** Observed worst case is 51 min; 90 gives ~40 min headroom. If real-world propagation ever exceeds 90 min, the timeout surfaces a Reauthorize prompt as a last resort — but do not lower the cap without fresh evidence.
- **Surface the wait state in the tray UI.** Keep the tray green (Reauthorize *did* succeed), set status to "Reauthorized: waiting for Google propagation…", and hide the Reauthorize menu item so the user doesn't loop. On probe success, clear the suspension and resume on the next file event.
- **Use an empty `batchUpdate` payload for the probe.** No requests array — it's the cheapest write call that exercises the same authorization path.

**Warning signs:**
- `permanent auth failure — suspending writes` log line within seconds of `Reauthorize complete`, with `globalPostReauthPending=true`.
- User repeatedly clicks Reauthorize and watcher never reaches a successful upload.
- `post-reauth probe: drive.file write still propagating` repeating with increasing `attempt` counter (expected for ~50 min after picker; only a problem if it reaches the 90-attempt timeout).
- Tray flapping green → red → green every minute or two as Reauthorize completes and the next write immediately fails.

**Phase to address:**
**Phase 2 (watcher robustness/error handling)** — discovered during the Day-4 `invalid_grant` soak (2026-05-07). Final fix shipped as `runPostReauthProbe` in commit `304b8bb`. Two intermediate designs were tried and superseded: a synchronous probe holding `batchMu` (commit `c9aef96`, blocked everything for 25 min) and an in-`onRefresh` `PingNoLock` (commits `4f20fc2`/`7072955`, didn't address propagation). Full chain: `13f4dac → 2d7128d → 4f20fc2 → 9ba1759 → 7072955 → 32d19d4 → c9aef96 → 304b8bb`. Soak evidence: `docs/soak-reports/2026-05-07-day4-auth05-sc1.md` § Day 4. Full session transcript: `docs/soak-transcripts/2026-05-07-day4-auth05-sc1.md`.

---

### Pitfall 8: PigParse rate-limiting / annoying the operator

**What goes wrong:**
PigParse is a free community service operated by a volunteer (Vyemm/PigParse devs). They re-aggregate prices server-side every 10 minutes. If SquireBot polls them aggressively from each of 12 watchers (12 × N requests/day), or polls per-item-detail when a single bulk endpoint exists, we look like an abusive client. Three failure modes: (1) operator silently rate-limits or blocks our User-Agent, sheet stops getting price updates; (2) operator publicly complains about the abusive client; (3) PigParse goes down because we're a contributor to load — community fallout.

**Why it happens:**
- "We have 12 users, who cares" thinking ignores that PigParse serves the entire P99 community and may have low total throughput headroom.
- Putting the polling in the watcher (per-machine) instead of Apps Script (server-side) multiplies request count by 12.
- Per-item-detail calls when `getall/{server}` returns everything in one request.

**How to avoid:**
- **Polling lives ONLY in Apps Script**, never in the watcher. One request per day from one place, regardless of how many guildies are running.
- **Use `/api/item/getall/1`** (one bulk request) instead of per-item lookups for the daily refresh. Cache result in a hidden sheet tab keyed by item ID; lookups are sheet-side.
- **Cache aggressively in `CacheService`** — 6-hour cache absorbs trigger-firing weirdness.
- **Conditional requests when supported** — send `If-Modified-Since` if PigParse exposes it (verify via Swagger).
- **Identify ourselves** — `User-Agent: SquireBot/1.0 (+https://github.com/<owner>/squirebot; contact: jbowen@mncivic.com)`. Operator can find us if there's a problem.
- **Reach out to the operator before going live** — PROJECT.md already calls this a "good citizen" expectation. Email/forum DM saying "We're a 12-user guild tool that will hit `/api/item/getall/1` once daily. Is that load OK?" Plus offers to coordinate if cadence needs to change.
- **Backoff on 429/503** — exponential, honor `Retry-After` header.

**Warning signs:**
- HTTP 429 or 503 responses in Apps Script logs.
- PigParse data freezes for >24h.
- Forum post or email from PigParse operator.

**Phase to address:**
**Phase 3 (external-data integration)** — establish the polite-fetch pattern as the *only* way to call PigParse, before any "let me just add one more endpoint" expansion.

---

### Pitfall 9: MediaWiki etiquette violations — getting blocked from wiki.project1999.com

**What goes wrong:**
The P1999 wiki is community-run and runs on MediaWiki. MediaWiki defaults to throttling/blocking clients that don't follow API etiquette. Specifically dangerous patterns: (1) parallel page fetches (multiple simultaneous requests for `action=parse`), (2) missing or generic `User-Agent` (e.g., just `Mozilla/5.0`), (3) tight-loop fetching of all per-item pages without backoff. A wiki admin who notices unusual traffic from one IP/UA can block at the MediaWiki level (config-based) or at the web-server level (IP block). If the source IP is shared (Apps Script runs on Google IPs that are shared with other Apps Script users), an over-aggressive scrape can get *all of Apps Script* blocked from the wiki — affecting other community tools.

**Why it happens:**
- Default `UrlFetchApp` calls don't set a custom User-Agent.
- "We have 1000 items to scrape, let's just loop through them" naive loop.
- Apps Script runs in Google's IP space, which is large and *not* dedicated to us — collateral damage radius is wide.

**How to avoid:**
- **Custom User-Agent with contact info** on every wiki request. MediaWiki etiquette explicitly requires a UA that identifies you and includes a contact channel.
- **Serial fetches with ~1s delay** between requests. `Utilities.sleep(1000)` is acceptable in Apps Script trigger context (eats into the 6-min budget but keeps us friendly).
- **Use the API, not HTML scraping** — `api.php?action=parse` is faster, smaller, and signals to admins that we're a well-behaved client.
- **ETag / If-Modified-Since**: store last response's ETag in `PropertiesService` per URL, send `If-None-Match`, skip processing on 304.
- **Weekly cadence is plenty** for wiki data — it doesn't change minute-to-minute. Don't refresh more often than necessary.
- **Resume-able multi-page scrape** — if the 6-min execution cap hits mid-scrape, store cursor in `PropertiesService` and let the next trigger pick up where we left off. Don't restart from scratch.
- **Reach out** — same as PigParse, courtesy email or forum post identifying ourselves to the wiki admins. Cheap insurance.

**Warning signs:**
- HTTP 403 or 429 from `wiki.project1999.com`.
- Apps Script logs showing scrape jobs hitting the 6-min cap (suggests we're not backing off enough or fetching too much per run).
- Wiki forum post about traffic anomalies.

**Phase to address:**
**Phase 3 (wiki scraper)** — built into the `politeFetch` helper before the first scrape runs.

---

### Pitfall 10: Apps Script 6-minute execution limit hit during wiki/inventory bulk processing

**What goes wrong:**
Apps Script consumer accounts have a hard 6-minute (360s) wall-clock limit per execution. Wiki scrape across 14 classes × ~200 spell pages each = 2800 page fetches at 1s apart = 47 min total. Single trigger run gets through ~360 pages then dies with `Exceeded maximum execution time`. If the code restarts the scrape from page 0 each invocation, the 7th day (next weekly trigger) starts over and never finishes either. Tab-of-spell-progression-checklists is permanently empty.

**Why it happens:**
- Developers used to Node.js or Go assume "long-running script just runs longer." Apps Script doesn't.
- The error is a hard kill — no `defer` or `finally` runs reliably after the timeout.

**How to avoid:**
- **Re-entrant trigger pattern**: store cursor (e.g., `lastClassProcessed`, `lastPageWithinClass`) in `PropertiesService`. At start of run, read cursor; at end of each item processed, update cursor and check elapsed time; if elapsed > 5 minutes, save cursor and exit cleanly (don't try to do "one more").
- **Schedule a follow-up trigger** with `ScriptApp.newTrigger().timeBased().after(60000).create()` — runs in 1 minute, picks up from the cursor. Continues until cursor reaches end of work.
- **Self-deleting one-shot triggers** — when scheduling the resume, also delete it at end of work to avoid trigger sprawl.
- **Budget conservatively** — exit at 5 minutes elapsed (not 5:55) to leave room for cleanup.
- **Aggregate writes** — don't write to the sheet after every page fetch (slow, quota-burning); buffer in memory, flush every 50 items or at end of run.

**Warning signs:**
- Apps Script execution log shows `Error: Exceeded maximum execution time`.
- Wiki data tab partially populated, gaps follow consistent class/letter pattern.
- Spell-progression checklist missing late-alphabet classes (Warrior, Wizard).

**Phase to address:**
**Phase 3 (wiki scraper)** — design re-entrancy in from the start, even if v1 wiki workload theoretically fits in one run. Reentrancy-as-architecture means future wiki expansion (more pages, deeper parse) doesn't break things.

---

### Pitfall 11: Workbook cell-count limits — Google Sheets caps at 10 million cells

**What goes wrong:**
Google Sheets workbooks max out at **10 million cells across all tabs** (raised from 5M in 2022, current as of 2026). Per-character inventory of a long-played P99 main with multiple alts can hit 1500+ rows × 5 columns = 7500 cells per character. 12 guildies × 4 alts each = 48 characters × 7500 = 360K cells just for inventory. Add spellbooks (200 spells × 12 columns × 48 chars = 115K), wiki cache (10K items × 20 columns = 200K), price history (10K items × 30 days × 5 columns = 1.5M), guild-wide consolidated search index (360K rows × 8 columns = 2.9M)... we approach 5M cells and that's before any growth. Hit the cap and the whole workbook becomes read-only for new content; existing formulas break with `#REF`.

**Why it happens:**
- Cell count compounds across tabs — a feature is "small" per-tab but the sum is large.
- "Hidden helper tabs" (price history, scrape staging, deduplication scratch) accumulate without anyone counting them.
- Formulas that ARRAYFORMULA across large ranges materialize as filled cells from the cap's perspective.

**How to avoid:**
- **Budget the cell count up front.** List every tab, estimate row × column footprint, sum. Target <2M cells in v1; alarm at 5M.
- **Compress price history** — store time-series as one column "history_json" with last 30 prices encoded as JSON, parsed at display time. 30 prices in JSON = 1 cell, vs. 30 cells.
- **Don't materialize aggregates that can be Apps Script computed on demand.** A cross-character search that runs as a sidebar JS function over the underlying tabs doesn't need a precomputed "all items" mega-tab.
- **Trim per-character data** — characters at level cap with full inventory, fine. A guildie's level-15 alt with one bag does NOT need pre-allocated rows for 1500 items.
- **Archive old characters** (link to Pitfall 4) — data leaves the live workbook for a separate archive workbook.
- **Monitor** — Apps Script can read `Spreadsheet.getNumSheets()`-style metrics; have a weekly trigger compute total cell usage and post it to a Status tab.

**Warning signs:**
- Cell count growth >100K/week.
- New row inserts producing `#ERROR`.
- Apps Script writes returning quota errors that turn out to be cell-cap.

**Phase to address:**
**Phase 2 (sheet schema)** — design with the cap in mind. **Phase 4+ (operational monitoring)** — add automated tracking once schema stabilizes.

---

### Pitfall 12: Cell-formula recalc storms when bulk-loading inventory rows

**What goes wrong:**
A guildie's `/outputfile inventory` produces 800 rows. Watcher writes them in one batch. The sheet has cross-character lookup formulas (`VLOOKUP`, `QUERY`, `INDEX/MATCH`) that depend on inventory. Each of those formulas re-evaluates on the 800-row insert, AND the wiki tooltip column (which itself has formulas), AND the price column. Total recalc fan-out: 800 × ~10 dependent formula chains = 8000+ cell recalcs in one transaction. Sheet hangs for 10-30 seconds for *every* guildie viewing it. Multi-guildie compound: 3 guildies fire `/outputfile` within a minute = 3 sequential recalc storms = the sheet feels unusable for 90 seconds.

**Why it happens:**
- "Just write the data, the sheet handles formulas" is reasonable in single-user spreadsheets and breaks down at scale.
- VLOOKUP cost is O(N) per lookup; ARRAYFORMULA-d VLOOKUP across 1000s of rows is O(N²)-ish.
- Volatile functions (`NOW()`, `RAND()`, `INDIRECT()`) recalc on every change, multiplying the storm.

**How to avoid:**
- **Computed columns live in Apps Script, not formulas** — the watcher writes inventory; an Apps Script `onChange` trigger (or a 5-min scheduled job) computes derived columns (wiki tooltip, current price, quest indicator) and writes them as static values. Sheet sees plain data, no formula recalc.
- **No `ARRAYFORMULA(VLOOKUP(...))`** spanning 1000s of rows. Convert to Apps Script-computed static values.
- **No volatile functions** in heavy-traffic ranges. Use scheduled `setValue` for "current time" instead of `NOW()`.
- **Batch writes**: one `batchUpdate` for the full 800 rows, not 800 single writes.
- **Use Sheets API `valueInputOption=RAW`** when writing — `USER_ENTERED` parses every cell for formulas/dates, slow.

**Warning signs:**
- Guildies report "the sheet froze for a long time."
- Apps Script logs show triggers running for >30s consistently.
- Slow scrolling/typing in the workbook.

**Phase to address:**
**Phase 2 (sheet schema)** — formula-vs-Apps-Script-computed decision is foundational; getting it wrong is a refactor. **Phase 4 (performance)** — measure before assuming we have a problem; some formulas may be fine.

---

### Pitfall 13: Schema fragility — "someone deleted the tab" / "someone renamed the tab"

**What goes wrong:**
Universal-visibility means every guildie has full edit access to the workbook. A well-meaning guildie reorganizes tabs, renames "Inventory_Bobalt" to "Bob's Stuff," or deletes "Wiki_Cache" thinking it's clutter. Watchers and Apps Script code that hard-coded those names fail. Worst case: a guildie accidentally drags a tab into another tab merging them, or deletes a tab outright. Sheet revision history can recover but only if someone notices fast enough.

**Why it happens:**
- Sheet tabs are user-editable like any other content.
- Code references tabs by name (typical Apps Script pattern: `SpreadsheetApp.getActiveSpreadsheet().getSheetByName('Wiki_Cache')`).
- Universal-visibility is by definition universal-edit-ability for our v1 (no granular permissions).

**How to avoid:**
- **Tab IDs, not tab names**, in code. `Spreadsheet.getSheetById(123456)` is stable across renames; `getSheetByName('foo')` is not. Apps Script supports both; choose ID.
- **Protected ranges** for system tabs (`Wiki_Cache`, `PigParse_Prices`, `Config`, character-data tabs): use `Range.protect()` with editor list = sheet owner only. Watchers write via Sheets API which bypasses range protection only if the OAuth'd user is also the protection owner — this needs care; the simpler model is to protect *display* of tabs (hide via developer metadata) and rely on social convention for the rest.
- **Hidden tabs for system data** — `sheet.hideSheet()`. Out of sight, less likely to be touched. Easily un-hideable for debugging.
- **Schema healthcheck on Apps Script trigger** — every hour, verify expected tabs exist, expected columns exist, expected ranges aren't empty when they should have content. Post anomalies to a Status tab. Alert (e.g., trigger failure email) if critical schema is missing.
- **Restore documentation** — README has "If you accidentally deleted X, here's how to restore from version history."
- **Conventional naming** — prefix system tabs with `_` (e.g., `_Wiki_Cache`) which signals "don't touch."

**Warning signs:**
- `getSheetByName(...)` returning `null` in logs.
- Watcher write returning "range not found."
- Sheet revision history showing tab rename/delete by a guildie.

**Phase to address:**
**Phase 2 (sheet schema)** — tab-ID convention from day one. **Phase 4 (operational monitoring)** — schema healthcheck after schema stabilizes.

---

### Pitfall 14: Auto-update gotchas — replacing a running .exe on Windows

**What goes wrong:**
Windows file-locking semantics: a running `.exe` can't be deleted or overwritten *by a normal application*. Naive auto-updater downloads `squirebot-new.exe`, tries to rename it to `squirebot.exe`, OS returns ERROR_SHARING_VIOLATION. Update silently fails. Or: updater tries during shutdown, partially overwrites the binary, leaves a corrupted executable. Guildie launches Windows next day, watcher fails to start, guildie gives up.

**Why it happens:**
- Linux/macOS allow overwriting running binaries (kernel keeps the old inode); Windows doesn't.
- `inconshreveable/go-update` (now `minio/selfupdate`) implements the workaround but only if used correctly — naive integration misses edge cases.

**How to avoid:**
- **Use `minio/selfupdate`'s recommended pattern**: write new binary to a sidecar path, atomically rename (`MoveFileEx` with `MOVEFILE_REPLACE_EXISTING | MOVEFILE_DELAY_UNTIL_REBOOT` flag, or the in-place pattern that the lib supports). Lib handles the Windows-specific ceremony.
- **Apply at startup**, not at runtime — watcher checks for `squirebot-pending.exe` on launch; if present, swaps it in before fully starting. Old binary is renamed `.old`, deleted on next launch. Eliminates the running-process problem.
- **Atomic write or no write** — partial updates corrupt; verify checksum of downloaded binary against `latest.json` manifest before swap.
- **Manifest-driven** — `latest.json` published with version, URL, SHA-256. Watcher compares its embedded version against manifest, only swaps if newer AND checksum matches.
- **Fallback recovery** — if startup-swap detects `squirebot.exe` corrupted (missing/zero-size), restore from `.old`. Logs the event; reports via systray on next launch.
- **Don't auto-update on every launch** — check once a day, not per launch. Don't surprise guildies with download spinners.
- **Test with deliberate failure injection** — kill the updater mid-download, verify recovery.

**Warning signs:**
- Watcher logs `ERROR_SHARING_VIOLATION` or `Access denied` on update.
- Guildies reporting watcher won't start after auto-update.
- Embedded version != manifest version persistently (stuck behind).

**Phase to address:**
**Phase 4 (auto-update pipeline)** — this is a Phase 4 concern; v1 launch can ship without auto-update if releases are infrequent. Adding it is non-trivial; budget integration testing time. STACK.md flags this as MEDIUM confidence on integration ease.

---

### Pitfall 15: Universal-visibility privacy regret — "everyone can see my bank toon's plat"

**What goes wrong:**
v1 chose universal visibility for simplicity. After 3 months, a guildie realizes: their bank toon has 800K plat, visible to everyone, including a guildie who later quits the guild and *retains read access to the workbook* until somebody manually un-shares. Or: a guildie's main has a Fungi Tunic visible to everyone, and they're being pestered for loans. Or: a guildie quits in anger, and their alt list (with all gear, all spells) is still visible to the active guild — feels invasive in retrospect.

**Why it happens:**
- "Universal visibility" is a pragmatic v1 choice but has long-tail surface area — the data is sensitive in aggregate even if each piece is innocuous.
- Sharing is granted per-Gmail; revoking requires the sheet owner to remember to do it.
- Universal also means *self*-visibility — guildies see their own data fine, but an alt they forgot about exists may surface uncomfortably.

**How to avoid (preemptive softening for v1):**
- **Per-character "hide from guild" flag** — single boolean per character on a Config tab. When set, that character's tab is hidden from rendered views (Apps Script filters it out of consolidated search, etc.). Watcher still updates it (so the guildie themselves can see it via the underlying tab). NOT a security boundary — anyone determined can navigate to the hidden tab — but a *politeness* boundary that handles 99% of "I don't want this in the loot-allocation view" needs.
- **Soft-delete on character removal** — a guildie can mark a character "Removed" and it disappears from views (data preserved for ~30 days then archived). Provides reversibility.
- **Soft-delete on guildie eviction** — when a guildie quits/leaves: (1) remove their email from the workbook share list (data stays, they lose access), (2) mark all their characters "Removed" via the soft-delete mechanism. Their data is preserved (for ~30 days) so officers can verify nothing's needed, then archived/deleted.
- **Documented eviction process** — Phase 4 deliverable; one-page README section covering the steps.
- **Don't expose plat amount to non-officers in v1.5+** — bank toon's manual plat field could become officer-visible only via tab protection. Honestly: v1 says don't bother, revisit if asked.

**Warning signs:**
- Guildies start avoiding `/outputfile` on certain alts.
- Loot/loan drama.
- A guildie's "alt I forgot about" being discovered by others.

**Phase to address:**
**Phase 2 (schema design)** — bake hide-flag and soft-delete fields in even if no UI yet; cheap to add, expensive to retrofit. **Phase 5+ (privacy refinements)** — if and when the universal-visibility decision is revisited.

---

## Moderate Pitfalls

### Pitfall 16: Sheet schema migrations across watcher binaries

**What goes wrong:**
v1.1 needs a new column on the inventory tab (e.g., "ItemType"). 12 watchers each at different binary versions; some on v1.0, some on v1.1. v1.0 watchers don't write the new column → sheet rows have gaps. v1.1 watchers expect the column → fail when reading their own old rows. Schema migrations across distributed clients is a small-scale version of the database-migration problem.

**Why it happens:**
- Watchers don't auto-update synchronously; rollout is gradual.
- Schema and code versioning aren't linked — easy to ship a code change that assumes a schema change happened first.

**How to avoid:**
- **One workbook (single database)** — already chosen, simplifies vs. per-guildie workbooks.
- **Backward-compatible writes** — new watcher version still writes columns the old version expected. Old version's writes still parse correctly under new version (extra columns OK, missing columns default).
- **Watcher reports its version** in a Status tab cell on every write. Apps Script can detect "guildie X is on v1.0, please update."
- **Apps Script handles forward-fill** — if old watcher writes, Apps Script trigger fills in new columns from cached data (e.g., wiki lookup for ItemType).
- **Force-update mechanism** — if a schema change is breaking, mark older binaries' write targets as read-only and force update via systray prompt.
- **Test the cross-version case** — staging workbook, mix of v1.0 + v1.1 watchers, observe.

**Warning signs:**
- Mixed-format rows (some have new column, some don't).
- Apps Script code branches on `if (row[X] === undefined)`.
- Older watchers logging unknown errors.

**Phase to address:**
**Phase 4 (operational maturity)** — first migration is the one that hurts.

---

### Pitfall 17: Latin-1 vs UTF-8 encoding in EQ-output files

**What goes wrong:**
EverQuest is a 1999 game. Its `/outputfile` writes text files in **Windows-1252** (a Latin-1 superset), not UTF-8. Some item names contain accented characters (`Manacle of Soulsenvy` is plain; but P99 has items with `é`, `ï`, `ñ` and special punctuation). A watcher that opens the file as UTF-8 sees mojibake on those characters: "Crystallized Pums'tar" becomes garbled. Joining against PigParse data (UTF-8) by name fails for affected items. Item ID join still works, so the bug surfaces as "wrong tooltip text," not "missing data" — easy to miss until a guildie mentions it.

**Why it happens:**
- Default `os.Open` + `bufio.Scanner` in Go reads bytes; treating them as UTF-8 silently corrupts.
- Test files are usually ASCII-clean, so the bug doesn't appear in dev.

**How to avoid:**
- **Decode explicitly as Windows-1252** when reading EQ output files. In Go: use `golang.org/x/text/encoding/charmap.Windows1252.NewDecoder()` over the file reader.
- **Verify on a real EQ install** with one of the affected items — Velious-era items, named-mob drops with apostrophes/accents, are good test cases.
- **Item ID join is canonical** — even with name mojibake, joining via item ID against wiki/PigParse keeps the data correct. Names are display-only.
- **Validate at parse time** — log a warning if any character outside printable ASCII appears in the name column when rendered as UTF-8 (that's the canary).

**Warning signs:**
- Item names with `?` or replacement chars in the sheet.
- Tooltips showing "wiki summary unavailable" for items that exist on the wiki.
- Discord reports "what is `Crystallized Pums??tar`?"

**Phase to address:**
**Phase 1 (file-format parsing)** — decode encoding correctly from the start; cheap to do, painful to retrofit because existing rows would need re-import.

---

### Pitfall 18: EQ folder discovery — multiple installs, Steam-vs-launcher, multibox setups

**What goes wrong:**
"Just look for the EQ folder" — except guildies have different setups:
- P99 official launcher (Titanium client) → `C:\P99\` or `C:\Program Files (x86)\EverQuest\`
- Manual Titanium install → user-chosen path
- Steam install (live EQ, NOT P99 — but a guildie might have both) → `C:\Program Files (x86)\Steam\steamapps\common\EverQuest\`
- Multibox setups with multiple EQ folder copies → `C:\P99-Box1\`, `C:\P99-Box2\` etc.
- Network drive / OneDrive-synced folder → `\\server\eq\` or `C:\Users\X\OneDrive\P99\`

Auto-discovery that hard-codes one path fails for half the guild. Asking "browse to your EQ folder" violates idiot-proof if the guildie doesn't know where it is.

**Why it happens:**
- Twenty-five years of EQ installs have accumulated patterns.
- P99 lacks a registry/installer that records the install path.

**How to avoid:**
- **Multi-strategy auto-discovery in priority order:**
  1. Check `%APPDATA%\SquireBot\config.json` for previously-configured path.
  2. Look for known P99 paths: `C:\P99\`, `C:\Program Files (x86)\Project1999\`, `C:\Program Files (x86)\EverQuest\`, `D:\P99\`, `D:\Program Files\Project1999\`.
  3. Check Windows registry uninstall keys for entries containing "EverQuest" or "Project1999."
  4. Heuristic file scan: walk `%USERPROFILE%`, `C:\`, `D:\` looking for a directory containing `eqgame.exe` AND `eqclient.ini` (those two together are EQ-specific).
  5. Fall back: prompt the guildie to drag-drop their EQ folder onto the systray icon, OR paste the path. Show validation immediately ("found `eqgame.exe` here").
- **Multi-folder support** — the watcher should handle a *list* of EQ folders for multiboxers. Config stores `eq_folders: [...]`, watcher watches all.
- **Document the multi-character-same-name case** — guildies running the same character name in two folders (rare but it happens) need a tiebreaker. Path-prefix the character name internally if so.
- **Detect non-EQ folders early** — verify `eqgame.exe` exists; reject if not, with a friendly error.

**Warning signs:**
- Watcher running with no inventory writes for a guildie (folder wrong).
- Folder path on a network drive or OneDrive — `fsnotify` may behave differently; test.
- Guildie has 3 alt folders, only one is being watched.

**Phase to address:**
**Phase 1 (first-run setup)** — auto-discovery is the second-most-important UX after OAuth. Get the heuristics right early.

---

### Pitfall 19: Stale data from a defunct watcher running a fork or old version

**What goes wrong:**
A guildie installs v1.0, never updates, and v1.0 has a parsing bug that misclassifies bagged items as banked. The bug is fixed in v1.2 but the guildie's data has been wrong for 4 months. Or: a guildie sets up SquireBot on two PCs (desktop + laptop), watches the same Google Drive-synced EQ folder, and both watchers fight over the same data, producing inconsistent writes.

**Why it happens:**
- No central enforcement of watcher version.
- Multiple watchers on shared storage = duplicate or conflicting writes.

**How to avoid:**
- **Watcher writes its version** to a Status tab on every write — visible to officers.
- **Apps Script alerts on stale versions** — a weekly check that posts "Guildie X on v1.0, current is v1.4."
- **Detect duplicate watchers** — if two writes for the same character arrive from two different machines (different installation IDs) within a short window, log it and surface a warning. Possibly add a `singleton lock` (e.g., named mutex via Win32) per character so only one watcher per character per machine wins.
- **Documented "one watcher per character" rule** — if a guildie has multiple PCs, run the watcher on the primary one only.

**Warning signs:**
- Status tab shows old version numbers.
- Inventory rows oscillating between two states (two watchers fighting).
- Same character's "last sync" timestamp jumping back in time.

**Phase to address:**
**Phase 4 (operational monitoring)** — version reporting is cheap; alerting and dedupe are nicer-to-have.

---

### Pitfall 20: OneDrive-synced AppData breaks DPAPI/Credential Manager assumptions

**What goes wrong:**
Some Windows configurations sync `%APPDATA%\Roaming` via OneDrive (Known Folder Move). Windows Credential Manager itself doesn't live there — credentials are stored in `%APPDATA%\Roaming\Microsoft\Credentials\` and protected by DPAPI MasterKeys in `...\Microsoft\Protect\<SID>\`. **DPAPI-encrypted blobs are bound to the user account's MasterKey**; they don't decrypt cross-machine even if the file is synced. If a guildie has KFM on AND uses two PCs, they get credential weirdness: the credential file appears on PC2 but DPAPI on PC2 can't decrypt it (different MasterKey). Microsoft KB documents related symptoms (Event 8196 "Master key decryption in memory failed", Event 8198 "DPAPI Unprotect failed").

Additionally: if a guildie's roaming profile is moved/restored from backup, DPAPI MasterKeys may be invalidated, wiping the stored OAuth refresh token. The watcher silently re-OAuths, which works (because Google still has the consent) but adds friction.

**Why it happens:**
- Windows 11 default-prompts users to enable OneDrive Known Folder Move.
- DPAPI is opaque to users; failures look like "the app forgot my credentials."
- We chose `wincred` because it's the Right Thing; Right Thing for one user, fragile across multi-PC setups.

**How to avoid:**
- **Accept some friction**: if credential decryption fails, fall back gracefully — log it, prompt the guildie to re-OAuth via systray. Don't crash.
- **Don't store anything in `%APPDATA%\Roaming` that we expect to survive cross-PC** — keep our config in `%LOCALAPPDATA%\SquireBot\` (NSIS install path already targets this for the binary, extend to config). Local-only by design.
- **Document the dual-PC case** — if a guildie uses SquireBot on two PCs, expect to OAuth on each one separately. Not a bug, an architectural reality of DPAPI.
- **Verify on a OneDrive-KFM-enabled test box** — STACK.md flags this as MEDIUM confidence; explicitly test.

**Warning signs:**
- "I had to re-OAuth after my Windows updated" reports.
- DPAPI failures in event log (Event ID 8196/8198) correlated with watcher failures.
- Guildies on machines that just had a profile rebuild/migration.

**Phase to address:**
**Phase 1 (token storage)** — pick storage paths correctly. **Phase 2 (error handling)** — fallback-to-reauth must work.

---

### Pitfall 21: Browser blocks the loopback redirect (AV / Edge security configurations)

**What goes wrong:**
The OAuth loopback flow opens `http://127.0.0.1:N` after the Google consent. Some configurations interfere:
- **Corporate AV / Edge Smart Network Defense** may flag any localhost redirect as suspicious and block it (rare but documented).
- **Edge / Chrome strict-mode HTTPS-only** — recent browser versions sometimes upgrade `http://127.0.0.1` to `https://`, which fails because we don't have a TLS cert for localhost. Browsers historically exempt 127.0.0.1 from this; verify against current browser versions.
- **Firewall (Windows Defender or third-party)** prompting to allow incoming connections to `squirebot.exe` on the chosen port. The guildie sees "SquireBot is trying to accept incoming network connections" — extra Allow click violating the "one click for Windows" promise.
- **Port already in use** — if N is fixed (e.g., 8080), some other app is on it, redirect lands on the wrong app.

**Why it happens:**
- Loopback flow assumes browser will speak HTTP to localhost — increasingly contested by browser security models.
- Windows Firewall by default prompts on first listener bind for any new app.

**How to avoid:**
- **Pick a random ephemeral port (49152-65535)** at runtime, don't fix the port. Reduces collision risk.
- **Localhost-listener Firewall hint** — NSIS installer adds a Firewall exception via `netsh advfirewall firewall add rule program=...` at install time, preventing the runtime prompt. Requires admin? — only for the rule add. Test if per-user rules work without UAC. If not, accept the one-time runtime prompt.
- **Loopback IP literal `127.0.0.1` not `localhost`** — Google's spec calls for the IP form; some browsers handle it more reliably.
- **Detect failure and offer manual code-paste fallback** — if the redirect doesn't land within 60s, surface a "If your browser didn't redirect, paste the URL here" dialog. Salvages stuck setups. (Note: pure OOB code-copy flow is deprecated, but a guildie pasting the redirect URL is fine because we can extract `?code=` from it.)
- **Verify on Edge with default SmartScreen + SmartScreen Network Protection on** — common P99-guildie config.

**Warning signs:**
- OAuth flow stalls after Google consent — browser shows blank tab or "couldn't reach 127.0.0.1."
- Firewall popup during first OAuth.
- Guildies report "I clicked Allow on Google but SquireBot doesn't know."

**Phase to address:**
**Phase 1 (OAuth flow)** — random port + manual-paste fallback are cheap insurance. **Phase 4 (Firewall integration)** — only if the runtime prompt becomes a complaint.

---

### Pitfall 22: Wiki page structure changes silently breaking the parser

**What goes wrong:**
P99 wiki pages (especially Velious tier pages and class spell lists) are community-edited. A wiki editor reorganizes `Players:Velious_Pre-Raid_Gear` from a flat slot table into a nested structure. SquireBot's parser, which expected `<table class="wikitable"><tr>...` format, returns empty results. Gear progression checklist quietly stops finding any data — every character looks like they have everything, or nothing. Compare to a hard error: the silent case is worse because nobody notices for weeks.

**Why it happens:**
- Wiki pages aren't APIs; they have no contract.
- Parsers without expectation-checking just return empty.
- "Empty" is indistinguishable from "no data at this tier" in our merge logic.

**How to avoid:**
- **Use MediaWiki API `action=parse&prop=wikitext`** — wikitext is more stable than rendered HTML and uses semantic templates (e.g., `{{ItemSlot|Head|...}}`). Parse templates not HTML.
- **Schema assertions**: "this scrape should yield ≥ N items / ≥ M classes." If yield is zero or far below baseline, raise an alert (Status-tab error) instead of writing empty data over good data.
- **Last-known-good preservation** — if a scrape returns < 50% of last week's row count, *don't overwrite* — keep last week's data, raise alert. Trust the historical baseline over a sus current run.
- **Monitor multiple anchor pages** — if all 14 class pages suddenly return empty, the fault is ours (parser broken). If only Necromancer returns empty, the wiki page changed.
- **Health check email** — once a week, email yourself the scrape stats: rows scraped per source, deltas vs. last week, anomalies.

**Warning signs:**
- Sudden drop in scrape row count vs. last week.
- Gear progression checklist showing all items missing for everyone.
- Apps Script logs showing successful HTTP 200 responses but zero parsed items.

**Phase to address:**
**Phase 3 (wiki scraper)** — assertions + last-known-good are core to the scraper, not nice-to-haves.

---

### Pitfall 23: Character file ownership collisions — two guildies have an alt named "Bob"

**What goes wrong:**
P99 character names are unique *per server*. So two guildies cannot both have a "Bob" on Blue. **However**: a guildie can name characters in different ways that collide *in our schema*. Example: Guildie A's main is "Bobalt" and they also have an alt "Bob_inventory.txt" parser key. If we naively key by character name only, we have a problem when:
- A guildie has a deleted character named "Bob" and creates a new one named "Bob"  → old + new file in the EQ folder, watcher uploads both, sheet has two "Bob" rows from the same guildie.
- A guildie with multiple PCs has stale `Bob-Inventory.txt` files from old patches still sitting in old folders.
- (Cross-guildie collision is impossible on Blue by P99's name-uniqueness, but if the guild expands to Green or migrates servers, this assumption breaks.)

**Why it happens:**
- File-name as key without owner namespacing.
- EQ keeps old character files around when a character is deleted.

**How to avoid:**
- **Composite key `<guildie_email>:<character_name>`** in all sheet rows. Per-guildie watchers can only write rows with their own email prefix.
- **File-mtime-based dedupe** — if multiple `Bob-Inventory.txt` files exist (e.g., in stale folders that fell into auto-discovery), prefer the one with the most recent mtime; warn on the rest.
- **Character delete/rename detection** — if a previously-seen file disappears for >30 days, mark the character as "Removed." If a character with a fresh file appears with the same name as a previously-removed one, treat it as new (different character) and use a numeric suffix in the sheet (`Bob (2)`).
- **Server field per character** — even if v1 is Blue-only, schema includes server column. Future-proofs against multi-server expansion (which is currently Out of Scope but may not always be).

**Warning signs:**
- Two rows for the same character name on the same guildie's character list.
- Inventory data oscillating between two states (two source files, both being read).
- Watcher writing to a character row that doesn't seem to match the active toon.

**Phase to address:**
**Phase 2 (sheet schema)** — composite key from start, server column reserved.

---

### Pitfall 24: P99 client patches changing the `/outputfile` format

**What goes wrong:**
The 5-column tab-separated format (`Location | Name | ID | Count | Slots`) has been stable for years on P99 (Titanium client). But P99 has shipped client patches that have changed: the location string format (e.g., `"Bag1-Slot1"` vs `"General-Slot1"`), addition of optional columns, locale changes. A future patch could add a 6th column or change separator. Watcher hard-coded to 5 columns parses incorrectly.

**Why it happens:**
- Tab-separated text without a versioned header.
- P99 patches are infrequent but real.

**How to avoid:**
- **Defensive parsing**: tolerate extra columns (ignore them, log a notice). Tolerate fewer columns (use defaults, log a warning).
- **Sentinel for first row** — first inventory row is reliably "Location\tName\tID..." or actual data; detect by content not position.
- **Subscribe to P99 patch notes** — `forums.project1999.com` or wiki `Recent_Patches` page. When a patch ships, smoke-test the parser against fresh `/outputfile` output.
- **Community signal** — other P99 inventory tools (P99 Inventory Parser, EQHTML, EQ1999-Bank) will break first if a format change happens. Watch their issue trackers.
- **Test corpus**: keep a dozen real `-Inventory.txt` and `-Spellbook.txt` files in the test suite. After every patch, add fresh samples.

**Warning signs:**
- Watcher logs "expected 5 columns, got 6" or similar.
- Inventory rows missing fields.
- New P99 patch announcement coincides with watcher write failures.

**Phase to address:**
**Phase 1 (parser)** — defensive parsing from the start. **Phase 4 (operational monitoring)** — log-based detection of format anomalies.

---

## Minor Pitfalls

### Pitfall 25: Logs without rotation, filling user disk

**What goes wrong:**
Watcher runs unattended for months. Each `/outputfile` event logs ~50 lines. Over a year, log file is 500MB+. On a guildie's older laptop with limited SSD, this matters. Worst case, watcher crashes from disk-full.

**How to avoid:**
- `lumberjack.v2` log rotation (already in STACK.md), 5 MB max, keep last 3.
- Verify the rotation actually happens (mock disk writes in test).

**Phase to address:**
**Phase 1 (logging setup)** — cheap, doesn't cost anything.

---

### Pitfall 26: `fsnotify` event flooding from EQ writing inventory in chunks

**What goes wrong:**
EQ writes `-Inventory.txt` non-atomically — open-truncate-write multiple lines-close. `fsnotify` may fire 5 events per single `/outputfile`. Watcher tries to process 5 times, gets partial files mid-write.

**How to avoid:**
- 500ms debounce per file-path. Only process after events go quiet.
- Re-read the file as a complete unit (don't trust event content).
- STACK.md already specifies this pattern.

**Phase to address:**
**Phase 1 (file watcher)** — built-in pattern.

---

### Pitfall 27: Apps Script daily trigger total runtime cap (90 min/day consumer)

**What goes wrong:**
Apps Script consumer accounts have a 90 min/day total-trigger-runtime cap. If our triggers run long (wiki scrape can take ~50 min in the worst case), and we have other triggers (PigParse refresh, schema healthcheck, aggregate computation), we approach the limit.

**How to avoid:**
- **Budget**: weekly wiki ~50 min, daily PigParse ~1 min, hourly aggregates ~5 min × 24 = 120 min/day worst case. Wait — that exceeds 90 min on aggregate-heavy days.
- **Actual budget**: cut hourly aggregates to every 4 hours = 6 × 5 = 30 min. Still tight.
- **Move expensive work to weekly** when possible. Most aggregates (cross-character search index) don't need to be hourly.
- **Monitor**: total runtime is visible in Apps Script dashboard; check weekly during early operations.
- **Workspace upgrade fallback** — if guild eventually shares a Workspace account (paid), cap rises.

**Phase to address:**
**Phase 3 (trigger schedule)** — design within the budget.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip code-signing cert | $0 spend, faster v1 ship | Every new guildie hits SmartScreen wall; ongoing setup support burden | OK for first month only — sign within 30 days. Document the "Run anyway" walkthrough in the README. |
| Hard-code workbook ID instead of Picker flow | Skips Picker setup | Breaks `drive.file` semantics; setup fails for new guildies | **Never.** This is a foundational architectural error. |
| Use `spreadsheets` scope instead of `drive.file` | Easier OAuth setup | Triggers Google verification process for any meaningful user count; consent screen is more alarming | **Never** for v1. Possible only if we ever exceed 100 users AND `drive.file` doesn't work for some new feature. |
| Watcher polls directory via `time.Tick` instead of `fsnotify` | Fewer dependencies | Lag, missed events, CPU waste | Acceptable for a dev-only fallback if `fsnotify` fails to initialize. Production must use `fsnotify`. |
| Skip last-updated timestamp on character data | Saves one column | Stale-data trust collapse (Pitfall 4) | **Never.** Foundational schema element. |
| Skip per-character namespacing (use char name only as key) | Simpler queries | Breaks on rename, deletion, multi-server | OK only if v1 explicitly forbids alts, which it doesn't. Use composite key. |
| Use `getSheetByName` instead of `getSheetById` | Slightly more readable code | Tab-rename breaks everything | OK only for tabs that are never named programmatically. System tabs use IDs. |
| Skip Apps Script lock service for low-frequency aggregate writes | Simpler code | Race conditions when traffic spikes | OK only for read-only aggregates. Any mutation through shared Apps Script needs locks. |
| Use `valueInputOption=USER_ENTERED` for batch writes | Pleasant date/formula parsing | 10x slower writes; recalc storms | Acceptable for one-off setup writes; never for hot-path watcher writes (use `RAW`). |
| Run wiki scrape from the watcher, not Apps Script | "Easy" — already have HTTP client there | 12x request multiplier vs. wiki; political risk | **Never.** Scraping is centralized. |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| **Google OAuth (consent screen)** | Leave in Testing mode | Move to Production after first dev test (no verification needed for `drive.file`). |
| **Google OAuth (scopes)** | Request `spreadsheets` "for compatibility" | `drive.file` only. Forces Picker flow but keeps consent simple and avoids verification. |
| **Google OAuth (loopback redirect)** | Hard-coded port 8080 | Random ephemeral port at runtime. |
| **Google OAuth (PKCE)** | Treated as optional | Required for desktop loopback flow. Always include. |
| **Google Drive Picker** | Skip Picker, use file ID directly | Picker is mandatory for `drive.file` to grant access to the workbook. |
| **Sheets API (concurrent writes)** | `values.update` on shared range | Per-character non-overlapping ranges. Use Apps Script Lock Service for any genuine shared mutation. |
| **Sheets API (write quotas)** | Assume "unlimited" because docs say no daily cap | 60 writes/min/user/project. Batch writes; one `batchUpdate` per `/outputfile` event. |
| **Sheets API (input parsing)** | `valueInputOption=USER_ENTERED` for everything | `RAW` for hot-path writes; `USER_ENTERED` only when intentional formula/date parsing wanted. |
| **Apps Script (triggers)** | Long-running synchronous trigger function | Re-entrant cursor pattern, exit at 5 min, schedule resume. |
| **Apps Script (locks)** | Lock the entire function body | Lock only the critical write section; do work outside the lock. |
| **Apps Script (locks)** | No try/finally around `releaseLock` | Always release in `finally`; orphan locks block other runs. |
| **Apps Script (sheet reads)** | `getValue()` per cell in a loop | One `getValues()` for the range, process in JS. |
| **Apps Script (writes)** | `setValue()` per cell | One `setValues()` per range. |
| **PigParse** | Per-item lookups via `getdetails` | `getall/{server}=1` once daily, cache. |
| **PigParse** | No User-Agent | `User-Agent: SquireBot/1.0 (+contact)` on every request. |
| **MediaWiki API** | HTML scrape rendered pages | `action=parse&prop=wikitext` (parse templates) or `action=query` for structured data. |
| **MediaWiki API** | Parallel page fetches | Serial with ~1s delay; ETag/`If-Modified-Since`. |
| **fsnotify** | Trust event payload | Re-stat the file; debounce 500ms. |
| **wincred / DPAPI** | Assume cross-machine portability | Per-machine. Re-OAuth on each PC. |
| **Discord (v2)** | Bot ToS Compliance | Read official Discord Bot Terms; don't scrape user messages without consent; rate-limit (50 req/sec). |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Recalc storm on bulk inventory write | Sheet hangs 10-30s after `/outputfile` | Computed columns in Apps Script, not formulas; `RAW` input mode | At 500+ row inventories with cross-tab `VLOOKUP` |
| Workbook cell-count growth | Sheet feels slow; new tabs error | Budget cells; use compressed JSON for time-series; archive old chars | At 5M cells (alarm threshold for 10M cap) |
| Apps Script 6-min execution timeout | Wiki scrape doesn't finish | Re-entrant cursor + scheduled resume | At ~15+ classes × 200+ pages |
| Apps Script 90-min daily trigger cap | Triggers stop firing | Move work to weekly cadence; cut polling frequency | At ~6 hours/week of trigger time on consumer Gmail |
| Sheets API 60 writes/min/user | 429 responses | One `batchUpdate` per inventory event (not one per row) | At 5+ alts run `/outputfile` simultaneously by one guildie |
| Concurrent watcher writes overwriting | Lost-update bug, missing rows | Per-character non-overlapping ranges | At 3+ guildies firing `/outputfile` within seconds |
| PigParse rate limit | Stale prices in tooltips | One bulk `getall` per day from Apps Script (not per-watcher) | At 12 watchers each polling, vs. our 1/day from Apps Script |
| Wiki rate limit | Empty scrape data | Polite UA + 1s sleep + ETag | At >10 req/sec sustained from one source |
| `fsnotify` event flood | Duplicate processing of same write | 500ms debounce; re-read file as unit | At every `/outputfile`, every watcher |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| OAuth refresh token stored in plaintext config | Anyone with file read can impersonate guildie against the sheet | Windows Credential Manager via `wincred` (DPAPI-backed). |
| Hard-coded OAuth client secret in binary | Public OAuth client compromise; impersonation | Desktop apps use PKCE without client secret. Never embed a secret. |
| Logging the access token / refresh token | Token leaks via log share | Redact `Authorization: Bearer ...` and `refresh_token` in all log statements. |
| Logging the workbook ID and guildie email together | PII + sensitive identifier in log | Hash or truncate emails in logs; don't share logs without redaction. |
| Sharing the workbook publicly ("anyone with link") | Strangers see guild bank contents | Share only with explicit guildie emails. Never set link-sharing public. |
| Upload `latest.json` / installer to S3 with overly permissive ACL | Integrity tampering, malware substitution | GitHub Releases with signed checksums; `latest.json` includes SHA-256, watcher verifies before swap. |
| Code-signing cert private key on developer's laptop | Cert theft → signed malware in our name | EV cert: hardware-token-bound by design. OV cert: store in Windows Cert Store with non-exportable flag, or HSM. |
| Apps Script Web App `executeAs` set to "User accessing the web app" | Permissions confusion (acts as caller, not script owner) | Set `executeAs` carefully per endpoint; for shared mutations, run as the script owner with appropriate input validation. |
| No validation of guildie email in OAuth callback | Any Google account that obtains OAuth code can write | Verify the OAuth'd email matches the workbook share list before trusting. Workbook share list is the ACL. |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Silent failure modes — watcher dies, guildie unaware | Stale data, eventual trust collapse | Systray icon color/badge state: green/working, yellow/warning, red/needs-attention. Tooltip explains. |
| Confusing "more info → run anyway" SmartScreen path | Guildies abandon at install | Either EV cert (no warning) or pre-recorded 30-second walkthrough video linked from the download page. |
| "Where's my EQ folder?" prompts | Confused guildie, support burden | Aggressive auto-discovery; only prompt as last resort; validate user input immediately. |
| OAuth flow hangs after Google consent | Guildie gives up | Detect timeout, offer manual code-paste fallback. |
| Guildies don't know their data is stale | Wrong loot decisions | Last-updated timestamp prominent on every view; conditional formatting ages it (orange/red). |
| Guildies edit system tabs by accident | Broken schema | Hidden + protected system tabs; `_` prefix as social signal. |
| No way to remove a guildie's data | Eviction drama | Documented soft-delete process; one-click "remove this character" button via Apps Script menu. |
| Item tooltips overwhelm with information | Hover blindness | Two-tier: cell-note for one-line summary, sidebar for rich detail on click. |
| First-run setup with multiple steps but no progress indicator | Guildie unsure if they're done | Setup wizard with explicit "Step 2 of 4" indicators. |
| Updates require manual reinstall | Maintenance fatigue | Auto-update; if not, in-app "update available" prompt with one-click action. |

---

## "Looks Done But Isn't" Checklist

- [ ] **OAuth flow:** Confirmed *both* Production publishing status (refresh token doesn't expire in 7d) AND Picker flow (`drive.file` actually grants access). Test by waiting 10 days post-OAuth and seeing if the watcher still writes.
- [ ] **Installer:** Tested on a *Standard User* (non-admin) Windows 11 VM. UAC must not prompt. SmartScreen path documented or eliminated by signing.
- [ ] **Encoding:** Tested with an inventory file containing a real apostrophe-quoted item name (e.g., a P99 Velious item with `'`). Tooltip text matches wiki.
- [ ] **Concurrency:** Simulated two watchers writing within 1 second to the workbook. Verified neither lost data via Sheet revision history.
- [ ] **Last-updated timestamp:** Surfaced on every guildie-visible view. Stale (>30 day) characters visually distinct.
- [ ] **fsnotify debounce:** Triggered `/outputfile` 5x within 1 second; watcher processed exactly once per file.
- [ ] **Refresh token recovery:** Manually revoked OAuth from Google account settings. Watcher detects, systray turns red, click reopens OAuth flow successfully.
- [ ] **Apps Script re-entrancy:** Wiki scrape interrupted mid-run (kill the trigger). Resumes from cursor on next run, doesn't restart from zero.
- [ ] **Schema fragility:** Manually rename a system tab. Watcher detects via tab-ID lookup, healthcheck alerts. (Or, reverts the rename gracefully.)
- [ ] **Cell budget:** Estimated cell count for typical guild (12 × 4 alts) under 2M after 1 year of operation.
- [ ] **PigParse contact:** Sent courtesy email to operator describing usage pattern; received acknowledgment or non-objection.
- [ ] **Wiki contact:** Identified ourselves to wiki admins (forum thread or email); polite-fetch pattern in code.
- [ ] **EQ folder discovery:** Tested on at least 3 different P99 install configurations (default path, custom path, multibox folder).
- [ ] **OneDrive AppData:** Tested with KFM enabled — confirmed credentials still work or fall back gracefully to re-OAuth.
- [ ] **DPAPI cross-machine:** Verified that re-installing on a second PC requires re-OAuth (expected behavior, document it).
- [ ] **Auto-update:** Tested update path with a deliberately corrupted download — recovery to known-good binary works.
- [ ] **Watcher version reporting:** Confirmed the version cell in the Status tab updates after every install.
- [ ] **Privacy soft-delete:** Documented the eviction process; tested it end-to-end on a fake guildie account.
- [ ] **Unhappy paths:** Watcher started with no internet; with EQ folder missing; with corrupted token; with workbook unshared. All produce clear systray status.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| OAuth Production toggle missed → tokens expiring | LOW | Switch to Production in Cloud Console; force re-OAuth across guild via systray prompt; document in changelog. |
| SmartScreen wall on installer | MEDIUM | Acquire/install code-signing cert; rebuild + re-release; communicate to guild "new download available, no scary warnings now." |
| Concurrent-write data loss | HIGH | Restore from Sheet revision history (manual, per-tab); re-architect to per-character ranges; force-re-output from each guildie to backfill. |
| Wiki parser broke on layout change | LOW-MEDIUM | Pause scraper (don't overwrite good data); update parser against new structure; test against schema-assertion baseline; resume. |
| PigParse rate-limited / blocked | LOW | Reach out to operator with apology; reduce cadence; switch to wiki/manual fallback for prices temporarily. |
| Stale data trust collapse | HIGH | Add timestamps everywhere (retrofit); send a "trust check" message to guild ("look at the new freshness column, we're back online"); rebuild confidence over weeks. |
| Workbook cell cap hit | MEDIUM | Move price history / wiki cache to a sibling workbook (split databases); update Apps Script lookups to span workbooks. |
| Tab deleted by guildie | LOW | Restore from version history (File → Version History); harden with hidden+protected tabs. |
| Guildie's refresh token revoked | LOW | Watcher prompts re-OAuth via systray; one click to recover. |
| Workbook accidentally deleted | HIGH | Restore from Drive trash (30-day window); if past 30 days, Google may help via support; otherwise rebuild from each watcher's local cache (if we kept one). |
| Code-signing cert leaked | HIGH | Revoke cert immediately; re-sign all releases with new cert; force-update guild via auto-update; potentially deal with reputation reset. |
| EQ patch broke `/outputfile` format | MEDIUM | Update parser; release patch; re-output across guild. |

---

## Pitfall-to-Phase Mapping

| Pitfall | Severity | Prevention Phase | Verification |
|---------|----------|------------------|--------------|
| 1. OAuth Testing-mode 7-day expiry | EXISTENTIAL | Phase 1 | Wait 10 days post-OAuth on test install; verify writes still work. |
| 2. SmartScreen warning | EXISTENTIAL | Phase 1 | Install on clean Win11 VM; check warning behavior; verify cert chain or walkthrough doc. |
| 3. Concurrent write lost-updates | EXISTENTIAL | Phase 2 | Two-watcher simulation; revision-history audit. |
| 4. Stale data trust collapse | EXISTENTIAL | Phase 2 (schema), Phase 3 (UI) | Last-updated column visible on every view; conditional formatting working. |
| 5. `drive.file` Picker missing | EXISTENTIAL | Phase 1 | First-run flow on uninitialized Google account succeeds. |
| 6. Per-machine UAC install | HIGH | Phase 1 | Standard-User install test, no UAC. |
| 7. Refresh token failure UX | HIGH | Phase 2 | Manually revoke OAuth; systray turns red; click recovers. |
| 7a. drive.file post-reauth write propagation (~50 min) | HIGH | Phase 2 | After revoke + Reauthorize, writes resume without further user action; `runPostReauthProbe` log lines visible until propagation; tray stays green throughout. |
| 8. PigParse rate-limit / annoyance | HIGH | Phase 3 | Operator contacted; one bulk request/day in Apps Script logs. |
| 9. Wiki etiquette violation | HIGH | Phase 3 | UA + ETag + 1s delay in `politeFetch`; admin contacted. |
| 10. Apps Script 6-min timeout | MEDIUM-HIGH | Phase 3 | Forced timeout test; cursor resume verified. |
| 11. Workbook cell cap | MEDIUM | Phase 2 (design), Phase 4 (monitoring) | Cell-budget estimate ≤ 2M; weekly count metric. |
| 12. Recalc storms | MEDIUM | Phase 2 | Bulk inventory write; observe sheet responsiveness. |
| 13. Schema fragility / tab rename | MEDIUM | Phase 2 (IDs), Phase 4 (healthcheck) | Manual rename test; healthcheck alerts. |
| 14. Auto-update gotchas | MEDIUM | Phase 4 | Deliberate-corruption update test; recovery works. |
| 15. Universal-visibility regret | MEDIUM | Phase 2 (schema fields), Phase 5 (process) | Soft-delete fields exist; eviction process documented. |
| 16. Cross-version migrations | MEDIUM | Phase 4 | Mixed-version watcher test on staging workbook. |
| 17. Latin-1 encoding | LOW-MEDIUM | Phase 1 | Test file with accented chars; tooltips render correctly. |
| 18. EQ folder discovery | MEDIUM | Phase 1 | Test on 3 install configurations. |
| 19. Stale watcher version | LOW | Phase 4 | Version reporting confirmed in Status tab. |
| 20. OneDrive / DPAPI quirks | LOW-MEDIUM | Phase 1 (storage), Phase 2 (fallback) | KFM-enabled test; re-OAuth fallback works. |
| 21. Loopback redirect blocked | MEDIUM | Phase 1 | Edge with strict mode test; manual-paste fallback works. |
| 22. Wiki page structure change | MEDIUM | Phase 3 | Schema assertions; last-known-good preservation. |
| 23. Character name collision | LOW-MEDIUM | Phase 2 | Composite key in schema; rename test. |
| 24. P99 client patch breakage | LOW | Phase 1 (defensive parser), Phase 4 (monitoring) | Defensive parsing; extra-column tolerance verified. |
| 25. Logs without rotation | LOW | Phase 1 | `lumberjack.v2` config verified. |
| 26. fsnotify event flood | LOW | Phase 1 | 5x rapid `/outputfile` test. |
| 27. Apps Script daily 90-min cap | LOW-MEDIUM | Phase 3 | Trigger budget under 90 min/day. |

**Existential pitfalls (1-5)** must be designed against during Phases 1-2 — these are the ones that *kill the project* if missed.
**High pitfalls (6-9)** are setup-time and external-relations risks; address in Phase 1 (installer/OAuth) and Phase 3 (external data).
**Medium pitfalls (10-22)** are operational; mostly Phase 2-4.
**Low pitfalls (23-27)** are polish/long-tail; can defer to Phase 4+.

---

## v2 Pitfall Preview (deferred — brief notes only)

When v2 (Discord pinger) becomes scoped, these will need full pitfall research:

- **Discord ToS compliance** — selfbots are forbidden; bots reading message content require "Message Content Intent" (privileged, requires Discord approval at >100 servers, currently we're at 3); rate limit is 50 req/sec global, plus per-route limits. Reading other servers' WTS channels without their explicit bot install is *forbidden by ToS* (would require auto-mod or scraping techniques that violate Discord's anti-scraping rules) — the v2 admin-invite prerequisite in PROJECT.md is not optional, it's the legal path.
- **Hosting costs** — Discord bots need always-on. Cloudflare Workers free tier (100K req/day) handles polling; for event-driven listening, a small VPS ($5/mo) or AWS Lambda (free tier covers 1M req/mo) is needed. Apps Script can poll PigParse but cannot receive Discord webhooks reliably.
- **False-positive auction matches** — wantlist for "Bag" matches every "Bag of Sewing" auction. Need item-ID-based matching, not name regex. PigParse exposes IDs; raw Discord WTS posts don't (they're free text). For Discord channels, fuzzy match + confirmation-via-DM ("did you want X for 200pp?") rather than auto-pinging on every match.
- **DM rate limits** — Discord DMs have aggressive rate limits and "stranger" detection (bots DMing users who haven't interacted recently can get flagged); guildies must opt-in by DMing the bot first.
- **Per-user Discord identity collection** — guildies enter their Discord ID once; verifying it (preventing typos that DM the wrong person) needs a confirmation handshake.
- **Bot invitation security** — Raid Alliance servers granting our bot Read access to their channels is a *trust transaction* with the alliance leadership. If our bot misbehaves (spams, leaks info, gets compromised), the alliance reputation is damaged, not just ours. Incident-response plan needed before deployment.
- **Notification fatigue** — guildies on multiple channels with broad wantlists get pinged constantly; need rate-limiting per user and "snooze" / digest options.

---

## Sources

- [Google: Manage App Audience (Testing → Production)](https://support.google.com/cloud/answer/15549945) — confirmed 7-day refresh-token expiration in Testing mode for external user type
- [Google Groups: Refresh Token Expires in 7 Days if OAuth Consent screen is Testing](https://groups.google.com/g/adwords-api/c/WDgwEZT6Cd0) — corroborating community report
- [Google: Restricted scope verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/restricted-scope-verification) and [Sensitive scope verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/sensitive-scope-verification) — `drive.file` is non-sensitive, no verification needed
- [Google: Choose Drive API scopes](https://developers.google.com/workspace/drive/api/guides/api-specific-auth) — `drive.file` semantics and Picker requirement
- [Google Sheets API: Usage limits](https://developers.google.com/workspace/sheets/api/limits) — 60 writes/min/user/project; 300/min/project; no daily cap (verified March 2026)
- [Apps Script: Lock Service](https://developers.google.com/apps-script/reference/lock) — concurrent-write protection patterns
- [Apps Script: Best Practices](https://developers.google.com/apps-script/guides/support/best-practices) — `setValues`-not-loop pattern, batching, flush-before-release-lock
- [Apps Script: Quotas for Google Services](https://developers.google.com/apps-script/guides/services/quotas) — 6-min execution, 90-min daily trigger, 20K UrlFetchApp/day consumer
- [Microsoft Learn: SmartScreen reputation for Windows app developers](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation) — code-signing requirements, EV cert behavior
- [Microsoft Learn: DPAPI MasterKey backup failures](https://learn.microsoft.com/en-us/troubleshoot/windows-server/certificates-and-public-key-infrastructure-pki/dpapi-masterkey-backup-failures) — Event 8196/8198 symptoms; roaming-profile gotchas
- [Winhelponline: Windows 10 forgets passwords](https://www.winhelponline.com/blog/windows-10-forgets-passwords-outlook-edge/) — DPAPI corruption symptoms relevant to Credential Manager
- [GitHub: barrier issue #295 — Binary installer blocked by SmartScreen](https://github.com/debauchee/barrier/issues/295) — case study of unsigned-binary SmartScreen UX impact
- [TechCommunity Microsoft: SmartScreen false positives](https://techcommunity.microsoft.com/discussions/windows-security/smartscreen-false-positives/4497486) — Defender ML heuristics and Go binaries
- [MediaWiki: API:Etiquette](https://www.mediawiki.org/wiki/API:Etiquette) — User-Agent + concurrency + caching guidelines
- [GitHub: minio/selfupdate](https://github.com/minio/selfupdate) — Windows replace-running-exe handling
- [GitHub: danieljoos/wincred](https://github.com/danieljoos/wincred) — Windows Credential Manager wrapper, DPAPI semantics
- [PigParse Swagger UI](https://pigparse.azurewebsites.net/swagger/index.html) — verified bulk endpoint exists for daily refresh
- [Project 1999 wiki: P99 patch history](https://wiki.project1999.com/Patch_Notes) — context for client-format-change risk
- [P1999 wiki robots.txt](https://wiki.project1999.com/robots.txt) — verified no crawl-delay set; we self-impose 1s
- STACK.md and FEATURES.md (sibling research) — architectural context and competitive landscape

---

*Pitfalls research for: SquireBot (Windows watcher + Google Sheets + light scraping, 12-user guild scale)*
*Researched: 2026-04-30*
