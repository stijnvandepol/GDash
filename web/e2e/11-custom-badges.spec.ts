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
  // The API has no named values yet: a stored path shows as a path of its own.
  await expect(page.getByRole('combobox', { name: 'Value 1', exact: true }).locator('option:checked')).toHaveText('(custom path)');
});

test('values named in Settings are chosen by name in the editor, with a rule built from them', async ({ adminPage: page }) => {
  await openTab(page, 'Settings');
  await page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: 'Data sources' }).click();
  const row = page.getByRole('listitem').filter({ hasText: 'Presence' });
  await row.getByRole('button', { name: 'Values' }).click();
  await page.getByRole('button', { name: 'Fetch the API now' }).click();
  await page.getByRole('button', { name: /^aanwezigheid_anna/ }).click();
  await page.getByRole('button', { name: /^temperatuur_binnen/ }).click();
  await expect(page.getByLabel('Name of value 1')).toHaveValue('Aanwezigheid anna');
  await page.getByLabel('Name of value 1').fill('Anna');
  await page.getByLabel('Unit of value 2').fill('°C');
  await page.getByRole('button', { name: 'Save values' }).click();
  await expect(page.getByText('The values of Presence are saved.')).toBeVisible();
  await expect(row).toContainText('2 values');

  await openTab(page, 'Dashboards');
  await page.getByRole('listitem').filter({ hasText: 'E2E Badges' }).getByRole('button', { name: 'Edit', exact: true }).click();
  await page.locator('.grid-stack-item[gs-id="home"]').click();
  const panel = page.getByRole('complementary', { name: 'Widget settings' });
  // A stored path with a name shows the name; one without, that it is a path of its own.
  await expect(panel.getByRole('combobox', { name: 'Value 1', exact: true }).locator('option:checked')).toHaveText('Anna');
  await expect(panel.getByRole('combobox', { name: 'Value 2', exact: true }).locator('option:checked')).toHaveText('(custom path)');
  // No JSON tree and no paths in the editor.
  await expect(panel.locator('.tree')).toHaveCount(0);
  await expect(panel.getByText('home.aanwezigheid')).toHaveCount(0);

  await panel.getByRole('button', { name: 'Add a rule' }).click();
  await panel.getByRole('combobox', { name: 'When', exact: true }).selectOption({ label: 'Anna' });
  await panel.getByRole('combobox', { name: 'Test of rule 1' }).selectOption({ label: 'is on' });
  await panel.getByRole('textbox', { name: 'Message', exact: true }).fill('Anna is home');
  // The written condition stays folded away.
  await expect(panel.getByRole('textbox', { name: 'Condition of rule 1' })).toBeHidden();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  const stored = (await (await page.request.get('/api/dashboards/e2e-badges')).json()) as { widgets: { settings: { rules?: unknown[] } }[] };
  expect(stored.widgets[0]!.settings.rules).toEqual([{ when: 'isOn(home.aanwezigheid_anna)', tone: 'warn', message: 'Anna is home' }]);
});
