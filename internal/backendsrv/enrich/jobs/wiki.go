package jobs

// wiki.go is the weekly P1999 wiki job (ENRICH-11) — the Go replacement for the
// three refreshWiki* triggers (items, spells, gear-tier) collapsed into ONE
// uninterrupted run. It composes the Wave-1 pieces and authors ZERO inline SQL
// (mirrors ingest/handler.go::bindAndReplace): for each wiki page it does
//
//	store.GetETag → politefetch.Fetch → extract wikitext → enrich.Parse* →
//	store.*Tx over ONE tx → store.SetETag
//
// DELETED, not ported (D-5/D-8 — REJECT if reintroduced): the 6-minute
// resumable-position machinery (the trigger's self-rescheduling save+resume), the
// failure-threshold abort, the 10M-cell watchdog, the expected-tab schema
// watchdog, the Apps Script document lock, the script-properties store, and the
// post-run gear/spell view-rebuild calls. A backend job has NO execution cap, so
// the weekly scrape is one uninterrupted pass; the single-writer DB + this
// package's per-job mutex (12-05) replace the Apps Script lock; and the view tabs
// belong to P14.
//
// Politeness (SC-4): a ctx-aware 1-second sleep runs BEFORE every wiki page fetch
// (the seam mirrors internal/update/check.go's checkSleepFn — a long sleep
// unwinds promptly on SIGTERM, and tests override it to a no-op). The 1s sleep is
// the JOB's, not the polite client's (politeFetch only sleeps between retries of
// one call).
//
// Resilience: a single bad page (fetch error, missing wikitext, parse failure) is
// logged-and-SKIPPED — the weekly run NEVER aborts on one failure (mirrors the
// auto-update loop's log-but-continue). A 304 on any resource SKIPS that
// resource's parse+write (Pitfall 6). The gear-tier full-table replace fires only
// with the COMPLETE combined set from BOTH pages (a failed/304'd page skips the
// replace rather than wiping the table with a partial set — Pitfall 1).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/enrich/politefetch"
	"github.com/boejowen/SquireBot/internal/backendsrv/store"
)

// wikiJobName is the job_run position-marker key + slog op for the weekly job.
const wikiJobName = "wiki_weekly"

// interRequestSleep is the 1-second courtesy pause between wiki page fetches
// (refreshWiki*.ts INTER_REQUEST_SLEEP_MS = 1000; ROADMAP SC-4).
const interRequestSleep = 1 * time.Second

// wikiSleepFn is the package-level ctx-aware sleep seam (mirrors
// internal/update/check.go's checkSleepFn). Production = realWikiSleep; tests
// override it via setWikiSleepNoop so the run is instant + retries don't wait.
var wikiSleepFn = realWikiSleep

// realWikiSleep waits d, returning early with ctx.Err() if ctx is cancelled, so
// a SIGTERM mid-crawl unwinds promptly. NEVER a bare time.Sleep here.
func realWikiSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// setWikiSleepNoop swaps wikiSleepFn for an instant no-op and returns a restore
// func. Test-only seam so the wiki job runs without real 1-second waits.
func setWikiSleepNoop() func() {
	prev := wikiSleepFn
	wikiSleepFn = func(context.Context, time.Duration) error { return nil }
	return func() { wikiSleepFn = prev }
}

// wikiVeliousGearSources are the 2 Velious gear-tier pages + their base tier,
// ported from refreshWikiGearTier.ts SOURCES. The parser re-tags Iksar-prefixed
// items on the Pre-Raid page to TierIksar.
var wikiVeliousGearSources = []struct {
	pageTitle string
	tier      enrich.Tier
}{
	{"Players:Velious Pre-Raid Gear", enrich.TierVeliousPreRaid},
	{"Players:Velious Raiding Gear", enrich.TierVeliousRaiding},
}

// classAbbrevToDisplay inverts enrich.CLASS_DISPLAY_TO_ABBREV so the spells pass
// can build each class's wiki page title (e.g. "NEC" → "Necromancer"), mirroring
// refreshWikiSpells.ts's CLASS_ABBREV_TO_DISPLAY lookup. Built once at init.
var classAbbrevToDisplay = invertClassMap()

