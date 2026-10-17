import { test, expect, type Page } from '@playwright/test'
import { mockApi, ok, signIn, API, openDetails } from './mockApi'
import { capabilitiesFixture } from '../src/test/aiFixtures'
const source = { photoId: 9, photoName: 'Original', format: 'jpg', nsId: 1 }
const items = Array.from({ length: 127 }, (_, i) => ({
  photo: { photoId: i + 100, photoName: `Similar ${i + 1}`, format: 'jpg', nsId: 2 },
  score: 1 - i / 200,
}))
const path = '/ai/photos/9/similar/page'
async function setup(page: Page) {
  await signIn(page)
  return mockApi(page, {
    '/ai/capabilities': ok(capabilitiesFixture),
    '/photo/getByNamespace': ok([source]),
    [path]: (call) => {
      const offset = Number((call.body as { cursor?: string }).cursor ?? 0)
      return ok({
        items: items.slice(offset, offset + 24),
        nextCursor: offset + 24 < items.length ? String(offset + 24) : null,
        hasMore: offset + 24 < items.length,
        profileId: 1,
        coverage: capabilitiesFixture.coverage,
        requestId: 'page-test',
      })
    },
    '/namespace/trace': (call) =>
      ok([
        { nsId: 1, nsName: 'root' },
        {
          nsId: Number(call.query.nsId),
          nsName: call.query.nsId === '2' ? 'Other folder' : 'Original folder',
        },
      ]),
    '/photo/getPhotoExif': (call) =>
      ok({ File_Size: call.query.photoId === '9' ? '1 MB' : '2 MB' }),
  })
}
async function open(page: Page) {
  await page.goto('/home/HomePart')
  await page.getByText('Original.jpg', { exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await showDetails(page)
}
const showDetails = openDetails
const findSimilar = (page: Page) => page.getByRole('button', { name: 'Find similar' })
test('Search retains text but removes image upload mode', async ({ page }) => {
  await setup(page)
  await page.goto('/home/SearchPart')
  await page.getByRole('combobox', { name: 'Search by' }).click()
  await expect(page.getByRole('option', { name: 'Natural language', exact: true })).toBeVisible()
  await expect(page.getByRole('option', { name: 'Image search', exact: true })).toHaveCount(0)
})
test('pages beyond 100 only after Find similar, preserves order and ends', async ({ page }) => {
  const calls = await setup(page)
  await open(page)
  await expect(findSimilar(page)).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Load more' })).toHaveCount(0)
  expect(calls.filter((c) => c.path === path)).toHaveLength(0)
  await findSimilar(page).click()
  for (let n = 1; n <= 6; n++) {
    await expect(
      page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
    ).toHaveCount(Math.min(24 * n, 127))
    if (n < 6) await page.getByRole('button', { name: 'Load more' }).click()
  }
  await expect(page.getByText('No more similar photos and videos')).toBeVisible()
  expect(calls.filter((c) => c.path === path)).toHaveLength(6)
  const names = await page
    .getByRole('list', { name: 'Similar photos and videos results' })
    .getByRole('button')
    .allTextContents()
  expect(names.map((x) => x.trim())).toEqual(items.map((x) => `${x.photo.photoName}.jpg`))
})
test('same dialog navigation updates metadata and back restores recommendations and scroll', async ({
  page,
}) => {
  const calls = await setup(page)
  await open(page)
  await findSimilar(page).click()
  const thumb = page.getByRole('button', { name: 'View Similar 1.jpg', exact: true })
  await thumb.scrollIntoViewIfNeeded()
  const scroll = await page.getByTestId('photo-viewer-scroll').evaluate((el) => el.scrollTop)
  await thumb.click()
  await expect(page.getByRole('dialog')).toHaveCount(1)
  await expect(
    page.getByTestId('photo-viewer-scroll').getByRole('heading', { name: 'Similar 1.jpg' }),
  ).toBeVisible()
  await showDetails(page)
  await expect(page.getByText('2 MB')).toBeVisible()
  await expect(page.getByText('/root/Other folder', { exact: true })).toBeVisible()
  expect(calls.filter((c) => c.path === '/ai/photos/100/similar/page')).toHaveLength(0)
  await page.getByRole('button', { name: 'Back to previous photo' }).click()
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await expect
    .poll(() => page.getByTestId('photo-viewer-scroll').evaluate((el) => el.scrollTop))
    .toBeGreaterThan(scroll - 5)
  expect(calls.filter((c) => c.path === path)).toHaveLength(1)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByText('Original.jpg', { exact: true })).toBeVisible()
})
test('failed next page preserves results and retries the same cursor', async ({ page }) => {
  await setup(page)
  let fail = true
  const cursors: string[] = []
  await page.route(API + path, (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    const cursor = route.request().postDataJSON().cursor
    if (!cursor) return route.fallback()
    cursors.push(cursor)
    const failed = fail
    fail = false
    return route.fulfill({
      status: failed ? 504 : 200,
      headers: { 'access-control-allow-origin': '*' },
      json: failed
        ? { msgCode: 504, t: null, error: 'query_timeout' }
        : ok({
            items: items.slice(24, 48),
            nextCursor: null,
            hasMore: false,
            profileId: 1,
            coverage: capabilitiesFixture.coverage,
            requestId: 'retry',
          }),
    })
  })
  await open(page)
  await findSimilar(page).click()
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByRole('alert')).toContainText('timed out')
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(48)
  expect(cursors).toEqual(['24', '24'])
})

