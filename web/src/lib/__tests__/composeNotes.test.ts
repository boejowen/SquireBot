// Vitest for the ported tooltip composer (Phase 14 Plan 14-02 Task 3).
//
// Ported from apps-script/src/__tests__/composeNotes.test.ts — the
// content-presence cases port directly (the .toContain assertions still hold
// against the HTML output; the call signature gains itemName + wikiUrl, and
// direction is now the TEXT discriminator "0"/"1"/"2" from the read API).
// ADDED: the mandatory XSS-escaping assertions (the HIGH-severity gate).

import { describe, it, expect } from 'vitest';
import { composeItemNote, escapeHtml, safeHttpUrl, wikiUrlFor } from '../tooltip/composeNotes';

describe('escapeHtml', () => {
  it('escapes &, <, >, ", and \' (ampersand first)', () => {
    expect(escapeHtml('a & b')).toBe('a &amp; b');
    expect(escapeHtml('<img>')).toBe('&lt;img&gt;');
    expect(escapeHtml(`"x" 'y'`)).toBe('&quot;x&quot; &#39;y&#39;');
    // ampersand-first: an existing entity-looking string is encoded once, not twice
    expect(escapeHtml('&lt;')).toBe('&amp;lt;');
  });
});

describe('composeItemNote', () => {
  it('full case: summary + WTS + WTB + quest flag + notes-links + wiki link', () => {
    const html = composeItemNote(
      'Words of the Spoken',
      'https://wiki.project1999.com/Words_of_the_Spoken',
      { summary: 'A reagent used for several spells.', is_quest_item: true },
      [
        { direction: '0', a30: 4500, t30: 75 },
        { direction: '1', a30: 3200, t30: 12 },
      ],
      [
        { quest_name: 'Call of the Hero', source: 'notes_link' },
        { quest_name: 'Death Pact', source: 'notes_link' },
      ],
    );
    expect(html).toContain('Words of the Spoken');
    expect(html).toContain('A reagent used for several spells.');
    expect(html).toContain('Recent ask: 4,500pp (30d avg, 75 transactions)');
    expect(html).toContain('Buy posts: 3,200pp (30d avg, 12 transactions)');
    expect(html).toContain('Quest item: yes (in-game flag)');
    expect(html).toContain('Used in quests: Call of the Hero, Death Pact');
    // wiki anchor carries rel="noopener" + target="_blank"
    expect(html).toContain('href="https://wiki.project1999.com/Words_of_the_Spoken"');
    expect(html).toContain('rel="noopener"');
    expect(html).toContain('target="_blank"');
  });

  it('no summary: just price lines', () => {
    const html = composeItemNote('Item', '', null, [{ direction: '0', a30: 100, t30: 5 }], []);
    expect(html).toContain('Recent ask: 100pp');
    expect(html).not.toContain('Buy posts');
  });

  it('no pigparse data: emits "No recent transactions" line', () => {
    const html = composeItemNote(
      'Item',
      '',
      { summary: 'some lore text', is_quest_item: false },
      [],
      [],
    );
    expect(html).toContain('some lore text');
    expect(html).toContain('No recent transactions on PigParse.');
  });

  it('quest flag without notes-links produces just the flag line', () => {
    const html = composeItemNote('Item', '', { summary: 'X', is_quest_item: true }, [], []);
    expect(html).toContain('Quest item: yes (in-game flag)');
    expect(html).not.toContain('Used in quests');
  });

  it('notes-links without quest flag still emit the quests-used line', () => {
    const html = composeItemNote(
      'Item',
      '',
      { summary: 'X', is_quest_item: false },
      [{ direction: '0', a30: 50, t30: 2 }],
      [
        { quest_name: 'A', source: 'notes_link' },
        { quest_name: 'B', source: 'notes_link' },
      ],
    );
    expect(html).toContain('Used in quests: A, B');
    expect(html).not.toContain('Quest item: yes');
  });

  it('caps quest-links at 5', () => {
    const links = Array.from({ length: 12 }, (_, k) => ({
      quest_name: `Quest${k}`,
      source: 'notes_link' as const,
    }));
    const html = composeItemNote('Item', '', null, [], links);
    expect(html).toContain('Quest0');
    expect(html).toContain('Quest4');
    expect(html).not.toContain('Quest5'); // capped at 5
  });

  it('filters out in_game_flag links from the "used in quests" list', () => {
    const html = composeItemNote(
      'Item',
      '',
      { summary: 'X', is_quest_item: true },
      [],
      [
        { quest_name: '[in-game QUEST flag]', source: 'in_game_flag' },
        { quest_name: 'Some Quest', source: 'notes_link' },
      ],
    );
    expect(html).not.toContain('Used in quests: [in-game QUEST flag]');
    expect(html).toContain('Used in quests: Some Quest');
  });

  it('treats a BOTH ("2") direction row as both ask and buy', () => {
    const html = composeItemNote('Item', '', null, [{ direction: '2', a30: 900, t30: 4 }], []);
    expect(html).toContain('Recent ask: 900pp');
    expect(html).toContain('Buy posts: 900pp');
  });

  it('formats large prices with commas', () => {
    const html = composeItemNote('Item', '', null, [{ direction: '0', a30: 1234567, t30: 1 }], []);
    expect(html).toContain('1,234,567pp');
  });

  it('rounds fractional pp to integer', () => {
    const html = composeItemNote('Item', '', null, [{ direction: '0', a30: 1234.789, t30: 1 }], []);
    expect(html).toContain('1,235pp');
  });

  // --- SECURITY: the HIGH-severity XSS-mitigation gate (T-14.02-01) ---------

  it('escapes a malicious item name (no live <img> tag)', () => {
    const html = composeItemNote('<img src=x onerror=alert(1)>', '', null, [], []);
    expect(html).not.toContain('<img src=x'); // never a live tag
    expect(html).toContain('&lt;img'); // rendered escaped
    expect(html).not.toContain('onerror=alert(1)>'); // the closing > is escaped too
  });

  it('escapes a malicious quest name with < and &', () => {
    const html = composeItemNote('Item', '', { summary: 'X', is_quest_item: false }, [], [
      { quest_name: '<b>Tunare</b> & Co', source: 'notes_link' },
    ]);
    expect(html).not.toContain('<b>Tunare</b>');
    expect(html).toContain('&lt;b&gt;Tunare&lt;/b&gt; &amp; Co');
  });

  it('escapes a malicious wiki summary', () => {
    const html = composeItemNote('Item', '', {
      summary: '<script>alert(1)</script>',
      is_quest_item: false,
    }, [], []);
    expect(html).not.toContain('<script>');
    expect(html).toContain('&lt;script&gt;');
  });

  it('does not let a quoted (http) wiki URL break out of the href attribute', () => {
    // A hostile but http URL containing a double-quote can't break out of the attribute.
    const html = composeItemNote('Item', 'https://x/"><script>alert(1)</script>', null, [], []);
    expect(html).not.toContain('"><script>');
    expect(html).toContain('&quot;&gt;&lt;script&gt;');
  });

  it('drops a javascript: wiki URL entirely — no href rendered, scheme absent (WR-01)', () => {
    const html = composeItemNote('Item', 'javascript:alert(1)', null, [], []);
    expect(html).not.toContain('javascript:');
    expect(html).not.toContain('tooltip-wiki-link');
    expect(html).not.toContain('<a ');
    expect(html).toContain('Item'); // the name still renders, inert
  });
});

