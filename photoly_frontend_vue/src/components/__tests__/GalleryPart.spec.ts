import { describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { byText, mountWithApp, pageText } from '@/test/utils'

const created = new Date(2024, 5, 1).toLocaleDateString(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
})
import GalleryPart from '../GalleryPart.vue'

function mockGalleries() {
  server.use(
    http.get(`${API}/gallery/getAll`, () =>
      ok([
        {
          gaId: 7,
          gaName: 'Summer',
          coverId: 3,
          coverColor: '#ff0000',
          createDate: '2024-06-01T10:00:00',
        },
      ]),
    ),
    http.get(`${API}/photo/getByGallery`, () => ok([])),
  )
}

describe('GalleryPart', () => {
  it('lists galleries with their creation date', async () => {
    mockGalleries()
    mountWithApp(GalleryPart)
    await vi.waitFor(() => expect(pageText()).toContain(`Created ${created}`))
  })

  it('opens a gallery with its photos, and goes back to the list', async () => {
    const calls = recordRequests()
    mockGalleries()
    mountWithApp(GalleryPart)
    await vi.waitFor(() => byText('Summer'))
    const all = () => calls.filter((c) => c.path === '/photo/getByGallery').map((c) => c.query.gaId)
    // The list first fetches each gallery's photos for its cover; count only what follows.
    await vi.waitFor(() => expect(all()).toEqual(['7']))
    const queries = () => all().slice(1)

    byText('Summer').click()
    await vi.waitFor(() => expect(queries()).toEqual(['7']))
    await vi.waitFor(() => expect(document.querySelector('h1')?.textContent).toContain('Summer'))
    expect(document.querySelector('[aria-current=page]')?.textContent).toBe('Summer')

    byText('Galleries').click()
    // Phones also get the shared "Albums" heading, so look at every heading.
    const headings = () => [...document.querySelectorAll('h1')].map((h) => h.textContent).join()
    await vi.waitFor(() => expect(headings()).toContain('Galleries'))
    expect(queries()).toEqual(['7'])
  })
})
