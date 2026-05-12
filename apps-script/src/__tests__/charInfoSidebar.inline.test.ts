// Phase 8 plan 08-02 — char-info sidebar inline-JS tests (TEST-02).
//
// Coverage:
//   TC1 — happy path: getCharsForForm populates the #charBody tbody with
//         a row per character (render() in showCharInfoSidebar.ts:162),
//         each row exposing class/level/race selects + input plus the
//         char_name cell. saveBtn becomes enabled.
//   TC2 — error path: getCharsForForm failure routes to showErr() which
//         writes 'Failed to load: <err>' into #msg with red color
//         (showCharInfoSidebar.ts:180).
//
// Mounts the live SIDEBAR_BODY string from the newly-exported
// buildSidebarHtml into JSDOM via mountSidebar (Plan 08-01); resolves
// enqueued google.script.run callbacks FIFO via dispatchRunCall /
// failRunCall.

import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showCharInfoSidebar';

describe('showCharInfoSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
  });

  // --- TC1 — D-03 happy path -------------------------------------------

  it('TC1 — chars list populates #charBody table and enables saveBtn', () => {
    const html = buildSidebarHtml();
    const m = mountSidebar(html);

    m.dispatchRunCall('getCharsForForm', [
      { char_name: 'Slampeach', class: 'SHD', level: 60, race: 'IKS' },
      { char_name: 'Findom', class: 'WIZ', level: 55, race: 'HEF' },
    ]);

    const body = m.document.getElementById('charBody') as HTMLTableSectionElement;
    expect(body).not.toBeNull();
    const bodyHtml = body.innerHTML;
    // render() emits one <tr data-i="N"> per char and the char_name cell
    // is rendered verbatim via escapeHtml().
    expect(bodyHtml).toContain('Slampeach');
    expect(bodyHtml).toContain('Findom');
    expect(body.querySelectorAll('tr[data-i]').length).toBe(2);
    // saveBtn is enabled at the end of render() (showCharInfoSidebar.ts:177).
    const saveBtn = m.document.getElementById('saveBtn') as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(false);
  });

  // --- TC2 — D-03 error path -------------------------------------------

  it('TC2 — getCharsForForm failure renders #msg with the error copy', () => {
    const html = buildSidebarHtml();
    const m = mountSidebar(html);

    m.failRunCall('getCharsForForm', { message: 'cache_miss' });

    const msg = m.document.getElementById('msg')!;
    expect(msg).not.toBeNull();
    const text = msg.textContent || msg.innerHTML;
    expect(text).toContain('Failed to load');
    expect(text).toContain('cache_miss');
    // showErr sets msg.style.color = '#c00' (showCharInfoSidebar.ts:182).
    expect(msg.style.color).toMatch(/c00|rgb\(204,\s*0,\s*0\)/i);
  });
});
