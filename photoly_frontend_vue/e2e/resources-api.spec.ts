import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'
test('admin resources, runtime and history are independent of AI', async ({ page }) => {
  await signIn(page)
  const now = new Date().toISOString()
  const point = {
    bootId: 'test',
    bucket: now,
    firstSampleAt: now,
    lastSampleAt: now,
    cpuSamples: 1,
    memorySamples: 1,
    cpuMean: 35,
    cpuPeak: 48,
    backendMeanBytes: 1024 ** 3,
    aiMeanBytes: 0,
    memoryPeakBytes: 1024 ** 3,
    partial: true,
  }
  const calls = await mockApi(page, {
    '/user/getInfo': ok({ userId: 1, role: 'admin', userName: 'Admin' }),
    '/admin/getAddress': ok({ upload: '/photos', config: '/config' }),
    '/admin/getSettings': ok({
      OnlineDownload: '0',
      SignUp: '0',
      SSafeUUID: '1',
      TokenDuration: 7,
    }),
    '/admin/databaseInfo': ok({ version: '17', schemaRevision: '16' }),
    '/admin/pingDatabase': ok(1),
    '/admin/resources/current': ok({
      sampledAt: now,
      cpuPercent: 35,
      backendBytes: 1024 ** 3,
      aiBytes: 0,
    }),
    '/admin/resources/history': ok({ items: [point] }),
    '/admin/runtime': ok({ startedAt: now, uptimeSeconds: 90061, bootId: 'test' }),
  })
  await page.goto('/thepersonincharge/dashboard')
  await expect(page.getByText(/Uptime/)).toContainText('1d 1h')
  const resources = page.getByRole('region', { name: 'Resource', exact: true })
  await expect(resources.getByRole('img', { name: 'CPU 35.0%' })).toBeVisible()
  await expect(resources.getByRole('img', { name: 'Memory 1.00 GB' })).toBeVisible()
  await resources.getByRole('button', { name: '12 hours' }).click()
  await expect
    .poll(() => calls.filter((c) => c.path === '/admin/resources/history').at(-1)?.query.hours)
    .toBe('12')
  await resources.getByText(/History data/).click()
  await expect(resources.getByRole('table')).toContainText('35.0% / 48.0%')
  await page.screenshot({ path: test.info().outputPath('resources.png') })
})
test('API catalog copies prefixed addresses and scoped commands with fallback', async ({
  page,
}) => {
  await signIn(page)
  await mockApi(page, {
    '/user/getInfo': ok({ userId: 1, role: 'user', userName: 'Alice' }),
    '/cred/query': ok([{ credId: 9, token: 'sample-token', authorization: 'R' }]),
  })
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: async () => {
          throw new Error('unavailable')
        },
      },
      configurable: true,
    })
  })
  await page.goto('/home/ManagePart#api')
  await page.getByText('API documents', { exact: true }).click()
  const list = page.getByRole('article', { name: 'List media', exact: true })
  await list.getByRole('button', { name: 'Copy cURL' }).click()
  await expect(page.getByLabel('Select and copy')).toHaveValue(/Authorization: Bearer YOUR_TOKEN/)
  await page.getByLabel('Token for examples').selectOption('9')
  await list.getByRole('button', { name: 'Copy cURL' }).click()
  await expect(page.getByLabel('Select and copy')).toHaveValue(/Authorization: Bearer sample-token/)
  await expect(
    page
      .getByRole('article', { name: 'Delete a media file' })
      .getByRole('button', { name: 'Copy cURL' }),
  ).toBeDisabled()
  await list.getByRole('button', { name: 'Copy address' }).click()
  await expect(page.getByLabel('Select and copy')).toHaveValue(/\/cred\/media$/)
  await page.screenshot({ path: test.info().outputPath('api-examples.png') })
})
