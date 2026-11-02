import { nextTick } from 'vue'
import { describe, it, expect, vi } from 'vitest'
import { http } from 'msw'
import { API, server, ok, json, recordRequests } from '@/test/server'
import { mountWithApp, button, pageText, type } from '@/test/utils'
import MediaNotes from '../MediaNotes.vue'
const saved = {
  id: 1,
  body: 'First\nSecond',
  x: 0.2,
  y: 0.7,
  contentVersion: 1,
  revision: 2,
  createdAt: '2026-01-01',
  updatedAt: '2026-01-01',
  needsReposition: false,
}
describe('MediaNotes', () => {
  it('saves multiline plain text with normalized photo coordinates', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/photo/notes`, () => ok({ id: 3 })))
    const wrapper = mountWithApp(MediaNotes, { photoId: 10, canPin: true })
    await vi.waitFor(() => expect(pageText()).toContain('Notes0'))
    button('Notes0').click()
    button('Pin on photo').click()
    await vi.waitFor(() =>
      expect((wrapper.vm as unknown as InstanceType<typeof MediaNotes>).placing).toBe(true),
    )
    ;(wrapper.vm as unknown as InstanceType<typeof MediaNotes>).position(0.25, 0.75)
    await vi.waitFor(() =>
      expect((wrapper.vm as unknown as InstanceType<typeof MediaNotes>).placing).toBe(false),
    )
    type(
      document.querySelector('textarea')! as unknown as HTMLInputElement,
      '<b>Memory</b>\n第二段',
    )
    await nextTick()
    button('Save').click()
    await vi.waitFor(() =>
      expect(calls.find((c) => c.path === '/photo/notes' && c.method === 'POST')?.body).toEqual({
        photoId: 10,
        body: '<b>Memory</b>\n第二段',
        x: 0.25,
        y: 0.75,
        contentVersion: 1,
      }),
    )
  })
  it('hides stale pins, preserves drafts on conflicts and guards departure', async () => {
    server.use(
      http.get(`${API}/photo/notes`, () =>
        ok({ items: [{ ...saved, needsReposition: true }], contentVersion: 2 }),
      ),
      http.patch(`${API}/photo/notes/1`, () => json({ msgCode: 409, t: 'note_conflict' })),
    )
    const wrapper = mountWithApp(MediaNotes, { photoId: 10, canPin: true })
    await vi.waitFor(() => expect(pageText()).toContain('Notes1'))
    button('Notes1').click()
    expect((wrapper.vm as unknown as InstanceType<typeof MediaNotes>).pins).toEqual([])
    button('Edit').click()
    await vi.waitFor(() => expect(document.querySelector('textarea')).toBeTruthy())
    type(document.querySelector('textarea')! as unknown as HTMLInputElement, 'Unsaved memory')
    await nextTick()
    button('Save').click()
    await vi.waitFor(() => expect(pageText()).toContain('Your draft is kept'))
    expect(document.querySelector<HTMLTextAreaElement>('textarea')!.value).toBe('Unsaved memory')
    const leaving = (wrapper.vm as unknown as InstanceType<typeof MediaNotes>).requestLeave()
    await vi.waitFor(() => expect(pageText()).toContain('Discard unsaved note?'))
    button('Keep editing').click()
    expect(await leaving).toBe(false)
    const discard = (wrapper.vm as unknown as InstanceType<typeof MediaNotes>).requestLeave()
    await vi.waitFor(() => expect(pageText()).toContain('Discard unsaved note?'))
    button('Discard changes').click()
    expect(await discard).toBe(true)
  })
})
