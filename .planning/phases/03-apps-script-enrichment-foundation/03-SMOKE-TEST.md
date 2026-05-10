# Phase 3 Smoke Test — End-to-End Verification

**Created:** 2026-05-10
**Owner:** Joe Bowen (boejowen)
**Goal:** Validate the 6 ROADMAP Phase 3 success criteria against a real workbook + real watcher + real wiki/PigParse traffic. Produces a PASS/FAIL/PARTIAL verdict per SC.

This is the equivalent of Phase 2's `docs/soak-runbook.md` for Phase 3 — but tighter, since Phase 3's failure modes are mostly observable in seconds, not days. Plan ~2 hours total elapsed; ~30 minutes of active attention.

---

## Pre-flight checklist

Confirm BEFORE running any test:

- [ ] **You're at watcher v0.3.0+.** `Get-Item "$env:LOCALAPPDATA\Programs\SquireBot\squirebot.exe" | % VersionInfo | % FileVersion` (PowerShell). If older, click tray → **Check for updates**, then quit + relaunch.
- [ ] **You have a fresh test workbook.** Either create a new Google Sheet or wipe the one you used during Phase 2 soak. The migration is destructive on `_pigparse` headers (extends them); easier to start fresh.
- [ ] **You have node 20+ and clasp set up.** From `apps-script/` directory: `node --version` (≥20), `npx clasp --version` (≥2.4 < 3.0).
- [ ] **You have at least one EQ character that produces inventory + spellbook output.** Slampeach (your Phase 1+2 test character) is fine.
- [ ] **You're prepared to monitor the workbook + watcher logs simultaneously.** Two windows open: workbook in browser, `Get-Content -Wait "$env:LOCALAPPDATA\SquireBot\squirebot.log"` in PowerShell.

---

## Setup phase (~10 minutes)

### Step 1 — Deploy Apps Script to test workbook

Follow `docs/apps-script-deploy.md` steps 1–10:

1. New Google Sheet → Extensions → Apps Script → copy script ID
2. `cd apps-script && cp .clasp.json.example .clasp.json` and paste the script ID
3. `npm install && npm run build && npm test` — **expect 99/99 tests passing**
4. `npx clasp login` (browser opens once)
5. `npx clasp push` (overwrite default `Code.gs`: yes)
6. **Refresh the workbook tab** so the SquireBot menu appears
7. SquireBot menu → **Run Migration** — approve OAuth scopes when prompted
8. Verify in `_meta` tab: `schema_version=2`, `theme=minimalist`, `contact_email` row present
9. SquireBot menu → **Install Triggers** — alert confirms 4 triggers installed

### Step 2 — Point watcher at the test workbook

If your dev box's SquireBot is configured for an old workbook, repick:

- Tray right-click → **Change Workbook…** → pick the test workbook
- Watcher log should show: `oauth callback received` (if reauth fired), `workbook picked`, `watcher started`

If your `_meta.canonical_id` was set during Phase 2 testing, the new (fresh) workbook will need scaffolding. The watcher's `ScaffoldSchemaV1` runs automatically on first contact.

### Step 3 — Initial dimension data

The view tab needs `_pigparse` + `_item_master` data to populate prices + wiki links. Without these, view rows show blank price + blank wiki cells (technically still PASS for SC-1 if the row appears within 30s, but not a useful smoke test).

- SquireBot menu → **Refresh PigParse Now** — wait ~10s, watch `_pigparse` populate. **Expect 7,000+ rows.**
- SquireBot menu → **Refresh Wiki Items Now** — this iterates the wiki for every distinct item ID currently in any `inv:*` tab. On a fresh workbook with no inventory yet, expect: log shows `total: 0`, returns immediately. We'll trigger it again after Step 4.

---

## Test phase

### SC-1 — End-to-end: upload → view row appears within ~30s with hyperlink + price + tooltip

**ROADMAP §SC-1:** "End-to-end test on a sample character: a watcher upload triggers `onChange` → consolidated `view` row appears within ~30s with hyperlink to P1999 wiki, current PigParse price, and a hover cell-note composing wiki summary + price summary + 'Quest item: used in X, Y, Z' line where applicable."

**Procedure:**
1. Note the current time precisely.
2. In EQ as Slampeach (or any test character): `/outputfile inventory`
3. Watch the `view` tab in the browser. Note the time when the first `Slampeach` row appears.
4. Hover over an Item cell that you know has a wiki entry (e.g., `Pearl` if Slampeach has one).

