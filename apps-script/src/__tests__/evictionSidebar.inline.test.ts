// Phase 8 plan 08-02 — eviction sidebar inline-JS tests (TEST-02).
//
// Coverage:
//   TE1 — happy path: getEvictionEmails populates #emailSel → user
//         selects an email → onEmailChange fires previewEviction → user
//         clicks #evictBtn → window.confirm returns true (stubbed) →
//         commitEviction resolves → #msg shows the success copy.
//   TE2 — error path: getEvictionEmails failure routes to showErr()
//         which writes "Eviction failed: …" into #msg.error.
//
// Pitfalls covered:
//   - 08-RESEARCH §Pitfalls #3: showEvictionSidebar.ts:344 calls
//     window.confirm(body) before commitEviction. JSDOM's default
//     returns false; happy path MUST stub it true.
//
// Mounts the live SIDEBAR_BODY string from the newly-exported
// buildSidebarHtml into JSDOM via mountSidebar (Plan 08-01); resolves
// enqueued google.script.run callbacks FIFO via dispatchRunCall /
// failRunCall.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showEvictionSidebar';

function installSessionMock(email: string | null): void {
  (globalThis as Record<string, unknown>).Session = {
    getEffectiveUser: () => ({ getEmail: () => (email ?? '') }),
    getActiveUser: () => ({ getEmail: () => (email ?? '') }),
  };
}

describe('showEvictionSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '3'],
      ['theme', 'minimalist'],
      ['guild_admins', JSON.stringify(['officer@example.com'])],
      ['workbook_owner_floor', 'officer@example.com'],
    ]);
    installSessionMock('officer@example.com');
  });
  afterEach(() => {
    delete (globalThis as Record<string, unknown>).Session;
    vi.restoreAllMocks();
  });

  // --- TE1 — D-03 happy path -------------------------------------------
  // 08-RESEARCH Pitfalls #3 mitigation: stub window.confirm true.

  it('TE1 — emails load, preview renders, confirm + commit fires success copy', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);

    // init() enqueues getEvictionEmails on mount.
    m.dispatchRunCall('getEvictionEmails', ['guildie-a@example.com', 'guildie-b@example.com']);

    // Confirm dropdown was populated.
    const sel = m.document.getElementById('emailSel') as HTMLSelectElement;
    expect(sel.innerHTML).toContain('guildie-a@example.com');

    // Select an email — fires the 'change' listener which calls previewEviction.
    sel.value = 'guildie-a@example.com';
    sel.dispatchEvent(new Event('change'));

    // Resolve previewEviction — must include >=1 char so evictBtn enables.
    m.dispatchRunCall('previewEviction', {
      chars: ['CharA', 'CharB'],
      graceUntil: new Date('2026-06-11T00:00:00Z').toISOString(),
    });

    // evictBtn should now be enabled.
    const evictBtn = m.document.getElementById('evictBtn') as HTMLButtonElement;
    expect(evictBtn.disabled).toBe(false);

    // Click the evict button — window.confirm is stubbed true so commit fires.
    evictBtn.click();
    m.dispatchRunCall('commitEviction', {
      affected: 2,
      graceUntil: new Date('2026-06-11T00:00:00Z').toISOString(),
    });

    const msg = m.document.getElementById('msg')!;
    expect(msg.textContent || msg.innerHTML).toContain('Marked 2 character(s) as removed');
  });

  // --- TE2 — D-03 error path -------------------------------------------

  it('TE2 — getEvictionEmails failure renders error in #msg', () => {
    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);

    m.failRunCall('getEvictionEmails', { message: 'unauth' });

    const msg = m.document.getElementById('msg')!;
    expect(msg).not.toBeNull();
    const text = msg.textContent || msg.innerHTML;
    expect(text).toContain('Eviction failed');
    expect(text).toContain('unauth');
  });
});
