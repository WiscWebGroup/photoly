import { defineComponent, h } from 'vue'
import { expect, it, vi } from 'vitest'
import { http, delay } from 'msw'
import { API, ok, server } from '@/test/server'
import { mountWithApp } from '@/test/utils'
import { useDateTimeline } from '../useDateTimeline'

function setup() {
  localStorage.setItem('HRD-Token', 'test-token')
  let timeline!: ReturnType<typeof useDateTimeline>
  const wrapper = mountWithApp(
    defineComponent({
      setup() {
        timeline = useDateTimeline(() => 'latest')
        return () => h('div')
      },
    }),
  )
  return { timeline, wrapper }
}

it('bounds concurrency, deduplicates requests and discards a cancelled response', async () => {
  let running = 0,
    max = 0,
    calls = 0
  server.use(
    http.get(`${API}/photo/getByDate`, async ({ request }) => {
      calls++
      running++
      max = Math.max(max, running)
      await delay(40)
      running--
      return ok({
        items: [
          {
            photoId: Number(new URL(request.url).searchParams.get('day')!.slice(-2)),
            photoName: 'photo',
          },
        ],
        nextCursor: null,
      })
    }),
  )
  const { timeline } = setup()
  const pages = Array.from(
    { length: 8 },
    (_, i) => timeline.pagesFor(`2025-06-${String(i + 1).padStart(2, '0')}`)[0]!,
  )
  for (const page of pages) {
    timeline.setVisible(page, true)
    timeline.setVisible(page, true)
  }
  await vi.waitFor(() => expect(calls).toBe(3))
  timeline.cancelDay(pages[7]!.day)
  await vi.waitFor(() => expect(timeline.photos.value).toHaveLength(7))
  expect(max).toBe(3)
  expect(calls).toBe(7)
  expect(pages[7]!.items).toBeUndefined()
})

it('keeps at most 20 ordinary pages and reloads evicted pages with their original cursor', async () => {
  const cursors: (string | null)[] = []
  server.use(
    http.get(`${API}/photo/getByDate`, ({ request }) => {
      const cursor = new URL(request.url).searchParams.get('cursor')
      cursors.push(cursor)
      const index = cursor ? Number(cursor) : 0
      return ok({
        items: [{ photoId: index + 1, photoName: `page-${index}` }],
        nextCursor: index < 24 ? String(index + 1) : null,
      })
    }),
  )
  const { timeline } = setup()
  const pages = timeline.pagesFor('2025-06-01')
  for (let i = 0; i < 25; i++) {
    const page = pages[i]!
    timeline.setVisible(page, true)
    await vi.waitFor(() => expect(page.items).toHaveLength(1))
    timeline.setVisible(page, false)
  }
  expect(timeline.photos.value).toHaveLength(20)
  expect(pages).toHaveLength(25)
  expect(pages[0]!.items).toBeUndefined()
  expect(pages[2]!.cursor).toBe('2')
  timeline.setVisible(pages[2]!, true)
  await vi.waitFor(() => expect(pages[2]!.items?.[0]?.photoId).toBe(3))
  expect(cursors.at(-1)).toBe('2')
  expect(timeline.photos.value).toHaveLength(20)
})

it('aborts a collapsed day and can reopen it while the cancelled request settles', async () => {
  let calls = 0
  server.use(
    http.get(`${API}/photo/getByDate`, async () => {
      const current = ++calls
      await delay(current === 1 ? 100 : 5)
      return ok({
        items: [{ photoId: current, photoName: `request-${current}` }],
        nextCursor: null,
      })
    }),
  )
  const { timeline } = setup()
  const page = timeline.pagesFor('2025-06-01')[0]!
  timeline.setVisible(page, true)
  await vi.waitFor(() => expect(calls).toBe(1))
  timeline.cancelDay(page.day)
  timeline.setVisible(page, true)
  await vi.waitFor(() => expect(page.items?.[0]?.photoId).toBe(2))
  await delay(120)
  expect(page.items?.[0]?.photoId).toBe(2)
  expect(page.error).toBe(false)
})
