import { expect, test } from '@playwright/test'
import { mockApi, ok } from './mockApi'

test('demo landing page links to the pre-filled sign in', async ({ page }) => {
  await mockApi(page, {
    '/demo': ok({ enabled: true, email: 'demo@example.com', password: 'demo-pass' }),
  })
  await page.goto('/')
  await expect(page).toHaveTitle('Photoly')
  await expect(page.getByRole('img', { name: 'Photoly' })).toBeVisible()
  await page.getByRole('link', { name: 'Try the demo' }).click()
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByLabel('Email', { exact: true })).toHaveValue('demo@example.com')
  await expect(page.getByRole('link', { name: 'Sign Up', exact: true })).toHaveCount(0)
})

test('a regular deployment skips the landing page', async ({ page }) => {
  await mockApi(page)
  await page.goto('/')
  await expect(page).toHaveURL(/\/login$/)
})
