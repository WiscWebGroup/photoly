import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, openDetails, photo, PNG, signIn } from './mockApi'

// Confirmations are `alertdialog`s now (the correct role for "are you sure?").
const dialog = (page: Page, title: string) =>
  page.getByRole('dialog').or(page.getByRole('alertdialog')).filter({ hasText: title })
const menuItem = (page: Page, name: string) => page.getByText(name, { exact: true })

// Context menus (and the move modal's mask) are unusable on touch today; Phase 4 owns mobile ergonomics.
const skipMobile = (isMobile: boolean) =>
  test.skip(isMobile, 'right-click menus / modal layout unusable on mobile (pre-existing)')

// The empty area of the folder view opens the upload / create-folder menu.
const openBlankMenu = (page: Page) =>
  page.getByText('Folders', { exact: true }).click({ button: 'right' })

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('upload a photo into the current folder', async ({ page, isMobile }) => {
  skipMobile(isMobile)
  const photos: object[] = []
  const calls = await mockApi(page, {
    '/photo/getByNamespace': () => ok(photos),
    '/photo/inserts': () => {
      photos.push(photo(10, 'beach'))
      return ok()
    },
  })
  await page.goto('/home/HomePart')
  await openBlankMenu(page)
  await menuItem(page, 'Upload Photo').click()

  const upload = dialog(page, 'Upload to Library')
  await upload
    .locator('input[type=file]')
    .setInputFiles({ name: 'beach.jpg', mimeType: 'image/jpeg', buffer: PNG })
  await upload.getByRole('button', { name: 'Upload' }).click()

  await expect(page.getByText('Upload successful!')).toBeVisible()
  await expect(upload).toBeHidden()
  await expect(page.getByText('beach.jpg')).toBeVisible()
  const body = String(calls.find((c) => c.path === '/photo/inserts')?.body)
  expect(body).toContain('filename="beach.jpg"')
  expect(body).toContain('[{"nsId":1}]')
})

test('create a folder', async ({ page, isMobile }) => {
  skipMobile(isMobile)
  const folders: object[] = []
  const calls = await mockApi(page, {
    '/namespace/getChildren': () => ok(folders),
    '/namespace/insert': ({ body }) => {
      folders.push({ nsId: 5, nsName: (body as { nsName: string }).nsName })
      return ok()
    },
  })
  await page.goto('/home/HomePart')
  await openBlankMenu(page)
  await menuItem(page, 'Create Folder').click()
  const create = dialog(page, 'New Folder')
  await create.getByPlaceholder('Folder Name').fill('Trips')
  await create.getByRole('button', { name: 'Create' }).click()

  await expect(page.getByText('Folder created!')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Trips' })).toBeVisible()
  expect(calls.find((c) => c.path === '/namespace/insert')?.body).toEqual({
    nsParentId: 1,
    nsName: 'Trips',
  })
})

test('delete a folder', async ({ page, isMobile }) => {
  skipMobile(isMobile)
  let folders = [{ nsId: 5, nsName: 'Trips' }]
  const calls = await mockApi(page, {
    '/namespace/getChildren': () => ok(folders),
    '/namespace/delete': ({ query }) => {
      folders = folders.filter((f) => String(f.nsId) !== query.nsId)
      return ok()
    },
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Trips' }).click({ button: 'right' })
  await menuItem(page, 'Delete').click()
  await dialog(page, 'Delete Trips?').getByRole('button', { name: 'Delete' }).click()

  await expect(page.getByText('Folder deleted!')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Trips' })).toBeHidden()
  expect(calls.find((c) => c.path === '/namespace/delete')?.query).toEqual({ nsId: '5' })
})

test('open a folder', async ({ page }) => {
  const calls = await mockApi(page, {
    '/namespace/getChildren': ({ query }) =>
      ok(query.parentId === '1' ? [{ nsId: 5, nsName: 'Trips' }] : []),
    '/photo/getByNamespace': ({ query }) =>
      ok(query.nsId === '5' ? [photo(10, 'beach', { nsId: 5 })] : []),
    '/namespace/trace': ({ query }) =>
      ok(
        query.nsId === '5'
          ? [
              { nsId: 1, nsName: 'root' },
              { nsId: 5, nsName: 'Trips' },
            ]
          : [{ nsId: 1, nsName: 'root' }],
      ),
  })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Trips' }).click()

  await expect(page.getByText('beach.jpg')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Trips' })).toBeHidden()
  expect(calls).toContainEqual(
    expect.objectContaining({ path: '/namespace/getChildren', query: { parentId: '5' } }),
  )

  // Breadcrumb (or the phone back link) to the library root.
  await page.getByRole('button', { name: 'Library' }).filter({ visible: true }).click()
  await expect(page.getByRole('button', { name: 'Trips' })).toBeVisible()
})

test('move a photo to another folder', async ({ page, isMobile }) => {
  skipMobile(isMobile)
  const calls = await mockApi(page, {
    '/namespace/getChildren': ({ query }) =>
      ok(query.parentId === '1' ? [{ nsId: 5, nsName: 'Trips' }] : []),
    '/photo/getByNamespace': ok([photo(10, 'beach')]),
    '/photo/changeNamespace': ok(),
  })
  await page.goto('/home/HomePart')
  await page.getByText('beach.jpg').click({ button: 'right' })
  await menuItem(page, 'Move To').click()

  const move = dialog(page, 'Move to a Folder')
  await move.getByText('Library', { exact: true }).click()
  await move.getByText('Trips', { exact: true }).click()
  await move.getByRole('button', { name: 'Move to Trips' }).click()

  await expect(page.getByText('Media Location Updated!')).toBeVisible()
  await expect(move).toBeHidden()
  expect(calls.find((c) => c.path === '/photo/changeNamespace')?.query).toEqual({
    photoId: '10',
    nsId: '5',
  })
})

test('view photo details', async ({ page }) => {
  await mockApi(page, {
    '/photo/getByNamespace': ok([photo(10, 'beach')]),
    '/photo/getPhotoExif': ok({ File_Size: '2 MB' }),
  })
  await page.goto('/home/HomePart')
  await page.getByText('beach.jpg').click()

  await expect(page.getByRole('heading', { name: 'beach.jpg' })).toBeVisible()
  // Details are collapsed until the Details button is pressed.
  await openDetails(page)
  await expect(page.getByText(/Uploaded .*2 MB/)).toBeVisible()
  await expect(page.getByText('/root', { exact: true })).toBeVisible()
})
