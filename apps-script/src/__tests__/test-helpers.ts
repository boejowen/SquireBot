// Vitest test helpers. Apps Script globals (SpreadsheetApp, LockService,
// PropertiesService, ScriptApp, UrlFetchApp, Utilities, CacheService,
// HtmlService) are mocked here. Each test that needs them calls
// installAppsScriptMocks() in beforeEach, then asserts via the captured
// state.

import { vi, beforeEach } from 'vitest';
import { createHash } from 'node:crypto';

export interface ProtectionRecord {
  rangeA1: string;          // synthetic e.g. "r5c2"
  description: string;
  warningOnly: boolean;
}

export interface FakeSheet {
  name: string;
  values: unknown[][]; // [row][col], row 0 = header
  notes: (string | null)[][];
  protections?: ProtectionRecord[];
}

export interface MockState {
  sheets: Map<string, FakeSheet>;
  lockHeld: boolean;
  lockTryLockReturn: boolean;
  properties: Map<string, string>;
  // Phase 8 plan 08-01 (D-04 / D-05): SEARCH-05 per-user MRU scope. A
  // SEPARATE Map so getUserProperties() writes do not bleed into the
  // document-scope tests that already passed in Phases 1-7.
  userProperties: Map<string, string>;
  triggers: Array<{ handler: string; type: string }>;
  fetchResponses: Map<string, { status: number; body: string; headers?: Record<string, string> }>;
  fetchCalls: Array<{ url: string; opts?: unknown }>;
  sleepCalls: number[];
  appendedRowsLog: Array<{ sheet: string; row: unknown[] }>;
  setValuesLog: Array<{ sheet: string; range: string; values: unknown[][] }>;
  // Phase 5 plan 05-03: real Map-backed CacheService mock with TTL.
  // Tests use vi.setSystemTime to advance Date.now() and trigger expiry.
  cache: Map<string, { value: string; expiresAt: number }>;
  // Phase 7 plan 07-02: capture SpreadsheetApp.getUi().alert(title, body,
  // buttonSet) calls so tests can assert non-admin failure modal copy
  // (D-03). Each entry records the three positional args passed to alert.
  // The mock returns 'OK' as a sentinel so callers reading the return
  // value (e.g., bootstrapGuildAdminsManual OK_CANCEL flow) get a
  // truthy-but-distinguishable value.
  alertCalls: Array<{ title: string; body: string; buttonSet: unknown }>;
  // Phase 7 WR-03 fix: alert() now also accepts an override return value
  // so tests can simulate a CANCEL response from an OK_CANCEL dialog.
  // Default 'OK' is preserved when this is unset/null.
  alertReturn?: 'OK' | 'CANCEL' | 'YES' | 'NO' | null;
  // Phase 7 WR-03 fix: SpreadsheetApp.getActiveSpreadsheet().toast(msg)
  // now stubbed (was missing — bootstrapGuildAdminsManual calls toast on
  // three branches and any test exercising the success path crashed
  // with "toast is not a function").
  toastCalls: string[];
}

export function makeSheet(name: string, headers: string[], dataRows: unknown[][] = []): FakeSheet {
  const values: unknown[][] = [headers.slice(), ...dataRows.map((r) => r.slice())];
  const notes: (string | null)[][] = values.map((r) => r.map(() => null));
  return { name, values, notes };
}

