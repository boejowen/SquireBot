import { describe, it, expect, beforeEach } from 'vitest';
import { installTriggers } from '../triggers/installTriggers';
import { resetMocks, type MockState } from './test-helpers';

describe('installTriggers', () => {
  let state: MockState;

  beforeEach(() => {
    state = resetMocks();
  });

  it('creates 4 triggers from a clean slate', () => {
    installTriggers();
    expect(state.triggers.length).toBe(4);
    const handlers = state.triggers.map((t) => t.handler).sort();
    expect(handlers).toEqual(['buildView', 'onChange', 'refreshPigparse', 'refreshWikiItems'].sort());
  });

  it('is idempotent: re-running deletes existing then re-creates', () => {
    installTriggers();
    expect(state.triggers.length).toBe(4);
    installTriggers();
    // Still 4 — old ones deleted before new ones created.
    expect(state.triggers.length).toBe(4);
  });

  it('does NOT delete triggers for non-SquireBot handlers', () => {
    state.triggers.push({ handler: 'someThirdPartyHandler', type: 'CLOCK' });
    installTriggers();
    expect(state.triggers.find((t) => t.handler === 'someThirdPartyHandler')).toBeDefined();
    expect(state.triggers.length).toBe(5); // 4 SquireBot + 1 third-party
  });
});
