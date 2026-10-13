import { expect, openTab, test } from './fixtures';

// Rain and weather warnings come from the stand-in in e2e/upstream.ts: a
// moderate shower starting 20 minutes from now in Oss, an orange wind
// warning in effect and a yellow rain warning tonight in Noord-Brabant, and
// a red thunderstorm warning in Zeeland.

const OSS = { name: 'Oss', latitude: 51.765, longitude: 5.518 };

test('a preview shows the rain to come and the weather warnings, large and small', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
  const created = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-weather-alerts',
      name: 'E2E Rain and warnings',
      widgets: [
        { id: 'rain-large', type: 'rain', x: 0, y: 0, w: 12, h: 7, settings: { place: OSS } },
        { id: 'warnings-large', type: 'warnings', x: 12, y: 0, w: 12, h: 7, settings: { region: 'Noord-Brabant' } },
        { id: 'rain-small', type: 'rain', x: 0, y: 7, w: 5, h: 3, settings: { place: OSS } },
        { id: 'warnings-small', type: 'warnings', x: 5, y: 7, w: 6, h: 3, settings: { region: 'Noord-Brabant' } },
        { id: 'warnings-quiet', type: 'warnings', x: 11, y: 7, w: 6, h: 3, settings: { region: 'Utrecht' } },
        { id: 'warnings-red', type: 'warnings', x: 17, y: 7, w: 7, h: 4, settings: { region: 'Zeeland' } },
      ],
    },
  });
  expect(created.status(), await created.text()).toBe(201);

  await page.goto('/preview/e2e-weather-alerts');
  const screen = page.locator('main.screen');
  const sentences = screen.locator('.rain .sentence');
  await expect(sentences).toHaveCount(2, { timeout: 15_000 });
  for (const sentence of await sentences.all()) await expect(sentence).toHaveText(/^Rain from \d\d:\d\d \(moderate\)$/);
  // Buienradar's terms ask for the credit on every tile with its data.
  await expect(screen.locator('.rain .source')).toHaveText(['Buienradar.nl', 'Buienradar.nl']);
  // The large tile has a bar per five minutes for two hours.
  await expect(screen.locator('.rain').first().locator('.bar')).toHaveCount(24);

  const banners = screen.locator('.warnings').first().locator('li.banner');
  await expect(banners).toHaveCount(2);
  await expect(banners.first()).toHaveClass(/orange/);
  await expect(banners.first()).toContainText('Code orange');
  await expect(banners.first()).toContainText('Wind');
  await expect(banners.first()).toContainText('until');
  await expect(banners.nth(1)).toContainText('Code yellow');
  await expect(banners.nth(1)).toContainText('Heavy rain');
  await expect(screen.locator('.warnings').nth(2)).toContainText('No warnings for Utrecht');
  // Red has a sign as well as its colour.
  const red = screen.locator('.warnings').nth(3).locator('li.banner');
  await expect(red).toContainText('Code red');
  await expect(red).toContainText('Thunderstorms');
  await expect(red.locator('svg.mark')).toBeVisible();

  // The household's language: Dutch on the screen.
  expect((await page.request.put('/api/settings/language', { data: { language: 'nl' } })).ok()).toBe(true);
  await page.reload();
  await expect(sentences.first()).toHaveText(/^Regen vanaf \d\d:\d\d \(matig\)$/, { timeout: 15_000 });
  await expect(banners.first()).toContainText('Code oranje');
  await expect(banners.first()).toContainText('Zware windstoten');
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
});

test('the editor offers the province and announcing for weather warnings', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByRole('listitem').filter({ hasText: 'E2E Rain and warnings' }).getByRole('button', { name: 'Edit', exact: true }).click();
  await page.locator('.grid-stack-item[gs-id="warnings-quiet"]').click();
  const panel = page.getByRole('complementary', { name: 'Widget settings' });
  const province = panel.getByRole('combobox', { name: 'Province' });
  await expect(province).toHaveValue('Utrecht');
  await province.selectOption({ label: 'From the weather location' });
  await panel.getByRole('checkbox', { name: 'Announce code orange and red on every screen' }).check();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  const stored = (await (await page.request.get('/api/dashboards/e2e-weather-alerts')).json()) as { widgets: { id: string; settings?: unknown }[] };
  expect(stored.widgets.find((w) => w.id === 'warnings-quiet')?.settings).toEqual({ announce: true });
});
