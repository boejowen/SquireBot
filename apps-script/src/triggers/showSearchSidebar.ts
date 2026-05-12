// showSearchSidebar — Phase 5 plan 05-03 task 2.
//
// HtmlService sidebar (SEARCH-01..04, TIP-04) that lets a guildie search
// every other guildie's inventory in one place. 300px wide per UI-SPEC
// §Spacing; theme-aware (CSS custom properties injected at render time
// from the active THEMES entry — sheets-default emits no token block);
// inline SIDEBAR_BODY template is the SINGLE SOURCE OF TRUTH for the
// HTML/CSS/JS body (analog: showCharInfoSidebar.ts — no companion
// .html file ships in v1 per Option A; a future refactor to
// HtmlService.createTemplateFromFile would extract this constant).
//
// Server-side public surface (4 google.script.run callbacks):
//   - showSearchSidebar()       — opens the panel
//   - getSearchInitialData()    — chars + slots + recent (one-shot read)
//   - runSearch(q, cf, sf)      — thin wrapper over searchIndex.runSearch
//   - pushRecentSearchCall(q)   — thin wrapper over searchIndex.pushRecentSearch
//
// Security: every interpolation into the SIDEBAR_BODY inline <script>
// flows through the inline escapeHtml() helper (cloned verbatim from
// showCharInfoSidebar.ts:154). T-05-03-01..04 in the threat register.

import { log } from '../lib/log';
import { getActiveTheme, THEMES, type Theme } from '../lib/themes';
import {
  runSearch as runSearchImpl,
  pushRecentSearch as pushRecentImpl,
  getRecentSearches,
  listInventorySlots,
  type SearchResult,
} from '../lib/searchIndex';
import { getActiveSpreadsheet } from '../lib/sheet-helpers';

const CHAR_OWNER_TAB = '_char_owner';

export function showSearchSidebar(): void {
  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];
  const html = HtmlService
    .createHtmlOutput(buildSidebarHtml(theme))
    .setTitle('SquireBot — Search')
    .setWidth(300);  // UI-SPEC §Spacing locks 300px (SEARCH-01)
  SpreadsheetApp.getUi().showSidebar(html);
  log('info', 'showSearchSidebar', { theme: themeKey });
}

export interface SearchInitialData {
  chars: string[];
  slots: string[];
  recent: string[];
}

export function getSearchInitialData(): SearchInitialData {
  const ss = getActiveSpreadsheet();
  const ownerSheet = ss.getSheetByName(CHAR_OWNER_TAB);
  const invSheetNames = new Set(
    ss.getSheets()
      .map((s) => s.getName())
      .filter((n) => n.startsWith('inv:'))
      .map((n) => n.slice(4)),
  );
  const chars: string[] = [];
  if (ownerSheet) {
    const lastRow = ownerSheet.getLastRow();
    if (lastRow > 1) {
      const values = ownerSheet.getRange(2, 1, lastRow - 1, 13).getValues();
      for (const r of values) {
        // col 9 = is_removed (1-based 9, 0-based 8)
        const v = r[8];
        const isRemoved = v === true || String(v).toLowerCase() === 'true';
        if (isRemoved) continue;
        const name = String(r[0] ?? '').trim();
        if (name && invSheetNames.has(name)) chars.push(name);
      }
    }
  } else {
    for (const n of invSheetNames) chars.push(n);
  }
  chars.sort();
  return {
    chars: Array.from(new Set(chars)),
    slots: listInventorySlots(),
    recent: getRecentSearches(),
  };
}

export function runSearch(query: string, charFilter: string, slotFilter: string): SearchResult {
  return runSearchImpl(query, charFilter, slotFilter);
}

export function pushRecentSearchCall(query: string): void {
  pushRecentImpl(query);
}

// --- HTML rendering -----------------------------------------------------

