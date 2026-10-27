import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

test.beforeEach(async ({ page, isMobile }) => {
  test.skip(!isMobile, 'the details sheet is phone-only')
  await signIn(page)
  await mockApi(page, { '/photo/getByNamespace': ok([photo(10, 'beach')]) })
  await page.goto('/home/HomePart')
  await page.getByText('beach.jpg', { exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
})

const sheet = (page: Page) => page.getByRole('complementary', { name: 'Photo details' })
const photoArea = (page: Page) => page.getByRole('region', { name: 'Photo' })
const title = (page: Page) => sheet(page).getByRole('heading', { name: 'beach', exact: true })
/** The sheet's controls sit under an `inert` wrapper while it is pulled down. */
const outOfReach = (page: Page) =>
  page.getByRole('switch', { name: 'Visibility' }).evaluate((el) => !!el.closest('[inert]'))
const height = async (page: Page) => (await photoArea(page).boundingBox())!.height

/** A one-finger vertical drag, sent as real touch events. */
async function drag(page: Page, x: number, from: number, to: number) {
  const cdp = await page.context().newCDPSession(page)
  const touch = (type: 'touchStart' | 'touchMove' | 'touchEnd', y?: number) =>
    cdp.send('Input.dispatchTouchEvent', {
      type,
      touchPoints: y === undefined ? [] : [{ x, y }],
    })
  await touch('touchStart', from)
  for (let i = 1; i <= 8; i++) await touch('touchMove', from + ((to - from) * i) / 8)
  await touch('touchEnd')
}
async function dragSheet(page: Page, by: number) {
  const box = (await title(page).boundingBox())!
  await drag(page, box.x + 20, box.y + box.height / 2, box.y + box.height / 2 + by)
}

test('the handle hides the sheet so the photo fills the screen, and brings it back', async ({
  page,
}) => {
  const handle = page.getByRole('button', { name: 'Hide details' })
  await expect(handle).toHaveAttribute('aria-expanded', 'true')
  const shown = await height(page)

  await handle.click()
  const show = page.getByRole('button', { name: 'Show details' })
  await expect(show).toHaveAttribute('aria-expanded', 'false')
  await expect.poll(() => height(page)).toBeGreaterThan(shown + 150)
  // Only the title peeks out; the rest of the sheet is out of reach.
  await expect(title(page)).toBeInViewport({ ratio: 1 })
  expect(await outOfReach(page)).toBe(true)

  await show.click()
  await expect(page.getByRole('button', { name: 'Hide details' })).toBeVisible()
  await expect.poll(() => height(page)).toBeLessThan(shown + 1)
  expect(await outOfReach(page)).toBe(false)
})

test('pull the sheet down to hide it, and up to show it again', async ({ page }) => {
  const shown = await height(page)
  await dragSheet(page, 20) // too short: it springs back
  await expect(page.getByRole('button', { name: 'Hide details' })).toBeVisible()

  await dragSheet(page, 140)
  await expect(page.getByRole('button', { name: 'Show details' })).toBeVisible()
  await expect.poll(() => height(page)).toBeGreaterThan(shown + 150)

  await dragSheet(page, -140)
  await expect(page.getByRole('button', { name: 'Hide details' })).toBeVisible()
  await expect.poll(() => height(page)).toBeLessThan(shown + 1)
})

test('pulling down mid-scroll scrolls the sheet instead of hiding it', async ({ page }) => {
  const scroller = page.getByTestId('photo-viewer-scroll')
  // Make sure there is something to scroll however short the sheet's content is.
  await scroller.evaluate((el) => {
    const spacer = document.createElement('div')
    spacer.style.cssText = 'height:600px;flex:none'
    el.append(spacer)
    el.scrollTo({ top: 120, behavior: 'instant' })
  })
  await expect.poll(() => scroller.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
  const box = (await sheet(page).boundingBox())!
  await drag(page, box.x + 40, box.y + 200, box.y + 300)
  await expect(page.getByRole('button', { name: 'Hide details' })).toBeAttached()
  await expect(page.getByRole('button', { name: 'Show details' })).toHaveCount(0)
})
