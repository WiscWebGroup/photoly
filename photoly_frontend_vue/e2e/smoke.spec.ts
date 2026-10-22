import { expect, test } from '@playwright/test'

test('welcome page links to sign in', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle('Photoly')
  await expect(page.getByRole('img', { name: 'Photoly' })).toBeVisible()
  // Phone widths fold the header links into a menu.
  const menu = page.getByRole('button', { name: 'Menu' })
  if (await menu.isVisible()) await menu.click()
  await page.getByRole('link', { name: 'Sign In', exact: true }).filter({ visible: true }).click()
  await expect(page).toHaveURL(/\/login$/)
})