test('expired cursor offers restart and never appends a different context', async ({ page }) => {
  const calls = await setup(page)
  await page.route(API + path, (route) => {
    if (route.request().method() !== 'POST' || !route.request().postDataJSON().cursor)
      return route.fallback()
    return route.fulfill({
      status: 409,
      headers: { 'access-control-allow-origin': '*' },
      json: { msgCode: 409, t: null, error: 'cursor_expired' },
    })
  })
  await open(page)
  await findSimilar(page).click()
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByRole('alert')).toContainText('expired')
  await page.getByRole('button', { name: 'Reload recommendations' }).click()
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(24)
  expect(calls.filter((c) => c.path === path).map((c) => c.body)).toEqual([
    { pageSize: 24 },
    { pageSize: 24 },
  ])
})
test('closing during a query discards late results and reopening starts fresh', async ({
  page,
}) => {
  await setup(page)
  let requests = 0
  await page.route(API + path, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    requests++
    await new Promise((resolve) => setTimeout(resolve, 650))
    await route
      .fulfill({
        headers: { 'access-control-allow-origin': '*' },
        json: ok({
          items: items.slice(0, 24),
          nextCursor: '24',
          hasMore: true,
          profileId: 1,
          coverage: capabilitiesFixture.coverage,
          requestId: 'late',
        }),
      })
      .catch(() => {})
  })
  await open(page)
  await findSimilar(page).click()
  await expect(page.getByText('Loading similar photos and videos…')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.waitForTimeout(750)
  await page.getByText('Original.jpg', { exact: true }).click()
  await showDetails(page)
  await expect(findSimilar(page)).toBeEnabled()
  await expect(
    page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem'),
  ).toHaveCount(0)
  expect(requests).toBe(1)
})
test('keyboard loading handles empty results and unavailable indexing keeps the image usable', async ({
  page,
}) => {
  const calls = await setup(page)
  await page.route(API + path, (route) =>
    route.request().method() !== 'POST'
      ? route.fallback()
      : route.fulfill({
          headers: { 'access-control-allow-origin': '*' },
          json: ok({
            items: [],
            nextCursor: null,
            hasMore: false,
            profileId: 1,
            coverage: capabilitiesFixture.coverage,
            requestId: 'empty',
          }),
        }),
  )
  await open(page)
  await findSimilar(page).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('No more similar photos and videos')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.route(API + '/ai/capabilities', (route) =>
    route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: ok({ ...capabilitiesFixture, ready: false, reason: 'no_active_profile' }),
    }),
  )
  await page.getByText('Original.jpg', { exact: true }).click()
  await showDetails(page)
  await expect(findSimilar(page)).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Download', exact: true })).toBeVisible()
  expect(calls.filter((c) => c.path === path)).toHaveLength(0)
})

