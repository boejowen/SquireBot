# EverQuest-Aesthetic Theme for the Shared Workbook

> **Status:** Architectural direction LOCKED 2026-05-08. Ship all four mockup themes as built-in options with a polished theme picker. Default theme + per-theme refinements remain to be decided during Phase 3.
>
> **Decision owner:** Project owner — picks the default theme and signs off on per-theme refinements during Phase 3 implementation.
>
> **Planted:** 2026-05-08 during Phase 2 soak. Refined to "all four + picker" 2026-05-08 after preview mockup review.

## Why this matters

The shared Google Sheets workbook is the headline UX surface for SquireBot. Every guildie opens it daily — it's where they answer "what does my character still need, and where in the guild is it?"

An aesthetic that's reminiscent of classic EverQuest UI converts the workbook from "yet another spreadsheet" into a *guild artifact* that feels native to the game the guild plays. This is morale-and-identity work, not feature work — the kind of polish that makes officers proud to hand the link to a new guildie ("you have to see what we built") and that subtle delight is what separates "tool we tolerate" from "tool we love."

SquireBot is a fan project for a Project 1999 (Classic EQ emulator) guild. The guild's identity is rooted in the original 1999–2002 era of the game. A workbook that visually echoes that era reinforces the project's belonging in that community.

**Why "all four" instead of picking one:** during preview-mockup review on 2026-05-08, all four candidate themes (Vanilla / Velious / Minimalist / Heavy) were judged worth shipping. Different guilds will have different tastes — a no-fuss officer cohort might want Minimalist; a Velious-cosplay guild might want Heavy. The marginal cost of building a 4-theme picker over a single-theme implementation is ~20–30% Phase 3 effort (vs. ~50–100% to retrofit it later), so the right call is to bake the picker in from day 1.

## The four built-in themes

Reference mockups: [docs/design/mockups/eq-aesthetic-preview.html](mockups/eq-aesthetic-preview.html). Open in any browser for a side-by-side comparison of all four with realistic SquireBot data (mock `gear_check` view tab + mock search sidebar per theme).

| Theme | Era | Intensity | Visual character |
|---|---|---|---|
| **A. Vanilla** | pre-Kunark (1999–2000) | Medium | Browns + golds, Cinzel + Crimson Text serif, "ye olde tavern" feel. Warm and grounded. |
| **B. Velious** | 2000–2001 | Medium | Icy blues + silver, Cinzel Decorative + IM Fell English, frosted accents. Matches the era your guild plays. |
| **C. Minimalist** | era-agnostic | Light | Muted EQ palette + Inter sans for body, Cinzel for accents only. "Stranger Things title style" — modern boutique tool that knows what EQ is. |
| **D. Heavy** | era-agnostic | Heavy | Parchment rows, beveled stone-panel headers, MedievalSharp display font, dark-red ink accents. Goes all-in on the guild-artifact feel. |

Per-theme color palettes, font choices, and visual notes are in the mockup HTML.

## Architectural design (LOCKED for Phase 3)

### Single source of truth: `_meta.theme`

A new row in the `_meta` dimension tab stores the active theme name. Default value: `minimalist` (lowest "yikes that's a lot" risk for a non-EQ-purist taking their first look at a fresh workbook). One workbook = one theme = one consistent view for the whole guild.

```
_meta:
  schema_version    | 1
  canonical_id      | <id>
  bank_toon_name    | <name>
  ...
  theme             | "minimalist"   ← NEW
```

### Theme registry in Apps Script

A TypeScript map keyed by theme name, holding all the design tokens each theme needs. Lives in `theme/registry.ts` (or similar — actual path is a Phase 3 plan decision). Rough shape:

```ts
type ThemeTokens = {
  // colors
  bg: string;
  panel: string;
  border: string;
  text: string;
  accent: string;
  statusOk: string;
  statusMissing: string;
  statusOther: string;
  // fonts (Google Fonts names)
  fontHeader: string;
  fontBody: string;
  // sidebar-only (CSS shadow/gradient strings)
  sidebarPanelEffect: string;
  sidebarHeaderEffect: string;
};

const THEMES: Record<ThemeName, ThemeTokens> = {
  vanilla:    { bg: '#2a1f15', accent: '#d4af37', fontHeader: 'Cinzel', ... },
  velious:    { bg: '#0f1729', accent: '#a8c5e0', fontHeader: 'Cinzel Decorative', ... },
  minimalist: { bg: '#1f1f1d', accent: '#b8915c', fontHeader: 'Cinzel', ... },
  heavy:      { bg: '#c9b072', accent: '#6b1a1a', fontHeader: 'MedievalSharp', ... },
};
```

