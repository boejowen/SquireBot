// P1999 wiki types. Schema verified against real fixtures captured at
// apps-script/src/__fixtures__/wiki-parse-*.json. See 03-RESEARCH.md §2
// for full template-shape decoding.

// ParsedWikiItem is the in-memory shape produced by parseItempage. Only
// the fields _item_master cares about; we ignore most of the {{Itempage}}
// parameters (dropsfrom, merchant_value, etc. — Phase 4+ uses some;
// Phase 3 doesn't need them in storage).
export interface ParsedWikiItem {
  itemname: string;
  page_title: string;        // canonical (post-redirect) title
  wiki_url: string;          // https://wiki.project1999.com/<slug>
  summary: string;           // first 200 chars of `notes`, links rendered as text
  is_quest_item: boolean;    // statsblock contains "QUEST ITEM"
  is_no_drop: boolean;
  is_lore: boolean;
  is_magic: boolean;
  is_temporary: boolean;
  slot: string | null;       // e.g. "HEAD", "CHEST", "BACK"
  classes: string[];         // ["ALL"] or ["WAR","CLR","PAL",...]
  ac: number | null;
  weight: number | null;
  effect: string | null;     // e.g. "Fungal Regrowth (Worn)"
  wikitext_sha1: string;     // hex digest for change-detection
}

export interface WikiQuestItemLink {
  item_id: number;
  item_name: string;
  quest_name: string;
  source: 'in_game_flag' | 'notes_link';
}

export type ParseResult =
  | { ok: true; item: ParsedWikiItem; questLinks: WikiQuestItemLink[] }
  | { ok: false; reason: 'no_itempage' | 'wikitext_too_short' | 'page_error'; detail?: string };
