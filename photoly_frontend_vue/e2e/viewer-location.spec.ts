import { expect, test } from '@playwright/test'
import { API, PNG, mockApi, ok, photo, signIn } from './mockApi'

test.beforeEach(async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/photo/getByNamespace': ok([photo(1, 'Boston'), photo(2, 'Zero'), photo(3, 'No GPS')]),
    '/photo/getPhotoExif': ({ query }) =>
      ok(
        query.photoId === '1'
          ? { GPS_Latitude: '42.2153', GPS_Longitude: '-71.5359' }
          : query.photoId === '2'
            ? { GPS_Latitude: '0', GPS_Longitude: '0' }
            : {},
      ),
  })
  await page.route(`${API}/photo/render/**`, (route) =>
    route.fulfill({
      contentType: 'image/svg+xml',
      headers: { 'access-control-allow-origin': '*' },
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><rect width="1200" height="800" fill="#348888"/></svg>',
    }),
  )
})

test('Location sits between Notes and Category, stays 2D and remembers folding for this viewing session', async ({
  page,
}, testInfo) => {
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click()
  const section = page.locator('[data-location-section]')
  const map = page.getByRole('region', { name: 'Photo location map' })
  await expect(page.getByRole('link', { name: 'Open full map', exact: true })).toHaveAttribute(
    'href',
    '/home/MapPart?photo=1',
  )
  await expect(section).toHaveAttribute('open', '')
  expect(
    await section.evaluate((el) => {
      const notes = document.querySelector('[aria-label="Notes"]')!
      const category = [...document.querySelectorAll('summary')].find(
        (s) => s.textContent === 'Category',
      )!
      return (
        !!(notes.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING) &&
        !!(el.compareDocumentPosition(category) & Node.DOCUMENT_POSITION_FOLLOWING)
      )
    }),
  ).toBe(true)
  await map.scrollIntoViewIfNeeded()
  await expect(map.locator('.leaflet-tile-loaded').first()).toBeVisible()
  await expect(map.getByTitle('Photo location')).toBeVisible()
  expect((await map.boundingBox())!.height).toBeGreaterThanOrEqual(198)
  await expect(map.locator('canvas')).toHaveCount(0)
  await map.focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: 'Boston.jpg', exact: true })).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('location.png') })
  await section.locator('summary').focus()
  await page.keyboard.press('i')
  await expect(page.locator('[data-location-map] .leaflet-pane')).toHaveCount(0)
  await page.keyboard.press('i')
  await map.scrollIntoViewIfNeeded()
  await expect(map.getByTitle('Photo location')).toBeVisible()
  await section.locator('summary').click()
  await expect(map.locator('.leaflet-pane')).toHaveCount(0)
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: 'Zero.jpg', exact: true })).toBeVisible()
  await expect(section).not.toHaveAttribute('open')
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: 'No GPS.jpg', exact: true })).toBeVisible()
  await expect(section).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Open full map' })).toHaveCount(0)
  await page.keyboard.press('ArrowLeft')
  await expect(section).not.toHaveAttribute('open')
  await section.locator('summary').click()
  await map.scrollIntoViewIfNeeded()
  await expect(map.getByTitle('Photo location')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.locator('[data-photo-id="1"]').click()
  await expect(section).toHaveAttribute('open', '')
})

test('offscreen maps wait; tile failures retry and a pending map never blocks closing', async ({
  page,
}) => {
  let tiles = 0
  let fail = true
  await page.route('https://tile.openstreetmap.org/**', (route) => {
    tiles++
    return fail ? route.abort() : route.fulfill({ body: PNG, contentType: 'image/png' })
  })
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click()
  const map = page.getByRole('region', { name: 'Photo location map' })
  // Keep Location offscreen regardless of desktop/mobile viewport height.
  await page.locator('[data-location-section]').evaluate((el) => {
    ;(el as HTMLElement).style.marginTop = '1500px'
  })
  await expect(page.locator('[data-photo-zoom]').filter({ visible: true })).toBeVisible()
  await page.waitForTimeout(600)
  expect(tiles).toBe(0)
  await map.scrollIntoViewIfNeeded()
  await expect(page.getByText('Could not load the map.', { exact: true })).toBeVisible()
  fail = false
  await page.getByRole('button', { name: 'Retry map' }).click()
  await expect(map.locator('.leaflet-tile-loaded').first()).toBeVisible()
  await expect(page.getByText('Could not load the map.', { exact: true })).toHaveCount(0)
  await page.locator('[data-location-section] summary').click()
  await page.route('https://tile.openstreetmap.org/**', () => {})
  await page.locator('[data-location-section] summary').click()
  await map.scrollIntoViewIfNeeded()
  await expect(map.getByTitle('Photo location')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
})

test('map initialization waits for the photo, including a failed photo opening', async ({
  page,
}) => {
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route(`${API}/photo/render/**`, async (route) => {
    await gate
    await route.abort()
  })
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click()
  const map = page.getByRole('region', { name: 'Photo location map' })
  await map.scrollIntoViewIfNeeded()
  await expect(map.locator('.leaflet-pane')).toHaveCount(0)
  release()
  await expect(map.getByTitle('Photo location')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
})
