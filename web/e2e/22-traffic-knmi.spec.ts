import { mkdir } from 'node:fs/promises';
import { KNMI_KEY } from './env';
import { expect, openIntegration, test } from './fixtures';

// The ANWB, Overpass and KNMI answer from e2e/upstream.ts,
// with the Go tests' recorded answers: jams on the A2 near Born, two fixed
// cameras on the A2 and the KNMI station at Ell.

test('under Integrations the KNMI key is tried, and ANWB traffic info needs the acknowledgement', async ({ adminPage: page }) => {
  const knmi = await openIntegration(page, 'KNMI measured weather');
  await knmi.getByRole('textbox', { name: 'EDR API key' }).fill('wrong-made-up-key');
  await knmi.getByRole('button', { name: 'Save' }).click();
  await expect(knmi.getByText('the KNMI did not accept this key')).toBeVisible();
  await knmi.getByRole('textbox', { name: 'EDR API key' }).fill(KNMI_KEY);
  await knmi.getByRole('button', { name: 'Save' }).click();
  await expect(knmi.getByText('Key set')).toBeVisible();
  await mkdir('test-results/screens', { recursive: true });
  await knmi.screenshot({ path: 'test-results/screens/integration-knmi.png' });
  for (const path of ['/api/knmi', '/api/settings', '/api/audit-events']) {
    expect(await (await page.request.get(path)).text()).not.toContain(KNMI_KEY);
  }

  const anwb = await openIntegration(page, 'ANWB traffic info (unofficial)');
  const turnOn = anwb.getByRole('button', { name: 'Turn on' });
  await expect(turnOn).toBeDisabled();
  expect((await page.request.put('/api/traffic', { data: { enabled: true } })).status()).toBe(422);
  await anwb.getByRole('checkbox', { name: /own household only/ }).check();
  await turnOn.click();
  await expect(anwb.getByText('On', { exact: true })).toBeVisible();
  await expect(anwb.getByRole('button', { name: 'Turn off' })).toBeVisible();
  await anwb.screenshot({ path: 'test-results/screens/integration-traffic.png' });
  await page.locator('section.group[aria-labelledby="group-travel"]').screenshot({ path: 'test-results/screens/integrations-travel.png' });
});

test('a screen shows traffic on the roads, fixed cameras and measured weather', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
  const born = { name: 'Born', latitude: 51.03, longitude: 5.81 };
  expect((await page.request.put('/api/settings/location', { data: born })).ok()).toBe(true);
  const widgets = [
    { id: 'weather', type: 'weather', x: 0, y: 0, w: 7, h: 7, settings: { current: 'knmi' } },
    {
      id: 'roads',
      type: 'traffic',
      x: 7,
      y: 0,
      w: 10,
      h: 14,
      settings: { roads: ['A2', 'A50'], show: ['jams', 'speedchecks', 'cameras', 'totals'], maxItems: 6 },
    },
    { id: 'near', type: 'traffic', x: 17, y: 0, w: 7, h: 14, settings: { radius: 10, show: ['jams', 'roadworks'] } },
  ];
  const dashboard = await page.request.post('/api/dashboards', {
    data: { id: 'e2e-traffic', name: 'E2E Traffic', theme: 'minimal-dark', widgets },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/preview/e2e-traffic');
  const screen = page.locator('main.screen');
  const roads = screen.locator('.traffic').first();
  await expect(roads).toContainText('Born → Urmond', { timeout: 15_000 });
  await expect(roads).toContainText('5 km · +11 min');
  await expect(roads).toContainText('Netherlands: 15 jams, 41 km');
  await expect(roads).toContainText('hm 236.1');
  await expect(roads).toContainText('Fixed cameras on your roads: 2');
  await expect(roads).toContainText('Unofficial · ANWB · © OpenStreetMap contributors');
  // The ANWB's own Dutch description stays as sent.
  await expect(roads).toContainText('Stilstaand verkeer. Kapotte auto.');

  // The tile's title names what it watches.
  const nearTile = screen.locator('section.widget').filter({ has: page.locator('.traffic') }).nth(1);
  await expect(nearTile.locator('.tile-title')).toHaveText('Traffic · within 10 km');
  const near = nearTile.locator('.traffic');
  await expect(near).toContainText('Born → Urmond');
  await expect(near).not.toContainText('Batadorp');

  const weather = screen.locator('.weather');
  await expect(weather).toContainText('13°', { timeout: 15_000 });
  await expect(weather).toContainText('Humidity 86%');
  // Where the number comes from stays on screen, also on a small tile.
  await expect(weather.locator('.measured')).toBeVisible();
  await expect(weather.locator('.measured')).toHaveText(/^Measured at Ell \d\d:\d0 · KNMI$/);

  // Screenshots in a dark and a light theme, for a look by eye.
  await mkdir('test-results/screens', { recursive: true });
  for (const theme of ['minimal-dark', 'minimal-light']) {
    const saved = await page.request.put('/api/dashboards/e2e-traffic', { data: { name: 'E2E Traffic', theme, widgets } });
    expect(saved.ok(), await saved.text()).toBe(true);
    await page.goto('/preview/e2e-traffic');
    await expect(screen).toHaveAttribute('data-theme', theme);
    await expect(screen.locator('.traffic').first()).toContainText('Born → Urmond', { timeout: 15_000 });
    await page.screenshot({ path: `test-results/screens/traffic-${theme}.png` });
  }
});
