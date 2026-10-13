import { describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { byText, mountWithApp, pageText } from '@/test/utils'
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
    await vi.waitFor(() => expect(pageText()).toContain('Created: 2024-06-01'))
  })

  it('loads a gallery on click and clears it on a second click', async () => {
    const calls = recordRequests()
    mockGalleries()
    mountWithApp(GalleryPart)
    await vi.waitFor(() => byText('Summer'))
    const queries = () =>
      calls.filter((c) => c.path === '/photo/getByGallery').map((c) => c.query.gaId)

    byText('Summer').click()
    await vi.waitFor(() => expect(queries()).toEqual(['7']))
    expect(byText('Summer').closest('button')?.getAttribute('aria-pressed')).toBe('true')

    byText('Summer').click()
    await vi.waitFor(() =>
      expect(byText('Summer').closest('button')?.getAttribute('aria-pressed')).toBe('false'),
    )
    expect(queries()).toEqual(['7'])
  })
})
