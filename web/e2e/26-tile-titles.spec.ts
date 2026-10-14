import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import type { Page } from '@playwright/test';
import { expect, openTab, test } from './fixtures';

// Every tile that does not explain itself says what it shows: a title in
// the TV's language, which the admin can change or turn off per widget.

const SCREENS = 'test-results/tile-titles';
const HOUR = 3_600_000;

function entry(value: unknown) {
  const now = Date.now();
  return { value, fetchedAt: new Date(now).toISOString(), staleAfter: new Date(now + HOUR).toISOString(), failing: false };
}

/** A screen with an empty household: nobody home, no pickups, and titles typed, turned off or left alone. */
function snapshot(language: string) {
  const id = 'e2e-titles';
  return {
    type: 'snapshot',
    version: 1,
    serverTime: new Date().toISOString(),
    language,
    textScale: 100,
    dashboard: {
      id,
      name: 'Titles',
      theme: 'minimal-dark',
      columns: 24,
      rows: 14,
      widgets: [
        { id: 'clock', type: 'clock', x: 0, y: 0, w: 10, h: 4, settings: { title: 'Keuken' } },
        { id: 'waste', type: 'waste', x: 10, y: 0, w: 6, h: 4, source: 'waste', settings: { calendar: 7 } },
        { id: 'home', type: 'homeypresence', x: 16, y: 0, w: 8, h: 4, source: 'homey.presence' },
        { id: 'quiet', type: 'homeypresence', x: 0, y: 4, w: 8, h: 4, source: 'homey.presence', settings: { hideTitle: true } },
        { id: 'people', type: 'custom', x: 8, y: 4, w: 8, h: 4, source: 'custom.default', settings: { source: 1, mode: 'badges', pairs: [] } },
        { id: 'weather', type: 'weather', x: 16, y: 4, w: 8, h: 4, source: 'weather' },
      ],
    },
    data: {
      waste: entry({ 7: [] }),
      'homey.presence': entry({ [`${id}/home`]: [], [`${id}/quiet`]: [] }),
      'custom.default': entry({ [`${id}/people`]: { state: 'ok', mode: 'badges', badges: [] } }),
      weather: entry({ temperatureC: 14.6, isDay: true, weatherCode: 3, windKmh: 18, todayMaxC: 16, todayMinC: 9 }),
    },
  };
}

async function openScreen(page: Page, language: string): Promise<void> {
  await page.route('**/api/dashboards/e2e-titles/preview*', (route) => route.fulfill({ json: snapshot(language) }));
  await page.goto('/preview/e2e-titles');
  await expect(page.locator('section.widget')).toHaveCount(6);
}

test('tiles have a title in the language of the TV, the admin’s own, or none', async ({ adminPage: page }) => {
  await openScreen(page, 'en');
  const titleOf = (id: number) => page.locator('section.widget').nth(id).locator('.tile-title');
  await expect(titleOf(0)).toHaveText('Keuken');
  await expect(titleOf(1)).toHaveText('Waste collection');
  await expect(titleOf(2)).toHaveText('Who’s home');
  await expect(titleOf(3)).toHaveCount(0);
  await expect(titleOf(4)).toHaveText('Who’s home');
  // The weather explains itself.
  await expect(titleOf(5)).toHaveCount(0);

  // An empty household is said calmly, not left blank.
  await expect(page.locator('section.widget').nth(1)).toContainText('No pickups planned');
  await expect(page.locator('section.widget').nth(2)).toContainText('Nobody is home');
  await expect(page.locator('section.widget').nth(4)).toContainText('Nobody is home');

  await page.unrouteAll();
  await openScreen(page, 'nl');
  await expect(page.locator('.tile-title')).toHaveText(['Keuken', 'Afval', 'Wie is thuis', 'Wie is thuis']);
  await expect(page.locator('section.widget').nth(2)).toContainText('Niemand thuis');
});

for (const screen of [
  { name: '1280x720@1.5', width: 1280, height: 720, scale: 1.5 },
  { name: '1920x1080', width: 1920, height: 1080, scale: 1 },
]) {
  test(`the Waste and Who's home tiles with their titles at ${screen.name}`, async ({ browser, baseURL }) => {
    const context = await browser.newContext({ baseURL, viewport: { width: screen.width, height: screen.height }, deviceScaleFactor: screen.scale });
    const page = await context.newPage();
    try {
      await openScreen(page, 'nl');
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      await mkdir(SCREENS, { recursive: true });
      await page.screenshot({ path: path.join(SCREENS, `empty-${screen.name}.png`) });
      // The title never pushes the data out of its tile.
      for (const tile of await page.locator('section.widget').all()) {
        const [outer, body] = await Promise.all([tile.boundingBox(), tile.locator('.tile-body').boundingBox()]);
        expect(body!.y + body!.height).toBeLessThanOrEqual(outer!.y + outer!.height + 0.5);
      }
    } finally {
      await context.close();
    }
  });
}

test('the editor sets a widget’s title or turns it off', async ({ adminPage: page }) => {
  const created = await page.request.post('/api/dashboards', {
    data: { id: 'e2e-title-editor', name: 'E2E Title editor', widgets: [{ id: 'bins', type: 'waste', x: 0, y: 0, w: 6, h: 4 }] },
  });
  expect(created.status(), await created.text()).toBe(201);
  await openTab(page, 'Dashboards');
  await page.getByRole('listitem').filter({ hasText: 'E2E Title editor' }).getByRole('button', { name: 'Edit', exact: true }).click();
  await page.locator('.grid-stack-item[gs-id="bins"]').click();
  const panel = page.getByRole('complementary', { name: 'Widget settings' });
  const title = panel.getByLabel('Title', { exact: true });
  await expect(title).toHaveAttribute('placeholder', 'Waste collection');
  await expect(panel).toContainText('Empty shows “Waste collection”, in the language of the TVs.');

  await title.fill('Containers');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();
  type Stored = { widgets: { settings?: Record<string, unknown> }[] };
  const stored = async () => ((await (await page.request.get('/api/dashboards/e2e-title-editor')).json()) as Stored).widgets[0]!.settings;
  expect(await stored()).toEqual({ title: 'Containers' });

  await panel.getByLabel('No title').check();
  await expect(title).toBeDisabled();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect.poll(stored).toEqual({ title: 'Containers', hideTitle: true });

  await page.goto('/preview/e2e-title-editor');
  await expect(page.locator('section.widget')).toHaveCount(1);
  await expect(page.locator('.tile-title')).toHaveCount(0);
  expect((await page.request.delete('/api/dashboards/e2e-title-editor')).status()).toBe(204);
});
