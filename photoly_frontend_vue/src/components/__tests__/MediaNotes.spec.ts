import { nextTick } from 'vue'
import { beforeEach, describe, it, expect, vi } from 'vitest'
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
  beforeEach(() => {
    HTMLElement.prototype.scrollIntoView = vi.fn()
    vi.stubGlobal(
      'matchMedia',
      vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })),
    )
  })
  it('saves multiline plain text with normalized photo coordinates', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/photo/notes`, () => ok({ id: 3 })))
    const wrapper = mountWithApp(MediaNotes, { photoId: 10, canPin: true })
    await vi.waitFor(() => expect(wrapper.find('button').text()).toBe('Notes0'))
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
    button('See all Pin-point notes (1)').click()
    await nextTick()
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
  it('separates notes, reveals one without all bubbles, and resets on collapse', async () => {
    server.use(
      http.get(`${API}/photo/notes`, () =>
        ok({
          items: [
            { ...saved, id: 10, x: null, y: null, body: 'General memory' },
            { ...saved, id: 11, body: 'Pinned memory' },
            { ...saved, id: 12, needsReposition: true, body: 'Old point' },
          ],
          contentVersion: 1,
        }),
      ),
    )
    const wrapper = mountWithApp(MediaNotes, { photoId: 10, canPin: true })
    const api = wrapper.vm as unknown as InstanceType<typeof MediaNotes>
    await vi.waitFor(() => expect(pageText()).toContain('Notes3'))
    button('Notes3').click()
    await nextTick()
    expect(document.querySelector('[aria-label="General Notes"]')?.textContent).toContain(
      'General memory',
    )
    expect(document.querySelector('[aria-label="Pin-point Notes"]')).toBeNull()
    expect(api.pins.map((p) => p.number)).toEqual([1])
    expect(api.showAllBubbles).toBe(false)
    await api.select(11)
    expect(document.querySelector('[aria-label="Pin-point Notes"]')?.textContent).toContain(
      'Pinned memory',
    )
    expect(api.showAllBubbles).toBe(false)
    expect(api.selectedId).toBe(11)
    expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalled()
    button('See all Pin-point notes (2)').click()
    await nextTick()
    expect(api.showAllBubbles).toBe(true)
    expect(pageText()).toContain('Point needs repositioning')
    button('Hide Pin-point notes (2)').click()
    await nextTick()
    expect(document.querySelector('[aria-label="Pin-point Notes"]')).toBeNull()
    button('See all Pin-point notes (2)').click()
    await nextTick()
    button('Notes3').click()
    await nextTick()
    expect(api.showAllBubbles).toBe(false)
    expect(api.selectedId).toBe(0)
    expect(api.pins).toEqual([])
  })
  it('preserves surviving pin colors and renumbers after deletion', async () => {
    let notes = [
      { ...saved, id: 1 },
      { ...saved, id: 2, body: 'Second point' },
    ]
    server.use(
      http.get(`${API}/photo/notes`, () => ok({ items: notes, contentVersion: 1 })),
      http.delete(`${API}/photo/notes/1`, () => {
        notes = notes.slice(1)
        return ok({ id: 1 })
      }),
    )
    const wrapper = mountWithApp(MediaNotes, { photoId: 10, canPin: true })
    const api = wrapper.vm as unknown as InstanceType<typeof MediaNotes>
    await vi.waitFor(() => expect(pageText()).toContain('Notes2'))
    button('Notes2').click()
    await nextTick()
    const color = api.pins[1]!.color
    await api.select(1)
    button('Delete', document.getElementById('media-note-1')!).click()
    await nextTick()
    button('Delete note').click()
    await vi.waitFor(() => expect(api.pins).toHaveLength(1))
    expect(api.pins[0]).toMatchObject({ id: 2, number: 1, color })
    expect(api.selectedId).toBe(0)
  })
})
