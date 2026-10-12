import { expect, openTab, test } from './fixtures';

// The test server runs with GDASH_UPDATE_CHECK=off and never asks GitHub
// (see global-setup.ts), so a newer release is answered here.
test('a newer release is announced calmly, and the check can be turned off', async ({ adminPage: page }) => {
  await page.route('**/api/system', async (route) => {
    const response = await route.fetch();
    const system = (await response.json()) as Record<string, unknown>;
    await route.fulfill({
      response,
      json: {
        ...system,
        updateCheckLocked: false,
        update: { version: '9.9.0', url: 'https://github.com/stijnvandepol/gdash/releases/tag/v9.9.0' },
      },
    });
  });
  await page.reload();

  const notice = page.getByRole('status').filter({ hasText: 'GDash 9.9.0 is available' });
  await expect(notice).toBeVisible();
  await expect(notice.getByRole('link', { name: "What's new" })).toHaveAttribute(
    'href',
    'https://github.com/stijnvandepol/gdash/releases/tag/v9.9.0',
  );
  await expect(notice.getByRole('link', { name: 'How to update' })).toHaveAttribute('href', /docs\/install\.md#updating$/);

  await openTab(page, 'Settings');
  await page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: 'Account' }).click();
  const updates = page.getByRole('region', { name: 'Updates' });
  const check = updates.getByLabel('Check for new versions once a day');
  await expect(check).toBeChecked();
  await check.uncheck();
  await expect(check).not.toBeChecked();
  const stored = (await (await page.request.get('/api/settings')).json()) as { settings: { updateCheck: boolean } };
  expect(stored.settings.updateCheck).toBe(false);
  await check.check();
  await expect(check).toBeChecked();
});
