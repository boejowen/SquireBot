// Pure parser for P1999 {{Itempage}} wikitext. No side effects, no API
// calls, no logging. Algorithm verified against the 5 wiki fixtures
// captured in research (Cloth Cap, Pearl, Cloak of Flames, Fungus
// Covered Scale Tunic, Fungi Tunic redirect).
//
// Caller is responsible for resolving redirects upstream by passing
// `redirects=true` on the wiki API URL — by the time we see wikitext
// here, it's already the resolved-target page's content.

import type { ParseResult, ParsedWikiItem, WikiQuestItemLink } from './wiki-types';

const MIN_WIKITEXT_LENGTH = 200;

// pageNameToSlug converts a page title to a wiki URL path segment.
// MediaWiki uses underscores for spaces; other special chars are
// percent-encoded. Verified against the 4 non-redirect fixtures.
export function pageNameToSlug(name: string): string {
  return encodeURIComponent(name.replace(/ /g, '_'));
}

export function wikiUrlFor(pageTitle: string): string {
  return `https://wiki.project1999.com/${pageNameToSlug(pageTitle)}`;
}

// computeSha1Hex computes a stable hex SHA-1 of the wikitext for the
// week-over-week change-detection short-circuit. Apps Script V8's
// Utilities.computeDigest returns signed bytes; we must convert to
// unsigned before hex-encoding.
export function computeSha1Hex(s: string): string {
  const bytes = Utilities.computeDigest(
    Utilities.DigestAlgorithm.SHA_1,
    Utilities.newBlob(s).getBytes(),
  );
  return bytes
    .map((b) => (b < 0 ? b + 256 : b).toString(16).padStart(2, '0'))
    .join('');
}

export function parseItempage(wikitext: string, pageTitle: string): ParseResult {
  if (typeof wikitext !== 'string' || wikitext.length < MIN_WIKITEXT_LENGTH) {
    return { ok: false, reason: 'wikitext_too_short' };
  }

  const blockBody = extractItempageBody(wikitext);
  if (blockBody === null) {
    return { ok: false, reason: 'no_itempage' };
  }

  const params = parseTemplateParams(blockBody);
  const itemname = (params.get('itemname') ?? pageTitle).trim();
  const notesRaw = (params.get('notes') ?? '').trim();
  const statsblockRaw = (params.get('statsblock') ?? '').trim();

  const statsblock = parseStatsblock(statsblockRaw);
  const summary = extractSummary(notesRaw);
  const wikitext_sha1 = computeSha1Hex(wikitext);

  const item: ParsedWikiItem = {
    itemname,
    page_title: pageTitle,
    wiki_url: wikiUrlFor(pageTitle),
    summary,
    is_quest_item: statsblock.flags.has('QUEST ITEM'),
    is_no_drop: statsblock.flags.has('NO DROP') || statsblock.flags.has('NO-DROP'),
    is_lore: statsblock.flags.has('LORE ITEM'),
    is_magic: statsblock.flags.has('MAGIC ITEM'),
    is_temporary: statsblock.flags.has('TEMPORARY'),
    slot: statsblock.kv.get('Slot') ?? null,
    classes: parseClasses(statsblock.kv.get('Class') ?? ''),
    ac: parseStatNumber(statsblock.kv.get('AC')),
    weight: parseStatNumber(statsblock.kv.get('WT')),
    effect: parseEffect(statsblock.kv.get('Effect')),
    wikitext_sha1,
  };

  const questLinks = harvestQuestLinks(notesRaw, item);
  return { ok: true, item, questLinks };
}

