/** Public documentation captures: synthetic fixtures only, no live backend access. */
import { readFileSync, mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'
import { API, mockApi, ok, photo, signIn } from './mockApi'

test.skip(process.env.PHOTOLY_DOCS_SCREENSHOTS !== '1', 'Run pnpm docs:screenshots explicitly')
test.use({
  viewport: { width: 1440, height: 1000 },
  locale: 'en-US',
  timezoneId: 'UTC',
  colorScheme: 'light',
  reducedMotion: 'reduce',
})
const output = fileURLToPath(new URL('../../docs/screenshots/', import.meta.url))
const landscape = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><rect width="1200" height="800" fill="#dceeed"/><circle cx="940" cy="150" r="75" fill="#e9be74"/><path d="M0 550 300 180 550 500 790 290 1200 570V800H0Z" fill="#789c9a"/><path d="M0 650 330 440 630 610 970 400 1200 570V800H0Z" fill="#386d6c"/><path d="M0 700Q300 560 650 710T1200 680V800H0Z" fill="#173f48"/></svg>`
// Intentionally schematic: captions explicitly identify these as synthetic map tiles.
const tile = `<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256"><rect width="256" height="256" fill="#dce8df"/><path d="M0 50Q80 10 150 110T256 210V256H0Z" fill="#b6d9df"/><path d="M0 90 256 190M60 0 180 256M0 230 256 30" stroke="#fffaf0" stroke-width="8"/><path d="M0 90 256 190M60 0 180 256M0 230 256 30" stroke="#d6c8b6" stroke-width="2"/></svg>`
const media = [photo(1, 'Mountain study'), photo(2, 'Coastal light'), photo(3, 'Forest shapes')]
const root = { nsId: 1, nsName: 'Library', sourceId: 1, sourceType: 'local' }
const trip = {
  nsId: 2,
  nsName: 'Sample journeys',
  sourceId: 1,
  sourceType: 'local',
  path: 'Library / Sample journeys',
}
const description =
  'A collection of illustrated landscapes.\nMountains, coastlines and quiet moments — sample content for the Photoly guide.'

async function raster(page: Page, svg: string) {
  const data = await page.evaluate(async (markup) => {
    const image = new Image()
    image.src = 'data:image/svg+xml,' + encodeURIComponent(markup)
    await image.decode()
    const canvas = document.createElement('canvas')
    canvas.width = image.width
    canvas.height = image.height
    canvas.getContext('2d')!.drawImage(image, 0, 0)
    return canvas.toDataURL('image/png').split(',')[1]!
  }, svg)
  return Buffer.from(data, 'base64')
}
async function setup(page: Page, overrides: Parameters<typeof mockApi>[1] = {}) {
  // Block every non-local request unless a synthetic fixture below handles it.
  await page.route('**/*', (route) => {
    const url = new URL(route.request().url())
    return url.origin === 'http://localhost:5173' && !url.pathname.startsWith('/api/')
      ? route.continue()
      : route.abort()
  })
  await signIn(page)
  await page.addInitScript(() => {
    localStorage.setItem('photoly-map-background', 'gradient')
    localStorage.setItem('photoly-map-projection', 'globe')
  })
  await mockApi(page, {
    '/user/getInfo': ok({
      userId: 1,
      userName: 'Demo reader',
      email: 'reader@example.test',
      role: 'user',
    }),
    '/namespace/getRoot': ok(root),
    '/namespace/getChildren': ok([trip]),
    '/namespace/trace': ok([root]),
    '/namespace/getNameById': ok('Library'),
    '/photo/getByNamespace': ok(media),
    '/photo/getCovers': ok([]),
    ...overrides,
  })
  const image = await raster(page, landscape)
  const mapTile = await raster(page, tile)
  await page.route(/https:\/\/(?:[abc]\.)?tile\.openstreetmap\.org\//, (route) =>
    route.fulfill({
      contentType: 'image/png',
      headers: { 'access-control-allow-origin': '*' },
      body: mapTile,
    }),
  )
  await page.route(`${API}/photo/render*/**`, (route) =>
    route.fulfill({
      contentType: 'image/png',
      headers: { 'access-control-allow-origin': '*' },
      body: image,
    }),
  )
}
async function capture(page: Page, name: string) {
  mkdirSync(output, { recursive: true })
  await page.evaluate(() => document.fonts.ready)
  await expect(page.locator('[data-sonner-toast]')).toHaveCount(0)
  await expect
    .poll(() =>
      page
        .locator('[data-photo-id] img')
        .evaluateAll((images) =>
          images.every(
            (img) =>
              (img as HTMLImageElement).complete && (img as HTMLImageElement).naturalWidth > 0,
          ),
        ),
    )
    .toBe(true)
  await page.screenshot({ path: `${output}${name}.png`, animations: 'disabled' })
}

test('pinned folders and default location', async ({ page }) => {
  await setup(page, {
    '/namespace/pins': ok({ items: [trip], defaultNsId: 2 }),
    '/namespace/trace': ok([root, trip]),
    '/namespace/getChildren': ok([]),
  })
  await page.goto('/home/HomePart')
  await expect(page.getByRole('heading', { name: 'Sample journeys', exact: true })).toBeVisible()
  const pin = page.locator('#main-sidebar').getByLabel('Pinned folders').getByRole('link')
  await expect(pin.getByLabel('Default location')).toBeVisible()
  await pin.click({ button: 'right' })
  await expect(page.getByRole('menu')).toBeVisible()
  await capture(page, 'pinned-default-location')
})

test('gallery description editor and detail', async ({ page }) => {
  const gallery = {
    gaId: 7,
    gaName: 'Illustrated journeys',
    description,
    coverId: 9,
    coverColor: '#16827d',
    createDate: '2026-01-15T12:00:00',
  }
  await setup(page, {
    '/gallery/getAll': ok([gallery]),
    '/photo/getByGallery': ok(media),
    '/photo-extra/getGlobalMapPhotoByGallery': ok([]),
  })
  await page.goto('/home/GalleryPart')
  await page.getByText('Illustrated journeys', { exact: true }).click()
  await expect(page.getByTestId('gallery-description')).toHaveText(description)
  await capture(page, 'gallery-description-detail')
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  await expect(page.getByRole('dialog').getByLabel('Description', { exact: true })).toHaveValue(
    description,
  )
  await capture(page, 'gallery-description-editor')
})

test('globe and date controls', async ({ page }) => {
  const places = [
    {
      photo_id: 1,
      photo_name: 'Sample coast',
      extra_photo_gps_lon: 0,
      extra_photo_gps_lat: 0,
      extra_photo_taken_date: '2026-01-15',
      format: 'jpg',
      ns_id: 1,
    },
    {
      photo_id: 2,
      photo_name: 'Sample island',
      extra_photo_gps_lon: 25,
      extra_photo_gps_lat: 15,
      extra_photo_taken_date: '2026-02-15',
      format: 'jpg',
      ns_id: 1,
    },
  ]
  await setup(page, {
    '/photo-extra/getGlobalMapPhoto': ok(places),
    '/photo-extra/getMapPlaces': ok(
      places.map((p) => ({ photoId: p.photo_id, place: p.photo_name, country: 'Demo' })),
    ),
  })
  await page.goto('/home/MapPart')
  await expect(page.locator('[data-engine]')).toHaveAttribute('data-ready', 'true')
  await expect(page.getByRole('button', { name: '3D globe', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(page.getByRole('link', { name: 'OpenStreetMap contributors' })).toBeVisible()
  await capture(page, 'map-globe')
})

test('viewer location section', async ({ page }) => {
  await setup(page, { '/photo/getPhotoExif': ok({ GPS_Latitude: '0', GPS_Longitude: '0' }) })
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click()
  const map = page.getByRole('region', { name: 'Photo location map' })
  await map.scrollIntoViewIfNeeded()
  await expect(map.locator('.leaflet-tile-loaded').first()).toBeVisible()
  await expect(page.getByRole('link', { name: 'Open full map', exact: true })).toBeVisible()
  await capture(page, 'viewer-location')
})

test('video player with synthetic test pattern', async ({ page }) => {
  await setup(page, { '/photo/getByNamespace': ok([photo(10, 'Video sample', { format: 'mp4' })]) })
  const bytes = readFileSync(fileURLToPath(new URL('./fixtures/player.mp4', import.meta.url)))
  await page.route(`${API}/photo/renderV/**`, (route) => {
    const range = route
      .request()
      .headers()
      .range?.match(/bytes=(\d+)-(\d*)/)
    const start = range ? Number(range[1]) : 0
    const end = range?.[2] ? Math.min(Number(range[2]), bytes.length - 1) : bytes.length - 1
    return route.fulfill({
      status: range ? 206 : 200,
      contentType: 'video/mp4',
      headers: {
        'access-control-allow-origin': '*',
        'accept-ranges': 'bytes',
        ...(range ? { 'content-range': `bytes ${start}-${end}/${bytes.length}` } : {}),
      },
      body: bytes.subarray(start, end + 1),
    })
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Video Video sample.mp4', exact: true }).click()
  const player = page.getByRole('region', { name: 'Video player' })
  const video = player.locator('video')
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState))
    .toBeGreaterThanOrEqual(2)
  await video.evaluate(async (v: HTMLVideoElement) => {
    v.muted = true
    await v.play()
    v.currentTime = 5
  })
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.seeking)).toBe(false)
  await video.evaluate((v: HTMLVideoElement) => v.pause())
  await player.hover()
  await capture(page, 'video-player')
})

test('maintenance and backup with empty password fields', async ({ page }) => {
  await setup(page, {
    '/admin/isAdmin': ok(true),
    '/admin/maintenance': ok({
      state: { mode: 'on' },
      problems: [],
      backupDirectory: '/backups',
      rollbacks: [],
      externalSources: [],
    }),
    '/admin/backups': ok([
      {
        id: '00000000-0000-4000-8000-000000000001',
        createdAt: '2026-01-15T12:00:00Z',
        size: 104857600,
        complete: true,
      },
    ]),
  })
  await page.goto('/thepersonincharge/maintenance')
  await expect(page.getByRole('heading', { name: 'Maintenance and Backup' })).toBeVisible()
  await expect(page.getByLabel('Backup password', { exact: true })).toHaveValue('')
  await capture(page, 'maintenance-backup')
})
