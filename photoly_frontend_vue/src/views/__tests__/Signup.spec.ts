import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import router from '@/router'
import { API, json, recordRequests, server } from '@/test/server'
import { button, field, link, mountWithApp, pageText, type } from '@/test/utils'
import Signup from '../Signup.vue'

function fillForm(username: string, email: string, password: string, confirm = password) {
  type(field('Username'), username)
  type(field('Email'), email)
  type(field('Password'), password)
  type(field('Confirm Password'), confirm)
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
    button('Create Account').click()
    await flushPromises()

    expect(pageText()).toContain('incorrect email, blank username or password format!')
    expect(calls).toEqual([])
  })

  it('blocks submission until the passwords match', async () => {
    const calls = recordRequests()
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret-1', 'secret-2')
    await flushPromises()
    expect(pageText()).toContain("Passwords don't match.")
    expect(field('Confirm Password').getAttribute('aria-invalid')).toBe('true')

    button('Create Account').click()
    await flushPromises()
    expect(calls).toEqual([])
  })

  it('waits for the confirmation to diverge before flagging it', async () => {
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret-1', 'sec')
    await flushPromises()
    expect(pageText()).not.toContain("Passwords don't match.")
  })

  it('rates password strength', async () => {
    mountWithApp(Signup)
    type(field('Password'), 'abc')
    await flushPromises()
    expect(document.querySelector('[role="meter"]')?.getAttribute('aria-valuetext')).toBe(
      'Too short',
    )
    type(field('Password'), 'Correct-Horse-9')
    await flushPromises()
    expect(document.querySelector('[role="meter"]')?.getAttribute('aria-valuetext')).toBe('Strong')
  })

  it('stores the token and goes home on success', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/user/signUp/`, () => json({ msgCode: 200, token: 'tok-new' })))
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret')
    button('Create Account').click()
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
    button('Create Account').click()

    await vi.waitFor(() => expect(pageText()).toContain('Register Closed!'))
    expect(localStorage.getItem('HRD-Token')).toBeNull()
  })

  it('reports other failures as bad credentials', async () => {
    server.use(http.post(`${API}/user/signUp/`, () => json({ msgCode: 500 })))
    mountWithApp(Signup)
    fillForm('alice', 'alice@example.com', 'secret')
    button('Create Account').click()

    await vi.waitFor(() => expect(pageText()).toContain('Wrong Email or Password!'))
  })

  it('links back to login', async () => {
    mountWithApp(Signup)
    link('Sign in').click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })
})
