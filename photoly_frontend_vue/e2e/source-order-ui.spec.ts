import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn, photo } from './mockApi'

async function mode(page: import('@playwright/test').Page, name: string) {
  if (page.viewportSize()!.width >= 768)
    await page.getByRole('button', { name, exact: true }).click()
  else {
    await page.getByRole('button', { name: 'Add', exact: true }).click()
    await page.getByRole('menuitem', { name, exact: true }).click()
  }
}
test('folder handles persist order across layouts and reload without moving files', async ({
  page,
}, info) => {
  await signIn(page)
  let folders = [
    { nsId: 2, nsName: 'Trips' },
    { nsId: 3, nsName: 'Archive' },
    { nsId: 4, nsName: 'Cloud', sourceType: 's3', isMount: true },
  ]
  const calls = await mockApi(page, {
    '/namespace/getChildren': () => ok(folders),
    '/namespace/order': ({ body }) => {
      const { nsId, beforeNsId } = body as { nsId: number; beforeNsId: number | null }
      const moved = folders.find((f) => f.nsId === nsId)!
      folders = folders.filter((f) => f.nsId !== nsId)
      folders.splice(
        beforeNsId === null ? folders.length : folders.findIndex((f) => f.nsId === beforeNsId),
        0,
        moved,
      )
      return ok(folders)
    },
  })
  await page.goto('/home/HomePart')
  const order = () =>
    page
      .locator('[data-order-id]')
      .evaluateAll((nodes) => nodes.map((n) => Number((n as HTMLElement).dataset.orderId)))
  await page.getByRole('button', { name: 'Reorder Cloud', exact: true }).focus()
  await page.keyboard.press('Home')
  await expect.poll(order).toEqual([4, 2, 3])
  for (const layout of ['Small cards', 'List view', 'Large cards']) {
    await mode(page, layout)
    await expect.poll(order).toEqual([4, 2, 3])
  }
  await page.reload()
  await expect.poll(order).toEqual([4, 2, 3])
  await mode(page, 'List view')
  const handle = page.getByRole('button', { name: 'Reorder Archive', exact: true })
  const start = (await handle.boundingBox())!,
    end = (await page.locator('[data-order-id="4"]').boundingBox())!
  if (info.project.name === 'mobile') {
    const touch = await page.context().newCDPSession(page)
    await touch.send('Input.dispatchTouchEvent', {
      type: 'touchStart',
      touchPoints: [{ x: start.x + start.width / 2, y: start.y + start.height / 2 }],
    })
    await touch.send('Input.dispatchTouchEvent', {
      type: 'touchMove',
      touchPoints: [{ x: end.x + end.width / 2, y: end.y + 3 }],
    })
    await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
    await touch.detach()
  } else {
    await page.mouse.move(start.x + start.width / 2, start.y + start.height / 2)
    await page.mouse.down()
    await page.mouse.move(end.x + end.width / 2, end.y + 3, { steps: 8 })
    await page.mouse.up()
  }
  await expect.poll(order).toEqual([3, 4, 2])
  expect(calls.filter((c) => c.path === '/namespace/order').length).toBe(2)
  expect(calls.some((c) => c.path === '/namespace/updateParent')).toBe(false)
  await expect(page.locator('[data-folder-id="2"]')).toHaveAttribute('draggable', 'true')
  await expect(page.locator('[data-folder-id="4"]')).toHaveAttribute('draggable', 'false')
  await page.screenshot({ path: info.outputPath('folder-order.png') })
})
test('S3 list saves and collapses forms; buttons align and sidebar toggle is centered', async ({
  page,
}, info) => {
  await signIn(page)
  type Connection = {
    sourceId: number
    name: string
    enabled: boolean
    canRemove: boolean
    values: Record<string, string>
  }
  const connections: Connection[] = []
  const calls = await mockApi(page, {
    '/user/storage/s3': ({ method, body }) => {
      if (method === 'POST') {
        const b = body as { name: string; values: Record<string, string> }
        connections.push({
          sourceId: connections.length + 1,
          name: b.name,
          enabled: true,
          canRemove: true,
          values: b.values,
        })
        return ok({ name: b.name })
      }
      return ok({
        connections,
        canSave: true,
        fields: [
          { name: 'endpoint', label: 'Endpoint', kind: 'text' },
          { name: 'bucket', label: 'Bucket', kind: 'text' },
        ],
      })
    },
    '/user/storage/sources': ok([
      {
        sourceId: 1,
        name: 'Photoly',
        sourceType: 'local',
        enabled: true,
        scanStatus: 'ready',
        scanPhase: 'complete',
      },
    ]),
    '/user/storage/webdav': ok([]),
    '/user/storage/smb': ok([]),
  })
  await page.goto('/home/ManagePart#storage')
  const scan = page.getByRole('button', { name: 'Scan source', exact: true }),
    refresh = page.getByRole('button', { name: 'Refresh status', exact: true })
  await expect(scan).toBeVisible()
  const a = (await scan.boundingBox())!,
    b = (await refresh.boundingBox())!
  expect(a.height).toBe(b.height)
  if (Math.abs(a.y - b.y) < 2) expect(b.x - a.x - a.width).toBeGreaterThanOrEqual(7)
  if (page.viewportSize()!.width >= 768) {
    const toggle = (await page.getByRole('button', { name: 'Hide sidebar' }).boundingBox())!
    expect(toggle.y + toggle.height / 2).toBeCloseTo(28, 0)
  }
  for (const alias of ['Trips', 'Backup']) {
    await page.getByRole('button', { name: 'Add S3', exact: true }).click()
    await page.getByLabel('Alias', { exact: true }).fill(alias)
    await page.getByLabel('Endpoint', { exact: true }).fill('s3.example')
    await page.getByLabel('Bucket', { exact: true }).fill('photos')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(page.getByLabel('Alias', { exact: true })).toHaveCount(0)
  }
  expect(calls.filter((c) => c.path === '/user/storage/s3' && c.method === 'POST')).toHaveLength(2)
  await expect(
    page.getByRole('region', { name: 'S3 connections' }).getByText('Backup', { exact: true }),
  ).toBeVisible()
  await page.screenshot({ path: info.outputPath('multi-s3-settings.png') })
})
test('gallery map clips landscape and portrait thumbnails but keeps count badges outside', async ({
  page,
}, info) => {
  await signIn(page)
  await mockApi(page, {
    '/gallery/getAll': ok([{ gaId: 7, gaName: 'Travel', createDate: '2024-06-01' }]),
    '/photo/getByGallery': ok([photo(10, 'Beach'), photo(11, 'Mountain'), photo(12, 'Portrait')]),
    '/photo-extra/getGlobalMapPhotoByGallery': ok(
      [10, 11, 12].map((id) => ({
        photo_id: id,
        photo_name: String(id),
        extra_photo_gps_lat: id === 12 ? 40 : 30,
        extra_photo_gps_lon: 110,
      })),
    ),
  })
  await page.route('**/photo/renderThumbnail/*', (route) => {
    const portrait = route.request().url().includes('photoId=12')
    return route.fulfill({
      contentType: 'image/svg+xml',
      body: `<svg xmlns="http://www.w3.org/2000/svg" width="${portrait ? 200 : 800}" height="${portrait ? 800 : 200}"><rect width="100%" height="100%" fill="#168d8a"/></svg>`,
    })
  })
  await page.goto('/home/GalleryPart')
  await page.getByText('Travel', { exact: true }).click()
  await expect(page.locator('.gallery-map-image')).toHaveCount(2)
  for (const crop of await page.locator('.gallery-map-image').all()) {
    await expect(crop).toHaveCSS('overflow', 'hidden')
    const box = (await crop.boundingBox())!,
      img = (await crop.locator('img').boundingBox())!
    expect(img.width).toBeLessThanOrEqual(box.width)
    expect(img.height).toBeLessThanOrEqual(box.height)
  }
  await expect(page.locator('.gallery-map-pin > span')).toHaveText('2')
  await page.screenshot({ path: info.outputPath('gallery-marker-crop.png') })
})
