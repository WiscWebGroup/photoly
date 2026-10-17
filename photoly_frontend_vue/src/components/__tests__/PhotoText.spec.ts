import { beforeEach, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http, delay } from 'msw'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import PhotoText from '../PhotoText.vue'

const initial = {
  photoId: 10,
  contentVersion: 1,
  revision: 0,
  text: null as string | null,
  languages: 'chi_sim+chi_tra+eng+jpn',
  engineVersion: null,
  updatedAt: null,
}
interface TextApi {
  requestLeave(): Promise<boolean>
  reset(): void
}
function edit(text: string) {
  const area = document.querySelector('textarea')!
  area.value = text
  area.dispatchEvent(new Event('input', { bubbles: true }))
}
async function open() {
  const wrapper = mountWithApp<TextApi>(PhotoText, { photoId: 10 })
  const detail = document.querySelector('details')!
  detail.open = true
  detail.dispatchEvent(new Event('toggle'))
  await vi.waitFor(() => expect(document.querySelector('textarea')).not.toBeNull())
  return wrapper
}
beforeEach(() => {
  server.use(
    http.get(`${API}/photo/ocr`, () => ok(initial)),
    http.post(`${API}/photo/ocr/recognize`, () =>
      ok({ ...initial, text: 'Hello 中文 日本語', revision: 1 }),
    ),
    http.put(`${API}/photo/ocr`, async ({ request }) => {
      const body = (await request.json()) as { text: string }
      return ok({ ...initial, text: body.text, revision: 2 })
    }),
  )
})

it('loads only on expansion and recognizes only after an explicit click', async () => {
  const calls = recordRequests()
  mountWithApp(PhotoText, { photoId: 10 })
  await flushPromises()
  expect(calls).toHaveLength(0)
  const detail = document.querySelector('details')!
  detail.open = true
  detail.dispatchEvent(new Event('toggle'))
  await vi.waitFor(() => expect(document.querySelector('textarea')).not.toBeNull())
  expect(calls.map((c) => c.method)).toEqual(['GET'])
  button('Recognize Text').click()
  await vi.waitFor(() =>
    expect(document.querySelector('textarea')!.value).toBe('Hello 中文 日本語'),
  )
  expect(calls.find((c) => c.method === 'POST')?.body).toEqual({
    photoId: 10,
    contentVersion: 1,
    revision: 0,
  })
})

it('copies the current draft, saves edits and accepts empty text', async () => {
  const copy = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: copy } })
  const wrapper = await open()
  edit('my corrected text')
  await flushPromises()
  button('Copy').click()
  await vi.waitFor(() => expect(copy).toHaveBeenCalledWith('my corrected text'))
  button('Save').click()
  // Save is also disabled while busy; "Unsaved changes" only clears once the PUT has landed.
  await vi.waitFor(() => expect(pageText()).not.toContain('Unsaved changes'))
  expect(await wrapper.vm.requestLeave()).toBe(true)
  edit('')
  await flushPromises()
  button('Save').click()
  await vi.waitFor(() => expect(pageText()).toContain('No text was recognized or saved'))
})

it('asks before recognizing again and keeps edits on recognition failure', async () => {
  server.use(
    http.get(`${API}/photo/ocr`, () => ok({ ...initial, text: 'saved', revision: 1 })),
    http.post(`${API}/photo/ocr/recognize`, () =>
      json({ msgCode: 504, t: { error: 'ocr_timeout' } }),
    ),
  )
  const calls = recordRequests()
  await open()
  edit('draft')
  await flushPromises()
  button('Recognize Again').click()
  await vi.waitFor(() => expect(pageText()).toContain('Recognize this image again?'))
  expect(calls.some((c) => c.method === 'POST')).toBe(false)
  button('Recognize and replace').click()
  await vi.waitFor(() => expect(pageText()).toContain('timed out'))
  expect(document.querySelector('textarea')!.value).toBe('draft')
})

it('guards leaving with cancel, failed save, retry, and discard', async () => {
  const wrapper = await open()
  edit('draft')
  await flushPromises()
  let leaving = wrapper.vm.requestLeave()
  await flushPromises()
  button('Cancel').click()
  expect(await leaving).toBe(false)
  expect(document.querySelector('textarea')!.value).toBe('draft')
  server.use(http.put(`${API}/photo/ocr`, () => json({ msgCode: 503, t: null })))
  leaving = wrapper.vm.requestLeave()
  await flushPromises()
  button('Save and continue').click()
  await vi.waitFor(() => expect(pageText()).toContain('Could not complete'))
  expect(document.querySelector('textarea')!.value).toBe('draft')
  server.use(http.put(`${API}/photo/ocr`, () => ok({ ...initial, text: 'draft', revision: 1 })))
  button('Save and continue').click()
  expect(await leaving).toBe(true)
  edit('discard me')
  await flushPromises()
  leaving = wrapper.vm.requestLeave()
  await flushPromises()
  button('Discard').click()
  expect(await leaving).toBe(true)
})

it('refreshes versions after conflict without discarding the local draft', async () => {
  await open()
  edit('draft')
  await flushPromises()
  server.use(
    http.put(`${API}/photo/ocr`, () => json({ msgCode: 409, t: { error: 'ocr_conflict' } })),
  )
  button('Save').click()
  await vi.waitFor(() => expect(pageText()).toContain('has changed'))
  expect(button('Save').disabled).toBe(true)
  server.use(http.get(`${API}/photo/ocr`, () => ok({ ...initial, text: 'other tab', revision: 3 })))
  button('Refresh saved version').click()
  await vi.waitFor(() => expect(button('Save').disabled).toBe(false))
  expect(document.querySelector('textarea')!.value).toBe('draft')
  const calls = recordRequests()
  server.use(http.put(`${API}/photo/ocr`, () => ok({ ...initial, text: 'draft', revision: 4 })))
  button('Save').click()
  await vi.waitFor(() =>
    expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({
      revision: 3,
      text: 'draft',
    }),
  )
})

it('ignores a late result after reset and reports clipboard failure', async () => {
  const wrapper = await open()
  server.use(
    http.post(`${API}/photo/ocr/recognize`, async () => {
      await delay(100)
      return ok({ ...initial, text: 'late', revision: 1 })
    }),
  )
  button('Recognize Text').click()
  await flushPromises()
  wrapper.vm.reset()
  await new Promise((resolve) => setTimeout(resolve, 150))
  expect(document.querySelector('textarea')).toBeNull()
  button('Retry loading text').click()
  await vi.waitFor(() => expect(document.querySelector('textarea')).not.toBeNull())
  edit('manual')
  await flushPromises()
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn().mockRejectedValue(new Error()) },
  })
  button('Copy').click()
  await vi.waitFor(() => expect(pageText()).toContain('Copy failed'))
})