### Theme-aware view-tab builders

Instead of hardcoded `range.setBackground('#2a1f15')`, builders read from the active theme:

```ts
const t = THEMES[getActiveTheme()];
range.setBackground(t.panel);
range.setFontFamily(t.fontHeader);
```

Effort: ~10–20% extra build-time complexity vs. a single-theme implementation. Worth it to avoid the retrofit cost.

### Theme-aware sidebar via CSS custom properties

The HtmlService sidebar template emits a `<style>` block with CSS custom properties pulled from the active theme's tokens, then all CSS rules reference those properties:

```html
<style>
  :root {
    --bg: <%= theme.bg %>;
    --accent: <%= theme.accent %>;
    --font-header: <%= theme.fontHeader %>;
    /* ...one block per theme; the rest of the CSS is theme-neutral */
  }
  .panel { background: var(--bg); }
  h3 { color: var(--accent); font-family: var(--font-header); }
</style>
```

Single sidebar template serves all four themes. The mockup HTML is already structured this way — Phase 3 implementation can borrow heavily from it.

### Polished picker UX (LOCKED)

A custom-menu entry — **SquireBot → Settings → Theme…** — opens an HtmlService modal dialog with:

- A **2×2 grid of preview tiles**, one per theme. Each tile renders a miniature live preview of the theme (header strip + 3-row mock view-tab + 1-line mock sidebar snippet) using the same CSS-custom-property approach.
- Tile click selects + highlights that theme.
- "Apply" button at the bottom: writes the chosen theme name to `_meta.theme`, closes the dialog, kicks off the rebuild.
- "Cancel" closes without changes.

Why polished over cheap (`_meta` cell + manual rebuild): the originator is making a one-time aesthetic decision they'll live with for months. A picker with live previews is the right ergonomics — the cheap approach (typing a string into a cell + hoping it spelled right + manually triggering rebuild) is hostile UX for a low-frequency-but-high-stakes choice.

### Theme-change rebuild trigger

When `_meta.theme` changes (detected by `onEdit` trigger AND by direct write from the picker dialog), all four view tabs (`view`, `gear_check`, `spell_check`, `bank`) get rebuilt with the new theme's tokens. At v1 scale (~120 landing tabs feeding 4 consolidated mega-tabs) this is seconds, well within Apps Script's 6-min execution cap. Chunk-and-resume not needed for v1.

### What's NOT in scope for v1

- **Custom user-defined themes** — out. Four built-ins is enough.
- **Per-user themes** — out. Workbook-wide theme is the right model for a shared guild artifact (everyone sees the same view, which is what makes officer-vs-guildie conversations work).
- **Theme inheritance / mix-and-match** — out (e.g., "Vanilla colors with Velious fonts" is not a thing).
- **Animated theme transitions** — out.
- **Light/dark mode auto-switching** — out (all four themes are intentionally dark-leaning; high-contrast is a separate accessibility decision).

## Technical context (carried over from initial seed)

### What works in Google Sheets cells

- **Color palette** — full control over cell background, font color, border color
- **Custom fonts** via Google Fonts integration (Cinzel, Cinzel Decorative, IM Fell English, MedievalSharp, Crimson Text, Inter — all confirmed working in cells per Phase 3 research, used in the mockup HTML)
- **Conditional formatting** — red MISSING / green OK / amber tier-progression maps cleanly to EQ HP/mana color language
- **Inline images** via `IMAGE()` formula — class icons, item icons, banner row at the top of view tabs
- **Drawings** (Insert > Drawing) — freeform decorative elements, can be positioned over the grid

### What does NOT work in Google Sheets cells

- ❌ **No per-cell or per-tab background images.** No stone-texture tiling behind the grid. **Affects Heavy theme:** the parchment-cell look from the mockup degrades to a solid warm tan (`#c9b072`) in real Sheets. Sidebar half of Heavy theme remains 100% faithful since HtmlService has no such limit.
- ❌ **No global theme/skin** that auto-applies to every new tab. Each tab needs styling at build time by Apps Script — which is exactly what the registry pattern handles.
- ❌ **Sheets' built-in filter view chrome** (the funnel icon + dropdowns) is unstylable. Only matters visually for Heavy theme; the other three themes are subtle enough that the standard filter chrome doesn't clash.

### HtmlService sidebar — full HTML+CSS freedom

