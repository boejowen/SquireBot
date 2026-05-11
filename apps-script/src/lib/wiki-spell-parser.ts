// Pure parser for P1999 per-class spell wiki pages. No side effects,
// no API calls. Algorithm verified against the 3 class fixtures captured
// in research (Necromancer pure caster, Paladin hybrid, Warrior
// degenerate-no-spells) PLUS three template variants surfaced during
// the v0.4.0-rc1 live smoke (2026-05-10):
//
// Variant 1 — {{SpellRow}} (CLR/PAL/NEC/MAG/ENC):
//   ==Level N==
//   {{SpellRow|name=Cavorting Bones|type=Summon|...}}
//
// Variant 2 — {{RadSpellRow}} (WIZ/DRU/SHM/RNG/SHD):
//   ==Level N==
//   {{RadSpellRow|name=Frost Bolt|kind=Damage|...}}
//
// Variant 3 — {{SongRow}} / {{Template:SongRow}} (BRD only):
//   No ==Level N== headers. Single big table; level is inline:
//   {{Template:SongRow|name=Chant of Battle|level=1|...}}
//
// Both header-driven variants (1 + 2) share the same structure: section
// header gives the level, template body gives the spell name. The Bard
// variant (3) extracts level from the template body itself. The parser
// uses the header pass first; if it produces zero rows it falls back to
// the inline-level pass.
//
// All template `name=` parameters may not be the FIRST parameter — the
// regex matches `|name=` anywhere within the template body, not just
// first-position (RESEARCH §5 #4). Same goes for `|level=` in variant 3.
//
// Templates can be invoked as `{{X|...}}` or `{{Template:X|...}}` —
// MediaWiki treats them as equivalent. Parser tolerates either form.

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
  // Empty array can mean (a) degenerate class page with no spells
  // (Warrior, Monk, Rogue) — emit zero rows OR (b) inline-level format
  // like Bard's {{Template:SongRow|level=N|...}} where level lives in
  // the template body. Try the inline pass as a fallback.

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

  // Fallback: header pass produced no rows. Try inline-level templates
  // (Bard). If this ALSO returns nothing, the class is genuinely
  // spell-less (Warrior/Monk/Rogue) — return ok with empty rows.
  if (spellCount === 0) {
    const inlineRows = extractInlineLevelSpells(wikitext, classAbbrev, now);
    rows.push(...inlineRows);
    spellCount += inlineRows.length;
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

// extractSpellNames pulls the `name=` parameter out of every spell
// template in the section body. Matches all known spell template
// variants surfaced during the v0.4.0-rc1 live smoke:
//   {{SpellRow}}      — CLR/PAL/NEC/MAG/ENC
//   {{RadSpellRow}}   — WIZ/SHM/RNG/SHD
//   {{RadSpellRow2}}  — DRU (numbered template revision)
// All optionally prefixed with `Template:`. Trailing `\d*` future-
// proofs against further numbered revisions (RadSpellRow3, etc.).
// Position-agnostic — matches `|name=` anywhere within the template
// body, not just first-position.
function extractSpellNames(body: string): string[] {
  const names: string[] = [];
  const templateRe = /\{\{\s*(?:Template:)?\s*(?:Rad)?SpellRow\d*\b([\s\S]*?)\}\}/g;
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

// extractInlineLevelSpells handles the Bard-style page format where there
// are no ==Level N== section headers and the level lives inside each
// template invocation as |level=N. Used as fallback when the header
// pass returns zero spells. Matches {{SongRow}} and {{Template:SongRow}}.
function extractInlineLevelSpells(
  wikitext: string,
  classAbbrev: string,
  now: string,
): WikiSpellRow[] {
  const rows: WikiSpellRow[] = [];
  const templateRe = /\{\{\s*(?:Template:)?\s*SongRow\b([\s\S]*?)\}\}/g;
  let m: RegExpExecArray | null;
  while ((m = templateRe.exec(wikitext)) !== null) {
    const tplBody = m[1];
    const nameMatch = tplBody.match(/\|\s*name\s*=\s*([^\n|}]+)/);
    const levelMatch = tplBody.match(/\|\s*level\s*=\s*(\d+)/);
    if (!nameMatch || !levelMatch) continue;
    const name = nameMatch[1].trim();
    const level = parseInt(levelMatch[1], 10);
    if (!name || !Number.isFinite(level) || level < 1 || level > 60) continue;
    rows.push({
      class: classAbbrev,
      level,
      spell_name: name,
      normalized_name: normalizeSpellName(name),
      last_refreshed: now,
    });
  }
  return rows;
}
