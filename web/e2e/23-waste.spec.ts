import { expect, openIntegration, openTab, test } from './fixtures';

// The waste calendar comes from the stand-in in e2e/upstream.ts: GFT
// tomorrow, PMD in three days, paper in nine, Christmas trees in twenty and
// bulky waste in a month.

const wasteLink = () => `${process.env.GDASH_E2E_UPSTREAM}/waste/afvalkalender.ics`;

test('a waste calendar is set up under Waste collection, and a Waste widget shows it without a choice', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
  const setup = await openIntegration(page, 'Waste collection');
  await expect(setup.getByRole('heading', { name: 'Waste collection' })).toBeVisible();

  // Where to find the link is one click away.
  await setup.getByText('Where to find the link').click();
  await expect(setup.getByRole('link', { name: 'mijnafvalwijzer.nl' })).toBeVisible();

  // Test reads the calendar without storing it and names what it recognised.
  await setup.getByLabel('Name').fill('Home');
  await setup.getByLabel('Address').fill(wasteLink());
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  const found = setup.getByRole('status').filter({ hasText: 'in the next 8 weeks' });
  await expect(found).toContainText('5 pickups in the next 8 weeks');
  await expect(found.locator('.kinds li')).toHaveText([
    /GFT \(organic waste\)\s*next \w{3} \d+ \w{3} · 1 in 8 weeks/,
    /PMD \(plastic & packaging\)\s*next/,
    /Paper\s*next/,
    /Christmas trees\s*next/,
  ]);
  await expect(found).toContainText('Shown by their own title: Grofvuil op afspraak.');
  expect(((await (await page.request.get('/api/settings')).json()) as { settings: { wasteCalendars: unknown[] } }).settings.wasteCalendars).toEqual([]);

  await setup.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(page.getByText('Added Home.')).toBeVisible();
  await expect(setup.getByRole('listitem').filter({ hasText: 'Home' })).toContainText('127.0.0.1');
  const card = page.locator('li.item').filter({ has: page.getByRole('heading', { name: 'Waste collection', exact: true }) });
  await expect(card).toContainText('Connected');
  await expect(card).toContainText('Home.');

  // The Calendars card lists appointments only.
  const calendars = await openIntegration(page, 'Calendars');
  await expect(calendars.getByRole('heading', { name: 'Calendars', exact: true })).toBeVisible();
  await expect(calendars.getByText('Home', { exact: true })).toHaveCount(0);

  // A Waste widget needs no choice with one waste calendar.
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Waste');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  const search = page.getByRole('complementary', { name: 'Add widgets' }).getByRole('searchbox', { name: 'Search widgets' });
  await search.fill('waste');
  await search.press('Enter');
  const panel = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(panel.getByRole('heading', { name: 'Selected Waste collection' })).toBeVisible();
  await expect(panel).toContainText('Shows Home, your waste calendar.');
  await expect(panel.getByRole('combobox', { name: 'Waste calendar' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();
  const stored = (await (await page.request.get('/api/dashboards/e2e-waste')).json()) as { widgets: { type: string; settings?: unknown }[] };
  expect(stored.widgets.find((w) => w.type === 'waste')?.settings).toBeUndefined();

  // The screen shows the next pickups of the only waste calendar.
  await page.goto('/preview/e2e-waste');
  const pickups = page.locator('main.screen .waste li');
  await expect(pickups.first()).toContainText('Organic waste', { timeout: 15_000 });
  await expect(pickups.first()).toContainText('Tomorrow');
  await expect(pickups.nth(1)).toContainText('Plastic & packaging');

  // Leave the server as the other journeys expect it.
  expect((await page.request.delete('/api/dashboards/e2e-waste')).status()).toBe(204);
  const { settings } = (await (await page.request.get('/api/settings')).json()) as { settings: { wasteCalendars: { id: number }[] } };
  for (const c of settings.wasteCalendars) expect((await page.request.delete(`/api/calendars/${c.id}`)).status()).toBe(204);
});

test('the Test of a waste calendar says what is wrong with a link', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Waste collection');
  await setup.getByLabel('Address').fill(`${process.env.GDASH_E2E_UPSTREAM}/waste/gone.ics`);
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByRole('alert')).toContainText('The calendar could not be read: calendar server answered 404 Not Found');
  await setup.getByLabel('Address').fill('mijnafvalwijzer');
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByRole('alert')).toContainText('paste the full address');
});
