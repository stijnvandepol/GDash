import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import type { Browser, Page } from '@playwright/test';
import { expect, test } from './fixtures';

// The owner's kitchen TV, a Chromecast HD: 1280x720 CSS pixels at 1.5.
// Its Cast runtime may be older than Chrome 105, which lacks container
// queries; ?compat=1 makes this Chromium use the fallback that such a TV
// uses (src/compat.ts), so both can be compared. The data is answered
// here, so the layout does not depend on a weather or news service.

const DASHBOARD = 'e2e-tv';
const SCREENS = 'test-results/tv-screens';

const TV = { width: 1280, height: 720, scale: 1.5 };
const FULL_HD = { width: 1920, height: 1080, scale: 1 };

const HOUR = 3_600_000;
const DAY = 24 * HOUR;

function localDate(ms: number): string {
  const d = new Date(ms);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function entry(value: unknown, now: number) {
  return { value, fetchedAt: new Date(now).toISOString(), staleAfter: new Date(now + HOUR).toISOString(), failing: false };
}

/** The owner's dashboard: clock, waste, who's home, weather with the KNMI, news and power use. */
function snapshot() {
  const now = Date.now();
  const dashboard = {
    id: DASHBOARD,
    name: 'Keuken',
    theme: 'glass',
    columns: 24,
    rows: 14,
    widgets: [
      { id: 'clock', type: 'clock', x: 0, y: 0, w: 10, h: 4 },
      { id: 'waste', type: 'waste', x: 10, y: 0, w: 6, h: 4, source: 'waste', settings: { calendar: 7 } },
      { id: 'home', type: 'homeypresence', x: 16, y: 0, w: 8, h: 4, source: 'homey.presence' },
      { id: 'weather', type: 'weather', x: 0, y: 4, w: 10, h: 6, source: 'weather', settings: { current: 'knmi' } },
      { id: 'news', type: 'news', x: 10, y: 4, w: 14, h: 10, source: 'news' },
      { id: 'power', type: 'custom', x: 0, y: 10, w: 10, h: 4, source: 'custom.default' },
    ],
  };
  const headlines = [
    'Kabinet presenteert plannen voor betaalbare huurwoningen in de grote steden',
    'Storm verwacht in het noorden: code oranje voor de Waddeneilanden vanaf vanavond',
    'Treinverkeer rond Utrecht weer op gang na een urenlange storing in de seinen',
    'Nieuw record: nooit eerder zoveel zonnestroom opgewekt in oktober',
    'Gemeente opent extra fietsenstalling bij het station',
  ];
  return {
    type: 'snapshot',
    version: 1,
    serverTime: new Date(now).toISOString(),
    language: 'nl',
    textScale: 100,
    dashboard,
    data: {
      waste: entry(
        {
          7: [
            { kind: 'gft', title: 'GFT', date: localDate(now + DAY) },
            { kind: 'pmd', title: 'PMD', date: localDate(now + 3 * DAY) },
          ],
        },
        now,
      ),
      'homey.presence': entry({ [`${DASHBOARD}/home`]: ['SV', 'EM'] }, now),
      weather: entry({ temperatureC: 14.6, isDay: true, weatherCode: 3, windKmh: 18, todayMaxC: 16, todayMinC: 9 }, now),
      'knmi.observation': entry({ station: 'Volkel', observedAt: new Date(now - 10 * 60_000).toISOString(), temperatureC: 14.2, humidity: 81, windKmh: 16 }, now),
      news: entry({ '1': { name: 'NOS Nieuws', items: headlines.map((title, i) => ({ title, published: new Date(now - i * HOUR).toISOString() })) } }, now),
      'custom.default': entry({ [`${DASHBOARD}/power`]: { state: 'ok', mode: 'value', title: 'Stroom', value: 1234, unit: ' W', label: 'Verbruik nu' } }, now),
    },
  };
}

async function openTv(browser: Browser, baseURL: string | undefined, screen: typeof TV, compat: boolean): Promise<Page> {
  const context = await browser.newContext({ baseURL, viewport: { width: screen.width, height: screen.height }, deviceScaleFactor: screen.scale });
  const page = await context.newPage();
  await page.route(`**/api/dashboards/${DASHBOARD}/preview*`, (route) => route.fulfill({ json: snapshot() }));
  await page.goto(`/preview/${DASHBOARD}${compat ? '?compat=1' : ''}`);
  await expect(page.locator('section.widget')).toHaveCount(6);
  await expect(page.locator('.badge')).toHaveText(['SV', 'EM']);
  await page.evaluate(() => document.fonts.ready.then(() => undefined));
  if (compat) {
    // The legacy stylesheet has loaded and the tiles carry their queries.
    await expect(page.locator('link[href$=".legacy.css"]').first()).toBeAttached();
    await expect.poll(() => page.evaluate(() => [...document.styleSheets].some((s) => s.href?.endsWith('.legacy.css') && s.cssRules.length > 0))).toBe(true);
    await expect(page.locator('section.widget[data-cq]')).toHaveCount(6);
    // This Chromium's own container queries are off, as on an older TV.
    expect(await page.locator('section.widget').first().evaluate((el) => getComputedStyle(el).containerType)).toBe('normal');
  }
  // Two frames, so the fallback's last measurements are drawn.
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  return page;
}

/** What to compare: the text of each widget, by a selector inside the stage. */
const MEASURED = [
  '.clock .time',
  '.clock .date',
  '.waste li .kind',
  '.waste li .when',
  '.badge',
  '.weather .temperature',
  '.weather .icon',
  '.weather .description',
  '.weather .range',
  '.weather .measured',
  '.news .source',
  '.news li .title',
  '.custom .title',
  '.custom .big',
  '.custom .label',
];

interface Box {
  selector: string;
  index: number;
  x: number;
  y: number;
  width: number;
  height: number;
  fontSize: number;
  visible: boolean;
}

function measure(page: Page): Promise<Box[]> {
  return page.evaluate((selectors) => {
    const out: Box[] = [];
    for (const selector of selectors) {
      document.querySelectorAll<HTMLElement>(selector).forEach((el, index) => {
        const r = el.getBoundingClientRect();
        out.push({
          selector,
          index,
          x: r.x,
          y: r.y,
          width: r.width,
          height: r.height,
          fontSize: parseFloat(getComputedStyle(el).fontSize),
          visible: el.getClientRects().length > 0,
        });
      });
    }
    return out;
  }, MEASURED);
}

/** Text that reaches outside its tile's content box, in CSS pixels. */
function overflowingText(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const problems: string[] = [];
    for (const tile of document.querySelectorAll<HTMLElement>('section.widget')) {
      const box = tile.getBoundingClientRect();
      const style = getComputedStyle(tile);
      // The stage is scaled; padding is in stage pixels.
      const scale = box.width / tile.offsetWidth;
      const inner = {
        left: box.left + parseFloat(style.paddingLeft) * scale - 1,
        right: box.right - parseFloat(style.paddingRight) * scale + 1,
        top: box.top + parseFloat(style.paddingTop) * scale - 1,
        bottom: box.bottom - parseFloat(style.paddingBottom) * scale + 1,
      };
      // The boxes of the elements that hold text, by their line height: a
      // range around the text would also count the empty space above and
      // below the glyphs that a face reserves.
      const walker = document.createTreeWalker(tile, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (!node.textContent?.trim() || !node.parentElement) continue;
        for (const r of node.parentElement.getClientRects()) {
          if (r.width === 0) continue;
          if (r.left < inner.left || r.right > inner.right || r.top < inner.top || r.bottom > inner.bottom) {
            problems.push(`${tile.firstElementChild?.className.split(' ')[0]}: "${node.textContent.trim().slice(0, 40)}"`);
            break;
          }
        }
      }
    }
    return problems;
  });
}

