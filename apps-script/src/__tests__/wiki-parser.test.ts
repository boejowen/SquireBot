import { describe, it, expect, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  parseItempage, pageNameToSlug, wikiUrlFor, computeSha1Hex,
} from '../lib/wiki-parser';
import { resetMocks } from './test-helpers';

function loadFixture(name: string): { wikitext: string; title: string } {
  const path = resolve(__dirname, `../__fixtures__/${name}.json`);
  const data = JSON.parse(readFileSync(path, 'utf8'));
  return {
    wikitext: data.parse.wikitext['*'],
    title: data.parse.title,
  };
}

describe('pageNameToSlug', () => {
  it('replaces spaces with underscores', () => {
    expect(pageNameToSlug('Cloth Cap')).toBe('Cloth_Cap');
  });
  it('preserves underscored input', () => {
    expect(pageNameToSlug('Cloak_of_Flames')).toBe('Cloak_of_Flames');
  });
  it('encodes apostrophes and special chars', () => {
    expect(pageNameToSlug("Lord Nagafen's Lair")).toBe("Lord_Nagafen's_Lair");
  });
});

describe('wikiUrlFor', () => {
  it('builds the canonical URL', () => {
    expect(wikiUrlFor('Cloth Cap')).toBe('https://wiki.project1999.com/Cloth_Cap');
  });
});

describe('computeSha1Hex', () => {
  beforeEach(() => { resetMocks(); });
  it('returns a 40-char hex string', () => {
    const sha = computeSha1Hex('test');
    expect(sha).toMatch(/^[0-9a-f]{40}$/);
  });
  it('is deterministic for the same input', () => {
    expect(computeSha1Hex('hello')).toBe(computeSha1Hex('hello'));
  });
});

describe('parseItempage — Cloth Cap (quest-flagged armor, all-class, with stats)', () => {
  beforeEach(() => { resetMocks(); });
  it('parses correctly', () => {
    const { wikitext, title } = loadFixture('wiki-parse-cloth-cap');
    const result = parseItempage(wikitext, title);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    const { item } = result;
    expect(item.itemname).toBe('Cloth Cap');
    expect(item.page_title).toBe('Cloth Cap');
    expect(item.wiki_url).toBe('https://wiki.project1999.com/Cloth_Cap');
    expect(item.is_quest_item).toBe(true);
    expect(item.is_lore).toBe(false);
    expect(item.is_magic).toBe(false);
    expect(item.slot).toBe('HEAD');
    expect(item.classes).toEqual(['ALL']);
    expect(item.ac).toBe(2);
    expect(item.weight).toBe(0.2);
    expect(item.wikitext_sha1).toMatch(/^[0-9a-f]{40}$/);
  });
  it('quest links include the in-game flag', () => {
    const { wikitext, title } = loadFixture('wiki-parse-cloth-cap');
    const result = parseItempage(wikitext, title);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    const flagLink = result.questLinks.find((l) => l.source === 'in_game_flag');
    expect(flagLink).toBeDefined();
    expect(flagLink!.quest_name).toBe('[in-game QUEST flag]');
  });
});

describe('parseItempage — Pearl (reagent, summary should mention Call of the Hero)', () => {
  beforeEach(() => { resetMocks(); });
  it('parses correctly with quest-link harvesting from notes', () => {
    const { wikitext, title } = loadFixture('wiki-parse-pearl');
    const result = parseItempage(wikitext, title);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    const { item, questLinks } = result;
    expect(item.itemname).toBe('Pearl');
    expect(item.is_quest_item).toBe(false); // statsblock has no QUEST ITEM
    expect(item.weight).toBe(0.1);
    expect(item.classes).toEqual(['ALL']);
    expect(item.ac).toBeNull();
    expect(item.summary).toMatch(/Call of the Hero/);
    expect(item.summary).toMatch(/Death Pact/);
    // No in_game_flag link (statsblock didn't have QUEST ITEM)
    expect(questLinks.find((l) => l.source === 'in_game_flag')).toBeUndefined();
    // notes_link entries for the 3 spell references
    const noteTargets = questLinks.filter((l) => l.source === 'notes_link').map((l) => l.quest_name);
    expect(noteTargets).toContain('Call of the Hero');
    expect(noteTargets).toContain('Death Pact');
    expect(noteTargets).toContain('Thicken Mana');
  });
});

describe('parseItempage — Cloak of Flames (Kunark all-class, MAGIC ITEM)', () => {
  beforeEach(() => { resetMocks(); });
  it('parses correctly', () => {
    const { wikitext, title } = loadFixture('wiki-parse-cloak-of-flames');
    const result = parseItempage(wikitext, title);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    const { item } = result;
    expect(item.itemname).toBe('Cloak of Flames');
    expect(item.is_magic).toBe(true);
    expect(item.is_quest_item).toBe(false);
    expect(item.is_lore).toBe(false);
    expect(item.slot).toBe('BACK');
    expect(item.ac).toBe(10);
    expect(item.classes).toEqual(['ALL']);
  });
});

describe('parseItempage — Fungus Covered Scale Tunic (LORE ITEM with effect)', () => {
  beforeEach(() => { resetMocks(); });
  it('parses correctly with multi-class restriction + Effect', () => {
    const { wikitext, title } = loadFixture('wiki-parse-fungus-covered-scale-tunic');
    const result = parseItempage(wikitext, title);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    const { item } = result;
    expect(item.itemname).toBe('Fungus Covered Scale Tunic');
    expect(item.is_lore).toBe(true);
    expect(item.is_quest_item).toBe(false);
    expect(item.slot).toBe('CHEST');
    expect(item.ac).toBe(21);
    expect(item.classes).toEqual([
      'WAR', 'CLR', 'PAL', 'RNG', 'SHD', 'DRU', 'MNK', 'BRD', 'ROG', 'SHM',
    ]);
    expect(item.effect).toContain('Fungal Regrowth');
  });
});

describe('parseItempage — edge cases', () => {
  beforeEach(() => { resetMocks(); });

  it('returns wikitext_too_short for empty input', () => {
    const r = parseItempage('', 'Empty');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('wikitext_too_short');
  });

  it('returns wikitext_too_short for tiny input', () => {
    const r = parseItempage('a'.repeat(100), 'Short');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('wikitext_too_short');
  });

  it('returns no_itempage when template absent', () => {
    const garbage = 'a'.repeat(500);
    const r = parseItempage(garbage, 'NoTpl');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('no_itempage');
  });

  it('produces deterministic SHA across re-parses of the same fixture', () => {
    const { wikitext, title } = loadFixture('wiki-parse-cloth-cap');
    const r1 = parseItempage(wikitext, title);
    const r2 = parseItempage(wikitext, title);
    expect(r1.ok && r2.ok).toBe(true);
    if (r1.ok && r2.ok) {
      expect(r1.item.wikitext_sha1).toBe(r2.item.wikitext_sha1);
    }
  });

  it('produces different SHA when wikitext changes', () => {
    const { wikitext, title } = loadFixture('wiki-parse-cloth-cap');
    const r1 = parseItempage(wikitext, title);
    const r2 = parseItempage(wikitext + ' modified', title);
    expect(r1.ok && r2.ok).toBe(true);
    if (r1.ok && r2.ok) {
      expect(r1.item.wikitext_sha1).not.toBe(r2.item.wikitext_sha1);
    }
  });
});
