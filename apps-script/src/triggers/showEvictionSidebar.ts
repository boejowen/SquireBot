// showEvictionSidebar — Phase 5 plan 05-04 task 1.
//
// HtmlService sidebar (DOC-02 implementation) for officer-initiated
// guildie eviction. 300px wide per UI-SPEC §Spacing; theme-aware via
// the same CSS-custom-property injection pattern landed in 05-03's
// showSearchSidebar; inline SIDEBAR_BODY is the SINGLE SOURCE OF TRUTH
// for the HTML/CSS/JS body (Option A — no companion .html file).
//
// Server-side public surface (1 opener + 3 google.script.run callbacks):
//   - showEvictionSidebar()       — opens the panel
//   - getEvictionEmails()         — distinct sorted owner_email list
//                                   (filters fully-removed; partial-removal kept)
//   - previewEviction(email)      — { chars[], graceUntil ISO+30d }
//   - commitEviction(email)       — lock-guarded cascade is_removed=TRUE
//                                   + append _meta.eviction_log entry
//
// Security: every interpolation into the SIDEBAR_BODY inline <script>
// flows through the inline escapeHtml() helper (cloned verbatim from
// showCharInfoSidebar.ts:154). T-05-04-01, T-05-04-02 in the threat
// register.
//
// _meta.eviction_log envelope shape (consumed by 05-02's
// weeklyEvictionArchive after the 30-day grace):
//   {
//     at: ISO8601,
//     email: string,
//     initiated_by: string,
//     grace_until: ISO8601,
//     chars: string[],
//     reason: 'evicted',
//   }
//
// Assumption A5: Session.getEffectiveUser().getEmail() may return ''
// in some sandbox contexts. Falls back to 'unknown' — the load-bearing
// audit-log fields (at, email, chars) are still recorded.

import { log } from '../lib/log';
import { getActiveTheme, THEMES, type Theme } from '../lib/themes';
import { getActiveSpreadsheet, readMetaRows, writeMetaRow } from '../lib/sheet-helpers';

const CHAR_OWNER = '_char_owner';
const COL_CHAR_NAME = 1;
const COL_OWNER_EMAIL = 2;
const COL_IS_REMOVED = 9;
const COL_COUNT = 13;  // SCHEMA-05 frozen at 13 columns
const GRACE_MS = 30 * 24 * 60 * 60 * 1000;

export function showEvictionSidebar(): void {
  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];
  const html = HtmlService
    .createHtmlOutput(buildSidebarHtml(theme))
    .setTitle('SquireBot — Evict guildie')
    .setWidth(300);
  SpreadsheetApp.getUi().showSidebar(html);
  log('info', 'showEvictionSidebar', { theme: themeKey });
}

export function getEvictionEmails(): string[] {
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(CHAR_OWNER);
  if (!sheet) return [];
  const lastRow = sheet.getLastRow();
  if (lastRow < 2) return [];
  const values = sheet.getRange(2, 1, lastRow - 1, COL_COUNT).getValues();
  const emails = new Set<string>();
  for (const r of values) {
    const isRemoved = r[COL_IS_REMOVED - 1] === true
                   || String(r[COL_IS_REMOVED - 1]).toLowerCase() === 'true';
    if (isRemoved) continue;
    const email = String(r[COL_OWNER_EMAIL - 1] ?? '').trim();
    if (email) emails.add(email);
  }
  return Array.from(emails).sort();
}

export interface EvictionPreview {
  chars: string[];
  graceUntil: string;
}

export function previewEviction(email: string): EvictionPreview {
  if (!email || typeof email !== 'string') {
    throw new Error('previewEviction: invalid email');
  }
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(CHAR_OWNER);
  if (!sheet) {
    return { chars: [], graceUntil: new Date(Date.now() + GRACE_MS).toISOString() };
  }
  const lastRow = sheet.getLastRow();
  const chars: string[] = [];
  if (lastRow >= 2) {
    const values = sheet.getRange(2, 1, lastRow - 1, COL_COUNT).getValues();
    for (const r of values) {
      if (String(r[COL_OWNER_EMAIL - 1] ?? '').trim() !== email) continue;
      const isRemoved = r[COL_IS_REMOVED - 1] === true
                     || String(r[COL_IS_REMOVED - 1]).toLowerCase() === 'true';
      if (isRemoved) continue;
      const name = String(r[COL_CHAR_NAME - 1] ?? '').trim();
      if (name) chars.push(name);
    }
  }
  chars.sort();
  return { chars, graceUntil: new Date(Date.now() + GRACE_MS).toISOString() };
}

export interface EvictionResult {
  affected: number;
  graceUntil: string;
}

