import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { Page } from '@playwright/test';
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

interface ListedSource {
  id: number;
  name: string;
  url: string;
  preset: string;
  values: { name: string; path: string; unit?: string }[];
}

async function listSources(page: Page): Promise<ListedSource[]> {
  return ((await (await page.request.get('/api/custom-sources')).json()) as { sources: ListedSource[] }).sources;
}

test('a HomeWizard P1 meter is set up by its IP address, tested first, and is not a custom API of your own', async ({ adminPage: page }) => {
  const card = page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: 'HomeWizard P1 meter', exact: true, level: 3 }) });
  const setup = await openIntegration(page, 'HomeWizard P1 meter');
  await expect(card).not.toContainText('Custom API');
  // Only what is the household's own; how GDash asks it is folded away.
  await expect(setup.getByText('Advanced', { exact: true })).toBeVisible();
  await expect(setup.getByRole('textbox', { name: 'Name', exact: true })).toBeHidden();

  const address = setup.getByLabel('IP address of the P1 meter');
  await address.fill(`http://${meterHost}`);
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByText('Enter an IP address or host name')).toBeVisible();
  await expect(address).toHaveAttribute('aria-invalid', 'true');

  // Test asks the meter without saving it, and shows what each value says now.
  await address.fill(meterHost);
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByText('HomeWizard P1 meter answered.')).toBeVisible({ timeout: 15_000 });
  await expect(setup.getByRole('listitem').filter({ hasText: 'Power now' })).toContainText('1234 W');
  await expect(setup.getByRole('listitem').filter({ hasText: 'Gas' })).toContainText('2569.646 m³');
  expect((await listSources(page)).filter((s) => s.preset === 'homewizard-p1')).toEqual([]);

  await setup.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('P1 meter is set up.')).toBeVisible();
  await expect(card).toContainText('Connected');
  await expect(card).toContainText('P1 meter.');
  await expect(setup.getByRole('list', { name: 'HomeWizard P1 meter set up' })).toContainText('P1 meter');
  await expect(setup.getByRole('button', { name: 'Add another meter' })).toBeVisible();

  const made = (await listSources(page)).find((s) => s.preset === 'homewizard-p1');
  expect(made?.name).toBe('P1 meter');
  expect(made?.url).toBe(`http://${meterHost}/api/v1/data`);
  expect(made?.values.map((v) => [v.name, v.path, v.unit])).toEqual([
    ['Power now', 'active_power_w', 'W'],
    ['Electricity in', 'total_power_import_kwh', 'kWh'],
    ['Electricity out', 'total_power_export_kwh', 'kWh'],
    ['Gas', 'total_gas_m3', 'm³'],
  ]);

  // Custom APIs are the household's own: the meter is not among them.
  const custom = await openIntegration(page, 'Custom APIs');
  await expect(custom.getByText('Show data from your own API')).toBeVisible();
  await expect(custom.getByRole('listitem').filter({ hasText: 'P1 meter' })).toHaveCount(0);
});

test('the P1 meter is a widget of its own in the editor, with its values by name, and shows the power on a screen', async ({ adminPage: page }) => {
  const meter = (await listSources(page)).find((s) => s.preset === 'homewizard-p1')!;
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Meter');
  await page.getByRole('button', { name: 'Create and edit' }).click();

  const palette = page.getByRole('complementary', { name: 'Add widgets' });
  await palette.getByRole('region', { name: 'Home', exact: true }).getByRole('button', { name: 'HomeWizard P1 meter' }).click();
  const settings = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(settings.getByRole('heading', { name: 'Selected HomeWizard P1 meter' })).toBeVisible();
  await expect(page.locator('.grid-stack-item[gs-id="homewizard-p1"]')).toHaveText('HomeWizard P1 meter');
  // No API to choose and no paths: the preset's views and values by name.
  await expect(settings.getByRole('combobox', { name: 'API' })).toHaveCount(0);
  const show = settings.getByRole('combobox', { name: 'Show', exact: true });
  await expect(show.locator('option:checked')).toHaveText('Power now');
  await expect(settings.getByRole('combobox', { name: 'Value', exact: true }).locator('option:checked')).toHaveText('Power now');
  await show.selectOption({ label: 'Meter readings' });
  await expect(settings.getByRole('checkbox', { name: 'Gas' })).toBeChecked();
  await expect(settings.getByRole('checkbox', { name: 'Power now' })).not.toBeChecked();
  await settings.getByRole('checkbox', { name: 'Gas' }).uncheck();
  await expect(settings.getByText('total_gas_m3')).toHaveCount(0);
  await show.selectOption({ label: 'Power now' });

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();
  const stored = (await (await page.request.get('/api/dashboards/e2e-meter')).json()) as { widgets: { type: string; settings: Record<string, unknown> }[] };
  // Stored as a Custom widget, so exports and the server do not change.
  expect(stored.widgets[0]).toMatchObject({ type: 'custom', settings: { source: meter.id, mode: 'value', path: 'active_power_w', unit: 'W', title: 'Power now' } });

  await page.getByRole('button', { name: 'Preview', exact: true }).click();
  await expect(page.frameLocator('iframe').locator('main.screen .big')).toHaveText(/^1[,.]?234\s*W$/, { timeout: 20_000 });
});
