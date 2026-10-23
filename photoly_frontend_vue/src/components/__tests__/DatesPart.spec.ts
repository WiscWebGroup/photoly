import { beforeEach, expect, it, vi } from 'vitest'
import { http, delay } from 'msw'
import { API, ok, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useUploadsStore } from '@/stores/uploads'
import DatesPart from '../DatesPart.vue'
import { viewMode } from '@/composables/useViewMode'

beforeEach(async () => {
  viewMode.value = 'list'
  localStorage.setItem('HRD-Token', 'test-token')
  await router.push('/home/DatesPart')
})

it('pages without changing upload dates, resets on upload completion, and clears on sign-out', async () => {
  const requests: URL[] = []
  server.use(
    http.get(`${API}/photo/getByDate`, ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      const second = !!url.searchParams.get('cursor')
      return ok({
        items: [
          {
            photoId: second ? 2 : 1,
            photoName: second ? 'Second' : 'First',
            format: 'jpg',
            uploadDate: '2026-01-01',
            timelineDate: second ? null : '2020-03-01',
          },
        ],
        nextCursor: second ? null : 'next',
      })
    }),
  )
  mountWithApp(DatesPart)
  await vi.waitFor(() => expect(pageText()).toContain('First.jpg'))
  expect(pageText()).toContain('2020')
  expect(pageText()).not.toContain('2026')
  button('Load more photos').click()
  await vi.waitFor(() => expect(pageText()).toContain('Second.jpg'))
  expect(requests[1]!.searchParams.get('cursor')).toBe('next')
  useUploadsStore().revision++
  await vi.waitFor(() => expect(requests).toHaveLength(3))
  await vi.waitFor(() => expect(pageText()).not.toContain('Second.jpg'))
  useAuthStore().clear()
  await vi.waitFor(() => expect(pageText()).not.toContain('First.jpg'))
})

it('ignores stale responses when sort changes and retains order in the URL', async () => {
  server.use(
    http.get(`${API}/photo/getByDate`, async ({ request }) => {
      const order = new URL(request.url).searchParams.get('order')
      if (order === 'latest') await delay(200)
      return ok({
        items: [{ photoId: 1, photoName: order, format: 'jpg', timelineDate: '2024-01-01' }],
        nextCursor: null,
      })
    }),
  )
  mountWithApp(DatesPart)
  button('Oldest').click()
  await vi.waitFor(() => expect(pageText()).toContain('oldest.jpg'))
  await new Promise((resolve) => setTimeout(resolve, 250))
  expect(pageText()).not.toContain('latest.jpg')
  expect(router.currentRoute.value.query.order).toBe('oldest')
})

it('filters by a date range from the header, keeps it in the URL, and clears it', async () => {
  const requests: URL[] = []
  server.use(
    http.get(`${API}/photo/getDateDays`, () =>
      ok({
        days: [
          { day: '2023-05-01', count: 2 },
          { day: '2024-07-04', count: 1 },
        ],
        undated: 1,
      }),
    ),
    http.get(`${API}/photo/getByDate`, ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      const from = url.searchParams.get('from')
      return ok({
        items: [{ photoId: 1, photoName: from ? `range-${from}` : 'all', format: 'jpg' }],
        nextCursor: null,
      })
    }),
  )
  mountWithApp(DatesPart)
  await vi.waitFor(() => expect(pageText()).toContain('all.jpg'))
  button('Date range: All time').click()
  await vi.waitFor(() => expect(button('2023')).toBeTruthy())
  expect(pageText()).toContain('Show 4 Photos')
  button('2023').click()
  await vi.waitFor(() => expect(pageText()).toContain('Show 2 Photos'))
  button('Show 2 Photos').click()
  await vi.waitFor(() => expect(pageText()).toContain('range-2023-01-01.jpg'))
  expect(requests.at(-1)!.searchParams.get('to')).toBe('2023-12-31')
  expect(router.currentRoute.value.query).toMatchObject({ from: '2023-01-01', to: '2023-12-31' })
  button('Oldest').click()
  await vi.waitFor(() => expect(requests.at(-1)!.searchParams.get('order')).toBe('oldest'))
  expect(requests.at(-1)!.searchParams.get('from')).toBe('2023-01-01')
  document.querySelector<HTMLButtonElement>('button[aria-label^="Date range:"]')!.click()
  await vi.waitFor(() => expect(button('Clear')).toBeTruthy())
  button('Clear').click()
  await vi.waitFor(() => expect(pageText()).toContain('all.jpg'))
  expect(router.currentRoute.value.query.from).toBeUndefined()
})
