import { expect, test } from '@playwright/test';
import { ADMIN_USERNAME, adminPassword, setupToken } from './env';

// Runs first on a server without an account: it creates the one the other
// journeys log in with.
test('first run: set up with the token, log out, log in, and a wrong password is refused', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Set up this server' })).toBeVisible();

  await page.getByLabel('Username').fill(ADMIN_USERNAME);
  await page.getByLabel('Password').fill(adminPassword());

  await page.getByLabel('Setup token').fill('not-the-token');
  await page.getByRole('button', { name: 'Create account' }).click();
  await expect(page.getByText('The setup token is not correct.')).toBeVisible();

  await page.getByLabel('Setup token').fill(setupToken());
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