test('changed threshold clears old recommendations and reloads a new context', async ({ page }) => {
  await setup(page)
  let changed = false
  await page.route(API + path, (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    if (route.request().postDataJSON().cursor) {
      changed = true
      return route.fulfill({
        status: 409,
        headers: { 'access-control-allow-origin': '*' },
        json: { t: null, msgCode: 409, error: 'search_settings_changed' },
      })
    }
    if (!changed) return route.fallback()
    return route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: ok({
        items: items.slice(0, 2),
        nextCursor: null,
        hasMore: false,
        profileId: 1,
        coverage: capabilitiesFixture.coverage,
        requestId: 'threshold',
        similarityThreshold: 0.995,
      }),
    })
  })
  await open(page)
  const results = page
    .getByRole('list', { name: 'Similar photos and videos results' })
    .getByRole('listitem')
  await findSimilar(page).click()
  await expect(results).toHaveCount(24)
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByRole('alert')).toContainText('threshold changed')
  await expect(results).toHaveCount(0)
  await page.getByRole('button', { name: 'Reload recommendations' }).click()
  await expect(results).toHaveCount(2)
  await expect(page.getByText('No more similar photos and videos')).toBeVisible()
})

test('capability polling removes cached results after a threshold change', async ({ page }) => {
  await setup(page)
  await page.clock.install()
  await open(page)
  await findSimilar(page).click()
  const results = page
    .getByRole('list', { name: 'Similar photos and videos results' })
    .getByRole('listitem')
  await expect(results).toHaveCount(24)
  await page.route(API + '/ai/capabilities', (route) =>
    route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: ok({ ...capabilitiesFixture, similarityThreshold: 0.9 }),
    }),
  )
  await page.clock.fastForward(16000)
  await expect(results).toHaveCount(0)
  await expect(page.getByRole('alert')).toContainText('threshold changed')
  await expect(page.getByRole('button', { name: 'Reload recommendations' })).toBeVisible()
})

const backToTop = (page: Page) =>
  page.getByRole('button', { name: 'Back to top and close similar photos and videos' })
const results = (page: Page) =>
  page.getByRole('list', { name: 'Similar photos and videos results' }).getByRole('listitem')
const scrollPosition = (page: Page) =>
  page.getByTestId('photo-viewer-scroll').evaluate((el) => el.scrollTop)

async function closeViewer(page: Page) {
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
}
async function reopenOriginal(page: Page) {
  await page.getByText('Original.jpg', { exact: true }).click()
  await showDetails(page)
  await expect(page.getByRole('dialog')).toBeVisible()
}

test('each photo keeps its own recommendations across closing and reopening', async ({ page }) => {
  const calls = await setup(page)
  await page.route(API + '/photo/getByNamespace*', (route) =>
    route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: ok([source, { ...source, photoId: 10, photoName: 'Another' }]),
    }),
  )
  await open(page)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
  await closeViewer(page)
  await page.getByText('Another.jpg', { exact: true }).click()
  await expect(results(page)).toHaveCount(0)
  await expect(backToTop(page)).toHaveCount(0)
  await expect.poll(() => scrollPosition(page)).toBe(0)
  expect(calls.filter((c) => c.path === '/ai/photos/10/similar/page')).toHaveLength(0)
  await closeViewer(page)
  await reopenOriginal(page)
  await expect(results(page)).toHaveCount(24)
  expect(calls.filter((c) => c.path === path)).toHaveLength(1)
})

test('retention starts on leaving, renews after returning, and expires after 30 seconds', async ({
  page,
}) => {
  const calls = await setup(page)
  await page.clock.install()
  await open(page)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
  await page.clock.fastForward(31000)
  await expect(results(page)).toHaveCount(24)
  await closeViewer(page)
  await page.clock.fastForward(28000)
  await reopenOriginal(page)
  await expect(results(page)).toHaveCount(24)
  await page.clock.fastForward(31000)
  await expect(results(page)).toHaveCount(24)
  await closeViewer(page)
  await page.clock.fastForward(30000)
  await reopenOriginal(page)
  await expect(results(page)).toHaveCount(0)
  await expect.poll(() => scrollPosition(page)).toBe(0)
  expect(calls.filter((c) => c.path === path)).toHaveLength(1)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
  expect(calls.filter((c) => c.path === path)).toHaveLength(2)
})

test('returning through history after expiry does not restore results or scroll', async ({
  page,
}) => {
  const calls = await setup(page)
  await page.clock.install()
  await open(page)
  await findSimilar(page).click()
  await page.getByRole('button', { name: 'View Similar 1.jpg', exact: true }).click()
  await expect(
    page.getByTestId('photo-viewer-scroll').getByRole('heading', { name: 'Similar 1.jpg' }),
  ).toBeVisible()
  await page.clock.fastForward(30000)
  await page.getByRole('button', { name: 'Back to previous photo' }).click()
  await expect(results(page)).toHaveCount(0)
  await expect.poll(() => scrollPosition(page)).toBe(0)
  expect(calls.filter((c) => c.path === path)).toHaveLength(1)
  await showDetails(page) // going back collapses the details again
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
})

