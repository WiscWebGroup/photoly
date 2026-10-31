import { expect, test } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

for (const layout of ['grid', 'compact', 'list']) {
  for (const [folders, media] of [
    [false, false],
    [true, false],
    [false, true],
    [true, true],
  ]) {
    test(`${layout}: folders=${folders}, media=${media} shows only populated sections`, async ({
      page,
    }) => {
      await signIn(page)
      await page.addInitScript((mode) => localStorage.setItem('photoly-view-mode', mode), layout)
      await mockApi(page, {
        '/namespace/getChildren': ok(folders ? [{ nsId: 2, nsName: 'Trips' }] : []),
        '/photo/getByNamespace': ok(media ? [photo(10, 'Beach')] : []),
      })
      await page.goto('/home/HomePart')
      await expect(page.getByLabel('Loading folder', { exact: true })).toHaveCount(0)
      await expect(page.getByRole('heading', { name: 'Nothing Here', exact: true })).toHaveCount(
        !folders && !media ? 1 : 0,
      )
      await expect(page.getByRole('heading', { name: 'Folders', exact: true })).toHaveCount(
        folders ? 1 : 0,
      )
      await expect(page.getByRole('heading', { name: 'Media', exact: true })).toHaveCount(
        media ? 1 : 0,
      )
      await expect(
        page.getByRole('listitem').getByRole('button', { name: 'New Folder', exact: true }),
      ).toHaveCount(0)
      await expect(page.getByRole('heading', { name: 'No photos here', exact: true })).toHaveCount(
        0,
      )
    })
  }
}
test('a failed folder load shows an error and can retry, without claiming it is empty', async ({
  page,
}) => {
  await signIn(page)
  let failed = true
  await mockApi(page, {
    '/namespace/getChildren': () => (failed ? { msgCode: 500, t: null } : ok([])),
  })
  await page.goto('/home/HomePart')
  await expect(
    page.getByRole('alert').filter({ hasText: 'Could not load this folder.' }),
  ).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Nothing Here' })).toHaveCount(0)
  failed = false
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Nothing Here' })).toBeVisible()
})
