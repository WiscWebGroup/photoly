import { expect, test } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

test.use({ viewport: { width: 2560, height: 1200 } })

test('on a very wide screen the library fills the width', async ({ page, isMobile }) => {
  test.skip(isMobile, 'desktop layout')
  await signIn(page)
  await mockApi(page, {
    '/photo/getByNamespace': ok(Array.from({ length: 30 }, (_, i) => photo(10 + i, `p${i}`))),
  })
  await page.goto('/home/HomePart')
  await expect(page.getByText('Saturday, June 1, 2024')).toBeVisible()

  // Photos stay about the same size and more of them fit in a row.
  const grid = page.getByRole('list').filter({ has: page.getByRole('button', { name: /p0/ }) })
  const columns = await grid.evaluate(
    (el) => getComputedStyle(el).gridTemplateColumns.split(' ').length,
  )
  expect(columns).toBeGreaterThan(8)

  // Right-clicking the empty space at the far right still offers upload / new folder.
  await page.mouse.click(2450, 1100, { button: 'right' })
  await expect(page.getByRole('menuitem', { name: 'Upload Photo' })).toBeVisible()
})
