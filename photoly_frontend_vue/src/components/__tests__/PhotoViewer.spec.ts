import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { stubCanvas, stubImage } from '@/test/canvas'
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
  await vi.waitFor(() => expect(pageText()).toContain('Uploaded 2024-06-01'))
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

    await vi.waitFor(() => expect(pageText()).toContain('sunset.jpg'))
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

    await vi.waitFor(() => expect(pageText()).toContain('Anyone with the link can see this'))
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

  describe('image editor', () => {
    beforeEach(() => {
      stubCanvas()
      stubImage(400, 300)
      server.use(
        http.get(`${API}/photo/render/:token`, () =>
          HttpResponse.arrayBuffer(new ArrayBuffer(2048), {
            headers: { 'Access-Control-Allow-Origin': '*' },
          }),
        ),
        http.post(`${API}/photo/insert`, () => ok(1)),
        http.post(`${API}/photo/replace`, () => ok(1)),
      )
    })

    async function openEditor(props = photo) {
      const wrapper = mountWithApp(PhotoViewer, { photo: props, nsPath: '/root' })
      await vi.waitFor(() => expect(pageText()).toContain('Uploaded 2024-06-01'))
      button('Edit Image').click()
      await vi.waitFor(() => expect(pageText()).toContain('Original: 2.0 KB'))
      return wrapper
    }
    const bodyOf = (calls: ReturnType<typeof recordRequests>, path: string) =>
      String(calls.find((c) => c.path === path)?.body)

    it('only offers editing for formats the browser can re-encode', async () => {
      mountWithApp(PhotoViewer, { photo: { ...photo, format: 'gif' }, nsPath: '/root' })
      await vi.waitFor(() => expect(pageText()).toContain('Uploaded 2024-06-01'))
      expect(() => button('Edit Image')).toThrow()
    })

    it('has nothing to save until something is edited', async () => {
      await openEditor()
      expect(button('Save as new image').disabled).toBe(true)
      expect(button('Overwrite original').disabled).toBe(true)
      button('Right').click()
      await vi.waitFor(() => expect(button('Save as new image').disabled).toBe(false))
      expect(pageText()).toContain('Cropped size: 300 × 400 px')
    })

    it('saves the edit as a new private image next to the original', async () => {
      const calls = recordRequests()
      const wrapper = await openEditor()
      button('Left').click()
      await vi.waitFor(() => expect(button('Save as new image').disabled).toBe(false))
      button('Save as new image').click()

      await vi.waitFor(() => expect(wrapper.emitted('changed')).toHaveLength(1))
      const body = bodyOf(calls, '/photo/insert')
      expect(body).toContain('filename="beach_edited.jpg"')
      expect(body).toContain('{"nsId":5,"visibility":0}')
      expect(calls.map((c) => c.path)).not.toContain('/photo/replace')
      // Back on the details view.
      await vi.waitFor(() => expect(pageText()).toContain('Uploaded 2024-06-01'))
    })

    it('saves the new image under a name the user picks', async () => {
      const calls = recordRequests()
      await openEditor()
      expect((input('New image name') as HTMLInputElement).value).toBe('beach_edited')
      button('Right').click()
      type(input('New image name'), '  beach at dusk ')
      await vi.waitFor(() => expect(button('Save as new image').disabled).toBe(false))
      button('Save as new image').click()

      await vi.waitFor(() =>
        expect(bodyOf(calls, '/photo/insert')).toContain('filename="beach at dusk.jpg"'),
      )
    })

    it('does not accept an empty or path-like new name', async () => {
      await openEditor()
      button('Right').click()
      for (const bad of ['', '   ', 'a/b', 'a\\b']) {
        type(input('New image name'), bad)
        await vi.waitFor(() => expect(pageText()).toContain('Enter a name without slashes'))
        expect(button('Save as new image').disabled).toBe(true)
        // Renaming only affects "save as new": overwriting keeps the original name.
        expect(button('Overwrite original').disabled).toBe(false)
      }
      type(input('New image name'), 'ok')
      await vi.waitFor(() => expect(button('Save as new image').disabled).toBe(false))
    })

    it('asks before overwriting, then replaces the original in place', async () => {
      const calls = recordRequests()
      const wrapper = await openEditor()
      button('Right').click()
      await vi.waitFor(() => expect(button('Overwrite original').disabled).toBe(false))
      button('Overwrite original').click()
      await vi.waitFor(() => expect(pageText()).toContain('Overwrite the original?'))
      expect(calls.map((c) => c.path)).not.toContain('/photo/replace')
      button('Overwrite').click()

      await vi.waitFor(() => expect(wrapper.emitted('changed')).toHaveLength(1))
      const body = bodyOf(calls, '/photo/replace')
      expect(body).toContain('name="photoId"')
      expect(body).toContain('filename="beach.jpg"')
      expect(calls.map((c) => c.path)).not.toContain('/photo/insert')
    })

    it('compresses to the chosen file size and shows the result', async () => {
      await openEditor()
      document.querySelector<HTMLElement>('[aria-label="Limit file size"]')!.click()
      await vi.waitFor(() => input('Max size'))
      type(input('Max size'), '40')
      // 400 × 300 px at full quality is 120 KB in the stubbed encoder; 40 KB needs quality ~0.33.
      await vi.waitFor(() => expect(pageText()).toMatch(/Result: 4\d\.\d KB|Result: 3\d\.\d KB/))
      expect(pageText()).toMatch(/quality 3\d%/)
      expect(button('Save as new image').disabled).toBe(false)
    })

    it('turns a PNG into a JPEG when a size limit is set', async () => {
      const calls = recordRequests()
      await openEditor({ ...photo, format: 'png' })
      document.querySelector<HTMLElement>('[aria-label="Limit file size"]')!.click()
      await vi.waitFor(() => expect(pageText()).toContain('saved as JPEG'))
      type(input('Max size'), '40')
      await vi.waitFor(() => expect(button('Save as new image').disabled).toBe(false))
      button('Save as new image').click()
      await vi.waitFor(() =>
        expect(bodyOf(calls, '/photo/insert')).toContain('filename="beach_edited.jpg"'),
      )
    })

    it('rejects an empty size limit', async () => {
      await openEditor()
      document.querySelector<HTMLElement>('[aria-label="Limit file size"]')!.click()
      await vi.waitFor(() => input('Max size'))
      type(input('Max size'), '')
      await vi.waitFor(() => expect(pageText()).toContain('Enter a size above 0.'))
      expect(button('Save as new image').disabled).toBe(true)
    })

    it('stays in the editor and reports a failed save', async () => {
      server.use(http.post(`${API}/photo/replace`, () => json({ msgCode: 400, t: 0 })))
      const wrapper = await openEditor()
      button('Right').click()
      await vi.waitFor(() => expect(button('Overwrite original').disabled).toBe(false))
      button('Overwrite original').click()
      await vi.waitFor(() => button('Overwrite'))
      button('Overwrite').click()
      await vi.waitFor(() => expect(pageText()).toContain('Overwriting the image failed'))
      expect(wrapper.emitted('changed')).toBeUndefined()
      expect(pageText()).toContain('Edit Image')
    })

    it('goes back to the details on cancel', async () => {
      await openEditor()
      button('Cancel').click()
      await vi.waitFor(() => expect(pageText()).toContain('Uploaded 2024-06-01'))
    })
  })
})
