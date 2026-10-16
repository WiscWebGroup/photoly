import { test, expect, type Page } from '@playwright/test'
import { mockApi, ok, signIn, API } from './mockApi'
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
}
async function scrollDown(page: Page) {
  const dialog = page.getByRole('dialog')
  await dialog.evaluate((el) => {
    el.dispatchEvent(new Event('touchmove', { bubbles: true }))
    el.scrollTop = el.scrollHeight
  })
}
test('Search retains text but removes image upload mode', async ({ page }) => {
  await setup(page)
  await page.goto('/home/SearchPart')
  await page.getByRole('combobox', { name: 'Search by' }).click()
  await expect(page.getByRole('option', { name: 'Natural language', exact: true })).toBeVisible()
  await expect(page.getByRole('option', { name: 'Image search', exact: true })).toHaveCount(0)
})
test('scroll pages beyond 100 without inference on open, preserves order and ends', async ({
  page,
}) => {
  const calls = await setup(page)
  await open(page)
  await expect(page.getByRole('button', { name: 'Find similar' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Load more' })).toBeEnabled()
  expect(calls.filter((c) => c.path === path)).toHaveLength(0)
  for (let n = 1; n <= 6; n++) {
    await scrollDown(page)
    await expect(
      page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
    ).toHaveCount(Math.min(24 * n, 127))
  }
  await expect(page.getByText('No more similar images')).toBeVisible()
  expect(calls.filter((c) => c.path === path)).toHaveLength(6)
  const names = await page
    .getByRole('list', { name: 'Similar image results' })
    .getByRole('button')
    .allTextContents()
  expect(names.map((x) => x.trim())).toEqual(items.map((x) => `${x.photo.photoName}.jpg`))
})
test('same dialog navigation updates metadata and back restores recommendations and scroll', async ({
  page,
}) => {
  const calls = await setup(page)
  await open(page)
  await page.getByRole('button', { name: 'Load more' }).click()
  const thumb = page.getByRole('button', { name: 'View Similar 1.jpg', exact: true })
  await thumb.scrollIntoViewIfNeeded()
  const scroll = await page.getByRole('dialog').evaluate((el) => el.scrollTop)
  await thumb.click()
  await expect(page.getByRole('dialog')).toHaveCount(1)
  await expect(page.getByText('Name: Similar 1.jpg', { exact: true })).toBeVisible()
  await expect(page.getByText('Size: 2 MB', { exact: true })).toBeVisible()
  await expect(page.getByText('Path: /root/Other folder', { exact: true })).toBeVisible()
  expect(calls.filter((c) => c.path === '/ai/photos/100/similar/page')).toHaveLength(0)
  await page.getByRole('button', { name: 'Back to previous photo' }).click()
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await expect
    .poll(() => page.getByRole('dialog').evaluate((el) => el.scrollTop))
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
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByRole('alert')).toContainText('timed out')
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
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
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
  ).toHaveCount(24)
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByRole('alert')).toContainText('expired')
  await page.getByRole('button', { name: 'Reload recommendations' }).click()
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
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
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByText('Loading similar images…')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.waitForTimeout(750)
  await page.getByText('Original.jpg', { exact: true }).click()
  await expect(page.getByRole('button', { name: 'Load more' })).toBeEnabled()
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
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
  await page.getByRole('button', { name: 'Load more' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText('No more similar images')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.route(API + '/ai/capabilities', (route) =>
    route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: ok({ ...capabilitiesFixture, ready: false, reason: 'no_active_profile' }),
    }),
  )
  await page.getByText('Original.jpg', { exact: true }).click()
  await expect(page.getByText('No search index has been activated yet.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Download', exact: true })).toBeVisible()
  await scrollDown(page)
  expect(calls.filter((c) => c.path === path)).toHaveLength(0)
})

test('tall viewport still has a scrollable recommendation entry', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1200 })
  const calls = await setup(page)
  await open(page)
  await expect(page.getByRole('button', { name: 'Load more' })).toBeEnabled()
  expect(calls.filter((c) => c.path === path)).toHaveLength(0)
  expect(
    await page.getByRole('dialog').evaluate((el) => el.scrollHeight - el.clientHeight),
  ).toBeGreaterThan(48)
  await page.getByRole('dialog').hover()
  await page.mouse.wheel(0, 1600)
  await expect(
    page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
  ).toHaveCount(24)
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
  const results = page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem')
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(results).toHaveCount(24)
  await page.getByRole('button', { name: 'Load more' }).click()
  await expect(page.getByRole('alert')).toContainText('threshold changed')
  await expect(results).toHaveCount(0)
  await page.getByRole('button', { name: 'Reload recommendations' }).click()
  await expect(results).toHaveCount(2)
  await expect(page.getByText('Minimum similarity: 0.995')).toBeVisible()
  await expect(page.getByText('No more similar images')).toBeVisible()
})

test('capability polling removes cached results after a threshold change', async ({ page }) => {
  await setup(page)
  await page.clock.install()
  await open(page)
  await page.getByRole('button', { name: 'Load more' }).click()
  const results = page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem')
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
