import { expect, openTab, test } from './fixtures';

// P2000 alerts come from the stand-in in e2e/upstream.ts: in Brabant-
// Zuidoost a house fire in Nuenen 10 minutes ago, a long fire alert in
// Eindhoven an hour ago, a gas smell in Eindhoven 2 hours ago, and police
// in Nuenen 30 minutes ago.

test('a preview shows the P2000 alerts of a region and of a place', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
  const created = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-p2000',
      name: 'E2E P2000',
      widgets: [
        { id: 'region', type: 'p2000', x: 0, y: 0, w: 12, h: 8, settings: { region: 'brabant-zuidoost', disciplines: ['brandweer', 'politie'], hours: 12 } },
        {
          id: 'place',
          type: 'p2000',
          x: 12,
          y: 0,
          w: 12,
          h: 8,
          settings: { places: [{ name: 'Nuenen', slug: 'nuenen', region: 'brabant-zuidoost' }], disciplines: ['brandweer'], hours: 1 },
        },
        // The smallest: two rows high, a strip with the latest alerts.
        { id: 'strip', type: 'p2000', x: 0, y: 8, w: 8, h: 2, settings: { region: 'brabant-zuidoost', disciplines: ['brandweer', 'politie'] } },
      ],
    },
  });
  expect(created.status(), await created.text()).toBe(201);

  await page.goto('/preview/e2e-p2000');
  const screen = page.locator('main.screen');
  const region = screen.locator('.p2000').first();
  await expect(region.locator('li .text')).toHaveText(
    [
      'Woningbrand op Dorpsstraat in Nuenen',
      'Politie naar Berg in Nuenen',
      'Brand in een bedrijfspand met uitslaande vlammen op de Vestdijk ter hoogte van het station in Eindhoven',
      'Gaslucht op Kerkstraat in Eindhoven',
    ],
    { timeout: 15_000 },
  );
  // A long alert takes a second line rather than being cut after a few words.
  const lines = await region.locator('li .text').nth(2).evaluate((el) => (el as HTMLElement).offsetHeight / parseFloat(getComputedStyle(el).lineHeight));
  expect(Math.round(lines)).toBe(2);
  await expect(region.locator('li .ago').first()).toHaveText('10 min');
  await expect(region.locator('li .ago').last()).toHaveText('2 h');
  // Alarmeringen.nl's licence asks for the credit.
  await expect(region.locator('.source')).toHaveText('Alarmeringen.nl');
  await expect(screen.getByRole('heading', { name: 'P2000 · Brabant-Zuidoost' })).toBeVisible();

  // A strip two rows high shows the newest alert whole, inside its tile.
  const strip = screen.locator('.p2000').nth(2);
  await expect(strip.locator('li:not([hidden]) .text').first()).toHaveText('Woningbrand op Dorpsstraat in Nuenen');
  const tile = await strip.evaluate((el) => el.closest('.widget')!.getBoundingClientRect());
  for (const li of await strip.locator('li:not([hidden])').all()) {
    const box = (await li.boundingBox())!;
    expect(box.y + box.height).toBeLessThanOrEqual(tile.bottom + 0.5);
  }
  await page.screenshot({ path: 'test-results/p2000/sizes.png' });

  // Only Nuenen's fire, within the hour.
  const place = screen.locator('.p2000').nth(1);
  await expect(place.locator('li .text')).toHaveText(['Woningbrand op Dorpsstraat in Nuenen']);
  await expect(screen.getByRole('heading', { name: 'P2000 · Nuenen' })).toBeVisible();
});

test('the editor finds a place with Test and saves it', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByRole('listitem').filter({ hasText: 'E2E P2000' }).getByRole('button', { name: 'Edit', exact: true }).click();
  await page.locator('.grid-stack-item[gs-id="region"]').click();
  const panel = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(panel.getByRole('combobox', { name: 'Safety region' })).toHaveValue('brabant-zuidoost');

  await panel.getByRole('radio', { name: 'Places' }).check();
  const name = panel.getByRole('textbox', { name: 'Place' });
  await name.fill('Nergenshuizen');
  await panel.getByRole('button', { name: 'Test and add' }).click();
  await expect(panel.getByRole('alert')).toContainText('alarmeringen.nl knows no place by this name');
  await name.fill('Nuenen');
  await panel.getByRole('button', { name: 'Test and add' }).click();
  await expect(panel.getByRole('status')).toHaveText('Nuenen found, in Brabant-Zuidoost.');
  await panel.getByRole('checkbox', { name: 'Ambulance (most alerts)' }).check();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  const stored = (await (await page.request.get('/api/dashboards/e2e-p2000')).json()) as { widgets: { id: string; settings?: unknown }[] };
  expect(stored.widgets.find((w) => w.id === 'region')?.settings).toEqual({
    places: [{ name: 'Nuenen', slug: 'nuenen', region: 'brabant-zuidoost' }],
    disciplines: ['brandweer', 'politie', 'ambulance'],
    hours: 12,
  });
});
