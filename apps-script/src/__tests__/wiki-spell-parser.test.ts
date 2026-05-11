import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseClassPage } from '../lib/wiki-spell-parser';
import { normalizeSpellName } from '../lib/wiki-spell-types';

function loadFixture(name: string): string {
  const path = resolve(__dirname, `../__fixtures__/${name}.json`);
  const data = JSON.parse(readFileSync(path, 'utf8'));
  return data.parse.wikitext['*'];
}

describe('normalizeSpellName', () => {
  it('lowercase + strip spaces + strip apostrophes', () => {
    expect(normalizeSpellName('Numb the Dead')).toBe('numbthedead');
  });
  it('strips Spell: prefix', () => {
    expect(normalizeSpellName('Spell: Burst of Flame')).toBe('burstofflame');
    expect(normalizeSpellName('SPELL: Endure Cold')).toBe('endurecold');
  });
  it('trims whitespace', () => {
    expect(normalizeSpellName('  Coldlight  ')).toBe('coldlight');
  });
  it('strips hyphens + punctuation', () => {
    expect(normalizeSpellName("Sense the Dead")).toBe('sensethedead');
    expect(normalizeSpellName('Drink-of-the-Wise')).toBe('drinkofthewise');
  });
  it('preserves digits', () => {
    expect(normalizeSpellName('Bind Affinity II')).toBe('bindaffinityii');
  });
});

describe('parseClassPage — Necromancer (pure caster)', () => {
  it('parses the live fixture cleanly', () => {
    const wikitext = loadFixture('wiki-class-necromancer');
    const result = parseClassPage(wikitext, 'NEC');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.levelHeaders).toBeGreaterThanOrEqual(20);  // ~25 level sections
    expect(result.spellCount).toBeGreaterThanOrEqual(100);
    // Spot-check a known Level 1 spell
    const lvl1 = result.rows.filter((r) => r.level === 1);
    expect(lvl1.length).toBeGreaterThan(5);
    const names = lvl1.map((r) => r.spell_name);
    expect(names).toContain('Cavorting Bones');
    expect(names).toContain('Coldlight');
    expect(names).toContain('Disease Cloud');
    // Class abbrev preserved
    expect(result.rows.every((r) => r.class === 'NEC')).toBe(true);
    // Normalized names always lowercase alphanumeric
    expect(result.rows.every((r) => /^[a-z0-9]+$/.test(r.normalized_name))).toBe(true);
  });
});

describe('parseClassPage — Paladin (hybrid)', () => {
  it('parses successfully with expected counts', () => {
    const wikitext = loadFixture('wiki-class-paladin');
    const result = parseClassPage(wikitext, 'PAL');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.levelHeaders).toBeGreaterThanOrEqual(15);  // ~17
    expect(result.spellCount).toBeGreaterThanOrEqual(50);    // ~66
    expect(result.rows.every((r) => r.class === 'PAL')).toBe(true);
  });
});

describe('parseClassPage — Warrior (degenerate no-spells case)', () => {
  it('returns ok with zero rows; not an error', () => {
    const wikitext = loadFixture('wiki-class-warrior');
    const result = parseClassPage(wikitext, 'WAR');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.levelHeaders).toBe(0);
    expect(result.spellCount).toBe(0);
    expect(result.rows).toEqual([]);
  });
});

describe('parseClassPage — edge cases', () => {
  it('returns wikitext_too_short on empty input', () => {
    const r = parseClassPage('', 'NEC');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('wikitext_too_short');
  });

  it('returns wikitext_too_short on tiny input', () => {
    const r = parseClassPage('a'.repeat(100), 'NEC');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('wikitext_too_short');
  });

  it('returns ok with zero rows when wikitext has no level headers', () => {
    const wikitext = 'lorem ipsum '.repeat(50);  // long enough, no headers
    const r = parseClassPage(wikitext, 'NEC');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(0);
  });

  it('handles name= NOT in first position within SpellRow template', () => {
    const wikitext = `Class blurb. ${'x'.repeat(200)}

==Level 5==
<table>
{{SpellRow
|type=Summon
|name=Reorder-First Test
|description=Whatever
}}
{{SpellRow|description=X|school=Foo|name=Order-Last Test|mana=10}}
</table>

==Level 9==
{{SpellRow|name=Simple First Test}}
`;
    const r = parseClassPage(wikitext, 'TST');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    const names = r.rows.map((s) => s.spell_name);
    expect(names).toContain('Reorder-First Test');
    expect(names).toContain('Order-Last Test');
    expect(names).toContain('Simple First Test');
  });

  it('terminates the last Level section at a subsequent non-Level header', () => {
    const wikitext = `${'x'.repeat(200)}

==Level 1==
{{SpellRow|name=KeepThis}}

==Class Notes==
{{SpellRow|name=ShouldNotEmitBecauseInDifferentSection}}
`;
    const r = parseClassPage(wikitext, 'NEC');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    const names = r.rows.map((s) => s.spell_name);
    expect(names).toEqual(['KeepThis']);
  });
});

