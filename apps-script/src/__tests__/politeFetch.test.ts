import { describe, it, expect, beforeEach } from 'vitest';
import { politeFetch } from '../lib/politeFetch';
import { resetMocks, type MockState } from './test-helpers';

const URL = 'https://example.com/probe';

describe('politeFetch', () => {
  let state: MockState;
  beforeEach(() => { state = resetMocks(); });

  it('returns FetchSuccess on 200', () => {
    state.fetchResponses.set(URL, { status: 200, body: '{"ok":true}', headers: { ETag: 'abc' } });
    const r = politeFetch(URL);
    expect(r.ok).toBe(true);
    if (r.ok) {
      expect(r.status).toBe(200);
      expect(r.body).toBe('{"ok":true}');
      expect(r.etag).toBe('abc');
      expect(r.fromCache).toBe(false);
      expect(r.retriesUsed).toBe(0);
    }
  });

  it('returns fromCache=true on 304', () => {
    state.fetchResponses.set(URL, { status: 304, body: '', headers: {} });
    const r = politeFetch(URL, { etag: 'abc' });
    expect(r.ok).toBe(true);
    if (r.ok) {
      expect(r.status).toBe(304);
      expect(r.fromCache).toBe(true);
      expect(r.etag).toBe('abc');
    }
  });

  it('sends User-Agent header by default', () => {
    state.fetchResponses.set(URL, { status: 200, body: 'x', headers: {} });
    politeFetch(URL);
    const opts = state.fetchCalls[0].opts as { headers: Record<string, string> };
    expect(opts.headers['User-Agent']).toMatch(/^SquireBot\//);
    expect(opts.headers['User-Agent']).toContain('github.com/boejowen/SquireBot');
  });

  it('sends If-None-Match when etag provided', () => {
    state.fetchResponses.set(URL, { status: 200, body: 'x', headers: {} });
    politeFetch(URL, { etag: 'xyz' });
    const opts = state.fetchCalls[0].opts as { headers: Record<string, string> };
    expect(opts.headers['If-None-Match']).toBe('xyz');
  });

  it('returns non-retriable error on 404 immediately', () => {
    state.fetchResponses.set(URL, { status: 404, body: 'Not Found', headers: {} });
    const r = politeFetch(URL);
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.status).toBe(404);
      expect(r.retriesUsed).toBe(0);
    }
    expect(state.sleepCalls.length).toBe(0); // no sleep — no retries
  });

  it('retries 5 times on 503 then surfaces error', () => {
    state.fetchResponses.set(URL, { status: 503, body: 'down', headers: {} });
    const r = politeFetch(URL);
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.status).toBe(503);
      expect(r.retriesUsed).toBe(5);
    }
    // Verify we slept on 5 retries (between attempts 0-1, 1-2, 2-3, 3-4, 4-5)
    expect(state.sleepCalls).toEqual([2000, 4000, 8000, 16000, 32000]);
  });

  it('honors Retry-After header (overrides backoff schedule)', () => {
    let calls = 0;
    state.fetchResponses.set(URL, { status: 429, body: '', headers: { 'Retry-After': '7' } });
    // Replace UrlFetchApp.fetch to flip to 200 after first 429.
    (globalThis as Record<string, unknown>).UrlFetchApp = {
      fetch: (_url: string, _opts: unknown) => {
        calls++;
        state.fetchCalls.push({ url: _url, opts: _opts });
        if (calls === 1) {
          return {
            getResponseCode: () => 429,
            getContentText: () => '',
            getAllHeaders: () => ({ 'Retry-After': '7' }),
            getHeaders: () => ({ 'Retry-After': '7' }),
          };
        }
        return {
          getResponseCode: () => 200,
          getContentText: () => 'ok',
          getAllHeaders: () => ({}),
          getHeaders: () => ({}),
        };
      },
    };
    const r = politeFetch(URL);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.retriesUsed).toBe(1);
    expect(state.sleepCalls).toEqual([7000]); // honored, not the schedule's 2000
  });

  it('returns network error on UrlFetchApp throw', () => {
    (globalThis as Record<string, unknown>).UrlFetchApp = {
      fetch: () => { throw new Error('connection refused'); },
    };
    const r = politeFetch(URL);
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.error).toContain('connection refused');
      expect(r.retriesUsed).toBe(5);
    }
    expect(state.sleepCalls.length).toBe(5); // retried all 5 times
  });

  it('429 then 200 returns success with retriesUsed=1', () => {
    let calls = 0;
    (globalThis as Record<string, unknown>).UrlFetchApp = {
      fetch: (_url: string) => {
        calls++;
        if (calls === 1) {
          return { getResponseCode: () => 429, getContentText: () => '',
                   getAllHeaders: () => ({}), getHeaders: () => ({}) };
        }
        return { getResponseCode: () => 200, getContentText: () => 'success',
                 getAllHeaders: () => ({}), getHeaders: () => ({}) };
      },
    };
    const r = politeFetch(URL);
    expect(r.ok).toBe(true);
    if (r.ok) {
      expect(r.body).toBe('success');
      expect(r.retriesUsed).toBe(1);
    }
  });

  it('caller-supplied userAgent overrides default', () => {
    state.fetchResponses.set(URL, { status: 200, body: 'x', headers: {} });
    politeFetch(URL, { userAgent: 'MyOverride/1.0' });
    const opts = state.fetchCalls[0].opts as { headers: Record<string, string> };
    expect(opts.headers['User-Agent']).toBe('MyOverride/1.0');
  });
});
