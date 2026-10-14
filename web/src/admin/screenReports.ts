// What the Displays page says about the reports screens send about their
// browsers (src/screenReport.ts, internal/screenreport).
import type { ScreenReport } from './api';

// The first match names the browser: Edge and Samsung Internet also say
// Chrome, Chrome also says Safari.
const BROWSERS: [RegExp, string][] = [
  [/\bEdg(?:e|A|iOS)?\/(\d+)/, 'Edge'],
  [/\bSamsungBrowser\/(\d+)/, 'Samsung Internet'],
  [/\bOPR\/(\d+)/, 'Opera'],
  [/\b(?:Firefox|FxiOS)\/(\d+)/, 'Firefox'],
  [/(?:Chrome|CriOS)\/(\d+)/, 'Chrome'],
  [/\bVersion\/(\d+(?:\.\d+)?).*\bSafari\//, 'Safari'],
];

// Where the browser runs, when that explains how it behaves.
const PLATFORMS: [RegExp, string][] = [
  [/\bCrKey\//, 'Cast'],
  [/\bTizen\b/, 'Tizen'],
  [/\bWeb0S\b/, 'webOS'],
];

/** The browser in a user agent in a few words: "Chrome 87 (Cast)". */
export function describeBrowser(userAgent: string): string {
  const browser = BROWSERS.map(([pattern, name]) => {
    const match = pattern.exec(userAgent);
    return match ? `${name} ${match[1]}` : null;
  }).find((found) => found !== null);
  const platform = PLATFORMS.find(([pattern]) => pattern.test(userAgent))?.[1];
  const name = browser ?? 'An unknown browser';
  return platform ? `${name} (${platform})` : name;
}

/** "1280 × 720 at 1.5× · the dashboard drawn at 67%". */
export function describeScreenSize(report: ScreenReport): string {
  const { width, height } = report.viewport;
  const ratio = report.devicePixelRatio === 1 ? '' : ` at ${Number(report.devicePixelRatio.toFixed(2))}×`;
  return `${width} × ${height}${ratio} · the dashboard drawn at ${Math.round(report.stageScale * 100)}%`;
}

const SLOW_RENDER_MS = 10_000;

/** What an admin should know about a screen, in plain words; empty when all is well. */
export function screenWarnings(report: ScreenReport): string[] {
  const warnings: string[] = [];
  const f = report.features;
  if (!f.containerUnits || !f.containerQueries) {
    warnings.push("This browser does not support container units and container queries (Chrome 105 and later); GDash uses its fallback, which sizes the text the same way.");
  } else if (report.compat) {
    warnings.push('The fallback for older browsers is on (?compat=1 in the address), although this browser does not need it.');
  }
  if (!f.where) warnings.push("This browser does not support :where() (Chrome 88 and later); GDash's fallback stylesheet covers it.");
  if (!f.flexGap) warnings.push('This browser leaves out the gaps between items in a row (Chrome 84 and later), so spacing on this screen is tighter than in the preview.');
  if (!f.aspectRatio) warnings.push('This browser does not support aspect-ratio (Chrome 88 and later); GDash sizes camera images and covers without it.');
  if (!report.fontsLoaded) warnings.push('The fonts had not loaded when the screen reported; text may show in another typeface until they have.');
  if (report.firstRenderMs === null) {
    warnings.push('The screen had not drawn a dashboard yet when it reported, for example because the display was asleep.');
  } else if (report.firstRenderMs > SLOW_RENDER_MS) {
    warnings.push(`The dashboard took ${Math.round(report.firstRenderMs / 1000)} seconds to appear; the screen or the network is slow.`);
  }
  if (report.errors.length > 0) {
    warnings.push(`The page ran into ${report.errors.length === 1 ? 'a script error' : `${report.errors.length} script errors`}, listed below.`);
  }
  return warnings;
}
