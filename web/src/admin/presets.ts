import type { CustomAuthKind, CustomHeader, CustomSource, CustomSourceInput, NamedList, NamedValue } from './api';
import type { IconName } from './Icon.svelte';
import type { CategoryId, Tag } from './integrations';

// Presets for a custom API: reviewable JSON files in ./presets, one per
// service, that fill in a new custom API with its address, authentication,
// refresh and named values. The address has placeholders, such as
// http://{ip}/api/v1/data, that the household fills in.

/** What a placeholder takes, and so how it is checked and put in the address. */
export type PlaceholderKind = 'host' | 'origin' | 'id' | 'text';

export interface Placeholder {
  key: string;
  label: string;
  kind: PlaceholderKind;
  /** Shown in the empty field. */
  example: string;
  /** Where to find it. */
  hint?: string;
}

export interface Preset {
  id: string;
  name: string;
  description: string;
  category: CategoryId;
  icon: IconName;
  tags: Tag[];
  /** The documentation the request and answer were checked against. */
  docs: string;
  /** Assumptions that could not be checked, for reviewers. */
  comment?: string;
  placeholders: Placeholder[];
  source: {
    name: string;
    url: string;
    method: 'GET' | 'POST';
    body: string;
    auth: { kind: CustomAuthKind; name: string };
    /** Where to find the key or token, under its field. */
    secretHint?: string;
    headers: CustomHeader[];
    intervalSeconds: number;
    sample: unknown;
  };
  values: NamedValue[];
  lists: NamedList[];
  /**
   * A rule to suggest for a widget that shows the named value or list in
   * for: summary in words, the rest as the widget editor stores it.
   */
  rule?: { for: string; summary: string; when: string; tone: string; message: string; hide?: boolean };
}

const PLACEHOLDER = /\{([a-z][a-zA-Z0-9]*)\}/g;

/** The placeholder keys in an address, in order, each once. */
export function placeholdersIn(url: string): string[] {
  return [...new Set([...url.matchAll(PLACEHOLDER)].map((m) => m[1]!))];
}

const HOST = /^[A-Za-z0-9]([A-Za-z0-9-]{0,62})(\.[A-Za-z0-9]([A-Za-z0-9-]{0,62}))*(:\d{1,5})?$/;
const ID = /^[A-Za-z0-9_-]{1,64}$/;
const MAX_TEXT = 200;

/** Why a filled-in placeholder cannot be used, or an empty string when it can. */
export function placeholderProblem(kind: PlaceholderKind, raw: string): string {
  const value = raw.trim();
  if (value === '') return 'Fill this in.';
  switch (kind) {
    case 'host':
      return HOST.test(value) ? '' : 'Enter an IP address or host name, such as 192.168.1.50, without http://.';
    case 'origin': {
      let url: URL;
      try {
        url = new URL(value);
      } catch {
        return 'Enter the full address, such as http://192.168.1.20:8096.';
      }
      if (url.protocol !== 'http:' && url.protocol !== 'https:') return 'The address starts with http:// or https://.';
      return url.search || url.hash || url.username ? 'Enter the address without ? or # or a user name.' : '';
    }
    case 'id':
      return ID.test(value) ? '' : 'Use only letters, digits, - and _.';
    case 'text':
      return value.length <= MAX_TEXT ? '' : `Use at most ${MAX_TEXT} characters.`;
  }
}

/** The address with each placeholder filled in, encoded for where it goes. */
export function fillAddress(preset: Preset, answers: Record<string, string>): string {
  const kinds = new Map(preset.placeholders.map((p) => [p.key, p.kind]));
  return preset.source.url.replace(PLACEHOLDER, (_, key: string) => {
    const value = (answers[key] ?? '').trim();
    switch (kinds.get(key)) {
      case 'host':
        return value;
      case 'origin':
        return value.replace(/\/+$/, '');
      default:
        return encodeURIComponent(value);
    }
  });
}

/** What the household fills in: a name, the placeholders, and the key or the user name and password. */
export interface PresetAnswers {
  name: string;
  answers: Record<string, string>;
  secret: string;
  /** The user name, for basic authentication. */
  user: string;
}

/** The new custom API a preset makes with the household's answers. */
export function presetInput(preset: Preset, { name, answers, secret, user }: PresetAnswers): CustomSourceInput {
  const { source } = preset;
  const authName = source.auth.kind === 'basic' ? user.trim() : source.auth.name;
  return {
    name: name.trim(),
    mode: 'fetch',
    url: fillAddress(preset, answers),
    method: source.method,
    body: source.method === 'POST' ? source.body : '',
    auth: { kind: source.auth.kind, name: authName, secret: source.auth.kind === 'none' ? '' : secret.trim() },
    headers: source.headers.map((h) => ({ ...h })),
    intervalSeconds: source.intervalSeconds,
    sample: source.sample,
  };
}

/**
 * The custom APIs that were made from this preset: their address has its
 * shape and they name the same paths. Presets for devices with one API,
 * such as the HomeWizard meters, share an address and differ in values.
 */
export function sourcesFrom(preset: Preset, sources: CustomSource[]): CustomSource[] {
  const pattern = preset.source.url
    .split(PLACEHOLDER)
    .map((part, i) => (i % 2 === 1 ? '.+?' : part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
    .join('');
  const address = new RegExp(`^${pattern}$`);
  const paths = namedPaths(preset);
  return sources.filter((s) => s.mode === 'fetch' && address.test(s.url) && namedPaths(s) === paths);
}

/** The paths of the named values and lists, as one comparable string. */
function namedPaths(named: Pick<Preset, 'values' | 'lists'>): string {
  return JSON.stringify([named.values.map((v) => v.path).sort(), named.lists.map((l) => l.path).sort()]);
}

/** The presets in ./presets, bundled into the build, in name order. */
export const PRESETS: Preset[] = Object.values(import.meta.glob<Preset>('./presets/*.json', { eager: true, import: 'default' })).sort(
  (a, b) => a.name.localeCompare(b.name),
);
