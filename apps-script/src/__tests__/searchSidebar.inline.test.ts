// Phase 8 plan 08-02 — search sidebar inline-JS tests (TEST-02).
//
// Coverage:
//   TS1 — happy path: getSearchInitialData → user types → runSearch
//         resolves → results render the matched item name in #results.
//   TS2 — error path: runSearch failure renders a #results error region
//         with "Search failed" + the error.message (see
//         showSearchSidebar.ts:277 — showErr().
//
// Mounts the live SIDEBAR_BODY string (via the newly-exported
// buildSidebarHtml) into JSDOM via mountSidebar() from Plan 08-01,
// then resolves enqueued google.script.run callbacks FIFO via
// dispatchRunCall / failRunCall.
//
// The window.confirm gotcha (08-RESEARCH §Pitfalls #3) does NOT apply
// to this sidebar — only eviction calls window.confirm.

import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showSearchSidebar';

describe('showSearchSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
  });

  // --- TS1 — D-03 happy path -------------------------------------------

  it('TS1 — initial-data loads, user searches, results render the matched item', () => {
    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);

    // init() fires getSearchInitialData on mount.
    m.dispatchRunCall('getSearchInitialData', {
      chars: ['Findom'],
      slots: ['Bag'],
      recent: [],
    });

    // Confirm initial-data populated the char dropdown so we know init()
    // ran end-to-end (not just enqueued the call).
    const charSel = m.document.getElementById('charSel') as HTMLSelectElement;
    expect(charSel.innerHTML).toContain('Findom');

    // User types + clicks the Search button.
    const q = m.document.getElementById('q') as HTMLInputElement;
    q.value = 'bone';
    (m.document.getElementById('searchBtn') as HTMLButtonElement).click();

    // Resolve runSearch — one matched group with the item name.
    m.dispatchRunCall('runSearch', {
      groups: [{ itemName: 'Bone Helm', itemId: 1234, rows: [], collapsed: false }],
      suggestions: [],
      coldFill: false,
      durationMs: 12,
    });

    const results = m.document.getElementById('results')!;
    expect(results.innerHTML).toContain('Bone Helm');
  });

  // --- TS2 — D-03 error path --------------------------------------------

  it('TS2 — runSearch failure renders the error region in #results', () => {
    const html = buildSidebarHtml(null);
    const m = mountSidebar(html);

    // Resolve initial-data so the form is wired up before searchBtn click.
    m.dispatchRunCall('getSearchInitialData', { chars: [], slots: [], recent: [] });

    const q = m.document.getElementById('q') as HTMLInputElement;
    q.value = 'bone';
    (m.document.getElementById('searchBtn') as HTMLButtonElement).click();

    m.failRunCall('runSearch', { message: 'CacheService unavailable' });

    const html2 = m.document.getElementById('results')!.innerHTML;
    expect(html2).toContain('Search failed');
    expect(html2).toContain('CacheService unavailable');
  });
});
