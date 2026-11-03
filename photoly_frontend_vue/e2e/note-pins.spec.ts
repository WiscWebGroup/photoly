import { test, expect } from '@playwright/test'
import { API, mockApi, ok, openDetails, photo, signIn } from './mockApi'
const coordinates = [
  [0, 0],
  [1, 0],
  [0, 1],
  [1, 1],
  [0.5, 0],
  [1, 0.5],
  [0.5, 1],
  [0, 0.5],
  [0.5, 0.5],
  [0.62, 0.62],
]
const longBody = 'A long memory with several details. '.repeat(35)
test.beforeEach(async ({ page }) => {
  await signIn(page)
  const base = {
    revision: 1,
    createdAt: '2026-01-01',
    updatedAt: '2026-01-01',
    contentVersion: 1,
    needsReposition: false,
  }
  await mockApi(page, {
    '/photo/getByNamespace': ok([photo(10, 'Pin landscape'), photo(11, 'Next photo')]),
    '/photo/notes': ({ query }) =>
      ok({
        contentVersion: 1,
        items:
          query.photoId === '10'
            ? [
                { ...base, id: 100, x: null, y: null, body: 'General trip memory' },
                ...coordinates.map(([x, y], i) => ({
                  ...base,
                  id: i + 1,
                  x,
                  y,
                  body: i === 8 ? longBody : `Memory at point ${i + 1}`,
                })),
              ]
            : [],
      }),
  })
  await page.route(`${API}/photo/render/**`, (route) =>
    route.fulfill({
      contentType: 'image/svg+xml',
      headers: { 'Access-Control-Allow-Origin': '*' },
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><defs><linearGradient id="sky" x2="0" y2="1"><stop stop-color="#2465a8"/><stop offset="1" stop-color="#8cb7b0"/></linearGradient></defs><rect width="1200" height="800" fill="url(#sky)"/><path d="M0 670L300 360 580 610 850 280 1200 690V800H0" fill="#244c4b"/></svg>',
    }),
  )
  await page.goto('/home/HomePart')
  await page.locator('[data-photo-id="10"]').click()
  await openDetails(page)
  const notes = page.getByRole('region', { name: 'Notes', exact: true })
  await notes.getByRole('button', { name: /^Notes/ }).click()
  await expect(page.locator('[data-note-overlay] .note-pin')).toHaveCount(10)
})
test('general-only default, point preview, full note and toggle reset', async ({
  page,
  isMobile,
}, testInfo) => {
  const notes = page.getByRole('region', { name: 'Notes', exact: true })
  const pointList = page.getByRole('region', { name: 'Pin-point Notes', exact: true })
  const point = page.getByRole('button', { name: 'Point 9', exact: true })
  await expect(notes.getByText('General trip memory')).toBeVisible()
  await expect(pointList).toHaveCount(0)
  await expect(page.locator('.note-bubble')).toHaveCount(0)
  await point.scrollIntoViewIfNeeded()
  if (isMobile) await point.tap()
  else await point.hover()
  const preview = page.getByRole('article', { name: 'Point 9 preview' })
  await expect(preview).toBeVisible()
  expect(await preview.locator('p').evaluate((p) => p.scrollHeight > p.clientHeight)).toBe(true)
  if (!isMobile) {
    await preview.hover()
    await page.waitForTimeout(200)
    await expect(preview).toBeVisible()
  }
  if (!isMobile) await notes.evaluate((el) => el.scrollIntoView({ block: 'start' }))
  await page.screenshot({
    path: testInfo.outputPath('pinpoint-preview.png'),
    animations: 'disabled',
  })
  await preview.getByRole('button', { name: 'View full note' }).click()
  await expect(pointList.getByText(longBody, { exact: true })).toBeVisible()
  await expect(page.locator('.note-bubble')).toHaveCount(1)
  await notes.getByRole('button', { name: 'See all Pin-point notes (10)', exact: true }).click()
  await expect(page.locator('.note-bubble')).toHaveCount(10)
  await page.locator('[data-note-overlay]').scrollIntoViewIfNeeded()
  if (!isMobile) await notes.evaluate((el) => el.scrollIntoView({ block: 'start' }))
  await page.screenshot({ path: testInfo.outputPath('pinpoint-all.png'), animations: 'disabled' })
  const geometry = await page.locator('[data-photo-zoom]').evaluate((el) => {
    const image = el.querySelector('img')!.getBoundingClientRect()
    const viewport = el.getBoundingClientRect()
    return {
      left: Math.max(image.left, viewport.left),
      top: Math.max(image.top, viewport.top),
      right: Math.min(image.right, viewport.right),
      bottom: Math.min(image.bottom, viewport.bottom),
      bubbles: [...el.querySelectorAll('.note-bubble')].map((b) => {
        const r = b.getBoundingClientRect()
        return { left: r.left, top: r.top, right: r.right, bottom: r.bottom }
      }),
    }
  })
  for (const b of geometry.bubbles) {
    expect(b.left).toBeGreaterThanOrEqual(geometry.left - 1)
    expect(b.top).toBeGreaterThanOrEqual(geometry.top - 1)
    expect(b.right).toBeLessThanOrEqual(geometry.right + 1)
    expect(b.bottom).toBeLessThanOrEqual(geometry.bottom + 1)
  }
  await notes.getByRole('button', { name: 'Hide Pin-point notes (10)', exact: true }).click()
  await expect(page.locator('.note-bubble')).toHaveCount(0)
  await expect(pointList).toHaveCount(0)
  await notes.getByRole('button', { name: 'See all Pin-point notes (10)', exact: true }).click()
  await notes.getByRole('button', { name: /^Notes/ }).click()
  await expect(page.locator('[data-note-overlay]')).toHaveCount(0)
  await notes.getByRole('button', { name: /^Notes/ }).click()
  await expect(page.locator('.note-bubble')).toHaveCount(0)
  await expect(pointList).toHaveCount(0)
})
test('click toggle, keyboard dismissal, zoom and drag keep previews anchored', async ({
  page,
  isMobile,
}, testInfo) => {
  const point = page.getByRole('button', { name: 'Point 9', exact: true })
  await point.scrollIntoViewIfNeeded()
  await point.click()
  await expect(page.locator('#pin-bubble-9')).toBeVisible()
  await point.click()
  await expect(page.locator('#pin-bubble-9')).toHaveCount(0)
  await point.evaluate((el: HTMLElement) => {
    el.blur()
    el.focus()
  })
  // A fresh pointer entry clears the suppression from the previous toggle.
  if (!isMobile) {
    await page.mouse.move(0, 0)
    await point.hover()
  }
  await point.press('Enter')
  await expect(page.locator('#pin-bubble-9')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.locator('.note-bubble')).toHaveCount(0)
  await expect(page.locator('[data-photo-viewer]')).toBeVisible()
  await point.click()
  if (!isMobile) {
    const viewport = page.locator('[data-photo-zoom]')
    const before = await page.locator('#pin-bubble-9').boundingBox()
    await point.hover()
    await page.mouse.wheel(0, -250)
    await expect(viewport).not.toHaveAttribute('data-scale', '1')
    const after = await page.locator('#pin-bubble-9').boundingBox()
    expect(after!.width).toBeCloseTo(before!.width, 0)
    const r = (await viewport.boundingBox())!
    await page.mouse.move(r.x + r.width * 0.7, r.y + r.height * 0.75)
    await page.mouse.down()
    await page.mouse.move(r.x + r.width * 0.7 + 35, r.y + r.height * 0.75 + 25, { steps: 5 })
    await page.mouse.up()
    const image = (await viewport.locator('img').boundingBox())!
    const pin = (await point.boundingBox())!
    expect(Math.abs(pin.x + pin.width / 2 - (image.x + image.width / 2))).toBeLessThan(2)
    expect(Math.abs(pin.y + pin.height / 2 - (image.y + image.height / 2))).toBeLessThan(2)
    await page.getByRole('button', { name: 'Fit to window' }).click()
  }
  await page.screenshot({
    path: testInfo.outputPath('pinpoint-single.png'),
    animations: 'disabled',
  })
  await page.keyboard.press('ArrowRight')
  await expect(page.locator('.note-pin')).toHaveCount(0)
})

