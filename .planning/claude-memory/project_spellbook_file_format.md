---
name: P99 spellbook file format
description: /outputfile spellbook produces tab-separated `Level\tName` rows (NOT `Slot\tName` as docs say). No spell IDs. No mem-slot data. Sample verified 2026-05-01 from SK char Slampeach.
type: project
originSessionId: dfdf0595-b2de-450e-a3e8-15ecb9220949
---
`/outputfile spellbook` on Project 1999 emits `<CharName>-Spellbook.txt` (e.g., `Slampeach-Spellbook.txt`) — the watcher's fsnotify pattern matcher needs this. The file is tab-separated with **two columns**:

1. `Level` — integer, the level at which the spell is granted/usable (e.g. 9, 15, 22, 29, 30, 34, 39, 49, 50, 51, 53)
2. `Name` — spell name string (e.g. `Lifetap`, `Siphon Life`, `Boil Blood`)

**No spell ID column.** Phase 4 `spell_check` must join on normalized spell name (already noted in ROADMAP.md Phase 4 success criterion 2).

**No mem-slot information.** The dump is everything *scribed in the spellbook*, not what's currently memorized in the 8 mem slots.

**Misnomer in existing docs:** CLAUDE.md, ARCHITECTURE.md, and Phase 1 design called the first column `Slot`. The values prove it's actually `Level` (Lifetap=9 is a level-9 spell, Boil Blood=53 is a level-53 spell). The Phase 2 schema-lock plan should rename the column to `Level` and update CLAUDE.md/ARCHITECTURE.md/SUMMARY.md to match.

**Why:** Verified against a real SK sample (49 entries, levels 9–53) on 2026-05-01.

**How to apply:** When designing the `spell:<Char>` landing tab schema in Phase 2, use columns `Level, Name, _uploaded_at`. When planning Phase 4 `spell_check`, the join key against `_wiki_spells` is normalized spell name (case/whitespace-folded); level filtering is `spell.level <= char.level`.

**Test fixture committed:** `internal/parse/testdata/Slampeach-Spellbook.txt` (created `1d6f575`, renamed from `sample-spellbook.txt` in the next commit to match real EQ filename pattern `<CharName>-Spellbook.txt`) — Slampeach the Shadowknight, 49 spells, 9–53 level range. Includes spells from all SK schools: Necromancy (Lifetap, Siphon Life), Conjuration (Leering Corpse, Animate Dead), Divination (Sense the Dead, Locate Corpse, Cancel Magic), Alteration (Shieldskin, Endure Cold), Evocation (Lifespike, Lifedraw).