describe('safeHttpUrl', () => {
  it('passes absolute http(s) URLs through (trimmed)', () => {
    expect(safeHttpUrl('https://wiki.project1999.com/Cloak')).toBe('https://wiki.project1999.com/Cloak');
    expect(safeHttpUrl('http://example.com')).toBe('http://example.com');
    expect(safeHttpUrl('  https://x.test/y  ')).toBe('https://x.test/y');
  });

  it('rejects non-http(s) schemes and relative/protocol-relative URLs', () => {
    expect(safeHttpUrl('javascript:alert(1)')).toBe('');
    expect(safeHttpUrl('JavaScript:alert(1)')).toBe(''); // case-insensitive
    expect(safeHttpUrl('data:text/html,<script>alert(1)</script>')).toBe('');
    expect(safeHttpUrl('vbscript:msgbox(1)')).toBe('');
    expect(safeHttpUrl('//evil.example.com')).toBe('');
    expect(safeHttpUrl('/relative/path')).toBe('');
    expect(safeHttpUrl('')).toBe('');
  });
});

describe('wikiUrlFor', () => {
  it('builds a P1999 wiki URL from the item name (spaces -> underscores)', () => {
    expect(wikiUrlFor('Frozen Efreeti Boots')).toBe('https://wiki.project1999.com/Frozen_Efreeti_Boots');
    expect(wikiUrlFor('Ghoulbane')).toBe('https://wiki.project1999.com/Ghoulbane');
  });
  it('returns empty string for a blank name', () => {
    expect(wikiUrlFor('')).toBe('');
    expect(wikiUrlFor('   ')).toBe('');
  });
});
