import { expect, test } from '@playwright/test'
import { mockApi, ok, photo, signIn } from './mockApi'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('create a tag and filter photos by it', async ({ page }) => {
  const tags = [{ tagId: 3, tagName: 'Sunset' }]
  const calls = await mockApi(page, {
    '/tag/getAll': () => ok(tags),
    '/tag/insert': ({ body }) => {
      tags.push({ tagId: 4, tagName: (body as { tagName: string }).tagName })
      return ok()
    },
    '/photo/getByTags': ok([photo(10, 'beach')]),
  })
  await page.goto('/home/TagPart')

  await page.getByRole('button', { name: 'Add New Tag' }).click()
  await page.getByPlaceholder('Tag Name').fill('Beach')
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(page.getByText('Created a New Tag!')).toBeVisible()
  await expect(page.getByText('Beach', { exact: true })).toBeVisible()
  expect(calls.find((c) => c.path === '/tag/insert')?.body).toEqual({ tagName: 'Beach' })

  await page.getByText('Sunset', { exact: true }).click()
  await expect(page.getByText('beach.jpg')).toBeVisible()
  expect(calls.find((c) => c.path === '/photo/getByTags')?.query).toEqual({ tagIds: '3' })
})

test('search photos by name', async ({ page }) => {
  const calls = await mockApi(page, { '/photo/searchPhoto': ok([photo(10, 'beach')]) })
  await page.goto('/home/SearchPart')
  await page.getByRole('radio', { name: 'Name', exact: true }).click()
  await page.getByRole('textbox').fill('bea')
  await page.getByRole('textbox').press('Enter')

  await expect(page.getByText('1 record(s) found')).toBeVisible()
  await expect(page.getByText('beach.jpg')).toBeVisible()
  expect(calls.find((c) => c.path === '/photo/searchPhoto')?.query).toEqual({
    option: 'photoName',
    query: 'bea',
  })
})

test('browse a gallery', async ({ page }) => {
  const calls = await mockApi(page, {
    '/gallery/getAll': ok([
      {
        gaId: 7,
        gaName: 'Summer',
        coverId: 1,
        coverColor: '#ff0000',
        createDate: '2024-06-01T00:00:00',
      },
    ]),
    '/photo/getByGallery': ok([photo(10, 'beach')]),
  })
  await page.goto('/home/GalleryPart')
  await expect(page.getByText(/^1 item · Created Jun 1, 2024$/)).toBeVisible()
  await page.getByText('Summer', { exact: true }).click()

  await expect(page.getByText('beach.jpg')).toBeVisible()
  expect(calls.find((c) => c.path === '/photo/getByGallery')?.query).toEqual({ gaId: '7' })
})

test('map shows a marker per geotagged photo', async ({ page }) => {
  await mockApi(page, {
    '/photo-extra/getGlobalMapPhoto': ok([
      {
        photo_id: 10,
        photo_name: 'beach',
        // Near the map's initial center so the marker is on screen.
        extra_photo_gps_lat: 20.5,
        extra_photo_gps_lon: 11.5,
        extra_photo_taken_date: '2024-06-01',
      },
      { photo_id: 11, photo_name: 'no-gps' },
    ]),
  })
  await page.goto('/home/MapPart')

  // MapPart currently stacks two copies of each marker; click the top one.
  const marker = page.getByTitle('beach').last()
  await expect(marker).toBeVisible()
  await expect(page.getByTitle('no-gps')).toHaveCount(0)

  await marker.click()
  await expect(page.getByRole('heading', { name: 'beach' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Open in Map' })).toHaveAttribute(
    'href',
    'https://maps.google.com/?q=20.5,11.5',
  )
})
