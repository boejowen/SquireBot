// showBankCoinSidebar — Phase 4 plan 04-04 task 1.
//
// Manual coin entry sidebar. /outputfile inventory does not include
// platinum/gold/silver/copper totals, so the bank toon's owner enters
// them by hand here. Saved values land in _meta.bank_coin_pp/gp/sp/cp
// + bank_coin_last_updated (the last_updated row is created lazily on
// first save; not part of the Phase 2 _meta scaffold). buildBank
// renders a coin row at row 2 using these values.
//
// Range.protect() is applied to the four bank_coin_* value cells in
// _meta by protectBankCoinCells (called from migrateToV3 + installTriggers)
// — direct edits trigger a warning prompt; the sidebar's saveBankCoin
// is the supported entry point.

import { log } from '../lib/log';
import { protectBankCoinCells } from '../lib/migrations';
import { readMetaRows, writeMetaRow } from '../lib/sheet-helpers';
import { buildBank } from '../tabs/buildBank';

export interface BankCoinForm {
  pp: number; gp: number; sp: number; cp: number;
}

export function showBankCoinSidebar(): void {
  const html = HtmlService.createHtmlOutput(buildSidebarHtml())
    .setTitle('SquireBot — Bank Coin')
    .setWidth(320);
  SpreadsheetApp.getUi().showSidebar(html);
}

export function getBankCoinForForm(): BankCoinForm {
  const meta = readMetaRows('_meta');
  const get = (key: string): number => {
    const row = meta.find((r) => r.key === key);
    if (!row) return 0;
    const n = parseInt(row.value || '0', 10);
    return Number.isFinite(n) && n >= 0 ? n : 0;
  };
  return {
    pp: get('bank_coin_pp'),
    gp: get('bank_coin_gp'),
    sp: get('bank_coin_sp'),
    cp: get('bank_coin_cp'),
  };
}

export function saveBankCoin(coin: BankCoinForm): void {
  for (const k of ['pp', 'gp', 'sp', 'cp'] as const) {
    const v = coin[k];
    if (typeof v !== 'number' || !Number.isFinite(v) || v < 0) {
      throw new Error(`saveBankCoin: invalid ${k} value ${String(v)}`);
    }
  }
  const lock = LockService.getDocumentLock();
  if (!lock.tryLock(30000)) throw new Error('saveBankCoin: lock_busy');
  try {
    writeMetaRow('_meta', 'bank_coin_pp', String(coin.pp));
    writeMetaRow('_meta', 'bank_coin_gp', String(coin.gp));
    writeMetaRow('_meta', 'bank_coin_sp', String(coin.sp));
    writeMetaRow('_meta', 'bank_coin_cp', String(coin.cp));
    writeMetaRow('_meta', 'bank_coin_last_updated', new Date().toISOString());
    log('info', 'saveBankCoin', { pp: coin.pp, gp: coin.gp, sp: coin.sp, cp: coin.cp });
  } finally {
    lock.releaseLock();
  }
  // protectBankCoinCells acquires no lock and is idempotent — closes
  // the lazy-creation gap where bank_coin_* rows didn't exist when
  // migrateToV3 / installTriggers ran. First save creates them; this
  // call protects them immediately. Re-runs are no-ops via description
  // match.
  protectBankCoinCells();
  // buildBank acquires its own lock — invoke outside the lock above so
  // we don't risk recursive contention.
  buildBank();
}

function buildSidebarHtml(): string {
  return `
<div style="font-family:Arial,sans-serif;padding:12px;font-size:13px">
  <h3 style="margin-top:0">Bank Coin</h3>
  <p style="color:#666;font-size:11px">
    Manual entry — /outputfile inventory does not include coin totals.
    These values appear in the bank tab's COIN row.
  </p>
  <table style="width:100%;border-collapse:collapse;margin-top:8px">
    <tr><td style="padding:3px 6px 3px 0">Platinum</td><td><input id="pp" type="number" min="0" style="width:100%"></td></tr>
    <tr><td style="padding:3px 6px 3px 0">Gold</td><td><input id="gp" type="number" min="0" style="width:100%"></td></tr>
    <tr><td style="padding:3px 6px 3px 0">Silver</td><td><input id="sp" type="number" min="0" style="width:100%"></td></tr>
    <tr><td style="padding:3px 6px 3px 0">Copper</td><td><input id="cp" type="number" min="0" style="width:100%"></td></tr>
  </table>
  <button id="saveBtn" onclick="save()" disabled style="margin-top:12px;padding:5px 14px">Save</button>
  <div id="msg" style="margin-top:10px;color:#080;font-size:12px"></div>
  <script>
    google.script.run.withSuccessHandler(populate).withFailureHandler(showErr).getBankCoinForForm();

    function populate(v) {
      document.getElementById('pp').value = v.pp || 0;
      document.getElementById('gp').value = v.gp || 0;
      document.getElementById('sp').value = v.sp || 0;
      document.getElementById('cp').value = v.cp || 0;
      document.getElementById('saveBtn').disabled = false;
    }

    function showErr(err) {
      const msg = document.getElementById('msg');
      msg.style.color = '#c00';
      msg.textContent = 'Failed to load: ' + (err && err.message || err);
    }

    function save() {
      const data = {
        pp: parseInt(document.getElementById('pp').value || '0', 10),
        gp: parseInt(document.getElementById('gp').value || '0', 10),
        sp: parseInt(document.getElementById('sp').value || '0', 10),
        cp: parseInt(document.getElementById('cp').value || '0', 10),
      };
      const msg = document.getElementById('msg');
      msg.style.color = '#666';
      msg.textContent = 'Saving…';
      document.getElementById('saveBtn').disabled = true;
      google.script.run
        .withSuccessHandler(function () {
          msg.style.color = '#080';
          msg.textContent = 'Saved.';
          document.getElementById('saveBtn').disabled = false;
        })
        .withFailureHandler(function (err) {
          msg.style.color = '#c00';
          msg.textContent = err && err.message || String(err);
          document.getElementById('saveBtn').disabled = false;
        })
        .saveBankCoin(data);
    }
  </script>
</div>
`;
}
