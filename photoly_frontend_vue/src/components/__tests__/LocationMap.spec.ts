import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import LocationMap from '../LocationMap.vue'

const mocks = vi.hoisted(() => {
  const map = { setView: vi.fn().mockReturnThis(), remove: vi.fn(), invalidateSize: vi.fn() }
  const marker = { addTo: vi.fn().mockReturnThis(), setLatLng: vi.fn() }
  const tile = { on: vi.fn().mockReturnThis(), addTo: vi.fn().mockReturnThis() }
  return { map, marker, tile, create: vi.fn(() => map), observe: vi.fn(), resize: vi.fn() }
})
vi.mock('leaflet', () => ({
  map: mocks.create,
  marker: vi.fn(() => mocks.marker),
  divIcon: vi.fn(),
  tileLayer: vi.fn(() => mocks.tile),
  control: { attribution: () => ({ addTo: vi.fn() }) },
}))
vi.mock('@vueuse/core', () => ({
  useIntersectionObserver: mocks.observe,
  useResizeObserver: mocks.resize,
}))

function setup(props = {}) {
  const wrapper = mount(LocationMap, {
    props: { latitude: 0, longitude: 0, active: true, ready: true, ...props },
  })
  const intersect = (visible: boolean) =>
    mocks.observe.mock.calls.at(-1)![1]([{ isIntersecting: visible }])
  return { wrapper, intersect }
}
const initialized = () => vi.waitFor(() => expect(mocks.create).toHaveBeenCalledTimes(1))

describe('LocationMap lifecycle', () => {
  beforeEach(() => vi.clearAllMocks())

  it('commits a fold before another photo can replace the component', () => {
    const { wrapper } = setup()
    wrapper
      .get('summary')
      .element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
    expect(wrapper.emitted('update:open')).toEqual([[false]])
  })

  it('waits for visibility, opening completion and an active panel', async () => {
    const { wrapper, intersect } = setup({ ready: false })
    await flushPromises()
    expect(mocks.create).not.toHaveBeenCalled()
    intersect(true)
    await flushPromises()
    expect(mocks.create).not.toHaveBeenCalled()
    await wrapper.setProps({ ready: true, active: false })
    expect(mocks.create).not.toHaveBeenCalled()
    await wrapper.setProps({ active: true })
    await initialized()
    expect(mocks.create).toHaveBeenCalledWith(
      expect.any(HTMLElement),
      expect.objectContaining({ scrollWheelZoom: false }),
    )
    expect(mocks.map.setView).toHaveBeenLastCalledWith([0, 0], 13)
    await wrapper.setProps({ latitude: 42, longitude: -71 })
    expect(mocks.map.setView).toHaveBeenLastCalledWith([42, -71], 13, { animate: false })
    expect(mocks.marker.setLatLng).toHaveBeenLastCalledWith([42, -71])
    await wrapper.setProps({ active: false })
    expect(mocks.map.remove).toHaveBeenCalledTimes(1)
  })

  it('does not initialize while collapsed; releases and recreates on collapse and reopen', async () => {
    const { wrapper, intersect } = setup({ open: false })
    intersect(true)
    await flushPromises()
    expect(mocks.create).not.toHaveBeenCalled()
    await wrapper.setProps({ open: true })
    await initialized()
    await wrapper.setProps({ open: false })
    expect(mocks.map.remove).toHaveBeenCalledTimes(1)
    await wrapper.setProps({ open: true })
    await vi.waitFor(() => expect(mocks.create).toHaveBeenCalledTimes(2))
    wrapper.unmount()
    expect(mocks.map.remove).toHaveBeenCalledTimes(2)
  })

  it('ignores an asynchronous initialization after closing or unmounting', async () => {
    const { wrapper, intersect } = setup()
    intersect(true)
    await nextTick() // import is pending
    wrapper.unmount()
    await flushPromises()
    expect(mocks.create).not.toHaveBeenCalled()
  })

  it('uses the newest coordinates when they change during initialization', async () => {
    const { wrapper, intersect } = setup()
    intersect(true)
    await wrapper.setProps({ latitude: 12, longitude: 34 })
    await initialized()
    expect(mocks.map.setView).toHaveBeenLastCalledWith([12, 34], 13)
  })

  it('reports tile failures and retries without blocking the rest of the viewer', async () => {
    const { wrapper, intersect } = setup()
    intersect(true)
    await initialized()
    const fail = mocks.tile.on.mock.calls.find(([event]) => event === 'tileerror')![1]
    fail()
    await nextTick()
    expect(wrapper.text()).toContain('Could not load the map.')
    await wrapper.get('button').trigger('click')
    await vi.waitFor(() => expect(mocks.create).toHaveBeenCalledTimes(2))
    expect(mocks.map.remove).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('Could not load the map.')
    // Events from the destroyed map cannot affect the new instance.
    fail()
    await nextTick()
    expect(wrapper.text()).not.toContain('Could not load the map.')
  })

  it('offers retry when initialization throws', async () => {
    mocks.create.mockImplementationOnce(() => {
      throw new Error('unavailable')
    })
    const { wrapper, intersect } = setup()
    intersect(true)
    await vi.waitFor(() => expect(wrapper.text()).toContain('Could not load the map.'))
    await wrapper.get('button').trigger('click')
    await vi.waitFor(() => expect(mocks.create).toHaveBeenCalledTimes(2))
    expect(wrapper.text()).not.toContain('Could not load the map.')
  })
})
