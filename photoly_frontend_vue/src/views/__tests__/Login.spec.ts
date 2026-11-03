import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import router from '@/router'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, field, mountWithApp, pageText, type } from '@/test/utils'
import Login from '../Login.vue'

// Navigation tests only need the route to change; skip loading the whole app behind it.
vi.mock('@/views/HomeView.vue', () => ({ default: { render: () => null } }))

describe('Login', () => {
  beforeEach(async () => {
    await router.push('/login')
  })

  it('rejects a malformed email without calling the API', async () => {
    const calls = recordRequests()
    mountWithApp(Login)
    type(field('Email'), 'not-an-email')
    type(field('Password'), 'secret')
    button('Sign In').click()
    await flushPromises()

    expect(pageText()).toContain('Please input correct email and password format!')
    expect(calls).toEqual([])
  })

  it('rejects an empty password', async () => {
    mountWithApp(Login)
    type(field('Email'), 'alice@example.com')
    button('Sign In').click()
    await flushPromises()

    expect(pageText()).toContain('Please input correct email and password format!')
  })

  it('stores the token and goes home on success', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/user/signIn/`, () => json({ msgCode: 200, token: 'tok-123' })))
    mountWithApp(Login)
    type(field('Email'), 'alice@example.com')
    type(field('Password'), 'secret')
    button('Sign In').click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/home/HomePart'))

    expect(calls).toMatchObject([
      {
        method: 'POST',
        path: '/user/signIn/',
        body: { email: 'alice@example.com', password: 'secret' },
      },
      { method: 'GET', path: '/maintenance/status' },
      { method: 'GET', path: '/namespace/pins' },
    ])
    expect(localStorage.getItem('HRD-Token')).toBe('tok-123')
  })

  it('shows an error and stays put on bad credentials', async () => {
    server.use(http.post(`${API}/user/signIn/`, () => json({ msgCode: 401 })))
    mountWithApp(Login)
    type(field('Email'), 'alice@example.com')
    type(field('Password'), 'wrong')
    field('Password').form!.requestSubmit()

    await vi.waitFor(() => expect(pageText()).toContain('Wrong Email or Password!'))
    expect(localStorage.getItem('HRD-Token')).toBeNull()
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('skips the form when a stored token is still valid', async () => {
    localStorage.setItem('HRD-Token', 'tok-123')
    let sentToken: string | null = null
    server.use(
      http.post(`${API}/user/pingToken/`, ({ request }) => {
        sentToken = request.headers.get('HRD-Token')
        return ok()
      }),
    )
    mountWithApp(Login)

    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/home/HomePart'))
    expect(sentToken).toBe('tok-123')
  })

  it('reveals the password on request', async () => {
    mountWithApp(Login)
    expect(field('Password').type).toBe('password')
    document.querySelector<HTMLButtonElement>('[aria-label="Show password"]')!.click()
    await flushPromises()
    expect(field('Password').type).toBe('text')
    expect(document.querySelector('[aria-label="Hide password"]')).not.toBeNull()
  })

  it('opens the saved default after login', async () => {
    server.use(
      http.post(`${API}/user/signIn/`, () => json({ msgCode: 200, token: 'tok' })),
      http.get(`${API}/namespace/pins`, () =>
        ok({ items: [{ nsId: 5, nsName: 'Trips', path: '/Trips' }], defaultNsId: 5 }),
      ),
    )
    mountWithApp(Login)
    type(field('Email'), 'alice@example.com')
    type(field('Password'), 'secret')
    button('Sign In').click()
    await vi.waitFor(() =>
      expect(router.currentRoute.value.fullPath).toBe('/home/HomePart?folder=5'),
    )
  })

  it('preserves a protected deep link through login instead of applying the default', async () => {
    await router.push('/home/HomePart?folder=8')
    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/home/HomePart?folder=8')
    server.use(
      http.post(`${API}/user/signIn/`, () => json({ msgCode: 200, token: 'tok' })),
      http.get(`${API}/namespace/pins`, () => ok({ items: [], defaultNsId: 5 })),
    )
    mountWithApp(Login)
    type(field('Email'), 'alice@example.com')
    type(field('Password'), 'secret')
    button('Sign In').click()
    await vi.waitFor(() =>
      expect(router.currentRoute.value.fullPath).toBe('/home/HomePart?folder=8'),
    )
  })
})