// extractItempageBody finds the `{{Itempage` template and returns its
// body (everything between the opening `{{Itempage` and the matching
// `}}`, exclusive). Handles nested {{...}} via depth counting. Returns
// null if no Itempage template is present.
function extractItempageBody(wikitext: string): string | null {
  const openIdx = wikitext.indexOf('{{Itempage');
  if (openIdx === -1) return null;
  // Start scanning AFTER the {{ at openIdx (depth=1 once we step past it).
  let depth = 1;
  let i = openIdx + 2; // skip past '{{'
  while (i < wikitext.length) {
    const ch = wikitext[i];
    const next = wikitext[i + 1];
    if (ch === '{' && next === '{') { depth++; i += 2; continue; }
    if (ch === '}' && next === '}') {
      depth--;
      if (depth === 0) {
        return wikitext.slice(openIdx + 2, i);
      }
      i += 2; continue;
    }
    i++;
  }
  return null; // unbalanced
}

// parseTemplateParams takes the body of a template (everything between
// `{{` and `}}` exclusive — including the template name as the first
// "param" before the first `|`) and returns a Map of named params.
// Splits on `|` only at template depth 0 so nested {{...}} params aren't
// shredded.
function parseTemplateParams(body: string): Map<string, string> {
  const params = new Map<string, string>();
  const segments = splitAtDepthZero(body, '|');
  // segments[0] is the template name itself ("Itempage" or "Itempage\n|param" — split on \n|).
  for (let s = 1; s < segments.length; s++) {
    const seg = segments[s];
    const eq = seg.indexOf('=');
    if (eq === -1) continue;
    const key = seg.slice(0, eq).trim();
    const value = seg.slice(eq + 1);
    if (key) params.set(key, value);
  }
  return params;
}

// splitAtDepthZero splits the input string on the given delimiter, but
// only when the {{ }} nesting depth is zero. Used by parseTemplateParams
// to avoid splitting inside nested templates.
function splitAtDepthZero(input: string, delim: string): string[] {
  const out: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    const next = input[i + 1];
    if (ch === '{' && next === '{') { depth++; i++; continue; }
    if (ch === '}' && next === '}') { depth--; i++; continue; }
    if (depth === 0 && ch === delim) {
      out.push(input.slice(start, i));
      start = i + 1;
    }
  }
  out.push(input.slice(start));
  return out;
}

interface ParsedStatsblock {
  flags: Set<string>;     // standalone uppercase lines like "QUEST ITEM"
  kv: Map<string, string>; // "Slot: HEAD", "AC: 2", etc.
}

// parseStatsblock takes the statsblock value (which is HTML-in-wikitext,
// not a structured template), splits on <br>, and classifies each line
// as either a standalone flag (no colon, all-uppercase) or a
// key:value pair. Multi-stat lines like "STR: +2  DEX: -10" are split
// further on two-space delimiter.
function parseStatsblock(raw: string): ParsedStatsblock {
  const flags = new Set<string>();
  const kv = new Map<string, string>();
  const lines = raw.split(/<br\s*\/?>/i).map((l) => l.trim()).filter(Boolean);
  for (const line of lines) {
    if (!line.includes(':')) {
      // Standalone flag.
      const upper = line.toUpperCase();
      if (/^[A-Z][A-Z\s\-]+$/.test(upper)) flags.add(upper);
      continue;
    }
    // Possibly multi-stat line (e.g., "STR: +2  DEX: -10  INT: +2  AGI: -10").
    const pieces = splitMultiStat(line);
    for (const piece of pieces) {
      const colonAt = piece.indexOf(':');
      if (colonAt === -1) continue;
      const key = piece.slice(0, colonAt).trim();
      const value = piece.slice(colonAt + 1).trim();
      if (key) kv.set(key, value);
    }
  }
  return { flags, kv };
}

// splitMultiStat splits on two-or-more-space runs that precede a known
// short-stat key (typically 2-3 uppercase letters or "Size"/"WT"). For a
// simple "Slot: HEAD" line, returns ["Slot: HEAD"] (single piece).
function splitMultiStat(line: string): string[] {
  // Pieces are separated by 2+ spaces followed by a stat-key+colon.
  // Conservative regex: `  +(?=[A-Za-z][A-Za-z]?[A-Za-z]?:)`.
  return line.split(/\s{2,}(?=[A-Za-z][A-Za-z]?[A-Za-z]?[A-Za-z]?[A-Za-z]?:)/);
}

