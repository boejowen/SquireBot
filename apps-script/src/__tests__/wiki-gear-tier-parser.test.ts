import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseGearTierPage } from '../lib/wiki-gear-tier-parser';

function loadFixture(name: string): string {
  const path = resolve(__dirname, `../__fixtures__/${name}.json`);
  const data = JSON.parse(readFileSync(path, 'utf8'));
  return data.parse.wikitext['*'];
}

describe('parseGearTierPage — Pre-Raid (with Iksar tagging)', () => {
  it('parses the live Pre-Raid fixture cleanly', () => {
    const wikitext = loadFixture('wiki-velious-preraid-gear');
    const result = parseGearTierPage(wikitext, 'Velious Pre-Raid/Group');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.classCount).toBe(14);  // 14 P99 classes
    expect(result.itemCount).toBeGreaterThan(400);  // ~480+ rows
    // Iksar item count: 4 actual {{:Iksar ...}} transclusions in the
    // current wiki snapshot (Iksar Hide Cap in Cleric + Magician,
    // Iksar Hide Manual ×2 in Magician). Earlier "24+" estimate was
    // based on a too-loose grep that matched [[Iksar]] race-page
    // wiki-links instead of item transclusions.
    expect(result.iksarCount).toBeGreaterThanOrEqual(4);
  });

  it('Iksar items get tier="Iksar" instead of "Velious Pre-Raid/Group"', () => {
    const wikitext = loadFixture('wiki-velious-preraid-gear');
    const result = parseGearTierPage(wikitext, 'Velious Pre-Raid/Group');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    const iksarRows = result.rows.filter((r) => r.tier === 'Iksar');
    expect(iksarRows.length).toBe(result.iksarCount);
    // Every Iksar-tagged row's item_name MUST start with "Iksar "
    expect(iksarRows.every((r) => r.item_name.startsWith('Iksar '))).toBe(true);
    // Conversely: NO Velious Pre-Raid/Group row should start with "Iksar "
    const preRaidRows = result.rows.filter((r) => r.tier === 'Velious Pre-Raid/Group');
    expect(preRaidRows.some((r) => r.item_name.startsWith('Iksar '))).toBe(false);
  });

  it('specific item check: Iksar Hide Cap appears on Cleric and Magician with tier=Iksar', () => {
    const wikitext = loadFixture('wiki-velious-preraid-gear');
    const result = parseGearTierPage(wikitext, 'Velious Pre-Raid/Group');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    // Iksar Hide Cap appears in 2 class sections (Cleric + Magician)
    // per current wiki snapshot. Both should be tagged tier='Iksar'.
    const ikc = result.rows.filter((r) => r.item_name === 'Iksar Hide Cap');
    expect(ikc.length).toBe(2);
    expect(ikc.every((r) => r.tier === 'Iksar')).toBe(true);
    expect(ikc.map((r) => r.class).sort()).toEqual(['CLR', 'MAG']);
  });

  it('non-Iksar item: Cloak of Flames gets regular Pre-Raid tier (if present)', () => {
    const wikitext = loadFixture('wiki-velious-preraid-gear');
    const result = parseGearTierPage(wikitext, 'Velious Pre-Raid/Group');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    // (Note: Cloak of Flames may not be on Pre-Raid; just confirm any
    // non-Iksar item is correctly tagged.)
    const sampleNonIksar = result.rows.find((r) =>
      r.tier === 'Velious Pre-Raid/Group' && !r.item_name.startsWith('Iksar ')
    );
    expect(sampleNonIksar).toBeDefined();
  });

  it('rank is 1-based and increments per slot', () => {
    const wikitext = loadFixture('wiki-velious-preraid-gear');
    const result = parseGearTierPage(wikitext, 'Velious Pre-Raid/Group');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    // Find a class+slot with multiple items
    const groupKey = (r: typeof result.rows[number]) =>
      `${r.class}|${r.tier}|${r.slot}`;
    const counts = new Map<string, number>();
    for (const r of result.rows) {
      counts.set(groupKey(r), (counts.get(groupKey(r)) ?? 0) + 1);
    }
    // Find any group with >= 2 items
    const multiGroup = [...counts.entries()].find(([, n]) => n >= 2);
    expect(multiGroup).toBeDefined();
    if (!multiGroup) return;
    const [key] = multiGroup;
    const ranks = result.rows.filter((r) => groupKey(r) === key).map((r) => r.rank);
    expect(Math.min(...ranks)).toBe(1);
    expect(ranks).toEqual([...ranks].sort((a, b) => a - b));
  });
});

