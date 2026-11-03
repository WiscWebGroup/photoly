import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'
const located = (id: number, name: string, lng: number, lat: number, date = '2024-06-01') => ({
  photo_id: id,
  photo_name: name,
  extra_photo_gps_lon: lng,
  extra_photo_gps_lat: lat,
  extra_photo_taken_date: date,
  format: 'jpg',
  ns_id: 1,
})
const photos = [
  located(1, 'Front', 0, 0),
  located(2, 'Back', 180, 0),
  located(3, 'Pacific', 179, 10, '2025-07-01'),
]
async function setup(page: Page, items = photos) {
  await signIn(page)
  return mockApi(page, {
    '/photo-extra/getGlobalMapPhoto': ok(items),
    '/photo-extra/getMapPlaces': ok(
      items.map((p) => ({ photoId: p.photo_id, place: p.photo_name, country: 'Test' })),
    ),
    '/gallery/getAll': ok([{ gaId: 7, gaName: 'Trip', createDate: '2024-06-01' }]),
    '/photo/getByGallery': ok(items.map((p) => photo(p.photo_id, p.photo_name))),
    '/photo-extra/getGlobalMapPhotoByGallery': ok(items),
  })
}
test('defaults to a globe, hides the far hemisphere and remembers a shared projection', async ({
  page,
}, info) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  const calls = await setup(page)
  await page.goto('/home/MapPart')
  const map = page.locator('[data-engine]')
  await expect(map).toHaveAttribute('data-engine', 'maplibre')
  await expect(page.getByRole('button', { name: '3D globe', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  await expect(page.getByTitle('Back', { exact: true })).toHaveCount(0)
  await expect(page.getByTitle('Pacific', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'OpenStreetMap contributors' })).toBeVisible()
  await page.screenshot({ path: info.outputPath('globe.png') })
  await page.getByRole('button', { name: 'Flat map', exact: true }).click()
  await expect(map).toHaveAttribute('data-projection', 'mercator')
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem('photoly-map-projection')))
    .toBe('mercator')
  expect(calls.filter((c) => c.path === '/photo-extra/getGlobalMapPhoto')).toHaveLength(1)
  await expect(map).toHaveAttribute('data-ready', 'true')
  await page.reload()
  await expect(map).toHaveAttribute('data-projection', 'mercator')
  await expect(map).toHaveAttribute('data-ready', 'true')
  await page.goto('/home/GalleryPart')
  await page.getByText('Trip', { exact: true }).click()
  const gallery = page.getByRole('complementary', { name: 'Gallery map' })
  await expect(gallery.locator('[data-engine]')).toHaveAttribute('data-projection', 'mercator')
  await gallery.getByRole('button', { name: '3D globe', exact: true }).click()
  await expect(gallery.locator('[data-engine]')).toHaveAttribute('data-ready', 'true')
  await page.goto('/home/MapPart')
  await expect(map).toHaveAttribute('data-projection', 'globe')
  expect(errors).toEqual([])
})
test('direct photo links zoom in; globe switching pulls back and restores the flat camera', async ({
  page,
}) => {
  await setup(page, [located(1, 'Front', 0, 0)])
  await page.addInitScript(() => localStorage.setItem('photoly-map-projection', 'mercator'))
  await page.goto('/home/MapPart?photo=1')
  const map = page.locator('[data-engine]')
  await expect.poll(async () => Number(await map.getAttribute('data-zoom'))).toBeCloseTo(12)
  await expect(page.getByRole('heading', { name: 'Front', exact: true })).toBeVisible()
  await page.getByRole('button', { name: '3D globe', exact: true }).click()
  await expect.poll(async () => Number(await map.getAttribute('data-zoom'))).toBeLessThan(3)
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Flat map', exact: true }).click()
  await expect.poll(async () => Number(await map.getAttribute('data-zoom'))).toBeCloseTo(12)
})
test('date filtering drives photos, heatmap and visible places in both projections', async ({
  page,
}, info) => {
  await setup(page, [located(1, 'Old', 0, 0), located(2, 'New', 50, 0, '2025-07-01')])
  await page.goto('/home/MapPart')
  for (const projection of ['3D globe', 'Flat map']) {
    await page.getByRole('button', { name: projection, exact: true }).click()
    await page.getByRole('button', { name: /Date range:/ }).click()
    await page.getByRole('radio', { name: '2025', exact: true }).click()
    await page.getByRole('button', { name: 'Show 1 Photo', exact: true }).click()
    await expect(page.getByTitle('Old', { exact: true })).toHaveCount(0)
    await expect(page.getByTitle('New', { exact: true })).toBeVisible()
    await page.getByRole('radio', { name: 'Heat', exact: true }).click()
    await expect(page.locator('[data-engine]')).toHaveAttribute('data-heatmap', 'true')
    await expect(page.locator('.photo-map-marker')).toHaveCount(0)
    await page.screenshot({ path: info.outputPath(`heat-${projection}.png`) })
    await page.getByRole('radio', { name: 'Photos', exact: true }).click()
    await expect(page.getByTitle('New', { exact: true })).toBeVisible()
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
test('WebGL failure keeps a functional flat map without overwriting the saved globe choice', async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem('photoly-map-projection', 'globe')
    const original = HTMLCanvasElement.prototype.getContext
    HTMLCanvasElement.prototype.getContext = function (
      this: HTMLCanvasElement,
      ...args: Parameters<typeof original>
    ) {
      if (String(args[0]).includes('webgl')) return null
      return Reflect.apply(original, this, args)
    } as typeof original
  })
  await setup(page, [located(1, 'Front', 0, 0)])
  await page.goto('/home/MapPart')
  await expect(page.locator('[data-engine]')).toHaveAttribute('data-engine', 'leaflet')
  await expect(page.getByRole('button', { name: '3D globe', exact: true })).toBeDisabled()
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  await page.getByTitle('Front', { exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Front', exact: true })).toBeVisible()
  await page.getByRole('radio', { name: 'Heat', exact: true }).click()
  const pixels = await page
    .locator('.leaflet-overlay-pane canvas')
    .evaluate((c: HTMLCanvasElement) =>
      c
        .getContext('2d')!
        .getImageData(0, 0, c.width, c.height)
        .data.some((v, i) => i % 4 === 3 && v > 0),
    )
  expect(pixels).toBe(true)
  expect(await page.evaluate(() => localStorage.getItem('photoly-map-projection'))).toBe('globe')
  await expect(page.getByRole('link', { name: 'OpenStreetMap contributors' })).toBeVisible()
})
test('context loss falls back and tile errors do not prevent viewing photos', async ({ page }) => {
  await setup(page, [located(1, 'Front', 0, 0)])
  await page.route('https://tile.openstreetmap.org/**', (r) => r.abort())
  await page.goto('/home/MapPart')
  const map = page.locator('[data-engine]')
  await expect(map).toHaveAttribute('data-engine', 'maplibre')
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  await map
    .locator('canvas')
    .evaluate((c) => c.dispatchEvent(new Event('webglcontextlost', { cancelable: true })))
  await expect(map).toHaveAttribute('data-engine', 'leaflet')
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
})
test('dense libraries render only visible cluster covers and no-location maps remain usable', async ({
  page,
}) => {
  const items = Array.from({ length: 5000 }, (_, i) => located(i + 1, `Photo ${i}`, 0, 0))
  let thumbnailRequests = 0
  page.on('request', (request) => {
    if (request.url().includes('/photo/renderThumbnail/')) thumbnailRequests++
  })
  await setup(page, items)
  await page.goto('/home/MapPart')
  await expect(page.getByTitle('5000 photos', { exact: true })).toBeVisible()
  await expect(page.locator('.photo-map-marker')).toHaveCount(1)
  expect(thumbnailRequests).toBeLessThan(10)
  await page.route('**/photo-extra/getGlobalMapPhoto**', (r) => r.fulfill({ json: ok([]) }))
  await page.reload()
  await expect(page.locator('[data-engine]')).toHaveAttribute('data-engine', 'maplibre')
  await expect(page.getByText('No photos have a location yet.', { exact: true })).toHaveCount(1)
  await expect(page.locator('.photo-map-marker')).toHaveCount(0)
})

test('rotating the globe updates visible photos and the Places in view list', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await setup(page, [located(1, 'Front', 0, 0), located(2, 'Back', 180, 0)])
  await page.goto('/home/MapPart')
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  const canvas = page.locator('[data-engine] canvas')
  // Keyboard pan rotates the globe without platform-dependent pointer inertia.
  for (let i = 0; i < 20 && !(await page.getByTitle('Back', { exact: true }).count()); i++) {
    await canvas.press('ArrowRight')
    await page.waitForTimeout(350)
  }
  await expect(page.getByTitle('Back', { exact: true })).toBeVisible()
  await expect(page.getByTitle('Front', { exact: true })).toHaveCount(0)
  const places = page.getByRole('region', { name: 'Places in view' })
  await expect(places.getByText('Back', { exact: true })).toBeVisible()
  await expect(places.getByText('Front', { exact: true })).toHaveCount(0)
  await places.getByRole('button', { name: /Back/ }).click()
  await expect
    .poll(async () => Number(await page.locator('[data-engine]').getAttribute('data-zoom')))
    .toBeGreaterThanOrEqual(10)
  await expect(page.getByRole('heading', { name: 'Back', exact: true })).toBeVisible()
})
test('Pacific photos fit across the date line, and failed location requests can be retried', async ({
  page,
}) => {
  await setup(page, [located(1, 'East', 179, 10), located(2, 'West', -179, 10)])
  let fail = true
  await page.route('**/photo-extra/getGlobalMapPhoto**', (r) =>
    r.fulfill({
      json: fail
        ? { msgCode: 500 }
        : ok([located(1, 'East', 179, 10), located(2, 'West', -179, 10)]),
    }),
  )
  await page.goto('/home/MapPart')
  await expect(page.getByText('Could not load photo locations.', { exact: true })).toBeVisible()
  fail = false
  await page.getByRole('button', { name: 'Retry map', exact: true }).click()
  await expect(page.getByTitle('2 photos', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Flat map', exact: true }).click()
  await expect(page.getByTitle('East', { exact: true })).toBeVisible()
  await expect(page.getByTitle('West', { exact: true })).toBeVisible()
  const lng = Number(await page.locator('[data-engine]').getAttribute('data-longitude'))
  expect(Math.abs(lng)).toBeGreaterThan(170)
})
