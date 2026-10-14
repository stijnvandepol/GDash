import type { Page } from '@playwright/test';
import { expect, openIntegration, openTab, test } from './fixtures';
import { NEWS_STAND_INS } from './upstream';

// Two sources from the built-in news catalogue, which the server reads from
// the stand-in in e2e/upstream.ts (GDASH_TEST_UPSTREAM, see
// cmd/gdash/news.go), and two News widgets that each show one of them.

const NOS = NEWS_STAND_INS['nos-algemeen']![0]!;
const TWEAKERS = NEWS_STAND_INS.tweakers![0]!;

/** Removes every news feed, such as the one the server seeded, so only the chosen sources are fetched. */
async function removeAllFeeds(page: Page) {
  const { settings } = (await (await page.request.get('/api/settings')).json()) as { settings: { newsFeeds: { id: number }[] } };
  for (const feed of settings.newsFeeds) expect((await page.request.delete(`/api/news-feeds/${feed.id}`)).status()).toBe(204);
}

/** Makes a dashboard in the editor with one News widget that shows only source. */
async function newsDashboard(page: Page, name: string, source: string) {
  await openTab(page, 'Dashboards');
  await page.getByLabel('Name').fill(name);
  await page.getByRole('button', { name: 'Create and edit' }).click();
  const search = page.getByRole('complementary', { name: 'Add widgets' }).getByRole('searchbox', { name: 'Search widgets' });
  await search.fill('headlines');
  await search.press('Enter');
  const settings = page.getByRole('complementary', { name: 'Widget settings' });
  await expect(settings.getByRole('heading', { name: 'Selected News' })).toBeVisible();
  await expect(settings.getByLabel('All chosen sources')).toBeChecked();
  await settings.getByLabel('Only these sources').check();
  await expect(settings.getByText('Tick at least one source, or choose all chosen sources.')).toBeVisible();
  await settings.getByRole('checkbox', { name: source, exact: true }).check();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved. Screens showing this dashboard update now.')).toBeVisible();
  await page.getByRole('button', { name: '← Dashboards' }).click();
}

test('news sources are chosen from the catalogue and each News widget shows only its own', async ({ adminPage: page }) => {
  await removeAllFeeds(page);
  const setup = await openIntegration(page, 'News feeds');
  const card = setup.locator('section.card', { has: page.getByRole('heading', { name: 'News', exact: true }) });

  const search = card.getByRole('searchbox', { name: 'Search news sources' });
  await search.fill('nos');
  await expect(card.getByRole('checkbox', { name: 'Tweakers', exact: true })).toHaveCount(0);
  await card.getByRole('checkbox', { name: 'NOS Nieuws', exact: true }).check();
  await expect(page.getByText('Added NOS Nieuws.')).toBeVisible();
  await search.fill('tweakers');
  await card.getByRole('checkbox', { name: 'Tweakers', exact: true }).check();
  await expect(page.getByText('Added Tweakers.')).toBeVisible();
  await expect(card.getByText('2 of 10 chosen')).toBeVisible();

  // The chosen sources are listed by name and where they are from, not by address.
  const chosen = card.getByRole('list').first();
  await expect(chosen.getByRole('listitem').filter({ hasText: 'NOS Nieuws' })).toContainText('Netherlands · General');
  await expect(chosen.getByRole('listitem').filter({ hasText: 'Tweakers' })).toContainText('Netherlands · Tech');
  await expect(chosen).not.toContainText('https://');

  await newsDashboard(page, 'E2E Kitchen news', 'NOS Nieuws');
  await newsDashboard(page, 'E2E Study news', 'Tweakers');

  // Each preview shows its own source under that source's name, and its
  // snapshot holds nothing of the other.
  await page.goto('/preview/e2e-kitchen-news');
  const kitchen = page.locator('main.screen');
  await expect(kitchen.getByText(NOS)).toBeVisible({ timeout: 15_000 });
  await expect(kitchen.getByRole('heading', { name: 'NOS Nieuws' })).toBeVisible();
  await expect(kitchen).not.toContainText(TWEAKERS);
  expect(await (await page.request.get('/api/dashboards/e2e-kitchen-news/preview')).text()).not.toContain(TWEAKERS);

  await page.goto('/preview/e2e-study-news');
  const study = page.locator('main.screen');
  await expect(study.getByText(TWEAKERS)).toBeVisible({ timeout: 15_000 });
  await expect(study.getByRole('heading', { name: 'Tweakers' })).toBeVisible();
  await expect(study).not.toContainText(NOS);
  expect(await (await page.request.get('/api/dashboards/e2e-study-news/preview')).text()).not.toContain(NOS);

  // A widget from before the choice shows every source, each headline with its source's name.
  const made = await page.request.post('/api/dashboards', {
    data: {
      id: 'e2e-all-news',
      name: 'E2E All news',
      widgets: [{ id: 'news', type: 'news', x: 0, y: 0, w: 14, h: 8, settings: { maxItems: 4 } }],
    },
  });
  expect(made.status()).toBe(201);
  await page.goto('/preview/e2e-all-news');
  const all = page.locator('main.screen');
  await expect(all.getByText(NOS)).toBeVisible({ timeout: 15_000 });
  await expect(all.getByText(TWEAKERS)).toBeVisible();
  await expect(all.getByRole('listitem').filter({ hasText: TWEAKERS })).toContainText('Tweakers');
  await expect(all.getByRole('listitem').filter({ hasText: NOS })).toContainText('NOS Nieuws');
});
