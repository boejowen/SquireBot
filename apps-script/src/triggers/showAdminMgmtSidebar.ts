// showAdminMgmtSidebar — Phase 7 plan 07-02.
//
// HtmlService sidebar (ADMIN-03 implementation) for admin-allowlist
// management. 300px wide per UI-SPEC §Spacing; theme-aware via the same
// CSS-custom-property injection pattern landed in 05-03's
// showSearchSidebar; inline SIDEBAR_BODY is the SINGLE SOURCE OF TRUTH
// for the HTML/CSS/JS body (Option A — no companion .html file).
//
// Server-side public surface (1 opener + 3 google.script.run callbacks):
//   - showAdminMgmtSidebar()      — opens the panel (admin-gated; non-admin → alert + return)
//   - getAdminList()              — { admins[], floor, callerEmail }
//   - addAdmin(email)             — { added, alreadyExists? } delegates to lib/admin
//   - removeAdmin(email)          — { removed, notFound? } delegates to lib/admin
//                                   (lib enforces owner-floor lockout server-side)
//
// Security: every interpolation into the SIDEBAR_BODY inline <script>
// flows through the inline escapeHtml() helper cloned verbatim from
// showEvictionSidebar.ts:257. T-07-02-* in the threat register.
//
// Owner-floor lockout is enforced at TWO layers:
//   1. Client-side: Remove button suppressed on floor row when caller != floor
//   2. Server-side: lib/admin.removeAdmin throws 'owner_floor_protected'
// The server-side check is the security boundary; the client-side check
// is the UX (prevents confusing error toasts for the common case).
//
// Audit trail: every addAdmin/removeAdmin appends an entry to
// _meta.admin_log via lib/admin.appendAdminLogEntry (called inside the
// lib primitive's lock envelope).
//
// NOTE on imports: lib/admin.ts exports getAdminList/addAdmin/removeAdmin
// under the same names this file exports (because Apps Script global
// names = google.script.run callback names = top-level export names). To
// avoid TS name collision, the lib primitives are imported under aliases
// (libGetAdminList / libAddAdmin / libRemoveAdmin).

import { log } from '../lib/log';
import { getActiveTheme, THEMES, type Theme } from '../lib/themes';
import {
  isAdmin,
  requireAdminOrThrow,
  normalizeEmail,
  getAdminList as libGetAdminList,
  addAdmin as libAddAdmin,
  removeAdmin as libRemoveAdmin,
} from '../lib/admin';

// --- Opener -------------------------------------------------------------

export function showAdminMgmtSidebar(): void {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* sandbox quirk — empty string fail-closes below */ }

  if (!isAdmin(callerEmail)) {
    const ui = SpreadsheetApp.getUi();
    ui.alert(
      'Not authorized',
      'Only guild officers can manage admins. Contact a workbook admin if you think this is wrong.',
      ui.ButtonSet.OK,
    );
    log('warn', 'showAdminMgmtSidebar', { notAuthorized: true, callerEmail });
    return;
  }

  const themeKey = getActiveTheme();
  const theme = THEMES[themeKey];
  const html = HtmlService
    .createHtmlOutput(buildSidebarHtml(theme))
    .setTitle('SquireBot — Manage admins')
    .setWidth(300);
  SpreadsheetApp.getUi().showSidebar(html);
  log('info', 'showAdminMgmtSidebar', { theme: themeKey, callerEmail });
}

// --- Read callback ------------------------------------------------------

export function getAdminList(): { admins: string[]; floor: string; callerEmail: string } {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* fail-closed below */ }
  requireAdminOrThrow(callerEmail);
  const { admins, floor } = libGetAdminList();
  log('info', 'getAdminList', { callerEmail, count: admins.length });
  return { admins, floor, callerEmail };
}

// --- Write callbacks ----------------------------------------------------

export function addAdmin(email: string): { added: boolean; alreadyExists?: boolean } {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* fail-closed below */ }
  requireAdminOrThrow(callerEmail);
  const target = String(email ?? '');
  log('info', 'addAdmin', { email: target, callerEmail });
  return libAddAdmin(target, callerEmail);
}

