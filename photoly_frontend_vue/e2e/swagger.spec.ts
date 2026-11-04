import { expect, test } from '@playwright/test'

// Point this at an isolated test backend. Operation execution below is intercepted;
// only the bundled Swagger page, spec and assets are fetched from the server.
const base = process.env.PHOTOLY_SWAGGER_URL
test.skip(!base, 'Set PHOTOLY_SWAGGER_URL to an isolated backend origin')

test('bundled Swagger authorizes sessions and exposes all five write/read methods', async ({
  page,
}) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(`${base}/docs/`)
  await expect(page.getByRole('heading', { name: /Photoly API/ })).toBeVisible()
  await expect(page.locator('.errors-wrapper')).toHaveCount(0)
  await page.getByRole('button', { name: 'Authorize', exact: true }).first().click()
  const dialog = page.locator('.dialog-ux')
  const session = dialog.locator('.auth-container').filter({ hasText: 'SessionToken' })
  await session.locator('input').fill('synthetic-docs-session')
  await dialog.getByRole('button', { name: 'Apply credentials', exact: true }).first().click()
  await dialog.getByRole('button', { name: 'Close', exact: true }).first().click()
  for (const [method, path, id, body] of [
    ['GET', '/namespace/pins', 'namespace_pins', ''],
    ['POST', '/ai/index', 'post_ai_index', ''],
    ['PUT', '/ai/preference', 'put_ai_preference', '{"image":"browser"}'],
    ['PATCH', '/ai/smart/settings', 'smart_set_settings', '{"deviceEnabled":true}'],
    ['DELETE', '/uploads', 'uploads_clear_upload_activity', ''],
  ]) {
    const block = page.locator(`.opblock[id$="-${id}"]`)
    await block.locator('.opblock-summary-control').click()
    await block.getByRole('button', { name: 'Try it out', exact: true }).click()
    if (body) await block.locator('textarea').fill(body)
    let seen = false
    await page.route(`${base}${path}`, async (route) => {
      expect(route.request().method()).toBe(method)
      expect(route.request().headers()['hrd-token']).toBe('synthetic-docs-session')
      seen = true
      await route.fulfill({ json: { msgCode: 200, t: null } })
    })
    await block.getByRole('button', { name: 'Execute', exact: true }).click()
    await expect.poll(() => seen).toBe(true)
    await expect(block.locator('.responses-wrapper')).toContainText('200')
  }
  await page.reload()
  await page.getByRole('button', { name: 'Authorize', exact: true }).first().click()
  await expect(
    page.locator('.dialog-ux .auth-container').filter({ hasText: 'SessionToken' }).locator('input'),
  ).toHaveValue('')
  expect(errors).toEqual([])
})
