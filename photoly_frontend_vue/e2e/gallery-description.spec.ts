import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'

test('gallery descriptions create, persist, edit, cancel and clear without appearing on cards', async ({
  page,
}) => {
  await signIn(page)
  let galleries: Record<string, unknown>[] = []
  let failSave = false
  const calls = await mockApi(page, {
    '/gallery/getAll': () => ok(galleries),
    '/gallery/insert': ({ body }) => {
      galleries = [{ ...(body as object), gaId: 7, createDate: '2024-06-01T10:00:00' }]
      return ok(true)
    },
    '/gallery/update': ({ body }) => {
      if (failSave) return { msgCode: 400, t: -1 }
      galleries = [{ ...galleries[0], ...(body as object) }]
      return ok(1)
    },
  })
  await page.goto('/home/GalleryPart')
  await page.getByRole('button', { name: 'New Gallery', exact: true }).last().click()
  let dialog = page.getByRole('dialog')
  await dialog.getByLabel('Name', { exact: true }).fill('Trip')
  const description = '旅行说明 📷\n<script>alert("plain text")</script>\n' + 'LongWord'.repeat(40)
  await dialog.getByLabel('Description', { exact: true }).fill(description)
  await expect(dialog).toBeVisible()
  expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
  await page.screenshot({ path: test.info().outputPath('gallery-description-editor.png') })
  await dialog.getByRole('button', { name: 'Create Gallery', exact: true }).click()
  await expect(dialog).toBeHidden()
  await expect(page.getByTestId('gallery-description')).toHaveCount(0)
  await expect(page.getByText(description, { exact: true })).toHaveCount(0)
  await page.getByText('Trip', { exact: true }).click()
  const detail = page.getByTestId('gallery-description')
  await expect(detail).toHaveText(description)
  await expect(detail.locator('script')).toHaveCount(0)
  await expect(detail).toHaveCSS('white-space', 'pre-wrap')
  expect(await detail.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
  await page.screenshot({ path: test.info().outputPath('gallery-description-detail.png') })
  await page.reload()
  await expect(detail).toHaveText(description)
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  dialog = page.getByRole('dialog')
  await expect(dialog.getByLabel('Description', { exact: true })).toHaveValue(description)
  await dialog.getByLabel('Description', { exact: true }).fill('Cancelled')
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(detail).toHaveText(description)
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  const text = dialog.getByLabel('Description', { exact: true })
  await text.fill('📷'.repeat(5001))
  await expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
  await expect(dialog.getByRole('alert')).toContainText('5,000 characters or fewer')
  await text.fill('📷'.repeat(5000))
  await expect(dialog.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Save Changes' }).click()
  await expect(dialog).toBeHidden()
  await expect(detail).toHaveText('📷'.repeat(5000))
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  await text.fill('Updated\n说明')
  failSave = true
  await dialog.getByRole('button', { name: 'Save Changes' }).click()
  await expect(page.getByText('Update Response Error!', { exact: true })).toBeVisible()
  await expect(text).toHaveValue('Updated\n说明')
  failSave = false
  await dialog.getByRole('button', { name: 'Save Changes' }).click()
  await expect(dialog).toBeHidden()
  await expect(detail).toHaveText('Updated\n说明')
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  await text.fill('')
  await dialog.getByRole('button', { name: 'Save Changes' }).click()
  await expect(dialog).toBeHidden()
  await expect(detail).toHaveCount(0)
  expect(calls.filter((c) => c.path === '/gallery/update').at(-1)?.body).toMatchObject({
    description: '',
  })
  await page.screenshot({ path: test.info().outputPath('gallery-description-cleared.png') })
})
