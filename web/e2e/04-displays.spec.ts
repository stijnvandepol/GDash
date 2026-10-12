import { expect, openTab, test } from './fixtures';

const NOTE_TEXT = 'Hello from a paired screen';
// Four letters, a dash, four letters or digits: what the screen shows.
const PAIRING_CODE = /^[A-Z0-9]{4}-[A-Z0-9]{4}$/;

interface WidgetTypes {
  types: Record<string, { minW: number; minH: number }>;
}

test('a display without a Cast device is connected from the web interface and shows its dashboard', async ({
  adminPage: page,
  browser,
  baseURL,
}) => {
  // A dashboard with one note, made through the API: the editor has its own journey.
  const types = (await (await page.request.get('/api/widget-types')).json()) as WidgetTypes;
  const note = types.types.text!;
  const created = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-board',
      name: 'E2E board',
      widgets: [{ id: 'note', type: 'text', x: 0, y: 0, w: note.minW, h: note.minH, settings: { text: NOTE_TEXT } }],
    },
  });
  expect(created.status()).toBe(201);
  // The page opened before the dashboard existed; it lists dashboards on load.
  await page.reload();

  await openTab(page, 'Displays');
  const form = page.getByRole('form', { name: 'Add a display' });
  await form.getByLabel('Name', { exact: true }).fill('E2E Kitchen');
  // The address follows from the name and is only shown, not asked for.
  await expect(form.getByText('/d/e2e-kitchen')).toBeVisible();
  // The newest dashboard is offered, since it is usually the one made for the new display.
  await expect(form.getByLabel('Shows')).toHaveValue('e2e-board');
  await form.getByRole('button', { name: 'Add display' }).click();

  const row = page.getByRole('listitem').filter({ hasText: 'E2E Kitchen' });
  await expect(row.getByLabel('Dashboard for E2E Kitchen')).toHaveValue('e2e-board');
  // Without a screen, the row says so and asks for the code right there.
  await expect(row).toContainText('No screen yet');
  await expect(row).toContainText(`${baseURL}/d/e2e-kitchen`);
  await expect(row).not.toContainText('paired screens');

  // A second browser has no session: it is a TV that was never connected.
  const screen = await browser.newContext({ baseURL });
  try {
    const tv = await screen.newPage();
    await tv.goto('/d/e2e-kitchen');
    const code = tv.getByText(PAIRING_CODE);
    await expect(code).toBeVisible();
    await expect(tv.getByText('Connect this screen')).toBeVisible();
    await expect(tv.getByText('/d/e2e-kitchen')).toBeVisible();
    await expect(tv.getByText(NOTE_TEXT)).toHaveCount(0);

    await row.getByLabel('Code shown on the screen').fill((await code.textContent()) ?? '');
    await row.getByRole('button', { name: 'Connect', exact: true }).click();
    await expect(row).toContainText('Shown on 1 screen');
    await expect(row.getByLabel('Code shown on the screen')).toHaveCount(0);

    // The screen polls every few seconds, then streams the dashboard.
    await expect(tv.getByText(NOTE_TEXT)).toBeVisible({ timeout: 20_000 });
    await expect(code).toHaveCount(0);
  } finally {
    await screen.close();
  }

  // Another screen is a step away, with Cancel to back out.
  await row.getByRole('button', { name: 'Add another screen…' }).click();
  await expect(row.getByLabel('Code shown on the screen')).toBeVisible();
  await row.getByRole('button', { name: 'Cancel' }).click();
  await expect(row.getByLabel('Code shown on the screen')).toHaveCount(0);
});

/** A display on a Cast TV as the list shows it, with what its keeper found. */
const CAST_DISPLAY = {
  id: 'e2e-cast',
  name: 'E2E Cast',
  castDeviceId: 'e2e-device',
  createdAt: '2026-10-12T10:00:00Z',
  dashboardId: '',
  activeFrom: '',
  activeUntil: '',
  schedule: [],
  rotation: [],
  rotateSeconds: 0,
  textScale: 100,
  pairedScreens: 2,
  cast: { state: 'showing', since: '2026-10-12T10:00:00Z' },
};

test('a display on a Cast TV leads with what the TV is doing', async ({ adminPage: page }) => {
  // The suite never talks to a Cast device, not even by discovery: a
  // display on one exists only in the answers to the web interface.
  await page.route('**/api/cast-devices', (route) =>
    route.fulfill({
      json: { devices: [{ id: 'e2e-device', name: 'Tv Keuken', model: 'Chromecast', addr: '192.0.2.1:8009', hasScreen: true }] },
    }),
  );
  await page.route('**/api/displays', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const response = await route.fetch();
    const body = (await response.json()) as { displays: unknown[] };
    await route.fulfill({ response, json: { displays: [...body.displays, CAST_DISPLAY] } });
  });
  // The page loaded its lists before these answers were in place.
  await page.reload();

  await openTab(page, 'Displays');
  const row = page.getByRole('listitem').filter({ hasText: 'E2E Cast' });
  await expect(row).toContainText(/On Tv Keuken since \d\d:\d\d/);
  // The TV pairs itself; only the screens besides it are counted.
  await expect(row).toContainText('Also open on 2 other screens');
  await expect(row).not.toContainText('No screen yet');
  await expect(row.getByLabel('Code shown on the screen')).toHaveCount(0);

  await row.getByRole('button', { name: 'Open on another screen…' }).click();
  await expect(row).toContainText('GDash keeps this display on Tv Keuken by itself');
  await expect(row.getByLabel('Code shown on the screen')).toBeVisible();
});

test('a TV that someone uses can be given back to the dashboard', async ({ adminPage: page }) => {
  // The list is answered here: a display on a real Cast device would make
  // the server search the network and cast to it.
  let state = 'yielded';
  const tv = {
    id: 'e2e-living-room',
    name: 'E2E Living room',
    castDeviceId: '0123456789abcdef0123456789abcdef',
    createdAt: '2026-10-12T08:00:00Z',
    pairedScreens: 0,
    dashboardId: '',
    activeFrom: '',
    activeUntil: '',
    schedule: [],
    rotation: [],
    rotateSeconds: 0,
    textScale: 100,
  };
  await page.route('**/api/displays', (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ json: { displays: [{ ...tv, cast: { state, since: '2026-10-12T08:30:00Z' } }] } })
      : route.fallback(),
  );
  const takeovers: string[] = [];
  await page.route('**/api/displays/*/cast-takeovers', (route) => {
    takeovers.push(new URL(route.request().url()).pathname);
    state = 'showing';
    return route.fulfill({ status: 204 });
  });

  // The web interface opened on the real list; load it again through the routes.
  await page.reload();
  await openTab(page, 'Displays');
  const row = page.getByRole('listitem').filter({ hasText: 'E2E Living room' });
  await expect(row).toContainText('Someone is using the TV; GDash waits until it is switched off');
  await row.getByRole('button', { name: 'Show dashboard now on E2E Living room' }).click();
  const dialog = page.getByRole('dialog', { name: 'Show the dashboard now?' });
  await dialog.getByRole('button', { name: 'Show dashboard' }).click();

  await expect(row).toContainText('On the TV since');
  await expect(row.getByRole('button', { name: /Show dashboard now/ })).toHaveCount(0);
  expect(takeovers).toEqual(['/api/displays/e2e-living-room/cast-takeovers']);
});