function themeStyleBlock(theme: Theme | null): string {
  if (!theme) return '';  // sheets-default — emit no token block per UI-SPEC §Color
  return `
    <style>
      :root {
        --bg: ${theme.headerBg};
        --bg-row: ${theme.rowAltBg};
        --fg: ${theme.rowFg};
        --fg-header: ${theme.headerFg};
        --accent-bg: ${theme.accentBg};
        --accent-fg: ${theme.accentFg};
        --font-header: ${theme.fontFamily};
        --font-body: ${theme.fontFamily};
        --space-xs: 4px; --space-sm: 8px; --space-md: 12px; --space-lg: 16px; --space-xl: 24px;
      }
    </style>`;
}

export function buildSidebarHtml(theme: Theme | null): string {
  const tokens = themeStyleBlock(theme);
  // For sheets-default we still need the spacing scale + neutral
  // fallback color tokens so the body CSS has reasonable defaults.
  const fallbackVars = !theme
    ? '<style>:root { --space-xs:4px; --space-sm:8px; --space-md:12px; --space-lg:16px; --space-xl:24px; --bg:#f8f9fa; --bg-row:#fff; --fg:#222; --fg-header:#222; --accent-bg:#1a73e8; --accent-fg:#fff; --font-header:Arial,sans-serif; --font-body:Arial,sans-serif; }</style>'
    : '';
  return tokens + fallbackVars + SIDEBAR_BODY;
}

/**
 * Inline sidebar body — single source of truth for the search-sidebar
 * HTML/CSS/JS. Mirrors showCharInfoSidebar.ts: the template lives in
 * the .ts file as a String.raw constant. No companion .html file is
 * shipped in v1 (Option A — see plan 05-03 scope_notes). A future
 * refactor to HtmlService.createTemplateFromFile would extract this
 * constant into apps-script/src/sidebars/searchSidebar.html; that
 * move is a v1.0.x polish item.
 */
