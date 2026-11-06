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
  page.on('pageerror', (e) => {
    // WebKit reports the mocked cross-origin upload poll that navigation aborts as a CORS error.
    if (!/due to access control checks/.test(e.message)) errors.push(e.message)
  })
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
    localStorage.setItem('photoly-map-background', 'gradient')
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
  await expect(page.getByRole('button', { name: 'Globe background', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Day/night', exact: true })).toHaveCount(0)
  expect(await page.evaluate(() => localStorage.getItem('photoly-map-background'))).toBe('gradient')
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

test('day/night defaults on, persists across maps, and keeps markers and heat usable', async ({
  page,
}) => {
  const calls = await setup(page, [located(1, 'Front', 0, 0), located(2, 'East', 60, 0)])
  await page.clock.setFixedTime(new Date('2026-03-20T06:00:00Z'))
  await page.goto('/home/MapPart')
  const map = page.locator('[data-engine]')
  const toggle = page.getByRole('button', { name: 'Day/night', exact: true })
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')
  await expect(map).toHaveAttribute('data-ready', 'true')
  await expect(map).toHaveAttribute('data-day-night-available', 'true')
  await expect(map).toHaveAttribute('data-day-night', 'true')
  const canvas = await map.locator('canvas').elementHandle()
  await toggle.click()
  await expect(map).toHaveAttribute('data-day-night', 'false')
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  expect(await map.locator('canvas').evaluate((node, old) => node === old, canvas)).toBe(true)
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem('photoly-map-day-night')))
    .toBe('false')
  await page.reload()
  await expect(toggle).toHaveAttribute('aria-pressed', 'false')
  await page.goto('/home/GalleryPart')
  await page.getByText('Trip', { exact: true }).click()
  const gallery = page.getByRole('complementary', { name: 'Gallery map' })
  await expect(gallery.getByRole('button', { name: 'Day/night', exact: true })).toHaveAttribute(
    'aria-pressed',
    'false',
  )
  await gallery.getByRole('button', { name: 'Day/night', exact: true }).click()
  await page.goto('/home/MapPart')
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')
  await expect(map).toHaveAttribute('data-day-night', 'true')
  await page.getByRole('radio', { name: 'Heat', exact: true }).click()
  await expect(map).toHaveAttribute('data-heatmap', 'true')
  await page.getByRole('radio', { name: 'Photos', exact: true }).click()
  await page.getByTitle('Front', { exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Front', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Flat map', exact: true }).click()
  await expect(toggle).toHaveCount(0)
  await expect(map).toHaveAttribute('data-day-night', 'false')
  expect(await page.evaluate(() => localStorage.getItem('photoly-map-day-night'))).toBe('true')
  expect(calls.filter((call) => call.path === '/photo-extra/getGlobalMapPhoto')).toHaveLength(3)
})

test('illumination shader failure leaves a working map and preserves the preference', async ({
  page,
}) => {
  await page.addInitScript(() => {
    const original = WebGL2RenderingContext.prototype.shaderSource
    WebGL2RenderingContext.prototype.shaderSource = function (shader, source) {
      original.call(
        this,
        shader,
        source.includes('uniform vec3 u_sun;') ? 'invalid shader' : source,
      )
    }
  })
  await setup(page, [located(1, 'Front', 0, 0)])
  await page.goto('/home/MapPart')
  await expect(page.getByRole('button', { name: 'Day/night', exact: true })).toBeDisabled()
  await expect(page.getByRole('status').filter({ hasText: 'Day/night unavailable' })).toBeVisible()
  await expect(page.locator('[data-engine]')).toHaveAttribute('data-engine', 'maplibre')
  await page.getByTitle('Front', { exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Front', exact: true })).toBeVisible()
  expect(await page.evaluate(() => localStorage.getItem('photoly-map-day-night'))).toBe('true')
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
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.setViewportSize({ width: 1280, height: 900 })
  await setup(page, [located(1, 'Front', 0, 0), located(2, 'Back', 180, 0)])
  await page.goto('/home/MapPart')
  await expect(page.getByTitle('Front', { exact: true })).toBeVisible()
  const map = page.locator('[data-engine]')
  const canvas = map.locator('canvas')
  await expect(map).toHaveAttribute('data-ready', 'true')
  // Keyboard pan rotates the globe without platform-dependent pointer inertia.
  for (let i = 0; i < 20 && !(await page.getByTitle('Back', { exact: true }).count()); i++) {
    const longitude = await map.getAttribute('data-longitude')
    await canvas.press('ArrowRight')
    await expect(map).not.toHaveAttribute('data-longitude', longitude!)
    await expect(map).toHaveAttribute('data-ready', 'true')
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

test('globe backgrounds persist, share across maps and preserve the map and photo requests', async ({
  page,
}, info) => {
  const calls = await setup(page, [located(1, 'Acadia', -68.3, 44.3)])
  await page.goto('/home/MapPart')
  const map = page.locator('[data-engine]')
  await expect(map).toHaveAttribute('data-background', 'stars')
  await expect(map).toHaveAttribute('data-ready', 'true')
  const canvas = await map.locator('canvas').elementHandle()
  const initialZoom = await map.getAttribute('data-zoom')
  for (const [value, label] of [
    ['gradient', 'Dark gradient'],
    ['simple', 'Simple'],
    ['stars', 'Starfield'],
  ] as const) {
    await page.getByRole('button', { name: 'Globe background', exact: true }).click()
    await page.getByRole('menuitemradio', { name: label, exact: true }).click()
    await expect(map).toHaveAttribute('data-background', value)
    expect(await canvas!.evaluate((c) => c.isConnected)).toBe(true)
    expect(await map.getAttribute('data-zoom')).toBe(initialZoom)
    await expect
      .poll(() => page.evaluate(() => localStorage.getItem('photoly-map-background')))
      .toBe(value)
    await page.screenshot({ path: info.outputPath(`background-${value}.png`) })
  }
  expect(calls.filter((c) => c.path === '/photo-extra/getGlobalMapPhoto')).toHaveLength(1)
  await page.getByRole('button', { name: 'Globe background', exact: true }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('menuitemradio', { name: 'Starfield', exact: true })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  await expect(map).toHaveAttribute('data-background', 'gradient')
  await page.reload()
  await expect(map).toHaveAttribute('data-background', 'gradient')
  await page.getByRole('button', { name: 'Flat map', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Globe background', exact: true })).toHaveCount(0)
  await expect.poll(() => map.evaluate((el) => getComputedStyle(el).backgroundImage)).toBe('none')
  await page.getByRole('button', { name: '3D globe', exact: true }).click()
  await page.goto('/home/GalleryPart')
  await page.getByText('Trip', { exact: true }).click()
  const gallery = page.getByRole('complementary', { name: 'Gallery map' })
  await expect(gallery.locator('[data-engine]')).toHaveAttribute('data-background', 'gradient')
  await expect
    .poll(async () => Number(await gallery.locator('[data-engine]').getAttribute('data-zoom')))
    .toBeCloseTo(11)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
for (const items of [
  [located(1, 'Acadia', -68.3, 44.3)],
  [located(1, 'Acadia', -68.3, 44.3), located(2, 'Trail', -68.3, 44.3)],
  [located(1, 'East', 179, 10), located(2, 'West', -179, 10)],
]) {
  test(`Gallery initially fits ${items.map((p) => p.photo_name).join('/')} and can refit after an overview`, async ({
    page,
  }) => {
    const calls = await setup(page, items)
    await page.goto('/home/GalleryPart')
    await page.getByText('Trip', { exact: true }).click()
    const gallery = page.getByRole('complementary', { name: 'Gallery map' })
    const map = gallery.locator('[data-engine]')
    await expect(map).toHaveAttribute('data-projection', 'globe')
    await expect(map).toHaveAttribute('data-ready', 'true')
    await expect.poll(async () => Number(await map.getAttribute('data-zoom'))).toBeGreaterThan(4)
    const fittedZoom = Number(await map.getAttribute('data-zoom'))
    const initialRequests = calls.filter(
      (c) => c.path === '/photo-extra/getGlobalMapPhotoByGallery',
    ).length
    if (items[0]!.photo_name === 'East') {
      expect(Math.abs(Number(await map.getAttribute('data-longitude')))).toBeGreaterThan(170)
    }
    await gallery.getByRole('button', { name: 'Flat map', exact: true }).click()
    await gallery.getByRole('button', { name: '3D globe', exact: true }).click()
    await expect.poll(async () => Number(await map.getAttribute('data-zoom'))).toBeLessThan(3)
    await page.waitForTimeout(650)
    const overviewZoom = await map.getAttribute('data-zoom')
    await gallery.getByRole('button', { name: 'Hide gallery map', exact: true }).click()
    await page.getByRole('button', { name: 'Show gallery map', exact: true }).click()
    expect(await map.getAttribute('data-zoom')).toBe(overviewZoom)
    await gallery.getByRole('button', { name: 'Show all photo locations', exact: true }).click()
    await expect
      .poll(async () => Number(await map.getAttribute('data-zoom')))
      .toBeCloseTo(fittedZoom)
    expect(calls.filter((c) => c.path === '/photo-extra/getGlobalMapPhotoByGallery')).toHaveLength(
      initialRequests,
    )
    await expect(gallery.getByRole('link', { name: 'OpenStreetMap contributors' })).toBeVisible()
  })
}

test('Smart Gallery waits for GPS, retries errors and refits when changing galleries', async ({
  page,
}) => {
  await setup(page)
  await page.route('**/gallery/getAll?*', (r) =>
    r.fulfill({
      json: ok([
        {
          gaId: 7,
          gaName: 'Acadia trip',
          createDate: '2024-06-01',
          smart: {
            id: 1,
            source: 'trip',
            name: 'Acadia trip',
            suppressed: false,
            basis: { gpsCount: 1 },
          },
        },
        { gaId: 8, gaName: 'World trip', createDate: '2024-07-01' },
        { gaId: 9, gaName: 'No GPS', createDate: '2024-08-01' },
      ]),
    }),
  )
  await page.route('**/photo/getByGallery?*', (r) =>
    r.fulfill({ json: ok([photo(1, 'One'), photo(2, 'Two')]) }),
  )
  let fail = true
  let release = () => {}
  const pending = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/photo-extra/getGlobalMapPhotoByGallery?*', async (r) => {
    const id = new URL(r.request().url()).searchParams.get('gaId')
    if (id === '7') await pending
    await r.fulfill({
      json:
        id === '7'
          ? fail
            ? { msgCode: 500 }
            : ok([located(1, 'Acadia', -68.3, 44.3)])
          : ok(id === '8' ? [located(1, 'West', -100, 30), located(2, 'East', 20, 50)] : []),
    })
  })
  await page.goto('/home/GalleryPart')
  await page.getByText('Acadia trip', { exact: true }).click()
  const gallery = page.getByRole('complementary', { name: 'Gallery map' })
  await expect(gallery).toBeHidden()
  release()
  await expect(gallery.getByText('Could not load photo locations.')).toBeVisible()
  fail = false
  await gallery.getByRole('button', { name: 'Retry map', exact: true }).click()
  await expect
    .poll(async () => Number(await gallery.locator('[data-engine]').getAttribute('data-zoom')))
    .toBeCloseTo(11)
  await page.getByRole('button', { name: 'Galleries', exact: true }).click()
  await page.getByText('World trip', { exact: true }).click()
  await expect(gallery.locator('[data-engine]')).toHaveAttribute('data-ready', 'true')
  await expect
    .poll(async () => Number(await gallery.locator('[data-engine]').getAttribute('data-zoom')))
    .toBeLessThan(4)
  await expect(gallery.getByTitle('West', { exact: true })).toBeVisible()
  await expect(gallery.getByTitle('East', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Galleries', exact: true }).click()
  await page.getByText('No GPS', { exact: true }).click()
  await expect(gallery).toBeHidden()
})
