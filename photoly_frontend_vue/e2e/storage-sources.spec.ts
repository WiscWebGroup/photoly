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
  await expect(mount.locator('.text-sky-700').first()).toBeVisible()
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

test('local and WebDAV mounts have separate colors and protected actions', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/namespace/getRoot': ok({ nsId: 1, nsName: '/', sourceId: 1, sourceType: 'local' }),
    '/namespace/trace': ok([{ nsId: 1, nsName: '/', sourceId: 1, sourceType: 'local' }]),
    '/namespace/getChildren': ok([
      { nsId: 4, nsName: 'NAS', sourceId: 4, sourceType: 'local', isMount: true },
      { nsId: 5, nsName: 'Cloud photos', sourceId: 5, sourceType: 'webdav', isMount: true },
    ]),
    '/user/storage/sources': ok([]),
  })
  await page.goto('/home/HomePart')
  const local = page.locator('[data-folder-id="4"]')
  const dav = page.locator('[data-folder-id="5"]')
  await expect(local).toContainText('本机挂载')
  await expect(local.locator('.text-amber-700').first()).toBeVisible()
  await expect(dav).toContainText('WebDAV')
  await expect(dav.locator('.text-purple-700').first()).toBeVisible()
  await expect(dav).toHaveAttribute('draggable', 'false')
  await dav.click({ button: 'right' })
  await expect(page.getByRole('menuitem', { name: /Move To/ })).toBeDisabled()
  await expect(page.getByRole('menuitem', { name: /Rename/ })).toBeDisabled()
})

test('SMB shows indexing progress and refreshes only the current folder', async ({ page }) => {
  await signIn(page)
  const root = { nsId: 1, nsName: '/', sourceId: 1, sourceType: 'local' }
  const smb = { nsId: 9, nsName: 'NAS photos', sourceId: 9, sourceType: 'smb', isMount: true }
  let queued = false
  const calls = await mockApi(page, {
    '/namespace/getRoot': ok(root),
    '/namespace/getChildren': ({ query }) => ok(query.parentId === '9' ? [] : [smb]),
    '/namespace/trace': ({ query }) => ok(query.nsId === '9' ? [root, smb] : [root]),
    '/user/storage/sources': () =>
      ok([
        {
          sourceId: 9,
          sourceType: 'smb',
          enabled: true,
          revision: 1,
          scanStatus: 'scanning',
          scanPhase: 'processing',
          scanCompleted: 3,
          scanTotal: 8,
          scanDiscovered: 12,
          scanWaiting: 1,
          scanRetries: 0,
          scanQueued: queued,
        },
      ]),
    '/user/storage/sources/9/scan': () => {
      queued = true
      return ok({ jobId: 42, state: 'queued' })
    },
  })
  await page.goto('/home/HomePart')
  const mount = page.locator('[data-folder-id="9"]')
  await expect(mount.locator('.text-teal-700').first()).toBeVisible()
  await mount.click()
  await page
    .getByRole('button', { name: 'Storage activity', exact: true })
    .filter({ visible: true })
    .click()
  await expect(page.getByText('Reading media information and generating previews')).toBeVisible()
  await expect(page.getByRole('progressbar', { name: 'Media indexing progress' })).toHaveAttribute(
    'value',
    '3',
  )
  await page.getByRole('button', { name: 'Refresh this folder' }).click()
  await expect
    .poll(() => calls.filter((c) => c.path === '/user/storage/sources/9/scan').map((c) => c.body))
    .toEqual([{ nsId: 9, force: false }])
  await expect(page.getByText('Scan queued', { exact: false }).first()).toBeVisible()
})
