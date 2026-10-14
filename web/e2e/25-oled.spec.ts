import { mkdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { expect, openTab, test } from './fixtures';

// The OLED and night presets in the style picker. The presets are also
// drawn on a realistic dashboard at 1920x1080, one picture each and a
// contact sheet of all of them, for a look by eye; the data is answered
// here, so they do not depend on any service.

const OLED_THEMES: [string, string][] = [
  ['void', 'Void'],
  ['ember', 'Ember'],
  ['moonlight', 'Moonlight'],
  ['aurora', 'Aurora'],
  ['deep-sea', 'Deep sea'],
  ['firefly', 'Firefly'],
  ['graphite-oled', 'Graphite OLED'],
  ['red-night', 'Red night'],
  ['starlight', 'Starlight'],
  ['ink', 'Ink'],
];

const SCREENS = 'test-results/oled-screens';
const HOUR = 3_600_000;
const DAY = 24 * HOUR;

interface WidgetTypes {
  types: Record<string, { minW: number; minH: number }>;
}

test('the style picker marks each theme for OLED and can show only the OLED-friendly ones', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E OLED');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  await page.getByRole('button', { name: 'Clock', exact: true }).click();
  await page.getByRole('button', { name: /^Style/ }).click();
  const picker = page.getByRole('dialog', { name: 'Style' });
  const themes = picker.locator('.themes');

  // Every card says how it does on OLED.
  await expect(picker.getByRole('radio', { name: /^Walnut OLED OK/ })).toBeAttached();
  await expect(picker.getByRole('radio', { name: /^Minimal light Bright/ })).toBeAttached();
  await expect(picker.getByRole('radio', { name: /^Starlight OLED-friendly/ })).toBeAttached();

  await picker.getByLabel('Only OLED-friendly').check();
  await mkdir(SCREENS, { recursive: true });
  await picker.screenshot({ path: path.join(SCREENS, 'style-picker.png') });
  for (const [, name] of OLED_THEMES) await expect(themes.getByText(name, { exact: true })).toBeVisible();
  await expect(themes.getByText('Walnut', { exact: true })).toHaveCount(0);
  await expect(themes.getByText('Minimal light', { exact: true })).toHaveCount(0);
  // A section without one friendly theme is left out.
  await expect(picker.getByRole('heading', { name: 'Character' })).toHaveCount(0);
  await expect(picker.getByRole('heading', { name: 'OLED and night' })).toBeVisible();

  // Each is also a choice for after sunset.
  const night = picker.getByLabel('Style between sunset and sunrise');
  for (const [, name] of OLED_THEMES) await expect(night.getByRole('option', { name, exact: true })).toBeAttached();

  await themes.getByText('Ember', { exact: true }).click();
  await expect(picker.getByRole('radio', { name: /^Ember/ })).toBeChecked();
  await picker.getByLabel('Only OLED-friendly').uncheck();
  await expect(themes.getByText('Walnut', { exact: true })).toBeVisible();
});

