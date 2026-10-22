import type { Page } from '@playwright/test'
import { API, ok } from './mockApi'
import type { UploadItem } from '../src/stores/uploads'

export async function mockUploads(
  page: Page,
  options: { delay?: number; imported?: (item: UploadItem) => void } = {},
) {
  const batches: {
    id: string
    name: string
    nsId: number
    sealed: boolean
    cancelled: boolean
    createdAt: string
  }[] = []
  const items: UploadItem[] = []
  let nextPhoto = 10
  const cors = {
    'access-control-allow-origin': '*',
    'access-control-allow-headers': '*',
    'access-control-allow-methods': '*',
  }
  await page.route(`${API}/uploads**`, async (route) => {
    const request = route.request(),
      url = new URL(request.url()),
      parts = url.pathname.split('/').filter(Boolean)
    if (request.method() === 'OPTIONS') return route.fulfill({ status: 204, headers: cors })
    let response: unknown = 1
    if (parts.length === 1 && request.method() === 'POST') {
      const data = request.postDataJSON()
      batches.unshift({
        ...data,
        sealed: false,
        cancelled: false,
        createdAt: new Date().toISOString(),
      })
      response = data.id
    } else if (parts.length === 1) {
      response = {
        batches: batches.map((b) => ({
          ...b,
          counts: Object.fromEntries(
            ['pending', 'ready', 'done', 'failed', 'skipped', 'cancelled'].map((state) => {
              const group = items.filter((i) => i.batchId === b.id && i.state === state)
              return [state, { count: group.length, bytes: group.reduce((n, i) => n + i.size, 0) }]
            }),
          ),
        })),
        total: batches.length,
        maxFileBytes: 629145600,
      }
    } else if (parts[2] === 'items' && parts.length === 3 && request.method() === 'POST') {
      items.push(
        ...request
          .postDataJSON()
          .items.map((i: UploadItem) => ({
            ...i,
            batchId: parts[1]!,
            state: 'pending',
            error: null,
            photoId: null,
            nsId: null,
            sourceId: null,
          })),
      )
    } else if (parts[2] === 'items' && parts.length === 3) {
      const all = items.filter((i) => i.batchId === parts[1]),
        p = Number(url.searchParams.get('page') ?? 1)
      response = { items: all.slice((p - 1) * 50, p * 50), total: all.length }
    } else if (parts[4] === 'content') {
      if (options.delay) await new Promise((resolve) => setTimeout(resolve, options.delay))
      const item = items.find((i) => i.id === parts[3])!
      item.state = 'done'
      item.photoId = nextPhoto++
      item.nsId = batches.find((b) => b.id === parts[1])!.nsId
      options.imported?.(item)
      response = item
    } else if (parts[2] === 'finish') batches.find((b) => b.id === parts[1])!.sealed = true
    await route.fulfill({ json: ok(response), headers: cors })
  })
  return { batches, items }
}
