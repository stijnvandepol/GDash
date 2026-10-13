import type { CustomAuthKind, CustomHeader, CustomLogin, CustomSource, CustomSourceInput, NamedList, NamedValue } from './api';
import type { Rule } from './customWidget';
import type { IconName } from './Icon.svelte';
import type { CategoryId, Tag } from './integrations';
import { resolvePath } from './jsonPath';

// Presets: reviewable JSON files in ./presets, one per device or service,
// that the household sees as integrations of their own. Underneath, each
// setup is a custom API with the preset's address, authentication, refresh
// and named values, and a widget is a Custom widget made from one of the
// preset's views. The address has placeholders, such as
// http://{ip}/api/v1/data, that the household fills in. A source keeps the
// preset's id, so the page finds the sources of each preset.

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
    /** For the kind "login", how to log in; its address takes the same placeholders. */
    auth: { kind: CustomAuthKind; name: string; login?: CustomLogin };
    /** Where to find the key or token, under its field. */
    secretHint?: string;
    headers: CustomHeader[];
    intervalSeconds: number;
    sample: unknown;
  };
  values: NamedValue[];
  lists: NamedList[];
  /** What one setup is, lowercase and singular, such as meter or server: "Add another meter". */
  instance: string;
  /** The widgets it offers; the first is the one a new widget starts as. */
  views: PresetView[];
}

/** A ready-made Custom widget of a preset. */
export interface PresetView {
  id: string;
  /** What the widget's settings call it, such as Power now. */
  name: string;
  mode: 'value' | 'keyValues' | 'list' | 'badges';
  title?: string;
  /**
   * The named values it shows, by name, with the label each gets: value
   * mode shows the first, large, with its label under it.
   */
  values?: { name: string; label?: string }[];
  /** The named list it shows, by name, in list mode. */
  list?: string;
  /** Its size on the dashboard's grid, at least the Custom widget's smallest. */
  w: number;
  h: number;
  /** Rules as the widget stores them, with conditions in internal/custom's language. */
  rules?: Rule[];
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
export function fillAddress(preset: Preset, answers: Record<string, string>, address = preset.source.url): string {
  const kinds = new Map(preset.placeholders.map((p) => [p.key, p.kind]));
  return address.replace(PLACEHOLDER, (_, key: string) => {
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

/** The address's placeholders as filled in for a source of the preset, or null for an address of another shape. */
export function answersFrom(preset: Preset, url: string): Record<string, string> | null {
  const keys: string[] = [];
  const pattern = preset.source.url
    .split(PLACEHOLDER)
    .map((part, i) => {
      if (i % 2 === 0) return part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      keys.push(part);
      return '(.+?)';
    })
    .join('');
  const match = new RegExp(`^${pattern}$`).exec(url);
  if (!match) return null;
  const kinds = new Map(preset.placeholders.map((p) => [p.key, p.kind]));
  const answers: Record<string, string> = {};
  keys.forEach((key, i) => {
    const raw = match[i + 1]!;
    const kind = kinds.get(key);
    answers[key] = kind === 'host' || kind === 'origin' ? raw : decodeURIComponent(raw);
  });
  return answers;
}

/** What the household fills in: a name, the placeholders, and the key or the user name and password. */
export interface PresetAnswers {
  name: string;
  answers: Record<string, string>;
  secret: string;
  /** The user name, for basic authentication. */
  user: string;
  intervalSeconds?: number;
}

/** The custom API a preset makes with the household's answers; it keeps the preset's id. */
export function presetInput(preset: Preset, { name, answers, secret, user, intervalSeconds }: PresetAnswers): CustomSourceInput {
  const { source } = preset;
  const authName = source.auth.kind === 'basic' ? user.trim() : source.auth.name;
  const login = source.auth.login;
  return {
    name: name.trim(),
    mode: 'fetch',
    url: fillAddress(preset, answers),
    method: source.method,
    body: source.method === 'POST' ? source.body : '',
    auth: {
      kind: source.auth.kind,
      name: authName,
      secret: source.auth.kind === 'none' ? '' : secret.trim(),
      ...(source.auth.kind === 'login' && login ? { login: { ...login, url: fillAddress(preset, answers, login.url) } } : {}),
    },
    headers: source.headers.map((h) => ({ ...h })),
    intervalSeconds: intervalSeconds ?? source.intervalSeconds,
    sample: source.sample,
    preset: preset.id,
  };
}

/** The sources that were set up with this preset. */
export function madeFrom(preset: Preset, sources: CustomSource[]): CustomSource[] {
  return sources.filter((s) => s.preset === preset.id);
}

/** The household's own APIs: the sources that no preset made. */
export function ownSources(sources: CustomSource[]): CustomSource[] {
  return sources.filter((s) => !s.preset);
}

/** The preset a source was set up with, if any this web interface knows. */
export function presetOf(source: Pick<CustomSource, 'preset'> | undefined): Preset | undefined {
  return source?.preset ? PRESETS.find((p) => p.id === source.preset) : undefined;
}

/**
 * Own APIs that look made from this preset before sources kept their
 * preset: their address has its shape and they name the same paths.
 * Presets for devices with one API, such as the HomeWizard meters, share
 * an address and differ in values.
 */
export function sourcesFrom(preset: Preset, sources: CustomSource[]): CustomSource[] {
  const pattern = preset.source.url
    .split(PLACEHOLDER)
    .map((part, i) => (i % 2 === 1 ? '.+?' : part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
    .join('');
  const address = new RegExp(`^${pattern}$`);
  const paths = namedPaths(preset);
  return ownSources(sources).filter((s) => s.mode === 'fetch' && address.test(s.url) && namedPaths(s) === paths);
}

/** What a named value says in an answer, with its unit, for the setup's test. */
export function shownValue(data: unknown, value: NamedValue): string {
  const { found, value: v } = resolvePath(data, value.path);
  if (!found) return 'Not in the answer';
  const text = typeof v === 'string' ? v : (JSON.stringify(v) ?? 'null');
  const clipped = text.length > 60 ? `${text.slice(0, 59)}…` : text;
  return value.unit && typeof v === 'number' ? `${clipped} ${value.unit}` : clipped;
}

/** How many items a named list has in an answer. */
export function shownList(data: unknown, list: NamedList): string {
  const { found, value } = resolvePath(data, list.path);
  if (!found || !Array.isArray(value)) return 'Not in the answer';
  return value.length === 1 ? '1 item' : `${value.length} items`;
}

/** The paths of the named values and lists, as one comparable string. */
function namedPaths(named: Pick<Preset, 'values' | 'lists'>): string {
  return JSON.stringify([named.values.map((v) => v.path).sort(), named.lists.map((l) => l.path).sort()]);
}

/** The presets in ./presets, bundled into the build, in name order. */
export const PRESETS: Preset[] = Object.values(import.meta.glob<Preset>('./presets/*.json', { eager: true, import: 'default' })).sort(
  (a, b) => a.name.localeCompare(b.name),
);