test('floating back-to-top stays visible, collapses results and offers Find similar again', async ({
  page,
}) => {
  const calls = await setup(page)
  await open(page)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
  await expect(findSimilar(page)).toHaveCount(0)
  await page.getByTestId('photo-viewer-scroll').evaluate((el) => {
    el.scrollTop = el.scrollHeight
  })
  await expect(backToTop(page)).toBeInViewport()
  const dialogBox = await page.getByRole('dialog').boundingBox()
  const buttonBox = await backToTop(page).boundingBox()
  expect(buttonBox!.x + buttonBox!.width).toBeLessThanOrEqual(dialogBox!.x + dialogBox!.width)
  expect(buttonBox!.y + buttonBox!.height).toBeLessThanOrEqual(dialogBox!.y + dialogBox!.height)
  await backToTop(page).click()
  await expect(results(page)).toHaveCount(0)
  await expect(backToTop(page)).toHaveCount(0)
  await expect.poll(() => scrollPosition(page)).toBe(0)
  await expect(findSimilar(page)).toBeEnabled()
  await closeViewer(page)
  await reopenOriginal(page)
  await expect(results(page)).toHaveCount(0)
  expect(calls.filter((c) => c.path === path)).toHaveLength(1)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
  expect(calls.filter((c) => c.path === path)).toHaveLength(2)
})

test('back-to-top during a request discards late results and allows a new query', async ({
  page,
}) => {
  await setup(page)
  let release!: () => void
  const pending = new Promise<void>((resolve) => {
    release = resolve
  })
  let requests = 0
  await page.route(API + path, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    requests++
    if (requests === 1) await pending
    await route
      .fulfill({
        headers: { 'access-control-allow-origin': '*' },
        json: ok({
          items: items.slice(0, 24),
          nextCursor: '24',
          hasMore: true,
          profileId: 1,
          coverage: capabilitiesFixture.coverage,
          requestId: 'delayed',
        }),
      })
      .catch(() => {})
  })
  await open(page)
  await findSimilar(page).click()
  await expect(page.getByText('Loading similar photos and videos…')).toBeVisible()
  await backToTop(page).click()
  release()
  await expect(results(page)).toHaveCount(0)
  await expect.poll(() => scrollPosition(page)).toBe(0)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(24)
  expect(requests).toBe(2)
})

test('video viewer loads mixed recommendations, restores history and collapses at top', async ({
  page,
}) => {
  await signIn(page)
  const video = { ...source, format: 'mp4' }
  const hits = [
    items[0]!,
    { ...items[1]!, photo: { ...items[1]!.photo, format: 'mp4' }, matchedTimestampMs: 2500 },
  ]
  const calls = await mockApi(page, {
    '/ai/capabilities': ok({
      ...capabilitiesFixture,
      formats: [...capabilitiesFixture.formats, 'mp4'],
    }),
    '/photo/getByNamespace': ok([video]),
    [path]: ok({
      items: hits,
      nextCursor: null,
      hasMore: false,
      profileId: 1,
      coverage: capabilitiesFixture.coverage,
      requestId: 'video',
    }),
  })
  await page.goto('/home/HomePart')
  await page.getByText('Original.mp4', { exact: true }).click()
  await showDetails(page)
  await expect(findSimilar(page)).toBeEnabled()
  await expect(page.getByRole('dialog').locator('video')).toHaveCount(1)
  expect(calls.filter((c) => c.path === path)).toHaveLength(0)
  await findSimilar(page).click()
  await expect(results(page)).toHaveCount(2)
  await page.getByRole('button', { name: 'View Similar 2.mp4', exact: true }).click()
  await expect(
    page.getByTestId('photo-viewer-scroll').getByRole('heading', { name: 'Similar 2.mp4' }),
  ).toBeVisible()
  await expect(results(page)).toHaveCount(0)
  await page.getByRole('button', { name: 'Back to previous photo' }).click()
  await expect(results(page)).toHaveCount(2)
  await backToTop(page).click()
  await expect(results(page)).toHaveCount(0)
  await expect
    .poll(() => page.getByTestId('photo-viewer-scroll').evaluate((el) => el.scrollTop))
    .toBe(0)
  expect(calls.filter((c) => c.path === path)).toHaveLength(1)
})