The HtmlService search sidebar is where each theme's aesthetic carries the heaviest visual lift. Beveled stone panels, gold trim, parchment rows, serif fonts, custom icons — all standard CSS work via the CSS-custom-properties pattern above.

### Net split

- **Cell-based view tabs** carry ~60% of each theme's feel (color, font, borders, inline icons). All four themes translate well except Heavy's parchment, which falls back to solid color.
- **HtmlService sidebar** carries the remaining ~40% with full visual freedom for all four themes.

## Asset sourcing (LOCKED)

### Default: P99 wiki assets

[`wiki.project1999.com`](https://wiki.project1999.com/) hosts CC-BY-SA-licensed item icons, class icons, and zone screenshots. Default asset source. Attribution required (single line in the workbook footer or sidebar credits).

### Daybreak chrome bitmaps: NOT used

Original Sony / Daybreak EQ UI chrome bitmaps are technically Daybreak IP. We're recreating the *aesthetic* with original CSS / SVG / styling rather than copying chrome bitmaps verbatim. This decision is now LOCKED to avoid drifting into the gray area.

If a future guild operator forks SquireBot and wants to ship the literal Daybreak chrome under the "tolerated for non-commercial fan use" P99-community norm, that's their fork's call — not the upstream default.

## Open questions remaining for Phase 3

Most of the original open questions are now resolved by "ship all four." The remaining ones:

1. **Default theme.** Recommendation: Minimalist. Rationale: lowest "yikes that's a lot" risk for a non-EQ-purist taking their first look at a brand-new workbook; the originator can switch to a more assertive theme via the picker if they want. Open to override.
2. **Per-theme refinements during implementation.** The mockup HTML colors/fonts are starting points, not locked values. Phase 3 implementation will inevitably tweak (e.g., contrast against the conditional-formatting yellow may need a palette adjustment). Reviewer: project owner, sign-off per theme during Phase 3.
3. **Performance ceiling for `IMAGE()`-heavy themes.** The Heavy theme implies more inline images (class icons, decorative dividers) than the other three. Phase 3 needs to validate that a fully-loaded `view` tab with Heavy theme renders snappily on a 12-guildie / ~120-tab workbook. If not, drop image density on Heavy or warn the user via a tooltip on the picker tile ("Heavy: may render slower on larger workbooks").
4. **Filter UX clash check.** Phase 3 should test the standard Sheets filter funnel against each theme. If Heavy clashes badly enough, surface filtering through the SquireBot custom menu instead (theme-aware filter UI).
5. **Picker preview tile fidelity.** The picker dialog's mini-preview tiles need to match the actual rendered output closely enough that the originator's choice isn't "surprising" when they first see the rebuilt views. Phase 3: build the picker tiles AFTER the view-tab styling is implemented so the preview can sample real CSS.

## Scope estimate

**Medium-Large.** The "all four + polished picker" approach is roughly 20–30% more Phase 3 effort than a single-theme implementation:

- Theme registry definition: ~100–200 LOC of declarative tokens × 4 themes = ~400–800 LOC of pure data
- Theme-aware view-tab builders (token lookups instead of hardcoded values): +10–20% over single-theme builder complexity
- Sidebar CSS via custom properties: roughly equal effort to a single-theme sidebar (the work is structuring the properties, then 4× variable definitions)
- Theme-change rebuild trigger + onEdit handler: ~50 LOC
- Polished picker dialog (HtmlService modal, 2×2 preview tiles, apply/cancel): ~2–4 hours of focused work

Net: instead of "Medium" for one theme, this is "Medium-Large" for four-themes-plus-picker. Well worth the ~25% surcharge to ship the choice.

## Related references

- [`docs/design/mockups/eq-aesthetic-preview.html`](mockups/eq-aesthetic-preview.html) — side-by-side preview of all four themes (MUST READ before Phase 3 picks default + per-theme refinements)
- `.planning/research/ARCHITECTURE.md` § View Tabs — full schema for `view`, `gear_check`, `spell_check`, `bank` (tabs the registry styles)
- `.planning/research/ARCHITECTURE.md` § Flow D — sidebar interaction flow
- `.planning/research/STACK.md` — confirms HtmlService for sidebar (no other UI framework)
- `.planning/research/FEATURES.md` § MVP Definition — aesthetic is not in v1 MVP; this is polish layer landing in Phase 3
- `CLAUDE.md` — locks the consolidated-mega-tab decision (no per-character view tabs); themes must work for filterable mega-tabs with a leading `Char` column
- `README.md` § Project status — Phase 3 is the next major milestone after Phase 2 soak completes
