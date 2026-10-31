import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

const longName = 'A very long folder name 中文文件夹 that must stay on a single line'
async function setup(page: Page) {
  await signIn(page)
  await mockApi(page, {
    '/namespace/getChildren': ok([
      { nsId: 2, nsName: longName },
      { nsId: 3, nsName: 'SMB', isMount: true, sourceType: 'smb', sourceId: 3 },
      { nsId: 4, nsName: 'WebDAV', isMount: true, sourceType: 'webdav', sourceId: 4 },
      { nsId: 5, nsName: 'S3', isMount: true, sourceType: 's3', sourceId: 5 },
    ]),
    '/photo/getCovers': ok([{ id: 2, count: 3, coverIds: [10, 11, 12] }]),
    '/photo/getByNamespace': ok([photo(10, 'beach'), photo(11, 'video', { format: 'mp4' })]),
  })
  await page.goto('/home/HomePart')
  await expect(page.getByText('3 items', { exact: true })).toBeVisible()
}
async function mode(page: Page, name: string) {
  const desktop = page.getByRole('button', { name, exact: true })
  if (page.viewportSize()!.width >= 768) await desktop.click()
  else {
    await page.getByRole('button', { name: 'Add', exact: true }).click()
    await page.getByRole('menuitem', { name, exact: true }).click()
  }
  // Let the card enter animation and ResizeObserver complete before measuring.
  await expect(page.locator('ul[data-layout]').first()).toHaveAttribute(
    'data-layout',
    name === 'Small cards' ? 'compact' : name === 'Large cards' ? 'grid' : 'list',
  )
}
const folder = (page: Page) => page.locator('[data-folder-id="2"]')
const media = (page: Page) => page.locator('[data-photo-id="10"]')
const add = (page: Page) =>
  page.getByRole('listitem').getByRole('button', { name: 'New Folder', exact: true })

