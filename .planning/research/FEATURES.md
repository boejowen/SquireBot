# Feature Research

**Domain:** Project 1999 (Classic EverQuest emulator) guild-inventory tooling
**Researched:** 2026-04-30
**Confidence:** HIGH for the existing-tools landscape (every tool below was directly probed); HIGH for the table-stakes/differentiator split because the "realistic competition" is well-characterized (manual Google Sheets + EQHTML/WinEQDB); MEDIUM for v2 Discord-pinger feature shape because the prerequisite Raid Alliance access has not been validated.

---

## Existing P99 Tooling Landscape — One-Paragraph Summary

The P99 ecosystem already has multiple tools that read `<CharName>-Inventory.txt` and `<CharName>-Spellbook.txt`: **EQHTML** (VBScript -> static HTML, single-user), **WinEQDB** (Windows app, single-user, deepest feature set including missing-spell tracking), **P99 Inventory Parser** (drag-and-drop browser sortable table, multi-character but local-only, has stale 2022 prices), **P99 Companion** (Windows app, multi-character search, GINA/Discord/middleman integrations), **EQ1999-Bank** (Python, multi-character searchable, no prices, no progression), and **Magelo Import** (per-character profile published to a third-party site). The auction side has **PigParse** (price aggregator, REST API) and **TunnelQuestBot** (Discord wantlist bot for Green server). **Critical gap: zero existing tools handle guild-wide aggregation** — every inventory tool above is single-user and runs on one PC. Guilds today maintain shared-bank state in **manually-updated Google Sheets**, which is SquireBot's actual realistic competitor.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = guild abandons SquireBot for the manual sheet.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Idiot-proof Windows installer** (one click + one Google "Allow" + one Windows "Allow") | Set by PROJECT.md as a hard constraint. Mixed-technical-comfort guild — anything more complex loses 2-3 members at install. | M | Already mandated; STACK.md picks NSIS per-user + code-signing cert + loopback PKCE OAuth. Cost of getting this wrong = product fails before it starts. |
| **Inventory file watcher** (auto-detect `<Char>-Inventory.txt` writes) | EQHTML, WinEQDB, P99 Companion, EQ1999-Bank all read these files. Anything that requires manual upload-after-/outputfile is a regression. | S | `fsnotify` + 500ms debounce. Re-stat+re-read on event (ignore payload). |
| **Spellbook file watcher** (same for `-Spellbook.txt`) | Every multi-character tool above handles spellbooks. EQHTML/WinEQDB/P99 Companion all expose missing-spell views. | S | Same mechanism as inventory. |
| **Per-character inventory view** (bags/banks/equipped, browsable per char) | This is THE core feature of every existing tool. P99 Inventory Parser, P99 Companion, EQ1999-Bank, EQHTML, WinEQDB all do this. | S | Sheet rendering — one tab per character, or a filterable consolidated tab. |
| **Per-character spellbook view** (which spells the character knows) | EQHTML "Known Spells", WinEQDB "Spells tab", P99 Companion "Spellbook Search" — universal table-stake. | S | One tab per character, sourced from `-Spellbook.txt`. |
| **Cross-character item search** (find item across all chars in workbook) | EQ1999-Bank ("searchable database"), WinEQDB ("Full inventory search across all characters"), P99 Companion ("Inventory Search across all characters"), P99 Inventory Parser. The minimum bar of multi-character utility. | M | Apps Script `HtmlService` sidebar with an `=QUERY(...)` or hand-rolled JS scanner over a consolidated range. |
| **Direct wiki link on every item** | Every existing P99 tool that has been web-based (P99 Inventory Parser, Magelo Import) links to wiki.project1999.com. Guildies expect to right-click -> wiki. | S | URL is `https://wiki.project1999.com/<Item_Name_underscored>`; build at render time from item name. |
| **Item ID as the join key** (not name) | The PROJECT.md Context section calls this out; PigParse exposes `getdetails/{itemid}`. Names drift, IDs don't. | S | Already in the file format (column 3); discipline to USE it everywhere. |
| **Watcher autostarts on Windows logon** | Without this, guildies forget to launch SquireBot, the sheet goes stale, the guild stops trusting it. Silent death. | S | NSIS adds `HKCU\...\Run` entry per STACK.md. |
| **Sheet is universally accessible** (any device with a browser) | Implicit in "Google Sheet as backend." P99 Inventory Parser is browser-based; that's already a quality bar. | — | Free with the architecture. |
| **No data loss on watcher crash/restart** | Guildies will close/reboot. If returning means re-running OAuth, re-configuring EQ folder, or losing state, support burden explodes. | S | Refresh token in Credential Manager (STACK.md); EQ folder path in `%AppData%\config.json`; resume on launch. |

