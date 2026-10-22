import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn, TOKEN } from './mockApi'

test('login lands on home with the root folder', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signIn': { msgCode: 200, token: TOKEN } })
  await page.goto('/login')
  await page.getByLabel('Email', { exact: true }).fill('alice@example.com')
  await page.getByLabel('Password', { exact: true }).fill('secret')
  await page.getByRole('button', { name: 'Sign In' }).click()

  await expect(page).toHaveURL(/\/home\/HomePart$/)
  await expect(page.getByText('Folders')).toBeVisible()
  expect(calls.find((c) => c.path === '/user/signIn')?.body).toEqual({
    email: 'alice@example.com',
    password: 'secret',
  })
  expect(await page.evaluate(() => localStorage.getItem('HRD-Token'))).toBe(TOKEN)
})

test('login shows an error for wrong credentials', async ({ page }) => {
  await mockApi(page, { '/user/signIn': { msgCode: 401 } })
  await page.goto('/login')
  await page.getByLabel('Email', { exact: true }).fill('alice@example.com')
  await page.getByLabel('Password', { exact: true }).fill('wrong')
  await page.getByLabel('Password', { exact: true }).press('Enter')

  await expect(page.getByText('Wrong Email or Password!')).toBeVisible()
  await expect(page).toHaveURL(/\/login$/)
})

test('signup lands on home', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signUp': { msgCode: 200, token: TOKEN } })
  await page.goto('/login')
  await page.getByRole('link', { name: 'Sign Up', exact: true }).click()
  await expect(page).toHaveURL(/\/signup$/)
  await page.getByLabel('Username', { exact: true }).fill('alice')
  await page.getByLabel('Email', { exact: true }).fill('alice@example.com')
  await page.getByLabel('Password', { exact: true }).fill('secret')
  await page.getByLabel('Confirm Password', { exact: true }).fill('secret')
  await page.getByRole('button', { name: 'Create Account' }).click()

  await expect(page).toHaveURL(/\/home\/HomePart$/)
  expect(calls.find((c) => c.path === '/user/signUp')?.body).toMatchObject({
    email: 'alice@example.com',
    password: 'secret',
  })
})

test('home redirects to login without a token', async ({ page }) => {
  await mockApi(page)
  await page.goto('/home/HomePart')
  await expect(page).toHaveURL(/\/login$/)
})

test('logout clears the token and returns to the welcome page', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signOut': ok() })
  await page.goto('/login')
  await page.evaluate((token) => localStorage.setItem('HRD-Token', token), TOKEN)
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Logout' }).click()

  await expect(page).toHaveURL(/\/$/)
  expect(calls.map((c) => c.path)).toContain('/user/signOut')
  expect(await page.evaluate(() => localStorage.getItem('HRD-Token'))).toBeNull()
})

test('signed-in users skip the login form', async ({ page }) => {
  await signIn(page)
  await mockApi(page)
  await page.goto('/login')
  await expect(page).toHaveURL(/\/home\/HomePart$/)
})
