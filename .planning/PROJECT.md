# SquireBot

## What This Is

SquireBot is a small Windows app that every member of a ~12-person Project 1999 (Classic EverQuest emulator) guild installs on their PC. It watches the EQ folder for the tab-separated text files produced by the in-game `/outputfile inventory` and `/outputfile spellbook` commands, and pushes their contents into a single shared Google Sheet. The sheet is the real product — it joins each guildie's character data with information scraped from the [P1999 wiki](https://wiki.project1999.com/) and prices from [PigParse](https://pigparse.azurewebsites.net) to give the guild a unified view of every character's gear, spells, progression toward Velious-tier readiness, the shared bank's contents, and (in a later phase) Discord-driven alerts when a wantlisted item shows up for sale.

## Core Value

**Every guildie can answer "what does my character still need, and where in the guild is it?" without leaving the spreadsheet.** Inventory and spell data lands in the sheet automatically; progression, gaps, and prices are computed for them. If everything else fails, this must work.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — ship to validate)

### Active

<!-- Current milestone scope. v1 = core watcher + sheet. v2 = Discord pinger. -->

**v1 — Core watcher and shared sheet**

- [ ] **Idiot-proof Windows installer** for SquireBot — guildie downloads, runs, clicks "Allow" once for Windows and once for Google, and is done. No JSON keys, no manual paths, no command-line steps.
- [ ] **Per-guildie Google OAuth** — each guildie's SquireBot writes to the shared guild Google Sheet using their own Google account; the sheet is shared with each guildie's email. (Service-account model rejected as not idiot-proof.)
- [ ] **Inventory file watcher** — detects new/updated `<CharName>-Inventory.txt` files in the configured EQ folder and pushes their contents to the sheet. File format is tab-separated with five columns: `Location | Name | ID | Count | Slots`.
- [ ] **Spellbook file watcher** — same mechanism for `<CharName>-Spellbook.txt`.
- [ ] **Wiki scraper, refreshed weekly** — pulls (a) class-by-class spell lists from the P1999 wiki, (b) the "Velious Pre-Raid/Group", "Velious Raiding", and "Iksar" gear-tier pages, (c) per-item wiki pages for short summaries and quest associations.
- [ ] **PigParse price scraper, refreshed on a schedule** — pulls current EC bazaar prices keyed by item ID. Cadence to be determined in research; daily is the floor.
- [ ] **Per-character inventory view** — every guildie's characters' bags, banks, and equipped slots, browsable in the sheet.
- [ ] **Per-character spellbook view + spell progression checklist** — each character shows the spells they currently know vs. the spells their class can train at their current level (sourced from the wiki).
- [ ] **Per-character gear progression checklist** — each character shows current equipped gear vs. the recommended slots from `Velious Pre-Raid/Group` + `Velious Raiding` (and `Iksar` tier if the character is Iksar). Output reads like a "shopping list": what's missing, per slot.
- [ ] **Shared bank character view** — a designated guild bank character's full inventory is shown to everyone, plus a manually-maintained platinum/gold/silver/copper field (since `/outputfile inventory` does not include coin totals).
- [ ] **Shared bank cross-character search** — search bar that finds any item across every character in the workbook, not just the bank toon.
- [ ] **Item tooltips on every inventory row** — quest-item indicator + which quests the item is used for, P1999 wiki summary blurb, PigParse price summary.
- [ ] **Direct wiki link** on every item, opening the item's P1999 wiki page.
- [ ] **Universal visibility** — every guildie can see every character's data; no privacy tiers in v1.

**v2 — Wantlist and Discord pinger** *(deferred; see Constraints below for prerequisites)*

- [ ] **Per-user wantlist** in the sheet — each guildie marks items they'd like to buy or quest for.
- [ ] **EC tunnel auction monitor (P1999 Blue)** — when a wantlisted item is being auctioned in EC, ping the user via Discord DM. Likely fed by PigParse (already harvests EC auction data) rather than by parsing player chat logs.
- [ ] **WTS monitor across three Raid Alliance Discord servers** — Discord bot reads designated trade channels, regex-matches wantlist items, DMs the user.
- [ ] **Quest-target raid monitor across the same three Discord servers** — when a raid target tied to a wantlisted item's quest is announced as a current target, DM the user. Requires a curated `quest → raid target NPC` lookup.

### Out of Scope

