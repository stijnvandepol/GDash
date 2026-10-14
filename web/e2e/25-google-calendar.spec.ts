import { expect, openIntegration, test } from './fixtures';
import { GOOGLE_CALENDAR_PATH } from './upstream';

// The Google calendar comes from the stand-in in e2e/upstream.ts, at an
// address shaped like Google's secret iCal address: a weekly swimming
// lesson since last year and the dentist tomorrow.

const upstream = () => process.env.GDASH_E2E_UPSTREAM ?? '';

test('a Google calendar is linked by its secret iCal address, tested first, and shown on a screen', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Calendars');

  // The steps for Google are one click away.
  const guides = setup.locator('summary');
  await expect(guides).toHaveText(['Google Calendar', 'Outlook and Microsoft 365', 'iCloud']);
  await guides.first().click();
  await expect(setup.getByText('Secret address in iCal format', { exact: true })).toBeVisible();

  // A link that opens Google Calendar in the browser is explained before any test.
  const address = setup.getByLabel('Address');
  await address.fill('https://calendar.google.com/calendar/embed?src=family.e2e%40gmail.com&ctz=Europe%2FAmsterdam');
  await expect(setup.getByRole('status').filter({ hasText: 'embedding the calendar in a web page' })).toBeVisible();
  await address.fill('https://calendar.google.com/calendar/ical/family.e2e%40gmail.com/public/basic.ics');
  await expect(setup.getByRole('status').filter({ hasText: 'only works when the calendar is public' })).toBeVisible();

  // A web page instead of a calendar says so.
  await address.fill(`${upstream()}/calendar/embed`);
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByRole('alert')).toContainText('This address shows a web page, not an iCal calendar.');

  // The secret address: Test reads it now, without storing it.
  await setup.getByLabel('Name').fill('Gezin');
  await address.fill(upstream() + GOOGLE_CALENDAR_PATH);
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  const found = setup.getByRole('status').filter({ hasText: 'in the next 30 days' });
  await expect(found).toContainText('events in the next 30 days in “Gezin”');
  await expect(found.locator('.events li')).toHaveCount(5);
  await expect(found.locator('.events li').filter({ hasText: 'Tandarts' })).toHaveCount(1);
  await expect(found.locator('.events li').filter({ hasText: 'Zwemles' }).first()).toBeVisible();
  await expect(found).not.toContainText('Overleg 2015');
  expect(((await (await page.request.get('/api/settings')).json()) as { settings: { calendars: unknown[] } }).settings.calendars).toEqual([]);

  await setup.getByRole('button', { name: 'Add', exact: true }).click();
  await expect(page.getByText('Added Gezin.')).toBeVisible();
  await expect(setup.getByRole('listitem').filter({ hasText: 'Gezin' })).toContainText('127.0.0.1');

  // A Calendar widget shows the dentist.
  const created = await page.request.post('/api/dashboards', {
    data: { id: 'e2e-google', name: 'E2E Google', widgets: [{ id: 'cal', type: 'calendar', x: 0, y: 0, w: 12, h: 8 }] },
  });
  expect(created.status(), await created.text()).toBe(201);
  await page.goto('/preview/e2e-google');
  await expect(page.locator('main.screen .calendar')).toContainText('Tandarts', { timeout: 15_000 });

  // Leave the server as the other journeys expect it.
  expect((await page.request.delete('/api/dashboards/e2e-google')).status()).toBe(204);
  const { settings } = (await (await page.request.get('/api/settings')).json()) as { settings: { calendars: { id: number }[] } };
  for (const c of settings.calendars) expect((await page.request.delete(`/api/calendars/${c.id}`)).status()).toBe(204);
});

test('the Test of a Google calendar explains a 404', async ({ adminPage: page }) => {
  const setup = await openIntegration(page, 'Calendars');
  await setup.getByLabel('Address').fill(`${upstream()}/calendar/ical/family.e2e%40gmail.com/public/basic.ics`);
  await setup.getByRole('button', { name: 'Test', exact: true }).click();
  await expect(setup.getByRole('alert')).toContainText('calendar server answered 404 Not Found');
});
