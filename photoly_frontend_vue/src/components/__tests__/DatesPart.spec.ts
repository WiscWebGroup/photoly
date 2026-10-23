import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { http, delay } from 'msw'
import { API, ok, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useUploadsStore } from '@/stores/uploads'
import DatesPart from '../DatesPart.vue'
import { viewMode } from '@/composables/useViewMode'

const observers = new Map<Element, IntersectionObserverCallback>()
function visibility(key: string, visible = true) {
  const element = document.querySelector(`[data-date-page="${key}"]`)!
  expect(element).toBeTruthy()
  observers.get(element)?.(
    [{ isIntersecting: visible, target: element } as IntersectionObserverEntry],
    {} as IntersectionObserver,
  )
}
const summary = {
  totalCount: 104,
  undatedCount: 1,
  days: [
    { date: '2020-03-01', count: 1 },
    { date: '2025-06-01', count: 102 },
  ],
}
const requests: URL[] = []
beforeEach(async () => {
  observers.clear()
  requests.length = 0
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(private callback: IntersectionObserverCallback) {}
      target?: Element
      observe(target: Element) {
        this.target = target
        observers.set(target, this.callback)
      }
      disconnect() {
        if (this.target) observers.delete(this.target)
      }
    },
  )
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
  viewMode.value = 'list'
  localStorage.setItem('HRD-Token', 'test-token')
  await router.push('/home/DatesPart')
  server.use(
    http.get(`${API}/photo/getDateSummary`, () => ok(summary)),
    http.get(`${API}/photo/getByDate`, ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      const day = url.searchParams.get('day')!
      const second = !!url.searchParams.get('cursor')
      return ok({
        items: [
          {
            photoId: day === '2020-03-01' ? 3 : second ? 2 : 1,
            photoName: `${day}-${second ? 'second' : 'first'}`,
            format: 'jpg',
            uploadDate: '2026-01-01',
            timelineDate: day === 'undated' ? null : day,
          },
        ],
        nextCursor: day === '2025-06-01' && !second ? 'next' : null,
      })
    }),
  )
})
afterEach(() => vi.unstubAllGlobals())

it('loads only visible pages of expanded months, pages by day, and recycles offscreen cards', async () => {
  mountWithApp(DatesPart)
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2025-06-01:0"]')).toBeTruthy(),
  )
  expect(requests).toHaveLength(0)
  expect(document.querySelector('[data-date-page="2020-03-01:0"]')).toBeNull()
  visibility('2025-06-01:0')
  await vi.waitFor(() => expect(pageText()).toContain('2025-06-01-first.jpg'))
  expect(pageText()).not.toContain('2026')
  visibility('2025-06-01:1')
  await vi.waitFor(() => expect(pageText()).toContain('2025-06-01-second.jpg'))
  expect(requests[1]!.searchParams.get('cursor')).toBe('next')
  expect(requests[1]!.searchParams.get('day')).toBe('2025-06-01')
  visibility('2025-06-01:0', false)
  await vi.waitFor(() => expect(document.querySelector('[data-photo-id="1"]')).toBeNull())
  visibility('2025-06-01:0')
  await vi.waitFor(() => expect(document.querySelector('[data-photo-id="1"]')).toBeTruthy())
  expect(requests).toHaveLength(2)
  button('March 2020, 1 photo').click()
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2020-03-01:0"]')).toBeTruthy(),
  )
  expect(document.querySelector('[data-date-page="2025-06-01:0"]')).toBeTruthy()
})

it('jumps straight to a calendar day, remembers hiding, refreshes on uploads and clears on sign-out', async () => {
  mountWithApp(DatesPart)
  await vi.waitFor(() => expect(pageText()).toContain('Photo calendar'))
  const year = document.querySelector<HTMLSelectElement>('[aria-label="Calendar year"]')!
  year.value = '2020'
  year.dispatchEvent(new Event('change'))
  const month = document.querySelector<HTMLSelectElement>('[aria-label="Calendar month"]')!
  month.value = '03'
  month.dispatchEvent(new Event('change'))
  await vi.waitFor(() => expect(button('2020-03-01, 1 photo')).toBeTruthy())
  button('2020-03-01, 1 photo').click()
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2020-03-01:0"]')).toBeTruthy(),
  )
  visibility('2020-03-01:0')
  await vi.waitFor(() => expect(pageText()).toContain('2020-03-01-first.jpg'))
  expect(requests.map((r) => r.searchParams.get('day'))).toEqual(['2020-03-01'])
  button('Hide photo calendar').click()
  await vi.waitFor(() =>
    expect(localStorage.getItem('photoly-dates-calendar-visible')).toBe('false'),
  )
  button('Show photo calendar').click()
  useUploadsStore().revision++
  await vi.waitFor(() => expect(pageText()).not.toContain('2020-03-01-first.jpg'))
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2020-03-01:0"]')).toBeTruthy(),
  )
  visibility('2020-03-01:0')
  await vi.waitFor(() => expect(requests).toHaveLength(2))
  useAuthStore().clear()
  await vi.waitFor(() => expect(pageText()).not.toContain('2020-03-01-first.jpg'))
  expect(document.querySelector('[aria-label="Photo calendar"]')).toBeNull()
})

