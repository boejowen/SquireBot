// Phase 5 plan 05-03 task 2 — vitest scenarios for the search sidebar
// trigger. Coverage per plan <behavior>:
//   Test 1   sidebar opens at 300px with locked title + body strings
//   Test 2   themed render injects CSS custom properties from THEMES
//   Test 3   sheets-default render emits NO :root token block
//   Test 4   getSearchInitialData returns chars + slots + recent
//   Test 5   runSearch passthrough to searchIndex.runSearch
//   Test 6   pushRecentSearchCall passthrough to searchIndex.pushRecentSearch
//   Test 7   XSS escape — file-grep level (every interpolation uses escapeHtml)
//   Test 8   onOpen menu wires the Search… item to showSearchSidebar

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { resetMocks, makeSheet, seedMeta, type MockState } from './test-helpers';

const CHAR_OWNER_HEADERS = [
  'char_name', 'owner_email', 'display_name', 'discord_handle',
  'class', 'level', 'is_bank_toon', 'is_hidden', 'is_removed',
  'first_seen', 'last_seen', 'server', 'watcher_version', 'race',
];
const INV_HEADERS = ['Location', 'Name', 'ID', 'Count', 'Slots', '_uploaded_at'];

function seedCharOwner(state: MockState, chars: string[]): void {
  const rows = chars.map((c) => [
    c, 'x@example.com', '', '', 'SHD', 60, 'FALSE', 'FALSE', 'FALSE',
    '2026-04-01T00:00:00Z', '2026-05-09T00:00:00Z', 'blue', '0.4.0', 'IKS',
  ]);
  state.sheets.set('_char_owner', makeSheet('_char_owner', CHAR_OWNER_HEADERS, rows));
}

function seedInv(state: MockState, char: string, rows: unknown[][]): void {
  state.sheets.set(`inv:${char}`, makeSheet(`inv:${char}`, INV_HEADERS, rows));
}

// We import each test fresh from the module under test. resetModules is
// not strictly required since the trigger reads globals at call time.
async function loadTrigger() {
  // Import dynamically so vi.mock can apply per-describe.
  return await import('../triggers/showSearchSidebar');
}

describe('showSearchSidebar — open + width + title + body', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
    seedCharOwner(state, ['Findom']);
    seedInv(state, 'Findom', [
      ['HEAD-1', 'Bone Helm', 1234, 1, 0, '2026-05-11T00:00:00Z'],
    ]);
  });

  it('Test 1 — opens 300px-wide sidebar with locked title + body strings', async () => {
    const { showSearchSidebar } = await loadTrigger();
    showSearchSidebar();
    const captured = (state as MockState & { lastSidebar?: { _html: string; _title: string; _width: number } }).lastSidebar;
    expect(captured).toBeDefined();
    expect(captured!._width).toBe(300);
    expect(captured!._title).toBe('SquireBot — Search');
    expect(captured!._html).toContain('Search');           // h3 title
    expect(captured!._html).toContain('Item name…');       // placeholder
    expect(captured!._html).toContain('Searching…');       // loading state
    expect(captured!._html).toContain('Did you mean:');    // fuzzy state
    expect(captured!._html).toContain('Recent:');          // footer label
    expect(captured!._html).toContain('Results may be up to 60 seconds stale.');  // SEARCH-03 affordance
  });
});

describe('showSearchSidebar — theme injection', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedCharOwner(state, ['Findom']);
    seedInv(state, 'Findom', []);
  });

  it('Test 2 — themed (minimalist) emits :root token block with THEMES values', async () => {
    seedMeta(state, [['schema_version', '3'], ['theme', 'minimalist']]);
    const { showSearchSidebar } = await loadTrigger();
    showSearchSidebar();
    const captured = (state as MockState & { lastSidebar?: { _html: string } }).lastSidebar;
    const html = captured!._html;
    // The minimalist theme tokens (lib/themes.ts:54-59) — match a few
    // distinctive values to confirm the block was server-rendered.
    expect(html).toContain('--bg: #f5f5f5');         // theme.headerBg
    expect(html).toContain('--bg-row: #fafafa');     // theme.rowAltBg
    expect(html).toContain('--fg: #222222');         // theme.rowFg
    expect(html).toContain('--accent-bg: #e0e0e0');  // theme.accentBg
    expect(html).toContain('--font-body: Inter, Arial, sans-serif');
  });

  it('Test 3 — sheets-default emits NO :root color token block', async () => {
    seedMeta(state, [['schema_version', '3'], ['theme', 'sheets-default']]);
    const { showSearchSidebar } = await loadTrigger();
    showSearchSidebar();
    const captured = (state as MockState & { lastSidebar?: { _html: string } }).lastSidebar;
    const html = captured!._html;
    // The themed-block uses ' --bg: <color>;' (note: lowercased theme
    // color hex with no #fafafa color). sheets-default fallback block
    // uses different formatting (compact, all on one line). Assert the
    // distinguishing absence: no '--bg: #f5f5f5' (that's the
    // minimalist token).
    expect(html).not.toContain('--bg: #f5f5f5');
    expect(html).not.toContain('--bg: #3a2616');  // vanilla
    // And the fallback compact block IS present for spacing tokens:
    expect(html).toContain('--space-xs:4px');  // fallback block uses compact syntax
  });
});

