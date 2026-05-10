// Pure parser for P1999 Velious gear-tier wiki pages. Algorithm verified
// against the 2 fixtures from research (Pre-Raid + Raiding pages).
//
// Page shape (per 04-RESEARCH.md §2):
//   == [[ClassName]] ==
//   <ul><li>  '''Slot'''  - {{:Item1}}, {{:Item2}}, {{:Item3}}
//   </li><li> '''OtherSlot''' - {{:Item}}
//   </li></ul>
//
// The Iksar racial tier doesn't have its own page — Iksar racial items
// are inline on the Pre-Raid page within the regular class sections.
// Identifiable by name pattern (`Iksar Hide Cap`, `Iksar Hide Leggings`,
// etc.). Parser detects via `name.startsWith('Iksar ')` on the Pre-Raid
// page only and tags those items with `tier='Iksar'` instead of
// `tier='Velious Pre-Raid/Group'` — single emit per item.

import type {
  GearTierParseResult,
  WikiGearTierRow,
  Tier,
} from './wiki-gear-tier-types';
import { CLASS_DISPLAY_TO_ABBREV, WIKI_SLOT_TO_INV_SLOTS } from './eq-constants';

const MIN_WIKITEXT_LENGTH = 200;

export function parseGearTierPage(
  wikitext: string,
  baseTier: 'Velious Pre-Raid/Group' | 'Velious Raiding',
): GearTierParseResult {
  if (typeof wikitext !== 'string' || wikitext.length < MIN_WIKITEXT_LENGTH) {
    return { ok: false, reason: 'wikitext_too_short' };
  }

  const sections = splitOnClassHeaders(wikitext);
  if (sections.length === 0) {
    return { ok: false, reason: 'no_class_sections' };
  }

  const rows: WikiGearTierRow[] = [];
  const unknownSlotsSet = new Set<string>();
  const now = new Date().toISOString();
  let iksarCount = 0;

  for (const section of sections) {
    const classAbbrev = CLASS_DISPLAY_TO_ABBREV[section.classDisplay];
    if (!classAbbrev) continue;  // Not a real EQ class — skip noise sections.

    const liBlocks = extractListItems(section.body);
    for (const li of liBlocks) {
      const slot = extractSlotLabel(li);
      if (!slot) continue;
      if (!(slot in WIKI_SLOT_TO_INV_SLOTS)) {
        unknownSlotsSet.add(slot);
        // Continue emitting rows for this slot — gear_check filtering
        // can still surface them; just won't match against any inv slot.
      }
      const itemNames = extractItemNames(li);
      for (let i = 0; i < itemNames.length; i++) {
        const itemName = itemNames[i];
        const isIksar = baseTier === 'Velious Pre-Raid/Group'
          && itemName.startsWith('Iksar ');
        const effectiveTier: Tier = isIksar ? 'Iksar' : baseTier;
        if (isIksar) iksarCount++;
        rows.push({
          tier: effectiveTier,
          class: classAbbrev,
          slot,
          item_id: null,
          item_name: itemName,
          rank: i + 1,
          last_refreshed: now,
        });
      }
    }
  }

  return {
    ok: true,
    rows,
    classCount: sections.length,
    itemCount: rows.length,
    iksarCount,
    unknownSlots: Array.from(unknownSlotsSet).sort(),
  };
}

interface ClassSection {
  classDisplay: string;
  body: string;
}

// splitOnClassHeaders walks `== [[ClassName]] ==` headers (multiline).
// Body for each is the wikitext between this header and the next ==X==
// header (or end-of-document).
function splitOnClassHeaders(wikitext: string): ClassSection[] {
  const out: ClassSection[] = [];

  // All ==X== header positions (any level-2 header).
  const anyHeaderRe = /^==[^=].*==\s*$/gm;
  const anyHeaderPositions: number[] = [];
  let m: RegExpExecArray | null;
  while ((m = anyHeaderRe.exec(wikitext)) !== null) {
    anyHeaderPositions.push(m.index);
  }

  // Match `== [[ClassName]] ==` specifically.
  const classHeaderRe = /^==\s*\[\[([^\]]+)\]\]\s*==\s*$/gm;
  let classMatch: RegExpExecArray | null;
  while ((classMatch = classHeaderRe.exec(wikitext)) !== null) {
    const classDisplay = classMatch[1].trim();
    const headerEnd = classMatch.index + classMatch[0].length;
    const nextHeaderIdx = anyHeaderPositions.find((p) => p > headerEnd);
    const body = nextHeaderIdx !== undefined
      ? wikitext.slice(headerEnd, nextHeaderIdx)
      : wikitext.slice(headerEnd);
    out.push({ classDisplay, body });
  }
  return out;
}

// extractListItems pulls `<li>...</li>` blocks from the section body.
// Non-greedy match to handle <ul><li>...<li>...</li></ul> patterns where
// li tags overlap (P1999 wiki sometimes omits closing </li> before next
// <li>).
function extractListItems(body: string): string[] {
  const out: string[] = [];
  // Greedy-up-to-next-<li>-or-</li>-or-</ul> approach handles unclosed
  // <li> tags (which appear in some sections).
  const re = /<li[^>]*>([\s\S]*?)(?=<\/li>|<li[^>]*>|<\/ul>|$)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(body)) !== null) {
    out.push(m[1]);
  }
  return out;
}

// extractSlotLabel pulls the bolded slot name from an <li>.
// Pattern: `'''SlotName'''` (wikitext bold). Trims + returns; null if not found.
function extractSlotLabel(li: string): string | null {
  const m = li.match(/'''([^']+)'''/);
  return m ? m[1].trim() : null;
}

// extractItemNames pulls all `{{:ItemName}}` template transclusions from
// an <li>. Strips parenthetical notes from item names.
function extractItemNames(li: string): string[] {
  const out: string[] = [];
  const re = /\{\{:([^}|]+)(?:\|[^}]*)?\}\}/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(li)) !== null) {
    const name = stripParenNotes(m[1].trim());
    if (name) out.push(name);
  }
  return out;
}

// stripParenNotes removes any ` (anything)` suffix or embedded notes
// from an item name. "Whetstone (Worn)" → "Whetstone".
function stripParenNotes(s: string): string {
  return s.replace(/\s*\([^)]*\)\s*/g, ' ').replace(/\s+/g, ' ').trim();
}
