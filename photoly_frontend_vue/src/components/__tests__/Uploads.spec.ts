import { describe, expect, it, vi } from 'vitest'
import { http, delay } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import { useUploadsStore } from '@/stores/uploads'
import { useAuthStore } from '@/stores/auth'
import UploadCenter from '../UploadCenter.vue'
import { droppedFiles } from '@/lib/uploadFiles'

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

describe('directory drag and drop', () => {
  it('reads every directory-reader batch and preserves nested relative paths', async () => {
    let calls = 0
    const file = {
      isFile: true,
      name: 'a.jpg',
      file: (resolve: (f: File) => void) => resolve(new File(['x'], 'a.jpg')),
    }
    const directory = {
      isFile: false,
      name: 'Folder',
      createReader: () => ({
        readEntries: (resolve: (entries: unknown[]) => void) => resolve(calls++ < 2 ? [file] : []),
      }),
    }
    const result = await droppedFiles({
      items: [{ webkitGetAsEntry: () => directory }],
      files: [],
    } as unknown as DataTransfer)
    expect(result.map((i) => i.path)).toEqual(['Folder/a.jpg', 'Folder/a.jpg'])
    expect(calls).toBe(3)
  })
})