export function removeAdmin(email: string): { removed: boolean; notFound?: boolean } {
  let callerEmail = '';
  try {
    callerEmail = normalizeEmail(Session.getEffectiveUser().getEmail());
  } catch (_e) { /* fail-closed below */ }
  requireAdminOrThrow(callerEmail);
  const target = String(email ?? '');
  log('info', 'removeAdmin', { email: target, callerEmail });
  return libRemoveAdmin(target, callerEmail);
}

// --- HTML rendering -----------------------------------------------------
// CLONE VERBATIM from showEvictionSidebar.ts:191-217. Same :root token
// map, same six theme tokens, same fallback inline style for sheets-default.

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
 * Inline sidebar body — single source of truth for the admin-mgmt
 * sidebar HTML/CSS/JS. Mirrors showEvictionSidebar.ts pattern: the
 * template lives in the .ts file as a String.raw constant. No companion
 * .html file is shipped in v1 (Option A — see plan 05-04 scope_notes).
 *
 * Diff vs. eviction sidebar:
 *   - Body swaps <select>+preview+single-button → <ul>+input+button.
 *   - Adds .remove-btn rule per 07-UI-SPEC §Color (transparent bg,
 *     opacity 0.7, 11px, 24px min-height, var(--bg) border, hover →
 *     opacity 1 + accent border). Subordinate to the primary Add button.
 *   - escapeHtml helper at top of <script> is verbatim from
 *     showEvictionSidebar.ts:257 (T-07-02-03 XSS hardening).
 */
