import { deflateSync } from 'node:zlib'
import { expect, test } from '@playwright/test'
import { API, mockApi, ok, photo, signIn } from './mockApi'

/** A solid-colour PNG, so the editor has a real picture to decode and re-encode. */
function png(width: number, height: number) {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    return c >>> 0
  })
  const crc = (buf: Buffer) => {
    let c = 0xffffffff
    for (const b of buf) c = crcTable[(c ^ b) & 0xff]! ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type: string, data: Buffer) => {
    const body = Buffer.concat([Buffer.from(type), data])
    const out = Buffer.alloc(body.length + 8)
    out.writeUInt32BE(data.length, 0)
    body.copy(out, 4)
    out.writeUInt32BE(crc(body), body.length + 4)
    return out
  }
  const header = Buffer.alloc(13)
  header.writeUInt32BE(width, 0)
  header.writeUInt32BE(height, 4)
  header.set([8, 2, 0, 0, 0], 8)
  // Noisy rows so JPEG quality visibly changes the size.
  const rows = Buffer.alloc((width * 3 + 1) * height)
  for (let i = 0; i < rows.length; i++) rows[i] = i % (width * 3 + 1) ? (i * 2654435761) >>> 24 : 0
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(rows)),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

async function openEditor(
  page: import('@playwright/test').Page,
  extra: Record<string, object> = {},
) {
  const calls = await mockApi(page, {
    '/photo/getByNamespace': ok([photo(10, 'beach', { format: 'png' })]),
    '/photo/getPhotoExif': ok({}),
    '/photo/insert': ok(1),
    '/photo/replace': ok(1),
    ...extra,
  })
  const picture = png(400, 300)
  // Registered after mockApi's catch-all, so it wins for the full-size picture.
  await page.route(`${API}/photo/render/**`, (route) =>
    route.fulfill({
      body: picture,
      contentType: 'image/png',
      headers: { 'access-control-allow-origin': '*' },
    }),
  )
  await page.goto('/home/HomePart')
  await page.getByText('beach.png').click()
  await page.getByRole('button', { name: 'Edit Image' }).click()
  await expect(page.getByText('Cropped size: 400 × 300 px')).toBeVisible()
  return calls
}

test('rotate, drag-crop and save as a new image', async ({ page }) => {
  const calls = await openEditor(page)
  await page.getByRole('button', { name: 'Right' }).click()
  await expect(page.getByText('Cropped size: 300 × 400 px')).toBeVisible()

  // Drag the south-east corner to the middle of the picture.
  const stage = (await page.getByTestId('editor-stage').boundingBox())!
  const corner = (await page.locator('[data-handle=se]').boundingBox())!
  await page.mouse.move(corner.x + corner.width / 2, corner.y + corner.height / 2)
  await page.mouse.down()
  await page.mouse.move(stage.x + stage.width / 2, stage.y + stage.height / 2, { steps: 5 })
  await page.mouse.up()
  await expect(page.getByText(/Cropped size: 1[4-6]\d × 2[01]\d px/)).toBeVisible()

  await page.getByRole('button', { name: 'Save as new image' }).click()
  await expect(page.getByText('Saved as a new image (private)')).toBeVisible()
  const upload = calls.find((c) => c.path === '/photo/insert')!
  expect(String(upload.body)).toContain('filename="beach_edited.png"')
  expect(String(upload.body)).toContain('{"nsId":1,"visibility":0}')
})

test('reduce to a file size and overwrite after confirming', async ({ page }) => {
  const calls = await openEditor(page)
  await page.getByLabel('Limit file size').click()
  await page.getByLabel('Maximum file size').fill('20')
  await expect(page.getByText(/Result: \d+\.\d KB \(400 × 300 px, quality \d+%\)/)).toBeVisible()
  await expect(page.getByText('saved as JPEG')).toBeVisible()

  await page.getByRole('button', { name: 'Overwrite original' }).click()
  await expect(page.getByText('Overwrite the original?')).toBeVisible()
  expect(calls.some((c) => c.path === '/photo/replace')).toBe(false)
  await page.getByRole('button', { name: 'Overwrite', exact: true }).click()
  await expect(page.getByText('Image overwritten')).toBeVisible()
  const replace = calls.find((c) => c.path === '/photo/replace')!
  expect(String(replace.body)).toContain('filename="beach.jpg"')
})
