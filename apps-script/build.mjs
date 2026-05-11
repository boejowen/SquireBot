import esbuild from 'esbuild';
import { readFileSync, copyFileSync, mkdirSync } from 'node:fs';

const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

// clasp pushes everything under rootDir (./dist). The Apps Script
// manifest must be alongside Code.js — copy it in.
mkdirSync('dist', { recursive: true });
copyFileSync('appsscript.json', 'dist/appsscript.json');

// Apps Script V8 has no ES modules at runtime — every top-level function
// must be a global. esbuild bundles to an IIFE bound to AppsScript; the
// footer below re-exports each trigger as a top-level global so Apps
// Script's trigger system can find them by name.
//
// CRITICAL: every name listed here MUST also be exported from src/Code.ts,
// and vice versa. The CI assertion below catches divergence at build time
// (lesson from Phase 3 bug d0a2645 — migrateToV2 was Code.ts-exported
// but missing from TRIGGER_GLOBALS, causing "Script function not found"
// when called from the SquireBot menu).
const TRIGGER_GLOBALS = [
  // Phase 1+2: scaffold/heartbeat triggers run from the Go side; nothing
  // here needs them as globals.
  // Phase 3:
  'refreshPigparse',
  'refreshWikiItems',
  'onChange',
  'onOpen',
  'buildView',
  'buildBank',
  'setTheme',
  'installTriggers',
  'showThemePickerModal',
  'migrateToV2',
  // Phase 4 plan 04-01:
  'migrateToV3',
  'showCharInfoSidebar',
  'getCharsForForm',  // google.script.run callback from sidebar
  'saveCharInfo',     // google.script.run callback from sidebar
  // Phase 4 plan 04-02:
  'refreshWikiSpells',
  'buildSpellCheck',
  // Phase 4 plan 04-03:
  'refreshWikiGearTier',
  'buildGearCheck',
  // Phase 4 plan 04-04:
  'showBankCoinSidebar',
  'getBankCoinForForm',  // google.script.run callback from sidebar
  'saveBankCoin',        // google.script.run callback from sidebar
  'monitorCellCount',
  'protectBankCoinCells',  // callable from script editor for retro re-apply
  // Phase 5 plan 05-01:
  'weeklySchemaHealthcheck',
  'protectBankToonName',
  'hideAllSystemTabs',
  // Phase 5 plan 05-02:
  'weeklyStaleCharArchive',
  'weeklyEvictionArchive',
  'moveCharToArchive',
  // Phase 5 plan 05-03:
  'showSearchSidebar',
  'getSearchInitialData',
  'runSearch',
  'pushRecentSearchCall',
  'prewarmSearchCache',
];

// CI assertion: Code.ts exports MUST match TRIGGER_GLOBALS exactly.
assertExportsMatchGlobals();

const footer = TRIGGER_GLOBALS
  .map((name) => `function ${name}() { return AppsScript.${name}.apply(null, arguments); }`)
  .join('\n');

const watch = process.argv.includes('--watch');

const ctx = await esbuild.context({
  entryPoints: ['src/Code.ts'],
  bundle: true,
  format: 'iife',
  globalName: 'AppsScript',
  outfile: 'dist/Code.js',
  target: 'es2019',
  platform: 'neutral',
  define: { __VERSION__: JSON.stringify(pkg.version) },
  footer: { js: `\n// --- top-level globals for Apps Script trigger lookup ---\n${footer}\n` },
  logLevel: 'info',
});

if (watch) {
  await ctx.watch();
  console.log('watching apps-script/src for changes...');
} else {
  await ctx.rebuild();
  await ctx.dispose();
}

// ----- helpers -----

function assertExportsMatchGlobals() {
  const codeTs = readFileSync('src/Code.ts', 'utf8');
  const exportedNames = new Set();

  // Match `export { name1, name2 as alias2, ... }` (multi-line via /s).
  for (const m of codeTs.matchAll(/export\s+\{([^}]+)\}/g)) {
    for (const raw of m[1].split(',')) {
      const name = raw.trim().split(/\s+as\s+/)[0].trim();
      if (name) exportedNames.add(name);
    }
  }
  // Match `export function name(...)` and `export const name = ...`.
  for (const m of codeTs.matchAll(/^export\s+(?:function|const|let)\s+(\w+)/gm)) {
    exportedNames.add(m[1]);
  }
  // Match `export { name } from '...'` re-exports.
  for (const m of codeTs.matchAll(/export\s+\{([^}]+)\}\s+from\s+/g)) {
    for (const raw of m[1].split(',')) {
      const name = raw.trim().split(/\s+as\s+/)[0].trim();
      if (name) exportedNames.add(name);
    }
  }

  const globals = new Set(TRIGGER_GLOBALS);
  const missingFromGlobals = [...exportedNames].filter((n) => !globals.has(n));
  const missingFromExports = [...globals].filter((n) => !exportedNames.has(n));

  if (missingFromGlobals.length || missingFromExports.length) {
    console.error('\n❌  TRIGGER_GLOBALS / Code.ts exports out of sync.');
    if (missingFromGlobals.length) {
      console.error('  Exported by Code.ts but NOT in TRIGGER_GLOBALS:');
      missingFromGlobals.forEach((n) => console.error('    -', n));
      console.error('  → Apps Script will fail with "Script function not found" when these are invoked.');
    }
    if (missingFromExports.length) {
      console.error('  Listed in TRIGGER_GLOBALS but NOT exported by Code.ts:');
      missingFromExports.forEach((n) => console.error('    -', n));
      console.error('  → The footer wrapper will reference an undefined AppsScript.<name>.');
    }
    console.error('');
    process.exit(1);
  }
}