const SIDEBAR_BODY = String.raw`
    <style>
      * { box-sizing: border-box; }
      body { margin: 0; font-family: var(--font-body, Arial, sans-serif); color: var(--fg, #222); background: var(--bg-row, #fff); }
      .sidebar { padding: 12px var(--space-lg) var(--space-lg); }
      h3 { margin: 0 0 var(--space-xs); font-family: var(--font-header, Arial, sans-serif); font-size: 16px; font-weight: 600; line-height: 1.3; color: var(--fg-header, #222); }
      .desc { font-size: 11px; line-height: 1.4; opacity: 0.7; margin-bottom: var(--space-md); }
      label { font-size: 11px; line-height: 1.4; display: block; margin-top: var(--space-sm); }
      input[type="text"] { font: inherit; font-size: 13px; padding: var(--space-sm); border: 1px solid var(--bg, #dadce0); border-radius: 3px; width: 100%; background: var(--bg-row, #fff); color: var(--fg, #222); margin-top: var(--space-xs); }
      input[type="text"]:focus { outline: 2px solid var(--accent-bg, #1a73e8); outline-offset: -1px; }
      ul#adminList { list-style: none; padding-left: var(--space-lg); margin: var(--space-sm) 0 var(--space-xl) 0; }
      ul#adminList li { display: flex; justify-content: space-between; align-items: center; gap: var(--space-sm); padding: var(--space-xs) 0; }
      .remove-btn { background: transparent; color: var(--fg, #222); opacity: 0.7; border: 1px solid var(--bg, #dadce0); font-size: 11px; padding: 4px 8px; border-radius: 3px; min-height: 24px; cursor: pointer; font-family: inherit; }
      .remove-btn:hover { opacity: 1; border-color: var(--accent-bg, #1a73e8); }
      button.primary { margin-top: var(--space-lg); font: inherit; font-size: 13px; padding: var(--space-sm) var(--space-md); background: var(--accent-bg, #1a73e8); color: var(--accent-fg, #fff); border: 0; border-radius: 3px; cursor: pointer; min-height: 32px; width: 100%; }
      button.primary[disabled] { opacity: 0.5; cursor: not-allowed; }
      #msg { margin-top: var(--space-md); font-size: 13px; line-height: 1.5; }
      #msg.error { color: #c00; }
      #msg.success { color: #060; }
    </style>
    <div class="sidebar">
      <h3>Manage admins</h3>
      <p class="desc">Manage who can evict guildies. The owner-floor email cannot be removed by anyone else.</p>
      <div id="listRegion">
        <label id="listHeading">Current admins:</label>
        <ul id="adminList"></ul>
      </div>
      <label for="addInput">Add admin</label>
      <input id="addInput" type="text" placeholder="email@example.com" aria-label="New admin email address" autocomplete="off" />
      <button id="addBtn" class="primary">Add admin</button>
      <div id="msg" aria-live="polite"></div>
    </div>
    <script>
      function escapeHtml(s) { const d = document.createElement('div'); d.textContent = String(s == null ? '' : s); return d.innerHTML; }
      var state = { admins: [], floor: '', callerEmail: '' };
      function setMsg(text, cls) { var m = document.getElementById('msg'); m.textContent = text; m.className = cls || ''; }
      function routeError(err) {
        var m = (err && err.message) ? String(err.message) : String(err || '');
        if (/owner_floor_protected/.test(m)) setMsg('Owner-floor protected — only the workbook owner can remove themselves. No changes were written.', 'error');
        else if (/not_authorized/.test(m)) setMsg('Not authorized — you are no longer an admin. Please close this sidebar.', 'error');
        else if (/invalid_email/.test(m)) setMsg('Invalid email: ' + m + '. No changes were written.', 'error');
        else if (/lock_busy/.test(m)) setMsg('Action failed: another admin action is in flight. Please retry. No changes were written.', 'error');
        else setMsg('Action failed: ' + m + '. No changes were written.', 'error');
      }
      function renderList(payload) {
        state = payload || { admins: [], floor: '', callerEmail: '' };
        var heading = document.getElementById('listHeading');
        heading.textContent = 'Current admins (' + state.admins.length + '):';
        var ul = document.getElementById('adminList');
        if (state.admins.length === 0) {
          ul.innerHTML = '<li style="font-size:11px;opacity:0.7;justify-content:flex-start">No admins yet. Click "Initialize Admin Allowlist (manual)" under the SquireBot menu to bootstrap.</li>';
        } else {
          ul.innerHTML = state.admins.map(function(email) {
            var isFloor = (email === state.floor);
            var showRemove = !isFloor || (state.callerEmail === state.floor);
            var annotation = isFloor ? ' (owner)' : '';
            var tooltip = isFloor ? ' title="This is the workbook owner. The owner-floor lockout protection prevents anyone else from removing this email."' : '';
            var btn = showRemove
              ? '<button class="remove-btn" aria-label="Remove admin ' + escapeHtml(email) + '" data-email="' + escapeHtml(email) + '">Remove</button>'
              : '';
            return '<li' + tooltip + '><span>' + escapeHtml(email) + escapeHtml(annotation) + '</span>' + btn + '</li>';
          }).join('');
          Array.prototype.forEach.call(ul.querySelectorAll('.remove-btn'), function(btn) {
            btn.addEventListener('click', function() { onRemove(btn.getAttribute('data-email')); });
          });
        }
        var addInput = document.getElementById('addInput');
        if (addInput) addInput.focus();
      }
      function onAdd() {
        var input = document.getElementById('addInput');
        var value = String(input.value || '').trim();
        if (!value) { setMsg('Invalid email: empty. No changes were written.', 'error'); return; }
        var btn = document.getElementById('addBtn');
        btn.disabled = true;
        google.script.run
          .withSuccessHandler(function(result) {
            btn.disabled = false;
            input.value = '';
            if (result && result.alreadyExists) setMsg('Already in list: ' + value + '.', 'success');
            else setMsg('Admin added: ' + value + '.', 'success');
            google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();
          })
          .withFailureHandler(function(err) { btn.disabled = false; routeError(err); })
          .addAdmin(value);
      }
      function onRemove(email) {
        if (!window.confirm('Remove ' + email + ' from the admin allowlist? They will no longer be able to evict members or manage admins. This is reversible by adding them back via this same sidebar.')) return;
        google.script.run
          .withSuccessHandler(function(result) {
            if (result && result.notFound) setMsg('Not found in list: ' + email + '.', 'success');
            else setMsg('Admin removed: ' + email + '.', 'success');
            google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();
          })
          .withFailureHandler(routeError)
          .removeAdmin(email);
      }
      function init() {
        document.getElementById('addBtn').addEventListener('click', onAdd);
        document.getElementById('addInput').addEventListener('keydown', function(e) { if (e.key === 'Enter') onAdd(); });
        google.script.run.withSuccessHandler(renderList).withFailureHandler(routeError).getAdminList();
      }
      if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
      else init();
    </script>`;
