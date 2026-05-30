// composeItemNote — pure function that builds the item-tooltip HTML for a
// view/bank row. Client TS port of apps-script/src/tabs/composeNotes.ts
// (Phase 14 Plan 14-02 Task 3). Content order + the MAX_QUEST_LINKS_IN_NOTE=5
// cap + formatPp are ported from v1; the OUTPUT FORMAT changes from
// newline-joined plain text (the old Range.setNote() string) to escaped rich
// HTML for the Svelte ItemTooltip popover (WEB-04 / D-08).
//
// SECURITY (the one HIGH-severity item in P14 — T-14.02-01, ASVS V5
// output-encoding): composeItemNote emits HTML built from server data that
// contains wiki/user-controlled strings (item name, wiki summary, quest
// names) and the wiki URL. EVERY interpolated dynamic value is run through
// escapeHtml() before injection; only the structural tags (<p>, <a>, <div>,
// <span>) are literal. The Svelte consumer (Plan 04) is the ONLY place that
// {@html}s this output, and it is fully escaped here — so a malicious item
// name like `<img src=x onerror=alert(1)>` renders as inert text, never a
// live tag. A vitest assertion proves it.
//
// JSON CONTRACT (the read API shape from Plan 14-01, locked in compute/types.go):
//   - The view/bank row carries `prices: PriceDetail[]` where each PriceDetail
//     has `direction` (TEXT — "0"=WTS, "1"=WTB, "2"=BOTH), `a30` (number),
//     `t30` (number). `price` (the top-line pick) may be null; that field is
//     not consumed here — composeItemNote reads the per-direction `prices[]`.
//   - `wiki_summary` (string|null), `is_quest_item` (bool), and
//     `quest_links: { quest_name, source }[]` (source 'in_game_flag'|'notes_link')
//     are likewise inline on the row so the tooltip needs no second fetch.

export type PriceDirection = '0' | '1' | '2'; // "0"=WTS, "1"=WTB, "2"=BOTH

export interface PigparsePriceRow {
  direction: PriceDirection;
  a30: number;
  t30: number;
}

export interface WikiSummaryForNote {
  summary: string;
  is_quest_item: boolean;
}

export interface QuestLinkForNote {
  quest_name: string;
  source: 'in_game_flag' | 'notes_link';
}

const MAX_QUEST_LINKS_IN_NOTE = 5;

/**
 * HTML output-encoding (ASVS V5). Replace `&` FIRST so the entity ampersands
 * we introduce aren't double-encoded, then the angle brackets and quotes.
 */
export function escapeHtml(s: string): string {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * Build the rich-HTML tooltip for an item. Returns an HTML string the Svelte
 * ItemTooltip injects via {@html}. Content order mirrors the v1 composeItemNote
 * (14-UI-SPEC Item Tooltip Contract):
 *   1. Item name (+ wiki <a> link, accent, rel="noopener" target="_blank")
 *   2. Wiki summary <p>            (omitted if absent)
 *   3. Price lines                 (Recent ask / Buy posts, or the no-data line)
 *   4. Quest flag                  (when is_quest_item)
 *   5. Used in quests: ...         (notes_link only, max 5)
 *
 * Every dynamic value is escapeHtml()'d; only structural tags are literal.
 */
export function composeItemNote(
  itemName: string,
  wikiUrl: string,
  summary: WikiSummaryForNote | null,
  pigparseRows: PigparsePriceRow[],
  questLinks: QuestLinkForNote[],
): string {
  const parts: string[] = [];

  // 1. Item name + optional wiki link (the URL is escaped inside the href).
  const safeName = escapeHtml(itemName);
  if (wikiUrl) {
    parts.push(
      `<div class="tooltip-title"><span class="tooltip-item-name">${safeName}</span> ` +
        `<a class="tooltip-wiki-link" href="${escapeHtml(wikiUrl)}" target="_blank" rel="noopener">wiki</a></div>`,
    );
  } else {
    parts.push(`<div class="tooltip-title"><span class="tooltip-item-name">${safeName}</span></div>`);
  }

  // 2. Wiki summary paragraph (omit the block if absent).
  if (summary?.summary) {
    parts.push(`<p class="tooltip-summary">${escapeHtml(summary.summary)}</p>`);
  }

  // 3. Price lines. direction "0"=WTS / "1"=WTB / "2"=BOTH (a BOTH row counts
  //    as both an ask and a buy). Numeric a30/t30 are not user-controlled, but
  //    pass through escapeHtml anyway for a uniform output-encoding posture.
  const wts = pigparseRows.find((r) => r.direction === '0' || r.direction === '2');
  const wtb = pigparseRows.find((r) => r.direction === '1' || r.direction === '2');
  if (wts || wtb) {
    if (wts) {
      parts.push(
        `<p class="tooltip-price">Recent ask: ${escapeHtml(formatPp(wts.a30))}pp ` +
          `(30d avg, ${escapeHtml(String(wts.t30))} transactions)</p>`,
      );
    }
    if (wtb) {
      parts.push(
        `<p class="tooltip-price">Buy posts: ${escapeHtml(formatPp(wtb.a30))}pp ` +
          `(30d avg, ${escapeHtml(String(wtb.t30))} transactions)</p>`,
      );
    }
  } else {
    parts.push('<p class="tooltip-price tooltip-no-price">No recent transactions on PigParse.</p>');
  }

  // 4. Quest flag.
  if (summary?.is_quest_item) {
    parts.push('<p class="tooltip-quest-flag">Quest item: yes (in-game flag)</p>');
  }

  // 5. Used-in-quests list (notes_link only, max 5; each name escaped).
  const noteLinks = questLinks
    .filter((l) => l.source === 'notes_link')
    .slice(0, MAX_QUEST_LINKS_IN_NOTE);
  if (noteLinks.length > 0) {
    const names = noteLinks.map((l) => escapeHtml(l.quest_name)).join(', ');
    parts.push(`<p class="tooltip-quests">Used in quests: ${names}</p>`);
  }

  return parts.join('');
}

/** Ported as-is from v1: round + en-US comma-grouping; "0" for non-finite. */
function formatPp(n: number): string {
  if (!Number.isFinite(n)) return '0';
  return Math.round(n).toLocaleString('en-US');
}
