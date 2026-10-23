import { it, expect, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, server, recordRequests } from '@/test/server'
import { mountWithApp, button, pageText } from '@/test/utils'
import MediaTaskDialog from '../MediaTaskDialog.vue'
import OnlineDownloadButton from '../OnlineDownloadButton.vue'
import { useAuthStore } from '@/stores/auth'

it('uses the original photo folder and warns about animation first frames', async () => {
  server.use(
    http.get(`${API}/photo/mediaInfo`, () => ok({ nsId: 42, animated: true, live: false })),
    http.get(`${API}/namespace/trace`, () =>
      ok([
        { nsId: 1, nsName: '/' },
        { nsId: 42, nsName: 'Original folder' },
      ]),
    ),
    http.post(`${API}/photo/convert`, () => ok('batch')),
  )
  const calls = recordRequests()
  mountWithApp(MediaTaskDialog, {
    open: true,
    photo: { photoId: 7, photoName: 'Animated', format: 'gif', nsId: 9 },
  })
  await vi.waitFor(() => expect(pageText()).toContain('Original folder'))
  expect(pageText()).toContain('first animation frame')
  button('Save copy').click()
  await vi.waitFor(() => expect(calls.some((c) => c.path === '/photo/convert')).toBe(true))
  const request = calls.find((c) => c.path === '/photo/convert')!
  expect(request.body).toMatchObject({ photoId: 7, format: 'jpeg' })
  expect(request.body).not.toHaveProperty('nsId')
})

it('only offers Online Download when enabled and hides it after logout', async () => {
  server.use(http.get(`${API}/photo/mediaCapabilities`, () => ok({ onlineDownload: true })))
  useAuthStore().setToken('test-token')
  mountWithApp(OnlineDownloadButton)
  await vi.waitFor(() => expect(button('Online Download')).toBeTruthy())
  useAuthStore().clear()
  await vi.waitFor(() =>
    expect(document.querySelector('[aria-label="Online Download"]')).toBeNull(),
  )
})
