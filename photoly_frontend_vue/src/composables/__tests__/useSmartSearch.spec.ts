import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { useSmartSearch } from '../useSmartSearch'
import { useAuthStore } from '@/stores/auth'
import { call } from '@/lib/http'
import { aiCall } from '@/lib/ai'
import { capabilitiesFixture } from '@/test/aiFixtures'

vi.mock('@/lib/http', () => ({ call: vi.fn(), http: {} }))
vi.mock('@/lib/ai', () => ({ aiCall: vi.fn() }))
const one = { photoId: 1, photoName: 'one' },
  two = { photoId: 2, photoName: 'two' }
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((r) => {
    resolve = r
  })
  return { promise, resolve }
}
function setup() {
  let search!: ReturnType<typeof useSmartSearch>
  const pinia = createPinia()
  const wrapper = mount(
    defineComponent({
      setup() {
        search = useSmartSearch()
        return () => h('div')
      },
    }),
    { global: { plugins: [pinia] } },
  )
  return { search, wrapper, auth: useAuthStore(pinia) }
}

describe('Smart Search requests', () => {
  beforeEach(() => {
    vi.mocked(call).mockReset().mockResolvedValue([one])
    vi.mocked(aiCall)
      .mockReset()
      .mockImplementation(async (options) =>
        options.url === '/ai/capabilities' ? capabilitiesFixture : { items: [{ photo: two }] },
      )
  })
  it('shows text matches before semantic completion, then merges', async () => {
    const pending = deferred<unknown>()
    vi.mocked(aiCall).mockImplementation((options) =>
      options.url === '/ai/capabilities'
        ? Promise.resolve(capabilitiesFixture)
        : (pending.promise as never),
    )
    const { search } = setup()
    const complete = search.search('  cats  ')
    await flushPromises()
    expect(search.results.value).toEqual([one])
    expect(search.busy.value).toBe(true)
    expect(vi.mocked(call).mock.calls[0]![0].params).toEqual({ query: 'cats' })
    pending.resolve({ items: [{ photo: two }, { photo: one }] })
    await complete
    expect(search.results.value).toEqual([one, two])
    expect(search.busy.value).toBe(false)
  })
  it('keeps metadata when AI is disabled or fails', async () => {
    vi.mocked(aiCall).mockResolvedValueOnce({ ...capabilitiesFixture, ready: false })
    const { search } = setup()
    await search.search('cat')
    expect(aiCall).toHaveBeenCalledTimes(1)
    expect(search.results.value).toEqual([one])
    expect(search.semanticNotice.value).toContain('unavailable')
    vi.mocked(aiCall).mockRejectedValueOnce(new Error('offline'))
    await search.search('dog')
    expect(search.results.value).toEqual([one])
    expect(search.semanticNotice.value).toContain('could not finish')
  })
  it('keeps semantic results when text search fails', async () => {
    vi.mocked(call).mockRejectedValueOnce(new Error('offline'))
    const { search } = setup()
    await search.search('cat')
    expect(search.results.value).toEqual([two])
    expect(search.textNotice.value).toContain('unavailable')
  })
  it('cancels older searches and ignores late responses', async () => {
    const pending = deferred<unknown>()
    vi.mocked(call).mockReturnValueOnce(pending.promise as never)
    const { search } = setup()
    const old = search.search('old')
    const signal = vi.mocked(call).mock.calls[0]![0].signal as AbortSignal
    await search.search('new')
    expect(signal.aborted).toBe(true)
    pending.resolve([{ photoId: 99, photoName: 'stale' }])
    await old
    expect(search.results.value.map((p) => p.photoId)).toEqual([1, 2])
  })
  it('clears on account change and unmount; empty and oversized inputs make no requests', async () => {
    const { search, auth, wrapper } = setup()
    await search.search('   ')
    await search.search('x'.repeat(513))
    expect(call).not.toHaveBeenCalled()
    expect(aiCall).not.toHaveBeenCalled()
    expect(search.error.value).toContain('512')
    await search.search('cat')
    auth.setToken('another-user')
    await flushPromises()
    expect(search.results.value).toEqual([])
    await search.search('again')
    const signal = vi.mocked(call).mock.calls.at(-1)![0].signal as AbortSignal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    expect(search.results.value).toEqual([])
  })
})
