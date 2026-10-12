import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, byText, mountWithApp, pageText } from '@/test/utils'
import MediaList from '../MediaList.vue'

type MediaListApi = { updatePhotoList(list: object[]): void; updateNsId(nsId: number): void }

const photos = () => [
  {
    photoId: 10,
    photoName: 'beach',
    format: 'jpg',
    nsId: 5,
    uploadDate: '2024-06-01',
    visibility: 0,
    checked: false,
  },
  {
    photoId: 11,
    photoName: 'dog',
    format: 'png',
    nsId: 5,
    uploadDate: '2024-06-02',
    visibility: 1,
    checked: false,
  },
]

function rightClick(el: Element) {
  el.dispatchEvent(
    new MouseEvent('contextmenu', { bubbles: true, button: 2, clientX: 10, clientY: 10 }),
  )
}

async function mountWithPhotos(props: Record<string, unknown> = {}) {
  const wrapper = mountWithApp<MediaListApi>(MediaList, {
    nsOrNot: false,
    detachEnabled: false,
    ...props,
  })
  wrapper.vm.updatePhotoList(photos())
  await vi.waitFor(() => byText('beach.jpg'))
  return wrapper
}

async function enterSelectMode() {
  document.querySelector<HTMLElement>('[role="switch"]')!.click()
  await vi.waitFor(() => button('Select All'))
}

describe('MediaList', () => {
  beforeEach(() => {
    server.use(
      http.get(`${API}/namespace/getRoot`, () => ok({ nsId: 1, nsName: 'root' })),
      http.get(`${API}/namespace/getChildren`, () => ok([{ nsId: 6, nsName: 'Trips' }])),
      http.get(`${API}/namespace/trace`, () =>
        ok([
          { nsId: 1, nsName: 'root' },
          { nsId: 5, nsName: 'Holiday' },
        ]),
      ),
      http.get(`${API}/photo/getTagByPhoto`, () => ok([{ tag_id: 3, tag_name: 'Sunset' }])),
      http.get(`${API}/photo/getGalleryByPhoto`, () => ok([{ ga_id: 7, ga_name: 'Summer' }])),
      http.get(`${API}/photo/getPhotoExif`, () => ok({ File_Size: '2 MB' })),
      http.post(`${API}/photo/delete`, () => ok()),
      http.post(`${API}/photo/changeNamespace`, () => ok()),
    )
  })

  it('renders the list passed to updatePhotoList()', async () => {
    const wrapper = await mountWithPhotos()
    expect(pageText()).toContain('dog.png')

    wrapper.vm.updatePhotoList([])
    await vi.waitFor(() => expect(pageText()).not.toContain('beach.jpg'))
  })

  it('opens photo details on click, tracing the photo folder when nsOrNot is false', async () => {
    const calls = recordRequests()
    await mountWithPhotos()
    byText('beach.jpg').click()

    await vi.waitFor(() => expect(pageText()).toContain('Path: /root/Holiday'))
    expect(pageText()).toContain('Name: beach.jpg')
    expect(pageText()).toContain('Upload Date: 2024-06-01')
    expect(pageText()).toContain('Size: 2 MB')
    expect(pageText()).toContain('Shared: No')
    expect(calls).toContainEqual(
      expect.objectContaining({ path: '/namespace/trace', query: { nsId: '5' } }),
    )
    for (const path of ['/photo/getTagByPhoto', '/photo/getGalleryByPhoto', '/photo/getPhotoExif'])
      expect(calls).toContainEqual(expect.objectContaining({ path, query: { photoId: '10' } }))

    byText('Tags').click()
    await vi.waitFor(() => expect(pageText()).toContain('Sunset'))
  })

  it('updateNsId() traces the folder up front when nsOrNot is true', async () => {
    const calls = recordRequests()
    const wrapper = await mountWithPhotos({ nsOrNot: true })
    wrapper.vm.updateNsId(5)
    await vi.waitFor(() =>
      expect(calls).toContainEqual(
        expect.objectContaining({ path: '/namespace/trace', query: { nsId: '5' } }),
      ),
    )

    byText('beach.jpg').click()
    await vi.waitFor(() => expect(pageText()).toContain('Path: /root/Holiday'))
    expect(calls.filter((c) => c.path === '/namespace/trace')).toHaveLength(1)
  })

  it('deletes all selected photos and asks the parent to reload', async () => {
    const calls = recordRequests()
    const wrapper = await mountWithPhotos()
    await enterSelectMode()
    button('Select All').click()
    button('Delete Selected').click()
    await vi.waitFor(() => expect(pageText()).toContain('Confirm to Delete'))
    button('Delete').click()

    await vi.waitFor(() => expect(wrapper.emitted('queryPhotos')).toHaveLength(2))
    expect(calls.filter((c) => c.path === '/photo/delete').map((c) => c.query.photoId)).toEqual([
      '10',
      '11',
    ])
    expect(pageText()).toContain('Photo deleted!')
  })

  it('moves selected photos to the folder picked in the move dialog', async () => {
    const calls = recordRequests()
    const wrapper = await mountWithPhotos()
    await enterSelectMode()
    button('Select All').click()
    button('Move').click()
    await vi.waitFor(() => byText('root'))
    byText('root').click()
    await vi.waitFor(() => byText('Trips'))
    byText('Trips').click()
    button('Update').click()

    await vi.waitFor(() => expect(wrapper.emitted('queryPhotos')).toHaveLength(2))
    expect(calls.filter((c) => c.path === '/photo/changeNamespace').map((c) => c.query)).toEqual([
      { photoId: '10', nsId: '6' },
      { photoId: '11', nsId: '6' },
    ])
    expect(pageText()).toContain('Media Location Updated!')
  })

  it('emits doDetach with the photo list when detach is enabled', async () => {
    const wrapper = await mountWithPhotos({ detachEnabled: true, detachStr: 'Tag' })
    await enterSelectMode()
    button('Remove From Tag').click()
    await vi.waitFor(() => expect(pageText()).toContain('Confirm to Detach'))
    button('Confirm').click()

    const [[detached]] = wrapper.emitted('doDetach') as [[{ photoId: number }[]]]
    expect(detached.map((p) => p.photoId)).toEqual([10, 11])
  })

  it('hides the detach action when detach is disabled', async () => {
    await mountWithPhotos()
    await enterSelectMode()
    expect(pageText()).not.toContain('Remove From')
  })

  it('deletes a single photo from its context menu', async () => {
    const calls = recordRequests()
    const wrapper = await mountWithPhotos()
    rightClick(byText('dog.png'))
    await vi.waitFor(() => byText('Delete'))
    byText('Delete').click()

    await vi.waitFor(() => expect(wrapper.emitted('queryPhotos')).toHaveLength(1))
    expect(calls.filter((c) => c.path === '/photo/delete').map((c) => c.query)).toEqual([
      { photoId: '11' },
    ])
  })
})
