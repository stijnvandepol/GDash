import { expect, openTab, test } from './fixtures';

interface StoredDisplay {
  id: string;
  activeFrom: string;
  activeUntil: string;
  schedule: { from: string; until: string; dashboardId: string; days: string[] }[];
}

// The test makes its own display through the API, so it runs alone too.
test('a schedule with active hours and a weekday time slot is saved', async ({ adminPage: page }) => {
  const displays = (await (await page.request.get('/api/displays')).json()) as { displays: StoredDisplay[] };
  if (!displays.displays.some((d) => d.id === 'e2e-schedule')) {
    const made = await page.request.post('/api/displays', { data: { id: 'e2e-schedule', name: 'E2E Schedule', castDeviceId: '' } });
    expect(made.status()).toBe(201);
  }

  await openTab(page, 'Displays');
  const item = page.getByRole('listitem').filter({ hasText: 'E2E Schedule' });
  await item.getByRole('button', { name: 'Schedule' }).click();
  await expect(page.getByRole('heading', { name: 'Schedule for E2E Schedule' })).toBeVisible();

  const hours = page.getByRole('group', { name: 'When the screen is on' });
  await hours.getByLabel('Always on').uncheck();
  await hours.getByLabel('On from').fill('07:00');
  await hours.getByLabel('until').fill('22:00');

  const times = page.getByRole('group', { name: 'Another dashboard at set times' });
  await times.getByRole('button', { name: 'Add a time' }).click();
  await times.getByLabel('From').fill('06:30');
  await times.getByLabel('until').fill('09:00');
  await times.getByRole('group', { name: 'Days for time slot 1' }).getByRole('button', { name: 'Mo' }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Schedule for E2E Schedule' })).toBeHidden();

  const stored = ((await (await page.request.get('/api/displays')).json()) as { displays: StoredDisplay[] }).displays.find(
    (d) => d.id === 'e2e-schedule',
  )!;
  expect(stored.activeFrom).toBe('07:00');
  expect(stored.activeUntil).toBe('22:00');
  expect(stored.schedule).toHaveLength(1);
  expect(stored.schedule[0]).toMatchObject({ from: '06:30', until: '09:00', days: ['mon'] });
});
