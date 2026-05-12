// onOpen — Phase 3 plan 03-04 task 5; Phase 7 plan 07-03 added lazy
// admin-bootstrap + 2 new menu items (Manage Admins…, Initialize
// Admin Allowlist (manual)).
//
// Adds the SquireBot custom menu when the workbook opens. Phase 3
// shipped a minimal theme picker modal; Phase 5 replaced it with the
// polished picker. Phase 7 adds the admin-allowlist surface.

import { bootstrapGuildAdmins } from '../lib/admin';
import { log } from '../lib/log';

export function onOpen(): void {
  // Phase 7 plan 07-03 (D-01): lazy admin bootstrap on every workbook
  // open. Errors NEVER throw out of onOpen (would break the menu for
  // everyone). bootstrapGuildAdmins is idempotent + lock-wrapped
  // internally + returns silently with reason='lock_busy' on contention
  // — the wrapping try/catch is belt-and-suspenders for unexpected
  // primitive failures.
  try {
    bootstrapGuildAdmins();
  } catch (err) {
    log('warn', 'onOpen.bootstrap_failed', { error: String(err) });
  }

  SpreadsheetApp.getUi()
    .createMenu('SquireBot')
    .addItem('Install Triggers', 'installTriggers')
    .addSeparator()
    .addItem('Rebuild Views Now', 'buildView')
    .addItem('Refresh PigParse Now', 'refreshPigparse')
    .addItem('Refresh Wiki Items Now', 'refreshWikiItems')
    .addItem('Refresh Wiki Spells Now', 'refreshWikiSpells')
    .addItem('Refresh Wiki Gear Tier Now', 'refreshWikiGearTier')
    .addItem('Run Cell-Count Check Now', 'monitorCellCount')
    .addSeparator()
    .addItem('Set Character Info…', 'showCharInfoSidebar')
    .addItem('Set Bank Coin…', 'showBankCoinSidebar')
    .addItem('Search…', 'showSearchSidebar')
    .addItem('Evict Guildie…', 'showEvictionSidebar')
    .addItem('Manage Admins…', 'showAdminMgmtSidebar')
    .addItem('Set Theme…', 'showThemePickerModal')
    .addSeparator()
    .addItem('Run Migration (v=3)', 'migrateToV3')
    .addItem('Run Migration (v=2 legacy)', 'migrateToV2')
    .addSeparator()
    .addItem('Initialize Admin Allowlist (manual)', 'bootstrapGuildAdminsManual')
    .addToUi();
}

// Minimal modal — 6 themes as plain links. Phase 5 polishes per
// docs/design/mockups/eq-aesthetic-picker.html.
export function showThemePickerModal(): void {
  const html = HtmlService.createHtmlOutput(`
    <div style="font-family:sans-serif;padding:1.5em;color:#222">
      <h2 style="margin-top:0">Choose a theme</h2>
      <p style="color:#555;font-size:0.9em">Click a theme to apply it. The view + bank tabs rebuild automatically.</p>
      <ul style="list-style:none;padding-left:0;line-height:1.8">
        <li><a href="#" onclick="apply('vanilla');return false">Vanilla — browns + golds, Cinzel</a></li>
        <li><a href="#" onclick="apply('kunark');return false">Kunark — jungle greens + copper</a></li>
        <li><a href="#" onclick="apply('velious');return false">Velious — icy blues + silver</a></li>
        <li><a href="#" onclick="apply('minimalist');return false"><b>Minimalist (default)</b> — muted, Inter</a></li>
        <li><a href="#" onclick="apply('heavy');return false">Heavy — parchment + stone, MedievalSharp</a></li>
        <li><a href="#" onclick="apply('sheets-default');return false">Sheets default — no styling at all</a></li>
      </ul>
      <p style="color:#888;font-size:0.8em;margin-top:2em">Polished 6-tile picker ships in Phase 5.</p>
      <script>
        function apply(key) {
          google.script.run
            .withSuccessHandler(() => google.script.host.close())
            .withFailureHandler((err) => alert('Failed: ' + err.message))
            .setTheme(key);
        }
      </script>
    </div>
  `).setWidth(420).setHeight(420);
  SpreadsheetApp.getUi().showModalDialog(html, 'SquireBot — Theme');
}
