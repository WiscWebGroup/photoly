import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

test.beforeEach(async ({ page, isMobile }) => {
  test.skip(!isMobile, 'the floating "+" button is phone-only')
  await signIn(page)
  await mockApi(page, {
    '/photo/getByNamespace': ok(Array.from({ length: 40 }, (_, i) => photo(10 + i, `p${i}`))),
  })
  await page.goto('/home/HomePart')
  await expect(page.getByText('p0.jpg', { exact: true })).toBeVisible()
})

/** A finger tap (held briefly, as real taps are), sent as touch events. */
async function tap(page: Page, x: number, y: number) {
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] })
  await page.waitForTimeout(100)
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
}
/** The "+": by label, since the open menu hides the rest of the page from assistive tech. */
async function tapAdd(page: Page) {
  const add = page.locator('button[aria-label="Add"]')
  await add.click({ trial: true }) // waits until it has settled and nothing covers it
  const box = (await add.boundingBox())!
  await tap(page, box.x + box.width / 2, box.y + box.height / 2)
}
/** Open the menu and let its entrance animation settle. */
async function openMenu(page: Page) {
  await tapAdd(page)
  await expect(menu(page)).toBeVisible()
  await page.waitForTimeout(400)
}
const menu = (page: Page) => page.getByRole('menu')
const viewer = (page: Page) => page.getByRole('dialog')

test('tapping the "+" again closes its menu without opening the photo under it', async ({
  page,
}) => {
  await openMenu(page)
  await tapAdd(page)
  await expect(menu(page)).toHaveCount(0)
  await page.waitForTimeout(400)
  await expect(viewer(page)).toHaveCount(0)
})

test('tapping beside the open menu only closes it', async ({ page }) => {
  await openMenu(page)
  const thumb = (await page.getByText('p1.jpg', { exact: true }).boundingBox())!
  await tap(page, thumb.x + 10, thumb.y + thumb.height / 2)
  await expect(menu(page)).toHaveCount(0)
  await page.waitForTimeout(400)
  await expect(viewer(page)).toHaveCount(0)

  // With the menu closed, photos open as usual.
  await tap(page, thumb.x + 10, thumb.y + thumb.height / 2)
  await expect(viewer(page)).toBeVisible()
})

test('menu items still work', async ({ page }) => {
  await openMenu(page)
  const item = (await page.getByRole('menuitem', { name: 'New Folder' }).boundingBox())!
  await tap(page, item.x + 20, item.y + item.height / 2)
  await expect(page.getByRole('dialog', { name: 'New Folder' })).toBeVisible()
})
