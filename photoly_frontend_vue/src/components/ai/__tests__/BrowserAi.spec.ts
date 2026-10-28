import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import { API, ok, server } from '@/test/server'
import { browserCapabilities } from '@/test/fakeBrowserAi'
import { button, mountWithApp, pageText } from '@/test/utils'
import { AiError, type AiSettings, type AiStatus } from '@/lib/ai'
import { setSharedBrowserAi, useBrowserAiStore } from '@/stores/browserAi'
import { useUploadsStore } from '@/stores/uploads'
import AdminInferenceCard from '../AdminInferenceCard.vue'
import AiDeviceSettings from '../AiDeviceSettings.vue'
import BrowserModelLoading from '../BrowserModelLoading.vue'
import UploadCenter from '@/components/UploadCenter.vue'

afterEach(() => setSharedBrowserAi(undefined))

const GIB = 1024 ** 3
const status = (free = 1.5, worker: object = { running: true, memoryMB: 819, models: { image: { loaded: true, variant: 'fp32', loadedAt: new Date().toISOString() }, text: { loaded: false } } }) =>
  ({
    settings: {},
    services: {
      runtime: { online: true, heartbeatAt: null, observed: { state: 'ready', resources: { memory_total: 8 * GIB, memory_available: free * GIB, sampled_at: new Date().toISOString() } } },
    },
    worker,
    indexSources: { server: 924, browser: 260, waiting: 100, spotCheck: { checked: 26, matched: 26 } },
  }) as unknown as AiStatus
const radios = (group: string) =>
  [...document.querySelectorAll<HTMLButtonElement>(`[role=radiogroup][aria-labelledby="${group}"] [role=radio]`)]
const draft = { placement_image: 'server', placement_text: 'user', browser_text_model: 'small' } as unknown as AiSettings

function card(load = vi.fn().mockResolvedValue({}), unload = vi.fn().mockResolvedValue({}), stat = status()) {
  const w = mountWithApp(AdminInferenceCard, { draft, status: stat, stale: false, busy: false, load, unload })
  return { w, load, unload }
}

describe('Admin · Where AI runs', () => {
  it('shows both models with Server / Browser / Let users choose, memory, fits and worker', () => {
    card()
    expect(radios('inf-image').map((r) => r.textContent!.trim())).toEqual(['Server', 'Browser', 'Let users choose'])
    expect(radios('inf-text')[2]!.getAttribute('aria-checked')).toBe('true')
    const text = pageText()
    expect(text).toContain('1.5 GB free of 8 GB')
    expect(text).toContain('Search model, small')
    expect(text).toContain('Fits')
    expect(text).toContain("Doesn't fit") // the full text model needs 2.2 GB
    expect(text).toContain('Running, 0.8 GB')
    expect(text).toContain('Browsers: 260')
    expect(text).toContain('26 of 26 matched')
  })
  it('emits a placement change for Browser without touching the server', async () => {
    const { w, load } = card()
    radios('inf-text')[1]!.click()
    await flushPromises()
    expect(w.emitted('set')).toEqual([[{ placement_text: 'browser' }]])
    expect(load).not.toHaveBeenCalled()
  })
  it('loads the model when Server is picked and asks before loading what may not fit', async () => {
    const load = vi
      .fn()
      .mockRejectedValueOnce(new AiError('low-memory', 409, { code: 'low-memory', needMB: 900, availableMB: 600 }))
      .mockResolvedValue({})
    const { w } = card(load)
    radios('inf-text')[0]!.click()
    await flushPromises()
    expect(load).toHaveBeenCalledWith('text', false)
    expect(pageText()).toContain('Not enough memory for server search')
    expect(pageText()).toContain('0.9 GB')
    expect(pageText()).toContain('0.6 GB free')
    expect(w.emitted('set')).toBeUndefined()
    button('Load Anyway').click()
    await flushPromises()
    expect(load).toHaveBeenLastCalledWith('text', true)
    expect(w.emitted('set')).toEqual([[{ placement_text: 'server' }]])
  })
  it('keeps the placement when the dialog is declined', async () => {
    const load = vi.fn().mockRejectedValue(new AiError('low-memory', 409, { needMB: 900, availableMB: 600 }))
    const { w } = card(load)
    radios('inf-text')[0]!.click()
    await flushPromises()
    button('Keep in Browsers').click()
    await flushPromises()
    expect(w.emitted('set')).toBeUndefined()
    expect(load).toHaveBeenCalledTimes(1)
  })
  it('reports a dead worker instead of switching', async () => {
    const load = vi.fn().mockRejectedValue(new AiError('worker-died', 503))
    const { w } = card(load)
    radios('inf-text')[0]!.click()
    await flushPromises()
    expect(w.emitted('error')![0]![0]).toMatch(/AI worker stopped/)
    expect(w.emitted('set')).toBeUndefined()
  })
  it('unloads a loaded model from the worker', async () => {
    const { unload } = card()
    button('Unload image model').click()
    await flushPromises()
    expect(unload).toHaveBeenCalledWith('image')
  })
})

