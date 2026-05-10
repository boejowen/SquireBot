// Pure parser for P1999 per-class spell wiki pages. No side effects,
// no API calls. Algorithm verified against the 3 class fixtures captured
// in research (Necromancer pure caster, Paladin hybrid, Warrior
// degenerate-no-spells).
//
// Page shape (per 04-RESEARCH.md §3):
//   ==Level N==
//   <table ...>
//   {{SpellHeaderRow ...}}
//   {{SpellRow
//   |name=Cavorting Bones
//   |type=Summon
//   |...
//   }}
//   {{SpellRow|name=Coldlight|type=Utility|...}}
//   ...
//
// The {{SpellRow}} template's `name` parameter may not always be the
// FIRST parameter — the regex below matches `|name=` anywhere within
// the template body, not just first-position (RESEARCH §5 #4).

import type { SpellParseResult, WikiSpellRow } from './wiki-spell-types';
import { normalizeSpellName } from './wiki-spell-types';

const MIN_WIKITEXT_LENGTH = 200;

export function parseClassPage(
  wikitext: string,
  classAbbrev: string,
): SpellParseResult {
  if (typeof wikitext !== 'string' || wikitext.length < MIN_WIKITEXT_LENGTH) {
    return { ok: false, reason: 'wikitext_too_short' };
  }

  const sections = splitOnLevelHeaders(wikitext);
  // sections is [{ level: number, body: string }, ...].
  // Empty array means class page has no ==Level N== headers — degenerate
  // (e.g. Warrior). NOT an error; emit zero spell rows.

  const rows: WikiSpellRow[] = [];
  const now = new Date().toISOString();
  let spellCount = 0;
  for (const section of sections) {
    const spellNames = extractSpellNames(section.body);
    for (const name of spellNames) {
      const trimmed = name.trim();
      if (!trimmed) continue;
      rows.push({
        class: classAbbrev,
        level: section.level,
        spell_name: trimmed,
        normalized_name: normalizeSpellName(trimmed),
        last_refreshed: now,
      });
      spellCount++;
    }
  }

  return {
    ok: true,
    rows,
    levelHeaders: sections.length,
    spellCount,
  };
}

interface LevelSection {
  level: number;
  body: string; // wikitext between this header and the next ==...== header
}

// splitOnLevelHeaders returns one entry per ==Level N== header. The
// body is everything from the end of one header to the start of the
// next ==...== (any wikitext header at level 2). Top-of-page content
// before the first level header (class blurb, picking-the-right-race,
// etc.) is discarded.
function splitOnLevelHeaders(wikitext: string): LevelSection[] {
  const out: LevelSection[] = [];
  // Find all level-header positions.
  const headerRe = /^==\s*Level\s+(\d+)\s*==\s*$/gm;
  // We also need to know where ANY ==X== header lives, so a non-Level
  // header (like "==Class Highlights==" appearing after the spells
  // section) terminates the last spell body.
  const anyHeaderRe = /^==[^=].*==\s*$/gm;

  // Capture all anyHeaderRe positions to determine section ends.
  const anyHeaderPositions: number[] = [];
  let m: RegExpExecArray | null;
  while ((m = anyHeaderRe.exec(wikitext)) !== null) {
    anyHeaderPositions.push(m.index);
  }

  // Now walk Level headers; for each, body ends at the next anyHeader
  // position OR end-of-wikitext.
  let lvlMatch: RegExpExecArray | null;
  while ((lvlMatch = headerRe.exec(wikitext)) !== null) {
    const level = parseInt(lvlMatch[1], 10);
    if (!Number.isFinite(level) || level < 1 || level > 60) continue;
    const headerEnd = lvlMatch.index + lvlMatch[0].length;
    // Find the next anyHeader strictly after headerEnd.
    const nextHeaderIdx = anyHeaderPositions.find((p) => p > headerEnd);
    const body = nextHeaderIdx !== undefined
      ? wikitext.slice(headerEnd, nextHeaderIdx)
      : wikitext.slice(headerEnd);
    out.push({ level, body });
  }
  return out;
}

// extractSpellNames pulls the `name=` parameter out of every {{SpellRow}}
// template in the section body. Position-agnostic — matches `|name=`
// anywhere within the template body, not just first-position.
function extractSpellNames(body: string): string[] {
  const names: string[] = [];
  const templateRe = /\{\{\s*SpellRow\b([\s\S]*?)\}\}/g;
  let m: RegExpExecArray | null;
  while ((m = templateRe.exec(body)) !== null) {
    const tplBody = m[1];
    const nameMatch = tplBody.match(/\|\s*name\s*=\s*([^\n|}]+)/);
    if (nameMatch) {
      names.push(nameMatch[1]);
    }
  }
  return names;
}
