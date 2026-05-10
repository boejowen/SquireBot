// composeItemNote — pure function that builds the Range.setNote() string
// for a view/bank row. Composes wiki summary + price lines + quest-item
// info per Phase 3 RESEARCH §3 and 03-04-PLAN task 1.
//
// Note size cap is 50KB per cell (verified per RESEARCH §7); our worst-
// case composition is ~600 chars, well within budget.

import type { PigparseDirection } from '../lib/pigparse-types';

export interface PigparsePriceRow {
  direction: PigparseDirection;
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

export function composeItemNote(
  summary: WikiSummaryForNote | null,
  pigparseRows: PigparsePriceRow[],
  questLinks: QuestLinkForNote[],
): string {
  const parts: string[] = [];

  if (summary?.summary) {
    parts.push(summary.summary);
  }

  const wts = pigparseRows.find((r) => r.direction === 0);
  const wtb = pigparseRows.find((r) => r.direction === 1);

  if (wts || wtb) {
    parts.push(''); // blank line separator
    if (wts) parts.push(`Recent ask: ${formatPp(wts.a30)}pp (30d avg, ${wts.t30} transactions)`);
    if (wtb) parts.push(`Buy posts: ${formatPp(wtb.a30)}pp (30d avg, ${wtb.t30} transactions)`);
  } else {
    parts.push('');
    parts.push('No recent transactions on PigParse.');
  }

  if (summary?.is_quest_item) {
    parts.push('');
    parts.push('Quest item: yes (in-game flag)');
  }

  const noteLinks = questLinks
    .filter((l) => l.source === 'notes_link')
    .slice(0, MAX_QUEST_LINKS_IN_NOTE);
  if (noteLinks.length > 0) {
    if (!summary?.is_quest_item) parts.push(''); // separator if we didn't add the quest-flag block
    parts.push(`Used in quests: ${noteLinks.map((l) => l.quest_name).join(', ')}`);
  }

  return parts.join('\n').trim();
}

function formatPp(n: number): string {
  if (!Number.isFinite(n)) return '0';
  return Math.round(n).toLocaleString('en-US');
}
