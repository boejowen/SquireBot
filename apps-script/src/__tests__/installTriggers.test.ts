import { describe, it, expect, beforeEach } from 'vitest';
import { installTriggers } from '../triggers/installTriggers';
import { resetMocks, makeSheet, type MockState } from './test-helpers';

describe('installTriggers', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
    // _meta with bank_coin_* rows so protectBankCoinCells has something
    // to do — confirms the wiring is exercised on every install run.
    state.sheets.set('_meta', makeSheet('_meta', ['key', 'value'], [
      ['schema_version', '3'],
      ['bank_coin_pp', '0'], ['bank_coin_gp', '0'],
      ['bank_coin_sp', '0'], ['bank_coin_cp', '0'],
    ]));
  });

  it('creates 7 triggers from a clean slate', () => {
    installTriggers();
    expect(state.triggers.length).toBe(7);
    const handlers = state.triggers.map((t) => t.handler).sort();
    expect(handlers).toEqual([
      'buildView', 'monitorCellCount', 'onChange',
      'refreshPigparse', 'refreshWikiGearTier', 'refreshWikiItems', 'refreshWikiSpells',
    ]);
  });

  it('is idempotent: re-running deletes existing then re-creates', () => {
    installTriggers();
    expect(state.triggers.length).toBe(7);
    installTriggers();
    expect(state.triggers.length).toBe(7);
  });

  it('does NOT delete triggers for non-SquireBot handlers', () => {
    state.triggers.push({ handler: 'someThirdPartyHandler', type: 'CLOCK' });
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'someThirdPartyHandler')).toBeDefined();
    expect(state.triggers.length).toBe(8); // 7 SquireBot + 1 third-party
  });

  it('cleans up stale Phase-3 triggers (handlers in SQUIREBOT_HANDLERS) before re-creating', () => {
    // Pretend a previous installTriggers run from the Phase-3 era left
    // exactly 4 triggers behind. Those handlers are still in
    // SQUIREBOT_HANDLERS, so they get deleted before the 7 new are created.
    state.triggers.push({ handler: 'onChange', type: 'ON_CHANGE' });
    state.triggers.push({ handler: 'buildView', type: 'CLOCK' });
    state.triggers.push({ handler: 'refreshPigparse', type: 'CLOCK' });
    state.triggers.push({ handler: 'refreshWikiItems', type: 'CLOCK' });

    installTriggers();
    expect(state.triggers.length).toBe(7);
    // No duplicates: each handler appears exactly once.
    const counts: Record<string, number> = {};
    state.triggers.forEach((t) => { counts[t.handler] = (counts[t.handler] ?? 0) + 1; });
    Object.values(counts).forEach((c) => expect(c).toBe(1));
  });

  it('applies bank-coin cell protection (defensive re-apply on every install)', () => {
    installTriggers();
    const meta = state.sheets.get('_meta')!;
    expect(meta.protections?.length).toBe(4);
  });

  it('re-applies bank-coin protection idempotently', () => {
    installTriggers();
    installTriggers();
    installTriggers();
    expect(state.sheets.get('_meta')!.protections?.length).toBe(4);
  });
});
