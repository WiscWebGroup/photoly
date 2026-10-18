import { flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { byText, mountWithApp } from '@/test/utils'
import TagPart from '../TagPart.vue'

function mockTags() {
  server.use(
    http.get(`${API}/tag/getAll`, () =>
      ok([
        { tagId: 3, tagName: 'Sunset' },
        { tagId: 4, tagName: 'Beach' },
      ]),
    ),
    http.get(`${API}/photo/getByTags`, () => ok([])),
  )
}

describe('TagPart', () => {
  it('queries photos for every selected tag, and none once all are cleared', async () => {
    const calls = recordRequests()
    mockTags()
    mountWithApp(TagPart)
    await vi.waitFor(() => byText('Sunset'))
    const photoQueries = () =>
      calls.filter((c) => c.path === '/photo/getByTags').map((c) => c.query.tagIds)

    byText('Sunset').click()
    await vi.waitFor(() => expect(photoQueries()).toEqual(['3']))
    byText('Beach').click()
    await vi.waitFor(() => expect(photoQueries()).toEqual(['3', '3,4']))

    byText('Sunset').click()
    byText('Beach').click()
    await vi.waitFor(() => expect(photoQueries()).toEqual(['3', '3,4', '4']))
    expect(byText('Sunset').getAttribute('aria-pressed')).toBe('false')
    expect(byText('Beach').getAttribute('aria-pressed')).toBe('false')
  })

  it('deselects every tag at once', async () => {
    mockTags()
    mountWithApp(TagPart)
    await vi.waitFor(() => byText('Sunset'))
    expect(document.body.textContent).not.toContain('Deselect all')

    byText('Sunset').click()
    byText('Beach').click()
    await vi.waitFor(() => byText('Deselect all (2)'))
    byText('Deselect all (2)').click()
    await flushPromises()

    expect(byText('Sunset').getAttribute('aria-pressed')).toBe('false')
    expect(byText('Beach').getAttribute('aria-pressed')).toBe('false')
    // The button leaves through a transition, so it is removed a frame later.
    await vi.waitFor(() => expect(document.body.textContent).not.toContain('Deselect all'))
  })

  it('refuses to create an empty tag', async () => {
    const calls = recordRequests()
    mockTags()
    mountWithApp(TagPart)
    await vi.waitFor(() => byText('Sunset'))
    byText('Add').click()
    await flushPromises()

    expect(calls.map((c) => c.path)).not.toContain('/tag/insert')
  })
})
