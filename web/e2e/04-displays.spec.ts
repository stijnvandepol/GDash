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
  await expect(page.getByLabel('Display page address')).toHaveValue('e2e-kitchen');
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
