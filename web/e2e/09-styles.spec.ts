import AxeBuilder from '@axe-core/playwright';
import { expect, openTab, test } from './fixtures';

test('a style and a typeface are picked in the editor, saved, and shown on the TV page', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Styled');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  await page.getByRole('button', { name: 'Clock', exact: true }).click();

  await page.getByRole('button', { name: /^Style/ }).click();
  const picker = page.getByRole('dialog', { name: 'Style' });
  // The samples are decoration (hidden from screen readers); the names and
  // notes next to them are what has to read well.
  const { violations } = await new AxeBuilder({ page }).include('dialog[open]').analyze();
  const serious = violations
    .filter((v) => v.impact === 'serious' || v.impact === 'critical')
    .map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`);
  expect(serious, serious.join('\n')).toEqual([]);
  // A card is pressed as a whole; its radio button is for the keyboard.
  await picker.getByText('Walnut', { exact: true }).click();
  await expect(picker.getByRole('radio', { name: /Walnut/ })).toBeChecked();
  await picker.getByText('Bodoni', { exact: true }).click();
  await expect(picker.getByRole('radio', { name: /Bodoni/ })).toBeChecked();
  await picker.getByLabel('Style between sunset and sunrise').selectOption({ label: 'Night' });
  await picker.getByRole('button', { name: 'Done' }).click();
  await expect(picker).toBeHidden();
  // The button names what was picked.
  await expect(page.getByRole('button', { name: /^Style/ })).toContainText('Walnut');
  await expect(page.getByRole('button', { name: /^Style/ })).toContainText('Bodoni');
  await expect(page.getByRole('button', { name: /^Style/ })).toContainText('Night after sunset');

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  const stored = (await (await page.request.get('/api/dashboards/e2e-styled')).json()) as { theme: string; font: string; nightTheme: string };
  expect(stored).toMatchObject({ theme: 'walnut', font: 'bodoni', nightTheme: 'night' });

  // The TV page carries both, so its tiles take the theme and the typeface.
  await page.goto('/preview/e2e-styled');
  const screen = page.locator('main.screen');
  await expect(screen).toHaveAttribute('data-theme', 'walnut');
  await expect(screen).toHaveAttribute('data-font', 'bodoni');
  await expect
    .poll(() => screen.evaluate((el) => getComputedStyle(el).fontFamily))
    .toContain('Bodoni Moda');
});
