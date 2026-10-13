import { adminPassword } from './env';
import { expect, openIntegration, openTab, test } from './fixtures';

test.describe('settings', () => {
  test.beforeEach(async ({ adminPage }) => {
    await openTab(adminPage, 'Settings');
  });

  test('the language of the TVs can be changed', async ({ adminPage: page }) => {
    const screens = page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: 'Screens' });
    await screens.click();
    await expect(screens).toHaveAttribute('aria-current', 'page');
    const language = page.getByLabel('Language on the TVs');
    await expect(language).toHaveValue('en');
    await language.selectOption('nl');

    await page.reload();
    await openTab(page, 'Settings');
    await screens.click();
    await expect(language).toHaveValue('nl');
    const stored = await page.request.get('/api/settings');
    expect(((await stored.json()) as { settings: { language: string } }).settings.language).toBe('nl');
  });

  test('a backup is refused with a wrong password and downloads with the right one', async ({ adminPage: page }) => {
    await page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: 'Account' }).click();
    const section = page.locator('section.card', { has: page.getByRole('heading', { name: 'Backup' }) });

    await section.getByLabel('Your password').fill('not the password at all');
    await section.getByRole('button', { name: 'Download backup' }).click();
    await expect(section.getByText('this is not your password')).toBeVisible();

    await section.getByLabel('Your password').fill(adminPassword());
    const download = page.waitForEvent('download');
    await section.getByRole('button', { name: 'Download backup' }).click();
    expect((await download).suggestedFilename()).toMatch(/^gdash-backup-\d{4}-\d{2}-\d{2}\.zip$/);
    await expect(section.getByLabel('Your password')).toHaveValue('');
  });
});

// The sources are fetched in the background and may fail without internet:
// these tests check what is stored and shown, not the data.
test.describe('integrations', () => {
  test('a location entered as coordinates is saved without a place search', async ({ adminPage: page }) => {
    const setup = await openIntegration(page, 'Weather');
    const section = setup.locator('section.card', { has: page.getByRole('heading', { name: 'Weather location' }) });
    await section.getByRole('button', { name: 'Enter coordinates' }).click();

    await section.getByLabel('Name').fill('Nowhere');
    await section.getByLabel('Latitude').fill('north');
    await section.getByLabel('Longitude').fill('5.2');
    await section.getByRole('button', { name: 'Save' }).click();
    await expect(section.getByText('Enter the latitude and longitude as numbers')).toBeVisible();

    await section.getByLabel('Name').fill('E2E Home');
    await section.getByLabel('Latitude').fill('52,10');
    await section.getByLabel('Longitude').fill('5.20');
    await section.getByRole('button', { name: 'Save' }).click();

    await expect(section.getByText('E2E Home')).toBeVisible();
    await expect(section.getByText('(52.10, 5.20)')).toBeVisible();
    // The card says so too.
    await expect(page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: 'Weather', exact: true, level: 3 }) })).toContainText('Connected');

    await page.reload();
    await openIntegration(page, 'Weather');
    await expect(section.getByText('(52.10, 5.20)')).toBeVisible();
  });

  test('a news feed can be added and stays in the list', async ({ adminPage: page }) => {
    const setup = await openIntegration(page, 'News feeds');
    const section = setup.locator('section.card', { has: page.getByRole('heading', { name: 'News', exact: true }) });
    await section.getByLabel('Name').fill('E2E feed');
    await section.getByLabel('Address').fill('https://feeds.example.invalid/e2e.xml');
    await section.getByRole('button', { name: 'Add' }).click();

    const item = section.getByRole('listitem').filter({ hasText: 'E2E feed' });
    await expect(item).toContainText('https://feeds.example.invalid/e2e.xml');
    // The form is empty again, ready for the next feed.
    await expect(section.getByLabel('Name')).toHaveValue('');

    await page.reload();
    await openIntegration(page, 'News feeds');
    await expect(item).toBeVisible();
  });
});
