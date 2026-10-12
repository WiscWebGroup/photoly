import { describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, byText, mountWithApp, pageText } from '@/test/utils'
import MoveToShowFolder from '../MoveToShowFolder.vue'

function mockFolders() {
  server.use(
    http.get(`${API}/namespace/getRoot`, () => ok({ nsId: 1, nsName: 'root' })),
    http.get(`${API}/namespace/getChildren`, ({ request }) =>
      new URL(request.url).searchParams.get('parentId') === '1'
        ? ok([
            { nsId: 5, nsName: 'Trips' },
            { nsId: 6, nsName: 'Pets' },
          ])
        : ok([]),
    ),
  )
}

async function open(props = {}) {
  const wrapper = mountWithApp<{ openModal(): void }>(MoveToShowFolder, props)
  wrapper.vm.openModal()
  await vi.waitFor(() => expect(pageText()).toContain('Move to a New Location'))
  await vi.waitFor(() => byText('root'))
  return wrapper
}

async function openAndExpandRoot(props = {}) {
  const wrapper = await open(props)
  byText('root').click()
  await vi.waitFor(() => byText('Trips'))
  return wrapper
}

describe('MoveToShowFolder', () => {
  it('loads the root folder but stays closed until openModal()', async () => {
    const calls = recordRequests()
    mockFolders()
    mountWithApp(MoveToShowFolder)
    await vi.waitFor(() => expect(calls.map((c) => c.path)).toContain('/namespace/getRoot'))

    expect(pageText()).not.toContain('Move to a New Location')
  })

  it('lazy-loads children and emits the selected folder on Update', async () => {
    const calls = recordRequests()
    mockFolders()
    const wrapper = await openAndExpandRoot()
    expect(calls).toContainEqual(
      expect.objectContaining({ path: '/namespace/getChildren', query: { parentId: '1' } }),
    )

    byText('Trips').click()
    await vi.waitFor(() => expect(pageText()).toContain('Currently Selected: Trips'))
    button('Update').click()

    expect(wrapper.emitted('moveToUpdate')).toEqual([[5]])
    await vi.waitFor(() => expect(pageText()).not.toContain('Move to a New Location'))
  })

  it('emits -1 when nothing is selected', async () => {
    mockFolders()
    const wrapper = await open()
    button('Update').click()

    expect(wrapper.emitted('moveToUpdate')).toEqual([[-1]])
  })

  it('Cancel closes without emitting', async () => {
    mockFolders()
    const wrapper = await open()
    button('Cancel').click()

    await vi.waitFor(() => expect(pageText()).not.toContain('Move to a New Location'))
    expect(wrapper.emitted('moveToUpdate')).toBeUndefined()
  })

  it('cannot select the folder being moved when disableOrgNsId is set', async () => {
    mockFolders()
    const wrapper = await openAndExpandRoot({ originalNsId: 5, disableOrgNsId: true })

    byText('Pets').click()
    await vi.waitFor(() => expect(pageText()).toContain('Currently Selected: Pets'))
    byText('Trips').click()
    button('Update').click()

    expect(wrapper.emitted('moveToUpdate')).toEqual([[6]])
  })
})
