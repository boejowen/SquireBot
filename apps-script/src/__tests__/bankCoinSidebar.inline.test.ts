// Phase 8 plan 08-02 — bank-coin sidebar inline-JS tests (TEST-02).
//
// Coverage:
//   TB1 — happy path: getBankCoinForForm populates pp/gp/sp/cp inputs
//         and enables #saveBtn (populate() in showBankCoinSidebar.ts:96).
//   TB2 — error path: getBankCoinForForm failure renders '#msg' with the
//         error color + 'Failed to load: <err>' copy (showErr in
//         showBankCoinSidebar.ts:104).
//
// Mounts the live SIDEBAR_BODY string from the newly-exported
// buildSidebarHtml into JSDOM via mountSidebar (Plan 08-01); resolves
// enqueued google.script.run callbacks FIFO via dispatchRunCall /
// failRunCall.

import { describe, it, expect, beforeEach } from 'vitest';
import { resetMocks, seedMeta, mountSidebar, type MockState } from './test-helpers';
import { buildSidebarHtml } from '../triggers/showBankCoinSidebar';

describe('showBankCoinSidebar — inline JS', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [
      ['schema_version', '3'],
      ['theme', 'minimalist'],
      ['bank_toon_name', 'BankToon'],
    ]);
  });

  // --- TB1 — D-03 happy path -------------------------------------------

  it('TB1 — initial-data populates pp/gp/sp/cp inputs and enables saveBtn', () => {
    const html = buildSidebarHtml();
    const m = mountSidebar(html);

    // The inline script fires getBankCoinForForm immediately on mount.
    m.dispatchRunCall('getBankCoinForForm', { pp: 100, gp: 50, sp: 25, cp: 7 });

    const pp = m.document.getElementById('pp') as HTMLInputElement;
    const gp = m.document.getElementById('gp') as HTMLInputElement;
    const sp = m.document.getElementById('sp') as HTMLInputElement;
    const cp = m.document.getElementById('cp') as HTMLInputElement;
    // populate() writes Number-valued inputs; .value reads back as string.
    expect(pp.value).toBe('100');
    expect(gp.value).toBe('50');
    expect(sp.value).toBe('25');
    expect(cp.value).toBe('7');

    const saveBtn = m.document.getElementById('saveBtn') as HTMLButtonElement;
    expect(saveBtn.disabled).toBe(false);
  });

  // --- TB2 — D-03 error path -------------------------------------------

  it('TB2 — getBankCoinForForm failure renders #msg with the error copy', () => {
    const html = buildSidebarHtml();
    const m = mountSidebar(html);

    m.failRunCall('getBankCoinForForm', { message: 'denied' });

    const msg = m.document.getElementById('msg')!;
    expect(msg).not.toBeNull();
    const text = msg.textContent || msg.innerHTML;
    expect(text).toContain('Failed to load');
    expect(text).toContain('denied');
    // showErr sets msg.style.color = '#c00' (showBankCoinSidebar.ts:106).
    expect(msg.style.color).toMatch(/c00|rgb\(204,\s*0,\s*0\)/i);
  });
});
