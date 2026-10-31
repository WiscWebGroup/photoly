import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn, TOKEN } from './mockApi'

test('login lands on home with the root folder', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signIn': { msgCode: 200, token: TOKEN } })
  await page.goto('/login')
  await page.getByLabel('Email', { exact: true }).fill('alice@example.com')
  await page.getByLabel('Password', { exact: true }).fill('secret')
  await page.getByRole('button', { name: 'Sign In' }).click()

  await expect(page).toHaveURL(/\/home\/HomePart$/)
  await expect(page.getByRole('heading', { name: 'Nothing Here', exact: true })).toBeVisible()
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

test('an unresponsive login times out and allows retry', async ({ page }) => {
  await mockApi(page, { '/user/signIn': { msgCode: 200, token: TOKEN } })
  // Leave the request pending, as when a previously reachable LAN address goes away.
  await page.route('**/user/signIn/', () => {})
  await page.goto('/login')
  await page.getByLabel('Email', { exact: true }).fill('alice@example.com')
  await page.getByLabel('Password', { exact: true }).fill('secret')
  const submit = page.getByRole('button', { name: 'Sign In' })
  await submit.click()
  await expect(submit).toBeDisabled()
  await expect(page.getByRole('alert')).toHaveText('Could not reach the server.', {
    timeout: 20_000,
  })
  await expect(submit).toBeEnabled()
  expect(await page.evaluate(() => localStorage.getItem('HRD-Token'))).toBeNull()
  await page.unroute('**/user/signIn/')
  await submit.click()
  await expect(page).toHaveURL(/\/home\/HomePart$/)
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

test('logout clears the token and returns to sign in', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signOut': ok() })
  await page.goto('/login')
  await page.evaluate((token) => localStorage.setItem('HRD-Token', token), TOKEN)
  // Phones sign out from the account page; wider screens from the sidebar.
  await page.goto('/home/ManagePart')
  await page
    .getByRole('button', { name: /^Sign out$/i })
    .filter({ visible: true })
    .click()

  await expect(page).toHaveURL(/\/login$/)
  expect(calls.map((c) => c.path)).toContain('/user/signOut')
  expect(await page.evaluate(() => localStorage.getItem('HRD-Token'))).toBeNull()
})

test('signed-in users skip the login form', async ({ page }) => {
  await signIn(page)
  await mockApi(page)
  await page.goto('/login')
  await expect(page).toHaveURL(/\/home\/HomePart$/)
})