- **Other servers (P99 Green, P99 Red, live EQ)** — guild plays Blue; building for one server is hard enough.
- **Mobile app** — the sheet is reachable from any browser; native mobile is unnecessary scope.
- **Inventory privacy tiers** — universal visibility was an explicit choice for v1; revisit only if a member opts out.
- **DKP / loot council systems** — adjacent problem space, not part of this product.
- **Real-time inventory diffing alerts (e.g., "your Fungi just disappeared")** — interesting but not core to the "what's missing?" Core Value.
- **Magelo-style external character profiles** — sheet is the front-end; we're not publishing public character pages.
- **Coin tracking from `/outputfile inventory`** — file format does not contain coin amounts; bank toon's plat is a manual field.
- **Service-account or shared-credentials Google auth** — incompatible with the idiot-proof setup goal.

## Context

- **Domain**: Project 1999 ("P99") is a community-run Classic EverQuest emulator. Players use the in-game `/outputfile inventory` and `/outputfile spellbook` commands to dump tab-separated text files into the EQ install folder, named `<CharName>-Inventory.txt` and `<CharName>-Spellbook.txt`.
- **Inventory file format**: tab-separated, five columns: `Location | Name | ID | Count | Slots`. The `ID` column is the canonical EQ item ID and is the right join key against wiki and PigParse data; item `Name` strings can drift but IDs are stable. The file does **not** contain coin/platinum totals.
- **Audience**: a single P99 guild, ~12 active members, mixed technical comfort. The setup ceiling is "click the installer, click Allow on a Google sheet permission, click Allow on a Windows permission." Anything more complex must be hidden.
- **Existing community tools** (for inspiration / non-overlap): EQHTML and "P99 Inventory Parser" both read these same files but render local-only views. SquireBot's differentiator is *guild-wide aggregation*, *progression checklists vs. wiki tiers*, and *price awareness*.
- **External data sources we depend on**: P1999 wiki (HTML scraping, polite cadence), PigParse (`pigparse.azurewebsites.net`, mechanism TBD in research). Both are community-run; we should be respectful citizens — cache aggressively, avoid hammering, and reach out to maintainers if/when our load becomes meaningful.
- **Sheet as primary UI**: the workbook does the heavy lifting (lookups, search, tooltips). Apps Script will likely be needed for richer tooltips and the search UX. The watcher app stays small.

## Constraints

- **Tech stack**: Watcher must be a single-binary Windows install with auto-update or trivial update. Stack TBD (Python+PyInstaller, Go, .NET, or Tauri are the front-runners) — research phase will pick.
- **Auth**: Must work end-to-end after one Google "Allow" click and one Windows "Allow" click. No JSON keys, no manual paths.
- **Backend**: Single shared Google Sheet workbook. No separate server, database, or web app in v1. (v2's Discord bot is the first piece of always-on infrastructure.)
- **External-data politeness**: Wiki refreshed weekly at most; PigParse refresh cadence TBD but conservative.
- **v2 prerequisites (must be cleared before v2 phase starts)**:
  1. **Raid Alliance bot invites** — admin/owner permission required in all three external Discord servers; user does not have this yet and will need to negotiate.
  2. **PigParse data access** — confirm whether PigParse exposes a queryable endpoint or requires HTML scraping; courtesy contact with the operator.
  3. **Per-user Discord identity** — guildies enter their Discord username/ID once so the bot knows who to DM.
- **Compatibility**: Windows-only watcher (P99 client is Windows-native; Mac/Linux players run via Wine and remain a non-goal for v1). Sheet itself is OS-agnostic.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Per-guildie Google OAuth (not service account) | Service accounts require distributing a JSON key, which violates the idiot-proof setup constraint. OAuth's one-click consent is the only model that meets the bar. | — Pending |
| Universal visibility (everyone sees everything) | Guild is small and trust-rich; tiered permissions are pure complexity tax for v1. Revisit only on opt-out request. | — Pending |
| Manual platinum entry for the bank toon | `/outputfile inventory` does not contain coin amounts; verified against community parsers and the Fanra wiki. Manual is the only honest option. | ✓ Locked by file format |
| Wiki data via weekly scrape, not curated tab | Wiki is the source of truth and shifts as the meta evolves; weekly scraping keeps things current with negligible upkeep. Item IDs are stable enough to absorb the brittleness. | — Pending |
| Discord pinger deferred to v2 | Different infrastructure (always-on bot, external-server permissions, scheduled poller) than the core watcher+sheet; shipping the core first delivers value to the guild months earlier. | — Pending |
| Sheet-as-UI rather than custom web app | Spreadsheet is already a familiar guild artifact; Apps Script handles the dynamic bits (search, tooltips); avoids needing to host anything in v1. | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-30 after initialization*
