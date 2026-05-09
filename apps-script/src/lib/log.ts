// Structured logging for Apps Script. Apps Script's STACKDRIVER inspects
// strings; emitting JSON keeps logs greppable later via the Cloud Logs
// Explorer's `jsonPayload` filters. Fields beyond {ts, level, op} are
// caller-supplied context.

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export function log(level: LogLevel, op: string, fields: Record<string, unknown> = {}): void {
  const entry = { ts: new Date().toISOString(), level, op, ...fields };
  // console.log goes to STACKDRIVER per appsscript.json exceptionLogging.
  // eslint-disable-next-line no-console
  console.log(JSON.stringify(entry));
}
