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
  await picker.locator('.themes').getByText('Walnut', { exact: true }).click();
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

test('the film and game styles are listed in their groups, and a preview draws them in their own typeface', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Games');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  await page.getByRole('button', { name: 'Clock', exact: true }).click();
  await page.getByRole('button', { name: /^Style/ }).click();
  const picker = page.getByRole('dialog', { name: 'Style' });
  const section = (name: string) => picker.locator(`.section-head:has(> h3:text-is("${name}")) + .themes`);
  for (const name of ['Hyperspace', 'Dark side', 'Desert planet', 'Wizard school', 'Middle realm', 'Rain city', 'Time machine', 'Agent 00']) {
    await expect(section('Film and series').getByRole('radio', { name: new RegExp(`^${name}`) })).toBeAttached();
  }
  await expect(section('Games').getByRole('radio')).toHaveCount(4);
  for (const name of ['Shiny hunt', 'Pocket console', 'Block world', 'Quest menu']) {
    await expect(section('Games').getByRole('radio', { name: new RegExp(`^${name}`) })).toBeAttached();
  }
  await section('Games').getByText('Pocket console', { exact: true }).click();
  await expect(picker.getByText('The theme’s own, Jersey 10')).toBeVisible();
  await picker.getByRole('button', { name: 'Done' }).click();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  for (const [theme, family] of [
    ['pocket-console', 'Jersey 10'],
    ['hyperspace', 'Oswald Variable'],
  ] as const) {
    const saved = await page.request.put('/api/dashboards/e2e-games', {
      data: { name: 'E2E Games', theme, widgets: [{ id: 'clock', type: 'clock', x: 0, y: 0, w: 8, h: 4 }] },
    });
    expect(saved.ok(), await saved.text()).toBe(true);
    await page.goto('/preview/e2e-games');
    const screen = page.locator('main.screen');
    await expect(screen).toHaveAttribute('data-theme', theme);
    await expect.poll(() => screen.evaluate((el) => getComputedStyle(el).fontFamily)).toContain(family);
    // The face itself has loaded, so the clock is not drawn in a fallback.
    await expect
      .poll(() => page.evaluate((f) => [...document.fonts].some((face) => face.family.replace(/["']/g, '') === f && face.status === 'loaded'), family))
      .toBe(true);
  }
});

test('the art styles have a section of their own, and their motif changes from tile to tile on the TV page', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill('E2E Art');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  await page.getByRole('button', { name: 'Clock', exact: true }).click();
  await page.getByRole('button', { name: '← Add widgets' }).click();
  await page.getByRole('button', { name: 'Note', exact: true }).click();

  await page.getByRole('button', { name: /^Style/ }).click();
  const picker = page.getByRole('dialog', { name: 'Style' });
  await expect(picker.getByRole('heading', { name: 'Art & abstract' })).toBeVisible();
  for (const name of ['De Stijl', 'Delft blue', 'Tessellation', 'Bauhaus', 'Starry swirl', 'Golden mosaic', 'Great wave', 'Colour field', 'Memphis', 'Art deco', 'Risograph', 'Brutalist']) {
    await expect(picker.getByRole('radio', { name: new RegExp(`^${name}`) })).toHaveCount(1);
  }
  // The sample is drawn in the theme: De Stijl's black ground and its new typeface.
  const choice = picker.locator('label.choice').filter({ hasText: 'De Stijl' });
  const sample = choice.locator('.sample');
  await choice.click();
  await expect(picker.getByRole('radio', { name: /^De Stijl/ })).toBeChecked();
  expect(await sample.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(20, 20, 20)');
  expect(await sample.evaluate((el) => getComputedStyle(el).fontFamily)).toContain('Jost');
  await picker.getByRole('button', { name: 'Done' }).click();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  await page.goto('/preview/e2e-art');
  const screen = page.locator('main.screen');
  await expect(screen).toHaveAttribute('data-theme', 'de-stijl');
  const tiles = page.locator('section.widget');
  await expect(tiles).toHaveCount(2);
  // A red plane on the first tile, a blue one on the second.
  const image = (i: number) => tiles.nth(i).evaluate((el) => getComputedStyle(el).backgroundImage);
  expect(await image(0)).toContain('rgb(212, 32, 42)');
  expect(await image(1)).toContain('rgb(29, 79, 163)');
});
