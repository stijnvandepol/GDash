import { expect, openTab, test } from './fixtures';

const NOTE_TEXT = 'Hello from a paired screen';
// Four letters, a dash, four letters or digits: what the screen shows.
const PAIRING_CODE = /^[A-Z0-9]{4}-[A-Z0-9]{4}$/;

interface WidgetTypes {
  types: Record<string, { minW: number; minH: number }>;
}

test('a display without a Cast device is paired from the web interface and shows its dashboard', async ({
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

  await openTab(page, 'Displays');
  await page.getByLabel('Name', { exact: true }).fill('E2E Kitchen');
  // The address follows from the name and is only shown, not asked for.
  await expect(page.getByText('/d/e2e-kitchen')).toBeVisible();
  await page.getByRole('button', { name: 'Add display' }).click();

  const row = page.getByRole('listitem').filter({ hasText: 'E2E Kitchen' });
  await expect(row).toContainText('Browser or kiosk');
  await expect(row).toContainText('0 paired screens');
  await row.getByLabel('Dashboard for E2E Kitchen').selectOption({ label: 'E2E board' });
  await expect(row.getByLabel('Dashboard for E2E Kitchen')).toHaveValue('e2e-board');

  // A second browser has no session: it is a TV that was never paired.
  const screen = await browser.newContext({ baseURL });
  try {
    const tv = await screen.newPage();
    await tv.goto('/d/e2e-kitchen');
    const code = tv.getByText(PAIRING_CODE);
    await expect(code).toBeVisible();
    await expect(tv.getByText(NOTE_TEXT)).toHaveCount(0);

    await row.getByRole('button', { name: 'Pair a screen' }).click();
    await page.getByLabel('Code shown on the screen').fill((await code.textContent()) ?? '');
    await page.getByRole('button', { name: 'Pair', exact: true }).click();
    await expect(row).toContainText('1 paired screen');

    // The screen polls every few seconds, then streams the dashboard.
    await expect(tv.getByText(NOTE_TEXT)).toBeVisible({ timeout: 20_000 });
    await expect(code).toHaveCount(0);
  } finally {
    await screen.close();
  }
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

  await expect(row).toContainText('On screen since');
  await expect(row.getByRole('button', { name: /Show dashboard now/ })).toHaveCount(0);
  expect(takeovers).toEqual(['/api/displays/e2e-living-room/cast-takeovers']);
});
