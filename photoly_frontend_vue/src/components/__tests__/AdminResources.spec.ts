import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import AdminResources from '../AdminResources.vue'
import BackendUptime from '../BackendUptime.vue'
import { call } from '@/lib/http'
vi.mock('@/lib/http', () => ({ call: vi.fn() }))
let wrapper: VueWrapper | undefined
const sample = {
  sampledAt: '2026-11-02T12:00:00Z',
  cpuPercent: 42.5,
  backendBytes: 1024 ** 3,
  aiBytes: 512 * 1024 ** 2,
}
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date(sample.sampledAt))
  vi.mocked(call).mockImplementation(async (c) =>
    c.url?.endsWith('history')
      ? { items: [] }
      : c.url?.endsWith('runtime')
        ? { uptimeSeconds: 90061, startedAt: sample.sampledAt }
        : sample,
  )
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.mocked(call).mockReset()
})
describe('admin telemetry', () => {
  it('shows host CPU, total RSS and empty history without inventing values', async () => {
    wrapper = mount(AdminResources)
    await flushPromises()
    expect(wrapper.text()).toContain('42.5%')
    expect(wrapper.text()).toContain('1.50 GB')
    expect(wrapper.text()).toContain('Backend (includes Scheduler)')
    expect(wrapper.text()).toContain('No samples yet')
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '12 hours')!
      .trigger('click')
    await flushPromises()
    expect(call).toHaveBeenCalledWith({ url: '/admin/resources/history', params: { hours: 12 } })
  })
  it('stops requests when hidden and after unmount, and separates current/history cadence', async () => {
    wrapper = mount(AdminResources)
    await flushPromises()
    vi.mocked(call).mockClear()
    await vi.advanceTimersByTimeAsync(30000)
    await flushPromises()
    expect(call).toHaveBeenCalledTimes(1)
    const hidden = vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    await vi.advanceTimersByTimeAsync(600000)
    expect(call).toHaveBeenCalledTimes(1)
    hidden.mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(call).toHaveBeenCalledTimes(3)
    wrapper.unmount()
    wrapper = undefined
    await vi.advanceTimersByTimeAsync(600000)
    expect(call).toHaveBeenCalledTimes(3)
  })
  it('keeps the last sample and reports refresh errors', async () => {
    wrapper = mount(AdminResources)
    await flushPromises()
    vi.mocked(call).mockRejectedValue(new Error('offline'))
    await vi.advanceTimersByTimeAsync(30000)
    await flushPromises()
    expect(wrapper.text()).toContain('last known sample')
    expect(wrapper.text()).toContain('42.5%')
  })
  it('calibrates uptime locally and resets it after a backend restart', async () => {
    wrapper = mount(BackendUptime)
    await flushPromises()
    expect(wrapper.text()).toContain('1d 1h 1m 1s')
    vi.mocked(call).mockResolvedValue({ uptimeSeconds: 4, startedAt: sample.sampledAt })
    await vi.advanceTimersByTimeAsync(30000)
    await flushPromises()
    expect(wrapper.text()).toContain('0d 0h 0m 4s')
    vi.mocked(call).mockRejectedValue(new Error('offline'))
    await vi.advanceTimersByTimeAsync(30000)
    await flushPromises()
    expect(wrapper.text()).toContain('Last known status')
    vi.mocked(call).mockClear()
    await vi.advanceTimersByTimeAsync(10000)
    expect(call).not.toHaveBeenCalled()
  })
})
