import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import { http as client } from '@/lib/http'
import { API, ok, server } from '@/test/server'
import { useAuthStore } from '../auth'
import { usePeopleStore } from '../people'

const preference = {
  enabled: true,
  paused: false,
  placement: 'browser',
  generation: 1,
  available: true,
  visible: true,
  choosable: true,
  reason: '',
}
const job = {
  id: 1,
  token: 'lease-1',
  photoId: 2,
  version: 1,
  fingerprint: 'test',
  generation: 1,
  placement: 'browser',
  status: 'running',
}
class FaceWorker {
  static instances: FaceWorker[] = []
  onmessage?: (event: { data: unknown }) => void
  onerror?: (event: { message: string }) => void
  postMessage = vi.fn()
  terminate = vi.fn()
  constructor() {
    FaceWorker.instances.push(this)
  }
}
let store: ReturnType<typeof usePeopleStore>
beforeEach(async () => {
  vi.stubGlobal('Worker', FaceWorker)
  vi.spyOn(client, 'post').mockResolvedValue({ data: new ArrayBuffer(8) })
  FaceWorker.instances = []
  useAuthStore().setToken('people-session')
  store = usePeopleStore()
  await flushPromises()
  server.use(
    http.get(`${API}/ai/people/settings`, () => ok({ preference, model: { fingerprint: 'test' } })),
    http.get(`${API}/ai/people/status`, () =>
      ok({ preference, total: 1, ready: 0, failed: 0, processing: 0, pending: 1 }),
    ),
    http.post(`${API}/ai/people/jobs/claim`, () => ok({ job })),
  )
})
afterEach(() => {
  store.stop()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

it('submits a zero-face success with the exact lease and never switches placement', async () => {
  const submitted: unknown[] = []
  server.use(
    http.post(`${API}/ai/people/jobs/result`, async ({ request }) => {
      submitted.push(await request.json())
      return ok({ stored: true })
    }),
  )
  store.start()
  await vi.waitFor(() => expect(FaceWorker.instances[0]?.postMessage).toHaveBeenCalled())
  FaceWorker.instances[0]!.onmessage?.({ data: { faces: [], device: 'wasm' } })
  await vi.waitFor(() => expect(submitted).toEqual([{ job, faces: [] }]))
  expect(store.device).toBe('wasm')
  expect(store.preference?.placement).toBe('browser')
})

it('stopping during a settings refresh cannot start a new job afterward', async () => {
  let release!: () => void
  let claims = 0
  const pending = new Promise<void>((resolve) => {
    release = resolve
  })
  server.use(
    http.get(`${API}/ai/people/settings`, async () => {
      await pending
      return ok({ preference, model: {} })
    }),
    http.post(`${API}/ai/people/jobs/claim`, () => {
      claims++
      return ok({ job })
    }),
  )
  store.start()
  store.stop()
  release()
  await flushPromises()
  await flushPromises()
  expect(claims).toBe(0)
  expect(FaceWorker.instances).toHaveLength(0)
})

it('account pause terminates in-flight inference and discards a late result', async () => {
  let submitted = 0
  const paused = { ...preference, paused: true, reason: 'paused' }
  server.use(
    http.post(`${API}/ai/people/jobs/result`, () => {
      submitted++
      return ok({ stored: true })
    }),
  )
  store.start()
  await vi.waitFor(() => expect(FaceWorker.instances[0]?.postMessage).toHaveBeenCalled())
  const worker = FaceWorker.instances[0]!
  server.use(
    http.patch(`${API}/ai/people/settings`, () => ok({ preference: paused, model: {} })),
    http.get(`${API}/ai/people/settings`, () => ok({ preference: paused, model: {} })),
  )
  await store.change({ paused: true })
  worker.onmessage?.({ data: { faces: [], device: 'wasm' } })
  await flushPromises()
  expect(worker.terminate).toHaveBeenCalled()
  expect(submitted).toBe(0)
  expect(store.visible).toBe(true)
  expect(store.working).toBe(false)
})

it('server placement never creates a browser worker or claims a browser task', async () => {
  let claims = 0
  server.use(
    http.get(`${API}/ai/people/settings`, () =>
      ok({ preference: { ...preference, placement: 'server' }, model: {} }),
    ),
    http.post(`${API}/ai/people/jobs/claim`, () => {
      claims++
      return ok({ job })
    }),
  )
  store.start()
  await vi.waitFor(() => expect(store.status).not.toBeNull())
  expect(claims).toBe(0)
  expect(FaceWorker.instances).toHaveLength(0)
})
