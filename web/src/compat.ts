// What the TV page does on a browser older than the one it was written for.
// Widgets size their text to their tile with container units (cqw, cqh)
// and container queries, which Chrome only has since version 105; many Cast
// runtimes are older. On such a browser this module adds the legacy
// stylesheets that build/legacyCss.ts writes, and sets on every tile, in
// pixels, what one container unit is and which container queries it meets.
// Modern browsers load none of it.
//
// ?compat=1 in the address of a TV page uses the fallback in any browser,
// so tests and a curious admin can compare it with the real thing.
import { LEGACY_SUFFIX, QUERY_ATTRIBUTE, UNIT_PROPERTIES } from './compatNames';

const FORCE_PARAMETER = 'compat';
const QUERY_IN_SELECTOR = /data-cq~="([^"]+)"/g;
const CONDITION = /^(width|height)(<=|>=|<|>|=)(\d*\.?\d+)px$/;

/** Fired on window when the fallback changed the size of a tile's units. */
export const CONTAINER_SIZE_EVENT = 'gdash:container-size';

/** The CSS features a screen reports; the TV page relies on some of them. */
export interface Features {
  containerUnits: boolean;
  containerQueries: boolean;
  where: boolean;
  has: boolean;
  colorMix: boolean;
  aspectRatio: boolean;
  flexGap: boolean;
  inset: boolean;
  lineClamp: boolean;
  textWrap: boolean;
  nesting: boolean;
  cascadeLayers: boolean;
}

/** Which of the features this browser has. */
export function detectFeatures(): Features {
  const supports = (property: string, value: string) => CSS.supports(property, value);
  // Chrome before 83 does not know selector() and answers false, which is
  // right: it has neither :where() nor :has().
  const selector = (s: string) => CSS.supports(`selector(${s})`);
  return {
    containerUnits: supports('width', '1cqw'),
    containerQueries: supports('container-type', 'size'),
    where: selector(':where(a)'),
    has: selector(':has(a)'),
    colorMix: supports('color', 'color-mix(in srgb, red 50%, blue)'),
    aspectRatio: supports('aspect-ratio', '1'),
    flexGap: supportsFlexGap(),
    inset: supports('inset', '0'),
    lineClamp: supports('-webkit-line-clamp', '2'),
    textWrap: supports('text-wrap', 'balance'),
    nesting: selector('&'),
    cascadeLayers: 'CSSLayerBlockRule' in window,
  };
}

// A flex browser without gap reports gap as supported, since grids have it;
// only measuring tells.
function supportsFlexGap(): boolean {
  const flex = document.createElement('div');
  flex.style.cssText = 'display:flex;flex-direction:column;row-gap:1px;position:absolute;visibility:hidden';
  flex.append(document.createElement('div'), document.createElement('div'));
  document.body.append(flex);
  const supported = flex.scrollHeight === 1;
  flex.remove();
  return supported;
}

/** Whether the address asks for the fallback in any browser. */
export function compatForced(search: string): boolean {
  return new URLSearchParams(search).get(FORCE_PARAMETER) === '1';
}

/** Whether the page uses the fallback; set once by startCompat. */
export const compat = { active: false };

/**
 * Prepares the page for the browser it runs in. Call it before the page
 * mounts: the Svelte runtime needs String.prototype.replaceAll.
 */
export function startCompat(): void {
  installReplaceAll();
  const features = detectFeatures();
  compat.active = compatForced(location.search) || !features.containerUnits || !features.containerQueries;
  if (compat.active) addLegacyStylesheets();
}

