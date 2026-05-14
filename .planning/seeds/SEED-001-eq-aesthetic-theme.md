---
id: SEED-001
status: promoted
planted: 2026-05-08
promoted: 2026-05-08
promoted_to: docs/design/eq-aesthetic-theme.md
planted_during: v1.0 / Phase 2 (Watcher Robustness + Schema Lock — soak in progress)
trigger_when: Phase 3 (Apps Script + view tabs + HtmlService sidebar) starts being planned or built — specifically before any view-tab styling decisions get locked in
scope: Medium
---

> **PROMOTED 2026-05-08** — canonical version now at `docs/design/eq-aesthetic-theme.md` (tracked in git).
> **REFINED 2026-05-08 (later same day)** — architectural direction LOCKED to "ship five themed options (Vanilla / Kunark / Velious / Minimalist / Heavy) + a `sheets-default` opt-out with a polished 3×2 HtmlService picker." Default theme DECIDED: Minimalist. Per-theme color/font refinements still to be decided during Phase 3. See the design doc for full architectural design (theme registry, `_meta.theme`, sidebar CSS-custom-properties pattern, picker UX, `clearTheme` helper).
> This seed file remains in place so `/gsd-new-milestone` and `/gsd-discuss-phase` scans of `.planning/seeds/` still surface it during Phase 3 trigger matching. The docs/design/ version is the source of truth for content.

# SEED-001: EQ-aesthetic theme for shared Google Sheet workbook

## Why This Matters

The shared workbook is the headline UX surface for the entire product — every guildie opens it daily. An aesthetic that's reminiscent of classic EverQuest UI (color palette, possibly original art assets) would convert the workbook from "yet another spreadsheet" into a *guild artifact* that feels native to the game the guild plays. This is morale-and-identity work, not feature work. It's the kind of polish that makes officers proud to hand the link to a new guildie ("you have to see what we built"), and that subtle delight is what separates "tool we tolerate" from "tool we love."

Practical case: SquireBot is a fan project for a Project 1999 (Classic EQ emulator) guild. The guild's identity is rooted in the original 1999-2002 era of the game. A workbook that visually echoes that era reinforces the project's belonging in that community.

## When to Surface

**Trigger:** When Phase 3 (Apps Script + view tabs + HtmlService search sidebar) starts being planned or built — specifically *before* any view-tab styling decisions get locked in.

This seed should be presented during `/gsd-new-milestone` or `/gsd-discuss-phase` for Phase 3 when the milestone scope matches any of these conditions:

- Apps Script work begins (TypeScript via clasp + esbuild scaffold)
- View tabs (`view`, `gear_check`, `spell_check`, `bank`) get scaffolded or styled
- HtmlService search sidebar gets implemented
- Any conditional formatting / cell-coloring decision is on the table for view tabs
- Custom-menu (`onOpen` SquireBot menu) work begins

If Phase 3 ends without this seed being addressed, surface it during Phase 4 (polish/observability) as a candidate for visual-polish work.

## Deliverable Expectation

**The user explicitly asked: "ultimately, I'd like you to show me a few different options I can choose between."**

When this seed is woken, do NOT pick a single direction unilaterally. Produce **2–4 distinct aesthetic mockups or descriptions** that span the design space — e.g.:

- Option A: vanilla pre-Kunark (browns + golds, simple serif)
- Option B: Velious-era (icy blues + silver + heavier panels)
- Option C: minimalist EQ-flavored (muted EQ palette, subtle, "EQ-inspired" rather than "EQ-replicated")
- Option D: heavy aesthetic — embedded class icons, parchment-look rows, full sidebar reskin

Present them side-by-side (small HTML mockups would be ideal — `/gsd-sketch` is the right tool) and let the user pick a direction before committing to view-tab styling code.

## Scope Estimate

**Medium** — Affects view-tab styling code (Apps Script `setBackground`/`setFontColor`/`setBorder`/`setNote`/`IMAGE()` calls), the HtmlService sidebar HTML+CSS, and possibly an onOpen custom-menu styling pass. Adds a half-phase of design exploration (mockup generation + user choice) before the view-tab build phase. Low technical risk; main cost is the iteration loop on aesthetic choices.

If the user picks the "minimalist" option, this could collapse to Small (palette + font + border decisions only). If the user picks "heavy," it could expand to Large if asset-curation becomes its own workstream.

## Technical Context (worth preserving)

