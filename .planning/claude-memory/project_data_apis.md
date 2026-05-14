---
name: P99 wiki and PigParse expose real APIs (not scraping targets)
description: Critical finding — both external data sources have structured APIs, which dramatically simplifies the data layer and changes the right phase ordering
type: project
originSessionId: 0f5dc45a-4a2f-4d87-8a75-2502ff440f06
---
Both data sources SquireBot depends on are typed APIs, not HTML scraping targets:

- **PigParse**: REST API at `https://pigparse.azurewebsites.net/swagger/index.html`. Endpoints include `GET /api/item/getall/{server}`, `GET /api/item/getdetails/{itemid}`, `GET /api/item/getmultiple/{server}`, `POST /api/item/wiki`. Item ID is the join key, matching the inventory file's `ID` column.
- **P1999 wiki**: MediaWiki API enabled at `https://wiki.project1999.com/api.php`. `action=parse` and `action=query&prop=revisions&rvprop=content` both work. `robots.txt` blocks only AI crawlers (GPTBot, etc.) — normal API access is fine. No crawl-delay specified.

**Why:** Verified 2026-04-30 by gsd-project-researcher (Stack dimension).

**How to apply:**
- Do NOT plan or implement HTML scraping for either source — use the APIs.
- The PigParse Swagger spec defines the contract; generate or hand-roll a typed client against it. (Apps Script side: `UrlFetchApp` + JSON parse; Go side: define Go structs from the spec.)
- For the wiki, prefer `action=parse&page=...&prop=wikitext` over scraping rendered HTML — the wikitext is more stable than the rendered page.
- This changes phase ordering: the original "scrape weekly" plan becomes "schedule API calls daily/weekly via Apps Script time-driven triggers"; the work is materially smaller than HTML scraping.
