import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'

test('S3 has a distinct mount appearance and protected folder actions', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/namespace/getRoot': ok({ nsId: 1, nsName: '/', sourceId: 1, sourceType: 'local' }),
    '/namespace/trace': ok([{ nsId: 1, nsName: '/', sourceId: 1, sourceType: 'local' }]),
    '/namespace/getChildren': ok([
      { nsId: 2, nsName: 'Trips', sourceId: 1, sourceType: 'local', isMount: false },
      { nsId: 3, nsName: 'S3', sourceId: 2, sourceType: 's3', isMount: true },
    ]),
    '/user/storage/sources': ok([]),
  })
  await page.goto('/home/HomePart')
  const mount = page.locator('[data-folder-id="3"]')
  await expect(mount).toBeVisible()
  await expect(mount).toHaveAttribute('draggable', 'false')
  await expect(mount.locator('.text-sky-700')).toBeVisible()
  await mount.click({ button: 'right' })
  await expect(page.getByRole('menuitem', { name: /Move To/ })).toBeDisabled()
  await expect(page.getByRole('menuitem', { name: /Rename/ })).toBeDisabled()
})

test('folder ZIP uses a browser document download instead of an XHR blob', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/namespace/getChildren': ok([{ nsId: 2, nsName: 'Trips', sourceId: 1, sourceType: 'local' }]),
  })
  // WebKit does not emit download events for route.fulfill bodies. Continue to a
  // real HTTP attachment response so all browser engines exercise native saving.
  const server = createServer((_request, response) => {
    response.writeHead(200, {
      'Content-Type': 'application/zip',
      'Content-Disposition': 'attachment; filename="Trips.zip"',
    })
    response.end(Buffer.from('504b0506000000000000000000000000000000000000', 'hex'))
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const port = (server.address() as AddressInfo).port
  let resourceType = ''
  try {
    await page.route('**/namespace/downloadNs/**', async (route) => {
      resourceType = route.request().resourceType()
      await route.continue({ url: `http://127.0.0.1:${port}/Trips.zip` })
    })
    await page.goto('/home/HomePart')
    await page.locator('[data-folder-id="2"]').click({ button: 'right' })
    const download = page.waitForEvent('download')
    await page.getByRole('menuitem', { name: /Download/ }).click()
    expect((await download).suggestedFilename()).toBe('Trips.zip')
    expect(resourceType).toBe('document')
  } finally {
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})
