import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'

test('all toolbar activities use anchored panels and Storage shows concise colored metrics', async ({
  page,
}) => {
  await signIn(page)
  await mockApi(page, {
    '/photo/mediaCapabilities': ok({ onlineDownload: true }),
    '/user/storage/sources': ok([
      {
        sourceId: 9,
        sourceType: 'smb',
        name: 'NAS',
        enabled: true,
        revision: 1,
        scanStatus: 'scanning',
        scanPhase: 'processing',
        scanCompleted: 3,
        scanTotal: 8,
        scanDiscovered: 12,
        scanAutomaticRetries: 2,
        scanBlocked: 1,
        scanRetries: 3,
      },
    ]),
  })
  await page.goto('/home/HomePart')
  let panelWidth = 0
  for (const [label, panelName] of [
    ['Storage activity', 'storage'],
    ['Upload activity', 'uploads'],
    ['Online Download', 'download'],
  ]) {
    const trigger = page.getByRole('button', { name: label!, exact: true })
    await trigger.click()
    const panel = page.locator(`[data-activity-panel="${panelName}"]`)
    await expect(panel).toBeVisible()
    await expect
      .poll(async () => (await panel.boundingBox())!.width)
      .toBeCloseTo(Math.min(440, page.viewportSize()!.width - 24), 0)
    const panelBox = (await panel.boundingBox())!
    const triggerBox = (await trigger.boundingBox())!
    expect(panelBox.y).toBeGreaterThanOrEqual(triggerBox.y + triggerBox.height)
    expect(panelBox.x).toBeGreaterThanOrEqual(0)
    expect(panelBox.x + panelBox.width).toBeLessThanOrEqual(page.viewportSize()!.width)
    if (panelWidth) expect(panelBox.width).toBeCloseTo(panelWidth, 0)
    panelWidth = panelBox.width
    await expect(page.locator('[data-slot="dialog-overlay"]')).toHaveCount(0)
    if (panelName === 'storage') {
      await expect(panel.locator('dt')).toHaveText([
        'Discovered',
        'Processed',
        'Retrying',
        'Warning',
      ])
      await expect(panel.locator('dd')).toHaveText(['12', '3/8', '2', '1'])
      await expect(panel.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '37.5')
      await expect(panel.locator('dl .bg-blue-500')).toHaveCount(1)
      await expect(panel.locator('dl .bg-primary')).toHaveCount(1)
      await expect(panel.locator('dl .bg-amber-400')).toHaveCount(2)
    }
    await panel.getByRole('button', { name: 'Close', exact: true }).click()
    await expect(panel).toBeHidden()
  }
})

test('Online Download retains its draft while choosing a destination', async ({ page }) => {
  await signIn(page)
  await mockApi(page, { '/photo/mediaCapabilities': ok({ onlineDownload: true }) })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Online Download', exact: true }).click()
  await page.getByLabel('Image URL').fill('https://example.com/photo.jpg')
  await page.getByRole('button', { name: 'Choose folder', exact: true }).click()
  const picker = page.getByRole('dialog', { name: 'Save to folder' })
  await expect(picker).toBeVisible()
  await picker.getByRole('button', { name: 'Library', exact: true }).click()
  await picker.getByRole('button', { name: 'Use folder', exact: true }).click()
  await expect(picker).toBeHidden()
  await expect(page.locator('[data-activity-panel="download"]')).toBeVisible()
  await expect(page.getByLabel('Image URL')).toHaveValue('https://example.com/photo.jpg')
})
