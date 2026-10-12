import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn, TOKEN } from './mockApi'

test('login lands on home with the root folder', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signIn': { msgCode: 200, token: TOKEN } })
  await page.goto('/login')
  await page.getByPlaceholder('Email').fill('alice@example.com')
  await page.getByPlaceholder('Password').fill('secret')
  await page.getByRole('button', { name: 'Login' }).click()

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
  await page.getByPlaceholder('Email').fill('alice@example.com')
  await page.getByPlaceholder('Password').fill('wrong')
  await page.getByPlaceholder('Password').press('Enter')

  await expect(page.getByText('Wrong Email or Password!')).toBeVisible()
  await expect(page).toHaveURL(/\/login$/)
})

test('signup lands on home', async ({ page }) => {
  const calls = await mockApi(page, { '/user/signUp': { msgCode: 200, token: TOKEN } })
  await page.goto('/login')
  await page.getByRole('button', { name: 'Signup' }).click()
  await expect(page).toHaveURL(/\/signup$/)
  await page.getByPlaceholder('Username').fill('alice')
  await page.getByPlaceholder('Email').fill('alice@example.com')
  await page.getByPlaceholder('Password').fill('secret')
  await page.getByRole('button', { name: 'Signup' }).click()

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
