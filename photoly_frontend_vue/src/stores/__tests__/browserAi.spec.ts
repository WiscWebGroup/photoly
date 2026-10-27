import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { webcrypto } from 'node:crypto'
import { http, HttpResponse } from 'msw'
import { API, ok, server } from '@/test/server'
import { browserCapabilities, fakeRuntime } from '@/test/fakeBrowserAi'
import { createBrowserAi } from '@/lib/browserAi'
import { setSharedBrowserAi, useBrowserAiStore } from '../browserAi'
import { useAuthStore } from '../auth'
import { useUploadsStore } from '../uploads'

beforeEach(() => {
  vi.stubGlobal('createImageBitmap', vi.fn())
  vi.stubGlobal('OffscreenCanvas', class {})
  vi.stubGlobal('crypto', { getRandomValues: webcrypto.getRandomValues.bind(webcrypto) })
  useAuthStore().setToken('t')
})
afterEach(() => {
  vi.unstubAllGlobals()
  setSharedBrowserAi(undefined)
})

function setup(caps: Record<string, unknown> = {}) {
  const fake = fakeRuntime()
  const ai = createBrowserAi({
    loadRuntime: async () => fake.runtime,
    isolated: () => true,
    cores: () => 4,
    cacheApi: () => undefined,
    prepare: async () => new Float32Array(3 * 224 * 224),
    reload: vi.fn(),
  })
  setSharedBrowserAi(ai)
  const store = useBrowserAiStore()
  store.setCapabilities(browserCapabilities(caps) as never)
  return { store, ai, fake }
}

describe('search', () => {
  it('embeds the query here and posts only the vector', async () => {
    const { store } = setup()
    const bodies: unknown[] = []
    server.use(
      http.post(`${API}/ai/search/vector`, async ({ request }) => {
        bodies.push(await request.json())
        return ok({ items: [], profileId: 1, coverage: {}, requestId: 'r' })
      }),
    )
    const response = await store.searchText('a cat', { topK: 5, galleryId: 2 })
    expect(response.requestId).toBe('r')
    expect(bodies).toEqual([{ topK: 5, galleryId: 2, fingerprint: 'fp-1', vector: [0.6, 0.8, 0] }])
  })
  it('sends the text to the server when the model runs there or the user asks', async () => {
    const { store, fake } = setup()
    const bodies: unknown[] = []
    server.use(
      http.post(`${API}/ai/search/text`, async ({ request }) => {
        bodies.push(await request.json())
        return ok({ items: [] })
      }),
    )
    await store.searchText('a cat', { topK: 5 }, { server: true })
    store.setCapabilities(
      browserCapabilities({ placement: { image: 'server', text: 'server' } }) as never,
    )
    await store.searchText('a dog', { topK: 5 })
    expect(bodies).toEqual([
      { topK: 5, text: 'a cat' },
      { topK: 5, text: 'a dog' },
    ])
    expect(fake.runtime.SiglipTextModel.from_pretrained).not.toHaveBeenCalled()
  })
  it('defaults to the server for capabilities without placement', async () => {
    const { store } = setup()
    store.setCapabilities(browserCapabilities({ placement: undefined }) as never)
    expect(store.textInBrowser).toBe(false)
    expect(store.imageInBrowser).toBe(false)
  })
  it('reports a model that cannot run here as a browser error', async () => {
    const { store, fake } = setup()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(fake.runtime.SiglipTextModel.from_pretrained).mockRejectedValueOnce(new Error('oom'))
    await expect(store.searchText('x', {})).rejects.toMatchObject({ code: 'browser_search_failed' })
  })
  it('keeps the AI error when the vector search is refused', async () => {
    const { store } = setup()
    server.use(
      http.post(`${API}/ai/search/vector`, () =>
        HttpResponse.json(
          { error: 'fingerprint_mismatch' },
          { status: 409, headers: { 'Access-Control-Allow-Origin': '*' } },
        ),
      ),
    )
    await expect(store.searchText('x', {})).rejects.toMatchObject({
      code: 'fingerprint_mismatch',
      status: 409,
    })
  })
})

describe('preference', () => {
  it('saves the choice and re-reads what is effective', async () => {
    const { store } = setup()
    const puts: unknown[] = []
    server.use(
      http.put(`${API}/ai/preference`, async ({ request }) => {
        puts.push(await request.json())
        return ok({})
      }),
      http.get(`${API}/ai/capabilities`, () =>
        ok(
          browserCapabilities({
            placement: {
              image: 'browser',
              text: 'server',
              choosable: { image: false, text: true },
            },
          }),
        ),
      ),
    )
    await store.setPreference('text', 'server')
    expect(puts).toEqual([{ text: 'server' }])
    expect(store.placementOf('text')).toBe('server')
  })
})

