// Writes a legacy stylesheet next to every built one, for TV browsers older
// than Chrome 105 that lack container queries and container units, and
// Chrome 85 to 87 that also lack :where(). The modern stylesheets stay as
// they are; src/compat.ts adds the legacy ones only on such a browser.
//
// A legacy stylesheet is the whole stylesheet again, so its rules keep
// their order among themselves, with:
// - container units written as calc() over custom properties that
//   compat.ts sets in pixels on every widget tile (--cqw is 1% of the
//   tile's content width);
// - each unnamed @container block turned into its rules behind an
//   attribute selector, [data-cq~="height<=420px"], which compat.ts puts on
//   the tiles that meet the query;
// - :where(.svelte-x), the scoping Svelte uses after the first class of a
//   selector, written as .svelte-x. An older browser drops a whole rule over
//   :where(); with a plain class the rule applies, a little more specific,
//   as in Svelte 4.
// Named containers (@container page, in the web interface) are left out:
// the web interface does not run on a TV.
import postcss, { type AtRule, type Root } from 'postcss';
import type { Plugin } from 'vite';
import { LEGACY_SUFFIX, QUERY_ATTRIBUTE, UNIT_PROPERTIES } from '../src/compatNames.ts';

const UNIT_PATTERN = /(?<![\w.#-])(-?(?:\d*\.)?\d+)cq(w|h|i|b|min|max)\b/g;
const WHERE_CLASS_PATTERN = /:where\((\.[\w-]+)\)/g;
const RANGE_CONDITION = /^\(\s*(width|height)\s*(<=|>=|<|>|=)\s*(\d*\.?\d+)px\s*\)$/;
const PREFIXED_CONDITION = /^\(\s*(min-|max-)?(width|height)\s*:\s*(\d*\.?\d+)px\s*\)$/;

// cqi and cqb are the inline and block size: width and height on a TV.
const UNIT_PROPERTY: Record<string, string> = {
  w: UNIT_PROPERTIES.w,
  i: UNIT_PROPERTIES.w,
  h: UNIT_PROPERTIES.h,
  b: UNIT_PROPERTIES.h,
  min: UNIT_PROPERTIES.min,
  max: UNIT_PROPERTIES.max,
};

/** Container units in a CSS value as calc() over the tile's unit properties. */
export function replaceContainerUnits(value: string): string {
  return value.replace(UNIT_PATTERN, (_, amount: string, unit: string) => `calc(${amount} * var(${UNIT_PROPERTY[unit]}))`);
}

/**
 * The token for an unnamed container query, such as "width>=420px&height>=170px".
 * It throws for a query the fallback cannot evaluate, so a new kind of
 * query fails the build instead of silently never matching on a TV.
 */
export function queryToken(params: string): string {
  return params
    .split(/\s+and\s+/i)
    .map((condition) => {
      const range = RANGE_CONDITION.exec(condition.trim());
      if (range) return `${range[1]}${range[2]}${range[3]}px`;
      const prefixed = PREFIXED_CONDITION.exec(condition.trim());
      if (prefixed) {
        const operator = prefixed[1] === 'min-' ? '>=' : prefixed[1] === 'max-' ? '<=' : '=';
        return `${prefixed[2]}${operator}${prefixed[3]}px`;
      }
      throw new Error(`legacy CSS: the container query "${params}" is not supported by the TV fallback; use width and height in px, joined by "and"`);
    })
    .join('&');
}

function unfoldContainer(rule: AtRule): void {
  if (!rule.params.trim().startsWith('(')) {
    rule.remove();
    return;
  }
  const token = queryToken(rule.params);
  rule.each((child) => {
    if (child.type === 'comment') return;
    if (child.type !== 'rule') {
      throw new Error(`legacy CSS: @container ${rule.params} holds a ${child.type === 'atrule' ? '@' + child.name : child.type}; the TV fallback only unfolds plain rules`);
    }
    rule.before(child.clone({ selectors: child.selectors.map((selector) => `[${QUERY_ATTRIBUTE}~="${token}"] ${selector}`) }));
  });
  rule.remove();
}

/** The legacy stylesheet for a built one; only a comment when nothing in it needs a fallback. */
export function legacyStylesheet(css: string): string {
  const root: Root = postcss.parse(css);
  let changed = false;
  root.walkAtRules('container', (rule) => {
    changed = true;
    unfoldContainer(rule);
  });
  root.walkRules((rule) => {
    const selector = rule.selector.replace(WHERE_CLASS_PATTERN, '$1');
    if (selector !== rule.selector) {
      changed = true;
      rule.selector = selector;
    }
  });
  root.walkDecls((decl) => {
    const value = replaceContainerUnits(decl.value);
    if (value !== decl.value) {
      changed = true;
      decl.value = value;
    }
  });
  if (!changed) return '/* Nothing in this stylesheet needs a fallback on older TV browsers. */\n';
  // The browser already has the faces from the modern stylesheet.
  root.walkAtRules('font-face', (rule) => {
    rule.remove();
  });
  return root.toString();
}

/** The Vite plugin: emits X.legacy.css for every built X.css. */
export function legacyCss(): Plugin {
  return {
    name: 'gdash-legacy-css',
    apply: 'build',
    enforce: 'post',
    generateBundle(_, bundle) {
      for (const file of Object.values(bundle)) {
        if (file.type !== 'asset' || !file.fileName.endsWith('.css')) continue;
        const source = typeof file.source === 'string' ? file.source : new TextDecoder().decode(file.source);
        this.emitFile({ type: 'asset', fileName: file.fileName.slice(0, -'.css'.length) + LEGACY_SUFFIX, source: legacyStylesheet(source) });
      }
    },
  };
}
