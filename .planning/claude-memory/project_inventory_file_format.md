---
name: P99 inventory file format
description: The /outputfile inventory dump format and what it does NOT contain — pinned because the project depends on parsing it
type: project
originSessionId: 0f5dc45a-4a2f-4d87-8a75-2502ff440f06
---
P99's `/outputfile inventory` produces `<CharName>-Inventory.txt`, tab-separated with exactly five columns: **Location | Name | ID | Count | Slots**.

**Why:** Verified 2026-04-30 against Fanra wiki and P99 community parsing scripts; this is the column contract SquireBot's parser must target.

**How to apply:**
- Inventory parser must split on TAB, expect exactly 5 columns, and tolerate a header row.
- The file does NOT include platinum/gold/silver/copper — coin tracking for the guild bank character must come from a separate manual-entry path in the sheet, not from the file.
- `Location` values cover equipped slots (Charm, Ear, Head…), bag slots, and bank slots; full enumeration lives in the EQ inventory slot reference (eqemu docs / Fanra wiki).
- `ID` is the canonical EQ item ID and is the right primary key for joining against wiki gear-tier data; `Name` strings can shift over time but IDs are stable.