export function installAppsScriptMocks(state: MockState): void {
  function getSheetByName(this: unknown, name: string): unknown {
    const s = state.sheets.get(name);
    if (!s) return null;
    return makeSheetProxy(s, state);
  }

  function makeSheetProxy(s: FakeSheet, state: MockState) {
    return {
      getName: () => s.name,
      getLastRow: () => s.values.length,
      getLastColumn: () => (s.values[0]?.length ?? 0),
      getMaxRows: () => Math.max(s.values.length, 1000),
      // Phase 5 plan 05-01: hide/show + sheet-id mocks used by
      // hideAllSystemTabs() (migrations.ts) and weeklySchemaHealthcheck()
      // (triggers/weeklySchemaHealthcheck.ts). getSheetId returns a stable
      // deterministic hash of the sheet name so tests can predict IDs.
      isSheetHidden: () => Boolean((s as FakeSheet & { _hidden?: boolean })._hidden),
      hideSheet: () => { (s as FakeSheet & { _hidden?: boolean })._hidden = true; },
      showSheet: () => { (s as FakeSheet & { _hidden?: boolean })._hidden = false; },
      getSheetId: () => s.name.split('').reduce((h, c) => h * 31 + c.charCodeAt(0), 0) | 0,
      getRange(rowOrA1: number | string, col?: number, numRows?: number, numCols?: number) {
        if (typeof rowOrA1 === 'string') {
          throw new Error('A1 notation not supported in mock');
        }
        const r = rowOrA1; const c = col!;
        const nr = numRows ?? 1; const nc = numCols ?? 1;
        const range: Record<string, unknown> = {};
        range.getValues = () => {
          const out: unknown[][] = [];
          for (let i = 0; i < nr; i++) {
            const row: unknown[] = [];
            for (let j = 0; j < nc; j++) {
              row.push(s.values[r - 1 + i]?.[c - 1 + j] ?? '');
            }
            out.push(row);
          }
          return out;
        };
        range.setValue = (v: unknown) => {
          ensureRow(s, r); ensureCol(s, r, c);
          s.values[r - 1][c - 1] = v;
          state.setValuesLog.push({ sheet: s.name, range: `r${r}c${c}`, values: [[v]] });
          return range;
        };
        range.setValues = (vals: unknown[][]) => {
          for (let i = 0; i < vals.length; i++) {
            for (let j = 0; j < vals[i].length; j++) {
              ensureRow(s, r + i); ensureCol(s, r + i, c + j);
              s.values[r - 1 + i][c - 1 + j] = vals[i][j];
            }
          }
          state.setValuesLog.push({ sheet: s.name, range: `r${r}c${c}-${nr}x${nc}`, values: vals });
          return range;
        };
        range.setNotes = (notes: (string | null)[][]) => {
          for (let i = 0; i < notes.length; i++) {
            for (let j = 0; j < notes[i].length; j++) {
              ensureRow(s, r + i); ensureCol(s, r + i, c + j);
              // Ensure notes parallel array exists for any rows the
              // value array may have grown without notes-side alignment
              // (e.g., direct test mutations of s.values).
              while (s.notes.length < s.values.length) {
                s.notes.push(new Array(s.values[s.notes.length]?.length ?? 0).fill(null));
              }
              while (s.notes[r - 1 + i].length < c + j) {
                s.notes[r - 1 + i].push(null);
              }
              s.notes[r - 1 + i][c - 1 + j] = notes[i][j];
            }
          }
          return range;
        };
        range.setNote = (note: string) => {
          s.notes[r - 1][c - 1] = note;
          return range;
        };
        range.clearContent = () => {
          for (let i = 0; i < nr; i++) {
            for (let j = 0; j < nc; j++) {
              if (s.values[r - 1 + i]) s.values[r - 1 + i][c - 1 + j] = '';
            }
          }
          return range;
        };
        range.setBackground = () => range;
        range.setFontColor = () => range;
        range.setFontFamily = () => range;
        range.setFontWeight = () => range;
        range.setBorder = () => range;
        range.getA1Notation = () => `r${r}c${c}`;
        range.protect = () => {
          const protection: ProtectionRecord = {
            rangeA1: `r${r}c${c}`,
            description: '',
            warningOnly: false,
          };
          if (!s.protections) s.protections = [];
          s.protections.push(protection);
          const builder = {
            setDescription: (d: string) => { protection.description = d; return builder; },
            setWarningOnly: (w: boolean) => { protection.warningOnly = w; return builder; },
            getRange: () => ({ getA1Notation: () => protection.rangeA1 }),
            getDescription: () => protection.description,
            remove: () => {
              const idx = s.protections!.indexOf(protection);
              if (idx >= 0) s.protections!.splice(idx, 1);
            },
          };
          return builder;
        };
        return range;
      },
      getProtections: (_type: unknown) => (s.protections ?? []).map((p) => ({
        getRange: () => ({ getA1Notation: () => p.rangeA1 }),
        getDescription: () => p.description,
        remove: () => {
          const i = s.protections!.indexOf(p);
          if (i >= 0) s.protections!.splice(i, 1);
        },
      })),
      appendRow: (row: unknown[]) => {
        s.values.push(row.slice());
        s.notes.push(row.map(() => null));
        state.appendedRowsLog.push({ sheet: s.name, row: row.slice() });
      },
      deleteRow: (rowIndex: number) => {
        // 1-based; row 1 is the header — refuse to delete it.
        if (rowIndex < 2 || rowIndex > s.values.length) return;
        s.values.splice(rowIndex - 1, 1);
        s.notes.splice(rowIndex - 1, 1);
      },
      setConditionalFormatRules: (rules: unknown[]) => {
        // Stash for assertion.
        (s as FakeSheet & { _condFormatRules?: unknown[] })._condFormatRules = rules;
      },
      insertSheet: () => {},
    };
  }

  function ensureRow(s: FakeSheet, r: number): void {
    while (s.values.length < r) {
      const cols = s.values[0]?.length ?? 0;
      s.values.push(new Array(cols).fill(''));
      s.notes.push(new Array(cols).fill(null));
    }
  }
  function ensureCol(s: FakeSheet, r: number, c: number): void {
    while (s.values[r - 1].length < c) {
      s.values[r - 1].push('');
      s.notes[r - 1].push(null);
    }
  }

  // Recursive menu chain mock — each method returns the same builder
  // so any chain depth works.
  const menuBuilder: Record<string, unknown> = {};
  menuBuilder.addItem = () => menuBuilder;
  menuBuilder.addSeparator = () => menuBuilder;
  menuBuilder.addSubMenu = () => menuBuilder;
  menuBuilder.addToUi = () => {};

  // newConditionalFormatRule chain — captures whenFormulaSatisfied,
  // setBackground, setRanges; build() returns a tagged object.
  function makeRuleBuilder() {
    const built: Record<string, unknown> = { _kind: 'conditional-rule' };
    const builder = {
      whenFormulaSatisfied: (formula: string) => { built.formula = formula; return builder; },
      setBackground: (bg: string) => { built.background = bg; return builder; },
      setRanges: (ranges: unknown[]) => { built.ranges = ranges; return builder; },
      build: () => built,
    };
    return builder;
  }

  (globalThis as Record<string, unknown>).SpreadsheetApp = {
    getActiveSpreadsheet: () => ({
      getSheetByName,
      insertSheet: (name: string) => {
        const s: FakeSheet = { name, values: [[]], notes: [[]] };
        state.sheets.set(name, s);
        return makeSheetProxy(s, state);
      },
      getSheets: () => Array.from(state.sheets.values()).map((s) => makeSheetProxy(s, state)),
      // Phase 7 WR-03 fix: toast(msg) capture so tests covering
      // bootstrapGuildAdminsManual's success / already-initialized /
      // failure branches can assert the user-facing copy without
      // crashing with "toast is not a function".
      toast: (msg?: unknown) => {
        state.toastCalls.push(String(msg ?? ''));
      },
    }),
    getUi: () => ({
      // Phase 7 plan 07-02: capture alert(title, body, buttonSet) calls
      // into state.alertCalls. Returns 'OK' by default — most v1.x call
      // sites pass ButtonSet.OK and ignore the return. The
      // bootstrapGuildAdminsManual OK_CANCEL flow compares against
      // ui.Button.OK / CANCEL; tests that need to simulate a CANCEL
      // response set state.alertReturn = 'CANCEL' before the call.
      alert: (title?: unknown, body?: unknown, buttonSet?: unknown) => {
        state.alertCalls.push({
          title: String(title ?? ''),
          body: String(body ?? ''),
          buttonSet,
        });
        return state.alertReturn ?? 'OK';
      },
      createMenu: () => menuBuilder,
      showModalDialog: () => {},
      // Phase 5 plan 05-03: capture the served HtmlOutput so tests can
      // assert sidebar title/width/body without intercepting
      // SpreadsheetApp.getUi() itself.
      showSidebar: (output: unknown) => {
        (state as MockState & { lastSidebar?: unknown }).lastSidebar = output;
      },
      // Phase 7 plan 07-02: ButtonSet + Button enums used by
      // bootstrapGuildAdminsManual (lib/admin) and any future callers.
      ButtonSet: { OK: 'OK', OK_CANCEL: 'OK_CANCEL', YES_NO: 'YES_NO' },
      Button: { OK: 'OK', CANCEL: 'CANCEL', YES: 'YES', NO: 'NO' },
    }),
    newConditionalFormatRule: () => makeRuleBuilder(),
    ProtectionType: { RANGE: 'RANGE', SHEET: 'SHEET' },
  };

  (globalThis as Record<string, unknown>).LockService = {
    getDocumentLock: () => ({
      tryLock: vi.fn((_ms: number) => state.lockTryLockReturn),
      releaseLock: vi.fn(() => { state.lockHeld = false; }),
    }),
  };

  (globalThis as Record<string, unknown>).PropertiesService = {
    getDocumentProperties: () => ({
      getProperty: (k: string) => state.properties.get(k) ?? null,
      setProperty: (k: string, v: string) => { state.properties.set(k, v); },
      deleteProperty: (k: string) => { state.properties.delete(k); },
    }),
    // Phase 8 plan 08-01 (D-04 / D-05): per-user scope, backed by a SEPARATE
    // Map so SEARCH-05's getUserProperties() writes don't bleed into the
    // document-scope tests that already passed in Phases 1-7. getScriptProperties
    // aliases the same per-user Map for the rare current consumer; no test
    // currently distinguishes the two and the production code never reads
    // script-scope, so the alias is safe.
    getUserProperties: () => ({
      getProperty: (k: string) => state.userProperties.get(k) ?? null,
      setProperty: (k: string, v: string) => { state.userProperties.set(k, v); },
      deleteProperty: (k: string) => { state.userProperties.delete(k); },
    }),
    getScriptProperties: () => ({
      getProperty: (k: string) => state.userProperties.get(k) ?? null,
      setProperty: (k: string, v: string) => { state.userProperties.set(k, v); },
      deleteProperty: (k: string) => { state.userProperties.delete(k); },
    }),
  };

  // newTrigger returns a fluent builder. Each terminal create() pushes
  // an entry into state.triggers so callers can assert what was created.
  function makeTriggerBuilder(handler: string) {
    let builderType = 'CLOCK';
    const builder = {
      timeBased: () => {
        builderType = 'CLOCK';
        return {
          after: (_ms: number) => ({ create: () => { state.triggers.push({ handler, type: 'CLOCK' }); } }),
          everyHours: (_n: number) => ({ create: () => { state.triggers.push({ handler, type: 'CLOCK' }); } }),
          everyDays: (_n: number) => ({
            atHour: (_h: number) => ({
              inTimezone: (_tz: string) => ({ create: () => { state.triggers.push({ handler, type: 'CLOCK' }); } }),
              create: () => { state.triggers.push({ handler, type: 'CLOCK' }); },
            }),
            create: () => { state.triggers.push({ handler, type: 'CLOCK' }); },
          }),
          atHour: (_h: number) => ({
            everyDays: (_n: number) => ({
              inTimezone: (_tz: string) => ({ create: () => { state.triggers.push({ handler, type: 'CLOCK' }); } }),
              create: () => { state.triggers.push({ handler, type: 'CLOCK' }); },
            }),
            inTimezone: (_tz: string) => ({ create: () => { state.triggers.push({ handler, type: 'CLOCK' }); } }),
            create: () => { state.triggers.push({ handler, type: 'CLOCK' }); },
          }),
          onWeekDay: (_d: string) => ({
            atHour: (_h: number) => ({
              inTimezone: (_tz: string) => ({ create: () => { state.triggers.push({ handler, type: 'CLOCK' }); } }),
              create: () => { state.triggers.push({ handler, type: 'CLOCK' }); },
            }),
            create: () => { state.triggers.push({ handler, type: 'CLOCK' }); },
          }),
        };
      },
      forSpreadsheet: (_ss: unknown) => ({
        onChange: () => ({ create: () => { state.triggers.push({ handler, type: 'ON_CHANGE' }); } }),
      }),
      _type: () => builderType,
    };
    return builder;
  }

  (globalThis as Record<string, unknown>).ScriptApp = {
    getProjectTriggers: () => state.triggers.map((t) => ({
      getHandlerFunction: () => t.handler,
      getEventType: () => t.type,
    })),
    deleteTrigger: vi.fn((trig: { getHandlerFunction: () => string; getEventType: () => string }) => {
      // Remove the first matching entry from state.triggers
      const handler = trig.getHandlerFunction();
      const type = trig.getEventType();
      const idx = state.triggers.findIndex((t) => t.handler === handler && t.type === type);
      if (idx >= 0) state.triggers.splice(idx, 1);
    }),
    newTrigger: vi.fn((handler: string) => makeTriggerBuilder(handler)),
    EventType: { CLOCK: 'CLOCK', ON_CHANGE: 'ON_CHANGE' },
    WeekDay: { SUNDAY: 'SUNDAY' },
  };

  (globalThis as Record<string, unknown>).UrlFetchApp = {
    fetch: vi.fn((url: string, opts?: unknown) => {
      state.fetchCalls.push({ url, opts });
      const resp = state.fetchResponses.get(url);
      if (!resp) throw new Error(`UrlFetchApp.fetch mock: no response configured for ${url}`);
      return {
        getResponseCode: () => resp.status,
        getContentText: () => resp.body,
        getHeaders: () => resp.headers ?? {},
        getAllHeaders: () => resp.headers ?? {},
      };
    }),
  };

  (globalThis as Record<string, unknown>).Utilities = {
    sleep: vi.fn((ms: number) => { state.sleepCalls.push(ms); }),
    computeDigest: vi.fn((_alg: string, bytes: number[]) => {
      // Real SHA-1 via node:crypto so deterministic-and-distinct hash
      // tests are meaningful. Apps Script returns signed bytes (-128..127);
      // mirror that by mapping each unsigned byte > 127 to its signed
      // equivalent so wiki-parser.ts's b<0?b+256 conversion still works.
      const buf = Buffer.from(bytes.map((b) => (b < 0 ? b + 256 : b)));
      const hash = createHash('sha1').update(buf).digest();
      return Array.from(hash).map((b) => (b > 127 ? b - 256 : b));
    }),
    DigestAlgorithm: { SHA_1: 'SHA_1' },
    newBlob: (s: string) => ({ getBytes: () => Array.from(s).map((c) => c.charCodeAt(0)) }),
  };

  // Phase 5 plan 05-03: Map-backed TTL-respecting CacheService mock.
  // Replaces the prior no-op stub. searchIndex.ts uses get/put/putAll/getAll/
  // remove/removeAll — all five methods honored. TTL is evaluated against
  // Date.now() so tests can use vi.useFakeTimers + vi.setSystemTime to
  // exercise expiry. Missing keys are omitted from getAll output (not
  // returned as null) — mirrors real CacheService.getAll() behavior.
  (globalThis as Record<string, unknown>).CacheService = {
    getDocumentCache: () => ({
      get(key: string) {
        const e = state.cache.get(key);
        if (!e) return null;
        if (Date.now() > e.expiresAt) { state.cache.delete(key); return null; }
        return e.value;
      },
      put(key: string, value: string, ttlSec: number) {
        state.cache.set(key, { value, expiresAt: Date.now() + ttlSec * 1000 });
      },
      putAll(values: Record<string, string>, ttlSec: number) {
        const exp = Date.now() + ttlSec * 1000;
        for (const [k, v] of Object.entries(values)) {
          state.cache.set(k, { value: v, expiresAt: exp });
        }
      },
      getAll(keys: string[]) {
        const out: Record<string, string> = {};
        for (const k of keys) {
          const e = state.cache.get(k);
          if (!e) continue;
          if (Date.now() > e.expiresAt) { state.cache.delete(k); continue; }
          out[k] = e.value;
        }
        return out;
      },
      remove(key: string) { state.cache.delete(key); },
      removeAll(keys: string[]) { for (const k of keys) state.cache.delete(k); },
    }),
  };

  // Phase 5 plan 05-03: HtmlService mock returns a fluent builder so
  // setTitle/setWidth/setHeight chain in any order. The `_html` and
  // `_title`/`_width` getters let tests assert the served HTML and
  // sidebar metadata without intercepting showSidebar() itself.
  (globalThis as Record<string, unknown>).HtmlService = {
    createHtmlOutput: (html: string) => {
      const out: Record<string, unknown> = { _html: html };
      out.setTitle = (t: string) => { out._title = t; return out; };
      out.setWidth = (w: number) => { out._width = w; return out; };
      out.setHeight = (h: number) => { out._height = h; return out; };
      return out;
    },
  };

  // eslint-disable-next-line no-console
  if (!('console' in globalThis)) (globalThis as Record<string, unknown>).console = console;
}

