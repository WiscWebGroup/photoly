import { expect, test } from '@playwright/test'
import { mockApi, ok, openDetails, photo, signIn } from './mockApi'
import { capabilitiesFixture } from '../src/test/aiFixtures'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})
async function setup(
  page: import('@playwright/test').Page,
  extra: Parameters<typeof mockApi>[1] = {},
) {
  const records = new Map<number, { text: string | null; revision: number }>()
  const calls = await mockApi(page, {
    '/photo/getByNamespace': ok([photo(10, 'first'), photo(11, 'second')]),
    '/photo/getPhotoExif': ok({}),
    '/photo/ocr': (call) => {
      const body = call.body as { photoId: number; text: string; revision: number }
      const id = call.method === 'GET' ? Number(call.query.photoId) : body.photoId
      if (call.method === 'PUT') records.set(id, { text: body.text, revision: body.revision + 1 })
      return ok({
        photoId: id,
        contentVersion: 1,
        languages: 'chi_sim+chi_tra+eng+jpn',
        engineVersion: null,
        updatedAt: null,
        ...(records.get(id) ?? { text: null, revision: 0 }),
      })
    },
    '/photo/ocr/recognize': (call) => {
      const { photoId, revision } = call.body as { photoId: number; revision: number }
      records.set(photoId, { text: 'Hello 中文 日本語', revision: revision + 1 })
      return ok({ photoId, contentVersion: 1, ...records.get(photoId) })
    },
    ...extra,
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'first.jpg', exact: true }).click()
  return calls
}
async function expand(page: import('@playwright/test').Page) {
  await openDetails(page)
  await page.locator('summary').filter({ hasText: 'Extract Text' }).click()
  await expect(page.getByLabel('Image text')).toBeVisible()
}

test('recognize, edit, copy and reopen persisted text; other photos stay independent', async ({
  page,
}) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: async (text: string) => {
          sessionStorage.setItem('copied-ocr', text)
        },
      },
    })
  })
  const calls = await setup(page)
  expect(calls.some((c) => c.path === '/photo/ocr')).toBe(false)
  await expand(page)
  expect(calls.some((c) => c.path === '/photo/ocr/recognize')).toBe(false)
  await page.getByRole('button', { name: 'Recognize Text', exact: true }).click()
  await expect(page.getByLabel('Image text')).toHaveValue('Hello 中文 日本語')
  await page.getByLabel('Image text').scrollIntoViewIfNeeded()
  await page.screenshot({ path: test.info().outputPath('ocr-panel.png') })
  await page.getByLabel('Image text').fill('corrected 中文 日本語')
  await page.getByRole('button', { name: 'Copy', exact: true }).click()
  await expect
    .poll(() => page.evaluate(() => sessionStorage.getItem('copied-ocr')))
    .toBe('corrected 中文 日本語')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(page.getByText('Text saved', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByRole('button', { name: 'first.jpg', exact: true }).click()
  await expand(page)
  await expect(page.getByLabel('Image text')).toHaveValue('corrected 中文 日本語')
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByRole('button', { name: 'second.jpg', exact: true }).click()
  await expand(page)
  await expect(page.getByLabel('Image text')).toHaveValue('')
})

test('cancel, save or discard before closing; re-recognition requires confirmation', async ({
  page,
}) => {
  await setup(page)
  await expand(page)
  await page.getByLabel('Image text').fill('draft')
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await expect(page.getByRole('alertdialog')).toBeVisible()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.getByLabel('Image text')).toHaveValue('draft')
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByRole('button', { name: 'Save and continue' }).click()
  await expect(page.getByRole('dialog')).not.toBeVisible()
  await page.getByRole('button', { name: 'first.jpg', exact: true }).click()
  await expand(page)
  await expect(page.getByLabel('Image text')).toHaveValue('draft')
  await page.getByRole('button', { name: 'Recognize Again' }).click()
  await expect(page.getByText('Recognize this image again?')).toBeVisible()
  await page.getByRole('button', { name: 'Recognize and replace' }).click()
  await expect(page.getByLabel('Image text')).toHaveValue('Hello 中文 日本語')
  await page.getByLabel('Image text').fill('discard this')
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(page.getByRole('dialog')).not.toBeVisible()
})

test('similar-image navigation and history protect each image draft', async ({ page }) => {
  const calls = await setup(page, {
    '/ai/capabilities': ok(capabilitiesFixture),
    '/ai/photos/10/similar/page': ok({
      items: [{ photo: photo(11, 'second'), score: 0.9 }],
      nextCursor: null,
      hasMore: false,
      profileId: 1,
      coverage: capabilitiesFixture.coverage,
      requestId: 'ocr-navigation',
    }),
  })
  await expand(page)
  await page.getByLabel('Image text').fill('first draft')
  await page.getByRole('button', { name: 'Find similar' }).click() // results are on demand
  await page.getByTestId('photo-viewer-scroll').evaluate((element) => {
    element.dispatchEvent(new Event('touchmove', { bubbles: true }))
    element.scrollTop = element.scrollHeight
    element.dispatchEvent(new Event('scroll'))
  })
  await page.getByRole('button', { name: 'View second.jpg', exact: true }).click()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.getByLabel('Image text')).toHaveValue('first draft')
  await page.getByRole('button', { name: 'View second.jpg', exact: true }).click()
  await page.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(
    page.getByTestId('photo-viewer-scroll').getByRole('heading', { name: 'second.jpg' }),
  ).toBeVisible()
  await expand(page)
  await expect(page.getByLabel('Image text')).toHaveValue('')
  await page.getByLabel('Image text').fill('second saved')
  await page.getByRole('button', { name: 'Back to previous photo' }).click()
  await page.getByRole('button', { name: 'Save and continue' }).click()
  await expect(
    page.getByTestId('photo-viewer-scroll').getByRole('heading', { name: 'first.jpg' }),
  ).toBeVisible()
  await expand(page)
  await expect(page.getByLabel('Image text')).toHaveValue('')
  expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({
    photoId: 11,
    text: 'second saved',
  })
})