describe('getSearchInitialData', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3']]);
    seedCharOwner(state, ['Abulus', 'Findom', 'Slampeach']);
    seedInv(state, 'Abulus', []);
    seedInv(state, 'Findom', []);
    seedInv(state, 'Slampeach', []);
  });

  it('Test 4 — returns chars (sorted, is_removed filtered) + slots + recent', async () => {
    // Pre-seed recent cache.
    (state as MockState & { cache: Map<string, { value: string; expiresAt: number }> })
      .cache.set('squirebot:search:recent', {
        value: JSON.stringify(['q1', 'q2']),
        expiresAt: Date.now() + 60_000,
      });
    const { getSearchInitialData } = await loadTrigger();
    const out = getSearchInitialData();
    expect(out.chars).toEqual(['Abulus', 'Findom', 'Slampeach']);
    expect(out.slots).toContain('HEAD');
    expect(out.slots).toContain('GENERAL');
    expect(out.slots.length).toBeGreaterThanOrEqual(20);
    expect(out.recent).toEqual(['q1', 'q2']);
  });
});

describe('runSearch + pushRecentSearchCall passthroughs', () => {
  let state: MockState;
  beforeEach(() => {
    state = resetMocks();
    seedMeta(state, [['schema_version', '3']]);
    seedCharOwner(state, ['Findom']);
    seedInv(state, 'Findom', [
      ['HEAD-1', 'Bone Helm', 1234, 1, 0, '2026-05-11T00:00:00Z'],
    ]);
  });

  it('Test 5 — runSearch passthrough returns the searchIndex result envelope', async () => {
    const { runSearch } = await loadTrigger();
    const out = runSearch('bone', 'any', 'any');
    expect(out.groups.length).toBe(1);
    expect(out.groups[0].itemName).toBe('Bone Helm');
    expect(typeof out.durationMs).toBe('number');
  });

  it('Test 6 — pushRecentSearchCall populates CacheService.recent', async () => {
    const { pushRecentSearchCall } = await loadTrigger();
    pushRecentSearchCall('q1');
    pushRecentSearchCall('q2');
    const cache = (state as MockState & { cache: Map<string, { value: string; expiresAt: number }> }).cache;
    const raw = cache.get('squirebot:search:recent');
    expect(raw).toBeDefined();
    expect(JSON.parse(raw!.value)).toEqual(['q2', 'q1']);
  });
});

describe('XSS defense + menu wiring (file-grep checks)', () => {
  it('Test 7 — every interpolation in the inline script uses escapeHtml or Number()', () => {
    const file = readFileSync(
      path.resolve(__dirname, '..', 'triggers', 'showSearchSidebar.ts'),
      'utf8',
    );
    // The inline escapeHtml helper must be present.
    expect(file).toContain('function escapeHtml');
    // Every '+ <var> +' style concatenation against user-derived data
    // must funnel through escapeHtml. Grep for known user-derived
    // identifiers and assert each appears wrapped by escapeHtml(...)
    // somewhere in the file.
    const userVars = ['g.itemName', 'g.wikiUrl', 'g.wikiSummary', 'row.char', 'row.location'];
    for (const v of userVars) {
      const wrappedPattern = new RegExp(`escapeHtml\\(\\s*${v.replace('.', '\\.')}\\s*\\)`);
      expect(wrappedPattern.test(file)).toBe(true);
    }
    // count is rendered via Number() coercion (numeric, no escape needed)
    expect(file).toContain('Number(row.count)');
  });

  it('Test 8 — onOpen.ts wires Search… → showSearchSidebar exactly once', () => {
    const file = readFileSync(
      path.resolve(__dirname, '..', 'triggers', 'onOpen.ts'),
      'utf8',
    );
    const matches = file.match(/addItem\('Search…',\s*'showSearchSidebar'\)/g);
    expect(matches).not.toBeNull();
    expect(matches!.length).toBe(1);
  });
});

// Keep vi from complaining about unused import.
void vi;
