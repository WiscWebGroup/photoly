import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('admin sees the paginated user list', async ({ page }) => {
  const calls = await mockApi(page, {
    '/admin/isAdmin': ok(true),
    '/admin/getUsers': ok({
      users: [
        {
          userId: 1,
          userName: 'alice',
          email: 'alice@example.com',
          createDate: '2024-01-01',
          role: 'admin',
          uuid: 'uuid-alice',
          mediaCount: 1204,
        },
        {
          userId: 2,
          userName: 'bob',
          email: 'bob@example.com',
          createDate: '2024-02-01',
          role: 'user',
          uuid: 'uuid-bob',
          mediaCount: 0,
        },
      ],
      total: 2,
    }),
  })
  await page.goto('/thepersonincharge/usermanage')

  await expect(page.getByRole('heading', { name: 'User Management' })).toBeVisible()
  await expect(page.getByRole('cell', { name: '1,204' })).toBeVisible()
  await expect(page.getByRole('cell', { name: '2024-02-01' })).toBeVisible()
  await expect(page.getByText('1–2 of 2')).toBeVisible()
  // Username and email are editable in place.
  const values = await page
    .getByRole('textbox')
    .evaluateAll((els) => els.map((el) => (el as HTMLInputElement).value))
  expect(values).toEqual(expect.arrayContaining(['alice', 'bob', 'alice@example.com']))
  expect(calls.find((c) => c.path === '/admin/getUsers')?.query).toEqual({
    rowsPerPage: '8',
    page: '1',
  })

  await page.getByRole('radio', { name: 'Admins' }).click()
  await expect(page.getByRole('radio', { name: 'Admins' })).toBeChecked()
  await expect
    .poll(() => calls.filter((c) => c.path === '/admin/getUsers').at(-1)?.query)
    .toEqual({ rowsPerPage: '8', page: '1', role: 'admin' })
})

test('admin can search users', async ({ page }) => {
  const calls = await mockApi(page, { '/admin/isAdmin': ok(true) })
  await page.goto('/thepersonincharge/usermanage')
  await page.getByRole('textbox').first().fill('bob')
  await page.getByRole('button', { name: 'Search' }).click()

  await expect
    .poll(() => calls.find((c) => c.path === '/admin/getUsers' && c.query.search)?.query)
    .toEqual({ rowsPerPage: '8', page: '1', search: 'bob' })
})

test('non-admins are told they are not an admin', async ({ page }) => {
  await mockApi(page, { '/admin/isAdmin': ok(false) })
  await page.goto('/thepersonincharge/usermanage')
  await expect(page.getByText('You are not an admin!')).toBeVisible()
})