const SIDEBAR_BODY = String.raw`
    <style>
      * { box-sizing: border-box; }
      body { margin: 0; font-family: var(--font-body, Arial, sans-serif); color: var(--fg, #222); background: var(--bg-row, #fff); }
      .sidebar { padding: 12px var(--space-lg) var(--space-lg); }
      h3 { margin: 0 0 var(--space-xs); font-family: var(--font-header, Arial, sans-serif); font-size: 16px; font-weight: 600; line-height: 1.3; color: var(--fg-header, #222); }
      .desc { font-size: 11px; line-height: 1.4; color: var(--fg); opacity: 0.7; margin-bottom: var(--space-md); }
      .form { display: flex; flex-direction: column; gap: var(--space-sm); }
      .form label { font-size: 11px; line-height: 1.4; }
      .form input, .form select { font: inherit; font-size: 13px; padding: var(--space-sm); border: 1px solid var(--bg, #dadce0); border-radius: 3px; width: 100%; background: var(--bg-row, #fff); color: var(--fg, #222); }
      .form input:focus, .form select:focus { outline: 2px solid var(--accent-bg, #1a73e8); outline-offset: -1px; }
      .form button { font: inherit; font-size: 13px; padding: var(--space-sm) var(--space-md); background: var(--accent-bg, #1a73e8); color: var(--accent-fg, #fff); border: 0; border-radius: 3px; cursor: pointer; min-height: 32px; }
      .form button[disabled] { opacity: 0.6; cursor: wait; }
      .results { margin-top: var(--space-md); }
      .empty, .error, .no-results { padding: var(--space-xl) 0; font-size: 13px; }
      .error { color: #c00; }
      .group { border-top: 1px solid var(--bg, #eee); margin-top: var(--space-sm); }
      .group-header { padding: var(--space-sm) var(--space-lg); background: var(--bg, #f8f9fa); font-size: 13px; font-weight: 600; min-height: 32px; cursor: pointer; line-height: 1.4; }
      .group-header .badge { color: var(--accent-bg, #1a73e8); font-weight: 400; margin-left: var(--space-xs); }
      .group-row { padding: var(--space-sm) var(--space-lg); font-size: 13px; line-height: 1.5; }
      .group-row .meta { font-size: 11px; opacity: 0.7; line-height: 1.4; }
      .group-row a { color: var(--fg, #222); text-decoration: underline; }
      .recent { margin-top: var(--space-lg); padding-top: var(--space-md); border-top: 1px solid var(--bg, #eee); font-size: 11px; }
      .recent button { font: inherit; font-size: 11px; background: transparent; border: 0; color: var(--fg, #222); text-decoration: underline; cursor: pointer; padding: 2px var(--space-xs); }
      .recent button:hover { color: var(--accent-bg, #1a73e8); }
      .item-name { font-weight: 600; }
      .truncate { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    </style>
    <div class="sidebar">
      <h3>Search</h3>
      <p class="desc">Find items across every character's inventory.</p>
      <div class="form" id="form">
        <input id="q" type="text" placeholder="Item name…" aria-label="Search items" autofocus />
        <label for="charSel">Char</label>
        <select id="charSel"><option value="any">Any character</option></select>
        <label for="slotSel">Slot</label>
        <select id="slotSel"><option value="any">Any slot</option></select>
        <button id="searchBtn" title="Results may be up to 60 seconds stale.">Search</button>
      </div>
      <div id="results" class="results" aria-live="polite">
        <div class="empty">Type an item name to search.<br/><span class="desc">Matches every character's inventory. Cache refreshes every 60 seconds.</span></div>
      </div>
      <div id="recent" class="recent" hidden><span class="desc">Recent:</span> <span id="recentList"></span></div>
    </div>
    <script>
      function escapeHtml(s) { const d = document.createElement('div'); d.textContent = String(s == null ? '' : s); return d.innerHTML; }
      var initial = null;
      function init() {
        google.script.run.withSuccessHandler(function (d) {
          initial = d;
          var charSel = document.getElementById('charSel');
          (d.chars || []).forEach(function (c) {
            var o = document.createElement('option'); o.value = c; o.textContent = c; charSel.appendChild(o);
          });
          var slotSel = document.getElementById('slotSel');
          (d.slots || []).forEach(function (s) {
            var o = document.createElement('option'); o.value = s; o.textContent = s; slotSel.appendChild(o);
          });
          renderRecent(d.recent || []);
        }).withFailureHandler(showErr).getSearchInitialData();
        document.getElementById('searchBtn').addEventListener('click', submit);
        document.getElementById('q').addEventListener('keydown', function (e) {
          if (e.key === 'Enter') { e.preventDefault(); submit(); }
          if (e.key === 'Escape') { document.getElementById('q').value = ''; renderEmpty(); }
        });
      }
      function submit() {
        var q = document.getElementById('q').value.trim();
        if (!q) return;
        var cf = document.getElementById('charSel').value;
        var sf = document.getElementById('slotSel').value;
        setLoading(true);
        google.script.run
          .withSuccessHandler(function (r) { setLoading(false); render(r, q); google.script.run.pushRecentSearchCall(q); })
          .withFailureHandler(function (err) { setLoading(false); showErr(err); })
          .runSearch(q, cf, sf);
      }
      function setLoading(on) {
        var b = document.getElementById('searchBtn');
        b.disabled = !!on; b.textContent = on ? 'Searching…' : 'Search';
      }
      function render(r, q) {
        var box = document.getElementById('results');
        if (!r || !r.groups || r.groups.length === 0) {
          if (r && r.suggestions && r.suggestions.length > 0) {
            box.innerHTML = '<div class="no-results"><strong>No matches for "' + escapeHtml(q) + '".</strong><br/>Did you mean: ' +
              r.suggestions.map(function (s) { return '<button class="rec" data-q="' + escapeHtml(s) + '">' + escapeHtml(s) + '</button>'; }).join(', ') + '?</div>';
            Array.prototype.forEach.call(box.querySelectorAll('button.rec'), function (b) {
              b.addEventListener('click', function () { document.getElementById('q').value = b.dataset.q; submit(); });
            });
          } else {
            box.innerHTML = '<div class="no-results"><strong>No matches for "' + escapeHtml(q) + '".</strong><br/>No similar item names found. Check spelling or try a shorter term.</div>';
          }
          return;
        }
        var parts = [];
        r.groups.forEach(function (g, gi) {
          if (g.collapsed) {
            parts.push('<div class="group"><div class="group-header" data-gi="' + gi + '" aria-expanded="false">' + escapeHtml(g.itemName) + ' <span class="badge">· ' + g.rows.length + ' chars  [expand]</span></div><div class="group-body" hidden></div></div>');
          } else {
            parts.push('<div class="group"><div class="group-header" data-gi="' + gi + '" aria-expanded="true">' + escapeHtml(g.itemName) + '</div><div class="group-body">' + renderRows(g) + '</div></div>');
          }
        });
        box.innerHTML = parts.join('');
        Array.prototype.forEach.call(box.querySelectorAll('.group-header'), function (h) {
          h.addEventListener('click', function () { toggleGroup(h, r.groups[Number(h.dataset.gi)]); });
        });
      }
      function renderRows(g) {
        return g.rows.map(function (row) {
          var meta = '(no wiki entry yet)';
          if (g.wikiUrl) {
            var price = (g.pricePp == null) ? 'price unknown' : (g.pricePp + ' pp');
            meta = '<a href="' + escapeHtml(g.wikiUrl) + '" target="_blank" aria-label="Open ' + escapeHtml(g.itemName) + ' on the P1999 wiki">Wiki</a> · ' + escapeHtml(price);
          }
          var title = g.wikiSummary ? ' title="' + escapeHtml(g.wikiSummary) + '"' : '';
          return '<div class="group-row"><div class="item-name truncate"' + title + '>' + escapeHtml(row.char) + ': ' + escapeHtml(row.location) + ', count ' + Number(row.count) + '</div><div class="meta">' + meta + '</div></div>';
        }).join('');
      }
      function toggleGroup(header, g) {
        var body = header.nextElementSibling;
        var expanded = !body.hidden;
        if (expanded) {
          body.hidden = true; header.setAttribute('aria-expanded', 'false');
          header.innerHTML = escapeHtml(g.itemName) + ' <span class="badge">· ' + g.rows.length + ' chars  [expand]</span>';
        } else {
          body.hidden = false; header.setAttribute('aria-expanded', 'true');
          body.innerHTML = renderRows(g);
          header.innerHTML = escapeHtml(g.itemName) + ' <span class="badge">· ' + g.rows.length + ' chars  [collapse]</span>';
        }
      }
      function renderEmpty() {
        document.getElementById('results').innerHTML = '<div class="empty">Type an item name to search.<br/><span class="desc">Matches every character\'s inventory. Cache refreshes every 60 seconds.</span></div>';
      }
      function renderRecent(list) {
        var wrap = document.getElementById('recent');
        var span = document.getElementById('recentList');
        if (!list || list.length === 0) { wrap.hidden = true; return; }
        wrap.hidden = false;
        span.innerHTML = list.map(function (q) { return '<button class="rec" data-q="' + escapeHtml(q) + '">' + escapeHtml(q) + '</button>'; }).join(' ');
        Array.prototype.forEach.call(span.querySelectorAll('button.rec'), function (b) {
          b.addEventListener('click', function () { document.getElementById('q').value = b.dataset.q; submit(); });
        });
      }
      function showErr(err) {
        var box = document.getElementById('results');
        box.innerHTML = '<div class="error">Search failed: ' + escapeHtml(err && err.message ? err.message : err) + '. Try again or check the log.</div>';
      }
      init();
    </script>`;
