import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { webcrypto } from 'node:crypto'
import { http, delay, HttpResponse } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import { useUploadsStore } from '@/stores/uploads'
import { useAuthStore } from '@/stores/auth'
import UploadCenter from '../UploadCenter.vue'
import { droppedFiles } from '@/lib/uploadFiles'

// Exercise all queue scenarios under the same API availability as ordinary LAN HTTP.
beforeEach(() =>
  vi.stubGlobal('crypto', { getRandomValues: webcrypto.getRandomValues.bind(webcrypto) }),
)
afterEach(() => vi.unstubAllGlobals())

function fixtures() {
  const manifest: { id: string; path: string; size: number }[] = []
  server.use(
    http.post(`${API}/uploads`, async ({ request }) =>
      ok(((await request.json()) as { id: string }).id),
    ),
    http.post(`${API}/uploads/:id/items`, async ({ request }) => {
      manifest.push(...((await request.json()) as { items: typeof manifest }).items)
      return ok(1)
    }),
    http.post(`${API}/uploads/:id/finish`, () => ok(1)),
    http.put(`${API}/uploads/:bid/items/:id/content`, ({ params }) =>
      ok({ id: params.id, state: 'ready' }),
    ),
  )
  useAuthStore().setToken('test-upload-token')
  const uploads = useUploadsStore()
  uploads.start(2, 'Trips')
  return { uploads, manifest }
}

it('queues 100 files without truncation, at most two transmissions at once', async () => {
  const { uploads, manifest } = fixtures()
  let active = 0,
    maximum = 0,
    sent = 0
  server.use(
    http.put(`${API}/uploads/:bid/items/:id/content`, async ({ params }) => {
      active++
      maximum = Math.max(maximum, active)
      await delay(5)
      active--
      sent++
      return ok({ id: params.id, state: 'ready' })
    }),
  )
  await uploads.enqueue(
    Array.from({ length: 100 }, (_, i) => ({
      file: new File(['jpeg'], `${i}.jpg`),
      path: `Folder/${i}.jpg`,
    })),
  )
  uploads.pickerOpen = false
  uploads.open = false
  await vi.waitFor(() => expect(sent).toBe(100), { timeout: 10000 })
  expect(maximum).toBe(2)
  expect(manifest).toHaveLength(100)
  expect(manifest[99]!.path).toBe('Folder/99.jpg')
  expect(new Set(manifest.map((i) => i.id)).size).toBe(100)
  await vi.waitFor(() => expect(uploads.local).toHaveLength(0))
})

it('registers 1000 entries in pages and retains unsupported entries for skip reporting', async () => {
  const { uploads, manifest } = fixtures()
  const calls = recordRequests()
  await uploads.enqueue(
    Array.from({ length: 1000 }, (_, i) => ({
      file: new File(['x'], `${i}.txt`),
      path: `${i}.txt`,
    })),
  )
  expect(manifest).toHaveLength(1000)
  expect(calls.filter((c) => c.path.endsWith('/items'))).toHaveLength(2)
  expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(0)
})

it('clears pending files and cancels transmissions on account change', async () => {
  const { uploads } = fixtures()
  let calls = 0
  server.use(
    http.put(`${API}/uploads/:bid/items/:id/content`, async () => {
      calls++
      await delay(100)
      return ok({ state: 'ready' })
    }),
  )
  await uploads.enqueue(
    Array.from({ length: 10 }, (_, i) => ({ file: new File(['x'], `${i}.jpg`), path: `${i}.jpg` })),
  )
  await vi.waitFor(() => expect(calls).toBe(2))
  useAuthStore().setToken('different-account')
  expect(uploads.local).toHaveLength(0)
  expect(uploads.batches).toHaveLength(0)
  await delay(150)
  expect(calls).toBe(2)
})