export function commitEviction(email: string): EvictionResult {
  if (!email || typeof email !== 'string') throw new Error('commitEviction: invalid email');
  const ss = getActiveSpreadsheet();
  const sheet = ss.getSheetByName(CHAR_OWNER);
  if (!sheet) throw new Error('_char_owner missing — run migrateToV3 first');

  // Canonical envelope (LockService.getDocumentLock().tryLock(30000))
  // cloned from migrations.ts:89-92 — every lock-guarded write in the
  // apps-script project uses the same shape (Pitfall P6 mitigation).
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) throw new Error('commitEviction: lock_busy');
  try {
    const lastRow = sheet.getLastRow();
    const values = lastRow > 0
      ? sheet.getRange(1, 1, lastRow, COL_COUNT).getValues()
      : [];
    const affectedChars: string[] = [];
    for (let i = 1; i < values.length; i++) {
      if (String(values[i][COL_OWNER_EMAIL - 1] ?? '').trim() !== email) continue;
      const isRemoved = values[i][COL_IS_REMOVED - 1] === true
                     || String(values[i][COL_IS_REMOVED - 1]).toLowerCase() === 'true';
      const name = String(values[i][COL_CHAR_NAME - 1] ?? '').trim();
      // Only flip FALSE→TRUE; already-removed chars are not toggled
      // (defensive — re-eviction is a no-op for already-removed rows).
      if (!isRemoved) {
        sheet.getRange(i + 1, COL_IS_REMOVED).setValue(true);
      }
      // The audit-log entry still records every matched char (insertion
      // order preserved from row scan, NOT sorted).
      if (name) affectedChars.push(name);
    }

    const graceUntil = new Date(Date.now() + GRACE_MS).toISOString();
    let initiatedBy = 'unknown';
    try {
      // Assumption A5: in some sandbox contexts Session.getEffectiveUser()
      // returns an empty email. Falls back to 'unknown' — load-bearing
      // fields (at, email, chars) are still recorded.
      const effective = Session.getEffectiveUser().getEmail();
      if (effective) initiatedBy = effective;
    } catch (_e) { /* sandbox quirk — fall through to 'unknown' */ }

    const entry = {
      at: new Date().toISOString(),
      email,
      initiated_by: initiatedBy,
      grace_until: graceUntil,
      chars: affectedChars,
      reason: 'evicted' as const,
    };
    // Append to _meta.eviction_log JSON array. Tolerates malformed
    // existing JSON (graceful fall-through to []) so a single corrupt
    // log row can't block all future evictions — T-05-04-07 mitigation.
    const meta = readMetaRows('_meta');
    const row = meta.find((r) => r.key === 'eviction_log');
    let list: unknown[] = [];
    if (row && row.value) {
      try {
        const parsed = JSON.parse(row.value);
        if (Array.isArray(parsed)) list = parsed;
      } catch (_e) {
        log('warn', 'commitEviction', { malformedExistingLog: true });
      }
    }
    list.push(entry);
    writeMetaRow('_meta', 'eviction_log', JSON.stringify(list));

    log('info', 'commitEviction', {
      email, affected: affectedChars.length, graceUntil, initiatedBy,
    });
    return { affected: affectedChars.length, graceUntil };
  } finally {
    lock.releaseLock();
  }
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

function buildSidebarHtml(theme: Theme | null): string {
  const tokens = themeStyleBlock(theme);
  // For sheets-default still emit the spacing scale + neutral fallback
  // color tokens so the body CSS has reasonable defaults.
  const fallbackVars = !theme
    ? '<style>:root { --space-xs:4px; --space-sm:8px; --space-md:12px; --space-lg:16px; --space-xl:24px; --bg:#f8f9fa; --bg-row:#fff; --fg:#222; --fg-header:#222; --accent-bg:#1a73e8; --accent-fg:#fff; --font-header:Arial,sans-serif; --font-body:Arial,sans-serif; }</style>'
    : '';
  return tokens + fallbackVars + SIDEBAR_BODY;
}

/**
 * Inline sidebar body — single source of truth for the eviction-sidebar
 * HTML/CSS/JS. Mirrors showCharInfoSidebar.ts / showSearchSidebar.ts:
 * the template lives in the .ts file as a String.raw constant. No
 * companion .html file is shipped in v1 (Option A — see plan 05-04
 * scope_notes). A future refactor to HtmlService.createTemplateFromFile
 * would extract this constant into apps-script/src/sidebars/evictionSidebar.html;
 * that move is a v1.0.x polish item.
 */
