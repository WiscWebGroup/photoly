import { describe, it, expect, vi } from 'vitest'
import { http } from 'msw'
import { API, server, ok, json, recordRequests } from '@/test/server'
import { mountWithApp, button } from '@/test/utils'
import DuplicateGroupCard from '../DuplicateGroupCard.vue'
const group = {
  key: 'a'.repeat(64) + ':still',
  version: 'b'.repeat(32),
  count: 3,
  keptCount: 1,
  ignored: false,
}
const member = (id: number, kept = false) => ({
  photoId: id,
  photoName: `Photo ${id}`,
  fileName: `${id}.png`,
  format: 'png',
  nsId: 1,
  sourceId: 1,
  contentVersion: 1,
  bytes: 1000,
  kept,
  folder: '/',
  sourceName: 'Photoly',
  sourceType: 'local',
})
describe('DuplicateGroupCard', () => {
  it('starts unselected, protects Keep and sends versioned changes', async () => {
    const calls = recordRequests()
    server.use(
      http.get(`${API}/photo/duplicates/members`, () =>
        ok({ items: [member(1, true), member(2), member(3)], total: 3, version: group.version }),
      ),
      http.post(`${API}/photo/duplicates/keep`, () => ok({ ok: true })),
    )
    const w = mountWithApp(DuplicateGroupCard, { group, disabled: false })
    await vi.waitFor(() => expect(w.findAll('input')).toHaveLength(3))
    expect(w.findAll('input').every((n) => !(n.element as HTMLInputElement).checked)).toBe(true)
    expect(w.findAll('input')[0]!.attributes('disabled')).toBeDefined()
    button('Select other copies').click()
    await vi.waitFor(() => expect(w.emitted('select')?.at(-1)?.[0]).toEqual([member(2), member(3)]))
    await w.get('[aria-label="Unmark 1.png"]').trigger('click')
    await vi.waitFor(() => expect(w.emitted('changed')).toHaveLength(1))
    expect(calls.find((c) => c.path.endsWith('/keep'))?.body).toEqual({
      key: group.key,
      version: group.version,
      value: false,
      photoId: 1,
    })
  })
  it('shows stale results without reporting success', async () => {
    server.use(
      http.get(`${API}/photo/duplicates/members`, () =>
        ok({ items: [member(1), member(2)], total: 2, version: group.version }),
      ),
      http.post(`${API}/photo/duplicates/ignore`, () =>
        json({ msgCode: 409, t: 'duplicate_changed' }),
      ),
    )
    const w = mountWithApp(DuplicateGroupCard, {
      group: { ...group, keptCount: 0 },
      disabled: false,
    })
    await vi.waitFor(() => expect(w.findAll('input')).toHaveLength(2))
    expect(button('Select other copies').disabled).toBe(true)
    button('Ignore group').click()
    await vi.waitFor(() => expect(w.get('[role="alert"]').text()).toContain('duplicates changed'))
    expect(w.emitted('changed')).toBeUndefined()
  })
})
