import { expect, test } from '@playwright/test'
import { readFile, mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
const API = 'http://localhost:18384'
const samples = resolve('../photoly_backend/tests/fixtures/ai')
const output = resolve('../docs/verification/similar-scroll')
test.skip(
  process.env.PHOTOLY_REAL_AI !== '1',
  'Explicit real CUDA acceptance only; never target the production API.',
)

test('two real CUDA models through the browser, controls, switching and rollback', async ({
  page,
}) => {
  await mkdir(output, { recursive: true })
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto('/login')
  await page.getByPlaceholder('Email', { exact: true }).fill('admin@test.local')
  await page.getByPlaceholder('Password', { exact: true }).fill('test-password-only')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page).toHaveURL(/HomePart/)
  const token = await page.evaluate(() => localStorage.getItem('HRD-Token'))
  async function api(path: string) {
    const response = await page.request.get(API + path, { headers: { 'HRD-Token': token! } })
    expect(response.status()).toBe(200)
    return (await response.json()).t
  }
  const root = await api('/namespace/getRoot')
  for (const name of ['astronaut.png', 'chelsea.png', 'coffee.png', 'rocket.jpg']) {
    const existing = await api('/photo/getByNamespace?nsId=' + root.nsId)
    if (
      existing.some(
        (p: { photoName: string; format: string }) => `${p.photoName}.${p.format}` === name,
      )
    )
      continue
    const response = await page.request.post(API + '/photo/insert', {
      headers: { 'HRD-Token': token! },
      multipart: {
        file: { name, mimeType: 'image/png', buffer: await readFile(resolve(samples, name)) },
        photo: JSON.stringify({ nsId: root.nsId }),
      },
    })
    expect((await response.json()).msgCode).toBe(200)
  }
  await page.goto('/thepersonincharge/usermanage')
  const toggle = page.getByRole('switch', { name: `AI search for user ${root.userId}` })
  await expect(toggle).toBeEnabled()
  if (!(await toggle.isChecked())) await toggle.click()
  await expect(toggle).toBeChecked()
  await page.goto('/thepersonincharge/ai')
  await expect(page.getByLabel('Inference device').locator('option[value=cuda]')).toBeEnabled({
    timeout: 45000,
  })
  await page.getByLabel('Inference device').selectOption('cuda')
  await page.getByLabel('Enable AI search', { exact: true }).check()
  await page.getByLabel('Minimum background interval (seconds)', { exact: true }).fill('1')
  await page.getByRole('button', { name: 'Save AI settings' }).click()
  await expect.poll(async () => (await api('/admin/ai/settings')).device).toBe('cuda')
  async function operation(button: string, confirm = false) {
    if (
      [
        'Enter maintenance',
        'Exit maintenance',
        'Backfill',
        'Retry failed',
        'Rebuild',
        'Delete profile and vectors',
      ].includes(button) &&
      !(await page.getByRole('button', { name: button, exact: true }).isVisible())
    )
      await page.getByText('Advanced lifecycle controls', { exact: true }).click()
    const accepted = page.waitForResponse(
      (r) => r.url() === API + '/admin/ai/operations' && r.request().method() === 'POST',
    )
    await page.getByRole('button', { name: button, exact: true }).click()
    if (confirm) await page.getByRole('button', { name: 'Confirm operation', exact: true }).click()
    const response = await accepted
    expect(response.status(), await response.text()).toBe(202)
    const job = (await response.json()).t
    const start = Date.now()
    await expect
      .poll(
        async () => {
          const state = await api('/admin/ai/operations/' + job.operationId)
          if (['failed', 'cancelled'].includes(state.status)) throw new Error(JSON.stringify(state))
          return state.status
        },
        { timeout: 180000, intervals: [1000, 2000] },
      )
      .toBe('succeeded')
    await page.getByRole('button', { name: 'Refresh status', exact: true }).click()
    await expect(page.getByText('operation(s) pending.', { exact: false })).toHaveCount(0, {
      timeout: 15000,
    })
    return Date.now() - start
  }
  async function search(mode: 'Natural language' | 'Image search') {
    await page.goto('/home/SearchPart')
    await page.getByRole('combobox', { name: 'Search by' }).click()
    await page.getByRole('option', { name: mode, exact: true }).click()
  }
  const reports: object[] = []
  const ids: number[] = []
  for (const provider of ['siglip2', 'qwen3_vl']) {
    if (provider === 'qwen3_vl') {
      await operation('Enter maintenance', true)
      await operation('Unload model')
    }
    await page.getByLabel('Registered model', { exact: true }).selectOption(provider)
    await page.getByRole('button', { name: 'Create profile', exact: true }).click()
    await expect
      .poll(
        async () =>
          (await api('/admin/ai/profiles')).profiles.find(
            (p: { provider: string }) => p.provider === provider,
          )?.indexStatus,
        { timeout: 30000 },
      )
      .toBe('ready')
    const profile = (await api('/admin/ai/profiles')).profiles.find(
      (p: { provider: string }) => p.provider === provider,
    )
    ids.push(profile.profileId)
    await page.getByRole('button', { name: 'Refresh status', exact: true }).click()
    await page
      .getByLabel('Selected profile', { exact: true })
      .selectOption(String(profile.profileId))
    const loadMs = await operation('Load model')
    if ((await api('/admin/ai/settings')).scheduler === 'stopped')
      await operation('Start scheduler')
    await expect(page.getByRole('button', { name: 'Activate profile', exact: true })).toBeEnabled({
      timeout: 120000,
    })
    await operation('Activate profile')
    const state = await api('/admin/ai/status')
    expect(state.services.runtime.observed.config).toMatchObject({
      device: 'cuda',
      precision: 'bfloat16',
    })
    expect(state.coverage.ready).toBe(4)
    await page.screenshot({ path: resolve(output, provider + '-admin.png'), fullPage: true })
    const queryTimes: number[] = []
    for (const query of ['a cat', '一只猫']) {
      await search('Natural language')
      await page.getByLabel('Describe the image').fill(query)
      const response = page.waitForResponse(
        (r) => r.url() === API + '/ai/search/text' && r.request().method() === 'POST',
      )
      const start = Date.now()
      await page.getByRole('button', { name: 'Search images', exact: true }).click()
      const value = await response
      expect(value.status(), await value.text()).toBe(200)
      expect((await value.json()).t.items).toHaveLength(4)
      queryTimes.push(Date.now() - start)
      await expect(page.getByText('Indexed 4 / 4 supported images', { exact: false })).toBeVisible()
    }
    await page.goto('/home/HomePart')
    const all = await api('/photo/getByNamespace?nsId=' + root.nsId)
    const source = all.find((photo: { photoName: string }) => photo.photoName === 'chelsea')
    let similarCalls = 0
    const count = (request: import('@playwright/test').Request) => {
      if (request.url().endsWith('/similar/page') && request.method() === 'POST') similarCalls++
    }
    page.on('request', count)
    await page.getByText(`${source.photoName}.${source.format}`, { exact: true }).last().click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Load more', exact: true })).toBeEnabled()
    expect(similarCalls).toBe(0)
    const similarResponse = page.waitForResponse(
      (r) =>
        r.url().endsWith(`/ai/photos/${source.photoId}/similar/page`) &&
        r.request().method() === 'POST',
    )
    const similarStart = Date.now()
    await page.getByRole('dialog').hover()
    await page.mouse.wheel(0, 1600)
    const similar = await similarResponse
    expect(similar.status(), await similar.text()).toBe(200)
    const similarHits = (await similar.json()).t.items
    expect(similarHits).toHaveLength(3)
    expect(
      similarHits.every((h: { photo: { photoId: number } }) => h.photo.photoId !== source.photoId),
    ).toBe(true)
    queryTimes.push(Date.now() - similarStart)
    await expect(page.getByText('No more similar images')).toBeVisible()
    await page.screenshot({
      path: resolve(output, provider + '-recommendations.png'),
      fullPage: true,
    })
    const target = similarHits[0].photo
    await page
      .getByRole('button', { name: `View ${target.photoName}.${target.format}`, exact: true })
      .click()
    await expect(
      page.getByText(`Name: ${target.photoName}.${target.format}`, { exact: true }),
    ).toBeVisible()
    await page.getByRole('button', { name: 'Back to previous photo' }).click()
    await expect(
      page.getByRole('list', { name: 'Similar image results' }).getByRole('listitem'),
    ).toHaveCount(3)
    expect(similarCalls).toBe(1)
    page.off('request', count)
    // Exercise a real cursor even though the four-photo browser fixture fits one default page.
    const paged = await page.request.post(API + `/ai/photos/${source.photoId}/similar/page`, {
      headers: { 'HRD-Token': token! },
      data: { pageSize: 1 },
    })
    expect(paged.status()).toBe(200)
    const firstPage = (await paged.json()).t
    const nextPage = await page.request.post(API + `/ai/photos/${source.photoId}/similar/page`, {
      headers: { 'HRD-Token': token! },
      data: { cursor: firstPage.nextCursor },
    })
    expect(nextPage.status()).toBe(200)
    expect((await nextPage.json()).t.items[0].photo.photoId).not.toBe(
      firstPage.items[0].photo.photoId,
    )
    await page.goto('/thepersonincharge/ai')
    await page
      .getByLabel('Selected profile', { exact: true })
      .selectOption(String(profile.profileId))
    await operation('Restart model')
    await operation('Pause scheduler')
    await expect.poll(async () => (await api('/admin/ai/settings')).scheduler).toBe('paused')
    await operation('Resume scheduler')
    await page.getByLabel('Minimum background interval (seconds)', { exact: true }).fill('3')
    await page.getByRole('button', { name: 'Save AI settings', exact: true }).click()
    await expect
      .poll(async () => (await api('/admin/ai/settings')).background_interval_seconds)
      .toBe(3)
    reports.push({
      provider,
      profileId: profile.profileId,
      loadMs,
      queryTimesMs: queryTimes,
      coverage: state.coverage,
      config: state.services.runtime.observed.config,
      resources: state.services.runtime.observed.resources,
    })
  }
  await operation('Enter maintenance', true)
  await operation('Unload model')
  await page.getByLabel('Selected profile', { exact: true }).selectOption(String(ids[0]))
  await operation('Load model')
  await expect(page.getByRole('button', { name: 'Activate profile', exact: true })).toBeEnabled({
    timeout: 30000,
  })
  await operation('Activate profile')
  expect((await api('/admin/ai/settings')).search_profile_id).toBe(ids[0])
  await page.reload()
  await expect(page.getByRole('heading', { name: 'AI Management', exact: true })).toBeVisible()
  expect((await api('/admin/ai/profiles')).profiles).toHaveLength(2)
  expect(errors).toEqual([])
  await writeFile(
    resolve(output, 'cuda-browser.json'),
    JSON.stringify({ reports, rollback: true, browserErrors: errors }, null, 2),
  )
})
