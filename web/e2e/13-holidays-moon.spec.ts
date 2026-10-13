import { expect, test } from './fixtures';
import { SCHOOL_HOLIDAYS } from './school-holidays';

// The school holidays come from the stand-in that the global setup starts
// (school-holidays.ts); the public holidays and the moon need no network.

test('a screen lists the coming holidays per region and draws the moon', async ({ adminPage: page }) => {
  expect((await page.request.put('/api/settings/language', { data: { language: 'en' } })).ok()).toBe(true);
  const dashboard = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-holidays',
      name: 'E2E Holidays',
      widgets: [
        { id: 'north', type: 'holidays', x: 0, y: 0, w: 10, h: 7, settings: { region: 'north', maxItems: 3 } },
        { id: 'middle', type: 'holidays', x: 10, y: 0, w: 8, h: 7, settings: { region: 'middle', show: 'school' } },
        { id: 'moon', type: 'moon', x: 18, y: 0, w: 6, h: 7 },
      ],
    },
  });
  expect(dashboard.status(), await dashboard.text()).toBe(201);

  await page.goto('/preview/e2e-holidays');
  const [north, middle, moon] = [0, 1, 2].map((i) => page.locator('main.screen section.widget').nth(i));

  // The north is on its holiday now; the middle starts in four days.
  await expect(north.locator('li').first()).toContainText('Autumn holiday', { timeout: 15_000 });
  await expect(north.locator('li').first()).toContainText('Now, until ');
  // maxItems 3: the autumn and Christmas holidays and the next public holiday.
  await expect(north.locator('li')).toHaveCount(3);
  await expect(middle.locator('li')).toHaveCount(2);
  await expect(middle.locator('li').first()).toContainText(`Starts in ${SCHOOL_HOLIDAYS.middle.start} days (`);
  await expect(middle.locator('li').nth(1)).toContainText('Christmas holiday');

  await expect(moon).toContainText(/% lit/);
  await expect(moon).toContainText('Full moon');
  await expect(moon).toContainText('New moon');
  await expect(moon.locator('svg path.lit')).toHaveAttribute('d', /^M50 4 A46 46 0 0 [01] 50 96 A[\d.]+ 46 0 0 [01] 50 4 Z$/);

  // The screen receives the regions its widgets show, not the others.
  const snapshot = (await (await page.request.get('/api/dashboards/e2e-holidays/preview')).json()) as {
    data: Record<string, { value: Record<string, unknown> }>;
  };
  expect(Object.keys(snapshot.data['holidays.nl']!.value).sort()).toEqual(['middle', 'north', 'public']);

  // In Dutch the holidays keep their Dutch names.
  expect((await page.request.put('/api/settings/language', { data: { language: 'nl' } })).ok()).toBe(true);
  await expect(middle.locator('li').first()).toContainText(`Herfstvakantie`);
  await expect(middle.locator('li').first()).toContainText(`Begint over ${SCHOOL_HOLIDAYS.middle.start} dagen`);
  await expect(moon).toContainText('Volle maan');
});
