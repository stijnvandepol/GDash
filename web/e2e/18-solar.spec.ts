import { expect, openIntegration, test } from './fixtures';

// The server asks the stand-in in solar-stand-in.ts, through
// e2e/upstream.ts, instead of Open-Meteo:
// 5250 W/m² hours of sun today and 40% of that tomorrow. 5 kWp at a
// performance ratio of 0.8 turns each W/m² hour into 4 Wh, so today is
// 21 kWh and tomorrow 8.4, with the best hour from 12:00.

test('the solar panels are set up under Integrations', async ({ adminPage: page }) => {
  // The same place as the settings journey, for when this file runs alone.
  const location = { name: 'E2E Home', latitude: 52.1, longitude: 5.2 };
  expect((await page.request.put('/api/settings/location', { data: location })).ok()).toBe(true);
  const section = await openIntegration(page, 'Solar panels');
  // Without a place of their own, the panels are at the weather location.
  await expect(section.getByText('The weather location from Integrations.')).toBeVisible();
  await section.getByLabel('Tilt (°)').fill('35');
  await section.getByLabel('Direction (°)').fill('-20');
  await section.getByLabel('Peak power (kWp)').fill('5');
  await section.getByLabel('Performance ratio').fill('0.8');
  await section.getByRole('button', { name: 'Save' }).click();

  await expect(section.getByText('5 kWp facing south at 35°')).toBeVisible();
  await expect(section.getByText('At the weather location · estimate from Open-Meteo, performance ratio 0.8')).toBeVisible();
  await expect(section.getByText('Add a Solar forecast widget to a dashboard to fetch the estimate.')).toBeVisible();
  await expect(page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: 'Solar panels', exact: true, level: 3 }) })).toContainText('5 kWp.');
});

test('a screen shows the expected production today, its best hour and tomorrow', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
  const dashboard = await page.request.post('/api/dashboards', {
    data: { id: 'e2e-solar', name: 'E2E Solar', widgets: [{ id: 'solar', type: 'solar', x: 0, y: 0, w: 8, h: 5 }] },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-solar');
  const screen = page.locator('main.screen');
  await expect(screen.getByText('~21', { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(screen.getByText('today', { exact: true })).toBeVisible();
  await expect(screen.getByText('Peak 12:00')).toBeVisible();
  await expect(screen.getByText('Tomorrow ~8.4 kWh')).toBeVisible();
  // It is mid-morning at the stand-in's panels: the hours still to come
  // run to the end of the last sunny hour.
  await expect(screen.getByText('Rest of today')).toBeVisible();
  await expect(screen.getByText('19:00')).toBeVisible();

  // One request for the one group of panels, facing the way they were set up.
  const asked = (await (await page.request.get(`${process.env.GDASH_E2E_UPSTREAM}/open-meteo-solar/requests`)).json()) as string[];
  expect(asked.length).toBeGreaterThan(0);
  expect(asked[0]).toContain('hourly=global_tilted_irradiance');
  expect(asked[0]).toContain('tilt=35');
  expect(asked[0]).toContain('azimuth=-20');
});

test('the Solar panels setup says when the estimate was fetched', async ({ adminPage: page }) => {
  const section = await openIntegration(page, 'Solar panels');
  await expect(section.getByText(/^Updated /)).toBeVisible({ timeout: 10_000 });
});
