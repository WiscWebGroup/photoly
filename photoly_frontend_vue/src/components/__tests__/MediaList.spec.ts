import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, byText, mountWithApp, pageText } from '@/test/utils'
import { viewMode } from '@/composables/useViewMode'
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
  rightClick(byText('dog.png'))
  await vi.waitFor(() => byText('Select'))
  byText('Select').click()
  await vi.waitFor(() => button('Select All'))
}

/** Selects every photo; the bar's actions enable once something is selected. */
async function selectAll() {
  button('Select All').click()
  await vi.waitFor(() => expect(pageText()).toContain('2 selected'))
}

describe('MediaList', () => {
  beforeEach(() => {
    viewMode.value = 'grid'
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

    await vi.waitFor(() => expect(pageText()).toContain('Library / Holiday'))
    expect(pageText()).toContain('Uploaded')
    expect(pageText()).toContain('beach.jpg')
    expect(pageText()).toContain('2 MB')
    expect(calls).toContainEqual(
      expect.objectContaining({ path: '/namespace/trace', query: { nsId: '5' } }),
    )
    for (const path of ['/photo/getTagByPhoto', '/photo/getGalleryByPhoto', '/photo/getPhotoExif'])
      expect(calls).toContainEqual(
        expect.objectContaining({
          path,
          query: {
            photoId: '10',
            ...(/get(Tag|Gallery)ByPhoto$/.test(path) ? { includeSmart: 'true' } : {}),
          },
        }),
      )

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
    await vi.waitFor(() => expect(pageText()).toContain('Library / Holiday'))
    expect(calls.filter((c) => c.path === '/namespace/trace')).toHaveLength(1)
  })

  it('deletes all selected photos and asks the parent to reload', async () => {
    const calls = recordRequests()
    const wrapper = await mountWithPhotos()
    await enterSelectMode()
    await selectAll()
    button('Delete Selected').click()
    await vi.waitFor(() => expect(pageText()).toContain('Delete 2 items?'))
    button('Delete').click()

    // One reload after the whole batch, not one per photo.
    await vi.waitFor(() => expect(wrapper.emitted('queryPhotos')).toHaveLength(1))
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
    await selectAll()
    button('Move').click()
    await vi.waitFor(() => byText('Library'))
    byText('Library').click()
    await vi.waitFor(() => byText('Trips'))
    byText('Trips').click()
    await vi.waitFor(() => button('Move to Trips'))
    button('Move to Trips').click()

    await vi.waitFor(() => expect(wrapper.emitted('queryPhotos')).toHaveLength(1))
    expect(calls.filter((c) => c.path === '/photo/changeNamespace').map((c) => c.query)).toEqual([
      { photoId: '10', nsId: '6' },
      { photoId: '11', nsId: '6' },
    ])
    expect(pageText()).toContain('Media Location Updated!')
  })

  it('emits doDetach with the photo list when detach is enabled', async () => {
    const wrapper = await mountWithPhotos({ detachEnabled: true, detachStr: 'Tag' })
    await enterSelectMode()
    await selectAll()
    button('Remove From Tag').click()
    await vi.waitFor(() => expect(pageText()).toContain('Remove from this tag?'))
    button('Remove').click()

    const [[detached]] = wrapper.emitted('doDetach') as [[{ photoId: number }[]]]
    expect(detached.map((p) => p.photoId)).toEqual([10, 11])
  })

  it('groups photos by upload day when asked', async () => {
    await mountWithPhotos({ groupByDay: true })
    const day = (d: number) =>
      new Date(2024, 5, d).toLocaleDateString(undefined, {
        weekday: 'long',
        day: 'numeric',
        month: 'long',
        year: 'numeric',
      })
    // Newest day first.
    const headings = [...document.querySelectorAll('h3')].map((h) => h.textContent)
    expect(headings).toEqual([day(2), day(1)])
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

  it('lays photos out as a list with dates when list view is chosen', async () => {
    await mountWithPhotos()
    const photoList = () => byText('beach.jpg').closest('ul')!
    expect(photoList().className).toContain('grid')

    document.querySelector<HTMLButtonElement>('button[aria-label="List view"]')!.click()
    await vi.waitFor(() => expect(photoList().className).toContain('flex-col'))
    expect(photoList().className).not.toContain('grid')
    expect(pageText()).toContain(
      new Date(2024, 5, 1).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      }),
    )
  })

  it('has no layout toggle when the parent shows its own', async () => {
    await mountWithPhotos({ hideToggle: true })
    expect(document.querySelector('button[aria-label="List view"]')).toBeNull()
  })
})
