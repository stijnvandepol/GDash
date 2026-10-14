// Names shared by the build step that writes the legacy stylesheets
// (build/legacyCss.ts) and the page code that uses them (compat.ts).

/** Every stylesheet X.css is accompanied by X.legacy.css. */
export const LEGACY_SUFFIX = '.legacy.css';

/** The attribute on a widget tile that lists the container queries it meets. */
export const QUERY_ATTRIBUTE = 'data-cq';

/** The custom properties that stand in for one container unit, in pixels. */
export const UNIT_PROPERTIES = { w: '--cqw', h: '--cqh', min: '--cqmin', max: '--cqmax' } as const;
