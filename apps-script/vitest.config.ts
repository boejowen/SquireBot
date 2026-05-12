import { defineConfig } from 'vitest/config';

// Phase 8 plan 08-01 (TEST-01): JSDOM is the default test environment so
// sidebar inline-JS tests (Plan 08-02) and any future DOM-touching test get a
// DOM by default. Existing tests that mock Apps Script globals
// (SpreadsheetApp, CacheService, etc.) treat JSDOM as a no-op. Per CONTEXT
// D-01, per-test `// @vitest-environment node` overrides remain available but
// no current test needs one.
export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['src/__tests__/**/*.test.ts'],
    exclude: ['node_modules', 'dist', 'src/__fixtures__'],
    globals: false,
  },
});