describe('parseGearTierPage — Raiding (no Iksar)', () => {
  it('parses the live Raiding fixture cleanly', () => {
    const wikitext = loadFixture('wiki-velious-raiding-gear');
    const result = parseGearTierPage(wikitext, 'Velious Raiding');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.classCount).toBe(14);
    expect(result.itemCount).toBeGreaterThan(400);
    // Raiding page has zero Iksar items per RESEARCH §1a
    expect(result.iksarCount).toBe(0);
  });

  it('all Raiding rows have tier="Velious Raiding"', () => {
    const wikitext = loadFixture('wiki-velious-raiding-gear');
    const result = parseGearTierPage(wikitext, 'Velious Raiding');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.rows.every((r) => r.tier === 'Velious Raiding')).toBe(true);
  });
});

describe('parseGearTierPage — synthetic edge cases', () => {
  it('parenthetical notes stripped from item names', () => {
    const wt = `${'x'.repeat(200)}

== [[Monk]] ==
<ul><li> '''Head''' - {{:Whetstone (Worn)}}, {{:Plain Helm}} (rare)
</li></ul>
`;
    const r = parseGearTierPage(wt, 'Velious Pre-Raid/Group');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    const names = r.rows.map((x) => x.item_name);
    expect(names).toContain('Whetstone');
    expect(names).toContain('Plain Helm');
    expect(names).not.toContain('Whetstone (Worn)');
  });

  it('unknown slot label tracked in unknownSlots', () => {
    const wt = `${'x'.repeat(200)}

== [[Monk]] ==
<ul><li> '''GreaterEars''' - {{:Mystery Earring}}
</li><li> '''Head''' - {{:Real Helm}}
</li></ul>
`;
    const r = parseGearTierPage(wt, 'Velious Pre-Raid/Group');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.unknownSlots).toContain('GreaterEars');
    expect(r.unknownSlots).not.toContain('Head');  // Head is in WIKI_SLOT_TO_INV_SLOTS
    // Mystery Earring is still emitted (slot=GreaterEars, just won't match inv)
    const myst = r.rows.find((x) => x.item_name === 'Mystery Earring');
    expect(myst).toBeDefined();
    expect(myst!.slot).toBe('GreaterEars');
  });

  it('returns wikitext_too_short on empty input', () => {
    const r = parseGearTierPage('', 'Velious Pre-Raid/Group');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('wikitext_too_short');
  });

  it('returns no_class_sections when wikitext has no class headers', () => {
    const wt = `${'x'.repeat(500)}

== Generic Header ==
random content
`;
    const r = parseGearTierPage(wt, 'Velious Pre-Raid/Group');
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toBe('no_class_sections');
  });

  it('skips sections with class names not in CLASS_DISPLAY_TO_ABBREV (noise)', () => {
    const wt = `${'x'.repeat(200)}

== [[Foo Class]] ==
<ul><li> '''Head''' - {{:Should Not Emit}} </li></ul>

== [[Monk]] ==
<ul><li> '''Head''' - {{:Real Item}} </li></ul>
`;
    const r = parseGearTierPage(wt, 'Velious Pre-Raid/Group');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    const names = r.rows.map((x) => x.item_name);
    expect(names).toContain('Real Item');
    expect(names).not.toContain('Should Not Emit');
  });

  it('Iksar tagging only fires on Pre-Raid, NOT on Raiding', () => {
    const wt = `${'x'.repeat(200)}

== [[Monk]] ==
<ul><li> '''Head''' - {{:Iksar Hide Cap}} </li></ul>
`;
    const r = parseGearTierPage(wt, 'Velious Raiding');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.iksarCount).toBe(0);
    expect(r.rows[0].tier).toBe('Velious Raiding');
    expect(r.rows[0].item_name).toBe('Iksar Hide Cap');
  });

  it('handles unclosed <li> tags (some wiki sections skip closing tag before next <li>)', () => {
    const wt = `${'x'.repeat(200)}

== [[Monk]] ==
<ul><li> '''Head''' - {{:Helm A}}
<li> '''Chest''' - {{:Plate B}}
</ul>
`;
    const r = parseGearTierPage(wt, 'Velious Pre-Raid/Group');
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    const names = r.rows.map((x) => x.item_name);
    expect(names).toContain('Helm A');
    expect(names).toContain('Plate B');
  });

  it('item_id is always null (wiki transclusions do not expose IDs)', () => {
    const wikitext = loadFixture('wiki-velious-preraid-gear');
    const result = parseGearTierPage(wikitext, 'Velious Pre-Raid/Group');
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.rows.every((r) => r.item_id === null)).toBe(true);
  });
});
