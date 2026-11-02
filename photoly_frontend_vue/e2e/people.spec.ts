import { expect, test } from '@playwright/test'
import { mockApi, ok, photo, signIn, openDetails } from './mockApi'
const pref = {
  enabled: true,
  paused: false,
  placement: 'server',
  generation: 1,
  available: true,
  visible: true,
  choosable: true,
  reason: '',
}
const people = [
  { id: 1, name: 'Alice', hidden: false, count: 2, coverFaceId: null },
  { id: 2, name: '', hidden: false, count: 1, coverFaceId: null },
]
function routes() {
  let preference = { ...pref }
  let list = people.map((p) => ({ ...p }))
  let assigned = 1
  return {
    '/ai/people/settings': (c: { method: string; body: unknown }) => {
      if (c.method === 'PATCH') {
        preference = { ...preference, ...(c.body as object) }
        preference.visible = preference.enabled
        preference.reason = preference.paused ? 'paused' : ''
      }
      return ok({ preference, model: {} })
    },
    '/ai/people/status': () =>
      ok({ preference, total: 3, ready: 3, failed: 0, processing: 0, pending: 0 }),
    '/ai/people/jobs/claim': ok({ job: null }),
    '/ai/people/persons': (c: { query: Record<string, string> }) =>
      ok({
        items: list
          .filter((p) =>
            c.query.mode === 'all'
              ? true
              : c.query.mode === 'hidden'
                ? p.hidden
                : c.query.mode === 'pending'
                  ? !p.name && p.count === 1
                  : !p.hidden && (!!p.name || p.count >= 2),
          )
          .filter((p) => !c.query.q || p.name.includes(c.query.q)),
        nextAfter: null,
      }),
    '/ai/people/persons/1': (c: { method: string; body: unknown }) => {
      if (c.method === 'PATCH') list[0] = { ...list[0]!, ...(c.body as object) }
      return ok(list[0])
    },
    '/ai/people/persons/2': () => ok(list.find((p) => p.id === 2) ?? list[0]),
    '/ai/people/persons/1/photos': ok({
      items: [photo(10, 'portrait'), photo(11, 'friends')],
      nextAfter: null,
    }),
    '/ai/people/persons/2/photos': ok({ items: [photo(11, 'friends')], nextAfter: null }),
    '/ai/people/persons/1/faces': () =>
      ok({
        items: [
          {
            id: 1,
            photoId: 10,
            personId: assigned,
            name: list[0]!.name,
            box: [0.1, 0.1, 0.2, 0.2],
            dismissed: false,
          },
        ],
        nextAfter: null,
      }),
    '/ai/people/faces/1': (c: { body: unknown }) => {
      assigned = (c.body as { personId: number }).personId
      return ok({ personId: assigned })
    },
    '/ai/people/persons/1/merge': () => {
      list = [{ ...list[0]!, count: 3 }]
      return ok(list[0])
    },
    '/ai/people/photos/10': () =>
      ok({
        status: 'ready',
        items: [{ id: 1, photoId: 10, personId: 1, name: list[0]!.name }],
        nextAfter: null,
      }),
    '/photo/getByNamespace': ok([photo(10, 'portrait')]),
    '/photo/getByGallery': ok([photo(10, 'portrait')]),
    '/gallery/getAll': ok([
      { gaId: 1, gaName: 'Family', createDate: '2026-01-01', coverId: 1, coverColor: '#0d9488' },
    ]),
    '/user/getInfo': ok({ userName: 'Owner', email: 'owner@example.test', role: 'user' }),
    '/ai/people/data': () => {
      preference = { ...preference, enabled: false, visible: false }
      list = []
      return ok({ cleared: true })
    },
    '/ai/people/retry': ok({ queued: true }),
  }
}
test.beforeEach(async ({ page }) => {
  await signIn(page)
})
test('people list, rename, corrections, hidden and mobile Albums entry', async ({
  page,
  isMobile,
}) => {
  const calls = await mockApi(page, routes())
  await page.goto('/home/PeoplePart')
  await expect(page.getByRole('button', { name: 'Alice 2 photos' })).toBeVisible()
  if (isMobile)
    await expect(
      page.getByRole('navigation', { name: 'Album type' }).getByRole('link', { name: 'People' }),
    ).toBeVisible()
  else
    await expect(
      page
        .getByRole('navigation', { name: 'Main' })
        .getByRole('link', { name: 'People', exact: true }),
    ).toBeVisible()
  await page.getByRole('button', { name: 'To organize', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Unnamed person 1 photo' })).toBeVisible()
  await page.getByRole('button', { name: 'People', exact: true }).click()
  await page.getByRole('button', { name: 'Alice 2 photos' }).click()
  await expect(page.getByRole('button', { name: 'portrait.jpg', exact: true })).toBeVisible()
  await page.getByLabel('Person name', { exact: true }).fill('Alicia')
  await page.getByRole('button', { name: 'Save name' }).click()
  await expect(page.getByRole('heading', { name: 'Alicia', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Manage faces' }).click()
  await page.getByRole('button', { name: "This isn't this person" }).click()
  await page
    .getByRole('dialog')
    .last()
    .getByRole('button', { name: 'Unnamed person', exact: true })
    .click()
  expect(
    calls.some(
      (c) => c.path === '/ai/people/faces/1' && (c.body as { personId: number })?.personId === 2,
    ),
  ).toBe(true)
  await page
    .getByRole('dialog', { name: 'Manage faces', exact: true })
    .getByRole('button', { name: 'Close', exact: true })
    .click()
  await page.getByRole('button', { name: 'Hide person', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Restore person' })).toBeVisible()
  await page.screenshot({ path: test.info().outputPath('people-detail.png') })
})
test('viewer and gallery open the whole-library person page', async ({ page }) => {
  await mockApi(page, routes())
  await page.goto('/home/GalleryPart')
  await page.getByRole('button', { name: /^Family/ }).click()
  await page.getByRole('button', { name: 'People', exact: true }).click()
  await page.getByRole('dialog').getByRole('button', { name: /Alice/ }).click()
  await expect(page).toHaveURL(/PeoplePart\?person=1/)
  await page.getByRole('button', { name: 'portrait.jpg', exact: true }).click()
  await openDetails(page)
  await page.getByRole('button', { name: 'People in this photo' }).click()
  await expect(
    page.getByRole('dialog').last().getByRole('button', { name: 'Alice', exact: true }),
  ).toBeVisible()
  await page.getByRole('dialog').last().getByRole('button', { name: 'Alice', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Alicia', exact: true })).toHaveCount(0)
  await page.goBack()
  await expect(page).toHaveURL(/GalleryPart\?gallery=1/)
  await expect(page.getByRole('heading', { name: 'Family', exact: true })).toBeVisible()
})
test('settings pause, opt-out hides entry, and clear requires confirmation', async ({ page }) => {
  const calls = await mockApi(page, routes())
  await page.goto('/home/ManagePart#people')
  await page.getByRole('button', { name: 'Pause processing' }).click()
  await expect(page.getByRole('button', { name: 'Resume processing' })).toBeVisible()
  await page.getByRole('button', { name: 'Delete all face data', exact: true }).click()
  await expect(page.getByRole('alertdialog')).toBeVisible()
  expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: 'Delete all face data', exact: true })
    .click()
  await expect(page.getByRole('checkbox', { name: 'People for my library' })).not.toBeChecked()
  expect(
    calls.some(
      (c) =>
        c.path === '/ai/people/data' &&
        c.method === 'DELETE' &&
        (c.body as { confirm: boolean }).confirm,
    ),
  ).toBe(true)
})

test('merge selection survives switching between People and To organize', async ({ page }) => {
  const calls = await mockApi(page, routes())
  await page.goto('/home/PeoplePart')
  await page.getByRole('checkbox', { name: 'Select Alice' }).check()
  await page.getByRole('button', { name: 'To organize', exact: true }).click()
  await page.getByRole('checkbox', { name: 'Select unnamed person 2' }).check()
  await page.getByRole('button', { name: 'Merge people', exact: true }).click()
  await page.getByRole('combobox', { name: 'Person to keep' }).selectOption('1')
  await page.getByRole('button', { name: 'Merge selected people' }).click()
  await expect(page.getByRole('heading', { name: 'Alice', exact: true })).toBeVisible()
  expect(
    calls.some(
      (c) =>
        c.path === '/ai/people/persons/1/merge' &&
        JSON.stringify((c.body as { ids: number[] }).ids) === '[2]',
    ),
  ).toBe(true)
})

test('disabled People hides entry and does not request people or jobs', async ({ page }) => {
  const disabled = { ...pref, available: false, visible: false, reason: 'people_disabled' }
  const calls = await mockApi(page, {
    ...routes(),
    '/ai/people/settings': ok({ preference: disabled, model: {} }),
  })
  await page.goto('/home/PeoplePart')
  await expect(
    page.getByText('Enable People in your account settings when AI access is available.'),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'People', exact: true })).toHaveCount(0)
  expect(
    calls.some((c) => c.path === '/ai/people/persons' || c.path === '/ai/people/jobs/claim'),
  ).toBe(false)
})
