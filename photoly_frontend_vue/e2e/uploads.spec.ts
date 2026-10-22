import { test, expect } from '@playwright/test'
import { mockApi, signIn, PNG, ok } from './mockApi'
import { mockUploads } from './uploadsMock'

test.beforeEach(async ({ page }) => {
  // Localhost exposes randomUUID; deliberately remove it to reproduce LAN HTTP.
  await page.addInitScript(() =>
    Object.defineProperty(globalThis.crypto, 'randomUUID', {
      value: undefined,
      configurable: true,
    }),
  )
})

test('bulk uploads survive closing the panel, navigation, and history reload', async ({ page }) => {
  await signIn(page)
  await mockApi(page)
  const server = await mockUploads(page, { delay: 150 })
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await page.getByRole('button', { name: 'New upload', exact: true }).click()
  await page
    .getByLabel('Choose files or ZIP')
    .setInputFiles(
      Array.from({ length: 18 }, (_, i) => ({ name: `photo-${i}.jpg`, mimeType: '', buffer: PNG })),
    )
  const picker = page.getByRole('dialog')
  await expect(picker.getByText('photo-17.jpg', { exact: true })).toBeVisible()
  await picker.getByRole('button', { name: 'Upload', exact: true }).click()
  await expect(
    page.getByRole('dialog').getByRole('heading', { name: 'Uploads', exact: true }),
  ).toBeVisible()
  await page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true }).click()
  await page
    .getByRole('link', { name: /Search/ })
    .last()
    .click()
  await expect(page).toHaveURL(/SearchPart/)
  await expect.poll(() => server.items.filter((i) => i.state === 'done').length).toBe(18)
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await expect(page.getByText('18 uploaded', { exact: false })).toBeVisible()
  await page.screenshot({
    path: '/tmp/photoly-upload-center.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.reload()
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await expect(page.getByText('18 uploaded', { exact: false })).toBeVisible()
  await page.getByRole('button', { name: 'Details', exact: true }).click()
  await expect(page.getByText('photo-17.jpg', { exact: true })).toBeVisible()
})

test('admin can edit the default group and a personal storage override', async ({ page }) => {
  await signIn(page)
  const user = {
    userId: 2,
    userName: 'Alice',
    email: 'alice@example.com',
    role: 'user',
    createDate: '2026-01-01',
    mediaCount: 3,
    groupId: 1,
    groupName: 'Default',
    usedBytes: 12000,
    capacityBytes: 50e9,
    storageOverride: false,
    maxStorageBytes: null,
  }
  const groups = [
    { id: 1, name: 'Default', maxStorageBytes: 50e9, isDefault: true, memberCount: 1 },
  ]
  const calls = await mockApi(page, {
    '/admin/isAdmin': ok(true),
    '/admin/getUsers': () => ok({ users: [user], total: 1 }),
    '/admin/ai/users': ok([{ userId: 2, enabled: false }]),
    '/admin/user-groups': () => ok(groups),
    '/admin/user-groups/1': ({ body }) => {
      groups[0]!.maxStorageBytes = (body as { maxStorageBytes: number }).maxStorageBytes
      return ok(1)
    },
    '/admin/users/2/storage': ok({}),
  })
  await page.goto('/thepersonincharge/usermanage')
  await page.getByRole('tab', { name: 'Groups', exact: true }).click()
  await page.getByRole('button', { name: 'Edit group Default' }).click()
  await page.getByLabel('Max Storage per user (GB)', { exact: true }).fill('75')
  await page.getByRole('button', { name: 'Save group', exact: true }).click()
  await expect(page.getByText('75 GB', { exact: true })).toBeVisible()
  expect(calls.find((c) => c.path === '/admin/user-groups/1')?.body).toEqual({
    name: 'Default',
    maxStorageBytes: 75e9,
  })
  await expect(page.getByRole('dialog')).toBeHidden()

  await page.getByRole('tab', { name: 'Users', exact: true }).click()
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
    .toBe(await page.evaluate(() => document.documentElement.clientWidth))
  await page.getByRole('button', { name: 'Storage for Alice', exact: true }).click()
  await page.getByLabel('Storage limit', { exact: true }).selectOption('limit')
  await page.getByLabel('Max Storage (GB)', { exact: true }).fill('100')
  await page.getByRole('button', { name: 'Save storage', exact: true }).click()
  await expect
    .poll(() => calls.find((c) => c.path === '/admin/users/2/storage')?.body)
    .toEqual({ groupId: 1, storageOverride: true, maxStorageBytes: 100e9 })
  await expect(page.getByRole('dialog')).toBeHidden()
  await page.getByRole('tab', { name: 'Groups', exact: true }).click()
  await page.screenshot({
    path: '/tmp/photoly-storage-groups.png',
    fullPage: true,
    animations: 'disabled',
  })
})

test('directory picker preserves nested relative paths for more than 15 files', async ({
  page,
}) => {
  const { mkdtemp, mkdir, writeFile, rm } = await import('node:fs/promises')
  const { tmpdir } = await import('node:os')
  const { join, basename } = await import('node:path')
  const directory = await mkdtemp(join(tmpdir(), 'photoly-directory-'))
  try {
    await mkdir(join(directory, '旅行', '北京'), { recursive: true })
    await mkdir(join(directory, '旅行', '上海'))
    for (let i = 0; i < 18; i++) await writeFile(join(directory, '旅行', '北京', `${i}.jpg`), PNG)
    await writeFile(join(directory, '旅行', '上海', '0.jpg'), PNG)
    await signIn(page)
    await mockApi(page)
    const server = await mockUploads(page)
    await page.goto('/home/HomePart')
    await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
    await page.getByRole('button', { name: 'New upload', exact: true }).click()
    await page.getByLabel('Choose folder', { exact: true }).setInputFiles(directory)
    await expect(
      page.getByText(`${basename(directory)}/旅行/北京/17.jpg`, { exact: true }),
    ).toBeVisible()
    await page.getByRole('dialog').getByRole('button', { name: 'Upload', exact: true }).click()
    await expect.poll(() => server.items.filter((i) => i.state === 'done').length).toBe(19)
    expect(server.items.every((i) => i.path.startsWith(`${basename(directory)}/旅行/`))).toBe(true)
    expect(server.items.map((i) => i.path)).toContain(`${basename(directory)}/旅行/北京/0.jpg`)
    expect(server.items.map((i) => i.path)).toContain(`${basename(directory)}/旅行/上海/0.jpg`)
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

test('ZIP can be queued without randomUUID', async ({ page }) => {
  await signIn(page)
  await mockApi(page)
  const server = await mockUploads(page)
  await page.goto('/home/HomePart')
  await page.getByRole('button', { name: 'Upload activity' }).filter({ visible: true }).click()
  await page.getByRole('button', { name: 'New upload', exact: true }).click()
  await page
    .getByLabel('Choose files or ZIP')
    .setInputFiles({ name: '旅行.zip', mimeType: 'application/zip', buffer: Buffer.from('PK') })
  await page.getByRole('dialog').getByRole('button', { name: 'Upload', exact: true }).click()
  await expect.poll(() => server.items.filter((i) => i.state === 'done').length).toBe(1)
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
  expect(server.batches[0]!.id).toMatch(uuid)
  expect(server.items[0]!.id).toMatch(uuid)
  expect(server.items[0]!.path).toBe('旅行.zip')
})
