import { test, expect } from '@playwright/test'
import { mockApi, signIn, ok, photo } from './mockApi'

const summary = {
  nsId: 5,
  nsName: 'Trips',
  photoCount: 42,
  videoCount: 3,
  folderCount: 6,
  originalBytes: 12000000,
  thumbnailBytes: 500000,
  totalBytes: 12500000,
  complete: false,
  missingFileCount: 1,
  places: [{ name: 'New York City, United States', count: 12 }],
  cameras: Array.from({ length: 12 }, (_, i) => ({ name: `Camera ${i + 1}`, count: 1 })),
  lenses: [],
}

test('Dates navigation, server ordering, paging, viewer folder link, and URL persistence', async ({
  page,
}) => {
  await signIn(page)
  const calls = await mockApi(page, {
    '/photo/getByDate': ({ query }) =>
      ok({
        items: query.cursor
          ? [photo(3, 'Undated', { timelineDate: null })]
          : query.order === 'oldest'
            ? [
                photo(1, 'Early', { timelineDate: '2020-01-01T10:00:00' }),
                photo(2, 'Later', { timelineDate: '2025-06-01T12:00:00' }),
              ]
            : [
                photo(2, 'Later', { timelineDate: '2025-06-01T12:00:00' }),
                photo(1, 'Early', { timelineDate: '2020-01-01T10:00:00' }),
              ],
        nextCursor: query.cursor ? null : 'next-page',
      }),
    '/namespace/trace': ok([
      { nsId: 1, nsName: '/' },
      { nsId: 5, nsName: 'Trips' },
    ]),
  })
  await page.goto('/home/HomePart')
  await page.getByRole('link', { name: 'Dates', exact: true }).filter({ visible: true }).click()
  await expect(page.getByRole('heading', { name: 'Dates', exact: true })).toBeVisible()
  const cards = page.locator('[data-photo-id]')
  await expect(cards.first()).toHaveAttribute('data-photo-id', '2')
  await page.getByRole('button', { name: 'Load more photos' }).click()
  await expect(cards).toHaveCount(3)
  expect(
    calls.some((c) => c.path === '/photo/getByDate' && c.query.cursor === 'next-page'),
  ).toBeTruthy()
  await page.getByRole('button', { name: 'Oldest', exact: true }).click()
  await expect(page).toHaveURL(/order=oldest/)
  await expect(cards.first()).toHaveAttribute('data-photo-id', '1')
  await expect(cards).toHaveCount(2)
  await page.reload()
  await expect(page.getByRole('button', { name: 'Oldest', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(cards.first()).toHaveAttribute('data-photo-id', '1')
  await page.screenshot({ path: '/tmp/photoly-dates.png', animations: 'disabled' })
  await cards.first().click()
  // The viewer's Details panel exposes a link to the real folder.
  const details = page
    .getByRole('button', { name: 'Details', exact: true })
    .filter({ visible: true })
  if (await details.count()) await details.first().click()
  await expect(page.getByRole('link', { name: 'Open folder', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Open folder', exact: true }).click()
  await expect(page).toHaveURL(/HomePart\?folder=1/)
})

test('folder Details supports right-click, paging, missing files and Escape', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/namespace/getChildren': ok([{ nsId: 5, nsName: 'Trips', nsParentId: 1 }]),
    '/namespace/getDetails': ok(summary),
  })
  await page.goto('/home/HomePart')
  const folder = page.locator('[data-folder-id="5"]')
  await expect(page.getByRole('button', { name: 'Actions for Trips' })).toHaveCount(0)
  await folder.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Details', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Trips — Details' })).toBeVisible()
  await expect(dialog.getByText('42', { exact: true })).toBeVisible()
  await expect(dialog.getByText('12.5 MB', { exact: true })).toBeVisible()
  await expect(dialog.getByText(/Storage totals are incomplete/)).toBeVisible()
  await expect(dialog.getByText('New York City, United States')).toBeVisible()
  await expect(dialog.getByText('No information available.')).toBeVisible()
  await dialog.getByRole('button', { name: 'Next Cameras' }).click()
  await expect(dialog.getByText('Camera 11', { exact: true })).toBeVisible()
  await page.screenshot({ path: '/tmp/photoly-folder-details.png', animations: 'disabled' })
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(folder).toBeFocused()
  await page.locator('[data-folder-id="5"]').click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Details', exact: true }).click()
  await expect(page.getByRole('dialog').getByText('Camera 1', { exact: true })).toBeVisible()
})

test('Dates and folder Details offer retry after a failed request', async ({ page }) => {
  await signIn(page)
  let failedDates = true
  let failedFolder = true
  await mockApi(page, {
    '/photo/getByDate': () => {
      if (failedDates) {
        failedDates = false
        return { msgCode: 500, t: null }
      }
      return ok({
        items: [photo(1, 'Recovered', { timelineDate: '2024-01-01' })],
        nextCursor: null,
      })
    },
    '/namespace/getChildren': ok([{ nsId: 5, nsName: 'Trips', nsParentId: 1 }]),
    '/namespace/getDetails': () => {
      if (failedFolder) {
        failedFolder = false
        return { msgCode: 400, t: null }
      }
      return ok(summary)
    },
  })
  await page.goto('/home/DatesPart')
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(page.locator('[data-photo-id="1"]')).toBeVisible()
  await page.goto('/home/HomePart')
  await page.locator('[data-folder-id="5"]').click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Details', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(dialog.getByText('42', { exact: true })).toBeVisible()
})
