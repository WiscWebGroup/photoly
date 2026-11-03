import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { test, expect, type Page } from '@playwright/test'
import { API, mockApi, ok, photo, signIn } from './mockApi'

const videoBytes = readFileSync(fileURLToPath(new URL('./fixtures/player.mp4', import.meta.url)))

const portraitBytes = readFileSync(
  fileURLToPath(new URL('./fixtures/player-portrait.mp4', import.meta.url)),
)

async function openVideo(page: Page, failInitially = false, bytes = videoBytes) {
  await signIn(page)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await mockApi(page, {
    '/photo/getByNamespace': ok([
      photo(10, 'Player sample', { format: 'mp4' }),
      photo(11, 'Next sample', { format: 'mp4' }),
      photo(12, 'Still photo'),
    ]),
  })
  let failing = failInitially
  await page.route(`${API}/photo/renderV/**`, (route) => {
    if (failing) return route.fulfill({ status: 404 })
    const range = route
      .request()
      .headers()
      .range?.match(/bytes=(\d+)-(\d*)/)
    const start = range ? Number(range[1]) : 0
    const end = range?.[2] ? Math.min(Number(range[2]), bytes.length - 1) : bytes.length - 1
    return route.fulfill({
      status: range ? 206 : 200,
      contentType: 'video/mp4',
      headers: {
        'access-control-allow-origin': '*',
        'cross-origin-resource-policy': 'cross-origin',
        'accept-ranges': 'bytes',
        ...(range ? { 'content-range': `bytes ${start}-${end}/${bytes.length}` } : {}),
      },
      body: bytes.subarray(start, end + 1),
    })
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Video Player sample.mp4', exact: true }).click()
  const player = page.getByRole('region', { name: 'Video player' })
  await expect(player).toBeVisible()
  return {
    player,
    video: player.locator('video'),
    recover: () => {
      failing = false
    },
  }
}

test('video plays, seeks, changes speed and exposes shipped license notices', async ({
  page,
}, testInfo) => {
  const { player, video } = await openVideo(page)
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState))
    .toBeGreaterThanOrEqual(1)
  expect(await video.evaluate((v: HTMLVideoElement) => v.paused)).toBe(true)
  await player.focus()
  await page.keyboard.press('Space')
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime))
    .toBeGreaterThan(0.2)
  await page.keyboard.press('Space')
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.paused)).toBe(true)
  await page.keyboard.press('ArrowRight')
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime)).toBeGreaterThan(5)
  await expect(page.getByRole('dialog', { name: 'Player sample.mp4', exact: true })).toBeVisible()

  await player.hover()
  const progress = player.locator('.art-control-progress')
  const box = await progress.boundingBox()
  expect(box).toBeTruthy()
  if (testInfo.project.name === 'mobile')
    await progress.tap({ position: { x: box!.width * 0.6, y: box!.height / 2 } })
  else await progress.click({ position: { x: box!.width * 0.6, y: box!.height / 2 } })
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime))
    .toBeGreaterThan(10)
  await player.locator('.art-control-setting').click()
  await expect(player.getByRole('link', { name: 'Third-party licenses' })).toHaveAttribute(
    'href',
    '/player-licenses.txt',
  )
  await player.getByText('Play Speed', { exact: true }).click()
  await player.locator('.art-settings').getByText('1.5', { exact: true }).click()
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.playbackRate)).toBe(1.5)

  const rect = await player.boundingBox()
  expect(rect!.width).toBeLessThanOrEqual(page.viewportSize()!.width)
  expect(rect!.height).toBeLessThan(page.viewportSize()!.height)
  expect(rect!.width / rect!.height).toBeCloseTo(16 / 9, 1)
  await page.screenshot({ path: testInfo.outputPath('video-player.png') })
  const license = await page.request.get('/player-licenses.txt')
  expect(license.ok()).toBe(true)
  const notices = await license.text()
  for (const name of ['artplayer@5.4.0', 'option-validator', 'kind-of', 'screenfull', 'Hint.css'])
    expect(notices).toContain(name)
  expect(notices).toContain('Permission is hereby granted')
  await page.keyboard.press('Escape')
  await expect(page.locator('[data-photo-viewer]')).toHaveCount(0)
})

test('failed playback retries and navigation stops the previous video', async ({ page }) => {
  const { player, video, recover } = await openVideo(page, true)
  await expect(page.getByText('Could not load this media. Please retry.')).toBeVisible()
  recover()
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState))
    .toBeGreaterThanOrEqual(1)
  await expect(page.getByText('Could not load this media. Please retry.')).toBeHidden()
  await player.focus()
  await page.keyboard.press('Space')
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.paused)).toBe(false)
  const oldVideo = await video.elementHandle()
  // Focus outside the player: the viewer's usual navigation shortcut still works.
  await page.locator('[data-photo-viewer]').focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('dialog', { name: 'Next sample.mp4', exact: true })).toBeVisible()
  await expect.poll(() => oldVideo!.evaluate((v: HTMLVideoElement) => v.paused)).toBe(true)
  await expect(player.locator('video')).toHaveCount(1)
  await page.keyboard.press('Escape')
  await expect(page.locator('[data-photo-viewer]')).toHaveCount(0)
})

test('fullscreen Escape is consumed before viewer close, and player swipes stay local', async ({
  page,
}) => {
  const { player, video } = await openVideo(page)
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.readyState))
    .toBeGreaterThanOrEqual(1)
  await player.evaluate((element) => {
    // Synthetic touch events work in WebKit too, where the Touch constructor is absent.
    for (const [type, x] of [
      ['touchstart', 250],
      ['touchmove', 30],
      ['touchend', 30],
    ] as const) {
      const event = new Event(type, { bubbles: true, cancelable: true })
      const touch = {
        identifier: 1,
        target: element,
        clientX: x,
        clientY: 200,
        pageX: x,
        pageY: 200,
      }
      Object.defineProperty(event, 'touches', { value: type === 'touchend' ? [] : [touch] })
      Object.defineProperty(event, 'changedTouches', { value: [touch] })
      element.dispatchEvent(event)
    }
  })
  await expect(page.getByRole('dialog', { name: 'Player sample.mp4', exact: true })).toBeVisible()
  if (await page.evaluate(() => document.fullscreenEnabled)) {
    await player.hover()
    await player.locator('.art-control-fullscreen').click()
    await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(true)
    await page.keyboard.press('Escape')
    await expect.poll(() => page.evaluate(() => !!document.fullscreenElement)).toBe(false)
    await expect(page.getByRole('dialog', { name: 'Player sample.mp4', exact: true })).toBeVisible()
  }
  if (await page.evaluate(() => !!document.pictureInPictureEnabled))
    await expect(player.locator('.art-control-pip')).toBeAttached()
  await page.keyboard.press('Escape')
  await expect(page.locator('[data-photo-viewer]')).toHaveCount(0)
})

test('portrait clips fit the stage and retain usable controls', async ({ page }, testInfo) => {
  const { player, video } = await openVideo(page, false, portraitBytes)
  await expect.poll(() => video.evaluate((v: HTMLVideoElement) => v.videoHeight)).toBe(320)
  await expect
    .poll(() => player.evaluate((el) => el.getBoundingClientRect().width))
    .toBeGreaterThanOrEqual(320)
  const bounds = await player.boundingBox()
  expect(bounds!.height).toBeLessThan(page.viewportSize()!.height)
  expect(bounds!.x).toBeGreaterThanOrEqual(0)
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width)
  await player.hover()
  await player.locator('.art-control-setting').click()
  await expect(player.getByRole('link', { name: 'Third-party licenses' })).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('portrait-player.png') })
})
