// Phase 10 plan 10-02 — Admin-Mgmt sidebar inline-JS tests (TEST-03).
//
// Closes the v1.0.1 TEST-02 deferral note: 5/5 shipping sidebars now have
// inline-JS coverage (Search, Eviction, Bank-Coin, Char-Info from v1.0.1;
// Admin-Mgmt added in v1.0.2).
//
// Coverage per Phase 10 CONTEXT.md D-03 (2-test pattern locked, mirroring
// the 4 existing inline-JS sidebars):
//   TM1 — happy path: user fills #addInput, clicks #addBtn, addAdmin
//         succeeds -> success message renders in #msg + input is cleared.
//   TM2 — error path: addAdmin fails with 'invalid_email' -> routeError
//         renders 'Invalid email: ...' in #msg + input is NOT cleared.
//
// Admin-gate coverage (server-side requireAdminOrThrow path) is in
// adminMgmtSidebar.test.ts TS2 + admin.test.ts — NOT duplicated here.
// Remove-button + owner-floor-lockout flows are deferred to v1.1 per
// CONTEXT.md D-03 (2-test limit).
//
// Consumes the IIFE-fixed mountSidebar from Plan 10-01 (CONTEXT.md D-02)
// so top-level `var state` and `function escapeHtml` declarations in the
// sidebar inline script stay scoped to the IIFE.

import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showAdminMgmtSidebar';

describe('showAdminMgmtSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '3'],
      ['theme', 'sheets-default'],
    ]);
  });

  // --- TM1 — D-03 happy path ------------------------------------------

  it('TM1 — successful addAdmin renders success copy in #msg and clears #addInput', () => {
    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);

    // init() fires getAdminList immediately on mount — drain it.
    m.dispatchRunCall('getAdminList', {
      admins: ['existing@example.com'],
      floor: 'owner@example.com',
      callerEmail: 'owner@example.com',
    });

    // User fills the email input and clicks Add.
    const addInput = m.document.getElementById('addInput') as HTMLInputElement;
    addInput.value = 'newadmin@example.com';
    (m.document.getElementById('addBtn') as HTMLButtonElement).click();

    // Drain addAdmin success.
    m.dispatchRunCall('addAdmin', { added: true });

    // The success handler enqueues a second getAdminList for the
    // refresh render — drain it (mirrors the pattern in
    // searchSidebar.inline.test.ts TS1 post Plan 10-01 WR-04 fix).
    m.dispatchRunCall('getAdminList', {
      admins: ['existing@example.com', 'newadmin@example.com'],
      floor: 'owner@example.com',
      callerEmail: 'owner@example.com',
    });

    const msg = m.document.getElementById('msg')!;
    expect(msg.textContent || msg.innerHTML).toContain('Admin added: newadmin@example.com');
    // Input cleared by inline-JS line 247 (input.value = '' on success).
    expect(addInput.value).toBe('');
  });

  // --- TM2 — D-03 error path ------------------------------------------

  it('TM2 — addAdmin failure (invalid_email) renders error copy in #msg and preserves #addInput', () => {
    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);

    // Drain initial getAdminList from init().
    m.dispatchRunCall('getAdminList', {
      admins: ['existing@example.com'],
      floor: 'owner@example.com',
      callerEmail: 'owner@example.com',
    });

    const addInput = m.document.getElementById('addInput') as HTMLInputElement;
    addInput.value = 'bademail';
    const addBtn = m.document.getElementById('addBtn') as HTMLButtonElement;
    addBtn.click();

    // Fail addAdmin — routeError matches /invalid_email/ branch.
    m.failRunCall('addAdmin', { message: 'invalid_email' });

    const msg = m.document.getElementById('msg')!;
    const text = msg.textContent || msg.innerHTML;
    expect(text).toContain('Invalid email');
    expect(text).toContain('invalid_email');
    // Input NOT cleared on failure (preserves user's typed value).
    expect(addInput.value).toBe('bademail');
    // Button re-enabled by inline-JS line 252 (btn.disabled = false).
    expect(addBtn.disabled).toBe(false);
  });
});
