---
sketch: 001
name: app-shell-5tab-nav
question: "What's the right top-level frame for the 5 tabs (Characters/Inventory/Banks/Wishlist/Settings), the per-tab search, and the character list?"
winner: "A"
tags: [nav, ia, shell]
decisions:
  - "Variant A (top tabs) chosen 2026-06-17 — matches the spec's 'tabs along the top'."
  - "Notifications move OUT of Settings and onto the WISHLIST tab — every alert is a wishlist-item ping, so the unread badge + alert inbox + ping prefs belong with the wishlist. Settings keeps Theme / Watcher Codes / Set Class & Level / My Characters / Admin."
---

# Sketch 001: App Shell & 5-Tab Nav

## Design Question
Your spec says "each user sees a series of tabs along the top: Characters, Inventory, Banks, Wishlist, and Settings." This sketch tries three structural frames for that, and shows how the **Characters** tab's list + search behaves inside each (your characters first A-Z → guild → banks/bots; search matches your characters first). The four other tabs are stubs that point at their own sketch.

## How to View
open .planning/sketches/001-app-shell-5tab-nav/index.html

(Use the top bar to switch variants A/B/C. Click tabs, type in the search, hover/click a character row. Bottom-right toolbar swaps theme + viewport.)

## Variants
- **A: Top tabs** — horizontal tab strip under the wordmark, exactly as the spec describes ("tabs along the top"). Per-tab search + list below. The path of least resistance and the closest match to your words.
- **B: Left sidebar** — the 5 tabs as a vertical rail with identity pinned at the bottom; content fills the right. More label room and a calmer header; costs horizontal space.
- **C: Search-forward** — top tabs, but each tab leads with a large hero search that previews matches as you type (your characters ranked first), list below. Leans into the "search prioritizes your characters" behavior you specified for every tab.

## What to Look For
- Does the tab set read as the primary navigation, or does it compete with the per-tab search?
- Where should the unread/notifications signal live now that Notifications folds into Settings (note the Settings badge in A/B)?
- Sidebar (B) vs top tabs (A): which suits a 5-tab app that will also hold dense grids?
- In C, is the always-present hero search worth the vertical space on every tab, or only on Inventory/Characters?
- Identity + sign-out placement (top-right "Joe ▾" vs sidebar footer).