function localDate(ms: number): string {
  const d = new Date(ms);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function entry(value: unknown, now: number) {
  return { value, fetchedAt: new Date(now).toISOString(), staleAfter: new Date(now + HOUR).toISOString(), failing: false };
}

/** A household's dashboard: clock, weather, calendar, news, who's home and the waste collection. */
function snapshot(theme: string) {
  const now = Date.now();
  const id = 'e2e-oled-look';
  const at = (days: number, hour: number, minute = 0) => {
    const d = new Date(now + days * DAY);
    d.setHours(hour, minute, 0, 0);
    return d.toISOString();
  };
  const event = (title: string, start: string, end: string, color: string, allDay = false) => ({ title, start, end, allDay, calendar: 'Family', color });
  return {
    type: 'snapshot',
    version: 1,
    serverTime: new Date(now).toISOString(),
    language: 'en',
    textScale: 100,
    dashboard: {
      id,
      name: 'Living room',
      theme,
      columns: 24,
      rows: 14,
      widgets: [
        { id: 'clock', type: 'clock', x: 0, y: 0, w: 10, h: 5 },
        { id: 'weather', type: 'weather', x: 0, y: 5, w: 10, h: 5, source: 'weather' },
        { id: 'calendar', type: 'calendar', x: 10, y: 0, w: 14, h: 8, source: 'calendar', settings: { maxItems: 5 } },
        { id: 'news', type: 'news', x: 10, y: 8, w: 14, h: 6, source: 'news', settings: { maxItems: 3 } },
        { id: 'home', type: 'homeypresence', x: 0, y: 10, w: 4, h: 4, source: 'homey.presence' },
        { id: 'waste', type: 'waste', x: 4, y: 10, w: 6, h: 4, source: 'waste', settings: { calendar: 7 } },
      ],
    },
    data: {
      weather: entry({ temperatureC: 11.4, isDay: false, weatherCode: 2, windKmh: 14, todayMaxC: 15, todayMinC: 7 }, now),
      calendar: entry(
        [
          event('Dentist, Emma', at(0, 16, 30), at(0, 17, 15), 'blue'),
          event('Football training', at(0, 19), at(0, 20, 30), 'green'),
          event('Grandma’s birthday', localDate(now + DAY), localDate(now + 2 * DAY), 'pink', true),
          event('Parents’ evening at school', at(1, 19, 30), at(1, 21), 'orange'),
          event('Car to the garage', at(2, 8, 15), at(2, 9), 'purple'),
        ],
        now,
      ),
      news: entry(
        {
          '1': {
            name: 'World news',
            items: [
              'Night trains return on the line to Vienna after five years',
              'Record harvest for apple growers after a warm, wet summer',
              'City council approves a new cycle bridge over the river',
            ].map((title, i) => ({ title, published: new Date(now - i * HOUR).toISOString() })),
          },
        },
        now,
      ),
      'homey.presence': entry({ [`${id}/home`]: ['SV', 'EM', 'JO'] }, now),
      waste: entry(
        {
          7: [
            { kind: 'gft', title: 'GFT', date: localDate(now + DAY) },
            { kind: 'paper', title: 'Paper', date: localDate(now + 4 * DAY) },
          ],
        },
        now,
      ),
    },
  };
}

test('each OLED and night preset is drawn on a household dashboard, for a look by eye', async ({ browser, baseURL }) => {
  test.setTimeout(120_000);
  await mkdir(SCREENS, { recursive: true });
  const context = await browser.newContext({ baseURL, viewport: { width: 1920, height: 1080 } });
  try {
    const page = await context.newPage();
    for (const [theme] of OLED_THEMES) {
      await page.unrouteAll();
      await page.route('**/api/dashboards/e2e-oled-look/preview*', (route) => route.fulfill({ json: snapshot(theme) }));
      await page.goto('/preview/e2e-oled-look');
      await expect(page.locator('main.screen')).toHaveAttribute('data-theme', theme);
      await expect(page.locator('section.widget')).toHaveCount(6);
      await expect(page.locator('.badge')).toHaveCount(3);
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      // The backdrop is pure black wherever no tile or text is.
      const corner = await page.locator('main.screen').evaluate((el) => getComputedStyle(el).backgroundColor);
      expect(corner, theme).toBe('rgb(0, 0, 0)');
      await page.screenshot({ path: path.join(SCREENS, `${theme}.png`) });
    }
    // All of them on one sheet, five by two.
    const images = await Promise.all(OLED_THEMES.map(async ([theme, name]) => ({ name, src: `data:image/png;base64,${(await readFile(path.join(SCREENS, `${theme}.png`))).toString('base64')}` })));
    await page.setViewportSize({ width: 2600, height: 700 });
    // A blank page: the server's content security policy refuses inline styles.
    await page.goto('about:blank');
    await page.setContent(
      `<body style="margin:0;background:#202020;font:600 18px system-ui;color:#ddd;display:grid;grid-template-columns:repeat(5,480px);gap:24px 24px;padding:24px">` +
        images.map((i) => `<figure style="margin:0"><img src="${i.src}" width="480" height="270" style="display:block;border:1px solid #444"><figcaption style="padding-top:6px">${i.name}</figcaption></figure>`).join('') +
        '</body>',
    );
    await page.evaluate(() => Promise.all([...document.images].map((img) => img.decode())));
    await page.screenshot({ path: path.join(SCREENS, 'contact-sheet.png'), fullPage: true });
  } finally {
    await context.close();
  }
});
