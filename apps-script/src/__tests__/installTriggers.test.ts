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
      ['bank_toon_name', 'Findom'],  // Phase 5 plan 05-01
    ]));
  });

  it('creates 10 triggers from a clean slate', () => {
    installTriggers();
    // Phase 5 plan 05-02: was 8 (after 05-01 added weeklySchemaHealthcheck);
    // now 10 with weeklyStaleCharArchive + weeklyEvictionArchive.
    expect(state.triggers.length).toBe(10);
    const handlers = state.triggers.map((t) => t.handler).sort();
    expect(handlers).toEqual([
      'buildView', 'monitorCellCount', 'onChange',
      'refreshPigparse', 'refreshWikiGearTier', 'refreshWikiItems', 'refreshWikiSpells',
      'weeklyEvictionArchive', 'weeklySchemaHealthcheck', 'weeklyStaleCharArchive',
    ]);
  });

  it('is idempotent: re-running deletes existing then re-creates', () => {
    installTriggers();
    expect(state.triggers.length).toBe(10);
    installTriggers();
    expect(state.triggers.length).toBe(10);
  });

  it('does NOT delete triggers for non-SquireBot handlers', () => {
    state.triggers.push({ handler: 'someThirdPartyHandler', type: 'CLOCK' });
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'someThirdPartyHandler')).toBeDefined();
    expect(state.triggers.length).toBe(11); // 10 SquireBot + 1 third-party
  });

  it('cleans up stale Phase-3 triggers (handlers in SQUIREBOT_HANDLERS) before re-creating', () => {
    // Pretend a previous installTriggers run from the Phase-3 era left
    // exactly 4 triggers behind. Those handlers are still in
    // SQUIREBOT_HANDLERS, so they get deleted before the 10 new are created.
    state.triggers.push({ handler: 'onChange', type: 'ON_CHANGE' });
    state.triggers.push({ handler: 'buildView', type: 'CLOCK' });
    state.triggers.push({ handler: 'refreshPigparse', type: 'CLOCK' });
    state.triggers.push({ handler: 'refreshWikiItems', type: 'CLOCK' });

    installTriggers();
    expect(state.triggers.length).toBe(10);
    // No duplicates: each handler appears exactly once.
    const counts: Record<string, number> = {};
    state.triggers.forEach((t) => { counts[t.handler] = (counts[t.handler] ?? 0) + 1; });
    Object.values(counts).forEach((c) => expect(c).toBe(1));
  });

  it('applies bank-coin cell protection (defensive re-apply on every install)', () => {
    installTriggers();
    const meta = state.sheets.get('_meta')!;
    // 4 bank_coin_* + 1 bank_toon_name (Phase 5 plan 05-01) = 5 total
    expect(meta.protections?.length).toBe(5);
  });

  it('re-applies bank-coin protection idempotently', () => {
    installTriggers();
    installTriggers();
    installTriggers();
    // 4 bank_coin_* + 1 bank_toon_name (Phase 5 plan 05-01) = 5 total
    expect(state.sheets.get('_meta')!.protections?.length).toBe(5);
  });

  // Phase 5 plan 05-01: install registers weeklySchemaHealthcheck and
  // invokes protectBankToonName + hideAllSystemTabs.
  it('registers weeklySchemaHealthcheck as a SQUIREBOT_HANDLERS entry', () => {
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'weeklySchemaHealthcheck'))
      .toBeDefined();
  });

  it('protects _meta.bank_toon_name with the locked description string', () => {
    installTriggers();
    const meta = state.sheets.get('_meta')!;
    const desc = 'Edit only via SquireBot → Set Bank Coin… (sets the bank-toon name used by the bank view and search).';
    const toonProt = meta.protections?.find((p) => p.description === desc);
    expect(toonProt).toBeDefined();
    expect(toonProt!.warningOnly).toBe(true);
  });

  it('hides all _-prefixed system tabs (idempotent)', () => {
    // Seed an extra system tab + a view tab to verify selective hide.
    state.sheets.set('_char_owner', makeSheet('_char_owner', ['char_name']));
    state.sheets.set('view', makeSheet('view', ['Char']));
    installTriggers();
    expect((state.sheets.get('_meta') as never as { _hidden?: boolean })._hidden).toBe(true);
    expect((state.sheets.get('_char_owner') as never as { _hidden?: boolean })._hidden).toBe(true);
    expect((state.sheets.get('view') as never as { _hidden?: boolean })._hidden).toBeFalsy();
  });

  // Phase 5 plan 05-02: cumulative-survival assertions. Guards against
  // a buggy executor that recreates installTriggers from scratch and
  // drops 05-01's contributions.
  it('registers weeklyStaleCharArchive (05-02)', () => {
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'weeklyStaleCharArchive'))
      .toBeDefined();
  });

  it('registers weeklyEvictionArchive (05-02)', () => {
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'weeklyEvictionArchive'))
      .toBeDefined();
  });

  it('05-01 weeklySchemaHealthcheck registration survives 05-02 wiring', () => {
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'weeklySchemaHealthcheck'))
      .toBeDefined();
  });

  // Phase 5 plan 05-04: eviction sidebar adds NO new time-driven
  // trigger — its callbacks (showEvictionSidebar / commitEviction)
  // live in TRIGGER_GLOBALS (build-side) but NEVER in SQUIREBOT_HANDLERS
  // (time-trigger-side). Handler count stays at 10.
  it('eviction-sidebar callbacks do NOT appear in SQUIREBOT_HANDLERS (05-04)', () => {
    installTriggers();
    expect(state.triggers.length).toBe(10);
    const handlers = state.triggers.map((t) => t.handler);
    expect(handlers).not.toContain('showEvictionSidebar');
    expect(handlers).not.toContain('commitEviction');
    expect(handlers).not.toContain('getEvictionEmails');
    expect(handlers).not.toContain('previewEviction');
  });

  // Cumulative-survival gate (05-04): all prior Phase-5 handler entries
  // from 05-01 + 05-02 must still register after 05-04 wires the
  // eviction sidebar into the build.
  it('05-01..05-02 handler entries survive 05-04 wiring (cumulative-survival)', () => {
    installTriggers();
    const handlers = state.triggers.map((t) => t.handler);
    expect(handlers).toContain('weeklySchemaHealthcheck');  // 05-01
    expect(handlers).toContain('weeklyStaleCharArchive');   // 05-02
    expect(handlers).toContain('weeklyEvictionArchive');    // 05-02
  });
});
