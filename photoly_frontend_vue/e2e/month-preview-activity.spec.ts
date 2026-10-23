import { expect, test } from '@playwright/test'
import { API, mockApi, ok, signIn } from './mockApi'

test('collapsed months show bounded thumbnail previews without loading their photo pages', async ({
  page,
}, testInfo) => {
  await signIn(page)
  const calls = await mockApi(page, {
    '/photo/getDateSummary': ok({
      totalCount: 32,
      undatedCount: 0,
      days: [
        { date: '2025-06-01', count: 20 },
        { date: '2025-05-01', count: 12 },
      ],
    }),
    '/photo/getMonthPreview': ok(Array.from({ length: 10 }, (_, i) => i + 1)),
    '/photo/getByDate': ok({ items: [], nextCursor: null }),
  })
  await page.goto('/home/DatesPart')
  const month = page.getByRole('button', { name: 'May 2025, 12 photos', exact: true })
  await month.scrollIntoViewIfNeeded()
  await expect(month.locator('img')).toHaveCount(10)
  await expect(month).toHaveAttribute('aria-expanded', 'false')
  expect(
    calls.filter((c) => c.path === '/photo/getByDate').every((c) => c.query.day !== '2025-05-01'),
  ).toBe(true)
  expect(
    calls.filter((c) => c.path === '/photo/getMonthPreview' && c.query.month === '2025-05'),
  ).toHaveLength(1)
  const overflow = await month.evaluate((e) => e.scrollWidth > e.clientWidth)
  expect(overflow).toBe(false)
  await page.screenshot({ path: testInfo.outputPath('month-previews.png'), fullPage: true })
})

test('failure alerts can be dismissed and finished activity cleared while active tasks remain', async ({
  page,
}) => {
  await signIn(page)
  await mockApi(page)
  let failedCount = 1
  let finished = true
  const batch = (id: string, state: string) => ({
    id,
    name: id,
    nsId: 1,
    sealed: true,
    cancelled: false,
    createdAt: '2026-01-01',
    counts: { [state]: { count: 1, bytes: 100 } },
  })
  await page.route(`${API}/uploads**`, async (route) => {
    const req = route.request()
    if (req.method() === 'OPTIONS')
      return route.fulfill({
        status: 204,
        headers: {
          'access-control-allow-origin': '*',
          'access-control-allow-methods': '*',
          'access-control-allow-headers': '*',
        },
      })
    if (req.url().includes('acknowledge-failures')) {
      failedCount = 0
      return route.fulfill({ json: ok(1), headers: { 'access-control-allow-origin': '*' } })
    }
    if (req.method() === 'DELETE') {
      finished = false
      failedCount = 0
      return route.fulfill({
        json: ok(['Failed import']),
        headers: { 'access-control-allow-origin': '*' },
      })
    }
    const batches = [
      ...(finished ? [batch('Failed import', 'failed')] : []),
      batch('Active import', 'processing'),
    ]
    await route.fulfill({
      json: ok({ batches, total: batches.length, maxFileBytes: 1e6, failedCount, activeCount: 1 }),
      headers: { 'access-control-allow-origin': '*' },
    })
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await page.getByRole('button', { name: 'Dismiss failure alerts' }).click()
  await expect(page.getByRole('button', { name: 'Dismiss failure alerts' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible()
  await page.reload()
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await expect(page.getByRole('button', { name: 'Dismiss failure alerts' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Clear finished' }).click()
  await expect(page.getByRole('button', { name: 'Failed import', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Active import', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Cancel remaining', exact: true })).toBeVisible()
  await page.reload()
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await expect(page.getByRole('button', { name: 'Failed import', exact: true })).toHaveCount(0)
})
