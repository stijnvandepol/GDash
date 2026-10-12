import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openTab, test } from './fixtures';

// A stand-in for a household's own API that says who is home, in the
// shape of a Home Assistant template sensor.
const ANSWER = {
  home: {
    aanwezigheid_anna: true,
    aanwezigheid_bram: false,
    aanwezigheid_cas: 'home',
    aanwezigheid_dewi: 'not_home',
    temperatuur_binnen: 21.2,
  },
};

let api: Server;
let apiUrl = '';

test.beforeAll(async () => {
  api = createServer((_req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(ANSWER));
  });
  await new Promise<void>((resolve) => api.listen(0, '127.0.0.1', resolve));
  apiUrl = `http://127.0.0.1:${(api.address() as AddressInfo).port}/presence`;
});

test.afterAll(async () => {
  await new Promise((resolve) => api.close(resolve));
});

test('a Custom widget in badges mode shows only the people who are home', async ({ adminPage: page }) => {
  const created = await page.request.post('/api/custom-sources', {
    data: { name: 'Presence', url: apiUrl, method: 'GET', body: '', auth: { kind: 'none' }, headers: [], intervalSeconds: 30, sample: null },
  });
  expect(created.status()).toBe(201);
  const { id } = (await created.json()) as { id: number };

  const pairs = ['anna', 'bram', 'cas', 'dewi'].map((name) => ({ label: name.slice(0, 2).toUpperCase(), path: `home.aanwezigheid_${name}` }));
  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-badges',
      name: 'E2E Badges',
      widgets: [{ id: 'home', type: 'custom', x: 0, y: 0, w: 8, h: 5, source: 'custom.default', settings: { source: id, mode: 'badges', title: 'Home', pairs } }],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-badges');
  const badges = page.locator('.badge');
  await expect(badges).toHaveText(['AN', 'CA'], { timeout: 15_000 });
  // Who is away never reaches the screen, not even hidden.
  await expect(page.locator('main.screen')).not.toContainText('BR');
  await expect(page.locator('main.screen')).not.toContainText('DE');
});

test('the editor offers badges with a hint on what turns one on', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByRole('listitem').filter({ hasText: 'E2E Badges' }).getByRole('button', { name: 'Edit', exact: true }).click();
  await page.locator('.grid-stack-item[gs-id="home"]').click();
  const show = page.getByLabel('Show');
  await expect(show).toHaveValue('badges');
  await expect(page.getByText('A badge shows its label, such as initials, while its value is on')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add a badge' })).toBeVisible();
});
