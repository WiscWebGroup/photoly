import { expect, test, type Page } from '@playwright/test'
import type { MapEngine } from '../src/lib/mapEngine'

declare global {
  interface Window {
    lightingTest: MapEngine
  }
}

async function harness(page: Page) {
  await page.clock.setFixedTime(new Date('2026-03-20T06:00:00Z'))
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.route('**/__day-night-test', (route) =>
    route.fulfill({
      contentType: 'text/html',
      body: '<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body style="margin:0"><div id="map" style="position:absolute;inset:0"></div><div style="position:absolute;top:12px;left:12px;color:white;font:14px sans-serif;pointer-events:none">2026-03-20 06:00 UTC · synthetic test tiles</div></body></html>',
    }),
  )
  await page.goto('/__day-night-test')
  const png = await page.evaluate(() => {
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = 256
    const ctx = canvas.getContext('2d')!
    ctx.fillStyle = '#c5d9dc'
    ctx.fillRect(0, 0, 256, 256)
    return canvas.toDataURL().split(',')[1]!
  })
  await page.route('https://tile.openstreetmap.org/**', (route) =>
    route.fulfill({
      contentType: 'image/png',
      headers: { 'access-control-allow-origin': '*' },
      body: Buffer.from(png, 'base64'),
    }),
  )
  await page.evaluate(async () => {
    const path = '/src/lib/maplibreEngine.ts'
    const { createMapLibreEngine } = await import(/* @vite-ignore */ path)
    const css = document.createElement('link')
    css.rel = 'stylesheet'
    css.href = '/src/lib/photoMap.css'
    document.head.append(css)
    const el = document.getElementById('map')!
    el.dataset.engine = 'maplibre'
    el.dataset.projection = 'globe'
    el.dataset.background = 'gradient'
    window.lightingTest = await createMapLibreEngine(el, 'globe')
    window.lightingTest.initialize([[0, 0]])
    window.lightingTest.dayNight(true)
  })
  await expect(page.locator('#map')).toHaveAttribute('data-ready', 'true')
  await expect(page.locator('#map')).toHaveAttribute('data-day-night-available', 'true')
  await expect(page.locator('#map')).toHaveAttribute('data-day-night', 'true')
}

// Decode screenshots instead of reading an already-cleared WebGL drawing buffer.
async function samples(page: Page, positions: number[][]) {
  // Wait for the map render and the following compositor frame, including WebKit.
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
      }),
  )
  const png = (await page.locator('#map').screenshot({ scale: 'css' })).toString('base64')
  return page.evaluate(
    async ({ png, positions }) => {
      const image = new Image()
      image.src = `data:image/png;base64,${png}`
      await image.decode()
      const canvas = document.createElement('canvas')
      canvas.width = image.width
      canvas.height = image.height
      const ctx = canvas.getContext('2d')!
      ctx.drawImage(image, 0, 0)
      return positions.map(([x, y]) =>
        Array.from(
          ctx.getImageData(Math.floor(x! * image.width), Math.floor(y! * image.height), 1, 1).data,
        ).slice(0, 3),
      )
    },
    { png, positions },
  )
}

async function view(page: Page, longitude: number, latitude: number, zoom?: number) {
  await page.evaluate(
    async ({ longitude, latitude, zoom }) => {
      await new Promise<void>((resolve) =>
        window.lightingTest.flyTo(
          [longitude, latitude],
          zoom ?? window.lightingTest.camera().zoom,
          resolve,
        ),
      )
    },
    { longitude, latitude, zoom },
  )
  await expect(page.locator('#map')).toHaveAttribute('data-ready', 'true')
}

test('real sunlight, soft terminator, stable poles and date line, and zoom fade', async ({
  page,
}, info) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await harness(page)
  const positions = [
    [0.42, 0.5],
    [0.58, 0.5],
    [0.02, 0.5],
    [0.5, 0.5],
  ]
  const lit = await samples(page, positions)
  expect(lit[1]![0]! - lit[0]![0]!).toBeGreaterThan(65)
  expect(lit[3]![0]).toBeGreaterThan(lit[0]![0]!)
  expect(lit[3]![0]).toBeLessThan(lit[1]![0]!)
  await page.screenshot({ path: info.outputPath('terminator.png') })
  await page.evaluate(() => window.lightingTest.dayNight(false))
  await expect
    .poll(async () => {
      const off = await samples(page, positions)
      return Math.abs(off[0]![0]! - off[1]![0]!)
    })
    .toBeLessThan(3)
  const off = await samples(page, positions)
  expect(Math.abs(off[0]![0]! - off[1]![0]!)).toBeLessThan(3)
  expect(off[2]).toEqual(lit[2]) // Space outside the sphere is untouched.
  await page.evaluate(() => window.lightingTest.dayNight(true))
  await view(page, 90, 0)
  const day = (await samples(page, [[0.5, 0.5]]))[0]!
  await page.screenshot({ path: info.outputPath('day.png') })
  await view(page, -90, 0)
  const night = (await samples(page, [[0.5, 0.5]]))[0]!
  expect(day[0]! - night[0]!).toBeGreaterThan(65)
  await page.screenshot({ path: info.outputPath('night.png') })
  await view(page, -90, 0, 4.5)
  const halfway = (await samples(page, [[0.5, 0.5]]))[0]!
  expect(halfway[0]).toBeGreaterThan(night[0]! + 30)
  await view(page, -90, 0, 6)
  await expect(page.locator('#map')).toHaveAttribute('data-day-night', 'false')
  const city = (await samples(page, [[0.5, 0.5]]))[0]!
  expect(city[0]).toBeGreaterThan(halfway[0]! + 30)
  expect(city[0]).toBeGreaterThan(day[0]! + 10)
  await page.screenshot({ path: info.outputPath('city.png') })
  for (const [name, longitude, latitude] of [
    ['date-line', 180, 0],
    ['north-pole', 0, 85],
    ['south-pole', 0, -85],
  ] as const) {
    // MapLibre zoom measures Mercator scale; compensate latitude to keep the whole
    // globe in frame, including the cap beyond the raster tiles' ±85.051° limit.
    await view(page, longitude, latitude, 1.5 + Math.log2(Math.cos((latitude * Math.PI) / 180)))
    await expect(page.locator('#map')).toHaveAttribute('data-day-night', 'true')
    await page.screenshot({ path: info.outputPath(`${name}.png`) })
    const center = (await samples(page, [[0.5, 0.5]]))[0]!
    expect(center[0]).toBeGreaterThan(70) // No hole or double-darkened overlapping hemisphere.
  }
  expect(errors).toEqual([])
  await page.evaluate(() => window.lightingTest.destroy())
})
