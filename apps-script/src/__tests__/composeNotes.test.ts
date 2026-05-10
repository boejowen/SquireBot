import { describe, it, expect } from 'vitest';
import { composeItemNote } from '../tabs/composeNotes';

describe('composeItemNote', () => {
  it('full case: summary + WTS + WTB + quest flag + notes-links', () => {
    const note = composeItemNote(
      { summary: 'A reagent used for several spells.', is_quest_item: true },
      [
        { direction: 0, a30: 4500, t30: 75 },
        { direction: 1, a30: 3200, t30: 12 },
      ],
      [
        { quest_name: 'Call of the Hero', source: 'notes_link' },
        { quest_name: 'Death Pact', source: 'notes_link' },
      ],
    );
    expect(note).toContain('A reagent used for several spells.');
    expect(note).toContain('Recent ask: 4,500pp (30d avg, 75 transactions)');
    expect(note).toContain('Buy posts: 3,200pp (30d avg, 12 transactions)');
    expect(note).toContain('Quest item: yes (in-game flag)');
    expect(note).toContain('Used in quests: Call of the Hero, Death Pact');
  });

  it('no summary: just price lines + quest info', () => {
    const note = composeItemNote(
      null,
      [{ direction: 0, a30: 100, t30: 5 }],
      [],
    );
    expect(note).toContain('Recent ask: 100pp');
    expect(note).not.toContain('Buy posts');
  });

  it('no pigparse data: emits "No recent transactions" line', () => {
    const note = composeItemNote(
      { summary: 'some lore text', is_quest_item: false },
      [],
      [],
    );
    expect(note).toContain('some lore text');
    expect(note).toContain('No recent transactions on PigParse.');
  });

  it('quest flag without notes-links produces just the flag line', () => {
    const note = composeItemNote(
      { summary: 'X', is_quest_item: true },
      [],
      [],
    );
    expect(note).toContain('Quest item: yes (in-game flag)');
    expect(note).not.toContain('Used in quests');
  });

  it('notes-links without quest flag: still emits quests-used line', () => {
    const note = composeItemNote(
      { summary: 'X', is_quest_item: false },
      [{ direction: 0, a30: 50, t30: 2 }],
      [
        { quest_name: 'A', source: 'notes_link' },
        { quest_name: 'B', source: 'notes_link' },
      ],
    );
    expect(note).toContain('Used in quests: A, B');
    expect(note).not.toContain('Quest item: yes');
  });

  it('caps quest-links at 5', () => {
    const links = Array.from({ length: 12 }, (_, k) => ({
      quest_name: `Quest${k}`, source: 'notes_link' as const,
    }));
    const note = composeItemNote(null, [], links);
    expect(note).toContain('Quest0');
    expect(note).toContain('Quest4');
    expect(note).not.toContain('Quest5'); // capped at 5
  });

  it('filters out in_game_flag links from the "used in quests" list', () => {
    const note = composeItemNote(
      { summary: 'X', is_quest_item: true },
      [],
      [
        { quest_name: '[in-game QUEST flag]', source: 'in_game_flag' },
        { quest_name: 'Some Quest', source: 'notes_link' },
      ],
    );
    expect(note).not.toContain('Used in quests: [in-game QUEST flag]');
    expect(note).toContain('Used in quests: Some Quest');
  });

  it('formats large prices with commas', () => {
    const note = composeItemNote(
      null,
      [{ direction: 0, a30: 1234567, t30: 1 }],
      [],
    );
    expect(note).toContain('1,234,567pp');
  });

  it('rounds fractional pp to integer', () => {
    const note = composeItemNote(
      null,
      [{ direction: 0, a30: 1234.789, t30: 1 }],
      [],
    );
    expect(note).toContain('1,235pp');
  });
});