### Differentiators (vs. Manual Google Sheet AND vs. EQHTML/WinEQDB)

Features that justify SquireBot existing instead of "just keep using the manual sheet" or "everyone install WinEQDB."

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Guild-wide aggregation** (every guildie's data in one workbook) | THE differentiator. No existing P99 inventory tool does this — they're all single-user. The manual sheet does it but with hours of human upkeep per week. SquireBot makes it automatic. | M | Architecture is already designed for this (per-user OAuth, shared workbook). The product IS this feature. |
| **Wiki-driven gear progression checklist** (Velious Pre-Raid + Velious Raiding + Iksar tier per slot, per character) | NOTHING in the P99 ecosystem auto-checks character gear against the wiki tier pages. Players manually cross-reference today. EQProgression.com publishes BiS guides but they're static and not character-aware. | L | Apps Script weekly scrape of `Players:Velious_Pre-Raid_Gear` + `Players:Velious_Raiding_Gear` + `Iksar`. Per-char join: equipped slot vs. tier rec -> "missing" rows. Wiki tier pages are class-segmented (`/Ranger`, `/Warrior` etc.) — scrape per class. |
| **Wiki-driven spell progression checklist** (what's trainable at current level vs. what character knows) | EQHTML and WinEQDB have "missing spells" views, but they're driven from local file inspection without wiki-tier awareness. Their "missing" is "spell exists in game but I don't have it" — not "spell I can train AT MY LEVEL." | M | Apps Script scrape of class spell list pages from wiki. Filter by character level. Compare to spellbook contents. |
| **Live PigParse pricing on every inventory row** | P99 Inventory Parser had stale 2022 prices baked in. PigParse is updated continuously but lives on a separate site. SquireBot puts current EC bazaar prices in-sheet, alongside ownership data. Nothing else combines those. | M | Daily `GET /api/item/getall/1` -> price-by-item-ID lookup table in a hidden tab. Tooltip composer joins it in. |
| **Item tooltips with quest indicator + summary + price** (hover on cell) | Apps Script `Range.setNote` or `=HYPERLINK` + sidebar. Combines wiki summary, "this is a quest item, used in: X, Y", and current PigParse price into one hover. No existing tool does this composite. | M | Cell-note for plaintext (limited), sidebar for rich. Refreshed when scrapes run. |
| **Shared bank cross-character search bar** | Specifically designed so a guildie can ask "does ANYONE have a Lustrous Russet Coat?" and get an answer across the entire guild's chars + bank toon — not just their own characters. Existing tools answer this for one user's characters. | M | Free once "guild-wide aggregation" + "cross-character item search" are both shipped — they compose. |
| **Manually-edited bank-toon coin field** (PP/GP/SP/CP) | The `/outputfile inventory` format does NOT include coin totals. Honest acknowledgment of the gap; one editable cell on the bank-toon tab. Manual sheets already do this; we preserve the workflow. | S | Single cell, no automation. Documented in Out of Scope as "manual is the only honest option." |
| **Quest-item awareness** (which items are quest turn-ins, with quest names) | Wiki has this in item-page categories ("Category:<Quest> Items"). Aggregating it into a tooltip is novel. Helps guildies decide "can I vendor this?" — currently a wiki-search-per-item chore. | M | Per-item wiki page parse during the weekly scrape; cache in a hidden sheet keyed by item ID. |
| **Always-current** (file watcher pushes within seconds of `/outputfile`) | Manual Google Sheet workflow is "log out, alt-tab to browser, paste, save." SquireBot is "type `/outputfile inventory`, done." The latency win is the felt experience. | — | Free with file watcher. |
| **Inventory file format is canonical** (we don't ask guildies to install client mods) | EQ Inventory Parser-style tools require nothing extra in-game. P99 Companion / WinEQDB also use these files. We follow the same convention; guildies type one already-familiar slash command. | — | Free; just discipline to never demand log parsing or client modification in v1. |

### v2 Differentiators (Discord pinger phase)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Per-user wantlist tied to Discord identity** | TunnelQuestBot does this for individuals on Green via in-game log parsing. SquireBot does it for the guild via PigParse's already-harvested EC data — no log-on-eachpipeline. The guild-wide wantlist visible in the sheet is the novel part: "who else wants this?" | M | Sheet column per guildie. Discord username field per guildie in setup. |
| **PigParse-fed EC auction monitor** (DM when wantlisted item posted) | TunnelQuestBot parses player chat logs and only works while *that user* is in EC. PigParse ingests auctions server-side; using it as the feed means notifications work even when no guildie is in EC. | M | Either Apps Script polls PigParse `/api/item/getmultiple` against wantlist on a 10-min trigger, or a tiny Cloudflare Worker bot does it (STACK.md flagged as v2 infra question). |
| **WTS monitor across three Raid Alliance Discord servers** | Nothing in P99 does this today. Guildies currently scroll WTS channels manually. Cross-server scanning is the unique value. | L | Discord bot reads designated channels with regex+item-name match. Requires admin/owner permission in those servers — PROJECT.md flags this as a prerequisite blocker. |
| **Quest-target raid monitor** (DM when raid mob tied to a wantlisted item's quest is announced) | Connects three pieces of data nothing else connects: wantlist -> quest -> raid target NPC -> external Discord raid announcement. P99 raiding scene has channels announcing target swaps; matching against a guildie's wantlist is novel. | L | Requires curated `quest -> raid target NPC` lookup table. Bot watches announcement channels for matched NPC names. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but violate Core Value, scope, or the "no separate server" constraint.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **DKP / loot council system** | Adjacent and natural-feeling — "you have inventory data, surely add DKP?" EQDKP / OpenDKP exist but are heavy. | Different problem (raid attendance + bidding + officer workflow), heavy state machine, governance-political. PROJECT.md explicitly Out of Scope. Adding it would dilute Core Value. | Stay out of it. If guild needs DKP, point at OpenDKP / EQDKP Plus. SquireBot's data is *useful input* to DKP (what gear someone has), but the systems stay separate. |
| **Real-time inventory diffing alerts** ("your Fungi just disappeared") | Sounds cool. Sounds insurance-y. | Triggers false-positive storms (loot/unloot, banking, tradeskill use). Not aligned with "what does my character still need" Core Value. PROJECT.md Out of Scope. | If desired later, build as an opt-in v3 feature with explicit allow-list of "items I care about diffs on." |
| **Magelo-style external character profiles** | Magelo Import already exists; guildies might want public char pages. | Public-web hosting violates "no separate server" constraint. Privacy footgun for a sheet that holds full bank contents. PROJECT.md Out of Scope. | If guildies want public profiles, they can use Magelo Import directly with the same files SquireBot reads. |
| **Inventory privacy tiers** ("hide my mule's contents from the guild") | Common request in any guild tool. | PROJECT.md explicitly chose universal visibility for v1; trust-rich 12-person guild. Tiers are a complexity tax. | Revisit only if a member opts out. Honest opt-out = "stop running SquireBot on that toon." |
| **Coin tracking from `/outputfile inventory`** | Guildies will ask. | File format does not contain coins. Verified against community parsers + Fanra wiki. | Manual single-cell field on bank-toon tab. Documented limitation. |
| **Mobile native app** | "I want to check inventory on my phone." | Sheet is already browser-accessible from mobile. Native app = duplicated UI surface, app-store overhead, push-notification complexity. PROJECT.md Out of Scope. | Google Sheets mobile app already covers this. |
| **Other servers (P99 Green, P99 Red, live EQ)** | Some guildies may have alts elsewhere. | Different price feeds (PigParse Green data exists; live EQ doesn't have PigParse), different wiki tier-page conventions, different raid alliances. PROJECT.md Out of Scope. | Hard "Blue only" boundary; revisit only if guild migrates. |
| **Service-account / shared-credential Google auth** | Easier for the developer. | Requires distributing a JSON key to ~12 guildies, which violates the idiot-proof setup constraint. Already rejected in Key Decisions. | Per-user OAuth with `drive.file` + Google Picker. Locked. |
| **In-game overlay / client mod** | "Show this in EverQuest itself!" | P99 strict client-mod policies; risk of getting guildies banned; massive scope creep. | Sheet stays the UI. Out of band by design. |
| **Mob timer / raid timer integration** | P99 Companion has mob timers; WinEQDB has mob timers. | Different problem domain (event-driven, log-parsing-heavy). Adjacent to v2 Discord work but not the same. | Leave to existing tools (P99 Companion, WinEQDB, dedicated raid bots). |
| **GINA trigger generation for missing spells** | EQHTML 3.0 and WinEQDB both do this. | GINA is a separate tool with its own config UX; bundling is feature creep. The data we *would* generate (list of spells you don't have at your level) is exposed in the sheet — guildies who use GINA can author their own triggers from it. | Sheet exposes the underlying "missing spells" list. Guildies can copy-paste into GINA themselves if desired. |
| **Hotkey/social/UI file syncing** | WinEQDB does this. | Wholly different problem (file format management for client config), not inventory. | Out of scope; Out of Project. |
| **Public hosted price API** | "We could expose prices like PigParse!" | We're a *consumer* of PigParse, not a competing producer. Building an API means servers, costs, support. | Use PigParse. Be a good citizen. |
| **Auction log scraping for wantlist** (v2 alternative) | TunnelQuestBot's existing pattern. | Requires every guildie to keep `/log on` while in EC, parse logs locally, push events somewhere. Fragile, per-user infrastructure. | Use PigParse's already-aggregated EC data instead. STACK.md confirms this is feasible via REST API. |

---

## Coverage Gaps in Existing P99 Tools That SquireBot Uniquely Fills

This is the "why does this product exist" answer.

1. **Guild-wide aggregation of inventory data.** EQHTML, WinEQDB, P99 Companion, EQ1999-Bank, P99 Inventory Parser are ALL single-user. Today's guild-wide solution is a manually-maintained Google Sheet. **SquireBot is the first automated multi-user solution in the P99 ecosystem.**
2. **Character-aware gear progression checklist against wiki tier pages.** Wiki has the data (`Players:Velious_Pre-Raid_Gear`, `Players:Velious_Raiding_Gear`); EQProgression.com has BiS guides; nobody auto-joins those against your character's equipped slots. SquireBot does.
3. **Live PigParse prices joined into the inventory view.** P99 Inventory Parser had baked-in 2022 prices. PigParse itself is a separate site. SquireBot is the first place where "what I have" and "what it's worth right now" coexist.
4. **Quest-item awareness on every row.** Wiki has the categorization; manual lookup is current state of the art. SquireBot bakes it into tooltips.
5. **Wantlist that surfaces "who else wants this" across the guild** (v2). TunnelQuestBot is per-user; SquireBot's wantlist is shared by design.
6. **Cross-Discord-server WTS scanning** (v2). No tool today watches multiple raid-alliance Discord WTS channels for guild-aligned wantlists.
7. **Quest -> raid-target -> wantlist join** (v2). The connection between "I want item X" and "the mob that drops the precursor for item X is being raided RIGHT NOW in <Alliance Discord>" is novel.

---

## Feature Dependencies

```
Idiot-proof installer
    └──required-by──> Inventory watcher (no install = no watcher)
                        └──required-by──> Per-character inventory view
                                            └──required-by──> Cross-character search
                                                                └──required-by──> Shared bank cross-char search
                                            └──required-by──> Item tooltips (need rows to attach to)
                                            └──required-by──> Gear progression checklist (need equipped data)

Per-guildie OAuth
    └──required-by──> Inventory watcher (can't write to sheet otherwise)
    └──required-by──> Spellbook watcher

Spellbook watcher
    └──required-by──> Per-character spellbook view
                        └──required-by──> Spell progression checklist

Wiki scraper (weekly)
    └──required-by──> Gear progression checklist (needs Velious tier pages)
    └──required-by──> Spell progression checklist (needs class spell lists)
    └──required-by──> Item tooltips (needs item summaries + quest assoc)

PigParse client (daily)
    └──required-by──> Item tooltips (price line)
    └──required-by──> v2 EC auction monitor (live wantlist match)

Universal visibility (v1 decision)
    └──enables-simplification-of──> Cross-character search (no ACL)
    └──enables-simplification-of──> Shared bank search

v2 Discord pinger
    ├──requires──> Per-user wantlist (data to match against)
    ├──requires──> PigParse client (or expanded version) for EC monitor
    ├──requires──> Per-user Discord identity (sheet field; setup step)
    ├──requires──> Raid Alliance Discord admin invites (PROJECT.md prerequisite — currently a hard blocker)
    └──requires──> Curated quest -> raid-target lookup (manual data work for the quest-target monitor)

v2 Discord pinger ──conflicts-with──> "no separate server" v1 constraint
    (Resolution: v2 explicitly lifts the constraint; introduces always-on infra)
```

### Dependency Notes

- **Idiot-proof installer dominates.** Every other v1 feature depends on guildies actually running the watcher. Get the install wrong and the rest of the feature list is theoretical.
- **Wiki scraper feeds three different features** (gear checklist, spell checklist, tooltips). Build it once, with extensible parsing, and pay back across all three.
- **PigParse client is a small dependency for v1** (just powering tooltip price line) but a load-bearing dependency for v2 EC auction monitor — design the client with both consumers in mind.
- **v2 has TWO independent Discord prerequisites** (Raid Alliance admin access + per-user Discord identity collection). Either one missing kills v2. PROJECT.md flags admin access as currently un-negotiated.
- **Universal visibility is a feature *and* a simplification.** Phasing in privacy tiers later means rebuilding cross-character search and shared-bank search with ACL — significantly larger than building either feature initially. Treat the v1 universal-visibility decision as load-bearing.

---

## MVP Definition

### Launch With (v1) — must ship together

These are coupled. Skipping any of them breaks the value proposition.

- [ ] **Idiot-proof Windows installer + per-user Google OAuth** — entry condition for everything else.
- [ ] **Inventory file watcher** — feeds every inventory feature.
- [ ] **Spellbook file watcher** — feeds every spellbook feature.
- [ ] **Per-character inventory view** in sheet — table-stake; ships with the watcher.
- [ ] **Per-character spellbook view** in sheet — table-stake; ships with the watcher.
- [ ] **Cross-character item search** (sidebar) — table-stake for "guild" tooling.
- [ ] **Shared bank character view** (designated bank toon's full inventory + manual coin field) — table-stake.
- [ ] **Direct wiki link on every item** — table-stake; one formula per row.
- [ ] **Weekly wiki scrape** of Velious Pre-Raid + Velious Raiding + Iksar tier pages + class spell lists + per-item summaries.
- [ ] **Daily PigParse pull** (`/api/item/getall/1`).
- [ ] **Item tooltips** (cell-note + sidebar detail combining wiki summary, quest-item flag, current price).
- [ ] **Per-character gear progression checklist** — the differentiator that justifies SquireBot existing instead of EQHTML+manual sheet.
- [ ] **Per-character spell progression checklist** — same.

### Add After Validation (v1.1+) — defer if v1 is at risk

- [ ] **Code-signing certificate** if not in v1 — eliminates SmartScreen warning. Ship unsigned to start; add within ~30 days.
- [ ] **Auto-update pipeline** for the watcher (`minio/selfupdate` + GitHub Releases manifest).
- [ ] **Tradeskill skill levels per char** (WinEQDB has this; sourced from `/outputfile`'s skills file if guildies opt in).
- [ ] **"Keys" view per character** (P99 Companion has this; key items are quest-flagged in wiki).
- [ ] **Bank-toon platinum *history*** (sheet column with a "what changed" running log, manual updates only since coin isn't in file format).

### Future Consideration (v2+) — explicit deferral

- [ ] **Per-user wantlist** in sheet — gateway to all v2 Discord features.
- [ ] **EC tunnel auction monitor (PigParse-fed)** — first DM feature, lowest infra risk among v2 items.
- [ ] **WTS monitor across three Raid Alliance Discord servers** — blocked on Discord admin invites.
- [ ] **Quest-target raid monitor** — blocked on (a) Discord admin invites and (b) curated quest -> raid-target NPC lookup.
- [ ] **Inventory diffing alerts** — explicit Out of Scope per PROJECT.md, mentioned only to log the deferral.

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Idiot-proof installer | HIGH | MEDIUM | P1 |
| Per-user OAuth (loopback + drive.file) | HIGH | MEDIUM | P1 |
| Inventory file watcher | HIGH | LOW | P1 |
| Spellbook file watcher | HIGH | LOW | P1 |
| Per-character inventory view | HIGH | LOW | P1 |
| Per-character spellbook view | HIGH | LOW | P1 |
| Cross-character item search | HIGH | MEDIUM | P1 |
| Shared bank cross-char search | HIGH | MEDIUM | P1 |
| Manual coin field on bank toon | MEDIUM | LOW | P1 |
| Direct wiki links | MEDIUM | LOW | P1 |
| Item tooltips (basic: wiki summary) | HIGH | MEDIUM | P1 |
| PigParse price line in tooltips | HIGH | MEDIUM | P1 |
| Quest-item indicator in tooltips | MEDIUM | MEDIUM | P1 |
| Gear progression checklist (wiki tiers) | HIGH | HIGH | P1 |
| Spell progression checklist (wiki class lists) | HIGH | MEDIUM | P1 |
| Watcher autostart on logon | HIGH | LOW | P1 |
| Code-signing cert | MEDIUM | LOW (cost) / LOW (effort) | P1 if budget; else P2 |
| Auto-update pipeline | MEDIUM | MEDIUM | P2 |
| Tradeskill skill view | LOW | LOW | P2 |
| Keys view | LOW | LOW | P2 |
| Per-user wantlist (data only) | MEDIUM | LOW | P2 |
| EC tunnel monitor (Discord DM) | HIGH | MEDIUM | P3 (v2) |
| WTS monitor across alliances | HIGH | HIGH | P3 (v2, blocked) |
| Quest-target raid monitor | HIGH | HIGH | P3 (v2, blocked) |
| Inventory diffing alerts | LOW | MEDIUM | NEVER (anti-feature) |
| DKP system | n/a | HIGH | NEVER (anti-feature) |
| Mobile native app | LOW | HIGH | NEVER (anti-feature) |
| Privacy tiers | LOW (v1) | HIGH | NEVER in v1; revisit on opt-out |

**Priority key:**
- **P1**: Must ship for v1 launch — without these, the product doesn't deliver Core Value.
- **P2**: Should ship after v1 stabilizes — fills out the "polished v1" experience.
- **P3**: v2 scope — requires net-new infrastructure or external negotiation.
- **NEVER**: Documented anti-features — explicit deferral with reasoning.

---

## Competitor Feature Analysis

| Feature | EQHTML | WinEQDB | P99 Inv. Parser | EQ1999-Bank | P99 Companion | Manual Google Sheet | TunnelQuestBot | **SquireBot** |
|---------|--------|---------|-----------------|-------------|---------------|---------------------|----------------|---------------|
| Multi-character | yes | yes | yes | yes | yes | yes (manual) | per-user | **yes (multi-user)** |
| **Multi-USER (guild-wide)** | NO | NO | NO | NO | NO | yes (manual upkeep) | NO | **YES (automated)** |
| Inventory view | static HTML | tabbed UI | sortable table | search list | search UI | freeform cells | per-char tabs | **rendered tabs + sidebar** |
| Spellbook view | yes | yes | no | no | yes | rare | no | **yes** |
| Spell missing list | yes (game-wide) | yes (game-wide) | no | no | yes (game-wide) | no | no | **yes (level-aware via wiki)** |
| Gear progression vs. wiki tiers | NO | NO | NO | NO | NO | manual lookup | NO | **YES (auto-joined Velious Pre-Raid + Raiding + Iksar)** |
| Live prices | no | no | stale 2022 | no | no | manual | n/a | **yes (PigParse daily)** |
| Cross-char item search | no | yes (local) | yes (local) | yes (local) | yes (local) | filter manually | no | **yes (guild-wide)** |
| Quest-item awareness | no | no | no | no | no | manual | no | **yes (wiki categories)** |
| Wiki link per item | no | no | yes | no | yes (dbl-click) | manual | no | **yes** |
| Auto-refresh on `/outputfile` | no (re-run) | no (re-run) | drag-drop | re-run | re-run | manual | n/a | **yes (file watcher)** |
| Coin tracking | no | yes (manual?) | no | no | no | manual | no | **manual on bank toon** |
| Wantlist + auction DM | no | no | no | no | no | no | yes (per-user, log-fed) | **v2: guild-wide, PigParse-fed** |
| Cross-Alliance Discord WTS | no | no | no | no | no | no | no | **v2: yes** |
| Quest-target raid alerts | no | no | no | no | no | no | no | **v2: yes** |
| Setup complexity | medium (VBScript) | medium (Windows app) | trivial (browser) | medium (Python) | medium (Windows app) | high (per guildie human upkeep) | medium (Discord + log-on) | **low (one installer, one OAuth)** |
| Hosting model | local-only | local-only | local-only | local-only | local-only | Google | Discord-hosted bot | **local watcher + Google + Apps Script** |

**Competitive read:** SquireBot loses on raw single-user inventory feature *count* to WinEQDB (which has years of accreted features: hotkeys, biographies, mob timers, etc.). SquireBot wins on every axis the **guild** cares about: shared visibility, automation, progression checklists, and price awareness. The realistic competition is the manual Google Sheet — which loses on automation, freshness, and wiki-driven checklists.

---

## Sources

- [EQHTML on P99 Wiki](https://wiki.project1999.com/EQHTML) — directly fetched; full feature list extracted
- [WinEQDB on P99 Wiki](https://wiki.project1999.com/WinEQDB) — directly fetched; tab-by-tab feature breakdown
- [P99 Inventory Parser (Eklorin / D. Churchill, Memorial U Newfoundland)](https://www.cs.mun.ca/~dchurchill/eq/inventory/) — directly fetched
- [P99 Companion features page](https://windmillhelix.github.io/P99Companion/features.html) — directly fetched
- [EQ1999-Bank on GitHub (Chriscrpntr)](https://github.com/Chriscrpntr/EQ1999-Bank) — directly fetched; minimal docs but functionality confirmed
- [P99 Inventory Management Website forum thread (Serendib)](https://www.project1999.com/forums/showthread.php?t=400761) — directly fetched
- [Sharing the love - simple inventory assistance script (parseinventory.py)](https://www.project1999.com/forums/showthread.php?t=199503) — directly fetched
- [Magelo Import on P99 Wiki](https://wiki.project1999.com/Magelo_Import) — directly fetched
- [TunnelQuestBot on GitHub (jamesjamail)](https://github.com/jamesjamail/TunnelQuestBot) and [forum thread](https://www.project1999.com/forums/showthread.php?t=349892) — directly fetched
- [PigParse home (P99 Tunnel Prices)](https://pigparse.azurewebsites.net/) — referenced in STACK.md
- [Players:Velious Pre-Raid Gear](https://wiki.project1999.com/Players:Velious_Pre-Raid_Gear) — confirmed structure (per-class subpages, slot-by-slot recommendations)
- [Players:Velious Raiding Gear](https://wiki.project1999.com/Players:Velious_Raiding_Gear) — confirmed "good" through "best in slot" tier structure
- [Velious Armor Checklist](https://wiki.project1999.com/Velious_Armor_Checklist), [Velious Priest Armor Checklist](https://wiki.project1999.com/Velious_Priest_Armor_Checklist), [Velious Melee Armor Checklist](https://wiki.project1999.com/Velious_Melee_Armor_Checklist) — wiki has multiple checklist pages we can scrape
- [EQEmu CharBrowser](https://github.com/maudigan/charbrowser) — adjacent (server-side magelo clone for EQEmu); not P99-applicable
- [P99 Raid History](https://wiki.project1999.com/Raid_History) — used to understand raid alliance landscape for v2 context
- [Raid guild loot systems other than DKP? (P99 forum)](https://www.project1999.com/forums/showthread.php?t=257351) — used to confirm DKP space is well-covered by EQDKP/OpenDKP and is correctly Out of Scope

---
*Feature research for: SquireBot (P99 guild-inventory tool)*
*Researched: 2026-04-30*
