import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { aiCall } from '@/lib/ai'
import { capabilitiesFixture } from '@/test/aiFixtures'
import { button, mountWithApp } from '@/test/utils'
import SimilarRecommendations, { type RecommendationSnapshot } from '../SimilarRecommendations.vue'

vi.mock('@/lib/ai', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/ai')>()),
  aiCall: vi.fn(),
}))
const page = {
  items: [{ photo: { photoId: 20, photoName: 'related', format: 'jpg', nsId: 1 }, score: 0.9 }],
  nextCursor: 'next',
  hasMore: true,
  profileId: 1,
  coverage: capabilitiesFixture.coverage,
  requestId: 'test',
  similarityThreshold: null,
}
interface RecommendationsApi {
  capture(): RecommendationSnapshot
  collapse(): void
  expanded: boolean
}
const query = vi.fn()
async function mountRecommendations(resume?: RecommendationSnapshot, format = 'jpg') {
  const root = document.createElement('div')
  document.body.appendChild(root)
  const wrapper = mountWithApp<RecommendationsApi>(SimilarRecommendations, {
    photo: { photoId: 10, photoName: 'original', format, nsId: 1 },
    scrollRoot: root,
    resume,
  })
  await flushPromises()
  return { root, wrapper }
}
function scroll(root: HTMLElement, top: number, user = false) {
  if (user) root.dispatchEvent(new Event('touchmove'))
  root.scrollTop = top
  root.dispatchEvent(new Event('scroll'))
}

describe('SimilarRecommendations reset', () => {
  beforeEach(() => {
    query.mockReset().mockResolvedValue(page)
    vi.mocked(aiCall).mockImplementation((options) =>
      options.url === '/ai/capabilities' ? Promise.resolve(capabilitiesFixture) : query(options),
    )
  })

  it('supports video sources and renders videos with the existing media icon', async () => {
    vi.mocked(aiCall).mockImplementation((options) =>
      options.url === '/ai/capabilities'
        ? Promise.resolve({
            ...capabilitiesFixture,
            formats: [...capabilitiesFixture.formats, 'mp4'],
          })
        : query(options),
    )
    query.mockResolvedValue({
      ...page,
      items: [
        ...page.items,
        {
          photo: { photoId: 21, photoName: 'clip', format: 'mp4', nsId: 1 },
          score: 0.8,
          matchedTimestampMs: 2500,
        },
      ],
    })
    const { root, wrapper } = await mountRecommendations(undefined, 'mp4')
    expect(query).not.toHaveBeenCalled()
    scroll(root, 100, true)
    await flushPromises()
    expect(wrapper.vm.capture().result?.items).toHaveLength(2)
    const video = wrapper.find('button[aria-label="View clip.mp4"] img')
    expect(video.attributes('src')).toContain('Video.png')
    wrapper.vm.collapse()
    expect(wrapper.vm.capture().result).toBeNull()
  })

  it('requires fresh downward scroll after collapse and resets the previous trigger position', async () => {
    const { root, wrapper } = await mountRecommendations()
    scroll(root, 100)
    await flushPromises()
    expect(query).not.toHaveBeenCalled()
    scroll(root, 200, true)
    await flushPromises()
    expect(query).toHaveBeenCalledTimes(1)
    expect(wrapper.vm.expanded).toBe(true)
    wrapper.vm.collapse()
    scroll(root, 0)
    scroll(root, 100)
    await flushPromises()
    expect(query).toHaveBeenCalledTimes(1)
    expect(wrapper.vm.capture().result).toBeNull()
    scroll(root, 120, true)
    await flushPromises()
    expect(query).toHaveBeenCalledTimes(2)
    expect(query.mock.calls[1]![0].data).toEqual({ pageSize: 24 })
  })

  it('aborts an in-flight request and ignores its late result', async () => {
    let finish!: (value: typeof page) => void
    query.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve
        }),
    )
    const { wrapper } = await mountRecommendations()
    button('Load more').click()
    await flushPromises()
    expect(wrapper.vm.expanded).toBe(true)
    const signal = query.mock.calls[0]![0].signal as AbortSignal
    wrapper.vm.collapse()
    expect(signal.aborted).toBe(true)
    finish(page)
    await flushPromises()
    expect(wrapper.vm.expanded).toBe(false)
    expect(wrapper.vm.capture().result).toBeNull()
    button('Load more').click()
    await flushPromises()
    expect(wrapper.vm.capture().result?.items).toHaveLength(1)
  })

  it('clears errors as well as results when collapsed', async () => {
    query.mockRejectedValueOnce(new Error('request failed'))
    const { wrapper } = await mountRecommendations()
    button('Load more').click()
    await flushPromises()
    expect(document.querySelector('[role="alert"]')).not.toBeNull()
    wrapper.vm.collapse()
    await flushPromises()
    expect(document.querySelector('[role="alert"]')).toBeNull()
    expect(wrapper.vm.expanded).toBe(false)
    button('Load more').click()
    await flushPromises()
    expect(wrapper.vm.capture().result?.items).toHaveLength(1)
  })

  it('restores valid snapshots but rejects expired pagination state', async () => {
    const snapshot = { result: page, cursor: 'next', hasMore: true, expiresAt: Date.now() + 10000 }
    const { wrapper } = await mountRecommendations(snapshot)
    expect(wrapper.vm.capture().result?.items).toHaveLength(1)
    expect(query).not.toHaveBeenCalled()
    const expired = await mountRecommendations({ ...snapshot, expiresAt: Date.now() - 1 })
    expect(expired.wrapper.vm.capture().result).toBeNull()
    expect(expired.wrapper.vm.expanded).toBe(false)
  })
})
