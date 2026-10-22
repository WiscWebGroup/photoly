import { createServer } from 'node:http'
import { once } from 'node:events'
import type { AddressInfo } from 'node:net'
import { expect, test as base, type Page } from '@playwright/test'
import { API, mockApi, ok, signIn } from './mockApi'

// Receive the actual multipart upload: WebKit's intercepted postData omits file bytes.
const test = base.extend<{ uploads: { files: Buffer[]; failFirst: boolean } }>({
  uploads: async ({ page }, use) => {
    await setup(page)
    const uploads = { files: [] as Buffer[], failFirst: false }
    const server = createServer(async (request, response) => {
      response.setHeader('Access-Control-Allow-Origin', '*')
      response.setHeader('Access-Control-Allow-Headers', '*')
      response.setHeader('Access-Control-Allow-Methods', 'POST, OPTIONS')
      if (request.method === 'OPTIONS') {
        response.writeHead(204)
        response.end()
        return
      }
      const chunks: Buffer[] = []
      for await (const chunk of request) chunks.push(Buffer.from(chunk))
      uploads.files.push(uploadedFile(Buffer.concat(chunks)))
      response.setHeader('Content-Type', 'application/json')
      response.end(
        JSON.stringify(
          uploads.failFirst && uploads.files.length === 1 ? { t: -1, msgCode: 400 } : ok(1),
        ),
      )
    })
    server.listen(0, '127.0.0.1')
    await once(server, 'listening')
    const port = (server.address() as AddressInfo).port
    await page.route(API + '/user/updateAvatar', (route) =>
      route.continue({ url: `http://127.0.0.1:${port}/avatar` }),
    )
    try {
      await use(uploads)
    } finally {
      server.closeAllConnections()
      await new Promise<void>((resolve) => server.close(() => resolve()))
    }
  },
})

async function setup(page: Page) {
  await signIn(page)
  await mockApi(page, {
    '/user/getInfo': ok({
      userId: 1,
      userName: 'Alice',
      email: 'alice@example.com',
      createDate: '2024-01-01',
      role: 'user',
    }),
  })
  await page.goto('/home/ManagePart')
  await page.getByRole('button', { name: 'Change Picture', exact: true }).click()
  await expect(page.getByTestId('avatar-crop')).toBeVisible()
}
async function imageFile(page: Page, width: number, height: number, transparent = false) {
  const data = await page.evaluate(
    ({ width, height, transparent }) => {
      const canvas = document.createElement('canvas')
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')!
      if (!transparent) {
        ctx.fillStyle = 'red'
        ctx.fillRect(0, 0, width, height)
        ctx.fillStyle = 'blue'
        ctx.fillRect(width / 2, 0, width / 2, height)
      }
      return canvas.toDataURL('image/png').split(',')[1]!
    },
    { width, height, transparent },
  )
  return { name: 'avatar.png', mimeType: 'image/png', buffer: Buffer.from(data, 'base64') }
}
function uploadedFile(body: Buffer) {
  const start = body.indexOf('\r\n\r\n') + 4
  const end = body.lastIndexOf('\r\n--')
  return body.subarray(start, end)
}
async function inspectJpeg(page: Page, bytes: Buffer) {
  return page.evaluate(async (data) => {
    const img = new Image()
    img.src = `data:image/jpeg;base64,${data}`
    await img.decode()
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = 512
    const ctx = canvas.getContext('2d')!
    ctx.drawImage(img, 0, 0)
    return {
      width: img.naturalWidth,
      height: img.naturalHeight,
      corner: [...ctx.getImageData(0, 0, 1, 1).data],
      center: [...ctx.getImageData(256, 256, 1, 1).data],
    }
  }, bytes.toString('base64'))
}

for (const size of [
  { width: 800, height: 400 },
  { width: 400, height: 800 },
]) {
  test(`crop ${size.width}x${size.height}, pan and save a square JPEG`, async ({
    page,
    uploads,
  }, testInfo) => {
    await page
      .getByLabel('Upload Avatar')
      .setInputFiles(await imageFile(page, size.width, size.height))
    await expect(page.getByRole('button', { name: 'Submit', exact: true })).toBeEnabled()
    await page.getByLabel('Zoom', { exact: true }).fill('2')
    const crop = page.getByTestId('avatar-crop')
    await crop.focus()
    await page.keyboard.press('ArrowRight')
    const box = (await crop.boundingBox())!
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.down()
    await page.mouse.move(box.x + box.width - 4, box.y + box.height / 2, { steps: 5 })
    await page.mouse.up()
    await page.screenshot({ path: testInfo.outputPath('avatar-crop.png') })
    await page.getByRole('button', { name: 'Submit', exact: true }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    const saved = uploads.files[0]!
    expect(saved?.subarray(0, 2).toString('hex')).toBe('ffd8')
    const decoded = await inspectJpeg(page, saved!)
    expect([decoded.width, decoded.height]).toEqual([512, 512])
    expect(decoded.corner[0]).toBeGreaterThan(240)
    expect(decoded.corner[1]).toBeLessThan(15)
    expect(decoded.center[0]).toBeGreaterThan(240)
    await expect(page.locator('img[src*="/user/getAvatar/"]').first()).toHaveAttribute(
      'src',
      /\?v=1$/,
    )
  })
}

test('transparent image becomes white; failed upload retains crop and can retry', async ({
  page,
  uploads,
}) => {
  uploads.failFirst = true
  await page.getByLabel('Upload Avatar').setInputFiles(await imageFile(page, 300, 800, true))
  await expect(page.getByRole('button', { name: 'Submit', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Submit', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveText('Could not save the avatar. Please try again.')
  await expect(page.getByTestId('avatar-crop')).toBeVisible()
  await page.getByRole('button', { name: 'Submit', exact: true }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  expect((await inspectJpeg(page, uploads.files.at(-1)!)).corner).toEqual([255, 255, 255, 255])
  expect(uploads.files).toHaveLength(2)
})

test('cancel sends no upload, and invalid images can be replaced', async ({ page, uploads }) => {
  await page
    .getByLabel('Upload Avatar')
    .setInputFiles({ name: 'bad.png', mimeType: 'image/png', buffer: Buffer.from('bad') })
  await expect(page.getByRole('alert')).toContainText('Could not load this image')
  await page.getByLabel('Upload Avatar').setInputFiles(await imageFile(page, 400, 300))
  await expect(page.getByRole('alert')).toHaveCount(0)
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  expect(uploads.files).toEqual([])
})
