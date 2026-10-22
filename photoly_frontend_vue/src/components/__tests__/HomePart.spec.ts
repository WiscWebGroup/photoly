import { beforeEach, describe, expect, it, vi } from 'vitest'
import { delay, http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, byText, input, mountWithApp, pageText, type } from '@/test/utils'
import { viewMode } from '@/composables/useViewMode'
import HomePart from '../HomePart.vue'

async function mountHome() {
  const wrapper = mountWithApp(HomePart)
  await vi.waitFor(() => byText('Trips'))
  return wrapper
}

const ariaButton = (label: string) =>
  document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!

describe('HomePart', () => {
  beforeEach(() => {
    viewMode.value = 'grid'
    server.use(
      http.get(`${API}/namespace/getRoot`, () => ok({ nsId: 1, nsName: 'root' })),
      http.get(`${API}/namespace/getChildren`, () =>
        ok([
          { nsId: 5, nsName: 'Trips' },
          { nsId: 6, nsName: 'Pets' },
        ]),
      ),
      http.get(`${API}/namespace/trace`, () => ok([{ nsId: 1, nsName: 'root' }])),
      http.get(`${API}/photo/getByNamespace`, () => ok([])),
      http.post(`${API}/namespace/insert`, () => ok()),
      http.post(`${API}/namespace/delete`, () => ok()),
    )
  })

  it('loads the root folder with its subfolders', async () => {
    const calls = recordRequests()
    await mountHome()
    expect(pageText()).toContain('Pets')
    expect(calls).toContainEqual(
      expect.objectContaining({ path: '/photo/getByNamespace', query: { nsId: '1' } }),
    )
  })

  it('shows a progress bar while photos upload, then closes the dialog', async () => {
    server.use(
      http.post(`${API}/photo/inserts`, async () => {
        await delay(100)
        return ok()
      }),
    )
    await mountHome()
    button('Upload').click()
    await vi.waitFor(() => expect(document.querySelector('input[type=file]')).not.toBeNull())
    const picker = document.querySelector<HTMLInputElement>('input[type=file]')!
    Object.defineProperty(picker, 'files', {
      value: [new File(['x'], 'beach.jpg', { type: 'image/jpeg' })],
    })
    picker.dispatchEvent(new Event('change', { bubbles: true }))
    await vi.waitFor(() => byText('beach.jpg'))
    expect(document.querySelector('[role=progressbar]')).toBeNull()

    button('Upload', document.querySelector('[role=dialog]')!).click()
    await vi.waitFor(() => expect(document.querySelector('[role=progressbar]')).not.toBeNull())
    await vi.waitFor(() => expect(pageText()).toContain('Upload successful!'))
    expect(document.querySelector('[role=progressbar]')).toBeNull()
  })

  it('rejects reserved folder names without calling the API', async () => {
    const calls = recordRequests()
    await mountHome()
    button('New Folder').click()
    await vi.waitFor(() => input('Folder Name'))
    type(input('Folder Name'), 'root')
    button('Create').click()

    await vi.waitFor(() => expect(pageText()).toContain('Please input a valid folder name!'))
    expect(calls.map((c) => c.path)).not.toContain('/namespace/insert')
  })

  it('deletes every selected folder after confirmation', async () => {
    const calls = recordRequests()
    await mountHome()
    byText('Trips').dispatchEvent(
      new MouseEvent('contextmenu', { bubbles: true, button: 2, clientX: 10, clientY: 10 }),
    )
    await vi.waitFor(() => byText('Select'))
    byText('Select').click()
    await vi.waitFor(() => button('Select All'))
    button('Select All').click()
    button('Delete', document.querySelector('[role=toolbar]')!).click()
    await vi.waitFor(() => expect(pageText()).toContain('Delete 2 items?'))
    button('Delete', document.querySelector('[role=alertdialog]')!).click()

    await vi.waitFor(() =>
      expect(calls.filter((c) => c.path === '/namespace/delete')).toHaveLength(2),
    )
    expect(
      calls
        .filter((c) => c.path === '/namespace/delete')
        .map((c) => c.query.nsId)
        .sort(),
    ).toEqual(['5', '6'])
  })

  it('switches the folders between grid and list layouts and remembers the choice', async () => {
    await mountHome()
    const folders = () => byText('Trips').closest('ul')!
    expect(folders().className).toContain('grid')
    expect(ariaButton('Grid view').getAttribute('aria-pressed')).toBe('true')

    ariaButton('List view').click()
    await vi.waitFor(() => expect(folders().className).toContain('flex-col'))
    expect(folders().className).not.toContain('grid')
    expect(ariaButton('List view').getAttribute('aria-pressed')).toBe('true')
    expect(localStorage.getItem('photoly-view-mode')).toBe('list')

    ariaButton('Grid view').click()
    await vi.waitFor(() => expect(folders().className).toContain('grid'))
  })
})
