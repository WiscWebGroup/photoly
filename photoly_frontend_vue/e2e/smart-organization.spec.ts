import { expect, test } from '@playwright/test'
import { mockApi, ok, openDetails, photo, signIn } from './mockApi'
import { settingsFixture, statusFixture, adminCapabilitiesFixture } from '../src/test/aiFixtures'

const camera = {
  id: 1,
  source: 'camera',
  name: 'iPhone 13',
  suppressed: false,
  basis: { method: 'EXIF device model' },
}
const trip = {
  id: 2,
  source: 'trip',
  name: 'Times in Hong Kong and Shenzhen',
  suppressed: false,
  basis: {
    startDate: '2026-10-01',
    endDate: '2026-10-07',
    places: { 'Hong Kong': 4, Shenzhen: 3 },
    method: 'Capture time, nearby locations and visual similarity',
  },
}

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('opt in, refresh and restore smart items', async ({ page }) => {
  let enabled = false
  let removed = true
  const calls = await mockApi(page, {
    '/user/getInfo': ok({
      userName: 'Alice',
      email: 'alice@example.com',
      role: 'user',
      createDate: '2026-01-01',
    }),
    '/ai/smart/settings': (call) => {
      if (call.method === 'PATCH') enabled = (call.body as { enabled: boolean }).enabled
      return ok({ enabled, available: true, reason: null })
    },
    '/ai/smart/status': ok({
      processed: 25,
      total: 50,
      failed: 1,
      pending: true,
      reason: 'waiting_for_vectors',
    }),
    '/ai/smart/dismissed': () =>
      ok({ items: removed ? [{ ...camera, suppressed: true }] : [], nextAfter: null }),
    '/ai/smart/items/1/restore': () => {
      removed = false
      return ok({ restored: true })
    },
  })
  await page.goto('/home/ManagePart')
  const toggle = page.getByRole('checkbox', { name: 'Enable my Smart Tag' })
  await expect(toggle).not.toBeChecked()
  await toggle.check()
  await expect(page.getByText('Smart Tag settings saved.')).toBeVisible()
  await expect(page.getByText('25 / 50 media inspected · 1 failed')).toBeVisible()
  await page.getByRole('button', { name: 'Refresh and retry' }).click()
  await page.getByText('Deleted smart items', { exact: true }).click()
  await page.getByRole('button', { name: 'Restore iPhone 13' }).click()
  await expect(page.getByText('No deleted smart items.')).toBeVisible()
  expect(calls.some((c) => c.path === '/ai/smart/refresh')).toBe(true)
  await page
    .locator('[aria-label="Smart Tag settings"]')
    .screenshot({ path: test.info().outputPath('smart-settings.png') })
})

test('smart tags, gallery details and viewer selectors have consistent badges', async ({
  page,
}) => {
  const calls = await mockApi(page, {
    '/tag/getAll': ok([
      { tagId: 1, tagName: 'iPhone 13', smart: camera },
      { tagId: 2, tagName: 'iPhone 13', smart: null },
      { tagId: 3, tagName: 'iPhone 13 Pro', smart: camera },
    ]),
    '/gallery/getAll': ok([
      {
        gaId: 2,
        gaName: trip.name,
        createDate: '2026-10-08T00:00:00',
        coverId: 3,
        coverColor: '#0d9488',
        smart: trip,
      },
    ]),
    '/photo/getByGallery': ok([photo(10, 'travel'), photo(11, 'movie', { format: 'mp4' })]),
    '/photo/getTagByPhoto': ok([{ tag_id: 1, tag_name: 'iPhone 13', smart: camera }]),
    '/photo/getGalleryByPhoto': ok([{ ga_id: 2, ga_name: trip.name, smart: trip }]),
    '/photo/getPhotoExif': ok({}),
  })
  await page.goto('/home/TagPart')
  await expect(page.getByRole('button', { name: /iPhone 13 Smart.*Camera/ })).toBeVisible()
  await expect(page.getByRole('button', { name: 'iPhone 13', exact: true })).toBeVisible()
  await page.goto('/home/GalleryPart')
  await page.getByRole('button', { name: /Times in Hong Kong and Shenzhen/ }).click()
  await expect(page.getByLabel('Smart organization details')).toContainText(
    '2026-10-01 – 2026-10-07',
  )
  await expect(page.getByText('movie.mp4')).toBeVisible()
  await page.screenshot({ path: test.info().outputPath('smart-gallery.png') })
  await page.locator('p[title="travel.jpg"]').click()
  const dialog = page.getByRole('dialog')
  await openDetails(page)
  // "Tags and galleries" is open by default in the details pane.
  await expect(dialog.getByText(/Smart.*Camera/)).toBeVisible()
  await dialog.getByRole('button', { name: 'Add tag' }).click()
  await expect(page.getByRole('button', { name: /iPhone 13 Pro.*Smart/ })).toBeVisible()
  expect(
    calls
      .filter((c) =>
        [
          '/tag/getAll',
          '/gallery/getAll',
          '/photo/getTagByPhoto',
          '/photo/getGalleryByPhoto',
        ].includes(c.path),
      )
      .every((c) => c.query.includeSmart === 'true'),
  ).toBe(true)
})

test('administrator enables Smart Tag and sees progress and pause reasons', async ({ page }) => {
  let settings = { ...settingsFixture, smart_tag_enabled: false }
  const calls = await mockApi(page, {
    '/admin/auth': ok(true),
    '/admin/ai/capabilities': ok(adminCapabilitiesFixture),
    '/admin/ai/profiles': ok({ registry: [], profiles: [] }),
    '/admin/ai/settings': (call) => {
      if (call.method === 'PATCH')
        settings = {
          ...settings,
          ...(call.body as { settings: object }).settings,
          generation: settings.generation + 1,
        }
      return ok(settings)
    },
    '/admin/ai/status': () =>
      ok({
        ...statusFixture,
        settings,
        smartTag: { processed: 30, total: 50, failed: 2, reason: 'maintenance' },
      }),
  })
  await page.goto('/thepersonincharge/ai')
  await page.getByLabel('Smart Tag', { exact: true }).check()
  await expect(page.getByLabel('Smart Tag progress')).toContainText('30 / 50')
  await expect(page.getByLabel('Smart Tag progress')).toContainText('Paused for maintenance')
  await expect
    .poll(() =>
      calls.some(
        (c) =>
          c.method === 'PATCH' &&
          (c.body as { settings?: { smart_tag_enabled?: boolean } })?.settings
            ?.smart_tag_enabled === true,
      ),
    )
    .toBe(true)
})