func invertClassMap() map[string]string {
	m := make(map[string]string, len(enrich.CLASS_DISPLAY_TO_ABBREV))
	for display, abbrev := range enrich.CLASS_DISPLAY_TO_ABBREV {
		m[abbrev] = display
	}
	return m
}

// wikiEnvelope is the MediaWiki action=parse response shape (the same fields the
// parser tests' loadWikitext + the TS triggers read). An `error` object means the
// page is missing/invalid — treated as a per-page skip.
type wikiEnvelope struct {
	Parse struct {
		Title    string `json:"title"`
		Wikitext struct {
			Star string `json:"*"`
		} `json:"wikitext"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// fetchOutcome distinguishes the three terminal states of one page fetch.
type fetchOutcome int

const (
	fetchGotPage   fetchOutcome = iota // 200 + valid wikitext → page is set
	fetchUnchanged                     // 304 → skip this resource (Pitfall 6)
	fetchSkip                          // !OK / error envelope / empty wikitext → log+continue
)

// RunWiki runs the weekly wiki scrape as ONE uninterrupted pass: items+quest
// links, then per-class spells, then the 2-page gear-tier full replace. fetch is
// injected (politefetch.Fetch in production; an httptest-backed fetcher in tests).
// It returns nil even when individual pages fail (they are logged + skipped); it
// returns a non-nil error only on a wholesale setup failure (e.g. the inventory
// union read fails). The job_run position marker advances to 'ok' on completion.
func RunWiki(ctx context.Context, db *sql.DB, fetch politefetch.Fetcher) error {
	s := store.NewStore(db)
	now := time.Now().UTC()

	itemsOK, itemsUnchanged, itemsFail, err := runWikiItems(ctx, db, s, fetch, now)
	if err != nil {
		// A wholesale failure (couldn't read the inventory union) — record + return.
		slog.Error(wikiJobName+": items pass setup failed", "err", err)
		_ = s.SetJobRun(ctx, wikiJobName, now, "error", "items_setup_failed")
		return fmt.Errorf("%s: items pass: %w", wikiJobName, err)
	}

	spellsOK, spellsSkip := runWikiSpells(ctx, db, s, fetch, now)
	gearOK, gearRows := runWikiGearTier(ctx, db, fetch, now)

	detail := fmt.Sprintf(
		"items_ok=%d items_unchanged=%d items_fail=%d spells_classes=%d spells_skip=%d gear_replaced=%t gear_rows=%d",
		itemsOK, itemsUnchanged, itemsFail, spellsOK, spellsSkip, gearOK, gearRows,
	)
	slog.Info(wikiJobName+": ok", "detail", detail)
	return s.SetJobRun(ctx, wikiJobName, now, "ok", detail)
}

// runWikiItems is sub-pass A: iterate the distinct (item_id,name) inventory union,
// fetch+parse each item's wiki page, SHA-1 short-circuit, and upsert item_master
// + quest_items in one tx per item. Returns (written, unchanged, failed) counts.
func runWikiItems(ctx context.Context, db *sql.DB, s *store.Store, fetch politefetch.Fetcher, now time.Time) (written, unchanged, failed int, err error) {
	refs, rerr := s.DistinctInventoryItemIDs(ctx)
	if rerr != nil {
		return 0, 0, 0, rerr
	}
	nowStr := now.Format(time.RFC3339)

	for _, ref := range refs {
		// Politeness: 1s ctx-aware sleep before every wiki fetch.
		if serr := wikiSleepFn(ctx, interRequestSleep); serr != nil {
			slog.Info(wikiJobName+": items pass interrupted", "err", serr)
			return written, unchanged, failed, nil // ctx cancelled — stop cleanly, not an error
		}

		url := wikiParseURL(ref.Name)
		page, outcome := fetchWikiPage(ctx, s, fetch, url)
		switch outcome {
		case fetchUnchanged:
			unchanged++
			continue
		case fetchSkip:
			failed++
			continue
		}

		item, questLinks, ok, reason := enrich.ParseItempage(page.wikitext, page.title)
		if !ok {
			slog.Warn(wikiJobName+": item parse skipped", "item_id", ref.ItemID, "reason", reason)
			failed++
			continue
		}

		didWrite, werr := upsertItemAndQuests(ctx, db, ref, item, questLinks, nowStr)
		if werr != nil {
			// A per-item DB error is logged + skipped (the run marches on).
			slog.Warn(wikiJobName+": item write failed", "item_id", ref.ItemID, "err", werr)
			failed++
			continue
		}
		// Persist the page ETag after a successful fetch+parse — WHETHER OR NOT we wrote. The
		// ETag corresponds to the page we just validated, so caching it lets the next run
		// 304-skip an unchanged page. (Previously only set after a write; combined with the
		// one-time icon-backfill etag_cache clear, that left every no-write item with no
		// cached ETag and thus re-fetched on every weekly run.)
		if serr := s.SetETag(ctx, url, page.etag, page.lastModified); serr != nil {
			slog.Warn(wikiJobName+": item set etag failed", "item_id", ref.ItemID, "err", serr)
		}
		if !didWrite {
			unchanged++
			continue
		}
		written++
	}
	return written, unchanged, failed, nil
}

// upsertItemAndQuests applies the SHA-1 short-circuit then, if changed, writes
// item_master + quest_items for ref in ONE tx. Returns didWrite=false (and no
// error) when the wikitext SHA-1 is unchanged (the upsert is skipped, mirroring
// the Sheet's readItemMasterSha early-return).
func upsertItemAndQuests(ctx context.Context, db *sql.DB, ref store.ItemRef, item enrich.ParsedWikiItem, questLinks []enrich.WikiQuestItemLink, nowStr string) (didWrite bool, err error) {
	tx, err := db.BeginTx(ctx, nil) // BEGIN IMMEDIATE (single-writer DSN)
	if err != nil {
		return false, fmt.Errorf("begin item tx (item_id=%d): %w", ref.ItemID, err)
	}
	defer tx.Rollback() // no-op after Commit

	existingSHA, existingIcon, existingStats, existingFlagsJSON, err := store.GetItemMasterFreshnessTx(ctx, tx, ref.ItemID)
	if err != nil {
		return false, err
	}
	// parsedFlagsJSON is the freshly-parsed flag set encoded by the SAME canonical
	// helper the upsert + backfill use (NOT a local json.Marshal), so a flagless
	// item's "[]" byte-equals the stored "[]" and is NOT re-written every pass (D-06
	// idempotency). A pre-00016 row's NULL flags_json reads "" here, which differs
	// from this value and so correctly re-writes once to backfill.
	parsedFlagsJSON := store.MarshalFlags(item.Flags)
	if existingSHA == item.WikitextSHA1 && existingIcon == int64(item.IconID) &&
		existingStats == item.Statsblock && existingFlagsJSON == parsedFlagsJSON {
		// The wikitext AND the icon AND the statsblock AND the flag set are all unchanged —
		// skip the write (the empty tx rolls back via defer). NOTE: SHA-1 alone is NOT
		// sufficient — a row written before the icon_id (00012), statsblock (00013) or
		// flags_json (00016) columns has the same SHA-1 yet a 0 icon / "" statsblock / NULL
		// flags_json, and must still be re-written to backfill those derived fields.
		return false, nil
	}

	if err := store.UpsertItemMasterTx(ctx, tx, store.ItemMaster{
		ItemID:        int(ref.ItemID),
		Name:          item.ItemName,
		WikiSummary:   item.Summary,
		WikiURL:       item.WikiURL,
		Slot:          item.Slot,
		IsQuestItem:   item.IsQuestItem,
		WikitextSHA1:  item.WikitextSHA1,
		LastRefreshed: nowStr,
		IconID:        item.IconID,     // INV-04: carry the parsed lucy_img_ID into item_master.icon_id
		Statsblock:    item.Statsblock, // INV-02: carry the cleaned stat block into item_master.statsblock

		// Phase 37 (ENRICH-12/13, 00016): carry the parsed flag/effect fields.
		IsLore:       item.IsLore,
		IsNoDrop:     item.IsNoDrop,
		IsMagic:      item.IsMagic,
		IsTemporary:  item.IsTemporary,
		IsClicky:     item.IsClicky,
		ClickyEffect: item.ClickyEffect,
		HasHaste:     item.HasHaste,
		HastePct:     item.HastePct,
		FlagsJSON:    parsedFlagsJSON, // the ONE canonical MarshalFlags string (D-06)
	}); err != nil {
		return false, err
	}

	quests := make([]store.QuestItem, 0, len(questLinks))
	for _, l := range questLinks {
		quests = append(quests, store.QuestItem{
			ItemID:        int(ref.ItemID),
			QuestName:     l.QuestName,
			SourceURL:     l.SourceURL,
			Source:        l.Source,
			LastRefreshed: nowStr,
		})
	}
	if err := store.ReplaceQuestItemsForIDTx(ctx, tx, ref.ItemID, quests); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit item tx (item_id=%d): %w", ref.ItemID, err)
	}
	return true, nil
}

// runWikiSpells is sub-pass B: for each of the 14 classes, fetch+parse the class
// spell page and per-class replace wiki_spells in one tx. The post-run spell-view
// rebuild is DROPPED (P14 owns views — D-8). Returns (classesWritten,
// classesSkipped).
func runWikiSpells(ctx context.Context, db *sql.DB, s *store.Store, fetch politefetch.Fetcher, now time.Time) (written, skipped int) {
	nowStr := now.Format(time.RFC3339)

	for _, class := range enrich.CLASSES {
		if serr := wikiSleepFn(ctx, interRequestSleep); serr != nil {
			slog.Info(wikiJobName+": spells pass interrupted", "err", serr)
			return written, skipped
		}

		display, ok := classAbbrevToDisplay[class]
		if !ok {
			slog.Warn(wikiJobName+": no display name for class", "class", class)
			skipped++
			continue
		}
		url := wikiParseURL(display)
		page, outcome := fetchWikiPage(ctx, s, fetch, url)
		if outcome != fetchGotPage {
			skipped++
			continue
		}

		rows, perr := enrich.ParseClassPage(page.wikitext, class)
		if perr != nil { // ParseClassPage never errors today, but be defensive.
			slog.Warn(wikiJobName+": class parse failed", "class", class, "err", perr)
			skipped++
			continue
		}

		spells := make([]store.WikiSpell, 0, len(rows))
		for _, r := range rows {
			spells = append(spells, store.WikiSpell{
				Class:          r.Class,
				Level:          r.Level,
				SpellName:      r.SpellName,
				NormalizedName: r.NormalizedName,
				LastRefreshed:  nowStr,
			})
		}
		if werr := replaceSpellsForClass(ctx, db, class, spells); werr != nil {
			slog.Warn(wikiJobName+": class write failed", "class", class, "err", werr)
			skipped++
			continue
		}
		if serr := s.SetETag(ctx, url, page.etag, page.lastModified); serr != nil {
			slog.Warn(wikiJobName+": class set etag failed", "class", class, "err", serr)
		}
		written++
	}
	return written, skipped
}

// replaceSpellsForClass wraps the per-class replace in its own tx (one class =
// one atomic DELETE+INSERT, D-12). Even a degenerate 0-row class (Warrior) is
// replaced, clearing any stale prior rows.
func replaceSpellsForClass(ctx context.Context, db *sql.DB, class string, spells []store.WikiSpell) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin spells tx (class=%q): %w", class, err)
	}
	defer tx.Rollback()
	if err := store.UpsertWikiSpellsForClassTx(ctx, tx, class, spells); err != nil {
		return err
	}
	return tx.Commit()
}

// runWikiGearTier is sub-pass C: fetch+parse BOTH Velious gear pages, accumulate
// the combined row set, then full-table replace wiki_gear_tier ONCE. The
// post-run gear-view rebuild is DROPPED (D-8).
//
// Gear is a wholesale full-replace, NOT a per-resource conditional fetch: it
// needs the COMPLETE combined set from BOTH pages every run, so both pages are
// fetched UNCONDITIONALLY (no ETag / If-None-Match — and NO SetETag for them),
// exactly like the TS gear trigger, which called politeFetch(url) with no etag
// option (refreshWikiGearTier.ts:193-194) and rebuilt the table on every
// successful weekly run. Earlier this pass sent per-page ETags AND advanced them
// immediately on a 200; combined with the "replace only when BOTH pages are
// fresh" rule, a single changed page advanced its cached ETag without the replace
// firing, so its edit was dropped — and stayed dropped, because next run that
// page 304'd on the freshly-cached ETag (the H-01 staleness trap). Dropping the
// ETag conditional for gear removes the trap and restores TS fidelity.
//
// Pitfall 1 still holds: the full replace fires ONLY with the COMPLETE set — if
// either page is unavailable (fetch error, error envelope, empty wikitext, or
// parse failure) we SKIP the replace rather than wipe the table with a partial
// set (mirrors the TS 1/2-success partial_failure abort). Returns (replaced,
// combinedRowCount).
func runWikiGearTier(ctx context.Context, db *sql.DB, fetch politefetch.Fetcher, now time.Time) (replaced bool, rowCount int) {
	nowStr := now.Format(time.RFC3339)

	var allRows []store.WikiGearTier
	allFresh := true // every page returned a fresh 200 we could parse
	for _, src := range wikiVeliousGearSources {
		if serr := wikiSleepFn(ctx, interRequestSleep); serr != nil {
			slog.Info(wikiJobName+": gear pass interrupted", "err", serr)
			return false, 0
		}

		// Unconditional fetch (no ETag) — a 304 must be impossible here, or the
		// replace below could never assemble the complete set.
		url := wikiParseURL(src.pageTitle)
		page, outcome := fetchWikiPageUnconditional(ctx, fetch, url)
		if outcome != fetchGotPage {
			// Fetch error / error envelope / empty wikitext → we don't have this
			// page's current rows, so a full replace would wipe the OTHER page's
			// rows. Skip the replace (Pitfall 1).
			slog.Warn(wikiJobName+": gear page unavailable; skipping full replace", "page", src.pageTitle)
			allFresh = false
			continue
		}

		rows, perr := enrich.ParseGearTierPage(page.wikitext, src.tier)
		if perr != nil {
			slog.Warn(wikiJobName+": gear parse failed", "page", src.pageTitle, "err", perr)
			allFresh = false
			continue
		}
		for _, r := range rows {
			var id sql.NullInt64
			if r.ItemID != nil { // always nil from the parser, but stay faithful to the type
				id = sql.NullInt64{Int64: int64(*r.ItemID), Valid: true}
			}
			allRows = append(allRows, store.WikiGearTier{
				Tier:          string(r.Tier),
				Class:         r.Class,
				Slot:          r.Slot,
				ItemID:        id,
				ItemName:      r.ItemName,
				Rank:          r.Rank,
				LastRefreshed: nowStr,
			})
		}
		// NO SetETag here: gear pages are full-replaced every run, never
		// conditionally fetched (see the staleness-trap note above).
	}

	if !allFresh {
		// A page was missing — don't clobber the table with a partial set
		// (Pitfall 1 + the TS partial-failure abort). Leave existing rows.
		return false, len(allRows)
	}

	if err := replaceGearTier(ctx, db, allRows); err != nil {
		slog.Warn(wikiJobName+": gear full replace failed", "err", err)
		return false, len(allRows)
	}
	return true, len(allRows)
}

// replaceGearTier wraps the full-table replace in one tx (DELETE-all + INSERT of
// the combined set — the only correct strategy because wiki_gear_tier's UNIQUE is
// NULL-poisoned; a per-row upsert would duplicate every Sunday, Pitfall 1).
func replaceGearTier(ctx context.Context, db *sql.DB, rows []store.WikiGearTier) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin gear tx: %w", err)
	}
	defer tx.Rollback()
	if err := store.ReplaceWikiGearTierTx(ctx, tx, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// wikiPageResult carries the fetched page + its ETag state for the SetETag step.
type wikiPageResult struct {
	wikitext     string
	title        string
	etag         string
	lastModified string
}

// fetchWikiPage performs one polite wiki fetch for url: it reads the cached
// ETag/Last-Modified, fetches (304-aware), and on a 200 decodes the action=parse
// envelope to extract the inner wikitext + resolved title. It returns the page +
// fetchGotPage on success, fetchUnchanged on a 304 (skip the resource — Pitfall
// 6), or fetchSkip on a fetch error / MediaWiki error envelope / empty wikitext
// (the caller logs + continues; the weekly run never aborts on one page).
func fetchWikiPage(ctx context.Context, s *store.Store, fetch politefetch.Fetcher, url string) (wikiPageResult, fetchOutcome) {
	etag, lastMod, err := s.GetETag(ctx, url)
	if err != nil {
		slog.Warn(wikiJobName+": etag read failed; fetching unconditionally", "url", url, "err", err)
		etag, lastMod = "", ""
	}

	res := fetch(ctx, url, politefetch.Options{ETag: etag, LastModified: lastMod})
	if res.FromCache { // 304 — unchanged, skip parse+write (Pitfall 6)
		return wikiPageResult{}, fetchUnchanged
	}
	return decodeWikiResult(url, res)
}

// fetchWikiPageUnconditional fetches url with NO conditional headers (empty
// Options{} ⇒ no If-None-Match / If-Modified-Since), so MediaWiki always returns
// a full 200 — a 304 is impossible. Used by the gear-tier pass, which is a
// wholesale full-table replace that needs the COMPLETE combined set from BOTH
// pages every run: the TS gear trigger (refreshWikiGearTier.ts:193-194) called
// politeFetch(url) with NO etag option for exactly this reason. A per-page ETag
// here would let a single changed page advance its cache entry without the
// replace firing (the replace only fires when BOTH pages are fresh), permanently
// dropping that page's edit — the H-01 staleness trap. So gear deliberately skips
// the ETag cache (read AND write). It can still return fetchSkip on a fetch
// error / error envelope / empty wikitext (the caller skips the replace then).
func fetchWikiPageUnconditional(ctx context.Context, fetch politefetch.Fetcher, url string) (wikiPageResult, fetchOutcome) {
	res := fetch(ctx, url, politefetch.Options{})
	// No conditional headers were sent, so res.FromCache can't be true; if a
	// fetcher ever returned it anyway, treat it as a skip (we have no body to
	// parse) rather than silently dropping the page from the combined set.
	if res.FromCache {
		slog.Warn(wikiJobName+": unexpected 304 on unconditional gear fetch; skipping", "url", url)
		return wikiPageResult{}, fetchSkip
	}
	return decodeWikiResult(url, res)
}

// decodeWikiResult turns a non-304 FetchResult into a wikiPageResult: it checks
// the HTTP outcome, decodes the action=parse envelope, and extracts the inner
// wikitext + resolved title. Shared by the conditional (items/spells) and
// unconditional (gear) fetch paths. Returns fetchSkip on a fetch error /
// MediaWiki error envelope / empty wikitext (the caller logs + continues).
func decodeWikiResult(url string, res politefetch.FetchResult) (wikiPageResult, fetchOutcome) {
	if !res.OK {
		slog.Warn(wikiJobName+": wiki fetch failed", "url", url, "status", res.Status, "err", res.Err)
		return wikiPageResult{}, fetchSkip
	}

	var env wikiEnvelope
	if uerr := json.Unmarshal(res.Body, &env); uerr != nil {
		slog.Warn(wikiJobName+": wiki json decode failed", "url", url, "err", uerr)
		return wikiPageResult{}, fetchSkip
	}
	if env.Error != nil {
		slog.Warn(wikiJobName+": wiki api error", "url", url, "code", env.Error.Code)
		return wikiPageResult{}, fetchSkip
	}
	wikitext := env.Parse.Wikitext.Star
	if wikitext == "" {
		slog.Warn(wikiJobName+": wiki empty wikitext", "url", url)
		return wikiPageResult{}, fetchSkip
	}
	// env.Parse.Title is the redirect-resolved canonical title; ParseItempage
	// uses it for the wiki_url. An empty title is acceptable (the parser falls
	// back internally), but in practice the API always returns it on a 200.
	return wikiPageResult{
		wikitext:     wikitext,
		title:        env.Parse.Title,
		etag:         res.ETag,
		lastModified: res.LastModified,
	}, fetchGotPage
}
