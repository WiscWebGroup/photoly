import { expect, test } from '@playwright/test'
import { mockApi, signIn, ok } from './mockApi'

test('public license page and notice files work without downloading models', async ({
  page,
  request,
}) => {
  await mockApi(page)
  await page.route('**/legal', async (route) => {
    if (route.request().resourceType() === 'document') return route.continue()
    await route.fulfill({
      json: {
        revision: 'backend-test',
        sourceURL: 'https://example.org/source.tar.gz',
        license: 'AGPL-3.0-or-later',
      },
    })
  })
  const modelRequests: string[] = []
  page.on('request', (request) => {
    if (/\.onnx($|\?)/.test(request.url())) modelRequests.push(request.url())
  })
  await page.goto('/login')
  await page.getByRole('link', { name: 'Licenses & source' }).click()
  await expect(page.getByRole('heading', { name: 'Licenses & source' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Download backend source' })).toHaveAttribute(
    'href',
    'https://example.org/source.tar.gz',
  )
  await expect(page.getByRole('link', { name: 'Read the Photoly license' })).toHaveAttribute(
    'href',
    '/legal/AGPL-3.0.txt',
  )
  for (const path of [
    '/legal/AGPL-3.0.txt',
    '/legal/third-party-notices.txt',
    '/legal/dependencies.json',
  ]) {
    const response = await request.get(path)
    expect(response.status()).toBe(200)
    expect(response.headers()['content-type']).not.toContain('text/html')
  }
  expect(modelRequests).toEqual([])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('signed-in account has a license entry', async ({ page }) => {
  await signIn(page)
  await mockApi(page)
  await page.goto('/home/ManagePart')
  await page.getByRole('link', { name: 'Licenses & source' }).last().click()
  await expect(page.getByRole('heading', { name: 'Licenses & source' })).toBeVisible()
})

test('administrator navigation exposes public legal information', async ({ page }) => {
  await signIn(page)
  await mockApi(page, { '/admin/isAdmin': ok(true) })
  await page.goto('/thepersonincharge/dashboard')
  await page.getByRole('link', { name: 'Licenses & source' }).filter({ visible: true }).click()
  await expect(page.getByRole('heading', { name: 'Licenses & source' })).toBeVisible()
})
