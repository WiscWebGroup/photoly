import { expect, test, type Page } from '@playwright/test'
import { mockApi, ok, type ApiCall, signIn } from './mockApi'

function fixture() {
  const root = { nsId: 1, nsName: 'Library', sourceId: 1, sourceType: 'local' }
  const local = {
    nsId: 2,
    nsName: 'Trips',
    sourceId: 1,
    sourceType: 'local',
    path: 'Library / Trips',
  }
  const remote = {
    nsId: 3,
    nsName: 'Remote',
    sourceId: 2,
    sourceType: 'webdav',
    isMount: true,
    path: 'Library / Remote',
  }
  const folders = [root, local, remote]
  const state = {
    ids: [] as number[],
    defaultNsId: null as number | null,
    offline: false,
    failSave: false,
  }
  const snapshot = () =>
    ok({
      items: state.ids.map((id) => folders.find((f) => f.nsId === id)),
      defaultNsId: state.defaultNsId,
    })
  const routes = {
    '/namespace/pins': snapshot,
    '/namespace/getRoot': ok(root),
    '/namespace/getChildren': ({ query }: ApiCall) =>
      ok(query.parentId === '1' ? [local, remote] : []),
    '/namespace/trace': ({ query }: ApiCall) =>
      ok(query.nsId === '1' ? [root] : [root, folders.find((f) => f.nsId === Number(query.nsId))]),
    '/photo/getByNamespace': ok([]),
    '/user/storage/health': ({ query }: ApiCall) =>
      ok(
        (query.ids ?? '')
          .split(',')
          .filter(Boolean)
          .map((id) => ({
            sourceId: Number(id),
            state: state.offline && id === '2' ? 'unreachable' : 'available',
          })),
      ),
    '/namespace/pin': ({ body }: ApiCall) => {
      if (state.failSave) return { msgCode: 500, t: null }
      const id = (body as { nsId: number }).nsId
      if (!state.ids.includes(id)) state.ids.push(id)
      return snapshot()
    },
    '/namespace/defaultLocation': ({ body }: ApiCall) => {
      state.defaultNsId = (body as { nsId: number | null }).nsId
      return snapshot()
    },
    '/namespace/unpin': ({ body }: ApiCall) => {
      const id = (body as { nsId: number }).nsId
      state.ids = state.ids.filter((value) => value !== id)
      if (state.defaultNsId === id) state.defaultNsId = null
      return snapshot()
    },
  }
  const setup = async (page: Page) => {
    await signIn(page)
    await mockApi(page, routes)
  }
  return { state, setup }
}

for (const [id, name] of [
  [2, 'Trips'],
  [3, 'Remote'],
] as const) {
  test(`${name}: pin, default startup, cross-device read and unpin`, async ({
    page,
    browser,
    isMobile,
  }) => {
    test.skip(isMobile, 'Pinned navigation is desktop-only')
    const f = fixture()
    await f.setup(page)
    await page.goto('/home/HomePart')
    const sidebar = page.locator('#main-sidebar')
    await expect(sidebar.getByLabel('Pinned folders')).toHaveCount(0)
    await page.locator(`[data-folder-id="${id}"]`).click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Pin to sidebar' }).click()
    const pin = sidebar.locator(`a[href="/home/HomePart?folder=${id}"]`)
    await expect(pin).toBeVisible()
    await pin.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Set as default location' }).click()
    await expect(pin.getByLabel('Default location')).toBeVisible()
    await expect(page).toHaveURL(/folder=1$/)

    await page.goto('/home/HomePart')
    await expect(page).toHaveURL(new RegExp(`folder=${id}$`))
    await expect(pin).toHaveAttribute('aria-current', 'page')
    const photos = sidebar.locator('nav a[href="/home/HomePart"]')
    await expect(photos).not.toHaveAttribute('aria-current')
    await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()
    await page.screenshot({ path: test.info().outputPath('pinned-sidebar.png') })

    const device = await browser.newContext()
    try {
      const second = await device.newPage()
      await f.setup(second)
      await second.goto('http://localhost:5173/home/HomePart')
      await expect(second).toHaveURL(new RegExp(`folder=${id}$`))
      await expect(second.locator('#main-sidebar [aria-label="Pinned folders"]')).toContainText(
        name,
      )
    } finally {
      await device.close()
    }

    await photos.click()
    await expect(page).toHaveURL(/folder=1$/)
    await page.reload()
    await expect(page).toHaveURL(/folder=1$/)
    await expect(photos).toHaveAttribute('aria-current', 'page')
    await pin.click()
    await expect(page).toHaveURL(new RegExp(`folder=${id}$`))
    await page.goBack()
    await expect(page).toHaveURL(/folder=1$/)
    await page.goForward()
    await expect(pin).toHaveAttribute('aria-current', 'page')
    await pin.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Unpin', exact: true }).click()
    await expect(sidebar.getByLabel('Pinned folders')).toHaveCount(0)
    await expect(page).toHaveURL(new RegExp(`folder=${id}$`))
    expect(f.state.defaultNsId).toBeNull()
    await page.goto('/home/HomePart')
    await expect(page).toHaveURL(/folder=1$/)
  })
}

