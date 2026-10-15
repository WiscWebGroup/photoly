import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, ok, recordRequests, server } from '@/test/server'
import { mountWithApp } from '@/test/utils'
import AppShell from '../AppShell.vue'

describe('AppShell', () => {
  beforeEach(async () => {
    localStorage.setItem('HRD-Token', 'tok')
    await router.push('/')
  })

  it('links to every section, (sidebar and tab bar)', () => {
    mountWithApp(AppShell)
    for (const part of ['HomePart', 'MapPart', 'GalleryPart', 'TagPart', 'SearchPart', 'ManagePart']) {
      expect(document.querySelectorAll(`a[href="/home/${part}"]`).length).toBeGreaterThanOrEqual(2)
    }
    expect(document.querySelector('img[alt="Photoly"]')).not.toBeNull()
  })

  it('logout signs out on the server, forgets the token and goes to the welcome page', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/user/signOut/`, () => ok()))
    await router.push('/login')
    mountWithApp(AppShell)
    document.querySelector<HTMLButtonElement>('button[aria-label="Logout"]')!.click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/'))
    expect(calls.map((c) => c.path)).toEqual(['/user/signOut/'])
    expect(localStorage.getItem('HRD-Token')).toBeNull()
  })
})
