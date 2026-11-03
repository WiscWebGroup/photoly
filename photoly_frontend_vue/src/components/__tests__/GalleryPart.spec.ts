import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, byText, mountWithApp, pageText } from '@/test/utils'

const created = new Date(2024, 5, 1).toLocaleDateString(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
})
import GalleryPart from '../GalleryPart.vue'

// Keep real navigation/guards without loading the entire home page for a component test.
vi.mock('@/views/HomeView.vue', () => ({ default: { render: () => null } }))

function mockGalleries(description?: string) {
  server.use(
    http.get(`${API}/gallery/getAll`, () =>
      ok([
        {
          gaId: 7,
          gaName: 'Summer',
          description,
          coverId: 3,
          coverColor: '#ff0000',
          createDate: '2024-06-01T10:00:00',
        },
      ]),
    ),
    http.get(`${API}/photo/getByGallery`, () => ok([])),
    http.get(`${API}/photo/getCovers`, () => ok([])),
    http.get(`${API}/namespace/getRoot`, () => ok([])),
    http.get(`${API}/photo-extra/getGlobalMapPhotoByGallery`, () => ok([])),
  )
}

describe('GalleryPart', () => {
  beforeEach(async () => {
    localStorage.setItem('HRD-Token', 'test-token')
    // Finish initial navigation before mounting: otherwise query updates can start
    // a lazy login-page import that outlives this test's environment.
    await router.push('/home/GalleryPart')
  })

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
    const queries = () =>
      calls.filter((c) => c.path === '/photo/getByGallery').map((c) => c.query.gaId)

    byText('Summer').click()
    await vi.waitFor(() =>
      expect(router.currentRoute.value.fullPath).toBe('/home/GalleryPart?gallery=7'),
    )
    await vi.waitFor(() => expect(queries()).toEqual(['7']))
    await vi.waitFor(() => expect(document.querySelector('h1')?.textContent).toContain('Summer'))
    expect(
      document.querySelector('[aria-label="Gallery path"] [aria-current=page]')?.textContent,
    ).toBe('Summer')

    byText('Galleries').click()
    await vi.waitFor(() => expect(router.currentRoute.value.fullPath).toBe('/home/GalleryPart'))
    // Phones also get the shared "Albums" heading, so look at every heading.
    const headings = () => [...document.querySelectorAll('h1')].map((h) => h.textContent).join()
    await vi.waitFor(() => expect(headings()).toContain('Galleries'))
    expect(queries()).toEqual(['7'])
  })

  it('opens the gallery selected by a deep link', async () => {
    mockGalleries()
    await router.push('/home/GalleryPart?gallery=7')
    const calls = recordRequests()
    mountWithApp(GalleryPart)

    await vi.waitFor(() => expect(document.querySelector('h1')?.textContent).toContain('Summer'))
    await vi.waitFor(() =>
      expect(
        calls.filter((c) => c.path === '/photo/getByGallery').map((c) => c.query.gaId),
      ).toEqual(['7']),
    )
    expect(router.currentRoute.value.fullPath).toBe('/home/GalleryPart?gallery=7')
  })
  it('shows multiline descriptions as text only in details and prefills the edit dialog', async () => {
    const description = '旅行 📷\n<b>plain text</b>'
    mockGalleries(description)
    mountWithApp(GalleryPart)
    await vi.waitFor(() => byText('Summer'))
    expect(pageText()).not.toContain(description)
    byText('Summer').click()
    await vi.waitFor(() =>
      expect(document.querySelector('[data-testid="gallery-description"]')?.textContent).toBe(
        description,
      ),
    )
    expect(document.querySelector('[data-testid="gallery-description"] b')).toBeNull()
    button('Edit').click()
    await vi.waitFor(() =>
      expect(document.querySelector<HTMLTextAreaElement>('#gallery-description')?.value).toBe(
        description,
      ),
    )
    button('Cancel').click()
  })

  it('edits a legacy gallery with no description and renders no empty detail block', async () => {
    mockGalleries()
    mountWithApp(GalleryPart)
    await vi.waitFor(() => byText('Summer'))
    byText('Summer').click()
    await vi.waitFor(() => button('Edit'))
    expect(document.querySelector('[data-testid="gallery-description"]')).toBeNull()
    button('Edit').click()
    await vi.waitFor(() =>
      expect(document.querySelector<HTMLTextAreaElement>('#gallery-description')?.value).toBe(''),
    )
    button('Cancel').click()
  })
})
