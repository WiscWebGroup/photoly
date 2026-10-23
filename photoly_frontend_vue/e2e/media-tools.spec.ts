import { test, expect } from '@playwright/test'
import { mockApi, signIn, photo, ok, PNG } from './mockApi'
import { mockUploads } from './uploadsMock'

test('conversion from context menu uses source folder and queues one task', async ({
  page,
}, testInfo) => {
  await signIn(page)
  const calls = await mockApi(page, {
    '/photo/getByNamespace': ok([photo(1, 'Animation', { format: 'gif', nsId: 42 })]),
    '/photo/mediaInfo': ok({ photoId: 1, animated: true, live: false, nsId: 42 }),
    '/namespace/trace': ({ query }) =>
      ok(
        query.nsId === '42'
          ? [
              { nsId: 1, nsName: '/' },
              { nsId: 42, nsName: 'Actual folder' },
            ]
          : [{ nsId: 1, nsName: '/' }],
      ),
    '/photo/convert': ok('conversion-batch'),
  })
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Format Convert' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('Library / Actual folder')).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('format-convert.png'), animations: 'disabled' })
  await expect(dialog.getByText('Only the first animation frame will be saved.')).toBeVisible()
  await dialog.getByLabel('Format', { exact: true }).selectOption('png')
  await dialog.getByRole('button', { name: 'Save copy' }).click()
  await expect.poll(() => calls.filter((c) => c.path === '/photo/convert').length).toBe(1)
  expect(calls.find((c) => c.path === '/photo/convert')!.body).toMatchObject({
    photoId: 1,
    format: 'png',
  })
})

test('online download appears to the left of upload and targets the current folder', async ({
  page,
}) => {
  await signIn(page)
  const calls = await mockApi(page, {
    '/photo/mediaCapabilities': ok({ onlineDownload: true }),
    '/namespace/trace': ok([
      { nsId: 1, nsName: '/' },
      { nsId: 42, nsName: 'Trips' },
    ]),
    '/photo/onlineDownload': ok('download-batch'),
  })
  await page.goto('/home/HomePart?folder=42')
  const download = page
    .getByRole('button', { name: 'Online Download', exact: true })
    .filter({ visible: true })
  const upload = page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true })
  await expect(download).toBeVisible()
  expect((await download.boundingBox())!.x).toBeLessThan((await upload.boundingBox())!.x)
  await download.click()
  await page.getByLabel('Image URL').fill('https://example.com/image.png')
  await page.getByLabel('Format', { exact: true }).selectOption('gif')
  await expect(page.getByRole('dialog').getByText('Library / Trips')).toBeVisible()
  await page.getByRole('button', { name: 'Download', exact: true }).click()
  await expect.poll(() => calls.filter((c) => c.path === '/photo/onlineDownload').length).toBe(1)
  expect(calls.find((c) => c.path === '/photo/onlineDownload')!.body).toMatchObject({
    url: 'https://example.com/image.png',
    format: 'gif',
    nsId: 42,
  })
})

test('paste adds a preview and does not submit until Upload', async ({ page }) => {
  await signIn(page)
  await mockApi(page)
  const server = await mockUploads(page)
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await page.getByRole('button', { name: 'New upload', exact: true }).click()
  await expect(page.getByRole('dialog', { name: 'Upload to Library' })).toBeVisible()
  await page.getByRole('dialog', { name: 'Upload to Library' }).evaluate(
    (dialog, bytes) => {
      const transfer = new DataTransfer()
      transfer.items.add(new File([new Uint8Array(bytes)], 'image.png', { type: 'image/png' }))
      const event = new Event('paste', { bubbles: true, cancelable: true })
      Object.defineProperty(event, 'clipboardData', { value: transfer })
      dialog.dispatchEvent(event)
    },
    [...PNG],
  )
  await expect(page.getByRole('dialog').getByText(/Clipboard .*\.png/)).toBeVisible()
  expect(server.items).toHaveLength(0)
  await page.getByRole('button', { name: 'Upload', exact: true }).click()
  await expect.poll(() => server.items.length).toBe(1)
})

test('Live viewer exposes muted playback and original pair download', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/photo/getByNamespace': ok([photo(1, 'Live cover'), photo(2, 'Still cover')]),
    '/photo/mediaInfo': ({ query }) =>
      ok({
        photoId: Number(query.photoId),
        live: query.photoId === '1',
        liveStatus: 'ready',
        animated: false,
      }),
  })
  // A stalled video keeps the play element available to inspect without a codec fixture.
  await page.route('**/photo/renderLive/**', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 1000))
    await route.abort()
  })
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="1"]').click()
  await expect(page.getByText('LIVE', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Download Live Photo originals' })).toBeVisible()
  await page.getByRole('button', { name: 'Play Live Photo' }).click()
  const video = page.locator('video')
  await expect(video).toHaveJSProperty('muted', true)
  await page.getByRole('button', { name: 'Enable sound' }).click()
  await expect(video).toHaveJSProperty('muted', false)
  await page.getByRole('button', { name: 'Stop Live Photo' }).click()
  await expect(video).toHaveCount(0)
  await page.getByRole('button', { name: 'Play Live Photo' }).click()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByText('LIVE', { exact: true })).toHaveCount(0)
  await expect(video).toHaveCount(0)
})