**Pass conditions:**
- View row appears in ≤30s from `/outputfile inventory` execution.
- Hover note shows: wiki summary text + `Recent ask: <N>pp (30d avg, ...)` line + (if quest item) `Quest item: yes (in-game flag)` line.
- Wiki cell shows `wiki` link that opens `https://wiki.project1999.com/<item>` when clicked.
- Price column has a number for items that have PigParse data (most common items) and is blank for obscure items (acceptable).

**If FAIL:**
- View row never appears: check `_meta.last_error` and the script editor's Executions tab for `buildView` failures. Common cause: triggers not installed → re-run **Install Triggers**.
- View row appears but no price/wiki: `_pigparse` or `_item_master` empty. Re-run **Refresh PigParse Now** + **Refresh Wiki Items Now** (the latter needs >0 inv rows to do anything meaningful — that's why Step 4 is gated by Step 3).
- View row appears >30s late: likely `onChange` lagging. The 1-hour backstop will eventually catch up. Note the actual delay — anything <60s is still acceptable.

### SC-1.b — Wiki summaries populate after first inventory upload

After SC-1's inventory upload, **re-run Refresh Wiki Items Now**. This time it has work to do.

**Pass conditions:**
- Log shows `total: <N>` matching the count of distinct item IDs in `inv:Slampeach`.
- `_item_master` populates with one row per item.
- `_quest_items` populates if any of Slampeach's items have notes-link references (Pearl will, if owned).
- Re-run the trigger again immediately — log shows most items as `unchanged` (SHA-1 short-circuit working).
- Hover the `view` row's Item cell again — note now includes the wiki summary paragraph.

### SC-2 — Daily PigParse trigger writes _pigparse with row-count assertion

**ROADMAP §SC-2:** Already partially exercised by Step 3's manual refresh. To verify the assertion:

**Procedure:**
1. Set `_status.last_pigparse_row_count` artificially high: in the `_status` tab, edit the row to `last_pigparse_row_count = 99999`.
2. SquireBot menu → **Refresh PigParse Now**.
3. Check `_meta.last_error` immediately after.

**Pass conditions:**
- `_meta.last_error` contains `truncated_response` JSON with `today=<actual count> last=99999`.
- `_pigparse` data rows untouched (compare row count before/after; should be unchanged).
- `_status.last_pigparse_row_count` still = `99999` (preserved, NOT overwritten with the small value).

**Then revert:** set `_status.last_pigparse_row_count = 0` and re-run **Refresh PigParse Now**. Verify `_pigparse` repopulates and `last_error = {}`.

### SC-3 — Long-running scrape resumes from cursor

**ROADMAP §SC-3:** "A long-running scrape interrupted at the 5-minute mark resumes correctly from the cursor stored in `PropertiesService` after the self-rescheduled trigger fires."

This is hard to verify deliberately — Apps Script doesn't expose a way to inject a fake clock. Easiest path:

**Procedure (organic verification):**
1. Need ≥150 distinct item IDs in inventory tabs to push wiki refresh past 5 min wall-clock at 1 fetch/sec. Slampeach alone might not be enough; if only one character, generate ~150 fake inv rows by editing a temp inv tab manually.
2. SquireBot menu → **Refresh Wiki Items Now**.
3. After ~5 min, log should emit `{"checkpoint":true,"remaining":<N>}` and the function returns.
4. Check `PropertiesService` cursor: in the script editor, run `console.log(PropertiesService.getDocumentProperties().getProperty('wiki_refresh_cursor'))`. Should print a JSON object with `remaining`, `total`, `successes`, `failures`, `unchanged`, `started`.
5. ~60s later, the self-scheduled trigger fires. Log: `{"resuming":true,"total":<N>,"remaining":<N-already-processed>,...}`.
6. Eventually log shows `{"done":true,...}` and the cursor property is deleted.

**Pass conditions:**
- Log emits `checkpoint:true` mid-run.
- `wiki_refresh_cursor` property is set after checkpoint and cleared after `done`.
- Total processed across the multiple resumed runs equals the initial total.
- No item processed twice (verify by counting `_item_master` rows after = unique item ID count).

**If you don't have 150+ items handy:** mark SC-3 as PASS-by-code-inspection (the resumable cursor logic is unit-tested in `apps-script/src/__tests__/refreshWikiItems.test.ts` — the test "Mid-budget checkpoint" exercises the same code path with mocked `Date.now()`).

### SC-4 — politeFetch behavior

**ROADMAP §SC-4:** "All outbound HTTP goes through `politeFetch(url)` with identifying User-Agent, ETag/`If-Modified-Since`, `CacheService`, exponential backoff on `429/503/504`, and the 1-second `Utilities.sleep` between wiki requests — verified by inspecting Apps Script logs across one full refresh cycle."

**Procedure:**
1. Open script editor → Executions tab. Trigger **Refresh Wiki Items Now**.
2. Click into the latest execution. Read the log entries.

**Pass conditions:**
- Every wiki fetch logged with the URL pattern `https://wiki.project1999.com/api.php?...&redirects=true`.
- No log entry shows back-to-back wiki fetches in <1s (caller-side sleep is verified).
- If any 429/503/504 occurs (rare during normal use), a `politeFetch` warn log shows `attempt:<N> status:<code> waitMs:<ms>` and the next fetch shows the wait actually happened.

**Pass-by-code-inspection alternative:** the politeFetch retry behavior is unit-tested in `apps-script/src/__tests__/politeFetch.test.ts` (10 cases including 429/503/504 retries + Retry-After honoring + exhaustion). UA string is asserted there too. If you don't see real 429s during smoke (likely), accept those tests as evidence and only verify the inter-request 1s sleep manually.

### SC-5 — Last Synced conditional formatting + LockService

**ROADMAP §SC-5:** "The `Last Synced` cell on every `view` row uses conditional formatting (green ≤7d, orange ≤30d, red >30d), and `LockService.getDocumentLock().tryLock(30000)` in `try/finally` guards every aggregate write that touches a shared range."

**Procedure (conditional formatting):**
1. Open the `view` tab → Format menu → **Conditional formatting**.
2. Should see 3 rules attached to column H (Last Synced).
3. The freshest row (just-uploaded) has a green background. Verify visually.

**Procedure (LockService — manual stress):**
1. Open script editor → run `buildView()` from the function dropdown.
2. WHILE that's running (~5–10s), in a second tab open the script editor and run `buildView()` again.
3. Second run should log `{"skipped":"lock_busy"}` and return cleanly.

**Pass conditions:**
- 3 conditional formatting rules visible on view tab col H.
- Concurrent `buildView()` invocations don't both write — second emits `lock_busy`.

### SC-6 — Courtesy contact emails — WAIVED

**ROADMAP §SC-6:** "Courtesy contact emails to the PigParse operator and the P1999 wiki admins are sent and acknowledged **before** the daily/weekly triggers fire against live infrastructure."

**Status:** WAIVED per user decision 2026-05-09. Live triggers deploy without acknowledgment gate. UA stays GitHub-link-only until user opts in. See 03-CONTEXT.md §Courtesy Contact for the rationale.

This is a documented waiver, not a regression. Phase 3 verdict should call it out as such.

---

## Verdict template

After running the test, write the verdict to `docs/phase3-smoke-verdict.md` with this shape:

```markdown
# Phase 3 Smoke Test Verdict

**Tested:** YYYY-MM-DD HH:MM:SSZ
**Tester:** Joe Bowen
**Watcher version:** 0.3.0
**Apps Script bundle:** dist/Code.js (1,607 lines, 9/9 globals)

| SC  | Result | Evidence |
|-----|--------|----------|
| 1   | PASS / FAIL / PARTIAL | First Slampeach row appeared in <Ns> after /outputfile inventory; tooltip shows wiki+price+quest |
| 1.b | ... | refreshWikiItems populated _item_master with N rows |
| 2   | ... | _pigparse row count assertion fired correctly on artificial last_count=99999 |
| 3   | ... | Either organic checkpoint observed OR pass-by-code-inspection (refreshWikiItems test #N) |
| 4   | ... | Inspected 1 full refresh cycle in Executions tab; UA + 1s sleep verified |
| 5   | ... | 3 conditional formatting rules on col H; concurrent buildView() emits lock_busy |
| 6   | WAIVED | per user decision 2026-05-09 |

## Findings

- (one bullet per issue or noteworthy observation)

## Verdict

PASS / FAIL / PARTIAL — `phase3-complete` tag deferred / proceed.
```

---

## When to run

**Don't rush.** Wait until:

1. Watcher v0.3.0 has actually rolled out to your dev box (check the version in the tray menu's "About" or via `Get-Item ... | % VersionInfo`).
2. You have ~2 hours uninterrupted.
3. Your dev workbook is in a state you don't mind wiping (the migration changes header rows).

If anything in SC-1 fails clearly (no view row in 60s, error in `_meta.last_error`, OAuth scope rejected), STOP and capture the script editor's Executions tab log + `_meta.last_error` value. Most Phase 3 failures will be visible within minutes — not the days-long observation window Phase 2 needed.

---

*Plan: 03-SMOKE-TEST.md*
*Authored: 2026-05-10 (post-watcher-v0.3.0-ship)*
*Next step after PASS verdict: tag `phase3-complete`, then `/gsd-discuss-phase 4` for Differentiator Features (gear_check + spell_check + bank coin sidebar).*