describe('Account · AI on this device', () => {
  function mountDevice(caps: Record<string, unknown> = {}) {
    const w = mountWithApp(AiDeviceSettings)
    const store = useBrowserAiStore(w.vm.$pinia)
    store.setCapabilities(browserCapabilities(caps) as never)
    return { w, store }
  }
  it('lets the user choose where search runs and saves it', async () => {
    const puts: unknown[] = []
    server.use(
      http.put(`${API}/ai/preference`, async ({ request }) => {
        puts.push(await request.json())
        return ok({})
      }),
    )
    mountDevice()
    await flushPromises()
    const choices = radios('ai-text-l')
    expect(choices.map((c) => c.textContent)).toEqual([
      expect.stringContaining('In this browser'),
      expect.stringContaining('On the server'),
    ])
    choices[1]!.click()
    await vi.waitFor(() => expect(puts).toEqual([{ text: 'server' }]))
  })
  it('shows what the admin fixed for everyone as locked', async () => {
    mountDevice()
    await flushPromises()
    expect(pageText()).toContain('Set by your admin for everyone.')
    expect(document.querySelector('[aria-labelledby="ai-image-l"]')).toBeNull()
    expect(document.querySelector('[aria-labelledby="ai-text-l"]')).not.toBeNull()
  })
  it('offers keep and remove, and the small and full search model', async () => {
    const { store } = mountDevice()
    store.saved = { text: true, image: false }
    store.storage = { usage: 1, quota: 2, persisted: false }
    await flushPromises()
    expect(button('Keep on This Device')).toBeTruthy()
    expect(button('Remove from This Device')).toBeTruthy()
    const options = [...document.querySelectorAll('#ai-text-model option')].map((o) => o.textContent)
    expect(options).toEqual(['Small, 318 MB (recommended)', 'Full, 1.13 GB'])
  })
  it('is empty on servers without browser inference', async () => {
    mountDevice({ placement: undefined })
    await flushPromises()
    expect(document.body.textContent).not.toContain('Where AI runs for you')
  })
})

describe('Search · model loading', () => {
  function loading(source: 'download' | 'device', loaded: number, total: number) {
    const w = mountWithApp(BrowserModelLoading)
    const store = useBrowserAiStore(w.vm.$pinia)
    store.setCapabilities(browserCapabilities() as never)
    Object.assign(store.state, { phase: 'loading', kind: 'text', source, loadedMB: loaded, totalMB: total, percent: Math.round((loaded / total) * 100) })
    return w
  }
  it('says Downloading with the size for a first load', async () => {
    loading('download', 131, 318)
    await flushPromises()
    expect(pageText()).toContain('Downloading the search model')
    expect(pageText()).toContain('131 of 318 MB')
    expect(pageText()).toContain('This happens once')
  })
  it('says Loading from this device when the files are saved', async () => {
    loading('device', 212, 283)
    await flushPromises()
    expect(pageText()).toContain('Loading the search model from this device')
    expect(pageText()).toContain('Nothing is downloaded')
  })
  it('offers a server search only where users may choose', async () => {
    const w = loading('download', 1, 2)
    await flushPromises()
    button('Search on the Server This Time').click()
    expect(w.emitted('server')).toHaveLength(1)
  })
  it('is hidden when not loading', () => {
    const w = mountWithApp(BrowserModelLoading)
    useBrowserAiStore(w.vm.$pinia).state.phase = 'idle'
    expect(document.body.textContent).not.toContain('search model')
  })
})

describe('Library · in-browser indexing', () => {
  it('shows "Indexing for search in this browser" with the count and the fallback note', async () => {
    const w = mountWithApp(UploadCenter)
    const uploads = useUploadsStore(w.vm.$pinia)
    uploads.indexing = { total: 12, done: 3, failed: 0, current: 'a.jpg' }
    uploads.open = true
    await flushPromises()
    expect(pageText()).toContain('Indexing for search in this browser')
    expect(pageText()).toContain('3 of 12')
    expect(pageText()).toContain('If you close it, the server indexes the rest.')
  })
  it('names photos the server will index instead', async () => {
    const w = mountWithApp(UploadCenter)
    const uploads = useUploadsStore(w.vm.$pinia)
    uploads.indexing = { total: 4, done: 3, failed: 1, current: '' }
    uploads.open = true
    await flushPromises()
    expect(pageText()).toContain('Indexed for search in this browser')
    expect(pageText()).toContain('1 photo will be indexed by the server instead.')
  })
})