describe('parseClassPage — template variants (Phase 4 fix-pack)', () => {
  it('matches {{RadSpellRow}} the same as {{SpellRow}} (WIZ/DRU/SHM/RNG/SHD)', () => {
    const wikitext = `${'x'.repeat(200)}

==Level 1==
{{RadSpellRow|name=Frost Bolt|kind=Damage|targ=Single|mana=10}}
{{RadSpellRow|name=Minor Shielding|targ=Self|kind=Buff}}

==Level 4==
{{RadSpellRow|name=Gate|kind=Teleport|targ=Self|mana=70}}
`;
    const r = parseClassPage(wikitext, 'WIZ');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(3);
    const names = r.rows.map((s) => s.spell_name);
    expect(names).toContain('Frost Bolt');
    expect(names).toContain('Minor Shielding');
    expect(names).toContain('Gate');
    // Levels read from headers, not template fields.
    expect(r.rows.find((s) => s.spell_name === 'Frost Bolt')!.level).toBe(1);
    expect(r.rows.find((s) => s.spell_name === 'Gate')!.level).toBe(4);
  });

  it('matches {{RadSpellRow2}} (DRU uses this numbered variant)', () => {
    const wikitext = `${'x'.repeat(200)}

==Level 1==
{{RadSpellRow2|name=Burst of Flame|kind=Damage|targ=Single|mana=7}}
{{RadSpellRow2|name=Minor Healing|kind=Heal|targ=Single|mana=10}}

==Level 5==
{{RadSpellRow2|name=Skin like Wood|kind=Buff|targ=Self|mana=15}}
`;
    const r = parseClassPage(wikitext, 'DRU');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(3);
    expect(r.rows.find((s) => s.spell_name === 'Burst of Flame')!.level).toBe(1);
    expect(r.rows.find((s) => s.spell_name === 'Skin like Wood')!.level).toBe(5);
  });

  it('handles mixed SpellRow + RadSpellRow within same section', () => {
    const wikitext = `${'x'.repeat(200)}

==Level 1==
{{SpellRow|name=OldStyleSpell}}
{{RadSpellRow|name=NewStyleSpell}}
`;
    const r = parseClassPage(wikitext, 'CLR');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(2);
    expect(r.rows.map((s) => s.spell_name).sort()).toEqual(['NewStyleSpell', 'OldStyleSpell']);
  });

  it('Bard-style: {{Template:SongRow}} with inline level= field, no headers', () => {
    const wikitext = `${'x'.repeat(200)}
=Description=
The Bard sings songs.

=Songs=
{{Template:SongRow|name=Chant of Battle|level=1|instrument=Percussion}}
{{Template:SongRow|name=Lullaby|level=5|instrument=String}}
{{Template:SongRow
|name=Multi-line Song
|level=12
|instrument=Wind
|description=A test
}}
`;
    const r = parseClassPage(wikitext, 'BRD');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.levelHeaders).toBe(0);  // no ==Level N== headers
    expect(r.spellCount).toBe(3);
    expect(r.rows.find((s) => s.spell_name === 'Chant of Battle')!.level).toBe(1);
    expect(r.rows.find((s) => s.spell_name === 'Lullaby')!.level).toBe(5);
    expect(r.rows.find((s) => s.spell_name === 'Multi-line Song')!.level).toBe(12);
  });

  it('Bard-style without Template: prefix also matches', () => {
    const wikitext = `${'x'.repeat(200)}
=Songs=
{{SongRow|name=Plain Form Song|level=8|instrument=String}}
`;
    const r = parseClassPage(wikitext, 'BRD');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(1);
    expect(r.rows[0].spell_name).toBe('Plain Form Song');
    expect(r.rows[0].level).toBe(8);
  });

  it('Bard-style with out-of-range level is skipped (defensive)', () => {
    const wikitext = `${'x'.repeat(200)}
=Songs=
{{SongRow|name=Real Song|level=20}}
{{SongRow|name=Bogus Level|level=99}}
{{SongRow|name=Negative Level|level=0}}
`;
    const r = parseClassPage(wikitext, 'BRD');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(1);
    expect(r.rows[0].spell_name).toBe('Real Song');
  });

  it('inline-level fallback only fires when header pass produces ZERO rows', () => {
    // Mixed page with both: ==Level== headers AND a stray SongRow.
    // Header pass succeeds → fallback should not fire.
    const wikitext = `${'x'.repeat(200)}
==Level 1==
{{SpellRow|name=HeaderSpell}}
=Songs=
{{SongRow|name=ShouldNotAppear|level=10}}
`;
    const r = parseClassPage(wikitext, 'CLR');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(1);
    expect(r.rows[0].spell_name).toBe('HeaderSpell');
  });

  it('Warrior-style (no spells anywhere) still returns ok with zero rows', () => {
    const wikitext = `${'x'.repeat(200)}
=Description=
Warriors don't cast.
=Skills=
Just hit things.
`;
    const r = parseClassPage(wikitext, 'WAR');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.spellCount).toBe(0);
    expect(r.rows).toEqual([]);
  });
});