for (const width of [390, 700, 1100, 1600, 2560]) {
  test(`compact cards halve both dimensions at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 })
    await setup(page)
    for (const hidden of width >= 768 ? [false, true] : [false]) {
      if (hidden) await page.getByRole('button', { name: 'Hide sidebar' }).click()
      await mode(page, 'Large cards')
      const largeFolder = (await folder(page).boundingBox())!
      const largeMedia = (await media(page).boundingBox())!
      await mode(page, 'Small cards')
      await expect
        .poll(async () => {
          const smallFolder = (await folder(page).boundingBox())!
          const smallMedia = (await media(page).boundingBox())!
          return Math.max(
            Math.abs(smallFolder.width - largeFolder.width / 2),
            Math.abs(smallFolder.height - largeFolder.height / 2),
            Math.abs(smallMedia.width - largeMedia.width / 2),
            Math.abs(smallMedia.height - largeMedia.height / 2),
          )
        })
        .toBeLessThanOrEqual(2)
      expect((await add(page).boundingBox())!.height).toBeCloseTo(
        (await folder(page).boundingBox())!.height,
        0,
      )
      const title = folder(page).locator(`[title="${longName}"]`)
      await expect(title).toHaveCSS('text-overflow', 'ellipsis')
      await expect(title).toHaveCSS('white-space', 'nowrap')
      for (const id of [3, 4, 5]) {
        const box = (await page.locator(`[data-folder-id="${id}"]`).boundingBox())!
        expect(box.height).toBeCloseTo((await folder(page).boundingBox())!.height, 0)
      }
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBe(true)
    }
  })
}

test('sidebar state and all layout modes survive reload; selections survive switching', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await setup(page)
  await page.getByRole('button', { name: 'Hide sidebar' }).click()
  await expect(page.getByRole('button', { name: 'Show sidebar' })).toHaveAttribute(
    'aria-expanded',
    'false',
  )
  await folder(page).click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Select', exact: true }).click()
  for (const name of ['Small cards', 'List view', 'Large cards']) {
    await mode(page, name)
    await expect(folder(page)).toHaveAttribute('aria-pressed', 'true')
  }
  for (const [name, value] of [
    ['Small cards', 'compact'],
    ['List view', 'list'],
    ['Large cards', 'grid'],
  ]) {
    await mode(page, name!)
    await page.reload()
    await expect(page.getByRole('button', { name: name!, exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(await page.evaluate(() => localStorage.getItem('photoly-view-mode'))).toBe(value)
    await expect(page.getByRole('button', { name: 'Show sidebar' })).toBeVisible()
    await expect(page.locator('#main-sidebar')).toBeHidden()
  }
  await page.getByRole('button', { name: 'Show sidebar' }).click()
  await expect(page.locator('#main-sidebar')).toBeVisible()
})

test('sidebar resize preserves the map location, filter and open place', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await signIn(page)
  await mockApi(page, {
    '/photo-extra/getGlobalMapPhoto': ok([
      { photo_id: 10, photo_name: 'beach', extra_photo_gps_lat: 20.5, extra_photo_gps_lon: 11.5 },
    ]),
    '/photo-extra/getMapPlaces': ok([{ photoId: 10, place: 'Ghat', country: 'Libya' }]),
  })
  await page.goto('/home/MapPart')
  const marker = page.getByTitle('beach').last()
  await marker.click()
  const map = page.locator('.leaflet-container')
  const beforeMap = (await map.boundingBox())!
  const beforeMarker = (await marker.boundingBox())!
  await page.getByRole('button', { name: 'Hide sidebar' }).click()
  await expect.poll(async () => (await map.boundingBox())!.width).toBe(beforeMap.width + 232)
  // A centered marker stays at the same position relative to the enlarged map center.
  await expect
    .poll(async () => {
      const after = (await marker.boundingBox())!
      const bounds = (await map.boundingBox())!
      return Math.abs(
        after.x -
          bounds.x -
          bounds.width / 2 -
          (beforeMarker.x - beforeMap.x - beforeMap.width / 2),
      )
    })
    .toBeLessThanOrEqual(2)
  await expect(page.getByRole('heading', { name: 'Ghat' })).toBeVisible()
  await expect(page.getByRole('radio', { name: 'Photos', exact: true })).toBeChecked()
})

test('Dates remeasures rows after sidebar and layout changes without losing selection', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await signIn(page)
  await mockApi(page, {
    '/photo/getDateSummary': ok({
      totalCount: 30,
      undatedCount: 0,
      days: [{ date: '2025-06-01', count: 30 }],
    }),
    '/photo/getByDate': ok({
      items: Array.from({ length: 30 }, (_, i) =>
        photo(i + 1, `Photo ${i}`, { timelineDate: '2025-06-01T10:00:00' }),
      ),
      nextCursor: null,
    }),
  })
  await page.goto('/home/DatesPart')
  const first = page.locator('[data-photo-id="1"]')
  await expect(first).toBeVisible()
  await first.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Select', exact: true }).click()
  await expect(first).toHaveAttribute('aria-pressed', 'true')
  const top = (await first.boundingBox())!.y
  await page.getByRole('button', { name: 'Hide sidebar' }).click()
  await expect(first).toBeVisible()
  expect(Math.abs((await first.boundingBox())!.y - top)).toBeLessThanOrEqual(2)
  const datePage = page.locator('[data-date-page]').first()
  await expect
    .poll(async () =>
      datePage.evaluate((el) =>
        Math.abs(
          el.getBoundingClientRect().height - el.firstElementChild!.getBoundingClientRect().height,
        ),
      ),
    )
    .toBeLessThanOrEqual(2)
  await page.getByRole('button', { name: 'Small cards', exact: true }).click()
  await expect(first).toHaveAttribute('aria-pressed', 'true')
  await expect
    .poll(async () =>
      datePage.evaluate((el) =>
        Math.abs(
          el.getBoundingClientRect().height - el.firstElementChild!.getBoundingClientRect().height,
        ),
      ),
    )
    .toBeLessThanOrEqual(2)
})