test('offline default falls back without removing settings; explicit links retain their target', async ({
  page,
}) => {
  const f = fixture()
  f.state.ids = [3]
  f.state.defaultNsId = 3
  f.state.offline = true
  await f.setup(page)
  await page.goto('/home/HomePart')
  await expect(page).toHaveURL(/folder=1$/)
  await expect(page.getByText('Your default folder is unavailable. Opening Library.')).toBeVisible()
  expect(f.state.defaultNsId).toBe(3)
  expect(f.state.ids).toEqual([3])
  await page.goto('/home/HomePart?folder=2')
  await expect(page.getByRole('heading', { name: 'Trips', exact: true })).toBeVisible()
  await page.goto('/home/HomePart?folder=3')
  await expect(page.getByRole('heading', { name: 'Remote', exact: true })).toBeVisible()
  await expect(page).toHaveURL(/folder=3$/)
  f.state.offline = false
  await page.goto('/home/HomePart')
  await expect(page).toHaveURL(/folder=3$/)
})

test('failed Pin keeps the sidebar unchanged', async ({ page, isMobile }) => {
  test.skip(isMobile, 'Desktop context menu')
  const f = fixture()
  f.state.failSave = true
  await f.setup(page)
  await page.goto('/home/HomePart')
  await page.locator('[data-folder-id="2"]').click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Pin to sidebar' }).click()
  await expect(page.getByText('Could not save pinned folders. Please try again.')).toBeVisible()
  await expect(page.locator('#main-sidebar [aria-label="Pinned folders"]')).toHaveCount(0)
})

test('clearing the default keeps all pins and does not navigate', async ({ page, isMobile }) => {
  test.skip(isMobile, 'Desktop context menu')
  const f = fixture()
  f.state.ids = [2, 3]
  f.state.defaultNsId = 2
  await f.setup(page)
  await page.goto('/home/HomePart')
  await expect(page).toHaveURL(/folder=2$/)
  const pin = page.locator('#main-sidebar a[href="/home/HomePart?folder=2"]')
  await pin.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Clear default location' }).click()
  await expect(pin.getByLabel('Default location')).toHaveCount(0)
  await expect(page).toHaveURL(/folder=2$/)
  expect(f.state.ids).toEqual([2, 3])
  await page.goto('/home/HomePart')
  await expect(page).toHaveURL(/folder=1$/)
})

test('many long pin names scroll while account controls stay visible', async ({
  page,
  isMobile,
}) => {
  test.skip(isMobile, 'Desktop sidebar')
  await signIn(page)
  await mockApi(page, {
    '/namespace/pins': ok({
      items: Array.from({ length: 50 }, (_, i) => ({
        nsId: i + 2,
        nsName: `A long folder name for travel photographs ${i}`,
        path: `Library / Remote / Travel ${i}`,
        sourceType: 'webdav',
      })),
      defaultNsId: null,
    }),
  })
  await page.goto('/home/HomePart?folder=1')
  const sidebar = page.locator('#main-sidebar')
  const nav = sidebar.getByRole('navigation')
  await expect(sidebar.getByLabel('Pinned folders').getByRole('link')).toHaveCount(50)
  expect(await nav.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true)
  await expect(sidebar.getByRole('button', { name: 'Sign out' })).toBeVisible()
  const last = sidebar.getByLabel('Pinned folders').getByRole('link').last()
  await last.scrollIntoViewIfNeeded()
  await expect(last).toBeInViewport()
  await expect(last).toHaveAttribute('title', 'Library / Remote / Travel 49 (webdav)')
})
