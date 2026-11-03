import { test, expect } from '@playwright/test'
import { API, PNG, mockApi, ok, photo, signIn } from './mockApi'

test('viewer shows metadata while the image waits, then clears loading; failures can retry', async ({
  page,
}) => {
  await signIn(page)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await mockApi(page, {
    '/photo/getByNamespace': ok([photo(10, 'Slow photo')]),
    '/photo/getPhotoExif': ok({ Model: 'Camera already loaded' }),
  })
  let release!: () => void
  const gate = new Promise<void>((r) => (release = r))
  let fail = true
  await page.route(`${API}/photo/render/**`, async (r) => {
    await gate
    if (fail) return r.abort()
    return r.fulfill({
      body: PNG,
      contentType: 'image/png',
      headers: { 'access-control-allow-origin': '*' },
    })
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Slow photo.jpg', exact: true }).click()
  await expect(page.getByText('Loading photo…', { exact: true })).toBeVisible()
  await expect(page.getByText('Camera already loaded', { exact: true })).toBeVisible()
  await page.screenshot({
    path: '/tmp/photoly-viewer-loading-' + test.info().project.name + '.png',
  })
  release()
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible()
  await expect(page.getByText('Loading photo…', { exact: true })).toBeHidden()
  fail = false
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(page.locator('[data-photo-zoom]').last()).toBeVisible()
  await expect(page.getByText('Loading photo…', { exact: true })).toBeHidden()
})

test('share link and QR follow the API address; clipboard failure offers manual copying', async ({
  page,
}) => {
  await signIn(page)
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true })
    document.execCommand = () => false
  })
  const calls = await mockApi(page, {
    '/photo/getByNamespace': ok([photo(10, 'Shared photo', { visibility: 1 })]),
    '/photo/getPath': (c) =>
      ok(c.query.shareBase + '/photo/renderToken?path=owner/photo.jpg&token=photo-only'),
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Shared photo.jpg', exact: true }).click()
  await page.getByRole('button', { name: 'Share', exact: true }).click()
  await expect(page.getByRole('textbox', { name: 'Link', exact: true })).toHaveValue(
    API + '/photo/renderToken?path=owner/photo.jpg&token=photo-only',
  )
  await expect(page.getByRole('img', { name: 'QR code', exact: true })).toHaveAttribute(
    'src',
    new RegExp('shareBase=' + encodeURIComponent(API)),
  )
  await page.getByRole('button', { name: 'Copy Link', exact: true }).click()
  await expect(page.getByText('Clipboard unavailable. Select and copy the link.')).toBeVisible()
  expect(calls.find((c) => c.path === '/photo/getPath')?.query.shareBase).toBe(API)
  await expect(page.getByRole('textbox', { name: 'Link', exact: true })).toBeFocused()
})

test('Category uses compact rectangular collage covers', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/photo/categories': ok({
      items: [
        { category: 'panorama', count: 3, previews: [photo(1, 'a'), photo(2, 'b'), photo(3, 'c')] },
        { category: 'portrait', count: 1, previews: [photo(4, 'd')] },
      ],
      pending: 0,
    }),
  })
  await page.goto('/home/CategoryPart')
  const card = page.getByRole('button', { name: 'Panoramas 3 items', exact: true })
  await expect(card).toBeVisible()
  await expect(card.locator('img')).toHaveCount(3)
  expect(
    await card
      .locator('img')
      .evaluateAll((images) => images.every((i) => getComputedStyle(i).transform === 'none')),
  ).toBe(true)
  await page.screenshot({
    path: '/tmp/photoly-category-polish-' + test.info().project.name + '.png',
  })
})
