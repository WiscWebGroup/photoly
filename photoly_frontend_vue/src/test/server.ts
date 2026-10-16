import { http, HttpResponse, type JsonBodyType } from 'msw'
import { setupServer } from 'msw/node'
import { capabilitiesFixture } from './aiFixtures'

export const API = import.meta.env.VITE_APP_BASE_URL

// The API is cross-origin to jsdom's page, so jsdom enforces CORS (incl. preflight).
const cors = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'HRD-Token, Content-Type, Idempotency-Key',
  'Access-Control-Allow-Methods': 'GET, POST, PATCH',
}
export const json = (body: JsonBodyType) => HttpResponse.json(body, { headers: cors })

export const server = setupServer(
  http.get(`${API}/ai/smart/settings`, () =>
    json({ msgCode: 200, t: { enabled: false, available: false, reason: 'smart_tag_disabled' } }),
  ),
  http.get(`${API}/ai/smart/status`, () =>
    json({
      msgCode: 200,
      t: { processed: 0, total: 0, failed: 0, reason: 'smart_tag_disabled', pending: false },
    }),
  ),
  http.get(`${API}/ai/smart/dismissed`, () =>
    json({ msgCode: 200, t: { items: [], nextAfter: null } }),
  ),
  http.get(`${API}/ai/capabilities`, () =>
    json({
      msgCode: 200,
      t: {
        ...capabilitiesFixture,
        enabled: false,
        globalEnabled: false,
        ready: false,
        reason: 'disabled',
      },
    }),
  ),
  http.options('*', () => new HttpResponse(null, { status: 204, headers: cors })),
)

/** Backend envelope: `{ msgCode, t }`. */
export const ok = (t: unknown = null) => json({ msgCode: 200, t })

/** Records every request msw sees, for asserting on API calls. */
export function recordRequests() {
  const calls: { method: string; path: string; query: Record<string, string>; body: unknown }[] = []
  server.events.on('request:start', async ({ request }) => {
    if (request.method === 'OPTIONS') return
    const url = new URL(request.url)
    const text = await request.clone().text()
    let body: unknown = text
    try {
      body = JSON.parse(text)
    } catch {
      // not JSON
    }
    calls.push({
      method: request.method,
      path: url.pathname,
      query: Object.fromEntries(url.searchParams),
      body,
    })
  })
  return calls
}
