import { expect, openTab, test } from './fixtures';

interface StoredDisplay {
  id: string;
  active: { days: string[]; from: string; until: string }[];
  schedule: { from: string; until: string; dashboardId: string; days: string[] }[];
}

// The test makes its own display through the API, so it runs alone too.
test('a screen is on in several blocks on different days, and another dashboard has its own time', async ({ adminPage: page }) => {
  const displays = (await (await page.request.get('/api/displays')).json()) as { displays: StoredDisplay[] };
  if (!displays.displays.some((d) => d.id === 'e2e-schedule')) {
    const made = await page.request.post('/api/displays', { data: { id: 'e2e-schedule', name: 'E2E Schedule', castDeviceId: '' } });
    expect(made.status()).toBe(201);
  }

  await openTab(page, 'Displays');
  const item = page.getByRole('listitem').filter({ hasText: 'E2E Schedule' });
  await item.getByRole('button', { name: 'Schedule' }).click();
  await expect(page.getByRole('heading', { name: 'Schedule for E2E Schedule' })).toBeVisible();

  const hours = page.getByRole('group', { name: 'When the screen is on', exact: true });
  await expect(hours).toContainText('Always on, all day, every day.');

  // Monday to Wednesday from 6 to 10.
  await hours.getByRole('button', { name: 'Add a time' }).click();
  const first = hours.getByRole('group', { name: 'Time 1 for the screen' });
  await first.getByRole('button', { name: 'Th', exact: true }).click();
  await first.getByRole('button', { name: 'Fr', exact: true }).click();
  await first.getByLabel('From').fill('06:00');
  await first.getByLabel('until').fill('10:00');

  // Friday from 7 to 12.
  await hours.getByRole('button', { name: 'Add a time' }).click();
  const second = hours.getByRole('group', { name: 'Time 2 for the screen' });
  for (const day of ['Mo', 'Tu', 'We', 'Th', 'Sa', 'Su']) await second.getByRole('button', { name: day, exact: true }).click();
  await second.getByLabel('From').fill('07:00');
  await second.getByLabel('until').fill('12:00');

  // The weekend from 9 to 4, and again in the evening.
  await hours.getByRole('button', { name: 'Add a time' }).click();
  const third = hours.getByRole('group', { name: 'Time 3 for the screen' });
  await third.getByRole('button', { name: 'Weekend' }).click();
  await third.getByLabel('From').fill('09:00');
  await third.getByLabel('until').fill('16:00');
  await hours.getByRole('button', { name: 'Add a time' }).click();
  const fourth = hours.getByRole('group', { name: 'Time 4 for the screen' });
  await fourth.getByRole('button', { name: 'Weekend' }).click();
  await fourth.getByLabel('From').fill('19:00');
  await fourth.getByLabel('until').fill('23:00');

  await expect(hours.locator('.summary')).toContainText(
    'On: Mon–Wed 06:00–10:00, Fri 07:00–12:00, Weekend 09:00–16:00, Weekend 19:00–23:00. Dark otherwise.',
  );

  // A block needs a day.
  for (const day of ['Sa', 'Su']) await fourth.getByRole('button', { name: day, exact: true }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Time 4: choose at least one day.')).toBeVisible();
  await fourth.getByRole('button', { name: 'Weekend' }).click();

  // Another dashboard at set times is a part of its own.
  await page.getByText('Show another dashboard at set times').click();
  const times = page.locator('details.optional');
  await times.getByRole('button', { name: 'Add a time' }).click();
  await times.getByLabel('From').fill('06:30');
  await times.getByLabel('until').fill('09:00');
  await times.getByRole('group', { name: 'Days for time slot 1' }).getByRole('button', { name: 'Mo' }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Schedule for E2E Schedule' })).toBeHidden();

  const stored = ((await (await page.request.get('/api/displays')).json()) as { displays: StoredDisplay[] }).displays.find(
    (d) => d.id === 'e2e-schedule',
  )!;
  expect(stored.active).toEqual([
    { days: ['mon', 'tue', 'wed'], from: '06:00', until: '10:00' },
    { days: ['fri'], from: '07:00', until: '12:00' },
    { days: ['sat', 'sun'], from: '09:00', until: '16:00' },
    { days: ['sat', 'sun'], from: '19:00', until: '23:00' },
  ]);
  expect(stored.schedule).toHaveLength(1);
  expect(stored.schedule[0]).toMatchObject({ from: '06:30', until: '09:00', days: ['mon'] });

  // The list says it in words, and the editor shows it again as saved.
  await expect(page.getByRole('listitem').filter({ hasText: 'E2E Schedule' })).toContainText('On Mon–Wed 06:00–10:00, Fri 07:00–12:00');
});