**What works in Google Sheets cells:**
- Color palette (cell background, font color, border color) — full control
- Custom fonts via Google Fonts integration
- Conditional formatting (red MISSING / green OK / amber tier — maps to EQ HP/mana color language)
- Inline images via `IMAGE()` formula (class icons, item icons, banner row)
- Drawings (Insert > Drawing) — freeform decorative elements

**What does NOT work in Google Sheets cells:**
- ❌ No per-cell or per-tab background images (no stone-texture tiling)
- ❌ No global theme/skin applied automatically — each tab needs styling at build time

**HtmlService sidebar — full HTML+CSS freedom.** This is where the EQ aesthetic can carry the heaviest visual lift. Beveled stone panels, gold trim, parchment rows, serif fonts — all standard CSS work. The sidebar is the search UI per the architecture (Flow D in `ARCHITECTURE.md`), so it's a high-value place to invest in aesthetic.

**Net:** Cell-based styling gets ~60% of the EQ feel; the sidebar carries the remaining ~40%.

## Asset Sourcing (locked-in considerations)

- **P1999 wiki** (`wiki.project1999.com`) hosts CC-BY-SA-licensed item icons, class icons, and zone screenshots. Clean to use under the wiki's license terms (attribution required).
- **Original Sony/Daybreak EQ UI chrome bitmaps** (the actual stone-panel skins, button textures, EQ logo) are technically Daybreak IP. Using them in a non-commercial fan tool for a single guild is standard P99-community practice — P99 itself runs on Daybreak's IP under tolerated terms — but worth a deliberate decision rather than drifting into it.
- **Default recommendation:** lean on P99-wiki-licensed assets and recreate the *aesthetic* with original styling rather than copying chrome bitmaps verbatim. Revisit if the user explicitly wants the literal Daybreak chrome.

## Open Questions for Phase 3

These are NOT to answer now — they should be raised when this seed surfaces:

1. **Era preference:** vanilla pre-Kunark (browns + golds, simple) vs. Kunark (jungle greens) vs. Velious (icy blues + silver). Affects palette decisively.
2. **Asset boundary:** lean on P99-wiki assets only, OR allow Daybreak chrome bitmaps in spirit of P99-community practice?
3. **Aesthetic intensity:** subtle "EQ-flavored" (muted, professional, just-the-vibe) vs. heavy "EQ-replica" (parchment rows, embedded item-icon chrome, full sidebar reskin)?
4. **Performance trade-offs:** every `IMAGE()` formula is a Sheets-side fetch. At what density does a heavy-asset view tab start to feel sluggish on a 12-guildie / ~120-tab workbook?
5. **Filter UX in EQ aesthetic:** Sheets' built-in filter view chrome (the funnel icon + dropdowns) is unstylable. Does an EQ-themed view tab clash visually with the standard Sheets filter UI? If so, do we surface filtering through the SquireBot custom menu instead?

## Breadcrumbs

Related code and decisions found in the current codebase:

- `.planning/research/ARCHITECTURE.md` § View Tabs (lines 265+) — full schema for view, gear_check, spell_check, bank
- `.planning/research/ARCHITECTURE.md` § Flow D (line 482+) — sidebar interaction flow
- `.planning/research/STACK.md` — confirms HtmlService for sidebar, no other UI framework
- `.planning/research/SUMMARY.md` — three-layer pancake architecture
- `.planning/research/FEATURES.md` § MVP Definition (line 151+) — aesthetic is not in v1 MVP; this is polish layer
- `CLAUDE.md` — locks the consolidated-mega-tab decision (no per-character view tabs); aesthetic must work for filterable mega-tabs
- `README.md` § Project status — Phase 3 is the next major milestone after Phase 2 soak completes

No view-tab styling code exists yet; Phase 3 is greenfield for aesthetic.

## Notes

- User flagged this on 2026-05-08 in mid-Phase-2-soak conversation, immediately after I described the planned UI shape. They explicitly deferred ("we don't have to figure it out now") and asked it be planted open-ended.
- The right tool for the eventual surfacing is likely `/gsd-sketch` (UI/design ideas with throwaway HTML mockups) — that command is purpose-built for exactly this "show me a few options" deliverable.
- This seed is `.planning/`-scoped (gitignored), so it lives only on the user's machine. If we want this finding to ship to the repo, the seed could later be promoted to a `docs/design/` artifact during Phase 3 planning.
