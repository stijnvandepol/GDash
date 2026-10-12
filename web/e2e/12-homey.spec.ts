import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openTab, test } from './fixtures';

// A stand-in for a Homey Pro's local Web API: a thermostat and a door in
// the kitchen, and four people of whom Anna and Cas are home.
const KEY = 'e2e-homey-stand-in-key';
const ANSWERS: Record<string, unknown> = {
  '/api/manager/zones/zone': { z1: { id: 'z1', name: 'Kitchen', parent: null } },
  '/api/manager/devices/device': {
    d1: {
      id: 'd1',
      name: 'Thermostat',
      zone: 'z1',
      available: true,
      capabilitiesObj: {
        measure_temperature: { id: 'measure_temperature', type: 'number', title: 'Temperature', value: 21.46, units: '°C', decimals: 1, getable: true },
      },
    },
    d2: {
      id: 'd2',
      name: 'Back door',
      zone: 'z1',
      available: true,
      capabilitiesObj: { alarm_contact: { id: 'alarm_contact', type: 'boolean', title: 'Contact alarm', value: true, getable: true } },
    },
  },
  '/api/manager/users/user': {
    'u-anna': { id: 'u-anna', name: 'Anna de Vries', present: true },
    'u-bram': { id: 'u-bram', name: 'Bram', present: false },
    'u-cas': { id: 'u-cas', name: 'Cas', present: true },
    'u-dewi': { id: 'u-dewi', name: 'Dewi', present: null },
  },
};

let homey: Server;
let homeyUrl = '';

test.beforeAll(async () => {
  homey = createServer((req, res) => {
    const answer = ANSWERS[req.url ?? ''];
    if (req.headers.authorization !== `Bearer ${KEY}`) {
      res.writeHead(401).end();
    } else if (answer === undefined) {
      res.writeHead(404).end();
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(answer));
    }
  });
  await new Promise<void>((resolve) => homey.listen(0, '127.0.0.1', resolve));
  homeyUrl = `http://127.0.0.1:${(homey.address() as AddressInfo).port}`;
});

test.afterAll(async () => {
  // GDash keeps its connection to Homey open between fetches.
  homey.closeAllConnections();
  await new Promise((resolve) => homey.close(resolve));
});

test('Homey is connected under Settings, and a wrong key is named', async ({ adminPage: page }) => {
  await openTab(page, 'Settings');
  const section = page.locator('section.card', { has: page.getByRole('heading', { name: 'Homey', exact: true }) });
  await section.getByLabel('Address').fill(homeyUrl);
  await section.getByLabel('API key').fill('not-the-key');
  await section.getByRole('button', { name: 'Connect' }).click();
  await expect(section.getByText('Homey did not accept this API key')).toBeVisible();

  await section.getByLabel('API key').fill(KEY);
  await section.getByRole('button', { name: 'Connect' }).click();
  await expect(section.getByText('Connected')).toBeVisible();
  await expect(section.getByText(homeyUrl)).toBeVisible();
  const settings = await page.request.get('/api/settings');
  expect(await settings.text()).not.toContain(KEY);
});

test('a screen shows Homey values and only the people who are home', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'nl' } })).ok()).toBe(true);
  const users = [
    { id: 'u-anna', label: 'AV' },
    { id: 'u-bram', label: 'BR' },
    { id: 'u-cas', label: 'CS' },
    { id: 'u-dewi', label: 'DW' },
  ];
  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-homey',
      name: 'E2E Homey',
      widgets: [
        { id: 'values', type: 'homey', x: 0, y: 0, w: 8, h: 5, settings: { capabilities: ['d1/measure_temperature', 'd2/alarm_contact'] } },
        { id: 'home', type: 'homeypresence', x: 8, y: 0, w: 8, h: 5, settings: { users } },
      ],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-homey');
  const screen = page.locator('main.screen');
  await expect(screen.locator('.badge')).toHaveText(['AV', 'CS'], { timeout: 15_000 });
  await expect(screen).toContainText('Thermostat · Temperature');
  await expect(screen).toContainText('21,5°C');
  await expect(screen).toContainText('Back door · Contact alarm');
  await expect(screen).toContainText('Open');

  // Who is away, or unknown, reaches the screen in no form: not shown,
  // not in the data and not in the layout.
  await expect(screen).not.toContainText('BR');
  const snapshot = await (await page.request.get('/api/dashboards/e2e-homey/preview')).text();
  for (const away of ['BR', 'DW', 'u-bram', 'u-dewi']) expect(snapshot).not.toContain(away);
});

test('the editor finds Homey values and people', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByRole('listitem').filter({ hasText: 'E2E Homey' }).getByRole('button', { name: 'Edit', exact: true }).click();

  await page.locator('.grid-stack-item[gs-id="values"]').click();
  const chosen = page.getByRole('listitem').filter({ hasText: 'Back door · Contact alarm' });
  await expect(page.getByRole('listitem').filter({ hasText: 'Thermostat · Temperature' })).toBeVisible();
  await chosen.getByRole('button', { name: 'Remove', exact: true }).click();
  await expect(chosen).toHaveCount(0);
  // The zone counts as part of the name.
  await page.getByLabel('Search Homey devices').fill('kitchen door');
  await expect(page.getByText('Kitchen · true')).toBeVisible();
  await page.getByRole('button', { name: 'Back door · Contact alarm' }).click();
  await expect(chosen).toBeVisible();

  await page.locator('.grid-stack-item[gs-id="home"]').click();
  await expect(page.getByLabel('Label for Anna de Vries')).toHaveValue('AV');
  await page.getByRole('listitem').filter({ hasText: 'Anna de Vries' }).getByRole('button', { name: 'Remove', exact: true }).click();
  await expect(page.getByLabel('Label for Anna de Vries')).toHaveCount(0);
  await page.getByRole('button', { name: 'Add Anna de Vries' }).click();
  await expect(page.getByLabel('Label for Anna de Vries')).toHaveValue('AV');
});
