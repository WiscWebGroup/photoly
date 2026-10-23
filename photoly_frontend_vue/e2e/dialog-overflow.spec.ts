import { expect, test, type Locator } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

async function expectContained(dialog: Locator) {
  await expect(dialog).toBeVisible()
  await expect(async () => {
    const overflow = await dialog.evaluate((element) => {
      const bounds = element.getBoundingClientRect()
      return {
        viewport: bounds.left < 0 || bounds.right > window.innerWidth,
        content: element.scrollWidth > element.clientWidth + 1,
        children: [...element.children].some((child) => {
          const rect = child.getBoundingClientRect()
          return rect.left < bounds.left - 1 || rect.right > bounds.right + 1
        }),
      }
    })
    expect(overflow).toEqual({ viewport: false, content: false, children: false })
  }).toPass()
}

for (const name of [
  'Terraria-Crossover-update-adds-aliens-from-Dont-Starve',
  'VeryLongUnbrokenFilename'.repeat(12),
]) {
  test(`long filename stays inside share and delete dialogs: ${name.slice(0, 24)}`, async ({
    page,
  }) => {
    await signIn(page)
    await mockApi(page, {
      '/photo/getByNamespace': ok([photo(10, name, { visibility: 1 })]),
      '/photo/getPhotoExif': ok({ File_Size: '0.24 MB' }),
      '/photo/getPath': ok(`http://localhost:8384/photo/renderToken?path=${'a'.repeat(200)}`),
    })
    await page.goto('/home/HomePart')
    await page.getByText(`${name}.jpg`, { exact: true }).click()
    await page.getByRole('button', { name: 'Share', exact: true }).click()
    const share = page.getByRole('dialog', { name: 'Share Photo', exact: true })
    await expect(share.getByRole('button', { name: 'Save QR Code' })).toBeVisible()
    await expectContained(share)
    await share.getByRole('button', { name: 'Done', exact: true }).click()

    await page.getByRole('button', { name: 'More', exact: true }).click()
    await page.getByRole('menuitem', { name: /^Delete/ }).click()
    const confirm = page.getByRole('alertdialog')
    await expectContained(confirm)
    await confirm.getByRole('button', { name: 'Cancel', exact: true }).click()
  })
}
