---
name: Sheet uses consolidated filterable view tabs (not per-character views)
description: Critical schema decision — per-character view tabs would exceed Google's 200-tab limit at guild scale; never propose per-character view layouts
type: project
originSessionId: 0f5dc45a-4a2f-4d87-8a75-2502ff440f06
---
The SquireBot workbook uses **consolidated filterable view tabs** with a leading `Char` column, NOT per-character view tabs.

**Why:** 12 guildies × ~10 characters × ~5 view types ≈ 600 tabs would breach Google Sheets' hard 200-tab/workbook limit. Verified during research synthesis 2026-04-30. Override of the per-character view layout originally proposed in ARCHITECTURE.md.

**How to apply:**
- Landing tabs ARE per-character (`inv:<CharName>`, `spell:<CharName>` — ~120 tabs total, comfortable). Keep this.
- View tabs are consolidated: `view`, `gear_check`, `spell_check`, `bank`. Each has a `Char` column and dropdown filters.
- Search is a HtmlService sidebar (not a tab) joining across all `inv:*`.
- If you ever see a plan or doc proposing `view:<Char>`, `gear_check:<Char>`, or any per-character view tab — that's the superseded architecture. Reject and reroute to consolidated tabs.
