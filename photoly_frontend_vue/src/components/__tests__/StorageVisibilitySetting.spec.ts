import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { mountWithApp, pageText } from '@/test/utils'
import StorageVisibilitySetting from '../StorageVisibilitySetting.vue'

const toggle = () =>
  document.querySelector<HTMLButtonElement>(
    'button[role="switch"][aria-label="Show media from other storage"]',
  )!

beforeEach(() => localStorage.setItem('HRD-Token', 'tok'))

describe('StorageVisibilitySetting', () => {
  it('reflects the saved choice and saves a change', async () => {
    const calls = recordRequests()
    server.use(
      http.get(`${API}/user/getStorage`, () =>
        ok({
          usedBytes: 0,
          capacityBytes: null,
          backend: 'S3-compatible',
          showOtherStorage: false,
        }),
      ),
      http.post(`${API}/user/updateStorageVisibility`, () => json({ msgCode: 200, t: 1 })),
    )
    mountWithApp(StorageVisibilitySetting)
    await vi.waitFor(() => expect(pageText()).toContain('New uploads are saved to S3-compatible'))
    await vi.waitFor(() => expect(toggle().disabled).toBe(false))
    expect(toggle().getAttribute('aria-checked')).toBe('false')

    toggle().click()
    await vi.waitFor(() => expect(toggle().getAttribute('aria-checked')).toBe('true'))
    const post = calls.find((c) => c.path === '/user/updateStorageVisibility')
    expect(post?.query).toEqual({ show: 'true' })
  })
})
