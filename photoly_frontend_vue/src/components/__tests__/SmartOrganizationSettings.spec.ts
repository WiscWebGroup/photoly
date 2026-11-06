import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import { API, ok, server, recordRequests, json } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import SmartOrganizationSettings from '../SmartOrganizationSettings.vue'
import SmartBadge from '../SmartBadge.vue'
import { useAuthStore } from '@/stores/auth'

const item = {
  id: 8,
  name: 'iPhone 13',
  source: 'camera' as const,
  suppressed: true,
  basis: { method: 'EXIF device model' },
}
function mock(enabled = false, available = true) {
  let removed = false
  let devices = enabled
  let trips = enabled
  server.use(
    http.get(`${API}/ai/smart/settings`, () =>
      ok({
        enabled: devices || trips,
        devices,
        trips,
        available,
        reason: available ? null : 'ai_permission_required',
      }),
    ),
    http.patch(`${API}/ai/smart/settings`, async ({ request }) => {
      const body = (await request.json()) as { devices?: boolean; trips?: boolean }
      devices = body.devices ?? devices
      trips = body.trips ?? trips
      return ok({ enabled: devices || trips, devices, trips, available })
    }),
    http.get(`${API}/ai/smart/status`, () =>
      ok({
        processed: 25,
        total: 50,
        failed: 2,
        reason: 'waiting_for_vectors',
        pending: true,
        phase: 'metadata',
      }),
    ),
    http.get(`${API}/ai/smart/dismissed`, () =>
      ok({ items: removed ? [] : [item], nextAfter: null }),
    ),
    http.post(`${API}/ai/smart/items/8/restore`, () => {
      removed = true
      return ok({ restored: true })
    }),
    http.post(`${API}/ai/smart/refresh`, () => ok({ queued: true })),
  )
}

describe('Smart organization settings', () => {
  it('defaults off, persists opt-in, shows progress, refreshes and restores', async () => {
    mock()
    const calls = recordRequests()
    const wrapper = mountWithApp(SmartOrganizationSettings)
    await vi.waitFor(() => expect(pageText()).toContain('25 / 50'))
    const toggle = wrapper.get('input[type=checkbox]')
    expect((toggle.element as HTMLInputElement).checked).toBe(false)
    await toggle.setValue(true)
    await vi.waitFor(() => expect(calls.some((c) => c.method === 'PATCH')).toBe(true))
    await vi.waitFor(() => expect(button('Refresh and retry').disabled).toBe(false))
    button('Refresh and retry').click()
    await vi.waitFor(() => expect(calls.some((c) => c.path.endsWith('/refresh'))).toBe(true))
    await vi.waitFor(() => expect(button('Restore').disabled).toBe(false))
    button('Restore').click()
    await vi.waitFor(() => expect(pageText()).toContain('No deleted smart items.'))
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ devices: true })
    expect((wrapper.findAll('input[type=checkbox]')[1].element as HTMLInputElement).checked).toBe(
      false,
    )
    expect(pageText()).toContain('2 failed')
  })

  it('explains missing permissions and permits an opted-in user to switch off', async () => {
    mock(true, false)
    const wrapper = mountWithApp(SmartOrganizationSettings)
    await vi.waitFor(() => expect(pageText()).toContain('AI access from an administrator'))
    expect((wrapper.get('input').element as HTMLInputElement).disabled).toBe(false)
    expect(button('Refresh and retry').disabled).toBe(true)
    await wrapper.get('input').setValue(false)
    await vi.waitFor(() =>
      expect((wrapper.get('input').element as HTMLInputElement).disabled).toBe(true),
    )
  })

  it('keeps the saved toggle on failures and allows retry', async () => {
    mock()
    server.use(http.patch(`${API}/ai/smart/settings`, () => json({ msgCode: 503 })))
    const wrapper = mountWithApp(SmartOrganizationSettings)
    await vi.waitFor(() => expect(pageText()).toContain('25 / 50'))
    await wrapper.get('input').setValue(true)
    await vi.waitFor(() => expect(pageText()).toContain('Could not save Smart Organization changes'))
    expect((wrapper.get('input').element as HTMLInputElement).checked).toBe(false)
  })

  it('aborts and clears private state on logout, ignoring late results', async () => {
    localStorage.setItem('HRD-Token', 'alice')
    mock()
    const wrapper = mountWithApp(SmartOrganizationSettings)
    await vi.waitFor(() => expect(pageText()).toContain('iPhone 13'))
    useAuthStore().clear()
    await flushPromises()
    expect(wrapper.text()).not.toContain('iPhone 13')
    expect((wrapper.get('input').element as HTMLInputElement).disabled).toBe(true)
  })

  it('labels the source without relying on color', () => {
    const wrapper = mountWithApp(SmartBadge, { smart: item })
    expect(wrapper.text()).toContain('Camera')
    expect(wrapper.text()).toContain('Smart')
    expect(wrapper.find('svg').exists()).toBe(true)
  })
})

it('rejects malformed settings responses without breaking the account page', async () => {
  mock()
  server.use(http.get(`${API}/ai/smart/dismissed`, () => ok([])))
  const wrapper = mountWithApp(SmartOrganizationSettings)
  await vi.waitFor(() => expect(pageText()).toContain('Could not load Smart Organization settings'))
  expect(wrapper.text()).toContain('No deleted smart items.')
})
