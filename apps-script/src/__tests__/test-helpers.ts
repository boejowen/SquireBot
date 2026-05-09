// Vitest test helpers. Apps Script globals (SpreadsheetApp, LockService,
// PropertiesService, ScriptApp, UrlFetchApp, Utilities, CacheService,
// HtmlService) are mocked here. Each test that needs them calls
// installAppsScriptMocks() in beforeEach, then asserts via the captured
// state.

import { vi, beforeEach } from 'vitest';

export interface FakeSheet {
  name: string;
  values: unknown[][]; // [row][col], row 0 = header
  notes: (string | null)[][];
}

export interface MockState {
  sheets: Map<string, FakeSheet>;
  lockHeld: boolean;
  lockTryLockReturn: boolean;
  properties: Map<string, string>;
  triggers: Array<{ handler: string; type: string }>;
  fetchResponses: Map<string, { status: number; body: string; headers?: Record<string, string> }>;
  fetchCalls: Array<{ url: string; opts?: unknown }>;
  sleepCalls: number[];
  appendedRowsLog: Array<{ sheet: string; row: unknown[] }>;
  setValuesLog: Array<{ sheet: string; range: string; values: unknown[][] }>;
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
      getRange(rowOrA1: number | string, col?: number, numRows?: number, numCols?: number) {
        if (typeof rowOrA1 === 'string') {
          throw new Error('A1 notation not supported in mock');
        }
        const r = rowOrA1; const c = col!;
        const nr = numRows ?? 1; const nc = numCols ?? 1;
        return {
          getValues: () => {
            const out: unknown[][] = [];
            for (let i = 0; i < nr; i++) {
              const row: unknown[] = [];
              for (let j = 0; j < nc; j++) {
                row.push(s.values[r - 1 + i]?.[c - 1 + j] ?? '');
              }
              out.push(row);
            }
            return out;
          },
          setValue: (v: unknown) => {
            ensureRow(s, r); ensureCol(s, r, c);
            s.values[r - 1][c - 1] = v;
            state.setValuesLog.push({ sheet: s.name, range: `r${r}c${c}`, values: [[v]] });
            return this;
          },
          setValues: (vals: unknown[][]) => {
            for (let i = 0; i < vals.length; i++) {
              for (let j = 0; j < vals[i].length; j++) {
                ensureRow(s, r + i); ensureCol(s, r + i, c + j);
                s.values[r - 1 + i][c - 1 + j] = vals[i][j];
              }
            }
            state.setValuesLog.push({ sheet: s.name, range: `r${r}c${c}-${nr}x${nc}`, values: vals });
            return this;
          },
          setNotes: (notes: (string | null)[][]) => {
            for (let i = 0; i < notes.length; i++) {
              for (let j = 0; j < notes[i].length; j++) {
                ensureRow(s, r + i); ensureCol(s, r + i, c + j);
                s.notes[r - 1 + i][c - 1 + j] = notes[i][j];
              }
            }
            return this;
          },
          setNote: (note: string) => {
            s.notes[r - 1][c - 1] = note;
            return this;
          },
          clearContent: () => {
            for (let i = 0; i < nr; i++) {
              for (let j = 0; j < nc; j++) {
                if (s.values[r - 1 + i]) s.values[r - 1 + i][c - 1 + j] = '';
              }
            }
            return this;
          },
          setBackground: () => this, setFontColor: () => this,
          setFontFamily: () => this, setFontWeight: () => this,
          setBorder: () => this,
        };
      },
      appendRow: (row: unknown[]) => {
        s.values.push(row.slice());
        s.notes.push(row.map(() => null));
        state.appendedRowsLog.push({ sheet: s.name, row: row.slice() });
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

  (globalThis as Record<string, unknown>).SpreadsheetApp = {
    getActiveSpreadsheet: () => ({
      getSheetByName,
      insertSheet: (name: string) => {
        const s: FakeSheet = { name, values: [[]], notes: [[]] };
        state.sheets.set(name, s);
        return makeSheetProxy(s, state);
      },
      getSheets: () => Array.from(state.sheets.values()).map((s) => makeSheetProxy(s, state)),
    }),
    getUi: () => ({ alert: () => {}, createMenu: () => ({ addItem: () => ({ addItem: () => ({}), addSeparator: () => ({ addItem: () => ({}) }), addToUi: () => {} }) }) }),
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
  };

  (globalThis as Record<string, unknown>).ScriptApp = {
    getProjectTriggers: () => state.triggers.map((t) => ({
      getHandlerFunction: () => t.handler,
      getEventType: () => t.type,
    })),
    deleteTrigger: vi.fn(),
    newTrigger: vi.fn(),
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
    computeDigest: vi.fn(() => new Array(20).fill(0)),
    DigestAlgorithm: { SHA_1: 'SHA_1' },
    newBlob: (s: string) => ({ getBytes: () => Array.from(s).map((c) => c.charCodeAt(0)) }),
  };

  (globalThis as Record<string, unknown>).CacheService = {
    getDocumentCache: () => ({
      get: () => null,
      put: () => {},
    }),
  };

  (globalThis as Record<string, unknown>).HtmlService = {
    createHtmlOutput: (html: string) => ({
      setWidth: () => ({ setHeight: () => ({}) }),
      _html: html,
    }),
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
    triggers: [],
    fetchResponses: new Map(),
    fetchCalls: [],
    sleepCalls: [],
    appendedRowsLog: [],
    setValuesLog: [],
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
