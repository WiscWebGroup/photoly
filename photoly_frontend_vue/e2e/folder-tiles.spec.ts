import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

const addTile = (page: Page) =>
  page.getByRole('listitem').getByRole('button', { name: 'New Folder', exact: true })

/** Heights of the folder cards and of the "New Folder" tile after them. */
async function heights(page: Page) {
  const all = await page
    .getByRole('list')
    .filter({ has: addTile(page) })
    .locator(':scope > li > button')
    .evaluateAll((els) => els.map((el) => Math.round(el.getBoundingClientRect().height)))
  return { cards: all.slice(0, -1), add: all.at(-1) }
}

test('"New Folder" is as tall as the folder cards at every width', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/namespace/getChildren': ok([
      { nsId: 2, nsName: 'Trips' },
      { nsId: 3, nsName: 'Family' },
    ]),
    // One folder shows an item count, the other doesn't.
    '/photo/getCovers': ok([{ id: 2, count: 3, coverIds: [10, 11, 12] }]),
    '/photo/getByNamespace': ok([photo(10, 'beach')]),
  })
  for (const width of [390, 700, 1100, 1600, 2560]) {
    await page.setViewportSize({ width, height: 1000 })
    await page.goto('/home/HomePart')
    await expect(addTile(page)).toBeVisible()
    await expect
      .poll(async () => {
        const { cards, add } = await heights(page)
        return `${cards.length} cards, ${cards.every((h) => h === add) ? 'same' : 'different'}`
      }, `at ${width}px`)
      .toBe('2 cards, same')
  }
})

test('"New Folder" alone is as tall as an empty folder card', async ({ page, isMobile }) => {
  await signIn(page)
  await mockApi(page, { '/namespace/getChildren': ok([]) })
  await page.goto('/home/HomePart')
  await expect(addTile(page)).toBeVisible()
  // Preview (96px, 128px from 640px wide) + name row (44px) + border (2px).
  expect(Math.round((await addTile(page).boundingBox())!.height)).toBe(isMobile ? 142 : 174)
})
