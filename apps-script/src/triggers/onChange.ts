// onChange — Phase 3 plan 03-04 task 4.
//
// Apps Script's `e` for OTHER changeType doesn't reliably tell us which
// sheet was edited (per RESEARCH §10 gap #2 — confirmed against docs).
// Pragmatic strategy: on any change, debounce 10s, then run buildView
// + buildBank. Heartbeat-driven false positives waste ~12 rebuilds/day
// (one per guildie's daily heartbeat); each takes ~5–10s; ~2 min/day
// budget cost; acceptable.
//
// buildView's own debounce inside its function body is the actual
// safety net — onChange doesn't gate on time.

import { log } from '../lib/log';
import { buildView } from '../tabs/buildView';
import { buildBank } from '../tabs/buildBank';

export function onChange(_e?: GoogleAppsScript.Events.SheetsOnChange): void {
  log('debug', 'onChange', { fired: true });
  // buildView debounces internally (10s window); calling it on every
  // onChange is safe.
  buildView();
  buildBank();
}
