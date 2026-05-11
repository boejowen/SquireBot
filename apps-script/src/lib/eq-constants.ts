// EverQuest Project 1999 constants. Single source of truth for class
// + race lists, slot vocabulary mapping, and other static data shared
// across Phase 4+ triggers/builders.
//
// Source: P1999 wiki + in-game stats screen (verified during Phase 4
// research; see 04-RESEARCH.md §2 for slot vocab derivation).

export const CLASSES = [
  'WAR', 'CLR', 'PAL', 'RNG', 'SHD', 'DRU', 'MNK', 'BRD',
  'ROG', 'SHM', 'NEC', 'WIZ', 'MAG', 'ENC',
] as const;
export type ClassAbbrev = typeof CLASSES[number];

// Maps wiki class display name (e.g. "Necromancer") to 3-letter abbrev.
// Used by refreshWikiSpells + refreshWikiGearTier when normalizing
// class names from wiki page section headers.
export const CLASS_DISPLAY_TO_ABBREV: Record<string, ClassAbbrev> = {
  'Bard': 'BRD',
  'Cleric': 'CLR',
  'Druid': 'DRU',
  'Enchanter': 'ENC',
  'Magician': 'MAG',
  'Monk': 'MNK',
  'Necromancer': 'NEC',
  'Paladin': 'PAL',
  'Ranger': 'RNG',
  'Rogue': 'ROG',
  'Shadow Knight': 'SHD',
  'Shaman': 'SHM',
  'Warrior': 'WAR',
  'Wizard': 'WIZ',
};

// Inverse — abbrev → wiki display name. Used by refreshWikiSpells when
// constructing the wiki page URL for a given class.
export const CLASS_ABBREV_TO_DISPLAY: Record<ClassAbbrev, string> = (() => {
  const out = {} as Record<ClassAbbrev, string>;
  for (const [display, abbrev] of Object.entries(CLASS_DISPLAY_TO_ABBREV)) {
    out[abbrev] = display;
  }
  return out;
})();

export const RACES = [
  'HUM', 'BAR', 'ERU', 'ELF', 'HIE', 'DEF', 'HEF', 'DWF',
  'TRL', 'OGR', 'HFL', 'GNM', 'IKS', 'VAH',
] as const;
export type RaceAbbrev = typeof RACES[number];

// Wiki gear-tier pages use prose slot labels (Ears, Fingers, Wrists,
// etc.); inv tab Slot column uses EQ in-game tokens (EAR1, EAR2, etc.).
// Pair slots (Ears/Fingers/Wrists) map to TWO inv slots — gear_check
// considers a char OK if the recommended item is in EITHER slot.
export const WIKI_SLOT_TO_INV_SLOTS: Record<string, string[]> = {
  'Ears': ['EAR1', 'EAR2'],
  'Fingers': ['FINGER1', 'FINGER2'],
  'Wrists': ['WRIST1', 'WRIST2'],
  'Neck': ['NECK'],
  'Head': ['HEAD'],
  'Face': ['FACE'],
  'Chest': ['CHEST'],
  'Arms': ['ARMS'],
  'Back': ['BACK'],
  'Waist': ['WAIST'],
  'Shoulders': ['SHOULDERS'],
  'Legs': ['LEGS'],
  'Hands': ['HANDS'],
  'Feet': ['FEET'],
  'Primary': ['PRIMARY'],
  'Secondary': ['SECONDARY'],
  'Range': ['RANGE'],
};

export function isClassAbbrev(s: unknown): s is ClassAbbrev {
  return typeof s === 'string' && (CLASSES as readonly string[]).includes(s);
}

export function isRaceAbbrev(s: unknown): s is RaceAbbrev {
  return typeof s === 'string' && (RACES as readonly string[]).includes(s);
}

// Phase 5 plan 05-03: P99-known inventory slot vocabulary for the search
// sidebar's Slot filter dropdown. Per CONTEXT D-01 and PATTERNS §eq-constants
// the hardcoded list is the chosen default over the scrape-from-data
// alternative — simpler for tests and stable across workbooks. The slot
// filter does `loc.toUpperCase().startsWith(slotFilterUpper)` against the
// `inv:*` Location column (Assumption A2 per RESEARCH).
export const INVENTORY_SLOTS = [
  'HEAD', 'CHEST', 'EAR1', 'EAR2', 'ARMS', 'WRIST1', 'WRIST2',
  'LEGS', 'FEET', 'HANDS', 'NECK', 'FINGER1', 'FINGER2',
  'SHOULDERS', 'BACK', 'WAIST', 'RANGE', 'AMMO', 'PRIMARY', 'SECONDARY',
  'FACE',
  'GENERAL', 'BANK', 'HELD', 'CURSOR',
] as const;
export type InventorySlot = typeof INVENTORY_SLOTS[number];
