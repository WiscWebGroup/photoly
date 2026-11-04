import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'
const key = 'a'.repeat(64) + ':still',
  version = 'b'.repeat(32)
const member = (id: number) => ({
  photoId: id,
  photoName: `Photo ${id}`,
  fileName: `copy-${id}.png`,
  format: 'png',
  nsId: 1,
  sourceId: 1,
  contentVersion: 1,
  bytes: 234567,
  kept: false,
  folder: '/Summer',
  sourceName: 'Photoly',
  sourceType: 'local',
})
test('duplicates keep, ignore, restore, preview and confirmed cleanup', async ({ page }, info) => {
  await signIn(page)
  let items = [member(1), member(2), member(3)],
    ignored = false
  const calls = await mockApi(page, {
    '/photo/duplicates': ({ query }) =>
      ok({
        items:
          items.length > 1 && (query.ignored === 'true') === ignored
            ? [
                {
                  key,
                  version,
                  count: items.length,
                  keptCount: items.filter((p) => p.kept).length,
                  ignored,
                },
              ]
            : [],
        total: items.length > 1 && (query.ignored === 'true') === ignored ? 1 : 0,
      }),
    '/photo/duplicates/members': () => ok({ items, total: items.length, version }),
    '/photo/duplicates/keep': ({ body }) => {
      const b = body as { photoId: number; value: boolean }
      items = items.map((p) => (p.photoId === b.photoId ? { ...p, kept: b.value } : p))
      return ok({ ok: true })
    },
    '/photo/duplicates/ignore': ({ body }) => {
      ignored = (body as { value: boolean }).value
      return ok({ ok: true })
    },
    '/photo/duplicates/delete': ({ body }) => {
      items = items.filter((p) => !(body as { photoIds: number[] }).photoIds.includes(p.photoId))
      return ok({ ok: true })
    },
  })
  await page.goto('/home/HomePart')
  await page
    .getByRole('link', { name: 'Duplicate Detection', exact: true })
    .filter({ visible: true })
    .click()
  await expect(page.getByRole('heading', { name: 'Duplicate Detection' })).toBeVisible()
  await expect(page.getByRole('checkbox', { name: 'Select copy-1.png' })).not.toBeChecked()
  await expect(page.getByRole('button', { name: 'Delete selected (0)' })).toBeDisabled()
  await page.getByRole('button', { name: 'Keep copy-1.png', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'Select copy-1.png' })).toBeDisabled()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Unmark copy-1.png' })).toBeVisible()
  await page.getByRole('button', { name: 'Preview copy-1.png' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toBeHidden()
  await page.getByRole('button', { name: 'Ignore group', exact: true }).click()
  await expect(page.getByText('No duplicates to review')).toBeVisible()
  await page.getByRole('button', { name: 'Ignored', exact: true }).click()
  await page.getByRole('button', { name: 'Restore group', exact: true }).click()
  await expect(page.getByText('No ignored groups')).toBeVisible()
  await page.getByRole('button', { name: 'To review', exact: true }).click()
  await page.getByRole('button', { name: 'Select other copies', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Delete selected (2)' })).toBeEnabled()
  await page.screenshot({ path: info.outputPath('duplicates.png'), fullPage: true })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  await page.getByRole('button', { name: 'Delete selected (2)' }).click()
  await expect(page.getByRole('alertdialog')).toContainText('permanently deletes')
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  expect(calls.filter((c) => c.path === '/photo/duplicates/delete')).toHaveLength(0)
  await page.getByRole('button', { name: 'Delete selected (2)' }).click()
  await page.getByRole('button', { name: 'Permanently delete', exact: true }).click()
  await expect(page.getByText('2 photos deleted.', { exact: true })).toBeVisible()
  await expect(page.getByText('No duplicates to review')).toBeVisible()
  expect(items.map((p) => p.photoId)).toEqual([1])
})
test('pagination, retry and partial failure clear stale selections', async ({ page }) => {
  await signIn(page)
  let fail = true,
    done = false
  const secondKey = 'c'.repeat(64) + ':still'
  const group = (key: string, count: number) => ({
    key,
    version,
    count,
    keptCount: 0,
    ignored: false,
  })
  await mockApi(page, {
    '/photo/duplicates': ({ query }) =>
      fail
        ? { msgCode: 500, t: null }
        : ok({
            items: Number(query.offset)
              ? [group(secondKey, 2)]
              : [group(key, 25), ...(!done ? [group(secondKey, 2)] : [])],
            total: 11,
          }),
    '/photo/duplicates/members': ({ query }) =>
      ok({
        items:
          query.key === secondKey
            ? [member(50), member(51)]
            : Number(query.offset)
              ? [member(25)]
              : [member(1), member(2)],
        total: query.key === secondKey ? 2 : 25,
        version,
      }),
    '/photo/duplicates/delete': ({ body }) => {
      if ((body as { key: string }).key === key) return { msgCode: 409, t: 'duplicate_changed' }
      done = true
      return ok({ ok: true })
    },
  })
  await page.goto('/home/DuplicatesPart')
  await expect(page.getByRole('alert')).toContainText('retry')
  fail = false
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await page.getByRole('button', { name: 'Next photos', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'Select copy-25.png' })).toBeVisible()
  await page.getByRole('button', { name: 'Previous photos', exact: true }).click()
  await page.getByRole('button', { name: 'Next groups', exact: true }).first().click()
  await expect(page.getByRole('checkbox', { name: 'Select copy-1.png', exact: true })).toHaveCount(
    0,
  )
  await page.getByRole('button', { name: 'Previous groups', exact: true }).first().click()
  await page.getByRole('checkbox', { name: 'Select copy-1.png', exact: true }).check()
  await page.getByRole('checkbox', { name: 'Select copy-50.png', exact: true }).check()
  await page.getByRole('button', { name: 'Delete selected (2)' }).click()
  await page.getByRole('button', { name: 'Permanently delete', exact: true }).click()
  await expect(page.getByRole('status').filter({ hasText: '1 photo deleted.' })).toContainText(
    '1 group(s) failed',
  )
  await expect(page.getByRole('button', { name: 'Delete selected (0)' })).toBeDisabled()
})

test('ten-item pagination and Keep preserve both pages, selections and scroll', async ({
  page,
}) => {
  await signIn(page)
  const groups = Array.from({ length: 12 }, (_, i) => ({
    key: (i + 1).toString(16).padStart(64, '0') + ':still',
    version,
    count: i === 10 ? 12 : 2,
    keptCount: 0,
    ignored: false,
  }))
  const items = new Map(
    groups.map((g, i) => [
      g.key,
      Array.from({ length: g.count }, (_, j) => member((i + 1) * 100 + j + 1)),
    ]),
  )
  const calls = await mockApi(page, {
    '/photo/duplicates': ({ query }) =>
      ok({
        items: groups.slice(Number(query.offset), Number(query.offset) + Number(query.limit)),
        total: groups.length,
      }),
    '/photo/duplicates/members': ({ query }) => {
      const photos = items.get(query.key!)!
      return ok({
        items: photos.slice(Number(query.offset), Number(query.offset) + Number(query.limit)),
        total: photos.length,
        version,
      })
    },
    '/photo/duplicates/keep': ({ body }) => {
      const b = body as { key: string; photoId: number; value: boolean }
      items.get(b.key)!.find((p) => p.photoId === b.photoId)!.kept = b.value
      groups.find((g) => g.key === b.key)!.keptCount += b.value ? 1 : -1
      return ok({ ok: true })
    },
  })
  await page.goto('/home/DuplicatesPart')
  const pagination = page.getByRole('navigation', { name: 'Duplicate pagination top' })
  await expect(pagination.getByLabel('Groups per page')).toHaveValue('10')
  await expect(page.locator('section[aria-label="Duplicate group"]')).toHaveCount(10)
  await pagination.getByRole('button', { name: 'Next groups' }).click()
  await expect(pagination).toContainText('Page 2 of 2')
  const group = page.locator('section[aria-label="Duplicate group"]').first()
  await expect(group.getByRole('checkbox')).toHaveCount(10)
  await group.getByRole('button', { name: 'Next photos' }).click()
  await expect(group).toContainText('11–12 / 12')
  await page.getByRole('checkbox', { name: 'Select copy-1201.png', exact: true }).check()
  const keep = page.getByRole('button', { name: 'Keep copy-1111.png', exact: true })
  await keep.scrollIntoViewIfNeeded()
  const before = await page.locator('main').evaluate((el) => el.scrollTop)
  expect(before).toBeGreaterThan(100)
  const reads = calls.filter(
    (c) => c.method === 'GET' && c.path.startsWith('/photo/duplicates'),
  ).length
  await keep.click()
  await expect(page.getByRole('button', { name: 'Unmark copy-1111.png' })).toBeVisible()
  await expect(group).toContainText('1 marked Keep')
  expect(
    Math.abs((await page.locator('main').evaluate((el) => el.scrollTop)) - before),
  ).toBeLessThan(3)
  await expect(group).toContainText('11–12 / 12')
  await expect(pagination).toContainText('Page 2 of 2')
  await expect(
    page.getByRole('checkbox', { name: 'Select copy-1201.png', exact: true }),
  ).toBeChecked()
  await page.getByRole('button', { name: 'Unmark copy-1111.png' }).click()
  await expect(keep).toBeVisible()
  expect(
    Math.abs((await page.locator('main').evaluate((el) => el.scrollTop)) - before),
  ).toBeLessThan(3)
  expect(
    calls.filter((c) => c.method === 'GET' && c.path.startsWith('/photo/duplicates')),
  ).toHaveLength(reads)
  await pagination.getByLabel('Groups per page').selectOption('20')
  await expect(pagination).toContainText('Page 1 of 1')
  await expect(page.locator('section[aria-label="Duplicate group"]')).toHaveCount(12)
})