it('resumes only pending files that match their original path and size', async () => {
  const { uploads } = fixtures()
  uploads.start(2, 'Trips', 'existing')
  server.use(
    http.get(`${API}/uploads/existing/items`, () =>
      ok({
        items: [{ id: 'item', path: 'Trip/a.jpg', size: 4, state: 'pending', sourceId: null }],
        total: 1,
      }),
    ),
  )
  const calls = recordRequests()
  await uploads.enqueue([{ file: new File(['jpeg'], 'a.jpg'), path: 'Trip/a.jpg' }])
  await vi.waitFor(() =>
    expect(calls.some((c) => c.path === '/uploads/existing/items/item/content')).toBe(true),
  )
  expect(calls.some((c) => c.path === '/uploads' && c.method === 'POST')).toBe(false)
})

it('renders a bounded picker and keeps the dialog closable', async () => {
  mountWithApp(UploadCenter)
  const { uploads } = fixtures()
  await vi.waitFor(() =>
    expect(document.querySelector('[aria-label="Choose files or ZIP"]')).not.toBeNull(),
  )
  const input = document.querySelector<HTMLInputElement>('[aria-label="Choose files or ZIP"]')!
  Object.defineProperty(input, 'files', {
    value: Array.from({ length: 100 }, (_, i) => new File(['x'], `${i}.jpg`)),
  })
  input.dispatchEvent(new Event('change', { bubbles: true }))
  await vi.waitFor(() => expect(pageText()).toContain('100 files'))
  expect(document.querySelectorAll('li')).toHaveLength(50)
  button('Next').click()
  await vi.waitFor(() => expect(pageText()).toContain('99.jpg'))
  uploads.pickerOpen = false
  await vi.waitFor(() => expect(document.querySelector('[role="dialog"]')).toBeNull())
})

it.each(['single.jpg', 'archive.zip'])('queues %s without randomUUID', async (name) => {
  const { uploads, manifest } = fixtures()
  const calls = recordRequests()
  await uploads.enqueue([{ file: new File(['content'], name), path: name }])
  await vi.waitFor(() => expect(uploads.local).toHaveLength(0))
  expect(manifest).toHaveLength(1)
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
  expect(manifest[0]!.id).toMatch(uuid)
  const create = calls.find((c) => c.path === '/uploads' && c.method === 'POST')!
  expect((create.body as { id: string }).id).toMatch(uuid)
  expect((create.body as { id: string }).id).not.toBe(manifest[0]!.id)
})

it('retries a failed transmission with the same batch and item IDs', async () => {
  const { uploads, manifest } = fixtures()
  const sent: string[] = []
  server.use(
    http.put(`${API}/uploads/:bid/items/:id/content`, ({ request, params }) => {
      sent.push(new URL(request.url).pathname)
      return sent.length === 1
        ? HttpResponse.json({ msgCode: 503, t: 'transient failure' })
        : ok({ id: params.id, state: 'ready' })
    }),
    http.post(`${API}/uploads/:bid/retry`, () => ok(1)),
    http.get(`${API}/uploads/:bid/items`, () =>
      ok({ items: manifest.map((i) => ({ ...i, state: 'pending' })), total: manifest.length }),
    ),
  )
  await uploads.enqueue([{ file: new File(['jpeg'], 'a.jpg'), path: '旅行/北京/a.jpg' }])
  await vi.waitFor(() => expect(uploads.local[0]?.state).toBe('failed'))
  const batchId = uploads.local[0]!.batchId
  await uploads.action(batchId, 'retry')
  await vi.waitFor(() => expect(sent).toHaveLength(2))
  expect(sent[0]).toBe(`/uploads/${batchId}/items/${manifest[0]!.id}/content`)
  expect(sent[1]).toBe(sent[0])
  expect(manifest).toHaveLength(1)
  await vi.waitFor(() => expect(uploads.local).toHaveLength(0))
})

