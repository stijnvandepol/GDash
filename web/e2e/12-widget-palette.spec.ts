import AxeBuilder from '@axe-core/playwright';
import { expect, openTab, test } from './fixtures';

test('the widget palette finds widgets by search and adds them from their category', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Palette');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  const palette = page.getByRole('complementary', { name: 'Add widgets' });
  const search = palette.getByRole('searchbox', { name: 'Search widgets' });
  await expect(search).toBeVisible();

  // The search reads descriptions too: pollen finds the air quality widget only.
  await search.fill('pollen');
  await expect(palette.locator('button.item')).toHaveCount(1);
  await expect(palette.getByRole('button', { name: 'Air quality', exact: true })).toBeVisible();
  await search.press('Escape');
  await expect(search).toHaveValue('');
  await expect(palette.getByRole('button', { name: 'Clock', exact: true })).toBeVisible();

  // Enter adds the first match, and the panel shows its settings.
  await search.fill('fuel');
  await search.press('Enter');
  const settings = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(settings.getByRole('heading', { name: 'Selected Fuel prices' })).toBeFocused();

  // Back to the palette: the search has focus, and the arrow keys walk the widgets.
  await settings.getByRole('button', { name: '← Add widgets' }).click();
  await expect(search).toBeFocused();
  await search.press('ArrowDown');
  await expect(palette.getByRole('button', { name: 'Clock', exact: true })).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(palette.getByRole('button', { name: 'Countdown', exact: true })).toBeFocused();

  // A category lists its widgets; one is added with a click.
  const household = palette.getByRole('region', { name: 'Agenda & household' });
  await expect(household.locator('button.item .name')).toHaveText(['Calendar', 'Waste collection', 'Note']);
  await household.getByRole('button', { name: 'Note', exact: true }).click();
  await expect(settings.getByRole('heading', { name: 'Selected Note' })).toBeVisible();

  // The editor with the palette has no serious accessibility problems.
  await settings.getByRole('button', { name: '← Add widgets' }).click();
  const { violations } = await new AxeBuilder({ page }).include('.editor').analyze();
  const serious = violations
    .filter((v) => v.impact === 'serious' || v.impact === 'critical')
    .map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`);
  expect(serious, serious.join('\n')).toEqual([]);

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();
  const stored = (await (await page.request.get('/api/dashboards/e2e-palette')).json()) as { widgets: { type: string }[] };
  expect(stored.widgets.map((w) => w.type).sort()).toEqual(['fuel', 'text']);
});