// parseClasses takes the Class statsblock value (e.g., "ALL" or
// "WAR CLR PAL RNG SHD DRU MNK BRD ROG SHM") and returns the list of
// class abbreviations. Whitespace-collapses; deduplicates.
function parseClasses(raw: string): string[] {
  if (!raw) return [];
  const tokens = raw.split(/\s+/).map((t) => t.trim()).filter(Boolean);
  return Array.from(new Set(tokens));
}

// parseStatNumber pulls the first number out of a stat value. "AC: 21"
// → 21; "WT: 0.2  Size: SMALL" (already split by parseStatsblock so we
// see "0.2") → 0.2; missing → null.
function parseStatNumber(raw: string | undefined): number | null {
  if (!raw) return null;
  const m = raw.match(/-?\d+(?:\.\d+)?/);
  if (!m) return null;
  const n = parseFloat(m[0]);
  return Number.isFinite(n) ? n : null;
}

// parseEffect extracts the effect display name from a value like
// "[[Fungal Regrowth]] (Worn)". Strips the wiki-link brackets.
function parseEffect(raw: string | undefined): string | null {
  if (!raw) return null;
  const cleaned = raw.replace(/\[\[([^|\]]+)(?:\|[^\]]+)?\]\]/g, '$1').trim();
  return cleaned || null;
}

// extractSummary takes the raw `notes` body, strips wiki-links + HTML,
// and returns the first MAX_SUMMARY_LEN chars (cut on word boundary
// when possible). Empty notes → empty string (cell-note will skip the
// summary line).
const MAX_SUMMARY_LEN = 200;

function extractSummary(notes: string): string {
  if (!notes) return '';
  // Render wiki-links as their display text (or target if no display).
  let text = notes.replace(
    /\[\[([^|\]]+)(?:\|([^\]]+))?\]\]/g,
    (_m, target, display) => display ?? target,
  );
  // Strip HTML tags.
  text = text.replace(/<[^>]+>/g, ' ');
  // Strip remaining template noise like `{{Foo}}`.
  text = text.replace(/\{\{[^}]+\}\}/g, '');
  // Collapse whitespace.
  text = text.replace(/\s+/g, ' ').trim();
  if (text.length <= MAX_SUMMARY_LEN) return text;
  // Truncate on word boundary near the limit.
  const cut = text.slice(0, MAX_SUMMARY_LEN);
  const lastSpace = cut.lastIndexOf(' ');
  return (lastSpace > MAX_SUMMARY_LEN - 30 ? cut.slice(0, lastSpace) : cut).trim() + '…';
}

// harvestQuestLinks scans the notes body for [[wiki link]] targets and
// emits one WikiQuestItemLink per unique target. If the item also has
// the in-game QUEST ITEM flag, prepends a separate link with
// source='in_game_flag' so the cell-note can show both signals.
//
// Phase 4 will filter the notes_link harvest against the actual quest
// catalog (some [[links]] in notes are zone names, mob names, etc., not
// quests). Phase 3 stores them all so the data is recoverable.
function harvestQuestLinks(notes: string, item: ParsedWikiItem): WikiQuestItemLink[] {
  const links: WikiQuestItemLink[] = [];
  if (item.is_quest_item) {
    links.push({
      item_id: 0, // caller fills in
      item_name: item.itemname,
      quest_name: '[in-game QUEST flag]',
      source: 'in_game_flag',
    });
  }
  if (!notes) return links;
  const seen = new Set<string>();
  const re = /\[\[([^|\]#]+)(?:#[^|\]]*)?(?:\|[^\]]+)?\]\]/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(notes)) !== null) {
    const target = m[1].trim();
    if (!target || seen.has(target)) continue;
    seen.add(target);
    links.push({
      item_id: 0, // caller fills in
      item_name: item.itemname,
      quest_name: target,
      source: 'notes_link',
    });
  }
  return links;
}