describe('directory drag and drop', () => {
  it('reads more than 100 entries and retains the outer folder and nested same-name files', async () => {
    const file = (name: string) => ({
      isFile: true,
      name,
      file: (resolve: (f: File) => void) => resolve(new File(['x'], name)),
    })
    let rootReads = 0
    function directory(name: string, entries: unknown[]) {
      return {
        isFile: false,
        name,
        createReader: () => {
          let offset = 0
          return {
            readEntries: (resolve: (entries: unknown[]) => void) => {
              if (name === '旅行') rootReads++
              resolve(entries.slice(offset, (offset += 100)))
            },
          }
        },
      }
    }
    const root = directory('旅行', [
      ...Array.from({ length: 150 }, (_, i) => file(`${i}.jpg`)),
      directory('北京', [directory('故宫', [file('same.jpg')])]),
      directory('上海', [file('same.jpg')]),
      directory('空目录', []),
    ])
    const result = await droppedFiles({
      items: [{ webkitGetAsEntry: () => root }],
      files: [],
    } as unknown as DataTransfer)
    expect(result).toHaveLength(152)
    expect(new Set(result.map((i) => i.path)).size).toBe(152)
    expect(result.every((i) => i.path.startsWith('旅行/'))).toBe(true)
    expect(result.map((i) => i.path)).toContain('旅行/北京/故宫/same.jpg')
    expect(result.map((i) => i.path)).toContain('旅行/上海/same.jpg')
    expect(rootReads).toBe(3)
  })
})

it('pastes images once per event, keeps text paste, and waits for Upload', async () => {
  mountWithApp(UploadCenter)
  const { uploads, manifest } = fixtures()
  await vi.waitFor(() => expect(document.querySelector('[role="dialog"]')).not.toBeNull())
  const dialog = document.querySelector('[role="dialog"]')!
  function paste(items: { kind: string; type: string; getAsFile: () => File | null }[]) {
    const event = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'clipboardData', { value: { items } })
    dialog.dispatchEvent(event)
    return event
  }
  const file = new File(['png'], 'image.png', { type: 'image/png' })
  const item = { kind: 'file', type: 'image/png', getAsFile: () => file }
  expect(paste([item]).defaultPrevented).toBe(true)
  expect(
    paste([{ kind: 'string', type: 'text/plain', getAsFile: () => null }]).defaultPrevented,
  ).toBe(false)
  expect(paste([]).defaultPrevented).toBe(false)
  paste([item])
  await vi.waitFor(() => expect(pageText()).toContain('2 files'))
  expect(manifest).toHaveLength(0)
  button('Upload').click()
  await vi.waitFor(() => expect(manifest).toHaveLength(2))
  expect(new Set(manifest.map((item) => item.path)).size).toBe(2)
  expect(manifest[0]!.path).toMatch(/^Clipboard .*\.png$/)
  uploads.pickerOpen = false
})

it('dismisses failure alerts while retaining retryable history, then clears finished activity', async () => {
  const { uploads } = fixtures()
  let failedCount = 1
  let batches = [
    {
      id: 'finished',
      name: 'Failed import',
      sealed: true,
      cancelled: false,
      nsId: 2,
      createdAt: '2026-01-01',
      counts: { failed: { count: 1, bytes: 4 } },
    },
  ]
  server.use(
    http.get(`${API}/uploads`, () =>
      ok({ batches, total: batches.length, maxFileBytes: 1e6, failedCount }),
    ),
    http.post(`${API}/uploads/acknowledge-failures`, () => {
      failedCount = 0
      return ok(1)
    }),
    http.delete(`${API}/uploads`, () => {
      batches = []
      return ok(['finished'])
    }),
  )
  await uploads.refresh()
  expect(uploads.failed).toBe(true)
  await uploads.manageActivity('acknowledge')
  expect(uploads.failed).toBe(false)
  expect(uploads.batches[0]!.counts.failed!.count).toBe(1)
  await uploads.refresh()
  expect(uploads.failed).toBe(false)
  failedCount = 1
  await uploads.refresh()
  expect(uploads.failed).toBe(true)
  failedCount = 0
  uploads.page = 2
  await uploads.manageActivity('clear')
  expect(uploads.batches).toHaveLength(0)
  expect(uploads.page).toBe(1)
  expect(uploads.failed).toBe(false)
})
