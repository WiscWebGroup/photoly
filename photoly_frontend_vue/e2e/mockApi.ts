import type { Page, Request } from '@playwright/test'
import { capabilitiesFixture } from '../src/test/aiFixtures'

// Must match VITE_APP_BASE_URL in .env (the dev server Playwright starts).
export const API = 'http://localhost:8384'
export const TOKEN = 'e2e-token'

export const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=',
  'base64',
)

/** Backend envelope: `{ msgCode, t }`. */
export const ok = (t: unknown = null, extra: object = {}) => ({ msgCode: 200, t, ...extra })

export type ApiCall = {
  method: string
  path: string
  query: Record<string, string>
  body: unknown
}
type Reply = object | ((call: ApiCall) => object)

const cors = {
  'access-control-allow-origin': '*',
  'access-control-allow-headers': '*',
  'access-control-allow-methods': '*',
}

// What a freshly signed-up user with an empty library sees.
const defaults: Record<string, Reply> = {
  '/ai/smart/settings': ok({ enabled: false, available: false, reason: 'smart_tag_disabled' }),
  '/ai/smart/status': ok({
    processed: 0,
    total: 0,
    failed: 0,
    reason: 'smart_tag_disabled',
    pending: false,
  }),
  '/ai/smart/dismissed': ok({ items: [], nextAfter: null }),
  '/ai/capabilities': ok({
    ...capabilitiesFixture,
    enabled: false,
    globalEnabled: false,
    ready: false,
    reason: 'disabled',
  }),
  '/user/pingToken': ok(),
  '/namespace/getRoot': ok({ nsId: 1, nsName: 'root' }),
  '/namespace/trace': ok([{ nsId: 1, nsName: 'root' }]),
}

function parseBody(req: Request) {
  try {
    return req.postDataJSON()
  } catch {
    return req.postData()
  }
}

/**
 * Fakes the backend: image endpoints get a 1px PNG, `routes` (keyed by path
 * without trailing slash) override `defaults`, anything else is `ok([])`.
 * Returns the list of API calls made, for asserting on.
 */
export async function mockApi(page: Page, routes: Record<string, Reply> = {}) {
  const calls: ApiCall[] = []
  await page.route('https://*.tile.openstreetmap.org/**', (route) =>
    route.fulfill({ body: PNG, contentType: 'image/png' }),
  )
  await page.route(`${API}/**`, async (route) => {
    const req = route.request()
    if (req.method() === 'OPTIONS') return route.fulfill({ status: 204, headers: cors })

    const url = new URL(req.url())
    const path = url.pathname.replace(/\/$/, '')
    if (/^\/(photo\/(render|renderThumbnail|getQR2)|user\/getAvatar)\b/.test(path)) {
      return route.fulfill({ body: PNG, contentType: 'image/png', headers: cors })
    }

    const call = {
      method: req.method(),
      path,
      query: Object.fromEntries(url.searchParams),
      body: parseBody(req),
    }
    calls.push(call)
    const reply = routes[path] ?? defaults[path] ?? ok([])
    await route.fulfill({ json: typeof reply === 'function' ? reply(call) : reply, headers: cors })
  })
  return calls
}

export async function signIn(page: Page) {
  await page.addInitScript((token) => localStorage.setItem('HRD-Token', token), TOKEN)
}

export const photo = (photoId: number, photoName: string, extra: object = {}) => ({
  photoId,
  photoName,
  format: 'jpg',
  nsId: 1,
  uploadDate: '2024-06-01',
  visibility: 0,
  ...extra,
})