const SIDEBAR_BODY = String.raw`
    <style>
      * { box-sizing: border-box; }
      body { margin: 0; font-family: var(--font-body, Arial, sans-serif); color: var(--fg, #222); background: var(--bg-row, #fff); }
      .sidebar { padding: 12px var(--space-lg) var(--space-lg); }
      h3 { margin: 0 0 var(--space-xs); font-family: var(--font-header, Arial, sans-serif); font-size: 16px; font-weight: 600; line-height: 1.3; color: var(--fg-header, #222); }
      .desc { font-size: 11px; line-height: 1.4; opacity: 0.7; margin-bottom: var(--space-md); }
      label { font-size: 11px; line-height: 1.4; display: block; margin-top: var(--space-sm); }
      select { font: inherit; font-size: 13px; padding: var(--space-sm); border: 1px solid var(--bg, #dadce0); border-radius: 3px; width: 100%; background: var(--bg-row, #fff); color: var(--fg, #222); }
      select:focus { outline: 2px solid var(--accent-bg, #1a73e8); outline-offset: -1px; }
      .preview { margin-top: var(--space-md); font-size: 13px; line-height: 1.5; }
      .preview ul { margin: var(--space-xs) 0; padding-left: var(--space-lg); }
      .preview .grace { font-size: 11px; opacity: 0.7; margin-top: var(--space-xs); }
      button.primary { margin-top: var(--space-lg); font: inherit; font-size: 13px; padding: var(--space-sm) var(--space-md); background: var(--accent-bg, #1a73e8); color: var(--accent-fg, #fff); border: 0; border-radius: 3px; cursor: pointer; min-height: 32px; width: 100%; }
      button.primary[disabled] { opacity: 0.5; cursor: not-allowed; }
      #msg { margin-top: var(--space-md); font-size: 13px; line-height: 1.5; }
      #msg.error { color: #c00; }
      #msg.success { color: #060; }
    </style>
    <div class="sidebar">
      <h3>Evict guildie</h3>
      <p class="desc">Mark a departed guildie's characters as removed. A 30-day grace period applies before auto-archive.</p>
      <label for="emailSel">Guildie email</label>
      <select id="emailSel" aria-label="Guildie email"><option value="">Choose…</option></select>
      <div id="preview" class="preview" hidden></div>
      <button id="evictBtn" class="primary" disabled>Evict</button>
      <div id="msg" aria-live="polite"></div>
    </div>
    <script>
      function escapeHtml(s) { const d = document.createElement('div'); d.textContent = String(s == null ? '' : s); return d.innerHTML; }
      var currentEmail = null;
      var currentPreview = null;
      function init() {
        google.script.run.withSuccessHandler(function (emails) {
          var sel = document.getElementById('emailSel');
          (emails || []).forEach(function (e) {
            var o = document.createElement('option'); o.value = e; o.textContent = e; sel.appendChild(o);
          });
        }).withFailureHandler(showErr).getEvictionEmails();
        document.getElementById('emailSel').addEventListener('change', onEmailChange);
        document.getElementById('evictBtn').addEventListener('click', onEvict);
      }
      function onEmailChange() {
        var email = document.getElementById('emailSel').value;
        currentEmail = email;
        currentPreview = null;
        var msg = document.getElementById('msg');
        msg.textContent = ''; msg.className = '';
        document.getElementById('evictBtn').disabled = true;
        document.getElementById('preview').hidden = true;
        if (!email) return;
        google.script.run.withSuccessHandler(function (p) {
          currentPreview = p;
          var box = document.getElementById('preview');
          var graceStr = p.graceUntil ? new Date(p.graceUntil).toDateString() : '';
          if (!p.chars || p.chars.length === 0) {
            box.innerHTML = '<div>No characters found for this email.</div>' +
              '<div class="grace">Grace expires: ' + escapeHtml(graceStr) + ' (30 days from today).</div>';
            box.hidden = false;
            document.getElementById('evictBtn').disabled = true;
            return;
          }
          box.innerHTML = '<div><strong>Characters affected (' + p.chars.length + '):</strong></div><ul>' +
            p.chars.map(function (c) { return '<li>' + escapeHtml(c) + '</li>'; }).join('') +
            '</ul><div class="grace">Grace expires: ' + escapeHtml(graceStr) + ' (30 days from today).</div>';
          box.hidden = false;
          document.getElementById('evictBtn').disabled = false;
        }).withFailureHandler(showErr).previewEviction(email);
      }
      function onEvict() {
        if (!currentEmail || !currentPreview || !currentPreview.chars || currentPreview.chars.length === 0) return;
        var n = currentPreview.chars.length;
        var body = 'This will mark ' + n + ' character(s) as removed. They will auto-archive after 30 days. This is reversible by editing _char_owner.is_removed back to FALSE before then.\n\nEvict ' + currentEmail + '?';
        if (!window.confirm(body)) return;
        document.getElementById('evictBtn').disabled = true;
        google.script.run
          .withSuccessHandler(function (r) {
            var graceStr = r.graceUntil ? new Date(r.graceUntil).toDateString() : '';
            var msg = document.getElementById('msg');
            msg.className = 'success';
            msg.textContent = 'Marked ' + r.affected + ' character(s) as removed. Grace until ' + graceStr + '.';
          })
          .withFailureHandler(function (err) {
            document.getElementById('evictBtn').disabled = false;
            showErr(err);
          })
          .commitEviction(currentEmail);
      }
      function showErr(err) {
        var msg = document.getElementById('msg');
        msg.className = 'error';
        var detail = err && err.message ? err.message : String(err);
        msg.textContent = 'Eviction failed: ' + detail + '. No changes were written.';
      }
      init();
    </script>`;