test('edge previews remain within the photo after resize and reduced motion', async ({
  page,
  isMobile,
}) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const overlay = page.locator('[data-note-overlay]')
  await overlay.scrollIntoViewIfNeeded()
  for (let i = 1; i <= 8; i++) {
    const point = overlay.getByRole('button', { name: `Point ${i}`, exact: true })
    await point.focus()
    const bubble = page.locator(`#pin-bubble-${i}`)
    await expect(bubble).toBeVisible()
    const image = (await page.locator('[data-photo-zoom] img').boundingBox())!
    const box = (await bubble.boundingBox())!
    expect(box.x).toBeGreaterThanOrEqual(image.x - 1)
    expect(box.y).toBeGreaterThanOrEqual(image.y - 1)
    expect(box.x + box.width).toBeLessThanOrEqual(image.x + image.width + 1)
    expect(box.y + box.height).toBeLessThanOrEqual(image.y + image.height + 1)
    expect(await point.evaluate((el) => getComputedStyle(el).animationName)).toBe('none')
    await page.keyboard.press('Escape')
  }
  if (!isMobile) {
    await overlay.getByRole('button', { name: 'Point 9', exact: true }).click()
    await page.setViewportSize({ width: 1100, height: 850 })
    await expect
      .poll(async () => {
        const image = (await page.locator('[data-photo-zoom] img').boundingBox())!
        const pin = (await overlay
          .getByRole('button', { name: 'Point 9', exact: true })
          .boundingBox())!
        return Math.abs(pin.x + pin.width / 2 - image.x - image.width / 2)
      })
      .toBeLessThan(2)
    const bubble = (await page.locator('#pin-bubble-9').boundingBox())!
    expect(bubble.width).toBeLessThanOrEqual(240)
  }
})
