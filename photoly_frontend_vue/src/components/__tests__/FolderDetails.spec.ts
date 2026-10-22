import { beforeEach, expect, it, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { API, ok, server } from '@/test/server'
import { button, byText, mountWithApp, pageText } from '@/test/utils'
import FolderDetails from '../FolderDetails.vue'
import { useAuthStore } from '@/stores/auth'

const details = {
  nsId: 5,
  nsName: 'Trips',
  photoCount: 12,
  videoCount: 2,
  folderCount: 3,
  originalBytes: 1000,
  thumbnailBytes: 200,
  totalBytes: 1200,
  complete: false,
  missingFileCount: 1,
  places: [{ name: 'New York City, United States', count: 4 }],
  cameras: Array.from({ length: 11 }, (_, i) => ({ name: `Camera ${i + 1}`, count: 1 })),
  lenses: [],
}
beforeEach(() => localStorage.setItem('HRD-Token', 'test-token'))

it('shows recursive statistics, partial sizes, metadata pages and missing metadata', async () => {
  server.use(http.get(`${API}/namespace/getDetails`, () => ok(details)))
  mountWithApp(FolderDetails, { folder: { nsId: 5, nsName: 'Trips' } })
  await vi.waitFor(() => expect(pageText()).toContain('New York City'))
  expect(pageText()).toContain('1.2 kB')
  expect(pageText()).toContain('Storage totals are incomplete')
  expect(pageText()).toContain('No information available.')
  expect(() => byText('Camera 11')).toThrow()
  button('Next Cameras').click()
  await vi.waitFor(() => expect(byText('Camera 11')).toBeTruthy())
  expect(() => byText('Camera 10')).toThrow()
  useAuthStore().clear()
  await vi.waitFor(() => expect(pageText()).not.toContain('New York City'))
})

it('retries a failed request without displaying stale totals', async () => {
  let first = true
  server.use(
    http.get(`${API}/namespace/getDetails`, () => {
      if (first) {
        first = false
        return HttpResponse.json({ msgCode: 400, t: -1 })
      }
      return ok({ ...details, complete: true, missingFileCount: 0 })
    }),
  )
  mountWithApp(FolderDetails, { folder: { nsId: 5, nsName: 'Trips' } })
  await vi.waitFor(() => expect(pageText()).toContain('Could not load folder details'))
  button('Retry').click()
  await vi.waitFor(() => expect(pageText()).toContain('New York City'))
  expect(pageText()).not.toContain('Storage totals are incomplete')
})
