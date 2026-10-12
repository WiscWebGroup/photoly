import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import router from '@/router'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, input, mountWithApp, pageText, type } from '@/test/utils'
import Login from '../Login.vue'

describe('Login', () => {
  beforeEach(async () => {
    await router.push('/login')
  })

  it('rejects a malformed email without calling the API', async () => {
    const calls = recordRequests()
    mountWithApp(Login)
    type(input('Email'), 'not-an-email')
    type(input('Password'), 'secret')
    button('Login').click()
    await flushPromises()

    expect(pageText()).toContain('Please input correct email and password format!')
    expect(calls).toEqual([])
  })

  it('rejects an empty password', async () => {
    mountWithApp(Login)
    type(input('Email'), 'alice@example.com')
    button('Login').click()
    await flushPromises()

    expect(pageText()).toContain('Please input correct email and password format!')
  })

  it('stores the token and goes home on success', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/user/signIn/`, () => json({ msgCode: 200, token: 'tok-123' })))
    mountWithApp(Login)
    type(input('Email'), 'alice@example.com')
    type(input('Password'), 'secret')
    button('Login').click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/home/HomePart'))

    expect(calls).toMatchObject([
      {
        method: 'POST',
        path: '/user/signIn/',
        body: { email: 'alice@example.com', password: 'secret' },
      },
    ])
    expect(localStorage.getItem('HRD-Token')).toBe('tok-123')
  })

  it('shows an error and stays put on bad credentials', async () => {
    server.use(http.post(`${API}/user/signIn/`, () => json({ msgCode: 401 })))
    mountWithApp(Login)
    type(input('Email'), 'alice@example.com')
    type(input('Password'), 'wrong')
    input('Password').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))

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

  it('links to signup', async () => {
    mountWithApp(Login)
    button('Signup').click()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/signup')
  })
})
