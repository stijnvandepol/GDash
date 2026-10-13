import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, openIntegration, openTab, test } from './fixtures';

// A stand-in for a HomeWizard P1 meter's local API v1, in the shape of its
// documentation.
const P1_ANSWER = {
  smr_version: 50,
  meter_model: 'ISKRA 2M550T-101',
  total_power_import_kwh: 13779.338,
  total_power_export_kwh: 12.5,
  active_power_w: 1234,
  total_gas_m3: 2569.646,
};

let meter: Server;
let meterHost = '';

test.beforeAll(async () => {
  meter = createServer((req, res) => {
    if (req.url !== '/api/v1/data') {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(P1_ANSWER));
  });
  await new Promise<void>((resolve) => meter.listen(0, '127.0.0.1', resolve));
  meterHost = `127.0.0.1:${(meter.address() as AddressInfo).port}`;
});

test.afterAll(async () => {
  meter.closeAllConnections();
  await new Promise((resolve) => meter.close(resolve));
});

test('the search and the filters find integrations, and a reload keeps the search', async ({ adminPage: page }) => {
  await openTab(page, 'Integrations');
  const search = page.getByRole('searchbox', { name: 'Search integrations' });
  await search.fill('homewizard');
  await expect(page.getByRole('button', { name: 'Set up HomeWizard P1 meter' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Home Assistant', exact: true })).toHaveCount(0);
  await expect(page).toHaveURL(/#integrations\?q=homewizard$/);

  await page.reload();
  await expect(search).toHaveValue('homewizard');
  await expect(page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Integrations' })).toHaveAttribute('aria-current', 'page');

  // A category name finds what is in it.
  await search.fill('travel');
  await expect(page.getByRole('heading', { name: 'Departures', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Weather', exact: true })).toHaveCount(0);

  await search.fill('');
  const filters = page.getByRole('group', { name: 'Show' });
  await filters.getByRole('button', { name: 'Netherlands' }).click();
  await expect(filters.getByRole('button', { name: 'Netherlands' })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByRole('heading', { name: 'Fuel prices', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Home Assistant', exact: true })).toHaveCount(0);
  await filters.getByRole('button', { name: 'No key' }).click();
  await expect(page.getByRole('heading', { name: 'Todoist', exact: true })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Calendars', exact: true })).toBeVisible();
  await filters.getByRole('button', { name: 'All' }).click();

  // Nothing found: a custom API is the way out.
  await search.fill('a service nobody has heard of');
  await expect(page.getByRole('heading', { name: 'Nothing matches “a service nobody has heard of”' })).toBeVisible();
  await page.getByRole('button', { name: 'Use a Custom API' }).click();
  await expect(page.getByRole('region', { name: 'Set up Custom APIs' })).toBeVisible();
  await expect(search).toHaveValue('');
});

test('a setup opens under its card with focus, and closes back to it', async ({ adminPage: page }) => {
  await openTab(page, 'Integrations');
  const open = page.getByRole('button', { name: 'Set up Home Assistant' });
  await open.click();
  const setup = page.getByRole('region', { name: 'Set up Home Assistant' });
  await expect(setup).toBeFocused();
  await expect(open).toHaveAttribute('aria-expanded', 'true');
  await expect(setup.getByRole('heading', { name: 'Home Assistant', exact: true })).toBeVisible();
  await expect(setup.getByRole('textbox', { name: 'Address' })).toBeVisible();

  await setup.getByRole('button', { name: 'Close Home Assistant' }).click();
  await expect(setup).toHaveCount(0);
  await expect(open).toBeFocused();
  await expect(open).toHaveAttribute('aria-expanded', 'false');
});

test('a custom API made from the HomeWizard P1 preset has its named values', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'HomeWizard P1 meter');
  const address = setup.getByLabel('IP address of the P1 meter');
  await address.fill(`http://${meterHost}`);
  await setup.getByRole('button', { name: 'Add HomeWizard P1 meter' }).click();
  await expect(setup.getByText('Enter an IP address or host name')).toBeVisible();
  await expect(address).toHaveAttribute('aria-invalid', 'true');

  await address.fill(meterHost);
  await setup.getByRole('button', { name: 'Add HomeWizard P1 meter' }).click();
  await expect(setup).toContainText('P1 meter is added as a custom API with 4 values.');
  // GDash asks the meter at once and shows what each value says now.
  await expect(setup.getByText('HomeWizard P1 meter answered.')).toBeVisible({ timeout: 15_000 });
  await expect(setup.getByRole('listitem').filter({ hasText: 'Power now' })).toContainText('1234 W');
  await expect(setup.getByRole('listitem').filter({ hasText: 'Gas' })).toContainText('2569.646 m³');

  const sources = (await (await page.request.get('/api/custom-sources')).json()) as {
    sources: { name: string; url: string; values: { name: string; path: string; unit?: string }[] }[];
  };
  const made = sources.sources.find((s) => s.name === 'P1 meter');
  expect(made?.url).toBe(`http://${meterHost}/api/v1/data`);
  expect(made?.values.map((v) => [v.name, v.path, v.unit])).toEqual([
    ['Power now', 'active_power_w', 'W'],
    ['Electricity in', 'total_power_import_kwh', 'kWh'],
    ['Electricity out', 'total_power_export_kwh', 'kWh'],
    ['Gas', 'total_gas_m3', 'm³'],
  ]);

  // The card knows it is made, and Custom APIs lists it with its values.
  await expect(page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: 'HomeWizard P1 meter', exact: true, level: 3 }) })).toContainText('As P1 meter.');
  await setup.getByRole('button', { name: 'Open Custom APIs' }).click();
  const custom = page.getByRole('region', { name: 'Set up Custom APIs' });
  await expect(custom.getByRole('listitem').filter({ hasText: 'P1 meter' }).first()).toContainText('4 values');
});
