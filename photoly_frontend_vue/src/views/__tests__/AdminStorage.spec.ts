import { expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, server } from '@/test/server'
import { mountWithApp, pageText } from '@/test/utils'
import AdminStorage from '../AdminStorage.vue'
it('shows personal storage rather than a global backend switch', async () => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    http.get(`${API}/user/storage/s3`, () =>
      ok({ configured: false, enabled: false, values: {}, fields: [], canSave: true }),
    ),
    http.get(`${API}/user/storage/sources`, () => ok([])),
  )
  mountWithApp(AdminStorage)
  await vi.waitFor(() => expect(pageText()).toContain('Your S3 storage'))
  expect(pageText()).not.toContain('Currently using')
})
