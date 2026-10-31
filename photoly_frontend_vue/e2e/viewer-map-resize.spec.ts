import { expect, test } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

test('wheel zoom follows the pointer, clamps dragging, resets on navigation and leaves metadata scroll alone', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await signIn(page)
  await mockApi(page, {
    '/photo/getByNamespace': ok([photo(1, 'Beach'), photo(2, 'Mountain')]),
    '/photo/mediaInfo': ok({ live: true, liveStatus: 'ready' }),
  })
  // Keep the video request pending so playback state is deterministic without a codec fixture.
  await page.route('**/photo/renderLive/*', () => {})
  // Give the rendered photo realistic dimensions without an external fixture.
  await page.route('**/photo/render/*', (route) =>
    route.fulfill({
      contentType: 'image/svg+xml',
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><rect width="1200" height="800" fill="#348888"/></svg>',
    }),
  )
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click()
  const zoom = page.locator('[data-photo-zoom]').filter({ visible: true })
  await expect(zoom).toHaveAttribute('data-scale', '1')
  const bounds = (await zoom.boundingBox())!
  const anchor = { x: bounds.x + bounds.width * 0.65, y: bounds.y + bounds.height * 0.55 }
  await page.mouse.move(anchor.x, anchor.y)
  await page.mouse.wheel(0, -350)
  await expect.poll(async () => Number(await zoom.getAttribute('data-scale'))).toBeGreaterThan(1)
  await expect(page.getByRole('button', { name: 'Fit to window' })).toBeVisible()
  await page.mouse.down()
  await page.mouse.move(anchor.x + 500, anchor.y + 300, { steps: 5 })
  await page.mouse.up()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByRole('button', { name: 'Fit to window' }).click()
  await expect(zoom).toHaveAttribute('data-scale', '1')
  await page.mouse.move(anchor.x, anchor.y)
  await page.mouse.wheel(0, -200)
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: /Mountain/ })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Beach', exact: true })).toBeHidden()
  await expect(zoom).toHaveCount(1)
  await expect(zoom).toHaveAttribute('data-scale', '1')
  const details = page.getByRole('complementary', { name: 'Photo details' })
  await details.hover()
  await page.mouse.wheel(0, -300)
  await expect(zoom).toHaveAttribute('data-scale', '1')
  await page.mouse.move(anchor.x, anchor.y)
  await page.mouse.wheel(0, -200)
  await expect.poll(async () => Number(await zoom.getAttribute('data-scale'))).toBeGreaterThan(1)
  await page.getByRole('button', { name: 'Play Live Photo', exact: true }).click()
  await expect(zoom).toHaveCount(0)
  await page.getByRole('button', { name: 'Stop Live Photo', exact: true }).click()
  await expect(zoom).toHaveAttribute('data-scale', '1')
})

test('Gallery map clusters photos, opens the shared viewer, persists visibility and hides without GPS', async ({
  page,
}, testInfo) => {
  await signIn(page)
  const galleries = [
    { gaId: 7, gaName: 'Trip', createDate: '2024-06-01', coverId: 1 },
    { gaId: 8, gaName: 'No GPS', createDate: '2024-06-02', coverId: 1 },
  ]
  await mockApi(page, {
    '/gallery/getAll': ok(galleries),
    '/photo/getByGallery': ok([photo(10, 'Beach'), photo(11, 'Mountain'), photo(12, 'Portrait')]),
    '/photo-extra/getGlobalMapPhotoByGallery': ({ query }) =>
      ok(
        query.gaId === '7'
          ? [10, 11].map((id) => ({
              photo_id: id,
              photo_name: id === 10 ? 'Beach' : 'Mountain',
              extra_photo_gps_lat: 30,
              extra_photo_gps_lon: 110,
            }))
          : [],
      ),
  })
  await page.goto('/home/GalleryPart')
  await page.getByText('Trip', { exact: true }).click()
  const map = page.getByRole('complementary', { name: 'Gallery map' })
  await expect(map).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('gallery-map.png') })
  await map.getByTitle('2 photos', { exact: true }).click()
  await page.getByRole('button', { name: 'View Beach', exact: true }).click()
  await expect(page.getByRole('dialog', { name: /Beach/ })).toBeVisible()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: /Mountain/ })).toBeVisible()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: /Portrait/ })).toBeVisible()
  await page.keyboard.press('Escape')
  await map.getByRole('button', { name: 'Hide gallery map' }).click()
  await expect(page.getByRole('button', { name: 'Show gallery map' })).toBeVisible()
  await page.getByRole('button', { name: 'Galleries', exact: true }).click()
  await page.getByText('No GPS', { exact: true }).click()
  await expect(map).toBeHidden()
  await page.getByRole('button', { name: 'Galleries', exact: true }).click()
  await page.getByText('Trip', { exact: true }).click()
  await expect(page.getByRole('button', { name: 'Show gallery map' })).toBeVisible()
})

test('Gallery map failures can be retried', async ({ page }) => {
  await signIn(page)
  let fail = true
  await mockApi(page, {
    '/gallery/getAll': ok([{ gaId: 7, gaName: 'Trip', createDate: '2024-06-01' }]),
    '/photo/getByGallery': ok([photo(10, 'Beach')]),
    '/photo-extra/getGlobalMapPhotoByGallery': () =>
      fail
        ? { msgCode: 500 }
        : ok([
            { photo_id: 10, photo_name: 'Beach', extra_photo_gps_lat: 0, extra_photo_gps_lon: 0 },
          ]),
  })
  await page.goto('/home/GalleryPart')
  await page.getByText('Trip', { exact: true }).click()
  await expect(page.getByText('Could not load photo locations.')).toBeVisible()
  fail = false
  await page.getByRole('button', { name: 'Retry map' }).click()
  await expect(
    page.getByRole('complementary', { name: 'Gallery map' }).getByTitle('Beach', { exact: true }),
  ).toBeVisible()
})

test('Map and Tags sidebars resize independently with pointer and keyboard, and retain their widths', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await signIn(page)
  await mockApi(page)
  await page.goto('/home/MapPart')
  const mapHandle = page.getByRole('separator', { name: 'Resize map sidebar' })
  await expect(mapHandle).toHaveAttribute('aria-valuenow', '320')
  await mapHandle.press('ArrowRight')
  await expect(mapHandle).toHaveAttribute('aria-valuenow', '330')
  const box = (await mapHandle.boundingBox())!
  await page.mouse.move(box.x + box.width / 2, box.y + 50)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width / 2 + 70, box.y + 50)
  await page.mouse.up()
  await expect(mapHandle).toHaveAttribute('aria-valuenow', '400')
  await page.reload()
  await expect(mapHandle).toHaveAttribute('aria-valuenow', '400')
  await page.goto('/home/TagPart')
  const tagHandle = page.getByRole('separator', { name: 'Resize tags sidebar' })
  await expect(tagHandle).toHaveAttribute('aria-valuenow', '320')
  await tagHandle.press('End')
  await expect(tagHandle).toHaveAttribute('aria-valuenow', '480')
  await tagHandle.dblclick()
  await expect(tagHandle).toHaveAttribute('aria-valuenow', '320')
  await page.setViewportSize({ width: 800, height: 900 })
  await expect(tagHandle).toHaveAttribute('aria-valuenow', '288')
  await page.setViewportSize({ width: 390, height: 850 })
  await expect(tagHandle).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
