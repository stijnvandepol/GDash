import type { CustomAuthKind, CustomSource, CustomSourceInput } from './api';

/** How often a custom API can be fetched; internal/custom allows 30 seconds to 24 hours. */
export const INTERVALS: { seconds: number; label: string }[] = [
  { seconds: 30, label: 'Every 30 seconds' },
  { seconds: 60, label: 'Every minute' },
  { seconds: 300, label: 'Every 5 minutes' },
  { seconds: 900, label: 'Every 15 minutes' },
  { seconds: 1800, label: 'Every 30 minutes' },
  { seconds: 3600, label: 'Every hour' },
  { seconds: 21600, label: 'Every 6 hours' },
  { seconds: 86400, label: 'Once a day' },
];

export const AUTH_KINDS: { kind: CustomAuthKind; label: string; nameLabel?: string; secretLabel?: string }[] = [
  { kind: 'none', label: 'None (an open API)' },
  { kind: 'bearer', label: 'Bearer token', secretLabel: 'Token' },
  { kind: 'header', label: 'API key in a header', nameLabel: 'Header name', secretLabel: 'API key' },
  { kind: 'query', label: 'API key in the address', nameLabel: 'Parameter name', secretLabel: 'API key' },
  { kind: 'basic', label: 'User name and password', nameLabel: 'User name', secretLabel: 'Password' },
];

export const MAX_HEADERS = 10;
// The server takes a sample of at most 48 KB.
const MAX_SAMPLE_BYTES = 48 * 1024;

/** The form for a new source. */
export function emptyInput(): CustomSourceInput {
  return {
    name: '',
    url: '',
    method: 'GET',
    body: '',
    auth: { kind: 'none', name: '', secret: '' },
    headers: [],
    intervalSeconds: 300,
    sample: null,
  };
}

/** The form for an existing source: secrets stay empty, which keeps them. */
export function toInput(source: CustomSource): CustomSourceInput {
  return {
    name: source.name,
    url: source.url,
    method: source.method,
    body: source.body,
    auth: { kind: source.auth.kind, name: source.auth.name, secret: '' },
    headers: source.headers.map((h) => ({ ...h })),
    intervalSeconds: source.intervalSeconds,
    sample: source.sample ?? null,
  };
}

/** The sample as text for the form. */
export function sampleText(sample: unknown): string {
  return sample === null || sample === undefined ? '' : JSON.stringify(sample, null, 2);
}

/** Reads a pasted sample answer; empty means none. */
export function parseSample(text: string): { sample: unknown } | { error: string } {
  if (text.trim() === '') return { sample: null };
  if (new Blob([text]).size > MAX_SAMPLE_BYTES) return { error: 'Paste a sample of at most 48 KB.' };
  try {
    return { sample: JSON.parse(text) as unknown };
  } catch {
    return { error: 'This is not valid JSON. Paste the answer exactly as the API sends it.' };
  }
}

/** What the secret field says when a secret is stored and may be kept. */
export function secretPlaceholder(source: CustomSource | undefined, kind: CustomAuthKind): string {
  return source?.auth.secretSet && source.auth.kind === kind ? 'Stored; leave empty to keep it' : '';
}

export function describeInterval(seconds: number): string {
  return INTERVALS.find((i) => i.seconds === seconds)?.label ?? `Every ${seconds} seconds`;
}

/** The size of an answer: "312 bytes" or "14 KB". */
export function describeSize(bytes: number): string {
  return bytes < 1024 ? `${bytes} bytes` : `${Math.round(bytes / 1024)} KB`;
}

/** The host of an address, to show in the list. */
export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}
