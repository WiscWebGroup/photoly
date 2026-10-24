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

test('Dates groups, lazy paging, calendar jump, viewer folder link and URL persistence', async ({
  page,
}, testInfo) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await signIn(page)
  const calls = await mockApi(page, {
    '/photo/getDateSummary': ok({
      totalCount: 122,
      undatedCount: 1,
      days: [
        { date: '2020-01-01', count: 1 },
        { date: '2025-06-01', count: 120 },
      ],
    }),
    '/photo/getByDate': ({ query }) =>
      ok({
        items:
          query.day === '2020-01-01'
            ? [photo(1, 'Early', { timelineDate: '2020-01-01T10:00:00' })]
            : query.day === 'undated'
              ? [photo(500, 'Undated', { timelineDate: null })]
              : Array.from({ length: query.cursor ? 20 : 100 }, (_, i) =>
                  photo((query.cursor ? 102 : 2) + i, `Later ${i}`, {
                    timelineDate: '2025-06-01T12:00:00',
                  }),
                ),
        nextCursor: query.day === '2025-06-01' && !query.cursor ? 'next-page' : null,
      }),
    '/namespace/trace': ok([
      { nsId: 1, nsName: '/' },
      { nsId: 5, nsName: 'Trips' },
    ]),
  })
  await page.goto('/home/DatesPart')
  await expect(page.getByRole('heading', { name: 'Dates', exact: true })).toBeVisible()
  await expect(page.locator('[data-photo-id="2"]')).toBeVisible()
  expect(
    calls.filter((c) => c.path === '/photo/getByDate').every((c) => c.query.day === '2025-06-01'),
  ).toBeTruthy()
  await page.getByRole('button', { name: 'Hide photo calendar' }).click()
  await page.locator('[data-date-page="2025-06-01:1"]').scrollIntoViewIfNeeded()
  await expect(page.locator('[data-photo-id="102"]')).toBeVisible()
  expect(
    calls.some((c) => c.path === '/photo/getByDate' && c.query.cursor === 'next-page'),
  ).toBeTruthy()
  await page.getByRole('button', { name: 'Show photo calendar' }).click()
  await page.getByLabel('Calendar year', { exact: true }).selectOption('2020')
  await page.getByLabel('Calendar month', { exact: true }).selectOption('01')
  await page.getByRole('button', { name: '2020-01-01, 1 photo', exact: true }).click()
  await expect(page.locator('[data-photo-id="1"]')).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'June 2025, 120 photos', exact: true }),
  ).toHaveAttribute('aria-expanded', 'true')
  await expect(
    page.getByRole('button', { name: 'January 2020, 1 photo', exact: true }),
  ).toHaveAttribute('aria-expanded', 'true')
  await page.screenshot({ path: testInfo.outputPath('dates-calendar.png') })
  await page.getByRole('button', { name: 'Hide photo calendar' }).click()
  await page.getByRole('button', { name: 'Oldest', exact: true }).click()
  await expect(page).toHaveURL(/order=oldest/)
  await expect(page.locator('[data-photo-id="1"]')).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Oldest', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(page.getByRole('button', { name: 'Show photo calendar' })).toBeVisible()
  await expect(page.locator('[data-photo-id="1"]')).toBeVisible()
  await page.locator('[data-photo-id="1"]').click()
  const details = page
    .getByRole('button', { name: /^(Details|Show details)$/ })
    .filter({ visible: true })
  if (await details.count()) await details.first().click()
  await expect(page.getByRole('link', { name: 'Open folder', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Open folder', exact: true }).click()
  await expect(page).toHaveURL(/HomePart\?folder=1/)
  expect(errors).toEqual([])
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
    '/photo/getDateSummary': () => {
      if (failedDates) {
        failedDates = false
        return { msgCode: 500, t: null }
      }
      return ok({ totalCount: 1, undatedCount: 0, days: [{ date: '2024-01-01', count: 1 }] })
    },
    '/photo/getByDate': ok({
      items: [photo(1, 'Recovered', { timelineDate: '2024-01-01' })],
      nextCursor: null,
    }),
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

test('large libraries fetch only visited dates and keep rendered pages bounded', async ({
  page,
}) => {
  await signIn(page)
  const days = Array.from({ length: 36 * 12 }, (_, index) => ({
    date: `${1990 + Math.floor(index / 12)}-${String((index % 12) + 1).padStart(2, '0')}-15`,
    count: 10000,
  }))
  const calls = await mockApi(page, {
    '/photo/getDateSummary': ok({ totalCount: days.length * 10000, undatedCount: 0, days }),
    '/photo/getByDate': ({ query }) => {
      const offset = Number(query.cursor || 0)
      return ok({
        items: Array.from({ length: 100 }, (_, i) =>
          photo(offset + i + 1, `${query.day}-${offset + i}`, { timelineDate: query.day }),
        ),
        nextCursor: offset + 100 < 10000 ? String(offset + 100) : null,
      })
    },
  })
  await page.goto('/home/DatesPart')
  await expect(page.locator('[data-photo-id]').first()).toBeVisible()
  expect(
    calls.filter((c) => c.path === '/photo/getByDate').every((c) => c.query.day === '2025-12-15'),
  ).toBeTruthy()
  await page.getByLabel('Calendar year', { exact: true }).selectOption('1990')
  await page.getByLabel('Calendar month', { exact: true }).selectOption('01')
  await page.getByRole('button', { name: '1990-01-15, 10000 photos', exact: true }).click()
  const day = page.locator('[data-date-day="1990-01-15"]')
  await expect(day.locator('[data-photo-id]').first()).toBeVisible()
  await expect.poll(async () => (await day.boundingBox())!.y).toBeLessThan(200)
  expect(
    calls
      .filter((c) => c.path === '/photo/getByDate')
      .every((c) => ['1990-01-15', '2025-12-15'].includes(c.query.day!)),
  ).toBeTruthy()
  await expect.poll(() => page.locator('[data-photo-id]').count()).toBeLessThanOrEqual(300)
  // Recycling must actually remove the old month's cards, rather than just hide their images.
  await expect(page.locator('[data-date-day="2025-12-15"] [data-photo-id]')).toHaveCount(0)
  expect(calls.filter((c) => c.path === '/photo/getByDate').length).toBeLessThanOrEqual(6)
})