describe('indexing during upload', () => {
  function uploadFixtures(embedding: (body: unknown) => Response | undefined = () => undefined) {
    const order: string[] = []
    const embeddings: unknown[] = []
    server.use(
      http.post(`${API}/uploads`, async ({ request }) =>
        ok(((await request.json()) as { id: string }).id),
      ),
      http.post(`${API}/uploads/:id/items`, () => ok(1)),
      http.post(`${API}/uploads/:id/finish`, () => ok(1)),
      http.put(`${API}/uploads/:bid/items/:id/content`, ({ params }) => {
        order.push('content')
        return ok({ id: params.id, state: 'ready' })
      }),
      http.put(`${API}/uploads/:bid/items/:id/embedding`, async ({ request }) => {
        const body = await request.json()
        order.push('embedding')
        embeddings.push(body)
        return embedding(body) ?? ok(1)
      }),
    )
    const uploads = useUploadsStore()
    uploads.start(2, 'Trips')
    return { uploads, order, embeddings }
  }
  const files = (names: string[]) => names.map((n) => ({ file: new File(['x'], n), path: n }))

  it('PUTs the vector after the content, only for photos the browser can decode', async () => {
    const { store } = setup()
    server.use(http.get(`${API}/ai/capabilities`, () => ok(browserCapabilities())))
    const { uploads, order, embeddings } = uploadFixtures()
    await uploads.enqueue(files(['a.jpg', 'b.mov', 'c.zip']))
    await vi.waitFor(() => expect(uploads.indexing.done).toBe(1))
    await vi.waitFor(() => expect(uploads.local).toHaveLength(0))
    expect(order.filter((o) => o === 'content')).toHaveLength(3)
    expect(order.indexOf('embedding')).toBeGreaterThan(order.indexOf('content'))
    expect(embeddings).toEqual([{ fingerprint: 'fp-1', vector: [0.6, 0.8, 0] }])
    expect(uploads.indexing).toMatchObject({ total: 1, done: 1, failed: 0 })
    expect(store.imageInBrowser).toBe(true)
  })
  it('falls back to the server when the vector is rejected', async () => {
    setup()
    server.use(http.get(`${API}/ai/capabilities`, () => ok(browserCapabilities())))
    const { uploads } = uploadFixtures(() =>
      HttpResponse.json(
        { msgCode: 400, t: 'rejected' },
        { headers: { 'Access-Control-Allow-Origin': '*' } },
      ),
    )
    await uploads.enqueue(files(['a.jpg', 'b.png']))
    await vi.waitFor(() => expect(uploads.indexing.failed).toBe(2))
    expect(uploads.indexing.done).toBe(0)
  })
  it('does not index in the browser when the server does', async () => {
    setup()
    server.use(
      http.get(`${API}/ai/capabilities`, () =>
        ok(browserCapabilities({ placement: { image: 'server', text: 'server' } })),
      ),
    )
    const { uploads, embeddings } = uploadFixtures()
    await uploads.enqueue(files(['a.jpg']))
    await vi.waitFor(() => expect(uploads.local).toHaveLength(0))
    expect(embeddings).toHaveLength(0)
    expect(uploads.indexing.total).toBe(0)
  })
  it('stops trying the browser model after it fails to load', async () => {
    const { fake } = setup()
    server.use(http.get(`${API}/ai/capabilities`, () => ok(browserCapabilities())))
    vi.mocked(fake.runtime.SiglipVisionModel.from_pretrained).mockRejectedValueOnce(
      new Error('oom'),
    )
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const { uploads, embeddings } = uploadFixtures()
    await uploads.enqueue(files(['a.jpg', 'b.jpg', 'c.jpg']))
    await vi.waitFor(() => expect(uploads.indexing.failed).toBe(3))
    expect(fake.runtime.SiglipVisionModel.from_pretrained).toHaveBeenCalledTimes(1)
    expect(embeddings).toHaveLength(0)
  })
})

describe('indexLibrary', () => {
  it('lets the server queue its share and embeds nothing here', async () => {
    const { store } = setup()
    server.use(
      http.post(`${API}/ai/index`, () =>
        ok({ placement: 'server', serverQueued: 3, browserPending: 0 }),
      ),
    )
    await store.indexLibrary()
    expect(store.library).toMatchObject({
      phase: 'done',
      placement: 'server',
      serverQueued: 3,
      done: 0,
    })
  })

  it('embeds each pending photo in this browser and counts unreadable ones', async () => {
    const { store } = setup()
    const puts: number[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) =>
        url.includes('photoId=2')
          ? new Response('', { status: 404 })
          : new Response(new Blob(['x'])),
      ),
    )
    server.use(
      http.post(`${API}/ai/index`, () =>
        ok({ placement: 'browser', serverQueued: 0, browserPending: 2 }),
      ),
      http.get(`${API}/ai/index/pending`, ({ request }) =>
        ok({
          items:
            new URL(request.url).searchParams.get('after') === '0'
              ? [{ photoId: 1 }, { photoId: 2 }]
              : [],
        }),
      ),
      http.put(`${API}/ai/photos/:id/embedding`, ({ params }) => {
        puts.push(Number(params.id))
        return ok({ status: 'stored' })
      }),
    )
    await store.indexLibrary()
    expect(puts).toEqual([1])
    expect(store.library).toMatchObject({ phase: 'done', total: 2, done: 1, failed: 1 })
  })
})
