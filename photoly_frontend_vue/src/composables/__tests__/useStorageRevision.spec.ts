import { afterEach, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { call } from '@/lib/http'
import { useStorageRevision } from '../useStorageRevision'

vi.mock('@/lib/http', () => ({ call: vi.fn() }))
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('defers changed revisions while selecting and stops polling when unmounted', async () => {
  vi.useFakeTimers()
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  let revision = 1
  let selecting = false
  vi.mocked(call).mockImplementation(async () => [{ sourceId: 1, revision, enabled: true }])
  const refresh = vi.fn()
  const wrapper = mount(
    defineComponent({
      setup() {
        useStorageRevision(refresh, () => selecting)
        return () => null
      },
    }),
  )
  await flushPromises()
  revision = 2
  selecting = true
  await vi.advanceTimersByTimeAsync(15_000)
  expect(refresh).not.toHaveBeenCalled()
  selecting = false
  await vi.advanceTimersByTimeAsync(15_000)
  expect(refresh).toHaveBeenCalledTimes(1)
  await vi.advanceTimersByTimeAsync(15_000)
  expect(refresh).toHaveBeenCalledTimes(1)
  wrapper.unmount()
  revision = 3
  await vi.advanceTimersByTimeAsync(15_000)
  expect(refresh).toHaveBeenCalledTimes(1)
})
