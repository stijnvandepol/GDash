import { describe, expect, it } from 'vitest';
import type { ScreenReport } from './api';
import { describeBrowser, describeScreenSize, screenWarnings } from './screenReports';

const MODERN = {
  containerUnits: true,
  containerQueries: true,
  where: true,
  has: true,
  colorMix: true,
  aspectRatio: true,
  flexGap: true,
  inset: true,
  lineClamp: true,
  textWrap: true,
  nesting: true,
  cascadeLayers: true,
};

function report(change: Partial<ScreenReport> = {}): ScreenReport {
  return {
    userAgent: 'Mozilla/5.0',
    viewport: { width: 1280, height: 720 },
    devicePixelRatio: 1.5,
    stageScale: 0.6667,
    features: MODERN,
    compat: false,
    fontsLoaded: true,
    firstRenderMs: 1800,
    errors: [],
    ...change,
  };
}

describe('describeBrowser', () => {
  it.each([
    [
      'Mozilla/5.0 (X11; Linux armv7l) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36 CrKey/1.56.500000 DeviceType/Chromecast',
      'Chrome 91 (Cast)',
    ],
    [
      'Mozilla/5.0 (Linux; Android 12.0; Build/STTL.240206.002) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 CrKey/1.56.500000 DeviceType/AndroidTV',
      'Chrome 87 (Cast)',
    ],
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36', 'Chrome 140'],
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0', 'Edge 140'],
    [
      'Mozilla/5.0 (SMART-TV; LINUX; Tizen 6.0) AppleWebKit/537.36 (KHTML, like Gecko) 76.0.3809.146/6.0 TV Safari/537.36 SamsungBrowser/4.0',
      'Samsung Internet 4 (Tizen)',
    ],
    ['Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1', 'Safari 17.4'],
    ['Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0', 'Firefox 131'],
    ['Mozilla/5.0 (Web0S; Linux/SmartTV) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.79 Safari/537.36 WebAppManager', 'Chrome 79 (webOS)'],
    ['curl/8.0', 'An unknown browser'],
  ])('%s is %s', (userAgent, want) => {
    expect(describeBrowser(userAgent)).toBe(want);
  });
});

describe('describeScreenSize', () => {
  it('names the size, the pixel ratio and how large the dashboard is drawn', () => {
    expect(describeScreenSize(report())).toBe('1280 × 720 at 1.5× · the dashboard drawn at 67%');
    expect(describeScreenSize(report({ viewport: { width: 1920, height: 1080 }, devicePixelRatio: 1, stageScale: 1 }))).toBe(
      '1920 × 1080 · the dashboard drawn at 100%',
    );
  });
});

describe('screenWarnings', () => {
  it('has nothing to say about a modern browser', () => {
    expect(screenWarnings(report())).toEqual([]);
  });

  it('explains what an older TV browser lacks and what GDash does about it', () => {
    const warnings = screenWarnings(
      report({
        features: { ...MODERN, containerUnits: false, containerQueries: false, where: false, flexGap: false, aspectRatio: false, has: false },
        compat: true,
        fontsLoaded: false,
        firstRenderMs: 14_200,
        errors: [{ message: 'TypeError: x is not a function', source: '/assets/App.js:1:2' }],
      }),
    );
    expect(warnings).toEqual([
      'This browser does not support container units and container queries (Chrome 105 and later); GDash uses its fallback, which sizes the text the same way.',
      "This browser does not support :where() (Chrome 88 and later); GDash's fallback stylesheet covers it.",
      'This browser leaves out the gaps between items in a row (Chrome 84 and later), so spacing on this screen is tighter than in the preview.',
      'This browser does not support aspect-ratio (Chrome 88 and later); GDash sizes camera images and covers without it.',
      'The fonts had not loaded when the screen reported; text may show in another typeface until they have.',
      'The dashboard took 14 seconds to appear; the screen or the network is slow.',
      'The page ran into a script error, listed below.',
    ]);
  });

  it('says when the fallback was asked for on a browser that does not need it', () => {
    expect(screenWarnings(report({ compat: true }))).toEqual([
      'The fallback for older browsers is on (?compat=1 in the address), although this browser does not need it.',
    ]);
  });

  it('says when no dashboard was drawn yet', () => {
    expect(screenWarnings(report({ firstRenderMs: null }))).toEqual([
      'The screen had not drawn a dashboard yet when it reported, for example because the display was asleep.',
    ]);
  });
});
