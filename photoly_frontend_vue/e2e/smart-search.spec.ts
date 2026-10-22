import { expect, test } from '@playwright/test'
import { API, mockApi, ok, photo, signIn } from './mockApi'
import { capabilitiesFixture } from '../src/test/aiFixtures'

const a = photo(1, 'Name match'),
  b = photo(2, 'Both sources'),
  c = photo(3, 'Semantic match')
const semantic = {
  items: [
    { photo: c, score: 0.9 },
    { photo: b, score: 0.8 },
  ],
  profileId: 1,
  similarityThreshold: null,
  coverage: capabilitiesFixture.coverage,
  requestId: 'smart',
}
test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('defaults to a single query, shows text first and then fused results', async ({ page }) => {
  const calls = await mockApi(page, {
    '/ai/capabilities': ok(capabilitiesFixture),
    '/photo/searchSmart': ok([a, b]),
  })
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  await page.route(API + '/ai/search/text', async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    expect(route.request().postDataJSON()).toEqual({ text: '海边 holiday', topK: 100 })
    await pending
    await route.fulfill({ json: ok(semantic), headers: { 'access-control-allow-origin': '*' } })
  })
  await page.goto('/home/SearchPart')
  await expect(page.getByRole('radio', { name: 'Smart', exact: true })).toBeChecked()
  await expect(page.getByRole('textbox')).toHaveCount(1)
  await expect(page.getByLabel('Result count')).toHaveCount(0)
  await expect(page.getByLabel('Filter album')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'All folders' })).toHaveCount(0)
  await page.getByRole('textbox').fill('海边 holiday')
  expect(calls.filter((c) => c.path === '/photo/searchSmart')).toHaveLength(0)
  await page.getByRole('textbox').press('Enter')
  await expect(page.getByRole('listitem')).toHaveText(['Name match.jpg', 'Both sources.jpg'])
  await expect(
    page.getByText('Searching… Results will update when semantic search finishes.'),
  ).toBeVisible()
  finish()
  await expect(page.getByRole('listitem')).toHaveText([
    'Both sources.jpg',
    'Name match.jpg',
    'Semantic match.jpg',
  ])
  await expect(page.getByText('3 result(s) found')).toBeVisible()
  await page.getByText('Both sources.jpg', { exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
})

test('AI unavailable still searches metadata and empty input clears results', async ({ page }) => {
  const calls = await mockApi(page, { '/photo/searchSmart': ok([a]) })
  await page.goto('/home/SearchPart')
  await page.getByRole('textbox').fill('test')
  await page.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(page.getByText('Name match.jpg', { exact: true })).toBeVisible()
  await expect(page.getByText('Semantic search is unavailable.', { exact: false })).toBeVisible()
  expect(calls.filter((c) => c.path === '/ai/search/text')).toHaveLength(0)
  await page.getByRole('textbox').fill('   ')
  await page.getByRole('textbox').press('Enter')
  await expect(page.getByRole('listitem')).toHaveCount(0)
  expect(calls.filter((c) => c.path === '/photo/searchSmart')).toHaveLength(1)
})

test('text failure keeps semantic matches and result count stays bounded', async ({ page }) => {
  await mockApi(page, {
    '/ai/capabilities': ok(capabilitiesFixture),
    '/photo/searchSmart': { msgCode: 503, t: null },
    '/ai/search/text': ok({
      ...semantic,
      items: Array.from({ length: 100 }, (_, i) => ({
        photo: photo(i + 1, `Hit ${i + 1}`),
        score: 1 - i / 200,
      })),
    }),
  })
  await page.goto('/home/SearchPart')
  await page.getByRole('textbox').fill('cat')
  await page.getByRole('textbox').press('Enter')
  await expect(page.getByText('Showing top 100 results')).toBeVisible()
  await expect(page.getByRole('listitem')).toHaveCount(100)
  await expect(
    page.getByText('Name, tag and gallery search is unavailable. Try again.'),
  ).toBeVisible()
})

test('mode changes ignore a late smart response and preserve traditional search', async ({
  page,
}) => {
  await mockApi(page, { '/photo/searchPhoto': ok([c]) })
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  await page.route(API + '/photo/searchSmart*', async (route) => {
    await pending
    await route
      .fulfill({ json: ok([a]), headers: { 'access-control-allow-origin': '*' } })
      .catch(() => {})
  })
  await page.goto('/home/SearchPart')
  await page.getByRole('textbox').fill('test')
  await page.getByRole('textbox').press('Enter')
  await page.getByRole('radio', { name: 'Name', exact: true }).click()
  await page.getByRole('textbox').press('Enter')
  finish()
  await expect(page.getByRole('listitem')).toHaveText(['Semantic match.jpg'])
})
