import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import router from '@/router'
import { API, json, recordRequests, server } from '@/test/server'
import { button, input, mountWithApp, pageText, type } from '@/test/utils'
import Signup from '../Signup.vue'

function fillForm(username: string, email: string, password: string) {
  type(input('Username'), username)
  type(input('Email'), email)
  type(input('Password'), password)
}

// Navigation tests only need the route to change; skip loading the whole app behind it.
vi.mock('@/views/HomeView.vue', () => ({ default: { render: () => null } }))

describe('Signup', () => {
  beforeEach(async () => {
    await router.push('/signup')
  })

  it('requires a username, valid email and password', async () => {
    const calls = recordRequests()
    mountWithApp(Signup)
    fillForm('', 'alice@example.com', 'secret')
    button('Signup').click()
    await flushPromises()

    expect(pageText()).toContain('incorrect email, blank username or password format!')
    expect(calls).toEqual([])
  })

  it('stores the token and goes home on success', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/user/signUp/`, () => json({ msgCode: 200, token: 'tok-new' })))
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret')
    button('Signup').click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/home/HomePart'))

    expect(calls).toMatchObject([
      {
        method: 'POST',
        path: '/user/signUp/',
        body: { userName: 'alice', email: 'alice@example.com', password: 'secret' },
      },
    ])
    expect(localStorage.getItem('HRD-Token')).toBe('tok-new')
  })

  it('reports when registration is closed', async () => {
    server.use(
      http.post(`${API}/user/signUp/`, () =>
        json({ msgCode: 403, t: 'Register is closed by Admin' }),
      ),
    )
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret')
    button('Signup').click()

    await vi.waitFor(() => expect(pageText()).toContain('Register Closed!'))
    expect(localStorage.getItem('HRD-Token')).toBeNull()
  })

  it('reports other failures as bad credentials', async () => {
    server.use(http.post(`${API}/user/signUp/`, () => json({ msgCode: 500 })))
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret')
    button('Signup').click()

    await vi.waitFor(() => expect(pageText()).toContain('Wrong Email or Password!'))
  })

  it('links back to login', async () => {
    mountWithApp(Signup)
    button('Already have an account?').click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })
})