export function newMockState(): MockState {
  return {
    sheets: new Map(),
    lockHeld: false,
    lockTryLockReturn: true,
    properties: new Map(),
    userProperties: new Map(),
    triggers: [],
    fetchResponses: new Map(),
    fetchCalls: [],
    sleepCalls: [],
    appendedRowsLog: [],
    setValuesLog: [],
    cache: new Map(),
    alertCalls: [],
    alertReturn: null,
    toastCalls: [],
  };
}

// resetMocks is the recommended beforeEach. Returns the fresh state.
export function resetMocks(): MockState {
  const state = newMockState();
  installAppsScriptMocks(state);
  return state;
}

// Convenience: pre-seed _meta with the given KV rows.
export function seedMeta(state: MockState, rows: Array<[string, string]>): void {
  const sheet = makeSheet('_meta', ['key', 'value'], rows);
  state.sheets.set('_meta', sheet);
}

beforeEach(() => {
  vi.restoreAllMocks();
});

// ----------------------------------------------------------------------------
// Phase 8 plan 08-01 (D-04): mountSidebar(html) JSDOM helper for TEST-02
// sidebar inline-JS tests. Parses a SIDEBAR_BODY string, installs a
// controllable google.script.run Proxy mock BEFORE re-executing inline
// <script> blocks (init() runs immediately at the bottom of each sidebar's
// inline JS and reads window.google synchronously), then returns the realm
// plus dispatch helpers so tests can resolve enqueued call promises FIFO.
//
// JSDOM gotcha #1: per HTML5 spec, <script> tags inserted via innerHTML do
// NOT execute. We work around this by extracting <script> textContent and
// executing it ourselves.
//
// JSDOM gotcha #2 (Plan 08-02 RED-phase finding): vitest's `jsdom` test
// environment evaluates JSDOM-attached inline <script> elements via
// `vm.runInContext` AGAINST A DIFFERENT VM CONTEXT from the test realm.
// Writes from the inline script (e.g. `window.foo = 1`) do NOT appear on
// the test-side `window`, and the script's `google` binding cannot see
// the test-side stub installed via `window.google = ...`. Two windows,
// one document. Symptomatic error: `ReferenceError: google is not defined`
// even after the helper sets `window.google` first.
//
// Mitigation: indirect-eval (`(0, eval)(src)`) executes the script in the
// TEST realm's global scope, so `document`, `window`, and the pre-set
// `google` all resolve to the test-side bindings. Top-level `function` /
// `var` declarations from the sidebar (e.g. `submit()`, `init()`,
// `currentEmail`) become properties of the test-realm globalThis, which
// vitest's jsdom env aliases to `window` — exactly what the inline JS
// expects in a real Apps Script HtmlService iframe.
// ----------------------------------------------------------------------------

