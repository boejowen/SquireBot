import esbuild from 'esbuild';
import { readFileSync } from 'node:fs';

const pkg = JSON.parse(readFileSync('./package.json', 'utf8'));

// Apps Script V8 has no ES modules at runtime — every top-level function
// must be a global. esbuild bundles to an IIFE bound to AppsScript; the
// footer below re-exports each trigger as a top-level global so Apps
// Script's trigger system can find them by name.
const TRIGGER_GLOBALS = [
  'refreshPigparse',
  'refreshWikiItems',
  'onChange',
  'onOpen',
  'buildView',
  'buildBank',
  'setTheme',
  'installTriggers',
  'showThemePickerModal',
];

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
