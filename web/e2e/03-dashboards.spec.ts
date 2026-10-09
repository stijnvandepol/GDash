import { readFile } from 'node:fs/promises';
import { expect, openTab, test } from './fixtures';

const NOTE_TEXT = 'Buy milk for the e2e suite';

interface StoredDashboard {
  name: string;
  widgets: { type: string; settings?: { text?: string } }[];
}

test('create a dashboard, add a clock and a note, edit the note, save and export', async ({ adminPage: page }) => {
  await openTab(page, 'Dashboards');

  await page.getByLabel('Name').fill('E2E Evening');
  await page.getByRole('button', { name: 'Create and edit' }).click();
  await expect(page.getByLabel('Dashboard name')).toHaveValue('E2E Evening');

  await page.getByRole('button', { name: 'Clock', exact: true }).click();
  await page.getByRole('button', { name: 'Note', exact: true }).click();
  // The new widget is selected, so its settings show.
  await page.getByLabel('Text', { exact: true }).fill(NOTE_TEXT);

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();

  const response = await page.request.get('/api/dashboards/e2e-evening');
  expect(response.ok()).toBe(true);
  const stored = (await response.json()) as StoredDashboard;
  expect(stored.name).toBe('E2E Evening');
  expect(stored.widgets.map((w) => w.type).sort()).toEqual(['clock', 'text']);
  expect(stored.widgets.find((w) => w.type === 'text')?.settings?.text).toBe(NOTE_TEXT);

  await page.getByRole('button', { name: '← Dashboards' }).click();
  const row = page.getByRole('listitem').filter({ hasText: 'E2E Evening' });
  await expect(row).toContainText('2 widgets');

  const download = page.waitForEvent('download');
  await row.getByRole('button', { name: 'Export' }).click();
  const file = await download;
  expect(file.suggestedFilename()).toBe('e2e-evening.gdash.json');
  const exported = JSON.parse(await readFile(await file.path(), 'utf8')) as StoredDashboard;
  expect(exported.name).toBe('E2E Evening');
  expect(exported.widgets.find((w) => w.type === 'text')?.settings?.text).toBe(NOTE_TEXT);

  // The exported file is a valid dashboard: importing it adds a copy.
  await page.getByLabel('Dashboard file').setInputFiles(await file.path());
  const copies = page.getByRole('listitem').filter({ hasText: 'E2E Evening' });
  await expect(copies).toHaveCount(2);

  // Deleting asks in GDash's own dialog: Escape keeps the dashboard, the
  // Delete button removes it.
  await copies.last().getByRole('button', { name: 'Delete' }).click();
  const dialog = page.getByRole('dialog', { name: 'Delete this dashboard?' });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(copies).toHaveCount(2);
  await copies.last().getByRole('button', { name: 'Delete' }).click();
  await dialog.getByRole('button', { name: 'Delete' }).click();
  await expect(copies).toHaveCount(1);
});
