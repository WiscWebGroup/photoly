import { expect, test } from '@playwright/test'

test('welcome page links to sign in', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle('Photoly')
  await page.getByRole('button', { name: 'Sign In' }).first().click()
  await expect(page).toHaveURL(/\/login$/)
})
