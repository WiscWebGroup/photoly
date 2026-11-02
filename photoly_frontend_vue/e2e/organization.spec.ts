import { expect, test } from '@playwright/test'
import { mockApi, ok, photo, signIn, openDetails, API } from './mockApi'
import type { MediaNote } from '../src/components/MediaNotes.vue'
test.beforeEach(async ({ page }) => {
  await signIn(page)
})
test('categories open media; private notes persist and block unsaved navigation', async ({
  page,
  isMobile,
}) => {
  const notes: MediaNote[] = []
  let portraitOverride: boolean | null = null
  const calls = await mockApi(page, {
    '/photo/category': (c) => {
      if (c.method === 'PUT') {
        portraitOverride = (c.body as { included: boolean | null }).included
        return ok(null)
      }
      return ok([{ category: 'portrait', automatic: false, override: portraitOverride }])
    },
    '/photo/categories': ok({ items: [{ category: 'panorama', count: 1 }], pending: 0 }),
    '/photo/categories/panorama/photos': ok({ items: [photo(10, 'panorama')], nextAfter: null }),
    '/photo/notes': (c) => {
      if (c.method === 'POST') {
        notes.push({
          ...(c.body as MediaNote),
          id: notes.length + 1,
          revision: 1,
          needsReposition: false,
          createdAt: '2026-01-01',
          updatedAt: '2026-01-01',
        })
        return ok({ id: notes.length })
      }
      return ok({ items: notes, contentVersion: 1 })
    },
  })
  await page.route(`${API}/photo/render/**`, (r) =>
    r.fulfill({
      contentType: 'image/svg+xml',
      headers: { 'Access-Control-Allow-Origin': '*' },
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><rect width="1200" height="800" fill="#528777"/></svg>',
    }),
  )
  await page.goto('/home/CategoryPart')
  await page.getByRole('button', { name: /Panoramas 1 items/ }).click()
  await page.getByRole('button', { name: 'panorama.jpg', exact: true }).click()
  await openDetails(page)
  await page.getByText('Category', { exact: true }).last().click()
  await page.getByRole('combobox').selectOption('yes')
  await expect(page.getByRole('combobox')).toHaveValue('yes')
  const panel = page.getByRole('region', { name: 'Notes' })
  await panel.getByRole('button', { name: /Notes/ }).click()
  await panel.getByRole('button', { name: 'Add note', exact: true }).click()
  await panel
    .getByRole('textbox', { name: 'New note' })
    .fill('Our first trip\nA special day <script>not HTML</script>')
  await panel.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(panel.getByText('Our first trip', { exact: false })).toBeVisible()
  await panel.getByRole('button', { name: 'Pin on photo' }).click()
  const point = page.getByRole('button', { name: 'Choose note position' })
  await point.scrollIntoViewIfNeeded()
  if (isMobile) await point.tap({ position: { x: 40, y: 30 } })
  else await point.click({ position: { x: 40, y: 30 } })
  await panel.getByRole('textbox', { name: 'New note' }).fill('This exact spot')
  await panel.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Note 2', exact: true })).toBeAttached()
  expect(notes[1]!.x).toBeGreaterThan(0)
  expect(notes[1]!.x).toBeLessThan(1)
  if (!isMobile) {
    const image = page.locator('[data-photo-zoom] img')
    await image.hover()
    await page.mouse.wheel(0, -300)
    await expect(page.locator('[data-photo-zoom]')).not.toHaveAttribute('data-scale', '1')
    const imageBox = (await image.boundingBox())!
    const pinBox = (await page.getByRole('button', { name: 'Note 2', exact: true }).boundingBox())!
    expect(
      Math.abs((pinBox.x + pinBox.width / 2 - imageBox.x) / imageBox.width - notes[1]!.x!),
    ).toBeLessThan(0.01)
    await page.getByRole('button', { name: 'Fit to window' }).click()
  }
  await panel.getByRole('button', { name: 'Add note', exact: true }).click()
  await panel.getByRole('textbox', { name: 'New note' }).fill('Unsaved')
  await panel.getByRole('button', { name: 'Cancel', exact: true }).click()
  await page.getByRole('button', { name: 'Keep editing' }).click()
  await expect(panel.getByRole('textbox', { name: 'New note' })).toHaveValue('Unsaved')
  await panel.getByRole('button', { name: 'Cancel', exact: true }).click()
  await page.getByRole('button', { name: 'Discard changes' }).click()
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  expect(calls.filter((c) => c.path === '/photo/notes' && c.method === 'POST')).toHaveLength(2)
  await page.screenshot({ path: test.info().outputPath('category-notes.png') })
})
test('disconnected mount keeps cached folder contents and can recover', async ({ page }) => {
  const mount = { nsId: 5, nsName: 'NAS', sourceId: 12, sourceType: 'smb', isMount: true }
  let state = 'unreachable'
  await mockApi(page, {
    '/namespace/getChildren': (c) => ok(c.query.parentId === '1' ? [mount] : []),
    '/namespace/trace': (c) =>
      ok(
        c.query.nsId === '5' ? [{ nsId: 1, nsName: 'root' }, mount] : [{ nsId: 1, nsName: 'root' }],
      ),
    '/photo/getByNamespace': (c) =>
      ok(c.query.nsId === '5' ? [photo(10, 'cached', { nsId: 5, sourceId: 12 })] : []),
    '/user/storage/health': (c) => {
      if (c.method === 'POST') state = 'available'
      return ok([{ sourceId: 12, state, checkedAt: new Date().toISOString() }])
    },
  })
  await page.goto('/home/HomePart')
  await expect(page.getByText('Storage unavailable', { exact: true })).toBeVisible()
  await page.getByText('NAS', { exact: true }).click()
  await expect(
    page.getByText('Photoly cannot connect to this storage source.', { exact: false }),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'cached.jpg', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Check again' }).click()
  await expect(
    page.getByText('Photoly cannot connect to this storage source.', { exact: false }),
  ).toHaveCount(0)
})
