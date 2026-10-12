import { expect, test } from '@playwright/test';
import { ADMIN_USERNAME, adminPassword } from './env';

// Runs first on a server without an account: it creates the one the other
// journeys log in with.
test('first run: set up from the home network without a token, log out, log in, and a wrong password is refused', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Set up this server' })).toBeVisible();

  await page.getByLabel('Username').fill(ADMIN_USERNAME);
  await page.getByLabel('Password').fill(adminPassword());

  // The test server is on the loopback address and has just started, so
  // the person installing it is not asked for the token in its log
  // (internal/auth SetupWindow; the token path is tested in Go).
  await expect(page.getByLabel('Setup token')).toHaveCount(0);
  await page.getByRole('button', { name: 'Create account' }).click();
  await expect(page.getByRole('button', { name: /Account menu/ })).toBeVisible();
  await expect(page.getByRole('banner').getByText(ADMIN_USERNAME)).toBeVisible();

  await page.getByRole('button', { name: /Account menu/ }).click();
  await page.getByRole('menuitem', { name: 'Log out' }).click();
  await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();

  await page.getByLabel('Username').fill(ADMIN_USERNAME);
  await page.getByLabel('Password').fill(`${adminPassword()}-wrong`);
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.getByText('The username or password is not correct.')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Log in' })).toBeVisible();

  await page.getByLabel('Password').fill(adminPassword());
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.getByRole('button', { name: /Account menu/ })).toBeVisible();

  // The session is a cookie: a reload stays logged in.
  await page.reload();
  await expect(page.getByRole('navigation').getByRole('button', { name: 'Displays' })).toBeVisible();
});
