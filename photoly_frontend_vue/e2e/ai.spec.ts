import { expect, test, type Page } from '@playwright/test'
import { API, mockApi, ok, signIn } from './mockApi'
import {
  capabilitiesFixture,
  adminCapabilitiesFixture,
  statusFixture,
  profilesFixture,
} from '../src/test/aiFixtures'

const photos = [
  { photoId: 9, photoName: 'Relevant cat', format: 'jpg', nsId: 1 },
  { photoId: 2, photoName: 'Second result', format: 'png', nsId: 1 },
]
const response = {
  items: photos.map((photo, i) => ({ photo, score: 0.9 - i / 10 })),
  profileId: 1,
  coverage: capabilitiesFixture.coverage,
  requestId: 'query-test',
}
async function mode(page: Page, name = 'Natural language') {
  await page.goto('/home/SearchPart')
  await page.getByRole('combobox', { name: 'Search by' }).click()
  await page.getByRole('option', { name, exact: true }).click()
}
async function searchMocks(page: Page) {
  return mockApi(page, {
    '/ai/capabilities': ok(capabilitiesFixture),
    '/ai/search/text': ok(response),
    '/ai/search/image': ok(response),
    '/photo/getByNamespace': ok(photos),
    '/ai/photos/9/similar': ok(response),
  })
}
test.beforeEach(async ({ page }) => {
  await signIn(page)
})
test('text search preserves ranking, shows coverage and sends only on submit', async ({ page }) => {
  const calls = await searchMocks(page)
  await mode(page)
  await page.getByLabel('Describe the image').fill('阳光下的猫')
  expect(calls.filter((c) => c.path === '/ai/search/text')).toHaveLength(0)
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByText('Coverage is partial.', { exact: false })).toBeVisible()
  await expect(page.getByText('Request: query-test', { exact: false })).toBeVisible()
  expect(calls.find((c) => c.path === '/ai/search/text')?.body).toEqual({
    text: '阳光下的猫',
    topK: 20,
  })
  await expect(page.getByRole('region', { name: 'AI search' }).getByRole('listitem')).toHaveText([
    'Relevant cat.jpg',
    'Second result.png',
  ])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
test('AI access denial does not log out the user', async ({ page }) => {
  await searchMocks(page)
  await page.route(`${API}/ai/search/text`, (route) =>
    route.fulfill({
      status: 403,
      headers: { 'access-control-allow-origin': '*' },
      json: { msgCode: 403, error: 'ai_not_enabled', t: null },
    }),
  )
  await mode(page)
  await page.getByLabel('Describe the image').fill('cat')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('not enabled')
  expect(await page.evaluate(() => localStorage.getItem('HRD-Token'))).toBe('e2e-token')
  await expect(page).toHaveURL(/SearchPart/)
})
test('busy, timeout and empty results are distinct', async ({ page }) => {
  await searchMocks(page)
  let attempt = 0
  await page.route(`${API}/ai/search/text`, (route) => {
    attempt++
    return route.fulfill({
      status: attempt === 1 ? 429 : attempt === 2 ? 504 : 200,
      headers: { 'access-control-allow-origin': '*' },
      json:
        attempt < 3
          ? {
              msgCode: attempt === 1 ? 429 : 504,
              t: null,
              error: attempt === 1 ? 'user_query_busy' : 'query_timeout',
            }
          : ok({ ...response, items: [] }),
    })
  })
  await mode(page)
  await page.getByLabel('Describe the image').fill('cat')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('previous query')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('timed out')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByText('No matching indexed photos and videos were returned.')).toBeVisible()
})
test('cancelled late query does not replace newer results', async ({ page }) => {
  await searchMocks(page)
  await page.route(`${API}/ai/search/text`, async (route) => {
    const old = route.request().postDataJSON().text === 'old'
    if (old) await new Promise((resolve) => setTimeout(resolve, 800))
    await route
      .fulfill({
        headers: { 'access-control-allow-origin': '*' },
        json: ok({ ...response, requestId: old ? 'late-old' : 'new-query' }),
      })
      .catch(() => {})
  })
  await mode(page)
  await page.getByLabel('Describe the image').fill('old')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await page.getByRole('button', { name: 'Cancel waiting' }).click()
  await page.getByLabel('Describe the image').fill('new')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByText('Request: new-query', { exact: false })).toBeVisible()
  await page.waitForTimeout(900)
  await expect(page.getByText('Request: late-old', { exact: false })).toHaveCount(0)
})
async function adminMocks(page: Page) {
  const state = structuredClone(statusFixture)
  const operations: object[] = []
  const calls = await mockApi(page, {
    '/admin/isAdmin': ok(true),
    '/admin/ai/status': () => ok(state),
    '/admin/ai/capabilities': ok(adminCapabilitiesFixture),
    '/admin/ai/profiles': ok(profilesFixture),
    '/admin/ai/jobs': (c) =>
      ok({ items: c.query.status ? [] : operations, nextAfter: null, nextBefore: null }),
    '/admin/ai/settings': (c) => {
      const data = c.body as { generation: number; settings: object }
      Object.assign(state.settings, data.settings, { generation: data.generation + 1 })
      return ok(state.settings)
    },
    '/admin/ai/operations': (c) => {
      const data = c.body as { command: string; generation: number }
      state.settings.generation++
      const job = {
        operationId: 1,
        command: data.command,
        kind: 'operation',
        status: 'queued',
        generation: state.settings.generation,
        stage: 'queued',
        attempts: 0,
        result: {},
        createdAt: new Date().toISOString(),
        finishedAt: null,
      }
      operations.push(job)
      return { msgCode: 202, t: job }
    },
  })
  return { calls, state, operations }
}
test('admin limits validate pairs, save only changes and display read-only concurrency', async ({
  page,
}) => {
  const { calls } = await adminMocks(page)
  await page.goto('/thepersonincharge/ai')
  await expect(page.getByRole('heading', { name: 'AI Management', exact: true })).toBeVisible()
  await page.getByLabel('CPU resume (%)', { exact: true }).fill('95')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect(page.getByRole('alert')).toContainText('must be')
  expect(calls.some((c) => c.path === '/admin/ai/settings')).toBe(false)
  await page.getByLabel('CPU resume (%)', { exact: true }).fill('70')
  await page.getByLabel('Minimum background interval (seconds)', { exact: true }).fill('12')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect
    .poll(() => calls.find((c) => c.path === '/admin/ai/settings')?.body)
    .toEqual({ generation: 0, settings: { background_interval_seconds: 12 } })
  await expect(page.getByText('Concurrency 1 · Batch 1 (read-only)')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
test('queued operations remain pending and survive page refresh', async ({ page }) => {
  const { calls } = await adminMocks(page)
  await page.goto('/thepersonincharge/ai')
  await page.getByRole('button', { name: 'Load model', exact: true }).click()
  await expect(page.getByText('1 operation(s) pending.', { exact: false })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Load model', exact: true })).toBeDisabled()
  await page.reload()
  await expect(page.getByText('1 operation(s) pending.', { exact: false })).toBeVisible()
  expect(calls.filter((c) => c.path === '/admin/ai/operations')).toHaveLength(1)
})
test('generation conflict preserves the draft until explicitly reloaded', async ({ page }) => {
  const { state } = await adminMocks(page)
  await page.route(`${API}/admin/ai/settings`, (route) => {
    state.settings.generation = 2
    state.settings.background_interval_seconds = 7
    return route.fulfill({
      status: 409,
      headers: { 'access-control-allow-origin': '*' },
      json: { t: null, msgCode: 409, error: 'generation_conflict' },
    })
  })
  await page.goto('/thepersonincharge/ai')
  await page.getByLabel('Minimum background interval (seconds)', { exact: true }).fill('12')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect(page.getByRole('alert')).toContainText('another session')
  await expect(
    page.getByLabel('Minimum background interval (seconds)', { exact: true }),
  ).toHaveValue('12')
  await page.getByRole('button', { name: 'Reload saved settings (discard draft)' }).click()
  await expect(
    page.getByLabel('Minimum background interval (seconds)', { exact: true }),
  ).toHaveValue('7')
})
test('user AI switch uses one batch read and rolls back a failed update', async ({ page }) => {
  const calls = await mockApi(page, {
    '/admin/isAdmin': ok(true),
    '/admin/getUserPage': ok(
      [{ userId: 3, userName: 'alice', email: 'alice@test', role: 'user', uuid: 'test-user' }],
      { pageNum: 1 },
    ),
    '/admin/ai/users': ok([{ userId: 3, enabled: false }]),
  })
  await page.route(`${API}/admin/users/3/ai`, (route) =>
    route.fulfill({
      status: 503,
      headers: { 'access-control-allow-origin': '*' },
      json: { t: null, msgCode: 503, error: 'network_error' },
    }),
  )
  await page.goto('/thepersonincharge/usermanage')
  const toggle = page.getByRole('switch', { name: 'AI search for user 3' })
  await toggle.click()
  await expect(page.getByRole('alert')).toContainText('could not be reached')
  await expect(toggle).not.toBeChecked()
  expect(calls.filter((c) => c.path === '/admin/ai/users')).toHaveLength(1)
})

test('unknown operation outcome is recovered after reload with the same idempotency key', async ({
  page,
}) => {
  const { operations } = await adminMocks(page)
  const keys: string[] = []
  await page.route(`${API}/admin/ai/operations`, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    keys.push(route.request().headers()['idempotency-key']!)
    if (keys.length === 1) return route.abort('failed')
    const body = route.request().postDataJSON()
    const job = {
      operationId: 8,
      command: body.command,
      generation: 1,
      kind: 'operation',
      status: 'queued',
      stage: 'queued',
      attempts: 0,
      result: {},
      createdAt: new Date().toISOString(),
      finishedAt: null,
    }
    operations.push(job)
    await route.fulfill({
      status: 202,
      headers: { 'access-control-allow-origin': '*' },
      json: { msgCode: 202, t: job },
    })
  })
  await page.goto('/thepersonincharge/ai')
  await page.getByRole('button', { name: 'Load model', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('outcome unknown')
  await page.reload()
  await expect(page.getByText('1 operation(s) pending.', { exact: false })).toBeVisible()
  expect(keys).toHaveLength(2)
  expect(keys[0]).toBeTruthy()
  expect(keys[1]).toBe(keys[0])
})
test('account switch clears prior search results', async ({ page }) => {
  await searchMocks(page)
  await mode(page)
  await page.getByLabel('Describe the image').fill('cat')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByText('Request: query-test', { exact: false })).toBeVisible()
  await page.evaluate(() => {
    localStorage.setItem('HRD-Token', 'another-account')
    window.dispatchEvent(
      new StorageEvent('storage', { key: 'HRD-Token', newValue: 'another-account' }),
    )
  })
  await expect(page.getByText('Request: query-test', { exact: false })).toHaveCount(0)
  await expect(page.getByLabel('Describe the image')).toHaveValue('')
})
test('disabled AI and offline services remain distinct from a broken library', async ({ page }) => {
  await mockApi(page, {
    '/ai/capabilities': ok({ ...capabilitiesFixture, ready: false, reason: 'maintenance' }),
  })
  await mode(page)
  await expect(
    page.getByText('AI search is temporarily unavailable during model maintenance.'),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Search images', exact: true })).toBeDisabled()
  const { state } = await adminMocks(page)
  state.services.runtime.online = false
  await page.goto('/thepersonincharge/ai')
  await expect(page.getByText('Offline / unknown', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Load model', exact: true })).toBeDisabled()
})

test('admin similarity threshold defaults to blank, persists decimals and can be cleared', async ({
  page,
}) => {
  const { calls, state } = await adminMocks(page)
  await page.goto('/thepersonincharge/ai')
  const field = page.getByLabel('Similarity threshold (optional)')
  await expect(field).toHaveValue('')
  await field.fill('1.01')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  expect(calls.filter((c) => c.path === '/admin/ai/settings')).toHaveLength(0)
  await field.fill('0.825')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect.poll(() => state.settings.similarity_threshold).toBe(0.825)
  expect(calls.find((c) => c.path === '/admin/ai/settings')?.body).toEqual({
    generation: 0,
    settings: { similarity_threshold: 0.825 },
  })
  await page.reload()
  await expect(field).toHaveValue('0.825')
  await field.fill('-0.2')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect.poll(() => state.settings.similarity_threshold).toBe(-0.2)
  await page.getByRole('button', { name: 'No threshold', exact: true }).click()
  await expect(field).toHaveValue('')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect.poll(() => state.settings.similarity_threshold).toBeNull()
  expect(calls.filter((c) => c.path === '/admin/ai/settings').at(-1)?.body).toEqual({
    generation: 2,
    settings: { similarity_threshold: null },
  })
})

test('Apple Silicon device saves MPS float32 and unavailable devices stay disabled', async ({
  page,
}) => {
  const { calls, state } = await adminMocks(page)
  state.settings.device = 'cuda'
  state.settings.precision = 'bfloat16'
  await page.route(`${API}/admin/ai/capabilities`, (route) =>
    route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: ok({
        ...adminCapabilitiesFixture,
        devices: [
          { device: 'cpu', precision: 'float32', available: true, reason: null },
          { device: 'cuda', precision: 'bfloat16', available: false, reason: 'device_unavailable' },
          { device: 'mps', precision: 'float32', available: true, reason: null },
        ],
      }),
    }),
  )
  await page.goto('/thepersonincharge/ai')
  await page.getByLabel('Inference device').selectOption('mps')
  await expect(page.getByLabel('Inference device')).toHaveValue('mps')
  await expect(
    page.getByRole('option', { name: 'Apple Silicon GPU (MPS) / float32' }),
  ).toBeAttached()
  await expect(
    page.getByLabel('Inference device').locator('option[value="cuda"]'),
  ).toHaveJSProperty('disabled', true)
  await page.getByRole('button', { name: 'Save AI settings', exact: true }).click()
  await expect
    .poll(() => calls.filter((call) => call.path === '/admin/ai/settings').length)
    .toBeGreaterThan(0)
  expect(calls.find((call) => call.path === '/admin/ai/settings')?.body).toMatchObject({
    settings: { device: 'mps', precision: 'float32' },
  })
})

test('LAN HTTP loads controls, polls and recovers operations without secure-context crypto', async ({
  page,
}) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.addInitScript(() => {
    Object.defineProperty(crypto, 'subtle', { value: undefined, configurable: true })
    Object.defineProperty(crypto, 'randomUUID', { value: undefined, configurable: true })
  })
  const { calls, state, operations } = await adminMocks(page)
  const keys: string[] = []
  await page.route(`${API}/admin/ai/operations`, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    keys.push(route.request().headers()['idempotency-key']!)
    if (keys.length === 1) return route.abort('failed')
    const job = {
      operationId: 9,
      command: 'model.load',
      kind: 'operation',
      status: 'queued',
      generation: 1,
      stage: 'queued',
      attempts: 0,
      result: {},
      createdAt: new Date().toISOString(),
      finishedAt: null,
    }
    operations.push(job)
    await route.fulfill({
      status: 202,
      headers: { 'access-control-allow-origin': '*' },
      json: { msgCode: 202, t: job },
    })
  })
  await page.goto('/thepersonincharge/ai')
  await expect(page.getByRole('heading', { name: 'Index maintenance service' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Load model', exact: true })).toBeEnabled()
  state.settings.generation = 7
  await expect(page.getByText(/Configuration generation 7/)).toBeVisible({ timeout: 12000 })
  expect(calls.filter((c) => c.path === '/admin/ai/status').length).toBeGreaterThan(1)
  await page.getByRole('button', { name: 'Load model', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('outcome unknown')
  await page.reload()
  await expect(page.getByText('1 operation(s) pending.', { exact: false })).toBeVisible()
  expect(keys).toHaveLength(2)
  expect(keys[0]).toMatch(/^[a-f0-9]{32}$/)
  expect(keys[1]).toBe(keys[0])
  expect(errors).toEqual([])
})
