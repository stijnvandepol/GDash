import { locale, t } from '../i18n.svelte';
import type { Entry } from '../protocol';

/** A rule's colour: good, a warning or bad, in the theme's own colours. */
export type Tone = 'ok' | 'warn' | 'bad';

/** What the server sends for one Custom widget (internal/custom.Shown). */
export interface Shown {
  state: 'ok' | 'loading' | 'failing' | 'missing';
  stale?: boolean;
  failing?: boolean;
  mode: 'value' | 'list' | 'keyValues' | 'badges';
  title?: string;
  label?: string;
  unit?: string;
  decimals?: number;
  value?: unknown;
  rows?: { cells: unknown[]; tone?: Tone }[];
  pairs?: { label: string; value: unknown }[];
  /** The labels of the badges that are on; those that are off are not sent. */
  badges?: string[];
  tone?: Tone;
  message?: string;
  hidden?: boolean;
}

/** The widget's own part of the shared entry, keyed by dashboard and widget. */
export function shownFor(entry: Entry | undefined, key: string): Shown | undefined {
  const all = entry?.value;
  if (typeof all !== 'object' || all === null) return undefined;
  const shown = (all as Record<string, unknown>)[key];
  if (typeof shown !== 'object' || shown === null) return undefined;
  const s = shown as Shown;
  return typeof s.state === 'string' && typeof s.mode === 'string' ? s : undefined;
}

// Without decimals set, a number shows as sent, up to this many.
const DEFAULT_MAX_DECIMALS = 2;

/** A value as text in the screen's language: numbers grouped, true as yes. */
export function formatCell(value: unknown, decimals?: number): string {
  if (value === null || value === undefined) return '–';
  if (typeof value === 'number') {
    const options =
      decimals === undefined
        ? { maximumFractionDigits: DEFAULT_MAX_DECIMALS }
        : { minimumFractionDigits: decimals, maximumFractionDigits: decimals };
    return new Intl.NumberFormat(locale(), options).format(value);
  }
  if (typeof value === 'boolean') return t(value ? 'customYes' : 'customNo');
  return String(value);
}

/** What the widget says while it has no data to show. */
export function waitingFor(shown: Shown | undefined): string {
  if (!shown || shown.state === 'loading') return t('customLoading');
  if (shown.state === 'missing') return t('customMissing');
  return t('customFailed');
}
