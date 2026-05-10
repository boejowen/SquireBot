// P1999 wiki per-class spell types. Schema verified against live class
// page fixtures captured at apps-script/src/__fixtures__/wiki-class-*.json
// (Necromancer, Paladin, Warrior). See 04-RESEARCH.md §3 for full
// page-shape decoding.

export interface WikiSpellRow {
  class: string;          // 3-letter abbrev from CLASSES
  level: number;          // 1..60
  spell_name: string;     // verbatim from {{SpellRow|name=...}}
  normalized_name: string; // toLowerCase + alphanumeric-only — join key
  last_refreshed: string;  // ISO 8601
}

export type SpellParseResult =
  | {
      ok: true;
      rows: WikiSpellRow[];
      levelHeaders: number;
      spellCount: number;
    }
  | {
      ok: false;
      reason: 'wikitext_too_short' | 'page_error';
      detail?: string;
    };

// normalizeSpellName is the canonical join key for spell_check. The
// spellbook landing tab (spell:<Char>) carries human-typed spell names
// like "Endure Cold" or "spell: Burst of Flame"; the wiki returns
// "Endure Cold" or similar. Normalizing both sides to lowercase +
// alphanumeric-only collapses these variants to the same key.
//
// Edge cases handled:
//   - leading/trailing whitespace
//   - "Spell: " prefix (sometimes appears in spellbook output)
//   - apostrophes ("Numb the Dead" → "numbthedead")
//   - other punctuation, spaces, hyphens
export function normalizeSpellName(s: string): string {
  return s
    .trim()
    .toLowerCase()
    .replace(/^spell:\s*/i, '')
    .replace(/[^a-z0-9]/g, '');
}