/** String.prototype.replaceAll (Chrome 85) for older browsers. */
export function replaceAll(this: string, pattern: string | RegExp, replacement: string | ((match: string, ...rest: unknown[]) => string)): string {
  if (pattern instanceof RegExp) {
    if (!pattern.global) throw new TypeError('replaceAll must be called with a global RegExp');
    return this.replace(pattern, replacement as string);
  }
  const literal = new RegExp(String(pattern).replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g');
  return this.replace(literal, replacement as string);
}

function installReplaceAll(): void {
  const prototype = String.prototype as { replaceAll?: unknown };
  if (typeof prototype.replaceAll === 'function') return;
  Object.defineProperty(String.prototype, 'replaceAll', { value: replaceAll, configurable: true, writable: true });
}

function addLegacyStylesheets(): void {
  for (const link of document.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"]')) {
    const url = new URL(link.href, location.href);
    if (url.origin !== location.origin || !url.pathname.endsWith('.css') || url.pathname.endsWith(LEGACY_SUFFIX)) continue;
    const legacy = document.createElement('link');
    legacy.rel = 'stylesheet';
    legacy.href = url.pathname.slice(0, -'.css'.length) + LEGACY_SUFFIX;
    legacy.addEventListener('load', () => {
      queries = null;
      for (const [tile, size] of sizes) applySize(tile, size);
    });
    link.after(legacy);
  }
}

interface Size {
  width: number;
  height: number;
}

// The tiles the fallback looks after, with their content size.
const sizes = new Map<HTMLElement, Size>();
// The container queries of the loaded legacy stylesheets; read again when
// one has loaded.
let queries: string[] | null = null;

function readQueries(): string[] {
  const found = new Set<string>();
  const visit = (rules: CSSRuleList) => {
    for (const rule of rules) {
      if (rule instanceof CSSStyleRule) {
        for (const match of rule.selectorText.matchAll(QUERY_IN_SELECTOR)) found.add(match[1]!);
      } else if (rule instanceof CSSMediaRule) {
        visit(rule.cssRules);
      }
    }
  };
  for (const sheet of document.styleSheets) {
    if (!sheet.href?.endsWith(LEGACY_SUFFIX)) continue;
    try {
      visit(sheet.cssRules);
    } catch {
      // A sheet that is still loading has no rules to read yet; its load
      // event reads them again.
    }
  }
  return [...found];
}

/** Whether a tile of this content size meets a query token, such as "width>=420px&height>=170px". */
export function meetsQuery(token: string, size: Size): boolean {
  return token.split('&').every((condition) => {
    const match = CONDITION.exec(condition);
    if (!match) return false;
    const actual = match[1] === 'width' ? size.width : size.height;
    const limit = Number(match[3]);
    switch (match[2]) {
      case '<=':
        return actual <= limit;
      case '>=':
        return actual >= limit;
      case '<':
        return actual < limit;
      case '>':
        return actual > limit;
      default:
        return actual === limit;
    }
  });
}

function applySize(tile: HTMLElement, size: Size): void {
  queries ??= readQueries();
  const unit = { w: size.width / 100, h: size.height / 100 };
  tile.style.setProperty(UNIT_PROPERTIES.w, `${unit.w}px`);
  tile.style.setProperty(UNIT_PROPERTIES.h, `${unit.h}px`);
  tile.style.setProperty(UNIT_PROPERTIES.min, `${Math.min(unit.w, unit.h)}px`);
  tile.style.setProperty(UNIT_PROPERTIES.max, `${Math.max(unit.w, unit.h)}px`);
  tile.setAttribute(QUERY_ATTRIBUTE, queries.filter((q) => meetsQuery(q, size)).join(' '));
  window.dispatchEvent(new Event(CONTAINER_SIZE_EVENT));
}

function contentSize(tile: HTMLElement): Size {
  // clientWidth is the layout size, before the stage is scaled to the
  // screen, as container units are.
  const style = getComputedStyle(tile);
  return {
    width: tile.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
    height: tile.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom),
  };
}

/**
 * Svelte action for a widget tile: on a browser without container queries
 * it measures the tile and keeps its unit properties and queries current.
 * Elsewhere it does nothing.
 */
export function containerFallback(tile: HTMLElement) {
  if (!compat.active) return;
  // Without this, a browser that has container queries (?compat=1) would
  // still apply its own; an older one ignores it.
  tile.style.setProperty('container-type', 'normal');
  const first = contentSize(tile);
  sizes.set(tile, first);
  applySize(tile, first);
  const observer = new ResizeObserver((entries) => {
    const box = entries[entries.length - 1]!.contentRect;
    const size = { width: box.width, height: box.height };
    sizes.set(tile, size);
    applySize(tile, size);
  });
  observer.observe(tile);
  return {
    destroy() {
      observer.disconnect();
      sizes.delete(tile);
    },
  };
}
