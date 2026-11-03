import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, ok, recordRequests, server } from '@/test/server'
import { mountWithApp } from '@/test/utils'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import AppShell from '../AppShell.vue'
import { byText } from '@/test/utils'

describe('AppShell', () => {
  beforeEach(async () => {
    localStorage.setItem('HRD-Token', 'tok')
    server.use(
      http.get(`${API}/user/getStorage`, () => ok({ usedBytes: 0, capacityBytes: 50000000000 })),
      http.get(`${API}/photo/mediaCapabilities`, () => ok({})),
      http.get(`${API}/user/getInfo/`, () =>
        ok({ userId: 1, userName: 'alice', email: 'a@b.c', createDate: '', role: 'user' }),
      ),
    )
    await router.push('/')
  })

  it('links to every section from the sidebar, and the main ones from the tab bar', () => {
    mountWithApp(AppShell)
    const links = (part: string) => document.querySelectorAll(`a[href="/home/${part}"]`).length
    // Sidebar, plus the phone tab bar (or the avatar, for the account).
    for (const part of ['HomePart', 'MapPart', 'GalleryPart', 'SearchPart', 'ManagePart'])
      expect(links(part)).toBeGreaterThanOrEqual(2)
    // Phones reach tags through the Albums tab.
    expect(links('TagPart')).toBeGreaterThanOrEqual(1)
    expect(document.querySelector('img[alt="Photoly"]')).not.toBeNull()
  })

  it('hides and restores the sidebar, retaining the preference on remount', async () => {
    const options = { global: { plugins: [createPinia(), router] }, attachTo: document.body }
    const wrapper = mount(AppShell, options)
    const toggle = () =>
      document.querySelector<HTMLButtonElement>('button[aria-controls="main-sidebar"]')!
    expect(toggle().getAttribute('aria-expanded')).toBe('true')
    toggle().click()
    await vi.waitFor(() => expect(localStorage.getItem('photoly-sidebar-hidden')).toBe('true'))
    expect(toggle().getAttribute('aria-label')).toBe('Show sidebar')
    expect(document.querySelector('#main-sidebar')!.getAttribute('aria-hidden')).toBe('true')
    expect(document.querySelector('#main-sidebar')!.hasAttribute('inert')).toBe(true)
    wrapper.unmount()
    mount(AppShell, options)
    expect(toggle().getAttribute('aria-expanded')).toBe('false')
    toggle().click()
    await vi.waitFor(() => expect(toggle().getAttribute('aria-expanded')).toBe('true'))
    expect(localStorage.getItem('photoly-sidebar-hidden')).toBe('false')
  })

  it('shows pins above Library, highlights only the current pin and supports unpinning', async () => {
    server.use(
      http.get(`${API}/namespace/pins`, () =>
        ok({ items: [{ nsId: 5, nsName: 'Trips', path: 'Library / Trips' }], defaultNsId: 5 }),
      ),
      http.post(`${API}/namespace/unpin`, () => ok({ items: [], defaultNsId: null })),
    )
    await router.push('/home/HomePart?folder=5')
    mountWithApp(AppShell)
    await vi.waitFor(() =>
      expect(document.querySelector('[aria-label="Pinned folders"]')).not.toBeNull(),
    )
    const sidebar = document.querySelector('#main-sidebar')!
    const pin = sidebar.querySelector('a[href="/home/HomePart?folder=5"]')!
    const photos = sidebar.querySelector('nav a[href="/home/HomePart"]')!
    expect(pin.getAttribute('aria-current')).toBe('page')
    expect(photos.hasAttribute('aria-current')).toBe(false)
    expect(sidebar.textContent!.indexOf('Pinned')).toBeLessThan(
      sidebar.textContent!.indexOf('Library'),
    )
    expect(pin.querySelector('[aria-label="Default location"]')).not.toBeNull()
    pin.dispatchEvent(
      new MouseEvent('contextmenu', { bubbles: true, button: 2, clientX: 10, clientY: 10 }),
    )
    await vi.waitFor(() => byText('Clear default location'))
    byText('Unpin').click()
    await vi.waitFor(() =>
      expect(sidebar.querySelector('[aria-label="Pinned folders"]')).toBeNull(),
    )
    expect(router.currentRoute.value.query.folder).toBe('5')
    expect(photos.getAttribute('aria-current')).toBe('page')
  })

  it('reloads pins when the window regains focus', async () => {
    let items: { nsId: number; nsName: string; path: string }[] = []
    server.use(http.get(`${API}/namespace/pins`, () => ok({ items, defaultNsId: null })))
    mountWithApp(AppShell)
    await vi.waitFor(() =>
      expect(document.querySelector('[aria-label="Pinned folders"]')).toBeNull(),
    )
    items = [{ nsId: 8, nsName: 'Other device', path: 'Library / Other device' }]
    window.dispatchEvent(new Event('focus'))
    await vi.waitFor(() =>
      expect(document.querySelector('[aria-label="Pinned folders"]')?.textContent).toContain(
        'Other device',
      ),
    )
  })

  it('"/" jumps to search unless typing in a field', async () => {
    await router.push('/home/HomePart')
    mountWithApp(AppShell)
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }))
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/home/SearchPart'))

    await router.push('/home/HomePart')
    const field = document.createElement('input')
    document.body.append(field)
    field.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }))
    await new Promise((r) => setTimeout(r, 20))
    expect(router.currentRoute.value.path).toBe('/home/HomePart')
    field.remove()
  })

  it('sign out signs out on the server, forgets the token and goes to the sign-in page', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/user/signOut/`, () => ok()))
    await router.push('/signup')
    mountWithApp(AppShell)
    document.querySelector<HTMLButtonElement>('button[aria-label="Sign out"]')!.click()
    // `/` is the demo landing page; a regular deployment lands on sign-in.
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
    expect(calls.filter((c) => c.method === 'POST').map((c) => c.path)).toEqual(['/user/signOut/'])
    expect(localStorage.getItem('HRD-Token')).toBeNull()
  })
})