it('ignores stale responses when sort changes and cancels collapsed months', async () => {
  server.use(
    http.get(`${API}/photo/getByDate`, async ({ request }) => {
      const url = new URL(request.url)
      requests.push(url)
      if (url.searchParams.get('order') === 'latest') await delay(200)
      return ok({
        items: [
          {
            photoId: 1,
            photoName: url.searchParams.get('order'),
            format: 'jpg',
            timelineDate: '2020-03-01',
          },
        ],
        nextCursor: null,
      })
    }),
  )
  mountWithApp(DatesPart)
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2025-06-01:0"]')).toBeTruthy(),
  )
  visibility('2025-06-01:0')
  await vi.waitFor(() => expect(requests).toHaveLength(1))
  button('Oldest').click()
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2020-03-01:0"]')).toBeTruthy(),
  )
  visibility('2020-03-01:0')
  await vi.waitFor(() => expect(pageText()).toContain('oldest.jpg'))
  await delay(250)
  expect(pageText()).not.toContain('latest.jpg')
  expect(router.currentRoute.value.query.order).toBe('oldest')
  button('March 2020, 1 photo').click()
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2020-03-01:0"]')).toBeNull(),
  )
})

it('shows empty library and retries failed day requests locally', async () => {
  let failed = true
  server.use(
    http.get(`${API}/photo/getByDate`, () => {
      if (failed) {
        failed = false
        return ok(null)
      }
      return ok({
        items: [{ photoId: 9, photoName: 'Recovered', format: 'jpg' }],
        nextCursor: null,
      })
    }),
  )
  mountWithApp(DatesPart)
  await vi.waitFor(() =>
    expect(document.querySelector('[data-date-page="2025-06-01:0"]')).toBeTruthy(),
  )
  visibility('2025-06-01:0')
  await vi.waitFor(() => expect(pageText()).toContain('Could not load photos.'))
  button('Retry photos').click()
  await vi.waitFor(() => expect(pageText()).toContain('Recovered.jpg'))
  server.use(
    http.get(`${API}/photo/getDateSummary`, () => ok({ totalCount: 0, undatedCount: 0, days: [] })),
  )
  useUploadsStore().revision++
  await vi.waitFor(() => expect(pageText()).toContain('No photos yet'))
})

it('loads at most ten thumbnail IDs only for visible month cards without expanding the month', async () => {
  const previews: URL[] = []
  server.use(
    http.get(`${API}/photo/getMonthPreview`, ({ request }) => {
      previews.push(new URL(request.url))
      return ok([40])
    }),
  )
  mountWithApp(DatesPart)
  await vi.waitFor(() =>
    expect(document.querySelectorAll('[data-testid="month-preview"]')).toHaveLength(2),
  )
  expect(previews).toHaveLength(0)
  const card = document.querySelectorAll('[data-testid="month-preview"]')[1]!
  const show = (visible: boolean) =>
    observers.get(card)?.(
      [{ isIntersecting: visible, target: card } as IntersectionObserverEntry],
      {} as IntersectionObserver,
    )
  show(true)
  await vi.waitFor(() => expect(card.querySelectorAll('img')).toHaveLength(1))
  expect(previews[0]!.searchParams.get('month')).toBe('2020-03')
  expect(button('March 2020, 1 photo').getAttribute('aria-expanded')).toBe('false')
  expect(requests).toHaveLength(0)
  show(false)
  await Promise.resolve()
  show(true)
  await Promise.resolve()
  expect(previews).toHaveLength(1)
})