async function screenshot(page: Page, name: string): Promise<void> {
  await mkdir(SCREENS, { recursive: true });
  await page.screenshot({ path: path.join(SCREENS, `${name}.png`) });
}

test('the fallback for older TVs draws the owner\'s layout as the browser\'s own container queries do', async ({ browser, baseURL }) => {
  const native = await openTv(browser, baseURL, TV, false);
  const compat = await openTv(browser, baseURL, TV, true);
  try {
    // The native page loads no legacy stylesheet and leaves the tiles alone.
    await expect(native.locator('link[href$=".legacy.css"]')).toHaveCount(0);
    await expect(native.locator('section.widget[data-cq]')).toHaveCount(0);

    const expected = await measure(native);
    const actual = await measure(compat);
    expect(actual.map((b) => `${b.selector}#${b.index}`)).toEqual(expected.map((b) => `${b.selector}#${b.index}`));
    for (const [i, want] of expected.entries()) {
      const got = actual[i]!;
      const where = `${want.selector}#${want.index}`;
      expect(got.visible, `${where} visible`).toBe(want.visible);
      expect(Math.abs(got.fontSize - want.fontSize), `${where} font size ${got.fontSize} vs ${want.fontSize}`).toBeLessThan(0.1);
      for (const key of ['x', 'y', 'width', 'height'] as const) {
        expect(Math.abs(got[key] - want[key]), `${where} ${key} ${got[key]} vs ${want[key]}`).toBeLessThan(1);
      }
    }
    await screenshot(native, 'native-1280x720@1.5');
    await screenshot(compat, 'compat-1280x720@1.5');
  } finally {
    await native.context().close();
    await compat.context().close();
  }
});

for (const [name, screen, compat] of [
  ['native-1280x720@1.5', TV, false],
  ['compat-1280x720@1.5', TV, true],
  ['native-1920x1080', FULL_HD, false],
] as const) {
  test(`on the owner's layout nothing overflows its tile and the text stays readable (${name})`, async ({ browser, baseURL }) => {
    const page = await openTv(browser, baseURL, screen, compat);
    try {
      expect(await overflowingText(page)).toEqual([]);
      const size = async (selector: string) => parseFloat(await page.locator(selector).first().evaluate((el) => getComputedStyle(el).fontSize));
      // Sizes on the 1920-pixel stage; a 1280 TV draws them at two thirds.
      expect(await size('.waste li .kind')).toBeGreaterThanOrEqual(34);
      expect(await size('.weather .measured')).toBeGreaterThanOrEqual(24);
      expect(await size('.weather .description')).toBeGreaterThanOrEqual(36);
      // A badge stays a circle with its ring.
      const badge = await page.locator('.badge').first().evaluate((el) => {
        const s = getComputedStyle(el);
        return { width: el.offsetWidth, height: el.offsetHeight, border: parseFloat(s.borderTopWidth) };
      });
      expect(Math.abs(badge.width - badge.height)).toBeLessThan(1);
      expect(badge.height).toBeGreaterThanOrEqual(56);
      expect(badge.border).toBeGreaterThanOrEqual(3);
      // All five headlines of the tall news tile are shown.
      await expect(page.locator('.news li:not([hidden])')).toHaveCount(5);
      await screenshot(page, `layout-${name}`);
    } finally {
      await page.context().close();
    }
  });
}
