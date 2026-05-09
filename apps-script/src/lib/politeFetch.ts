// politeFetch — courtesy HTTP wrapper for outbound calls (PigParse +
// P1999 wiki). Identifying User-Agent, retry on 429/503/504 with
// Retry-After honoring, exponential backoff per Phase 3 RESEARCH §5.
//
// Caller is responsible for inter-request sleep (1s between wiki
// requests per ROADMAP SC-4); this function does NOT sleep between
// successful calls. Only between retries of a single failing call.
//
// Returns a discriminated union: success with body+headers OR error
// with status + retriesUsed for diagnostics.

import { log } from './log';

declare const __VERSION__: string;
const VERSION = typeof __VERSION__ === 'string' ? __VERSION__ : '0.3.0';
const DEFAULT_USER_AGENT = `SquireBot/${VERSION} (+https://github.com/boejowen/SquireBot)`;
const RETRY_DELAYS_MS = [2000, 4000, 8000, 16000, 32000];
const RETRY_STATUSES = new Set([429, 503, 504]);

export interface FetchSuccess {
  ok: true;
  status: number;
  body: string;
  headers: Record<string, string>;
  fromCache: boolean;
  etag?: string;
  retriesUsed: number;
}

export interface FetchError {
  ok: false;
  status: number;
  error: string;
  retriesUsed: number;
}

export type FetchResult = FetchSuccess | FetchError;

export interface FetchOpts {
  etag?: string;
  userAgent?: string;
  // method defaults to 'get'. POST is supported but doesn't follow
  // redirects (UrlFetchApp default) — fine for our use case (POST is
  // never used in Phase 3).
  method?: 'get' | 'post';
}

export function politeFetch(url: string, opts: FetchOpts = {}): FetchResult {
  const headers: Record<string, string> = {
    'User-Agent': opts.userAgent ?? DEFAULT_USER_AGENT,
  };
  if (opts.etag) headers['If-None-Match'] = opts.etag;

  const fetchOpts: GoogleAppsScript.URL_Fetch.URLFetchRequestOptions = {
    method: opts.method ?? 'get',
    headers,
    muteHttpExceptions: true,
    followRedirects: true,
    validateHttpsCertificates: true,
  };

  let lastStatus = 0;
  let lastError = '';
  for (let attempt = 0; attempt <= RETRY_DELAYS_MS.length; attempt++) {
    let resp: GoogleAppsScript.URL_Fetch.HTTPResponse;
    try {
      resp = UrlFetchApp.fetch(url, fetchOpts);
    } catch (e) {
      lastError = `network: ${(e as Error).message}`;
      lastStatus = 0;
      log('warn', 'politeFetch', { url, attempt, lastError });
      if (attempt < RETRY_DELAYS_MS.length) {
        Utilities.sleep(RETRY_DELAYS_MS[attempt]);
        continue;
      }
      break;
    }

    const status = resp.getResponseCode();
    lastStatus = status;
    const allHeaders = resp.getAllHeaders() as Record<string, string>;

    if (status === 200) {
      const etagHeader = allHeaders['ETag'] ?? allHeaders['etag'];
      return {
        ok: true,
        status,
        body: resp.getContentText(),
        headers: allHeaders,
        fromCache: false,
        etag: etagHeader,
        retriesUsed: attempt,
      };
    }

    if (status === 304) {
      return {
        ok: true,
        status,
        body: '',
        headers: allHeaders,
        fromCache: true,
        etag: opts.etag,
        retriesUsed: attempt,
      };
    }

    if (RETRY_STATUSES.has(status)) {
      if (attempt >= RETRY_DELAYS_MS.length) {
        lastError = `${status} retries exhausted`;
        break;
      }
      const retryAfterRaw = allHeaders['Retry-After'] ?? allHeaders['retry-after'];
      const retryAfterMs = parseRetryAfterMs(retryAfterRaw);
      const waitMs = retryAfterMs ?? RETRY_DELAYS_MS[attempt];
      log('warn', 'politeFetch', { url, attempt, status, waitMs, retryAfter: retryAfterRaw });
      Utilities.sleep(waitMs);
      continue;
    }

    // Non-retriable 4xx/5xx (404, 401, etc.) — surface immediately.
    return {
      ok: false,
      status,
      error: `non-retriable ${status}`,
      retriesUsed: attempt,
    };
  }

  return {
    ok: false,
    status: lastStatus,
    error: lastError || `unknown failure (last status ${lastStatus})`,
    retriesUsed: RETRY_DELAYS_MS.length,
  };
}

function parseRetryAfterMs(raw: string | undefined): number | null {
  if (!raw) return null;
  const seconds = parseInt(raw, 10);
  if (Number.isFinite(seconds) && seconds >= 0 && seconds <= 600) {
    return seconds * 1000;
  }
  return null;
}