export interface MountedSidebar {
  document: Document;
  window: Window & typeof globalThis;
  runCalls: Array<{ method: string; args: unknown[] }>;
  dispatchRunCall: (method: string, payload: unknown) => void;
  failRunCall: (method: string, error: { message: string }) => void;
  getPendingCalls: () => Array<{ method: string; args: unknown[] }>;
}

export function mountSidebar(html: string): MountedSidebar {
  // 1. Reset the body.
  document.body.innerHTML = '';

  // 2. Parse the HTML into a detached <template> so the browser parser
  //    splits <script> nodes from the rest without executing them.
  const tpl = document.createElement('template');
  tpl.innerHTML = html;

  // 3. Walk the parsed fragment, separating script nodes from the rest.
  //    Scripts may be nested anywhere (showBankCoinSidebar.ts and
  //    showCharInfoSidebar.ts inline their <script> INSIDE the outer <div>
  //    wrapper rather than at top level). A top-level walk would miss
  //    those nested scripts and let JSDOM auto-evaluate them in its
  //    separate vm context when the fragment is appended to body —
  //    triggering the exact 'google is not defined' failure we're fixing.
  //    querySelectorAll('script') on the template's content gets EVERY
  //    <script> regardless of nesting depth; we capture their source then
  //    remove them from the fragment before appending.
  const scripts: HTMLScriptElement[] = Array.from(
    tpl.content.querySelectorAll('script'),
  );
  scripts.forEach((s) => s.parentNode?.removeChild(s));
  const frag = document.createDocumentFragment();
  Array.from(tpl.content.childNodes).forEach((node) => {
    frag.appendChild(node.cloneNode(true));
  });
  document.body.appendChild(frag);

  // 4. Build the google.script.run fluent mock. The chain shape is
  //    `.withSuccessHandler(s).withFailureHandler(f).METHOD(args)` but any
  //    handler is optional (Search's pushRecentSearchCall is fire-and-forget).
  //    Each terminal METHOD invocation enqueues a record so dispatch can
  //    resolve handlers FIFO.
  const runCalls: Array<{ method: string; args: unknown[] }> = [];
  // eslint-disable-next-line @typescript-eslint/ban-types
  const pendingByMethod = new Map<string, Array<{ success?: Function; failure?: Function }>>();

  // eslint-disable-next-line @typescript-eslint/ban-types
  function makeChain(handlers: { success?: Function; failure?: Function }): unknown {
    return new Proxy({}, {
      get(_t, prop: string) {
        if (prop === 'withSuccessHandler') {
          // eslint-disable-next-line @typescript-eslint/ban-types
          return (fn: Function) => makeChain({ ...handlers, success: fn });
        }
        if (prop === 'withFailureHandler') {
          // eslint-disable-next-line @typescript-eslint/ban-types
          return (fn: Function) => makeChain({ ...handlers, failure: fn });
        }
        // Terminal method invocation.
        return (...args: unknown[]) => {
          runCalls.push({ method: prop, args });
          const queue = pendingByMethod.get(prop) ?? [];
          queue.push(handlers);
          pendingByMethod.set(prop, queue);
        };
      },
    });
  }

  (window as unknown as Record<string, unknown>).google = {
    script: { run: makeChain({}), host: { close: () => { /* no-op for sidebars */ } } },
  };

  // 5. Execute each inline script's source in the TEST realm's global scope
  //    via indirect eval. See JSDOM gotcha #2 in the helper-block header:
  //    appending <script> elements to document.head runs them in a separate
  //    JSDOM vm context whose `window` is NOT the test-side `window` and
  //    whose `google` binding cannot see the stub installed two lines above.
  //    Indirect eval `(0, eval)(src)` evaluates source as a Program in the
  //    surrounding realm — top-level `var` / `function` declarations attach
  //    to the test-realm globalThis (which is `window` under vitest jsdom),
  //    so subsequent event handlers fire the same `submit()` / `init()` /
  //    `onEmailChange()` the sidebar bundle declares.
  //
  //    Security note: this helper is test-only and never bundled into
  //    dist/Code.js (esbuild's entry is src/Code.ts, which does NOT import
  //    test-helpers). The eval risk is bounded to trusted fixture HTML
  //    authored in the apps-script/src/triggers/show*Sidebar.ts files.
  scripts.forEach((orig) => {
    const src = orig.textContent || '';
    if (!src.trim()) return;
    // eslint-disable-next-line no-eval
    (0, eval)(src);
  });

  function dispatchRunCall(method: string, payload: unknown): void {
    const queue = pendingByMethod.get(method);
    if (!queue || queue.length === 0) {
      throw new Error(`mountSidebar.dispatchRunCall: no pending ${method} call`);
    }
    const next = queue.shift()!;
    if (next.success) next.success(payload);
  }

  function failRunCall(method: string, error: { message: string }): void {
    const queue = pendingByMethod.get(method);
    if (!queue || queue.length === 0) {
      throw new Error(`mountSidebar.failRunCall: no pending ${method} call`);
    }
    const next = queue.shift()!;
    if (next.failure) next.failure(error);
  }

  function getPendingCalls(): Array<{ method: string; args: unknown[] }> {
    const out: Array<{ method: string; args: unknown[] }> = [];
    pendingByMethod.forEach((queue, method) => {
      queue.forEach(() => out.push({ method, args: [] }));
    });
    return out;
  }

  return {
    document,
    window: window as Window & typeof globalThis,
    runCalls,
    dispatchRunCall,
    failRunCall,
    getPendingCalls,
  };
}
