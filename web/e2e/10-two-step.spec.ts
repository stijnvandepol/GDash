import { createHmac } from 'node:crypto';
import { request as playwrightRequest, type Page } from '@playwright/test';
import { ADMIN_USERNAME, adminPassword, baseUrl } from './env';
import { expect, openTab, test } from './fixtures';

// Two-step login from setup to switching it off, with codes computed the
// way an authenticator app does (RFC 6238: HMAC-SHA-1, 6 digits, 30 s).
// It runs last and leaves two-step login off, also when it fails halfway,
// because every other journey logs in with the password alone.

const BASE32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
const PERIOD_SECONDS = 30;

function decodeBase32(text: string): Buffer {
  const bytes: number[] = [];
  let value = 0;
  let bits = 0;
  for (const char of text.replace(/\s/g, '').toUpperCase()) {
    value = ((value << 5) | BASE32.indexOf(char)) & 0xffff;
    bits += 5;
    if (bits >= 8) {
      bits -= 8;
      bytes.push((value >>> bits) & 0xff);
    }
  }
  return Buffer.from(bytes);
}

function currentStep(): number {
  return Math.floor(Date.now() / 1000 / PERIOD_SECONDS);
}

function codeAt(secret: Buffer, step: number): string {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(step));
  const mac = createHmac('sha1', secret).update(counter).digest();
  const offset = mac.readUInt8(mac.length - 1) & 0x0f;
  return String((mac.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).padStart(6, '0');
}

// Recovery codes that were shown and not used yet, for the clean-up.
let unusedRecoveryCodes: string[] = [];

test.afterAll(async () => {
  const api = await playwrightRequest.newContext({ baseURL: baseUrl() });
  try {
    const login = await api.post('/api/sessions', { data: { username: ADMIN_USERNAME, password: adminPassword() } });
    if (login.status() !== 202) return;
    const { pending } = (await login.json()) as { pending: string };
    const [first, second] = unusedRecoveryCodes;
    if (!first || !second) throw new Error('Two-step login is still on and no recovery codes are left to turn it off.');
    expect((await api.post('/api/sessions/two-step', { data: { pending, code: first } })).status()).toBe(201);
    const off = await api.delete('/api/me/two-step', { data: { password: adminPassword(), code: second } });
    expect(off.status()).toBe(204);
  } finally {
    await api.dispose();
  }
});

async function openAccount(page: Page) {
  await openTab(page, 'Settings');
  await page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: 'Account' }).click();
  return page.locator('section.card', { has: page.getByRole('heading', { name: 'Two-step login' }) });
}

async function logOutAndEnterPassword(page: Page) {
  await page.getByRole('button', { name: /Account menu/ }).click();
  await page.getByRole('menuitem', { name: 'Log out' }).click();
  await page.getByLabel('Username').fill(ADMIN_USERNAME);
  await page.getByLabel('Password').fill(adminPassword());
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.getByRole('heading', { name: 'Two-step login' })).toBeVisible();
}

test('two-step login: set up, log in with a code and a recovery code, turn off', async ({ adminPage: page }) => {
  test.setTimeout(60_000);
  let section = await openAccount(page);
  await section.getByLabel('Your password').fill(adminPassword());
  await section.getByRole('button', { name: 'Set up two-step login' }).click();
  await expect(section.getByRole('img', { name: /QR code/ })).toBeVisible();
  const secret = decodeBase32((await section.locator('.secret').textContent()) ?? '');
  expect(secret).toHaveLength(20);

  const firstStep = currentStep();
  await section.getByLabel('Code', { exact: true }).fill(codeAt(secret, firstStep));
  await section.getByRole('button', { name: 'Turn on two-step login' }).click();
  await expect(section.getByText('Two-step login is on.')).toBeVisible();
  unusedRecoveryCodes = await section.locator('.codes li').allTextContents();
  expect(unusedRecoveryCodes).toHaveLength(10);
  const download = page.waitForEvent('download');
  await section.getByRole('button', { name: 'Download as text' }).click();
  expect((await download).suggestedFilename()).toBe('gdash-recovery-codes.txt');
  await section.getByRole('button', { name: 'I have saved them' }).click();
  await expect(section.getByText('10 recovery codes left.')).toBeVisible();

  // The password alone no longer logs in; a wrong code is refused, the
  // next code from the app works (the first one is used up).
  await logOutAndEnterPassword(page);
  await page.getByLabel('Code', { exact: true }).fill('000000');
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.getByText('The code is not correct.')).toBeVisible();
  await page.getByLabel('Code', { exact: true }).fill(codeAt(secret, firstStep + 1));
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.getByRole('button', { name: /Account menu/ })).toBeVisible();

  // A recovery code logs in once.
  await logOutAndEnterPassword(page);
  await page.getByRole('button', { name: 'Use a recovery code' }).click();
  const recovery = unusedRecoveryCodes.shift() ?? '';
  await page.getByLabel('Recovery code').fill(recovery);
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.getByRole('button', { name: /Account menu/ })).toBeVisible();

  // Turning it off needs the password and a code; the app's codes of this
  // minute are used up, so a recovery code it is.
  section = await openAccount(page);
  await expect(section.getByText('9 recovery codes left.')).toBeVisible();
  await section.getByLabel('Your password').fill(adminPassword());
  await section.getByLabel('Code from your app, or a recovery code').fill(recovery);
  await section.getByRole('button', { name: 'Turn off two-step login' }).click();
  await expect(section.getByText('this code is not correct')).toBeVisible();
  await section.getByLabel('Your password').fill(adminPassword());
  await section.getByLabel('Code from your app, or a recovery code').fill(unusedRecoveryCodes.shift() ?? '');
  await section.getByRole('button', { name: 'Turn off two-step login' }).click();
  await expect(section.getByText('Off', { exact: true })).toBeVisible();

  const login = await page.request.post('/api/sessions', { data: { username: ADMIN_USERNAME, password: adminPassword() } });
  expect(login.status()).toBe(201);
});
