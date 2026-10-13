import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, byText, input, mountWithApp, pageText, type } from '@/test/utils'
import PhotoViewer from '../PhotoViewer.vue'

const photo = {
  photoId: 10,
  photoName: 'beach',
  format: 'jpg',
  nsId: 5,
  uploadDate: '2024-06-01',
  visibility: 0,
}

async function openViewer() {
  const wrapper = mountWithApp(PhotoViewer, { photo, nsPath: '/root' })
  await vi.waitFor(() => expect(pageText()).toContain('Name: beach.jpg'))
  return wrapper
}

describe('PhotoViewer', () => {
  beforeEach(() => {
    server.use(
      http.get(`${API}/photo/getTagByPhoto`, () => ok([{ tag_id: 3, tag_name: 'Sunset' }])),
      http.get(`${API}/photo/getGalleryByPhoto`, () => ok([])),
      http.get(`${API}/photo/getPhotoExif`, () => ok({ GPS_Latitude: '', Model: 'X100' })),
      http.post(`${API}/photo/update`, () => ok()),
      http.post(`${API}/photo/deleteTag`, () => ok()),
    )
  })

  it('renames the photo, shows the new name and tells the parent', async () => {
    const calls = recordRequests()
    const wrapper = await openViewer()
    type(input('New Name'), 'sunset')
    button('Update').click()

    await vi.waitFor(() => expect(pageText()).toContain('Name: sunset.jpg'))
    expect(calls.find((c) => c.path === '/photo/update')?.body).toEqual({
      photoId: 10,
      photoName: 'sunset',
    })
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('rejects an empty name without calling the API', async () => {
    const calls = recordRequests()
    await openViewer()
    type(input('New Name'), '')
    button('Update').click()

    await vi.waitFor(() => expect(pageText()).toContain('Image name cannot be empty!'))
    expect(calls.map((c) => c.path)).not.toContain('/photo/update')
  })

  it('toggles visibility', async () => {
    const calls = recordRequests()
    await openViewer()
    document.querySelector<HTMLElement>('[aria-label=Visibility]')!.click()

    await vi.waitFor(() => expect(pageText()).toContain('Shared: Yes'))
    expect(calls.find((c) => c.path === '/photo/update')?.body).toEqual({
      photoId: 10,
      visibility: 1,
    })
  })

  it('detaches a tag', async () => {
    const calls = recordRequests()
    await openViewer()
    await vi.waitFor(() => byText('Sunset'))
    document.querySelector<HTMLElement>('[aria-label="Remove Sunset"]')!.click()

    await vi.waitFor(() => expect(calls.map((c) => c.path)).toContain('/photo/deleteTag'))
    expect(calls.find((c) => c.path === '/photo/deleteTag')?.query).toEqual({
      photoId: '10',
      tagId: '3',
    })
  })
})
