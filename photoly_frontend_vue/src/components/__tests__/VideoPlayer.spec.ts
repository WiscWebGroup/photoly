import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import VideoPlayer from '../VideoPlayer.vue'

const mock = vi.hoisted(() => ({ create: vi.fn(), resize: vi.fn() }))
vi.mock('artplayer', () => ({
  default: class {
    constructor(options: unknown) {
      return mock.create(options)
    }
  },
}))
vi.mock('@vueuse/core', () => ({ useResizeObserver: mock.resize }))

function instance() {
  const listeners = new Map<string, () => void>()
  return {
    on: vi.fn((name: string, fn: () => void) => listeners.set(name, fn)),
    fire: (name: string) => listeners.get(name)?.(),
    video: { paused: true, videoWidth: 1920, videoHeight: 1080 },
    pause: vi.fn(),
    play: vi.fn().mockResolvedValue(undefined),
    destroy: vi.fn(),
    duration: 30,
    currentTime: 29,
    seek: 0,
    volume: 0.95,
    fullscreen: false,
    fullscreenWeb: false,
  }
}
function setup() {
  return mount(VideoPlayer, { props: { src: '/video/1', poster: '/poster/1', mediaId: 1 } })
}

describe('VideoPlayer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mock.create.mockImplementation(() => instance())
  })

  it('loads inline without autoplay and reports dimensions and metadata readiness', async () => {
    const wrapper = setup()
    await flushPromises()
    expect(mock.create).toHaveBeenCalledWith(
      expect.objectContaining({
        autoplay: false,
        playsInline: true,
        hotkey: false,
        url: '/video/1',
      }),
    )
    mock.resize.mock.calls[0]![1]([{ contentRect: { width: 800, height: 300 } }])
    const art = mock.create.mock.results[0]!.value
    art.fire('video:loadedmetadata')
    await nextTick()
    expect(wrapper.emitted('ready')).toHaveLength(1)
    expect(wrapper.get('[data-video-player]').attributes('style')).toContain('height: 300px')
    art.video.videoWidth = 1080
    art.video.videoHeight = 1920
    art.fire('video:loadedmetadata')
    await nextTick()
    expect(wrapper.get('[data-video-player]').attributes('style')).toContain('width: 320px')
    wrapper.unmount()
    expect(art.pause).toHaveBeenCalled()
    expect(art.destroy).toHaveBeenCalledOnce()
  })

  it('pauses for close, replaces the source, and ignores callbacks from the old player', async () => {
    const wrapper = setup()
    await flushPromises()
    const old = mock.create.mock.results[0]!.value
    wrapper.vm.pause()
    expect(old.pause).toHaveBeenCalledOnce()
    await wrapper.setProps({ src: '/video/2', mediaId: 2 })
    await flushPromises()
    expect(old.destroy).toHaveBeenCalledOnce()
    old.fire('video:error')
    old.fire('video:loadedmetadata')
    expect(wrapper.emitted('error')).toBeUndefined()
    expect(wrapper.emitted('ready')).toBeUndefined()
    const current = mock.create.mock.results[1]!.value
    current.fire('video:loadedmetadata')
    expect(wrapper.emitted('ready')).toHaveLength(1)
    wrapper.unmount()
    expect(current.destroy).toHaveBeenCalledOnce()
  })

  it('reports one failure and can retry the same URL with a new media identity', async () => {
    const wrapper = setup()
    await flushPromises()
    const old = mock.create.mock.results[0]!.value
    old.fire('video:error')
    old.fire('video:error')
    old.fire('video:loadedmetadata')
    expect(wrapper.emitted('error')).toHaveLength(1)
    expect(wrapper.emitted('ready')).toBeUndefined()
    await wrapper.setProps({ mediaId: '1:retry' })
    await flushPromises()
    mock.create.mock.results[1]!.value.fire('video:loadedmetadata')
    expect(wrapper.emitted('ready')).toHaveLength(1)
    wrapper.unmount()
  })

  it('reports initialization failure and allows retry', async () => {
    mock.create.mockImplementationOnce(() => {
      throw new Error('initialization failed')
    })
    const wrapper = setup()
    await flushPromises()
    expect(wrapper.emitted('error')).toHaveLength(1)
    await wrapper.setProps({ mediaId: 'retry' })
    await flushPromises()
    expect(mock.create).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('does not mount after closing during the dynamic import', async () => {
    const wrapper = setup()
    await nextTick()
    wrapper.unmount()
    await flushPromises()
    expect(mock.create).not.toHaveBeenCalled()
  })

  it('bounds focused playback shortcuts and leaves unrelated document keys alone', async () => {
    const wrapper = setup()
    await flushPromises()
    const art = mock.create.mock.results[0]!.value
    const region = wrapper.get('[data-video-player]')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }))
    expect(art.seek).toBe(0)
    await region.trigger('keydown', { key: 'ArrowRight' })
    expect(art.seek).toBe(30)
    await region.trigger('keydown', { key: 'ArrowUp' })
    expect(art.volume).toBe(1)
    await region.trigger('keydown', { key: ' ' })
    expect(art.play).toHaveBeenCalledOnce()
    art.fullscreen = true
    expect(wrapper.vm.exitFullscreen()).toBe(true)
    expect(art.fullscreen).toBe(false)
    expect(wrapper.vm.exitFullscreen()).toBe(false)
    wrapper.unmount()
  })
})
