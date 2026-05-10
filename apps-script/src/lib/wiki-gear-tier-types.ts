// Velious gear-tier wiki page types. Schema verified against live
// fixtures captured at apps-script/src/__fixtures__/wiki-velious-*-gear.json.
// See 04-RESEARCH.md §2 for full page-shape decoding.

export type Tier = 'Velious Pre-Raid/Group' | 'Velious Raiding' | 'Iksar';

export interface WikiGearTierRow {
  tier: Tier;
  class: string;        // 3-letter abbrev from CLASSES
  slot: string;         // wiki vocab: 'Head', 'Chest', 'Primary', 'Ears', etc.
  item_id: number | null; // NULL — wiki transclusions don't expose IDs
  item_name: string;    // verbatim from {{:Name}}, parenthetical notes stripped
  rank: number;         // 1-based position in the slot's recommendation list
  last_refreshed: string;  // ISO 8601
}

export type GearTierParseResult =
  | {
      ok: true;
      rows: WikiGearTierRow[];
      classCount: number;
      itemCount: number;
      iksarCount: number;        // for Pre-Raid pages — number of items tagged 'Iksar'
      unknownSlots: string[];     // wiki slot labels NOT in WIKI_SLOT_TO_INV_SLOTS
    }
  | {
      ok: false;
      reason: 'wikitext_too_short' | 'no_class_sections';
      detail?: string;
    };
