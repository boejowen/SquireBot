# EverQuest-Aesthetic Theme for the Shared Workbook

> **Status:** Open design question. To be decided during Phase 3 (Apps Script + view tabs + sidebar) before view-tab styling code is written.
>
> **Decision owner:** Project owner — must pick a direction from a multi-option proposal.
>
> **Planted:** 2026-05-08 during Phase 2 soak.

## Why this matters

The shared Google Sheets workbook is the headline UX surface for SquireBot. Every guildie opens it daily — it's where they answer "what does my character still need, and where in the guild is it?"

An aesthetic that's reminiscent of classic EverQuest UI converts the workbook from "yet another spreadsheet" into a *guild artifact* that feels native to the game the guild plays. This is morale-and-identity work, not feature work — the kind of polish that makes officers proud to hand the link to a new guildie ("you have to see what we built") and that subtle delight is what separates "tool we tolerate" from "tool we love."

SquireBot is a fan project for a Project 1999 (Classic EQ emulator) guild. The guild's identity is rooted in the original 1999–2002 era of the game. A workbook that visually echoes that era reinforces the project's belonging in that community.

## Deliverable expectation

When this gets picked up in Phase 3, the right move is **not** to pick one aesthetic direction unilaterally. Instead, produce **2–4 distinct mockups or descriptions** that span the design space and let the project owner choose. `/gsd-sketch` (UI/design ideas with throwaway HTML mockups) is purpose-built for exactly this side-by-side mockup deliverable.

A reasonable starting spectrum:

| Option | Era | Intensity | Sketch |
|---|---|---|---|
| **A. Vanilla** | pre-Kunark (1999–2000) | Medium | Browns + golds, simple serif, beveled headers |
| **B. Velious** | 2000–2001 | Medium | Icy blues + silver, heavier panels, frosted accents |
| **C. Minimalist EQ-flavored** | era-agnostic | Light | Muted EQ palette + serif font, "EQ-inspired" rather than "EQ-replicated" |
| **D. Heavy EQ-replica** | pick one era | Heavy | Embedded class icons, parchment-look rows, full sidebar reskin, banner row with EQ-style frame |

These are starting points — Phase 3 should iterate, not commit to one of these four.

## Technical context

### What works in Google Sheets cells

- **Color palette** — full control over cell background, font color, border color
- **Custom fonts** via Google Fonts integration (e.g., MedievalSharp, IM Fell English, Cinzel for serifs)
- **Conditional formatting** — red MISSING / green OK / amber tier-progression maps cleanly to EQ HP/mana color language
- **Inline images** via `IMAGE()` formula — class icons, item icons, banner row at the top of view tabs
- **Drawings** (Insert > Drawing) — freeform decorative elements, can be positioned over the grid

### What does NOT work in Google Sheets cells

- ❌ **No per-cell or per-tab background images.** No stone-texture tiling behind the grid.
- ❌ **No global theme/skin** that auto-applies to every new tab. Each tab needs styling at build time by Apps Script.
- ❌ **Sheets' built-in filter view chrome** (the funnel icon + dropdowns) is unstylable — risks visual clash with a heavy EQ aesthetic.

### HtmlService sidebar — full HTML+CSS freedom

The HtmlService search sidebar is where the EQ aesthetic can carry the heaviest visual lift. Beveled stone panels, gold trim, parchment rows, serif fonts, custom icons — all standard CSS work. The sidebar is the cross-character search surface (Flow D in `.planning/research/ARCHITECTURE.md`) and it's a high-value place to invest in aesthetic.

### Net split

- **Cell-based view tabs** carry ~60% of the EQ feel (color, font, borders, inline icons).
- **HtmlService sidebar** carries the remaining ~40% with full visual freedom.

## Asset sourcing

### P1999 wiki assets — clean to use

[`wiki.project1999.com`](https://wiki.project1999.com/) hosts CC-BY-SA-licensed item icons, class icons, and zone screenshots. Clean to use under the wiki's license terms (attribution required). Should be the default asset source.

### Original Sony / Daybreak EQ UI chrome bitmaps — gray area

Original UI assets (the actual stone-panel skins, button textures, EQ logo) are technically Daybreak IP. Using them in a non-commercial fan tool for a single guild is standard P99-community practice — P99 itself runs on Daybreak's IP under tolerated terms — but worth a deliberate decision rather than drifting into it.

**Default recommendation:** lean on P99-wiki-licensed assets and recreate the *aesthetic* with original styling rather than copying chrome bitmaps verbatim. Revisit if the project owner explicitly wants the literal Daybreak chrome.

## Open questions for Phase 3

These are NOT to answer now — they should be raised when this design gets picked up:

1. **Era preference:** vanilla pre-Kunark (browns + golds, simple) vs. Kunark (jungle greens) vs. Velious (icy blues + silver). Affects palette decisively.
2. **Asset boundary:** lean on P99-wiki assets only, OR allow Daybreak chrome bitmaps in spirit of P99-community practice?
3. **Aesthetic intensity:** subtle "EQ-flavored" (muted, professional, just-the-vibe) vs. heavy "EQ-replica" (parchment rows, embedded item-icon chrome, full sidebar reskin)?
4. **Performance trade-offs:** every `IMAGE()` formula is a Sheets-side fetch. At what density does a heavy-asset view tab start to feel sluggish on a 12-guildie / ~120-tab workbook?
5. **Filter UX in EQ aesthetic:** Sheets' built-in filter view chrome is unstylable. Does an EQ-themed view tab clash visually with the standard Sheets filter UI? If so, do we surface filtering through the SquireBot custom menu instead?

## Scope estimate

**Medium.** Affects view-tab styling code (Apps Script `setBackground` / `setFontColor` / `setBorder` / `setNote` / `IMAGE()` calls), the HtmlService sidebar HTML+CSS, and possibly an `onOpen` custom-menu styling pass. Adds a half-phase of design exploration (mockup generation + decision) before the view-tab build phase. Low technical risk; main cost is the iteration loop on aesthetic choices.

If "minimalist" wins, this collapses to Small (palette + font + border decisions only). If "heavy" wins, it could expand to Large if asset curation becomes its own workstream.

## Related references

- `.planning/research/ARCHITECTURE.md` § View Tabs — full schema for `view`, `gear_check`, `spell_check`, `bank`
- `.planning/research/ARCHITECTURE.md` § Flow D — sidebar interaction flow
- `.planning/research/STACK.md` — confirms HtmlService for sidebar (no other UI framework)
- `.planning/research/FEATURES.md` § MVP Definition — aesthetic is not in v1 MVP; this is polish layer
- `CLAUDE.md` — locks the consolidated-mega-tab decision (no per-character view tabs); aesthetic must work for filterable mega-tabs with a leading `Char` column
- `README.md` § Project status — Phase 3 is the next major milestone after Phase 2 soak completes
