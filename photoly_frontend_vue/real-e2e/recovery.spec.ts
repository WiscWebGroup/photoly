import { expect, test } from '@playwright/test'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
test.skip(
  process.env.PHOTOLY_REAL_RECOVERY !== '1',
  'Explicit post-restart CUDA browser verification only.',
)
test('restored services retain profiles, vectors, CUDA settings and UI search', async ({
  page,
}) => {
  await page.goto('/login')
  await page.getByPlaceholder('Email', { exact: true }).fill('admin@test.local')
  await page.getByPlaceholder('Password', { exact: true }).fill('test-password-only')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page).toHaveURL(/HomePart/)
  const token = await page.evaluate(() => localStorage.getItem('HRD-Token'))
  const api = async (path: string) =>
    (
      await (
        await page.request.get('http://localhost:18384' + path, {
          headers: { 'HRD-Token': token! },
        })
      ).json()
    ).t
  await expect
    .poll(async () => (await api('/ai/capabilities')).ready, { timeout: 180000, intervals: [2000] })
    .toBe(true)
  const profiles = await api('/admin/ai/profiles')
  const status = await api('/admin/ai/status')
  expect(profiles.profiles).toHaveLength(2)
  expect(
    profiles.profiles.every((p: { coverage: { ready: number } }) => p.coverage.ready === 4),
  ).toBe(true)
  expect(status.settings).toMatchObject({
    device: 'cuda',
    precision: 'bfloat16',
    background_interval_seconds: 3,
    enabled: true,
  })
  expect(status.services.runtime.observed.config).toMatchObject({
    device: 'cuda',
    precision: 'bfloat16',
  })
  await page.goto('/home/SearchPart')
  await page.getByRole('combobox', { name: 'Search by' }).click()
  await page.getByRole('option', { name: 'Natural language', exact: true }).click()
  await page.getByLabel('Describe the image').fill('a cat')
  await page.getByRole('button', { name: 'Search images', exact: true }).click()
  await expect(page.getByText('Indexed 4 / 4 supported images', { exact: false })).toBeVisible({
    timeout: 65000,
  })
  const output = resolve('../docs/verification/phase3')
  await mkdir(output, { recursive: true })
  await writeFile(
    resolve(output, 'restart-browser.json'),
    JSON.stringify(
      {
        restarted: true,
        settings: status.settings,
        profiles: profiles.profiles.map((p: { profileId: number; coverage: object }) => ({
          profileId: p.profileId,
          coverage: p.coverage,
        })),
        resources: status.services.runtime.observed.resources,
      },
      null,
      2,
    ),
  )
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/thepersonincharge/ai')
  await expect(page.getByRole('heading', { name: 'AI Management', exact: true })).toBeVisible()
  await page
    .getByRole('heading', { name: 'Settings and resource limits', exact: true })
    .scrollIntoViewIfNeeded()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: resolve(output, 'mobile-settings.png'), fullPage: true })
})
